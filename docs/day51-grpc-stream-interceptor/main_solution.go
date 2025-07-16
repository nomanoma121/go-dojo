// Day 51: gRPC Stream Interceptor
// 全てのStream RPCで共通の処理（ログ、認証）を挟み込む実装

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"
)

// ストリーム情報
type StreamServerInfo struct {
	FullMethod     string
	IsClientStream bool
	IsServerStream bool
}

// インターセプタ関数型定義
type StreamHandler func(srv interface{}, stream ServerStream) error
type StreamServerInterceptor func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error

// ServerStreamインターフェース
type ServerStream interface {
	SetHeader(map[string]string) error
	SendHeader(map[string]string) error
	SetTrailer(map[string]string)
	Context() context.Context
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

// ベースServerStream実装
type BaseServerStream struct {
	ctx context.Context
}

func (s *BaseServerStream) SetHeader(md map[string]string) error { return nil }
func (s *BaseServerStream) SendHeader(md map[string]string) error { return nil }
func (s *BaseServerStream) SetTrailer(md map[string]string) {}
func (s *BaseServerStream) Context() context.Context { return s.ctx }
func (s *BaseServerStream) SendMsg(m interface{}) error { return nil }
func (s *BaseServerStream) RecvMsg(m interface{}) error { return io.EOF }

// ラップしたServerStream
type WrappedServerStream struct {
	ServerStream
	sentCount     int64
	recvCount     int64
	startTime     time.Time
	lastActivity  time.Time
	mu            sync.RWMutex
}

func NewWrappedServerStream(stream ServerStream) *WrappedServerStream {
	now := time.Now()
	return &WrappedServerStream{
		ServerStream: stream,
		startTime:    now,
		lastActivity: now,
	}
}

func (w *WrappedServerStream) SendMsg(m interface{}) error {
	w.mu.Lock()
	w.sentCount++
	w.lastActivity = time.Now()
	w.mu.Unlock()
	
	return w.ServerStream.SendMsg(m)
}

func (w *WrappedServerStream) RecvMsg(m interface{}) error {
	w.mu.Lock()
	w.recvCount++
	w.lastActivity = time.Now()
	w.mu.Unlock()
	
	return w.ServerStream.RecvMsg(m)
}

func (w *WrappedServerStream) GetStats() (sent, recv int64, duration time.Duration) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sentCount, w.recvCount, time.Since(w.startTime)
}

// ストリームメトリクス
type StreamMetrics struct {
	ActiveStreams    map[string]int64
	CompletedStreams map[string]int64
	MessagesSent     map[string]int64
	MessagesReceived map[string]int64
	StreamDurations  map[string][]time.Duration
	mu               sync.RWMutex
}

func NewStreamMetrics() *StreamMetrics {
	return &StreamMetrics{
		ActiveStreams:    make(map[string]int64),
		CompletedStreams: make(map[string]int64),
		MessagesSent:     make(map[string]int64),
		MessagesReceived: make(map[string]int64),
		StreamDurations:  make(map[string][]time.Duration),
	}
}

func (m *StreamMetrics) StartStream(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveStreams[method]++
}

func (m *StreamMetrics) EndStream(method string, sent, recv int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.ActiveStreams[method]--
	m.CompletedStreams[method]++
	m.MessagesSent[method] += sent
	m.MessagesReceived[method] += recv
	m.StreamDurations[method] = append(m.StreamDurations[method], duration)
}

func (m *StreamMetrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]interface{})
	result["active_streams"] = copyInt64Map(m.ActiveStreams)
	result["completed_streams"] = copyInt64Map(m.CompletedStreams)
	result["messages_sent"] = copyInt64Map(m.MessagesSent)
	result["messages_received"] = copyInt64Map(m.MessagesReceived)
	
	// 平均ストリーム持続時間を計算
	avgDurations := make(map[string]time.Duration)
	for method, durations := range m.StreamDurations {
		if len(durations) > 0 {
			var total time.Duration
			for _, d := range durations {
				total += d
			}
			avgDurations[method] = total / time.Duration(len(durations))
		}
	}
	result["avg_stream_duration"] = avgDurations
	
	return result
}

func copyInt64Map(m map[string]int64) map[string]int64 {
	result := make(map[string]int64)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// ストリームレート制限
type StreamRateLimiter struct {
	activeStreams map[string]int
	maxStreams    map[string]int
	mu            sync.RWMutex
}

func NewStreamRateLimiter() *StreamRateLimiter {
	return &StreamRateLimiter{
		activeStreams: make(map[string]int),
		maxStreams: map[string]int{
			"default": 10,
		},
	}
}

func (srl *StreamRateLimiter) SetLimit(method string, limit int) {
	srl.mu.Lock()
	defer srl.mu.Unlock()
	srl.maxStreams[method] = limit
}

func (srl *StreamRateLimiter) CanStartStream(method string) bool {
	srl.mu.Lock()
	defer srl.mu.Unlock()
	
	limit, exists := srl.maxStreams[method]
	if !exists {
		limit = srl.maxStreams["default"]
	}
	
	current := srl.activeStreams[method]
	return current < limit
}

func (srl *StreamRateLimiter) StartStream(method string) {
	srl.mu.Lock()
	defer srl.mu.Unlock()
	srl.activeStreams[method]++
}

func (srl *StreamRateLimiter) EndStream(method string) {
	srl.mu.Lock()
	defer srl.mu.Unlock()
	if srl.activeStreams[method] > 0 {
		srl.activeStreams[method]--
	}
}

// インターセプタ実装

// StreamLoggingInterceptor ストリームログインターセプタ
func StreamLoggingInterceptor() StreamServerInterceptor {
	return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		_ = time.Now()
		
		log.Printf("[STREAM START] Method: %s, Type: client=%t server=%t", 
			info.FullMethod, info.IsClientStream, info.IsServerStream)
		
		// ラップしたストリームを作成
		wrappedStream := NewWrappedServerStream(ss)
		
		// ハンドラを実行
		err := handler(srv, wrappedStream)
		
		// 統計情報を取得
		sent, recv, duration := wrappedStream.GetStats()
		status := "SUCCESS"
		if err != nil {
			status = "ERROR"
		}
		
		log.Printf("[STREAM END] Method: %s, Duration: %v, Sent: %d, Recv: %d, Status: %s, Error: %v", 
			info.FullMethod, duration, sent, recv, status, err)
		
		return err
	}
}

// StreamAuthInterceptor ストリーム認証インターセプタ
func StreamAuthInterceptor() StreamServerInterceptor {
	return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		// Health チェックはスキップ
		if strings.Contains(info.FullMethod, "Health") {
			return handler(srv, ss)
		}
		
		// コンテキストからトークンを取得
		ctx := ss.Context()
		token := extractTokenFromContext(ctx)
		
		if token == "" {
			return fmt.Errorf("stream authentication required")
		}
		
		// トークンを検証
		_, err := validateStreamToken(token)
		if err != nil {
			return fmt.Errorf("stream authentication failed: %w", err)
		}
		
		return handler(srv, ss)
	}
}

// StreamMetricsInterceptor ストリームメトリクスインターセプタ
func StreamMetricsInterceptor(metrics *StreamMetrics) StreamServerInterceptor {
	return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		// ストリーム開始
		metrics.StartStream(info.FullMethod)
		
		// ラップしたストリームを作成
		wrappedStream := NewWrappedServerStream(ss)
		
		// ハンドラを実行
		err := handler(srv, wrappedStream)
		
		// メトリクスを記録
		sent, recv, duration := wrappedStream.GetStats()
		metrics.EndStream(info.FullMethod, sent, recv, duration)
		
		return err
	}
}

// StreamRateLimitInterceptor ストリームレート制限インターセプタ
func StreamRateLimitInterceptor(limiter *StreamRateLimiter) StreamServerInterceptor {
	return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		// レート制限チェック
		if !limiter.CanStartStream(info.FullMethod) {
			return fmt.Errorf("stream rate limit exceeded for method: %s", info.FullMethod)
		}
		
		// ストリーム開始
		limiter.StartStream(info.FullMethod)
		defer limiter.EndStream(info.FullMethod)
		
		return handler(srv, ss)
	}
}

// StreamRecoveryInterceptor ストリーム回復インターセプタ
func StreamRecoveryInterceptor() StreamServerInterceptor {
	return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[STREAM PANIC RECOVERY] Method: %s, Panic: %v", info.FullMethod, r)
				err = fmt.Errorf("stream internal error: panic recovered")
			}
		}()
		
		return handler(srv, ss)
	}
}

// ChainStreamServer 複数のストリームインターセプタを連鎖
func ChainStreamServer(interceptors ...StreamServerInterceptor) StreamServerInterceptor {
	switch len(interceptors) {
	case 0:
		return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
			return handler(srv, ss)
		}
	case 1:
		return interceptors[0]
	default:
		return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
			chainerHandler := func(currentSrv interface{}, currentStream ServerStream) error {
				return ChainStreamServer(interceptors[1:]...)(currentSrv, currentStream, info, handler)
			}
			return interceptors[0](srv, ss, info, chainerHandler)
		}
	}
}

// ユーティリティ関数
func extractTokenFromContext(ctx context.Context) string {
	if token, ok := ctx.Value("token").(string); ok {
		return token
	}
	return ""
}

func validateStreamToken(token string) (string, error) {
	validTokens := map[string]string{
		"stream_token_123": "user1",
		"stream_token_456": "user2",
		"admin_stream":     "admin",
	}
	
	if userID, exists := validTokens[token]; exists {
		return userID, nil
	}
	
	return "", fmt.Errorf("invalid stream token")
}

// テスト用のストリーミングサービス
type StreamingService struct {
	data []*StreamMessage
	mu   sync.RWMutex
}

type StreamMessage struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Timestamp int64 `json:"timestamp"`
}

func NewStreamingService() *StreamingService {
	return &StreamingService{
		data: make([]*StreamMessage, 0),
	}
}

// 模擬ストリーミングメソッド
func (s *StreamingService) ServerSideStream(req *StreamRequest, stream ServerStream) error {
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

func (s *StreamingService) ClientSideStream(stream ServerStream) (*StreamResponse, error) {
	var messages []*StreamMessage
	
	for {
		var msg StreamMessage
		err := stream.RecvMsg(&msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}
	
	return &StreamResponse{
		Count:   int32(len(messages)),
		Status:  "SUCCESS",
		Summary: fmt.Sprintf("Received %d messages", len(messages)),
	}, nil
}

type StreamRequest struct {
	Filter string `json:"filter"`
	Limit  int32  `json:"limit"`
}

type StreamResponse struct {
	Count   int32  `json:"count"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

// インターセプタ付きサーバー
type InterceptorStreamServer struct {
	service     *StreamingService
	interceptor StreamServerInterceptor
}

func NewInterceptorStreamServer(service *StreamingService, interceptor StreamServerInterceptor) *InterceptorStreamServer {
	return &InterceptorStreamServer{
		service:     service,
		interceptor: interceptor,
	}
}

func (s *InterceptorStreamServer) ServerSideStream(req *StreamRequest, stream ServerStream) error {
	info := &StreamServerInfo{
		FullMethod:     "/StreamingService/ServerSideStream",
		IsClientStream: false,
		IsServerStream: true,
	}
	
	handler := func(srv interface{}, ss ServerStream) error {
		return s.service.ServerSideStream(req, ss)
	}
	
	return s.interceptor(s.service, stream, info, handler)
}

func (s *InterceptorStreamServer) ClientSideStream(stream ServerStream) (*StreamResponse, error) {
	info := &StreamServerInfo{
		FullMethod:     "/StreamingService/ClientSideStream",
		IsClientStream: true,
		IsServerStream: false,
	}
	
	var result *StreamResponse
	var resultErr error
	
	handler := func(srv interface{}, ss ServerStream) error {
		resp, err := s.service.ClientSideStream(ss)
		result = resp
		resultErr = err
		return err
	}
	
	err := s.interceptor(s.service, stream, info, handler)
	if err != nil {
		return nil, err
	}
	
	return result, resultErr
}

func main() {
	fmt.Println("Day 51: gRPC Stream Interceptor")
	fmt.Println("Run 'go test -v' to see the stream interceptor system in action")
}