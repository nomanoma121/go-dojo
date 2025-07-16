package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLoggingInterceptor(t *testing.T) {
	service := NewUserService()
	interceptor := LoggingInterceptor()
	server := NewInterceptorServer(service, interceptor)

	ctx := context.Background()
	req := &UserRequest{UserID: "user1"}

	resp, err := server.GetUser(ctx, req)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.ID != "user1" {
		t.Errorf("Expected user ID 'user1', got %s", resp.ID)
	}

	if resp.Name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got %s", resp.Name)
	}

	t.Log("Logging interceptor successfully logged request and response")
}

func TestAuthInterceptor_ValidToken(t *testing.T) {
	service := NewUserService()
	interceptor := AuthInterceptor()
	server := NewInterceptorServer(service, interceptor)

	// 有効なトークンでリクエスト
	ctx := context.WithValue(context.Background(), tokenKey, "token123")
	req := &UserRequest{UserID: "user1"}

	resp, err := server.GetUser(ctx, req)

	if err != nil {
		t.Fatalf("Expected no error with valid token, got %v", err)
	}

	if resp.ID != "user1" {
		t.Errorf("Expected user ID 'user1', got %s", resp.ID)
	}

	t.Log("Auth interceptor successfully validated token")
}

func TestAuthInterceptor_InvalidToken(t *testing.T) {
	service := NewUserService()
	interceptor := AuthInterceptor()
	server := NewInterceptorServer(service, interceptor)

	// 無効なトークンでリクエスト
	ctx := context.WithValue(context.Background(), tokenKey, "invalid_token")
	req := &UserRequest{UserID: "user1"}

	resp, err := server.GetUser(ctx, req)

	if err == nil {
		t.Fatal("Expected error with invalid token")
	}

	if resp != nil {
		t.Error("Expected nil response with invalid token")
	}

	if !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("Expected 'invalid token' error, got %v", err)
	}

	t.Log("Auth interceptor correctly rejected invalid token")
}

func TestAuthInterceptor_MissingToken(t *testing.T) {
	service := NewUserService()
	interceptor := AuthInterceptor()
	server := NewInterceptorServer(service, interceptor)

	// トークンなしでリクエスト
	ctx := context.Background()
	req := &UserRequest{UserID: "user1"}

	resp, err := server.GetUser(ctx, req)

	if err == nil {
		t.Fatal("Expected error with missing token")
	}

	if resp != nil {
		t.Error("Expected nil response with missing token")
	}

	if !strings.Contains(err.Error(), "authentication token required") {
		t.Errorf("Expected 'authentication token required' error, got %v", err)
	}

	t.Log("Auth interceptor correctly required authentication token")
}

func TestMetricsInterceptor(t *testing.T) {
	service := NewUserService()
	metrics := NewMetrics()
	interceptor := MetricsInterceptor(metrics)
	server := NewInterceptorServer(service, interceptor)

	ctx := context.Background()
	req := &UserRequest{UserID: "user1"}

	// 複数のリクエストを実行
	for i := 0; i < 3; i++ {
		_, err := server.GetUser(ctx, req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
	}

	// メトリクスを確認
	metricsData := metrics.GetMetrics()

	requestCount := metricsData["request_count"].(map[string]int64)
	if requestCount["/UserService/GetUser"] != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount["/UserService/GetUser"])
	}

	errorCount := metricsData["error_count"].(map[string]int64)
	if errorCount["/UserService/GetUser"] != 0 {
		t.Errorf("Expected 0 errors, got %d", errorCount["/UserService/GetUser"])
	}

	avgResponseTime := metricsData["avg_response_time"].(map[string]time.Duration)
	if avgResponseTime["/UserService/GetUser"] <= 0 {
		t.Error("Expected positive average response time")
	}

	t.Logf("Metrics interceptor recorded: %d requests, %d errors, avg time: %v", 
		requestCount["/UserService/GetUser"], 
		errorCount["/UserService/GetUser"],
		avgResponseTime["/UserService/GetUser"])
}

func TestRateLimitInterceptor_AllowedRequests(t *testing.T) {
	service := NewUserService()
	limiter := NewRateLimiter(3, time.Second) // 1秒間に3リクエストまで
	interceptor := RateLimitInterceptor(limiter)
	server := NewInterceptorServer(service, interceptor)

	clientID := "test_client"
	ctx := context.WithValue(context.Background(), clientIDKey, clientID)
	req := &UserRequest{UserID: "user1"}

	// 制限内のリクエスト
	for i := 0; i < 3; i++ {
		_, err := server.GetUser(ctx, req)
		if err != nil {
			t.Fatalf("Request %d should be allowed, got error: %v", i+1, err)
		}
	}

	t.Log("Rate limiter correctly allowed requests within limit")
}

func TestRateLimitInterceptor_ExceededLimit(t *testing.T) {
	service := NewUserService()
	limiter := NewRateLimiter(2, time.Second) // 1秒間に2リクエストまで
	interceptor := RateLimitInterceptor(limiter)
	server := NewInterceptorServer(service, interceptor)

	clientID := "test_client"
	ctx := context.WithValue(context.Background(), clientIDKey, clientID)
	req := &UserRequest{UserID: "user1"}

	// 制限内のリクエスト
	for i := 0; i < 2; i++ {
		_, err := server.GetUser(ctx, req)
		if err != nil {
			t.Fatalf("Request %d should be allowed, got error: %v", i+1, err)
		}
	}

	// 制限を超えるリクエスト
	_, err := server.GetUser(ctx, req)
	if err == nil {
		t.Fatal("Expected rate limit error")
	}

	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("Expected 'rate limit exceeded' error, got %v", err)
	}

	t.Log("Rate limiter correctly blocked request exceeding limit")
}

func TestRecoveryInterceptor(t *testing.T) {
	service := NewUserService()
	interceptor := RecoveryInterceptor()
	_ = NewInterceptorServer(service, interceptor)

	// パニックを発生させるハンドラをモック
	panicHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("test panic")
	}

	info := &ServerInfo{
		FullMethod: "/TestService/Panic",
		Server:     service,
	}

	ctx := context.Background()
	req := &UserRequest{UserID: "user1"}

	resp, err := interceptor(ctx, req, info, panicHandler)

	if err == nil {
		t.Fatal("Expected error from panic recovery")
	}

	if resp != nil {
		t.Error("Expected nil response from panic recovery")
	}

	if !strings.Contains(err.Error(), "panic recovered") {
		t.Errorf("Expected 'panic recovered' error, got %v", err)
	}

	t.Log("Recovery interceptor successfully recovered from panic")
}

func TestChainUnaryServer(t *testing.T) {
	service := NewUserService()
	metrics := NewMetrics()

	// 複数のインターセプタを連鎖
	chainedInterceptor := ChainUnaryServer(
		LoggingInterceptor(),
		AuthInterceptor(),
		MetricsInterceptor(metrics),
		RecoveryInterceptor(),
	)

	server := NewInterceptorServer(service, chainedInterceptor)

	// 認証トークン付きでリクエスト
	ctx := context.WithValue(context.Background(), tokenKey, "token123")
	req := &UserRequest{UserID: "user1"}

	resp, err := server.GetUser(ctx, req)

	if err != nil {
		t.Fatalf("Expected no error with chained interceptors, got %v", err)
	}

	if resp.ID != "user1" {
		t.Errorf("Expected user ID 'user1', got %s", resp.ID)
	}

	// メトリクスが記録されたことを確認
	metricsData := metrics.GetMetrics()
	requestCount := metricsData["request_count"].(map[string]int64)
	if requestCount["/UserService/GetUser"] != 1 {
		t.Errorf("Expected 1 request in metrics, got %d", requestCount["/UserService/GetUser"])
	}

	t.Log("Chained interceptors executed successfully")
}

func TestClientInterceptors(t *testing.T) {
	service := NewUserService()
	serverInterceptor := LoggingInterceptor()
	server := NewInterceptorServer(service, serverInterceptor)

	// クライアントインターセプタを設定
	client := NewInterceptorClient(server, 
		ClientLoggingInterceptor(),
		ClientAuthInterceptor("token123"),
	)

	ctx := context.Background()
	req := &UserRequest{UserID: "user1"}

	resp, err := client.GetUser(ctx, req)

	if err != nil {
		t.Fatalf("Expected no error with client interceptors, got %v", err)
	}

	if resp.ID != "user1" {
		t.Errorf("Expected user ID 'user1', got %s", resp.ID)
	}

	t.Log("Client interceptors executed successfully")
}

func TestCreateUserWithInterceptors(t *testing.T) {
	service := NewUserService()
	metrics := NewMetrics()
	limiter := NewRateLimiter(10, time.Second)

	chainedInterceptor := ChainUnaryServer(
		LoggingInterceptor(),
		AuthInterceptor(),
		MetricsInterceptor(metrics),
		RateLimitInterceptor(limiter),
	)

	server := NewInterceptorServer(service, chainedInterceptor)

	ctx := context.WithValue(context.Background(), tokenKey, "admin")
	ctx = context.WithValue(ctx, clientIDKey, "admin_client")

	req := &CreateUserRequest{
		Name:  "Alice Smith",
		Email: "alice@example.com",
	}

	resp, err := server.CreateUser(ctx, req)

	if err != nil {
		t.Fatalf("Expected no error creating user, got %v", err)
	}

	if resp.Name != "Alice Smith" {
		t.Errorf("Expected name 'Alice Smith', got %s", resp.Name)
	}

	if resp.Email != "alice@example.com" {
		t.Errorf("Expected email 'alice@example.com', got %s", resp.Email)
	}

	if resp.Status != "active" {
		t.Errorf("Expected status 'active', got %s", resp.Status)
	}

	// メトリクスを確認
	metricsData := metrics.GetMetrics()
	requestCount := metricsData["request_count"].(map[string]int64)
	if requestCount["/UserService/CreateUser"] != 1 {
		t.Errorf("Expected 1 create request in metrics, got %d", requestCount["/UserService/CreateUser"])
	}

	t.Logf("Successfully created user with all interceptors: %+v", resp)
}

func TestRateLimiterRecovery(t *testing.T) {
	limiter := NewRateLimiter(2, 100*time.Millisecond)

	clientID := "test_client"

	// 制限まで使用
	for i := 0; i < 2; i++ {
		if !limiter.IsAllowed(clientID) {
			t.Fatalf("Request %d should be allowed", i+1)
		}
	}

	// 制限を超える
	if limiter.IsAllowed(clientID) {
		t.Fatal("Request should be blocked")
	}

	// 時間経過後に回復
	time.Sleep(150 * time.Millisecond)

	if !limiter.IsAllowed(clientID) {
		t.Fatal("Request should be allowed after time window")
	}

	t.Log("Rate limiter correctly recovered after time window")
}

func TestMetricsErrorCounting(t *testing.T) {
	service := NewUserService()
	metrics := NewMetrics()
	interceptor := MetricsInterceptor(metrics)
	server := NewInterceptorServer(service, interceptor)

	ctx := context.Background()

	// 成功リクエスト
	successReq := &UserRequest{UserID: "user1"}
	_, err := server.GetUser(ctx, successReq)
	if err != nil {
		t.Fatalf("Success request failed: %v", err)
	}

	// 失敗リクエスト
	failReq := &UserRequest{UserID: "nonexistent"}
	_, err = server.GetUser(ctx, failReq)
	if err == nil {
		t.Fatal("Expected error for nonexistent user")
	}

	// メトリクスを確認
	metricsData := metrics.GetMetrics()
	requestCount := metricsData["request_count"].(map[string]int64)
	errorCount := metricsData["error_count"].(map[string]int64)

	if requestCount["/UserService/GetUser"] != 2 {
		t.Errorf("Expected 2 total requests, got %d", requestCount["/UserService/GetUser"])
	}

	if errorCount["/UserService/GetUser"] != 1 {
		t.Errorf("Expected 1 error, got %d", errorCount["/UserService/GetUser"])
	}

	t.Logf("Metrics correctly tracked: %d requests, %d errors", 
		requestCount["/UserService/GetUser"], 
		errorCount["/UserService/GetUser"])
}

// ベンチマークテスト
func BenchmarkInterceptorChain(b *testing.B) {
	service := NewUserService()
	metrics := NewMetrics()
	limiter := NewRateLimiter(1000000, time.Second) // 高い制限

	chainedInterceptor := ChainUnaryServer(
		LoggingInterceptor(),
		MetricsInterceptor(metrics),
		RateLimitInterceptor(limiter),
		RecoveryInterceptor(),
	)

	server := NewInterceptorServer(service, chainedInterceptor)
	ctx := context.WithValue(context.Background(), clientIDKey, "bench_client")
	req := &UserRequest{UserID: "user1"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := server.GetUser(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRateLimiter(b *testing.B) {
	limiter := NewRateLimiter(1000000, time.Second)
	clientID := "bench_client"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		limiter.IsAllowed(clientID)
	}
}