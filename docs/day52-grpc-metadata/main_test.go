package main

import (
	"context"
	"strings"
	"testing"
)

// テスト用のモック実装

// MockServerStream テスト用のサーバーストリーム
type MockServerStream struct {
	ctx context.Context
}

func NewMockServerStream(ctx context.Context) *MockServerStream {
	return &MockServerStream{ctx: ctx}
}

func (s *MockServerStream) Context() context.Context { return s.ctx }
func (s *MockServerStream) SendMsg(m interface{}) error { return nil }
func (s *MockServerStream) RecvMsg(m interface{}) error { return nil }

// テストケース

func TestMetadataManager_Basic(t *testing.T) {
	manager := NewMetadataManager()
	
	// プロパゲーターを追加
	requestIDPropagator := NewRequestIDPropagator()
	tracePropagator := NewTracePropagator()
	manager.AddPropagator(requestIDPropagator)
	manager.AddPropagator(tracePropagator)
	
	// フィルターを追加
	securityFilter := NewMetadataSecurityFilter()
	manager.AddFilter(securityFilter)
	
	// バリデーターを追加
	authValidator := NewAuthMetadataValidator()
	authValidator.AddSkipPath("/Test/SkipAuth")
	manager.AddValidator(authValidator)
	
	if len(manager.propagators) != 2 {
		t.Errorf("Expected 2 propagators, got %d", len(manager.propagators))
	}
	
	if len(manager.filters) != 1 {
		t.Errorf("Expected 1 filter, got %d", len(manager.filters))
	}
	
	if len(manager.validators) != 1 {
		t.Errorf("Expected 1 validator, got %d", len(manager.validators))
	}
	
	t.Log("MetadataManager initialized successfully")
}

func TestRequestIDPropagator(t *testing.T) {
	propagator := NewRequestIDPropagator()
	ctx := context.Background()
	md := make(MockMetadata)
	
	// リクエストIDなしの場合
	newCtx, newMD, err := propagator.Propagate(ctx, md)
	if err != nil {
		t.Errorf("Propagate failed: %v", err)
	}
	
	requestID := getMetadataValue(newMD, RequestIDKey)
	if requestID == "" {
		t.Error("Expected request ID to be generated")
	}
	
	ctxRequestID, ok := newCtx.Value(RequestIDKey).(string)
	if !ok || ctxRequestID != requestID {
		t.Error("Request ID not properly set in context")
	}
	
	// 既存のリクエストIDがある場合
	existingID := "existing-req-123"
	md.Set(RequestIDKey, existingID)
	
	_, newMD2, err := propagator.Propagate(ctx, md)
	if err != nil {
		t.Errorf("Propagate with existing ID failed: %v", err)
	}
	
	if getMetadataValue(newMD2, RequestIDKey) != existingID {
		t.Error("Expected existing request ID to be preserved")
	}
	
	t.Logf("Request ID propagated successfully: %s", requestID)
}

func TestTracePropagator(t *testing.T) {
	propagator := NewTracePropagator()
	ctx := context.Background()
	md := make(MockMetadata)
	
	// 新しいトレースの場合
	_, newMD, err := propagator.Propagate(ctx, md)
	if err != nil {
		t.Errorf("Propagate failed: %v", err)
	}
	
	traceID := getMetadataValue(newMD, TraceIDKey)
	spanID := getMetadataValue(newMD, SpanIDKey)
	
	if traceID == "" {
		t.Error("Expected trace ID to be generated")
	}
	
	if spanID == "" {
		t.Error("Expected span ID to be generated")
	}
	
	// 既存のトレースの場合
	existingTraceID := "existing-trace-123"
	existingSpanID := "existing-span-456"
	
	md2 := make(MockMetadata)
	md2.Set(TraceIDKey, existingTraceID)
	md2.Set(SpanIDKey, existingSpanID)
	
	_, newMD2, err := propagator.Propagate(ctx, md2)
	if err != nil {
		t.Errorf("Propagate with existing trace failed: %v", err)
	}
	
	if getMetadataValue(newMD2, TraceIDKey) != existingTraceID {
		t.Error("Expected existing trace ID to be preserved")
	}
	
	newSpanID := getMetadataValue(newMD2, SpanIDKey)
	parentSpanID := getMetadataValue(newMD2, ParentSpanIDKey)
	
	if newSpanID == existingSpanID {
		t.Error("Expected new span ID to be generated")
	}
	
	if parentSpanID != existingSpanID {
		t.Error("Expected parent span ID to be set to existing span ID")
	}
	
	t.Logf("Trace propagated: trace=%s, span=%s, parent=%s", 
		getMetadataValue(newMD2, TraceIDKey), newSpanID, parentSpanID)
}

func TestAuthMetadataValidator(t *testing.T) {
	validator := NewAuthMetadataValidator()
	
	// 有効なトークンのテスト
	ctx := context.Background()
	md := make(MockMetadata)
	md.Set(AuthorizationKey, "Bearer token123")
	
	err := validator.Validate(ctx, md)
	if err != nil {
		t.Errorf("Expected valid token to pass validation: %v", err)
	}
	
	// 無効なトークンのテスト
	md2 := make(MockMetadata)
	md2.Set(AuthorizationKey, "Bearer invalidtoken")
	
	err = validator.Validate(ctx, md2)
	if err == nil {
		t.Error("Expected invalid token to fail validation")
	}
	
	// トークンなしのテスト
	md3 := make(MockMetadata)
	
	err = validator.Validate(ctx, md3)
	if err == nil {
		t.Error("Expected missing token to fail validation")
	}
	
	// スキップパスのテスト
	ctxWithSkipPath := context.WithValue(ctx, "method", "/Health/Check")
	
	err = validator.Validate(ctxWithSkipPath, md3)
	if err != nil {
		t.Errorf("Expected skip path to bypass validation: %v", err)
	}
	
	t.Log("Auth metadata validator working correctly")
}

func TestMetadataSecurityFilter(t *testing.T) {
	filter := NewMetadataSecurityFilter()
	
	md := make(MockMetadata)
	md.Set(RequestIDKey, "req-123")
	md.Set("password", "secret123")
	md.Set("internal-token", "internal-secret")
	md.Set("unknown-key", "unknown-value")
	
	filtered := filter.Filter(md)
	
	// 許可されたキーは残る
	if getMetadataValue(filtered, RequestIDKey) == "" {
		t.Error("Expected allowed key to be preserved")
	}
	
	// 機密キーは削除される
	if getMetadataValue(filtered, "password") != "" {
		t.Error("Expected sensitive key to be filtered")
	}
	
	if getMetadataValue(filtered, "internal-token") != "" {
		t.Error("Expected sensitive key to be filtered")
	}
	
	// 未知のキーは削除される
	if getMetadataValue(filtered, "unknown-key") != "" {
		t.Error("Expected unknown key to be filtered")
	}
	
	t.Logf("Filtered metadata: %d keys -> %d keys", md.Len(), filtered.Len())
}

func TestMetadataChain(t *testing.T) {
	chain := NewMetadataChain()
	
	// テスト用プロセッサーを作成
	processor1 := &TestMetadataProcessor{name: "processor1"}
	processor2 := &TestMetadataProcessor{name: "processor2"}
	
	chain.AddProcessor(processor1)
	chain.AddProcessor(processor2)
	
	ctx := context.Background()
	md := make(MockMetadata)
	md.Set("input", "test")
	
	resultCtx, resultMD, err := chain.Process(ctx, md)
	if err != nil {
		t.Errorf("Chain processing failed: %v", err)
	}
	
	// 両方のプロセッサーが実行されたかチェック
	if getMetadataValue(resultMD, "processor1") == "" {
		t.Error("Expected processor1 to be executed")
	}
	
	if getMetadataValue(resultMD, "processor2") == "" {
		t.Error("Expected processor2 to be executed")
	}
	
	// コンテキストの値もチェック
	if value, ok := resultCtx.Value("processor1").(string); !ok || value != "executed" {
		t.Error("Expected processor1 context value")
	}
	
	if value, ok := resultCtx.Value("processor2").(string); !ok || value != "executed" {
		t.Error("Expected processor2 context value")
	}
	
	t.Log("Metadata chain processed successfully")
}

// TestMetadataProcessor テスト用のメタデータプロセッサー
type TestMetadataProcessor struct {
	name string
}

func (p *TestMetadataProcessor) Process(ctx context.Context, md MockMetadata) (context.Context, MockMetadata, error) {
	// メタデータに処理済みマークを追加
	md.Set(p.name, "executed")
	
	// コンテキストにも値を追加
	ctx = context.WithValue(ctx, p.name, "executed")
	
	return ctx, md, nil
}

func TestMetadataAwareClient(t *testing.T) {
	manager := NewMetadataManager()
	manager.AddPropagator(NewRequestIDPropagator())
	
	client := NewMetadataAwareClient(manager)
	client.SetDefaultMetadata(ClientVersionKey, "1.0.0")
	client.SetDefaultMetadata("custom-header", "custom-value")
	
	ctx := context.Background()
	preparedCtx, err := client.PrepareContext(ctx)
	if err != nil {
		t.Errorf("PrepareContext failed: %v", err)
	}
	
	// コンテキストが変更されているかチェック
	if preparedCtx == ctx {
		t.Error("Expected context to be modified")
	}
	
	t.Log("Metadata aware client prepared context successfully")
}

func TestMetadataManager_ProcessIncoming(t *testing.T) {
	manager := NewMetadataManager()
	manager.AddPropagator(NewRequestIDPropagator())
	manager.AddPropagator(NewTracePropagator())
	manager.AddFilter(NewMetadataSecurityFilter())
	
	// 認証バリデーターをスキップパス付きで追加
	authValidator := NewAuthMetadataValidator()
	authValidator.AddSkipPath("/Test/ProcessIncoming")
	manager.AddValidator(authValidator)
	
	ctx := context.WithValue(context.Background(), "method", "/Test/ProcessIncoming")
	md := make(MockMetadata)
	md.Set(RequestIDKey, "test-req-123")
	md.Set("password", "should-be-filtered")
	
	processedCtx, err := manager.ProcessIncoming(ctx, md)
	if err != nil {
		t.Errorf("ProcessIncoming failed: %v", err)
	}
	
	// リクエストIDがコンテキストに設定されているかチェック
	if requestID, ok := processedCtx.Value(RequestIDKey).(string); !ok || requestID == "" {
		t.Error("Expected request ID in processed context")
	}
	
	// トレースIDがコンテキストに設定されているかチェック
	if traceID, ok := processedCtx.Value(TraceIDKey).(string); !ok || traceID == "" {
		t.Error("Expected trace ID in processed context")
	}
	
	t.Log("Incoming metadata processed successfully")
}

func TestMetadataManager_ProcessOutgoing(t *testing.T) {
	manager := NewMetadataManager()
	manager.AddPropagator(NewRequestIDPropagator())
	
	ctx := context.WithValue(context.Background(), RequestIDKey, "outgoing-req-123")
	ctx = context.WithValue(ctx, TraceIDKey, "outgoing-trace-456")
	
	processedCtx, err := manager.ProcessOutgoing(ctx)
	if err != nil {
		t.Errorf("ProcessOutgoing failed: %v", err)
	}
	
	// コンテキストが処理されているかチェック
	if processedCtx == ctx {
		t.Error("Expected context to be processed")
	}
	
	t.Log("Outgoing metadata processed successfully")
}

func TestMetadataManager_UnaryServerInterceptor(t *testing.T) {
	manager := NewMetadataManager()
	manager.AddPropagator(NewRequestIDPropagator())
	
	interceptor := manager.UnaryServerInterceptor()
	
	ctx := context.Background()
	req := &TestRequest{Message: "test"}
	info := &UnaryServerInfo{FullMethod: "/Test/UnaryMethod"}
	
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		// リクエストIDがコンテキストに設定されているかチェック
		if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
			return &TestResponse{Message: "success", RequestID: requestID}, nil
		}
		return &TestResponse{Message: "no request ID"}, nil
	}
	
	response, err := interceptor(ctx, req, info, handler)
	if err != nil {
		t.Errorf("Unary interceptor failed: %v", err)
	}
	
	if resp, ok := response.(*TestResponse); ok {
		if resp.RequestID == "" {
			t.Error("Expected request ID in response")
		}
		t.Logf("Unary interceptor response: %+v", resp)
	} else {
		t.Error("Expected TestResponse type")
	}
}

func TestMetadataManager_StreamServerInterceptor(t *testing.T) {
	manager := NewMetadataManager()
	manager.AddPropagator(NewRequestIDPropagator())
	
	interceptor := manager.StreamServerInterceptor()
	
	ctx := context.Background()
	stream := NewMockServerStream(ctx)
	info := &StreamServerInfo{
		FullMethod:     "/Test/StreamMethod",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	handlerCalled := false
	handler := func(srv interface{}, ss ServerStream) error {
		handlerCalled = true
		
		// ストリームコンテキストにリクエストIDがあるかチェック
		if requestID, ok := ss.Context().Value(RequestIDKey).(string); ok && requestID != "" {
			t.Logf("Stream handler received request ID: %s", requestID)
		} else {
			t.Error("Expected request ID in stream context")
		}
		
		return nil
	}
	
	err := interceptor(nil, stream, info, handler)
	if err != nil {
		t.Errorf("Stream interceptor failed: %v", err)
	}
	
	if !handlerCalled {
		t.Error("Expected handler to be called")
	}
	
	t.Log("Stream interceptor executed successfully")
}

// テスト用の構造体
type TestRequest struct {
	Message string `json:"message"`
}

type TestResponse struct {
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// ユーティリティ関数のテスト

func TestGenerateIDs(t *testing.T) {
	// リクエストID生成テスト
	requestID1 := generateRequestID()
	requestID2 := generateRequestID()
	
	if requestID1 == requestID2 {
		t.Error("Expected unique request IDs")
	}
	
	if !strings.HasPrefix(requestID1, "req-") {
		t.Error("Expected request ID to have 'req-' prefix")
	}
	
	// トレースID生成テスト
	traceID1 := generateTraceID()
	traceID2 := generateTraceID()
	
	if traceID1 == traceID2 {
		t.Error("Expected unique trace IDs")
	}
	
	if !strings.HasPrefix(traceID1, "trace-") {
		t.Error("Expected trace ID to have 'trace-' prefix")
	}
	
	// スパンID生成テスト
	spanID1 := generateSpanID()
	spanID2 := generateSpanID()
	
	if spanID1 == spanID2 {
		t.Error("Expected unique span IDs")
	}
	
	if !strings.HasPrefix(spanID1, "span-") {
		t.Error("Expected span ID to have 'span-' prefix")
	}
	
	t.Logf("Generated IDs: req=%s, trace=%s, span=%s", requestID1, traceID1, spanID1)
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"Valid Bearer token", "Bearer token123", "token123"},
		{"Bearer with spaces", "Bearer  token456  ", "token456"},
		{"No Bearer prefix", "token789", ""},
		{"Empty header", "", ""},
		{"Only Bearer", "Bearer", ""},
		{"Different case", "bearer token123", ""},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := extractBearerToken(test.header)
			if result != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, result)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	validTokens := []string{"token123", "token456", "admintoken", "testtoken"}
	invalidTokens := []string{"invalid", "", "wrong", "expired"}
	
	for _, token := range validTokens {
		userID, err := validateToken(token)
		if err != nil {
			t.Errorf("Expected token %s to be valid: %v", token, err)
		}
		if userID == "" {
			t.Errorf("Expected non-empty user ID for token %s", token)
		}
	}
	
	for _, token := range invalidTokens {
		_, err := validateToken(token)
		if err == nil {
			t.Errorf("Expected token %s to be invalid", token)
		}
	}
}

func TestMockMetadata(t *testing.T) {
	md := make(MockMetadata)
	
	// Set and Get
	md.Set("key1", "value1")
	values := md.Get("key1")
	if len(values) != 1 || values[0] != "value1" {
		t.Error("Set/Get failed")
	}
	
	// Append
	md.Append("key1", "value2", "value3")
	values = md.Get("key1")
	if len(values) != 3 {
		t.Error("Append failed")
	}
	
	// Copy
	copy := md.Copy()
	copy.Set("key2", "value4")
	
	if len(md.Get("key2")) != 0 {
		t.Error("Copy should be independent")
	}
	
	// Delete
	md.Delete("key1")
	if len(md.Get("key1")) != 0 {
		t.Error("Delete failed")
	}
	
	t.Log("MockMetadata operations working correctly")
}

// ベンチマークテスト

func BenchmarkRequestIDGeneration(b *testing.B) {
	for i := 0; i < b.N; i++ {
		generateRequestID()
	}
}

func BenchmarkMetadataPropagation(b *testing.B) {
	propagator := NewRequestIDPropagator()
	ctx := context.Background()
	
	for i := 0; i < b.N; i++ {
		md := make(MockMetadata)
		propagator.Propagate(ctx, md)
	}
}

func BenchmarkMetadataFiltering(b *testing.B) {
	filter := NewMetadataSecurityFilter()
	md := make(MockMetadata)
	md.Set(RequestIDKey, "req-123")
	md.Set("password", "secret")
	md.Set("internal-token", "token")
	
	for i := 0; i < b.N; i++ {
		filter.Filter(md)
	}
}