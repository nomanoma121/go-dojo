package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserStore(t *testing.T) {
	store := NewUserStore()
	
	// Test CreateUser
	user := &User{Name: "John Doe", Email: "john@example.com"}
	createdUser := store.CreateUser(user)
	
	if createdUser.ID == 0 {
		t.Error("Expected user ID to be assigned")
	}
	if createdUser.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", createdUser.Name)
	}
	if createdUser.Email != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got '%s'", createdUser.Email)
	}
	
	// Test GetUser
	retrievedUser, exists := store.GetUser(createdUser.ID)
	if !exists {
		t.Error("Expected user to exist")
	}
	if retrievedUser.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", retrievedUser.Name)
	}
	
	// Test GetAllUsers
	users := store.GetAllUsers()
	if len(users) != 1 {
		t.Errorf("Expected 1 user, got %d", len(users))
	}
	
	// Test UpdateUser
	updatedUser := &User{Name: "Jane Doe", Email: "jane@example.com"}
	result, exists := store.UpdateUser(createdUser.ID, updatedUser)
	if !exists {
		t.Error("Expected user to exist for update")
	}
	if result.Name != "Jane Doe" {
		t.Errorf("Expected updated name 'Jane Doe', got '%s'", result.Name)
	}
	
	// Test DeleteUser
	deleted := store.DeleteUser(createdUser.ID)
	if !deleted {
		t.Error("Expected user to be deleted")
	}
	
	// Verify user is deleted
	_, exists = store.GetUser(createdUser.ID)
	if exists {
		t.Error("Expected user to not exist after deletion")
	}
}

func TestUserHandler(t *testing.T) {
	handler := NewUserHandler()
	
	// Test cases for different HTTP operations
	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		expectedStatus int
		setup          func(*UserHandler) int // Returns created user ID if applicable
	}{
		{
			name:           "GET /users - empty list",
			method:         "GET",
			path:           "/users",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST /users - create user",
			method:         "POST",
			path:           "/users",
			body:           `{"name":"John Doe","email":"john@example.com"}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "POST /users - invalid JSON",
			method:         "POST",
			path:           "/users",
			body:           `{"name":"John Doe","email":}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST /users - missing name",
			method:         "POST",
			path:           "/users",
			body:           `{"email":"john@example.com"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST /users - missing email",
			method:         "POST",
			path:           "/users",
			body:           `{"name":"John Doe"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GET /users/{id} - user not found",
			method:         "GET",
			path:           "/users/999",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "GET /users/{id} - invalid ID",
			method:         "GET",
			path:           "/users/abc",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "PUT /users/{id} - user not found",
			method:         "PUT",
			path:           "/users/999",
			body:           `{"name":"Jane Doe","email":"jane@example.com"}`,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "DELETE /users/{id} - user not found",
			method:         "DELETE",
			path:           "/users/999",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "PATCH /users - method not allowed",
			method:         "PATCH",
			path:           "/users",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var userID int
			if tt.setup != nil {
				userID = tt.setup(handler)
			}
			
			// Replace {id} in path with actual user ID
			path := tt.path
			if userID > 0 {
				path = "/users/" + string(rune(userID+'0'))
			}
			
			var req *http.Request
			var err error
			
			if tt.body != "" {
				req, err = http.NewRequest(tt.method, path, bytes.NewBufferString(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, err = http.NewRequest(tt.method, path, nil)
			}
			
			if err != nil {
				t.Fatal(err)
			}
			
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, status)
			}
		})
	}
}

func TestUserHandlerIntegration(t *testing.T) {
	handler := NewUserHandler()
	
	// Create a user
	userJSON := `{"name":"John Doe","email":"john@example.com"}`
	req, _ := http.NewRequest("POST", "/users", bytes.NewBufferString(userJSON))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, rr.Code)
	}
	
	var createdUser User
	if err := json.NewDecoder(rr.Body).Decode(&createdUser); err != nil {
		t.Fatal("Failed to decode created user")
	}
	
	// Get the created user
	req, _ = http.NewRequest("GET", "/users/1", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	
	var retrievedUser User
	if err := json.NewDecoder(rr.Body).Decode(&retrievedUser); err != nil {
		t.Fatal("Failed to decode retrieved user")
	}
	
	if retrievedUser.ID != createdUser.ID {
		t.Errorf("Expected ID %d, got %d", createdUser.ID, retrievedUser.ID)
	}
	if retrievedUser.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", retrievedUser.Name)
	}
	
	// Update the user
	updateJSON := `{"name":"Jane Doe","email":"jane@example.com"}`
	req, _ = http.NewRequest("PUT", "/users/1", bytes.NewBufferString(updateJSON))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	
	var updatedUser User
	if err := json.NewDecoder(rr.Body).Decode(&updatedUser); err != nil {
		t.Fatal("Failed to decode updated user")
	}
	
	if updatedUser.Name != "Jane Doe" {
		t.Errorf("Expected name 'Jane Doe', got '%s'", updatedUser.Name)
	}
	
	// Get all users
	req, _ = http.NewRequest("GET", "/users", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	
	var users []User
	if err := json.NewDecoder(rr.Body).Decode(&users); err != nil {
		t.Fatal("Failed to decode users list")
	}
	
	if len(users) != 1 {
		t.Errorf("Expected 1 user, got %d", len(users))
	}
	
	// Delete the user
	req, _ = http.NewRequest("DELETE", "/users/1", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusNoContent {
		t.Fatalf("Expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
	
	// Verify user is deleted
	req, _ = http.NewRequest("GET", "/users/1", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusNotFound {
		t.Fatalf("Expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}

// Benchmark tests
func BenchmarkUserStore_CreateUser(b *testing.B) {
	store := NewUserStore()
	user := &User{Name: "Benchmark User", Email: "bench@example.com"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testUser := *user // Copy the user
		store.CreateUser(&testUser)
	}
}

func BenchmarkUserStore_GetUser(b *testing.B) {
	store := NewUserStore()
	user := &User{Name: "Benchmark User", Email: "bench@example.com"}
	createdUser := store.CreateUser(user)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetUser(createdUser.ID)
	}
}

func BenchmarkUserHandler_CreateUser(b *testing.B) {
	handler := NewUserHandler()
	userJSON := `{"name":"Benchmark User","email":"bench@example.com"}`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/users", bytes.NewBufferString(userJSON))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}