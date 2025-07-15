# Day 37: DBコネクションプール最適化実装

## 🎯 本日の目標

`sql.DB`のコネクションプール設定を深く理解し、負荷に応じた動的調整、監視機能、実用的な最適化パターンを実装する。プロダクション環境で求められる高度なデータベース接続管理技術を習得できるようになる。

## 📖 解説

### コネクションプールが解決する問題

データベースアプリケーションでは、接続管理が性能の鍵となります。適切なコネクションプール設定なしでは、以下の問題が発生します：

#### 問題1: 接続作成のオーバーヘッド

```go
// ❌ 毎回新しい接続を作成（非効率）
func badDatabaseAccess() error {
    for i := 0; i < 1000; i++ {
        db, err := sql.Open("postgres", dsn)
        if err != nil {
            return err
        }
        defer db.Close()
        
        // クエリ実行...
        // 接続作成・切断のオーバーヘッドが1000回発生
    }
    return nil
}
```

**この方法の問題点：**
- TCP接続確立のネットワークラウンドトリップ（通常2-10ms）
- データベース認証処理のオーバーヘッド
- SSL/TLSハンドシェイクの時間
- 接続数制限による接続拒否エラー

#### 問題2: 接続リークとリソース枯渇

```go
// ❌ 接続リークが発生する危険なパターン
func connectionLeakExample(db *sql.DB) error {
    for i := 0; i < 10000; i++ {
        go func() {
            // 長時間実行される処理
            rows, err := db.Query("SELECT * FROM large_table WHERE heavy_computation = ?", i)
            if err != nil {
                return // 接続がリークする
            }
            
            // rows.Close()し忘れ = 接続がリークする
            for rows.Next() {
                // 処理...
            }
            // rows.Close() が呼ばれていない
        }()
    }
    return nil
}
```

### Goのsql.DBコネクションプール詳細

Goの`database/sql`パッケージは、高度なコネクションプール機能を内蔵しています：

#### コネクションプールの内部動作

```go
// コネクションプールの状態を表現する構造体
type ConnectionPoolStats struct {
    MaxOpenConnections int // 設定された最大接続数
    OpenConnections    int // 現在のオープン接続数
    InUse             int // 使用中の接続数
    Idle              int // アイドル状態の接続数
    WaitCount         int64 // 接続待ちが発生した回数
    WaitDuration      time.Duration // 接続待ちの合計時間
    MaxIdleClosed     int64 // アイドル制限で閉じられた接続数
    MaxLifetimeClosed int64 // 生存時間制限で閉じられた接続数
}

// 実際のステータス取得
func getPoolStats(db *sql.DB) ConnectionPoolStats {
    stats := db.Stats()
    return ConnectionPoolStats{
        MaxOpenConnections: stats.MaxOpenConnections,
        OpenConnections:    stats.OpenConnections,
        InUse:             stats.InUse,
        Idle:              stats.Idle,
        WaitCount:         stats.WaitCount,
        WaitDuration:      stats.WaitDuration,
        MaxIdleClosed:     stats.MaxIdleClosed,
        MaxLifetimeClosed: stats.MaxLifetimeClosed,
    }
}
```

### 実用的なコネクションプール設定

#### 基本設定パターン

```go
package main

import (
    "database/sql"
    "time"
    "context"
    "fmt"
    _ "github.com/lib/pq"
)

// 環境別の推奨設定
type PoolConfig struct {
    MaxOpenConns    int
    MaxIdleConns    int
    ConnMaxLifetime time.Duration
    ConnMaxIdleTime time.Duration
    Environment     string
}

var poolConfigs = map[string]PoolConfig{
    "development": {
        MaxOpenConns:    5,
        MaxIdleConns:    2,
        ConnMaxLifetime: 1 * time.Hour,
        ConnMaxIdleTime: 30 * time.Minute,
        Environment:     "development",
    },
    "testing": {
        MaxOpenConns:    10,
        MaxIdleConns:    3,
        ConnMaxLifetime: 30 * time.Minute,
        ConnMaxIdleTime: 15 * time.Minute,
        Environment:     "testing",
    },
    "production": {
        MaxOpenConns:    25,
        MaxIdleConns:    5,
        ConnMaxLifetime: 5 * time.Minute,
        ConnMaxIdleTime: 1 * time.Minute,
        Environment:     "production",
    },
    "high-load": {
        MaxOpenConns:    100,
        MaxIdleConns:    20,
        ConnMaxLifetime: 2 * time.Minute,
        ConnMaxIdleTime: 30 * time.Second,
        Environment:     "high-load",
    },
}

func setupOptimizedConnectionPool(dsn, environment string) (*sql.DB, error) {
    config, exists := poolConfigs[environment]
    if !exists {
        config = poolConfigs["production"] // デフォルト設定
    }
    
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // コネクションプール設定を適用
    db.SetMaxOpenConns(config.MaxOpenConns)
    db.SetMaxIdleConns(config.MaxIdleConns)
    db.SetConnMaxLifetime(config.ConnMaxLifetime)
    db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

    // 接続テスト
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := db.PingContext(ctx); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    fmt.Printf("Database connection pool initialized for %s environment:\n", config.Environment)
    fmt.Printf("  MaxOpenConns: %d\n", config.MaxOpenConns)
    fmt.Printf("  MaxIdleConns: %d\n", config.MaxIdleConns)
    fmt.Printf("  ConnMaxLifetime: %v\n", config.ConnMaxLifetime)
    fmt.Printf("  ConnMaxIdleTime: %v\n", config.ConnMaxIdleTime)

    return db, nil
}
```

#### 動的コネクションプール調整

```go
// 負荷に応じてプール設定を動的に調整
type AdaptiveConnectionPool struct {
    db               *sql.DB
    currentMaxOpen   int
    currentMaxIdle   int
    lastAdjustment   time.Time
    adjustmentMutex  sync.RWMutex
    statsCollector   *PoolStatsCollector
}

type PoolStatsCollector struct {
    samples       []ConnectionPoolStats
    sampleCount   int
    maxSamples    int
    totalRequests int64
    mu            sync.RWMutex
}

func NewAdaptiveConnectionPool(db *sql.DB) *AdaptiveConnectionPool {
    return &AdaptiveConnectionPool{
        db:             db,
        currentMaxOpen: 25,
        currentMaxIdle: 5,
        lastAdjustment: time.Now(),
        statsCollector: &PoolStatsCollector{
            maxSamples: 100,
            samples:    make([]ConnectionPoolStats, 0, 100),
        },
    }
}

func (acp *AdaptiveConnectionPool) StartMonitoring(interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        defer ticker.Stop()
        for range ticker.C {
            acp.collectAndAdjust()
        }
    }()
}

func (acp *AdaptiveConnectionPool) collectAndAdjust() {
    stats := getPoolStats(acp.db)
    acp.statsCollector.addSample(stats)
    
    // 1分以上経過した場合のみ調整を検討
    if time.Since(acp.lastAdjustment) < time.Minute {
        return
    }
    
    acp.adjustmentMutex.Lock()
    defer acp.adjustmentMutex.Unlock()
    
    // 接続待ちが頻発している場合、接続数を増加
    if stats.WaitCount > 0 && stats.InUse >= acp.currentMaxOpen*80/100 {
        newMaxOpen := min(acp.currentMaxOpen+5, 100)
        newMaxIdle := min(acp.currentMaxIdle+2, 20)
        
        acp.db.SetMaxOpenConns(newMaxOpen)
        acp.db.SetMaxIdleConns(newMaxIdle)
        
        acp.currentMaxOpen = newMaxOpen
        acp.currentMaxIdle = newMaxIdle
        acp.lastAdjustment = time.Now()
        
        fmt.Printf("Pool expanded: MaxOpen=%d, MaxIdle=%d (Wait events detected)\n", 
            newMaxOpen, newMaxIdle)
    }
    
    // 使用率が低い場合、接続数を減少
    if stats.InUse <= acp.currentMaxOpen*20/100 && acp.currentMaxOpen > 5 {
        newMaxOpen := max(acp.currentMaxOpen-3, 5)
        newMaxIdle := max(acp.currentMaxIdle-1, 2)
        
        acp.db.SetMaxOpenConns(newMaxOpen)
        acp.db.SetMaxIdleConns(newMaxIdle)
        
        acp.currentMaxOpen = newMaxOpen
        acp.currentMaxIdle = newMaxIdle
        acp.lastAdjustment = time.Now()
        
        fmt.Printf("Pool shrunk: MaxOpen=%d, MaxIdle=%d (Low utilization)\n", 
            newMaxOpen, newMaxIdle)
    }
}

func (psc *PoolStatsCollector) addSample(stats ConnectionPoolStats) {
    psc.mu.Lock()
    defer psc.mu.Unlock()
    
    if len(psc.samples) >= psc.maxSamples {
        // 古いサンプルを削除（FIFO）
        copy(psc.samples, psc.samples[1:])
        psc.samples = psc.samples[:len(psc.samples)-1]
    }
    
    psc.samples = append(psc.samples, stats)
    psc.sampleCount++
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}
```

### パフォーマンス監視とアラート

プロダクション環境での継続的な監視：

```go
type PoolPerformanceMonitor struct {
    db           *sql.DB
    alertManager *AlertManager
    ticker       *time.Ticker
    done         chan struct{}
    metrics      *PoolMetrics
}

type PoolMetrics struct {
    UtilizationRate     float64
    WaitTimeP99         time.Duration
    ConnectionErrors    int64
    HealthCheckFailures int64
    LastFullPoolTime    time.Time
    mu                  sync.RWMutex
}

type AlertManager struct {
    webhookURL  string
    emailSender EmailSender
    slackBot    SlackBot
}

func NewPoolPerformanceMonitor(db *sql.DB, alertManager *AlertManager) *PoolPerformanceMonitor {
    return &PoolPerformanceMonitor{
        db:           db,
        alertManager: alertManager,
        ticker:       time.NewTicker(30 * time.Second),
        done:         make(chan struct{}),
        metrics:      &PoolMetrics{},
    }
}

func (ppm *PoolPerformanceMonitor) StartMonitoring() {
    go func() {
        defer ppm.ticker.Stop()
        for {
            select {
            case <-ppm.ticker.C:
                ppm.collectMetrics()
                ppm.checkAlertConditions()
            case <-ppm.done:
                return
            }
        }
    }()
}

func (ppm *PoolPerformanceMonitor) collectMetrics() {
    stats := ppm.db.Stats()
    
    ppm.metrics.mu.Lock()
    defer ppm.metrics.mu.Unlock()
    
    // 使用率計算
    if stats.MaxOpenConnections > 0 {
        ppm.metrics.UtilizationRate = float64(stats.InUse) / float64(stats.MaxOpenConnections)
    }
    
    // 接続待ち時間の計算（P99近似）
    if stats.WaitCount > 0 {
        avgWaitTime := stats.WaitDuration / time.Duration(stats.WaitCount)
        ppm.metrics.WaitTimeP99 = avgWaitTime * 3 // 簡易P99近似
    }
    
    // プールが満杯になった時刻を記録
    if stats.InUse == stats.MaxOpenConnections {
        ppm.metrics.LastFullPoolTime = time.Now()
    }
}

func (ppm *PoolPerformanceMonitor) checkAlertConditions() {
    ppm.metrics.mu.RLock()
    defer ppm.metrics.mu.RUnlock()
    
    // アラート条件1: 使用率が90%を超えている
    if ppm.metrics.UtilizationRate > 0.9 {
        alert := Alert{
            Level:   "WARNING",
            Message: fmt.Sprintf("Connection pool utilization high: %.2f%%", ppm.metrics.UtilizationRate*100),
            Time:    time.Now(),
        }
        ppm.alertManager.SendAlert(alert)
    }
    
    // アラート条件2: 接続待ち時間が1秒を超えている
    if ppm.metrics.WaitTimeP99 > time.Second {
        alert := Alert{
            Level:   "CRITICAL", 
            Message: fmt.Sprintf("Connection wait time critical: %v", ppm.metrics.WaitTimeP99),
            Time:    time.Now(),
        }
        ppm.alertManager.SendAlert(alert)
    }
    
    // アラート条件3: プールが満杯状態が5分以上続いている
    if !ppm.metrics.LastFullPoolTime.IsZero() && 
       time.Since(ppm.metrics.LastFullPoolTime) > 5*time.Minute {
        alert := Alert{
            Level:   "CRITICAL",
            Message: "Connection pool has been full for over 5 minutes",
            Time:    time.Now(),
        }
        ppm.alertManager.SendAlert(alert)
    }
}

type Alert struct {
    Level   string
    Message string
    Time    time.Time
}

func (am *AlertManager) SendAlert(alert Alert) {
    go func() {
        // Slack通知
        if am.slackBot != nil {
            am.slackBot.PostMessage(fmt.Sprintf("[%s] %s at %s", 
                alert.Level, alert.Message, alert.Time.Format(time.RFC3339)))
        }
        
        // Email通知（CRITICAL時のみ）
        if alert.Level == "CRITICAL" && am.emailSender != nil {
            am.emailSender.Send("DB Pool Alert", alert.Message)
        }
        
        // Webhook通知
        if am.webhookURL != "" {
            am.sendWebhook(alert)
        }
    }()
}
```

### 設定項目の詳細解説

#### MaxOpenConns（最大オープン接続数）

同時に開ける最大接続数を制限します。

```go
// デフォルトは無制限（0）
db.SetMaxOpenConns(25)

// 無制限に設定
db.SetMaxOpenConns(0)
```

**設定のポイント:**
- 高すぎる値: データベースの接続制限を超える可能性
- 低すぎる値: 接続待機によるパフォーマンス低下
- 推奨値: CPU数 × 2〜4 または データベースの接続制限の70-80%

#### MaxIdleConns（最大アイドル接続数）

プールに保持するアイドル接続の最大数です。

```go
// デフォルトは2
db.SetMaxIdleConns(5)

// アイドル接続を無効化
db.SetMaxIdleConns(0)
```

**設定のポイント:**
- 高すぎる値: 不要な接続によるリソース消費
- 低すぎる値: 接続作成のオーバーヘッド増加
- 推奨値: MaxOpenConnsの20-50%

#### ConnMaxLifetime（接続の最大生存時間）

接続が作成されてから自動的に閉じられるまでの時間です。

```go
// デフォルトは無制限
db.SetConnMaxLifetime(5 * time.Minute)

// 無制限に設定
db.SetConnMaxLifetime(0)
```

**設定のポイント:**
- データベース側のタイムアウトより短く設定
- ロードバランサーのタイムアウトより短く設定
- 推奨値: 1-30分

#### ConnMaxIdleTime（アイドル接続の最大生存時間）

アイドル状態の接続が閉じられるまでの時間です。

```go
// デフォルトは無制限
db.SetConnMaxIdleTime(30 * time.Second)

// 無制限に設定
db.SetConnMaxIdleTime(0)
```

### コネクションプール監視

```go
package main

import (
    "database/sql"
    "fmt"
    "time"
)

type PoolMonitor struct {
    db     *sql.DB
    name   string
    ticker *time.Ticker
    done   chan bool
}

func NewPoolMonitor(db *sql.DB, name string, interval time.Duration) *PoolMonitor {
    return &PoolMonitor{
        db:     db,
        name:   name,
        ticker: time.NewTicker(interval),
        done:   make(chan bool),
    }
}

func (pm *PoolMonitor) Start() {
    go func() {
        for {
            select {
            case <-pm.ticker.C:
                pm.logStats()
            case <-pm.done:
                return
            }
        }
    }()
}

func (pm *PoolMonitor) Stop() {
    pm.ticker.Stop()
    pm.done <- true
}

func (pm *PoolMonitor) logStats() {
    stats := pm.db.Stats()
    fmt.Printf("[%s] Pool Stats: Open=%d, InUse=%d, Idle=%d, WaitCount=%d, WaitDuration=%v\n",
        pm.name,
        stats.OpenConnections,
        stats.InUse,
        stats.Idle,
        stats.WaitCount,
        stats.WaitDuration,
    )
}
```

### 環境別の最適化設定

#### 開発環境

```go
func setupDevelopmentPool(db *sql.DB) {
    db.SetMaxOpenConns(5)
    db.SetMaxIdleConns(2)
    db.SetConnMaxLifetime(1 * time.Minute)
    db.SetConnMaxIdleTime(30 * time.Second)
}
```

#### ステージング環境

```go
func setupStagingPool(db *sql.DB) {
    db.SetMaxOpenConns(15)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(3 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)
}
```

#### 本番環境

```go
func setupProductionPool(db *sql.DB) {
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(10)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(2 * time.Minute)
}
```

### パフォーマンステスト

```go
func BenchmarkConnectionPool(b *testing.B) {
    db := setupTestDB()
    defer db.Close()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            var count int
            err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
            if err != nil {
                b.Error(err)
            }
        }
    })
}
```

📝 **課題**

以下の機能を持つコネクションプール管理システムを実装してください：

1. **`PoolConfig`**: コネクションプール設定構造体
2. **`ConnectionManager`**: 接続管理システム
3. **`HealthChecker`**: データベース接続ヘルスチェック
4. **`PoolMonitor`**: リアルタイム統計監視
5. **`LoadTester`**: コネクションプール負荷テスト
6. **動的設定変更**: 実行時の設定調整機能

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestPoolConfig_Apply
--- PASS: TestPoolConfig_Apply (0.01s)
=== RUN   TestConnectionManager_BasicOperations
--- PASS: TestConnectionManager_BasicOperations (0.02s)
=== RUN   TestHealthChecker_Integration
--- PASS: TestHealthChecker_Integration (0.05s)
=== RUN   TestPoolMonitor_Statistics
--- PASS: TestPoolMonitor_Statistics (0.10s)
=== RUN   TestLoadTester_ConcurrentAccess
--- PASS: TestLoadTester_ConcurrentAccess (0.15s)
PASS
ok      day37-connection-pool    0.330s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **database/sql**パッケージ: コネクションプール設定
2. **time**パッケージ: タイムアウト設定とタイマー
3. **context**パッケージ: タイムアウト付きクエリ実行
4. **sync**パッケージ: 統計データの並行安全性
5. **testing**パッケージ: ベンチマークテストの実装

設定のポイント：
- **環境に応じた最適化**: 開発/ステージング/本番で異なる設定
- **監視とアラート**: プール使用率の監視
- **グレースフル設定変更**: 既存接続に影響を与えない更新
- **負荷テスト**: 設定の妥当性検証

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```