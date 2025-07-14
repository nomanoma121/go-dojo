package main

import (
	"context"
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

	// Connect to database
	if err := pool.Retry(func() error {
		var err error
		testDB, err = sql.Open("postgres", fmt.Sprintf("postgres://test:secret@localhost:%s/testdb?sslmode=disable", resource.GetPort("5432/tcp")))
		if err != nil {
			return err
		}
		return testDB.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	// Setup database schema and test data
	if err := setupDatabase(testDB); err != nil {
		log.Fatalf("Could not setup database: %s", err)
	}

	if err := seedTestData(testDB); err != nil {
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

func TestDataLoader_Load(t *testing.T) {
	// Simple batch function that returns the key doubled
	batchFn := func(ctx context.Context, keys []int) ([]int, []error) {
		results := make([]int, len(keys))
		errors := make([]error, len(keys))
		for i, key := range keys {
			results[i] = key * 2
			errors[i] = nil
		}
		return results, errors
	}

	loader := NewDataLoader(batchFn)
	ctx := context.Background()

	// Test single load
	result, err := loader.Load(ctx, 5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != 10 {
		t.Errorf("Expected 10, got %d", result)
	}

	// Test cache hit
	result2, err := loader.Load(ctx, 5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result2 != 10 {
		t.Errorf("Expected 10, got %d", result2)
	}
}

func TestDataLoader_LoadMany(t *testing.T) {
	// Batch function that returns the key doubled
	batchFn := func(ctx context.Context, keys []int) ([]int, []error) {
		results := make([]int, len(keys))
		errors := make([]error, len(keys))
		for i, key := range keys {
			results[i] = key * 2
			errors[i] = nil
		}
		return results, errors
	}

	loader := NewDataLoader(batchFn)
	ctx := context.Background()

	// Test multiple loads
	keys := []int{1, 2, 3, 4, 5}
	results, errors := loader.LoadMany(ctx, keys)

	if len(results) != len(keys) {
		t.Errorf("Expected %d results, got %d", len(keys), len(results))
	}

	for i, key := range keys {
		if errors[i] != nil {
			t.Errorf("Expected no error for key %d, got %v", key, errors[i])
		}
		expectedResult := key * 2
		if results[i] != expectedResult {
			t.Errorf("Expected %d for key %d, got %d", expectedResult, key, results[i])
		}
	}
}

func TestDataLoader_Cache(t *testing.T) {
	callCount := 0
	batchFn := func(ctx context.Context, keys []int) ([]int, []error) {
		callCount++
		results := make([]int, len(keys))
		errors := make([]error, len(keys))
		for i, key := range keys {
			results[i] = key * 2
			errors[i] = nil
		}
		return results, errors
	}

	loader := NewDataLoader(batchFn)
	ctx := context.Background()

	// First load
	result1, err := loader.Load(ctx, 5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result1 != 10 {
		t.Errorf("Expected 10, got %d", result1)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 batch call, got %d", callCount)
	}

	// Second load (should use cache)
	result2, err := loader.Load(ctx, 5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result2 != 10 {
		t.Errorf("Expected 10, got %d", result2)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 batch call (cached), got %d", callCount)
	}

	// Clear cache
	loader.Clear()

	// Third load (should call batch again)
	result3, err := loader.Load(ctx, 5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result3 != 10 {
		t.Errorf("Expected 10, got %d", result3)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 batch calls after cache clear, got %d", callCount)
	}
}

func TestDataLoader_Batch(t *testing.T) {
	var batchSizes []int
	var mu sync.Mutex

	batchFn := func(ctx context.Context, keys []int) ([]int, []error) {
		mu.Lock()
		batchSizes = append(batchSizes, len(keys))
		mu.Unlock()

		results := make([]int, len(keys))
		errors := make([]error, len(keys))
		for i, key := range keys {
			results[i] = key * 2
			errors[i] = nil
		}
		return results, errors
	}

	loader := NewDataLoader(batchFn, WithMaxBatchSize[int, int](3))
	ctx := context.Background()

	// Create multiple concurrent loads
	var wg sync.WaitGroup
	keys := []int{1, 2, 3, 4, 5}

	for _, key := range keys {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			loader.Load(ctx, k)
		}(key)
	}

	wg.Wait()

	// Allow some time for batching
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have at least one batch
	if len(batchSizes) == 0 {
		t.Error("Expected at least one batch, got none")
	}

	// Check that batches respect max batch size
	for i, size := range batchSizes {
		if size > 3 {
			t.Errorf("Batch %d size %d exceeds max batch size 3", i, size)
		}
	}
}

func TestDataLoader_ClearKey(t *testing.T) {
	callCount := 0
	batchFn := func(ctx context.Context, keys []int) ([]int, []error) {
		callCount++
		results := make([]int, len(keys))
		errors := make([]error, len(keys))
		for i, key := range keys {
			results[i] = key * 2
			errors[i] = nil
		}
		return results, errors
	}

	loader := NewDataLoader(batchFn)
	ctx := context.Background()

	// Load data
	result1, err := loader.Load(ctx, 5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result1 != 10 {
		t.Errorf("Expected 10, got %d", result1)
	}

	// Clear specific key
	loader.ClearKey(5)

	// Load again (should call batch again)
	result2, err := loader.Load(ctx, 5)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result2 != 10 {
		t.Errorf("Expected 10, got %d", result2)
	}

	if callCount != 2 {
		t.Errorf("Expected 2 batch calls after clearing key, got %d", callCount)
	}
}

func TestUserLoader_Integration(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	userLoader := NewUserLoader(testDB)
	ctx := context.Background()

	// Test loading a single user
	user, err := userLoader.Load(ctx, 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if user == nil {
		t.Error("Expected user, got nil")
	}
	if user.Name != "Alice Johnson" {
		t.Errorf("Expected 'Alice Johnson', got %s", user.Name)
	}

	// Test loading multiple users
	userIDs := []int{1, 2, 3}
	users, errors := userLoader.LoadMany(ctx, userIDs)

	if len(users) != len(userIDs) {
		t.Errorf("Expected %d users, got %d", len(userIDs), len(users))
	}

	expectedNames := []string{"Alice Johnson", "Bob Smith", "Charlie Brown"}
	for i, userID := range userIDs {
		if errors[i] != nil {
			t.Errorf("Expected no error for user %d, got %v", userID, errors[i])
		}
		if users[i] == nil {
			t.Errorf("Expected user for ID %d, got nil", userID)
			continue
		}
		if users[i].Name != expectedNames[i] {
			t.Errorf("Expected '%s', got '%s'", expectedNames[i], users[i].Name)
		}
	}

	// Test loading non-existent user
	user, err = userLoader.Load(ctx, 999)
	if err == nil {
		t.Error("Expected error for non-existent user, got nil")
	}
	if user != nil {
		t.Error("Expected nil user for non-existent ID, got user")
	}
}

func TestPostLoader_Integration(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	postLoader := NewPostLoader(testDB)
	ctx := context.Background()

	// Test loading posts for a user with posts
	posts, err := postLoader.LoadByUserID(ctx, 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(posts) != 2 {
		t.Errorf("Expected 2 posts for user 1, got %d", len(posts))
	}

	// Test loading posts for multiple users
	userIDs := []int{1, 2, 5} // User 5 has no posts
	allPosts, errors := postLoader.LoadManyByUserIDs(ctx, userIDs)

	if len(allPosts) != len(userIDs) {
		t.Errorf("Expected %d result sets, got %d", len(userIDs), len(allPosts))
	}

	// Check user 1 posts
	if errors[0] != nil {
		t.Errorf("Expected no error for user 1, got %v", errors[0])
	}
	if len(allPosts[0]) != 2 {
		t.Errorf("Expected 2 posts for user 1, got %d", len(allPosts[0]))
	}

	// Check user 2 posts
	if errors[1] != nil {
		t.Errorf("Expected no error for user 2, got %v", errors[1])
	}
	if len(allPosts[1]) != 3 {
		t.Errorf("Expected 3 posts for user 2, got %d", len(allPosts[1]))
	}

	// Check user 5 posts (should be empty)
	if errors[2] != nil {
		t.Errorf("Expected no error for user 5, got %v", errors[2])
	}
	if len(allPosts[2]) != 0 {
		t.Errorf("Expected 0 posts for user 5, got %d", len(allPosts[2]))
	}
}

func TestStatsCollector(t *testing.T) {
	stats := NewStatsCollector()

	// Record some requests
	stats.RecordRequest(true)  // cache hit
	stats.RecordRequest(false) // cache miss
	stats.RecordRequest(true)  // cache hit

	// Record some batches
	stats.RecordBatch(3, 10*time.Millisecond)
	stats.RecordBatch(2, 5*time.Millisecond)

	// Get stats
	result := stats.GetStats()

	if result.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", result.TotalRequests)
	}
	if result.CacheHits != 2 {
		t.Errorf("Expected 2 cache hits, got %d", result.CacheHits)
	}
	if result.CacheMisses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", result.CacheMisses)
	}
	if result.BatchCount != 2 {
		t.Errorf("Expected 2 batches, got %d", result.BatchCount)
	}
	if result.AverageBatchSize != 2.5 {
		t.Errorf("Expected average batch size 2.5, got %f", result.AverageBatchSize)
	}
	if result.TotalLoadTime != 15*time.Millisecond {
		t.Errorf("Expected total load time 15ms, got %v", result.TotalLoadTime)
	}

	// Reset stats
	stats.Reset()
	result = stats.GetStats()

	if result.TotalRequests != 0 {
		t.Errorf("Expected 0 total requests after reset, got %d", result.TotalRequests)
	}
}

func TestDataLoader_WithOptions(t *testing.T) {
	batchFn := func(ctx context.Context, keys []int) ([]int, []error) {
		results := make([]int, len(keys))
		errors := make([]error, len(keys))
		for i, key := range keys {
			results[i] = key * 2
			errors[i] = nil
		}
		return results, errors
	}

	stats := NewStatsCollector()
	loader := NewDataLoader(
		batchFn,
		WithMaxBatchSize[int, int](5),
		WithBatchTimeout[int, int](20*time.Millisecond),
		WithStatsCollector[int, int](stats),
	)

	ctx := context.Background()

	// Test with custom stats collector
	result, err := loader.Load(ctx, 10)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != 20 {
		t.Errorf("Expected 20, got %d", result)
	}

	// Check if stats were recorded
	statsResult := stats.GetStats()
	if statsResult.TotalRequests != 1 {
		t.Errorf("Expected 1 request recorded in stats, got %d", statsResult.TotalRequests)
	}
}

func BenchmarkDataLoader_Load(b *testing.B) {
	batchFn := func(ctx context.Context, keys []int) ([]int, []error) {
		results := make([]int, len(keys))
		errors := make([]error, len(keys))
		for i, key := range keys {
			results[i] = key * 2
			errors[i] = nil
		}
		return results, errors
	}

	loader := NewDataLoader(batchFn)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Load random key to test cache behavior
			key := b.N % 100
			loader.Load(ctx, key)
		}
	})
}

func BenchmarkDataLoader_LoadMany(b *testing.B) {
	batchFn := func(ctx context.Context, keys []int) ([]int, []error) {
		results := make([]int, len(keys))
		errors := make([]error, len(keys))
		for i, key := range keys {
			results[i] = key * 2
			errors[i] = nil
		}
		return results, errors
	}

	loader := NewDataLoader(batchFn)
	ctx := context.Background()
	keys := []int{1, 2, 3, 4, 5}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		loader.LoadMany(ctx, keys)
	}
}

func BenchmarkUserLoader_Integration(b *testing.B) {
	if testDB == nil {
		b.Skip("Database not available")
	}

	userLoader := NewUserLoader(testDB)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Load random user ID to test cache behavior
			userID := (b.N % 5) + 1
			userLoader.Load(ctx, userID)
		}
	})
}