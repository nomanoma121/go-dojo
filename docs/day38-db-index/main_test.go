package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	_ "github.com/lib/pq"
)

var testDB *sql.DB
var testDSN string

func TestMain(m *testing.M) {
	// Setup dockertest
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// Start PostgreSQL container
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "13",
		Env: []string{
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_USER=test",
			"POSTGRES_DB=testdb",
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	// Set expiration for the container
	if err := resource.Expire(120); err != nil {
		log.Fatalf("Could not set expiration: %s", err)
	}

	testDSN = fmt.Sprintf("postgres://test:secret@localhost:%s/testdb?sslmode=disable", resource.GetPort("5432/tcp"))

	// Connect to database
	if err := pool.Retry(func() error {
		var err error
		testDB, err = sql.Open("postgres", testDSN)
		if err != nil {
			return err
		}
		return testDB.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	// Setup test schema and data
	if err := setupTestDatabase(testDB); err != nil {
		log.Fatalf("Could not setup test database: %s", err)
	}

	if err := seedTestData(testDB, 1000, 5000); err != nil {
		log.Fatalf("Could not seed test data: %s", err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	testDB.Close()
	
	// Exit with the test result code
	if code != 0 {
		log.Fatalf("Tests failed with code: %d", code)
	}
}

func TestQueryAnalyzer_ExplainQuery(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	analyzer := NewQueryAnalyzer(testDB)

	// Test simple query
	query := "SELECT * FROM users WHERE email = $1"
	results, err := analyzer.ExplainQuery(query, "user1@example.com")
	if err != nil {
		t.Errorf("Failed to explain query: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected some explain results")
	}

	// Should have at least one result with node type
	found := false
	for _, result := range results {
		if result.NodeType != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected at least one result with NodeType")
	}
}

func TestQueryAnalyzer_AnalyzeQueryPlan(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	analyzer := NewQueryAnalyzer(testDB)

	// Create a query that should result in a sequential scan
	query := "SELECT * FROM users WHERE age > 30"
	results, err := analyzer.ExplainQuery(query, "")
	if err != nil {
		t.Errorf("Failed to explain query: %v", err)
	}

	analysis := analyzer.AnalyzeQueryPlan(results)

	if analysis.TotalCost <= 0 {
		t.Error("Expected positive total cost")
	}

	if analysis.RowsProcessed < 0 {
		t.Error("Expected non-negative rows processed")
	}

	// Should have some scan type
	if !analysis.HasSeqScan && !analysis.HasIndexScan {
		t.Error("Expected either sequential scan or index scan")
	}
}

func TestIndexAdvisor_Recommendations(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	advisor := NewIndexAdvisor(testDB)

	// Analyze queries that should trigger recommendations
	queries := []string{
		"SELECT * FROM users WHERE email = $1",
		"SELECT * FROM users WHERE city = $1",
		"SELECT * FROM orders WHERE user_id = $1",
		"SELECT * FROM orders WHERE status = $1",
	}

	args := [][]interface{}{
		{"user1@example.com"},
		{"Tokyo"},
		{1},
		{"pending"},
	}

	for i, query := range queries {
		err := advisor.AnalyzeQuery(query, args[i]...)
		if err != nil {
			t.Errorf("Failed to analyze query %d: %v", i, err)
		}
	}

	recommendations := advisor.GetRecommendations()

	if len(recommendations) == 0 {
		t.Error("Expected some index recommendations")
	}

	// Check if recommendations have required fields
	for _, rec := range recommendations {
		if rec.TableName == "" {
			t.Error("Expected table name in recommendation")
		}
		if len(rec.Columns) == 0 {
			t.Error("Expected columns in recommendation")
		}
		if rec.IndexType == "" {
			t.Error("Expected index type in recommendation")
		}
		if rec.Priority <= 0 {
			t.Error("Expected positive priority")
		}
	}
}

func TestIndexAdvisor_GenerateIndexSQL(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	advisor := NewIndexAdvisor(testDB)

	// Add a recommendation manually for testing
	advisor.recommendations = []IndexRecommendation{
		{
			TableName: "users",
			Columns:   []string{"email"},
			IndexType: "btree",
			Reason:    "Test recommendation",
			Priority:  1,
		},
		{
			TableName: "orders",
			Columns:   []string{"user_id", "status"},
			IndexType: "btree",
			Reason:    "Test composite index",
			Priority:  2,
		},
	}

	sqlStatements := advisor.GenerateIndexSQL()

	if len(sqlStatements) == 0 {
		t.Error("Expected some SQL statements")
	}

	for _, sql := range sqlStatements {
		if !strings.Contains(strings.ToUpper(sql), "CREATE INDEX") {
			t.Errorf("Expected CREATE INDEX in SQL: %s", sql)
		}
	}

	// Check first statement contains expected elements
	firstSQL := sqlStatements[0]
	if !strings.Contains(firstSQL, "users") {
		t.Error("Expected table name 'users' in first SQL statement")
	}
	if !strings.Contains(firstSQL, "email") {
		t.Error("Expected column 'email' in first SQL statement")
	}
}

func TestPerformanceTester_BenchmarkQuery(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	tester := NewPerformanceTester(testDB, "users")
	ctx := context.Background()

	query := "SELECT COUNT(*) FROM users WHERE city = $1"
	iterations := 5

	benchmark, err := tester.BenchmarkQuery(ctx, query, iterations, "Tokyo")
	if err != nil {
		t.Errorf("Failed to benchmark query: %v", err)
	}

	if benchmark.Query != query {
		t.Error("Benchmark should store the original query")
	}

	if benchmark.Iterations != iterations {
		t.Error("Benchmark should store the number of iterations")
	}

	if benchmark.SuccessCount == 0 {
		t.Error("Expected some successful executions")
	}

	if benchmark.TotalDuration <= 0 {
		t.Error("Expected positive total duration")
	}

	if benchmark.AverageDuration <= 0 {
		t.Error("Expected positive average duration")
	}

	if benchmark.MinDuration <= 0 {
		t.Error("Expected positive minimum duration")
	}

	if benchmark.MaxDuration < benchmark.MinDuration {
		t.Error("Max duration should be >= min duration")
	}
}

func TestPerformanceTester_CompareWithIndex(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	// Make sure we start without the index
	testDB.Exec("DROP INDEX IF EXISTS test_city_idx")

	tester := NewPerformanceTester(testDB, "users")
	ctx := context.Background()

	query := "SELECT * FROM users WHERE city = $1"
	indexSQL := "CREATE INDEX test_city_idx ON users(city)"
	iterations := 3

	result, err := tester.CompareWithIndex(ctx, query, indexSQL, iterations, "Tokyo")
	if err != nil {
		t.Errorf("Failed to compare with index: %v", err)
	}

	if !result.IndexCreated {
		t.Error("Index should have been created")
	}

	if result.IndexName == "" {
		t.Error("Index name should be set")
	}

	if result.QueryBefore.SuccessCount == 0 {
		t.Error("Expected successful executions before index")
	}

	if result.QueryAfter.SuccessCount == 0 {
		t.Error("Expected successful executions after index")
	}

	// Cleanup
	testDB.Exec("DROP INDEX IF EXISTS test_city_idx")
}

func TestIndexMaintenance_UsageStats(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	maintenance := NewIndexMaintenance(testDB)

	// Create a test index
	_, err := testDB.Exec("CREATE INDEX IF NOT EXISTS test_email_idx ON users(email)")
	if err != nil {
		t.Errorf("Failed to create test index: %v", err)
	}

	// Execute some queries to generate usage stats
	for i := 0; i < 5; i++ {
		testDB.QueryRow("SELECT id FROM users WHERE email = $1", fmt.Sprintf("user%d@example.com", i+1))
	}

	// Get usage stats
	stats, err := maintenance.GetIndexUsageStats()
	if err != nil {
		t.Errorf("Failed to get index usage stats: %v", err)
	}

	if len(stats) == 0 {
		t.Error("Expected some index usage statistics")
	}

	// Check stats structure
	for _, stat := range stats {
		if stat.SchemaName == "" {
			t.Error("Expected schema name in stats")
		}
		if stat.TableName == "" {
			t.Error("Expected table name in stats")
		}
		if stat.IndexName == "" {
			t.Error("Expected index name in stats")
		}
		if stat.Size < 0 {
			t.Error("Expected non-negative index size")
		}
	}

	// Cleanup
	testDB.Exec("DROP INDEX IF EXISTS test_email_idx")
}

func TestIndexMaintenance_FindUnusedIndexes(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	maintenance := NewIndexMaintenance(testDB)

	// Create an unused index
	_, err := testDB.Exec("CREATE INDEX IF NOT EXISTS unused_test_idx ON users(name)")
	if err != nil {
		t.Errorf("Failed to create unused test index: %v", err)
	}

	// Find unused indexes
	unusedIndexes, err := maintenance.FindUnusedIndexes()
	if err != nil {
		t.Errorf("Failed to find unused indexes: %v", err)
	}

	// Should find our unused index
	found := false
	for _, indexName := range unusedIndexes {
		if indexName == "unused_test_idx" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Should have found unused_test_idx in unused indexes")
	}

	// Cleanup
	testDB.Exec("DROP INDEX IF EXISTS unused_test_idx")
}

func TestIndexMaintenance_GetIndexSizes(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	maintenance := NewIndexMaintenance(testDB)

	sizes, err := maintenance.GetIndexSizes()
	if err != nil {
		t.Errorf("Failed to get index sizes: %v", err)
	}

	if len(sizes) == 0 {
		t.Error("Expected some index sizes")
	}

	// Check that sizes are non-negative
	for indexName, size := range sizes {
		if size < 0 {
			t.Errorf("Index %s has negative size: %d", indexName, size)
		}
	}
}

func TestQueryOptimizer_Integration(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	optimizer := NewQueryOptimizer(testDB)

	query := "SELECT * FROM users WHERE city = $1 AND age > $2"
	result, err := optimizer.OptimizeQuery(query, "Tokyo", 25)
	if err != nil {
		t.Errorf("Failed to optimize query: %v", err)
	}

	if result.OriginalQuery != query {
		t.Error("Original query should be preserved")
	}

	if result.OptimizedQuery == "" {
		t.Error("Optimized query should be set")
	}

	if result.EstimatedGain < 0 {
		t.Error("Estimated gain should be non-negative")
	}

	// Should have plan analysis
	if result.PlanAnalysis.TotalCost <= 0 {
		t.Error("Expected positive total cost in plan analysis")
	}

	// Should have some recommendations
	if len(result.Recommendations) == 0 {
		t.Error("Expected some optimization recommendations")
	}
}

func TestReportGenerator_PerformanceReport(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	generator := NewReportGenerator(testDB)

	report, err := generator.GeneratePerformanceReport()
	if err != nil {
		t.Errorf("Failed to generate performance report: %v", err)
	}

	if report.Timestamp.IsZero() {
		t.Error("Report timestamp should be set")
	}

	if len(report.TableSizes) == 0 {
		t.Error("Expected some table sizes in report")
	}

	if len(report.IndexSizes) == 0 {
		t.Error("Expected some index sizes in report")
	}

	if report.Summary == "" {
		t.Error("Report summary should be set")
	}

	// Check table sizes are reasonable
	for tableName, size := range report.TableSizes {
		if size <= 0 {
			t.Errorf("Table %s has non-positive size: %d", tableName, size)
		}
	}
}

func TestSetupAndSeedFunctions(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	// Test creating and dropping indexes
	err := createIndexes(testDB)
	if err != nil {
		t.Errorf("Failed to create indexes: %v", err)
	}

	// Verify indexes were created
	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname LIKE 'idx_%'").Scan(&count)
	if err != nil {
		t.Errorf("Failed to count indexes: %v", err)
	}
	if count == 0 {
		t.Error("Expected some indexes to be created")
	}

	// Test dropping indexes
	err = dropIndexes(testDB)
	if err != nil {
		t.Errorf("Failed to drop indexes: %v", err)
	}

	// Verify indexes were dropped
	err = testDB.QueryRow("SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public' AND indexname LIKE 'idx_%'").Scan(&count)
	if err != nil {
		t.Errorf("Failed to count indexes after drop: %v", err)
	}
	if count > 0 {
		t.Error("Expected indexes to be dropped")
	}
}

func BenchmarkQueryWithoutIndex(b *testing.B) {
	if testDB == nil {
		b.Skip("Database not available")
	}

	// Make sure index doesn't exist
	testDB.Exec("DROP INDEX IF EXISTS bench_city_idx")

	query := "SELECT COUNT(*) FROM users WHERE city = $1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var count int
		err := testDB.QueryRow(query, "Tokyo").Scan(&count)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkQueryWithIndex(b *testing.B) {
	if testDB == nil {
		b.Skip("Database not available")
	}

	// Create index
	_, err := testDB.Exec("CREATE INDEX IF NOT EXISTS bench_city_idx ON users(city)")
	if err != nil {
		b.Fatal(err)
	}

	query := "SELECT COUNT(*) FROM users WHERE city = $1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var count int
		err := testDB.QueryRow(query, "Tokyo").Scan(&count)
		if err != nil {
			b.Error(err)
		}
	}

	// Cleanup
	testDB.Exec("DROP INDEX IF EXISTS bench_city_idx")
}

func BenchmarkComplexQuery(b *testing.B) {
	if testDB == nil {
		b.Skip("Database not available")
	}

	query := `
		SELECT u.name, COUNT(o.id) as order_count, AVG(o.amount) as avg_amount
		FROM users u
		LEFT JOIN orders o ON u.id = o.user_id
		WHERE u.city = $1 AND o.status = $2
		GROUP BY u.id, u.name
		ORDER BY order_count DESC
		LIMIT 10
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := testDB.Query(query, "Tokyo", "completed")
		if err != nil {
			b.Error(err)
			continue
		}
		
		for rows.Next() {
			var name string
			var orderCount int
			var avgAmount float64
			rows.Scan(&name, &orderCount, &avgAmount)
		}
		rows.Close()
	}
}