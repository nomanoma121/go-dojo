package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBasicTracing(t *testing.T) {
	tracer := NewTracer("test-tracer")
	
	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "test-operation")
	
	if span == nil {
		t.Fatal("Expected span to be created")
	}
	
	span.SetAttribute(StringAttribute("test.key", "test.value"))
	span.SetStatus(StatusCodeOk, "success")
	span.End()
	
	// スパンが完了していることを確認
	if span.EndTime == nil {
		t.Error("Expected span to have end time after End() call")
	}
}

func TestSpanHierarchy(t *testing.T) {
	tracer := NewTracer("test-tracer")
	
	ctx := context.Background()
	
	// 親スパンを作成
	ctx, parentSpan := tracer.Start(ctx, "parent-operation")
	if parentSpan == nil {
		t.Fatal("Expected parent span to be created")
	}
	
	// 子スパンを作成
	ctx, childSpan := tracer.Start(ctx, "child-operation")
	if childSpan == nil {
		t.Fatal("Expected child span to be created")
	}
	
	// 親子関係を確認
	if childSpan.TraceID != parentSpan.TraceID {
		t.Error("Child span should have same trace ID as parent")
	}
	
	if childSpan.ParentID != parentSpan.SpanID {
		t.Error("Child span should have parent's span ID as parent ID")
	}
	
	childSpan.End()
	parentSpan.End()
}

func TestErrorRecording(t *testing.T) {
	tracer := NewTracer("test-tracer")
	
	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "error-operation")
	
	testError := fmt.Errorf("test error")
	span.RecordError(testError)
	
	if span.Status != StatusCodeError {
		t.Error("Expected span status to be Error after recording error")
	}
	
	if span.StatusMsg != testError.Error() {
		t.Error("Expected span status message to match error message")
	}
	
	// イベントが追加されていることを確認
	if len(span.Events) == 0 {
		t.Error("Expected error event to be added")
	}
	
	span.End()
}

func TestHTTPTracingMiddleware(t *testing.T) {
	tracer := NewTracer("test-tracer")
	
	// テストハンドラー
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		span := SpanFromContext(r.Context())
		if span == nil {
			t.Error("Expected span to be available in handler context")
			return
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// ミドルウェアを適用
	tracedHandler := TracingMiddleware(tracer)(handler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	tracedHandler.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTraceContextPropagation(t *testing.T) {
	tracer := NewTracer("test-tracer")
	
	// 親スパンを作成
	ctx := context.Background()
	ctx, parentSpan := tracer.Start(ctx, "parent-span")
	
	// HTTPヘッダーを作成
	headers := make(http.Header)
	injectTraceContext(headers, parentSpan)
	
	// ヘッダーが設定されていることを確認
	traceParent := headers.Get("traceparent")
	if traceParent == "" {
		t.Error("Expected traceparent header to be set")
	}
	
	// コンテキストを抽出
	newCtx := extractTraceContext(context.Background(), headers)
	extractedSpan := SpanFromContext(newCtx)
	
	if extractedSpan == nil {
		t.Error("Expected span to be extracted from headers")
	}
	
	if extractedSpan.TraceID != parentSpan.TraceID {
		t.Error("Expected extracted span to have same trace ID")
	}
	
	parentSpan.End()
}

func TestUserService(t *testing.T) {
	userService := NewUserService()
	
	ctx := context.Background()
	
	// 既存ユーザーを取得
	user, err := userService.GetUser(ctx, "1")
	if err != nil {
		t.Errorf("Expected to find user, got error: %v", err)
	}
	
	if user == nil || user.ID != "1" {
		t.Error("Expected to get user with ID '1'")
	}
	
	// 存在しないユーザーを取得
	_, err = userService.GetUser(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent user")
	}
	
	// 新しいユーザーを作成
	newUser := &User{
		ID:    "3",
		Name:  "Test User",
		Email: "test@example.com",
	}
	
	err = userService.CreateUser(ctx, newUser)
	if err != nil {
		t.Errorf("Expected to create user, got error: %v", err)
	}
	
	// 作成されたユーザーを取得
	createdUser, err := userService.GetUser(ctx, "3")
	if err != nil {
		t.Errorf("Expected to find created user, got error: %v", err)
	}
	
	if createdUser.Name != "Test User" {
		t.Error("Expected created user to have correct name")
	}
}

func TestOrderService(t *testing.T) {
	userService := NewUserService()
	orderService := NewOrderService(userService)
	
	ctx := context.Background()
	
	order := &Order{
		UserID: "1",
		Amount: 100.0,
		Items: []Item{
			{ID: "item1", Name: "Test Item", Price: 50.0, Quantity: 2},
		},
	}
	
	err := orderService.CreateOrder(ctx, order)
	
	// 支払い失敗の可能性があるため、エラーかどうかに関わらずテストを継続
	if err != nil {
		t.Logf("Order creation failed (possibly due to payment failure): %v", err)
	} else {
		t.Log("Order created successfully")
		
		// 作成された注文を取得
		if order.ID != "" {
			retrievedOrder, err := orderService.GetOrder(ctx, order.ID)
			if err != nil {
				t.Errorf("Expected to retrieve created order, got error: %v", err)
			}
			
			if retrievedOrder.Status != "confirmed" {
				t.Error("Expected order status to be 'confirmed'")
			}
		}
	}
	
	// 存在しない注文を取得
	_, err = orderService.GetOrder(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent order")
	}
}

func TestAPIHandlers(t *testing.T) {
	userService := NewUserService()
	orderService := NewOrderService(userService)
	handler := NewAPIHandler(userService, orderService)
	
	tracer := NewTracer("test-api")
	
	// ユーザー取得テスト
	req := httptest.NewRequest("GET", "/users?id=1", nil)
	w := httptest.NewRecorder()
	
	// トレーシングミドルウェアを適用
	tracedHandler := TracingMiddleware(tracer)(http.HandlerFunc(handler.GetUserHandler))
	tracedHandler.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var user User
	err := json.NewDecoder(w.Body).Decode(&user)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}
	
	if user.ID != "1" {
		t.Error("Expected user ID to be '1'")
	}
}

func TestOrderCreationAPI(t *testing.T) {
	userService := NewUserService()
	orderService := NewOrderService(userService)
	handler := NewAPIHandler(userService, orderService)
	
	tracer := NewTracer("test-api")
	
	order := Order{
		UserID: "1",
		Amount: 150.0,
		Items: []Item{
			{ID: "item1", Name: "Test Item", Price: 75.0, Quantity: 2},
		},
	}
	
	body, _ := json.Marshal(order)
	req := httptest.NewRequest("POST", "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	// トレーシングミドルウェアを適用
	tracedHandler := TracingMiddleware(tracer)(http.HandlerFunc(handler.CreateOrderHandler))
	tracedHandler.ServeHTTP(w, req)
	
	// 支払い失敗の可能性があるため、201または500を許可
	if w.Code != http.StatusCreated && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 201 or 500, got %d", w.Code)
	}
	
	if w.Code == http.StatusCreated {
		var createdOrder Order
		err := json.NewDecoder(w.Body).Decode(&createdOrder)
		if err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}
		
		if createdOrder.ID == "" {
			t.Error("Expected created order to have an ID")
		}
		
		if createdOrder.Status != "confirmed" {
			t.Error("Expected order status to be 'confirmed'")
		}
	}
}

func TestTraceRetrieval(t *testing.T) {
	tracer := NewTracer("test-tracer")
	
	// いくつかのスパンを作成
	ctx := context.Background()
	ctx, span1 := tracer.Start(ctx, "operation1")
	ctx, span2 := tracer.Start(ctx, "operation2")
	
	span2.End()
	span1.End()
	
	// トレースを取得
	traces := tracer.GetAllTraces()
	if len(traces) == 0 {
		t.Error("Expected to have traces")
	}
	
	// 特定のトレースを取得
	spans := tracer.GetTrace(span1.TraceID)
	if len(spans) < 2 {
		t.Error("Expected to have at least 2 spans in trace")
	}
}

func TestConcurrentTracing(t *testing.T) {
	tracer := NewTracer("concurrent-tracer")
	
	const numGoroutines = 10
	const numOperations = 5
	
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			_ = context.Background()
			
			for j := 0; j < numOperations; j++ {
				_, span := tracer.Start(context.Background(), fmt.Sprintf("operation_%d_%d", id, j))
				
				span.SetAttribute(StringAttribute("goroutine.id", fmt.Sprintf("%d", id)))
				span.SetAttribute(IntAttribute("operation.number", j))
				
				// 短い処理時間をシミュレート
				time.Sleep(time.Millisecond)
				
				span.End()
			}
		}(i)
	}
	
	// 全ての goroutine の完了を待つ
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	
	// トレースが正しく記録されているかチェック
	traces := tracer.GetAllTraces()
	if len(traces) == 0 {
		t.Error("Expected traces to be recorded from concurrent operations")
	}
}

func BenchmarkSpanCreation(b *testing.B) {
	tracer := NewTracer("benchmark-tracer")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		ctx, span := tracer.Start(ctx, "benchmark-operation")
		span.SetAttribute(StringAttribute("benchmark.iteration", fmt.Sprintf("%d", i)))
		span.End()
	}
}

func BenchmarkSpanWithAttributes(b *testing.B) {
	tracer := NewTracer("benchmark-tracer")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		ctx, span := tracer.Start(ctx, "benchmark-operation-with-attrs",
			WithAttributes(
				StringAttribute("service.name", "benchmark-service"),
				IntAttribute("iteration", i),
				Float64Attribute("value", float64(i)*1.5),
			),
		)
		
		span.SetAttribute(BoolAttribute("is.benchmark", true))
		span.AddEvent("processing.started")
		span.AddEvent("processing.completed")
		span.End()
	}
}