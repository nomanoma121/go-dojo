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

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **適応的TTL**: アクセス頻度に基づく動的TTL調整
2. **階層的キャッシュ**: L1/L2キャッシュでの段階的保護
3. **プリディクティブキャッシング**: アクセスパターン予測に基づく事前ロード
4. **レート制限**: 個別クライアントのリクエスト制限
5. **分散協調**: 複数のRedisインスタンス間での協調制御

Thundering Herd 対策の実装を通じて、高負荷環境でのシステム設計の重要な側面を学びましょう！