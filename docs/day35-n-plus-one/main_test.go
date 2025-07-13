package main

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	_ "github.com/lib/pq"
)

var (
	testDB   *sql.DB
	pool     *dockertest.Pool
	resource *dockertest.Resource
)

func TestMain(m *testing.M) {
	var err error
	pool, err = dockertest.NewPool("")
	if err != nil {
		panic(fmt.Sprintf("Could not connect to docker: %s", err))
	}

	// Start PostgreSQL container
	resource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			"POSTGRES_PASSWORD=test",
			"POSTGRES_DB=testdb",
			"POSTGRES_USER=postgres",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		panic(fmt.Sprintf("Could not start resource: %s", err))
	}

	hostAndPort := resource.GetHostPort("5432/tcp")
	databaseUrl := fmt.Sprintf("postgres://postgres:test@%s/testdb?sslmode=disable", hostAndPort)

	// Wait for database to be ready
	pool.MaxWait = 120 * time.Second
	if err = pool.Retry(func() error {
		var err error
		testDB, err = sql.Open("postgres", databaseUrl)
		if err != nil {
			return err
		}
		return testDB.Ping()
	}); err != nil {
		panic(fmt.Sprintf("Could not connect to database: %s", err))
	}

	// Setup database
	if err := setupDatabase(testDB); err != nil {
		panic(fmt.Sprintf("Could not setup database: %s", err))
	}

	// Seed test data
	if err := seedTestData(testDB); err != nil {
		panic(fmt.Sprintf("Could not seed test data: %s", err))
	}

	code := m.Run()

	// Cleanup
	if err := pool.Purge(resource); err != nil {
		panic(fmt.Sprintf("Could not purge resource: %s", err))
	}

	if code != 0 {
		panic("Tests failed")
	}
}

func TestUserService_GetUsersWithPostsNaive(t *testing.T) {
	userService := NewUserService(testDB)
	ctx := context.Background()

	users, err := userService.GetUsersWithPostsNaive(ctx)
	if err != nil {
		t.Fatalf("GetUsersWithPostsNaive failed: %v", err)
	}

	if len(users) == 0 {
		t.Error("Expected users to be returned")
	}

	// Verify that users have posts
	totalPosts := 0
	for _, user := range users {
		totalPosts += len(user.Posts)
	}

	if totalPosts == 0 {
		t.Error("Expected users to have posts")
	}

	t.Logf("Retrieved %d users with %d total posts using naive approach", len(users), totalPosts)
}

func TestUserService_GetUsersWithPostsEager(t *testing.T) {
	userService := NewUserService(testDB)
	ctx := context.Background()

	users, err := userService.GetUsersWithPostsEager(ctx)
	if err != nil {
		t.Fatalf("GetUsersWithPostsEager failed: %v", err)
	}

	if len(users) == 0 {
		t.Error("Expected users to be returned")
	}

	// Verify that users have posts
	totalPosts := 0
	for _, user := range users {
		totalPosts += len(user.Posts)
	}

	if totalPosts == 0 {
		t.Error("Expected users to have posts")
	}

	t.Logf("Retrieved %d users with %d total posts using eager loading", len(users), totalPosts)
}

func TestUserService_GetUsersWithPostsBatch(t *testing.T) {
	userService := NewUserService(testDB)
	ctx := context.Background()

	users, err := userService.GetUsersWithPostsBatch(ctx)
	if err != nil {
		t.Fatalf("GetUsersWithPostsBatch failed: %v", err)
	}

	if len(users) == 0 {
		t.Error("Expected users to be returned")
	}

	// Verify that users have posts
	totalPosts := 0
	for _, user := range users {
		totalPosts += len(user.Posts)
	}

	if totalPosts == 0 {
		t.Error("Expected users to have posts")
	}

	t.Logf("Retrieved %d users with %d total posts using batch loading", len(users), totalPosts)
}

func TestUserService_GetUsersByIDsWithPosts(t *testing.T) {
	userService := NewUserService(testDB)
	ctx := context.Background()

	// Test with specific user IDs
	userIDs := []int{1, 3, 5}
	users, err := userService.GetUsersByIDsWithPosts(ctx, userIDs)
	if err != nil {
		t.Fatalf("GetUsersByIDsWithPosts failed: %v", err)
	}

	if len(users) != len(userIDs) {
		t.Errorf("Expected %d users, got %d", len(userIDs), len(users))
	}

	// Verify user IDs match
	for i, user := range users {
		expectedID := userIDs[i]
		if user.ID != expectedID {
			t.Errorf("Expected user ID %d, got %d", expectedID, user.ID)
		}
	}

	t.Logf("Retrieved %d specific users with their posts", len(users))
}

func TestPostService_GetPostsByUserIDs(t *testing.T) {
	postService := NewPostService(testDB)
	ctx := context.Background()

	userIDs := []int{1, 2, 3}
	posts, err := postService.GetPostsByUserIDs(ctx, userIDs)
	if err != nil {
		t.Fatalf("GetPostsByUserIDs failed: %v", err)
	}

	if len(posts) == 0 {
		t.Error("Expected posts to be returned")
	}

	// Verify all posts belong to the specified users
	for _, post := range posts {
		found := false
		for _, userID := range userIDs {
			if post.UserID == userID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Post %d belongs to user %d, which is not in the requested list", post.ID, post.UserID)
		}
	}

	t.Logf("Retrieved %d posts for users %v", len(posts), userIDs)
}

func TestPostService_GetPostsWithAuthorsNaive(t *testing.T) {
	postService := NewPostService(testDB)
	ctx := context.Background()

	posts, err := postService.GetPostsWithAuthorsNaive(ctx)
	if err != nil {
		t.Fatalf("GetPostsWithAuthorsNaive failed: %v", err)
	}

	if len(posts) == 0 {
		t.Error("Expected posts to be returned")
	}

	// Verify posts have authors
	for _, post := range posts {
		if post.Author == nil {
			t.Errorf("Post %d should have an author", post.ID)
		}
	}

	t.Logf("Retrieved %d posts with authors using naive approach", len(posts))
}

func TestPostService_GetPostsWithAuthorsOptimized(t *testing.T) {
	postService := NewPostService(testDB)
	ctx := context.Background()

	posts, err := postService.GetPostsWithAuthorsOptimized(ctx)
	if err != nil {
		t.Fatalf("GetPostsWithAuthorsOptimized failed: %v", err)
	}

	if len(posts) == 0 {
		t.Error("Expected posts to be returned")
	}

	// Verify posts have authors
	for _, post := range posts {
		if post.Author == nil {
			t.Errorf("Post %d should have an author", post.ID)
		}
	}

	t.Logf("Retrieved %d posts with authors using optimized approach", len(posts))
}

func TestQueryCounter(t *testing.T) {
	counter := NewQueryCounter(testDB)

	if counter.GetCount() != 0 {
		t.Error("Initial count should be 0")
	}

	// Execute some queries
	ctx := context.Background()
	_, err := counter.QueryContext(ctx, "SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if counter.GetCount() != 1 {
		t.Errorf("Expected count 1, got %d", counter.GetCount())
	}

	_ = counter.QueryRowContext(ctx, "SELECT id FROM users LIMIT 1")
	if counter.GetCount() != 2 {
		t.Errorf("Expected count 2, got %d", counter.GetCount())
	}

	counter.Reset()
	if counter.GetCount() != 0 {
		t.Error("Count should be 0 after reset")
	}
}

func TestPerformanceProfiler(t *testing.T) {
	profiler := NewPerformanceProfiler()
	
	profiler.Start()
	
	// Simulate some query durations
	profiler.AddQuery(10 * time.Millisecond)
	profiler.AddQuery(20 * time.Millisecond)
	profiler.AddQuery(30 * time.Millisecond)
	
	count, total, avg := profiler.GetStats()
	
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
	
	expectedTotal := 60 * time.Millisecond
	if total != expectedTotal {
		t.Errorf("Expected total %v, got %v", expectedTotal, total)
	}
	
	expectedAvg := 20 * time.Millisecond
	if avg != expectedAvg {
		t.Errorf("Expected average %v, got %v", expectedAvg, avg)
	}
}

// Benchmark tests to compare performance

func BenchmarkUserService_GetUsersWithPostsNaive(b *testing.B) {
	userService := NewUserService(testDB)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := userService.GetUsersWithPostsNaive(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUserService_GetUsersWithPostsEager(b *testing.B) {
	userService := NewUserService(testDB)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := userService.GetUsersWithPostsEager(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUserService_GetUsersWithPostsBatch(b *testing.B) {
	userService := NewUserService(testDB)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := userService.GetUsersWithPostsBatch(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPostService_GetPostsWithAuthorsNaive(b *testing.B) {
	postService := NewPostService(testDB)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := postService.GetPostsWithAuthorsNaive(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPostService_GetPostsWithAuthorsOptimized(b *testing.B) {
	postService := NewPostService(testDB)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := postService.GetPostsWithAuthorsOptimized(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Table-driven test for comprehensive comparison
func TestNPlusOneComparison(t *testing.T) {
	userService := NewUserService(testDB)
	postService := NewPostService(testDB)
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() (interface{}, error)
	}{
		{
			name: "Users-Naive",
			fn: func() (interface{}, error) {
				return userService.GetUsersWithPostsNaive(ctx)
			},
		},
		{
			name: "Users-Eager",
			fn: func() (interface{}, error) {
				return userService.GetUsersWithPostsEager(ctx)
			},
		},
		{
			name: "Users-Batch",
			fn: func() (interface{}, error) {
				return userService.GetUsersWithPostsBatch(ctx)
			},
		},
		{
			name: "Posts-Naive",
			fn: func() (interface{}, error) {
				return postService.GetPostsWithAuthorsNaive(ctx)
			},
		},
		{
			name: "Posts-Optimized",
			fn: func() (interface{}, error) {
				return postService.GetPostsWithAuthorsOptimized(ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			result, err := tt.fn()
			duration := time.Since(start)

			if err != nil {
				t.Fatalf("%s failed: %v", tt.name, err)
			}

			if result == nil {
				t.Fatalf("%s returned nil result", tt.name)
			}

			t.Logf("%s completed in %v", tt.name, duration)
		})
	}
}

// Helper function for edge case testing
func TestEmptyResults(t *testing.T) {
	userService := NewUserService(testDB)
	ctx := context.Background()

	// Test with empty user IDs
	users, err := userService.GetUsersByIDsWithPosts(ctx, []int{})
	if err != nil {
		t.Fatalf("GetUsersByIDsWithPosts with empty IDs failed: %v", err)
	}

	if len(users) != 0 {
		t.Errorf("Expected 0 users for empty IDs, got %d", len(users))
	}

	// Test with non-existent user IDs
	users, err = userService.GetUsersByIDsWithPosts(ctx, []int{9999, 10000})
	if err != nil {
		t.Fatalf("GetUsersByIDsWithPosts with non-existent IDs failed: %v", err)
	}

	if len(users) != 0 {
		t.Errorf("Expected 0 users for non-existent IDs, got %d", len(users))
	}
}