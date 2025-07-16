package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// User represents a user in the system
type User struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Age         int       `json:"age"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description,omitempty"`
}

// ValidationError represents validation errors
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, ", ")
}

// SearchQuery represents search parameters
type SearchQuery struct {
	Name     string   `json:"name,omitempty"`
	Email    string   `json:"email,omitempty"`
	Role     string   `json:"role,omitempty"`
	MinAge   int      `json:"min_age,omitempty"`
	MaxAge   int      `json:"max_age,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
}

// ErrorResponse represents API error response
type ErrorResponse struct {
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// DataProcessor handles data processing operations
type DataProcessor struct {
	algorithms map[string]SortAlgorithm
}

// SortAlgorithm defines sorting algorithm interface
type SortAlgorithm func([]int)

// UserRepository manages user data
type UserRepository struct {
	users  map[int]*User
	nextID int
}

// NewUserRepository creates a new user repository
func NewUserRepository() *UserRepository {
	return &UserRepository{
		users:  make(map[int]*User),
		nextID: 1,
	}
}

// Create creates a new user
func (r *UserRepository) Create(user User) (*User, error) {
	if err := ValidateUser(user); err != nil {
		return nil, err
	}
	
	user.ID = r.nextID
	r.nextID++
	user.CreatedAt = time.Now()
	
	r.users[user.ID] = &user
	return &user, nil
}

// GetByID retrieves user by ID
func (r *UserRepository) GetByID(id int) (*User, error) {
	user, exists := r.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

// Update updates existing user
func (r *UserRepository) Update(id int, user User) (*User, error) {
	if _, exists := r.users[id]; !exists {
		return nil, fmt.Errorf("user not found")
	}
	
	if err := ValidateUser(user); err != nil {
		return nil, err
	}
	
	user.ID = id
	user.CreatedAt = r.users[id].CreatedAt
	r.users[id] = &user
	return &user, nil
}

// Delete deletes user by ID
func (r *UserRepository) Delete(id int) error {
	if _, exists := r.users[id]; !exists {
		return fmt.Errorf("user not found")
	}
	delete(r.users, id)
	return nil
}

// Search searches users based on query
func (r *UserRepository) Search(query SearchQuery) []*User {
	var result []*User
	
	for _, user := range r.users {
		if query.Name != "" && !strings.Contains(strings.ToLower(user.Name), strings.ToLower(query.Name)) {
			continue
		}
		if query.Email != "" && !strings.Contains(strings.ToLower(user.Email), strings.ToLower(query.Email)) {
			continue
		}
		if query.Role != "" && user.Role != query.Role {
			continue
		}
		if query.MinAge > 0 && user.Age < query.MinAge {
			continue
		}
		if query.MaxAge > 0 && user.Age > query.MaxAge {
			continue
		}
		if len(query.Keywords) > 0 {
			hasAllKeywords := true
			for _, keyword := range query.Keywords {
				if !strings.Contains(strings.ToLower(user.Description), strings.ToLower(keyword)) {
					hasAllKeywords = false
					break
				}
			}
			if !hasAllKeywords {
				continue
			}
		}
		result = append(result, user)
	}
	
	return result
}

// GetAll returns all users
func (r *UserRepository) GetAll() []*User {
	result := make([]*User, 0, len(r.users))
	for _, user := range r.users {
		result = append(result, user)
	}
	return result
}

// ValidateUser validates user data
func ValidateUser(user User) error {
	var errors ValidationErrors
	
	if strings.TrimSpace(user.Name) == "" {
		errors = append(errors, ValidationError{Field: "name", Message: "name is required"})
	}
	if !validateEmail(user.Email) {
		errors = append(errors, ValidationError{Field: "email", Message: "invalid email format"})
	}
	if user.Age <= 0 || user.Age > 150 {
		errors = append(errors, ValidationError{Field: "age", Message: "age must be between 1 and 150"})
	}
	if user.Role != "admin" && user.Role != "user" {
		errors = append(errors, ValidationError{Field: "role", Message: "role must be 'admin' or 'user'"})
	}
	
	if len(errors) > 0 {
		return errors
	}
	return nil
}

// NewDataProcessor creates a new data processor
func NewDataProcessor() *DataProcessor {
	return &DataProcessor{
		algorithms: map[string]SortAlgorithm{
			"BubbleSort": BubbleSort,
			"QuickSort":  QuickSort,
			"MergeSort":  MergeSort,
		},
	}
}

// Sort sorts data using specified algorithm
func (dp *DataProcessor) Sort(data []int, algorithm string) error {
	alg, exists := dp.algorithms[algorithm]
	if !exists {
		return fmt.Errorf("algorithm not found: %s", algorithm)
	}
	alg(data)
	return nil
}

// GetAvailableAlgorithms returns list of available algorithms
func (dp *DataProcessor) GetAvailableAlgorithms() []string {
	var algorithms []string
	for name := range dp.algorithms {
		algorithms = append(algorithms, name)
	}
	sort.Strings(algorithms)
	return algorithms
}

// Transform applies transformation to data
func (dp *DataProcessor) Transform(data []int, operation string) ([]int, error) {
	result := make([]int, len(data))
	
	switch operation {
	case "double":
		for i, v := range data {
			result[i] = v * 2
		}
	case "square":
		for i, v := range data {
			result[i] = v * v
		}
	case "abs":
		for i, v := range data {
			if v < 0 {
				result[i] = -v
			} else {
				result[i] = v
			}
		}
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
	
	return result, nil
}

// Filter filters data based on predicate
func (dp *DataProcessor) Filter(data []int, predicate string) ([]int, error) {
	var result []int
	
	for _, v := range data {
		switch predicate {
		case "positive":
			if v > 0 {
				result = append(result, v)
			}
		case "negative":
			if v < 0 {
				result = append(result, v)
			}
		case "even":
			if v%2 == 0 {
				result = append(result, v)
			}
		case "odd":
			if v%2 != 0 {
				result = append(result, v)
			}
		default:
			return nil, fmt.Errorf("unknown predicate: %s", predicate)
		}
	}
	
	return result, nil
}

// Calculate performs statistical calculations
func (dp *DataProcessor) Calculate(data []int, operation string) (float64, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("empty data")
	}
	
	switch operation {
	case "mean":
		sum := 0
		for _, v := range data {
			sum += v
		}
		return float64(sum) / float64(len(data)), nil
	case "median":
		sorted := make([]int, len(data))
		copy(sorted, data)
		sort.Ints(sorted)
		n := len(sorted)
		if n%2 == 0 {
			return float64(sorted[n/2-1]+sorted[n/2]) / 2.0, nil
		}
		return float64(sorted[n/2]), nil
	case "mode":
		freq := make(map[int]int)
		for _, v := range data {
			freq[v]++
		}
		maxFreq := 0
		mode := 0
		for v, f := range freq {
			if f > maxFreq {
				maxFreq = f
				mode = v
			}
		}
		return float64(mode), nil
	case "stddev":
		mean := 0.0
		for _, v := range data {
			mean += float64(v)
		}
		mean /= float64(len(data))
		
		variance := 0.0
		for _, v := range data {
			variance += (float64(v) - mean) * (float64(v) - mean)
		}
		variance /= float64(len(data))
		
		return math.Sqrt(variance), nil
	default:
		return 0, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Sorting algorithms
func BubbleSort(data []int) {
	n := len(data)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if data[j] > data[j+1] {
				data[j], data[j+1] = data[j+1], data[j]
			}
		}
	}
}

func QuickSort(data []int) {
	if len(data) <= 1 {
		return
	}
	quickSortHelper(data, 0, len(data)-1)
}

func quickSortHelper(data []int, low, high int) {
	if low < high {
		pi := partition(data, low, high)
		quickSortHelper(data, low, pi-1)
		quickSortHelper(data, pi+1, high)
	}
}

func partition(data []int, low, high int) int {
	pivot := data[high]
	i := low - 1
	
	for j := low; j < high; j++ {
		if data[j] <= pivot {
			i++
			data[i], data[j] = data[j], data[i]
		}
	}
	data[i+1], data[high] = data[high], data[i+1]
	return i + 1
}

func MergeSort(data []int) {
	if len(data) <= 1 {
		return
	}
	mergeSortHelper(data, 0, len(data)-1)
}

func mergeSortHelper(data []int, left, right int) {
	if left < right {
		mid := (left + right) / 2
		mergeSortHelper(data, left, mid)
		mergeSortHelper(data, mid+1, right)
		merge(data, left, mid, right)
	}
}

func merge(data []int, left, mid, right int) {
	leftArr := make([]int, mid-left+1)
	rightArr := make([]int, right-mid)
	
	copy(leftArr, data[left:mid+1])
	copy(rightArr, data[mid+1:right+1])
	
	i, j, k := 0, 0, left
	
	for i < len(leftArr) && j < len(rightArr) {
		if leftArr[i] <= rightArr[j] {
			data[k] = leftArr[i]
			i++
		} else {
			data[k] = rightArr[j]
			j++
		}
		k++
	}
	
	for i < len(leftArr) {
		data[k] = leftArr[i]
		i++
		k++
	}
	
	for j < len(rightArr) {
		data[k] = rightArr[j]
		j++
		k++
	}
}

// UserMatcher provides flexible user matching
type UserMatcher struct {
	ID       *int
	Name     *string
	Email    *string
	Role     *string
	MinAge   *int
	MaxAge   *int
	Contains []string
}

// Matches checks if user matches criteria
func (m UserMatcher) Matches(user User) bool {
	if m.ID != nil && user.ID != *m.ID {
		return false
	}
	if m.Name != nil && user.Name != *m.Name {
		return false
	}
	if m.Email != nil && user.Email != *m.Email {
		return false
	}
	if m.Role != nil && user.Role != *m.Role {
		return false
	}
	if m.MinAge != nil && user.Age < *m.MinAge {
		return false
	}
	if m.MaxAge != nil && user.Age > *m.MaxAge {
		return false
	}
	if len(m.Contains) > 0 {
		for _, keyword := range m.Contains {
			if !strings.Contains(strings.ToLower(user.Description), strings.ToLower(keyword)) {
				return false
			}
		}
	}
	return true
}

// UserBuilder provides fluent interface for user creation
type UserBuilder struct {
	user User
}

// NewUserBuilder creates a new user builder
func NewUserBuilder() *UserBuilder {
	return &UserBuilder{
		user: User{
			Name:      "Default User",
			Email:     "default@example.com",
			Age:       25,
			Role:      "user",
			CreatedAt: time.Now(),
		},
	}
}

// WithName sets user name
func (b *UserBuilder) WithName(name string) *UserBuilder {
	b.user.Name = name
	return b
}

// WithEmail sets user email
func (b *UserBuilder) WithEmail(email string) *UserBuilder {
	b.user.Email = email
	return b
}

// WithAge sets user age
func (b *UserBuilder) WithAge(age int) *UserBuilder {
	b.user.Age = age
	return b
}

// WithRole sets user role
func (b *UserBuilder) WithRole(role string) *UserBuilder {
	b.user.Role = role
	return b
}

// WithDescription sets user description
func (b *UserBuilder) WithDescription(desc string) *UserBuilder {
	b.user.Description = desc
	return b
}

// Build creates the user
func (b *UserBuilder) Build() User {
	return b.user
}

// HTTP API handlers
type UserAPI struct {
	repo *UserRepository
}

// NewUserAPI creates a new user API
func NewUserAPI(repo *UserRepository) *UserAPI {
	return &UserAPI{repo: repo}
}

// CreateUser handles POST /users
func (api *UserAPI) CreateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", nil)
		return
	}
	
	created, err := api.repo.Create(user)
	if err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			details := make(map[string]string)
			for _, validationErr := range validationErrors {
				details[validationErr.Field] = validationErr.Message
			}
			writeError(w, http.StatusBadRequest, "validation error", details)
			return
		}
		if validationErr, ok := err.(*ValidationError); ok {
			writeError(w, http.StatusBadRequest, "validation error", map[string]string{
				validationErr.Field: validationErr.Message,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	
	writeJSON(w, http.StatusCreated, created)
}

// GetUser handles GET /users/{id}
func (api *UserAPI) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}
	
	user, err := api.repo.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found", nil)
		return
	}
	
	writeJSON(w, http.StatusOK, user)
}

// UpdateUser handles PUT /users/{id}
func (api *UserAPI) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}
	
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON", nil)
		return
	}
	
	updated, err := api.repo.Update(id, user)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "User not found", nil)
			return
		}
		if validationErrors, ok := err.(ValidationErrors); ok {
			details := make(map[string]string)
			for _, validationErr := range validationErrors {
				details[validationErr.Field] = validationErr.Message
			}
			writeError(w, http.StatusBadRequest, "validation error", details)
			return
		}
		if validationErr, ok := err.(*ValidationError); ok {
			writeError(w, http.StatusBadRequest, "validation error", map[string]string{
				validationErr.Field: validationErr.Message,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "Internal server error", nil)
		return
	}
	
	writeJSON(w, http.StatusOK, updated)
}

// DeleteUser handles DELETE /users/{id}
func (api *UserAPI) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := extractIDFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID", nil)
		return
	}
	
	if err := api.repo.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, "User not found", nil)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// SearchUsers handles GET /users/search
func (api *UserAPI) SearchUsers(w http.ResponseWriter, r *http.Request) {
	query := SearchQuery{
		Name:   r.URL.Query().Get("name"),
		Email:  r.URL.Query().Get("email"),
		Role:   r.URL.Query().Get("role"),
	}
	
	if minAge := r.URL.Query().Get("min_age"); minAge != "" {
		if age, err := strconv.Atoi(minAge); err == nil {
			query.MinAge = age
		}
	}
	
	if maxAge := r.URL.Query().Get("max_age"); maxAge != "" {
		if age, err := strconv.Atoi(maxAge); err == nil {
			query.MaxAge = age
		}
	}
	
	if keywords := r.URL.Query().Get("keywords"); keywords != "" {
		query.Keywords = strings.Split(keywords, ",")
	}
	
	users := api.repo.Search(query)
	writeJSON(w, http.StatusOK, users)
}

// Helper functions
func validateEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func extractIDFromPath(path string) (int, error) {
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return 0, fmt.Errorf("invalid path")
	}
	
	idStr := parts[len(parts)-1]
	if idStr == "search" {
		return 0, fmt.Errorf("invalid path")
	}
	
	return strconv.Atoi(idStr)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, details map[string]string) {
	errorResponse := ErrorResponse{
		Message: message,
		Details: details,
	}
	writeJSON(w, status, errorResponse)
}

// Utility functions for pointers (used in tests)
func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func main() {
	// Example usage
	repo := NewUserRepository()
	api := NewUserAPI(repo)
	
	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			api.CreateUser(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	
	http.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			if strings.Contains(r.URL.Path, "/search") {
				api.SearchUsers(w, r)
			} else {
				api.GetUser(w, r)
			}
		case "PUT":
			api.UpdateUser(w, r)
		case "DELETE":
			api.DeleteUser(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	
	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}