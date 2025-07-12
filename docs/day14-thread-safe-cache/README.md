# Day 14: スレッドセーフなキャッシュ

## 🎯 本日の目標 (Today's Goal)

`sync.RWMutex`と`container/list`を使用して、高性能で並行アクセス可能なインメモリキャッシュシステムを実装し、実際のWebアプリケーションで使用できるレベルのキャッシュシステムを学習します。

## 📖 解説 (Explanation)

### スレッドセーフなキャッシュとは

インメモリキャッシュは、データベースやAPIへのアクセス回数を減らし、アプリケーションのパフォーマンスを大幅に向上させる重要な技術です。複数のgoroutineが同時にアクセスする環境では、データの一貫性を保ちながら高性能を実現するスレッドセーフな実装が必要です。

### なぜ重要なのか？

1. **パフォーマンス向上**: データベースアクセスを劇的に削減
2. **レスポンス時間短縮**: メモリアクセスはディスクアクセスより数千倍高速
3. **スケーラビリティ**: 外部リソースへの依存を減らし、システムの拡張性を向上
4. **コスト削減**: データベースやAPIの負荷を軽減してインフラコストを削減

### キャッシュ設計の重要な要素

#### 1. 並行アクセス制御
複数のgoroutineからの同時アクセスを安全に処理するため、適切な同期メカニズムが必要です：

- **Read-Write Mutex (`sync.RWMutex`)**: 読み取り操作を並行化し、書き込み操作を排他制御
- **Atomic Operations**: 統計情報の更新など、単純な操作の並行安全性
- **Lock-Free Structures**: `sync.Map`などの高性能な並行データ構造

#### 2. TTL (Time To Live) 機能
データの有効期限を管理し、古いデータの自動削除を実現：

```go
type CacheItem struct {
    Value      interface{}
    Expiration time.Time
    CreatedAt  time.Time
}

func (item *CacheItem) IsExpired() bool {
    return time.Now().After(item.Expiration)
}
```

#### 3. LRU (Least Recently Used) 削除
メモリ使用量を制限し、最も使用頻度の低いデータを効率的に削除：

```go
// 双方向連結リストとハッシュマップの組み合わせで O(1) 操作を実現
type LRUCache struct {
    capacity int
    items    map[string]*list.Element
    lruList  *list.List
}
```

#### 4. 統計情報とモニタリング
キャッシュの効率性を測定し、チューニングのための指標を提供：

- **ヒット率 (Hit Ratio)**: `hits / (hits + misses) * 100`
- **ミス率 (Miss Ratio)**: `misses / (hits + misses) * 100`
- **削除率 (Eviction Rate)**: 容量制限による削除の頻度

### パフォーマンス考慮事項

#### Read-Heavy vs Write-Heavy ワークロード
- **Read-Heavy**: `sync.RWMutex`で読み取りを並行化
- **Write-Heavy**: `sync.Map`やより細かい粒度のロックを検討

#### メモリ効率性
- **ポインタ vs 値**: 大きなオブジェクトはポインタで格納
- **メモリプール**: 頻繁に作成/削除されるオブジェクトの再利用
- **ガベージコレクション**: 参照の循環を避ける設計

## 📝 課題 (The Problem)

以下の要件を満たすスレッドセーフなキャッシュシステムを実装してください：

### 基本構造

```go
// Cache represents a thread-safe cache with TTL and LRU eviction
type Cache[K comparable, V any] struct {
    maxSize int
    items   map[K]*cacheItem[V]
    lruList *list.List
    mu      sync.RWMutex
    stats   *CacheStats
}

// cacheItem represents a cached item with metadata
type cacheItem[V any] struct {
    key        K
    value      V
    expiration time.Time
    element    *list.Element
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
    hits       int64
    misses     int64
    evictions  int64
    size       int64
}

// CacheConfig contains cache configuration options
type CacheConfig struct {
    MaxSize       int
    DefaultTTL    time.Duration
    CleanupInterval time.Duration
}
```

### 実装すべきメソッド

1. **NewCache**: 新しいキャッシュインスタンスを作成
2. **Set**: キーと値をキャッシュに設定（TTL付き）
3. **Get**: キーで値を取得（LRU更新付き）
4. **Delete**: 特定のキーを削除
5. **Clear**: すべてのエントリを削除
6. **GetStats**: 統計情報を取得
7. **Cleanup**: 期限切れエントリの手動削除

### 高度な機能

1. **BatchSet/BatchGet**: 複数のキーを一度に操作
2. **GetOrSet**: 存在しなければ設定、存在すれば取得
3. **Touch**: アクセス時間を更新（TTL延長）
4. **Keys/Values**: すべてのキーまたは値を取得

## ✅ 期待される挙動 (Expected Behavior)

正しく実装されたキャッシュシステムは以下のように動作します：

### 基本操作
```go
cache := NewCache[string, string](CacheConfig{
    MaxSize:    100,
    DefaultTTL: 5 * time.Minute,
})

// 設定
cache.Set("user:123", "John Doe", time.Hour)

// 取得（ヒット）
value, found := cache.Get("user:123")
// value = "John Doe", found = true

// 期限切れ後
time.Sleep(time.Hour + time.Second)
value, found = cache.Get("user:123")  
// value = "", found = false (期限切れで削除)
```

### LRU削除
```go
cache := NewCache[int, string](CacheConfig{MaxSize: 2})

cache.Set(1, "one", time.Hour)
cache.Set(2, "two", time.Hour)
cache.Set(3, "three", time.Hour)  // キー1が削除される

_, found := cache.Get(1)  // found = false
```

### 統計情報
```go
stats := cache.GetStats()
fmt.Printf("ヒット率: %.2f%%", stats.HitRate())
// ヒット率: 85.50%
```

## 💡 ヒント (Hints)

1. **Read-Write Mutex**: `sync.RWMutex`で読み取り操作を並行化し、書き込み時のみ排他制御
2. **Double-Checked Locking**: 期限切れチェックを効率化
3. **Atomic Operations**: 統計更新は`sync/atomic`で高性能に
4. **Container/List**: LRU実装には双方向連結リストが最適
5. **Generics**: 型安全性とパフォーマンスを両立

### パフォーマンス最適化のコツ

```go
// 良い例：読み取り専用操作は RLock を使用
func (c *Cache[K, V]) Get(key K) (V, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    // 読み取り処理...
}

// 悪い例：読み取りでも排他ロック
func (c *Cache[K, V]) Get(key K) (V, bool) {
    c.mu.Lock()  // 他の読み取りをブロックしてしまう
    defer c.mu.Unlock()
    // 読み取り処理...
}
```

## 実装パターンと設計判断

### 1. 期限切れチェック戦略
- **Lazy Expiration**: アクセス時に期限をチェック（メモリ効率）
- **Active Expiration**: 定期的なクリーンアップ（レスポンス時間優先）
- **Hybrid Approach**: 両方を組み合わせて最適化

### 2. ロック粒度の選択
- **粗い粒度**: 単一のmutexで全体を保護（実装簡単）
- **細かい粒度**: セグメント単位のロック（高並行性）
- **Lock-Free**: `sync.Map`やatomic操作（最高性能）

### 3. メモリ管理戦略
- **即座削除**: 期限切れ時に即座に削除
- **遅延削除**: 次回アクセス時に削除
- **バッチ削除**: 定期的にまとめて削除

## スコアカード

- ✅ **基本実装**: Get/Set/Delete操作が並行安全に動作する
- ✅ **TTL機能**: 有効期限切れのデータが適切に削除される
- ✅ **LRU削除**: 容量上限時に最も古いデータが削除される
- ✅ **統計情報**: ヒット率などの統計が正確に収集される
- ✅ **型安全性**: ジェネリクスによる型安全なインターフェース
- ✅ **高性能**: 読み取り操作の並行化とO(1)操作の実現

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```

## 参考資料

- [sync.RWMutex Documentation](https://pkg.go.dev/sync#RWMutex)
- [container/list Documentation](https://pkg.go.dev/container/list)
- [sync/atomic Documentation](https://pkg.go.dev/sync/atomic)
- [Go Generics Tutorial](https://go.dev/doc/tutorial/generics)
- [Effective Go - Concurrency](https://golang.org/doc/effective_go#concurrency)