# Day 36: DataLoaderパターン実装

## 🎯 本日の目標

N+1問題を根本的に解決するDataLoaderパターンを深く理解し、高性能なバッチ処理・キャッシュシステムを構築する。実用的なシナリオを通じて、大規模アプリケーションで求められるデータアクセス最適化技術を習得する。

## 📖 解説

### DataLoaderパターンとは

DataLoaderパターンは、**データベースクエリの最適化**と**メモリ効率の向上**を同時に実現する画期的なデザインパターンです。Facebook（現Meta）が開発したGraphQLの実装で採用され、現在では様々なアプリケーションアーキテクチャで使用されています。

**従来のアプローチの問題：**

```go
// ❌ 非効率なN+1問題のある実装
func GetUsersWithPosts(userIDs []int) ([]UserWithPosts, error) {
    var users []UserWithPosts
    
    for _, userID := range userIDs {
        // 1. ユーザーを取得（N回のクエリ）
        user, err := db.QueryRow("SELECT * FROM users WHERE id = ?", userID).Scan(...)
        if err != nil {
            return nil, err
        }
        
        // 2. 各ユーザーの投稿を取得（さらにN回のクエリ）
        posts, err := db.Query("SELECT * FROM posts WHERE user_id = ?", userID).Scan(...)
        if err != nil {
            return nil, err
        }
        
        users = append(users, UserWithPosts{User: user, Posts: posts})
    }
    
    return users // 合計 1 + N + N = 2N+1 回のクエリ実行
}
```

**DataLoaderを使った効率的なアプローチ：**

```go
// ✅ DataLoaderによる最適化された実装
func GetUsersWithPostsOptimized(userIDs []int) ([]UserWithPosts, error) {
    var users []UserWithPosts
    
    // 1. 全ユーザーを一括取得（1回のクエリ）
    usersData, err := userLoader.LoadMany(context.Background(), userIDs)
    if err != nil {
        return nil, err
    }
    
    // 2. 全投稿を一括取得（1回のクエリ）
    postsData, err := postLoader.LoadMany(context.Background(), userIDs)
    if err != nil {
        return nil, err
    }
    
    // 3. データを組み合わせ
    for i, user := range usersData {
        users = append(users, UserWithPosts{
            User:  user,
            Posts: postsData[i],
        })
    }
    
    return users // 合計 2回のクエリのみ
}
```

### DataLoaderの核心原理

#### 1. **バッチング（Batching）**

複数の個別リクエストを単一のバッチリクエストに自動的に結合：

```go
// 個別のリクエスト
userLoader.Load(ctx, 1)    // SELECT * FROM users WHERE id = 1
userLoader.Load(ctx, 2)    // SELECT * FROM users WHERE id = 2
userLoader.Load(ctx, 3)    // SELECT * FROM users WHERE id = 3

// DataLoaderが自動的に以下に最適化：
// SELECT * FROM users WHERE id IN (1, 2, 3)
```

#### 2. **リクエストレベルキャッシング**

同一リクエスト内での重複データアクセスを完全に除去：

```go
// 最初のアクセス
user1 := userLoader.Load(ctx, 1) // データベースからロード

// 同じリクエスト内での再アクセス
user1Again := userLoader.Load(ctx, 1) // キャッシュから即座に返却（DB未アクセス）
```

#### 3. **遅延実行（Deferred Execution）**

リクエストを即座に実行せず、最適なタイミングでバッチ処理：

```go
// これらの呼び出しは即座には実行されない
thunk1 := userLoader.LoadThunk(ctx, 1)
thunk2 := userLoader.LoadThunk(ctx, 2) 
thunk3 := userLoader.LoadThunk(ctx, 3)

// 実際のデータが必要になった時点で、まとめて実行
user1, err1 := thunk1()  // この時点で一括クエリが実行される
user2, err2 := thunk2()  // キャッシュから取得
user3, err3 := thunk3()  // キャッシュから取得
```

### 完全なDataLoader実装

#### 核心のDataLoader構造体

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// DataLoader は高性能バッチング・キャッシュシステムを提供
type DataLoader[K comparable, V any] struct {
    batchFn       BatchFunc[K, V]      // バッチ処理関数
    cache         map[K]*result[V]     // リクエストレベルキャッシュ
    batch         []K                  // 現在のバッチキュー
    waiting       map[K][]chan *result[V] // 待機中のリクエスト
    maxBatchSize  int                  // 最大バッチサイズ
    batchTimeout  time.Duration        // バッチタイムアウト
    mu            sync.Mutex           // 並行制御
    stats         *LoaderStats         // 統計情報
}

// BatchFunc はバッチ処理の関数型定義
type BatchFunc[K comparable, V any] func(ctx context.Context, keys []K) ([]V, []error)

// result は値とエラーをペアで保持
type result[V any] struct {
    value V
    err   error
    loadTime time.Time
}

// LoaderStats は DataLoader の統計情報
type LoaderStats struct {
    BatchCount      int64         // バッチ実行回数
    CacheHits       int64         // キャッシュヒット数
    CacheMisses     int64         // キャッシュミス数
    TotalLoadTime   time.Duration // 累積ロード時間
    AverageBatchSize float64      // 平均バッチサイズ
    mu              sync.RWMutex
}

// NewDataLoader は新しい DataLoader を作成
func NewDataLoader[K comparable, V any](
    batchFn BatchFunc[K, V],
    options ...Option[K, V],
) *DataLoader[K, V] {
    dl := &DataLoader[K, V]{
        batchFn:       batchFn,
        cache:         make(map[K]*result[V]),
        waiting:       make(map[K][]chan *result[V]),
        maxBatchSize:  100,
        batchTimeout:  16 * time.Millisecond,
        stats:         &LoaderStats{},
    }
    
    for _, opt := range options {
        opt(dl)
    }
    
    return dl
}

// Load は単一キーでデータをロード
func (dl *DataLoader[K, V]) Load(ctx context.Context, key K) (V, error) {
    return dl.LoadThunk(ctx, key)()
}

// LoadMany は複数キーでデータを一括ロード
func (dl *DataLoader[K, V]) LoadMany(ctx context.Context, keys []K) ([]V, []error) {
    thunks := make([]Thunk[V], len(keys))
    for i, key := range keys {
        thunks[i] = dl.LoadThunk(ctx, key)
    }
    
    values := make([]V, len(keys))
    errors := make([]error, len(keys))
    
    // 並行実行で効率化
    var wg sync.WaitGroup
    for i, thunk := range thunks {
        wg.Add(1)
        go func(index int, t Thunk[V]) {
            defer wg.Done()
            values[index], errors[index] = t()
        }(i, thunk)
    }
    wg.Wait()
    
    return values, errors
}

// Thunk は遅延実行可能な計算を表現
type Thunk[V any] func() (V, error)

// LoadThunk は遅延実行用の thunk を返却
func (dl *DataLoader[K, V]) LoadThunk(ctx context.Context, key K) Thunk[V] {
    dl.mu.Lock()
    defer dl.mu.Unlock()
    
    // キャッシュチェック
    if result, exists := dl.cache[key]; exists {
        dl.stats.recordCacheHit()
        return func() (V, error) {
            return result.value, result.err
        }
    }
    
    dl.stats.recordCacheMiss()
    
    // 結果チャネル作成
    resultCh := make(chan *result[V], 1)
    
    // 待機リストに追加
    if dl.waiting[key] == nil {
        dl.waiting[key] = []chan *result[V]{}
        dl.batch = append(dl.batch, key)
    }
    dl.waiting[key] = append(dl.waiting[key], resultCh)
    
    // バッチ実行トリガー
    if len(dl.batch) >= dl.maxBatchSize {
        dl.executeImmediately(ctx)
    } else if len(dl.batch) == 1 {
        // 最初の要素でタイマー開始
        go dl.executeAfterTimeout(ctx)
    }
    
    return func() (V, error) {
        select {
        case result := <-resultCh:
            return result.value, result.err
        case <-ctx.Done():
            var zero V
            return zero, ctx.Err()
        }
    }
}

// executeImmediately は現在のバッチを即座に実行
func (dl *DataLoader[K, V]) executeImmediately(ctx context.Context) {
    if len(dl.batch) == 0 {
        return
    }
    
    keys := make([]K, len(dl.batch))
    copy(keys, dl.batch)
    waiting := make(map[K][]chan *result[V])
    for k, v := range dl.waiting {
        waiting[k] = v
    }
    
    // バッチクリア
    dl.batch = dl.batch[:0]
    dl.waiting = make(map[K][]chan *result[V])
    
    go func() {
        startTime := time.Now()
        values, errors := dl.batchFn(ctx, keys)
        loadTime := time.Since(startTime)
        
        dl.stats.recordBatch(len(keys), loadTime)
        
        for i, key := range keys {
            var result *result[V]
            if i < len(values) && i < len(errors) {
                result = &result[V]{
                    value:    values[i],
                    err:      errors[i],
                    loadTime: time.Now(),
                }
            } else {
                var zero V
                result = &result[V]{
                    value:    zero,
                    err:      fmt.Errorf("missing result for key"),
                    loadTime: time.Now(),
                }
            }
            
            // キャッシュに保存
            dl.mu.Lock()
            dl.cache[key] = result
            dl.mu.Unlock()
            
            // 待機中の全チャネルに送信
            for _, ch := range waiting[key] {
                select {
                case ch <- result:
                default:
                }
                close(ch)
            }
        }
    }()
}

// executeAfterTimeout はタイムアウト後にバッチを実行
func (dl *DataLoader[K, V]) executeAfterTimeout(ctx context.Context) {
    timer := time.NewTimer(dl.batchTimeout)
    defer timer.Stop()
    
    select {
    case <-timer.C:
        dl.mu.Lock()
        if len(dl.batch) > 0 {
            dl.executeImmediately(ctx)
        }
        dl.mu.Unlock()
    case <-ctx.Done():
        return
    }
}

// GetStats は統計情報を取得
func (dl *DataLoader[K, V]) GetStats() LoaderStats {
    return dl.stats.get()
}

// ClearCache はキャッシュをクリア
func (dl *DataLoader[K, V]) ClearCache() {
    dl.mu.Lock()
    defer dl.mu.Unlock()
    dl.cache = make(map[K]*result[V])
}

// Prime はキャッシュに値を事前設定
func (dl *DataLoader[K, V]) Prime(key K, value V) {
    dl.mu.Lock()
    defer dl.mu.Unlock()
    dl.cache[key] = &result[V]{
        value:    value,
        err:      nil,
        loadTime: time.Now(),
    }
}
```

#### 統計情報管理

```go
// recordCacheHit はキャッシュヒットを記録
func (s *LoaderStats) recordCacheHit() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.CacheHits++
}

// recordCacheMiss はキャッシュミスを記録
func (s *LoaderStats) recordCacheMiss() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.CacheMisses++
}

// recordBatch はバッチ実行を記録
func (s *LoaderStats) recordBatch(batchSize int, loadTime time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    s.BatchCount++
    s.TotalLoadTime += loadTime
    
    // 移動平均で平均バッチサイズを計算
    alpha := 0.1 // 平滑化係数
    s.AverageBatchSize = alpha*float64(batchSize) + (1-alpha)*s.AverageBatchSize
}

// get は統計情報のコピーを取得
func (s *LoaderStats) get() LoaderStats {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return LoaderStats{
        BatchCount:       s.BatchCount,
        CacheHits:        s.CacheHits,
        CacheMisses:      s.CacheMisses,
        TotalLoadTime:    s.TotalLoadTime,
        AverageBatchSize: s.AverageBatchSize,
    }
}

// CacheHitRate はキャッシュヒット率を計算
func (s *LoaderStats) CacheHitRate() float64 {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    total := s.CacheHits + s.CacheMisses
    if total == 0 {
        return 0
    }
    return float64(s.CacheHits) / float64(total)
}
```

#### 設定オプション

```go
// Option は DataLoader の設定オプション
type Option[K comparable, V any] func(*DataLoader[K, V])

// WithMaxBatchSize は最大バッチサイズを設定
func WithMaxBatchSize[K comparable, V any](size int) Option[K, V] {
    return func(dl *DataLoader[K, V]) {
        if size > 0 {
            dl.maxBatchSize = size
        }
    }
}

// WithBatchTimeout はバッチタイムアウトを設定
func WithBatchTimeout[K comparable, V any](timeout time.Duration) Option[K, V] {
    return func(dl *DataLoader[K, V]) {
        if timeout > 0 {
            dl.batchTimeout = timeout
        }
    }
}

// WithCache はキャッシュの有効/無効を設定
func WithCache[K comparable, V any](enabled bool) Option[K, V] {
    return func(dl *DataLoader[K, V]) {
        if !enabled {
            dl.cache = nil
        }
    }
}

// WithStats は統計情報収集の有効/無効を設定
func WithStats[K comparable, V any](enabled bool) Option[K, V] {
    return func(dl *DataLoader[K, V]) {
        if !enabled {
            dl.stats = nil
        }
    }
}
```

### 実用的なDataLoader実装例

#### ユーザー・投稿システムでの活用

```go
// User と Post の実体
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type Post struct {
    ID     int    `json:"id"`
    Title  string `json:"title"`
    UserID int    `json:"user_id"`
    Body   string `json:"body"`
}

// UserLoader の実装
type UserLoader struct {
    loader *DataLoader[int, *User]
    db     *sql.DB
}

func NewUserLoader(db *sql.DB) *UserLoader {
    batchFn := func(ctx context.Context, userIDs []int) ([]*User, []error) {
        // IN句を使った効率的な一括取得
        query := `SELECT id, name, email FROM users WHERE id IN (` + 
                strings.Repeat("?,", len(userIDs)-1) + "?)"
        
        args := make([]interface{}, len(userIDs))
        for i, id := range userIDs {
            args[i] = id
        }
        
        rows, err := db.QueryContext(ctx, query, args...)
        if err != nil {
            // 全て同じエラーを返す
            errors := make([]error, len(userIDs))
            users := make([]*User, len(userIDs))
            for i := range errors {
                errors[i] = err
            }
            return users, errors
        }
        defer rows.Close()
        
        // 結果をマッピング
        userMap := make(map[int]*User)
        for rows.Next() {
            user := &User{}
            if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
                continue
            }
            userMap[user.ID] = user
        }
        
        // 元の順序で結果を構築
        users := make([]*User, len(userIDs))
        errors := make([]error, len(userIDs))
        for i, id := range userIDs {
            if user, found := userMap[id]; found {
                users[i] = user
                errors[i] = nil
            } else {
                users[i] = nil
                errors[i] = fmt.Errorf("user not found: %d", id)
            }
        }
        
        return users, errors
    }
    
    return &UserLoader{
        loader: NewDataLoader(batchFn,
            WithMaxBatchSize[int, *User](50),
            WithBatchTimeout[int, *User](10*time.Millisecond),
        ),
        db: db,
    }
}

func (ul *UserLoader) Load(ctx context.Context, userID int) (*User, error) {
    return ul.loader.Load(ctx, userID)
}

func (ul *UserLoader) LoadMany(ctx context.Context, userIDs []int) ([]*User, []error) {
    return ul.loader.LoadMany(ctx, userIDs)
}

// PostLoader の実装
type PostLoader struct {
    loader *DataLoader[int, []*Post]
    db     *sql.DB
}

func NewPostLoader(db *sql.DB) *PostLoader {
    batchFn := func(ctx context.Context, userIDs []int) ([][]*Post, []error) {
        query := `SELECT id, title, user_id, body FROM posts WHERE user_id IN (` +
                strings.Repeat("?,", len(userIDs)-1) + "?) ORDER BY user_id, id"
        
        args := make([]interface{}, len(userIDs))
        for i, id := range userIDs {
            args[i] = id
        }
        
        rows, err := db.QueryContext(ctx, query, args...)
        if err != nil {
            errors := make([]error, len(userIDs))
            posts := make([][]*Post, len(userIDs))
            for i := range errors {
                errors[i] = err
                posts[i] = []*Post{}
            }
            return posts, errors
        }
        defer rows.Close()
        
        // ユーザーIDごとに投稿をグループ化
        postMap := make(map[int][]*Post)
        for rows.Next() {
            post := &Post{}
            if err := rows.Scan(&post.ID, &post.Title, &post.UserID, &post.Body); err != nil {
                continue
            }
            postMap[post.UserID] = append(postMap[post.UserID], post)
        }
        
        // 結果を構築
        posts := make([][]*Post, len(userIDs))
        errors := make([]error, len(userIDs))
        for i, userID := range userIDs {
            if userPosts, found := postMap[userID]; found {
                posts[i] = userPosts
            } else {
                posts[i] = []*Post{} // 空のスライス
            }
            errors[i] = nil
        }
        
        return posts, errors
    }
    
    return &PostLoader{
        loader: NewDataLoader(batchFn,
            WithMaxBatchSize[int, []*Post](30),
            WithBatchTimeout[int, []*Post](15*time.Millisecond),
        ),
        db: db,
    }
}

func (pl *PostLoader) Load(ctx context.Context, userID int) ([]*Post, error) {
    return pl.loader.Load(ctx, userID)
}
```

#### パフォーマンス測定システム

```go
// LoaderMetrics は複数のDataLoaderの統計を集計
type LoaderMetrics struct {
    loaders map[string]StatProvider
    mu      sync.RWMutex
}

type StatProvider interface {
    GetStats() LoaderStats
}

func NewLoaderMetrics() *LoaderMetrics {
    return &LoaderMetrics{
        loaders: make(map[string]StatProvider),
    }
}

func (lm *LoaderMetrics) RegisterLoader(name string, loader StatProvider) {
    lm.mu.Lock()
    defer lm.mu.Unlock()
    lm.loaders[name] = loader
}

func (lm *LoaderMetrics) GetAggregatedStats() map[string]LoaderStats {
    lm.mu.RLock()
    defer lm.mu.RUnlock()
    
    stats := make(map[string]LoaderStats)
    for name, loader := range lm.loaders {
        stats[name] = loader.GetStats()
    }
    return stats
}

func (lm *LoaderMetrics) PrintReport() {
    stats := lm.GetAggregatedStats()
    
    fmt.Println("=== DataLoader Performance Report ===")
    for name, stat := range stats {
        fmt.Printf("\n%s:\n", name)
        fmt.Printf("  Batches: %d\n", stat.BatchCount)
        fmt.Printf("  Cache Hit Rate: %.2f%%\n", stat.CacheHitRate()*100)
        fmt.Printf("  Avg Batch Size: %.1f\n", stat.AverageBatchSize)
        if stat.BatchCount > 0 {
            avgTime := stat.TotalLoadTime / time.Duration(stat.BatchCount)
            fmt.Printf("  Avg Load Time: %v\n", avgTime)
        }
    }
}
```

#### GraphQL統合例

```go
// GraphQL リゾルバでのDataLoader活用
type Resolvers struct {
    userLoader *UserLoader
    postLoader *PostLoader
}

func (r *Resolvers) User(ctx context.Context, id int) (*User, error) {
    return r.userLoader.Load(ctx, id)
}

func (r *Resolvers) UserPosts(ctx context.Context, user *User) ([]*Post, error) {
    return r.postLoader.Load(ctx, user.ID)
}

// 複雑なクエリでの効果
func (r *Resolvers) UsersWithPosts(ctx context.Context, userIDs []int) ([]*UserWithPosts, error) {
    // 並行して両方のデータを取得
    usersCh := make(chan []*User, 1)
    postsCh := make(chan [][]*Post, 1)
    errCh := make(chan error, 2)
    
    go func() {
        users, errs := r.userLoader.LoadMany(ctx, userIDs)
        for _, err := range errs {
            if err != nil {
                errCh <- err
                return
            }
        }
        usersCh <- users
    }()
    
    go func() {
        posts, errs := r.postLoader.LoadMany(ctx, userIDs)
        for _, err := range errs {
            if err != nil {
                errCh <- err
                return
            }
        }
        postsCh <- posts
    }()
    
    var users []*User
    var posts [][]*Post
    
    for i := 0; i < 2; i++ {
        select {
        case u := <-usersCh:
            users = u
        case p := <-postsCh:
            posts = p
        case err := <-errCh:
            return nil, err
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
    
    // 結果を結合
    result := make([]*UserWithPosts, len(userIDs))
    for i := range userIDs {
        result[i] = &UserWithPosts{
            User:  users[i],
            Posts: posts[i],
        }
    }
    
    return result, nil
}

type UserWithPosts struct {
    User  *User   `json:"user"`
    Posts []*Post `json:"posts"`
}
```

## 📝 課題（実装要件）

`main_test.go`のテストケースをすべてパスするように、以下の機能を実装してください：

### 1. **汎用DataLoaderシステム**
- ジェネリクスを活用した型安全な実装
- バッチング・キャッシング・遅延実行の完全サポート
- 統計情報収集とパフォーマンス監視

### 2. **UserLoader（ユーザー専用ローダー）**
- データベースからの効率的なユーザー一括取得
- 欠損データの適切なハンドリング
- キャッシュの有効活用

### 3. **PostLoader（投稿専用ローダー）**
- ユーザーIDをキーとした投稿データの一括取得
- 1対多関係の効率的な処理
- 空の結果セットの適切な処理

### 4. **統合テストシナリオ**
- N+1問題の回避検証
- 並行アクセス時の安全性確認
- パフォーマンス改善効果の測定

**実装すべき関数：**

```go
// 汎用DataLoader
func NewDataLoader[K comparable, V any](batchFn BatchFunc[K, V], options ...Option[K, V]) *DataLoader[K, V]
func (dl *DataLoader[K, V]) Load(ctx context.Context, key K) (V, error)
func (dl *DataLoader[K, V]) LoadMany(ctx context.Context, keys []K) ([]V, []error)
func (dl *DataLoader[K, V]) GetStats() LoaderStats

// 専用ローダー
func NewUserLoader(db *sql.DB) *UserLoader
func NewPostLoader(db *sql.DB) *PostLoader
```

## ✅ 期待される挙動

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestDataLoader_Load
=== RUN   TestDataLoader_Load/Single_user_load
=== RUN   TestDataLoader_Load/Cache_effectiveness
--- PASS: TestDataLoader_Load (0.01s)

=== RUN   TestDataLoader_LoadMany
=== RUN   TestDataLoader_LoadMany/Batch_loading
=== RUN   TestDataLoader_LoadMany/Mixed_cache_miss_hit
--- PASS: TestDataLoader_LoadMany (0.02s)

=== RUN   TestDataLoader_Batch
=== RUN   TestDataLoader_Batch/Automatic_batching
=== RUN   TestDataLoader_Batch/Timeout_batching
--- PASS: TestDataLoader_Batch (0.02s)

=== RUN   TestUserLoader_Integration
=== RUN   TestUserLoader_Integration/N_plus_one_prevention
--- PASS: TestUserLoader_Integration (0.01s)

=== RUN   TestPostLoader_Integration
=== RUN   TestPostLoader_Integration/User_posts_batching
--- PASS: TestPostLoader_Integration (0.01s)

PASS
ok      day36-dataloader    0.075s
```

### パフォーマンスベンチマーク例
```bash
$ go test -bench=.
BenchmarkDataLoader_Load-8           5000000    250 ns/op      48 B/op    2 allocs/op
BenchmarkDataLoader_LoadMany-8       1000000   1500 ns/op     256 B/op    8 allocs/op
BenchmarkUserLoader_NPlus1-8           10000 100000 ns/op    2048 B/op   50 allocs/op
BenchmarkUserLoader_Optimized-8      500000   3000 ns/op     512 B/op   10 allocs/op
```

### 実行時ログ例
```
=== DataLoader Performance Report ===

UserLoader:
  Batches: 15
  Cache Hit Rate: 73.50%
  Avg Batch Size: 8.2
  Avg Load Time: 2.5ms

PostLoader:
  Batches: 12
  Cache Hit Rate: 45.20%
  Avg Batch Size: 12.1
  Avg Load Time: 4.1ms

Performance Improvement:
  Traditional N+1: 250 queries in 1.2s
  DataLoader: 27 batches in 85ms
  Speed Improvement: 14.1x
```

## 💡 ヒント

実装に詰まった場合は、以下のヒントを参考にしてください：

### 基本的なバッチ関数の実装
```go
func userBatchFn(ctx context.Context, userIDs []int) ([]*User, []error) {
    // 1. IN句でまとめて取得
    query := "SELECT id, name, email FROM users WHERE id IN (...)"
    
    // 2. 結果をマップに格納
    userMap := make(map[int]*User)
    
    // 3. 元の順序で結果を再構築
    users := make([]*User, len(userIDs))
    errors := make([]error, len(userIDs))
    
    return users, errors
}
```

### キャッシュの活用方法
```go
// 事前にデータを設定
userLoader.Prime(1, &User{ID: 1, Name: "Alice"})

// キャッシュからのヒット
user, err := userLoader.Load(ctx, 1) // DB未アクセス
```

### 統計情報の活用
```go
stats := userLoader.GetStats()
fmt.Printf("Cache Hit Rate: %.2f%%\n", stats.CacheHitRate()*100)
```

**重要な実装ポイント：**
- **並行安全性**: `sync.Mutex`を適切に使用
- **リソース管理**: goroutineリークの防止
- **エラーハンドリング**: 個別エラーとバッチエラーの区別
- **メモリ効率**: 不要なデータコピーの回避

## 実行方法

```bash
# データベース準備
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=testdb postgres:15

# テスト実行
go test -v
go test -race          # レースコンディション検出
go test -bench=.       # ベンチマーク測定
go test -cover         # カバレッジ確認
```