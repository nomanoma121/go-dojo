# Day 57: Prometheus Custom Metrics

## 🎯 本日の目標 (Today's Goal)

Prometheusカスタムメトリクスを実装し、HTTPリクエスト数、エラー率、レスポンス時間などのビジネスメトリクスを収集・公開する仕組みを習得する。プロダクションレベルの監視とアラートの基盤を構築する。

## 📖 解説 (Explanation)

### Prometheusとは

Prometheusは、SoundCloudで開発されたオープンソースの監視・アラートシステムです。時系列データベースとして設計されており、マイクロサービスやクラウドネイティブなアプリケーションの監視に特化しています。

```go
// 【Prometheus Metricsの重要性】運用可視性とシステム安定性の確保
// ❌ 問題例：メトリクス収集なしによる運用の盲点
func catastrophicBlindSystemOperation() {
    // 🚨 災害例：監視なしWebサーバーの運用
    
    // 【問題のシステム】メトリクス収集機能なし
    server := &http.Server{
        Addr:    ":8080",
        Handler: http.DefaultServeMux,
    }
    
    // 【致命的問題】システム状態が完全に不可視
    http.HandleFunc("/api/orders", func(w http.ResponseWriter, r *http.Request) {
        // 【監視不可能な処理】以下の情報が一切取得できない：
        // 1. リクエスト数: 何件のリクエストが来ているか不明
        // 2. レスポンス時間: ユーザー体験の品質が不明
        // 3. エラー率: 障害の発生頻度・種類が不明
        // 4. リソース使用率: CPU・メモリ・ディスクの使用状況不明
        // 5. ビジネスメトリクス: 売上・注文数・ユーザー行動不明
        
        // 実際のビジネスロジック（完全にブラックボックス）
        processOrder(r)
        
        // 【実際の災害シナリオ】：
        // 月曜朝9時：突然のアクセス集中でレスポンス時間が10秒に
        // → 運営チームは気づかない（監視なし）
        // → 顧客からの苦情で初めて障害を認知（2時間後）
        // → 原因調査に6時間（ログしかない状態）
        // → 修正に4時間（影響範囲が不明）
        // 
        // 【損害の詳細】：
        // - 顧客離脱: 2時間 × 遅延体験 = 推定70%のユーザーが離脱
        // - 売上損失: 1時間あたり500万円 × 12時間 = 6000万円
        // - 信頼失墜: SNSでの拡散、ブランドイメージ低下
        // - 復旧コスト: エンジニア10人 × 12時間 = 人的コスト大
        // - 機会損失: 競合他社へのユーザー流出
        
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Order processed"))
    })
    
    log.Println("Starting server without any monitoring...")
    
    // 【結果】：システムはブラックボックス状態で運用される
    // - パフォーマンス劣化の早期検知不可能
    // - 容量計画の根拠データなし
    // - SLA遵守状況の把握不可能
    // - 障害の予兆検知不可能
    
    server.ListenAndServe()
}

// ✅ 正解：エンタープライズ級Prometheus監視システム
type EnterprisePrometheusSystem struct {
    // 【基本メトリクス収集】
    registry          *prometheus.Registry        // メトリクス登録管理
    collector         *MetricsCollector          // メトリクス収集器
    exporter          *PrometheusExporter        // Prometheus形式エクスポート
    pusher            *PrometheusPusher          // プッシュゲートウェイ
    
    // 【高度メトリクス分析】
    aggregator        *MetricsAggregator         // メトリクス集約
    correlator        *MetricsCorrelator         // メトリクス相関分析
    predictor         *TrendPredictor            // トレンド予測
    anomalyDetector   *AnomalyDetector           // 異常検知
    
    // 【ビジネスメトリクス】
    businessTracker   *BusinessMetricsTracker    // ビジネス指標追跡
    sliCalculator     *SLICalculator             // SLI計算エンジン
    sloMonitor        *SLOMonitor               // SLO監視
    budgetManager     *ErrorBudgetManager        // エラーバジェット管理
    
    // 【アラート・通知】
    alertManager      *PrometheusAlertManager    // アラート管理
    escalationManager *AlertEscalationManager    // エスカレーション管理
    notificationHub   *NotificationHub           // 通知ハブ
    incidentManager   *IncidentManager           // インシデント管理
    
    // 【ダッシュボード・可視化】
    dashboardManager  *GrafanaDashboardManager   // Grafanaダッシュボード
    reportGenerator   *MetricsReportGenerator    // レポート生成
    heatmapGenerator  *HeatmapGenerator          // ヒートマップ生成
    topologyMapper    *ServiceTopologyMapper     // サービス依存関係マップ
    
    // 【運用・自動化】
    autoScaler        *MetricsBasedAutoScaler    // メトリクス連動スケーリング
    capacityPlanner   *CapacityPlanner          // 容量計画
    performanceOptimizer *PerformanceOptimizer   // パフォーマンス最適化
    costOptimizer     *CostOptimizer            // コスト最適化
}

// 【包括的メトリクス収集】企業レベルの監視システム
func (pms *EnterprisePrometheusSystem) InstrumentHTTPHandler(serviceName string, handler http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        startTime := time.Now()
        
        // 【STEP 1】リクエスト開始メトリクス
        pms.recordRequestStart(serviceName, r)
        
        // 【STEP 2】レスポンスライターのラップ（ステータスコード取得用）
        recorder := &ResponseRecorder{
            ResponseWriter: w,
            StatusCode:     http.StatusOK,
            BytesWritten:   0,
        }
        
        // 【STEP 3】ビジネスコンテキスト情報抽出
        businessContext := pms.extractBusinessContext(r)
        
        defer func() {
            duration := time.Since(startTime)
            
            // 【基本HTTPメトリクス】
            pms.recordHTTPMetrics(serviceName, r, recorder, duration)
            
            // 【ビジネスメトリクス】
            pms.recordBusinessMetrics(businessContext, recorder.StatusCode, duration)
            
            // 【パフォーマンスメトリクス】
            pms.recordPerformanceMetrics(serviceName, r.URL.Path, duration)
            
            // 【リソースメトリクス】
            pms.recordResourceUsage(serviceName)
            
            // 【SLI/SLO評価】
            pms.evaluateSLI(serviceName, recorder.StatusCode, duration)
            
            // 【異常検知】
            pms.detectAnomalies(serviceName, duration, recorder.StatusCode)
        }()
        
        // 【実際のハンドラー実行】
        handler.ServeHTTP(recorder, r)
    })
}

// 【ビジネスメトリクス追跡】売上・ユーザー行動の可視化
func (pms *EnterprisePrometheusSystem) recordBusinessMetrics(context *BusinessContext, statusCode int, duration time.Duration) {
    if context == nil {
        return
    }
    
    businessLabels := prometheus.Labels{
        "user_segment":    context.UserSegment,
        "product_category": context.ProductCategory,
        "campaign_id":     context.CampaignID,
        "ab_test_variant": context.ABTestVariant,
        "device_type":     context.DeviceType,
        "country":         context.Country,
    }
    
    switch context.BusinessEvent {
    case "order_placed":
        // 【注文完了メトリクス】
        pms.ordersTotal.With(businessLabels).Inc()
        if context.OrderValue > 0 {
            pms.orderValue.With(businessLabels).Add(context.OrderValue)
        }
        
        // 【コンバージョン追跡】
        pms.conversionsByFunnel.With(prometheus.Labels{
            "funnel_step": "purchase",
            "variant":     context.ABTestVariant,
        }).Inc()
        
    case "user_signup":
        // 【ユーザー登録メトリクス】
        pms.userSignupsTotal.With(businessLabels).Inc()
        
        // 【獲得コスト計算用】
        if context.AcquisitionChannel != "" {
            pms.acquisitionsByChannel.With(prometheus.Labels{
                "channel": context.AcquisitionChannel,
                "cost_bucket": pms.getCostBucket(context.AcquisitionCost),
            }).Inc()
        }
        
    case "payment_processed":
        // 【決済メトリクス】
        if statusCode == 200 {
            pms.paymentsSuccessTotal.With(businessLabels).Inc()
            pms.paymentAmount.With(businessLabels).Add(context.PaymentAmount)
        } else {
            pms.paymentsFailedTotal.With(prometheus.Labels{
                "failure_reason": context.PaymentFailureReason,
                "payment_method": context.PaymentMethod,
            }).Inc()
        }
    }
}
```

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

// 【Counter基本実装】単調増加するメトリクス
var totalRequests = prometheus.NewCounter(prometheus.CounterOpts{
    Name: "http_requests_total",
    Help: "Total number of HTTP requests",
})

// 【ラベル付きCounter】多次元メトリクス
var requestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total number of HTTP requests",
    },
    []string{"method", "endpoint", "status"},
)

// 【使用方法とCPU効率】
func recordHTTPRequest(method, endpoint string, statusCode int) {
    // 【STEP 1】基本カウンター増加（最高速）
    totalRequests.Inc()  // 1増加
    
    // 【STEP 2】ラベル付きカウンター（次元性データ）
    status := fmt.Sprintf("%d", statusCode)
    
    // 【内部動作】：
    // - ラベル値の組み合わせごとに独立したカウンターを内部生成
    // - 例：method="GET", endpoint="/api/users", status="200"
    // - ハッシュマップで高速検索・更新
    // - メモリ使用量：約64bytes/ラベル組み合わせ
    requestsTotal.WithLabelValues(method, endpoint, status).Inc()
    
    // 【高負荷時の最適化】バッチ増加
    if statusCode >= 500 {
        // 重大エラー時は大幅増加で優先度を示す
        requestsTotal.WithLabelValues(method, endpoint, status).Add(5)
    }
}

// 【メトリクス収集例】実際の本番環境での使用パターン
func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    
    // 【事前記録】リクエスト開始時点
    totalRequests.Inc()
    
    // ... ビジネスロジック実行 ...
    
    // 【事後記録】レスポンス完了時点
    status := 200
    if err != nil {
        status = 500
    }
    
    // 【重要】エラー状況も含めて完全な状況を記録
    requestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", status)).Inc()
    
    // 【運用効果】：
    // - リクエスト傾向の可視化
    // - エラー率の監視
    // - エンドポイント別の負荷分析
    // - アラート条件の設定基盤
}
```

#### 2. Gauge（ゲージ）

現在の値を表すメトリクス。増減両方が可能で、スナップショット的な値を表します。

**使用例:**
- CPU使用率
- メモリ使用量
- アクティブな接続数
- キューのサイズ

```go
// 【Gauge基本実装】現在値を表すメトリクス
var cpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
    Name: "cpu_usage_percent",
    Help: "Current CPU usage percentage",
})

// 【ラベル付きGauge】サービス別の状態管理
var activeConnections = prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "active_connections",
        Help: "Number of active connections",
    },
    []string{"service", "protocol"},
)

// 【使用方法とリアルタイム監視】
func updateSystemMetrics() {
    // 【STEP 1】直接値設定（スナップショット）
    currentCPU := getCurrentCPUUsage()
    cpuUsage.Set(currentCPU)  // 現在値を設定
    
    // 【STEP 2】増減操作（相対変化）
    if systemLoad == "high" {
        cpuUsage.Inc()    // 1増加
    } else if systemLoad == "low" {
        cpuUsage.Dec()    // 1減少
    }
    
    // 【STEP 3】任意の値による増減
    loadDelta := calculateLoadDelta()
    cpuUsage.Add(loadDelta)  // 正負両方の値で増減
    
    // 【STEP 4】サービス別のコネクション状態
    // 【内部動作】：
    // - 各ラベル組み合わせが独立したGauge
    // - リアルタイムで現在値を反映
    // - 時系列データベースで履歴を保存
    activeConnections.WithLabelValues("api", "http").Set(float64(getHTTPConnections()))
    activeConnections.WithLabelValues("api", "grpc").Set(float64(getGRPCConnections()))
    activeConnections.WithLabelValues("db", "postgres").Set(float64(getDBConnections()))
    
    // 【運用での重要性】：
    // - しきい値アラート設定
    // - 容量計画の基礎データ
    // - リアルタイム監視ダッシュボード
    // - 異常検知の基準値
}

// 【高頻度更新対応】効率的なGauge管理
type MetricsCollector struct {
    cpuGauge       prometheus.Gauge
    memoryGauge    prometheus.Gauge
    updateInterval time.Duration
    stopChan       chan struct{}
}

func (mc *MetricsCollector) StartCollection() {
    ticker := time.NewTicker(mc.updateInterval)
    go func() {
        for {
            select {
            case <-ticker.C:
                // 【定期収集】システム状態の定期的な取得
                mc.collectSystemMetrics()
            case <-mc.stopChan:
                ticker.Stop()
                return
            }
        }
    }()
}

func (mc *MetricsCollector) collectSystemMetrics() {
    // 【効率的な収集】一回のシステムコールで複数メトリクス取得
    stats := getSystemStats()
    
    // 【原子的更新】複数のGaugeを同時に更新
    mc.cpuGauge.Set(stats.CPUPercent)
    mc.memoryGauge.Set(float64(stats.MemoryBytes))
    
    // 【ログ出力】異常値の検出と記録
    if stats.CPUPercent > 80 {
        log.Printf("⚠️  High CPU usage detected: %.2f%%", stats.CPUPercent)
    }
}
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
// 【Histogram設計戦略】パフォーマンス分析に最適化されたバケット設計
var requestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "http_request_duration_seconds",
        Help: "HTTP request duration in seconds",
        // 【重要】Webアプリケーションに特化したバケット
        // 100ms以下: 優秀（ユーザー体験良好）
        // 500ms以下: 良好（許容範囲）
        // 1s以上: 改善要（ユーザー体験に影響）
        // 5s以上: 問題（タイムアウト対象）
        Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10},
    },
    []string{"method", "endpoint"},
)

// 【デフォルトバケット】汎用的な性能測定
var processingTime = prometheus.NewHistogram(prometheus.HistogramOpts{
    Name: "task_processing_seconds",
    Help: "Time spent processing tasks",
    // DefBuckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
    // 【特徴】：
    // - 5ms～10秒の広範囲をカバー
    // - 低レイテンシから高レイテンシまで対応
    // - 一般的な処理時間に最適化
    Buckets: prometheus.DefBuckets,
})

// 【実用的なHistogram使用パターン】
func measureAPIPerformance(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    
    // 【事前設定】測定開始時点
    method := r.Method
    endpoint := r.URL.Path
    
    // ... ビジネスロジック実行 ...
    
    // 【測定完了】経過時間の記録
    duration := time.Since(start).Seconds()
    
    // 【Histogram記録】分布データの蓄積
    requestDuration.WithLabelValues(method, endpoint).Observe(duration)
    
    // 【内部動作の詳細】：
    // Observe(0.25) の場合：
    // - http_request_duration_seconds_bucket{le="0.1"} += 0  (0.25 > 0.1)
    // - http_request_duration_seconds_bucket{le="0.5"} += 1  (0.25 <= 0.5)
    // - http_request_duration_seconds_bucket{le="1"} += 1    (累積)
    // - http_request_duration_seconds_bucket{le="2.5"} += 1  (累積)
    // - http_request_duration_seconds_bucket{le="5"} += 1    (累積)
    // - http_request_duration_seconds_bucket{le="10"} += 1   (累積)
    // - http_request_duration_seconds_bucket{le="+Inf"} += 1 (累積)
    // - http_request_duration_seconds_sum += 0.25           (合計値)
    // - http_request_duration_seconds_count += 1            (観測回数)
    
    // 【パフォーマンス分析】運用での活用方法
    if duration > 1.0 {
        log.Printf("⚠️  Slow request detected: %s %s took %.3fs", method, endpoint, duration)
    }
}

// 【カスタムバケット設計】ビジネス要件に応じたバケット戦略
var orderProcessingTime = prometheus.NewHistogram(prometheus.HistogramOpts{
    Name: "order_processing_seconds",
    Help: "Time spent processing orders",
    // 【注文処理専用バケット】
    // 5s以下: 即座処理（優秀）
    // 30s以下: 通常処理（良好）
    // 60s以下: 長時間処理（要監視）
    // 120s以上: 異常処理（要調査）
    Buckets: []float64{5, 30, 60, 120, 300, 600},
})

// 【大容量データ処理】用のバケット
var dataProcessingTime = prometheus.NewHistogram(prometheus.HistogramOpts{
    Name: "data_processing_seconds",
    Help: "Time spent processing large datasets",
    // 【バッチ処理専用バケット】
    // 分オーダーから時間オーダーまで対応
    Buckets: []float64{60, 300, 600, 1800, 3600, 7200}, // 1分～2時間
})

// 【統計分析】Histogramから統計情報を取得
func analyzePerformanceMetrics() {
    // 【PromQL例】実際の監視クエリ
    // 
    // 95パーセンタイル計算:
    // histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
    // 
    // 平均応答時間:
    // rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])
    // 
    // 1秒以上のリクエスト割合:
    // rate(http_request_duration_seconds_bucket{le="1"}[5m]) / rate(http_request_duration_seconds_count[5m])
    
    log.Printf("📊 Performance metrics collected and available for analysis")
}
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
// 【高性能メトリクス収集】プロダクション対応のミドルウェア
func metricsMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // 【STEP 1】アクティブ接続数の追跡
        activeConnections.Inc()
        defer activeConnections.Dec()
        
        // 【STEP 2】レスポンス情報キャプチャ用ラッパー
        ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        // 【STEP 3】パニック対応付きハンドラ実行
        defer func() {
            if err := recover(); err != nil {
                // パニック発生時もメトリクスを記録
                duration := time.Since(start).Seconds()
                httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, "500").Inc()
                httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
                
                // エラーログ
                log.Printf("💥 Panic in request %s %s: %v", r.Method, r.URL.Path, err)
                
                // HTTP 500エラーレスポンス
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        
        // 【STEP 4】実際の処理実行
        next(ww, r)
        
        // 【STEP 5】メトリクス記録（原子的操作）
        duration := time.Since(start).Seconds()
        status := fmt.Sprintf("%d", ww.statusCode)
        method := r.Method
        path := r.URL.Path
        
        // 【重要】全てのメトリクスを同時に更新
        httpRequestsTotal.WithLabelValues(method, path, status).Inc()
        httpRequestDuration.WithLabelValues(method, path).Observe(duration)
        
        // 【詳細分析】特定条件でのログ出力
        if duration > 1.0 {
            log.Printf("🐌 Slow request: %s %s took %.3fs (status: %s)", method, path, duration, status)
        }
        
        if ww.statusCode >= 500 {
            log.Printf("❌ Server error: %s %s returned %d", method, path, ww.statusCode)
        }
    }
}

// 【拡張ResponseWriter】ステータスコードとレスポンスサイズを追跡
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
    // 【拡張機能】レスポンスサイズの追跡
    rw.bytesWritten += int64(n)
    return n, err
}

// 【高度なメトリクス収集】詳細なHTTPメトリクス
func advancedMetricsMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // 【リクエスト情報の詳細収集】
        userAgent := r.Header.Get("User-Agent")
        clientIP := getClientIP(r)
        
        // 【拡張レスポンスライター】
        ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        // 【処理実行】
        next(ww, r)
        
        // 【包括的メトリクス記録】
        duration := time.Since(start).Seconds()
        status := fmt.Sprintf("%d", ww.statusCode)
        method := r.Method
        path := r.URL.Path
        
        // 【基本メトリクス】
        httpRequestsTotal.WithLabelValues(method, path, status).Inc()
        httpRequestDuration.WithLabelValues(method, path).Observe(duration)
        
        // 【レスポンスサイズメトリクス】
        if responseSizeHistogram != nil {
            responseSizeHistogram.WithLabelValues(method, path).Observe(float64(ww.bytesWritten))
        }
        
        // 【セキュリティメトリクス】
        if strings.Contains(userAgent, "bot") || strings.Contains(userAgent, "crawler") {
            botRequestsTotal.WithLabelValues(method, path).Inc()
        }
        
        // 【地理的メトリクス】IP地域別の分析
        region := getRegionFromIP(clientIP)
        if region != "" {
            requestsByRegion.WithLabelValues(region).Inc()
        }
        
        // 【異常検知】
        if ww.statusCode == 429 {
            // レート制限発動
            rateLimitHits.WithLabelValues(clientIP).Inc()
        }
        
        if ww.statusCode >= 400 {
            // クライアントエラー
            clientErrorsTotal.WithLabelValues(method, path, status).Inc()
        }
    }
}

// 【ユーティリティ関数】クライアントIP取得
func getClientIP(r *http.Request) string {
    // X-Forwarded-For, X-Real-IP, RemoteAddr の順で確認
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        return strings.Split(xff, ",")[0]
    }
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }
    return strings.Split(r.RemoteAddr, ":")[0]
}

// 【拡張メトリクス】レスポンスサイズ分析
var responseSizeHistogram = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "http_response_size_bytes",
        Help: "HTTP response size in bytes",
        Buckets: []float64{100, 1000, 10000, 100000, 1000000}, // 100B～1MB
    },
    []string{"method", "endpoint"},
)

// 【セキュリティメトリクス】ボットトラフィック
var botRequestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "bot_requests_total",
        Help: "Total number of bot requests",
    },
    []string{"method", "endpoint"},
)

// 【地理的メトリクス】地域別アクセス
var requestsByRegion = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "requests_by_region_total",
        Help: "Total number of requests by region",
    },
    []string{"region"},
)

// 【レート制限メトリクス】
var rateLimitHits = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "rate_limit_hits_total",
        Help: "Total number of rate limit hits",
    },
    []string{"client_ip"},
)

// 【クライアントエラーメトリクス】
var clientErrorsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "client_errors_total",
        Help: "Total number of client errors (4xx)",
    },
    []string{"method", "endpoint", "status"},
)
```

### ビジネスメトリクス

アプリケーション固有のビジネス指標の実装例：

```go
// 【ビジネスメトリクス設計】実際のプロダクトKPIに直結するメトリクス
var (
    // 【ユーザー関連メトリクス】顧客成長とエンゲージメント測定
    totalUsers = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "total_users",
        Help: "Total number of registered users",
    })
    
    userRegistrations = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "user_registrations_total",
        Help: "Total number of user registrations",
    })
    
    // 【拡張ユーザーメトリクス】アクティブユーザーの詳細分析
    activeUsers = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "active_users",
            Help: "Number of active users by time period",
        },
        []string{"period"}, // daily, weekly, monthly
    )
    
    userSessions = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "user_session_duration_seconds",
            Help: "User session duration in seconds",
            Buckets: []float64{60, 300, 900, 1800, 3600, 7200}, // 1分〜2時間
        },
        []string{"user_type"}, // premium, standard, guest
    )
    
    // 【注文関連メトリクス】売上とビジネス成果の測定
    totalOrders = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "orders_total",
            Help: "Total number of orders",
        },
        []string{"status"}, // created, paid, shipped, delivered, cancelled
    )
    
    orderValue = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "order_value_dollars",
            Help: "Order value in dollars",
            // 【重要】ビジネス戦略に基づくバケット設計
            // $10未満: 小額商品（デジタルコンテンツ等）
            // $50未満: 一般商品
            // $250未満: 中額商品
            // $1000以上: 高額商品・B2B取引
            Buckets: []float64{10, 50, 100, 250, 500, 1000, 2500, 5000},
        },
        []string{"currency", "product_category"},
    )
    
    // 【財務メトリクス】月次・年次収益の追跡
    totalRevenue = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "total_revenue_dollars",
            Help: "Total revenue in dollars",
        },
        []string{"currency", "revenue_type"}, // subscription, one_time, refund
    )
    
    // 【システムメトリクス】技術的健全性の監視
    databaseConnections = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "database_connections",
            Help: "Number of database connections",
        },
        []string{"state", "database"}, // active/idle, postgres/redis/mysql
    )
    
    cacheHitRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cache_hit_rate",
            Help: "Cache hit rate percentage",
        },
        []string{"cache_type", "service"}, // redis/memcache, user/product/session
    )
    
    // 【エラー率メトリクス】システム品質の監視
    applicationErrors = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "application_errors_total",
            Help: "Total number of application errors",
        },
        []string{"service", "error_type", "severity"}, // critical/warning/info
    )
)

// 【ビジネスロジック統合】メトリクス収集の実践的パターン
func createUser(user *User) error {
    start := time.Now()
    
    // 【事前記録】試行回数の追跡
    userRegistrationAttempts.Inc()
    
    err := userService.Create(user)
    if err == nil {
        // 【成功メトリクス】新規ユーザー獲得
        userRegistrations.Inc()
        totalUsers.Inc()
        
        // 【詳細分析】ユーザー属性別の分類
        if user.IsPremium {
            premiumUserRegistrations.Inc()
        }
        
        // 【地域別分析】
        if user.Country != "" {
            usersByCountry.WithLabelValues(user.Country).Inc()
        }
        
        // 【取得経路分析】
        if user.ReferralSource != "" {
            usersBySource.WithLabelValues(user.ReferralSource).Inc()
        }
        
        log.Printf("✅ New user registered: %s (total: %d)", user.Email, getCurrentUserCount())
        
    } else {
        // 【失敗分析】登録失敗の原因分類
        errorType := classifyRegistrationError(err)
        userRegistrationErrors.WithLabelValues(errorType).Inc()
        
        log.Printf("❌ User registration failed: %v (error type: %s)", err, errorType)
    }
    
    // 【パフォーマンス測定】
    registrationDuration.Observe(time.Since(start).Seconds())
    
    return err
}

func createOrder(order *Order) error {
    start := time.Now()
    orderAttempts.Inc()
    
    err := orderService.Create(order)
    if err == nil {
        // 【基本ビジネスメトリクス】
        totalOrders.WithLabelValues("created").Inc()
        orderValue.WithLabelValues(order.Currency, order.ProductCategory).Observe(order.Amount)
        
        // 【収益追跡】
        totalRevenue.WithLabelValues(order.Currency, "one_time").Add(order.Amount)
        
        // 【詳細ビジネス分析】
        if order.Amount > 1000 {
            highValueOrders.Inc()
        }
        
        // 【顧客セグメント分析】
        customerType := determineCustomerType(order.UserID)
        ordersByCustomerType.WithLabelValues(customerType).Inc()
        
        // 【在庫連動】
        for _, item := range order.Items {
            productSales.WithLabelValues(item.ProductID, item.Category).Inc()
            inventoryMovement.WithLabelValues(item.ProductID, "sold").Add(float64(item.Quantity))
        }
        
        log.Printf("💰 Order created: $%.2f %s (order ID: %s)", order.Amount, order.Currency, order.ID)
        
    } else {
        // 【注文失敗分析】
        failureReason := classifyOrderError(err)
        orderFailures.WithLabelValues(failureReason).Inc()
        
        // 【決済失敗の詳細分類】
        if strings.Contains(err.Error(), "payment") {
            paymentFailures.WithLabelValues(order.PaymentMethod, failureReason).Inc()
        }
    }
    
    // 【注文処理時間の監視】
    orderProcessingDuration.Observe(time.Since(start).Seconds())
    
    return err
}

// 【高度なビジネス分析】顧客ライフタイムバリュー計算
func updateCustomerMetrics(userID string, orderAmount float64) {
    // 【顧客価値計算】
    customerLifetimeValue := calculateCustomerLTV(userID)
    customerLTV.WithLabelValues("current").Set(customerLifetimeValue)
    
    // 【購入頻度分析】
    orderCount := getCustomerOrderCount(userID)
    if orderCount == 1 {
        firstTimeCustomers.Inc()
    } else {
        repeatCustomers.Inc()
        
        // 【リピート購入間隔】
        daysSinceLastOrder := getDaysSinceLastOrder(userID)
        repeatPurchaseInterval.Observe(float64(daysSinceLastOrder))
    }
    
    // 【顧客セグメント自動分類】
    segment := classifyCustomerSegment(customerLifetimeValue, orderCount)
    customerSegments.WithLabelValues(segment).Inc()
}

// 【補助メトリクス】ビジネス分析をサポートする追加指標
var (
    userRegistrationAttempts = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "user_registration_attempts_total",
        Help: "Total number of user registration attempts",
    })
    
    premiumUserRegistrations = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "premium_user_registrations_total",
        Help: "Total number of premium user registrations",
    })
    
    usersByCountry = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "users_by_country_total",
            Help: "Total number of users by country",
        },
        []string{"country"},
    )
    
    customerLTV = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "customer_lifetime_value_dollars",
            Help: "Customer lifetime value in dollars",
        },
        []string{"ltv_category"}, // current, predicted, average
    )
    
    registrationDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
        Name: "user_registration_duration_seconds",
        Help: "Time spent on user registration process",
        Buckets: prometheus.DefBuckets,
    })
)
```

### カスタムCollectorの実装

動的にメトリクスを収集する場合のカスタムCollector：

```go
// 【高度なカスタムCollector】動的メトリクス収集システム
type DBStatsCollector struct {
    db         *sql.DB
    dbName     string
    
    // 【基本接続メトリクス】
    openConnections    *prometheus.Desc
    inUseConnections   *prometheus.Desc
    idleConnections    *prometheus.Desc
    
    // 【詳細統計メトリクス】
    maxOpenConnections *prometheus.Desc
    waitCount          *prometheus.Desc
    waitDuration       *prometheus.Desc
    maxIdleClosed      *prometheus.Desc
    maxLifetimeClosed  *prometheus.Desc
}

func NewDBStatsCollector(db *sql.DB, dbName string) *DBStatsCollector {
    return &DBStatsCollector{
        db:     db,
        dbName: dbName,
        
        // 【メトリクス記述子定義】ラベル付きで詳細分類
        openConnections: prometheus.NewDesc(
            "database_open_connections",
            "Number of open database connections",
            []string{"database", "instance"}, // ラベルでDB種別とインスタンスを区別
            nil,
        ),
        inUseConnections: prometheus.NewDesc(
            "database_in_use_connections", 
            "Number of in-use database connections",
            []string{"database", "instance"},
            nil,
        ),
        idleConnections: prometheus.NewDesc(
            "database_idle_connections",
            "Number of idle database connections", 
            []string{"database", "instance"},
            nil,
        ),
        
        // 【拡張統計情報】
        maxOpenConnections: prometheus.NewDesc(
            "database_max_open_connections",
            "Maximum number of open connections allowed",
            []string{"database", "instance"},
            nil,
        ),
        waitCount: prometheus.NewDesc(
            "database_wait_count_total",
            "Total number of connections waited for",
            []string{"database", "instance"},
            nil,
        ),
        waitDuration: prometheus.NewDesc(
            "database_wait_duration_seconds_total",
            "Total time blocked waiting for new connections",
            []string{"database", "instance"},
            nil,
        ),
        maxIdleClosed: prometheus.NewDesc(
            "database_max_idle_closed_total",
            "Total number of connections closed due to SetMaxIdleConns",
            []string{"database", "instance"},
            nil,
        ),
        maxLifetimeClosed: prometheus.NewDesc(
            "database_max_lifetime_closed_total",
            "Total number of connections closed due to SetConnMaxLifetime",
            []string{"database", "instance"},
            nil,
        ),
    }
}

func (c *DBStatsCollector) Describe(ch chan<- *prometheus.Desc) {
    // 【記述子登録】全てのメトリクス記述子をPrometheusに通知
    ch <- c.openConnections
    ch <- c.inUseConnections
    ch <- c.idleConnections
    ch <- c.maxOpenConnections
    ch <- c.waitCount
    ch <- c.waitDuration
    ch <- c.maxIdleClosed
    ch <- c.maxLifetimeClosed
}

func (c *DBStatsCollector) Collect(ch chan<- prometheus.Metric) {
    // 【リアルタイム統計取得】データベース接続プールの現在状態
    stats := c.db.Stats()
    
    // 【インスタンス識別】複数DBインスタンス対応
    labels := []string{c.dbName, getInstanceID()}
    
    // 【基本メトリクス収集】接続プール状態
    ch <- prometheus.MustNewConstMetric(
        c.openConnections,
        prometheus.GaugeValue,
        float64(stats.OpenConnections),
        labels...,
    )
    
    ch <- prometheus.MustNewConstMetric(
        c.inUseConnections,
        prometheus.GaugeValue,
        float64(stats.InUse),
        labels...,
    )
    
    ch <- prometheus.MustNewConstMetric(
        c.idleConnections,
        prometheus.GaugeValue,
        float64(stats.Idle),
        labels...,
    )
    
    // 【設定値メトリクス】接続プール設定の可視化
    ch <- prometheus.MustNewConstMetric(
        c.maxOpenConnections,
        prometheus.GaugeValue,
        float64(stats.MaxOpenConnections),
        labels...,
    )
    
    // 【パフォーマンスメトリクス】待機統計
    ch <- prometheus.MustNewConstMetric(
        c.waitCount,
        prometheus.CounterValue,
        float64(stats.WaitCount),
        labels...,
    )
    
    ch <- prometheus.MustNewConstMetric(
        c.waitDuration,
        prometheus.CounterValue,
        stats.WaitDuration.Seconds(),
        labels...,
    )
    
    // 【接続管理メトリクス】クリーンアップ統計
    ch <- prometheus.MustNewConstMetric(
        c.maxIdleClosed,
        prometheus.CounterValue,
        float64(stats.MaxIdleClosed),
        labels...,
    )
    
    ch <- prometheus.MustNewConstMetric(
        c.maxLifetimeClosed,
        prometheus.CounterValue,
        float64(stats.MaxLifetimeClosed),
        labels...,
    )
    
    // 【健全性チェック】異常状態の検出
    c.checkDatabaseHealth(stats)
}

// 【健全性監視】データベース接続プールの問題検出
func (c *DBStatsCollector) checkDatabaseHealth(stats sql.DBStats) {
    utilizationRate := float64(stats.InUse) / float64(stats.MaxOpenConnections) * 100
    
    // 【アラート条件】
    if utilizationRate > 80 {
        log.Printf("⚠️  High DB connection utilization: %.1f%% (%d/%d)", 
            utilizationRate, stats.InUse, stats.MaxOpenConnections)
    }
    
    if stats.WaitCount > 0 {
        avgWaitTime := stats.WaitDuration.Milliseconds() / int64(stats.WaitCount)
        log.Printf("🕒 DB connection waits detected: %d waits, avg %dms", 
            stats.WaitCount, avgWaitTime)
    }
    
    // 【接続リーク検出】
    if stats.OpenConnections > stats.InUse+stats.Idle {
        leakedConnections := stats.OpenConnections - stats.InUse - stats.Idle
        log.Printf("🚨 Potential connection leak detected: %d connections unaccounted", 
            leakedConnections)
    }
}

// 【システムレベルCollector】マルチサービス環境対応
type SystemMetricsCollector struct {
    serviceName string
    
    // 【システムリソース】
    cpuUsage     *prometheus.Desc
    memoryUsage  *prometheus.Desc
    diskUsage    *prometheus.Desc
    networkIO    *prometheus.Desc
    
    // 【アプリケーション固有】
    goroutineCount *prometheus.Desc
    heapSize       *prometheus.Desc
    gcDuration     *prometheus.Desc
}

func NewSystemMetricsCollector(serviceName string) *SystemMetricsCollector {
    return &SystemMetricsCollector{
        serviceName: serviceName,
        
        cpuUsage: prometheus.NewDesc(
            "system_cpu_usage_percent",
            "System CPU usage percentage",
            []string{"service", "core"},
            nil,
        ),
        memoryUsage: prometheus.NewDesc(
            "system_memory_usage_bytes",
            "System memory usage in bytes",
            []string{"service", "type"}, // heap, stack, other
            nil,
        ),
        diskUsage: prometheus.NewDesc(
            "system_disk_usage_bytes",
            "System disk usage in bytes",
            []string{"service", "mount_point"},
            nil,
        ),
        networkIO: prometheus.NewDesc(
            "system_network_io_bytes_total",
            "System network I/O in bytes",
            []string{"service", "interface", "direction"}, // rx, tx
            nil,
        ),
        
        // 【Go固有メトリクス】
        goroutineCount: prometheus.NewDesc(
            "go_goroutines",
            "Number of goroutines that currently exist",
            []string{"service"},
            nil,
        ),
        heapSize: prometheus.NewDesc(
            "go_heap_size_bytes",
            "Go heap size in bytes",
            []string{"service", "type"}, // alloc, sys, idle
            nil,
        ),
        gcDuration: prometheus.NewDesc(
            "go_gc_duration_seconds",
            "Go garbage collection duration in seconds",
            []string{"service", "quantile"}, // 0.0, 0.25, 0.5, 0.75, 1.0
            nil,
        ),
    }
}

func (c *SystemMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
    ch <- c.cpuUsage
    ch <- c.memoryUsage
    ch <- c.diskUsage
    ch <- c.networkIO
    ch <- c.goroutineCount
    ch <- c.heapSize
    ch <- c.gcDuration
}

func (c *SystemMetricsCollector) Collect(ch chan<- prometheus.Metric) {
    // 【システム情報収集】
    systemStats := getSystemStats()
    runtimeStats := getGoRuntimeStats()
    
    labels := []string{c.serviceName}
    
    // 【CPU使用率】コア別の詳細情報
    for i, cpuPercent := range systemStats.CPUPerCore {
        ch <- prometheus.MustNewConstMetric(
            c.cpuUsage,
            prometheus.GaugeValue,
            cpuPercent,
            c.serviceName, fmt.Sprintf("core-%d", i),
        )
    }
    
    // 【メモリ使用量】種別ごとの分類
    memoryTypes := map[string]uint64{
        "heap":  runtimeStats.HeapAlloc,
        "stack": runtimeStats.StackInuse,
        "other": runtimeStats.Sys - runtimeStats.HeapSys - runtimeStats.StackSys,
    }
    
    for memType, usage := range memoryTypes {
        ch <- prometheus.MustNewConstMetric(
            c.memoryUsage,
            prometheus.GaugeValue,
            float64(usage),
            c.serviceName, memType,
        )
    }
    
    // 【Go runtime情報】
    ch <- prometheus.MustNewConstMetric(
        c.goroutineCount,
        prometheus.GaugeValue,
        float64(runtimeStats.NumGoroutine),
        labels...,
    )
}

// 【Collector登録】複数のカスタムCollectorを一括登録
func RegisterCustomCollectors(db *sql.DB, serviceName string) {
    // 【データベース統計Collector】
    dbCollector := NewDBStatsCollector(db, "main")
    prometheus.MustRegister(dbCollector)
    
    // 【システム統計Collector】
    systemCollector := NewSystemMetricsCollector(serviceName)
    prometheus.MustRegister(systemCollector)
    
    log.Printf("📊 Custom collectors registered for service: %s", serviceName)
}

// 【ユーティリティ関数】
func getInstanceID() string {
    hostname, _ := os.Hostname()
    return hostname
}

// 【プレースホルダー関数】実際の実装では適切なシステム情報取得ライブラリを使用
type SystemStats struct {
    CPUPerCore []float64
    MemoryUsed uint64
    DiskUsed   uint64
}

type GoRuntimeStats struct {
    HeapAlloc    uint64
    StackInuse   uint64
    Sys          uint64
    HeapSys      uint64
    StackSys     uint64
    NumGoroutine int
}

func getSystemStats() SystemStats {
    // 実際の実装では /proc/stat, /proc/meminfo 等を読み取り
    return SystemStats{}
}

func getGoRuntimeStats() GoRuntimeStats {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    return GoRuntimeStats{
        HeapAlloc:    m.HeapAlloc,
        StackInuse:   m.StackInuse,
        Sys:          m.Sys,
        HeapSys:      m.HeapSys,
        StackSys:     m.StackSys,
        NumGoroutine: runtime.NumGoroutine(),
    }
}
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