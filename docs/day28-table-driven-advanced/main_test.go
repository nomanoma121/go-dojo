package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUser(t *testing.T) {
	tests := []struct {
		name        string
		user        User
		wantErr     bool
		errContains []string
	}{
		{
			name: "valid user",
			user: User{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   25,
				Role:  "user",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			user: User{
				Name:  "",
				Email: "john@example.com",
				Age:   25,
				Role:  "user",
			},
			wantErr:     true,
			errContains: []string{"name"},
		},
		{
			name: "invalid email",
			user: User{
				Name:  "John Doe",
				Email: "invalid-email",
				Age:   25,
				Role:  "user",
			},
			wantErr:     true,
			errContains: []string{"email"},
		},
		{
			name: "negative age",
			user: User{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   -1,
				Role:  "user",
			},
			wantErr:     true,
			errContains: []string{"age"},
		},
		{
			name: "too old",
			user: User{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   200,
				Role:  "user",
			},
			wantErr:     true,
			errContains: []string{"age"},
		},
		{
			name: "invalid role",
			user: User{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   25,
				Role:  "invalid",
			},
			wantErr:     true,
			errContains: []string{"role"},
		},
		{
			name: "multiple validation errors",
			user: User{
				Name:  "",
				Email: "invalid",
				Age:   -1,
				Role:  "invalid",
			},
			wantErr:     true,
			errContains: []string{"name", "email", "age", "role"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUser(tt.user)
			
			if tt.wantErr {
				assert.Error(t, err)
				for _, contains := range tt.errContains {
					assert.Contains(t, err.Error(), contains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserRepository(t *testing.T) {
	repo := NewUserRepository()
	
	t.Run("Create", func(t *testing.T) {
		tests := []struct {
			name    string
			user    User
			wantErr bool
		}{
			{
				name: "valid user",
				user: User{Name: "John", Email: "john@example.com", Age: 25, Role: "user"},
			},
			{
				name:    "invalid user",
				user:    User{Name: "", Email: "invalid", Age: -1},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				user, err := repo.Create(tt.user)
				
				if tt.wantErr {
					assert.Error(t, err)
					assert.Nil(t, user)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, user)
					assert.NotZero(t, user.ID)
					assert.NotZero(t, user.CreatedAt)
				}
			})
		}
	})
	
	t.Run("GetByID", func(t *testing.T) {
		// Create test user first
		testUser := User{Name: "Test", Email: "test@example.com", Age: 30, Role: "user"}
		created, err := repo.Create(testUser)
		require.NoError(t, err)
		
		tests := []struct {
			name    string
			id      int
			wantErr bool
		}{
			{
				name: "existing user",
				id:   created.ID,
			},
			{
				name:    "non-existent user",
				id:      999,
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				user, err := repo.GetByID(tt.id)
				
				if tt.wantErr {
					assert.Error(t, err)
					assert.Nil(t, user)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, user)
					assert.Equal(t, tt.id, user.ID)
				}
			})
		}
	})
	
	t.Run("Update", func(t *testing.T) {
		// Create test user first
		testUser := User{Name: "Original", Email: "original@example.com", Age: 25, Role: "user"}
		created, err := repo.Create(testUser)
		require.NoError(t, err)
		
		tests := []struct {
			name    string
			id      int
			updates User
			wantErr bool
		}{
			{
				name: "valid update",
				id:   created.ID,
				updates: User{
					Name:  "Updated",
					Email: "updated@example.com",
					Age:   30,
					Role:  "admin",
				},
			},
			{
				name:    "non-existent user",
				id:      999,
				updates: User{Name: "Test", Email: "test@example.com"},
				wantErr: true,
			},
			{
				name: "invalid update",
				id:   created.ID,
				updates: User{
					Name:  "",
					Email: "invalid",
					Age:   -1,
				},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				user, err := repo.Update(tt.id, tt.updates)
				
				if tt.wantErr {
					assert.Error(t, err)
					assert.Nil(t, user)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, user)
					assert.Equal(t, tt.updates.Name, user.Name)
					assert.Equal(t, tt.updates.Email, user.Email)
				}
			})
		}
	})
	
	t.Run("Delete", func(t *testing.T) {
		// Create test user first
		testUser := User{Name: "ToDelete", Email: "delete@example.com", Age: 25, Role: "user"}
		created, err := repo.Create(testUser)
		require.NoError(t, err)
		
		tests := []struct {
			name    string
			id      int
			wantErr bool
		}{
			{
				name: "existing user",
				id:   created.ID,
			},
			{
				name:    "non-existent user",
				id:      999,
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := repo.Delete(tt.id)
				
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					
					// Verify user is deleted
					_, err := repo.GetByID(tt.id)
					assert.Error(t, err)
				}
			})
		}
	})
}

func TestUserRepositorySearch(t *testing.T) {
	repo := NewUserRepository()
	
	// Create test users
	testUsers := []User{
		{Name: "Alice Johnson", Email: "alice@example.com", Age: 25, Role: "user", Description: "Software developer working with Go"},
		{Name: "Bob Smith", Email: "bob@example.com", Age: 30, Role: "admin", Description: "System administrator with Linux expertise"},
		{Name: "Charlie Brown", Email: "charlie@company.com", Age: 35, Role: "user", Description: "Frontend developer specializing in React"},
		{Name: "Diana Prince", Email: "diana@example.com", Age: 28, Role: "user", Description: "Full-stack developer with Go and JavaScript"},
	}
	
	for _, user := range testUsers {
		_, err := repo.Create(user)
		require.NoError(t, err)
	}
	
	tests := []struct {
		name          string
		query         SearchQuery
		expectedCount int
		matcher       UserMatcher
	}{
		{
			name:          "search by name",
			query:         SearchQuery{Name: "Alice"},
			expectedCount: 1,
			matcher:       UserMatcher{Name: stringPtr("Alice Johnson")},
		},
		{
			name:          "search by email domain",
			query:         SearchQuery{Email: "@example.com"},
			expectedCount: 3,
		},
		{
			name:          "search by role",
			query:         SearchQuery{Role: "admin"},
			expectedCount: 1,
			matcher:       UserMatcher{Role: stringPtr("admin")},
		},
		{
			name:          "search by age range",
			query:         SearchQuery{MinAge: 28, MaxAge: 32},
			expectedCount: 2,
			matcher:       UserMatcher{MinAge: intPtr(28), MaxAge: intPtr(32)},
		},
		{
			name:          "search by keywords",
			query:         SearchQuery{Keywords: []string{"developer", "Go"}},
			expectedCount: 2,
			matcher:       UserMatcher{Contains: []string{"developer", "Go"}},
		},
		{
			name:          "search with multiple criteria",
			query:         SearchQuery{Role: "user", MinAge: 30},
			expectedCount: 1,
			matcher:       UserMatcher{Role: stringPtr("user"), MinAge: intPtr(30)},
		},
		{
			name:          "search with no results",
			query:         SearchQuery{Name: "NonExistent"},
			expectedCount: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := repo.Search(tt.query)
			
			assert.Len(t, results, tt.expectedCount)
			
			// If we have a matcher, verify all results match
			if tt.expectedCount > 0 && (tt.matcher.Name != nil || tt.matcher.Role != nil || 
				tt.matcher.MinAge != nil || tt.matcher.MaxAge != nil || len(tt.matcher.Contains) > 0) {
				for _, user := range results {
					assert.True(t, tt.matcher.Matches(*user), 
						"User %+v does not match criteria %+v", user, tt.matcher)
				}
			}
		})
	}
}

func TestDataProcessor(t *testing.T) {
	dp := NewDataProcessor()
	
	t.Run("Sort", func(t *testing.T) {
		algorithms := dp.GetAvailableAlgorithms()
		assert.NotEmpty(t, algorithms)
		assert.Contains(t, algorithms, "BubbleSort")
		assert.Contains(t, algorithms, "QuickSort")
		assert.Contains(t, algorithms, "MergeSort")
		
		testData := []int{64, 34, 25, 12, 22, 11, 90}
		expected := make([]int, len(testData))
		copy(expected, testData)
		sort.Ints(expected)
		
		for _, algorithm := range algorithms {
			t.Run(algorithm, func(t *testing.T) {
				data := make([]int, len(testData))
				copy(data, testData)
				
				err := dp.Sort(data, algorithm)
				assert.NoError(t, err)
				assert.Equal(t, expected, data)
			})
		}
		
		// Test invalid algorithm
		data := make([]int, len(testData))
		copy(data, testData)
		err := dp.Sort(data, "InvalidSort")
		assert.Error(t, err)
	})
	
	t.Run("Transform", func(t *testing.T) {
		tests := []struct {
			name      string
			input     []int
			operation string
			expected  []int
			wantErr   bool
		}{
			{
				name:      "double",
				input:     []int{1, 2, 3, 4},
				operation: "double",
				expected:  []int{2, 4, 6, 8},
			},
			{
				name:      "square",
				input:     []int{2, 3, 4},
				operation: "square",
				expected:  []int{4, 9, 16},
			},
			{
				name:      "abs",
				input:     []int{-1, 2, -3, 4},
				operation: "abs",
				expected:  []int{1, 2, 3, 4},
			},
			{
				name:      "invalid operation",
				input:     []int{1, 2, 3},
				operation: "invalid",
				wantErr:   true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := dp.Transform(tt.input, tt.operation)
				
				if tt.wantErr {
					assert.Error(t, err)
					assert.Nil(t, result)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expected, result)
				}
			})
		}
	})
	
	t.Run("Filter", func(t *testing.T) {
		tests := []struct {
			name      string
			input     []int
			predicate string
			expected  []int
			wantErr   bool
		}{
			{
				name:      "positive",
				input:     []int{-2, -1, 0, 1, 2, 3},
				predicate: "positive",
				expected:  []int{1, 2, 3},
			},
			{
				name:      "negative",
				input:     []int{-2, -1, 0, 1, 2},
				predicate: "negative",
				expected:  []int{-2, -1},
			},
			{
				name:      "even",
				input:     []int{1, 2, 3, 4, 5, 6},
				predicate: "even",
				expected:  []int{2, 4, 6},
			},
			{
				name:      "odd",
				input:     []int{1, 2, 3, 4, 5, 6},
				predicate: "odd",
				expected:  []int{1, 3, 5},
			},
			{
				name:      "invalid predicate",
				input:     []int{1, 2, 3},
				predicate: "invalid",
				wantErr:   true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := dp.Filter(tt.input, tt.predicate)
				
				if tt.wantErr {
					assert.Error(t, err)
					assert.Nil(t, result)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expected, result)
				}
			})
		}
	})
	
	t.Run("Calculate", func(t *testing.T) {
		tests := []struct {
			name      string
			input     []int
			operation string
			expected  float64
			wantErr   bool
		}{
			{
				name:      "mean",
				input:     []int{1, 2, 3, 4, 5},
				operation: "mean",
				expected:  3.0,
			},
			{
				name:      "median odd count",
				input:     []int{1, 3, 2, 5, 4},
				operation: "median",
				expected:  3.0,
			},
			{
				name:      "median even count",
				input:     []int{1, 2, 3, 4},
				operation: "median",
				expected:  2.5,
			},
			{
				name:      "mode",
				input:     []int{1, 2, 2, 3, 3, 3},
				operation: "mode",
				expected:  3.0,
			},
			{
				name:      "stddev",
				input:     []int{2, 4, 4, 4, 5, 5, 7, 9},
				operation: "stddev",
				expected:  2.0,
			},
			{
				name:      "invalid operation",
				input:     []int{1, 2, 3},
				operation: "invalid",
				wantErr:   true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := dp.Calculate(tt.input, tt.operation)
				
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.InDelta(t, tt.expected, result, 0.01)
				}
			})
		}
	})
}

func TestUserMatcher(t *testing.T) {
	users := []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com", Role: "user", Age: 25, Description: "Go developer"},
		{ID: 2, Name: "Bob", Email: "bob@company.com", Role: "admin", Age: 30, Description: "System administrator"},
		{ID: 3, Name: "Charlie", Email: "charlie@example.com", Role: "user", Age: 35, Description: "Frontend developer"},
	}
	
	tests := []struct {
		name     string
		matcher  UserMatcher
		expected []int // User IDs that should match
	}{
		{
			name:     "match by ID",
			matcher:  UserMatcher{ID: intPtr(1)},
			expected: []int{1},
		},
		{
			name:     "match by name",
			matcher:  UserMatcher{Name: stringPtr("Bob")},
			expected: []int{2},
		},
		{
			name:     "match by role",
			matcher:  UserMatcher{Role: stringPtr("user")},
			expected: []int{1, 3},
		},
		{
			name:     "match by age range",
			matcher:  UserMatcher{MinAge: intPtr(30), MaxAge: intPtr(40)},
			expected: []int{2, 3},
		},
		{
			name:     "match by keywords",
			matcher:  UserMatcher{Contains: []string{"developer"}},
			expected: []int{1, 3},
		},
		{
			name:     "match by multiple criteria",
			matcher:  UserMatcher{Role: stringPtr("user"), MinAge: intPtr(30)},
			expected: []int{3},
		},
		{
			name:     "no matches",
			matcher:  UserMatcher{Name: stringPtr("NonExistent")},
			expected: []int{},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actualIDs []int
			for _, user := range users {
				if tt.matcher.Matches(user) {
					actualIDs = append(actualIDs, user.ID)
				}
			}
			
			assert.Equal(t, tt.expected, actualIDs)
		})
	}
}

func TestUserBuilder(t *testing.T) {
	tests := []struct {
		name     string
		build    func() User
		expected User
	}{
		{
			name: "default user",
			build: func() User {
				return NewUserBuilder().Build()
			},
			expected: User{
				Name:  "Default User",
				Email: "default@example.com",
				Age:   25,
				Role:  "user",
			},
		},
		{
			name: "custom user",
			build: func() User {
				return NewUserBuilder().
					WithName("John Doe").
					WithEmail("john@example.com").
					WithAge(30).
					WithRole("admin").
					WithDescription("Senior developer").
					Build()
			},
			expected: User{
				Name:        "John Doe",
				Email:       "john@example.com",
				Age:         30,
				Role:        "admin",
				Description: "Senior developer",
			},
		},
		{
			name: "partial customization",
			build: func() User {
				return NewUserBuilder().
					WithName("Jane Smith").
					WithAge(28).
					Build()
			},
			expected: User{
				Name:  "Jane Smith",
				Email: "default@example.com",
				Age:   28,
				Role:  "user",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := tt.build()
			
			assert.Equal(t, tt.expected.Name, user.Name)
			assert.Equal(t, tt.expected.Email, user.Email)
			assert.Equal(t, tt.expected.Age, user.Age)
			assert.Equal(t, tt.expected.Role, user.Role)
			assert.Equal(t, tt.expected.Description, user.Description)
		})
	}
}

func TestUserAPI(t *testing.T) {
	repo := NewUserRepository()
	api := NewUserAPI(repo)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/users":
			api.CreateUser(w, r)
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/users/"):
			api.GetUser(w, r)
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/users/"):
			api.UpdateUser(w, r)
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/users/"):
			api.DeleteUser(w, r)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/search"):
			api.SearchUsers(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	
	t.Run("CreateUser", func(t *testing.T) {
		tests := []struct {
			name           string
			body           interface{}
			expectedStatus int
			bodyAssertion  func(t *testing.T, body []byte)
		}{
			{
				name: "valid user",
				body: User{Name: "John Doe", Email: "john@example.com", Age: 25, Role: "user"},
				expectedStatus: http.StatusCreated,
				bodyAssertion: func(t *testing.T, body []byte) {
					var user User
					err := json.Unmarshal(body, &user)
					require.NoError(t, err)
					assert.NotZero(t, user.ID)
					assert.Equal(t, "John Doe", user.Name)
				},
			},
			{
				name: "invalid user",
				body: User{Name: "", Email: "invalid", Age: -1},
				expectedStatus: http.StatusBadRequest,
				bodyAssertion: func(t *testing.T, body []byte) {
					var errResp ErrorResponse
					err := json.Unmarshal(body, &errResp)
					require.NoError(t, err)
					assert.Contains(t, errResp.Message, "validation")
				},
			},
			{
				name: "invalid JSON",
				body: "invalid json",
				expectedStatus: http.StatusBadRequest,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var body io.Reader
				if str, ok := tt.body.(string); ok {
					body = strings.NewReader(str)
				} else {
					jsonBody, _ := json.Marshal(tt.body)
					body = bytes.NewReader(jsonBody)
				}
				
				resp, err := http.Post(server.URL+"/users", "application/json", body)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				assert.Equal(t, tt.expectedStatus, resp.StatusCode)
				
				if tt.bodyAssertion != nil {
					bodyBytes, err := io.ReadAll(resp.Body)
					require.NoError(t, err)
					tt.bodyAssertion(t, bodyBytes)
				}
			})
		}
	})
	
	t.Run("GetUser", func(t *testing.T) {
		// Create test user first
		user := User{Name: "Test User", Email: "test@example.com", Age: 30, Role: "user"}
		created, err := repo.Create(user)
		require.NoError(t, err)
		
		tests := []struct {
			name           string
			userID         string
			expectedStatus int
			bodyAssertion  func(t *testing.T, body []byte)
		}{
			{
				name:           "existing user",
				userID:         fmt.Sprintf("%d", created.ID),
				expectedStatus: http.StatusOK,
				bodyAssertion: func(t *testing.T, body []byte) {
					var user User
					err := json.Unmarshal(body, &user)
					require.NoError(t, err)
					assert.Equal(t, created.ID, user.ID)
					assert.Equal(t, "Test User", user.Name)
				},
			},
			{
				name:           "non-existent user",
				userID:         "999",
				expectedStatus: http.StatusNotFound,
			},
			{
				name:           "invalid user ID",
				userID:         "invalid",
				expectedStatus: http.StatusBadRequest,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := http.Get(server.URL + "/users/" + tt.userID)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				assert.Equal(t, tt.expectedStatus, resp.StatusCode)
				
				if tt.bodyAssertion != nil {
					bodyBytes, err := io.ReadAll(resp.Body)
					require.NoError(t, err)
					tt.bodyAssertion(t, bodyBytes)
				}
			})
		}
	})
}

func TestParallelUserOperations(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"CreateUsers", testCreateMultipleUsersParallel},
		{"ReadUsers", testReadMultipleUsersParallel},
		{"UpdateUsers", testUpdateMultipleUsersParallel},
	}
	
	for _, tt := range tests {
		tt := tt // Capture loop variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}

func testCreateMultipleUsersParallel(t *testing.T) {
	repo := NewUserRepository()
	
	// Create multiple users in parallel
	const numUsers = 10
	results := make(chan *User, numUsers)
	errors := make(chan error, numUsers)
	
	for i := 0; i < numUsers; i++ {
		go func(i int) {
			user := User{
				Name:  fmt.Sprintf("User %d", i),
				Email: fmt.Sprintf("user%d@example.com", i),
				Age:   25 + i,
				Role:  "user",
			}
			
			created, err := repo.Create(user)
			if err != nil {
				errors <- err
			} else {
				results <- created
			}
		}(i)
	}
	
	// Collect results
	var users []*User
	for i := 0; i < numUsers; i++ {
		select {
		case user := <-results:
			users = append(users, user)
		case err := <-errors:
			t.Errorf("Error creating user: %v", err)
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for user creation")
		}
	}
	
	assert.Len(t, users, numUsers)
	
	// Verify all users have unique IDs
	ids := make(map[int]bool)
	for _, user := range users {
		assert.False(t, ids[user.ID], "Duplicate ID found: %d", user.ID)
		ids[user.ID] = true
	}
}

func testReadMultipleUsersParallel(t *testing.T) {
	repo := NewUserRepository()
	
	// Create test users first
	const numUsers = 5
	var createdUsers []*User
	for i := 0; i < numUsers; i++ {
		user := User{
			Name:  fmt.Sprintf("Read User %d", i),
			Email: fmt.Sprintf("read%d@example.com", i),
			Age:   20 + i,
			Role:  "user",
		}
		created, err := repo.Create(user)
		require.NoError(t, err)
		createdUsers = append(createdUsers, created)
	}
	
	// Read users in parallel
	results := make(chan *User, numUsers)
	errors := make(chan error, numUsers)
	
	for _, createdUser := range createdUsers {
		go func(userID int) {
			user, err := repo.GetByID(userID)
			if err != nil {
				errors <- err
			} else {
				results <- user
			}
		}(createdUser.ID)
	}
	
	// Collect results
	var retrievedUsers []*User
	for i := 0; i < numUsers; i++ {
		select {
		case user := <-results:
			retrievedUsers = append(retrievedUsers, user)
		case err := <-errors:
			t.Errorf("Error reading user: %v", err)
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for user retrieval")
		}
	}
	
	assert.Len(t, retrievedUsers, numUsers)
}

func testUpdateMultipleUsersParallel(t *testing.T) {
	repo := NewUserRepository()
	
	// Create test users first
	const numUsers = 5
	var createdUsers []*User
	for i := 0; i < numUsers; i++ {
		user := User{
			Name:  fmt.Sprintf("Update User %d", i),
			Email: fmt.Sprintf("update%d@example.com", i),
			Age:   25,
			Role:  "user",
		}
		created, err := repo.Create(user)
		require.NoError(t, err)
		createdUsers = append(createdUsers, created)
	}
	
	// Update users in parallel
	results := make(chan *User, numUsers)
	errors := make(chan error, numUsers)
	
	for i, createdUser := range createdUsers {
		go func(userID, index int) {
			updates := User{
				Name:  fmt.Sprintf("Updated User %d", index),
				Email: fmt.Sprintf("updated%d@example.com", index),
				Age:   30 + index,
				Role:  "admin",
			}
			
			user, err := repo.Update(userID, updates)
			if err != nil {
				errors <- err
			} else {
				results <- user
			}
		}(createdUser.ID, i)
	}
	
	// Collect results
	var updatedUsers []*User
	for i := 0; i < numUsers; i++ {
		select {
		case user := <-results:
			updatedUsers = append(updatedUsers, user)
		case err := <-errors:
			t.Errorf("Error updating user: %v", err)
		case <-time.After(time.Second):
			t.Fatal("Timeout waiting for user update")
		}
	}
	
	assert.Len(t, updatedUsers, numUsers)
	
	// Verify updates
	for _, user := range updatedUsers {
		assert.Equal(t, "admin", user.Role)
		assert.Contains(t, user.Name, "Updated User")
	}
}

// Helper functions (using functions from main_solution.go)

// Custom assertions for complex data
func assertUsersEqual(t *testing.T, expected, actual []*User) {
	require.Len(t, actual, len(expected))
	
	for i, expectedUser := range expected {
		actualUser := actual[i]
		assert.Equal(t, expectedUser.Name, actualUser.Name)
		assert.Equal(t, expectedUser.Email, actualUser.Email)
		assert.Equal(t, expectedUser.Age, actualUser.Age)
		assert.Equal(t, expectedUser.Role, actualUser.Role)
	}
}

func assertUserMatches(t *testing.T, pattern User, actual *User) {
	if pattern.Name != "" {
		assert.Equal(t, pattern.Name, actual.Name)
	}
	if pattern.Email != "" {
		assert.Equal(t, pattern.Email, actual.Email)
	}
	if pattern.Age != 0 {
		assert.Equal(t, pattern.Age, actual.Age)
	}
	if pattern.Role != "" {
		assert.Equal(t, pattern.Role, actual.Role)
	}
}

// Benchmark integration
func TestDataProcessorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	dp := NewDataProcessor()
	
	// Test with larger dataset
	size := 1000
	data := make([]int, size)
	for i := range data {
		data[i] = size - i // Reverse order for worst case
	}
	
	algorithms := dp.GetAvailableAlgorithms()
	for _, algorithm := range algorithms {
		t.Run(fmt.Sprintf("Sort%d_%s", size, algorithm), func(t *testing.T) {
			testData := make([]int, len(data))
			copy(testData, data)
			
			start := time.Now()
			err := dp.Sort(testData, algorithm)
			duration := time.Since(start)
			
			assert.NoError(t, err)
			assert.True(t, sort.IntsAreSorted(testData))
			
			t.Logf("Algorithm %s took %v for %d elements", algorithm, duration, size)
		})
	}
}