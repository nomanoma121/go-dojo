// Day 59: OpenTelemetry Distributed Tracing
// サービスをまたぐリクエストのトレース情報を設定・出力

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 模擬OpenTelemetryトレーシング構造体

// Trace ID とSpan ID
type TraceID string
type SpanID string

// スパンの種類
type SpanKind int

const (
	SpanKindInternal SpanKind = iota
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

// ステータスコード
type StatusCode int

const (
	StatusCodeUnset StatusCode = iota
	StatusCodeOk
	StatusCodeError
)

// 属性
type Attribute struct {
	Key   string
	Value interface{}
}

func StringAttribute(key, value string) Attribute {
	return Attribute{Key: key, Value: value}
}

func IntAttribute(key string, value int) Attribute {
	return Attribute{Key: key, Value: value}
}

func Float64Attribute(key string, value float64) Attribute {
	return Attribute{Key: key, Value: value}
}

func BoolAttribute(key string, value bool) Attribute {
	return Attribute{Key: key, Value: value}
}

// スパン
type Span struct {
	TraceID     TraceID
	SpanID      SpanID
	ParentID    SpanID
	Name        string
	Kind        SpanKind
	StartTime   time.Time
	EndTime     *time.Time
	Status      StatusCode
	StatusMsg   string
	Attributes  []Attribute
	Events      []SpanEvent
	mu          sync.RWMutex
}

type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes []Attribute
}

func (s *Span) SetAttribute(attr Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attributes = append(s.Attributes, attr)
}

func (s *Span) SetAttributes(attrs ...Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attributes = append(s.Attributes, attrs...)
}

func (s *Span) SetStatus(code StatusCode, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = code
	s.StatusMsg = message
}

func (s *Span) RecordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.Events = append(s.Events, SpanEvent{
		Name:      "exception",
		Timestamp: time.Now(),
		Attributes: []Attribute{
			StringAttribute("exception.type", fmt.Sprintf("%T", err)),
			StringAttribute("exception.message", err.Error()),
		},
	})
	
	s.Status = StatusCodeError
	s.StatusMsg = err.Error()
}

func (s *Span) AddEvent(name string, attrs ...Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.Events = append(s.Events, SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	})
}

func (s *Span) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	s.EndTime = &now
	
	// スパンをトレーサーに送信
	if s.SpanID != "" {
		globalTracer.FinishSpan(s)
	}
}

// スパンコンテキスト
type SpanContext struct {
	TraceID TraceID
	SpanID  SpanID
}

func (sc SpanContext) IsValid() bool {
	return sc.TraceID != "" && sc.SpanID != ""
}

// トレーサー
type Tracer struct {
	name     string
	spans    map[SpanID]*Span
	traces   map[TraceID][]*Span
	mu       sync.RWMutex
}

func NewTracer(name string) *Tracer {
	return &Tracer{
		name:   name,
		spans:  make(map[SpanID]*Span),
		traces: make(map[TraceID][]*Span),
	}
}

func (t *Tracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, *Span) {
	// 新しいスパンを作成
	span := &Span{
		Name:       name,
		Kind:       SpanKindInternal,
		StartTime:  time.Now(),
		Attributes: make([]Attribute, 0),
		Events:     make([]SpanEvent, 0),
	}
	
	// オプションを適用
	for _, opt := range opts {
		opt(span)
	}
	
	// 親スパンからTraceIDとParentIDを設定
	if parentSpan := SpanFromContext(ctx); parentSpan != nil {
		span.TraceID = parentSpan.TraceID
		span.ParentID = parentSpan.SpanID
	} else {
		span.TraceID = generateTraceID()
	}
	
	span.SpanID = generateSpanID()
	
	// スパンを登録
	t.mu.Lock()
	t.spans[span.SpanID] = span
	t.traces[span.TraceID] = append(t.traces[span.TraceID], span)
	t.mu.Unlock()
	
	// コンテキストにスパンを設定
	ctx = ContextWithSpan(ctx, span)
	
	return ctx, span
}

func (t *Tracer) FinishSpan(span *Span) {
	// スパン完了時の処理（エクスポートなど）
	log.Printf("[TRACE] Span finished: %s (TraceID: %s, SpanID: %s, Duration: %v)", 
		span.Name, span.TraceID, span.SpanID, 
		span.EndTime.Sub(span.StartTime))
}

func (t *Tracer) GetTrace(traceID TraceID) []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	spans, exists := t.traces[traceID]
	if !exists {
		return nil
	}
	
	result := make([]*Span, len(spans))
	copy(result, spans)
	return result
}

func (t *Tracer) GetAllTraces() map[TraceID][]*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	result := make(map[TraceID][]*Span)
	for traceID, spans := range t.traces {
		spansCopy := make([]*Span, len(spans))
		copy(spansCopy, spans)
		result[traceID] = spansCopy
	}
	
	return result
}

// スパンオプション
type SpanOption func(*Span)

func WithSpanKind(kind SpanKind) SpanOption {
	return func(span *Span) {
		span.Kind = kind
	}
}

func WithAttributes(attrs ...Attribute) SpanOption {
	return func(span *Span) {
		span.Attributes = append(span.Attributes, attrs...)
	}
}

// コンテキスト関連
type spanContextKey struct{}

func ContextWithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, spanContextKey{}, span)
}

func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanContextKey{}).(*Span); ok {
		return span
	}
	return nil
}

// グローバルトレーサー
var globalTracer = NewTracer("global")

func GetTracer(name string) *Tracer {
	return NewTracer(name)
}

// ID生成
func generateTraceID() TraceID {
	return TraceID(fmt.Sprintf("trace_%d_%d", time.Now().UnixNano(), rand.Intn(10000)))
}

func generateSpanID() SpanID {
	return SpanID(fmt.Sprintf("span_%d_%d", time.Now().UnixNano(), rand.Intn(10000)))
}

// HTTPトレーシングミドルウェア
func TracingMiddleware(tracer *Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// トレーシングヘッダーからコンテキストを復元
			ctx := r.Context()
			ctx = extractTraceContext(ctx, r.Header)
			
			// スパンを開始
			ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				WithSpanKind(SpanKindServer),
				WithAttributes(
					StringAttribute("http.method", r.Method),
					StringAttribute("http.url", r.URL.String()),
					StringAttribute("http.scheme", r.URL.Scheme),
					StringAttribute("http.host", r.Host),
					StringAttribute("http.user_agent", r.UserAgent()),
				),
			)
			defer span.End()
			
			// レスポンスライターをラップ
			ww := &tracingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			
			// トレーシングヘッダーをレスポンスに注入
			injectTraceContext(ww.Header(), span)
			
			// リクエストにコンテキストを設定
			r = r.WithContext(ctx)
			
			// 次のハンドラを実行
			next.ServeHTTP(ww, r)
			
			// HTTPレスポンス情報を記録
			span.SetAttributes(
				IntAttribute("http.status_code", ww.statusCode),
				IntAttribute("http.response_size", ww.size),
			)
			
			if ww.statusCode >= 400 {
				span.SetStatus(StatusCodeError, fmt.Sprintf("HTTP %d", ww.statusCode))
			} else {
				span.SetStatus(StatusCodeOk, "")
			}
		})
	}
}

type tracingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (w *tracingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *tracingResponseWriter) Write(b []byte) (int, error) {
	size, err := w.ResponseWriter.Write(b)
	w.size += size
	return size, err
}

// トレースコンテキストの抽出と注入
func extractTraceContext(ctx context.Context, headers http.Header) context.Context {
	traceParent := headers.Get("traceparent")
	if traceParent == "" {
		return ctx
	}
	
	// traceparentヘッダーをパース（簡略化）
	parts := strings.Split(traceParent, "-")
	if len(parts) != 4 {
		return ctx
	}
	
	traceID := TraceID(parts[1])
	spanID := SpanID(parts[2])
	
	// 親スパンを作成
	parentSpan := &Span{
		TraceID: traceID,
		SpanID:  spanID,
	}
	
	return ContextWithSpan(ctx, parentSpan)
}

func injectTraceContext(headers http.Header, span *Span) {
	if span != nil {
		// traceparentヘッダーを設定
		traceParent := fmt.Sprintf("00-%s-%s-01", span.TraceID, span.SpanID)
		headers.Set("traceparent", traceParent)
	}
}

// サービス実装

// ユーザーサービス
type UserService struct {
	tracer *Tracer
	users  map[string]*User
	mu     sync.RWMutex
}

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Status   string `json:"status"`
}

func NewUserService() *UserService {
	service := &UserService{
		tracer: GetTracer("user-service"),
		users:  make(map[string]*User),
	}
	
	// テストユーザーを追加
	service.users["1"] = &User{ID: "1", Name: "John Doe", Email: "john@example.com", Status: "active"}
	service.users["2"] = &User{ID: "2", Name: "Jane Smith", Email: "jane@example.com", Status: "active"}
	
	return service
}

func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
	ctx, span := s.tracer.Start(ctx, "GetUser",
		WithAttributes(
			StringAttribute("user.id", userID),
			StringAttribute("service.name", "user-service"),
		),
	)
	defer span.End()
	
	// データベースアクセスをシミュレート
	ctx, dbSpan := s.tracer.Start(ctx, "db.query",
		WithAttributes(
			StringAttribute("db.statement", "SELECT * FROM users WHERE id = ?"),
			StringAttribute("db.table", "users"),
		),
	)
	
	// 模擬的な遅延
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
	
	s.mu.RLock()
	user, exists := s.users[userID]
	s.mu.RUnlock()
	
	dbSpan.End()
	
	if !exists {
		err := fmt.Errorf("user not found: %s", userID)
		span.RecordError(err)
		return nil, err
	}
	
	span.AddEvent("user.found", StringAttribute("user.name", user.Name))
	return user, nil
}

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
	ctx, span := s.tracer.Start(ctx, "CreateUser",
		WithAttributes(
			StringAttribute("user.id", user.ID),
			StringAttribute("user.name", user.Name),
			StringAttribute("service.name", "user-service"),
		),
	)
	defer span.End()
	
	// バリデーション
	if user.Name == "" {
		err := fmt.Errorf("user name is required")
		span.RecordError(err)
		return err
	}
	
	// データベース書き込みをシミュレート
	ctx, dbSpan := s.tracer.Start(ctx, "db.insert",
		WithAttributes(
			StringAttribute("db.statement", "INSERT INTO users (id, name, email) VALUES (?, ?, ?)"),
			StringAttribute("db.table", "users"),
		),
	)
	
	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	
	s.mu.Lock()
	s.users[user.ID] = user
	s.mu.Unlock()
	
	dbSpan.End()
	
	span.AddEvent("user.created", StringAttribute("user.id", user.ID))
	return nil
}

// 注文サービス
type OrderService struct {
	tracer      *Tracer
	userService *UserService
	orders      map[string]*Order
	mu          sync.RWMutex
}

type Order struct {
	ID       string  `json:"id"`
	UserID   string  `json:"user_id"`
	Amount   float64 `json:"amount"`
	Status   string  `json:"status"`
	Items    []Item  `json:"items"`
}

type Item struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

func NewOrderService(userService *UserService) *OrderService {
	return &OrderService{
		tracer:      GetTracer("order-service"),
		userService: userService,
		orders:      make(map[string]*Order),
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, order *Order) error {
	ctx, span := s.tracer.Start(ctx, "CreateOrder",
		WithAttributes(
			StringAttribute("order.id", order.ID),
			StringAttribute("order.user_id", order.UserID),
			Float64Attribute("order.amount", order.Amount),
			StringAttribute("service.name", "order-service"),
		),
	)
	defer span.End()
	
	// ユーザー存在確認
	user, err := s.userService.GetUser(ctx, order.UserID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("user validation failed: %w", err)
	}
	
	span.AddEvent("user.validated", StringAttribute("user.name", user.Name))
	
	// 在庫確認
	ctx, inventorySpan := s.tracer.Start(ctx, "inventory.check",
		WithAttributes(
			IntAttribute("items.count", len(order.Items)),
		),
	)
	
	for _, item := range order.Items {
		// 在庫チェックをシミュレート
		time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)
		inventorySpan.AddEvent("item.checked", 
			StringAttribute("item.id", item.ID),
			IntAttribute("item.quantity", item.Quantity),
		)
	}
	
	inventorySpan.End()
	
	// 支払い処理
	ctx, paymentSpan := s.tracer.Start(ctx, "payment.process",
		WithAttributes(
			Float64Attribute("payment.amount", order.Amount),
			StringAttribute("payment.method", "credit_card"),
		),
	)
	
	// 支払い処理をシミュレート
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
	
	if rand.Float64() < 0.1 { // 10%の確率で支払い失敗
		err := fmt.Errorf("payment failed")
		paymentSpan.RecordError(err)
		paymentSpan.End()
		span.RecordError(err)
		return err
	}
	
	paymentSpan.AddEvent("payment.completed")
	paymentSpan.End()
	
	// 注文を保存
	order.Status = "confirmed"
	s.mu.Lock()
	s.orders[order.ID] = order
	s.mu.Unlock()
	
	span.AddEvent("order.created", StringAttribute("order.status", order.Status))
	return nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	ctx, span := s.tracer.Start(ctx, "GetOrder",
		WithAttributes(
			StringAttribute("order.id", orderID),
			StringAttribute("service.name", "order-service"),
		),
	)
	defer span.End()
	
	s.mu.RLock()
	order, exists := s.orders[orderID]
	s.mu.RUnlock()
	
	if !exists {
		err := fmt.Errorf("order not found: %s", orderID)
		span.RecordError(err)
		return nil, err
	}
	
	return order, nil
}

// APIハンドラー
type APIHandler struct {
	userService  *UserService
	orderService *OrderService
	tracer       *Tracer
}

func NewAPIHandler(userService *UserService, orderService *OrderService) *APIHandler {
	return &APIHandler{
		userService:  userService,
		orderService: orderService,
		tracer:       GetTracer("api-handler"),
	}
}

func (h *APIHandler) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	userID := r.URL.Query().Get("id")
	if userID == "" {
		http.Error(w, "user ID is required", http.StatusBadRequest)
		return
	}
	
	user, err := h.userService.GetUser(ctx, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *APIHandler) CreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	
	// 注文IDを生成
	order.ID = fmt.Sprintf("order_%d", time.Now().UnixNano())
	
	if err := h.orderService.CreateOrder(ctx, &order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func (h *APIHandler) TracesHandler(w http.ResponseWriter, r *http.Request) {
	traces := globalTracer.GetAllTraces()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(traces)
}

// サーバー設定
func SetupServer() *http.Server {
	userService := NewUserService()
	orderService := NewOrderService(userService)
	handler := NewAPIHandler(userService, orderService)
	
	tracer := GetTracer("http-server")
	
	mux := http.NewServeMux()
	mux.HandleFunc("/users", handler.GetUserHandler)
	mux.HandleFunc("/orders", handler.CreateOrderHandler)
	mux.HandleFunc("/traces", handler.TracesHandler)
	
	// トレーシングミドルウェアを適用
	tracedHandler := TracingMiddleware(tracer)(mux)
	
	return &http.Server{
		Addr:    ":8080",
		Handler: tracedHandler,
	}
}

func main() {
	fmt.Println("Day 59: OpenTelemetry Distributed Tracing")
	
	server := SetupServer()
	
	log.Printf("Starting server on %s", server.Addr)
	log.Printf("Traces available at http://localhost%s/traces", server.Addr)
	log.Fatal(server.ListenAndServe())
}