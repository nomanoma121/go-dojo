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
	if maxSize <= 0 {
		panic("cache size must be positive")
	}
	
	return &Cache[K, V]{
		maxSize: maxSize,
		items:   make(map[K]*cacheItem[K, V]),
		lruList: list.New(),
		stats:   &CacheStats{},
	}
}

// Get retrieves a value from the cache
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	item, exists := c.items[key]
	c.mu.RUnlock()
	
	if !exists {
		atomic.AddInt64(&c.stats.misses, 1)
		var zero V
		return zero, false
	}
	
	// Check expiration
	if item.isExpired() {
		c.mu.Lock()
		// Double-check after acquiring write lock
		if item, exists := c.items[key]; exists && item.isExpired() {
			c.lruList.Remove(item.element)
			delete(c.items, key)
		}
		c.mu.Unlock()
		
		atomic.AddInt64(&c.stats.misses, 1)
		var zero V
		return zero, false
	}
	
	// Move to front (most recently used)
	c.mu.Lock()
	// Double-check the item still exists
	if item, exists := c.items[key]; exists && !item.isExpired() {
		c.moveToFront(item)
		c.mu.Unlock()
		
		atomic.AddInt64(&c.stats.hits, 1)
		return item.value, true
	}
	c.mu.Unlock()
	
	atomic.AddInt64(&c.stats.misses, 1)
	var zero V
	return zero, false
}

// Set stores a value in the cache with optional TTL
func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	var expiration time.Time
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}
	
	// Check if item already exists
	if existingItem, exists := c.items[key]; exists {
		// Update existing item
		existingItem.value = value
		existingItem.expiration = expiration
		c.moveToFront(existingItem)
	} else {
		// Create new item
		element := c.lruList.PushFront(key)
		item := &cacheItem[K, V]{
			key:        key,
			value:      value,
			expiration: expiration,
			element:    element,
		}
		c.items[key] = item
		
		// Check if we need to evict
		if c.lruList.Len() > c.maxSize {
			c.evictLRU()
		}
	}
	
	atomic.AddInt64(&c.stats.sets, 1)
}

// Delete removes a value from the cache
func (c *Cache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	item, exists := c.items[key]
	if !exists {
		return false
	}
	
	c.lruList.Remove(item.element)
	delete(c.items, key)
	
	atomic.AddInt64(&c.stats.deletes, 1)
	return true
}

// Clear removes all items from the cache
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items = make(map[K]*cacheItem[K, V])
	c.lruList.Init()
}

// Size returns the current number of items in the cache
func (c *Cache[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return len(c.items)
}

// Stats returns a copy of the current cache statistics
func (c *Cache[K, V]) Stats() CacheStats {
	return CacheStats{
		hits:      atomic.LoadInt64(&c.stats.hits),
		misses:    atomic.LoadInt64(&c.stats.misses),
		evictions: atomic.LoadInt64(&c.stats.evictions),
		sets:      atomic.LoadInt64(&c.stats.sets),
		deletes:   atomic.LoadInt64(&c.stats.deletes),
	}
}

// CleanupExpired removes all expired items from the cache
func (c *Cache[K, V]) CleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	removed := 0
	for key, item := range c.items {
		if item.isExpired() {
			c.lruList.Remove(item.element)
			delete(c.items, key)
			removed++
		}
	}
	
	return removed
}

// GetOrSet retrieves a value or sets it if not present
func (c *Cache[K, V]) GetOrSet(key K, valueFunc func() (V, time.Duration)) (V, bool) {
	// First try to get
	if value, found := c.Get(key); found {
		return value, true
	}
	
	// Generate value and TTL
	value, ttl := valueFunc()
	
	// Set the value
	c.Set(key, value, ttl)
	
	return value, false
}

// Keys returns all keys currently in the cache
func (c *Cache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	keys := make([]K, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}
	
	return keys
}

// evictLRU removes the least recently used item
func (c *Cache[K, V]) evictLRU() {
	// Get the least recently used item (back of the list)
	element := c.lruList.Back()
	if element == nil {
		return
	}
	
	key := element.Value.(K)
	delete(c.items, key)
	c.lruList.Remove(element)
	
	atomic.AddInt64(&c.stats.evictions, 1)
}

// moveToFront moves an item to the front of the LRU list
func (c *Cache[K, V]) moveToFront(item *cacheItem[K, V]) {
	c.lruList.MoveToFront(item.element)
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
	cwc := &CacheWithCleanup[K, V]{
		Cache:           NewCache[K, V](maxSize),
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
	}
	
	go cwc.StartCleanup()
	
	return cwc
}

// StartCleanup starts the background cleanup goroutine
func (cwc *CacheWithCleanup[K, V]) StartCleanup() {
	defer close(cwc.cleanupDone)
	
	ticker := time.NewTicker(cwc.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			cwc.CleanupExpired()
		case <-cwc.stopCleanup:
			return
		}
	}
}

// Stop stops the background cleanup goroutine
func (cwc *CacheWithCleanup[K, V]) Stop() {
	close(cwc.stopCleanup)
	<-cwc.cleanupDone
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
	return &LoadingCache[K, V]{
		Cache:       NewCache[K, V](maxSize),
		loader:      loader,
		loadingKeys: make(map[K]chan struct{}),
	}
}

// Load retrieves a value, loading it if not present
func (lc *LoadingCache[K, V]) Load(key K) (V, error) {
	// First try to get from cache
	if value, found := lc.Get(key); found {
		return value, nil
	}
	
	// Check if another goroutine is already loading this key
	lc.loadingMu.Lock()
	if ch, exists := lc.loadingKeys[key]; exists {
		// Another goroutine is loading, wait for it
		lc.loadingMu.Unlock()
		<-ch
		
		// Try to get from cache again
		if value, found := lc.Get(key); found {
			return value, nil
		}
		
		// If still not found, it means loading failed
		// We need to try loading ourselves
	} else {
		// We are the first to try loading this key
		ch = make(chan struct{})
		lc.loadingKeys[key] = ch
		lc.loadingMu.Unlock()
		
		// Load the value
		value, err := lc.loader(key)
		
		// Clean up loading state
		lc.loadingMu.Lock()
		delete(lc.loadingKeys, key)
		lc.loadingMu.Unlock()
		
		// Notify waiting goroutines
		close(ch)
		
		if err != nil {
			var zero V
			return zero, err
		}
		
		// Store in cache (no TTL for loading cache)
		lc.Set(key, value, 0)
		
		return value, nil
	}
	
	// Fallback: try loading again (should rarely happen)
	value, err := lc.loader(key)
	if err != nil {
		var zero V
		return zero, err
	}
	
	lc.Set(key, value, 0)
	return value, nil
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