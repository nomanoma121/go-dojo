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
// 【Thread-Safe Cacheの重要性】高性能Webアプリケーションの基盤
// ❌ 問題例：Thread-Unsafeなキャッシュによる壊滅的データ競合
func disastrousUnsafeCacheUsage() {
    // 🚨 災害例：map[string]interface{}の直接使用
    unsafeCache := make(map[string]interface{})
    
    // 大量の並行アクセス
    var wg sync.WaitGroup
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            key := fmt.Sprintf("user:%d", id)
            
            // ❌ 並行書き込みでパニック発生
            unsafeCache[key] = User{ID: id, Name: fmt.Sprintf("User%d", id)}
            // fatal error: concurrent map writes
            
            // ❌ 読み取り中の書き込みでデータ破損
            if data, exists := unsafeCache[key]; exists {
                user := data.(User)
                log.Printf("User: %+v", user)
                // 時々空のデータや不正なデータが読み込まれる
            }
        }(i)
    }
    wg.Wait()
    // 結果：プログラム即座にクラッシュ、サービス停止
}

// ✅ 正解：プロダクション品質のThread-Safe Cache
type EnterpriseCacheItem[V any] struct {
    Value      V              // キャッシュされた値
    Expiration time.Time      // 有効期限
    CreatedAt  time.Time      // 作成時刻
    AccessedAt time.Time      // 最終アクセス時刻
    AccessCount int64         // アクセス回数（popularity tracking）
    Size       int64          // データサイズ（メモリ使用量追跡用）
}

// 【重要メソッド】期限切れ判定（高精度）
func (item *EnterpriseCacheItem[V]) IsExpired() bool {
    if item.Expiration.IsZero() {
        return false // TTLなしの場合は期限切れなし
    }
    return time.Now().After(item.Expiration)
}

// 【重要メソッド】人気度計算（LRU改良版）
func (item *EnterpriseCacheItem[V]) GetPopularityScore() float64 {
    // 最近のアクセス頻度 × アクセス回数
    timeFactor := 1.0 - float64(time.Since(item.AccessedAt)) / float64(24*time.Hour)
    if timeFactor < 0 {
        timeFactor = 0
    }
    return float64(item.AccessCount) * timeFactor
}

// 【高性能Thread-Safe Cache】企業レベルの実装
type ProductionCache[K comparable, V any] struct {
    // 【基本構成】
    maxSize     int                              // 最大アイテム数
    maxMemory   int64                            // 最大メモリ使用量（bytes）
    items       map[K]*EnterpriseCacheItem[V]    // データストレージ
    lruList     *list.List                       // LRU管理用双方向リスト
    
    // 【同期制御】
    mu          sync.RWMutex                     // 読み書き排他制御
    
    // 【統計・監視】
    stats       *DetailedCacheStats              // 詳細統計情報
    
    // 【高度な機能】
    cleanupTicker *time.Ticker                   // 定期クリーンアップ
    ctx          context.Context                 // 停止制御用コンテキスト
    cancel       context.CancelFunc              // キャンセル関数
    
    // 【パフォーマンス最適化】
    shards      []*CacheShard[K, V]             // シャード分割（ホットスポット回避）
    numShards   int                             // シャード数
    
    // 【メモリ管理】
    currentMemory int64                         // 現在のメモリ使用量
    gcTrigger     int64                         // GC実行閾値
}

// 【重要関数】プロダクション用キャッシュ初期化
func NewProductionCache[K comparable, V any](maxSize int, maxMemoryMB int) *ProductionCache[K, V] {
    ctx, cancel := context.WithCancel(context.Background())
    
    cache := &ProductionCache[K, V]{
        maxSize:     maxSize,
        maxMemory:   int64(maxMemoryMB) * 1024 * 1024, // MB to bytes
        items:       make(map[K]*EnterpriseCacheItem[V], maxSize),
        lruList:     list.New(),
        stats:       NewDetailedCacheStats(),
        ctx:         ctx,
        cancel:      cancel,
        numShards:   runtime.NumCPU() * 2, // CPU数の2倍のシャード
        gcTrigger:   int64(maxMemoryMB) * 1024 * 1024 * 8 / 10, // 80%でGC実行
    }
    
    // 【シャード初期化】ホットスポット対策
    cache.shards = make([]*CacheShard[K, V], cache.numShards)
    for i := 0; i < cache.numShards; i++ {
        cache.shards[i] = NewCacheShard[K, V](maxSize / cache.numShards)
    }
    
    // 【定期クリーンアップ開始】
    cache.startBackgroundCleanup()
    
    log.Printf("🚀 Production cache initialized: maxSize=%d, maxMemory=%dMB, shards=%d", 
        maxSize, maxMemoryMB, cache.numShards)
    
    return cache
}

// 【重要メソッド】高性能データ取得
func (c *ProductionCache[K, V]) Get(key K) (V, bool) {
    // 【シャード選択】負荷分散
    shard := c.selectShard(key)
    
    // 【読み取りロック】並行読み取り許可
    c.mu.RLock()
    item, exists := c.items[key]
    if !exists {
        c.mu.RUnlock()
        c.stats.RecordMiss()
        
        var zero V
        return zero, false
    }
    
    // 【期限切れチェック】
    if item.IsExpired() {
        c.mu.RUnlock()
        // 【期限切れアイテムの非同期削除】
        go c.deleteExpiredItem(key)
        
        c.stats.RecordMiss()
        c.stats.RecordExpiration()
        
        var zero V
        return zero, false
    }
    
    // 【アクセス情報更新】popularity tracking
    item.AccessedAt = time.Now()
    atomic.AddInt64(&item.AccessCount, 1)
    
    // 【LRU更新】最近使用したアイテムをリスト先頭に移動
    c.lruList.MoveToFront(item.element)
    
    value := item.Value
    c.mu.RUnlock()
    
    // 【統計更新】
    c.stats.RecordHit()
    
    return value, true
}

// 【重要メソッド】高性能データ設定
func (c *ProductionCache[K, V]) Set(key K, value V, ttl time.Duration) bool {
    // 【メモリサイズ計算】
    itemSize := c.calculateItemSize(key, value)
    
    // 【メモリ制限チェック】
    if atomic.LoadInt64(&c.currentMemory) + itemSize > c.maxMemory {
        // メモリ制限に達した場合は古いアイテムを削除
        c.evictOldItems(itemSize)
    }
    
    // 【書き込みロック】排他制御
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    var expiration time.Time
    if ttl > 0 {
        expiration = now.Add(ttl)
    }
    
    // 【既存アイテムの更新】
    if existingItem, exists := c.items[key]; exists {
        // メモリ使用量を調整
        atomic.AddInt64(&c.currentMemory, -existingItem.Size)
        atomic.AddInt64(&c.currentMemory, itemSize)
        
        // 値を更新
        existingItem.Value = value
        existingItem.Expiration = expiration
        existingItem.AccessedAt = now
        existingItem.Size = itemSize
        
        // LRUリストで先頭に移動
        c.lruList.MoveToFront(existingItem.element)
        
        c.stats.RecordUpdate()
        return true
    }
    
    // 【容量制限チェック】
    if len(c.items) >= c.maxSize {
        // LRU削除
        c.evictLRU()
    }
    
    // 【新アイテム作成】
    item := &EnterpriseCacheItem[V]{
        Value:       value,
        Expiration:  expiration,
        CreatedAt:   now,
        AccessedAt:  now,
        AccessCount: 1,
        Size:        itemSize,
    }
    
    // LRUリストに追加
    element := c.lruList.PushFront(key)
    item.element = element
    
    // マップに追加
    c.items[key] = item
    
    // メモリ使用量更新
    atomic.AddInt64(&c.currentMemory, itemSize)
    
    c.stats.RecordSet()
    return true
}

// 【重要メソッド】LRU削除（最も使用頻度の低いアイテムを削除）
func (c *ProductionCache[K, V]) evictLRU() {
    // リストの最後尾（最も古いアイテム）を取得
    element := c.lruList.Back()
    if element == nil {
        return
    }
    
    // キーを取得
    key := element.Value.(K)
    
    // アイテムを削除
    if item, exists := c.items[key]; exists {
        delete(c.items, key)
        c.lruList.Remove(element)
        
        // メモリ使用量更新
        atomic.AddInt64(&c.currentMemory, -item.Size)
        
        c.stats.RecordEviction()
        
        log.Printf("🗑️  LRU evicted: key=%v, size=%d bytes", key, item.Size)
    }
}

// 【高度な機能】バックグラウンドクリーンアップ
func (c *ProductionCache[K, V]) startBackgroundCleanup() {
    c.cleanupTicker = time.NewTicker(5 * time.Minute)
    
    go func() {
        defer c.cleanupTicker.Stop()
        
        for {
            select {
            case <-c.cleanupTicker.C:
                c.performCleanup()
                
            case <-c.ctx.Done():
                log.Println("🛑 Cache cleanup goroutine terminated")
                return
            }
        }
    }()
}

// 【重要メソッド】期限切れアイテムの一括削除
func (c *ProductionCache[K, V]) performCleanup() {
    start := time.Now()
    cleanedCount := 0
    freedBytes := int64(0)
    
    c.mu.Lock()
    
    // 期限切れアイテムを収集
    expiredKeys := make([]K, 0)
    for key, item := range c.items {
        if item.IsExpired() {
            expiredKeys = append(expiredKeys, key)
            freedBytes += item.Size
        }
    }
    
    // 一括削除
    for _, key := range expiredKeys {
        if item, exists := c.items[key]; exists {
            delete(c.items, key)
            c.lruList.Remove(item.element)
            cleanedCount++
        }
    }
    
    c.mu.Unlock()
    
    // メモリ使用量更新
    atomic.AddInt64(&c.currentMemory, -freedBytes)
    
    duration := time.Since(start)
    
    if cleanedCount > 0 {
        log.Printf("🧹 Cleanup completed: %d items removed, %d bytes freed (took %v)", 
            cleanedCount, freedBytes, duration)
    }
    
    c.stats.RecordCleanup(cleanedCount, freedBytes)
}

// 【統計情報構造体】詳細な監視データ
type DetailedCacheStats struct {
    hits         int64
    misses       int64
    sets         int64
    updates      int64
    evictions    int64
    expirations  int64
    cleanups     int64
    
    totalCleanedItems int64
    totalFreedBytes   int64
    
    mu sync.RWMutex
}

// 【重要メソッド】統計情報の取得
func (s *DetailedCacheStats) GetSummary() CacheSummary {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    totalRequests := s.hits + s.misses
    hitRate := float64(0)
    if totalRequests > 0 {
        hitRate = float64(s.hits) / float64(totalRequests) * 100
    }
    
    return CacheSummary{
        HitRate:           hitRate,
        TotalRequests:     totalRequests,
        Hits:              s.hits,
        Misses:            s.misses,
        Sets:              s.sets,
        Updates:           s.updates,
        Evictions:         s.evictions,
        Expirations:       s.expirations,
        Cleanups:          s.cleanups,
        TotalCleanedItems: s.totalCleanedItems,
        TotalFreedBytes:   s.totalFreedBytes,
    }
}

// 【実用例】Webアプリケーションでの使用
func WebApplicationCacheUsage() {
    // 【初期化】最大10,000アイテム、最大100MBメモリ使用
    userCache := NewProductionCache[string, User](10000, 100)
    defer userCache.Close()
    
    // 【高負荷シミュレーション】
    var wg sync.WaitGroup
    successCount := int64(0)
    
    for i := 0; i < 5000; i++ {
        wg.Add(1)
        go func(userID int) {
            defer wg.Done()
            
            key := fmt.Sprintf("user:%d", userID)
            
            // 【キャッシュ取得試行】
            if user, found := userCache.Get(key); found {
                atomic.AddInt64(&successCount, 1)
                log.Printf("✅ Cache hit for %s: %+v", key, user)
                return
            }
            
            // 【キャッシュミス時：データベースから取得】
            user := fetchUserFromDatabase(userID) // 仮想的なDB呼び出し
            if user.ID != 0 {
                // 【キャッシュに保存】TTL = 1時間
                userCache.Set(key, user, 1*time.Hour)
                atomic.AddInt64(&successCount, 1)
                log.Printf("💾 Cached user %s from database", key)
            }
        }(i)
    }
    
    wg.Wait()
    
    // 【統計情報表示】
    stats := userCache.GetStats()
    log.Printf("🎯 Final Cache Stats:")
    log.Printf("   Hit Rate: %.2f%%", stats.HitRate)
    log.Printf("   Total Requests: %d", stats.TotalRequests)
    log.Printf("   Cache Size: %d items", userCache.Size())
    log.Printf("   Memory Usage: %.2f MB", float64(userCache.MemoryUsage())/1024/1024)
    log.Printf("   Success Operations: %d", atomic.LoadInt64(&successCount))
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