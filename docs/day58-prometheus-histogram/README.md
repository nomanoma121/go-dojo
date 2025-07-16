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
// 【Webアプリケーション用バケット設計】ユーザーエクスペリエンスを重視した境界設定
webBuckets := []float64{
    // 【超高速レスポンス】1ms未満: 静的コンテンツ、CDNキャッシュ
    0.001, 
    // 【高速レスポンス】5ms未満: メモリキャッシュ、シンプルAPI
    0.005, 
    // 【良好レスポンス】10ms未満: 軽量データベースクエリ
    0.01, 
    // 【許容レスポンス】25ms未満: 複雑なビジネスロジック
    0.025, 
    // 【体感良好】50ms未満: 一般的なWebページ
    0.05, 
    // 【体感境界】100ms未満: ユーザーが即座と感じる限界
    0.1, 
    // 【体感遅延】250ms未満: 軽微な遅延を感じ始める
    0.25, 
    // 【明確な遅延】500ms未満: 明らかな遅延を感じる
    0.5, 
    // 【長い遅延】1秒未満: 継続的な操作に影響
    1.0, 
    // 【非常に遅い】2.5秒未満: ユーザーが操作を中断し始める
    2.5, 
    // 【限界遅延】5秒未満: 大半のユーザーが離脱
    5.0, 
    // 【タイムアウト寸前】10秒未満: タイムアウト設定の一般的な値
    10.0,
}

// 【API処理時間用バケット】サーバーサイド処理の詳細分析
apiBuckets := []float64{
    // 【即座処理】100ms未満: 軽量API（認証、バリデーション）
    0.1, 
    // 【迅速処理】500ms未満: 標準的なCRUD操作
    0.5, 
    // 【標準処理】1秒未満: 複雑な計算、外部API呼び出し
    1.0, 
    // 【重い処理】2秒未満: レポート生成、複雑なJOIN
    2.0, 
    // 【バッチ処理】5秒未満: データ集計、ファイル処理
    5.0, 
    // 【長期処理】10秒未満: 大量データ処理
    10.0, 
    // 【非同期候補】30秒未満: 非同期処理への移行検討
    30.0, 
    // 【非同期必須】60秒未満: 非同期処理が必要
    60.0, 
    // 【タイムアウト】120秒未満: 一般的なHTTPタイムアウト
    120.0,
}

// 【ファイルサイズ用バケット】ネットワーク転送とストレージ分析
fileSizeBuckets := []float64{
    // 【小さなファイル】1KB: アイコン、小さなJSON
    1024, 
    // 【軽量ファイル】4KB: 設定ファイル、小さなHTML
    4096, 
    // 【中小ファイル】16KB: 圧縮されたCSS/JS
    16384, 
    // 【標準ファイル】64KB: 通常のWebページ
    65536, 
    // 【大きなファイル】256KB: 大きなJSONレスポンス
    262144, 
    // 【画像ファイル】1MB: 圧縮された画像
    1048576, 
    // 【大容量ファイル】4MB: 高解像度画像
    4194304, 
    // 【非常に大きなファイル】16MB: 動画、大きなPDF
    16777216,
}

// 【データベースクエリ用バケット】高精度のパフォーマンス分析
dbBuckets := []float64{
    // 【超高速クエリ】0.1ms未満: インデックスによる主キー検索
    0.0001, 
    // 【高速クエリ】0.5ms未満: 単純なSELECT、インデックス活用
    0.0005, 
    // 【良好クエリ】1ms未満: 軽量なJOIN、小規模テーブル
    0.001, 
    // 【標準クエリ】5ms未満: 複雑なWHERE条件
    0.005, 
    // 【重いクエリ】10ms未満: 複数テーブルのJOIN
    0.01, 
    // 【非常に重いクエリ】50ms未満: 大量データのGROUP BY
    0.05, 
    // 【最適化要検討】100ms未満: インデックス追加を検討
    0.1, 
    // 【最適化必須】500ms未満: 緊急に最適化が必要
    0.5, 
    // 【問題クエリ】1秒未満: アプリケーション性能に深刻な影響
    1.0, 
    // 【危険クエリ】5秒未満: システム全体に影響
    5.0,
}

// 【ビジネス固有バケット】業界・用途別の特殊な境界設定
// 【金融取引】ミリ秒単位の超高精度監視
financialTradingBuckets := []float64{
    0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0,
}

// 【動画ストリーミング】帯域幅とバッファリング分析
videoStreamingBuckets := []float64{
    0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0, 120.0, 300.0,
}

// 【機械学習推論】推論時間の詳細分析
mlInferenceBuckets := []float64{
    0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0,
}

// 【IoTデータ処理】大量データの処理時間分析
iotProcessingBuckets := []float64{
    0.001, 0.01, 0.1, 1.0, 10.0, 60.0, 300.0, 1800.0, 3600.0,
}
```

#### 2. 指数的バケット生成

```go
import "github.com/prometheus/client_golang/prometheus"

// 【指数的バケット生成】性能要件に応じた動的バケット設計
func createExponentialBuckets() []float64 {
    // 【パラメータ解説】
    // Start=0.1: 開始値（100ms）
    // Factor=2: 各バケットは前の値の2倍
    // Count=10: 10個のバケットを生成
    // 結果: [0.1, 0.2, 0.4, 0.8, 1.6, 3.2, 6.4, 12.8, 25.6, 51.2]
    buckets := prometheus.ExponentialBuckets(0.1, 2, 10)
    
    // 【指数的バケットの特徴】：
    // - 低い値に高い解像度を提供
    // - 高い値には低い解像度（粗い粒度）
    // - レスポンス時間やファイルサイズに最適
    // - 多くの値が低い範囲に集中する分布に適している
    
    fmt.Printf("📊 Exponential buckets: %v\n", buckets)
    
    // 【実用例】レスポンス時間の詳細分析
    // 0.1s未満: 93%のリクエスト（詳細な分析が必要）
    // 0.1-1.0s: 6%のリクエスト（パフォーマンス監視）
    // 1.0s以上: 1%のリクエスト（問題のあるリクエスト）
    
    return buckets
}

// 【線形バケット生成】等間隔でのデータ分布分析
func createLinearBuckets() []float64 {
    // 【パラメータ解説】
    // Start=0: 開始値
    // Width=10: 各バケットの幅
    // Count=20: 20個のバケットを生成
    // 結果: [0, 10, 20, 30, ..., 190]
    linearBuckets := prometheus.LinearBuckets(0, 10, 20)
    
    // 【線形バケットの特徴】：
    // - 全範囲で等しい解像度を提供
    // - 一様分布や正規分布に適している
    // - スループット、エラー率、均等なカテゴリ分析に最適
    
    fmt.Printf("📊 Linear buckets: %v\n", linearBuckets)
    
    // 【実用例】スループット分析
    // 0-10 req/sec: 低負荷時間帯
    // 10-50 req/sec: 通常負荷時間帯
    // 50-100 req/sec: 高負荷時間帯
    // 100+ req/sec: ピーク負荷時間帯
    
    return linearBuckets
}

// 【カスタムバケット生成】ビジネス要件に特化した境界設計
func createCustomBuckets(serviceType string) []float64 {
    switch serviceType {
    case "microservice":
        // 【マイクロサービス】サービス間通信の最適化
        return []float64{
            0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0,
        }
        
    case "database":
        // 【データベース】クエリ性能の詳細分析
        return []float64{
            0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0,
        }
        
    case "batch":
        // 【バッチ処理】長時間処理の監視
        return []float64{
            1.0, 10.0, 30.0, 60.0, 300.0, 600.0, 1800.0, 3600.0, 7200.0,
        }
        
    case "realtime":
        // 【リアルタイム】超低遅延要求システム
        return []float64{
            0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1,
        }
        
    default:
        // 【汎用】バランスの取れた設計
        return prometheus.DefBuckets
    }
}

// 【バケット設計検証】実際のデータに基づく最適化
func validateBucketDesign(buckets []float64, sampleData []float64) BucketAnalysis {
    analysis := BucketAnalysis{
        Buckets: buckets,
        Distribution: make([]int, len(buckets)),
        TotalSamples: len(sampleData),
    }
    
    // 【データ分布の計算】
    for _, value := range sampleData {
        for i, bucket := range buckets {
            if value <= bucket {
                analysis.Distribution[i]++
                break
            }
        }
    }
    
    // 【バケット効率の計算】
    for i, count := range analysis.Distribution {
        percentage := float64(count) / float64(analysis.TotalSamples) * 100
        analysis.Efficiency = append(analysis.Efficiency, percentage)
        
        // 【警告】：空のバケットや偏りの検出
        if count == 0 {
            log.Printf("⚠️  Empty bucket: %f (may be unnecessary)", buckets[i])
        } else if percentage > 80 {
            log.Printf("⚠️  Overloaded bucket: %f (%.1f%% of data)", buckets[i], percentage)
        }
    }
    
    return analysis
}

// 【動的バケット調整】運用データに基づく自動最適化
func optimizeBuckets(currentBuckets []float64, historicalData []float64) []float64 {
    // 【STEP 1】現在の分布を分析
    analysis := validateBucketDesign(currentBuckets, historicalData)
    
    // 【STEP 2】データの統計情報を計算
    stats := calculateStatistics(historicalData)
    
    // 【STEP 3】最適なバケットを生成
    optimizedBuckets := make([]float64, 0)
    
    // 【戦略1】パーセンタイルベースの境界
    percentiles := []float64{0.5, 0.75, 0.9, 0.95, 0.99, 0.999}
    for _, p := range percentiles {
        value := calculatePercentile(historicalData, p)
        optimizedBuckets = append(optimizedBuckets, value)
    }
    
    // 【戦略2】データの自然な境界
    // 標準偏差に基づく境界
    for i := 1; i <= 3; i++ {
        boundary := stats.Mean + float64(i)*stats.StdDev
        optimizedBuckets = append(optimizedBuckets, boundary)
    }
    
    // 【STEP 4】バケットをソートして重複を除去
    sort.Float64s(optimizedBuckets)
    uniqueBuckets := removeDuplicates(optimizedBuckets)
    
    log.Printf("🔄 Bucket optimization complete: %d -> %d buckets", 
        len(currentBuckets), len(uniqueBuckets))
    
    return uniqueBuckets
}

// 【分析結果構造体】バケット設計の評価
type BucketAnalysis struct {
    Buckets       []float64 `json:"buckets"`
    Distribution  []int     `json:"distribution"`
    Efficiency    []float64 `json:"efficiency"`
    TotalSamples  int       `json:"total_samples"`
    Recommendations []string `json:"recommendations"`
}

// 【統計情報構造体】データ分布の特性
type DataStatistics struct {
    Mean   float64 `json:"mean"`
    Median float64 `json:"median"`
    StdDev float64 `json:"std_dev"`
    Min    float64 `json:"min"`
    Max    float64 `json:"max"`
    P95    float64 `json:"p95"`
    P99    float64 `json:"p99"`
}
```

### 高度なヒストグラム実装

#### 1. 複数次元ヒストグラム

```go
// 【高度なレイテンシトラッカー】多次元パフォーマンス分析システム
type RequestLatencyTracker struct {
    histogram          *prometheus.HistogramVec
    slowRequestCounter *prometheus.CounterVec
    requestSizeHist    *prometheus.HistogramVec
    concurrencyGauge   *prometheus.GaugeVec
    
    // 【統計情報】リアルタイム分析用
    stats              *LatencyStats
    slowThreshold      time.Duration
    sampleRate         float64
    mu                 sync.RWMutex
}

// 【レイテンシ統計】運用監視のためのリアルタイム集計
type LatencyStats struct {
    TotalRequests     int64         `json:"total_requests"`
    SlowRequests      int64         `json:"slow_requests"`
    AverageLatency    time.Duration `json:"average_latency"`
    P95Latency        time.Duration `json:"p95_latency"`
    P99Latency        time.Duration `json:"p99_latency"`
    LastUpdate        time.Time     `json:"last_update"`
    EndpointStats     map[string]*EndpointStats `json:"endpoint_stats"`
}

// 【エンドポイント固有統計】詳細なパフォーマンス分析
type EndpointStats struct {
    RequestCount      int64         `json:"request_count"`
    ErrorCount        int64         `json:"error_count"`
    AverageLatency    time.Duration `json:"average_latency"`
    MaxLatency        time.Duration `json:"max_latency"`
    MinLatency        time.Duration `json:"min_latency"`
    TotalLatency      time.Duration `json:"total_latency"`
    LastAccess        time.Time     `json:"last_access"`
}

func NewRequestLatencyTracker(slowThreshold time.Duration) *RequestLatencyTracker {
    return &RequestLatencyTracker{
        histogram: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_request_duration_seconds",
                Help: "Time spent on HTTP requests",
                // 【最適化されたバケット】Web アプリケーションのUX基準に基づく設計
                Buckets: []float64{
                    // 【優秀】1ms未満: 静的リソース、メモリキャッシュ
                    0.001, 
                    // 【良好】5ms未満: 軽量API、シンプルクエリ
                    0.005, 
                    // 【標準】10ms未満: 通常のビジネスロジック
                    0.01, 
                    // 【許容】25ms未満: 複雑な処理、外部API呼び出し
                    0.025, 
                    // 【体感良好】50ms未満: ユーザーが快適に感じる限界
                    0.05, 
                    // 【体感境界】100ms未満: 即座のレスポンスと感じる限界
                    0.1, 
                    // 【軽微な遅延】250ms未満: 僅かな遅延を感じ始める
                    0.25, 
                    // 【明確な遅延】500ms未満: 明らかな遅延として認識
                    0.5, 
                    // 【遅い】1秒未満: 継続的操作に支障
                    1.0, 
                    // 【非常に遅い】2.5秒未満: ユーザーの操作意欲に影響
                    2.5, 
                    // 【限界】5秒未満: 多くのユーザーが離脱
                    5.0, 
                    // 【タイムアウト寸前】10秒未満: 一般的なタイムアウト設定
                    10.0,
                },
            },
            []string{"method", "endpoint", "status_class", "user_type"}, // premium, standard, guest
        ),
        
        slowRequestCounter: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_slow_requests_total",
                Help: "Total number of slow HTTP requests",
            },
            []string{"method", "endpoint", "threshold_type"}, // warning, critical, severe
        ),
        
        requestSizeHist: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_request_size_bytes",
                Help: "Size of HTTP requests in bytes",
                Buckets: prometheus.ExponentialBuckets(64, 4, 10), // 64B to 64MB
            },
            []string{"method", "endpoint", "content_type"},
        ),
        
        concurrencyGauge: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "http_requests_in_flight",
                Help: "Number of HTTP requests currently being processed",
            },
            []string{"endpoint", "method"},
        ),
        
        stats: &LatencyStats{
            EndpointStats: make(map[string]*EndpointStats),
        },
        slowThreshold: slowThreshold,
        sampleRate:    1.0, // 100% sampling by default
    }
}

// 【包括的リクエスト追跡】多次元メトリクス収集
func (t *RequestLatencyTracker) TrackRequest(method, endpoint string, statusCode int, duration time.Duration, requestSize int64, userType string) {
    // 【サンプリング】高負荷時のメトリクス収集コスト削減
    if t.sampleRate < 1.0 && rand.Float64() > t.sampleRate {
        return
    }
    
    // 【基本分類】
    statusClass := fmt.Sprintf("%dxx", statusCode/100)
    
    // 【STEP 1】レイテンシヒストグラム更新
    t.histogram.WithLabelValues(method, endpoint, statusClass, userType).Observe(duration.Seconds())
    
    // 【STEP 2】リクエストサイズ分析
    if requestSize > 0 {
        contentType := "application/json" // 実際の実装では Content-Type ヘッダーから取得
        t.requestSizeHist.WithLabelValues(method, endpoint, contentType).Observe(float64(requestSize))
    }
    
    // 【STEP 3】遅いリクエストの特別な追跡
    t.trackSlowRequests(method, endpoint, duration)
    
    // 【STEP 4】統計情報の更新
    t.updateStats(method, endpoint, statusCode, duration)
    
    // 【STEP 5】異常検知
    t.detectAnomalies(method, endpoint, duration, statusCode)
}

// 【遅いリクエスト監視】パフォーマンス劣化の早期検出
func (t *RequestLatencyTracker) trackSlowRequests(method, endpoint string, duration time.Duration) {
    if duration > t.slowThreshold {
        // 【段階的アラート】遅延レベルに応じた分類
        var thresholdType string
        switch {
        case duration > t.slowThreshold*5: // 5倍以上
            thresholdType = "severe"
            log.Printf("🚨 SEVERE slow request: %s %s took %v (threshold: %v)", 
                method, endpoint, duration, t.slowThreshold)
        case duration > t.slowThreshold*2: // 2倍以上
            thresholdType = "critical"
            log.Printf("⚠️  CRITICAL slow request: %s %s took %v (threshold: %v)", 
                method, endpoint, duration, t.slowThreshold)
        default:
            thresholdType = "warning"
            log.Printf("⏰ WARNING slow request: %s %s took %v (threshold: %v)", 
                method, endpoint, duration, t.slowThreshold)
        }
        
        t.slowRequestCounter.WithLabelValues(method, endpoint, thresholdType).Inc()
    }
}

// 【統計情報更新】リアルタイム分析のための内部状態管理
func (t *RequestLatencyTracker) updateStats(method, endpoint string, statusCode int, duration time.Duration) {
    t.mu.Lock()
    defer t.mu.Unlock()
    
    // 【全体統計】
    t.stats.TotalRequests++
    if duration > t.slowThreshold {
        t.stats.SlowRequests++
    }
    
    // 【エンドポイント統計】
    endpointKey := fmt.Sprintf("%s %s", method, endpoint)
    if t.stats.EndpointStats[endpointKey] == nil {
        t.stats.EndpointStats[endpointKey] = &EndpointStats{
            MinLatency: duration,
            MaxLatency: duration,
        }
    }
    
    epStats := t.stats.EndpointStats[endpointKey]
    epStats.RequestCount++
    epStats.TotalLatency += duration
    epStats.AverageLatency = epStats.TotalLatency / time.Duration(epStats.RequestCount)
    epStats.LastAccess = time.Now()
    
    // 【最小・最大レイテンシ更新】
    if duration < epStats.MinLatency {
        epStats.MinLatency = duration
    }
    if duration > epStats.MaxLatency {
        epStats.MaxLatency = duration
    }
    
    // 【エラー統計】
    if statusCode >= 400 {
        epStats.ErrorCount++
    }
    
    t.stats.LastUpdate = time.Now()
}

// 【異常検知】統計的手法による異常パターンの検出
func (t *RequestLatencyTracker) detectAnomalies(method, endpoint string, duration time.Duration, statusCode int) {
    t.mu.RLock()
    defer t.mu.RUnlock()
    
    endpointKey := fmt.Sprintf("%s %s", method, endpoint)
    epStats, exists := t.stats.EndpointStats[endpointKey]
    if !exists || epStats.RequestCount < 10 {
        return // 統計的分析には最低10リクエストが必要
    }
    
    // 【異常検知条件】
    avgLatency := epStats.AverageLatency
    
    // 【条件1】平均の5倍以上の遅延
    if duration > avgLatency*5 {
        log.Printf("🔍 ANOMALY: %s %s latency %.3fs is 5x average (%.3fs)", 
            method, endpoint, duration.Seconds(), avgLatency.Seconds())
    }
    
    // 【条件2】エラー率の急激な上昇
    errorRate := float64(epStats.ErrorCount) / float64(epStats.RequestCount)
    if errorRate > 0.1 && statusCode >= 500 {
        log.Printf("🔍 ANOMALY: %s %s error rate %.1f%% with 5xx status", 
            method, endpoint, errorRate*100)
    }
    
    // 【条件3】突発的な高負荷
    if epStats.RequestCount > 100 {
        recentRequests := epStats.RequestCount / 10 // 直近10%のリクエスト
        if recentRequests > 50 {
            log.Printf("🔍 ANOMALY: %s %s high request rate detected", method, endpoint)
        }
    }
}

// 【統計情報取得】監視ダッシュボード用のデータ提供
func (t *RequestLatencyTracker) GetStats() LatencyStats {
    t.mu.RLock()
    defer t.mu.RUnlock()
    
    // 【ディープコピー】並行安全性を保証
    stats := LatencyStats{
        TotalRequests:  t.stats.TotalRequests,
        SlowRequests:   t.stats.SlowRequests,
        AverageLatency: t.stats.AverageLatency,
        P95Latency:     t.stats.P95Latency,
        P99Latency:     t.stats.P99Latency,
        LastUpdate:     t.stats.LastUpdate,
        EndpointStats:  make(map[string]*EndpointStats),
    }
    
    // 【エンドポイント統計のコピー】
    for key, epStats := range t.stats.EndpointStats {
        stats.EndpointStats[key] = &EndpointStats{
            RequestCount:   epStats.RequestCount,
            ErrorCount:     epStats.ErrorCount,
            AverageLatency: epStats.AverageLatency,
            MaxLatency:     epStats.MaxLatency,
            MinLatency:     epStats.MinLatency,
            TotalLatency:   epStats.TotalLatency,
            LastAccess:     epStats.LastAccess,
        }
    }
    
    return stats
}

// 【サンプリング率調整】負荷に応じた動的調整
func (t *RequestLatencyTracker) SetSampleRate(rate float64) {
    if rate < 0 || rate > 1.0 {
        log.Printf("❌ Invalid sample rate: %f (must be 0.0-1.0)", rate)
        return
    }
    
    t.mu.Lock()
    defer t.mu.Unlock()
    
    t.sampleRate = rate
    log.Printf("🔄 Sample rate updated to %.1f%%", rate*100)
}

// 【統計リセット】定期的なメトリクスクリーンアップ
func (t *RequestLatencyTracker) ResetStats() {
    t.mu.Lock()
    defer t.mu.Unlock()
    
    t.stats = &LatencyStats{
        EndpointStats: make(map[string]*EndpointStats),
    }
    
    log.Printf("🔄 Latency statistics reset")
}
```

#### 2. 自動的なメトリクス収集

```go
// 【高度なヒストグラムミドルウェア】包括的パフォーマンス監視システム
type HistogramMiddleware struct {
    latencyTracker     *RequestLatencyTracker
    sizeTracker        *prometheus.HistogramVec
    concurrencyGauge   *prometheus.GaugeVec
    throughputCounter  *prometheus.CounterVec
    errorCounter       *prometheus.CounterVec
    
    // 【高度な監視機能】
    responseTimeBySize *prometheus.HistogramVec
    userAgentTracker   *prometheus.CounterVec
    geolocationTracker *prometheus.CounterVec
    
    // 【実行時設定】
    config             *MiddlewareConfig
    rateLimiter        *RateLimiter
    alertManager       *AlertManager
}

// 【ミドルウェア設定】柔軟な動作制御
type MiddlewareConfig struct {
    SampleRate         float64       `json:"sample_rate"`         // メトリクス収集のサンプリング率
    SlowThreshold      time.Duration `json:"slow_threshold"`      // 遅いリクエストの閾値
    LargeRequestSize   int64         `json:"large_request_size"`  // 大きなリクエストの閾値
    EnableGeoTracking  bool          `json:"enable_geo_tracking"`  // 地理的位置追跡
    EnableUserAgent    bool          `json:"enable_user_agent"`    // ユーザーエージェント追跡
    MaxConcurrency     int           `json:"max_concurrency"`      // 最大同時実行数
    AlertThresholds    AlertThresholds `json:"alert_thresholds"`   // アラート閾値
}

// 【アラート閾値設定】
type AlertThresholds struct {
    ErrorRate        float64 `json:"error_rate"`          // エラー率 (0.0-1.0)
    P95Latency       float64 `json:"p95_latency"`         // P95レイテンシ (seconds)
    P99Latency       float64 `json:"p99_latency"`         // P99レイテンシ (seconds)
    Throughput       float64 `json:"throughput"`          // スループット (req/sec)
    ConcurrencyLimit int     `json:"concurrency_limit"`   // 同時実行数制限
}

func NewHistogramMiddleware(config *MiddlewareConfig) *HistogramMiddleware {
    if config == nil {
        config = &MiddlewareConfig{
            SampleRate:       1.0,
            SlowThreshold:    500 * time.Millisecond,
            LargeRequestSize: 1024 * 1024, // 1MB
            EnableGeoTracking: false,
            EnableUserAgent:   true,
            MaxConcurrency:    1000,
        }
    }
    
    return &HistogramMiddleware{
        latencyTracker: NewRequestLatencyTracker(config.SlowThreshold),
        
        sizeTracker: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_request_size_bytes",
                Help: "Size of HTTP requests in bytes",
                // 【リクエストサイズ特化バケット】
                Buckets: []float64{
                    64,     // 64B: 小さなGET request
                    256,    // 256B: クエリパラメータ付きGET
                    1024,   // 1KB: 小さなJSON payload
                    4096,   // 4KB: 中程度のJSON payload
                    16384,  // 16KB: 大きなJSON payload
                    65536,  // 64KB: 非常に大きなJSON payload
                    262144, // 256KB: ファイルアップロード
                    1048576, // 1MB: 大きなファイルアップロード
                    4194304, // 4MB: 非常に大きなファイル
                },
            },
            []string{"method", "endpoint", "content_type"},
        ),
        
        concurrencyGauge: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "http_requests_in_flight",
                Help: "Number of HTTP requests currently being processed",
            },
            []string{"endpoint", "method"},
        ),
        
        throughputCounter: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_per_second",
                Help: "HTTP requests per second",
            },
            []string{"method", "endpoint"},
        ),
        
        errorCounter: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_request_errors_total",
                Help: "Total number of HTTP request errors",
            },
            []string{"method", "endpoint", "error_type"},
        ),
        
        // 【高度な分析用メトリクス】
        responseTimeBySize: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "http_response_time_by_size_seconds",
                Help: "HTTP response time grouped by request size",
                Buckets: prometheus.DefBuckets,
            },
            []string{"size_category"}, // small, medium, large, xlarge
        ),
        
        userAgentTracker: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_by_user_agent",
                Help: "HTTP requests grouped by user agent type",
            },
            []string{"user_agent_type"}, // browser, mobile, bot, api
        ),
        
        geolocationTracker: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_by_location",
                Help: "HTTP requests grouped by geographic location",
            },
            []string{"country", "region"},
        ),
        
        config: config,
    }
}

// 【包括的リクエスト監視】多次元メトリクス収集
func (m *HistogramMiddleware) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        endpoint := r.URL.Path
        method := r.Method
        
        // 【STEP 1】サンプリング判定
        if rand.Float64() > m.config.SampleRate {
            next.ServeHTTP(w, r)
            return
        }
        
        // 【STEP 2】同時実行数制限チェック
        currentConcurrency := m.getCurrentConcurrency(endpoint, method)
        if currentConcurrency >= m.config.MaxConcurrency {
            http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
            m.errorCounter.WithLabelValues(method, endpoint, "concurrency_limit").Inc()
            return
        }
        
        // 【STEP 3】同時実行数追跡
        m.concurrencyGauge.WithLabelValues(endpoint, method).Inc()
        defer m.concurrencyGauge.WithLabelValues(endpoint, method).Dec()
        
        // 【STEP 4】リクエストサイズ分析
        requestSize := r.ContentLength
        if requestSize > 0 {
            contentType := r.Header.Get("Content-Type")
            if contentType == "" {
                contentType = "unknown"
            }
            m.sizeTracker.WithLabelValues(method, endpoint, contentType).Observe(float64(requestSize))
        }
        
        // 【STEP 5】ユーザーエージェント分析
        if m.config.EnableUserAgent {
            userAgentType := m.classifyUserAgent(r.Header.Get("User-Agent"))
            m.userAgentTracker.WithLabelValues(userAgentType).Inc()
        }
        
        // 【STEP 6】地理的位置分析
        if m.config.EnableGeoTracking {
            country, region := m.getGeolocation(r)
            if country != "" {
                m.geolocationTracker.WithLabelValues(country, region).Inc()
            }
        }
        
        // 【STEP 7】レスポンスライター拡張
        ww := &enhancedResponseWriter{
            ResponseWriter: w,
            statusCode:     http.StatusOK,
            bytesWritten:   0,
        }
        
        // 【STEP 8】エラー処理付きリクエスト実行
        defer func() {
            if err := recover(); err != nil {
                log.Printf("💥 Panic in request %s %s: %v", method, endpoint, err)
                m.errorCounter.WithLabelValues(method, endpoint, "panic").Inc()
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        
        // 【STEP 9】実際の処理実行
        next.ServeHTTP(ww, r)
        
        // 【STEP 10】包括的メトリクス記録
        duration := time.Since(start)
        m.recordComprehensiveMetrics(method, endpoint, ww.statusCode, duration, requestSize, ww.bytesWritten, r)
    })
}

// 【包括的メトリクス記録】全次元でのパフォーマンス分析
func (m *HistogramMiddleware) recordComprehensiveMetrics(method, endpoint string, statusCode int, duration time.Duration, requestSize int64, responseSize int64, r *http.Request) {
    // 【基本メトリクス】
    userType := m.determineUserType(r)
    m.latencyTracker.TrackRequest(method, endpoint, statusCode, duration, requestSize, userType)
    
    // 【スループット記録】
    m.throughputCounter.WithLabelValues(method, endpoint).Inc()
    
    // 【エラー分析】
    if statusCode >= 400 {
        errorType := m.classifyError(statusCode)
        m.errorCounter.WithLabelValues(method, endpoint, errorType).Inc()
    }
    
    // 【サイズ別レスポンス時間分析】
    sizeCategory := m.categorizeSizeCategory(requestSize)
    m.responseTimeBySize.WithLabelValues(sizeCategory).Observe(duration.Seconds())
    
    // 【詳細ログ】重要なメトリクス
    if duration > m.config.SlowThreshold {
        log.Printf("⏰ Slow request: %s %s took %v (size: %d bytes, response: %d bytes)", 
            method, endpoint, duration, requestSize, responseSize)
    }
    
    if requestSize > m.config.LargeRequestSize {
        log.Printf("📦 Large request: %s %s size %d bytes took %v", 
            method, endpoint, requestSize, duration)
    }
}

// 【ユーザーエージェント分類】クライアント種別の判定
func (m *HistogramMiddleware) classifyUserAgent(userAgent string) string {
    userAgent = strings.ToLower(userAgent)
    
    switch {
    case strings.Contains(userAgent, "bot") || strings.Contains(userAgent, "crawler") || strings.Contains(userAgent, "spider"):
        return "bot"
    case strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "android"):
        return "mobile"
    case strings.Contains(userAgent, "curl") || strings.Contains(userAgent, "wget") || strings.Contains(userAgent, "postman"):
        return "api"
    case strings.Contains(userAgent, "chrome") || strings.Contains(userAgent, "firefox") || strings.Contains(userAgent, "safari"):
        return "browser"
    default:
        return "unknown"
    }
}

// 【地理的位置取得】IPアドレスからの位置情報推定
func (m *HistogramMiddleware) getGeolocation(r *http.Request) (country, region string) {
    // 【実装例】実際の実装では GeoIP ライブラリを使用
    clientIP := m.getClientIP(r)
    
    // プレースホルダー実装
    // 実際の実装では MaxMind GeoIP2 や類似のライブラリを使用
    if strings.HasPrefix(clientIP, "192.168.") || strings.HasPrefix(clientIP, "10.") {
        return "private", "internal"
    }
    
    // 簡易的な判定例
    return "unknown", "unknown"
}

// 【ユーザー種別判定】認証情報からユーザーカテゴリを判定
func (m *HistogramMiddleware) determineUserType(r *http.Request) string {
    // 【実装例】実際の実装では認証トークンやセッション情報を使用
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return "guest"
    }
    
    // JWT トークンの解析やセッション情報の確認
    // プレースホルダー実装
    if strings.Contains(authHeader, "premium") {
        return "premium"
    }
    
    return "standard"
}

// 【エラー分類】HTTPステータスコードの詳細分類
func (m *HistogramMiddleware) classifyError(statusCode int) string {
    switch {
    case statusCode >= 400 && statusCode < 500:
        switch statusCode {
        case 401:
            return "unauthorized"
        case 403:
            return "forbidden"
        case 404:
            return "not_found"
        case 429:
            return "rate_limit"
        default:
            return "client_error"
        }
    case statusCode >= 500:
        switch statusCode {
        case 500:
            return "internal_error"
        case 502:
            return "bad_gateway"
        case 503:
            return "service_unavailable"
        case 504:
            return "gateway_timeout"
        default:
            return "server_error"
        }
    default:
        return "unknown"
    }
}

// 【サイズカテゴリ分類】リクエストサイズの分類
func (m *HistogramMiddleware) categorizeSizeCategory(size int64) string {
    switch {
    case size < 1024:
        return "small"   // < 1KB
    case size < 65536:
        return "medium"  // 1KB - 64KB
    case size < 1048576:
        return "large"   // 64KB - 1MB
    default:
        return "xlarge"  // > 1MB
    }
}

// 【現在の同時実行数取得】負荷制御用
func (m *HistogramMiddleware) getCurrentConcurrency(endpoint, method string) int {
    // 【実装例】実際の実装では Prometheus metrics から取得
    // ここでは簡易的な実装
    return 0
}

// 【クライアントIP取得】プロキシ環境対応
func (m *HistogramMiddleware) getClientIP(r *http.Request) string {
    // X-Forwarded-For, X-Real-IP, RemoteAddr の順で確認
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        return strings.Split(xff, ",")[0]
    }
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }
    return strings.Split(r.RemoteAddr, ":")[0]
}

// 【拡張レスポンスライター】詳細な応答情報追跡
type enhancedResponseWriter struct {
    http.ResponseWriter
    statusCode   int
    bytesWritten int64
}

func (rw *enhancedResponseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func (rw *enhancedResponseWriter) Write(b []byte) (int, error) {
    n, err := rw.ResponseWriter.Write(b)
    rw.bytesWritten += int64(n)
    return n, err
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