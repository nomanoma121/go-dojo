package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"
)

var (
	testDB   *sql.DB
	testPool *dockertest.Pool
	testResource *dockertest.Resource
)

func TestMain(m *testing.M) {
	var err error
	
	// Create docker pool
	testPool, err = dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not create docker pool: %s", err)
	}
	
	// Start PostgreSQL container
	testResource, err = testPool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "13",
		Env: []string{
			"POSTGRES_PASSWORD=testpass",
			"POSTGRES_DB=testdb",
			"POSTGRES_USER=testuser",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start PostgreSQL container: %s", err)
	}
	
	// Wait for database to be ready
	if err = testPool.Retry(func() error {
		var err error
		testDB, err = sql.Open("postgres", fmt.Sprintf(
			"postgres://testuser:testpass@localhost:%s/testdb?sslmode=disable",
			testResource.GetPort("5432/tcp")))
		if err != nil {
			return err
		}
		return testDB.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}
	
	// Run tests
	code := m.Run()
	
	// Cleanup
	if testDB != nil {
		testDB.Close()
	}
	if testPool != nil && testResource != nil {
		testPool.Purge(testResource)
	}
	
	os.Exit(code)
}

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	// Clean up tables for each test
	_, err := testDB.Exec("TRUNCATE TABLE posts, users RESTART IDENTITY CASCADE")
	require.NoError(t, err)
	
	require.NoError(t, InitSchema(testDB))
	
	return testDB, func() {
		// Cleanup after test
		testDB.Exec("TRUNCATE TABLE posts, users RESTART IDENTITY CASCADE")
	}
}

func TestDatabaseConnection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	err := db.Ping()
	assert.NoError(t, err)
}

func TestInitSchema(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	// Check that tables exist
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'users'
		)`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists)
	
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'posts'
		)`).Scan(&exists)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestUserRepository_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	repo := NewUserRepository(db)
	
	user := &User{
		Name:  "Test User",
		Email: "test@example.com",
	}
	
	err := repo.Create(user)
	require.NoError(t, err)
	
	assert.NotZero(t, user.ID)
	assert.NotZero(t, user.CreatedAt)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, "test@example.com", user.Email)
}

func TestUserRepository_Create_DuplicateEmail(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	repo := NewUserRepository(db)
	
	// Create first user
	user1 := &User{Name: "User 1", Email: "test@example.com"}
	err := repo.Create(user1)
	require.NoError(t, err)
	
	// Try to create second user with same email
	user2 := &User{Name: "User 2", Email: "test@example.com"}
	err = repo.Create(user2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestUserRepository_GetByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	userRepo := NewUserRepository(db)
	postRepo := NewPostRepository(db)
	
	// Create user
	user := &User{Name: "Test User", Email: "test@example.com"}
	err := userRepo.Create(user)
	require.NoError(t, err)
	
	// Create posts
	post1 := &Post{UserID: user.ID, Title: "Post 1", Content: "Content 1"}
	err = postRepo.Create(post1)
	require.NoError(t, err)
	
	post2 := &Post{UserID: user.ID, Title: "Post 2", Content: "Content 2"}
	err = postRepo.Create(post2)
	require.NoError(t, err)
	
	// Get user with posts
	retrieved, err := userRepo.GetByID(user.ID)
	require.NoError(t, err)
	
	assert.Equal(t, user.ID, retrieved.ID)
	assert.Equal(t, "Test User", retrieved.Name)
	assert.Equal(t, "test@example.com", retrieved.Email)
	assert.Len(t, retrieved.Posts, 2)
	assert.Equal(t, "Post 1", retrieved.Posts[0].Title)
	assert.Equal(t, "Post 2", retrieved.Posts[1].Title)
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	repo := NewUserRepository(db)
	
	user, err := repo.GetByID(999)
	assert.Error(t, err)
	assert.Nil(t, user)
}

func TestUserRepository_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	repo := NewUserRepository(db)
	
	// Create user
	user := &User{Name: "Original Name", Email: "original@example.com"}
	err := repo.Create(user)
	require.NoError(t, err)
	
	// Update user
	user.Name = "Updated Name"
	user.Email = "updated@example.com"
	err = repo.Update(user)
	require.NoError(t, err)
	
	// Verify update
	retrieved, err := repo.GetByID(user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", retrieved.Name)
	assert.Equal(t, "updated@example.com", retrieved.Email)
}

func TestUserRepository_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	userRepo := NewUserRepository(db)
	postRepo := NewPostRepository(db)
	
	// Create user and posts
	user := &User{Name: "Test User", Email: "test@example.com"}
	err := userRepo.Create(user)
	require.NoError(t, err)
	
	post := &Post{UserID: user.ID, Title: "Test Post", Content: "Content"}
	err = postRepo.Create(post)
	require.NoError(t, err)
	
	// Delete user
	err = userRepo.Delete(user.ID)
	require.NoError(t, err)
	
	// Verify user is deleted
	_, err = userRepo.GetByID(user.ID)
	assert.Error(t, err)
	
	// Verify posts are also deleted (cascade)
	posts, err := postRepo.GetByUserID(user.ID)
	require.NoError(t, err)
	assert.Empty(t, posts)
}

func TestPostRepository_Create(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	userRepo := NewUserRepository(db)
	postRepo := NewPostRepository(db)
	
	// Create user first
	user := &User{Name: "Test User", Email: "test@example.com"}
	err := userRepo.Create(user)
	require.NoError(t, err)
	
	// Create post
	post := &Post{
		UserID:  user.ID,
		Title:   "Test Post",
		Content: "Test Content",
	}
	
	err = postRepo.Create(post)
	require.NoError(t, err)
	
	assert.NotZero(t, post.ID)
	assert.NotZero(t, post.CreatedAt)
	assert.Equal(t, user.ID, post.UserID)
	assert.Equal(t, "Test Post", post.Title)
}

func TestPostRepository_GetByUserID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	userRepo := NewUserRepository(db)
	postRepo := NewPostRepository(db)
	
	// Create user
	user := &User{Name: "Test User", Email: "test@example.com"}
	err := userRepo.Create(user)
	require.NoError(t, err)
	
	// Create posts
	post1 := &Post{UserID: user.ID, Title: "First Post", Content: "First"}
	err = postRepo.Create(post1)
	require.NoError(t, err)
	
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	
	post2 := &Post{UserID: user.ID, Title: "Second Post", Content: "Second"}
	err = postRepo.Create(post2)
	require.NoError(t, err)
	
	// Get posts
	posts, err := postRepo.GetByUserID(user.ID)
	require.NoError(t, err)
	
	assert.Len(t, posts, 2)
	// Should be ordered by created_at DESC (newest first)
	assert.Equal(t, "Second Post", posts[0].Title)
	assert.Equal(t, "First Post", posts[1].Title)
}

func TestUserService_CreateUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	userRepo := NewUserRepository(db)
	postRepo := NewPostRepository(db)
	service := NewUserService(userRepo, postRepo)
	
	t.Run("valid user", func(t *testing.T) {
		req := &CreateUserRequest{
			Name:  "Valid User",
			Email: "valid@example.com",
		}
		
		user, err := service.CreateUser(req)
		require.NoError(t, err)
		
		assert.NotZero(t, user.ID)
		assert.Equal(t, "Valid User", user.Name)
		assert.Equal(t, "valid@example.com", user.Email)
	})
	
	t.Run("invalid name", func(t *testing.T) {
		req := &CreateUserRequest{
			Name:  "",
			Email: "valid@example.com",
		}
		
		user, err := service.CreateUser(req)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "name")
	})
	
	t.Run("invalid email", func(t *testing.T) {
		req := &CreateUserRequest{
			Name:  "Valid User",
			Email: "invalid-email",
		}
		
		user, err := service.CreateUser(req)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "email")
	})
}

func TestUserService_CreateUserWithPosts(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	userRepo := NewUserRepository(db)
	postRepo := NewPostRepository(db)
	service := NewUserService(userRepo, postRepo)
	
	t.Run("successful transaction", func(t *testing.T) {
		user := &User{
			Name:  "Transaction User",
			Email: "transaction@example.com",
		}
		
		posts := []Post{
			{Title: "Post 1", Content: "Content 1"},
			{Title: "Post 2", Content: "Content 2"},
		}
		
		err := service.CreateUserWithPosts(user, posts)
		require.NoError(t, err)
		
		// Verify user was created
		assert.NotZero(t, user.ID)
		
		// Verify posts were created
		userPosts, err := postRepo.GetByUserID(user.ID)
		require.NoError(t, err)
		assert.Len(t, userPosts, 2)
	})
	
	t.Run("rollback on validation error", func(t *testing.T) {
		user := &User{
			Name:  "", // Invalid name
			Email: "rollback@example.com",
		}
		
		posts := []Post{
			{Title: "Post 1", Content: "Content 1"},
		}
		
		err := service.CreateUserWithPosts(user, posts)
		assert.Error(t, err)
		
		// Verify no user was created
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM users WHERE email = 'rollback@example.com'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
		
		// Verify no posts were created
		err = db.QueryRow("SELECT COUNT(*) FROM posts WHERE title = 'Post 1'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestUserAPI_Integration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	server := httptest.NewServer(SetupServer(db))
	defer server.Close()
	
	t.Run("create user", func(t *testing.T) {
		payload := `{"name": "API User", "email": "api@example.com"}`
		resp, err := http.Post(server.URL+"/api/users", "application/json", 
			strings.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var user User
		err = json.NewDecoder(resp.Body).Decode(&user)
		require.NoError(t, err)
		
		assert.NotZero(t, user.ID)
		assert.Equal(t, "API User", user.Name)
		assert.Equal(t, "api@example.com", user.Email)
	})
	
	t.Run("create user validation error", func(t *testing.T) {
		payload := `{"name": "", "email": "invalid-email"}`
		resp, err := http.Post(server.URL+"/api/users", "application/json",
			strings.NewReader(payload))
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		
		var errResp ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errResp)
		require.NoError(t, err)
		
		assert.Equal(t, "validation failed", errResp.Error)
		assert.Contains(t, errResp.Details["name"], "required")
		assert.Contains(t, errResp.Details["email"], "invalid")
	})
}

func TestUserAPI_GetUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	server := httptest.NewServer(SetupServer(db))
	defer server.Close()
	
	// Create user first
	userRepo := NewUserRepository(db)
	postRepo := NewPostRepository(db)
	
	user := &User{Name: "Get User", Email: "get@example.com"}
	err := userRepo.Create(user)
	require.NoError(t, err)
	
	post := &Post{UserID: user.ID, Title: "User Post", Content: "Content"}
	err = postRepo.Create(post)
	require.NoError(t, err)
	
	t.Run("get existing user", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/api/users/%d", server.URL, user.ID))
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var retrieved User
		err = json.NewDecoder(resp.Body).Decode(&retrieved)
		require.NoError(t, err)
		
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, "Get User", retrieved.Name)
		assert.Equal(t, "get@example.com", retrieved.Email)
		assert.Len(t, retrieved.Posts, 1)
		assert.Equal(t, "User Post", retrieved.Posts[0].Title)
	})
	
	t.Run("get non-existent user", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/api/users/999", server.URL))
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestUserAPI_UpdateUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	server := httptest.NewServer(SetupServer(db))
	defer server.Close()
	
	// Create user first
	userRepo := NewUserRepository(db)
	user := &User{Name: "Original", Email: "original@example.com"}
	err := userRepo.Create(user)
	require.NoError(t, err)
	
	t.Run("update existing user", func(t *testing.T) {
		payload := `{"name": "Updated", "email": "updated@example.com"}`
		req, err := http.NewRequest("PUT", 
			fmt.Sprintf("%s/api/users/%d", server.URL, user.ID),
			strings.NewReader(payload))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var updated User
		err = json.NewDecoder(resp.Body).Decode(&updated)
		require.NoError(t, err)
		
		assert.Equal(t, user.ID, updated.ID)
		assert.Equal(t, "Updated", updated.Name)
		assert.Equal(t, "updated@example.com", updated.Email)
	})
}

func TestUserAPI_DeleteUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	server := httptest.NewServer(SetupServer(db))
	defer server.Close()
	
	// Create user first
	userRepo := NewUserRepository(db)
	user := &User{Name: "To Delete", Email: "delete@example.com"}
	err := userRepo.Create(user)
	require.NoError(t, err)
	
	t.Run("delete existing user", func(t *testing.T) {
		req, err := http.NewRequest("DELETE",
			fmt.Sprintf("%s/api/users/%d", server.URL, user.ID), nil)
		require.NoError(t, err)
		
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		
		// Verify user is deleted
		_, err = userRepo.GetByID(user.ID)
		assert.Error(t, err)
	})
}

func TestParallelTests(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"CreateUsers", testCreateMultipleUsers},
		{"GetUsers", testGetMultipleUsers},
		{"UpdateUsers", testUpdateMultipleUsers},
	}
	
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}

func testCreateMultipleUsers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	server := httptest.NewServer(SetupServer(db))
	defer server.Close()
	
	for i := 0; i < 5; i++ {
		payload := fmt.Sprintf(`{"name": "User %d", "email": "user%d@example.com"}`, i, i)
		resp, err := http.Post(server.URL+"/api/users", "application/json",
			strings.NewReader(payload))
		require.NoError(t, err)
		resp.Body.Close()
		
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	}
}

func testGetMultipleUsers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	userRepo := NewUserRepository(db)
	
	// Create test users
	for i := 0; i < 3; i++ {
		user := &User{
			Name:  fmt.Sprintf("Test User %d", i),
			Email: fmt.Sprintf("test%d@example.com", i),
		}
		err := userRepo.Create(user)
		require.NoError(t, err)
	}
	
	server := httptest.NewServer(SetupServer(db))
	defer server.Close()
	
	// Get each user
	for i := 1; i <= 3; i++ {
		resp, err := http.Get(fmt.Sprintf("%s/api/users/%d", server.URL, i))
		require.NoError(t, err)
		resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

func testUpdateMultipleUsers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	userRepo := NewUserRepository(db)
	
	// Create test users
	for i := 0; i < 3; i++ {
		user := &User{
			Name:  fmt.Sprintf("Original %d", i),
			Email: fmt.Sprintf("original%d@example.com", i),
		}
		err := userRepo.Create(user)
		require.NoError(t, err)
	}
	
	server := httptest.NewServer(SetupServer(db))
	defer server.Close()
	
	// Update each user
	for i := 1; i <= 3; i++ {
		payload := fmt.Sprintf(`{"name": "Updated %d", "email": "updated%d@example.com"}`, i, i)
		req, err := http.NewRequest("PUT",
			fmt.Sprintf("%s/api/users/%d", server.URL, i),
			strings.NewReader(payload))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

// Helper functions for testing
func createTestUser(t *testing.T, db *sql.DB, name, email string) *User {
	repo := NewUserRepository(db)
	user := &User{Name: name, Email: email}
	err := repo.Create(user)
	require.NoError(t, err)
	return user
}

func createTestPost(t *testing.T, db *sql.DB, userID int, title, content string) *Post {
	repo := NewPostRepository(db)
	post := &Post{UserID: userID, Title: title, Content: content}
	err := repo.Create(post)
	require.NoError(t, err)
	return post
}