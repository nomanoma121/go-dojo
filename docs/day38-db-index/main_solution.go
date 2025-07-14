package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// User represents a user entity
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	City      string    `json:"city"`
	CreatedAt time.Time `json:"created_at"`
}

// Order represents an order entity
type Order struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ExplainResult holds the result of EXPLAIN analysis
type ExplainResult struct {
	NodeType          string          `json:"Node Type"`
	Relation          string          `json:"Relation Name,omitempty"`
	Alias             string          `json:"Alias,omitempty"`
	StartupCost       float64         `json:"Startup Cost"`
	TotalCost         float64         `json:"Total Cost"`
	PlanRows          int             `json:"Plan Rows"`
	PlanWidth         int             `json:"Plan Width"`
	ActualStartupTime float64         `json:"Actual Startup Time,omitempty"`
	ActualTotalTime   float64         `json:"Actual Total Time,omitempty"`
	ActualRows        int             `json:"Actual Rows,omitempty"`
	IndexName         string          `json:"Index Name,omitempty"`
	IndexCondition    string          `json:"Index Cond,omitempty"`
	Filter            string          `json:"Filter,omitempty"`
	BuffersHit        int             `json:"Buffers Hit,omitempty"`
	BuffersRead       int             `json:"Buffers Read,omitempty"`
	Plans             []ExplainResult `json:"Plans,omitempty"`
}

// QueryAnalyzer analyzes SQL queries using EXPLAIN
type QueryAnalyzer struct {
	db *sql.DB
}

// NewQueryAnalyzer creates a new query analyzer
func NewQueryAnalyzer(db *sql.DB) *QueryAnalyzer {
	return &QueryAnalyzer{db: db}
}

// ExplainQuery executes EXPLAIN on a query and returns analysis results
func (qa *QueryAnalyzer) ExplainQuery(query string, args ...interface{}) ([]ExplainResult, error) {
	explainQuery := "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + query

	var jsonResult string
	err := qa.db.QueryRow(explainQuery, args...).Scan(&jsonResult)
	if err != nil {
		return nil, fmt.Errorf("failed to execute EXPLAIN: %w", err)
	}

	var plans []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonResult), &plans); err != nil {
		return nil, fmt.Errorf("failed to parse EXPLAIN result: %w", err)
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("empty EXPLAIN result")
	}

	plan, ok := plans[0]["Plan"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid EXPLAIN result format")
	}

	return parseExplainNode(plan), nil
}

func parseExplainNode(node map[string]interface{}) []ExplainResult {
	result := ExplainResult{}

	if nodeType, ok := node["Node Type"].(string); ok {
		result.NodeType = nodeType
	}
	if relation, ok := node["Relation Name"].(string); ok {
		result.Relation = relation
	}
	if alias, ok := node["Alias"].(string); ok {
		result.Alias = alias
	}
	if cost, ok := node["Startup Cost"].(float64); ok {
		result.StartupCost = cost
	}
	if cost, ok := node["Total Cost"].(float64); ok {
		result.TotalCost = cost
	}
	if rows, ok := node["Plan Rows"].(float64); ok {
		result.PlanRows = int(rows)
	}
	if width, ok := node["Plan Width"].(float64); ok {
		result.PlanWidth = int(width)
	}
	if time, ok := node["Actual Startup Time"].(float64); ok {
		result.ActualStartupTime = time
	}
	if time, ok := node["Actual Total Time"].(float64); ok {
		result.ActualTotalTime = time
	}
	if rows, ok := node["Actual Rows"].(float64); ok {
		result.ActualRows = int(rows)
	}
	if indexName, ok := node["Index Name"].(string); ok {
		result.IndexName = indexName
	}
	if indexCond, ok := node["Index Cond"].(string); ok {
		result.IndexCondition = indexCond
	}
	if filter, ok := node["Filter"].(string); ok {
		result.Filter = filter
	}

	results := []ExplainResult{result}

	// Parse child plans
	if plans, ok := node["Plans"].([]interface{}); ok {
		for _, planInterface := range plans {
			if childPlan, ok := planInterface.(map[string]interface{}); ok {
				childResults := parseExplainNode(childPlan)
				results = append(results, childResults...)
			}
		}
	}

	return results
}

// AnalyzeQueryPlan analyzes the query execution plan
func (qa *QueryAnalyzer) AnalyzeQueryPlan(results []ExplainResult) QueryPlanAnalysis {
	analysis := QueryPlanAnalysis{
		Recommendations: make([]string, 0),
	}

	for _, result := range results {
		analysis.TotalCost += result.TotalCost
		analysis.ExecutionTime += result.ActualTotalTime
		analysis.RowsProcessed += result.ActualRows
		analysis.BuffersUsed += result.BuffersHit + result.BuffersRead

		switch result.NodeType {
		case "Seq Scan":
			analysis.HasSeqScan = true
			if result.ActualTotalTime > 10.0 {
				analysis.Recommendations = append(analysis.Recommendations,
					fmt.Sprintf("Consider adding index on table %s", result.Relation))
			}
		case "Index Scan", "Index Only Scan", "Bitmap Index Scan":
			analysis.HasIndexScan = true
		}

		if result.Filter != "" && result.ActualTotalTime > 5.0 {
			analysis.Recommendations = append(analysis.Recommendations,
				fmt.Sprintf("Consider adding index for filter condition: %s", result.Filter))
		}
	}

	return analysis
}

// QueryPlanAnalysis holds query plan analysis results
type QueryPlanAnalysis struct {
	HasSeqScan      bool
	HasIndexScan    bool
	TotalCost       float64
	ExecutionTime   float64
	RowsProcessed   int
	BuffersUsed     int
	Recommendations []string
}

// IndexRecommendation suggests indexes based on query patterns
type IndexRecommendation struct {
	TableName    string
	Columns      []string
	IndexType    string
	Reason       string
	ExpectedGain float64
	Priority     int
}

// IndexAdvisor analyzes queries and suggests indexes
type IndexAdvisor struct {
	db              *sql.DB
	analyzer        *QueryAnalyzer
	queries         []string
	recommendations []IndexRecommendation
}

// NewIndexAdvisor creates a new index advisor
func NewIndexAdvisor(db *sql.DB) *IndexAdvisor {
	return &IndexAdvisor{
		db:              db,
		analyzer:        NewQueryAnalyzer(db),
		queries:         make([]string, 0),
		recommendations: make([]IndexRecommendation, 0),
	}
}

// AnalyzeQuery analyzes a query and generates index recommendations
func (ia *IndexAdvisor) AnalyzeQuery(query string, args ...interface{}) error {
	results, err := ia.analyzer.ExplainQuery(query, args...)
	if err != nil {
		return err
	}

	ia.queries = append(ia.queries, query)

	for _, result := range results {
		if result.NodeType == "Seq Scan" && result.ActualTotalTime > 5.0 {
			columns := ia.extractColumnsFromQuery(query, result.Relation)
			if len(columns) > 0 {
				recommendation := IndexRecommendation{
					TableName:    result.Relation,
					Columns:      columns,
					IndexType:    "btree",
					Reason:       "Sequential scan detected on large table",
					ExpectedGain: result.ActualTotalTime * 0.7,
					Priority:     ia.calculatePriority(result.ActualTotalTime),
				}
				ia.recommendations = append(ia.recommendations, recommendation)
			}
		}
	}

	return nil
}

func (ia *IndexAdvisor) extractColumnsFromQuery(query, tableName string) []string {
	query = strings.ToLower(query)
	tableName = strings.ToLower(tableName)

	// Simple pattern matching for WHERE clauses
	whereIndex := strings.Index(query, "where")
	if whereIndex == -1 {
		return []string{}
	}

	whereClause := query[whereIndex:]
	columns := make([]string, 0)

	// Look for common column patterns
	commonColumns := []string{"id", "email", "user_id", "created_at", "status", "name", "age", "city"}
	for _, col := range commonColumns {
		if strings.Contains(whereClause, col) {
			columns = append(columns, col)
		}
	}

	return columns
}

func (ia *IndexAdvisor) calculatePriority(executionTime float64) int {
	if executionTime > 50.0 {
		return 1 // High priority
	} else if executionTime > 20.0 {
		return 2 // Medium priority
	}
	return 3 // Low priority
}

// GetRecommendations returns all index recommendations
func (ia *IndexAdvisor) GetRecommendations() []IndexRecommendation {
	// Sort by priority
	recommendations := make([]IndexRecommendation, len(ia.recommendations))
	copy(recommendations, ia.recommendations)

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Priority < recommendations[j].Priority
	})

	return recommendations
}

// GenerateIndexSQL generates SQL statements to create recommended indexes
func (ia *IndexAdvisor) GenerateIndexSQL() []string {
	recommendations := ia.GetRecommendations()
	sqlStatements := make([]string, 0, len(recommendations))

	for i, rec := range recommendations {
		indexName := fmt.Sprintf("idx_%s_%s", rec.TableName, strings.Join(rec.Columns, "_"))
		columnsStr := strings.Join(rec.Columns, ", ")

		var sql string
		switch rec.IndexType {
		case "btree":
			sql = fmt.Sprintf("CREATE INDEX %s ON %s(%s);", indexName, rec.TableName, columnsStr)
		case "hash":
			sql = fmt.Sprintf("CREATE INDEX %s ON %s USING HASH(%s);", indexName, rec.TableName, columnsStr)
		case "gin":
			sql = fmt.Sprintf("CREATE INDEX %s ON %s USING GIN(%s);", indexName, rec.TableName, columnsStr)
		default:
			sql = fmt.Sprintf("CREATE INDEX %s ON %s(%s);", indexName, rec.TableName, columnsStr)
		}

		sqlStatements = append(sqlStatements, sql)
		
		// Limit to avoid too many recommendations
		if i >= 9 {
			break
		}
	}

	return sqlStatements
}

// QueryBenchmark holds benchmark results for a query
type QueryBenchmark struct {
	Query           string
	Iterations      int
	SuccessCount    int
	TotalDuration   time.Duration
	AverageDuration time.Duration
	MinDuration     time.Duration
	MaxDuration     time.Duration
}

// PerformanceTester tests query performance with and without indexes
type PerformanceTester struct {
	db    *sql.DB
	table string
}

// NewPerformanceTester creates a new performance tester
func NewPerformanceTester(db *sql.DB, table string) *PerformanceTester {
	return &PerformanceTester{
		db:    db,
		table: table,
	}
}

// BenchmarkQuery measures query performance
func (pt *PerformanceTester) BenchmarkQuery(ctx context.Context, query string, iterations int, args ...interface{}) (QueryBenchmark, error) {
	benchmark := QueryBenchmark{
		Query:       query,
		Iterations:  iterations,
		MinDuration: time.Hour, // Initialize with large value
	}

	var totalDuration time.Duration
	successCount := 0

	for i := 0; i < iterations; i++ {
		start := time.Now()

		rows, err := pt.db.QueryContext(ctx, query, args...)
		if err != nil {
			continue
		}

		// Consume all rows to ensure full execution
		for rows.Next() {
			// Do nothing, just consume
		}
		rows.Close()

		duration := time.Since(start)
		totalDuration += duration
		successCount++

		if duration < benchmark.MinDuration {
			benchmark.MinDuration = duration
		}
		if duration > benchmark.MaxDuration {
			benchmark.MaxDuration = duration
		}
	}

	benchmark.SuccessCount = successCount
	benchmark.TotalDuration = totalDuration
	if successCount > 0 {
		benchmark.AverageDuration = totalDuration / time.Duration(successCount)
	}

	return benchmark, nil
}

// CompareWithIndex compares query performance before and after index creation
func (pt *PerformanceTester) CompareWithIndex(ctx context.Context, query string, indexSQL string, iterations int, args ...interface{}) (IndexComparisonResult, error) {
	// Benchmark before index
	beforeBenchmark, err := pt.BenchmarkQuery(ctx, query, iterations, args...)
	if err != nil {
		return IndexComparisonResult{}, fmt.Errorf("failed to benchmark query before index: %w", err)
	}

	// Create index
	_, err = pt.db.ExecContext(ctx, indexSQL)
	if err != nil {
		return IndexComparisonResult{}, fmt.Errorf("failed to create index: %w", err)
	}

	indexCreated := true
	indexName := extractIndexNameFromSQL(indexSQL)

	// Benchmark after index
	afterBenchmark, err := pt.BenchmarkQuery(ctx, query, iterations, args...)
	if err != nil {
		return IndexComparisonResult{
			QueryBefore:  beforeBenchmark,
			IndexCreated: indexCreated,
			IndexName:    indexName,
		}, fmt.Errorf("failed to benchmark query after index: %w", err)
	}

	// Calculate improvement ratio
	var improvementRatio float64
	if beforeBenchmark.AverageDuration > 0 {
		improvementRatio = float64(beforeBenchmark.AverageDuration-afterBenchmark.AverageDuration) / float64(beforeBenchmark.AverageDuration)
	}

	return IndexComparisonResult{
		QueryBefore:      beforeBenchmark,
		QueryAfter:       afterBenchmark,
		ImprovementRatio: improvementRatio,
		IndexCreated:     indexCreated,
		IndexName:        indexName,
	}, nil
}

func extractIndexNameFromSQL(indexSQL string) string {
	parts := strings.Fields(indexSQL)
	for i, part := range parts {
		if strings.ToUpper(part) == "INDEX" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "unknown"
}

// IndexComparisonResult holds the results of index performance comparison
type IndexComparisonResult struct {
	QueryBefore      QueryBenchmark
	QueryAfter       QueryBenchmark
	ImprovementRatio float64
	IndexCreated     bool
	IndexName        string
}

// IndexUsage holds index usage statistics
type IndexUsage struct {
	SchemaName string
	TableName  string
	IndexName  string
	TupRead    int64
	TupFetch   int64
	Scans      int64
	Size       int64
}

// IndexMaintenance handles index maintenance tasks
type IndexMaintenance struct {
	db *sql.DB
}

// NewIndexMaintenance creates a new index maintenance manager
func NewIndexMaintenance(db *sql.DB) *IndexMaintenance {
	return &IndexMaintenance{db: db}
}

// GetIndexUsageStats returns index usage statistics
func (im *IndexMaintenance) GetIndexUsageStats() ([]IndexUsage, error) {
	query := `
		SELECT 
			schemaname,
			tablename,
			indexname,
			idx_tup_read,
			idx_tup_fetch,
			idx_scan,
			pg_relation_size(indexrelid) as size
		FROM pg_stat_user_indexes
		ORDER BY idx_scan DESC
	`

	rows, err := im.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usage []IndexUsage
	for rows.Next() {
		var idx IndexUsage
		err := rows.Scan(
			&idx.SchemaName,
			&idx.TableName,
			&idx.IndexName,
			&idx.TupRead,
			&idx.TupFetch,
			&idx.Scans,
			&idx.Size,
		)
		if err != nil {
			return nil, err
		}
		usage = append(usage, idx)
	}

	return usage, nil
}

// FindUnusedIndexes identifies potentially unused indexes
func (im *IndexMaintenance) FindUnusedIndexes() ([]string, error) {
	query := `
		SELECT indexname 
		FROM pg_stat_user_indexes 
		WHERE idx_scan = 0
		  AND indexname NOT LIKE '%_pkey'
	`

	rows, err := im.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var unused []string
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return nil, err
		}
		unused = append(unused, indexName)
	}

	return unused, nil
}

// GetIndexSizes returns the size of all indexes
func (im *IndexMaintenance) GetIndexSizes() (map[string]int64, error) {
	query := `
		SELECT 
			indexname,
			pg_relation_size(indexrelid) as size
		FROM pg_stat_user_indexes
	`

	rows, err := im.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sizes := make(map[string]int64)
	for rows.Next() {
		var indexName string
		var size int64
		if err := rows.Scan(&indexName, &size); err != nil {
			return nil, err
		}
		sizes[indexName] = size
	}

	return sizes, nil
}

// ReindexTable rebuilds all indexes for a table
func (im *IndexMaintenance) ReindexTable(tableName string) error {
	query := fmt.Sprintf("REINDEX TABLE %s", tableName)
	_, err := im.db.Exec(query)
	return err
}

// QueryOptimizer provides query optimization suggestions
type QueryOptimizer struct {
	db       *sql.DB
	analyzer *QueryAnalyzer
	advisor  *IndexAdvisor
	tester   *PerformanceTester
}

// NewQueryOptimizer creates a new query optimizer
func NewQueryOptimizer(db *sql.DB) *QueryOptimizer {
	return &QueryOptimizer{
		db:       db,
		analyzer: NewQueryAnalyzer(db),
		advisor:  NewIndexAdvisor(db),
		tester:   NewPerformanceTester(db, ""),
	}
}

// OptimizeQuery analyzes and suggests optimizations for a query
func (qo *QueryOptimizer) OptimizeQuery(query string, args ...interface{}) (OptimizationResult, error) {
	// Analyze the query
	results, err := qo.analyzer.ExplainQuery(query, args...)
	if err != nil {
		return OptimizationResult{}, fmt.Errorf("failed to analyze query: %w", err)
	}

	planAnalysis := qo.analyzer.AnalyzeQueryPlan(results)

	// Generate index suggestions
	err = qo.advisor.AnalyzeQuery(query, args...)
	if err != nil {
		return OptimizationResult{}, fmt.Errorf("failed to generate index suggestions: %w", err)
	}

	indexSuggestions := qo.advisor.GetRecommendations()

	// Generate optimized query (simplified)
	optimizedQuery := qo.optimizeQueryStructure(query)

	// Estimate performance gain
	estimatedGain := qo.estimatePerformanceGain(planAnalysis, indexSuggestions)

	recommendations := make([]string, 0)
	recommendations = append(recommendations, planAnalysis.Recommendations...)

	if planAnalysis.HasSeqScan {
		recommendations = append(recommendations, "Consider adding appropriate indexes to avoid sequential scans")
	}

	if planAnalysis.ExecutionTime > 100.0 {
		recommendations = append(recommendations, "Query execution time is high, consider optimization")
	}

	return OptimizationResult{
		OriginalQuery:    query,
		OptimizedQuery:   optimizedQuery,
		IndexSuggestions: indexSuggestions,
		PlanAnalysis:     planAnalysis,
		EstimatedGain:    estimatedGain,
		Recommendations:  recommendations,
	}, nil
}

func (qo *QueryOptimizer) optimizeQueryStructure(query string) string {
	// Simple query optimization - just return the original for now
	// In a real implementation, this would apply various optimization techniques
	return query
}

func (qo *QueryOptimizer) estimatePerformanceGain(analysis QueryPlanAnalysis, suggestions []IndexRecommendation) float64 {
	if analysis.HasSeqScan && len(suggestions) > 0 {
		return 0.6 // Estimate 60% improvement with indexes
	}
	if analysis.ExecutionTime > 50.0 {
		return 0.3 // Estimate 30% improvement for slow queries
	}
	return 0.1 // Minimal improvement expected
}

// OptimizationResult holds query optimization results
type OptimizationResult struct {
	OriginalQuery    string
	OptimizedQuery   string
	IndexSuggestions []IndexRecommendation
	PlanAnalysis     QueryPlanAnalysis
	EstimatedGain    float64
	Recommendations  []string
}

// ReportGenerator generates performance analysis reports
type ReportGenerator struct {
	db          *sql.DB
	optimizer   *QueryOptimizer
	maintenance *IndexMaintenance
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(db *sql.DB) *ReportGenerator {
	return &ReportGenerator{
		db:          db,
		optimizer:   NewQueryOptimizer(db),
		maintenance: NewIndexMaintenance(db),
	}
}

// GeneratePerformanceReport generates a comprehensive performance report
func (rg *ReportGenerator) GeneratePerformanceReport() (PerformanceReport, error) {
	report := PerformanceReport{
		Timestamp:     time.Now(),
		SlowQueries:   make([]QueryBenchmark, 0),
		TableSizes:    make(map[string]int64),
		IndexSizes:    make(map[string]int64),
	}

	// Get unused indexes
	unusedIndexes, err := rg.maintenance.FindUnusedIndexes()
	if err != nil {
		return report, fmt.Errorf("failed to get unused indexes: %w", err)
	}
	report.UnusedIndexes = unusedIndexes

	// Get index sizes
	indexSizes, err := rg.maintenance.GetIndexSizes()
	if err != nil {
		return report, fmt.Errorf("failed to get index sizes: %w", err)
	}
	report.IndexSizes = indexSizes

	// Get table sizes
	tableSizes, err := rg.getTableSizes()
	if err != nil {
		return report, fmt.Errorf("failed to get table sizes: %w", err)
	}
	report.TableSizes = tableSizes

	// Generate summary
	report.Summary = rg.generateSummary(report)

	return report, nil
}

func (rg *ReportGenerator) getTableSizes() (map[string]int64, error) {
	query := `
		SELECT 
			tablename,
			pg_total_relation_size(schemaname||'.'||tablename) as size
		FROM pg_tables 
		WHERE schemaname = 'public'
	`

	rows, err := rg.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sizes := make(map[string]int64)
	for rows.Next() {
		var tableName string
		var size int64
		if err := rows.Scan(&tableName, &size); err != nil {
			return nil, err
		}
		sizes[tableName] = size
	}

	return sizes, nil
}

func (rg *ReportGenerator) generateSummary(report PerformanceReport) string {
	summary := fmt.Sprintf("Performance Report generated at %s\n", report.Timestamp.Format(time.RFC3339))
	summary += fmt.Sprintf("Found %d unused indexes\n", len(report.UnusedIndexes))
	summary += fmt.Sprintf("Tracking %d tables and %d indexes\n", len(report.TableSizes), len(report.IndexSizes))

	if len(report.UnusedIndexes) > 0 {
		summary += "Consider dropping unused indexes to save space and improve write performance\n"
	}

	return summary
}

// PerformanceReport holds comprehensive database performance analysis
type PerformanceReport struct {
	Timestamp        time.Time
	SlowQueries      []QueryBenchmark
	UnusedIndexes    []string
	IndexSuggestions []IndexRecommendation
	TableSizes       map[string]int64
	IndexSizes       map[string]int64
	Summary          string
}

// setupTestDatabase creates test tables and data
func setupTestDatabase(db *sql.DB) error {
	schema := `
		DROP TABLE IF EXISTS orders CASCADE;
		DROP TABLE IF EXISTS users CASCADE;

		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INTEGER,
			city VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			amount DECIMAL(10,2) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`

	_, err := db.Exec(schema)
	return err
}

// seedTestData inserts test data for performance testing
func seedTestData(db *sql.DB, userCount, orderCount int) error {
	// Clear existing data
	_, err := db.Exec("DELETE FROM orders")
	if err != nil {
		return err
	}
	_, err = db.Exec("DELETE FROM users")
	if err != nil {
		return err
	}

	// Reset sequences
	_, err = db.Exec("ALTER SEQUENCE users_id_seq RESTART WITH 1")
	if err != nil {
		return err
	}
	_, err = db.Exec("ALTER SEQUENCE orders_id_seq RESTART WITH 1")
	if err != nil {
		return err
	}

	// Insert users
	cities := []string{"Tokyo", "Osaka", "Yokohama", "Nagoya", "Sapporo", "Fukuoka"}
	statuses := []string{"pending", "completed", "cancelled", "processing"}

	for i := 1; i <= userCount; i++ {
		_, err := db.Exec(
			"INSERT INTO users (name, email, age, city) VALUES ($1, $2, $3, $4)",
			fmt.Sprintf("User %d", i),
			fmt.Sprintf("user%d@example.com", i),
			20+rand.Intn(50),
			cities[rand.Intn(len(cities))],
		)
		if err != nil {
			return err
		}
	}

	// Insert orders
	for i := 1; i <= orderCount; i++ {
		userID := 1 + rand.Intn(userCount)
		amount := 10.0 + rand.Float64()*1000.0
		status := statuses[rand.Intn(len(statuses))]

		_, err := db.Exec(
			"INSERT INTO orders (user_id, amount, status) VALUES ($1, $2, $3)",
			userID, amount, status,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// createIndexes creates test indexes
func createIndexes(db *sql.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)",
		"CREATE INDEX IF NOT EXISTS idx_users_city ON users(city)",
		"CREATE INDEX IF NOT EXISTS idx_users_age ON users(age)",
		"CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status)",
		"CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_orders_user_status ON orders(user_id, status)",
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			return err
		}
	}

	return nil
}

// dropIndexes drops test indexes
func dropIndexes(db *sql.DB) error {
	indexes := []string{
		"DROP INDEX IF EXISTS idx_users_email",
		"DROP INDEX IF EXISTS idx_users_city",
		"DROP INDEX IF EXISTS idx_users_age",
		"DROP INDEX IF EXISTS idx_orders_user_id",
		"DROP INDEX IF EXISTS idx_orders_status",
		"DROP INDEX IF EXISTS idx_orders_created_at",
		"DROP INDEX IF EXISTS idx_orders_user_status",
	}

	for _, dropSQL := range indexes {
		if _, err := db.Exec(dropSQL); err != nil {
			return err
		}
	}

	return nil
}