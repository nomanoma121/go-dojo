package main

import (
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CacheStats tracks cache performance metrics
type CacheStats struct {
	hits      int64
	misses    int64
	evictions int64
	sets      int64
	deletes   int64
}

// GetHits returns the number of cache hits
func (cs *CacheStats) GetHits() int64 {
	return atomic.LoadInt64(&cs.hits)
}

// GetMisses returns the number of cache misses
func (cs *CacheStats) GetMisses() int64 {
	return atomic.LoadInt64(&cs.misses)
}

// GetEvictions returns the number of evictions
func (cs *CacheStats) GetEvictions() int64 {
	return atomic.LoadInt64(&cs.evictions)
}

// GetSets returns the number of set operations
func (cs *CacheStats) GetSets() int64 {
	return atomic.LoadInt64(&cs.sets)
}

// GetDeletes returns the number of delete operations
func (cs *CacheStats) GetDeletes() int64 {
	return atomic.LoadInt64(&cs.deletes)
}

// HitRate returns the cache hit rate (0.0 to 1.0)
func (cs *CacheStats) HitRate() float64 {
	hits := cs.GetHits()
	misses := cs.GetMisses()
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total)
}

// cacheItem represents a cached item with metadata
type cacheItem[K comparable, V any] struct {
	key        K
	value      V
	expiration time.Time
	element    *list.Element
}

// isExpired checks if the item has expired
func (ci *cacheItem[K, V]) isExpired() bool {
	return !ci.expiration.IsZero() && time.Now().After(ci.expiration)
}

// Cache represents a thread-safe cache with TTL and LRU eviction
type Cache[K comparable, V any] struct {
	maxSize int
	items   map[K]*cacheItem[K, V]
	lruList *list.List
	mu      sync.RWMutex
	stats   *CacheStats
}

// NewCache creates a new cache with the specified maximum size
func NewCache[K comparable, V any](maxSize int) *Cache[K, V] {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. maxSizeの妥当性チェック
	// 2. Cache構造体を初期化
	// 3. items mapを初期化
	// 4. LRU listを初期化
	// 5. 統計情報を初期化
	return nil
}

// Get retrieves a value from the cache
func (c *Cache[K, V]) Get(key K) (V, bool) {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 読み取りロックを取得
	// 2. キーが存在するかチェック
	// 3. アイテムが期限切れでないかチェック
	// 4. LRUリストで最近アクセスしたことを記録
	// 5. 統計情報を更新（ヒットまたはミス）
	// 6. 値を返す
	var zero V
	return zero, false
}

// Set stores a value in the cache with optional TTL
func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 書き込みロックを取得
	// 2. 有効期限を計算
	// 3. 既存のアイテムがあればLRUリストから削除
	// 4. 新しいアイテムを作成してLRUリストの先頭に追加
	// 5. itemsマップに追加
	// 6. 容量オーバーの場合は古いアイテムを削除
	// 7. 統計情報を更新
}

// Delete removes a value from the cache
func (c *Cache[K, V]) Delete(key K) bool {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 書き込みロックを取得
	// 2. キーが存在するかチェック
	// 3. LRUリストから削除
	// 4. itemsマップから削除
	// 5. 統計情報を更新
	// 6. 削除が成功したかを返す
	return false
}

// Clear removes all items from the cache
func (c *Cache[K, V]) Clear() {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 書き込みロックを取得
	// 2. itemsマップをクリア
	// 3. LRUリストをクリア
}

// Size returns the current number of items in the cache
func (c *Cache[K, V]) Size() int {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 読み取りロックを取得
	// 2. itemsマップのサイズを返す
	return 0
}

// Stats returns a copy of the current cache statistics
func (c *Cache[K, V]) Stats() CacheStats {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 統計情報のコピーを作成
	// 2. atomic操作を使用して各統計値を読み取り
	return CacheStats{}
}

// CleanupExpired removes all expired items from the cache
func (c *Cache[K, V]) CleanupExpired() int {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 書き込みロックを取得
	// 2. 全てのアイテムをチェック
	// 3. 期限切れのアイテムを削除
	// 4. 削除した数を返す
	return 0
}

// GetOrSet retrieves a value or sets it if not present
func (c *Cache[K, V]) GetOrSet(key K, valueFunc func() (V, time.Duration)) (V, bool) {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. まずGetを試行
	// 2. 見つからない場合はvalueFuncを呼び出し
	// 3. 計算結果をSetしてから返す
	// 4. 既存の値が見つかった場合はそれを返す
	var zero V
	return zero, false
}

// Keys returns all keys currently in the cache
func (c *Cache[K, V]) Keys() []K {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 読み取りロックを取得
	// 2. 全てのキーを収集
	// 3. キーのスライスを返す
	return nil
}

// evictLRU removes the least recently used item
func (c *Cache[K, V]) evictLRU() {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. LRUリストの末尾要素を取得
	// 2. 対応するキーを取得
	// 3. itemsマップから削除
	// 4. LRUリストから削除
	// 5. 統計情報を更新
}

// moveToFront moves an item to the front of the LRU list
func (c *Cache[K, V]) moveToFront(item *cacheItem[K, V]) {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. LRUリスト内でアイテムを先頭に移動
}

// CacheWithCleanup extends Cache with automatic cleanup functionality
type CacheWithCleanup[K comparable, V any] struct {
	*Cache[K, V]
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
	cleanupDone     chan struct{}
}

// NewCacheWithCleanup creates a cache with automatic expired item cleanup
func NewCacheWithCleanup[K comparable, V any](maxSize int, cleanupInterval time.Duration) *CacheWithCleanup[K, V] {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 基本のCacheを作成
	// 2. クリーンアップ用のチャネルを初期化
	// 3. バックグラウンドでクリーンアップGorutineを開始
	return nil
}

// StartCleanup starts the background cleanup goroutine
func (cwc *CacheWithCleanup[K, V]) StartCleanup() {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. Goroutineでクリーンアップループを開始
	// 2. 定期的にCleanupExpiredを呼び出し
	// 3. stopCleanupチャネルで停止を監視
}

// Stop stops the background cleanup goroutine
func (cwc *CacheWithCleanup[K, V]) Stop() {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. stopCleanupチャネルにシグナル送信
	// 2. クリーンアップの完了を待機
}

// LoadingCache extends Cache with loading functionality
type LoadingCache[K comparable, V any] struct {
	*Cache[K, V]
	loader      func(K) (V, error)
	loadingKeys map[K]chan struct{} // キーごとの読み込み完了通知
	loadingMu   sync.Mutex
}

// NewLoadingCache creates a cache that can load missing values
func NewLoadingCache[K comparable, V any](maxSize int, loader func(K) (V, error)) *LoadingCache[K, V] {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 基本のCacheを作成
	// 2. loaderファンクションを設定
	// 3. loadingKeysマップを初期化
	return nil
}

// Load retrieves a value, loading it if not present
func (lc *LoadingCache[K, V]) Load(key K) (V, error) {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. まずキャッシュから取得を試行
	// 2. 見つからない場合はloaderを使用
	// 3. 同じキーで並行してロードしないよう制御
	// 4. ロード結果をキャッシュに保存
	var zero V
	return zero, nil
}

func main() {
	fmt.Println("=== Thread-Safe Cache Demo ===")
	
	// 基本的なキャッシュの使用例
	cache := NewCache[string, int](3)
	
	// データを設定
	cache.Set("key1", 100, 1*time.Minute)
	cache.Set("key2", 200, 1*time.Minute)
	cache.Set("key3", 300, 1*time.Minute)
	
	// データを取得
	if value, found := cache.Get("key1"); found {
		fmt.Printf("key1: %d\n", value)
	}
	
	// 統計情報を表示
	stats := cache.Stats()
	fmt.Printf("Cache stats - Hits: %d, Misses: %d, Hit Rate: %.2f%%\n",
		stats.GetHits(), stats.GetMisses(), stats.HitRate()*100)
	
	// 容量を超えたデータを追加（LRU削除）
	cache.Set("key4", 400, 1*time.Minute)
	
	fmt.Printf("Cache size after adding key4: %d\n", cache.Size())
	
	// TTLの例
	fmt.Println("\n=== TTL Example ===")
	cache.Set("temporary", 999, 1*time.Second)
	
	if value, found := cache.Get("temporary"); found {
		fmt.Printf("Found temporary value: %d\n", value)
	}
	
	time.Sleep(1500 * time.Millisecond)
	
	if _, found := cache.Get("temporary"); !found {
		fmt.Println("Temporary value has expired")
	}
	
	// 自動クリーンアップ付きキャッシュの例
	fmt.Println("\n=== Auto-Cleanup Cache ===")
	cleanupCache := NewCacheWithCleanup[string, string](10, 500*time.Millisecond)
	defer cleanupCache.Stop()
	
	cleanupCache.Set("short-lived", "data", 300*time.Millisecond)
	
	fmt.Println("Waiting for automatic cleanup...")
	time.Sleep(1 * time.Second)
	
	// ローディングキャッシュの例
	fmt.Println("\n=== Loading Cache ===")
	loadingCache := NewLoadingCache[int, string](5, func(id int) (string, error) {
		// データベースからの読み込みをシミュレート
		time.Sleep(100 * time.Millisecond)
		return fmt.Sprintf("loaded-data-%d", id), nil
	})
	
	value, err := loadingCache.Load(123)
	if err == nil {
		fmt.Printf("Loaded value: %s\n", value)
	}
	
	// 2回目のアクセスはキャッシュから高速取得
	start := time.Now()
	value, err = loadingCache.Load(123)
	elapsed := time.Since(start)
	if err == nil {
		fmt.Printf("Cached value (took %v): %s\n", elapsed, value)
	}
}