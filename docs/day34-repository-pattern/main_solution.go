package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// User represents a user entity
type User struct {
	ID       int       `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Created  time.Time `json:"created"`
}

// Post represents a post entity
type Post struct {
	ID      int       `json:"id"`
	UserID  int       `json:"user_id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Created time.Time `json:"created"`
}

// UserRepository defines the interface for user data access
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id int) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, limit, offset int) ([]*User, error)
	FindBySpec(ctx context.Context, spec UserSpecification) ([]*User, error)
	WithTx(tx *sql.Tx) UserRepository
}

// PostRepository defines the interface for post data access
type PostRepository interface {
	Create(ctx context.Context, post *Post) error
	GetByID(ctx context.Context, id int) (*Post, error)
	GetByUserID(ctx context.Context, userID int) ([]*Post, error)
	Update(ctx context.Context, post *Post) error
	Delete(ctx context.Context, id int) error
	List(ctx context.Context, limit, offset int) ([]*Post, error)
	WithTx(tx *sql.Tx) PostRepository
}

// PostgreSQLUserRepository implements UserRepository for PostgreSQL
type PostgreSQLUserRepository struct {
	db *sql.DB
	tx *sql.Tx
}

// NewPostgreSQLUserRepository creates a new PostgreSQL user repository
func NewPostgreSQLUserRepository(db *sql.DB) UserRepository {
	return &PostgreSQLUserRepository{
		db: db,
	}
}

// getDB returns the appropriate database connection or transaction
func (r *PostgreSQLUserRepository) getDB() interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
} {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// Create creates a new user
func (r *PostgreSQLUserRepository) Create(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (username, email, created) 
		VALUES ($1, $2, $3) 
		RETURNING id`
	
	user.Created = time.Now()
	err := r.getDB().QueryRowContext(ctx, query, user.Username, user.Email, user.Created).
		Scan(&user.ID)
	return err
}

// GetByID retrieves a user by ID
func (r *PostgreSQLUserRepository) GetByID(ctx context.Context, id int) (*User, error) {
	query := `
		SELECT id, username, email, created 
		FROM users 
		WHERE id = $1`
	
	user := &User{}
	err := r.getDB().QueryRowContext(ctx, query, id).
		Scan(&user.ID, &user.Username, &user.Email, &user.Created)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

// GetByEmail retrieves a user by email
func (r *PostgreSQLUserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, username, email, created 
		FROM users 
		WHERE email = $1`
	
	user := &User{}
	err := r.getDB().QueryRowContext(ctx, query, email).
		Scan(&user.ID, &user.Username, &user.Email, &user.Created)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return user, err
}

// Update updates a user
func (r *PostgreSQLUserRepository) Update(ctx context.Context, user *User) error {
	query := `
		UPDATE users 
		SET username = $1, email = $2 
		WHERE id = $3`
	
	_, err := r.getDB().ExecContext(ctx, query, user.Username, user.Email, user.ID)
	return err
}

// Delete deletes a user by ID
func (r *PostgreSQLUserRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := r.getDB().ExecContext(ctx, query, id)
	return err
}

// List returns a paginated list of users
func (r *PostgreSQLUserRepository) List(ctx context.Context, limit, offset int) ([]*User, error) {
	query := `
		SELECT id, username, email, created 
		FROM users 
		ORDER BY id 
		LIMIT $1 OFFSET $2`
	
	rows, err := r.getDB().QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var users []*User
	for rows.Next() {
		user := &User{}
		err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Created)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	
	return users, rows.Err()
}

// FindBySpec finds users by specification
func (r *PostgreSQLUserRepository) FindBySpec(ctx context.Context, spec UserSpecification) ([]*User, error) {
	whereClause, args := spec.ToSQL()
	query := fmt.Sprintf("SELECT id, username, email, created FROM users WHERE %s", whereClause)
	
	rows, err := r.getDB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var users []*User
	for rows.Next() {
		user := &User{}
		err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Created)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	
	return users, rows.Err()
}

// WithTx returns a repository that uses the provided transaction
func (r *PostgreSQLUserRepository) WithTx(tx *sql.Tx) UserRepository {
	return &PostgreSQLUserRepository{db: r.db, tx: tx}
}

// PostgreSQLPostRepository implements PostRepository for PostgreSQL
type PostgreSQLPostRepository struct {
	db *sql.DB
	tx *sql.Tx
}

// NewPostgreSQLPostRepository creates a new PostgreSQL post repository
func NewPostgreSQLPostRepository(db *sql.DB) PostRepository {
	return &PostgreSQLPostRepository{
		db: db,
	}
}

// getDB returns the appropriate database connection or transaction
func (r *PostgreSQLPostRepository) getDB() interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
} {
	if r.tx != nil {
		return r.tx
	}
	return r.db
}

// Create creates a new post
func (r *PostgreSQLPostRepository) Create(ctx context.Context, post *Post) error {
	query := `
		INSERT INTO posts (user_id, title, content, created) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id`
	
	post.Created = time.Now()
	err := r.getDB().QueryRowContext(ctx, query, post.UserID, post.Title, post.Content, post.Created).
		Scan(&post.ID)
	return err
}

// GetByID retrieves a post by ID
func (r *PostgreSQLPostRepository) GetByID(ctx context.Context, id int) (*Post, error) {
	query := `
		SELECT id, user_id, title, content, created 
		FROM posts 
		WHERE id = $1`
	
	post := &Post{}
	err := r.getDB().QueryRowContext(ctx, query, id).
		Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return post, err
}

// GetByUserID retrieves posts by user ID
func (r *PostgreSQLPostRepository) GetByUserID(ctx context.Context, userID int) ([]*Post, error) {
	query := `
		SELECT id, user_id, title, content, created 
		FROM posts 
		WHERE user_id = $1 
		ORDER BY created DESC`
	
	rows, err := r.getDB().QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var posts []*Post
	for rows.Next() {
		post := &Post{}
		err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	
	return posts, rows.Err()
}

// Update updates a post
func (r *PostgreSQLPostRepository) Update(ctx context.Context, post *Post) error {
	query := `
		UPDATE posts 
		SET title = $1, content = $2 
		WHERE id = $3`
	
	_, err := r.getDB().ExecContext(ctx, query, post.Title, post.Content, post.ID)
	return err
}

// Delete deletes a post by ID
func (r *PostgreSQLPostRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM posts WHERE id = $1`
	_, err := r.getDB().ExecContext(ctx, query, id)
	return err
}

// List returns a paginated list of posts
func (r *PostgreSQLPostRepository) List(ctx context.Context, limit, offset int) ([]*Post, error) {
	query := `
		SELECT id, user_id, title, content, created 
		FROM posts 
		ORDER BY created DESC 
		LIMIT $1 OFFSET $2`
	
	rows, err := r.getDB().QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var posts []*Post
	for rows.Next() {
		post := &Post{}
		err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Created)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	
	return posts, rows.Err()
}

// WithTx returns a repository that uses the provided transaction
func (r *PostgreSQLPostRepository) WithTx(tx *sql.Tx) PostRepository {
	return &PostgreSQLPostRepository{db: r.db, tx: tx}
}

// MockUserRepository implements UserRepository for testing
type MockUserRepository struct {
	users  map[int]*User
	nextID int
	mu     sync.RWMutex
}

// NewMockUserRepository creates a new mock repository
func NewMockUserRepository() UserRepository {
	return &MockUserRepository{
		users:  make(map[int]*User),
		nextID: 1,
	}
}

// Create creates a user in memory
func (m *MockUserRepository) Create(ctx context.Context, user *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	user.ID = m.nextID
	m.nextID++
	user.Created = time.Now()
	
	// Copy user to avoid external modification
	userCopy := *user
	m.users[user.ID] = &userCopy
	return nil
}

// GetByID retrieves a user by ID from memory
func (m *MockUserRepository) GetByID(ctx context.Context, id int) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	user, exists := m.users[id]
	if !exists {
		return nil, nil
	}
	
	// Return a copy to avoid external modification
	userCopy := *user
	return &userCopy, nil
}

// GetByEmail retrieves a user by email from memory
func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, user := range m.users {
		if user.Email == email {
			userCopy := *user
			return &userCopy, nil
		}
	}
	
	return nil, nil
}

// Update updates a user in memory
func (m *MockUserRepository) Update(ctx context.Context, user *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.users[user.ID]; !exists {
		return fmt.Errorf("user with ID %d not found", user.ID)
	}
	
	userCopy := *user
	m.users[user.ID] = &userCopy
	return nil
}

// Delete deletes a user from memory
func (m *MockUserRepository) Delete(ctx context.Context, id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.users[id]; !exists {
		return fmt.Errorf("user with ID %d not found", id)
	}
	
	delete(m.users, id)
	return nil
}

// List returns a paginated list of users from memory
func (m *MockUserRepository) List(ctx context.Context, limit, offset int) ([]*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var users []*User
	count := 0
	
	for _, user := range m.users {
		if count < offset {
			count++
			continue
		}
		if len(users) >= limit {
			break
		}
		
		userCopy := *user
		users = append(users, &userCopy)
		count++
	}
	
	return users, nil
}

// FindBySpec finds users by specification from memory
func (m *MockUserRepository) FindBySpec(ctx context.Context, spec UserSpecification) ([]*User, error) {
	// For mock implementation, we'll use a simple approach
	// In a real implementation, you'd parse the SQL and apply the conditions
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var users []*User
	for _, user := range m.users {
		userCopy := *user
		users = append(users, &userCopy)
	}
	
	return users, nil
}

// WithTx returns the same repository (no transaction support in mock)
func (m *MockUserRepository) WithTx(tx *sql.Tx) UserRepository {
	return m
}

// UserService provides business logic for user operations
type UserService struct {
	userRepo UserRepository
	postRepo PostRepository
	db       *sql.DB
}

// NewUserService creates a new user service
func NewUserService(userRepo UserRepository, postRepo PostRepository, db *sql.DB) *UserService {
	return &UserService{
		userRepo: userRepo,
		postRepo: postRepo,
		db:       db,
	}
}

// CreateUserWithProfile creates a user and their profile in a single transaction
func (s *UserService) CreateUserWithProfile(ctx context.Context, user *User, bio string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Use transaction-aware repository
	txUserRepo := s.userRepo.WithTx(tx)
	
	// Create user
	err = txUserRepo.Create(ctx, user)
	if err != nil {
		return err
	}

	// Create profile (simulated)
	_, err = tx.ExecContext(ctx, 
		"INSERT INTO profiles (user_id, bio) VALUES ($1, $2)", 
		user.ID, bio)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CreateUserWithPost creates a user and their first post in a single transaction
func (s *UserService) CreateUserWithPost(ctx context.Context, user *User, post *Post) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Use transaction-aware repositories
	txUserRepo := s.userRepo.WithTx(tx)
	txPostRepo := s.postRepo.WithTx(tx)
	
	// Create user first
	err = txUserRepo.Create(ctx, user)
	if err != nil {
		return err
	}

	// Set the user ID for the post
	post.UserID = user.ID
	
	// Create post
	err = txPostRepo.Create(ctx, post)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// UnitOfWork manages multiple repositories in a single transaction
type UnitOfWork struct {
	db       *sql.DB
	tx       *sql.Tx
	userRepo UserRepository
	postRepo PostRepository
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(db *sql.DB) *UnitOfWork {
	return &UnitOfWork{
		db:       db,
		userRepo: NewPostgreSQLUserRepository(db),
		postRepo: NewPostgreSQLPostRepository(db),
	}
}

// Begin starts a new transaction
func (uow *UnitOfWork) Begin(ctx context.Context) error {
	tx, err := uow.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	uow.tx = tx
	return nil
}

// Users returns the user repository within the transaction
func (uow *UnitOfWork) Users() UserRepository {
	if uow.tx != nil {
		return uow.userRepo.WithTx(uow.tx)
	}
	return uow.userRepo
}

// Posts returns the post repository within the transaction
func (uow *UnitOfWork) Posts() PostRepository {
	if uow.tx != nil {
		return uow.postRepo.WithTx(uow.tx)
	}
	return uow.postRepo
}

// Commit commits the transaction
func (uow *UnitOfWork) Commit() error {
	if uow.tx == nil {
		return fmt.Errorf("no active transaction")
	}
	err := uow.tx.Commit()
	uow.tx = nil
	return err
}

// Rollback rolls back the transaction
func (uow *UnitOfWork) Rollback() error {
	if uow.tx == nil {
		return nil
	}
	err := uow.tx.Rollback()
	uow.tx = nil
	return err
}

// Specification pattern for complex queries

// UserSpecification defines criteria for querying users
type UserSpecification interface {
	ToSQL() (string, []interface{})
}

// UserByEmailSpec specification for finding users by email
type UserByEmailSpec struct {
	Email string
}

func (s UserByEmailSpec) ToSQL() (string, []interface{}) {
	return "email = $1", []interface{}{s.Email}
}

// UserCreatedAfterSpec specification for finding users created after a date
type UserCreatedAfterSpec struct {
	After time.Time
}

func (s UserCreatedAfterSpec) ToSQL() (string, []interface{}) {
	return "created > $1", []interface{}{s.After}
}

// UsernameLikeSpec specification for finding users by username pattern
type UsernameLikeSpec struct {
	Pattern string
}

func (s UsernameLikeSpec) ToSQL() (string, []interface{}) {
	return "username LIKE $1", []interface{}{s.Pattern}
}

// AndSpec combines specifications with AND
type AndSpec struct {
	Left, Right UserSpecification
}

func (s AndSpec) ToSQL() (string, []interface{}) {
	leftSQL, leftArgs := s.Left.ToSQL()
	rightSQL, rightArgs := s.Right.ToSQL()
	
	// Adjust parameter placeholders for the right side
	adjustedRightSQL := rightSQL
	for i := len(leftArgs); i > 0; i-- {
		placeholder := fmt.Sprintf("$%d", len(leftArgs)+1)
		adjustedRightSQL = strings.Replace(adjustedRightSQL, "$1", placeholder, 1)
	}
	
	sql := fmt.Sprintf("(%s) AND (%s)", leftSQL, adjustedRightSQL)
	args := append(leftArgs, rightArgs...)
	return sql, args
}

// OrSpec combines specifications with OR
type OrSpec struct {
	Left, Right UserSpecification
}

func (s OrSpec) ToSQL() (string, []interface{}) {
	leftSQL, leftArgs := s.Left.ToSQL()
	rightSQL, rightArgs := s.Right.ToSQL()
	
	// Adjust parameter placeholders for the right side
	adjustedRightSQL := rightSQL
	for i := len(leftArgs); i > 0; i-- {
		placeholder := fmt.Sprintf("$%d", len(leftArgs)+1)
		adjustedRightSQL = strings.Replace(adjustedRightSQL, "$1", placeholder, 1)
	}
	
	sql := fmt.Sprintf("(%s) OR (%s)", leftSQL, adjustedRightSQL)
	args := append(leftArgs, rightArgs...)
	return sql, args
}

// Database setup functions

// setupDatabase initializes the database schema
func setupDatabase(db *sql.DB) error {
	schema := `
	DROP TABLE IF EXISTS profiles;
	DROP TABLE IF EXISTS posts;
	DROP TABLE IF EXISTS users;

	CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(100) UNIQUE NOT NULL,
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

	CREATE TABLE profiles (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
		bio TEXT,
		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX idx_users_email ON users(email);
	CREATE INDEX idx_users_username ON users(username);
	CREATE INDEX idx_posts_user_id ON posts(user_id);
	CREATE INDEX idx_posts_created ON posts(created);
	CREATE INDEX idx_profiles_user_id ON profiles(user_id);
	`

	_, err := db.Exec(schema)
	return err
}

func main() {
	fmt.Println("=== Repository Pattern Demo ===")

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

	// Create repositories
	userRepo := NewPostgreSQLUserRepository(db)
	postRepo := NewPostgreSQLPostRepository(db)

	ctx := context.Background()

	// Demo 1: Basic repository usage
	fmt.Println("\n1. Basic Repository Operations:")
	
	user := &User{
		Username: "alice",
		Email:    "alice@example.com",
	}
	
	err = userRepo.Create(ctx, user)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created user: %+v\n", user)

	retrievedUser, err := userRepo.GetByID(ctx, user.ID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Retrieved user: %+v\n", retrievedUser)

	// Demo 2: Service layer with transactions
	fmt.Println("\n2. Service Layer with Transactions:")
	
	userService := NewUserService(userRepo, postRepo, db)
	
	newUser := &User{
		Username: "bob",
		Email:    "bob@example.com",
	}
	
	newPost := &Post{
		Title:   "My First Post",
		Content: "Hello, World!",
	}
	
	err = userService.CreateUserWithPost(ctx, newUser, newPost)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created user with post: User ID=%d, Post ID=%d\n", newUser.ID, newPost.ID)

	// Demo 3: Unit of Work pattern
	fmt.Println("\n3. Unit of Work Pattern:")
	
	uow := NewUnitOfWork(db)
	err = uow.Begin(ctx)
	if err != nil {
		panic(err)
	}

	anotherUser := &User{
		Username: "charlie",
		Email:    "charlie@example.com",
	}
	
	err = uow.Users().Create(ctx, anotherUser)
	if err != nil {
		uow.Rollback()
		panic(err)
	}

	anotherPost := &Post{
		UserID:  anotherUser.ID,
		Title:   "UoW Post",
		Content: "Created with Unit of Work",
	}
	
	err = uow.Posts().Create(ctx, anotherPost)
	if err != nil {
		uow.Rollback()
		panic(err)
	}

	err = uow.Commit()
	if err != nil {
		panic(err)
	}
	fmt.Printf("UoW: Created user and post successfully\n")

	// Demo 4: Specification pattern
	fmt.Println("\n4. Specification Pattern:")
	
	emailSpec := UserByEmailSpec{Email: "alice@example.com"}
	users, err := userRepo.FindBySpec(ctx, emailSpec)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Found %d users with email 'alice@example.com'\n", len(users))

	// Demo 5: Mock repository for testing
	fmt.Println("\n5. Mock Repository:")
	
	mockRepo := NewMockUserRepository()
	testUser := &User{
		Username: "testuser",
		Email:    "test@example.com",
	}
	
	err = mockRepo.Create(ctx, testUser)
	if err != nil {
		panic(err)
	}
	
	retrievedTestUser, err := mockRepo.GetByID(ctx, testUser.ID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Mock repo test: %+v\n", retrievedTestUser)

	fmt.Println("\nRepository pattern demo completed!")
}