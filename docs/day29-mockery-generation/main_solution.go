package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// User represents a user entity
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
}

// PaymentResult represents payment processing result
type PaymentResult struct {
	TransactionID string    `json:"transaction_id"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"`
	ProcessedAt   time.Time `json:"processed_at"`
}

// ProcessResult represents async processing result
type ProcessResult struct {
	Data  string `json:"data"`
	Error error  `json:"error"`
}

// Notification represents a notification
type Notification struct {
	ID      int       `json:"id"`
	UserID  int       `json:"user_id"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
	SentAt  time.Time `json:"sent_at"`
}

// Repository interfaces (to be mocked)

//go:generate mockery --name=UserRepository
type UserRepository interface {
	CreateUser(user *User) error
	GetUser(id int) (*User, error)
	UpdateUser(user *User) error
	DeleteUser(id int) error
	ListUsers() ([]*User, error)
	GetUserByEmail(email string) (*User, error)
}

//go:generate mockery --name=NotificationRepository
type NotificationRepository interface {
	SaveNotification(notification *Notification) error
	GetNotificationsByUser(userID int) ([]*Notification, error)
	MarkAsRead(notificationID int) error
}

// External service interfaces (to be mocked)

//go:generate mockery --name=EmailService
type EmailService interface {
	SendEmail(to, subject, body string) error
	SendWelcomeEmail(user *User) error
	SendPasswordResetEmail(user *User, resetToken string) error
	SendNotificationEmail(user *User, notification *Notification) error
}

//go:generate mockery --name=SMSService
type SMSService interface {
	SendSMS(phoneNumber, message string) error
	SendOTPSMS(phoneNumber, otp string) error
}

//go:generate mockery --name=PaymentProcessor
type PaymentProcessor interface {
	ProcessPayment(amount float64, cardToken string) (*PaymentResult, error)
	RefundPayment(transactionID string) error
	GetPaymentStatus(transactionID string) (string, error)
}

//go:generate mockery --name=HTTPClient
type HTTPClient interface {
	Get(url string) (*http.Response, error)
	Post(url string, body io.Reader) (*http.Response, error)
	Put(url string, body io.Reader) (*http.Response, error)
	Delete(url string) (*http.Response, error)
}

//go:generate mockery --name=AsyncProcessor
type AsyncProcessor interface {
	ProcessAsync(data string, callback func(result string, err error))
	ProcessWithChannel(data string) <-chan ProcessResult
}

// Service implementations

// UserService handles user-related business logic
type UserService struct {
	userRepo         UserRepository
	emailService     EmailService
	smsService       SMSService
	notificationRepo NotificationRepository
}

// NewUserService creates a new user service
func NewUserService(
	userRepo UserRepository,
	emailService EmailService,
	smsService SMSService,
	notificationRepo NotificationRepository,
) *UserService {
	return &UserService{
		userRepo:         userRepo,
		emailService:     emailService,
		smsService:       smsService,
		notificationRepo: notificationRepo,
	}
}

// CreateUser creates a new user with validation and notifications
func (s *UserService) CreateUser(user *User) error {
	// Validate user data
	if err := ValidateUser(user); err != nil {
		return err
	}

	// Check if email already exists
	existing, err := s.userRepo.GetUserByEmail(user.Email)
	if err == nil && existing != nil {
		return errors.New("email already exists")
	}

	// Set creation time
	user.CreatedAt = time.Now()

	// Create user in repository
	if err := s.userRepo.CreateUser(user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Send welcome email
	if err := s.emailService.SendWelcomeEmail(user); err != nil {
		// Log error but don't fail user creation
		fmt.Printf("Failed to send welcome email: %v\n", err)
	}

	// Create welcome notification
	notification := &Notification{
		UserID:  user.ID,
		Type:    "welcome",
		Message: FormatNotificationMessage("Welcome {{.Name}}! Your account has been created.", user),
		SentAt:  time.Now(),
	}

	if err := s.notificationRepo.SaveNotification(notification); err != nil {
		// Log error but don't fail user creation
		fmt.Printf("Failed to save welcome notification: %v\n", err)
	}

	return nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id int) (*User, error) {
	return s.userRepo.GetUser(id)
}

// UpdateUser updates user information
func (s *UserService) UpdateUser(user *User) error {
	// Validate user data
	if err := ValidateUser(user); err != nil {
		return err
	}

	// Check if user exists
	existing, err := s.userRepo.GetUser(user.ID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Check if email changed
	emailChanged := existing.Email != user.Email

	// Update user in repository
	if err := s.userRepo.UpdateUser(user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	// Send update notification if email changed
	if emailChanged {
		notification := &Notification{
			UserID:  user.ID,
			Type:    "email_update",
			Message: fmt.Sprintf("Your email has been updated to %s", user.Email),
			SentAt:  time.Now(),
		}

		if err := s.notificationRepo.SaveNotification(notification); err != nil {
			fmt.Printf("Failed to save email update notification: %v\n", err)
		}
	}

	return nil
}

// DeleteUser deletes a user
func (s *UserService) DeleteUser(id int) error {
	// Check if user exists
	if _, err := s.userRepo.GetUser(id); err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Delete user from repository
	if err := s.userRepo.DeleteUser(id); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Note: In a real implementation, you might want to clean up related notifications
	// For simplicity, we're not implementing that here

	return nil
}

// ListUsers returns all users
func (s *UserService) ListUsers() ([]*User, error) {
	return s.userRepo.ListUsers()
}

// RequestPasswordReset initiates password reset process
func (s *UserService) RequestPasswordReset(email string) error {
	// Find user by email
	user, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		return errors.New("user not found")
	}

	// Generate reset token
	resetToken := GenerateResetToken()

	// Send password reset email
	if err := s.emailService.SendPasswordResetEmail(user, resetToken); err != nil {
		return fmt.Errorf("failed to send password reset email: %w", err)
	}

	return nil
}

// NotificationService handles notification-related operations
type NotificationService struct {
	notificationRepo NotificationRepository
	emailService     EmailService
	smsService       SMSService
}

// NewNotificationService creates a new notification service
func NewNotificationService(
	notificationRepo NotificationRepository,
	emailService EmailService,
	smsService SMSService,
) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		emailService:     emailService,
		smsService:       smsService,
	}
}

// SendNotification sends a notification to a user
func (s *NotificationService) SendNotification(userID int, notificationType, message string, user *User) error {
	// Create notification record
	notification := &Notification{
		UserID:  userID,
		Type:    notificationType,
		Message: message,
		SentAt:  time.Now(),
	}

	// Save to repository
	if err := s.notificationRepo.SaveNotification(notification); err != nil {
		return fmt.Errorf("failed to save notification: %w", err)
	}

	// Send via appropriate channel
	switch notificationType {
	case "email":
		if err := s.emailService.SendNotificationEmail(user, notification); err != nil {
			return fmt.Errorf("failed to send email notification: %w", err)
		}
	case "sms":
		// For SMS, we'd need a phone number - this is a simplified example
		if err := s.smsService.SendSMS("1234567890", message); err != nil {
			return fmt.Errorf("failed to send SMS notification: %w", err)
		}
	}

	return nil
}

// GetNotifications retrieves notifications for a user
func (s *NotificationService) GetNotifications(userID int) ([]*Notification, error) {
	return s.notificationRepo.GetNotificationsByUser(userID)
}

// MarkAsRead marks a notification as read
func (s *NotificationService) MarkAsRead(notificationID int) error {
	return s.notificationRepo.MarkAsRead(notificationID)
}

// PaymentService handles payment processing
type PaymentService struct {
	paymentProcessor PaymentProcessor
	userRepo         UserRepository
	notificationSrv  *NotificationService
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	paymentProcessor PaymentProcessor,
	userRepo UserRepository,
	notificationSrv *NotificationService,
) *PaymentService {
	return &PaymentService{
		paymentProcessor: paymentProcessor,
		userRepo:         userRepo,
		notificationSrv:  notificationSrv,
	}
}

// ProcessPayment processes a payment for a user
func (s *PaymentService) ProcessPayment(userID int, amount float64, cardToken string) (*PaymentResult, error) {
	// Validate amount and card token
	if err := ValidatePaymentAmount(amount); err != nil {
		return nil, err
	}

	if cardToken == "" {
		return nil, errors.New("card token is required")
	}

	// Get user information
	user, err := s.userRepo.GetUser(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Process payment via payment processor
	result, err := s.paymentProcessor.ProcessPayment(amount, cardToken)
	if err != nil {
		return nil, fmt.Errorf("payment processing failed: %w", err)
	}

	// Send payment confirmation notification
	message := fmt.Sprintf("Payment of $%.2f has been processed successfully. Transaction ID: %s", amount, result.TransactionID)
	if err := s.notificationSrv.SendNotification(userID, "payment_confirmation", message, user); err != nil {
		// Log error but don't fail payment
		fmt.Printf("Failed to send payment confirmation: %v\n", err)
	}

	return result, nil
}

// RefundPayment processes a refund
func (s *PaymentService) RefundPayment(userID int, transactionID string) error {
	// Get user information
	user, err := s.userRepo.GetUser(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Process refund via payment processor
	if err := s.paymentProcessor.RefundPayment(transactionID); err != nil {
		return fmt.Errorf("refund processing failed: %w", err)
	}

	// Send refund confirmation notification
	message := fmt.Sprintf("Refund has been processed for transaction %s", transactionID)
	if err := s.notificationSrv.SendNotification(userID, "refund_confirmation", message, user); err != nil {
		// Log error but don't fail refund
		fmt.Printf("Failed to send refund confirmation: %v\n", err)
	}

	return nil
}

// APIService handles external API interactions
type APIService struct {
	httpClient HTTPClient
	baseURL    string
}

// NewAPIService creates a new API service
func NewAPIService(httpClient HTTPClient, baseURL string) *APIService {
	return &APIService{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// GetUserData retrieves user data from external API
func (s *APIService) GetUserData(userID int) (*User, error) {
	// Construct API URL
	url := fmt.Sprintf("%s/users/%d", s.baseURL, userID)

	// Make HTTP GET request
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	// Parse response
	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &user, nil
}

// SyncUserData synchronizes user data with external API
func (s *APIService) SyncUserData(user *User) error {
	// Construct API URL
	url := fmt.Sprintf("%s/users/%d", s.baseURL, user.ID)

	// Prepare request body
	body, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	// Make HTTP PUT request
	resp, err := s.httpClient.Put(url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API sync failed with status: %d", resp.StatusCode)
	}

	return nil
}

// AsyncService handles asynchronous processing
type AsyncService struct {
	processor AsyncProcessor
}

// NewAsyncService creates a new async service
func NewAsyncService(processor AsyncProcessor) *AsyncService {
	return &AsyncService{processor: processor}
}

// ProcessData processes data asynchronously with callback
func (s *AsyncService) ProcessData(data string, callback func(result string, err error)) {
	s.processor.ProcessAsync(data, callback)
}

// ProcessDataWithChannel processes data asynchronously with channel
func (s *AsyncService) ProcessDataWithChannel(data string) <-chan ProcessResult {
	return s.processor.ProcessWithChannel(data)
}

// Validation functions

// ValidateUser validates user data
func ValidateUser(user *User) error {
	if user == nil {
		return errors.New("user cannot be nil")
	}

	if strings.TrimSpace(user.Name) == "" {
		return errors.New("name is required")
	}

	if !ValidateEmail(user.Email) {
		return errors.New("invalid email format")
	}

	if user.Age <= 0 || user.Age >= 150 {
		return errors.New("age must be between 1 and 149")
	}

	return nil
}

// ValidateEmail validates email format
func ValidateEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// ValidatePaymentAmount validates payment amount
func ValidatePaymentAmount(amount float64) error {
	if amount <= 0 {
		return errors.New("amount must be positive")
	}

	if amount > 10000 {
		return errors.New("amount too large")
	}

	return nil
}

// Utility functions

// GenerateResetToken generates a password reset token
func GenerateResetToken() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	length := 32
	result := make([]byte, length)
	
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	
	return string(result)
}

// FormatNotificationMessage formats notification message
func FormatNotificationMessage(template string, user *User) string {
	message := template
	message = strings.ReplaceAll(message, "{{.Name}}", user.Name)
	message = strings.ReplaceAll(message, "{{.Email}}", user.Email)
	message = strings.ReplaceAll(message, "{{.ID}}", strconv.Itoa(user.ID))
	return message
}

// Example usage and main function
func main() {
	// This would normally be dependency injection setup
	fmt.Println("Mock generation example")
	fmt.Println("Run 'go generate ./...' to generate mocks")
	fmt.Println("Run tests with 'go test -v' to see mocks in action")
}