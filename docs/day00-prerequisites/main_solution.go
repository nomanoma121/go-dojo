package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserStore struct {
	mu     sync.RWMutex
	users  map[int]*User
	nextID int
}

func NewUserStore() *UserStore {
	return &UserStore{
		users:  make(map[int]*User),
		nextID: 1,
	}
}

func (s *UserStore) CreateUser(user *User) *User {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	user.ID = s.nextID
	s.nextID++
	s.users[user.ID] = user
	return user
}

func (s *UserStore) GetUser(id int) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	user, exists := s.users[id]
	return user, exists
}

func (s *UserStore) GetAllUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	users := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users
}

func (s *UserStore) UpdateUser(id int, user *User) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.users[id]; !exists {
		return nil, false
	}
	
	user.ID = id
	s.users[id] = user
	return user, true
}

func (s *UserStore) DeleteUser(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, exists := s.users[id]; !exists {
		return false
	}
	
	delete(s.users, id)
	return true
}

type UserHandler struct {
	store *UserStore
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		store: NewUserStore(),
	}
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	path := strings.TrimPrefix(r.URL.Path, "/users")
	
	switch r.Method {
	case http.MethodGet:
		if path == "" || path == "/" {
			h.handleGetUsers(w, r)
		} else {
			h.handleGetUser(w, r, path)
		}
	case http.MethodPost:
		h.handleCreateUser(w, r)
	case http.MethodPut:
		h.handleUpdateUser(w, r, path)
	case http.MethodDelete:
		h.handleDeleteUser(w, r, path)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *UserHandler) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	users := h.store.GetAllUsers()
	if err := json.NewEncoder(w).Encode(users); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *UserHandler) handleGetUser(w http.ResponseWriter, r *http.Request, path string) {
	id, err := strconv.Atoi(strings.Trim(path, "/"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	
	user, exists := h.store.GetUser(id)
	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	
	if err := json.NewEncoder(w).Encode(user); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *UserHandler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// バリデーション
	if user.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if user.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}
	
	createdUser := h.store.CreateUser(&user)
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdUser); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *UserHandler) handleUpdateUser(w http.ResponseWriter, r *http.Request, path string) {
	id, err := strconv.Atoi(strings.Trim(path, "/"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// バリデーション
	if user.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if user.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}
	
	updatedUser, exists := h.store.UpdateUser(id, &user)
	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	
	if err := json.NewEncoder(w).Encode(updatedUser); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *UserHandler) handleDeleteUser(w http.ResponseWriter, r *http.Request, path string) {
	id, err := strconv.Atoi(strings.Trim(path, "/"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	
	if !h.store.DeleteUser(id) {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	handler := NewUserHandler()
	http.Handle("/users", handler)
	http.Handle("/users/", handler)
	
	fmt.Println("Server starting on :8080")
	fmt.Println("Try the following endpoints:")
	fmt.Println("  GET    /users")
	fmt.Println("  POST   /users")
	fmt.Println("  GET    /users/{id}")
	fmt.Println("  PUT    /users/{id}")
	fmt.Println("  DELETE /users/{id}")
	fmt.Println("")
	fmt.Println("Example commands:")
	fmt.Println(`  curl -X POST http://localhost:8080/users -d '{"name":"John Doe","email":"john@example.com"}' -H "Content-Type: application/json"`)
	fmt.Println(`  curl http://localhost:8080/users`)
	fmt.Println(`  curl http://localhost:8080/users/1`)
	fmt.Println(`  curl -X PUT http://localhost:8080/users/1 -d '{"name":"Jane Doe","email":"jane@example.com"}' -H "Content-Type: application/json"`)
	fmt.Println(`  curl -X DELETE http://localhost:8080/users/1`)
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}