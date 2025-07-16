package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// テスト用のモック実装

// MockServerStream はテスト用のServerStream実装
type MockServerStream struct {
	ctx      context.Context
	messages []interface{}
	sent     []interface{}
	index    int
	mu       sync.Mutex
}

func NewMockServerStream(ctx context.Context, messages []interface{}) *MockServerStream {
	return &MockServerStream{
		ctx:      ctx,
		messages: messages,
		sent:     make([]interface{}, 0),
		index:    0,
	}
}

func (m *MockServerStream) SetHeader(md map[string]string) error { return nil }
func (m *MockServerStream) SendHeader(md map[string]string) error { return nil }
func (m *MockServerStream) SetTrailer(md map[string]string) {}
func (m *MockServerStream) Context() context.Context { return m.ctx }

func (m *MockServerStream) SendMsg(msg interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

func (m *MockServerStream) RecvMsg(msg interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.index >= len(m.messages) {
		return io.EOF
	}
	
	// 簡単なコピー（実際のgRPCではproto unmarshalが行われる）
	if srcMsg, ok := m.messages[m.index].(*StreamMessage); ok {
		if dstMsg, ok := msg.(*StreamMessage); ok {
			*dstMsg = *srcMsg
		}
	}
	
	m.index++
	return nil
}

func (m *MockServerStream) GetSentMessages() []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]interface{}, len(m.sent))
	copy(result, m.sent)
	return result
}

// テスト用のストリーミングサービス
type TestStreamingService struct{}

// Using types from main_solution.go

func (s *TestStreamingService) ServerSideStream(req *StreamRequest, stream ServerStream) error {
	for i := 0; i < 5; i++ {
		msg := &StreamMessage{
			ID:        fmt.Sprintf("msg_%d", i+1),
			Content:   fmt.Sprintf("Server message %d", i+1),
			Timestamp: time.Now().Unix(),
		}
		
		if err := stream.SendMsg(msg); err != nil {
			return err
		}
		
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func (s *TestStreamingService) ClientSideStream(stream ServerStream) (*StreamResponse, error) {
	var count int32
	
	for {
		var msg StreamMessage
		err := stream.RecvMsg(&msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		count++
	}
	
	return &StreamResponse{
		Count:   count,
		Status:  "SUCCESS",
		Summary: fmt.Sprintf("Received %d messages", count),
	}, nil
}

// テストケース

func TestWrappedServerStream(t *testing.T) {
	ctx := context.Background()
	mockStream := NewMockServerStream(ctx, nil)
	wrapped := NewWrappedServerStream(mockStream)
	
	// SendMsg のテスト
	for i := 0; i < 3; i++ {
		msg := &StreamMessage{
			ID:      fmt.Sprintf("test_%d", i),
			Content: fmt.Sprintf("Test message %d", i),
		}
		err := wrapped.SendMsg(msg)
		if err != nil {
			t.Errorf("SendMsg failed: %v", err)
		}
	}
	
	// RecvMsg のテスト
	messages := []*StreamMessage{
		{ID: "recv_1", Content: "Receive test 1"},
		{ID: "recv_2", Content: "Receive test 2"},
	}
	
	mockRecvStream := NewMockServerStream(ctx, []interface{}{messages[0], messages[1]})
	wrappedRecv := NewWrappedServerStream(mockRecvStream)
	
	for i := 0; i < 2; i++ {
		var msg StreamMessage
		err := wrappedRecv.RecvMsg(&msg)
		if err != nil {
			t.Errorf("RecvMsg failed: %v", err)
		}
	}
	
	// 統計情報を確認
	sent, recv, duration := wrapped.GetStats()
	if sent != 3 {
		t.Errorf("Expected 3 sent messages, got %d", sent)
	}
	
	sentRecv, recvRecv, _ := wrappedRecv.GetStats()
	if recvRecv != 2 {
		t.Errorf("Expected 2 received messages, got %d", recvRecv)
	}
	if sentRecv != 0 {
		t.Errorf("Expected 0 sent messages on recv stream, got %d", sentRecv)
	}
	
	if duration <= 0 {
		t.Error("Expected positive duration")
	}
	
	t.Logf("Wrapped stream stats: sent=%d, recv=%d, duration=%v", sent, recv, duration)
}

func TestStreamMetrics(t *testing.T) {
	metrics := NewStreamMetrics()
	method := "/TestService/TestMethod"
	
	// ストリーム開始
	metrics.StartStream(method)
	
	// ストリーム終了（統計情報付き）
	metrics.EndStream(method, 5, 3, 500*time.Millisecond)
	
	// メトリクスを取得
	result := metrics.GetMetrics()
	
	activeStreams := result["active_streams"].(map[string]int64)
	completedStreams := result["completed_streams"].(map[string]int64)
	messagesSent := result["messages_sent"].(map[string]int64)
	messagesReceived := result["messages_received"].(map[string]int64)
	
	if activeStreams[method] != 0 {
		t.Errorf("Expected 0 active streams, got %d", activeStreams[method])
	}
	
	if completedStreams[method] != 1 {
		t.Errorf("Expected 1 completed stream, got %d", completedStreams[method])
	}
	
	if messagesSent[method] != 5 {
		t.Errorf("Expected 5 messages sent, got %d", messagesSent[method])
	}
	
	if messagesReceived[method] != 3 {
		t.Errorf("Expected 3 messages received, got %d", messagesReceived[method])
	}
	
	t.Logf("Stream metrics collected successfully: %+v", result)
}

func TestStreamRateLimiter(t *testing.T) {
	limiter := NewStreamRateLimiter()
	method := "/TestService/RateLimitTest"
	
	// 制限を設定
	limiter.SetLimit(method, 2)
	
	// 最初の2つのストリームは成功するはず
	if !limiter.CanStartStream(method) {
		t.Error("Expected to be able to start first stream")
	}
	limiter.StartStream(method)
	
	if !limiter.CanStartStream(method) {
		t.Error("Expected to be able to start second stream")
	}
	limiter.StartStream(method)
	
	// 3つ目のストリームは拒否されるはず
	if limiter.CanStartStream(method) {
		t.Error("Expected to be rate limited for third stream")
	}
	
	// 1つのストリームを終了
	limiter.EndStream(method)
	
	// 再び開始できるはず
	if !limiter.CanStartStream(method) {
		t.Error("Expected to be able to start stream after one ended")
	}
	
	t.Log("Stream rate limiter working correctly")
}

func TestStreamLoggingInterceptor(t *testing.T) {
	service := &TestStreamingService{}
	interceptor := StreamLoggingInterceptor()
	
	ctx := context.Background()
	mockStream := NewMockServerStream(ctx, nil)
	
	info := &StreamServerInfo{
		FullMethod:     "/StreamingService/ServerSideStream",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	handler := func(srv interface{}, ss ServerStream) error {
		req := &StreamRequest{Filter: "test", Limit: 5}
		return service.ServerSideStream(req, ss)
	}
	
	start := time.Now()
	err := interceptor(service, mockStream, info, handler)
	duration := time.Since(start)
	
	if err != nil {
		t.Errorf("Stream logging interceptor failed: %v", err)
	}
	
	if duration < 400*time.Millisecond {
		t.Error("Expected stream to take at least 400ms")
	}
	
	sentMessages := mockStream.GetSentMessages()
	if len(sentMessages) != 5 {
		t.Errorf("Expected 5 sent messages, got %d", len(sentMessages))
	}
	
	t.Logf("Stream logging interceptor completed in %v", duration)
}

func TestStreamAuthInterceptor(t *testing.T) {
	service := &TestStreamingService{}
	interceptor := StreamAuthInterceptor()
	
	// 認証トークンありのテスト
	ctxWithToken := context.WithValue(context.Background(), "token", "stream_token_123")
	mockStreamAuth := NewMockServerStream(ctxWithToken, nil)
	
	info := &StreamServerInfo{
		FullMethod:     "/StreamingService/ServerSideStream",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	handler := func(srv interface{}, ss ServerStream) error {
		return nil // 認証成功時の処理
	}
	
	err := interceptor(service, mockStreamAuth, info, handler)
	if err != nil {
		t.Errorf("Stream auth interceptor failed with valid token: %v", err)
	}
	
	// 認証トークンなしのテスト
	ctxWithoutToken := context.Background()
	mockStreamNoAuth := NewMockServerStream(ctxWithoutToken, nil)
	
	err = interceptor(service, mockStreamNoAuth, info, handler)
	if err == nil {
		t.Error("Expected stream auth interceptor to fail without token")
	}
	
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("Expected authentication error, got: %v", err)
	}
	
	// Health チェックのスキップテスト
	healthInfo := &StreamServerInfo{
		FullMethod:     "/Health/Check",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	err = interceptor(service, mockStreamNoAuth, healthInfo, handler)
	if err != nil {
		t.Errorf("Expected health check to be skipped, got error: %v", err)
	}
	
	t.Log("Stream authentication interceptor working correctly")
}

func TestStreamMetricsInterceptor(t *testing.T) {
	service := &TestStreamingService{}
	metrics := NewStreamMetrics()
	interceptor := StreamMetricsInterceptor(metrics)
	
	ctx := context.Background()
	mockStream := NewMockServerStream(ctx, nil)
	
	info := &StreamServerInfo{
		FullMethod:     "/StreamingService/ServerSideStream",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	handler := func(srv interface{}, ss ServerStream) error {
		req := &StreamRequest{Filter: "test", Limit: 5}
		return service.ServerSideStream(req, ss)
	}
	
	err := interceptor(service, mockStream, info, handler)
	if err != nil {
		t.Errorf("Stream metrics interceptor failed: %v", err)
	}
	
	result := metrics.GetMetrics()
	completedStreams := result["completed_streams"].(map[string]int64)
	messagesSent := result["messages_sent"].(map[string]int64)
	
	if completedStreams[info.FullMethod] != 1 {
		t.Errorf("Expected 1 completed stream, got %d", completedStreams[info.FullMethod])
	}
	
	if messagesSent[info.FullMethod] != 5 {
		t.Errorf("Expected 5 messages sent, got %d", messagesSent[info.FullMethod])
	}
	
	t.Logf("Stream metrics interceptor collected metrics: %+v", result)
}

func TestStreamRateLimitInterceptor(t *testing.T) {
	service := &TestStreamingService{}
	limiter := NewStreamRateLimiter()
	limiter.SetLimit("/StreamingService/ServerSideStream", 1)
	
	interceptor := StreamRateLimitInterceptor(limiter)
	
	ctx := context.Background()
	mockStream1 := NewMockServerStream(ctx, nil)
	mockStream2 := NewMockServerStream(ctx, nil)
	
	info := &StreamServerInfo{
		FullMethod:     "/StreamingService/ServerSideStream",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	handler := func(srv interface{}, ss ServerStream) error {
		time.Sleep(200 * time.Millisecond) // ストリーム処理時間をシミュレート
		return nil
	}
	
	// 最初のストリームは成功するはず
	err := interceptor(service, mockStream1, info, handler)
	if err != nil {
		t.Errorf("First stream should succeed: %v", err)
	}
	
	// 並行して2つ目のストリームを開始
	done := make(chan error, 1)
	go func() {
		err := interceptor(service, mockStream2, info, handler)
		done <- err
	}()
	
	// 短時間待って2つ目のストリームを開始
	time.Sleep(50 * time.Millisecond)
	err = <-done
	
	if err == nil {
		t.Error("Second concurrent stream should be rate limited")
	}
	
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("Expected rate limit error, got: %v", err)
	}
	
	t.Log("Stream rate limit interceptor working correctly")
}

func TestStreamRecoveryInterceptor(t *testing.T) {
	service := &TestStreamingService{}
	interceptor := StreamRecoveryInterceptor()
	
	ctx := context.Background()
	mockStream := NewMockServerStream(ctx, nil)
	
	info := &StreamServerInfo{
		FullMethod:     "/StreamingService/PanicTest",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	handler := func(srv interface{}, ss ServerStream) error {
		panic("test panic")
	}
	
	err := interceptor(service, mockStream, info, handler)
	if err == nil {
		t.Error("Expected error from panic recovery")
	}
	
	if !strings.Contains(err.Error(), "panic recovered") {
		t.Errorf("Expected panic recovery error, got: %v", err)
	}
	
	t.Log("Stream recovery interceptor caught panic successfully")
}

func TestChainedStreamInterceptors(t *testing.T) {
	service := &TestStreamingService{}
	metrics := NewStreamMetrics()
	limiter := NewStreamRateLimiter()
	
	// 複数のインターセプタを組み合わせ
	chainedInterceptor := ChainStreamServer(
		StreamRecoveryInterceptor(),
		StreamLoggingInterceptor(),
		StreamMetricsInterceptor(metrics),
		StreamRateLimitInterceptor(limiter),
	)
	
	ctx := context.WithValue(context.Background(), "token", "stream_token_123")
	mockStream := NewMockServerStream(ctx, nil)
	
	info := &StreamServerInfo{
		FullMethod:     "/StreamingService/ChainTest",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	handler := func(srv interface{}, ss ServerStream) error {
		req := &StreamRequest{Filter: "test", Limit: 3}
		return service.ServerSideStream(req, ss)
	}
	
	err := chainedInterceptor(service, mockStream, info, handler)
	if err != nil {
		t.Errorf("Chained interceptors failed: %v", err)
	}
	
	// メトリクスが正しく収集されているかチェック
	result := metrics.GetMetrics()
	completedStreams := result["completed_streams"].(map[string]int64)
	
	if completedStreams[info.FullMethod] != 1 {
		t.Errorf("Expected 1 completed stream in metrics, got %d", completedStreams[info.FullMethod])
	}
	
	sentMessages := mockStream.GetSentMessages()
	if len(sentMessages) != 5 { // ServerSideStream sends 5 messages
		t.Errorf("Expected 5 sent messages, got %d", len(sentMessages))
	}
	
	t.Log("All chained interceptors executed successfully")
}

// ベンチマークテスト

func BenchmarkWrappedServerStream(b *testing.B) {
	ctx := context.Background()
	mockStream := NewMockServerStream(ctx, nil)
	wrapped := NewWrappedServerStream(mockStream)
	
	msg := &StreamMessage{
		ID:      "benchmark",
		Content: "Benchmark message",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wrapped.SendMsg(msg)
	}
}

func BenchmarkStreamInterceptorChain(b *testing.B) {
	service := &TestStreamingService{}
	metrics := NewStreamMetrics()
	limiter := NewStreamRateLimiter()
	
	chainedInterceptor := ChainStreamServer(
		StreamLoggingInterceptor(),
		StreamMetricsInterceptor(metrics),
		StreamRateLimitInterceptor(limiter),
	)
	
	ctx := context.WithValue(context.Background(), "token", "stream_token_123")
	
	info := &StreamServerInfo{
		FullMethod:     "/StreamingService/BenchmarkTest",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	handler := func(srv interface{}, ss ServerStream) error {
		return ss.SendMsg(&StreamMessage{ID: "bench", Content: "test"})
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockStream := NewMockServerStream(ctx, nil)
		chainedInterceptor(service, mockStream, info, handler)
	}
}