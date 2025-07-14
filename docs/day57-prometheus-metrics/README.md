# Day 57: Prometheus Custom Metrics

## 🎯 本日の目標 (Today's Goal)

Prometheusカスタムメトリクスを実装し、HTTPリクエスト数、エラー率、レスポンス時間などのビジネスメトリクスを収集・公開する仕組みを習得する。プロダクションレベルの監視とアラートの基盤を構築する。

## 📖 解説 (Explanation)

### Prometheusとは

Prometheusは、SoundCloudで開発されたオープンソースの監視・アラートシステムです。時系列データベースとして設計されており、マイクロサービスやクラウドネイティブなアプリケーションの監視に特化しています。

#### Prometheusの特徴

**Pull型アーキテクチャ**
- Prometheusサーバーが各サービスからメトリクスを定期的に取得
- ネットワーク障害時の耐性が高い
- サービス側の設定が簡単

**PromQL（Prometheus Query Language）**
- 柔軟なクエリ言語でメトリクスを分析
- 集計、フィルタリング、計算が可能
- アラート条件の定義に使用

**ラベルベースのデータモデル**
```
http_requests_total{method="GET", endpoint="/api/users", status="200"} 1234
http_requests_total{method="POST", endpoint="/api/orders", status="201"} 567
```

### メトリクスの種類

Prometheusでは4つの基本的なメトリクス型を提供しています：

#### 1. Counter（カウンター）

単調増加する累積メトリクス。値は増加のみで、リセット時は0に戻ります。

**使用例:**
- HTTPリクエスト総数
- エラー総数
- 送信バイト数

```go
import "github.com/prometheus/client_golang/prometheus"

// シンプルなカウンター
var totalRequests = prometheus.NewCounter(prometheus.CounterOpts{
    Name: "http_requests_total",
    Help: "Total number of HTTP requests",
})

// ラベル付きカウンター
var requestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total number of HTTP requests",
    },
    []string{"method", "endpoint", "status"},
)

// 使用方法
totalRequests.Inc()                                           // 1増加
requestsTotal.WithLabelValues("GET", "/api/users", "200").Inc() // ラベル付きで1増加
requestsTotal.WithLabelValues("POST", "/api/orders", "201").Add(5) // 5増加
```

#### 2. Gauge（ゲージ）

現在の値を表すメトリクス。増減両方が可能で、スナップショット的な値を表します。

**使用例:**
- CPU使用率
- メモリ使用量
- アクティブな接続数
- キューのサイズ

```go
// シンプルなゲージ
var cpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
    Name: "cpu_usage_percent",
    Help: "Current CPU usage percentage",
})

// ラベル付きゲージ
var activeConnections = prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "active_connections",
        Help: "Number of active connections",
    },
    []string{"service", "protocol"},
)

// 使用方法
cpuUsage.Set(75.5)                                    // 値を設定
cpuUsage.Inc()                                        // 1増加
cpuUsage.Dec()                                        // 1減少
cpuUsage.Add(10.5)                                    // 10.5増加
activeConnections.WithLabelValues("api", "http").Set(100) // ラベル付きで設定
```

#### 3. Histogram（ヒストグラム）

観測値を事前定義されたバケットに分類して、分布を測定します。

**自動的に生成されるメトリクス:**
- `<name>_bucket{le="<bucket>"}` - 各バケットの累積カウント
- `<name>_sum` - 全観測値の合計
- `<name>_count` - 観測回数の総数

**使用例:**
- HTTPリクエストの応答時間
- ファイルサイズ
- 処理時間

```go
// カスタムバケット定義
var requestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "http_request_duration_seconds",
        Help: "HTTP request duration in seconds",
        Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10}, // カスタムバケット
    },
    []string{"method", "endpoint"},
)

// デフォルトバケット使用
var processingTime = prometheus.NewHistogram(prometheus.HistogramOpts{
    Name: "task_processing_seconds",
    Help: "Time spent processing tasks",
    Buckets: prometheus.DefBuckets, // [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
})

// 使用方法
start := time.Now()
// ... 処理 ...
duration := time.Since(start).Seconds()
requestDuration.WithLabelValues("GET", "/api/users").Observe(duration)
```

#### 4. Summary（サマリー）

クォンタイル（パーセンタイル）を計算するメトリクス。クライアントサイドで計算されます。

**自動的に生成されるメトリクス:**
- `<name>{quantile="<φ>"}` - φ-quantile (0 ≤ φ ≤ 1)
- `<name>_sum` - 全観測値の合計
- `<name>_count` - 観測回数の総数

```go
var responseSummary = prometheus.NewSummaryVec(
    prometheus.SummaryOpts{
        Name: "http_response_time_seconds",
        Help: "HTTP response time in seconds",
        Objectives: map[float64]float64{
            0.5:  0.05,  // 50パーセンタイル、誤差5%
            0.9:  0.01,  // 90パーセンタイル、誤差1%
            0.99: 0.001, // 99パーセンタイル、誤差0.1%
        },
    },
    []string{"method"},
)

// 使用方法
responseSummary.WithLabelValues("GET").Observe(0.25)
```

### メトリクスの登録と公開

```go
package main

import (
    "log"
    "net/http"
    
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    // メトリクス定義
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )
    
    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint"},
    )
    
    activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "active_connections",
        Help: "Number of active connections",
    })
)

func init() {
    // メトリクスをPrometheusに登録
    prometheus.MustRegister(httpRequestsTotal)
    prometheus.MustRegister(httpRequestDuration)
    prometheus.MustRegister(activeConnections)
}

func main() {
    // メトリクス公開エンドポイント
    http.Handle("/metrics", promhttp.Handler())
    
    // アプリケーションエンドポイント
    http.HandleFunc("/api/users", metricsMiddleware(usersHandler))
    
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### ミドルウェアパターン

HTTPリクエストを自動的にメトリクス収集するミドルウェアの実装：

```go
func metricsMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // アクティブ接続数増加
        activeConnections.Inc()
        defer activeConnections.Dec()
        
        // レスポンスライターをラップしてステータスコードを取得
        ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        // 次のハンドラを実行
        next(ww, r)
        
        // メトリクス記録
        duration := time.Since(start).Seconds()
        status := fmt.Sprintf("%d", ww.statusCode)
        
        httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
        httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
    }
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}
```

### ビジネスメトリクス

アプリケーション固有のビジネス指標の実装例：

```go
var (
    // ユーザー関連メトリクス
    totalUsers = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "total_users",
        Help: "Total number of registered users",
    })
    
    userRegistrations = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "user_registrations_total",
        Help: "Total number of user registrations",
    })
    
    // 注文関連メトリクス
    totalOrders = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "orders_total",
            Help: "Total number of orders",
        },
        []string{"status"},
    )
    
    orderValue = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "order_value_dollars",
            Help: "Order value in dollars",
            Buckets: []float64{10, 50, 100, 250, 500, 1000, 2500, 5000},
        },
        []string{"currency"},
    )
    
    // システムメトリクス
    databaseConnections = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "database_connections",
            Help: "Number of database connections",
        },
        []string{"state"}, // active, idle
    )
    
    cacheHitRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cache_hit_rate",
            Help: "Cache hit rate percentage",
        },
        []string{"cache_type"},
    )
)

// ビジネスロジック内での使用例
func createUser(user *User) error {
    err := userService.Create(user)
    if err == nil {
        userRegistrations.Inc()
        totalUsers.Inc()
    }
    return err
}

func createOrder(order *Order) error {
    err := orderService.Create(order)
    if err == nil {
        totalOrders.WithLabelValues("created").Inc()
        orderValue.WithLabelValues(order.Currency).Observe(order.Amount)
    }
    return err
}
```

### カスタムCollectorの実装

動的にメトリクスを収集する場合のカスタムCollector：

```go
type DBStatsCollector struct {
    db *sql.DB
    
    openConnections *prometheus.Desc
    inUseConnections *prometheus.Desc
    idleConnections *prometheus.Desc
}

func NewDBStatsCollector(db *sql.DB) *DBStatsCollector {
    return &DBStatsCollector{
        db: db,
        openConnections: prometheus.NewDesc(
            "database_open_connections",
            "Number of open database connections",
            nil, nil,
        ),
        inUseConnections: prometheus.NewDesc(
            "database_in_use_connections", 
            "Number of in-use database connections",
            nil, nil,
        ),
        idleConnections: prometheus.NewDesc(
            "database_idle_connections",
            "Number of idle database connections", 
            nil, nil,
        ),
    }
}

func (c *DBStatsCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- c.openConnections
    ch <- c.inUseConnections  
    ch <- c.idleConnections
}

func (c *DBStatsCollector) Collect(ch chan<- prometheus.Metric) {
    stats := c.db.Stats()
    
    ch <- prometheus.MustNewConstMetric(
        c.openConnections,
        prometheus.GaugeValue,
        float64(stats.OpenConnections),
    )
    
    ch <- prometheus.MustNewConstMetric(
        c.inUseConnections,
        prometheus.GaugeValue,
        float64(stats.InUse),
    )
    
    ch <- prometheus.MustNewConstMetric(
        c.idleConnections,
        prometheus.GaugeValue,
        float64(stats.Idle),
    )
}

// 登録
prometheus.MustRegister(NewDBStatsCollector(db))
```

### メトリクスのベストプラクティス

#### 1. ネーミング規則

```go
// 良い例
http_requests_total          // カウンター
http_request_duration_seconds // ヒストグラム（単位付き）
process_cpu_usage_percent    // ゲージ（単位付き）

// 悪い例
requests        // あいまい
req_time        // 単位不明
http_req_cnt    // 省略形
```

#### 2. ラベルの設計

```go
// 良い例 - カーディナリティが制御されている
requestsTotal.WithLabelValues("GET", "/api/users", "200")

// 悪い例 - 高カーディナリティ（ユーザーIDごとに無限にメトリクスが増える）
requestsTotal.WithLabelValues("GET", "/api/users/12345", "200")
```

#### 3. パフォーマンス考慮

```go
// 効率的なメトリクス更新
var (
    mu sync.RWMutex
    labelCache = make(map[string]prometheus.Counter)
)

func getOrCreateCounter(method, endpoint, status string) prometheus.Counter {
    key := fmt.Sprintf("%s:%s:%s", method, endpoint, status)
    
    mu.RLock()
    counter, exists := labelCache[key]
    mu.RUnlock()
    
    if exists {
        return counter
    }
    
    mu.Lock()
    defer mu.Unlock()
    
    // ダブルチェック
    if counter, exists := labelCache[key]; exists {
        return counter
    }
    
    counter = requestsTotal.WithLabelValues(method, endpoint, status)
    labelCache[key] = counter
    return counter
}
```

## 📝 課題 (The Problem)

以下の機能を持つPrometheusメトリクスシステムを実装してください：

### 1. HTTPメトリクス収集

```go
type HTTPMetrics struct {
    RequestsTotal    *prometheus.CounterVec   // リクエスト総数
    RequestDuration  *prometheus.HistogramVec // レスポンス時間
    ActiveRequests   *prometheus.GaugeVec     // アクティブリクエスト数
    ErrorsTotal      *prometheus.CounterVec   // エラー総数
}
```

### 2. ビジネスメトリクス

- **ユーザーメトリクス**: 登録数、アクティブユーザー数
- **注文メトリクス**: 注文数、売上、平均注文額
- **商品メトリクス**: 在庫数、人気商品ランキング

### 3. システムメトリクス

- **データベース**: 接続数、クエリ時間、エラー率
- **キャッシュ**: ヒット率、ミス率、アイテム数
- **外部API**: 呼び出し回数、レスポンス時間、エラー率

### 4. カスタムCollector

動的にメトリクスを収集するCollectorの実装

### 5. メトリクス公開エンドポイント

`/metrics`エンドポイントでのPrometheus形式での公開

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestHTTPMetrics_RequestCounter
    main_test.go:45: HTTP request counter incremented correctly
--- PASS: TestHTTPMetrics_RequestCounter (0.01s)

=== RUN   TestHTTPMetrics_ResponseTime
    main_test.go:65: Response time histogram recorded correctly
--- PASS: TestHTTPMetrics_ResponseTime (0.01s)

=== RUN   TestBusinessMetrics_UserRegistration
    main_test.go:85: User registration metrics updated correctly
--- PASS: TestBusinessMetrics_UserRegistration (0.01s)

=== RUN   TestCustomCollector_DatabaseStats
    main_test.go:105: Custom database collector working correctly
--- PASS: TestCustomCollector_DatabaseStats (0.02s)

=== RUN   TestMetricsEndpoint_PrometheusFormat
    main_test.go:125: /metrics endpoint returns valid Prometheus format
--- PASS: TestMetricsEndpoint_PrometheusFormat (0.03s)

PASS
ok      day57-prometheus-metrics   0.156s
```

### メトリクス出力例

```
# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="GET",endpoint="/api/users",status="200"} 1234
http_requests_total{method="POST",endpoint="/api/orders",status="201"} 567

# HELP http_request_duration_seconds HTTP request duration in seconds
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",le="0.1"} 800
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",le="0.5"} 1200
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",le="+Inf"} 1234
http_request_duration_seconds_sum{method="GET",endpoint="/api/users"} 123.45
http_request_duration_seconds_count{method="GET",endpoint="/api/users"} 1234

# HELP total_users Total number of registered users
# TYPE total_users gauge
total_users 10543
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 必要なパッケージ

```go
import (
    "net/http"
    "time"
    "fmt"
    "sync"
    
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)
```

### メトリクス初期化パターン

```go
func NewHTTPMetrics() *HTTPMetrics {
    return &HTTPMetrics{
        RequestsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_total",
                Help: "Total number of HTTP requests",
            },
            []string{"method", "endpoint", "status"},
        ),
        RequestDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_request_duration_seconds",
                Help: "HTTP request duration in seconds",
                Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10},
            },
            []string{"method", "endpoint"},
        ),
        ActiveRequests: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "http_active_requests",
                Help: "Number of active HTTP requests",
            },
            []string{"endpoint"},
        ),
        ErrorsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_errors_total", 
                Help: "Total number of HTTP errors",
            },
            []string{"method", "endpoint", "status"},
        ),
    }
}
```

### ミドルウェア実装

```go
func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        endpoint := r.URL.Path
        
        // アクティブリクエスト数増加
        m.ActiveRequests.WithLabelValues(endpoint).Inc()
        defer m.ActiveRequests.WithLabelValues(endpoint).Dec()
        
        // レスポンスライターをラップ
        ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        // 次のハンドラを実行
        next.ServeHTTP(ww, r)
        
        // メトリクス記録
        duration := time.Since(start).Seconds()
        status := fmt.Sprintf("%d", ww.statusCode)
        
        m.RequestsTotal.WithLabelValues(r.Method, endpoint, status).Inc()
        m.RequestDuration.WithLabelValues(r.Method, endpoint).Observe(duration)
        
        if ww.statusCode >= 400 {
            m.ErrorsTotal.WithLabelValues(r.Method, endpoint, status).Inc()
        }
    })
}
```

### カスタムCollector例

```go
type SystemMetricsCollector struct {
    cpuUsage    *prometheus.Desc
    memoryUsage *prometheus.Desc
}

func NewSystemMetricsCollector() *SystemMetricsCollector {
    return &SystemMetricsCollector{
        cpuUsage: prometheus.NewDesc(
            "system_cpu_usage_percent",
            "System CPU usage percentage",
            nil, nil,
        ),
        memoryUsage: prometheus.NewDesc(
            "system_memory_usage_bytes",
            "System memory usage in bytes", 
            nil, nil,
        ),
    }
}

func (c *SystemMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- c.cpuUsage
    ch <- c.memoryUsage
}

func (c *SystemMetricsCollector) Collect(ch chan<- prometheus.Metric) {
    // システム情報を取得（実装は省略）
    cpuPercent := getCurrentCPUUsage()
    memoryBytes := getCurrentMemoryUsage()
    
    ch <- prometheus.MustNewConstMetric(
        c.cpuUsage,
        prometheus.GaugeValue,
        cpuPercent,
    )
    
    ch <- prometheus.MustNewConstMetric(
        c.memoryUsage,
        prometheus.GaugeValue,
        float64(memoryBytes),
    )
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **Alerting Rules**: Prometheusアラートルールの定義
2. **Service Discovery**: 動的サービス発見との統合
3. **Federation**: 複数Prometheusインスタンスの連携
4. **Export Metrics**: カスタムエクスポーターの実装
5. **Grafana Integration**: ダッシュボード用のメトリクス設計

Prometheusメトリクスの実装を通じて、プロダクションレベルの監視システム構築の基礎を学びましょう！