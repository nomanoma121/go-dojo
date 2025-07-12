package main

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBasicCacheOperations(t *testing.T) {
	t.Run("Set and Get", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("key1", 100, 0) // No TTL
		
		value, found := cache.Get("key1")
		if !found {
			t.Error("Expected to find key1")
		}
		if value != 100 {
			t.Errorf("Expected 100, got %d", value)
		}
	})
	
	t.Run("Get non-existent key", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		value, found := cache.Get("nonexistent")
		if found {
			t.Error("Should not find non-existent key")
		}
		if value != 0 {
			t.Errorf("Expected zero value, got %d", value)
		}
	})
	
	t.Run("Delete", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("key1", 100, 0)
		deleted := cache.Delete("key1")
		if !deleted {
			t.Error("Expected deletion to succeed")
		}
		
		_, found := cache.Get("key1")
		if found {
			t.Error("Key should be deleted")
		}
		
		// Delete non-existent key
		deleted = cache.Delete("nonexistent")
		if deleted {
			t.Error("Deleting non-existent key should return false")
		}
	})
	
	t.Run("Clear", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("key1", 100, 0)
		cache.Set("key2", 200, 0)
		cache.Set("key3", 300, 0)
		
		if cache.Size() != 3 {
			t.Errorf("Expected size 3, got %d", cache.Size())
		}
		
		cache.Clear()
		
		if cache.Size() != 0 {
			t.Errorf("Expected size 0 after clear, got %d", cache.Size())
		}
		
		_, found := cache.Get("key1")
		if found {
			t.Error("Cache should be empty after clear")
		}
	})
	
	t.Run("Size tracking", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		if cache.Size() != 0 {
			t.Error("Empty cache should have size 0")
		}
		
		cache.Set("key1", 100, 0)
		if cache.Size() != 1 {
			t.Errorf("Expected size 1, got %d", cache.Size())
		}
		
		cache.Set("key2", 200, 0)
		if cache.Size() != 2 {
			t.Errorf("Expected size 2, got %d", cache.Size())
		}
		
		cache.Delete("key1")
		if cache.Size() != 1 {
			t.Errorf("Expected size 1 after delete, got %d", cache.Size())
		}
	})
}

func TestCacheTTL(t *testing.T) {
	t.Run("TTL expiration", func(t *testing.T) {
		cache := NewCache[string, string](10)
		
		cache.Set("temporary", "value", 100*time.Millisecond)
		
		// Should be available immediately
		value, found := cache.Get("temporary")
		if !found || value != "value" {
			t.Error("Value should be available before expiration")
		}
		
		// Wait for expiration
		time.Sleep(150 * time.Millisecond)
		
		value, found = cache.Get("temporary")
		if found {
			t.Error("Value should be expired")
		}
	})
	
	t.Run("No TTL means no expiration", func(t *testing.T) {
		cache := NewCache[string, string](10)
		
		cache.Set("permanent", "value", 0) // No TTL
		
		time.Sleep(100 * time.Millisecond)
		
		value, found := cache.Get("permanent")
		if !found || value != "value" {
			t.Error("Value with no TTL should not expire")
		}
	})
	
	t.Run("Cleanup expired items", func(t *testing.T) {
		cache := NewCache[string, string](10)
		
		cache.Set("temp1", "value1", 50*time.Millisecond)
		cache.Set("temp2", "value2", 50*time.Millisecond)
		cache.Set("permanent", "value3", 0)
		
		if cache.Size() != 3 {
			t.Error("Should have 3 items initially")
		}
		
		time.Sleep(100 * time.Millisecond)
		
		removed := cache.CleanupExpired()
		if removed != 2 {
			t.Errorf("Expected to remove 2 expired items, got %d", removed)
		}
		
		if cache.Size() != 1 {
			t.Errorf("Expected 1 item remaining, got %d", cache.Size())
		}
		
		_, found := cache.Get("permanent")
		if !found {
			t.Error("Permanent item should still exist")
		}
	})
}

func TestCacheLRU(t *testing.T) {
	t.Run("LRU eviction", func(t *testing.T) {
		cache := NewCache[string, int](3) // Small capacity
		
		cache.Set("key1", 1, 0)
		cache.Set("key2", 2, 0)
		cache.Set("key3", 3, 0)
		
		// All should be present
		if cache.Size() != 3 {
			t.Error("Should have 3 items")
		}
		
		// Adding 4th item should evict least recently used (key1)
		cache.Set("key4", 4, 0)
		
		if cache.Size() != 3 {
			t.Error("Should still have 3 items after eviction")
		}
		
		_, found := cache.Get("key1")
		if found {
			t.Error("key1 should have been evicted")
		}
		
		// key2, key3, key4 should still exist
		for _, key := range []string{"key2", "key3", "key4"} {
			if _, found := cache.Get(key); !found {
				t.Errorf("Key %s should still exist", key)
			}
		}
	})
	
	t.Run("LRU order with access", func(t *testing.T) {
		cache := NewCache[string, int](3)
		
		cache.Set("key1", 1, 0)
		cache.Set("key2", 2, 0)
		cache.Set("key3", 3, 0)
		
		// Access key1 to make it most recently used
		cache.Get("key1")
		
		// Add key4, should evict key2 (not key1)
		cache.Set("key4", 4, 0)
		
		_, found := cache.Get("key2")
		if found {
			t.Error("key2 should have been evicted")
		}
		
		_, found = cache.Get("key1")
		if !found {
			t.Error("key1 should still exist after being accessed")
		}
	})
	
	t.Run("Update existing key maintains LRU order", func(t *testing.T) {
		cache := NewCache[string, int](3)
		
		cache.Set("key1", 1, 0)
		cache.Set("key2", 2, 0)
		cache.Set("key3", 3, 0)
		
		// Update key1 with new value
		cache.Set("key1", 10, 0)
		
		// Add key4, should evict key2 (oldest)
		cache.Set("key4", 4, 0)
		
		value, found := cache.Get("key1")
		if !found || value != 10 {
			t.Error("key1 should exist with updated value")
		}
		
		_, found = cache.Get("key2")
		if found {
			t.Error("key2 should have been evicted")
		}
	})
}

func TestCacheStats(t *testing.T) {
	t.Run("Hit and miss tracking", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		// Miss
		cache.Get("nonexistent")
		
		stats := cache.Stats()
		if stats.GetHits() != 0 {
			t.Error("Should have 0 hits")
		}
		if stats.GetMisses() != 1 {
			t.Error("Should have 1 miss")
		}
		
		// Set and hit
		cache.Set("key1", 100, 0)
		cache.Get("key1")
		
		stats = cache.Stats()
		if stats.GetHits() != 1 {
			t.Error("Should have 1 hit")
		}
		if stats.GetMisses() != 1 {
			t.Error("Should still have 1 miss")
		}
	})
	
	t.Run("Hit rate calculation", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		// 0 operations = 0% hit rate
		stats := cache.Stats()
		if stats.HitRate() != 0.0 {
			t.Error("Hit rate should be 0.0 with no operations")
		}
		
		cache.Set("key1", 100, 0)
		
		// 1 hit, 0 misses = 100%
		cache.Get("key1")
		stats = cache.Stats()
		if stats.HitRate() != 1.0 {
			t.Errorf("Expected hit rate 1.0, got %f", stats.HitRate())
		}
		
		// 1 hit, 1 miss = 50%
		cache.Get("nonexistent")
		stats = cache.Stats()
		if stats.HitRate() != 0.5 {
			t.Errorf("Expected hit rate 0.5, got %f", stats.HitRate())
		}
	})
	
	t.Run("Set and delete tracking", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("key1", 100, 0)
		cache.Set("key2", 200, 0)
		
		stats := cache.Stats()
		if stats.GetSets() != 2 {
			t.Errorf("Expected 2 sets, got %d", stats.GetSets())
		}
		
		cache.Delete("key1")
		
		stats = cache.Stats()
		if stats.GetDeletes() != 1 {
			t.Errorf("Expected 1 delete, got %d", stats.GetDeletes())
		}
	})
	
	t.Run("Eviction tracking", func(t *testing.T) {
		cache := NewCache[string, int](2) // Small capacity
		
		cache.Set("key1", 1, 0)
		cache.Set("key2", 2, 0)
		cache.Set("key3", 3, 0) // Should trigger eviction
		
		stats := cache.Stats()
		if stats.GetEvictions() != 1 {
			t.Errorf("Expected 1 eviction, got %d", stats.GetEvictions())
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("Concurrent reads and writes", func(t *testing.T) {
		cache := NewCache[int, string](1000)
		
		var wg sync.WaitGroup
		numGoroutines := 50
		numOperations := 100
		
		// Writers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := id*numOperations + j
					cache.Set(key, fmt.Sprintf("value-%d", key), 0)
				}
			}(i)
		}
		
		// Readers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := id*numOperations + j
					cache.Get(key)
				}
			}(i)
		}
		
		wg.Wait()
		
		// Verify cache state is consistent
		if cache.Size() != numGoroutines*numOperations {
			t.Errorf("Expected cache size %d, got %d", numGoroutines*numOperations, cache.Size())
		}
	})
	
	t.Run("Race condition in stats", func(t *testing.T) {
		cache := NewCache[int, int](1000)
		
		var wg sync.WaitGroup
		numGoroutines := 100
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Mix of operations
				cache.Set(id, id*2, 0)
				cache.Get(id)
				cache.Get(id + 10000) // Miss
				cache.Delete(id)
			}(i)
		}
		
		wg.Wait()
		
		stats := cache.Stats()
		
		// Verify consistency
		if stats.GetSets() != int64(numGoroutines) {
			t.Errorf("Expected %d sets, got %d", numGoroutines, stats.GetSets())
		}
		
		totalAccess := stats.GetHits() + stats.GetMisses()
		expectedAccess := int64(numGoroutines * 2) // 1 hit + 1 miss per goroutine
		if totalAccess != expectedAccess {
			t.Errorf("Expected %d total access, got %d", expectedAccess, totalAccess)
		}
	})
}

func TestAdvancedFeatures(t *testing.T) {
	t.Run("GetOrSet", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		callCount := 0
		valueFunc := func() (int, time.Duration) {
			callCount++
			return 42, 1 * time.Minute
		}
		
		// First call should invoke function
		value, found := cache.GetOrSet("key1", valueFunc)
		if found {
			t.Error("Should not be found on first call")
		}
		if value != 42 {
			t.Errorf("Expected 42, got %d", value)
		}
		if callCount != 1 {
			t.Error("Function should have been called once")
		}
		
		// Second call should use cached value
		value, found = cache.GetOrSet("key1", valueFunc)
		if !found {
			t.Error("Should be found on second call")
		}
		if value != 42 {
			t.Errorf("Expected 42, got %d", value)
		}
		if callCount != 1 {
			t.Error("Function should not have been called again")
		}
	})
	
	t.Run("Keys", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("key1", 1, 0)
		cache.Set("key2", 2, 0)
		cache.Set("key3", 3, 0)
		
		keys := cache.Keys()
		if len(keys) != 3 {
			t.Errorf("Expected 3 keys, got %d", len(keys))
		}
		
		keyMap := make(map[string]bool)
		for _, key := range keys {
			keyMap[key] = true
		}
		
		for _, expectedKey := range []string{"key1", "key2", "key3"} {
			if !keyMap[expectedKey] {
				t.Errorf("Expected key %s not found", expectedKey)
			}
		}
	})
}

func TestCacheWithCleanup(t *testing.T) {
	t.Run("Automatic cleanup", func(t *testing.T) {
		cache := NewCacheWithCleanup[string, string](10, 100*time.Millisecond)
		defer cache.Stop()
		
		cache.Set("temp1", "value1", 50*time.Millisecond)
		cache.Set("temp2", "value2", 50*time.Millisecond)
		cache.Set("permanent", "value3", 0)
		
		if cache.Size() != 3 {
			t.Error("Should have 3 items initially")
		}
		
		// Wait for cleanup to run
		time.Sleep(200 * time.Millisecond)
		
		if cache.Size() != 1 {
			t.Errorf("Expected 1 item after cleanup, got %d", cache.Size())
		}
		
		_, found := cache.Get("permanent")
		if !found {
			t.Error("Permanent item should still exist")
		}
	})
	
	t.Run("Stop cleanup", func(t *testing.T) {
		cache := NewCacheWithCleanup[string, string](10, 10*time.Millisecond)
		
		// Stop should complete without hanging
		done := make(chan struct{})
		go func() {
			cache.Stop()
			close(done)
		}()
		
		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Error("Stop() took too long")
		}
	})
}

func TestLoadingCache(t *testing.T) {
	t.Run("Load missing value", func(t *testing.T) {
		callCount := 0
		loader := func(key int) (string, error) {
			callCount++
			return fmt.Sprintf("loaded-%d", key), nil
		}
		
		cache := NewLoadingCache[int, string](10, loader)
		
		value, err := cache.Load(123)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if value != "loaded-123" {
			t.Errorf("Expected 'loaded-123', got '%s'", value)
		}
		if callCount != 1 {
			t.Error("Loader should have been called once")
		}
		
		// Second load should use cache
		value, err = cache.Load(123)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if value != "loaded-123" {
			t.Errorf("Expected 'loaded-123', got '%s'", value)
		}
		if callCount != 1 {
			t.Error("Loader should not have been called again")
		}
	})
	
	t.Run("Load with error", func(t *testing.T) {
		expectedError := errors.New("load failed")
		loader := func(key int) (string, error) {
			return "", expectedError
		}
		
		cache := NewLoadingCache[int, string](10, loader)
		
		value, err := cache.Load(123)
		if err != expectedError {
			t.Errorf("Expected specific error, got %v", err)
		}
		if value != "" {
			t.Errorf("Expected empty value on error, got '%s'", value)
		}
	})
	
	t.Run("Concurrent loading of same key", func(t *testing.T) {
		var loadCount int64
		loader := func(key int) (string, error) {
			atomic.AddInt64(&loadCount, 1)
			time.Sleep(100 * time.Millisecond) // Simulate slow load
			return fmt.Sprintf("loaded-%d", key), nil
		}
		
		cache := NewLoadingCache[int, string](10, loader)
		
		var wg sync.WaitGroup
		numGoroutines := 10
		results := make([]string, numGoroutines)
		
		// All goroutines try to load the same key
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				value, err := cache.Load(123)
				if err == nil {
					results[idx] = value
				}
			}(i)
		}
		
		wg.Wait()
		
		// Should only load once despite concurrent requests
		if loadCount != 1 {
			t.Errorf("Expected loader to be called once, was called %d times", loadCount)
		}
		
		// All results should be the same
		for i, result := range results {
			if result != "loaded-123" {
				t.Errorf("Result %d: expected 'loaded-123', got '%s'", i, result)
			}
		}
	})
}

// Benchmark tests
func BenchmarkCacheGet(b *testing.B) {
	cache := NewCache[int, string](1000)
	
	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i), 0)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Get(i % 1000)
			i++
		}
	})
}

func BenchmarkCacheSet(b *testing.B) {
	cache := NewCache[int, string](10000)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(i, fmt.Sprintf("value-%d", i), 0)
			i++
		}
	})
}

func BenchmarkCacheMixed(b *testing.B) {
	cache := NewCache[int, string](1000)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%4 == 0 {
				cache.Set(i, fmt.Sprintf("value-%d", i), 0)
			} else {
				cache.Get(i % 1000)
			}
			i++
		}
	})
}