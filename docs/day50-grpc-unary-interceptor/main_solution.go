// Day 50: gRPC Unary Interceptor
// 全てのUnary RPCで共通の処理（ログ、認証）を挟み込む実装

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// リクエスト/レスポンス構造
type UserRequest struct {
	UserID string `json:"user_id"`
}

type UserResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Status   string `json:"status"`
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// メタデータとコンテキスト
type ServerInfo struct {
	FullMethod string
	Server     interface{}
}

type CallInfo struct {
	FullMethod string
}

// インターセプタ関数型定義
type UnaryHandler func(ctx context.Context, req interface{}) (interface{}, error)
type UnaryServerInterceptor func(ctx context.Context, req interface{}, info *ServerInfo, handler UnaryHandler) (interface{}, error)
type UnaryClientInterceptor func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker, opts ...CallOption) error
type UnaryInvoker func(ctx context.Context, method string, req, reply interface{}, opts ...CallOption) error

type CallOption interface{}

// コンテキストキー
type contextKey string

const (
	clientIDKey = contextKey("clientID")
	tokenKey    = contextKey("token")
	userIDKey   = contextKey("userID")
)

// メトリクス構造
type Metrics struct {
	RequestCount    map[string]int64
	ErrorCount      map[string]int64
	ResponseTime    map[string][]time.Duration
	ActiveRequests  map[string]int64
	mu              sync.RWMutex
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestCount:   make(map[string]int64),
		ErrorCount:     make(map[string]int64),
		ResponseTime:   make(map[string][]time.Duration),
		ActiveRequests: make(map[string]int64),
	}
}

// RecordRequest メトリクスにリクエストを記録
func (m *Metrics) RecordRequest(method string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RequestCount[method]++
	m.ResponseTime[method] = append(m.ResponseTime[method], duration)

	if err != nil {
		m.ErrorCount[method]++
	}
}

// GetMetrics 現在のメトリクス情報を返す
func (m *Metrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]interface{})

	// リクエスト数
	requestCount := make(map[string]int64)
	for method, count := range m.RequestCount {
		requestCount[method] = count
	}
	result["request_count"] = requestCount

	// エラー数
	errorCount := make(map[string]int64)
	for method, count := range m.ErrorCount {
		errorCount[method] = count
	}
	result["error_count"] = errorCount

	// 平均レスポンス時間
	avgResponseTime := make(map[string]time.Duration)
	for method, times := range m.ResponseTime {
		if len(times) > 0 {
			var total time.Duration
			for _, t := range times {
				total += t
			}
			avgResponseTime[method] = total / time.Duration(len(times))
		}
	}
	result["avg_response_time"] = avgResponseTime

	return result
}

// レート制限構造
type RateLimiter struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	mu       sync.RWMutex
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// IsAllowed レート制限チェックを行う
func (r *RateLimiter) IsAllowed(clientID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// 古いリクエストを削除
	if requests, exists := r.requests[clientID]; exists {
		var validRequests []time.Time
		for _, reqTime := range requests {
			if reqTime.After(cutoff) {
				validRequests = append(validRequests, reqTime)
			}
		}
		r.requests[clientID] = validRequests
	}

	// 現在のリクエスト数をチェック
	currentRequests := len(r.requests[clientID])
	if currentRequests >= r.limit {
		return false
	}

	// 新しいリクエストを記録
	r.requests[clientID] = append(r.requests[clientID], now)
	return true
}

// サービス実装
type UserService struct {
	users map[string]*UserResponse
	mu    sync.RWMutex
}

func NewUserService() *UserService {
	service := &UserService{
		users: make(map[string]*UserResponse),
	}
	
	// テストユーザーを追加
	service.users["user1"] = &UserResponse{
		ID:     "user1",
		Name:   "John Doe",
		Email:  "john@example.com",
		Status: "active",
	}
	
	return service
}

// GetUser ユーザーを取得
func (s *UserService) GetUser(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[req.UserID]
	if !exists {
		return nil, fmt.Errorf("user not found: %s", req.UserID)
	}

	return user, nil
}

// CreateUser ユーザーを作成
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*UserResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID := fmt.Sprintf("user_%d", time.Now().Unix())
	user := &UserResponse{
		ID:     userID,
		Name:   req.Name,
		Email:  req.Email,
		Status: "active",
	}

	s.users[userID] = user
	return user, nil
}

// LoggingInterceptor リクエスト/レスポンスの詳細ログを出力するインターセプタ
func LoggingInterceptor() UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *ServerInfo, handler UnaryHandler) (interface{}, error) {
		start := time.Now()

		// リクエストログ
		log.Printf("[REQUEST] Method: %s, Request: %+v", info.FullMethod, req)

		// 実際のハンドラを実行
		resp, err := handler(ctx, req)

		// レスポンスログ
		duration := time.Since(start)
		status := "SUCCESS"
		if err != nil {
			status = "ERROR"
		}

		log.Printf("[RESPONSE] Method: %s, Duration: %v, Status: %s, Error: %v", 
			info.FullMethod, duration, status, err)

		return resp, err
	}
}

// AuthInterceptor 認証トークンを検証するインターセプタ
func AuthInterceptor() UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *ServerInfo, handler UnaryHandler) (interface{}, error) {
		// 認証が不要なメソッドをスキップ
		if strings.Contains(info.FullMethod, "Health") {
			return handler(ctx, req)
		}

		// トークンを抽出
		token := extractToken(ctx)
		if token == "" {
			return nil, fmt.Errorf("authentication token required")
		}

		// トークンを検証
		userID, err := validateToken(token)
		if err != nil {
			return nil, fmt.Errorf("invalid token: %w", err)
		}

		// ユーザーIDをコンテキストに追加
		ctx = context.WithValue(ctx, userIDKey, userID)

		return handler(ctx, req)
	}
}

// MetricsInterceptor メトリクス収集を行うインターセプタ
func MetricsInterceptor(metrics *Metrics) UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *ServerInfo, handler UnaryHandler) (interface{}, error) {
		start := time.Now()

		// アクティブリクエストを増加
		metrics.mu.Lock()
		metrics.ActiveRequests[info.FullMethod]++
		metrics.mu.Unlock()

		// ハンドラを実行
		resp, err := handler(ctx, req)

		// メトリクスを記録
		duration := time.Since(start)
		metrics.RecordRequest(info.FullMethod, duration, err)

		// アクティブリクエストを減少
		metrics.mu.Lock()
		metrics.ActiveRequests[info.FullMethod]--
		metrics.mu.Unlock()

		return resp, err
	}
}

// RateLimitInterceptor レート制限を行うインターセプタ
func RateLimitInterceptor(limiter *RateLimiter) UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *ServerInfo, handler UnaryHandler) (interface{}, error) {
		clientID := extractClientID(ctx)
		if clientID == "" {
			clientID = "unknown"
		}

		if !limiter.IsAllowed(clientID) {
			return nil, fmt.Errorf("rate limit exceeded for client: %s", clientID)
		}

		return handler(ctx, req)
	}
}

// RecoveryInterceptor パニックから回復するインターセプタ
func RecoveryInterceptor() UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *ServerInfo, handler UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC RECOVERY] Method: %s, Panic: %v", info.FullMethod, r)
				err = fmt.Errorf("internal server error: panic recovered")
			}
		}()

		return handler(ctx, req)
	}
}

// ChainUnaryServer 複数のインターセプタを連鎖させる関数
func ChainUnaryServer(interceptors ...UnaryServerInterceptor) UnaryServerInterceptor {
	switch len(interceptors) {
	case 0:
		return func(ctx context.Context, req interface{}, info *ServerInfo, handler UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	case 1:
		return interceptors[0]
	default:
		return func(ctx context.Context, req interface{}, info *ServerInfo, handler UnaryHandler) (interface{}, error) {
			chainerHandler := func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
				return ChainUnaryServer(interceptors[1:]...)(currentCtx, currentReq, info, handler)
			}
			return interceptors[0](ctx, req, info, chainerHandler)
		}
	}
}

// ClientLoggingInterceptor クライアントサイドのログインターセプタ
func ClientLoggingInterceptor() UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker, opts ...CallOption) error {
		start := time.Now()

		log.Printf("[CLIENT REQUEST] Method: %s, Request: %+v", method, req)

		err := invoker(ctx, method, req, reply, opts...)

		duration := time.Since(start)
		status := "SUCCESS"
		if err != nil {
			status = "ERROR"
		}

		log.Printf("[CLIENT RESPONSE] Method: %s, Duration: %v, Status: %s, Reply: %+v, Error: %v", 
			method, duration, status, reply, err)

		return err
	}
}

// ClientAuthInterceptor クライアントサイドの認証インターセプタ
func ClientAuthInterceptor(token string) UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker, opts ...CallOption) error {
		// トークンをコンテキストに追加
		ctx = context.WithValue(ctx, tokenKey, token)

		return invoker(ctx, method, req, reply, opts...)
	}
}

// ユーティリティ関数

// extractClientID コンテキストからクライアントIDを抽出
func extractClientID(ctx context.Context) string {
	if clientID, ok := ctx.Value(clientIDKey).(string); ok {
		return clientID
	}
	return ""
}

// extractToken コンテキストから認証トークンを抽出
func extractToken(ctx context.Context) string {
	if token, ok := ctx.Value(tokenKey).(string); ok {
		return token
	}
	return ""
}

// validateToken 認証トークンを検証
func validateToken(token string) (string, error) {
	// 簡単なトークン検証（実際にはJWT検証など）
	validTokens := map[string]string{
		"token123": "user1",
		"token456": "user2",
		"admin":    "admin",
	}

	if userID, exists := validTokens[token]; exists {
		return userID, nil
	}

	return "", fmt.Errorf("invalid token")
}

// インターセプタ付きサーバー
type InterceptorServer struct {
	service     *UserService
	interceptor UnaryServerInterceptor
}

func NewInterceptorServer(service *UserService, interceptor UnaryServerInterceptor) *InterceptorServer {
	return &InterceptorServer{
		service:     service,
		interceptor: interceptor,
	}
}

// GetUser インターセプタを通してサービスメソッドを呼び出し
func (s *InterceptorServer) GetUser(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	info := &ServerInfo{
		FullMethod: "/UserService/GetUser",
		Server:     s.service,
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		userReq := req.(*UserRequest)
		return s.service.GetUser(ctx, userReq)
	}

	resp, err := s.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}

	return resp.(*UserResponse), nil
}

// CreateUser インターセプタを通してサービスメソッドを呼び出し
func (s *InterceptorServer) CreateUser(ctx context.Context, req *CreateUserRequest) (*UserResponse, error) {
	info := &ServerInfo{
		FullMethod: "/UserService/CreateUser",
		Server:     s.service,
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		createReq := req.(*CreateUserRequest)
		return s.service.CreateUser(ctx, createReq)
	}

	resp, err := s.interceptor(ctx, req, info, handler)
	if err != nil {
		return nil, err
	}

	return resp.(*UserResponse), nil
}

// クライアント実装
type InterceptorClient struct {
	server       *InterceptorServer
	interceptors []UnaryClientInterceptor
}

func NewInterceptorClient(server *InterceptorServer, interceptors ...UnaryClientInterceptor) *InterceptorClient {
	return &InterceptorClient{
		server:       server,
		interceptors: interceptors,
	}
}

// GetUser クライアントインターセプタを通してサーバーメソッドを呼び出し
func (c *InterceptorClient) GetUser(ctx context.Context, req *UserRequest) (*UserResponse, error) {
	method := "/UserService/GetUser"
	var reply *UserResponse

	invoker := func(ctx context.Context, method string, req, reply interface{}, opts ...CallOption) error {
		userReq := req.(*UserRequest)
		result, err := c.server.GetUser(ctx, userReq)
		if err != nil {
			return err
		}
		
		// replyに結果をコピー
		if replyPtr, ok := reply.(**UserResponse); ok {
			*replyPtr = result
		}
		return nil
	}

	// インターセプタチェーンを実行
	finalInvoker := invoker
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		interceptor := c.interceptors[i]
		currentInvoker := finalInvoker
		finalInvoker = func(ctx context.Context, method string, req, reply interface{}, opts ...CallOption) error {
			return interceptor(ctx, method, req, reply, currentInvoker, opts...)
		}
	}

	err := finalInvoker(ctx, method, req, &reply)
	return reply, err
}

// CreateUser クライアントインターセプタを通してサーバーメソッドを呼び出し
func (c *InterceptorClient) CreateUser(ctx context.Context, req *CreateUserRequest) (*UserResponse, error) {
	method := "/UserService/CreateUser"
	var reply *UserResponse

	invoker := func(ctx context.Context, method string, req, reply interface{}, opts ...CallOption) error {
		createReq := req.(*CreateUserRequest)
		result, err := c.server.CreateUser(ctx, createReq)
		if err != nil {
			return err
		}
		
		if replyPtr, ok := reply.(**UserResponse); ok {
			*replyPtr = result
		}
		return nil
	}

	// インターセプタチェーンを実行
	finalInvoker := invoker
	for i := len(c.interceptors) - 1; i >= 0; i-- {
		interceptor := c.interceptors[i]
		currentInvoker := finalInvoker
		finalInvoker = func(ctx context.Context, method string, req, reply interface{}, opts ...CallOption) error {
			return interceptor(ctx, method, req, reply, currentInvoker, opts...)
		}
	}

	err := finalInvoker(ctx, method, req, &reply)
	return reply, err
}

func main() {
	fmt.Println("Day 50: gRPC Unary Interceptor")
	fmt.Println("Run 'go test -v' to see the interceptor system in action")
}