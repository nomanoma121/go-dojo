package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestUserService_GetUser_Success(t *testing.T) {
	server := NewUserServiceServer()
	
	// テストユーザーを事前に作成
	testUser := &User{
		ID:        "user001",
		Name:      "Test User",
		Email:     "test@example.com",
		Age:       25,
		CreatedAt: time.Now().Unix(),
	}
	server.users["user001"] = testUser

	// ユーザー取得テスト
	req := &GetUserRequest{UserID: "user001"}
	user, err := server.GetUser(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if user.ID != testUser.ID {
		t.Errorf("Expected user ID %s, got %s", testUser.ID, user.ID)
	}

	if user.Name != testUser.Name {
		t.Errorf("Expected user name %s, got %s", testUser.Name, user.Name)
	}

	t.Logf("Successfully retrieved user: %+v", user)
}

func TestUserService_GetUser_NotFound(t *testing.T) {
	server := NewUserServiceServer()
	
	req := &GetUserRequest{UserID: "nonexistent"}
	user, err := server.GetUser(context.Background(), req)

	if user != nil {
		t.Errorf("Expected nil user, got %+v", user)
	}

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// エラーコードの確認
	st, ok := FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}

	if st.Code() != NotFound {
		t.Errorf("Expected NotFound code, got %v", st.Code())
	}

	t.Logf("Correctly returned NotFound error: %v", err)
}

func TestUserService_GetUser_InvalidArgument(t *testing.T) {
	server := NewUserServiceServer()
	
	req := &GetUserRequest{UserID: ""}
	user, err := server.GetUser(context.Background(), req)

	if user != nil {
		t.Errorf("Expected nil user, got %+v", user)
	}

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// エラーコードの確認
	st, ok := FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}

	if st.Code() != InvalidArgument {
		t.Errorf("Expected InvalidArgument code, got %v", st.Code())
	}

	t.Logf("Correctly returned InvalidArgument error: %v", err)
}

func TestUserService_CreateUser_Success(t *testing.T) {
	server := NewUserServiceServer()
	
	testUser := &User{
		ID:    "user001",
		Name:  "Test User",
		Email: "test@example.com",
		Age:   25,
	}

	req := &CreateUserRequest{User: testUser}
	createdUser, err := server.CreateUser(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if createdUser.ID != testUser.ID {
		t.Errorf("Expected user ID %s, got %s", testUser.ID, createdUser.ID)
	}

	if createdUser.CreatedAt == 0 {
		t.Error("Expected CreatedAt to be set")
	}

	t.Logf("Successfully created user: %+v", createdUser)
}

func TestUserService_CreateUser_ValidationError(t *testing.T) {
	server := NewUserServiceServer()
	
	// 無効なユーザーデータ
	testUser := &User{
		ID:    "",           // 空のID
		Name:  "",           // 空の名前
		Email: "",           // 空のメール
		Age:   0,            // 無効な年齢
	}

	req := &CreateUserRequest{User: testUser}
	user, err := server.CreateUser(context.Background(), req)

	if user != nil {
		t.Errorf("Expected nil user, got %+v", user)
	}

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// エラーコードの確認
	st, ok := FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}

	if st.Code() != InvalidArgument {
		t.Errorf("Expected InvalidArgument code, got %v", st.Code())
	}

	// バリデーションエラーの詳細確認
	foundDetails := false
	for _, detail := range st.Details() {
		if errorDetails, ok := detail.(*ErrorDetails); ok {
			foundDetails = true
			if len(errorDetails.ValidationErrors) == 0 {
				t.Error("Expected validation errors, got none")
			}

			// 期待されるエラーフィールドのチェック
			expectedFields := map[string]bool{
				"id": false, "name": false, "email": false, "age": false,
			}
			
			for _, ve := range errorDetails.ValidationErrors {
				if _, exists := expectedFields[ve.Field]; exists {
					expectedFields[ve.Field] = true
				}
			}

			for field, found := range expectedFields {
				if !found {
					t.Errorf("Expected validation error for field %s", field)
				}
			}

			t.Logf("Validation errors found: %d", len(errorDetails.ValidationErrors))
		}
	}

	if !foundDetails {
		t.Error("Expected ErrorDetails in status")
	}

	t.Logf("Correctly returned validation errors: %v", err)
}

func TestUserService_CreateUser_AlreadyExists(t *testing.T) {
	server := NewUserServiceServer()
	
	// 既存ユーザーを作成
	existingUser := &User{
		ID:        "user001",
		Name:      "Existing User",
		Email:     "existing@example.com",
		Age:       30,
		CreatedAt: time.Now().Unix(),
	}
	server.users["user001"] = existingUser

	// 同じIDで新しいユーザーを作成しようとする
	newUser := &User{
		ID:    "user001",
		Name:  "New User",
		Email: "new@example.com",
		Age:   25,
	}

	req := &CreateUserRequest{User: newUser}
	user, err := server.CreateUser(context.Background(), req)

	if user != nil {
		t.Errorf("Expected nil user, got %+v", user)
	}

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// エラーコードの確認
	st, ok := FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}

	if st.Code() != AlreadyExists {
		t.Errorf("Expected AlreadyExists code, got %v", st.Code())
	}

	t.Logf("Correctly returned AlreadyExists error: %v", err)
}

func TestUserService_UpdateUser_Success(t *testing.T) {
	server := NewUserServiceServer()
	
	// 既存ユーザーを作成
	originalTime := time.Now().Unix()
	existingUser := &User{
		ID:        "user001",
		Name:      "Original User",
		Email:     "original@example.com",
		Age:       25,
		CreatedAt: originalTime,
	}
	server.users["user001"] = existingUser

	// ユーザー更新
	updatedUser := &User{
		ID:    "user001",
		Name:  "Updated User",
		Email: "updated@example.com",
		Age:   30,
	}

	req := &UpdateUserRequest{User: updatedUser}
	result, err := server.UpdateUser(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Name != updatedUser.Name {
		t.Errorf("Expected name %s, got %s", updatedUser.Name, result.Name)
	}

	if result.CreatedAt != originalTime {
		t.Errorf("Expected CreatedAt to be preserved: %d, got %d", originalTime, result.CreatedAt)
	}

	t.Logf("Successfully updated user: %+v", result)
}

func TestUserService_UpdateUser_NotFound(t *testing.T) {
	server := NewUserServiceServer()
	
	updateUser := &User{
		ID:    "nonexistent",
		Name:  "Updated User",
		Email: "updated@example.com",
		Age:   30,
	}

	req := &UpdateUserRequest{User: updateUser}
	user, err := server.UpdateUser(context.Background(), req)

	if user != nil {
		t.Errorf("Expected nil user, got %+v", user)
	}

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// エラーコードの確認
	st, ok := FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}

	if st.Code() != NotFound {
		t.Errorf("Expected NotFound code, got %v", st.Code())
	}

	t.Logf("Correctly returned NotFound error: %v", err)
}

func TestUserService_DeleteUser_Success(t *testing.T) {
	server := NewUserServiceServer()
	
	// 既存ユーザーを作成
	existingUser := &User{
		ID:        "user001",
		Name:      "Test User",
		Email:     "test@example.com",
		Age:       25,
		CreatedAt: time.Now().Unix(),
	}
	server.users["user001"] = existingUser

	req := &DeleteUserRequest{UserID: "user001"}
	err := server.DeleteUser(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// ユーザーが削除されたことを確認
	if _, exists := server.users["user001"]; exists {
		t.Error("Expected user to be deleted")
	}

	t.Log("Successfully deleted user")
}

func TestUserService_ListUsers_Success(t *testing.T) {
	server := NewUserServiceServer()
	
	// テストユーザーを複数作成
	for i := 1; i <= 15; i++ {
		user := &User{
			ID:        fmt.Sprintf("user%03d", i),
			Name:      fmt.Sprintf("User %d", i),
			Email:     fmt.Sprintf("user%d@example.com", i),
			Age:       int32(20 + i),
			CreatedAt: time.Now().Unix(),
		}
		server.users[user.ID] = user
	}

	// ページサイズ10でリクエスト
	req := &ListUsersRequest{PageSize: 10}
	resp, err := server.ListUsers(context.Background(), req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(resp.Users) != 10 {
		t.Errorf("Expected 10 users, got %d", len(resp.Users))
	}

	if resp.NextPageToken == "" {
		t.Error("Expected next page token")
	}

	t.Logf("Successfully retrieved %d users with next page token: %s", len(resp.Users), resp.NextPageToken)
}

func TestUserClient_GetUserWithRetry_Success(t *testing.T) {
	server := NewUserServiceServer()
	client := NewUserClient(server)
	
	// 模擬的な失敗とその後の成功をテストするため、
	// 一時的に利用不可能なサーバーを作成
	failingServer := &FailingUserServiceServer{
		server:       server,
		failAttempts: 2, // 最初の2回は失敗
	}
	failingClient := NewUserClient(failingServer)

	// テストユーザーを作成
	testUser := &User{
		ID:        "user001",
		Name:      "Test User",
		Email:     "test@example.com",
		Age:       25,
		CreatedAt: time.Now().Unix(),
	}
	server.users["user001"] = testUser

	config := RetryConfig{
		MaxAttempts: 3,
		BackoffBase: 10 * time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
		RetryableCodes: map[Code]bool{
			Unavailable: true,
		},
	}

	user, err := failingClient.GetUserWithRetry(context.Background(), "user001", config)

	if err != nil {
		t.Fatalf("Expected no error after retry, got %v", err)
	}

	if user.ID != testUser.ID {
		t.Errorf("Expected user ID %s, got %s", testUser.ID, user.ID)
	}

	t.Logf("Successfully retrieved user after retry: %+v", user)
}

func TestUserClient_GetUserWithRetry_NonRetryableError(t *testing.T) {
	server := NewUserServiceServer()
	client := NewUserClient(server)

	config := RetryConfig{
		MaxAttempts: 3,
		BackoffBase: 10 * time.Millisecond,
		MaxBackoff:  100 * time.Millisecond,
		RetryableCodes: map[Code]bool{
			Unavailable: true,
		},
	}

	// 存在しないユーザー（NotFoundエラー、リトライ不可）
	user, err := client.GetUserWithRetry(context.Background(), "nonexistent", config)

	if user != nil {
		t.Errorf("Expected nil user, got %+v", user)
	}

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// ErrUserNotFound が返されることを確認
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("Expected ErrUserNotFound, got %v", err)
	}

	t.Logf("Correctly failed without retry for non-retryable error: %v", err)
}

func TestUserClient_HandleError_ValidationError(t *testing.T) {
	server := NewUserServiceServer()
	client := NewUserClient(server)

	// バリデーションエラーを発生させる
	invalidUser := &User{
		ID:    "",
		Name:  "",
		Email: "invalid-email",
		Age:   -1,
	}

	_, err := client.CreateUser(context.Background(), invalidUser)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// バリデーションエラーメッセージが含まれることを確認
	errMsg := err.Error()
	if !strings.Contains(errMsg, "validation errors") {
		t.Errorf("Expected validation error message, got: %s", errMsg)
	}

	t.Logf("Correctly handled validation error: %v", err)
}

func TestIsValidEmail(t *testing.T) {
	testCases := []struct {
		email string
		valid bool
	}{
		{"test@example.com", true},
		{"user.name@domain.co.jp", true},
		{"user+tag@example.org", true},
		{"invalid.email", false},
		{"@example.com", false},
		{"test@", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := isValidEmail(tc.email)
		if result != tc.valid {
			t.Errorf("isValidEmail(%s) = %v, expected %v", tc.email, result, tc.valid)
		}
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == "" {
		t.Error("Expected non-empty request ID")
	}

	if id1 == id2 {
		t.Error("Expected unique request IDs")
	}

	if len(id1) != 16 { // 8バイト = 16文字の16進数
		t.Errorf("Expected request ID length 16, got %d", len(id1))
	}

	t.Logf("Generated request IDs: %s, %s", id1, id2)
}

func TestCalculateBackoff(t *testing.T) {
	client := &UserClient{}
	config := RetryConfig{
		BackoffBase: 100 * time.Millisecond,
		MaxBackoff:  1 * time.Second,
	}

	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},   // 100ms * 2^0 = 100ms
		{1, 200 * time.Millisecond},   // 100ms * 2^1 = 200ms
		{2, 400 * time.Millisecond},   // 100ms * 2^2 = 400ms
		{3, 800 * time.Millisecond},   // 100ms * 2^3 = 800ms
		{4, 1 * time.Second},          // 100ms * 2^4 = 1.6s, but capped at 1s
	}

	for _, tc := range testCases {
		result := client.calculateBackoff(tc.attempt, config)
		if result != tc.expected {
			t.Errorf("calculateBackoff(attempt=%d) = %v, expected %v", tc.attempt, result, tc.expected)
		}
	}
}

// FailingUserServiceServer はテスト用の一時的に失敗するサーバー
type FailingUserServiceServer struct {
	*UserServiceServer
	failAttempts int
	attempts     int
}

func (s *FailingUserServiceServer) GetUser(ctx context.Context, req *GetUserRequest) (*User, error) {
	s.attempts++
	
	if s.attempts <= s.failAttempts {
		return nil, Error(Unavailable, "service temporarily unavailable")
	}
	
	return s.UserServiceServer.GetUser(ctx, req)
}

func TestRetryableErrorTypes(t *testing.T) {
	client := &UserClient{}

	testCases := []struct {
		code      Code
		retryable bool
	}{
		{OK, false},
		{NotFound, false},
		{InvalidArgument, false},
		{Unavailable, true},
		{DeadlineExceeded, true},
		{Internal, true},
		{PermissionDenied, false},
	}

	for _, tc := range testCases {
		err := Error(tc.code, "test error")
		retryable := client.isRetryableError(err)
		
		if retryable != tc.retryable {
			t.Errorf("isRetryableError for code %v = %v, expected %v", tc.code, retryable, tc.retryable)
		}
	}
}

// ベンチマークテスト
func BenchmarkUserService_GetUser(b *testing.B) {
	server := NewUserServiceServer()
	
	// テストユーザーを作成
	testUser := &User{
		ID:        "user001",
		Name:      "Test User",
		Email:     "test@example.com",
		Age:       25,
		CreatedAt: time.Now().Unix(),
	}
	server.users["user001"] = testUser

	req := &GetUserRequest{UserID: "user001"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := server.GetUser(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUserService_CreateUser(b *testing.B) {
	server := NewUserServiceServer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user := &User{
			ID:    fmt.Sprintf("user%d", i),
			Name:  fmt.Sprintf("User %d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Age:   25,
		}
		
		req := &CreateUserRequest{User: user}
		_, err := server.CreateUser(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
	}
}