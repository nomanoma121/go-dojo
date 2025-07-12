package main

import (
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// NewCache creates a new cache with the specified maximum size
func NewCache[K comparable, V any](maxSize int) *Cache[K, V] {
	if maxSize <= 0 {
		panic("maxSize must be positive")
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
	c.mu.Lock()
	defer c.mu.Unlock()
	
	item, exists := c.items[key]
	if !exists {
		atomic.AddInt64(&c.stats.misses, 1)
		var zero V
		return zero, false
	}
	
	if item.isExpired() {
		// Remove expired item
		c.removeItem(item)
		atomic.AddInt64(&c.stats.misses, 1)
		var zero V
		return zero, false
	}
	
	// Move to front (most recently used)
	c.moveToFront(item)
	atomic.AddInt64(&c.stats.hits, 1)
	return item.value, true
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
		
		// Evict if over capacity
		if len(c.items) > c.maxSize {
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
	
	c.removeItem(item)
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
	now := time.Now()
	
	// Collect expired keys
	var expiredKeys []K
	for key, item := range c.items {
		if !item.expiration.IsZero() && now.After(item.expiration) {
			expiredKeys = append(expiredKeys, key)
		}
	}
	
	// Remove expired items
	for _, key := range expiredKeys {
		if item, exists := c.items[key]; exists {
			c.removeItem(item)
			removed++
		}
	}
	
	return removed
}

// GetOrSet retrieves a value or sets it if not present
func (c *Cache[K, V]) GetOrSet(key K, valueFunc func() (V, time.Duration)) (V, bool) {
	// Try to get first
	if value, found := c.Get(key); found {
		return value, true
	}
	
	// Not found, compute and set
	value, ttl := valueFunc()
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
	if c.lruList.Len() == 0 {
		return
	}
	
	// Get the least recently used item (back of list)
	oldest := c.lruList.Back()
	if oldest != nil {
		key := oldest.Value.(K)
		if item, exists := c.items[key]; exists {
			c.removeItem(item)
			atomic.AddInt64(&c.stats.evictions, 1)
		}
	}
}

// moveToFront moves an item to the front of the LRU list
func (c *Cache[K, V]) moveToFront(item *cacheItem[K, V]) {
	c.lruList.MoveToFront(item.element)
}

// removeItem removes an item from both the map and LRU list
func (c *Cache[K, V]) removeItem(item *cacheItem[K, V]) {
	delete(c.items, item.key)
	c.lruList.Remove(item.element)
}

// NewCacheWithCleanup creates a cache with automatic expired item cleanup
func NewCacheWithCleanup[K comparable, V any](maxSize int, cleanupInterval time.Duration) *CacheWithCleanup[K, V] {
	cache := NewCache[K, V](maxSize)
	
	cwc := &CacheWithCleanup[K, V]{
		Cache:           cache,
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
	}
	
	cwc.StartCleanup()
	return cwc
}

// StartCleanup starts the background cleanup goroutine
func (cwc *CacheWithCleanup[K, V]) StartCleanup() {
	go func() {
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
	}()
}

// Stop stops the background cleanup goroutine
func (cwc *CacheWithCleanup[K, V]) Stop() {
	close(cwc.stopCleanup)
	<-cwc.cleanupDone
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
	// Try to get from cache first
	if value, found := lc.Get(key); found {
		return value, nil
	}
	
	// Check if already loading
	lc.loadingMu.Lock()
	if waitChan, isLoading := lc.loadingKeys[key]; isLoading {
		lc.loadingMu.Unlock()
		
		// Wait for the loading to complete
		<-waitChan
		
		// Try to get again (should be loaded now)
		if value, found := lc.Get(key); found {
			return value, nil
		}
		// If still not found, the load must have failed
		var zero V
		return zero, ErrLoadFailed
	}
	
	// Start loading
	waitChan := make(chan struct{})
	lc.loadingKeys[key] = waitChan
	lc.loadingMu.Unlock()
	
	defer func() {
		// Cleanup loading state
		lc.loadingMu.Lock()
		delete(lc.loadingKeys, key)
		close(waitChan)
		lc.loadingMu.Unlock()
	}()
	
	// Load the value
	value, err := lc.loader(key)
	if err != nil {
		var zero V
		return zero, err
	}
	
	// Store in cache (no TTL for loaded values)
	lc.Set(key, value, 0)
	return value, nil
}

// Additional utility types and functions

// Error types
var (
	ErrLoadFailed = fmt.Errorf("failed to load value")
)

// SyncMap wrapper for comparison
type SyncMapCache[K comparable, V any] struct {
	data sync.Map
}

func NewSyncMapCache[K comparable, V any]() *SyncMapCache[K, V] {
	return &SyncMapCache[K, V]{}
}

func (smc *SyncMapCache[K, V]) Get(key K) (V, bool) {
	if value, ok := smc.data.Load(key); ok {
		return value.(V), true
	}
	var zero V
	return zero, false
}

func (smc *SyncMapCache[K, V]) Set(key K, value V) {
	smc.data.Store(key, value)
}

func (smc *SyncMapCache[K, V]) Delete(key K) {
	smc.data.Delete(key)
}

// Batch operations
func (c *Cache[K, V]) SetBatch(items map[K]V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for key, value := range items {
		var expiration time.Time
		if ttl > 0 {
			expiration = time.Now().Add(ttl)
		}
		
		if existingItem, exists := c.items[key]; exists {
			existingItem.value = value
			existingItem.expiration = expiration
			c.moveToFront(existingItem)
		} else {
			element := c.lruList.PushFront(key)
			item := &cacheItem[K, V]{
				key:        key,
				value:      value,
				expiration: expiration,
				element:    element,
			}
			c.items[key] = item
			
			if len(c.items) > c.maxSize {
				c.evictLRU()
			}
		}
		atomic.AddInt64(&c.stats.sets, 1)
	}
}

func (c *Cache[K, V]) GetBatch(keys []K) map[K]V {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	result := make(map[K]V)
	now := time.Now()
	
	for _, key := range keys {
		if item, exists := c.items[key]; exists {
			if item.expiration.IsZero() || now.Before(item.expiration) {
				result[key] = item.value
				c.moveToFront(item)
				atomic.AddInt64(&c.stats.hits, 1)
			} else {
				c.removeItem(item)
				atomic.AddInt64(&c.stats.misses, 1)
			}
		} else {
			atomic.AddInt64(&c.stats.misses, 1)
		}
	}
	
	return result
}

// Cache with write-through functionality
type WriteThroughCache[K comparable, V any] struct {
	*Cache[K, V]
	writer func(K, V) error
}

func NewWriteThroughCache[K comparable, V any](maxSize int, writer func(K, V) error) *WriteThroughCache[K, V] {
	return &WriteThroughCache[K, V]{
		Cache:  NewCache[K, V](maxSize),
		writer: writer,
	}
}

func (wtc *WriteThroughCache[K, V]) Set(key K, value V, ttl time.Duration) error {
	// Write to backing store first
	if err := wtc.writer(key, value); err != nil {
		return err
	}
	
	// Then update cache
	wtc.Cache.Set(key, value, ttl)
	return nil
}