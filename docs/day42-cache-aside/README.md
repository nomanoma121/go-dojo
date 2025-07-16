# Day 42: Cache-Aside パターン

## 🎯 本日の目標 (Today's Goal)

Cache-Aside（Lazy Loading）パターンを実装し、データベース負荷を軽減しながらデータの整合性を保つキャッシュシステムを構築できるようになる。キャッシュミス時の処理フローと、並行アクセス時の競合状態に対する対策を理解する。

## 📖 解説 (Explanation)

### Cache-Aside パターンとは

Cache-Aside（別名：Lazy Loading、Cache-on-Demand）は、最も一般的なキャッシュパターンです。アプリケーション側でキャッシュの読み書きを制御し、必要に応じてデータベースとの同期を行います。

### Cache-Aside の動作フロー

#### 読み取り処理（Read）

```
1. キャッシュからデータを読み取り試行
2. キャッシュヒット → データを返す
3. キャッシュミス → データベースからデータを取得
4. 取得したデータをキャッシュに保存
5. データを返す
```

#### 書き込み処理（Write）

```
1. データベースにデータを書き込み
2. キャッシュから該当データを削除（または更新）
```

### Cache-Aside の特徴

**利点：**
- アプリケーションが完全にキャッシュを制御
- キャッシュ障害時もシステムが動作継続
- 必要なデータのみキャッシュされる（Lazy Loading）
- 実装がシンプル

**欠点：**
- キャッシュミス時のレイテンシが大きい
- データの整合性管理が複雑
- 同じデータの重複ロードが発生する可能性

### 実装例

```go
// 【Cache-Aside基本実装】最も一般的なキャッシングパターン
func (s *UserService) GetUser(ctx context.Context, userID int) (*User, error) {
    start := time.Now()
    defer func() {
        s.recordResponseTime(time.Since(start))
    }()
    
    // 【STEP 1】キャッシュからの取得試行
    cacheKey := fmt.Sprintf("user:%d", userID)
    var user User
    err := s.cache.GetJSON(ctx, cacheKey, &user)
    if err == nil {
        // 【キャッシュヒット】高速レスポンス（通常1-3ms）
        s.recordCacheHit()
        log.Printf("⚡ CACHE HIT: User %d retrieved from cache in %v", userID, time.Since(start))
        return &user, nil
    }
    
    // 【STEP 2】キャッシュミス処理
    if err != ErrCacheMiss {
        // Redis接続エラーなどの異常系
        log.Printf("⚠️  Cache error for user %d: %v", userID, err)
        s.recordCacheError()
    }
    
    s.recordCacheMiss()
    log.Printf("💾 CACHE MISS: User %d - fetching from database", userID)
    
    // 【STEP 3】データベースからの取得
    user, err = s.db.GetUser(ctx, userID)
    if err != nil {
        s.recordDatabaseError()
        return nil, fmt.Errorf("failed to get user from database: %w", err)
    }
    
    // 【STEP 4】キャッシュへの保存（非同期で性能向上）
    go func() {
        cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := s.cache.SetJSON(cacheCtx, cacheKey, user, 1*time.Hour); err != nil {
            log.Printf("❌ Failed to cache user %d: %v", userID, err)
            s.recordCacheWriteError()
        } else {
            log.Printf("💾 Successfully cached user %d with 1h TTL", userID)
        }
    }()
    
    log.Printf("💾 DATABASE FETCH: User %d completed in %v", userID, time.Since(start))
    return &user, nil
}

// 【重要メソッド】メトリクス記録による性能監視
func (s *UserService) recordCacheHit() {
    atomic.AddInt64(&s.metrics.CacheHits, 1)
    atomic.AddInt64(&s.metrics.TotalRequests, 1)
}

func (s *UserService) recordCacheMiss() {
    atomic.AddInt64(&s.metrics.CacheMisses, 1)
    atomic.AddInt64(&s.metrics.TotalRequests, 1)
}

func (s *UserService) recordResponseTime(duration time.Duration) {
    s.metrics.mu.Lock()
    defer s.metrics.mu.Unlock()
    
    // 指数移動平均でレスポンス時間を追跡
    if s.metrics.AvgResponseTime == 0 {
        s.metrics.AvgResponseTime = duration
    } else {
        s.metrics.AvgResponseTime = time.Duration(
            float64(s.metrics.AvgResponseTime)*0.9 + float64(duration)*0.1,
        )
    }
}

// 【Cache-Aside特有の課題】データ整合性の確保
func (s *UserService) UpdateUser(ctx context.Context, user *User) error {
    // 【STEP 1】データベース更新を最優先で実行
    err := s.db.UpdateUser(ctx, user)
    if err != nil {
        return fmt.Errorf("failed to update user in database: %w", err)
    }
    
    // 【STEP 2】キャッシュ無効化で整合性を保証
    cacheKey := fmt.Sprintf("user:%d", user.ID)
    
    // 【重要】キャッシュ削除の失敗はサービス継続に影響しない
    if err := s.cache.Delete(ctx, cacheKey); err != nil {
        log.Printf("⚠️  Failed to invalidate cache for user %d: %v", user.ID, err)
        s.recordCacheInvalidationError()
        // キャッシュ削除失敗でもエラーは返さない（サービス継続性重視）
    } else {
        log.Printf("🗑️  Successfully invalidated cache for user %d", user.ID)
    }
    
    // 【STEP 3】関連キャッシュの無効化（必要に応じて）
    relatedKeys := []string{
        "users:all",                              // 全ユーザーリスト
        fmt.Sprintf("user_posts:%d", user.ID),    // ユーザー投稿
        fmt.Sprintf("user_stats:%d", user.ID),    // ユーザー統計
    }
    
    for _, key := range relatedKeys {
        if err := s.cache.Delete(ctx, key); err != nil {
            log.Printf("⚠️  Failed to invalidate related cache %s: %v", key, err)
        }
    }
    
    return nil
}
```

### 競合状態（Race Condition）の問題

複数のリクエストが同時に同じデータにアクセスした場合：

```
時刻 T1: Request A がキャッシュミス → DB アクセス開始
時刻 T2: Request B がキャッシュミス → DB アクセス開始  
時刻 T3: Request A が DB から取得完了 → キャッシュに保存
時刻 T4: Request B が DB から取得完了 → キャッシュに保存
```

この場合、同じデータに対して複数回のDBアクセスが発生します。

### 競合状態の対策

#### 1. Single Flight パターン

同じキーに対する複数のリクエストを統合：

```go
import "golang.org/x/sync/singleflight"

// 【Single Flight実装】同一データへの並行アクセス最適化
func (s *UserService) GetUser(ctx context.Context, userID int) (*User, error) {
    start := time.Now()
    cacheKey := fmt.Sprintf("user:%d", userID)
    
    // 【STEP 1】キャッシュからの高速取得試行
    var user User
    err := s.cache.GetJSON(ctx, cacheKey, &user)
    if err == nil {
        s.recordCacheHit()
        return &user, nil
    }
    
    // 【STEP 2】Single Flight によるデータベースアクセス統合
    // 同じuserIDに対する複数のリクエストを1つのDB操作にまとめる
    v, err, shared := s.sf.Do(cacheKey, func() (interface{}, error) {
        // 【重要】この関数は同じキーに対して一度だけ実行される
        log.Printf("🔄 Single Flight: Loading user %d from database", userID)
        
        // データベースから取得
        user, err := s.db.GetUser(ctx, userID)
        if err != nil {
            return nil, fmt.Errorf("failed to load user from DB: %w", err)
        }
        
        // 【キャッシュ保存】全ての待機中リクエストが利用できるように
        go func() {
            cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            
            if err := s.cache.SetJSON(cacheCtx, cacheKey, user, 1*time.Hour); err != nil {
                log.Printf("❌ Failed to cache user %d: %v", userID, err)
            } else {
                log.Printf("💾 User %d cached via Single Flight", userID)
            }
        }()
        
        return user, nil
    })
    
    if err != nil {
        s.recordDatabaseError()
        return nil, err
    }
    
    // 【メトリクス記録】Single Flight効果の測定
    if shared {
        s.recordSharedLoad()
        log.Printf("🤝 SHARED LOAD: User %d loaded via Single Flight (saved DB query)", userID)
    } else {
        s.recordCacheMiss()
        log.Printf("💾 CACHE MISS: User %d loaded from DB in %v", userID, time.Since(start))
    }
    
    return v.(*User), nil
}

// 【効果測定】Single Flight の恩恵を可視化
func (s *UserService) recordSharedLoad() {
    atomic.AddInt64(&s.metrics.SharedLoads, 1)
    atomic.AddInt64(&s.metrics.TotalRequests, 1)
    // SharedLoads が多いほど、重複クエリの削減効果が高い
}

// 【Single Flight応用】バッチ処理での活用例
func (s *UserService) GetUsersBatch(ctx context.Context, userIDs []int) ([]*User, error) {
    // 【並行処理】各ユーザーを独立してSingle Flight で取得
    type result struct {
        user *User
        err  error
        idx  int
    }
    
    results := make(chan result, len(userIDs))
    
    for i, userID := range userIDs {
        go func(idx, id int) {
            user, err := s.GetUser(ctx, id) // Single Flight が自動適用
            results <- result{user: user, err: err, idx: idx}
        }(i, userID)
    }
    
    // 結果を収集
    users := make([]*User, len(userIDs))
    var errors []error
    
    for i := 0; i < len(userIDs); i++ {
        res := <-results
        if res.err != nil {
            errors = append(errors, res.err)
        } else {
            users[res.idx] = res.user
        }
    }
    
    if len(errors) > 0 {
        return nil, fmt.Errorf("batch load failed: %d errors occurred", len(errors))
    }
    
    return users, nil
}
```

#### 2. 分散ロック

Redis を使用した分散ロック：

```go
// 【分散ロック実装】複数インスタンス間での排他制御
func (s *UserService) GetUserWithDistributedLock(ctx context.Context, userID int) (*User, error) {
    cacheKey := fmt.Sprintf("user:%d", userID)
    lockKey := fmt.Sprintf("lock:user:%d", userID)
    
    // 【STEP 1】キャッシュからの高速取得（ロック不要）
    var user User
    err := s.cache.GetJSON(ctx, cacheKey, &user)
    if err == nil {
        s.recordCacheHit()
        return &user, nil
    }
    
    // 【STEP 2】分散ロック取得でDB重複アクセスを防止
    lock, err := s.acquireLock(ctx, lockKey, 10*time.Second)
    if err != nil {
        // ロック取得失敗時のフォールバック戦略
        if err == ErrLockTimeout {
            log.Printf("⏰ Lock timeout for user %d, using fallback", userID)
            // タイムアウト時は直接DBアクセス（性能より整合性重視）
            return s.loadUserFromDB(ctx, userID)
        }
        return nil, fmt.Errorf("failed to acquire lock: %w", err)
    }
    defer func() {
        if releaseErr := lock.Release(); releaseErr != nil {
            log.Printf("⚠️  Failed to release lock for user %d: %v", userID, releaseErr)
        }
    }()
    
    // 【STEP 3】ロック取得後の二重チェック（重要）
    // 他のプロセスが既にキャッシュに保存している可能性
    err = s.cache.GetJSON(ctx, cacheKey, &user)
    if err == nil {
        s.recordCacheHit()
        log.Printf("🔒 Double-check cache hit for user %d", userID)
        return &user, nil
    }
    
    // 【STEP 4】データベースから取得（ロック保護下）
    s.recordCacheMiss()
    log.Printf("🔒 Protected DB access for user %d", userID)
    
    user, err = s.db.GetUser(ctx, userID)
    if err != nil {
        s.recordDatabaseError()
        return nil, fmt.Errorf("failed to get user from database: %w", err)
    }
    
    // 【STEP 5】キャッシュに保存（同じロック内で実行）
    if err := s.cache.SetJSON(ctx, cacheKey, user, 1*time.Hour); err != nil {
        log.Printf("❌ Failed to cache user %d: %v", userID, err)
        s.recordCacheWriteError()
        // キャッシュ失敗でもDBから取得したデータは返す
    } else {
        log.Printf("🔒 Successfully cached user %d with distributed lock", userID)
    }
    
    return &user, nil
}

// 【分散ロック実装】Redis SET NX PX コマンドを利用
type DistributedLock struct {
    redis    *redis.Client
    key      string
    value    string
    ttl      time.Duration
    released bool
    mu       sync.Mutex
}

func (s *UserService) acquireLock(ctx context.Context, key string, ttl time.Duration) (*DistributedLock, error) {
    // 【ユニーク値生成】ロックの所有権識別用
    lockValue := fmt.Sprintf("%s:%d", uuid.New().String(), time.Now().UnixNano())
    
    // 【Redis SET NX PX】原子的なロック取得
    result, err := s.cache.SetNX(ctx, key, lockValue, ttl).Result()
    if err != nil {
        return nil, fmt.Errorf("failed to acquire distributed lock: %w", err)
    }
    
    if !result {
        return nil, ErrLockTimeout
    }
    
    lock := &DistributedLock{
        redis: s.cache,
        key:   key,
        value: lockValue,
        ttl:   ttl,
    }
    
    // 【自動延長】長時間処理対応（必要に応じて）
    go lock.startAutoExtension(ctx)
    
    return lock, nil
}

func (dl *DistributedLock) Release() error {
    dl.mu.Lock()
    defer dl.mu.Unlock()
    
    if dl.released {
        return nil
    }
    
    // 【Luaスクリプト】原子的なロック解放
    // 自分が取得したロックのみ解放（他のプロセスのロックを誤解放防止）
    script := `
        if redis.call("GET", KEYS[1]) == ARGV[1] then
            return redis.call("DEL", KEYS[1])
        else
            return 0
        end
    `
    
    result, err := dl.redis.Eval(context.Background(), script, []string{dl.key}, dl.value).Result()
    if err != nil {
        return fmt.Errorf("failed to release lock: %w", err)
    }
    
    if result.(int64) == 0 {
        return fmt.Errorf("lock was not owned by this process")
    }
    
    dl.released = true
    return nil
}

func (dl *DistributedLock) startAutoExtension(ctx context.Context) {
    ticker := time.NewTicker(dl.ttl / 3) // TTLの1/3間隔で延長
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            dl.mu.Lock()
            if dl.released {
                dl.mu.Unlock()
                return
            }
            
            // ロック延長
            script := `
                if redis.call("GET", KEYS[1]) == ARGV[1] then
                    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
                else
                    return 0
                end
            `
            
            _, err := dl.redis.Eval(ctx, script, []string{dl.key}, dl.value, int64(dl.ttl/time.Millisecond)).Result()
            if err != nil {
                log.Printf("⚠️  Failed to extend lock %s: %v", dl.key, err)
            }
            dl.mu.Unlock()
            
        case <-ctx.Done():
            return
        }
    }
}

// 【エラー定義】
var (
    ErrLockTimeout = errors.New("lock acquisition timeout")
)
```

### TTL 戦略

適切なTTL設定により、データの新鮮性とパフォーマンスのバランスを取ります：

```go
const (
    UserCacheTTL     = 1 * time.Hour    // ユーザー情報
    ProductCacheTTL  = 30 * time.Minute // 商品情報
    SessionCacheTTL  = 15 * time.Minute // セッション情報
)
```

### メトリクス監視

キャッシュの効果を測定するためのメトリクス：

```go
type CacheMetrics struct {
    HitRate    float64 // ヒット率
    MissRate   float64 // ミス率
    LoadTime   time.Duration // データベース読み込み時間
    CacheSize  int64   // キャッシュサイズ
}
```

## 📝 課題 (The Problem)

以下の機能を持つ Cache-Aside パターンを実装してください：

### 1. UserService の実装

```go
type User struct {
    ID       int       `json:"id" db:"id"`
    Name     string    `json:"name" db:"name"`
    Email    string    `json:"email" db:"email"`
    CreateAt time.Time `json:"created_at" db:"created_at"`
}

type UserService struct {
    db    UserRepository
    cache CacheClient
    sf    *singleflight.Group
    metrics *ServiceMetrics
}
```

### 2. 必要なメソッドの実装

- `NewUserService(db UserRepository, cache CacheClient) *UserService`: サービスの初期化
- `GetUser(ctx context.Context, userID int) (*User, error)`: ユーザー取得（Cache-Aside）
- `CreateUser(ctx context.Context, user *User) error`: ユーザー作成
- `UpdateUser(ctx context.Context, user *User) error`: ユーザー更新
- `DeleteUser(ctx context.Context, userID int) error`: ユーザー削除
- `GetMetrics() ServiceMetrics`: サービスメトリクス取得

### 3. Single Flight による重複排除

同じユーザーIDに対する同時リクエストを統合してください。

### 4. メトリクス収集

キャッシュヒット率、レスポンス時間などの統計を収集してください。

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestUserService_CacheAside
    main_test.go:85: First access - cache miss, loaded from DB
    main_test.go:92: Second access - cache hit, served from cache
    main_test.go:99: Cache hit rate: 50.00%
--- PASS: TestUserService_CacheAside (0.03s)

=== RUN   TestUserService_SingleFlight
    main_test.go:125: 10 concurrent requests resulted in 1 DB query
    main_test.go:132: Single flight pattern working correctly
--- PASS: TestUserService_SingleFlight (0.02s)

=== RUN   TestUserService_UpdateInvalidation
    main_test.go:155: User created and cached
    main_test.go:162: User updated, cache invalidated
    main_test.go:169: Fresh data loaded from DB after update
--- PASS: TestUserService_UpdateInvalidation (0.04s)

PASS
ok      day42-cache-aside       0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### パッケージのインポート

```go
import (
    "context"
    "fmt"
    "time"
    
    "golang.org/x/sync/singleflight"
)
```

### Single Flight の使用

```go
type UserService struct {
    // ...
    sf *singleflight.Group
}

func (s *UserService) GetUser(ctx context.Context, userID int) (*User, error) {
    key := fmt.Sprintf("user:%d", userID)
    
    v, err, shared := s.sf.Do(key, func() (interface{}, error) {
        return s.loadUser(ctx, userID)
    })
    
    if err != nil {
        return nil, err
    }
    
    user := v.(*User)
    
    // shared が true の場合、他のリクエストと統合された
    if shared {
        s.metrics.SharedLoads++
    }
    
    return user, nil
}
```

### キャッシュキーの生成

```go
func userCacheKey(userID int) string {
    return fmt.Sprintf("user:%d", userID)
}

func allUsersCacheKey() string {
    return "users:all"
}
```

### メトリクスの原子的更新

```go
func (s *UserService) recordCacheHit() {
    atomic.AddInt64(&s.metrics.CacheHits, 1)
}

func (s *UserService) recordCacheMiss() {
    atomic.AddInt64(&s.metrics.CacheMisses, 1)
}
```

### エラーハンドリング

```go
user, err := s.loadFromCache(ctx, userID)
if err == ErrCacheMiss {
    // キャッシュミス - DB から読み込み
    return s.loadFromDB(ctx, userID)
} else if err != nil {
    // その他のエラー - フォールバック
    log.Printf("Cache error: %v, falling back to DB", err)
    return s.loadFromDB(ctx, userID)
}
```

### 書き込み操作でのキャッシュ無効化

```go
func (s *UserService) UpdateUser(ctx context.Context, user *User) error {
    // 1. データベースを更新
    err := s.db.UpdateUser(ctx, user)
    if err != nil {
        return err
    }
    
    // 2. キャッシュを無効化
    cacheKey := userCacheKey(user.ID)
    s.cache.Delete(ctx, cacheKey)
    
    // 3. 関連キャッシュも無効化
    s.cache.Delete(ctx, allUsersCacheKey())
    
    return nil
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **Bloom Filter**: 存在しないデータへのアクセスを効率的に判定
2. **Cache Warming**: アプリケーション起動時の事前キャッシュ
3. **Hierarchical Caching**: L1/L2 キャッシュの階層構造
4. **Adaptive TTL**: アクセス頻度に応じた TTL 調整
5. **Circuit Breaker**: データベース障害時のフォールバック制御

Cache-Aside パターンの実装を通じて、実際のプロダクション環境で使用されるキャッシング戦略の基礎を学びましょう！