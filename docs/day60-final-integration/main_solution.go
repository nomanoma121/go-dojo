// Day 60: 総集編 - Production-Ready Microservice
// slog、Prometheus、OpenTelemetryを統合したミニAPIサービス

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// ===== 構造化ログ (slog) =====

var logger *slog.Logger

func initLogger() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{Key: "timestamp", Value: a.Value}
			}
			return a
		},
	}
	
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// ===== Prometheusメトリクス =====

type Metrics struct {
	RequestsTotal   map[string]float64
	RequestDuration map[string][]float64
	ActiveRequests  map[string]float64
	ErrorsTotal     map[string]float64
	BusinessMetrics map[string]float64
	mu              sync.RWMutex
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal:   make(map[string]float64),
		RequestDuration: make(map[string][]float64),
		ActiveRequests:  make(map[string]float64),
		ErrorsTotal:     make(map[string]float64),
		BusinessMetrics: make(map[string]float64),
	}
}

func (m *Metrics) IncRequestsTotal(method, endpoint, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%s_%s", method, endpoint, status)
	m.RequestsTotal[key]++
}

func (m *Metrics) ObserveRequestDuration(method, endpoint string, duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%s", method, endpoint)
	m.RequestDuration[key] = append(m.RequestDuration[key], duration)
}

func (m *Metrics) SetActiveRequests(endpoint string, count float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ActiveRequests[endpoint] = count
}

func (m *Metrics) IncErrorsTotal(method, endpoint, errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s_%s_%s", method, endpoint, errorType)
	m.ErrorsTotal[key]++
}

func (m *Metrics) SetBusinessMetric(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BusinessMetrics[name] = value
}

func (m *Metrics) Export() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]interface{})
	result["http_requests_total"] = m.RequestsTotal
	result["http_errors_total"] = m.ErrorsTotal
	result["http_active_requests"] = m.ActiveRequests
	result["business_metrics"] = m.BusinessMetrics
	
	// 平均レスポンス時間を計算
	avgDuration := make(map[string]float64)
	for key, durations := range m.RequestDuration {
		if len(durations) > 0 {
			sum := 0.0
			for _, d := range durations {
				sum += d
			}
			avgDuration[key] = sum / float64(len(durations))
		}
	}
	result["http_request_duration_avg"] = avgDuration
	
	return result
}

// ===== 分散トレーシング =====

type TraceContext struct {
	TraceID  string `json:"trace_id"`
	SpanID   string `json:"span_id"`
	ParentID string `json:"parent_id,omitempty"`
}

type Span struct {
	TraceID   string                 `json:"trace_id"`
	SpanID    string                 `json:"span_id"`
	ParentID  string                 `json:"parent_id,omitempty"`
	Operation string                 `json:"operation"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Duration  *time.Duration         `json:"duration,omitempty"`
	Tags      map[string]interface{} `json:"tags"`
	Logs      []SpanLog              `json:"logs"`
	Error     *string                `json:"error,omitempty"`
}

type SpanLog struct {
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

type Tracer struct {
	serviceName string
	spans       map[string]*Span
	mu          sync.RWMutex
}

func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		serviceName: serviceName,
		spans:       make(map[string]*Span),
	}
}

func (t *Tracer) StartSpan(ctx context.Context, operation string) (context.Context, *Span) {
	span := &Span{
		TraceID:   generateTraceID(),
		SpanID:    generateSpanID(),
		Operation: operation,
		StartTime: time.Now(),
		Tags:      make(map[string]interface{}),
		Logs:      make([]SpanLog, 0),
	}
	
	// 親スパンから情報を継承
	if parentSpan := SpanFromContext(ctx); parentSpan != nil {
		span.TraceID = parentSpan.TraceID
		span.ParentID = parentSpan.SpanID
	}
	
	span.Tags["service.name"] = t.serviceName
	
	t.mu.Lock()
	t.spans[span.SpanID] = span
	t.mu.Unlock()
	
	ctx = ContextWithSpan(ctx, span)
	
	logger.Info("Span started",
		slog.String("trace_id", span.TraceID),
		slog.String("span_id", span.SpanID),
		slog.String("operation", operation),
	)
	
	return ctx, span
}

func (s *Span) SetTag(key string, value interface{}) {
	s.Tags[key] = value
}

func (s *Span) LogEvent(message string, fields map[string]interface{}) {
	s.Logs = append(s.Logs, SpanLog{
		Timestamp: time.Now(),
		Message:   message,
		Fields:    fields,
	})
}

func (s *Span) SetError(err error) {
	errorMsg := err.Error()
	s.Error = &errorMsg
	s.LogEvent("error", map[string]interface{}{
		"error.message": err.Error(),
		"error.type":    fmt.Sprintf("%T", err),
	})
}

func (s *Span) Finish() {
	now := time.Now()
	s.EndTime = &now
	duration := now.Sub(s.StartTime)
	s.Duration = &duration
	
	logger.Info("Span finished",
		slog.String("trace_id", s.TraceID),
		slog.String("span_id", s.SpanID),
		slog.String("operation", s.Operation),
		slog.Duration("duration", duration),
		slog.Bool("error", s.Error != nil),
	)
}

func (t *Tracer) GetSpans() map[string]*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	result := make(map[string]*Span)
	for k, v := range t.spans {
		result[k] = v
	}
	return result
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

func generateTraceID() string {
	return fmt.Sprintf("trace_%d_%04d", time.Now().UnixNano(), rand.Intn(10000))
}

func generateSpanID() string {
	return fmt.Sprintf("span_%d_%04d", time.Now().UnixNano(), rand.Intn(10000))
}

// ===== ビジネスロジック =====

// User ドメイン
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type UserService struct {
	users  map[string]*User
	tracer *Tracer
	mu     sync.RWMutex
}

func NewUserService(tracer *Tracer) *UserService {
	return &UserService{
		users:  make(map[string]*User),
		tracer: tracer,
	}
}

func (s *UserService) CreateUser(ctx context.Context, name, email string) (*User, error) {
	ctx, span := s.tracer.StartSpan(ctx, "UserService.CreateUser")
	defer span.Finish()
	
	span.SetTag("user.name", name)
	span.SetTag("user.email", email)
	
	// バリデーション
	if name == "" {
		err := fmt.Errorf("name is required")
		span.SetError(err)
		return nil, err
	}
	
	if email == "" {
		err := fmt.Errorf("email is required")
		span.SetError(err)
		return nil, err
	}
	
	user := &User{
		ID:        generateUserID(),
		Name:      name,
		Email:     email,
		Status:    "active",
		CreatedAt: time.Now(),
	}
	
	// データベースシミュレーション
	span.LogEvent("database.insert", map[string]interface{}{
		"table": "users",
		"id":    user.ID,
	})
	
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
	
	s.mu.Lock()
	s.users[user.ID] = user
	s.mu.Unlock()
	
	span.LogEvent("user.created", map[string]interface{}{
		"user.id": user.ID,
	})
	
	return user, nil
}

func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
	ctx, span := s.tracer.StartSpan(ctx, "UserService.GetUser")
	defer span.Finish()
	
	span.SetTag("user.id", userID)
	
	span.LogEvent("database.select", map[string]interface{}{
		"table": "users",
		"id":    userID,
	})
	
	time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)
	
	s.mu.RLock()
	user, exists := s.users[userID]
	s.mu.RUnlock()
	
	if !exists {
		err := fmt.Errorf("user not found: %s", userID)
		span.SetError(err)
		return nil, err
	}
	
	return user, nil
}

func (s *UserService) GetAllUsers(ctx context.Context) ([]*User, error) {
	ctx, span := s.tracer.StartSpan(ctx, "UserService.GetAllUsers")
	defer span.Finish()
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	users := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	
	span.SetTag("users.count", len(users))
	return users, nil
}

// Order ドメイン
type Order struct {
	ID       string    `json:"id"`
	UserID   string    `json:"user_id"`
	Amount   float64   `json:"amount"`
	Status   string    `json:"status"`
	Items    []Item    `json:"items"`
	CreatedAt time.Time `json:"created_at"`
}

type Item struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type OrderService struct {
	orders      map[string]*Order
	userService *UserService
	tracer      *Tracer
	mu          sync.RWMutex
}

func NewOrderService(userService *UserService, tracer *Tracer) *OrderService {
	return &OrderService{
		orders:      make(map[string]*Order),
		userService: userService,
		tracer:      tracer,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, userID string, items []Item) (*Order, error) {
	ctx, span := s.tracer.StartSpan(ctx, "OrderService.CreateOrder")
	defer span.Finish()
	
	span.SetTag("user.id", userID)
	span.SetTag("items.count", len(items))
	
	// ユーザー存在確認
	user, err := s.userService.GetUser(ctx, userID)
	if err != nil {
		span.SetError(err)
		return nil, fmt.Errorf("user validation failed: %w", err)
	}
	
	span.LogEvent("user.validated", map[string]interface{}{
		"user.name": user.Name,
	})
	
	// 金額計算
	var totalAmount float64
	for _, item := range items {
		totalAmount += item.Price * float64(item.Quantity)
	}
	
	span.SetTag("order.amount", totalAmount)
	
	// 支払い処理シミュレーション
	ctx, paymentSpan := s.tracer.StartSpan(ctx, "PaymentService.ProcessPayment")
	paymentSpan.SetTag("amount", totalAmount)
	paymentSpan.SetTag("currency", "USD")
	
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
	
	if rand.Float64() < 0.1 { // 10%の確率で失敗
		err := fmt.Errorf("payment failed")
		paymentSpan.SetError(err)
		paymentSpan.Finish()
		span.SetError(err)
		return nil, err
	}
	
	paymentSpan.LogEvent("payment.completed", nil)
	paymentSpan.Finish()
	
	// 注文作成
	order := &Order{
		ID:       generateOrderID(),
		UserID:   userID,
		Amount:   totalAmount,
		Status:   "confirmed",
		Items:    items,
		CreatedAt: time.Now(),
	}
	
	s.mu.Lock()
	s.orders[order.ID] = order
	s.mu.Unlock()
	
	span.LogEvent("order.created", map[string]interface{}{
		"order.id": order.ID,
		"status":   order.Status,
	})
	
	return order, nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*Order, error) {
	ctx, span := s.tracer.StartSpan(ctx, "OrderService.GetOrder")
	defer span.Finish()
	
	span.SetTag("order.id", orderID)
	
	s.mu.RLock()
	order, exists := s.orders[orderID]
	s.mu.RUnlock()
	
	if !exists {
		err := fmt.Errorf("order not found: %s", orderID)
		span.SetError(err)
		return nil, err
	}
	
	return order, nil
}

// ===== HTTPハンドラー =====

type APIServer struct {
	userService  *UserService
	orderService *OrderService
	metrics      *Metrics
	tracer       *Tracer
}

func NewAPIServer(userService *UserService, orderService *OrderService, metrics *Metrics, tracer *Tracer) *APIServer {
	return &APIServer{
		userService:  userService,
		orderService: orderService,
		metrics:      metrics,
		tracer:       tracer,
	}
}

func (s *APIServer) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// アクティブリクエスト数を増加
		s.metrics.SetActiveRequests(r.URL.Path, s.metrics.ActiveRequests[r.URL.Path]+1)
		
		// レスポンスライターをラップ
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		// トレーシングコンテキストを設定
		ctx, span := s.tracer.StartSpan(r.Context(), fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		span.SetTag("http.method", r.Method)
		span.SetTag("http.url", r.URL.String())
		span.SetTag("http.user_agent", r.UserAgent())
		
		r = r.WithContext(ctx)
		
		// 次のハンドラを実行
		next.ServeHTTP(ww, r)
		
		// メトリクスを記録
		duration := time.Since(start)
		status := strconv.Itoa(ww.statusCode)
		
		s.metrics.IncRequestsTotal(r.Method, r.URL.Path, status)
		s.metrics.ObserveRequestDuration(r.Method, r.URL.Path, duration.Seconds())
		s.metrics.SetActiveRequests(r.URL.Path, s.metrics.ActiveRequests[r.URL.Path]-1)
		
		if ww.statusCode >= 400 {
			s.metrics.IncErrorsTotal(r.Method, r.URL.Path, "http_error")
		}
		
		// スパンを完了
		span.SetTag("http.status_code", ww.statusCode)
		if ww.statusCode >= 400 {
			span.SetError(fmt.Errorf("HTTP %d", ww.statusCode))
		}
		span.Finish()
		
		// 構造化ログ
		logger.Info("HTTP request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.statusCode),
			slog.Duration("duration", duration),
			slog.String("trace_id", span.TraceID),
			slog.String("user_agent", r.UserAgent()),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (s *APIServer) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	user, err := s.userService.CreateUser(r.Context(), req.Name, req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// ビジネスメトリクス
	s.metrics.SetBusinessMetric("users_total", s.metrics.BusinessMetrics["users_total"]+1)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (s *APIServer) GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	users, err := s.userService.GetAllUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (s *APIServer) CreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		UserID string `json:"user_id"`
		Items  []Item `json:"items"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	order, err := s.orderService.CreateOrder(r.Context(), req.UserID, req.Items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	// ビジネスメトリクス
	s.metrics.SetBusinessMetric("orders_total", s.metrics.BusinessMetrics["orders_total"]+1)
	s.metrics.SetBusinessMetric("revenue_total", s.metrics.BusinessMetrics["revenue_total"]+order.Amount)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func (s *APIServer) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	metrics := s.metrics.Export()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (s *APIServer) TracesHandler(w http.ResponseWriter, r *http.Request) {
	spans := s.tracer.GetSpans()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spans)
}

func (s *APIServer) HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "ecommerce-api",
		"version":   "1.0.0",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// ===== ユーティリティ =====

func generateUserID() string {
	return fmt.Sprintf("user_%d", time.Now().UnixNano())
}

func generateOrderID() string {
	return fmt.Sprintf("order_%d", time.Now().UnixNano())
}

// ===== メイン関数 =====

func main() {
	// 構造化ログ初期化
	initLogger()
	
	logger.Info("Starting E-commerce Microservice",
		slog.String("service", "ecommerce-api"),
		slog.String("version", "1.0.0"),
	)
	
	// コンポーネント初期化
	metrics := NewMetrics()
	tracer := NewTracer("ecommerce-api")
	userService := NewUserService(tracer)
	orderService := NewOrderService(userService, tracer)
	apiServer := NewAPIServer(userService, orderService, metrics, tracer)
	
	// HTTPサーバー設定
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users", apiServer.CreateUserHandler)
	mux.HandleFunc("/api/users/list", apiServer.GetUsersHandler)
	mux.HandleFunc("/api/orders", apiServer.CreateOrderHandler)
	mux.HandleFunc("/metrics", apiServer.MetricsHandler)
	mux.HandleFunc("/traces", apiServer.TracesHandler)
	mux.HandleFunc("/health", apiServer.HealthHandler)
	
	// ミドルウェア適用
	handler := apiServer.MetricsMiddleware(mux)
	
	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
	
	// Graceful Shutdown
	go func() {
		logger.Info("Server starting", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed", slog.Any("error", err))
		}
	}()
	
	// シグナル待機
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	logger.Info("Server shutting down")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", slog.Any("error", err))
	}
	
	logger.Info("Server stopped gracefully")
}