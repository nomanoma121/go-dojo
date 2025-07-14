package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

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

	// Setup test schema
	if err := createTestSchema(testDB); err != nil {
		log.Fatalf("Could not create test schema: %s", err)
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

func TestPoolConfig_Apply(t *testing.T) {
	db, err := setupTestDatabase(testDSN)
	if err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}
	defer db.Close()

	config := PoolConfig{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 2 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
		Environment:     "test",
	}

	config.Apply(db)

	stats := db.Stats()
	if stats.MaxOpenConnections != 10 {
		t.Errorf("Expected MaxOpenConnections=10, got %d", stats.MaxOpenConnections)
	}
}

func TestPoolConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      PoolConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: PoolConfig{
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Minute,
				ConnMaxIdleTime: 30 * time.Second,
				Environment:     "test",
			},
			expectError: false,
		},
		{
			name: "negative MaxOpenConns",
			config: PoolConfig{
				MaxOpenConns: -1,
				Environment:  "test",
			},
			expectError: true,
		},
		{
			name: "MaxIdleConns > MaxOpenConns",
			config: PoolConfig{
				MaxOpenConns: 5,
				MaxIdleConns: 10,
				Environment:  "test",
			},
			expectError: true,
		},
		{
			name: "empty environment",
			config: PoolConfig{
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestDefaultConfigs(t *testing.T) {
	configs := DefaultConfigs()

	expectedEnvs := []string{"development", "staging", "production"}
	for _, env := range expectedEnvs {
		config, exists := configs[env]
		if !exists {
			t.Errorf("Expected config for environment %s", env)
			continue
		}

		if config.Environment != env {
			t.Errorf("Expected environment %s, got %s", env, config.Environment)
		}

		if err := config.Validate(); err != nil {
			t.Errorf("Default config for %s is invalid: %v", env, err)
		}
	}

	// Production should have higher limits than development
	devConfig := configs["development"]
	prodConfig := configs["production"]

	if prodConfig.MaxOpenConns <= devConfig.MaxOpenConns {
		t.Error("Production MaxOpenConns should be higher than development")
	}

	if prodConfig.MaxIdleConns <= devConfig.MaxIdleConns {
		t.Error("Production MaxIdleConns should be higher than development")
	}
}

func TestConnectionManager_BasicOperations(t *testing.T) {
	config := PoolConfig{
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
		Environment:     "test",
	}

	cm, err := NewConnectionManager(testDSN, config)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}
	defer cm.Close()

	// Test GetDB
	db := cm.GetDB()
	if db == nil {
		t.Error("GetDB returned nil")
	}

	// Test connection works
	var result int
	err = db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		t.Errorf("Failed to execute query: %v", err)
	}
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}

	// Test GetStats
	stats := cm.GetStats()
	if stats.MaxOpenConnections != 5 {
		t.Errorf("Expected MaxOpenConnections=5, got %d", stats.MaxOpenConnections)
	}
}

func TestConnectionManager_UpdateConfig(t *testing.T) {
	initialConfig := PoolConfig{
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
		Environment:     "test",
	}

	cm, err := NewConnectionManager(testDSN, initialConfig)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}
	defer cm.Close()

	newConfig := PoolConfig{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 2 * time.Minute,
		ConnMaxIdleTime: time.Minute,
		Environment:     "test",
	}

	err = cm.UpdateConfig(newConfig)
	if err != nil {
		t.Errorf("Failed to update config: %v", err)
	}

	stats := cm.GetStats()
	if stats.MaxOpenConnections != 10 {
		t.Errorf("Expected MaxOpenConnections=10 after update, got %d", stats.MaxOpenConnections)
	}

	// Test invalid config update
	invalidConfig := PoolConfig{
		MaxOpenConns: -1,
		Environment:  "test",
	}

	err = cm.UpdateConfig(invalidConfig)
	if err == nil {
		t.Error("Expected error when updating with invalid config")
	}
}

func TestHealthChecker_Integration(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	hc := NewHealthChecker(testDB, 100*time.Millisecond, 5*time.Second)

	// Test immediate check
	healthy, err := hc.CheckNow()
	if err != nil {
		t.Errorf("Expected no error from health check: %v", err)
	}
	if !healthy {
		t.Error("Expected database to be healthy")
	}

	// Test status retrieval
	isHealthy, errMsg, lastCheck := hc.IsHealthy()
	if !isHealthy {
		t.Error("Expected database to be healthy")
	}
	if errMsg != "" {
		t.Errorf("Expected no error message, got: %s", errMsg)
	}
	if lastCheck.IsZero() {
		t.Error("Expected lastCheck to be set")
	}

	// Test continuous monitoring
	hc.Start()
	time.Sleep(250 * time.Millisecond) // Wait for at least 2 checks
	hc.Stop()

	isHealthy, _, lastCheck2 := hc.IsHealthy()
	if !isHealthy {
		t.Error("Expected database to be healthy after monitoring")
	}
	if !lastCheck2.After(lastCheck) {
		t.Error("Expected lastCheck to be updated by monitoring")
	}
}

func TestPoolMonitor_Statistics(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	pm := NewPoolMonitor(testDB, "test", 50*time.Millisecond)

	// Test initial state
	stats := pm.GetStats()
	if len(stats) != 0 {
		t.Error("Expected no stats initially")
	}

	_, hasStats := pm.GetLatestStats()
	if hasStats {
		t.Error("Expected no latest stats initially")
	}

	// Start monitoring and collect some stats
	pm.Start()
	time.Sleep(150 * time.Millisecond) // Allow for 2-3 collections
	pm.Stop()

	stats = pm.GetStats()
	if len(stats) == 0 {
		t.Error("Expected some stats to be collected")
	}

	latest, hasStats := pm.GetLatestStats()
	if !hasStats {
		t.Error("Expected to have latest stats")
	}

	if latest.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	// Test clear stats
	pm.ClearStats()
	stats = pm.GetStats()
	if len(stats) != 0 {
		t.Error("Expected stats to be cleared")
	}
}

func TestLoadTester_ConcurrentAccess(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	// Create a simple query function
	queryFunc := func(db *sql.DB) error {
		var result int
		return db.QueryRow("SELECT 1").Scan(&result)
	}

	lt := NewLoadTester(testDB, 5, 100*time.Millisecond)
	lt.SetQueryFunc(queryFunc)

	results := lt.Run()

	if results.TotalRequests == 0 {
		t.Error("Expected some requests to be made")
	}

	if results.SuccessfulReqs == 0 {
		t.Error("Expected some successful requests")
	}

	if results.FailedReqs > results.SuccessfulReqs {
		t.Error("Expected more successful requests than failed requests")
	}

	if results.RequestsPerSecond <= 0 {
		t.Error("Expected positive requests per second")
	}

	if results.AvgResponseTime <= 0 {
		t.Error("Expected positive average response time")
	}

	// Test GetResults method
	retrievedResults := lt.GetResults()
	if retrievedResults.TotalRequests != results.TotalRequests {
		t.Error("GetResults should return the same results")
	}
}

func TestLoadTester_ErrorHandling(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	// Create a query function that always fails
	errorQueryFunc := func(db *sql.DB) error {
		return fmt.Errorf("simulated error")
	}

	lt := NewLoadTester(testDB, 2, 50*time.Millisecond)
	lt.SetQueryFunc(errorQueryFunc)

	results := lt.Run()

	if results.TotalRequests == 0 {
		t.Error("Expected some requests to be made")
	}

	if results.FailedReqs == 0 {
		t.Error("Expected some failed requests")
	}

	if results.SuccessfulReqs != 0 {
		t.Error("Expected no successful requests with error query")
	}

	if len(results.Errors) == 0 {
		t.Error("Expected some errors to be recorded")
	}
}

func TestPoolOptimizer(t *testing.T) {
	optimizer := NewPoolOptimizer()

	// Create sample statistics
	stats := []PoolStats{
		{
			Timestamp:       time.Now(),
			OpenConnections: 8,
			InUse:           6,
			Idle:            2,
			WaitCount:       5,
			WaitDuration:    10 * time.Millisecond,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
		},
		{
			Timestamp:       time.Now(),
			OpenConnections: 9,
			InUse:           7,
			Idle:            2,
			WaitCount:       8,
			WaitDuration:    15 * time.Millisecond,
			MaxOpenConns:    10,
			MaxIdleConns:    5,
		},
	}

	recommendations := optimizer.AnalyzeStats(stats)

	if len(recommendations) == 0 {
		t.Error("Expected some recommendations")
	}

	avgInUse, exists := recommendations["average_in_use"]
	if !exists {
		t.Error("Expected average_in_use in recommendations")
	}
	if avgInUse != 6.5 {
		t.Errorf("Expected average_in_use=6.5, got %v", avgInUse)
	}

	// Test config suggestion
	suggestedConfig := optimizer.SuggestConfig("production")
	if suggestedConfig.Environment != "production" {
		t.Error("Expected production environment in suggested config")
	}

	// Test GetRecommendations
	retrievedRecs := optimizer.GetRecommendations()
	if len(retrievedRecs) == 0 {
		t.Error("Expected to retrieve recommendations")
	}
}

func TestConnectionManager_ConcurrentAccess(t *testing.T) {
	config := PoolConfig{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
		Environment:     "test",
	}

	cm, err := NewConnectionManager(testDSN, config)
	if err != nil {
		t.Fatalf("Failed to create connection manager: %v", err)
	}
	defer cm.Close()

	var wg sync.WaitGroup
	numWorkers := 20
	numQueriesPerWorker := 10

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db := cm.GetDB()
			
			for j := 0; j < numQueriesPerWorker; j++ {
				var result int
				err := db.QueryRow("SELECT 1").Scan(&result)
				if err != nil {
					t.Errorf("Query failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	stats := cm.GetStats()
	if stats.MaxOpenConnections != 10 {
		t.Errorf("Expected MaxOpenConnections=10, got %d", stats.MaxOpenConnections)
	}
}

func BenchmarkConnectionPool_Queries(b *testing.B) {
	if testDB == nil {
		b.Skip("Database not available")
	}

	config := PoolConfig{
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: time.Minute,
		Environment:     "benchmark",
	}

	cm, err := NewConnectionManager(testDSN, config)
	if err != nil {
		b.Fatalf("Failed to create connection manager: %v", err)
	}
	defer cm.Close()

	db := cm.GetDB()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var result int
			err := db.QueryRow("SELECT 1").Scan(&result)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

func BenchmarkLoadTester_Performance(b *testing.B) {
	if testDB == nil {
		b.Skip("Database not available")
	}

	queryFunc := func(db *sql.DB) error {
		var result int
		return db.QueryRow("SELECT 1").Scan(&result)
	}

	for _, concurrency := range []int{1, 5, 10, 20} {
		b.Run(fmt.Sprintf("concurrency-%d", concurrency), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				lt := NewLoadTester(testDB, concurrency, 10*time.Millisecond)
				lt.SetQueryFunc(queryFunc)
				lt.Run()
			}
		})
	}
}