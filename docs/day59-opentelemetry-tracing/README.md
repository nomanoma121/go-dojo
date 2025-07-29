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
// 【コンテキスト管理の完全解説】分散システムでのトレース情報伝播
// ❌ 問題例：コンテキストの不適切な管理
func badContextHandling() {
    // 🚨 災害例：コンテキストを無視した処理
    span := tracer.Start(context.Background(), "bad-operation")
    // ❌ context.Background()を使用すると親スパンとの関係が失われる
    // ❌ 分散システムでトレースが断片化
    
    // 🚨 災害例：コンテキストを渡さない関数呼び出し
    processData() // コンテキストなし
    // ❌ 下位処理のスパンが分離される
    // ❌ 問題発生時の原因特定が困難
    
    span.End()
}

// ✅ 正解：適切なコンテキスト管理
func properContextHandling(ctx context.Context) {
    // 【STEP 1】親コンテキストからスパンを取得
    parentSpan := trace.SpanFromContext(ctx)
    
    // 【重要チェック】親スパンの有効性確認
    if parentSpan.SpanContext().IsValid() {
        log.Printf("📊 Parent trace found: %s", parentSpan.SpanContext().TraceID())
        // ✅ 親トレースとの関係が保たれている
    } else {
        log.Printf("⚠️  No parent trace context found")
        // ✅ 新しいトレースの開始点として適切
    }
    
    // 【STEP 2】子スパンの作成（親コンテキストを使用）
    childCtx, childSpan := tracer.Start(ctx, "child-operation")
    defer childSpan.End()
    
    // 【STEP 3】スパンコンテキストの情報取得
    spanContext := childSpan.SpanContext()
    if spanContext.IsValid() {
        traceID := spanContext.TraceID()
        spanID := spanContext.SpanID()
        
        // 【分散システム最適化】トレース情報の伝播
        log.Printf("🔗 Trace ID: %s, Span ID: %s", traceID, spanID)
        
        // 【重要】下位処理にコンテキストを正しく渡す
        processDataWithContext(childCtx)
        // ✅ 子スパンとの関係が維持される
        // ✅ 完全なトレース情報が取得可能
    }
}

// 【高度な使用例】並行処理でのコンテキスト管理
func advancedContextHandling(ctx context.Context) {
    // 【メインスパン作成】
    mainCtx, mainSpan := tracer.Start(ctx, "parallel-processing")
    defer mainSpan.End()
    
    // 【並行処理用チャネル】
    results := make(chan ProcessResult, 3)
    
    // 【STEP 1】各並行処理に独立したスパンを作成
    operations := []string{"validate", "transform", "store"}
    
    for _, op := range operations {
        go func(operation string) {
            // 【重要】各goroutineで独立したスパンを作成
            opCtx, opSpan := tracer.Start(mainCtx, fmt.Sprintf("parallel-%s", operation))
            defer opSpan.End()
            
            // 【属性設定】詳細な処理情報を記録
            opSpan.SetAttributes(
                attribute.String("operation.type", operation),
                attribute.String("worker.id", fmt.Sprintf("worker-%s", operation)),
                attribute.Int("worker.goroutine", runtime.NumGoroutine()),
            )
            
            // 【実際の処理】
            result := performOperation(opCtx, operation)
            
            // 【結果の記録】
            if result.Error != nil {
                opSpan.RecordError(result.Error)
                opSpan.SetStatus(codes.Error, result.Error.Error())
            } else {
                opSpan.SetStatus(codes.Ok, "operation completed successfully")
                opSpan.SetAttributes(
                    attribute.Int("result.count", result.Count),
                    attribute.Duration("result.duration", result.Duration),
                )
            }
            
            results <- result
        }(op)
    }
    
    // 【STEP 2】結果の収集と集約
    var allResults []ProcessResult
    for i := 0; i < len(operations); i++ {
        result := <-results
        allResults = append(allResults, result)
    }
    
    // 【STEP 3】メインスパンに集約結果を記録
    mainSpan.SetAttributes(
        attribute.Int("parallel.operations", len(operations)),
        attribute.Int("parallel.success", countSuccessful(allResults)),
        attribute.Int("parallel.errors", countErrors(allResults)),
    )
    
    if hasErrors(allResults) {
        mainSpan.SetStatus(codes.Error, "some parallel operations failed")
    } else {
        mainSpan.SetStatus(codes.Ok, "all parallel operations completed successfully")
    }
}
```

### OpenTelemetryアーキテクチャ

#### 1. アプリケーション層

```go
// 【高度なOpenTelemetryセットアップ】プロダクショングレードのトレーサー初期化
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func initTracer(serviceName string) (*trace.TracerProvider, error) {
    // 【STEP 1】Jaegerエクスポーターの詳細設定
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://localhost:14268/api/traces"),
        // 【オプション設定】プロダクション環境での設定例
        jaeger.WithUsername("jaeger-user"),
        jaeger.WithPassword("jaeger-password"),
    ))
    if err != nil {
        return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
    }

    // 【STEP 2】リソース情報の詳細設定
    // サービスを一意に特定するためのメタデータ
    resource := resource.NewWithAttributes(
        semconv.SchemaURL,
        // 【基本サービス情報】
        semconv.ServiceNameKey.String(serviceName),
        semconv.ServiceVersionKey.String("1.0.0"),
        semconv.ServiceInstanceIDKey.String(generateInstanceID()),
        semconv.DeploymentEnvironmentKey.String(getEnvironment()),
        
        // 【インフラ情報】
        semconv.HostNameKey.String(getHostname()),
        semconv.ProcessPIDKey.Int(os.Getpid()),
        semconv.OSTypeKey.String(runtime.GOOS),
        semconv.OSDescriptionKey.String(runtime.GOARCH),
        
        // 【アプリケーション情報】
        semconv.TelemetrySDKNameKey.String("opentelemetry"),
        semconv.TelemetrySDKLanguageKey.String("go"),
        semconv.TelemetrySDKVersionKey.String("1.21.0"),
        
        // 【カスタムメタデータ】
        attribute.String("team", "backend"),
        attribute.String("region", "us-west-2"),
        attribute.String("cluster", "production"),
        attribute.String("datacenter", "aws-oregon"),
    )

    // 【STEP 3】トレーサープロバイダーの高度な設定
    tp := trace.NewTracerProvider(
        // 【バッチエクスポーター】パフォーマンス最適化
        trace.WithBatcher(exporter,
            trace.WithBatchTimeout(2*time.Second),      // バッチ送信間隔
            trace.WithMaxExportBatchSize(512),         // 最大バッチサイズ
            trace.WithMaxQueueSize(2048),              // 最大キューサイズ
            trace.WithExportTimeout(10*time.Second),   // エクスポートタイムアウト
        ),
        
        // 【リソース情報】
        trace.WithResource(resource),
        
        // 【サンプリング戦略】環境に応じた最適化
        trace.WithSampler(createSmartSampler()),
        
        // 【スパンプロセッサー】カスタム処理の追加
        trace.WithSpanProcessor(NewPerformanceSpanProcessor()),
        trace.WithSpanProcessor(NewSecuritySpanProcessor()),
    )

    // 【STEP 4】グローバルトレーサー設定
    otel.SetTracerProvider(tp)
    
    // 【STEP 5】コンテキストプロパゲーター設定
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},    // W3C Trace Context
        propagation.Baggage{},         // W3C Baggage
        b3.New(),                      // B3 Propagation (Zipkin)
        jaeger.Propagator{},           // Jaeger Propagation
    ))
    
    log.Printf("✅ OpenTelemetry tracer initialized for service: %s", serviceName)
    return tp, nil
}

// 【スマートサンプリング】環境と状況に応じたサンプリング戦略
func createSmartSampler() trace.Sampler {
    environment := getEnvironment()
    
    switch environment {
    case "production":
        // 【本番環境】パフォーマンスを重視した低サンプリング
        return trace.TraceIDRatioBased(0.01) // 1%サンプリング
    case "staging":
        // 【ステージング環境】バランスを考慮した中程度サンプリング
        return trace.TraceIDRatioBased(0.1)  // 10%サンプリング
    default:
        // 【開発環境】デバッグを重視した高サンプリング
        return trace.AlwaysSample() // 100%サンプリング
    }
}

// 【パフォーマンススパンプロセッサー】遅いスパンの自動検出とアラート
type PerformanceSpanProcessor struct {
    slowThreshold time.Duration
    alertManager  *AlertManager
}

func NewPerformanceSpanProcessor() *PerformanceSpanProcessor {
    return &PerformanceSpanProcessor{
        slowThreshold: 1 * time.Second,
        alertManager:  NewAlertManager(),
    }
}

func (p *PerformanceSpanProcessor) OnStart(parent context.Context, s trace.ReadWriteSpan) {
    // スパン開始時のメタデータ追加
    s.SetAttributes(
        attribute.String("performance.processor", "enabled"),
        attribute.Int64("performance.start_time", time.Now().UnixNano()),
    )
}

func (p *PerformanceSpanProcessor) OnEnd(s trace.ReadOnlySpan) {
    duration := s.EndTime().Sub(s.StartTime())
    
    // 【遅いスパンの検出】
    if duration > p.slowThreshold {
        // アラート送信
        alert := SlowSpanAlert{
            TraceID:   s.SpanContext().TraceID().String(),
            SpanID:    s.SpanContext().SpanID().String(),
            SpanName:  s.Name(),
            Duration:  duration,
            Threshold: p.slowThreshold,
            Timestamp: time.Now(),
        }
        p.alertManager.SendAlert(alert)
        
        // 詳細ログ出力
        log.Printf("⏰ SLOW SPAN DETECTED: %s took %v (threshold: %v)", 
            s.Name(), duration, p.slowThreshold)
    }
    
    // 【メトリクス記録】
    spanDurationHistogram.WithLabelValues(
        s.Name(),
        s.Status().Code.String(),
    ).Observe(duration.Seconds())
}

func (p *PerformanceSpanProcessor) Shutdown(ctx context.Context) error {
    return nil
}

func (p *PerformanceSpanProcessor) ForceFlush(ctx context.Context) error {
    return nil
}

// 【セキュリティスパンプロセッサー】機密情報の自動マスキング
type SecuritySpanProcessor struct {
    sensitiveKeys []string
}

func NewSecuritySpanProcessor() *SecuritySpanProcessor {
    return &SecuritySpanProcessor{
        sensitiveKeys: []string{
            "password", "token", "secret", "key", "authorization",
            "credit_card", "ssn", "email", "phone", "address",
        },
    }
}

func (p *SecuritySpanProcessor) OnStart(parent context.Context, s trace.ReadWriteSpan) {
    // セキュリティメタデータ追加
    s.SetAttributes(
        attribute.String("security.processor", "enabled"),
        attribute.Bool("security.data_masked", false),
    )
}

func (p *SecuritySpanProcessor) OnEnd(s trace.ReadOnlySpan) {
    // 【機密情報マスキング】
    maskedCount := 0
    for _, attr := range s.Attributes() {
        for _, sensitiveKey := range p.sensitiveKeys {
            if strings.Contains(strings.ToLower(string(attr.Key)), sensitiveKey) {
                // 機密情報をマスキングしたスパンのコピーを作成
                maskedCount++
                log.Printf("🔒 SENSITIVE DATA MASKED: %s in span %s", 
                    attr.Key, s.Name())
            }
        }
    }
    
    if maskedCount > 0 {
        // セキュリティメトリクス更新
        securityMaskingCounter.WithLabelValues(
            s.Name(),
            "sensitive_data_masked",
        ).Add(float64(maskedCount))
    }
}

func (p *SecuritySpanProcessor) Shutdown(ctx context.Context) error {
    return nil
}

func (p *SecuritySpanProcessor) ForceFlush(ctx context.Context) error {
    return nil
}

// 【ユーティリティ関数】システム情報取得
func generateInstanceID() string {
    hostname, _ := os.Hostname()
    return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}

func getEnvironment() string {
    if env := os.Getenv("ENVIRONMENT"); env != "" {
        return env
    }
    return "development"
}

func getHostname() string {
    hostname, _ := os.Hostname()
    return hostname
}

// 【アラート管理】スパンベースのアラートシステム
type SlowSpanAlert struct {
    TraceID   string        `json:"trace_id"`
    SpanID    string        `json:"span_id"`
    SpanName  string        `json:"span_name"`
    Duration  time.Duration `json:"duration"`
    Threshold time.Duration `json:"threshold"`
    Timestamp time.Time     `json:"timestamp"`
}

type AlertManager struct {
    alerts chan SlowSpanAlert
}

func NewAlertManager() *AlertManager {
    am := &AlertManager{
        alerts: make(chan SlowSpanAlert, 100),
    }
    go am.processAlerts()
    return am
}

func (am *AlertManager) SendAlert(alert SlowSpanAlert) {
    select {
    case am.alerts <- alert:
        // アラート送信成功
    default:
        log.Printf("⚠️  Alert queue full, dropping alert for span: %s", alert.SpanName)
    }
}

func (am *AlertManager) processAlerts() {
    for alert := range am.alerts {
        // 実際の実装ではSlack、PagerDuty、メール通知など
        log.Printf("🚨 PERFORMANCE ALERT: %+v", alert)
    }
}

// 【メトリクス定義】Prometheusメトリクスとの連携
var (
    spanDurationHistogram = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "opentelemetry_span_duration_seconds",
            Help: "Duration of OpenTelemetry spans",
            Buckets: prometheus.DefBuckets,
        },
        []string{"span_name", "status_code"},
    )
    
    securityMaskingCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "opentelemetry_security_masked_total",
            Help: "Total number of masked sensitive data in spans",
        },
        []string{"span_name", "mask_type"},
    )
)

func init() {
    prometheus.MustRegister(spanDurationHistogram)
    prometheus.MustRegister(securityMaskingCounter)
}
```

#### 2. 手動インストルメンテーション

```go
// 【包括的なトレーシングサービス】プロダクショングレードのトレーシング実装
type TraceableService struct {
    tracer        trace.Tracer
    db           *sql.DB
    cache        *redis.Client
    serviceName  string
    
    // 【拡張機能】
    metrics      *TraceMetrics
    config       *ServiceConfig
    validator    *Validator
    circuitBreaker *CircuitBreaker
}

// 【サービス設定】
 type ServiceConfig struct {
    EnableCaching     bool          `json:"enable_caching"`
    CacheTimeout      time.Duration `json:"cache_timeout"`
    DBTimeout         time.Duration `json:"db_timeout"`
    MaxRetries        int           `json:"max_retries"`
    EnableCircuitBreaker bool       `json:"enable_circuit_breaker"`
    SlowQueryThreshold time.Duration `json:"slow_query_threshold"`
}

func NewTraceableService(serviceName string, db *sql.DB, cache *redis.Client) *TraceableService {
    return &TraceableService{
        tracer:      otel.Tracer(serviceName),
        db:         db,
        cache:      cache,
        serviceName: serviceName,
        metrics:    NewTraceMetrics(),
        config: &ServiceConfig{
            EnableCaching:      true,
            CacheTimeout:       5 * time.Minute,
            DBTimeout:         10 * time.Second,
            MaxRetries:        3,
            EnableCircuitBreaker: true,
            SlowQueryThreshold: 100 * time.Millisecond,
        },
        validator:     NewValidator(),
        circuitBreaker: NewCircuitBreaker(),
    }
}

// 【メインオペレーション】包括的なユーザー作成処理
func (s *TraceableService) CreateUser(ctx context.Context, user *User) error {
    // 【メインスパン】ビジネスオペレーション全体をカバー
    ctx, span := s.tracer.Start(ctx, "CreateUser",
        trace.WithSpanKind(trace.SpanKindServer),
        trace.WithAttributes(
            // 【ユーザー情報】マスキングを考慮した情報記録
            attribute.String("user.id", user.ID),
            attribute.String("user.email_hash", hashEmail(user.Email)), // メールアドレスはハッシュ化
            attribute.String("operation", "user_creation"),
            attribute.String("service.name", s.serviceName),
            
            // 【システム情報】
            attribute.String("system.version", "1.0.0"),
            attribute.String("request.id", getRequestID(ctx)),
            attribute.String("correlation.id", getCorrelationID(ctx)),
            
            // 【ビジネスコンテキスト】
            attribute.String("user.type", determineUserType(user)),
            attribute.String("registration.source", getRegistrationSource(ctx)),
            attribute.Bool("user.premium", user.IsPremium),
        ),
    )
    defer span.End()

    // 【スパンメタデータ拡張】
    defer func() {
        if r := recover(); r != nil {
            span.RecordError(fmt.Errorf("panic: %v", r))
            span.SetStatus(codes.Error, "operation panicked")
            span.SetAttributes(
                attribute.String("error.type", "panic"),
                attribute.String("error.stack", string(debug.Stack())),
            )
            panic(r) // re-panic
        }
    }()

    // 【STEP 1】バリデーション処理
    if err := s.validateUser(ctx, user); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "validation failed")
        span.SetAttributes(
            attribute.String("error.phase", "validation"),
            attribute.String("error.detail", err.Error()),
        )
        return fmt.Errorf("validation error: %w", err)
    }

    // 【STEP 2】重複チェック
    if err := s.checkUserExists(ctx, user); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "duplicate user check failed")
        return fmt.Errorf("duplicate check error: %w", err)
    }

    // 【STEP 3】データベース操作
    if err := s.insertUser(ctx, user); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "database insertion failed")
        span.SetAttributes(
            attribute.String("error.phase", "database_insertion"),
            attribute.String("error.detail", err.Error()),
        )
        return fmt.Errorf("database error: %w", err)
    }

    // 【STEP 4】キャッシュ更新（オプション）
    if s.config.EnableCaching {
        if err := s.updateUserCache(ctx, user); err != nil {
            // キャッシュエラーはファタルではない
            span.AddEvent("cache_update_failed",
                trace.WithAttributes(
                    attribute.String("cache.error", err.Error()),
                    attribute.String("cache.operation", "user_update"),
                ),
            )
            log.Printf("⚠️  Cache update failed: %v", err)
        }
    }

    // 【STEP 5】非同期処理起動
    s.triggerAsyncProcessing(ctx, user)

    // 【成功イベント】詳細なコンテキスト情報を記録
    span.AddEvent("user_created_successfully",
        trace.WithAttributes(
            attribute.String("user.id", user.ID),
            attribute.String("user.email_hash", hashEmail(user.Email)),
            attribute.Int64("timestamp", time.Now().Unix()),
            attribute.String("creation.method", "database_insert"),
            attribute.Bool("cache.updated", s.config.EnableCaching),
            attribute.String("next_steps", "async_processing_triggered"),
        ),
    )

    // 【メトリクス更新】
    s.metrics.UserCreations.WithLabelValues("success", user.Type).Inc()
    
    span.SetStatus(codes.Ok, "User created successfully")
    return nil
}

// 【高度なバリデーション】詳細なエラー情報とメトリクス付き
func (s *TraceableService) validateUser(ctx context.Context, user *User) error {
    ctx, span := s.tracer.Start(ctx, "ValidateUser",
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(
            attribute.String("validation.target", "user"),
            attribute.String("validation.version", "v1.0"),
        ),
    )
    defer span.End()

    validationErrors := make([]string, 0)
    validationStart := time.Now()

    // 【詳細バリデーション】
    validationRules := []struct {
        name     string
        check    func(*User) error
        severity string
    }{
        {"id_required", func(u *User) error {
            if u.ID == "" {
                return fmt.Errorf("ID is required")
            }
            return nil
        }, "critical"},
        {"id_format", func(u *User) error {
            if !isValidID(u.ID) {
                return fmt.Errorf("invalid ID format")
            }
            return nil
        }, "critical"},
        {"email_required", func(u *User) error {
            if u.Email == "" {
                return fmt.Errorf("email is required")
            }
            return nil
        }, "critical"},
        {"email_format", func(u *User) error {
            if !isValidEmail(u.Email) {
                return fmt.Errorf("invalid email format")
            }
            return nil
        }, "critical"},
        {"name_length", func(u *User) error {
            if len(u.Name) < 2 || len(u.Name) > 100 {
                return fmt.Errorf("name length must be between 2 and 100 characters")
            }
            return nil
        }, "warning"},
        {"name_content", func(u *User) error {
            if containsInvalidCharacters(u.Name) {
                return fmt.Errorf("name contains invalid characters")
            }
            return nil
        }, "warning"},
    }

    // 【ルール毎のバリデーション実行】
    for _, rule := range validationRules {
        ruleStart := time.Now()
        
        if err := rule.check(user); err != nil {
            validationErrors = append(validationErrors, err.Error())
            
            // 【ルール固有のスパンイベント】
            span.AddEvent(fmt.Sprintf("validation_rule_failed_%s", rule.name),
                trace.WithAttributes(
                    attribute.String("rule.name", rule.name),
                    attribute.String("rule.severity", rule.severity),
                    attribute.String("rule.error", err.Error()),
                    attribute.String("rule.duration", time.Since(ruleStart).String()),
                ),
            )
        }
        
        // 【ルールメトリクス】
        s.metrics.ValidationRules.WithLabelValues(
            rule.name,
            rule.severity,
            fmt.Sprintf("%t", err != nil),
        ).Inc()
    }

    // 【バリデーション結果の記録】
    validationDuration := time.Since(validationStart)
    span.SetAttributes(
        attribute.StringSlice("validation.errors", validationErrors),
        attribute.Int("validation.error_count", len(validationErrors)),
        attribute.String("validation.duration", validationDuration.String()),
        attribute.Bool("validation.success", len(validationErrors) == 0),
        attribute.Int("validation.rules_checked", len(validationRules)),
    )

    if len(validationErrors) > 0 {
        span.SetStatus(codes.Error, "validation failed")
        return fmt.Errorf("validation failed: %s", strings.Join(validationErrors, ", "))
    }

    span.SetStatus(codes.Ok, "validation passed")
    return nil
}

// 【重複チェック】キャッシュとデータベースを併用した効率的な重複確認
func (s *TraceableService) checkUserExists(ctx context.Context, user *User) error {
    ctx, span := s.tracer.Start(ctx, "CheckUserExists",
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(
            attribute.String("check.type", "duplicate_prevention"),
            attribute.String("check.target", "user"),
        ),
    )
    defer span.End()

    // 【STEP 1】キャッシュから高速チェック
    if s.config.EnableCaching {
        if exists, err := s.checkUserExistsInCache(ctx, user); err == nil {
            span.SetAttributes(
                attribute.Bool("cache.hit", true),
                attribute.Bool("user.exists", exists),
            )
            
            if exists {
                span.SetStatus(codes.Error, "user already exists (cache)")
                return fmt.Errorf("user already exists")
            }
            
            span.AddEvent("cache_check_passed",
                trace.WithAttributes(
                    attribute.String("cache.result", "user_not_found"),
                ),
            )
            return nil
        }
        
        // キャッシュエラーはフォールバック
        span.AddEvent("cache_check_failed",
            trace.WithAttributes(
                attribute.String("cache.error", err.Error()),
                attribute.String("fallback", "database_check"),
            ),
        )
    }

    // 【STEP 2】データベースでの確定的チェック
    if exists, err := s.checkUserExistsInDB(ctx, user); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "database check failed")
        return fmt.Errorf("failed to check user existence: %w", err)
    } else if exists {
        span.SetStatus(codes.Error, "user already exists (database)")
        return fmt.Errorf("user already exists")
    }

    span.SetStatus(codes.Ok, "user does not exist")
    return nil
}

// 【データベース操作】サーキットブレーカーとリトライ付き
func (s *TraceableService) insertUser(ctx context.Context, user *User) error {
    ctx, span := s.tracer.Start(ctx, "InsertUser",
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            attribute.String("db.system", "postgresql"),
            attribute.String("db.operation", "INSERT"),
            attribute.String("db.table", "users"),
            attribute.String("db.connection_pool", "main"),
            attribute.String("db.timeout", s.config.DBTimeout.String()),
        ),
    )
    defer span.End()

    // 【タイムアウト付きコンテキスト】
    dbCtx, cancel := context.WithTimeout(ctx, s.config.DBTimeout)
    defer cancel()

    // 【サーキットブレーカー保護】
    if s.config.EnableCircuitBreaker {
        return s.circuitBreaker.Execute(func() error {
            return s.performDatabaseInsertion(dbCtx, span, user)
        })
    }

    return s.performDatabaseInsertion(dbCtx, span, user)
}

// 【実際のデータベース操作】リトライとメトリクス付き
func (s *TraceableService) performDatabaseInsertion(ctx context.Context, span trace.Span, user *User) error {
    query := `INSERT INTO users (id, name, email, created_at, updated_at, user_type, is_premium) 
              VALUES ($1, $2, $3, $4, $5, $6, $7)`
    
    var lastErr error
    
    // 【リトライロジック】
    for attempt := 1; attempt <= s.config.MaxRetries; attempt++ {
        start := time.Now()
        
        _, err := s.db.ExecContext(ctx, query, 
            user.ID, user.Name, user.Email, 
            time.Now(), time.Now(), 
            user.Type, user.IsPremium)
        
        duration := time.Since(start)
        
        // 【クエリメトリクス】
        s.metrics.DBQueries.WithLabelValues(
            "INSERT", "users", fmt.Sprintf("%t", err == nil),
        ).Inc()
        
        s.metrics.DBQueryDuration.WithLabelValues(
            "INSERT", "users",
        ).Observe(duration.Seconds())
        
        // 【スパンイベント】
        span.AddEvent(fmt.Sprintf("db_query_attempt_%d", attempt),
            trace.WithAttributes(
                attribute.String("db.statement", query),
                attribute.String("db.duration", duration.String()),
                attribute.Int("db.attempt", attempt),
                attribute.Bool("db.success", err == nil),
            ),
        )
        
        if err == nil {
            // 【成功】
            span.SetAttributes(
                attribute.String("db.statement", query),
                attribute.String("db.total_duration", duration.String()),
                attribute.Int("db.attempts", attempt),
                attribute.Bool("db.slow_query", duration > s.config.SlowQueryThreshold),
            )
            
            if duration > s.config.SlowQueryThreshold {
                span.AddEvent("slow_query_detected",
                    trace.WithAttributes(
                        attribute.String("query.duration", duration.String()),
                        attribute.String("query.threshold", s.config.SlowQueryThreshold.String()),
                    ),
                )
            }
            
            span.SetStatus(codes.Ok, "User inserted successfully")
            return nil
        }
        
        lastErr = err
        
        // 【リトライ判定】
        if !isRetryableError(err) {
            break
        }
        
        if attempt < s.config.MaxRetries {
            // 指数バックオフ
            backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 100 * time.Millisecond
            time.Sleep(backoff)
            
            span.AddEvent(fmt.Sprintf("db_retry_attempt_%d", attempt),
                trace.WithAttributes(
                    attribute.String("retry.error", err.Error()),
                    attribute.String("retry.backoff", backoff.String()),
                ),
            )
        }
    }
    
    // 【最終失敗】
    span.RecordError(lastErr)
    span.SetStatus(codes.Error, "database insertion failed")
    span.SetAttributes(
        attribute.String("db.final_error", lastErr.Error()),
        attribute.Int("db.total_attempts", s.config.MaxRetries),
    )
    
    return fmt.Errorf("database insertion failed after %d attempts: %w", s.config.MaxRetries, lastErr)
}

// 【非同期処理】ユーザー作成後の後続処理
func (s *TraceableService) triggerAsyncProcessing(ctx context.Context, user *User) {
    // 【新しいコンテキスト】メインリクエストから独立した非同期処理
    go func() {
        // トレース情報を引き継ぎつつ、新しいコンテキストを作成
        asyncCtx, span := s.tracer.Start(context.Background(), "AsyncUserProcessing",
            trace.WithSpanKind(trace.SpanKindInternal),
            trace.WithAttributes(
                attribute.String("async.trigger", "user_creation"),
                attribute.String("user.id", user.ID),
                attribute.String("parent.trace_id", trace.SpanFromContext(ctx).SpanContext().TraceID().String()),
            ),
        )
        defer span.End()
        
        // 【非同期タスク実行】
        tasks := []struct{
            name string
            fn   func(context.Context, *User) error
        }{
            {"send_welcome_email", s.sendWelcomeEmail},
            {"update_analytics", s.updateAnalytics},
            {"trigger_recommendations", s.triggerRecommendations},
            {"setup_default_preferences", s.setupDefaultPreferences},
        }
        
        for _, task := range tasks {
            taskCtx, taskSpan := s.tracer.Start(asyncCtx, fmt.Sprintf("AsyncTask_%s", task.name),
                trace.WithSpanKind(trace.SpanKindInternal),
            )
            
            if err := task.fn(taskCtx, user); err != nil {
                taskSpan.RecordError(err)
                taskSpan.SetStatus(codes.Error, fmt.Sprintf("async task failed: %s", task.name))
                log.Printf("❌ Async task failed: %s for user %s: %v", task.name, user.ID, err)
            } else {
                taskSpan.SetStatus(codes.Ok, fmt.Sprintf("async task completed: %s", task.name))
            }
            
            taskSpan.End()
        }
        
        span.SetStatus(codes.Ok, "async processing completed")
    }()
}

// 【ユーティリティ関数】セキュリティとメタデータ処理
func hashEmail(email string) string {
    h := sha256.Sum256([]byte(email))
    return hex.EncodeToString(h[:8]) // 最初の8バイトを使用
}

func getRequestID(ctx context.Context) string {
    if id, ok := ctx.Value("request_id").(string); ok {
        return id
    }
    return "unknown"
}

func getCorrelationID(ctx context.Context) string {
    if id, ok := ctx.Value("correlation_id").(string); ok {
        return id
    }
    return generateCorrelationID()
}

func generateCorrelationID() string {
    return fmt.Sprintf("corr_%d_%s", time.Now().UnixNano(), generateShortID())
}

func determineUserType(user *User) string {
    if user.IsPremium {
        return "premium"
    }
    return "standard"
}

func getRegistrationSource(ctx context.Context) string {
    if source, ok := ctx.Value("registration_source").(string); ok {
        return source
    }
    return "direct"
}

func isRetryableError(err error) bool {
    // データベースエラーのリトライ判定
    errStr := strings.ToLower(err.Error())
    retryableErrors := []string{
        "connection refused", "connection reset", "timeout",
        "temporary failure", "deadlock", "lock wait timeout",
    }
    
    for _, retryable := range retryableErrors {
        if strings.Contains(errStr, retryable) {
            return true
        }
    }
    
    return false
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