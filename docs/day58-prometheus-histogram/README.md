# Day 58: Prometheus Histogram Metrics

## 🎯 本日の目標 (Today's Goal)

Prometheusヒストグラムメトリクスを実装し、リクエストのレイテンシ分布やパフォーマンス分析を行う高度な監視システムを構築できるようになる。パーセンタイル計算、アラート条件の設定、パフォーマンス分析手法を習得する。

## 📖 解説 (Explanation)

### ヒストグラムとは

ヒストグラムは観測値を事前定義されたバケット（区間）に分類して、値の分布を測定するメトリクス型です。レイテンシやファイルサイズなど、値の範囲が広く分布の形状が重要な指標の監視に適しています。

### ヒストグラムの構造

Prometheusヒストグラムは3つのメトリクスシリーズを自動生成します：

```
# バケット別の累積カウント
http_request_duration_seconds_bucket{le="0.1"} 850
http_request_duration_seconds_bucket{le="0.5"} 1200  
http_request_duration_seconds_bucket{le="1.0"} 1450
http_request_duration_seconds_bucket{le="+Inf"} 1500

# 全観測値の合計
http_request_duration_seconds_sum 425.3

# 観測回数の総数
http_request_duration_seconds_count 1500
```

### ヒストグラムの利点と特徴

#### 1. パーセンタイル計算

ヒストグラムから様々なパーセンタイルを計算できます：

```promql
# 95パーセンタイル
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# 50パーセンタイル（中央値）
histogram_quantile(0.50, rate(http_request_duration_seconds_bucket[5m]))

# 99.9パーセンタイル
histogram_quantile(0.999, rate(http_request_duration_seconds_bucket[5m]))
```

#### 2. 集約可能性

複数のインスタンスからのヒストグラムを集約できます：

```promql
# 複数インスタンスの95パーセンタイル
histogram_quantile(0.95, 
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le)
)

# サービス別の95パーセンタイル
histogram_quantile(0.95,
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service)
)
```

#### 3. SLI/SLO 監視

Service Level Indicators（SLI）とService Level Objectives（SLO）の監視：

```promql
# 95%のリクエストが100ms以内（SLI）
(
  sum(rate(http_request_duration_seconds_bucket{le="0.1"}[5m])) 
  / 
  sum(rate(http_request_duration_seconds_count[5m]))
) * 100

# エラーバジェット消費率
1 - (
  sum(rate(http_request_duration_seconds_bucket{le="0.1"}[5m])) 
  / 
  sum(rate(http_request_duration_seconds_count[5m]))
)
```

### バケット設計の考慮事項

#### 1. 適切なバケット境界

```go
// Webアプリケーション用（ミリ秒）
webBuckets := []float64{
    0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
}

// API処理時間用（秒）
apiBuckets := []float64{
    0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0, 120.0,
}

// ファイルサイズ用（バイト）
fileSizeBuckets := []float64{
    1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216,
}

// データベースクエリ用（ミリ秒）
dbBuckets := []float64{
    0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0,
}
```

#### 2. 指数的バケット生成

```go
import "github.com/prometheus/client_golang/prometheus"

// 指数的バケット生成
// Start=0.1, Factor=2, Count=10
// 結果: [0.1, 0.2, 0.4, 0.8, 1.6, 3.2, 6.4, 12.8, 25.6, 51.2]
buckets := prometheus.ExponentialBuckets(0.1, 2, 10)

// 線形バケット生成  
// Start=0, Width=10, Count=20
// 結果: [0, 10, 20, 30, ..., 190]
linearBuckets := prometheus.LinearBuckets(0, 10, 20)
```

### 高度なヒストグラム実装

#### 1. 複数次元ヒストグラム

```go
type RequestLatencyTracker struct {
    histogram *prometheus.HistogramVec
}

func NewRequestLatencyTracker() *RequestLatencyTracker {
    return &RequestLatencyTracker{
        histogram: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_request_duration_seconds",
                Help: "Time spent on HTTP requests",
                Buckets: []float64{
                    0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
                },
            },
            []string{"method", "endpoint", "status_class"}, // 2xx, 4xx, 5xx
        ),
    }
}

func (t *RequestLatencyTracker) TrackRequest(method, endpoint string, statusCode int, duration time.Duration) {
    statusClass := fmt.Sprintf("%dxx", statusCode/100)
    t.histogram.WithLabelValues(method, endpoint, statusClass).Observe(duration.Seconds())
}
```

#### 2. 自動的なメトリクス収集

```go
type HistogramMiddleware struct {
    latencyTracker    *RequestLatencyTracker
    sizeTracker       *prometheus.HistogramVec
    concurrencyGauge  *prometheus.GaugeVec
}

func NewHistogramMiddleware() *HistogramMiddleware {
    return &HistogramMiddleware{
        latencyTracker: NewRequestLatencyTracker(),
        sizeTracker: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_request_size_bytes",
                Help: "Size of HTTP requests in bytes",
                Buckets: prometheus.ExponentialBuckets(64, 4, 8), // 64B to 1GB
            },
            []string{"method", "endpoint"},
        ),
        concurrencyGauge: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "http_requests_in_flight",
                Help: "Number of HTTP requests currently being processed",
            },
            []string{"endpoint"},
        ),
    }
}

func (m *HistogramMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        endpoint := r.URL.Path
        
        // 同時実行数を追跡
        m.concurrencyGauge.WithLabelValues(endpoint).Inc()
        defer m.concurrencyGauge.WithLabelValues(endpoint).Dec()
        
        // リクエストサイズを記録
        if r.ContentLength > 0 {
            m.sizeTracker.WithLabelValues(r.Method, endpoint).Observe(float64(r.ContentLength))
        }
        
        // レスポンスライターをラップ
        ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        // 次のハンドラを実行
        next.ServeHTTP(ww, r)
        
        // レイテンシを記録
        duration := time.Since(start)
        m.latencyTracker.TrackRequest(r.Method, endpoint, ww.statusCode, duration)
    })
}
```

### パフォーマンス分析

#### 1. レイテンシ分析器

```go
type LatencyAnalyzer struct {
    tracker *RequestLatencyTracker
}

func (a *LatencyAnalyzer) AnalyzePerformance() PerformanceReport {
    // メトリクスファミリーを取得
    metricFamilies, err := prometheus.DefaultGatherer.Gather()
    if err != nil {
        return PerformanceReport{}
    }
    
    report := PerformanceReport{
        Timestamp: time.Now(),
        Endpoints: make([]EndpointPerformance, 0),
    }
    
    for _, mf := range metricFamilies {
        if mf.GetName() == "http_request_duration_seconds" {
            report.Endpoints = a.analyzeHistogramMetrics(mf)
        }
    }
    
    return report
}

func (a *LatencyAnalyzer) analyzeHistogramMetrics(mf *dto.MetricFamily) []EndpointPerformance {
    endpointStats := make(map[string]*EndpointPerformance)
    
    for _, metric := range mf.GetMetric() {
        labels := make(map[string]string)
        for _, label := range metric.GetLabel() {
            labels[label.GetName()] = label.GetValue()
        }
        
        endpoint := labels["endpoint"]
        if endpoint == "" {
            continue
        }
        
        if _, exists := endpointStats[endpoint]; !exists {
            endpointStats[endpoint] = &EndpointPerformance{
                Endpoint: endpoint,
                Buckets:  make([]BucketData, 0),
            }
        }
        
        hist := metric.GetHistogram()
        endpointStats[endpoint].Count = hist.GetSampleCount()
        endpointStats[endpoint].Sum = hist.GetSampleSum()
        
        if endpointStats[endpoint].Count > 0 {
            endpointStats[endpoint].Average = endpointStats[endpoint].Sum / float64(endpointStats[endpoint].Count)
        }
        
        // バケットデータを処理
        for _, bucket := range hist.GetBucket() {
            endpointStats[endpoint].Buckets = append(endpointStats[endpoint].Buckets, BucketData{
                UpperBound: bucket.GetUpperBound(),
                Count:      bucket.GetCumulativeCount(),
            })
        }
        
        // パーセンタイル計算
        endpointStats[endpoint].P50 = a.calculatePercentile(endpointStats[endpoint].Buckets, 0.5, endpointStats[endpoint].Count)
        endpointStats[endpoint].P95 = a.calculatePercentile(endpointStats[endpoint].Buckets, 0.95, endpointStats[endpoint].Count)
        endpointStats[endpoint].P99 = a.calculatePercentile(endpointStats[endpoint].Buckets, 0.99, endpointStats[endpoint].Count)
    }
    
    // マップをスライスに変換
    result := make([]EndpointPerformance, 0, len(endpointStats))
    for _, ep := range endpointStats {
        result = append(result, *ep)
    }
    
    return result
}

func (a *LatencyAnalyzer) calculatePercentile(buckets []BucketData, percentile float64, totalCount uint64) float64 {
    if len(buckets) == 0 || totalCount == 0 {
        return 0
    }
    
    targetCount := float64(totalCount) * percentile
    var prevBound float64 = 0
    var prevCount uint64 = 0
    
    for _, bucket := range buckets {
        if float64(bucket.Count) >= targetCount {
            // 線形補間でパーセンタイル値を計算
            if bucket.Count == prevCount {
                return prevBound
            }
            
            ratio := (targetCount - float64(prevCount)) / float64(bucket.Count-prevCount)
            return prevBound + ratio*(bucket.UpperBound-prevBound)
        }
        
        prevBound = bucket.UpperBound
        prevCount = bucket.Count
    }
    
    return buckets[len(buckets)-1].UpperBound
}

type PerformanceReport struct {
    Timestamp time.Time             `json:"timestamp"`
    Endpoints []EndpointPerformance `json:"endpoints"`
}

type EndpointPerformance struct {
    Endpoint string       `json:"endpoint"`
    Count    uint64       `json:"count"`
    Sum      float64      `json:"sum"`
    Average  float64      `json:"average"`
    P50      float64      `json:"p50"`
    P95      float64      `json:"p95"`
    P99      float64      `json:"p99"`
    Buckets  []BucketData `json:"buckets"`
}

type BucketData struct {
    UpperBound float64 `json:"upper_bound"`
    Count      uint64  `json:"count"`
}
```

#### 2. アラートシステム

```go
type AlertingSystem struct {
    tracker    *RequestLatencyTracker
    thresholds map[string]LatencyThreshold
    alertCh    chan Alert
}

type LatencyThreshold struct {
    P95Threshold float64 `json:"p95_threshold"`
    P99Threshold float64 `json:"p99_threshold"`
    ErrorRate    float64 `json:"error_rate"`
}

type Alert struct {
    Type         string    `json:"type"`
    Endpoint     string    `json:"endpoint"`
    Message      string    `json:"message"`
    Severity     string    `json:"severity"`
    Timestamp    time.Time `json:"timestamp"`
    CurrentValue float64   `json:"current_value"`
    Threshold    float64   `json:"threshold"`
}

func NewAlertingSystem(tracker *RequestLatencyTracker) *AlertingSystem {
    return &AlertingSystem{
        tracker:    tracker,
        thresholds: make(map[string]LatencyThreshold),
        alertCh:    make(chan Alert, 100),
    }
}

func (as *AlertingSystem) SetThreshold(endpoint string, threshold LatencyThreshold) {
    as.thresholds[endpoint] = threshold
}

func (as *AlertingSystem) StartMonitoring(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            as.checkThresholds()
        }
    }
}

func (as *AlertingSystem) checkThresholds() {
    analyzer := &LatencyAnalyzer{tracker: as.tracker}
    report := analyzer.AnalyzePerformance()
    
    for _, ep := range report.Endpoints {
        if threshold, exists := as.thresholds[ep.Endpoint]; exists {
            // P95レイテンシチェック
            if ep.P95 > threshold.P95Threshold {
                alert := Alert{
                    Type:         "latency_p95",
                    Endpoint:     ep.Endpoint,
                    Message:      "P95 latency exceeds threshold",
                    Severity:     "warning",
                    Timestamp:    time.Now(),
                    CurrentValue: ep.P95,
                    Threshold:    threshold.P95Threshold,
                }
                
                select {
                case as.alertCh <- alert:
                    log.Printf("Alert: P95 latency for %s is %.3fs (threshold: %.3fs)", 
                        ep.Endpoint, ep.P95, threshold.P95Threshold)
                default:
                    log.Printf("Alert channel full, dropping alert")
                }
            }
            
            // P99レイテンシチェック
            if ep.P99 > threshold.P99Threshold {
                alert := Alert{
                    Type:         "latency_p99",
                    Endpoint:     ep.Endpoint,
                    Message:      "P99 latency exceeds threshold",
                    Severity:     "critical",
                    Timestamp:    time.Now(),
                    CurrentValue: ep.P99,
                    Threshold:    threshold.P99Threshold,
                }
                
                select {
                case as.alertCh <- alert:
                    log.Printf("CRITICAL: P99 latency for %s is %.3fs (threshold: %.3fs)", 
                        ep.Endpoint, ep.P99, threshold.P99Threshold)
                default:
                    log.Printf("Alert channel full, dropping critical alert")
                }
            }
        }
    }
}

func (as *AlertingSystem) GetAlerts() <-chan Alert {
    return as.alertCh
}
```

### Grafanaダッシュボード対応

#### 1. ダッシュボード用メトリクス設計

```go
type GrafanaMetrics struct {
    // レイテンシヒストグラム
    RequestDuration *prometheus.HistogramVec
    
    // スループットカウンター  
    RequestsTotal *prometheus.CounterVec
    
    // エラー率カウンター
    ErrorsTotal *prometheus.CounterVec
    
    // 追加の分析用メトリクス
    SlowRequests    *prometheus.CounterVec  // 閾値を超えるリクエスト
    RequestSize     *prometheus.HistogramVec // リクエストサイズ分布
    ResponseSize    *prometheus.HistogramVec // レスポンスサイズ分布
}

func NewGrafanaMetrics() *GrafanaMetrics {
    return &GrafanaMetrics{
        RequestDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_request_duration_seconds",
                Help: "HTTP request duration in seconds",
                Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
            },
            []string{"method", "endpoint", "status_class"},
        ),
        RequestsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_total",
                Help: "Total number of HTTP requests",
            },
            []string{"method", "endpoint", "status"},
        ),
        ErrorsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_errors_total",
                Help: "Total number of HTTP errors",
            },
            []string{"method", "endpoint", "status"},
        ),
        SlowRequests: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_slow_requests_total",
                Help: "Total number of slow HTTP requests",
            },
            []string{"method", "endpoint", "threshold"},
        ),
        RequestSize: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_request_size_bytes",
                Help: "HTTP request size in bytes",
                Buckets: prometheus.ExponentialBuckets(64, 4, 8),
            },
            []string{"method", "endpoint"},
        ),
        ResponseSize: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_response_size_bytes", 
                Help: "HTTP response size in bytes",
                Buckets: prometheus.ExponentialBuckets(64, 4, 8),
            },
            []string{"method", "endpoint", "status_class"},
        ),
    }
}
```

#### 2. PromQL クエリ例

```promql
# 95パーセンタイルレイテンシ（5分間）
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# エンドポイント別の95パーセンタイル
histogram_quantile(0.95, 
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le, endpoint)
)

# SLI: 100ms以内のリクエスト割合
sum(rate(http_request_duration_seconds_bucket{le="0.1"}[5m])) 
/ 
sum(rate(http_request_duration_seconds_count[5m]))

# スループット（RPS）
sum(rate(http_requests_total[5m]))

# エラー率
sum(rate(http_errors_total[5m])) 
/ 
sum(rate(http_requests_total[5m]))

# 平均レスポンス時間
rate(http_request_duration_seconds_sum[5m]) 
/ 
rate(http_request_duration_seconds_count[5m])
```

## 📝 課題 (The Problem)

以下の機能を持つPrometheusヒストグラムシステムを実装してください：

### 1. リクエストレイテンシトラッカー

```go
type RequestLatencyTracker struct {
    histogram *prometheus.HistogramVec
}
```

### 2. 必要な機能

- **多次元メトリクス**: method, endpoint, status による分類
- **適切なバケット設計**: Webアプリケーションに適したバケット境界
- **自動収集ミドルウェア**: HTTPリクエストの自動メトリクス収集
- **パフォーマンス分析**: パーセンタイル計算とレポート生成
- **アラートシステム**: 閾値ベースのアラート機能

### 3. レポート機能

- エンドポイント別のパフォーマンス分析
- P50/P95/P99パーセンタイルの計算
- 平均レスポンス時間の算出
- スループット分析

### 4. 監視機能

- リアルタイムアラート
- 閾値超過の検出
- アラート履歴の管理

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestHistogram_BasicObservation
    main_test.go:45: Histogram observation recorded correctly
--- PASS: TestHistogram_BasicObservation (0.01s)

=== RUN   TestHistogram_MultipleObservations
    main_test.go:65: Multiple observations recorded correctly
    main_test.go:68: Bucket distribution is accurate
--- PASS: TestHistogram_MultipleObservations (0.01s)

=== RUN   TestPercentileCalculation
    main_test.go:85: P50: 0.250s, P95: 0.950s, P99: 0.990s
--- PASS: TestPercentileCalculation (0.02s)

=== RUN   TestAlertingSystem
    main_test.go:105: Alert triggered for P95 threshold violation
--- PASS: TestAlertingSystem (0.05s)

PASS
ok      day58-prometheus-histogram   0.156s
```

### メトリクス出力例

```
# HELP http_request_duration_seconds Time spent on HTTP requests
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",status_class="2xx",le="0.001"} 45
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",status_class="2xx",le="0.005"} 250
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",status_class="2xx",le="0.01"} 500
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",status_class="2xx",le="0.025"} 800
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",status_class="2xx",le="0.05"} 950
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",status_class="2xx",le="0.1"} 990
http_request_duration_seconds_bucket{method="GET",endpoint="/api/users",status_class="2xx",le="+Inf"} 1000
http_request_duration_seconds_sum{method="GET",endpoint="/api/users",status_class="2xx"} 15.5
http_request_duration_seconds_count{method="GET",endpoint="/api/users",status_class="2xx"} 1000
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 基本的なヒストグラム実装

```go
func NewRequestLatencyTracker() *RequestLatencyTracker {
    return &RequestLatencyTracker{
        histogram: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_request_duration_seconds",
                Help: "Time spent on HTTP requests",
                Buckets: []float64{
                    0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
                },
            },
            []string{"method", "endpoint", "status_class"},
        ),
    }
}
```

### パーセンタイル計算

```go
func calculatePercentile(buckets []BucketData, percentile float64, totalCount uint64) float64 {
    if len(buckets) == 0 || totalCount == 0 {
        return 0
    }
    
    targetCount := float64(totalCount) * percentile
    var prevBound float64 = 0
    var prevCount uint64 = 0
    
    for _, bucket := range buckets {
        if float64(bucket.Count) >= targetCount {
            if bucket.Count == prevCount {
                return prevBound
            }
            
            ratio := (targetCount - float64(prevCount)) / float64(bucket.Count-prevCount)
            return prevBound + ratio*(bucket.UpperBound-prevBound)
        }
        
        prevBound = bucket.UpperBound
        prevCount = bucket.Count
    }
    
    return buckets[len(buckets)-1].UpperBound
}
```

### ミドルウェア実装

```go
func (t *RequestLatencyTracker) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        next.ServeHTTP(ww, r)
        
        duration := time.Since(start)
        statusClass := fmt.Sprintf("%dxx", ww.statusCode/100)
        
        t.histogram.WithLabelValues(r.Method, r.URL.Path, statusClass).Observe(duration.Seconds())
    })
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **動的バケット調整**: 観測データに基づくバケット境界の最適化
2. **複数時間軸分析**: 短期/中期/長期トレンドの比較
3. **異常検知**: 統計的手法による異常パターンの検出
4. **容量計画**: 成長予測とキャパシティプランニング
5. **コスト分析**: リクエスト処理コストの詳細分析

Prometheusヒストグラムの実装を通じて、高度なパフォーマンス監視システムの構築手法を習得しましょう！