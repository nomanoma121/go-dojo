package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoggerInitialization(t *testing.T) {
	initLogger()
	
	if logger == nil {
		t.Fatal("Expected logger to be initialized")
	}
}

func TestMetricsCreation(t *testing.T) {
	metrics := NewMetrics()
	
	if metrics == nil {
		t.Fatal("Expected metrics to be created")
	}
	
	// リクエスト総数の増加をテスト
	metrics.IncRequestsTotal("GET", "/api/users", "200")
	exported := metrics.Export()
	
	if exported == nil {
		t.Error("Expected metrics to be exportable")
	}
}

func TestMetricsOperations(t *testing.T) {
	metrics := NewMetrics()
	
	// 様々なメトリクス操作をテスト
	metrics.IncRequestsTotal("GET", "/api/users", "200")
	metrics.IncRequestsTotal("POST", "/api/orders", "201")
	metrics.ObserveRequestDuration("GET", "/api/users", 0.1)
	metrics.SetActiveRequests("/api/users", 5)
	metrics.IncErrorsTotal("GET", "/api/users", "timeout")
	metrics.SetBusinessMetric("users_total", 100)
	
	exported := metrics.Export()
	
	// メトリクスが正しく記録されているかチェック
	if exported["http_requests_total"] == nil {
		t.Error("Expected http_requests_total to be recorded")
	}
	
	if exported["business_metrics"] == nil {
		t.Error("Expected business_metrics to be recorded")
	}
}

func TestTracerCreation(t *testing.T) {
	tracer := NewTracer("test-service")
	
	if tracer == nil {
		t.Fatal("Expected tracer to be created")
	}
}

func TestSpanOperations(t *testing.T) {
	tracer := NewTracer("test-service")
	
	ctx := context.Background()
	ctx, span := tracer.StartSpan(ctx, "test-operation")
	
	if span == nil {
		t.Fatal("Expected span to be created")
	}
	
	// スパンにタグとログを追加
	span.SetTag("test.key", "test.value")
	span.LogEvent("test event", map[string]interface{}{
		"field1": "value1",
		"field2": 42,
	})
	
	// エラーを設定
	testError := fmt.Errorf("test error")
	span.SetError(testError)
	
	span.Finish()
	
	// スパンが正しく完了していることを確認
	if span.EndTime == nil {
		t.Error("Expected span to have end time")
	}
	
	if span.Duration == nil {
		t.Error("Expected span to have duration")
	}
	
	if span.Error == nil {
		t.Error("Expected span to have error recorded")
	}
}

func TestSpanHierarchy(t *testing.T) {
	tracer := NewTracer("test-service")
	
	ctx := context.Background()
	
	// 親スパンを作成
	ctx, parentSpan := tracer.StartSpan(ctx, "parent-operation")
	
	// 子スパンを作成
	ctx, childSpan := tracer.StartSpan(ctx, "child-operation")
	
	// 親子関係を確認
	if childSpan.TraceID != parentSpan.TraceID {
		t.Error("Child span should have same trace ID as parent")
	}
	
	if childSpan.ParentID != parentSpan.SpanID {
		t.Error("Child span should have parent's span ID as parent ID")
	}
	
	childSpan.Finish()
	parentSpan.Finish()
}

func TestUserService(t *testing.T) {
	tracer := NewTracer("test-service")
	userService := NewUserService(tracer)
	
	if userService == nil {
		t.Fatal("Expected user service to be created")
	}
	
	ctx := context.Background()
	
	// ユーザー作成をテスト
	user, err := userService.CreateUser(ctx, "John Doe", "john@example.com")
	if err != nil {
		t.Errorf("Expected user creation to succeed, got error: %v", err)
	}
	
	if user == nil {
		t.Fatal("Expected user to be created")
	}
	
	if user.Name != "John Doe" {
		t.Error("Expected user name to be 'John Doe'")
	}
	
	if user.Email != "john@example.com" {
		t.Error("Expected user email to be 'john@example.com'")
	}
	
	// 作成されたユーザーを取得
	retrievedUser, err := userService.GetUser(ctx, user.ID)
	if err != nil {
		t.Errorf("Expected to retrieve user, got error: %v", err)
	}
	
	if retrievedUser.ID != user.ID {
		t.Error("Expected retrieved user to have same ID")
	}
	
	// 存在しないユーザーの取得
	_, err = userService.GetUser(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error when retrieving nonexistent user")
	}
}

func TestUserServiceValidation(t *testing.T) {
	tracer := NewTracer("test-service")
	userService := NewUserService(tracer)
	
	ctx := context.Background()
	
	// 名前が空の場合のテスト
	_, err := userService.CreateUser(ctx, "", "test@example.com")
	if err == nil {
		t.Error("Expected error when name is empty")
	}
	
	// メールが空の場合のテスト
	_, err = userService.CreateUser(ctx, "Test User", "")
	if err == nil {
		t.Error("Expected error when email is empty")
	}
}

func TestOrderService(t *testing.T) {
	tracer := NewTracer("test-service")
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	
	if orderService == nil {
		t.Fatal("Expected order service to be created")
	}
	
	ctx := context.Background()
	
	// まずユーザーを作成
	user, err := userService.CreateUser(ctx, "Jane Doe", "jane@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	
	// 注文アイテムを準備
	items := []Item{
		{ProductID: "prod1", Name: "Product 1", Price: 10.0, Quantity: 2},
		{ProductID: "prod2", Name: "Product 2", Price: 15.0, Quantity: 1},
	}
	
	// 注文作成をテスト（支払い失敗の可能性があるため）
	order, err := orderService.CreateOrder(ctx, user.ID, items)
	
	if err != nil {
		if strings.Contains(err.Error(), "payment failed") {
			t.Logf("Order creation failed due to payment failure (expected): %v", err)
			return
		}
		t.Errorf("Unexpected error during order creation: %v", err)
		return
	}
	
	// 注文が正常に作成された場合
	if order == nil {
		t.Fatal("Expected order to be created")
	}
	
	if order.UserID != user.ID {
		t.Error("Expected order to have correct user ID")
	}
	
	expectedAmount := 10.0*2 + 15.0*1 // 35.0
	if order.Amount != expectedAmount {
		t.Errorf("Expected order amount to be %.2f, got %.2f", expectedAmount, order.Amount)
	}
	
	if order.Status != "confirmed" {
		t.Error("Expected order status to be 'confirmed'")
	}
	
	// 作成された注文を取得
	retrievedOrder, err := orderService.GetOrder(ctx, order.ID)
	if err != nil {
		t.Errorf("Expected to retrieve order, got error: %v", err)
	}
	
	if retrievedOrder.ID != order.ID {
		t.Error("Expected retrieved order to have same ID")
	}
}

func TestOrderServiceValidation(t *testing.T) {
	tracer := NewTracer("test-service")
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	
	ctx := context.Background()
	
	// 存在しないユーザーで注文作成
	items := []Item{
		{ProductID: "prod1", Name: "Product 1", Price: 10.0, Quantity: 1},
	}
	
	_, err := orderService.CreateOrder(ctx, "nonexistent-user", items)
	if err == nil {
		t.Error("Expected error when creating order with nonexistent user")
	}
}

func TestAPIServer(t *testing.T) {
	tracer := NewTracer("test-service")
	metrics := NewMetrics()
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	
	apiServer := NewAPIServer(userService, orderService, metrics, tracer)
	
	if apiServer == nil {
		t.Fatal("Expected API server to be created")
	}
}

func TestCreateUserHandler(t *testing.T) {
	initLogger()
	tracer := NewTracer("test-service")
	metrics := NewMetrics()
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	apiServer := NewAPIServer(userService, orderService, metrics, tracer)
	
	// リクエストボディを準備
	requestBody := map[string]string{
		"name":  "Test User",
		"email": "test@example.com",
	}
	bodyBytes, _ := json.Marshal(requestBody)
	
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	// ミドルウェアを適用
	handler := apiServer.MetricsMiddleware(http.HandlerFunc(apiServer.CreateUserHandler))
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
	
	var user User
	err := json.NewDecoder(w.Body).Decode(&user)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}
	
	if user.Name != "Test User" {
		t.Error("Expected user name to be 'Test User'")
	}
}

func TestCreateOrderHandler(t *testing.T) {
	initLogger()
	tracer := NewTracer("test-service")
	metrics := NewMetrics()
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	apiServer := NewAPIServer(userService, orderService, metrics, tracer)
	
	// まずユーザーを作成
	user, err := userService.CreateUser(context.Background(), "Order User", "order@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	
	// 注文リクエストを準備
	requestBody := map[string]interface{}{
		"user_id": user.ID,
		"items": []map[string]interface{}{
			{
				"product_id": "prod1",
				"name":       "Test Product",
				"price":      20.0,
				"quantity":   2,
			},
		},
	}
	bodyBytes, _ := json.Marshal(requestBody)
	
	req := httptest.NewRequest("POST", "/api/orders", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	// ミドルウェアを適用
	handler := apiServer.MetricsMiddleware(http.HandlerFunc(apiServer.CreateOrderHandler))
	handler.ServeHTTP(w, req)
	
	// 支払い失敗の可能性があるため、201または400を許可
	if w.Code != http.StatusCreated && w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 201 or 400, got %d", w.Code)
	}
	
	if w.Code == http.StatusCreated {
		var order Order
		err := json.NewDecoder(w.Body).Decode(&order)
		if err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}
		
		if order.UserID != user.ID {
			t.Error("Expected order to have correct user ID")
		}
		
		if order.Amount != 40.0 { // 20.0 * 2
			t.Errorf("Expected order amount to be 40.0, got %.2f", order.Amount)
		}
	}
}

func TestMetricsHandler(t *testing.T) {
	tracer := NewTracer("test-service")
	metrics := NewMetrics()
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	apiServer := NewAPIServer(userService, orderService, metrics, tracer)
	
	// いくつかのメトリクスを記録
	metrics.IncRequestsTotal("GET", "/api/users", "200")
	metrics.SetBusinessMetric("users_total", 10)
	
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	
	apiServer.MetricsHandler(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var metricsData map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&metricsData)
	if err != nil {
		t.Errorf("Failed to decode metrics response: %v", err)
	}
	
	if metricsData["business_metrics"] == nil {
		t.Error("Expected business_metrics to be present")
	}
}

func TestTracesHandler(t *testing.T) {
	tracer := NewTracer("test-service")
	metrics := NewMetrics()
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	apiServer := NewAPIServer(userService, orderService, metrics, tracer)
	
	// いくつかのスパンを作成
	ctx, span := tracer.StartSpan(context.Background(), "test-operation")
	span.SetTag("test.key", "test.value")
	span.Finish()
	
	req := httptest.NewRequest("GET", "/traces", nil)
	w := httptest.NewRecorder()
	
	apiServer.TracesHandler(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var tracesData map[string]*Span
	err := json.NewDecoder(w.Body).Decode(&tracesData)
	if err != nil {
		t.Errorf("Failed to decode traces response: %v", err)
	}
	
	if len(tracesData) == 0 {
		t.Error("Expected traces to be present")
	}
}

func TestHealthHandler(t *testing.T) {
	tracer := NewTracer("test-service")
	metrics := NewMetrics()
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	apiServer := NewAPIServer(userService, orderService, metrics, tracer)
	
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	apiServer.HealthHandler(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var healthData map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&healthData)
	if err != nil {
		t.Errorf("Failed to decode health response: %v", err)
	}
	
	if healthData["status"] != "healthy" {
		t.Error("Expected status to be 'healthy'")
	}
	
	if healthData["service"] != "ecommerce-api" {
		t.Error("Expected service to be 'ecommerce-api'")
	}
}

func TestMiddlewareMetricsRecording(t *testing.T) {
	initLogger()
	tracer := NewTracer("test-service")
	metrics := NewMetrics()
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	apiServer := NewAPIServer(userService, orderService, metrics, tracer)
	
	// テストハンドラー
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// ミドルウェアを適用
	wrappedHandler := apiServer.MetricsMiddleware(handler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	wrappedHandler.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	// メトリクスが記録されているか確認
	exported := metrics.Export()
	if exported["http_requests_total"] == nil {
		t.Error("Expected http_requests_total to be recorded by middleware")
	}
}

func TestConcurrentRequests(t *testing.T) {
	initLogger()
	tracer := NewTracer("test-service")
	metrics := NewMetrics()
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	apiServer := NewAPIServer(userService, orderService, metrics, tracer)
	
	const numRequests = 10
	done := make(chan bool, numRequests)
	
	handler := apiServer.MetricsMiddleware(http.HandlerFunc(apiServer.HealthHandler))
	
	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("Request %d: Expected status 200, got %d", id, w.Code)
			}
		}(i)
	}
	
	// 全てのリクエストの完了を待つ
	for i := 0; i < numRequests; i++ {
		<-done
	}
	
	// メトリクスが正しく記録されているかチェック
	exported := metrics.Export()
	if exported["http_requests_total"] == nil {
		t.Error("Expected metrics to be recorded from concurrent requests")
	}
}

func BenchmarkUserCreation(b *testing.B) {
	tracer := NewTracer("benchmark-service")
	userService := NewUserService(tracer)
	
	ctx := context.Background()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		name := fmt.Sprintf("User%d", i)
		email := fmt.Sprintf("user%d@example.com", i)
		
		_, err := userService.CreateUser(ctx, name, email)
		if err != nil {
			b.Errorf("Failed to create user: %v", err)
		}
	}
}

func BenchmarkOrderCreation(b *testing.B) {
	tracer := NewTracer("benchmark-service")
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	
	ctx := context.Background()
	
	// テストユーザーを作成
	user, err := userService.CreateUser(ctx, "Benchmark User", "benchmark@example.com")
	if err != nil {
		b.Fatalf("Failed to create user: %v", err)
	}
	
	items := []Item{
		{ProductID: "prod1", Name: "Benchmark Product", Price: 10.0, Quantity: 1},
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := orderService.CreateOrder(ctx, user.ID, items)
		// 支払い失敗は無視
		if err != nil && !strings.Contains(err.Error(), "payment failed") {
			b.Errorf("Unexpected error: %v", err)
		}
	}
}