# Day 57: Prometheus Custom Metrics

## 🎯 本日の目標 (Today's Goal)

Prometheusカスタムメトリクスを実装し、HTTPリクエスト数、エラー率、レスポンス時間などのビジネスメトリクスを収集・公開する仕組みを習得する。プロダクションレベルの監視とアラートの基盤を構築する。

## 📖 解説 (Explanation)

### Prometheusとは

Prometheusは、時系列データベースとして設計されたオープンソースの監視システムです。メトリクスの収集、保存、クエリ、アラートを統合的に提供します。

### メトリクスの種類

1. **Counter**: 単調増加する累積メトリクス
2. **Gauge**: 上下する値のメトリクス
3. **Histogram**: 値の分布を測定
4. **Summary**: クォンタイルを提供

### 実装パターン

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// カウンターメトリクス
var requestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total number of HTTP requests",
    },
    []string{"method", "endpoint", "status"},
)

// ヒストグラムメトリクス
var requestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "http_request_duration_seconds",
        Help: "HTTP request duration in seconds",
        Buckets: prometheus.DefBuckets,
    },
    []string{"method", "endpoint"},
)

// ゲージメトリクス
var activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
    Name: "active_connections",
    Help: "Number of active connections",
})

func init() {
    prometheus.MustRegister(requestsTotal)
    prometheus.MustRegister(requestDuration)
    prometheus.MustRegister(activeConnections)
}
```

## 📝 課題 (The Problem)

Prometheusメトリクスシステムを使用して以下の機能を実装してください：

1. **HTTPメトリクス**: リクエスト数、レスポンス時間、エラー率
2. **ビジネスメトリクス**: ユーザー数、注文数、売上
3. **システムメトリクス**: CPU、メモリ、ディスク使用率
4. **カスタムメトリクス**: アプリケーション固有の指標
5. **メトリクス公開**: `/metrics`エンドポイントでの公開

## 💡 ヒント (Hints)

- `prometheus/client_golang`ライブラリの使用
- メトリクスの適切なラベル設計
- パフォーマンスを考慮したメトリクス収集
- Grafanaでの可視化を意識したメトリクス設計