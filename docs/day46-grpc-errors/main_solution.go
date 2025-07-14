// Day 46: gRPC Error Handling
// gRPCにおける詳細なエラーハンドリングシステムの実装

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// gRPCステータスコードの定義（簡単化版）
type Code int32

const (
	OK                 Code = 0
	Cancelled          Code = 1
	Unknown            Code = 2
	InvalidArgument    Code = 3
	DeadlineExceeded   Code = 4
	NotFound           Code = 5
	AlreadyExists      Code = 6
	PermissionDenied   Code = 7
	ResourceExhausted  Code = 8
	FailedPrecondition Code = 9
	Aborted            Code = 10
	OutOfRange         Code = 11
	Unimplemented      Code = 12
	Internal           Code = 13
	Unavailable        Code = 14
	DataLoss           Code = 15
	Unauthenticated    Code = 16
)

func (c Code) String() string {
	switch c {
	case OK:
		return "OK"
	case Cancelled:
		return "CANCELLED"
	case Unknown:
		return "UNKNOWN"
	case InvalidArgument:
		return "INVALID_ARGUMENT"
	case DeadlineExceeded:
		return "DEADLINE_EXCEEDED"
	case NotFound:
		return "NOT_FOUND"
	case AlreadyExists:
		return "ALREADY_EXISTS"
	case PermissionDenied:
		return "PERMISSION_DENIED"
	case ResourceExhausted:
		return "RESOURCE_EXHAUSTED"
	case FailedPrecondition:
		return "FAILED_PRECONDITION"
	case Aborted:
		return "ABORTED"
	case OutOfRange:
		return "OUT_OF_RANGE"
	case Unimplemented:
		return "UNIMPLEMENTED"
	case Internal:
		return "INTERNAL"
	case Unavailable:
		return "UNAVAILABLE"
	case DataLoss:
		return "DATA_LOSS"
	case Unauthenticated:
		return "UNAUTHENTICATED"
	default:
		return "UNKNOWN"
	}
}

// データ構造定義
type User struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Age       int32  `json:"age"`
	CreatedAt int64  `json:"created_at"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ErrorDetails struct {
	ValidationErrors []*ValidationError `json:"validation_errors"`
	RequestID        string             `json:"request_id"`
	Timestamp        int64              `json:"timestamp"`
}

// リクエスト/レスポンス定義
type GetUserRequest struct {
	UserID string `json:"user_id"`
}

type CreateUserRequest struct {
	User *User `json:"user"`
}

type UpdateUserRequest struct {
	User *User `json:"user"`
}

type DeleteUserRequest struct {
	UserID string `json:"user_id"`
}

type ListUsersRequest struct {
	PageSize  int32  `json:"page_size"`
	PageToken string `json:"page_token"`
}

type ListUsersResponse struct {
	Users         []*User `json:"users"`
	NextPageToken string  `json:"next_page_token"`
}

// エラー構造
type Status struct {
	code    Code
	message string
	details []interface{}
}

func NewStatus(code Code, message string) *Status {
	return &Status{
		code:    code,
		message: message,
		details: make([]interface{}, 0),
	}
}

func (s *Status) Code() Code {
	return s.code
}

func (s *Status) Message() string {
	return s.message
}

func (s *Status) Details() []interface{} {
	return s.details
}

func (s *Status) WithDetails(details ...interface{}) (*Status, error) {
	newStatus := *s
	newStatus.details = append(newStatus.details, details...)
	return &newStatus, nil
}

func (s *Status) Err() error {
	if s.code == OK {
		return nil
	}
	return &StatusError{status: s}
}

type StatusError struct {
	status *Status
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("rpc error: code = %s desc = %s", e.status.code, e.status.message)
}

func (e *StatusError) GRPCStatus() *Status {
	return e.status
}

// エラー作成ヘルパー関数
func Error(code Code, message string) error {
	return NewStatus(code, message).Err()
}

func Errorf(code Code, format string, args ...interface{}) error {
	return Error(code, fmt.Sprintf(format, args...))
}

func FromError(err error) (*Status, bool) {
	if err == nil {
		return NewStatus(OK, ""), true
	}

	if se, ok := err.(*StatusError); ok {
		return se.status, true
	}

	return NewStatus(Unknown, err.Error()), false
}

// リトライ設定
type RetryConfig struct {
	MaxAttempts    int
	BackoffBase    time.Duration
	MaxBackoff     time.Duration
	RetryableCodes map[Code]bool
}

func NewDefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BackoffBase: 100 * time.Millisecond,
		MaxBackoff:  5 * time.Second,
		RetryableCodes: map[Code]bool{
			Unavailable:      true,
			DeadlineExceeded: true,
			Internal:         true,
		},
	}
}

// UserService サーバー実装
type UserServiceServer struct {
	users map[string]*User
	mu    sync.RWMutex
}

func NewUserServiceServer() *UserServiceServer {
	return &UserServiceServer{
		users: make(map[string]*User),
	}
}

// GetUser ユーザーを取得します
func (s *UserServiceServer) GetUser(ctx context.Context, req *GetUserRequest) (*User, error) {
	// 入力検証
	if req.UserID == "" {
		st := NewStatus(InvalidArgument, "user_id is required")
		st, _ = st.WithDetails(&ValidationError{
			Field:   "user_id",
			Message: "cannot be empty",
		})
		return nil, st.Err()
	}

	s.mu.RLock()
	user, exists := s.users[req.UserID]
	s.mu.RUnlock()

	if !exists {
		return nil, Error(NotFound, "user not found")
	}

	return user, nil
}

// CreateUser ユーザーを作成します
func (s *UserServiceServer) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// バリデーション
	if err := s.validateUser(req.User); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 重複チェック
	if _, exists := s.users[req.User.ID]; exists {
		return nil, Errorf(AlreadyExists, "user %s already exists", req.User.ID)
	}

	// 作成日時を設定
	req.User.CreatedAt = time.Now().Unix()

	// ユーザー作成
	s.users[req.User.ID] = req.User
	return req.User, nil
}

// UpdateUser ユーザーを更新します
func (s *UserServiceServer) UpdateUser(ctx context.Context, req *UpdateUserRequest) (*User, error) {
	// バリデーション
	if err := s.validateUser(req.User); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 存在確認
	existingUser, exists := s.users[req.User.ID]
	if !exists {
		return nil, Error(NotFound, "user not found")
	}

	// 作成日時は保持
	req.User.CreatedAt = existingUser.CreatedAt

	// ユーザー更新
	s.users[req.User.ID] = req.User
	return req.User, nil
}

// DeleteUser ユーザーを削除します
func (s *UserServiceServer) DeleteUser(ctx context.Context, req *DeleteUserRequest) error {
	// 入力検証
	if req.UserID == "" {
		return Error(InvalidArgument, "user_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 存在確認
	if _, exists := s.users[req.UserID]; !exists {
		return Error(NotFound, "user not found")
	}

	// ユーザー削除
	delete(s.users, req.UserID)
	return nil
}

// ListUsers ユーザー一覧を取得します
func (s *UserServiceServer) ListUsers(ctx context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	// デフォルトページサイズの設定
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 全ユーザーを取得
	allUsers := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		allUsers = append(allUsers, user)
	}

	// ページング処理（簡単化）
	start := 0
	if req.PageToken != "" {
		// ページトークンは簡単化のため、開始インデックスとして扱う
		fmt.Sscanf(req.PageToken, "%d", &start)
	}

	end := start + int(pageSize)
	if end > len(allUsers) {
		end = len(allUsers)
	}

	users := allUsers[start:end]

	// 次のページトークンを生成
	var nextPageToken string
	if end < len(allUsers) {
		nextPageToken = fmt.Sprintf("%d", end)
	}

	return &ListUsersResponse{
		Users:         users,
		NextPageToken: nextPageToken,
	}, nil
}

// validateUser ユーザーのバリデーションを行います
func (s *UserServiceServer) validateUser(user *User) error {
	var validationErrors []*ValidationError

	if user.ID == "" {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "id",
			Message: "cannot be empty",
		})
	}

	if user.Name == "" {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "name",
			Message: "cannot be empty",
		})
	}

	if user.Email == "" {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "email",
			Message: "cannot be empty",
		})
	} else if !isValidEmail(user.Email) {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "email",
			Message: "invalid email format",
		})
	}

	if user.Age <= 0 {
		validationErrors = append(validationErrors, &ValidationError{
			Field:   "age",
			Message: "must be positive",
		})
	}

	if len(validationErrors) > 0 {
		st := NewStatus(InvalidArgument, "validation failed")
		st, _ = st.WithDetails(&ErrorDetails{
			ValidationErrors: validationErrors,
			RequestID:        generateRequestID(),
			Timestamp:        time.Now().Unix(),
		})
		return st.Err()
	}

	return nil
}

// isValidEmail メールアドレスの形式をチェックします
func isValidEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	match, _ := regexp.MatchString(pattern, email)
	return match
}

// generateRequestID ユニークなリクエストIDを生成します
func generateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// UserClient クライアント実装
type UserClient struct {
	server *UserServiceServer
}

func NewUserClient(server *UserServiceServer) *UserClient {
	return &UserClient{server: server}
}

// GetUser サーバーを呼び出してユーザーを取得します
func (c *UserClient) GetUser(ctx context.Context, userID string) (*User, error) {
	req := &GetUserRequest{UserID: userID}
	user, err := c.server.GetUser(ctx, req)
	if err != nil {
		return nil, c.handleError(err)
	}
	return user, nil
}

// CreateUser ユーザーを作成します
func (c *UserClient) CreateUser(ctx context.Context, user *User) (*User, error) {
	req := &CreateUserRequest{User: user}
	createdUser, err := c.server.CreateUser(ctx, req)
	if err != nil {
		return nil, c.handleError(err)
	}
	return createdUser, nil
}

// GetUserWithRetry リトライ機能付きでユーザーを取得します
func (c *UserClient) GetUserWithRetry(ctx context.Context, userID string, config RetryConfig) (*User, error) {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		user, err := c.GetUser(ctx, userID)
		if err == nil {
			return user, nil
		}

		lastErr = err

		// リトライ可能なエラーかチェック
		if !c.isRetryableError(err) {
			return nil, err
		}

		// 最後の試行でなければ待機
		if attempt < config.MaxAttempts-1 {
			backoff := c.calculateBackoff(attempt, config)
			time.Sleep(backoff)
		}
	}

	return nil, fmt.Errorf("max retry attempts reached: %w", lastErr)
}

// handleError gRPCエラーを適切に処理し、分類します
func (c *UserClient) handleError(err error) error {
	st, ok := FromError(err)
	if !ok {
		// gRPCエラーではない
		return err
	}

	switch st.Code() {
	case NotFound:
		return ErrUserNotFound
	case InvalidArgument:
		// 詳細なバリデーションエラーを処理
		return c.processValidationErrors(st)
	case Unavailable:
		// リトライ可能なエラー
		return ErrServiceUnavailable
	case DeadlineExceeded:
		return ErrTimeout
	case AlreadyExists:
		return ErrUserAlreadyExists
	default:
		return fmt.Errorf("gRPC error: %s", st.Message())
	}
}

// processValidationErrors バリデーションエラーの詳細を処理します
func (c *UserClient) processValidationErrors(st *Status) error {
	for _, detail := range st.Details() {
		if errorDetails, ok := detail.(*ErrorDetails); ok {
			var messages []string
			for _, ve := range errorDetails.ValidationErrors {
				messages = append(messages, fmt.Sprintf("%s: %s", ve.Field, ve.Message))
			}
			return fmt.Errorf("validation errors: %s", strings.Join(messages, ", "))
		}
	}
	return fmt.Errorf("validation failed: %s", st.Message())
}

// isRetryableError エラーがリトライ可能かどうかを判定します
func (c *UserClient) isRetryableError(err error) bool {
	st, ok := FromError(err)
	if !ok {
		return false
	}

	switch st.Code() {
	case Unavailable, DeadlineExceeded, Internal:
		return true
	default:
		return false
	}
}

// calculateBackoff 指数バックオフを計算します
func (c *UserClient) calculateBackoff(attempt int, config RetryConfig) time.Duration {
	backoff := config.BackoffBase * time.Duration(1<<attempt) // 指数バックオフ
	if backoff > config.MaxBackoff {
		backoff = config.MaxBackoff
	}
	return backoff
}

// カスタムエラー定義
var (
	ErrUserNotFound        = errors.New("user not found")
	ErrServiceUnavailable  = errors.New("service unavailable")
	ErrTimeout             = errors.New("request timeout")
	ErrValidationFailed    = errors.New("validation failed")
	ErrUserAlreadyExists   = errors.New("user already exists")
)

func main() {
	fmt.Println("Day 46: gRPC Error Handling")
	fmt.Println("Run 'go test -v' to see the error handling system in action")
}