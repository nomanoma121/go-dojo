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
func (s *UserService) GetUser(ctx context.Context, userID int) (*User, error) {
    // 1. キャッシュから取得を試行
    cacheKey := fmt.Sprintf("user:%d", userID)
    var user User
    err := s.cache.GetJSON(ctx, cacheKey, &user)
    if err == nil {
        // キャッシュヒット
        return &user, nil
    }
    
    // 2. キャッシュミス - データベースから取得
    user, err = s.db.GetUser(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // 3. キャッシュに保存
    s.cache.SetJSON(ctx, cacheKey, user, 1*time.Hour)
    
    return &user, nil
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
type singleflight.Group

func (s *UserService) GetUser(ctx context.Context, userID int) (*User, error) {
    cacheKey := fmt.Sprintf("user:%d", userID)
    
    // Single Flight でDB アクセスを統合
    v, err, _ := s.sf.Do(cacheKey, func() (interface{}, error) {
        return s.loadUserFromDB(ctx, userID)
    })
    
    if err != nil {
        return nil, err
    }
    
    return v.(*User), nil
}
```

#### 2. 分散ロック

Redis を使用した分散ロック：

```go
func (s *UserService) GetUser(ctx context.Context, userID int) (*User, error) {
    cacheKey := fmt.Sprintf("user:%d", userID)
    lockKey := fmt.Sprintf("lock:user:%d", userID)
    
    // 分散ロックを取得
    lock, err := s.acquireLock(ctx, lockKey, 10*time.Second)
    if err != nil {
        return nil, err
    }
    defer lock.Release()
    
    // ロック取得後、再度キャッシュを確認
    var user User
    err = s.cache.GetJSON(ctx, cacheKey, &user)
    if err == nil {
        return &user, nil
    }
    
    // DB から取得してキャッシュに保存
    // ...
}
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