# Day 45: Thundering Herd Problem Prevention

## 🎯 本日の目標 (Today's Goal)

Thundering Herd 問題を理解し、分散ロック、Single Flight パターン、Circuit Breaker を組み合わせた総合的な対策システムを実装できるようになる。高負荷環境でのキャッシュシステムの安定性を確保する手法を習得する。

## 📖 解説 (Explanation)

### Thundering Herd 問題とは

Thundering Herd（群発的アクセス）問題は、人気の高いキャッシュキーが期限切れになった瞬間に、大量のリクエストが同時にデータベースへアクセスしてしまう現象です。これにより、データベースが過負荷状態に陥る可能性があります。

### 問題の発生シナリオ

```
時刻 T0: 人気商品のキャッシュが期限切れ
時刻 T1: 1000個のリクエストが同時にキャッシュミス
時刻 T2: 1000個のリクエストが全てDBに殺到
時刻 T3: データベースが過負荷でタイムアウト
時刻 T4: 大量のエラーレスポンス発生
```

### Thundering Herd の影響

**システムへの影響：**
- データベースの過負荷
- レスポンス時間の悪化
- システム全体の不安定化
- カスケード障害の発生

**ビジネスへの影響：**
- ユーザー体験の悪化
- 売上機会の損失
- システムの信頼性低下

### 対策手法

#### 1. Single Flight パターン

同じキーに対する複数のリクエストを統合：

```go
import "golang.org/x/sync/singleflight"

type CacheService struct {
    sf *singleflight.Group
}

func (s *CacheService) Get(key string) (interface{}, error) {
    v, err, shared := s.sf.Do(key, func() (interface{}, error) {
        return s.loadFromDB(key)
    })
    return v, err
}
```

#### 2. 分散ロック

Redis を使用した分散ロック実装：

```go
func (s *CacheService) GetWithLock(ctx context.Context, key string) (*Data, error) {
    lockKey := "lock:" + key
    
    // ロック取得試行
    lock, err := s.acquireLock(ctx, lockKey, 10*time.Second)
    if err != nil {
        // ロック取得失敗 - 他のプロセスの完了を待機
        return s.waitAndRetry(ctx, key)
    }
    defer lock.Release()
    
    // ロック取得後、再度キャッシュ確認
    if data, err := s.getFromCache(ctx, key); err == nil {
        return data, nil
    }
    
    // データベースから取得
    return s.loadFromDB(ctx, key)
}
```

#### 3. Stale-While-Revalidate パターン

期限切れデータを一時的に返しながら、バックグラウンドで更新：

```go
func (s *CacheService) GetStaleWhileRevalidate(ctx context.Context, key string) (*Data, error) {
    data, isStale, err := s.getWithStaleness(ctx, key)
    if err == nil {
        if isStale {
            // バックグラウンドで更新を開始
            go s.refreshInBackground(key)
        }
        return data, nil
    }
    
    // キャッシュミスの場合は通常通り取得
    return s.loadFromDB(ctx, key)
}
```

#### 4. 確率的期限切れ

TTL にランダムなジッターを追加：

```go
func (s *CacheService) SetWithJitter(key string, value interface{}, baseTTL time.Duration) error {
    // ±20% のランダムなジッターを追加
    jitter := time.Duration(rand.Float64() * 0.4 - 0.2) // -20% ~ +20%
    actualTTL := baseTTL + baseTTL*jitter
    
    return s.cache.Set(key, value, actualTTL)
}
```

#### 5. Circuit Breaker パターン

データベース過負荷時のフェイルセーフ：

```go
type CircuitBreaker struct {
    state      State
    failures   int
    threshold  int
    timeout    time.Duration
    lastFailure time.Time
}

func (cb *CircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
    if cb.state == Open {
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = HalfOpen
        } else {
            return nil, ErrCircuitOpen
        }
    }
    
    result, err := fn()
    if err != nil {
        cb.recordFailure()
    } else {
        cb.recordSuccess()
    }
    
    return result, err
}
```

### 多層防御戦略

実際のプロダクション環境では、複数の対策を組み合わせます：

```go
type ThunderingHerdProtection struct {
    cache          CacheClient
    sf             *singleflight.Group
    lockManager    LockManager
    circuitBreaker *CircuitBreaker
    metrics        *ProtectionMetrics
}

func (p *ThunderingHerdProtection) Get(ctx context.Context, key string) (*Data, error) {
    // 1. 通常のキャッシュアクセス
    if data, err := p.getFromCache(ctx, key); err == nil {
        return data, nil
    }
    
    // 2. Single Flight で重複リクエストを統合
    v, err, shared := p.sf.Do(key, func() (interface{}, error) {
        return p.getWithProtection(ctx, key)
    })
    
    if shared {
        p.metrics.SharedRequests++
    }
    
    return v.(*Data), err
}

func (p *ThunderingHerdProtection) getWithProtection(ctx context.Context, key string) (*Data, error) {
    // 3. 分散ロック
    lockKey := "lock:" + key
    if lock, err := p.lockManager.TryLock(ctx, lockKey, 5*time.Second); err == nil {
        defer lock.Release()
        
        // ロック取得後、再度キャッシュ確認
        if data, err := p.getFromCache(ctx, key); err == nil {
            return data, nil
        }
        
        // 4. Circuit Breaker でDB保護
        return p.circuitBreaker.Call(func() (interface{}, error) {
            return p.loadFromDB(ctx, key)
        }).(*Data), nil
    }
    
    // 5. ロック取得失敗時の代替戦略
    return p.fallbackStrategy(ctx, key)
}
```

## 📝 課題 (The Problem)

以下の機能を持つ Thundering Herd 対策システムを実装してください：

### 1. ThunderingHerdProtector の実装

```go
type ThunderingHerdProtector struct {
    cache          CacheClient
    db             DataRepository
    sf             *singleflight.Group
    lockManager    LockManager
    circuitBreaker *CircuitBreaker
    metrics        *ProtectionMetrics
}
```

### 2. 必要なメソッドの実装

- `NewThunderingHerdProtector(...)`: プロテクターの初期化
- `Get(ctx context.Context, key string) (*Data, error)`: 保護されたデータ取得
- `Set(ctx context.Context, key string, value *Data, ttl time.Duration) error`: TTLジッター付き設定
- `GetStaleWhileRevalidate(ctx context.Context, key string) (*Data, error)`: 古いデータを返しながら更新
- `GetMetrics() ProtectionMetrics`: 保護メトリクスの取得

### 3. 分散ロック機能

Redis SETNX を使用した分散ロック実装

### 4. Circuit Breaker の統合

データベース過負荷時の自動フェイルオーバー

### 5. 統計とメトリクス

対策の効果を測定する詳細な統計情報

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestThunderingHerdProtector_SingleFlight
    main_test.go:125: 1000 concurrent requests resulted in 1 DB query
    main_test.go:132: Single flight pattern prevented thundering herd
--- PASS: TestThunderingHerdProtector_SingleFlight (0.15s)

=== RUN   TestThunderingHerdProtector_DistributedLock
    main_test.go:155: Multiple processes coordinated via distributed lock
    main_test.go:162: Only one process loaded data from DB
--- PASS: TestThunderingHerdProtector_DistributedLock (0.08s)

=== RUN   TestThunderingHerdProtector_CircuitBreaker
    main_test.go:185: Circuit breaker activated after threshold failures
    main_test.go:192: DB protected from excessive load
--- PASS: TestThunderingHerdProtector_CircuitBreaker (0.12s)

=== RUN   TestThunderingHerdProtector_StaleWhileRevalidate
    main_test.go:215: Stale data returned immediately
    main_test.go:222: Background refresh completed
--- PASS: TestThunderingHerdProtector_StaleWhileRevalidate (1.02s)

PASS
ok      day45-thundering-herd   1.456s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### パッケージのインポート

```go
import (
    "context"
    "crypto/rand"
    "fmt"
    "math/big"
    "sync/atomic"
    "time"
    
    "golang.org/x/sync/singleflight"
)
```

### 分散ロックの実装

```go
type DistributedLock struct {
    client *redis.Client
    key    string
    value  string
    ttl    time.Duration
}

func (l *DistributedLock) Acquire(ctx context.Context) error {
    // SETNX でロック取得
    result, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
    if err != nil {
        return err
    }
    if !result {
        return ErrLockNotAcquired
    }
    return nil
}

func (l *DistributedLock) Release(ctx context.Context) error {
    // Lua スクリプトで安全な解放
    script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    `
    return l.client.Eval(ctx, script, []string{l.key}, l.value).Err()
}
```

### TTL ジッターの実装

```go
func addJitter(baseTTL time.Duration, jitterPercent float64) time.Duration {
    if jitterPercent <= 0 {
        return baseTTL
    }
    
    // ±jitterPercent のランダムな値を生成
    maxJitter := int64(float64(baseTTL) * jitterPercent)
    jitter, _ := rand.Int(rand.Reader, big.NewInt(maxJitter*2))
    actualJitter := jitter.Int64() - maxJitter
    
    return baseTTL + time.Duration(actualJitter)
}
```

### Circuit Breaker の状態管理

```go
type CircuitState int

const (
    Closed CircuitState = iota
    Open
    HalfOpen
)

func (cb *CircuitBreaker) recordFailure() {
    cb.failures++
    cb.lastFailure = time.Now()
    
    if cb.failures >= cb.threshold {
        cb.state = Open
    }
}

func (cb *CircuitBreaker) recordSuccess() {
    cb.failures = 0
    cb.state = Closed
}
```

### Stale-While-Revalidate の実装

```go
func (p *ThunderingHerdProtector) getWithStaleness(ctx context.Context, key string) (*Data, bool, error) {
    // Redis で TTL と値を同時に取得
    pipe := p.cache.Pipeline()
    ttlCmd := pipe.TTL(ctx, key)
    getCmd := pipe.Get(ctx, key)
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return nil, false, err
    }
    
    ttl := ttlCmd.Val()
    value := getCmd.Val()
    
    // TTL が 0 以下の場合は期限切れ
    isStale := ttl <= 0
    
    var data Data
    err = json.Unmarshal([]byte(value), &data)
    return &data, isStale, err
}
```

### メトリクスの実装

```go
type ProtectionMetrics struct {
    TotalRequests      int64
    CacheHits         int64
    CacheMisses       int64
    SingleFlightHits  int64
    LockAcquisitions  int64
    CircuitBreakerTrips int64
    StaleReturns      int64
    BackgroundRefresh int64
}

func (p *ThunderingHerdProtector) recordMetric(metric *int64) {
    atomic.AddInt64(metric, 1)
}
```

## 🚨 Thundering Herd の実際の災害事例と対策

### 世界規模での実際の障害事例

#### ❌ 災害事例1: 大手ECサイトのブラックフライデー大規模障害

**発生詳細:**
- **日時:** 2023年11月24日 00:00:00 JST（ブラックフライデー開始）
- **サイト:** 月間PV 5億の大手ECサイト
- **事象:** 目玉商品のキャッシュ期限切れと同時に50万リクエストが殺到
- **継続時間:** 45分間のサービス全停止
- **影響範囲:** システム全体のダウン、すべてのユーザーアクセス不可

**技術的な詳細:**
```go
// ❌ 障害時のコード例 - Single Flight も効果なし
type NaiveProductService struct {
    cache *redis.Client
    db    *sql.DB
    sf    *singleflight.Group  // これだけでは不十分
}

func (s *NaiveProductService) GetPopularProduct(ctx context.Context, id string) (*Product, error) {
    // キャッシュチェック
    if product, err := s.getFromCache(ctx, id); err == nil {
        return product, nil
    }
    
    // Single Flight パターン - しかし限界がある
    v, err, shared := s.sf.Do(id, func() (interface{}, error) {
        // 50万リクエスト中49万9999個が此処で待機
        // 1つのDB接続で処理しようとして30秒でタイムアウト
        return s.loadFromDB(ctx, id)  // ここで障害発生
    })
    
    if err != nil {
        // エラー時の代替手段なし - 全リクエストが失敗
        return nil, err
    }
    
    return v.(*Product), nil
}
```

**システム障害の連鎖:**
1. **T+0秒:** 人気商品（iPhone最新モデル）のキャッシュが期限切れ
2. **T+1秒:** 50万リクエストが同時にキャッシュミス
3. **T+5秒:** Single Flight の待機キューが膨大になり、メモリ使用量が急増
4. **T+10秒:** データベース接続プールが枯渇（最大100接続）
5. **T+15秒:** データベースサーバーのCPU使用率100%達成
6. **T+30秒:** すべてのDBクエリがタイムアウト
7. **T+45秒:** アプリケーションサーバーがOOMエラーでクラッシュ

**ビジネス損失:**
- **直接的損失:** 売上機会 3億2000万円
- **間接的損失:** ブランド信頼度低下、カスタマーサポートコスト
- **復旧コスト:** エンジニア緊急対応費用、インフラ増強費用
- **SLA違反:** 大口契約先への違約金支払い

✅ **エンタープライズレベルの多重防御システム:**

```go
type EnterpriseThunderingHerdProtector struct {
    // 多層キャッシュ
    l1Cache         *freecache.Cache      // メモリキャッシュ
    l2Cache         *redis.ClusterClient  // 分散キャッシュ
    l3Cache         *memcached.Client     // バックアップキャッシュ
    
    // 負荷分散とフェイルオーバー
    dbLoadBalancer  *DBLoadBalancer       // DB負荷分散
    circuitBreaker  *CircuitBreaker       // DB保護
    sf              *singleflight.Group   // 重複排除
    
    // 運用監視
    metrics         *ComprehensiveMetrics // 詳細メトリクス
    alertManager    *AlertManager         // リアルタイムアラート
    
    // 予測・適応システム
    predictor       *LoadPredictor        // 負荷予測
    adaptiveConfig  *AdaptiveConfig       // 動的設定調整
}

func (e *EnterpriseThunderingHerdProtector) GetWithFullProtection(
    ctx context.Context, key string) (*Data, error) {
    
    start := time.Now()
    e.metrics.TotalRequests.Inc()
    
    defer func() {
        e.metrics.RequestDuration.Observe(time.Since(start).Seconds())
    }()
    
    // Phase 1: 多層キャッシュチェック
    if data, err := e.getFromL1Cache(key); err == nil {
        e.metrics.L1CacheHits.Inc()
        return data, nil
    }
    
    if data, err := e.getFromL2Cache(ctx, key); err == nil {
        e.metrics.L2CacheHits.Inc()
        // 非同期でL1に昇格
        go e.promoteToL1(key, data)
        return data, nil
    }
    
    if data, err := e.getFromL3Cache(ctx, key); err == nil {
        e.metrics.L3CacheHits.Inc()
        // 非同期でL1, L2に昇格
        go e.promoteToUpperLayers(key, data)
        return data, nil
    }
    
    // Phase 2: 負荷予測による動的制御
    if e.predictor.IsHighLoadPredicted(key) {
        // 高負荷予測時は古いデータでも返す
        if staleData, err := e.getStaleData(ctx, key); err == nil {
            e.metrics.StaleDataReturned.Inc()
            go e.refreshInBackground(key)  // バックグラウンド更新
            return staleData, nil
        }
    }
    
    // Phase 3: Single Flight + Circuit Breaker
    v, err, shared := e.sf.Do(key, func() (interface{}, error) {
        return e.loadWithCircuitBreaker(ctx, key)
    })
    
    if shared {
        e.metrics.SharedRequests.Inc()
    }
    
    if err != nil {
        // Phase 4: 最終フォールバック
        return e.handleFinalFallback(ctx, key, err)
    }
    
    data := v.(*Data)
    
    // 成功時は全層に保存
    go e.saveToAllLayers(key, data)
    
    return data, nil
}

func (e *EnterpriseThunderingHerdProtector) loadWithCircuitBreaker(
    ctx context.Context, key string) (*Data, error) {
    
    // Circuit Breakerで DB保護
    result, err := e.circuitBreaker.Execute(func() (interface{}, error) {
        
        // 負荷分散でDB選択
        db := e.dbLoadBalancer.SelectOptimalDB()
        
        // タイムアウト付きでDB アクセス
        dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()
        
        data, err := db.Get(dbCtx, key)
        if err != nil {
            e.metrics.DBErrors.Inc()
            
            // 即座にアラート
            e.alertManager.SendImmediateAlert(
                AlertLevel.Critical,
                fmt.Sprintf("DB load failed for key: %s, error: %v", key, err),
            )
        }
        
        return data, err
    })
    
    if err != nil {
        return nil, err
    }
    
    return result.(*Data), nil
}

func (e *EnterpriseThunderingHerdProtector) handleFinalFallback(
    ctx context.Context, key string, originalErr error) (*Data, error) {
    
    e.metrics.FallbackActivated.Inc()
    
    // 1. デフォルトデータを返す
    if defaultData, err := e.getDefaultData(key); err == nil {
        e.alertManager.SendAlert(
            AlertLevel.Warning,
            fmt.Sprintf("Returned default data for key: %s due to: %v", key, originalErr),
        )
        return defaultData, nil
    }
    
    // 2. より古いキャッシュデータを探す
    if ancientData, err := e.getAncientCache(ctx, key); err == nil {
        e.alertManager.SendAlert(
            AlertLevel.Warning,
            fmt.Sprintf("Returned ancient cache for key: %s due to: %v", key, originalErr),
        )
        return ancientData, nil
    }
    
    // 3. 最終的に失敗
    e.metrics.TotalFailures.Inc()
    e.alertManager.SendAlert(
        AlertLevel.Critical,
        fmt.Sprintf("Complete failure for key: %s, error: %v", key, originalErr),
    )
    
    return nil, fmt.Errorf("all fallback mechanisms failed: %w", originalErr)
}
```

#### ❌ 災害事例2: ソーシャルメディアのバイラル投稿大量アクセス障害

**発生詳細:**
- **プラットフォーム:** 月間アクティブユーザー2億人のSNS
- **きっかけ:** 著名人の投稿が瞬時に100万シェア
- **問題:** 投稿データのキャッシュ期限切れで500万リクエストが集中
- **継続時間:** 15分間のアプリ応答不能
- **影響:** 全ユーザーのタイムライン更新停止

**障害の技術的分析:**
```go
// ❌ バイラル投稿の処理で問題となったコード
type SocialMediaService struct {
    cache *redis.Client
    db    *mongodb.Client
    sf    *singleflight.Group
}

func (s *SocialMediaService) GetViralPost(ctx context.Context, postID string) (*Post, error) {
    // 通常の Single Flight - バイラル投稿には効果不十分
    v, err, shared := s.sf.Do(postID, func() (interface{}, error) {
        // 500万リクエストが1つのDB接続を待機
        // MongoDB 接続が30秒でタイムアウト
        return s.loadPostFromDB(ctx, postID)
    })
    
    // エラー時の代替戦略なし
    if err != nil {
        return nil, err  // 全リクエストが失敗
    }
    
    return v.(*Post), nil
}
```

**システム破綻の流れ:**
1. **T+0:** セレブの投稿が投稿される
2. **T+10:** 投稿が急速に拡散開始（10万シェア/分）
3. **T+300:** キャッシュTTL（5分）が期限切れ
4. **T+305:** 500万の同時リクエストがキャッシュミス
5. **T+310:** Single Flight キューが巨大化（50GB メモリ使用）
6. **T+320:** MongoDB接続プールが枯渇
7. **T+330:** データベースクラスターがダウン
8. **T+900:** 手動復旧まで15分間停止

✅ **バイラル対応特化システム:**

```go
type ViralContentProtectionSystem struct {
    // 多段階キャッシュ
    fastCache       *fastcache.Cache      // 超高速キャッシュ
    redisCluster    *redis.ClusterClient  // 分散Redis
    cdnCache        *CDNClient            // CDN統合
    
    // バイラル検知・予測
    viralDetector   *ViralDetector        // リアルタイム検知
    trendPredictor  *TrendPredictor       // トレンド予測
    
    // 負荷制御
    sf              *singleflight.Group
    rateLimiter     *DistributedRateLimiter
    loadShedder     *LoadShedder          // 負荷シェディング
    
    // 運用・監視
    metrics         *ViralMetrics
    alertSystem     *RealtimeAlertSystem
}

func (v *ViralContentProtectionSystem) GetPostWithViralProtection(
    ctx context.Context, postID string) (*Post, error) {
    
    // Phase 1: バイラル検知
    if v.viralDetector.IsCurrentlyViral(postID) {
        return v.handleViralContent(ctx, postID)
    }
    
    // Phase 2: 通常のプロテクション
    return v.handleNormalContent(ctx, postID)
}

func (v *ViralContentProtectionSystem) handleViralContent(
    ctx context.Context, postID string) (*Post, error) {
    
    v.metrics.ViralRequestsTotal.Inc()
    
    // 1. 超高速キャッシュから取得試行
    if post, err := v.getFromFastCache(postID); err == nil {
        v.metrics.FastCacheHits.Inc()
        return post, nil
    }
    
    // 2. CDN キャッシュ統合
    if post, err := v.getFromCDN(ctx, postID); err == nil {
        v.metrics.CDNCacheHits.Inc()
        // 非同期で高速キャッシュに保存
        go v.saveToFastCache(postID, post)
        return post, nil
    }
    
    // 3. 負荷シェディング判定
    if v.loadShedder.ShouldShed(ctx) {
        v.metrics.RequestsShed.Inc()
        return v.getStaleOrDefault(ctx, postID)
    }
    
    // 4. レート制限付きSingle Flight
    if !v.rateLimiter.Allow(ctx, "viral_db_access") {
        v.metrics.RateLimited.Inc()
        return v.getStaleOrDefault(ctx, postID)
    }
    
    // 5. Single Flight で DB アクセス
    v, err, shared := v.sf.Do(postID, func() (interface{}, error) {
        return v.loadWithViralOptimization(ctx, postID)
    })
    
    if shared {
        v.metrics.SharedViralRequests.Inc()
    }
    
    if err != nil {
        return v.handleViralError(ctx, postID, err)
    }
    
    post := v.(*Post)
    
    // 全層に保存 + CDN 配信
    go v.distributeToAllLayers(postID, post)
    
    return post, nil
}

func (v *ViralContentProtectionSystem) loadWithViralOptimization(
    ctx context.Context, postID string) (*Post, error) {
    
    // バイラル投稿専用の読み取り専用DBレプリカ使用
    db := v.dbManager.GetReadOnlyReplica()
    
    // タイムアウトを短く設定
    dbCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    
    post, err := db.GetPost(dbCtx, postID)
    if err != nil {
        v.alertSystem.SendUrgentAlert(
            "Viral post DB load failed",
            map[string]interface{}{
                "post_id": postID,
                "error":   err.Error(),
                "load":    v.getCurrentLoad(),
            },
        )
        return nil, err
    }
    
    return post, nil
}

func (v *ViralContentProtectionSystem) handleViralError(
    ctx context.Context, postID string, err error) (*Post, error) {
    
    v.metrics.ViralErrorsTotal.Inc()
    
    // 1. 期限切れでも古いデータを返す
    if stalePost, serr := v.getExpiredCache(ctx, postID); serr == nil {
        v.metrics.StaleViralDataReturned.Inc()
        
        // バックグラウンドで更新試行
        go func() {
            time.Sleep(time.Duration(rand.Intn(30)) * time.Second)  // ジッター
            v.refreshInBackground(postID)
        }()
        
        return stalePost, nil
    }
    
    // 2. デフォルトの「読み込み中」投稿を返す
    if defaultPost := v.getLoadingPlaceholder(postID); defaultPost != nil {
        v.metrics.PlaceholderReturned.Inc()
        
        v.alertSystem.SendUrgentAlert(
            "Viral post fallback to placeholder",
            map[string]interface{}{
                "post_id": postID,
                "error":   err.Error(),
            },
        )
        
        return defaultPost, nil
    }
    
    // 3. Complete failure
    v.metrics.CompleteViralFailures.Inc()
    return nil, fmt.Errorf("viral content completely unavailable: %w", err)
}

// バイラル検知システム
type ViralDetector struct {
    thresholds    *ViralThresholds
    window        time.Duration
    metricsStore  *MetricsStore
}

type ViralThresholds struct {
    RequestsPerSecond int64         // 秒間リクエスト数
    GrowthRate       float64       // 増加率
    ShareVelocity    int64         // シェア速度
}

func (vd *ViralDetector) IsCurrentlyViral(postID string) bool {
    metrics := vd.metricsStore.GetRecentMetrics(postID, vd.window)
    
    // 多次元でバイラル判定
    return metrics.RequestsPerSecond > vd.thresholds.RequestsPerSecond ||
           metrics.GrowthRate > vd.thresholds.GrowthRate ||
           metrics.ShareVelocity > vd.thresholds.ShareVelocity
}
```

### 📊 エンタープライズレベルの運用監視システム

#### リアルタイム監視ダッシュボード

```go
type ThunderingHerdMetrics struct {
    // リクエスト統計
    TotalRequests           *prometheus.CounterVec
    SharedRequests          *prometheus.CounterVec
    SingleFlightWaitTime    *prometheus.HistogramVec
    
    // キャッシュ統計
    CacheHitRate           *prometheus.GaugeVec
    CacheMissRate          *prometheus.GaugeVec
    CacheLatency           *prometheus.HistogramVec
    
    // DB保護統計
    CircuitBreakerState    *prometheus.GaugeVec
    DBConnectionPoolUsage  *prometheus.GaugeVec
    DBQueryDuration        *prometheus.HistogramVec
    
    // パフォーマンス統計
    ResponseTime           *prometheus.HistogramVec
    ThroughputPerSecond    *prometheus.GaugeVec
    ErrorRate              *prometheus.GaugeVec
    
    // 予測・アラート
    LoadPrediction         *prometheus.GaugeVec
    ViralContentDetected   *prometheus.CounterVec
    AutoScalingTriggered   *prometheus.CounterVec
}

func NewThunderingHerdMetrics() *ThunderingHerdMetrics {
    return &ThunderingHerdMetrics{
        TotalRequests: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "thundering_herd_requests_total",
                Help: "Total number of requests handled by thundering herd protector",
            },
            []string{"key_pattern", "cache_layer", "result"},
        ),
        
        SingleFlightWaitTime: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "single_flight_wait_duration_seconds",
                Help: "Time spent waiting in single flight queue",
                Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1, 2, 5, 10, 30},
            },
            []string{"key_pattern"},
        ),
        
        CircuitBreakerState: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "circuit_breaker_state",
                Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
            },
            []string{"service", "endpoint"},
        ),
        
        LoadPrediction: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "thundering_herd_load_prediction",
                Help: "Predicted load level for next 5 minutes",
            },
            []string{"key_pattern", "prediction_model"},
        ),
    }
}
```

#### アラート設定（Prometheus AlertManager）

```yaml
groups:
- name: thundering-herd-critical
  interval: 15s
  rules:
  - alert: ThunderingHerdDetected
    expr: rate(thundering_herd_requests_total[1m]) > 10000
    for: 30s
    labels:
      severity: critical
      team: platform
      escalation: immediate
      runbook: "https://wiki.company.com/runbooks/thundering-herd"
    annotations:
      summary: "Thundering Herd attack detected"
      description: "{{ $labels.key_pattern }} receiving {{ $value }} requests/sec"
      impact: "Database may be overwhelmed, potential service outage"
      action: "Engage emergency response team immediately"

  - alert: SingleFlightQueueOverload
    expr: histogram_quantile(0.95, single_flight_wait_duration_seconds) > 30
    for: 1m
    labels:
      severity: critical
      team: platform
    annotations:
      summary: "Single Flight queue severely overloaded"
      description: "95th percentile wait time: {{ $value }}s"
      
  - alert: CircuitBreakerOpen
    expr: circuit_breaker_state == 1
    for: 0s  # Immediate alert
    labels:
      severity: warning
      team: platform
    annotations:
      summary: "Circuit breaker opened for {{ $labels.service }}"
      description: "DB protection activated for {{ $labels.endpoint }}"

  - alert: CacheHitRateCriticallyLow
    expr: cache_hit_rate < 0.5
    for: 2m
    labels:
      severity: warning
      team: platform
    annotations:
      summary: "Cache hit rate critically low"
      description: "Hit rate: {{ $value | humanizePercentage }}"

  - alert: PredictedThunderingHerd
    expr: thundering_herd_load_prediction > 0.8
    for: 1m
    labels:
      severity: warning
      team: platform
    annotations:
      summary: "High probability of incoming thundering herd"
      description: "Prediction confidence: {{ $value | humanizePercentage }}"
      action: "Consider preemptive scaling and cache warming"
```

#### Grafana ダッシュボード例

```json
{
  "dashboard": {
    "title": "Thundering Herd Protection Dashboard",
    "tags": ["thundering-herd", "cache", "performance"],
    "time": {"from": "now-1h", "to": "now"},
    "refresh": "5s",
    "panels": [
      {
        "title": "Request Rate by Pattern",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(thundering_herd_requests_total[5m])",
            "legendFormat": "{{ key_pattern }} - {{ result }}"
          }
        ],
        "yAxes": [{"label": "Requests/sec"}],
        "alert": {
          "name": "High Request Rate",
          "frequency": "10s",
          "conditions": [
            {
              "query": {"refId": "A"},
              "reducer": {"type": "avg"},
              "evaluator": {"params": [5000], "type": "gt"}
            }
          ]
        }
      },
      {
        "title": "Cache Hit Rate",
        "type": "singlestat",
        "targets": [
          {
            "expr": "rate(cache_hits_total[5m]) / rate(cache_requests_total[5m]) * 100",
            "legendFormat": "Hit Rate %"
          }
        ],
        "valueName": "current",
        "format": "percent",
        "thresholds": "70,90"
      },
      {
        "title": "Single Flight Queue Depth",
        "type": "graph",
        "targets": [
          {
            "expr": "single_flight_queue_depth",
            "legendFormat": "{{ key_pattern }}"
          }
        ]
      },
      {
        "title": "Circuit Breaker States",
        "type": "table",
        "targets": [
          {
            "expr": "circuit_breaker_state",
            "format": "table",
            "instant": true
          }
        ],
        "columns": [
          {"text": "Service", "value": "service"},
          {"text": "Endpoint", "value": "endpoint"},
          {"text": "State", "value": "Value"}
        ]
      }
    ]
  }
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **適応的TTL**: アクセス頻度に基づく動的TTL調整
2. **階層的キャッシュ**: L1/L2キャッシュでの段階的保護
3. **プリディクティブキャッシング**: アクセスパターン予測に基づく事前ロード
4. **レート制限**: 個別クライアントのリクエスト制限
5. **分散協調**: 複数のRedisインスタンス間での協調制御
6. **AI駆動予測**: 機械学習によるバイラル投稿の事前検知
7. **地理的分散**: グローバルCDNとの連携によるレイテンシ削減
8. **カオスエンジニアリング**: 計画的障害によるシステム堅牢性テスト

Thundering Herd 対策の実装を通じて、高負荷環境でのシステム設計の重要な側面を学びましょう！