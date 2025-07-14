# Day 59: OpenTelemetry Distributed Tracing

## 🎯 本日の目標 (Today's Goal)

OpenTelemetryによる分散トレーシングを実装し、マイクロサービス間のリクエストフローを完全に可視化する高度なトレーシングシステムを構築できるようになる。分散システムでのパフォーマンス分析、エラー追跡、依存関係の監視を習得する。

## 📖 解説 (Explanation)

### OpenTelemetryとは

OpenTelemetryは、Cloud Native Computing Foundation（CNCF）でホストされているオープンソースの観測可能性フレームワークです。現代の分散システムでメトリクス、ログ、トレースの3つの柱となるテレメトリデータを統一的に処理できます。

#### OpenTelemetryの特徴

**ベンダーニュートラル**
- 特定のベンダーに依存しない標準化されたAPI
- Jaeger、Zipkin、Prometheus、Grafanaなど多様なバックエンドに対応
- アプリケーションコードを変更せずにバックエンドの切り替えが可能

**言語横断サポート**
- Go、Java、Python、.NET、JavaScript、Rustなど多言語対応
- 一貫したAPIとSDKで言語間の差を最小化

**高い拡張性**
- カスタムインストルメンテーション
- プラグイン可能な処理パイプライン
- サンプリング戦略の柔軟な設定

### 分散トレーシングの基本概念

#### 1. Trace（トレース）

一つのユーザーリクエストまたはワークフローの全体的な流れを表現します。

```
Trace ID: abc123-def456-ghi789
├── Frontend Service (100ms)
├── Auth Service (50ms)
├── User Service (200ms)
│   ├── Database Query (150ms)
│   └── Cache Lookup (10ms)
├── Order Service (300ms)
│   ├── Payment API (250ms)
│   └── Inventory Check (80ms)
└── Notification Service (30ms)
```

#### 2. Span（スパン）

トレース内の個別の操作や処理単位を表現します。

```go
type Span struct {
    TraceID     string
    SpanID      string
    ParentID    string
    Name        string
    Kind        SpanKind
    StartTime   time.Time
    EndTime     time.Time
    Attributes  []Attribute
    Events      []SpanEvent
    Status      SpanStatus
}

type SpanKind int

const (
    SpanKindInternal SpanKind = iota
    SpanKindServer
    SpanKindClient
    SpanKindProducer
    SpanKindConsumer
)
```

#### 3. Context（コンテキスト）

スパン間の親子関係とトレース情報を維持する仕組みです。

```go
// コンテキストからスパンを取得
span := trace.SpanFromContext(ctx)

// コンテキストにスパンを設定
ctx = trace.ContextWithSpan(ctx, span)

// 現在のスパンコンテキストを取得
spanContext := span.SpanContext()
if spanContext.IsValid() {
    traceID := spanContext.TraceID()
    spanID := spanContext.SpanID()
}
```

### OpenTelemetryアーキテクチャ

#### 1. アプリケーション層

```go
// 基本的なトレーサー設定
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func initTracer(serviceName string) (*trace.TracerProvider, error) {
    // Jaegerエクスポーターの設定
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://localhost:14268/api/traces"),
    ))
    if err != nil {
        return nil, err
    }

    // リソース情報の設定
    resource := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceNameKey.String(serviceName),
        semconv.ServiceVersionKey.String("1.0.0"),
        semconv.DeploymentEnvironmentKey.String("development"),
    )

    // トレーサープロバイダーの設定
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource),
        trace.WithSampler(trace.AlwaysSample()), // 全サンプリング（開発用）
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

#### 2. 手動インストルメンテーション

```go
type TraceableService struct {
    tracer trace.Tracer
    db     *sql.DB
}

func NewTraceableService(serviceName string, db *sql.DB) *TraceableService {
    return &TraceableService{
        tracer: otel.Tracer(serviceName),
        db:     db,
    }
}

func (s *TraceableService) CreateUser(ctx context.Context, user *User) error {
    // メインオペレーションのスパン
    ctx, span := s.tracer.Start(ctx, "CreateUser",
        trace.WithSpanKind(trace.SpanKindServer),
        trace.WithAttributes(
            attribute.String("user.id", user.ID),
            attribute.String("user.email", user.Email),
            attribute.String("operation", "user_creation"),
        ),
    )
    defer span.End()

    // バリデーションスパン
    if err := s.validateUser(ctx, user); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "validation failed")
        return err
    }

    // データベース操作スパン
    if err := s.insertUser(ctx, user); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "database insertion failed")
        return err
    }

    // カスタムイベントの記録
    span.AddEvent("User created successfully",
        trace.WithAttributes(
            attribute.String("user.id", user.ID),
            attribute.Int64("timestamp", time.Now().Unix()),
        ),
    )

    span.SetStatus(codes.Ok, "User created")
    return nil
}

func (s *TraceableService) validateUser(ctx context.Context, user *User) error {
    ctx, span := s.tracer.Start(ctx, "ValidateUser",
        trace.WithSpanKind(trace.SpanKindInternal),
    )
    defer span.End()

    validationErrors := make([]string, 0)

    if user.ID == "" {
        validationErrors = append(validationErrors, "ID is required")
    }
    if user.Email == "" {
        validationErrors = append(validationErrors, "Email is required")
    }
    if !isValidEmail(user.Email) {
        validationErrors = append(validationErrors, "Invalid email format")
    }

    if len(validationErrors) > 0 {
        span.SetAttributes(
            attribute.StringSlice("validation.errors", validationErrors),
            attribute.Int("validation.error_count", len(validationErrors)),
        )
        return fmt.Errorf("validation failed: %s", strings.Join(validationErrors, ", "))
    }

    span.SetAttributes(attribute.Bool("validation.success", true))
    return nil
}

func (s *TraceableService) insertUser(ctx context.Context, user *User) error {
    ctx, span := s.tracer.Start(ctx, "InsertUser",
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            attribute.String("db.system", "postgresql"),
            attribute.String("db.operation", "INSERT"),
            attribute.String("db.table", "users"),
        ),
    )
    defer span.End()

    start := time.Now()
    
    query := "INSERT INTO users (id, name, email, created_at) VALUES ($1, $2, $3, $4)"
    _, err := s.db.ExecContext(ctx, query, user.ID, user.Name, user.Email, time.Now())
    
    duration := time.Since(start)
    
    span.SetAttributes(
        attribute.String("db.statement", query),
        attribute.String("db.query_duration", duration.String()),
    )

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "database insertion failed")
        return err
    }

    span.SetStatus(codes.Ok, "User inserted successfully")
    return nil
}
```

#### 3. HTTP ミドルウェア実装

```go
import (
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    "go.opentelemetry.io/otel/propagation"
)

type TracingMiddleware struct {
    tracer     trace.Tracer
    propagator propagation.TextMapPropagator
}

func NewTracingMiddleware(serviceName string) *TracingMiddleware {
    return &TracingMiddleware{
        tracer:     otel.Tracer(serviceName),
        propagator: otel.GetTextMapPropagator(),
    }
}

func (m *TracingMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // HTTPヘッダーからコンテキストを抽出
        ctx := m.propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
        
        // HTTPリクエストスパンを作成
        ctx, span := m.tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
            trace.WithSpanKind(trace.SpanKindServer),
            trace.WithAttributes(
                semconv.HTTPMethodKey.String(r.Method),
                semconv.HTTPURLKey.String(r.URL.String()),
                semconv.HTTPSchemeKey.String(r.URL.Scheme),
                semconv.HTTPHostKey.String(r.Host),
                semconv.HTTPTargetKey.String(r.URL.Path),
                semconv.HTTPUserAgentKey.String(r.Header.Get("User-Agent")),
                semconv.HTTPRequestContentLengthKey.Int64(r.ContentLength),
            ),
        )
        defer span.End()

        // レスポンスライターをラップしてステータスコードを取得
        ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        // リクエストを更新されたコンテキストで処理
        next.ServeHTTP(ww, r.WithContext(ctx))

        // レスポンス情報をスパンに追加
        span.SetAttributes(
            semconv.HTTPStatusCodeKey.Int(ww.statusCode),
            semconv.HTTPResponseContentLengthKey.Int64(ww.bytesWritten),
        )

        // ステータスコードに基づいてスパンのステータスを設定
        if ww.statusCode >= 400 {
            span.SetStatus(codes.Error, http.StatusText(ww.statusCode))
        } else {
            span.SetStatus(codes.Ok, "")
        }
    })
}

type responseWriter struct {
    http.ResponseWriter
    statusCode   int
    bytesWritten int64
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    n, err := rw.ResponseWriter.Write(b)
    rw.bytesWritten += int64(n)
    return n, err
}
```

#### 4. 分散システム間のコンテキスト伝播

```go
type HTTPClient struct {
    client     *http.Client
    tracer     trace.Tracer
    propagator propagation.TextMapPropagator
}

func NewHTTPClient(serviceName string) *HTTPClient {
    return &HTTPClient{
        client:     &http.Client{Timeout: 30 * time.Second},
        tracer:     otel.Tracer(serviceName),
        propagator: otel.GetTextMapPropagator(),
    }
}

func (c *HTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
    // HTTPクライアントスパンを作成
    ctx, span := c.tracer.Start(ctx, "HTTP GET",
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            semconv.HTTPMethodKey.String("GET"),
            semconv.HTTPURLKey.String(url),
        ),
    )
    defer span.End()

    // リクエストを作成
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "failed to create request")
        return nil, err
    }

    // コンテキストをHTTPヘッダーに注入
    c.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

    start := time.Now()
    resp, err := c.client.Do(req)
    duration := time.Since(start)

    // リクエスト結果をスパンに記録
    span.SetAttributes(
        attribute.String("http.request_duration", duration.String()),
    )

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "HTTP request failed")
        return nil, err
    }

    span.SetAttributes(
        semconv.HTTPStatusCodeKey.Int(resp.StatusCode),
        semconv.HTTPResponseContentLengthKey.Int64(resp.ContentLength),
    )

    if resp.StatusCode >= 400 {
        span.SetStatus(codes.Error, resp.Status)
    } else {
        span.SetStatus(codes.Ok, "")
    }

    return resp, nil
}

func (c *HTTPClient) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
    ctx, span := c.tracer.Start(ctx, "HTTP POST",
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            semconv.HTTPMethodKey.String("POST"),
            semconv.HTTPURLKey.String(url),
            attribute.String("http.content_type", contentType),
        ),
    )
    defer span.End()

    req, err := http.NewRequestWithContext(ctx, "POST", url, body)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "failed to create request")
        return nil, err
    }

    req.Header.Set("Content-Type", contentType)
    c.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

    start := time.Now()
    resp, err := c.client.Do(req)
    duration := time.Since(start)

    span.SetAttributes(attribute.String("http.request_duration", duration.String()))

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "HTTP request failed")
        return nil, err
    }

    span.SetAttributes(
        semconv.HTTPStatusCodeKey.Int(resp.StatusCode),
        semconv.HTTPResponseContentLengthKey.Int64(resp.ContentLength),
    )

    if resp.StatusCode >= 400 {
        span.SetStatus(codes.Error, resp.Status)
    } else {
        span.SetStatus(codes.Ok, "")
    }

    return resp, nil
}
```

### 高度なトレーシング機能

#### 1. カスタムスパンプロセッサー

```go
type CustomSpanProcessor struct {
    next trace.SpanProcessor
}

func NewCustomSpanProcessor(next trace.SpanProcessor) trace.SpanProcessor {
    return &CustomSpanProcessor{next: next}
}

func (p *CustomSpanProcessor) OnStart(parent context.Context, s trace.ReadWriteSpan) {
    // スパン開始時の処理
    s.SetAttributes(
        attribute.String("processor.version", "1.0.0"),
        attribute.Int64("processor.start_time", time.Now().UnixNano()),
    )
    
    p.next.OnStart(parent, s)
}

func (p *CustomSpanProcessor) OnEnd(s trace.ReadOnlySpan) {
    // スパン終了時の処理
    duration := s.EndTime().Sub(s.StartTime())
    
    // 長時間実行されたスパンをログ出力
    if duration > 5*time.Second {
        log.Printf("Long running span detected: %s (duration: %v)", 
            s.Name(), duration)
    }
    
    p.next.OnEnd(s)
}

func (p *CustomSpanProcessor) Shutdown(ctx context.Context) error {
    return p.next.Shutdown(ctx)
}

func (p *CustomSpanProcessor) ForceFlush(ctx context.Context) error {
    return p.next.ForceFlush(ctx)
}
```

#### 2. サンプリング戦略

```go
type CustomSampler struct {
    rateLimiter *rate.Limiter
}

func NewCustomSampler(rps int) trace.Sampler {
    return &CustomSampler{
        rateLimiter: rate.NewLimiter(rate.Limit(rps), rps),
    }
}

func (s *CustomSampler) ShouldSample(p trace.SamplingParameters) trace.SamplingResult {
    // エラースパンは常にサンプリング
    if len(p.Attributes) > 0 {
        for _, attr := range p.Attributes {
            if attr.Key == "error" && attr.Value.AsBool() {
                return trace.SamplingResult{
                    Decision: trace.RecordAndSample,
                }
            }
        }
    }
    
    // 重要な操作は常にサンプリング
    importantOperations := []string{
        "CreateUser", "ProcessPayment", "PlaceOrder",
    }
    
    for _, op := range importantOperations {
        if strings.Contains(p.Name, op) {
            return trace.SamplingResult{
                Decision: trace.RecordAndSample,
            }
        }
    }
    
    // レート制限に基づくサンプリング
    if s.rateLimiter.Allow() {
        return trace.SamplingResult{
            Decision: trace.RecordAndSample,
        }
    }
    
    return trace.SamplingResult{
        Decision: trace.Drop,
    }
}

func (s *CustomSampler) Description() string {
    return "CustomSampler with rate limiting and priority-based sampling"
}
```

#### 3. バッチ設定とパフォーマンス最適化

```go
func initOptimizedTracer(serviceName string) (*trace.TracerProvider, error) {
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://localhost:14268/api/traces"),
    ))
    if err != nil {
        return nil, err
    }

    resource := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceNameKey.String(serviceName),
        semconv.ServiceVersionKey.String("1.0.0"),
    )

    // バッチプロセッサーの詳細設定
    batcher := trace.NewBatchSpanProcessor(exporter,
        trace.WithBatchTimeout(2*time.Second),        // バッチ送信間隔
        trace.WithMaxExportBatchSize(512),           // 最大バッチサイズ  
        trace.WithMaxQueueSize(2048),                // 最大キューサイズ
        trace.WithExportTimeout(10*time.Second),     // エクスポートタイムアウト
    )

    // カスタムプロセッサーとバッチプロセッサーを組み合わせ
    customProcessor := NewCustomSpanProcessor(batcher)

    tp := trace.NewTracerProvider(
        trace.WithSpanProcessor(customProcessor),
        trace.WithResource(resource),
        trace.WithSampler(NewCustomSampler(100)), // 100 RPS サンプリング
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

#### 4. トレース分析とメトリクス

```go
type TraceAnalyzer struct {
    tracer  trace.Tracer
    metrics TraceMetrics
}

type TraceMetrics struct {
    SpanCount       *prometheus.CounterVec
    SpanDuration    *prometheus.HistogramVec
    ErrorCount      *prometheus.CounterVec
    ServiceRequests *prometheus.CounterVec
}

func NewTraceAnalyzer(serviceName string) *TraceAnalyzer {
    return &TraceAnalyzer{
        tracer: otel.Tracer(serviceName),
        metrics: TraceMetrics{
            SpanCount: prometheus.NewCounterVec(
                prometheus.CounterOpts{
                    Name: "trace_span_total",
                    Help: "Total number of spans created",
                },
                []string{"service", "operation", "status"},
            ),
            SpanDuration: prometheus.NewHistogramVec(
                prometheus.HistogramOpts{
                    Name: "trace_span_duration_seconds",
                    Help: "Span duration in seconds",
                    Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30},
                },
                []string{"service", "operation"},
            ),
            ErrorCount: prometheus.NewCounterVec(
                prometheus.CounterOpts{
                    Name: "trace_errors_total",
                    Help: "Total number of errors in traces",
                },
                []string{"service", "operation", "error_type"},
            ),
            ServiceRequests: prometheus.NewCounterVec(
                prometheus.CounterOpts{
                    Name: "service_requests_total",
                    Help: "Total number of service requests",
                },
                []string{"source_service", "target_service", "operation"},
            ),
        },
    }
}

func (a *TraceAnalyzer) InstrumentedExecution(ctx context.Context, operationName string, fn func(context.Context) error) error {
    ctx, span := a.tracer.Start(ctx, operationName,
        trace.WithAttributes(
            attribute.String("instrumented.by", "trace_analyzer"),
        ),
    )
    defer span.End()

    start := time.Now()
    err := fn(ctx)
    duration := time.Since(start)

    // メトリクス記録
    status := "success"
    if err != nil {
        status = "error"
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        
        a.metrics.ErrorCount.WithLabelValues(
            "service", operationName, err.Error(),
        ).Inc()
    } else {
        span.SetStatus(codes.Ok, "")
    }

    a.metrics.SpanCount.WithLabelValues(
        "service", operationName, status,
    ).Inc()

    a.metrics.SpanDuration.WithLabelValues(
        "service", operationName,
    ).Observe(duration.Seconds())

    return err
}
```

## 📝 課題 (The Problem)

以下の機能を持つOpenTelemetry分散トレーシングシステムを実装してください：

### 1. トレーシング基盤

```go
type Tracer struct {
    provider   trace.TracerProvider
    tracer     trace.Tracer
    propagator propagation.TextMapPropagator
}
```

### 2. 必要な機能

- **基本トレーシング**: HTTP リクエストの自動トレース
- **コンテキスト伝播**: サービス間でのトレース情報の伝播
- **カスタムスパン**: アプリケーション固有の操作のトレース
- **エラートレーシング**: エラー情報の詳細な記録
- **パフォーマンス分析**: 処理時間とボトルネックの特定

### 3. ミドルウェア実装

- HTTP リクエスト/レスポンスの自動トレーシング
- データベースアクセスのトレーシング  
- 外部API呼び出しのトレーシング

### 4. カスタムエクスポーター

複数のバックエンドへの同時出力

### 5. メトリクス統合

Prometheusメトリクスとの連携

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestTracer_BasicSpan
    main_test.go:45: Basic span created and recorded correctly
--- PASS: TestTracer_BasicSpan (0.01s)

=== RUN   TestTracer_ChildSpan
    main_test.go:65: Child span relationship established correctly
--- PASS: TestTracer_ChildSpan (0.01s)

=== RUN   TestTracer_ContextPropagation
    main_test.go:85: Context propagation across services working
--- PASS: TestTracer_ContextPropagation (0.02s)

=== RUN   TestTracer_ErrorHandling
    main_test.go:105: Error information recorded in span correctly
--- PASS: TestTracer_ErrorHandling (0.01s)

=== RUN   TestMiddleware_HTTPTracing
    main_test.go:125: HTTP requests traced automatically
--- PASS: TestMiddleware_HTTPTracing (0.03s)

PASS
ok      day59-opentelemetry-tracing   0.156s
```

### トレース出力例

```json
{
  "traceID": "1234567890abcdef",
  "spanID": "abcdef1234567890",
  "parentSpanID": "fedcba0987654321",
  "operationName": "GET /api/users",
  "startTime": "2023-12-01T10:00:00Z",
  "endTime": "2023-12-01T10:00:00.150Z",
  "duration": "150ms",
  "tags": {
    "http.method": "GET",
    "http.url": "/api/users",
    "http.status_code": 200,
    "user.id": "user123",
    "service.name": "user-service"
  },
  "logs": [
    {
      "timestamp": "2023-12-01T10:00:00.050Z",
      "message": "Database query started"
    },
    {
      "timestamp": "2023-12-01T10:00:00.140Z", 
      "message": "Database query completed"
    }
  ]
}
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 必要なパッケージ

```go
import (
    "context"
    "time"
    "net/http"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)
```

### 基本的なトレーサー初期化

```go
func initTracer(serviceName string) (*trace.TracerProvider, error) {
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://localhost:14268/api/traces"),
    ))
    if err != nil {
        return nil, err
    }

    resource := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceNameKey.String(serviceName),
    )

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource),
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

### スパンの作成と管理

```go
func (t *Tracer) CreateSpan(ctx context.Context, name string) (context.Context, trace.Span) {
    return t.tracer.Start(ctx, name,
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(
            attribute.String("operation", name),
            attribute.Int64("timestamp", time.Now().Unix()),
        ),
    )
}

func (t *Tracer) RecordError(span trace.Span, err error) {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```

### HTTP ミドルウェア

```go
func (t *Tracer) HTTPMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
        
        ctx, span := t.tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path),
            trace.WithSpanKind(trace.SpanKindServer),
            trace.WithAttributes(
                semconv.HTTPMethodKey.String(r.Method),
                semconv.HTTPURLKey.String(r.URL.String()),
            ),
        )
        defer span.End()
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### コンテキスト伝播

```go
func (t *Tracer) InjectContext(ctx context.Context, headers http.Header) {
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
}

func (t *Tracer) ExtractContext(ctx context.Context, headers http.Header) context.Context {
    return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(headers))
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **自動インストルメンテーション**: データベースドライバーの自動トレーシング
2. **カスタムプロパゲーター**: 独自のコンテキスト伝播方式
3. **分散サンプリング**: ネットワーク全体でのサンプリング戦略
4. **リアルタイム分析**: ストリーミング処理でのトレース分析
5. **セキュリティ**: 機密情報の自動マスキング

OpenTelemetry分散トレーシングの実装を通じて、マイクロサービスアーキテクチャでの高度な観測可能性を実現しましょう！