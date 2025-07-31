# Day 44: Cache Invalidation Strategies

## 🎯 本日の目標 (Today's Goal)

様々なキャッシュ無効化戦略を実装し、データの整合性を保ちながら効率的なキャッシュ管理を行える技術を習得する。TTL、タグベース無効化、依存関係管理などの高度なキャッシュ戦略を理解する。

## 📖 解説 (Explanation)

### キャッシュ無効化の重要性

キャッシュは性能向上のために不可欠ですが、古いデータが残り続けるとシステムの整合性が損なわれます。効果的な無効化戦略により、パフォーマンスと整合性のバランスを取ります。

### 主な無効化戦略

#### 1. TTL (Time To Live) ベース
- 時間経過による自動無効化
- 設定が簡単で予測可能
- データの更新頻度に基づく調整が重要

#### 2. イベントドリブン無効化
- データ更新時の即座な無効化
- 高い整合性を保証
- 複雑な依存関係の管理が必要

#### 3. タグベース無効化
- 関連データをグループ化して一括無効化
- 柔軟な無効化ポリシー
- Redis Sets を活用した効率的な実装

#### 4. 依存関係ベース無効化
- データ間の依存関係を定義
- 連鎖的な無効化処理
- グラフ理論を活用した最適化

## 📝 課題 (The Problem)

以下の機能を持つ高度なキャッシュ無効化システムを実装してください：

### 1. CacheInvalidator の実装

```go
type CacheInvalidator struct {
    cache     CacheClient
    tagStore  TagStore
    ruleEngine RuleEngine
    metrics   *InvalidationMetrics
}
```

### 2. 必要なメソッドの実装

- `InvalidateByKey(ctx context.Context, key string) error`: 個別キー無効化
- `InvalidateByTag(ctx context.Context, tag string) error`: タグベース無効化
- `InvalidateByPattern(ctx context.Context, pattern string) error`: パターンマッチ無効化
- `InvalidateRelated(ctx context.Context, key string) error`: 関連データ無効化
- `SetTTL(ctx context.Context, key string, ttl time.Duration) error`: TTL更新
- `AddInvalidationRule(rule InvalidationRule) error`: 無効化ルール追加

### 3. 高度な機能

- 無効化の遅延実行とバッチ処理
- 無効化パフォーマンスの監視
- 循環依存の検出と回避
- 無効化失敗時の再試行機能

## ✅ 期待される挙動 (Expected Behavior)

```bash
$ go test -v
=== RUN   TestCacheInvalidation_TagBased
    main_test.go:85: Tagged cache invalidation successful
    main_test.go:92: All related items invalidated: 15
--- PASS: TestCacheInvalidation_TagBased (0.03s)

=== RUN   TestCacheInvalidation_DependencyChain
    main_test.go:125: Dependency chain invalidation completed
    main_test.go:132: Cascaded invalidation affected 8 keys
--- PASS: TestCacheInvalidation_DependencyChain (0.02s)
```

## 💡 ヒント (Hints)

### 基本構造

```go
type InvalidationRule struct {
    Trigger   string        // トリガーとなるキー
    Targets   []string      // 無効化対象のキー/パターン
    Delay     time.Duration // 遅延時間
    Condition func() bool   // 実行条件
}

type TagStore interface {
    AddTag(ctx context.Context, key, tag string) error
    GetKeysByTag(ctx context.Context, tag string) ([]string, error)
    RemoveTag(ctx context.Context, key, tag string) error
}
```

### Redis Lua スクリプトによる効率化

```lua
-- タグに関連するすべてのキーを一括削除
local tag = ARGV[1]
local keys = redis.call('SMEMBERS', 'tag:' .. tag)
for i=1,#keys do
    redis.call('DEL', keys[i])
end
redis.call('DEL', 'tag:' .. tag)
return #keys
```

## 🚨 エンタープライズ環境での実災害シナリオと対策

### 実際の障害事例とリカバリー戦略

#### ❌ 災害事例1: Redisクラスター部分障害による無効化失敗

**発生状況:** 
- 大手ECサイトでRedisクラスターの1ノードが障害でダウン
- 該当シャードの商品価格キャッシュが無効化できず
- セール価格更新が反映されずに正規価格で販売継続

**技術的な問題:**
```go
// ❌ 問題のあるコード例
func (invalidator *SimpleInvalidator) InvalidatePrice(productID string) error {
    key := fmt.Sprintf("price:%s", productID)
    // 単一クラスターのみに依存 - 障害時に失敗
    return invalidator.redisClient.Del(key).Err()
}
```

**ビジネス影響:**
- 推定損失: 売上2,000万円の機会損失
- 顧客満足度低下: 価格不整合によるクレーム500件
- システム信頼性低下: SLA違反による契約問題

✅ **企業レベルの冗長化対策:**

```go
type ResilientInvalidator struct {
    primaryCluster   *redis.ClusterClient
    fallbackCluster  *redis.ClusterClient
    backupQueue      *InvalidationQueue
    alertManager     *AlertManager
    metrics         *InvalidationMetrics
}

func (r *ResilientInvalidator) InvalidateWithFailover(
    ctx context.Context, key string) error {
    
    var errors []error
    
    // Primary cluster への無効化試行
    if err := r.primaryCluster.Del(ctx, key).Err(); err != nil {
        r.metrics.PrimaryFailures.Inc()
        errors = append(errors, fmt.Errorf("primary cluster failed: %w", err))
        
        // 即座にアラート送信
        r.alertManager.SendAlert(AlertLevel.Warning, 
            fmt.Sprintf("Primary cache invalidation failed for key: %s", key))
    }
    
    // Fallback cluster への無効化試行
    if err := r.fallbackCluster.Del(ctx, key).Err(); err != nil {
        r.metrics.FallbackFailures.Inc()
        errors = append(errors, fmt.Errorf("fallback cluster failed: %w", err))
    }
    
    // 両方失敗した場合はバックアップキューに保存
    if len(errors) == 2 {
        r.backupQueue.Enqueue(InvalidationTask{
            Key:        key,
            Timestamp:  time.Now(),
            RetryCount: 0,
            Priority:   High,
        })
        
        r.alertManager.SendAlert(AlertLevel.Critical,
            fmt.Sprintf("ALL cache invalidation failed for key: %s", key))
    }
    
    return combineErrors(errors)
}

// バックグラウンドでのリトライ処理
func (r *ResilientInvalidator) startRetryWorker(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            r.processRetryQueue(ctx)
        }
    }
}

func (r *ResilientInvalidator) processRetryQueue(ctx context.Context) {
    tasks := r.backupQueue.DequeueAll()
    
    for _, task := range tasks {
        if task.RetryCount >= 3 {
            // 3回失敗したらDead Letter Queueへ
            r.backupQueue.MoveToDLQ(task)
            continue
        }
        
        if err := r.InvalidateWithFailover(ctx, task.Key); err != nil {
            task.RetryCount++
            task.NextRetry = time.Now().Add(
                time.Duration(task.RetryCount) * time.Minute)
            r.backupQueue.Enqueue(task)
        }
    }
}
```

#### ❌ 災害事例2: 大規模バッチ無効化による性能劣化

**発生状況:**
- セール開始時に100万個の商品キャッシュを一括無効化
- 無効化処理に5分かかり、古い価格情報が残存
- データベースへの大量アクセスでレスポンス時間が30秒に劣化

**問題の分析:**
```go
// ❌ 問題のあるコード例 - 逐次処理で遅い
func (invalidator *NaiveInvalidator) InvalidateBatch(keys []string) error {
    for _, key := range keys {  // 100万回のループ
        if err := invalidator.client.Del(key).Err(); err != nil {
            return err  // 1つ失敗すると全体が止まる
        }
        time.Sleep(1 * time.Millisecond)  // 過度な配慮で遅延
    }
    return nil
}
```

✅ **高性能バッチ無効化システム:**

```go
type HighPerformanceBatchInvalidator struct {
    workerCount     int
    batchSize       int
    rateLimiter     *rate.Limiter
    clients         []*redis.Client  // 複数コネクション
    metrics        *BatchMetrics
}

func (b *HighPerformanceBatchInvalidator) InvalidateMassively(
    ctx context.Context, keys []string) error {
    
    start := time.Now()
    defer func() {
        b.metrics.BatchDuration.Observe(time.Since(start).Seconds())
        b.metrics.BatchSize.Observe(float64(len(keys)))
    }()
    
    // チャンクに分割
    chunks := b.chunkKeys(keys, b.batchSize)
    jobs := make(chan []string, len(chunks))
    results := make(chan BatchResult, len(chunks))
    
    // ワーカープール起動
    for i := 0; i < b.workerCount; i++ {
        go b.batchWorker(ctx, i, jobs, results)
    }
    
    // ジョブ投入
    for _, chunk := range chunks {
        select {
        case jobs <- chunk:
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    close(jobs)
    
    // 結果集約
    var totalErrors []error
    successCount := 0
    
    for i := 0; i < len(chunks); i++ {
        select {
        case result := <-results:
            if result.Error != nil {
                totalErrors = append(totalErrors, result.Error)
            } else {
                successCount += result.ProcessedCount
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    
    b.metrics.SuccessfulInvalidations.Add(float64(successCount))
    b.metrics.FailedInvalidations.Add(float64(len(totalErrors)))
    
    if len(totalErrors) > 0 {
        return fmt.Errorf("batch invalidation completed with %d errors: %v", 
            len(totalErrors), totalErrors[:min(5, len(totalErrors))])
    }
    
    return nil
}

func (b *HighPerformanceBatchInvalidator) batchWorker(
    ctx context.Context, workerID int, jobs <-chan []string, results chan<- BatchResult) {
    
    client := b.clients[workerID%len(b.clients)]  // 負荷分散
    
    for chunk := range jobs {
        // レート制限適用
        if err := b.rateLimiter.Wait(ctx); err != nil {
            results <- BatchResult{Error: err}
            continue
        }
        
        // Pipeline でまとめて実行
        pipe := client.Pipeline()
        for _, key := range chunk {
            pipe.Del(ctx, key)
        }
        
        cmds, err := pipe.Exec(ctx)
        if err != nil {
            results <- BatchResult{Error: err}
            continue
        }
        
        // 個別の結果をチェック
        failedCount := 0
        for _, cmd := range cmds {
            if cmd.Err() != nil {
                failedCount++
            }
        }
        
        results <- BatchResult{
            ProcessedCount: len(chunk) - failedCount,
            FailedCount:   failedCount,
        }
    }
}

type BatchResult struct {
    ProcessedCount int
    FailedCount    int
    Error         error
}
```

### 📊 エンタープライズ運用監視とアラート設定

#### Prometheus メトリクス定義

```go
type InvalidationMetrics struct {
    InvalidationRate      *prometheus.GaugeVec
    InvalidationLatency   *prometheus.HistogramVec
    FailedInvalidations   *prometheus.CounterVec
    CascadeDepth         *prometheus.HistogramVec
    QueueSize            *prometheus.GaugeVec
}

func NewInvalidationMetrics() *InvalidationMetrics {
    return &InvalidationMetrics{
        InvalidationRate: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "cache_invalidations_per_second",
                Help: "Cache invalidations per second by type and source",
            },
            []string{"type", "source", "cluster"},
        ),
        
        InvalidationLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "cache_invalidation_duration_seconds",
                Help: "Time taken to invalidate cache entries",
                Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2, 5},
            },
            []string{"method", "result", "cluster"},
        ),
        
        FailedInvalidations: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "cache_invalidation_failures_total",
                Help: "Total number of failed cache invalidations",
            },
            []string{"error_type", "cluster", "key_pattern"},
        ),
        
        CascadeDepth: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "cache_invalidation_cascade_depth",
                Help: "Depth of cascading invalidations",
                Buckets: []float64{1, 2, 3, 5, 10, 20, 50},
            },
            []string{"trigger_type"},
        ),
    }
}
```

#### 運用アラート設定例（AlertManager）

```yaml
groups:
- name: cache-invalidation-critical
  rules:
  - alert: CacheInvalidationRateCritical
    expr: rate(cache_invalidations_per_second[5m]) > 10000
    for: 1m
    labels:
      severity: critical
      team: backend
      runbook: "https://wiki.company.com/runbooks/cache-storm"
    annotations:
      summary: "Cache invalidation rate critically high"
      description: "Invalidation rate is {{ $value }} per second (threshold: 10000)"
      impact: "Potential cache storm affecting system performance"
      
  - alert: CacheInvalidationCascadeTooDeep
    expr: histogram_quantile(0.95, cache_invalidation_cascade_depth) > 10
    for: 2m
    labels:
      severity: warning
      team: backend
    annotations:
      summary: "Cache invalidation cascade too deep"
      description: "95th percentile cascade depth is {{ $value }} (threshold: 10)"
      
  - alert: CacheInvalidationFailureRateHigh
    expr: rate(cache_invalidation_failures_total[5m]) / rate(cache_invalidations_per_second[5m]) > 0.1
    for: 3m
    labels:
      severity: warning
      team: backend
    annotations:
      summary: "High cache invalidation failure rate"
      description: "Invalidation failure rate is {{ $value | humanizePercentage }}"

- name: cache-invalidation-capacity
  rules:
  - alert: InvalidationQueueBacklog
    expr: cache_invalidation_queue_size > 100000
    for: 5m
    labels:
      severity: warning
      team: backend
    annotations:
      summary: "Cache invalidation queue backlog growing"
      description: "Queue size: {{ $value }} items"
```

#### Grafana ダッシュボード設定例

```json
{
  "dashboard": {
    "title": "Cache Invalidation Monitoring",
    "panels": [
      {
        "title": "Invalidation Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(cache_invalidations_per_second[5m])",
            "legendFormat": "{{type}} - {{source}}"
          }
        ],
        "alert": {
          "conditions": [
            {
              "query": {"queryType": "", "refId": "A"},
              "reducer": {"type": "last", "params": []},
              "evaluator": {"params": [5000], "type": "gt"}
            }
          ],
          "executionErrorState": "alerting",
          "frequency": "10s",
          "handler": 1,
          "name": "Cache Invalidation Rate Alert"
        }
      }
    ]
  }
}
```

## 🚀 発展課題

1. **階層的タグシステム**: ネストしたタグによる細かい制御
2. **無効化スケジューリング**: cron のような定期実行
3. **分散無効化**: マルチインスタンス環境での同期
4. **無効化監査**: 無効化操作の完全なログ記録
5. **予測的無効化**: アクセスパターン分析に基づく最適化
6. **地理的分散キャッシュ**: 複数リージョン間での無効化同期

キャッシュ無効化戦略の実装を通じて、大規模システムでのデータ整合性管理技術を習得しましょう！