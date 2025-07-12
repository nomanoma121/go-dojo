package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// User represents a user entity
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	Posts     []Post    `json:"posts,omitempty"`
}

// Post represents a post entity
type Post struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUserRequest represents user creation request
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CreatePostRequest represents post creation request
type CreatePostRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ErrorResponse represents error response
type ErrorResponse struct {
	Error   string            `json:"error"`
	Details map[string]string `json:"details,omitempty"`
}

// UserRepository handles user data operations
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(user *User) error {
	query := `INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, created_at`
	err := r.db.QueryRow(query, user.Name, user.Email).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return fmt.Errorf("duplicate email: %s", user.Email)
		}
		return err
	}
	return nil
}

// GetByID retrieves user by ID with posts
func (r *UserRepository) GetByID(id int) (*User, error) {
	query := `SELECT id, name, email, created_at FROM users WHERE id = $1`
	user := &User{}
	err := r.db.QueryRow(query, id).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found: %d", id)
		}
		return nil, err
	}
	
	// Get posts
	postQuery := `SELECT id, user_id, title, content, created_at FROM posts WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(postQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.CreatedAt)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	
	user.Posts = posts
	return user, nil
}

// Update updates user information
func (r *UserRepository) Update(user *User) error {
	query := `UPDATE users SET name = $1, email = $2 WHERE id = $3`
	result, err := r.db.Exec(query, user.Name, user.Email, user.ID)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %d", user.ID)
	}
	
	return nil
}

// Delete deletes user and associated posts
func (r *UserRepository) Delete(id int) error {
	// PostgreSQL will cascade delete posts due to foreign key constraint
	query := `DELETE FROM users WHERE id = $1`
	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("user not found: %d", id)
	}
	
	return nil
}

// PostRepository handles post data operations
type PostRepository struct {
	db *sql.DB
}

// NewPostRepository creates a new post repository
func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{db: db}
}

// Create creates a new post
func (r *PostRepository) Create(post *Post) error {
	query := `INSERT INTO posts (user_id, title, content) VALUES ($1, $2, $3) RETURNING id, created_at`
	err := r.db.QueryRow(query, post.UserID, post.Title, post.Content).Scan(&post.ID, &post.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") {
			return fmt.Errorf("user not found: %d", post.UserID)
		}
		return err
	}
	return nil
}

// GetByUserID retrieves posts by user ID
func (r *PostRepository) GetByUserID(userID int) ([]Post, error) {
	query := `SELECT id, user_id, title, content, created_at FROM posts WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.CreatedAt)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	
	return posts, nil
}

// UserService handles user business logic
type UserService struct {
	userRepo *UserRepository
	postRepo *PostRepository
}

// NewUserService creates a new user service
func NewUserService(userRepo *UserRepository, postRepo *PostRepository) *UserService {
	return &UserService{
		userRepo: userRepo,
		postRepo: postRepo,
	}
}

// CreateUser creates a new user with validation
func (s *UserService) CreateUser(req *CreateUserRequest) (*User, error) {
	// Validate input
	if errors := validateUser(req); len(errors) > 0 {
		var errorMsgs []string
		for field, msg := range errors {
			errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", field, msg))
		}
		return nil, fmt.Errorf("validation failed: %s", strings.Join(errorMsgs, ", "))
	}
	
	user := &User{
		Name:  req.Name,
		Email: req.Email,
	}
	
	err := s.userRepo.Create(user)
	if err != nil {
		return nil, err
	}
	
	return user, nil
}

// GetUser retrieves user with posts
func (s *UserService) GetUser(id int) (*User, error) {
	return s.userRepo.GetByID(id)
}

// UpdateUser updates user information
func (s *UserService) UpdateUser(id int, req *CreateUserRequest) (*User, error) {
	// Validate input
	if errors := validateUser(req); len(errors) > 0 {
		var errorMsgs []string
		for field, msg := range errors {
			errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", field, msg))
		}
		return nil, fmt.Errorf("validation failed: %s", strings.Join(errorMsgs, ", "))
	}
	
	user := &User{
		ID:    id,
		Name:  req.Name,
		Email: req.Email,
	}
	
	err := s.userRepo.Update(user)
	if err != nil {
		return nil, err
	}
	
	return s.userRepo.GetByID(id)
}

// DeleteUser deletes user
func (s *UserService) DeleteUser(id int) error {
	return s.userRepo.Delete(id)
}

// CreateUserWithPosts creates user with initial posts in transaction
func (s *UserService) CreateUserWithPosts(user *User, posts []Post) error {
	// Validate user
	req := &CreateUserRequest{Name: user.Name, Email: user.Email}
	if errors := validateUser(req); len(errors) > 0 {
		return fmt.Errorf("user validation failed")
	}
	
	// Begin transaction
	tx, err := s.userRepo.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// Create user
	userQuery := `INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, created_at`
	err = tx.QueryRow(userQuery, user.Name, user.Email).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return err
	}
	
	// Create posts
	for i := range posts {
		postQuery := `INSERT INTO posts (user_id, title, content) VALUES ($1, $2, $3) RETURNING id, created_at`
		err = tx.QueryRow(postQuery, user.ID, posts[i].Title, posts[i].Content).Scan(&posts[i].ID, &posts[i].CreatedAt)
		if err != nil {
			return err
		}
		posts[i].UserID = user.ID
	}
	
	// Commit transaction
	return tx.Commit()
}

// UserHandler handles HTTP requests for users
type UserHandler struct {
	service *UserService
}

// NewUserHandler creates a new user handler
func NewUserHandler(service *UserService) *UserHandler {
	return &UserHandler{service: service}
}

// CreateUser handles POST /api/users
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", nil)
		return
	}
	
	user, err := h.service.CreateUser(&req)
	if err != nil {
		if strings.Contains(err.Error(), "validation failed") {
			details := validateUser(&req)
			writeError(w, http.StatusBadRequest, "validation failed", details)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user", nil)
		return
	}
	
	writeJSON(w, http.StatusCreated, user)
}

// GetUser handles GET /api/users/{id}
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID", nil)
		return
	}
	
	user, err := h.service.GetUser(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "user not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get user", nil)
		return
	}
	
	writeJSON(w, http.StatusOK, user)
}

// UpdateUser handles PUT /api/users/{id}
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID", nil)
		return
	}
	
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", nil)
		return
	}
	
	user, err := h.service.UpdateUser(id, &req)
	if err != nil {
		if strings.Contains(err.Error(), "validation failed") {
			details := validateUser(&req)
			writeError(w, http.StatusBadRequest, "validation failed", details)
			return
		}
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "user not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update user", nil)
		return
	}
	
	writeJSON(w, http.StatusOK, user)
}

// DeleteUser handles DELETE /api/users/{id}
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID", nil)
		return
	}
	
	err = h.service.DeleteUser(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "user not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete user", nil)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// PostHandler handles HTTP requests for posts
type PostHandler struct {
	postRepo *PostRepository
}

// NewPostHandler creates a new post handler
func NewPostHandler(postRepo *PostRepository) *PostHandler {
	return &PostHandler{postRepo: postRepo}
}

// CreatePost handles POST /api/users/{id}/posts
func (h *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	userID, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID", nil)
		return
	}
	
	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", nil)
		return
	}
	
	post := &Post{
		UserID:  userID,
		Title:   req.Title,
		Content: req.Content,
	}
	
	err = h.postRepo.Create(post)
	if err != nil {
		if strings.Contains(err.Error(), "user not found") {
			writeError(w, http.StatusNotFound, "user not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create post", nil)
		return
	}
	
	writeJSON(w, http.StatusCreated, post)
}

// Validation helpers
func validateUser(req *CreateUserRequest) map[string]string {
	errors := make(map[string]string)
	
	if strings.TrimSpace(req.Name) == "" {
		errors["name"] = "name is required"
	}
	
	if strings.TrimSpace(req.Email) == "" {
		errors["email"] = "email is required"
	} else if !validateEmail(req.Email) {
		errors["email"] = "invalid email format"
	}
	
	return errors
}

func validateEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// HTTP helpers
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, details map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	errResp := ErrorResponse{
		Error:   message,
		Details: details,
	}
	json.NewEncoder(w).Encode(errResp)
}

func extractIDFromPath(path string) (int, error) {
	// Extract ID from paths like "/api/users/123" or "/api/users/123/posts"
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "users" && i+1 < len(parts) {
			return strconv.Atoi(parts[i+1])
		}
	}
	return 0, fmt.Errorf("no ID found in path")
}

// Database helpers
func InitSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(100) UNIQUE NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE TABLE IF NOT EXISTS posts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
		title VARCHAR(200) NOT NULL,
		content TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	
	_, err := db.Exec(schema)
	return err
}

func SeedTestData(db *sql.DB) error {
	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	// Insert sample users
	users := []struct {
		name, email string
	}{
		{"Alice", "alice@example.com"},
		{"Bob", "bob@example.com"},
		{"Charlie", "charlie@example.com"},
	}
	
	var userIDs []int
	for _, user := range users {
		var id int
		err = tx.QueryRow(
			"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
			user.name, user.email).Scan(&id)
		if err != nil {
			return err
		}
		userIDs = append(userIDs, id)
	}
	
	// Insert sample posts
	posts := []struct {
		userIndex int
		title     string
		content   string
	}{
		{0, "Alice's First Post", "Hello from Alice!"},
		{0, "Alice's Second Post", "Another post by Alice"},
		{1, "Bob's Post", "Bob's thoughts"},
		{2, "Charlie's Introduction", "Hi, I'm Charlie"},
	}
	
	for _, post := range posts {
		_, err = tx.Exec(
			"INSERT INTO posts (user_id, title, content) VALUES ($1, $2, $3)",
			userIDs[post.userIndex], post.title, post.content)
		if err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

// Server setup
func SetupServer(db *sql.DB) http.Handler {
	userRepo := NewUserRepository(db)
	postRepo := NewPostRepository(db)
	userService := NewUserService(userRepo, postRepo)
	userHandler := NewUserHandler(userService)
	postHandler := NewPostHandler(postRepo)
	
	mux := http.NewServeMux()
	
	// User routes
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			userHandler.CreateUser(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	
	mux.HandleFunc("/api/users/", func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a posts endpoint
		if strings.HasSuffix(r.URL.Path, "/posts") && r.Method == "POST" {
			postHandler.CreatePost(w, r)
			return
		}
		
		// User endpoints
		switch r.Method {
		case "GET":
			userHandler.GetUser(w, r)
		case "PUT":
			userHandler.UpdateUser(w, r)
		case "DELETE":
			userHandler.DeleteUser(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	
	return mux
}

func main() {
	// Example usage - not used in tests
	db, err := sql.Open("postgres", "postgres://testuser:testpass@localhost/testdb?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	if err := InitSchema(db); err != nil {
		log.Fatal(err)
	}
	
	handler := SetupServer(db)
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}