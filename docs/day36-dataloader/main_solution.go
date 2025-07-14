package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// User represents a user entity
type User struct {
	ID      int       `json:"id"`
	Name    string    `json:"name"`
	Email   string    `json:"email"`
	Created time.Time `json:"created"`
}

// Post represents a post entity
type Post struct {
	ID      int       `json:"id"`
	UserID  int       `json:"user_id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Created time.Time `json:"created"`
}

// BatchFunc defines the function signature for batch loading
type BatchFunc[K comparable, V any] func(ctx context.Context, keys []K) ([]V, []error)

// DataLoader provides batching and caching functionality
type DataLoader[K comparable, V any] struct {
	batchFn      BatchFunc[K, V]
	cache        map[K]*result[V]
	batch        []K
	waiting      map[K][]chan *result[V]
	maxBatchSize int
	batchTimeout time.Duration
	mu           sync.Mutex
	stats        *StatsCollector
}

// result holds the value and error for a specific key
type result[V any] struct {
	value V
	err   error
}

// NewDataLoader creates a new DataLoader
func NewDataLoader[K comparable, V any](
	batchFn BatchFunc[K, V],
	options ...Option[K, V],
) *DataLoader[K, V] {
	dl := &DataLoader[K, V]{
		batchFn:      batchFn,
		cache:        make(map[K]*result[V]),
		waiting:      make(map[K][]chan *result[V]),
		maxBatchSize: 100,
		batchTimeout: 16 * time.Millisecond,
		stats:        NewStatsCollector(),
	}

	for _, opt := range options {
		opt(dl)
	}

	return dl
}

// Option defines configuration options for DataLoader
type Option[K comparable, V any] func(*DataLoader[K, V])

// WithMaxBatchSize sets the maximum batch size
func WithMaxBatchSize[K comparable, V any](size int) Option[K, V] {
	return func(dl *DataLoader[K, V]) {
		dl.maxBatchSize = size
	}
}

// WithBatchTimeout sets the batch timeout
func WithBatchTimeout[K comparable, V any](timeout time.Duration) Option[K, V] {
	return func(dl *DataLoader[K, V]) {
		dl.batchTimeout = timeout
	}
}

// WithStatsCollector sets the statistics collector
func WithStatsCollector[K comparable, V any](stats *StatsCollector) Option[K, V] {
	return func(dl *DataLoader[K, V]) {
		dl.stats = stats
	}
}

// Load loads a single value by key
func (dl *DataLoader[K, V]) Load(ctx context.Context, key K) (V, error) {
	return dl.LoadThunk(ctx, key)()
}

// LoadMany loads multiple values by keys
func (dl *DataLoader[K, V]) LoadMany(ctx context.Context, keys []K) ([]V, []error) {
	thunks := make([]Thunk[V], len(keys))
	for i, key := range keys {
		thunks[i] = dl.LoadThunk(ctx, key)
	}

	values := make([]V, len(keys))
	errors := make([]error, len(keys))

	for i, thunk := range thunks {
		values[i], errors[i] = thunk()
	}

	return values, errors
}

// Thunk represents a deferred computation
type Thunk[V any] func() (V, error)

// LoadThunk returns a thunk for deferred execution
func (dl *DataLoader[K, V]) LoadThunk(ctx context.Context, key K) Thunk[V] {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	// Check cache first
	if result, exists := dl.cache[key]; exists {
		if dl.stats != nil {
			dl.stats.RecordRequest(true)
		}
		return func() (V, error) {
			return result.value, result.err
		}
	}

	// Record cache miss
	if dl.stats != nil {
		dl.stats.RecordRequest(false)
	}

	// Create result channel
	resultCh := make(chan *result[V], 1)

	// Add to waiting list
	if dl.waiting[key] == nil {
		dl.waiting[key] = []chan *result[V]{}
		dl.batch = append(dl.batch, key)
	}
	dl.waiting[key] = append(dl.waiting[key], resultCh)

	// Trigger batch execution if needed
	if len(dl.batch) >= dl.maxBatchSize {
		dl.executeImmediately(ctx)
	} else if len(dl.batch) == 1 {
		// Start timer for first item in batch
		go dl.executeAfterTimeout(ctx)
	}

	return func() (V, error) {
		result := <-resultCh
		return result.value, result.err
	}
}

// executeImmediately executes the current batch immediately
func (dl *DataLoader[K, V]) executeImmediately(ctx context.Context) {
	if len(dl.batch) == 0 {
		return
	}

	keys := make([]K, len(dl.batch))
	copy(keys, dl.batch)
	waiting := make(map[K][]chan *result[V])
	for k, v := range dl.waiting {
		waiting[k] = v
	}

	// Clear current batch
	dl.batch = dl.batch[:0]
	dl.waiting = make(map[K][]chan *result[V])

	go func() {
		start := time.Now()
		values, errors := dl.batchFn(ctx, keys)
		duration := time.Since(start)

		// Record batch execution
		if dl.stats != nil {
			dl.stats.RecordBatch(len(keys), duration)
		}

		for i, key := range keys {
			var result *result[V]
			if i < len(values) && i < len(errors) {
				result = &result[V]{
					value: values[i],
					err:   errors[i],
				}
			} else {
				var zero V
				result = &result[V]{
					value: zero,
					err:   fmt.Errorf("missing result for key"),
				}
			}

			// Cache the result
			dl.mu.Lock()
			dl.cache[key] = result
			dl.mu.Unlock()

			// Send to all waiting channels
			for _, ch := range waiting[key] {
				ch <- result
				close(ch)
			}
		}
	}()
}

// executeAfterTimeout executes batch after timeout
func (dl *DataLoader[K, V]) executeAfterTimeout(ctx context.Context) {
	time.Sleep(dl.batchTimeout)

	dl.mu.Lock()
	defer dl.mu.Unlock()

	if len(dl.batch) > 0 {
		dl.executeImmediately(ctx)
	}
}

// Clear clears the cache
func (dl *DataLoader[K, V]) Clear() {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.cache = make(map[K]*result[V])
}

// ClearKey clears a specific key from cache
func (dl *DataLoader[K, V]) ClearKey(key K) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	delete(dl.cache, key)
}

// UserLoader wraps DataLoader for loading users
type UserLoader struct {
	loader *DataLoader[int, *User]
	db     *sql.DB
}

// NewUserLoader creates a new UserLoader
func NewUserLoader(db *sql.DB) *UserLoader {
	return &UserLoader{
		loader: NewDataLoader(batchLoadUsers(db)),
		db:     db,
	}
}

// Load loads a user by ID
func (ul *UserLoader) Load(ctx context.Context, userID int) (*User, error) {
	return ul.loader.Load(ctx, userID)
}

// LoadMany loads multiple users by IDs
func (ul *UserLoader) LoadMany(ctx context.Context, userIDs []int) ([]*User, []error) {
	return ul.loader.LoadMany(ctx, userIDs)
}

// batchLoadUsers loads multiple users in a single query
func batchLoadUsers(db *sql.DB) BatchFunc[int, *User] {
	return func(ctx context.Context, userIDs []int) ([]*User, []error) {
		if len(userIDs) == 0 {
			return []*User{}, []error{}
		}

		// Build placeholder string for SQL IN clause
		placeholders := make([]string, len(userIDs))
		args := make([]interface{}, len(userIDs))
		for i, id := range userIDs {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = id
		}

		query := fmt.Sprintf(`
			SELECT id, name, email, created_at 
			FROM users 
			WHERE id IN (%s)
			ORDER BY id
		`, fmt.Sprintf("%s", placeholders[0]))
		
		if len(placeholders) > 1 {
			query = fmt.Sprintf(`
				SELECT id, name, email, created_at 
				FROM users 
				WHERE id IN (%s)
				ORDER BY id
			`, fmt.Sprintf("%s", fmt.Sprintf("%s", placeholders[0])+","+fmt.Sprintf("%s", placeholders[1:])))
		}

		// Simplified query building
		query = `SELECT id, name, email, created_at FROM users WHERE id = ANY($1) ORDER BY id`
		rows, err := db.QueryContext(ctx, query, userIDs)
		if err != nil {
			errors := make([]error, len(userIDs))
			for i := range errors {
				errors[i] = err
			}
			return make([]*User, len(userIDs)), errors
		}
		defer rows.Close()

		// Create a map to store loaded users
		userMap := make(map[int]*User)
		for rows.Next() {
			var user User
			if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Created); err != nil {
				continue
			}
			userMap[user.ID] = &user
		}

		// Build result arrays maintaining the order of requested IDs
		users := make([]*User, len(userIDs))
		errors := make([]error, len(userIDs))
		for i, id := range userIDs {
			if user, found := userMap[id]; found {
				users[i] = user
				errors[i] = nil
			} else {
				users[i] = nil
				errors[i] = fmt.Errorf("user with ID %d not found", id)
			}
		}

		return users, errors
	}
}

// PostLoader wraps DataLoader for loading posts
type PostLoader struct {
	loader *DataLoader[int, []*Post]
	db     *sql.DB
}

// NewPostLoader creates a new PostLoader
func NewPostLoader(db *sql.DB) *PostLoader {
	return &PostLoader{
		loader: NewDataLoader(batchLoadPostsByUserID(db)),
		db:     db,
	}
}

// LoadByUserID loads posts by user ID
func (pl *PostLoader) LoadByUserID(ctx context.Context, userID int) ([]*Post, error) {
	return pl.loader.Load(ctx, userID)
}

// LoadManyByUserIDs loads posts for multiple user IDs
func (pl *PostLoader) LoadManyByUserIDs(ctx context.Context, userIDs []int) ([][]*Post, []error) {
	return pl.loader.LoadMany(ctx, userIDs)
}

// batchLoadPostsByUserID loads posts for multiple users in a single query
func batchLoadPostsByUserID(db *sql.DB) BatchFunc[int, []*Post] {
	return func(ctx context.Context, userIDs []int) ([][]*Post, []error) {
		if len(userIDs) == 0 {
			return [][]*Post{}, []error{}
		}

		query := `SELECT id, user_id, title, content, created_at FROM posts WHERE user_id = ANY($1) ORDER BY user_id, created_at DESC`
		rows, err := db.QueryContext(ctx, query, userIDs)
		if err != nil {
			errors := make([]error, len(userIDs))
			for i := range errors {
				errors[i] = err
			}
			return make([][]*Post, len(userIDs)), errors
		}
		defer rows.Close()

		// Group posts by user ID
		postsByUserID := make(map[int][]*Post)
		for rows.Next() {
			var post Post
			if err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created); err != nil {
				continue
			}
			postsByUserID[post.UserID] = append(postsByUserID[post.UserID], &post)
		}

		// Build result arrays maintaining the order of requested user IDs
		results := make([][]*Post, len(userIDs))
		errors := make([]error, len(userIDs))
		for i, userID := range userIDs {
			if posts, found := postsByUserID[userID]; found {
				results[i] = posts
			} else {
				results[i] = []*Post{} // Empty slice for users with no posts
			}
			errors[i] = nil
		}

		return results, errors
	}
}

// LoaderStats holds statistics about loader performance
type LoaderStats struct {
	TotalRequests    int
	CacheHits        int
	CacheMisses      int
	BatchCount       int
	AverageBatchSize float64
	TotalLoadTime    time.Duration
}

// StatsCollector collects statistics about DataLoader usage
type StatsCollector struct {
	stats           LoaderStats
	mu              sync.RWMutex
	totalBatchItems int
}

// NewStatsCollector creates a new statistics collector
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{}
}

// RecordRequest records a loader request
func (sc *StatsCollector) RecordRequest(cacheHit bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.stats.TotalRequests++
	if cacheHit {
		sc.stats.CacheHits++
	} else {
		sc.stats.CacheMisses++
	}
}

// RecordBatch records a batch execution
func (sc *StatsCollector) RecordBatch(batchSize int, duration time.Duration) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.stats.BatchCount++
	sc.totalBatchItems += batchSize
	sc.stats.TotalLoadTime += duration

	// Calculate average batch size
	if sc.stats.BatchCount > 0 {
		sc.stats.AverageBatchSize = float64(sc.totalBatchItems) / float64(sc.stats.BatchCount)
	}
}

// GetStats returns current statistics
func (sc *StatsCollector) GetStats() LoaderStats {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.stats
}

// Reset resets all statistics
func (sc *StatsCollector) Reset() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.stats = LoaderStats{}
	sc.totalBatchItems = 0
}

// setupDatabase initializes the database schema
func setupDatabase(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS posts (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			title VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
	`

	_, err := db.Exec(schema)
	return err
}

// seedTestData inserts test data into the database
func seedTestData(db *sql.DB) error {
	// Clear existing data
	if _, err := db.Exec("DELETE FROM posts"); err != nil {
		return err
	}
	if _, err := db.Exec("DELETE FROM users"); err != nil {
		return err
	}

	// Reset sequences
	if _, err := db.Exec("ALTER SEQUENCE users_id_seq RESTART WITH 1"); err != nil {
		return err
	}
	if _, err := db.Exec("ALTER SEQUENCE posts_id_seq RESTART WITH 1"); err != nil {
		return err
	}

	// Insert test users
	users := []struct {
		name  string
		email string
	}{
		{"Alice Johnson", "alice@example.com"},
		{"Bob Smith", "bob@example.com"},
		{"Charlie Brown", "charlie@example.com"},
		{"Diana Prince", "diana@example.com"},
		{"Eve Adams", "eve@example.com"},
	}

	for _, user := range users {
		_, err := db.Exec(
			"INSERT INTO users (name, email) VALUES ($1, $2)",
			user.name, user.email,
		)
		if err != nil {
			return err
		}
	}

	// Insert test posts
	posts := []struct {
		userID  int
		title   string
		content string
	}{
		{1, "Alice's First Post", "Hello, world! This is Alice."},
		{1, "Alice's Second Post", "Another post by Alice."},
		{2, "Bob's Adventure", "Bob went on an adventure today."},
		{2, "Bob's Recipe", "Here's Bob's favorite recipe."},
		{2, "Bob's Thoughts", "Some random thoughts by Bob."},
		{3, "Charlie's Update", "Charlie has some news to share."},
		{4, "Diana's Story", "Diana tells an interesting story."},
		{4, "Diana's Tutorial", "Diana explains how to do something."},
		{4, "Diana's Review", "Diana reviews a book."},
		{4, "Diana's Tips", "Diana shares some useful tips."},
	}

	for _, post := range posts {
		_, err := db.Exec(
			"INSERT INTO posts (user_id, title, content) VALUES ($1, $2, $3)",
			post.userID, post.title, post.content,
		)
		if err != nil {
			return err
		}
	}

	return nil
}