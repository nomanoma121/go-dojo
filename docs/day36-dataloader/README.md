# Day 36: Dataloaderパターン

🎯 **本日の目標**

N+1問題を効率的に解決するためのデータローダーパターンを実装できるようになる。

📖 **解説**

## Dataloaderパターンとは

Dataloaderパターンは、データベースへのクエリを効率的にバッチ化・キャッシュするデザインパターンです。特にGraphQLなどでN+1問題が発生しやすい環境で威力を発揮します。

### Dataloaderの特徴

1. **バッチ処理**: 複数のリクエストを一つのクエリにまとめる
2. **キャッシュ機能**: 同一リクエスト内での重複クエリを防ぐ
3. **遅延実行**: 実際に必要になるまでクエリを遅延させる
4. **並行安全性**: 複数のgoroutineから安全に使用可能

### 基本的なDataloader実装

```go
package main

import (
    "context"
    "sync"
    "time"
)

// DataLoader provides batching and caching functionality
type DataLoader[K comparable, V any] struct {
    batchFn     BatchFunc[K, V]
    cache       map[K]*result[V]
    batch       []K
    waiting     map[K][]chan *result[V]
    maxBatchSize int
    batchTimeout time.Duration
    mu          sync.Mutex
}

// BatchFunc defines the function signature for batch loading
type BatchFunc[K comparable, V any] func(ctx context.Context, keys []K) ([]V, []error)

// result holds the value and error for a specific key
type result[V any] struct {
    value V
    err   error
}

// NewDataLoader creates a new DataLoader
func NewDataLoader[K comparable, V any](
    batchFn BatchFunc[K, V],
    options ...Option[K, V],
) *DataLoader[K, V] {
    dl := &DataLoader[K, V]{
        batchFn:      batchFn,
        cache:        make(map[K]*result[V]),
        waiting:      make(map[K][]chan *result[V]),
        maxBatchSize: 100,
        batchTimeout: 16 * time.Millisecond,
    }
    
    for _, opt := range options {
        opt(dl)
    }
    
    return dl
}

// Load loads a single value by key
func (dl *DataLoader[K, V]) Load(ctx context.Context, key K) (V, error) {
    return dl.LoadThunk(ctx, key)()
}

// LoadMany loads multiple values by keys
func (dl *DataLoader[K, V]) LoadMany(ctx context.Context, keys []K) ([]V, []error) {
    thunks := make([]Thunk[V], len(keys))
    for i, key := range keys {
        thunks[i] = dl.LoadThunk(ctx, key)
    }
    
    values := make([]V, len(keys))
    errors := make([]error, len(keys))
    
    for i, thunk := range thunks {
        values[i], errors[i] = thunk()
    }
    
    return values, errors
}

// Thunk represents a deferred computation
type Thunk[V any] func() (V, error)

// LoadThunk returns a thunk for deferred execution
func (dl *DataLoader[K, V]) LoadThunk(ctx context.Context, key K) Thunk[V] {
    dl.mu.Lock()
    defer dl.mu.Unlock()
    
    // Check cache first
    if result, exists := dl.cache[key]; exists {
        return func() (V, error) {
            return result.value, result.err
        }
    }
    
    // Create result channel
    resultCh := make(chan *result[V], 1)
    
    // Add to waiting list
    if dl.waiting[key] == nil {
        dl.waiting[key] = []chan *result[V]{}
        dl.batch = append(dl.batch, key)
    }
    dl.waiting[key] = append(dl.waiting[key], resultCh)
    
    // Trigger batch execution if needed
    if len(dl.batch) >= dl.maxBatchSize {
        dl.executeImmediately(ctx)
    } else if len(dl.batch) == 1 {
        // Start timer for first item in batch
        go dl.executeAfterTimeout(ctx)
    }
    
    return func() (V, error) {
        result := <-resultCh
        return result.value, result.err
    }
}

// executeImmediately executes the current batch immediately
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
    
    // Clear current batch
    dl.batch = dl.batch[:0]
    dl.waiting = make(map[K][]chan *result[V])
    
    go func() {
        values, errors := dl.batchFn(ctx, keys)
        
        for i, key := range keys {
            var result *result[V]
            if i < len(values) && i < len(errors) {
                result = &result[V]{
                    value: values[i],
                    err:   errors[i],
                }
            } else {
                var zero V
                result = &result[V]{
                    value: zero,
                    err:   fmt.Errorf("missing result for key"),
                }
            }
            
            // Cache the result
            dl.mu.Lock()
            dl.cache[key] = result
            dl.mu.Unlock()
            
            // Send to all waiting channels
            for _, ch := range waiting[key] {
                ch <- result
                close(ch)
            }
        }
    }()
}

// executeAfterTimeout executes batch after timeout
func (dl *DataLoader[K, V]) executeAfterTimeout(ctx context.Context) {
    time.Sleep(dl.batchTimeout)
    
    dl.mu.Lock()
    defer dl.mu.Unlock()
    
    if len(dl.batch) > 0 {
        dl.executeImmediately(ctx)
    }
}
```

### 設定オプション

```go
// Option defines configuration options for DataLoader
type Option[K comparable, V any] func(*DataLoader[K, V])

// WithMaxBatchSize sets the maximum batch size
func WithMaxBatchSize[K comparable, V any](size int) Option[K, V] {
    return func(dl *DataLoader[K, V]) {
        dl.maxBatchSize = size
    }
}

// WithBatchTimeout sets the batch timeout
func WithBatchTimeout[K comparable, V any](timeout time.Duration) Option[K, V] {
    return func(dl *DataLoader[K, V]) {
        dl.batchTimeout = timeout
    }
}

// WithCache sets whether to enable caching
func WithCache[K comparable, V any](enabled bool) Option[K, V] {
    return func(dl *DataLoader[K, V]) {
        if !enabled {
            dl.cache = nil
        }
    }
}
```

📝 **課題**

以下の機能を持つDataloaderシステムを実装してください：

1. **`DataLoader`**: 汎用的なデータローダー
2. **`UserLoader`**: ユーザー専用データローダー
3. **`PostLoader`**: 投稿専用データローダー
4. **バッチ処理**: 複数のリクエストを効率的にまとめる
5. **キャッシュ機能**: 同一リクエスト内での重複防止
6. **統計情報**: バッチサイズ、キャッシュヒット率などの測定

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestDataLoader_Load
--- PASS: TestDataLoader_Load (0.01s)
=== RUN   TestDataLoader_LoadMany
--- PASS: TestDataLoader_LoadMany (0.02s)
=== RUN   TestDataLoader_Cache
--- PASS: TestDataLoader_Cache (0.01s)
=== RUN   TestDataLoader_Batch
--- PASS: TestDataLoader_Batch (0.02s)
=== RUN   TestUserLoader_Integration
--- PASS: TestUserLoader_Integration (0.01s)
PASS
ok      day36-dataloader    0.075s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **sync**パッケージ: 並行安全性のための`sync.Mutex`
2. **time**パッケージ: バッチタイムアウトのための`time.After`
3. **context**パッケージ: キャンセレーション対応
4. **channel**: 非同期結果の受け渡し
5. **ジェネリクス**: 型安全なデータローダー

バッチ処理のポイント：
- **タイミング**: 最大サイズに達するか、タイムアウトで実行
- **キャッシュ**: 同一キーの重複リクエストを防ぐ
- **エラーハンドリング**: バッチ内の個別エラーを適切に処理

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```