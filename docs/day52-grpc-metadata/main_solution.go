// Day 52: gRPC Metadata
// リクエストIDやトレース情報をgRPCメタデータで効率的に伝播する実装

package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"
)

// メタデータキー定数
const (
	RequestIDKey     = "request-id"
	TraceIDKey       = "trace-id"
	SpanIDKey        = "span-id"
	ParentSpanIDKey  = "parent-span-id"
	UserIDKey        = "user-id"
	AuthorizationKey = "authorization"
	ClientVersionKey = "client-version"
	ServerIDKey      = "server-id"
	TimestampKey     = "timestamp"
)

// MetadataManager メタデータの統合管理
type MetadataManager struct {
	propagators []MetadataPropagator
	filters     []MetadataFilter
	validators  []MetadataValidator
	mu          sync.RWMutex
}

// インターフェース定義
type MetadataPropagator interface {
	Propagate(ctx context.Context, md MockMetadata) (context.Context, MockMetadata, error)
}

type MetadataFilter interface {
	Filter(md MockMetadata) MockMetadata
}

type MetadataValidator interface {
	Validate(ctx context.Context, md MockMetadata) error
}

// NewMetadataManager メタデータマネージャーを作成
func NewMetadataManager() *MetadataManager {
	return &MetadataManager{
		propagators: make([]MetadataPropagator, 0),
		filters:     make([]MetadataFilter, 0),
		validators:  make([]MetadataValidator, 0),
	}
}

// AddPropagator プロパゲーターを追加
func (m *MetadataManager) AddPropagator(propagator MetadataPropagator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.propagators = append(m.propagators, propagator)
}

// AddFilter フィルターを追加
func (m *MetadataManager) AddFilter(filter MetadataFilter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filters = append(m.filters, filter)
}

// AddValidator バリデーターを追加
func (m *MetadataManager) AddValidator(validator MetadataValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validators = append(m.validators, validator)
}

// ProcessIncoming 受信メタデータを処理
func (m *MetadataManager) ProcessIncoming(ctx context.Context, md MockMetadata) (context.Context, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// メタデータのバリデーション
	for _, validator := range m.validators {
		if err := validator.Validate(ctx, md); err != nil {
			return ctx, fmt.Errorf("metadata validation failed: %w", err)
		}
	}

	// メタデータのフィルタリング
	filteredMD := md.Copy()
	for _, filter := range m.filters {
		filteredMD = filter.Filter(filteredMD)
	}

	// メタデータの伝播処理
	currentCtx := ctx
	currentMD := filteredMD
	
	for _, propagator := range m.propagators {
		var err error
		currentCtx, currentMD, err = propagator.Propagate(currentCtx, currentMD)
		if err != nil {
			return ctx, fmt.Errorf("metadata propagation failed: %w", err)
		}
	}

	return currentCtx, nil
}

// ProcessOutgoing 送信メタデータを処理
func (m *MetadataManager) ProcessOutgoing(ctx context.Context) (context.Context, error) {
	outgoingMD := make(MockMetadata)
	
	// コンテキストから必要な情報を取得して送信メタデータに設定
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		outgoingMD.Set(RequestIDKey, requestID)
	}
	
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		outgoingMD.Set(TraceIDKey, traceID)
	}
	
	if spanID, ok := ctx.Value(SpanIDKey).(string); ok && spanID != "" {
		outgoingMD.Set(SpanIDKey, spanID)
	}

	// タイムスタンプを追加
	outgoingMD.Set(TimestampKey, fmt.Sprintf("%d", time.Now().Unix()))
	
	// プロパゲーターを適用
	currentCtx := ctx
	currentMD := outgoingMD
	
	m.mu.RLock()
	for _, propagator := range m.propagators {
		var err error
		currentCtx, currentMD, err = propagator.Propagate(currentCtx, currentMD)
		if err != nil {
			m.mu.RUnlock()
			return ctx, fmt.Errorf("outgoing metadata propagation failed: %w", err)
		}
	}
	m.mu.RUnlock()

	return currentCtx, nil
}

// UnaryServerInterceptor Unaryインターセプター
func (m *MetadataManager) UnaryServerInterceptor() UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		// 受信メタデータの処理（実際の実装ではgrpc.metadataを使用）
		incomingMD := make(MockMetadata)
		processedCtx, err := m.ProcessIncoming(ctx, incomingMD)
		if err != nil {
			log.Printf("Failed to process incoming metadata: %v", err)
			return nil, err
		}

		// ハンドラを実行
		response, err := handler(processedCtx, req)
		
		// 送信メタデータの処理
		outgoingCtx, outErr := m.ProcessOutgoing(processedCtx)
		if outErr != nil {
			log.Printf("Failed to process outgoing metadata: %v", outErr)
		}

		// レスポンスヘッダーを設定（実際の実装では grpc.SetHeader を使用）
		_ = outgoingCtx

		return response, err
	}
}

// StreamServerInterceptor Streamインターセプター
func (m *MetadataManager) StreamServerInterceptor() StreamServerInterceptor {
	return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		ctx := ss.Context()
		
		// 受信メタデータの処理
		incomingMD := make(MockMetadata)
		processedCtx, err := m.ProcessIncoming(ctx, incomingMD)
		if err != nil {
			log.Printf("Failed to process incoming metadata: %v", err)
			return err
		}

		// ラップしたストリームを作成
		wrappedStream := &MetadataAwareServerStream{
			ServerStream: ss,
			ctx:          processedCtx,
			manager:      m,
		}

		return handler(srv, wrappedStream)
	}
}

// MetadataAwareServerStream メタデータ対応のサーバーストリーム
type MetadataAwareServerStream struct {
	ServerStream
	ctx     context.Context
	manager *MetadataManager
}

func (s *MetadataAwareServerStream) Context() context.Context {
	return s.ctx
}

// RequestIDPropagator リクエストIDの生成と伝播
type RequestIDPropagator struct{}

func NewRequestIDPropagator() *RequestIDPropagator {
	return &RequestIDPropagator{}
}

func (p *RequestIDPropagator) Propagate(ctx context.Context, md MockMetadata) (context.Context, MockMetadata, error) {
	requestID := getMetadataValue(md, RequestIDKey)
	if requestID == "" {
		requestID = generateRequestID()
		md.Set(RequestIDKey, requestID)
	}

	// コンテキストにリクエストIDを設定
	ctx = context.WithValue(ctx, RequestIDKey, requestID)
	
	log.Printf("Request ID propagated: %s", requestID)
	return ctx, md, nil
}

// TracePropagator 分散トレーシング情報の伝播
type TracePropagator struct{}

func NewTracePropagator() *TracePropagator {
	return &TracePropagator{}
}

func (p *TracePropagator) Propagate(ctx context.Context, md MockMetadata) (context.Context, MockMetadata, error) {
	traceID := getMetadataValue(md, TraceIDKey)
	spanID := getMetadataValue(md, SpanIDKey)
	parentSpanID := getMetadataValue(md, ParentSpanIDKey)

	// 新しいトレースの場合
	if traceID == "" {
		traceID = generateTraceID()
		md.Set(TraceIDKey, traceID)
	}

	// 新しいスパンを生成
	newSpanID := generateSpanID()
	if spanID != "" {
		md.Set(ParentSpanIDKey, spanID)
	}
	md.Set(SpanIDKey, newSpanID)

	// コンテキストに設定
	ctx = context.WithValue(ctx, TraceIDKey, traceID)
	ctx = context.WithValue(ctx, SpanIDKey, newSpanID)
	if parentSpanID != "" {
		ctx = context.WithValue(ctx, ParentSpanIDKey, parentSpanID)
	}

	log.Printf("Trace propagated: trace=%s, span=%s, parent=%s", traceID, newSpanID, parentSpanID)
	return ctx, md, nil
}

// AuthMetadataValidator 認証メタデータの検証
type AuthMetadataValidator struct {
	skipPaths map[string]bool
}

func NewAuthMetadataValidator() *AuthMetadataValidator {
	return &AuthMetadataValidator{
		skipPaths: map[string]bool{
			"/Health/Check": true,
			"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo": true,
		},
	}
}

func (v *AuthMetadataValidator) AddSkipPath(path string) {
	v.skipPaths[path] = true
}

func (v *AuthMetadataValidator) Validate(ctx context.Context, md MockMetadata) error {
	// パスのチェック（実際の実装では info.FullMethod を使用）
	if path, ok := ctx.Value("method").(string); ok {
		if v.skipPaths[path] {
			return nil
		}
	}

	authHeader := getMetadataValue(md, AuthorizationKey)
	if authHeader == "" {
		return fmt.Errorf("authorization header required")
	}

	token := extractBearerToken(authHeader)
	if token == "" {
		return fmt.Errorf("bearer token required")
	}

	userID, err := validateToken(token)
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	log.Printf("Authentication successful for user: %s", userID)
	return nil
}

// MetadataSecurityFilter 機密情報のフィルタリング
type MetadataSecurityFilter struct {
	sensitiveKeys map[string]bool
	allowedKeys   map[string]bool
}

func NewMetadataSecurityFilter() *MetadataSecurityFilter {
	return &MetadataSecurityFilter{
		sensitiveKeys: map[string]bool{
			"password":          true,
			"secret":           true,
			"private-key":      true,
			"internal-token":   true,
		},
		allowedKeys: map[string]bool{
			RequestIDKey:     true,
			TraceIDKey:       true,
			SpanIDKey:        true,
			ParentSpanIDKey:  true,
			UserIDKey:        true,
			AuthorizationKey: true,
			ClientVersionKey: true,
			ServerIDKey:      true,
			TimestampKey:     true,
		},
	}
}

func (f *MetadataSecurityFilter) AddSensitiveKey(key string) {
	f.sensitiveKeys[key] = true
}

func (f *MetadataSecurityFilter) AddAllowedKey(key string) {
	f.allowedKeys[key] = true
}

func (f *MetadataSecurityFilter) Filter(md MockMetadata) MockMetadata {
	filtered := make(MockMetadata)

	for key, values := range md {
		// 機密情報をフィルタリング
		if f.sensitiveKeys[key] {
			log.Printf("Filtered sensitive metadata key: %s", key)
			continue
		}

		// 許可されたキーのみを通す
		if f.allowedKeys[key] {
			filtered[key] = values
		} else {
			log.Printf("Filtered disallowed metadata key: %s", key)
		}
	}

	return filtered
}

// MetadataChain 複数のメタデータ処理を連鎖実行
type MetadataChain struct {
	processors []MetadataProcessor
}

type MetadataProcessor interface {
	Process(ctx context.Context, md MockMetadata) (context.Context, MockMetadata, error)
}

func NewMetadataChain() *MetadataChain {
	return &MetadataChain{
		processors: make([]MetadataProcessor, 0),
	}
}

func (c *MetadataChain) AddProcessor(processor MetadataProcessor) {
	c.processors = append(c.processors, processor)
}

func (c *MetadataChain) Process(ctx context.Context, md MockMetadata) (context.Context, MockMetadata, error) {
	currentCtx := ctx
	currentMD := md.Copy()

	for _, processor := range c.processors {
		var err error
		currentCtx, currentMD, err = processor.Process(currentCtx, currentMD)
		if err != nil {
			return ctx, md, fmt.Errorf("metadata chain processing failed: %w", err)
		}
	}

	return currentCtx, currentMD, nil
}

// MetadataAwareClient メタデータを自動注入するクライアント
type MetadataAwareClient struct {
	defaultMetadata MockMetadata
	manager         *MetadataManager
	mu              sync.RWMutex
}

func NewMetadataAwareClient(manager *MetadataManager) *MetadataAwareClient {
	return &MetadataAwareClient{
		defaultMetadata: make(MockMetadata),
		manager:         manager,
	}
}

func (c *MetadataAwareClient) SetDefaultMetadata(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultMetadata.Set(key, value)
}

func (c *MetadataAwareClient) PrepareContext(ctx context.Context) (context.Context, error) {
	c.mu.RLock()
	defaultMD := c.defaultMetadata.Copy()
	c.mu.RUnlock()

	// デフォルトメタデータをマージ
	md := make(MockMetadata)
	for key, values := range defaultMD {
		md[key] = values
	}

	// リクエストIDを自動生成
	if getMetadataValue(md, RequestIDKey) == "" {
		md.Set(RequestIDKey, generateRequestID())
	}

	// タイムスタンプを追加
	md.Set(TimestampKey, fmt.Sprintf("%d", time.Now().Unix()))

	// メタデータマネージャーで処理
	return c.manager.ProcessOutgoing(ctx)
}

// ユーティリティ関数

func getMetadataValue(md MockMetadata, key string) string {
	values := md.Get(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func generateRequestID() string {
	timestamp := time.Now().UnixNano()
	random, _ := rand.Int(rand.Reader, big.NewInt(10000))
	return fmt.Sprintf("req-%d-%d", timestamp, random.Int64())
}

func generateTraceID() string {
	timestamp := time.Now().UnixNano()
	random, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("trace-%d-%d", timestamp, random.Int64())
}

func generateSpanID() string {
	timestamp := time.Now().UnixNano()
	random, _ := rand.Int(rand.Reader, big.NewInt(100000))
	return fmt.Sprintf("span-%d-%d", timestamp, random.Int64())
}

func extractBearerToken(authHeader string) string {
	const bearerPrefix = "Bearer "
	if strings.HasPrefix(authHeader, bearerPrefix) {
		return strings.TrimSpace(authHeader[len(bearerPrefix):])
	}
	return ""
}

func validateToken(token string) (string, error) {
	// 簡単なトークン検証（実際の実装では JWT 検証など）
	validTokens := map[string]string{
		"token123": "user1",
		"token456": "user2",
		"admintoken": "admin",
		"testtoken": "testuser",
	}

	if userID, exists := validTokens[token]; exists {
		return userID, nil
	}

	return "", fmt.Errorf("invalid token")
}

// MockMetadata メタデータのモック実装
type MockMetadata map[string][]string

func (m MockMetadata) Get(key string) []string {
	if values, exists := m[key]; exists {
		return values
	}
	return []string{}
}

func (m MockMetadata) Set(key, value string) {
	m[key] = []string{value}
}

func (m MockMetadata) Append(key string, values ...string) {
	if existing, exists := m[key]; exists {
		m[key] = append(existing, values...)
	} else {
		m[key] = values
	}
}

func (m MockMetadata) Delete(key string) {
	delete(m, key)
}

func (m MockMetadata) Copy() MockMetadata {
	copy := make(MockMetadata)
	for key, values := range m {
		valueCopy := make([]string, len(values))
		for i, value := range values {
			valueCopy[i] = value
		}
		copy[key] = valueCopy
	}
	return copy
}

func (m MockMetadata) Len() int {
	return len(m)
}

func (m MockMetadata) Keys() []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// インターセプタ型定義
type UnaryServerInterceptor func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error)
type StreamServerInterceptor func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error

type UnaryServerInfo struct {
	FullMethod string
}

type StreamServerInfo struct {
	FullMethod     string
	IsClientStream bool
	IsServerStream bool
}

type UnaryHandler func(ctx context.Context, req interface{}) (interface{}, error)
type StreamHandler func(srv interface{}, stream ServerStream) error

type ServerStream interface {
	Context() context.Context
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

func main() {
	fmt.Println("Day 52: gRPC Metadata")
	fmt.Println("Run 'go test -v' to see the metadata system in action")
}