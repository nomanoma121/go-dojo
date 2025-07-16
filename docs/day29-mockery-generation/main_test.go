package main

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Mock implementations for testing (simple manual mocks)

// MockUserRepository implements UserRepository interface
type MockUserRepository struct {
	users   map[int]*User
	nextID  int
	emails  map[string]*User
	calls   map[string]int
	errors  map[string]error
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:  make(map[int]*User),
		nextID: 1,
		emails: make(map[string]*User),
		calls:  make(map[string]int),
		errors: make(map[string]error),
	}
}

func (m *MockUserRepository) CreateUser(user *User) error {
	m.calls["CreateUser"]++
	if err := m.errors["CreateUser"]; err != nil {
		return err
	}
	
	user.ID = m.nextID
	m.nextID++
	m.users[user.ID] = user
	m.emails[user.Email] = user
	return nil
}

func (m *MockUserRepository) GetUser(id int) (*User, error) {
	m.calls["GetUser"]++
	if err := m.errors["GetUser"]; err != nil {
		return nil, err
	}
	
	user, exists := m.users[id]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (m *MockUserRepository) UpdateUser(user *User) error {
	m.calls["UpdateUser"]++
	if err := m.errors["UpdateUser"]; err != nil {
		return err
	}
	
	if _, exists := m.users[user.ID]; !exists {
		return errors.New("user not found")
	}
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepository) DeleteUser(id int) error {
	m.calls["DeleteUser"]++
	if err := m.errors["DeleteUser"]; err != nil {
		return err
	}
	
	if _, exists := m.users[id]; !exists {
		return errors.New("user not found")
	}
	delete(m.users, id)
	return nil
}

func (m *MockUserRepository) ListUsers() ([]*User, error) {
	m.calls["ListUsers"]++
	if err := m.errors["ListUsers"]; err != nil {
		return nil, err
	}
	
	var users []*User
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
}

func (m *MockUserRepository) GetUserByEmail(email string) (*User, error) {
	m.calls["GetUserByEmail"]++
	if err := m.errors["GetUserByEmail"]; err != nil {
		return nil, err
	}
	
	user, exists := m.emails[email]
	if !exists {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (m *MockUserRepository) SetError(method string, err error) {
	m.errors[method] = err
}

func (m *MockUserRepository) GetCallCount(method string) int {
	return m.calls[method]
}

// MockEmailService implements EmailService interface
type MockEmailService struct {
	calls  map[string]int
	errors map[string]error
	emails []string
}

func NewMockEmailService() *MockEmailService {
	return &MockEmailService{
		calls:  make(map[string]int),
		errors: make(map[string]error),
		emails: make([]string, 0),
	}
}

func (m *MockEmailService) SendEmail(to, subject, body string) error {
	m.calls["SendEmail"]++
	m.emails = append(m.emails, to)
	return m.errors["SendEmail"]
}

func (m *MockEmailService) SendWelcomeEmail(user *User) error {
	m.calls["SendWelcomeEmail"]++
	m.emails = append(m.emails, user.Email)
	return m.errors["SendWelcomeEmail"]
}

func (m *MockEmailService) SendPasswordResetEmail(user *User, resetToken string) error {
	m.calls["SendPasswordResetEmail"]++
	m.emails = append(m.emails, user.Email)
	return m.errors["SendPasswordResetEmail"]
}

func (m *MockEmailService) SendNotificationEmail(user *User, notification *Notification) error {
	m.calls["SendNotificationEmail"]++
	m.emails = append(m.emails, user.Email)
	return m.errors["SendNotificationEmail"]
}

func (m *MockEmailService) SetError(method string, err error) {
	m.errors[method] = err
}

func (m *MockEmailService) GetCallCount(method string) int {
	return m.calls[method]
}

// MockNotificationRepository implements NotificationRepository interface
type MockNotificationRepository struct {
	notifications map[int]*Notification
	userNotifs    map[int][]*Notification
	nextID        int
	calls         map[string]int
	errors        map[string]error
}

func NewMockNotificationRepository() *MockNotificationRepository {
	return &MockNotificationRepository{
		notifications: make(map[int]*Notification),
		userNotifs:    make(map[int][]*Notification),
		nextID:        1,
		calls:         make(map[string]int),
		errors:        make(map[string]error),
	}
}

func (m *MockNotificationRepository) SaveNotification(notification *Notification) error {
	m.calls["SaveNotification"]++
	if err := m.errors["SaveNotification"]; err != nil {
		return err
	}
	
	notification.ID = m.nextID
	m.nextID++
	m.notifications[notification.ID] = notification
	m.userNotifs[notification.UserID] = append(m.userNotifs[notification.UserID], notification)
	return nil
}

func (m *MockNotificationRepository) GetNotificationsByUser(userID int) ([]*Notification, error) {
	m.calls["GetNotificationsByUser"]++
	if err := m.errors["GetNotificationsByUser"]; err != nil {
		return nil, err
	}
	
	return m.userNotifs[userID], nil
}

func (m *MockNotificationRepository) MarkAsRead(notificationID int) error {
	m.calls["MarkAsRead"]++
	return m.errors["MarkAsRead"]
}

func (m *MockNotificationRepository) SetError(method string, err error) {
	m.errors[method] = err
}

func (m *MockNotificationRepository) GetCallCount(method string) int {
	return m.calls[method]
}

// MockHTTPClient implements HTTPClient interface
type MockHTTPClient struct {
	responses map[string]*http.Response
	errors    map[string]error
	calls     map[string]int
}

func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make(map[string]*http.Response),
		errors:    make(map[string]error),
		calls:     make(map[string]int),
	}
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	m.calls["Get"]++
	if err := m.errors["Get"]; err != nil {
		return nil, err
	}
	
	if resp, exists := m.responses[url]; exists {
		return resp, nil
	}
	
	// Default response
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":1,"name":"Test User","email":"test@example.com","age":25}`)),
	}, nil
}

func (m *MockHTTPClient) Post(url string, body io.Reader) (*http.Response, error) {
	m.calls["Post"]++
	if err := m.errors["Post"]; err != nil {
		return nil, err
	}
	
	return &http.Response{StatusCode: http.StatusCreated}, nil
}

func (m *MockHTTPClient) Put(url string, body io.Reader) (*http.Response, error) {
	m.calls["Put"]++
	if err := m.errors["Put"]; err != nil {
		return nil, err
	}
	
	return &http.Response{StatusCode: http.StatusOK}, nil
}

func (m *MockHTTPClient) Delete(url string) (*http.Response, error) {
	m.calls["Delete"]++
	if err := m.errors["Delete"]; err != nil {
		return nil, err
	}
	
	return &http.Response{StatusCode: http.StatusNoContent}, nil
}

func (m *MockHTTPClient) SetError(method string, err error) {
	m.errors[method] = err
}

func (m *MockHTTPClient) GetCallCount(method string) int {
	return m.calls[method]
}

// Test functions

func TestUserService_CreateUser(t *testing.T) {
	tests := []struct {
		name          string
		user          *User
		emailError    error
		notifError    error
		expectError   bool
		expectCalls   map[string]int
	}{
		{
			name: "successful user creation",
			user: &User{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   30,
			},
			expectError: false,
			expectCalls: map[string]int{
				"CreateUser":         1,
				"GetUserByEmail":     1,
				"SendWelcomeEmail":   1,
				"SaveNotification":   1,
			},
		},
		{
			name: "invalid user data",
			user: &User{
				Name:  "",
				Email: "invalid",
				Age:   0,
			},
			expectError: true,
			expectCalls: map[string]int{
				"CreateUser":     0,
				"GetUserByEmail": 0,
			},
		},
		{
			name: "email already exists",
			user: &User{
				Name:  "John Doe",
				Email: "existing@example.com",
				Age:   30,
			},
			expectError: true,
			expectCalls: map[string]int{
				"CreateUser":     0,
				"GetUserByEmail": 1,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			userRepo := NewMockUserRepository()
			emailService := NewMockEmailService()
			smsService := &MockSMSService{}
			notificationRepo := NewMockNotificationRepository()
			
			// Setup existing user for email conflict test
			if tt.user != nil && tt.user.Email == "existing@example.com" {
				existingUser := &User{
					ID:    1,
					Name:  "Existing User",
					Email: "existing@example.com",
					Age:   25,
				}
				userRepo.CreateUser(existingUser)
				// Reset call count after setup
				userRepo.calls["CreateUser"] = 0
			}
			
			// Setup errors
			if tt.emailError != nil {
				emailService.SetError("SendWelcomeEmail", tt.emailError)
			}
			if tt.notifError != nil {
				notificationRepo.SetError("SaveNotification", tt.notifError)
			}
			
			// Create service
			service := NewUserService(userRepo, emailService, smsService, notificationRepo)
			
			// Execute
			err := service.CreateUser(tt.user)
			
			// Verify
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			
			// Verify call counts
			for method, expectedCount := range tt.expectCalls {
				switch method {
				case "CreateUser", "GetUserByEmail":
					if actualCount := userRepo.GetCallCount(method); actualCount != expectedCount {
						t.Errorf("Expected %d calls to %s, got %d", expectedCount, method, actualCount)
					}
				case "SendWelcomeEmail":
					if actualCount := emailService.GetCallCount(method); actualCount != expectedCount {
						t.Errorf("Expected %d calls to %s, got %d", expectedCount, method, actualCount)
					}
				case "SaveNotification":
					if actualCount := notificationRepo.GetCallCount(method); actualCount != expectedCount {
						t.Errorf("Expected %d calls to %s, got %d", expectedCount, method, actualCount)
					}
				}
			}
		})
	}
}

func TestUserService_GetUser(t *testing.T) {
	userRepo := NewMockUserRepository()
	emailService := NewMockEmailService()
	smsService := &MockSMSService{}
	notificationRepo := NewMockNotificationRepository()
	
	service := NewUserService(userRepo, emailService, smsService, notificationRepo)
	
	// Create test user
	testUser := &User{
		Name:  "Test User",
		Email: "test@example.com",
		Age:   25,
	}
	userRepo.CreateUser(testUser)
	
	// Test successful get
	user, err := service.GetUser(1)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if user.Name != "Test User" {
		t.Errorf("Expected name 'Test User', got: %s", user.Name)
	}
	
	// Test user not found
	_, err = service.GetUser(999)
	if err == nil {
		t.Error("Expected error for non-existent user")
	}
}

func TestAPIService_GetUserData(t *testing.T) {
	httpClient := NewMockHTTPClient()
	service := NewAPIService(httpClient, "https://api.example.com")
	
	// Test successful API call
	user, err := service.GetUserData(1)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if user.Name != "Test User" {
		t.Errorf("Expected name 'Test User', got: %s", user.Name)
	}
	
	// Test API error
	httpClient.SetError("Get", errors.New("network error"))
	_, err = service.GetUserData(1)
	if err == nil {
		t.Error("Expected error for network failure")
	}
}

func TestValidateUser(t *testing.T) {
	tests := []struct {
		name        string
		user        *User
		expectError bool
	}{
		{
			name: "valid user",
			user: &User{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   30,
			},
			expectError: false,
		},
		{
			name:        "nil user",
			user:        nil,
			expectError: true,
		},
		{
			name: "empty name",
			user: &User{
				Name:  "",
				Email: "john@example.com",
				Age:   30,
			},
			expectError: true,
		},
		{
			name: "invalid email",
			user: &User{
				Name:  "John Doe",
				Email: "invalid-email",
				Age:   30,
			},
			expectError: true,
		},
		{
			name: "invalid age",
			user: &User{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   0,
			},
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUser(tt.user)
			
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected bool
	}{
		{"user@example.com", true},
		{"user.name@example.com", true},
		{"user+tag@example.com", true},
		{"invalid-email", false},
		{"@example.com", false},
		{"user@", false},
		{"", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := ValidateEmail(tt.email)
			if result != tt.expected {
				t.Errorf("ValidateEmail(%q) = %v, expected %v", tt.email, result, tt.expected)
			}
		})
	}
}

func TestGenerateResetToken(t *testing.T) {
	token := GenerateResetToken()
	
	if len(token) != 32 {
		t.Errorf("Expected token length 32, got %d", len(token))
	}
	
	// Test uniqueness
	token2 := GenerateResetToken()
	if token == token2 {
		t.Error("Expected different tokens, got same token")
	}
}

func TestFormatNotificationMessage(t *testing.T) {
	user := &User{
		ID:    1,
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}
	
	template := "Welcome {{.Name}}! Your email is {{.Email}}"
	expected := "Welcome John Doe! Your email is john@example.com"
	
	result := FormatNotificationMessage(template, user)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// MockSMSService implements SMSService interface
type MockSMSService struct {
	calls  map[string]int
	errors map[string]error
}

func (m *MockSMSService) SendSMS(phoneNumber, message string) error {
	if m.calls == nil {
		m.calls = make(map[string]int)
	}
	m.calls["SendSMS"]++
	if m.errors != nil {
		return m.errors["SendSMS"]
	}
	return nil
}

func (m *MockSMSService) SendOTPSMS(phoneNumber, otp string) error {
	if m.calls == nil {
		m.calls = make(map[string]int)
	}
	m.calls["SendOTPSMS"]++
	if m.errors != nil {
		return m.errors["SendOTPSMS"]
	}
	return nil
}

func (m *MockSMSService) SetError(method string, err error) {
	if m.errors == nil {
		m.errors = make(map[string]error)
	}
	m.errors[method] = err
}

func (m *MockSMSService) GetCallCount(method string) int {
	if m.calls == nil {
		return 0
	}
	return m.calls[method]
}

// Benchmark tests
func BenchmarkUserService_CreateUser(b *testing.B) {
	userRepo := NewMockUserRepository()
	emailService := NewMockEmailService()
	smsService := &MockSMSService{}
	notificationRepo := NewMockNotificationRepository()
	
	service := NewUserService(userRepo, emailService, smsService, notificationRepo)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user := &User{
			Name:  "Test User",
			Email: "test@example.com",
			Age:   30,
		}
		service.CreateUser(user)
	}
}

func BenchmarkValidateEmail(b *testing.B) {
	email := "test@example.com"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateEmail(email)
	}
}