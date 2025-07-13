package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// User represents a user entity
type User struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Posts    []Post    `json:"posts,omitempty"`
	Created  time.Time `json:"created"`
}

// Post represents a post entity
type Post struct {
	ID      int       `json:"id"`
	UserID  int       `json:"user_id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Author  *User     `json:"author,omitempty"`
	Created time.Time `json:"created"`
}

// UserService handles user-related operations
type UserService struct {
	db *sql.DB
}

// NewUserService creates a new user service
func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

// GetUsersWithPostsNaive retrieves users with their posts using N+1 approach (problematic)
func (s *UserService) GetUsersWithPostsNaive(ctx context.Context) ([]User, error) {
	log.Println("🚨 Using NAIVE approach (N+1 problem)")
	
	// Step 1: Get all users (1 query)
	userQuery := "SELECT id, name, email, created FROM users ORDER BY id"
	rows, err := s.db.QueryContext(ctx, userQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Created)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	// Step 2: Get posts for each user (N queries - THIS IS THE PROBLEM!)
	for i := range users {
		postQuery := "SELECT id, user_id, title, content, created FROM posts WHERE user_id = $1 ORDER BY created DESC"
		postRows, err := s.db.QueryContext(ctx, postQuery, users[i].ID)
		if err != nil {
			return nil, err
		}

		var posts []Post
		for postRows.Next() {
			var post Post
			err := postRows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created)
			if err != nil {
				postRows.Close()
				return nil, err
			}
			posts = append(posts, post)
		}
		postRows.Close()
		users[i].Posts = posts
	}

	log.Printf("⚠️  Executed %d queries (1 for users + %d for posts)", 1+len(users), len(users))
	return users, nil
}

// GetUsersWithPostsEager retrieves users with their posts using eager loading (JOIN)
func (s *UserService) GetUsersWithPostsEager(ctx context.Context) ([]User, error) {
	log.Println("✅ Using EAGER LOADING approach (JOIN)")
	
	query := `
		SELECT u.id, u.name, u.email, u.created,
		       p.id, p.user_id, p.title, p.content, p.created
		FROM users u
		LEFT JOIN posts p ON u.id = p.user_id
		ORDER BY u.id, p.created DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userMap := make(map[int]*User)
	var userOrder []int

	for rows.Next() {
		var u User
		var p Post
		var postID, postUserID sql.NullInt64
		var postTitle, postContent sql.NullString
		var postCreated sql.NullTime

		err := rows.Scan(
			&u.ID, &u.Name, &u.Email, &u.Created,
			&postID, &postUserID, &postTitle, &postContent, &postCreated,
		)
		if err != nil {
			return nil, err
		}

		// If user doesn't exist in map, add it
		if _, exists := userMap[u.ID]; !exists {
			userMap[u.ID] = &User{
				ID:      u.ID,
				Name:    u.Name,
				Email:   u.Email,
				Created: u.Created,
				Posts:   []Post{},
			}
			userOrder = append(userOrder, u.ID)
		}

		// If post exists, add it to the user
		if postID.Valid {
			p.ID = int(postID.Int64)
			p.UserID = int(postUserID.Int64)
			p.Title = postTitle.String
			p.Content = postContent.String
			p.Created = postCreated.Time

			userMap[u.ID].Posts = append(userMap[u.ID].Posts, p)
		}
	}

	// Convert map to slice in original order
	var users []User
	for _, userID := range userOrder {
		users = append(users, *userMap[userID])
	}

	log.Printf("✅ Executed 1 query (JOIN)")
	return users, nil
}

// GetUsersWithPostsBatch retrieves users with their posts using batch loading (IN query)
func (s *UserService) GetUsersWithPostsBatch(ctx context.Context) ([]User, error) {
	log.Println("✅ Using BATCH LOADING approach (IN clause)")
	
	// Step 1: Get all users (1 query)
	userQuery := "SELECT id, name, email, created FROM users ORDER BY id"
	rows, err := s.db.QueryContext(ctx, userQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	var userIDs []int
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Created)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
		userIDs = append(userIDs, user.ID)
	}

	if len(userIDs) == 0 {
		return users, nil
	}

	// Step 2: Get all posts for all users in one query (1 query)
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	postQuery := fmt.Sprintf(`
		SELECT id, user_id, title, content, created 
		FROM posts 
		WHERE user_id IN (%s) 
		ORDER BY user_id, created DESC
	`, strings.Join(placeholders, ","))

	postRows, err := s.db.QueryContext(ctx, postQuery, args...)
	if err != nil {
		return nil, err
	}
	defer postRows.Close()

	// Group posts by user ID
	postsByUserID := make(map[int][]Post)
	for postRows.Next() {
		var post Post
		err := postRows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created)
		if err != nil {
			return nil, err
		}
		postsByUserID[post.UserID] = append(postsByUserID[post.UserID], post)
	}

	// Assign posts to users
	for i := range users {
		if posts, exists := postsByUserID[users[i].ID]; exists {
			users[i].Posts = posts
		}
	}

	log.Printf("✅ Executed 2 queries (1 for users + 1 batch for posts)")
	return users, nil
}

// GetUsersByIDsWithPosts retrieves specific users with their posts using batch loading
func (s *UserService) GetUsersByIDsWithPosts(ctx context.Context, userIDs []int) ([]User, error) {
	if len(userIDs) == 0 {
		return []User{}, nil
	}

	// Create placeholders for IN clause
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	// Get users
	userQuery := fmt.Sprintf(`
		SELECT id, name, email, created 
		FROM users 
		WHERE id IN (%s) 
		ORDER BY id
	`, strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, userQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Created)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	// Get posts for these users
	postQuery := fmt.Sprintf(`
		SELECT id, user_id, title, content, created 
		FROM posts 
		WHERE user_id IN (%s) 
		ORDER BY user_id, created DESC
	`, strings.Join(placeholders, ","))

	postRows, err := s.db.QueryContext(ctx, postQuery, args...)
	if err != nil {
		return nil, err
	}
	defer postRows.Close()

	// Group posts by user ID
	postsByUserID := make(map[int][]Post)
	for postRows.Next() {
		var post Post
		err := postRows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created)
		if err != nil {
			return nil, err
		}
		postsByUserID[post.UserID] = append(postsByUserID[post.UserID], post)
	}

	// Assign posts to users
	for i := range users {
		if posts, exists := postsByUserID[users[i].ID]; exists {
			users[i].Posts = posts
		}
	}

	return users, nil
}

// PostService handles post-related operations
type PostService struct {
	db *sql.DB
}

// NewPostService creates a new post service
func NewPostService(db *sql.DB) *PostService {
	return &PostService{db: db}
}

// GetPostsByUserIDs retrieves posts for multiple users in a single query
func (s *PostService) GetPostsByUserIDs(ctx context.Context, userIDs []int) ([]Post, error) {
	if len(userIDs) == 0 {
		return []Post{}, nil
	}

	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, title, content, created 
		FROM posts 
		WHERE user_id IN (%s) 
		ORDER BY created DESC
	`, strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// GetPostsWithAuthorsNaive retrieves posts with their authors using N+1 approach
func (s *PostService) GetPostsWithAuthorsNaive(ctx context.Context) ([]Post, error) {
	log.Println("🚨 Using NAIVE approach for posts (N+1 problem)")
	
	// Step 1: Get all posts (1 query)
	postQuery := "SELECT id, user_id, title, content, created FROM posts ORDER BY created DESC"
	rows, err := s.db.QueryContext(ctx, postQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	// Step 2: Get author for each post (N queries - PROBLEM!)
	for i := range posts {
		userQuery := "SELECT id, name, email, created FROM users WHERE id = $1"
		var user User
		err := s.db.QueryRowContext(ctx, userQuery, posts[i].UserID).
			Scan(&user.ID, &user.Name, &user.Email, &user.Created)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		if err != sql.ErrNoRows {
			posts[i].Author = &user
		}
	}

	log.Printf("⚠️  Executed %d queries (1 for posts + %d for authors)", 1+len(posts), len(posts))
	return posts, nil
}

// GetPostsWithAuthorsOptimized retrieves posts with their authors using optimized approach
func (s *PostService) GetPostsWithAuthorsOptimized(ctx context.Context) ([]Post, error) {
	log.Println("✅ Using OPTIMIZED approach for posts (JOIN)")
	
	query := `
		SELECT p.id, p.user_id, p.title, p.content, p.created,
		       u.id, u.name, u.email, u.created
		FROM posts p
		JOIN users u ON p.user_id = u.id
		ORDER BY p.created DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var post Post
		var user User
		err := rows.Scan(
			&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created,
			&user.ID, &user.Name, &user.Email, &user.Created,
		)
		if err != nil {
			return nil, err
		}
		post.Author = &user
		posts = append(posts, post)
	}

	log.Printf("✅ Executed 1 query (JOIN)")
	return posts, nil
}

// QueryCounter counts the number of database queries executed
type QueryCounter struct {
	count int
	db    *sql.DB
}

// NewQueryCounter creates a new query counter wrapper
func NewQueryCounter(db *sql.DB) *QueryCounter {
	return &QueryCounter{
		count: 0,
		db:    db,
	}
}

// Query executes a query and increments the counter
func (qc *QueryCounter) Query(query string, args ...interface{}) (*sql.Rows, error) {
	qc.count++
	log.Printf("📊 Query #%d: %s", qc.count, truncateQuery(query))
	return qc.db.Query(query, args...)
}

// QueryContext executes a query with context and increments the counter
func (qc *QueryCounter) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	qc.count++
	log.Printf("📊 Query #%d: %s", qc.count, truncateQuery(query))
	return qc.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a single-row query and increments the counter
func (qc *QueryCounter) QueryRow(query string, args ...interface{}) *sql.Row {
	qc.count++
	log.Printf("📊 Query #%d: %s", qc.count, truncateQuery(query))
	return qc.db.QueryRow(query, args...)
}

// QueryRowContext executes a single-row query with context and increments the counter
func (qc *QueryCounter) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	qc.count++
	log.Printf("📊 Query #%d: %s", qc.count, truncateQuery(query))
	return qc.db.QueryRowContext(ctx, query, args...)
}

// GetCount returns the current query count
func (qc *QueryCounter) GetCount() int {
	return qc.count
}

// Reset resets the query counter
func (qc *QueryCounter) Reset() {
	qc.count = 0
}

// truncateQuery truncates a SQL query for logging
func truncateQuery(query string) string {
	// Remove extra whitespace and newlines
	cleaned := strings.ReplaceAll(strings.TrimSpace(query), "\n", " ")
	cleaned = strings.ReplaceAll(cleaned, "\t", " ")
	for strings.Contains(cleaned, "  ") {
		cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	}
	
	if len(cleaned) > 100 {
		return cleaned[:97] + "..."
	}
	return cleaned
}

// PerformanceProfiler measures query performance
type PerformanceProfiler struct {
	queryCount    int
	totalDuration time.Duration
	startTime     time.Time
}

// NewPerformanceProfiler creates a new performance profiler
func NewPerformanceProfiler() *PerformanceProfiler {
	return &PerformanceProfiler{}
}

// Start starts profiling
func (p *PerformanceProfiler) Start() {
	p.startTime = time.Now()
	p.queryCount = 0
	p.totalDuration = 0
}

// AddQuery records a query execution
func (p *PerformanceProfiler) AddQuery(duration time.Duration) {
	p.queryCount++
	p.totalDuration += duration
}

// GetStats returns performance statistics
func (p *PerformanceProfiler) GetStats() (int, time.Duration, time.Duration) {
	var avgDuration time.Duration
	if p.queryCount > 0 {
		avgDuration = p.totalDuration / time.Duration(p.queryCount)
	}
	return p.queryCount, p.totalDuration, avgDuration
}

// setupDatabase initializes the database schema
func setupDatabase(db *sql.DB) error {
	schema := `
	DROP TABLE IF EXISTS posts;
	DROP TABLE IF EXISTS users;

	CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE posts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
		title VARCHAR(255) NOT NULL,
		content TEXT,
		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX idx_posts_user_id ON posts(user_id);
	CREATE INDEX idx_posts_created ON posts(created);
	CREATE INDEX idx_users_email ON users(email);
	`

	_, err := db.Exec(schema)
	return err
}

// seedTestData inserts test data into the database
func seedTestData(db *sql.DB) error {
	// Insert users
	userNames := []string{
		"Alice", "Bob", "Charlie", "Diana", "Eve",
		"Frank", "Grace", "Henry", "Ivy", "Jack",
	}

	for i, name := range userNames {
		email := fmt.Sprintf("%s@example.com", strings.ToLower(name))
		_, err := db.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", name, email)
		if err != nil {
			return err
		}

		// Insert 2-5 posts per user
		postCount := 2 + (i % 4) // 2-5 posts per user
		for j := 0; j < postCount; j++ {
			title := fmt.Sprintf("%s's Post #%d", name, j+1)
			content := fmt.Sprintf("This is the content of post #%d by %s. It contains some interesting information.", j+1, name)
			_, err := db.Exec("INSERT INTO posts (user_id, title, content) VALUES ($1, $2, $3)", i+1, title, content)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func main() {
	fmt.Println("=== N+1 Problem Solution Demo ===")

	// Database connection
	db, err := sql.Open("postgres", "postgres://postgres:test@localhost:5432/testdb?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Initialize database
	if err := setupDatabase(db); err != nil {
		panic(err)
	}

	// Seed test data
	if err := seedTestData(db); err != nil {
		panic(err)
	}

	fmt.Println("Database setup completed with test data.")

	userService := NewUserService(db)
	postService := NewPostService(db)
	ctx := context.Background()

	// Demo 1: N+1 Problem demonstration
	fmt.Println("\n=== 1. N+1 Problem Demonstration ===")
	
	fmt.Println("\n--- Naive Approach (N+1 Problem) ---")
	start := time.Now()
	users1, err := userService.GetUsersWithPostsNaive(ctx)
	naiveDuration := time.Since(start)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Retrieved %d users with %d total posts in %v\n", 
		len(users1), countTotalPosts(users1), naiveDuration)

	// Demo 2: Eager Loading solution
	fmt.Println("\n--- Eager Loading Solution (JOIN) ---")
	start = time.Now()
	users2, err := userService.GetUsersWithPostsEager(ctx)
	eagerDuration := time.Since(start)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Retrieved %d users with %d total posts in %v\n", 
		len(users2), countTotalPosts(users2), eagerDuration)

	// Demo 3: Batch Loading solution
	fmt.Println("\n--- Batch Loading Solution (IN clause) ---")
	start = time.Now()
	users3, err := userService.GetUsersWithPostsBatch(ctx)
	batchDuration := time.Since(start)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Retrieved %d users with %d total posts in %v\n", 
		len(users3), countTotalPosts(users3), batchDuration)

	// Demo 4: Posts with authors
	fmt.Println("\n=== 2. Posts with Authors ===")
	
	fmt.Println("\n--- Naive Approach ---")
	start = time.Now()
	posts1, err := postService.GetPostsWithAuthorsNaive(ctx)
	naivePostsDuration := time.Since(start)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Retrieved %d posts with authors in %v\n", len(posts1), naivePostsDuration)

	fmt.Println("\n--- Optimized Approach ---")
	start = time.Now()
	posts2, err := postService.GetPostsWithAuthorsOptimized(ctx)
	optimizedPostsDuration := time.Since(start)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Retrieved %d posts with authors in %v\n", len(posts2), optimizedPostsDuration)

	// Performance comparison
	fmt.Println("\n=== 3. Performance Comparison ===")
	fmt.Printf("Users with Posts:\n")
	fmt.Printf("  Naive (N+1):     %v\n", naiveDuration)
	fmt.Printf("  Eager Loading:   %v (%.1fx faster)\n", eagerDuration, float64(naiveDuration)/float64(eagerDuration))
	fmt.Printf("  Batch Loading:   %v (%.1fx faster)\n", batchDuration, float64(naiveDuration)/float64(batchDuration))
	
	fmt.Printf("\nPosts with Authors:\n")
	fmt.Printf("  Naive (N+1):     %v\n", naivePostsDuration)
	fmt.Printf("  Optimized:       %v (%.1fx faster)\n", optimizedPostsDuration, float64(naivePostsDuration)/float64(optimizedPostsDuration))

	fmt.Println("\nN+1 problem solution demo completed!")
}

// countTotalPosts counts the total number of posts across all users
func countTotalPosts(users []User) int {
	total := 0
	for _, user := range users {
		total += len(user.Posts)
	}
	return total
}