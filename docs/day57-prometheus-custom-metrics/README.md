# Day 57: Prometheusによるカスタムメトリクス

## 🎯 本日の目標 (Today's Goal)

Prometheusを使用してカスタムメトリクスを実装し、アプリケーションの詳細なモニタリングと可観測性を提供するシステムを構築する。メトリクスの種類、収集、エクスポート、アラートルールを包括的に習得する。

## 📖 解説 (Explanation)

### Prometheusメトリクスの基礎

Prometheusは時系列データベースとモニタリングシステムで、4つの主要なメトリクス型を提供します：

#### 1. Counter（カウンタ）

```go
type ServiceMetrics struct {
    requestsTotal     prometheus.Counter
    errorsTotal      prometheus.Counter
    httpRequestsTotal *prometheus.CounterVec
}

func NewServiceMetrics() *ServiceMetrics {
    sm := &ServiceMetrics{
        requestsTotal: prometheus.NewCounter(prometheus.CounterOpts{
            Namespace: "myapp",
            Subsystem: "api",
            Name:     "requests_total",
            Help:     "Total number of requests processed",
        }),
        errorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
            Namespace: "myapp",
            Subsystem: "api",
            Name:     "errors_total",
            Help:     "Total number of errors occurred",
        }),
        httpRequestsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: "myapp",
                Subsystem: "http",
                Name:     "requests_total",
                Help:     "Total HTTP requests by method and status",
            },
            []string{"method", "status_code", "endpoint"},
        ),
    }
    
    // Prometheusに登録
    prometheus.MustRegister(sm.requestsTotal)
    prometheus.MustRegister(sm.errorsTotal)
    prometheus.MustRegister(sm.httpRequestsTotal)
    
    return sm
}

func (sm *ServiceMetrics) RecordRequest(method, endpoint, statusCode string) {
    sm.requestsTotal.Inc()
    sm.httpRequestsTotal.WithLabelValues(method, statusCode, endpoint).Inc()
}

func (sm *ServiceMetrics) RecordError() {
    sm.errorsTotal.Inc()
}
```

#### 2. Gauge（ゲージ）

```go
type SystemMetrics struct {
    currentConnections  prometheus.Gauge
    memoryUsage        prometheus.Gauge
    goroutineCount     prometheus.Gauge
    queueSize          *prometheus.GaugeVec
}

func NewSystemMetrics() *SystemMetrics {
    sm := &SystemMetrics{
        currentConnections: prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: "myapp",
            Subsystem: "system",
            Name:     "connections_current",
            Help:     "Current number of active connections",
        }),
        memoryUsage: prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: "myapp",
            Subsystem: "system", 
            Name:     "memory_usage_bytes",
            Help:     "Current memory usage in bytes",
        }),
        goroutineCount: prometheus.NewGauge(prometheus.GaugeOpts{
            Namespace: "myapp",
            Subsystem: "system",
            Name:     "goroutines_current",
            Help:     "Current number of goroutines",
        }),
        queueSize: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Namespace: "myapp",
                Subsystem: "queues",
                Name:     "size_current",
                Help:     "Current queue size by queue name",
            },
            []string{"queue_name"},
        ),
    }
    
    prometheus.MustRegister(sm.currentConnections)
    prometheus.MustRegister(sm.memoryUsage)
    prometheus.MustRegister(sm.goroutineCount)
    prometheus.MustRegister(sm.queueSize)
    
    return sm
}

func (sm *SystemMetrics) UpdateConnections(count float64) {
    sm.currentConnections.Set(count)
}

func (sm *SystemMetrics) UpdateMemoryUsage(bytes float64) {
    sm.memoryUsage.Set(bytes)
}

func (sm *SystemMetrics) UpdateGoroutineCount() {
    sm.goroutineCount.Set(float64(runtime.NumGoroutine()))
}

func (sm *SystemMetrics) UpdateQueueSize(queueName string, size float64) {
    sm.queueSize.WithLabelValues(queueName).Set(size)
}
```

#### 3. Histogram（ヒストグラム）

```go
type RequestMetrics struct {
    requestDuration    *prometheus.HistogramVec
    requestSize        *prometheus.HistogramVec
    responseSize       *prometheus.HistogramVec
}

func NewRequestMetrics() *RequestMetrics {
    rm := &RequestMetrics{
        requestDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Namespace: "myapp",
                Subsystem: "http",
                Name:     "request_duration_seconds",
                Help:     "HTTP request duration in seconds",
                Buckets:  []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
            },
            []string{"method", "endpoint", "status_code"},
        ),
        requestSize: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Namespace: "myapp",
                Subsystem: "http",
                Name:     "request_size_bytes",
                Help:     "HTTP request size in bytes",
                Buckets:  prometheus.ExponentialBuckets(100, 10, 8), // 100B to 100MB
            },
            []string{"method", "endpoint"},
        ),
        responseSize: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Namespace: "myapp",
                Subsystem: "http",
                Name:     "response_size_bytes",
                Help:     "HTTP response size in bytes",
                Buckets:  prometheus.ExponentialBuckets(100, 10, 8),
            },
            []string{"method", "endpoint", "status_code"},
        ),
    }
    
    prometheus.MustRegister(rm.requestDuration)
    prometheus.MustRegister(rm.requestSize)
    prometheus.MustRegister(rm.responseSize)
    
    return rm
}

func (rm *RequestMetrics) RecordRequestDuration(method, endpoint, statusCode string, duration time.Duration) {
    rm.requestDuration.WithLabelValues(method, endpoint, statusCode).Observe(duration.Seconds())
}

func (rm *RequestMetrics) RecordRequestSize(method, endpoint string, size float64) {
    rm.requestSize.WithLabelValues(method, endpoint).Observe(size)
}

func (rm *RequestMetrics) RecordResponseSize(method, endpoint, statusCode string, size float64) {
    rm.responseSize.WithLabelValues(method, endpoint, statusCode).Observe(size)
}
```

#### 4. Summary（サマリー）

```go
type ProcessingMetrics struct {
    taskDuration    *prometheus.SummaryVec
    taskComplexity  *prometheus.SummaryVec
}

func NewProcessingMetrics() *ProcessingMetrics {
    pm := &ProcessingMetrics{
        taskDuration: prometheus.NewSummaryVec(
            prometheus.SummaryOpts{
                Namespace:  "myapp",
                Subsystem:  "tasks",
                Name:      "duration_seconds",
                Help:      "Task processing duration in seconds",
                Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
                MaxAge:     time.Hour,
                AgeBuckets: 5,
                BufCap:     500,
            },
            []string{"task_type", "worker_id"},
        ),
        taskComplexity: prometheus.NewSummaryVec(
            prometheus.SummaryOpts{
                Namespace:  "myapp",
                Subsystem:  "tasks",
                Name:      "complexity_score",
                Help:      "Task complexity score",
                Objectives: map[float64]float64{0.5: 0.05, 0.95: 0.01, 0.99: 0.001},
            },
            []string{"task_type"},
        ),
    }
    
    prometheus.MustRegister(pm.taskDuration)
    prometheus.MustRegister(pm.taskComplexity)
    
    return pm
}

func (pm *ProcessingMetrics) RecordTaskDuration(taskType, workerID string, duration time.Duration) {
    pm.taskDuration.WithLabelValues(taskType, workerID).Observe(duration.Seconds())
}

func (pm *ProcessingMetrics) RecordTaskComplexity(taskType string, complexity float64) {
    pm.taskComplexity.WithLabelValues(taskType).Observe(complexity)
}
```

### HTTPミドルウェアとの統合

```go
type MetricsMiddleware struct {
    requestMetrics *RequestMetrics
    serviceMetrics *ServiceMetrics
}

func NewMetricsMiddleware(reqMetrics *RequestMetrics, svcMetrics *ServiceMetrics) *MetricsMiddleware {
    return &MetricsMiddleware{
        requestMetrics: reqMetrics,
        serviceMetrics: svcMetrics,
    }
}

func (mm *MetricsMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // リクエストサイズを記録
        if r.ContentLength > 0 {
            mm.requestMetrics.RecordRequestSize(r.Method, r.URL.Path, float64(r.ContentLength))
        }
        
        // レスポンスライターをラップ
        wrapped := &responseWriter{
            ResponseWriter: w,
            statusCode:     200,
            responseSize:   0,
        }
        
        // 次のハンドラーを実行
        next.ServeHTTP(wrapped, r)
        
        // メトリクスを記録
        duration := time.Since(start)
        statusCode := strconv.Itoa(wrapped.statusCode)
        
        mm.serviceMetrics.RecordRequest(r.Method, r.URL.Path, statusCode)
        mm.requestMetrics.RecordRequestDuration(r.Method, r.URL.Path, statusCode, duration)
        mm.requestMetrics.RecordResponseSize(r.Method, r.URL.Path, statusCode, float64(wrapped.responseSize))
        
        if wrapped.statusCode >= 400 {
            mm.serviceMetrics.RecordError()
        }
    })
}

type responseWriter struct {
    http.ResponseWriter
    statusCode   int
    responseSize int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    size, err := rw.ResponseWriter.Write(b)
    rw.responseSize += size
    return size, err
}
```

### カスタムコレクタ

```go
type CustomCollector struct {
    appInfo     *prometheus.Desc
    uptime      *prometheus.Desc
    version     string
    startTime   time.Time
    buildInfo   map[string]string
}

func NewCustomCollector(version string, buildInfo map[string]string) *CustomCollector {
    return &CustomCollector{
        appInfo: prometheus.NewDesc(
            "myapp_info",
            "Application information",
            []string{"version", "commit", "build_date"},
            nil,
        ),
        uptime: prometheus.NewDesc(
            "myapp_uptime_seconds",
            "Application uptime in seconds",
            nil,
            nil,
        ),
        version:   version,
        startTime: time.Now(),
        buildInfo: buildInfo,
    }
}

func (cc *CustomCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- cc.appInfo
    ch <- cc.uptime
}

func (cc *CustomCollector) Collect(ch chan<- prometheus.Metric) {
    // アプリケーション情報
    ch <- prometheus.MustNewConstMetric(
        cc.appInfo,
        prometheus.GaugeValue,
        1,
        cc.version,
        cc.buildInfo["commit"],
        cc.buildInfo["build_date"],
    )
    
    // アップタイム
    uptime := time.Since(cc.startTime).Seconds()
    ch <- prometheus.MustNewConstMetric(
        cc.uptime,
        prometheus.GaugeValue,
        uptime,
    )
}
```

### メトリクス管理システム

```go
type MetricsManager struct {
    registry     *prometheus.Registry
    pushGateway  *push.Pusher
    gatherer     prometheus.Gatherer
    collectors   []prometheus.Collector
    config       *MetricsConfig
    mu           sync.RWMutex
}

type MetricsConfig struct {
    Namespace       string
    EnablePush      bool
    PushInterval    time.Duration
    PushGatewayURL  string
    JobName         string
    InstanceID      string
    DefaultLabels   map[string]string
}

func NewMetricsManager(config *MetricsConfig) *MetricsManager {
    registry := prometheus.NewRegistry()
    
    mm := &MetricsManager{
        registry:   registry,
        gatherer:   registry,
        collectors: make([]prometheus.Collector, 0),
        config:     config,
    }
    
    if config.EnablePush {
        mm.setupPushGateway()
    }
    
    return mm
}

func (mm *MetricsManager) setupPushGateway() {
    mm.pushGateway = push.New(mm.config.PushGatewayURL, mm.config.JobName).
        Collector(mm.gatherer).
        Grouping("instance", mm.config.InstanceID)
    
    // デフォルトラベルを追加
    for key, value := range mm.config.DefaultLabels {
        mm.pushGateway = mm.pushGateway.Grouping(key, value)
    }
}

func (mm *MetricsManager) RegisterCollector(collector prometheus.Collector) error {
    mm.mu.Lock()
    defer mm.mu.Unlock()
    
    if err := mm.registry.Register(collector); err != nil {
        return fmt.Errorf("failed to register collector: %w", err)
    }
    
    mm.collectors = append(mm.collectors, collector)
    return nil
}

func (mm *MetricsManager) StartPushMetrics(ctx context.Context) {
    if !mm.config.EnablePush || mm.pushGateway == nil {
        return
    }
    
    ticker := time.NewTicker(mm.config.PushInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := mm.pushGateway.Push(); err != nil {
                log.Printf("Failed to push metrics: %v", err)
            }
        case <-ctx.Done():
            // 最後にメトリクスをプッシュ
            if err := mm.pushGateway.Push(); err != nil {
                log.Printf("Failed to push final metrics: %v", err)
            }
            return
        }
    }
}

func (mm *MetricsManager) GetHandler() http.Handler {
    return promhttp.HandlerFor(mm.gatherer, promhttp.HandlerOpts{
        EnableOpenMetrics: true,
        Registry:         mm.registry,
    })
}

func (mm *MetricsManager) Gather() ([]*dto.MetricFamily, error) {
    return mm.gatherer.Gather()
}
```

## 📝 課題 (The Problem)

以下の機能を持つ包括的なPrometheusメトリクスシステムを実装してください：

### 1. 基本メトリクス
- Counter、Gauge、Histogram、Summaryの実装
- ラベル付きメトリクス
- カスタムバケットとパーセンタイル

### 2. HTTPメトリクス
- リクエスト数、エラー率、レスポンス時間
- リクエスト/レスポンスサイズ
- エンドポイント別の統計

### 3. システムメトリクス
- メモリ使用量、CPU使用率
- Goroutine数、接続数
- カスタムビジネスメトリクス

### 4. エクスポート機能
- HTTP エンドポイント経由の公開
- Push Gateway への送信
- メトリクス管理システム

### 5. アラート機能
- 閾値ベースのアラート
- メトリクス異常検知
- 通知システム連携

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestMetricsCollection
    main_test.go:45: Metrics collection working correctly
--- PASS: TestMetricsCollection (0.01s)

=== RUN   TestHTTPMetricsMiddleware
    main_test.go:65: HTTP metrics middleware functioning
--- PASS: TestHTTPMetricsMiddleware (0.02s)

=== RUN   TestCustomCollector
    main_test.go:85: Custom collector implementation correct
--- PASS: TestCustomCollector (0.01s)

PASS
ok      day57-prometheus-custom-metrics   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### メトリクス初期化

```go
func initializeMetrics() {
    // グローバル登録を避けるため、独自のレジストリを使用
    registry := prometheus.NewRegistry()
    
    // デフォルトメトリクス（Go runtime情報など）を追加
    registry.MustRegister(prometheus.NewGoCollector())
    registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
}
```

### レスポンスライターのラップ

```go
type instrumentedResponseWriter struct {
    http.ResponseWriter
    statusCode int
    size      int
}

func (w *instrumentedResponseWriter) WriteHeader(code int) {
    w.statusCode = code
    w.ResponseWriter.WriteHeader(code)
}

func (w *instrumentedResponseWriter) Write(b []byte) (int, error) {
    size, err := w.ResponseWriter.Write(b)
    w.size += size
    return size, err
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **分散トレーシング統合**: Jaegerとの連携
2. **動的メトリクス**: 実行時のメトリクス追加/削除
3. **メトリクス集約**: 複数インスタンスからの集約
4. **異常検知**: 機械学習ベースの異常検知
5. **ダッシュボード生成**: Grafanaダッシュボードの自動生成

Prometheusカスタムメトリクスの実装を通じて、本格的な可観測性システムの構築手法を習得しましょう！