package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheBasicOperations(t *testing.T) {
	t.Run("Set and Get", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("key1", 100, time.Hour)
		
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
		
		_, found := cache.Get("nonexistent")
		if found {
			t.Error("Expected not to find nonexistent key")
		}
	})
	
	t.Run("Delete key", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("key1", 100, time.Hour)
		deleted := cache.Delete("key1")
		
		if !deleted {
			t.Error("Expected deletion to succeed")
		}
		
		_, found := cache.Get("key1")
		if found {
			t.Error("Expected key to be deleted")
		}
	})
	
	t.Run("Size tracking", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		if cache.Size() != 0 {
			t.Error("Expected initial size to be 0")
		}
		
		cache.Set("key1", 100, time.Hour)
		cache.Set("key2", 200, time.Hour)
		
		if cache.Size() != 2 {
			t.Errorf("Expected size 2, got %d", cache.Size())
		}
	})
}

func TestCacheTTL(t *testing.T) {
	t.Run("TTL expiration", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("short", 100, 50*time.Millisecond)
		
		// Should be found immediately
		_, found := cache.Get("short")
		if !found {
			t.Error("Expected to find key immediately")
		}
		
		// Wait for expiration
		time.Sleep(100 * time.Millisecond)
		
		_, found = cache.Get("short")
		if found {
			t.Error("Expected key to be expired")
		}
	})
	
	t.Run("No TTL (permanent)", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("permanent", 100, 0) // 0 duration = no expiration
		
		time.Sleep(50 * time.Millisecond)
		
		_, found := cache.Get("permanent")
		if !found {
			t.Error("Expected permanent key to still exist")
		}
	})
}

func TestCacheLRU(t *testing.T) {
	t.Run("LRU eviction", func(t *testing.T) {
		cache := NewCache[string, int](2) // Small cache
		
		cache.Set("key1", 100, time.Hour)
		cache.Set("key2", 200, time.Hour)
		cache.Set("key3", 300, time.Hour) // Should evict key1
		
		_, found := cache.Get("key1")
		if found {
			t.Error("Expected key1 to be evicted")
		}
		
		_, found = cache.Get("key2")
		if !found {
			t.Error("Expected key2 to still exist")
		}
		
		_, found = cache.Get("key3")
		if !found {
			t.Error("Expected key3 to exist")
		}
	})
	
	t.Run("LRU update on access", func(t *testing.T) {
		cache := NewCache[string, int](2)
		
		cache.Set("key1", 100, time.Hour)
		cache.Set("key2", 200, time.Hour)
		
		// Access key1 to make it most recent
		cache.Get("key1")
		
		// Add key3, should evict key2 (least recent)
		cache.Set("key3", 300, time.Hour)
		
		_, found := cache.Get("key1")
		if !found {
			t.Error("Expected key1 to still exist (was accessed)")
		}
		
		_, found = cache.Get("key2")
		if found {
			t.Error("Expected key2 to be evicted")
		}
	})
}

func TestCacheStats(t *testing.T) {
	t.Run("Hit and miss counting", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		// Generate some hits and misses
		cache.Set("key1", 100, time.Hour)
		
		cache.Get("key1")    // hit
		cache.Get("key1")    // hit
		cache.Get("missing") // miss
		cache.Get("missing") // miss
		
		stats := cache.Stats()
		
		if stats.GetHits() != 2 {
			t.Errorf("Expected 2 hits, got %d", stats.GetHits())
		}
		
		if stats.GetMisses() != 2 {
			t.Errorf("Expected 2 misses, got %d", stats.GetMisses())
		}
		
		expectedHitRate := 0.5
		if hitRate := stats.HitRate(); hitRate != expectedHitRate {
			t.Errorf("Expected hit rate %.2f, got %.2f", expectedHitRate, hitRate)
		}
	})
}

func TestCacheConcurrency(t *testing.T) {
	t.Run("Concurrent read/write", func(t *testing.T) {
		cache := NewCache[int, string](100)
		var wg sync.WaitGroup
		
		// Writers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					key := id*100 + j
					cache.Set(key, fmt.Sprintf("value-%d", key), time.Hour)
				}
			}(i)
		}
		
		// Readers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 200; j++ {
					cache.Get(j)
				}
			}()
		}
		
		wg.Wait()
		
		// Verify some data exists
		if cache.Size() == 0 {
			t.Error("Expected cache to have some items after concurrent operations")
		}
	})
	
	t.Run("Race condition protection", func(t *testing.T) {
		cache := NewCache[int, int](10)
		var counter int64
		var wg sync.WaitGroup
		
		// Multiple goroutines incrementing counter through cache
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				val := atomic.AddInt64(&counter, 1)
				cache.Set(1, int(val), time.Hour)
				
				stored, found := cache.Get(1)
				if found && stored > 0 {
					// Value should be positive
				}
			}()
		}
		
		wg.Wait()
	})
}

func TestCacheCleanup(t *testing.T) {
	t.Run("Manual cleanup", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("expired1", 100, 10*time.Millisecond)
		cache.Set("expired2", 200, 10*time.Millisecond)
		cache.Set("valid", 300, time.Hour)
		
		time.Sleep(50 * time.Millisecond) // Let items expire
		
		cleaned := cache.CleanupExpired()
		if cleaned != 2 {
			t.Errorf("Expected to clean 2 items, cleaned %d", cleaned)
		}
		
		if cache.Size() != 1 {
			t.Errorf("Expected size 1 after cleanup, got %d", cache.Size())
		}
	})
}

func TestCacheAdvanced(t *testing.T) {
	t.Run("GetOrSet", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		// First call should set the value
		value, wasLoaded := cache.GetOrSet("key1", func() (int, time.Duration) {
			return 42, time.Hour
		})
		
		if value != 42 {
			t.Errorf("Expected 42, got %d", value)
		}
		if wasLoaded {
			t.Error("Expected value to be computed, not loaded from cache")
		}
		
		// Second call should get from cache
		value, wasLoaded = cache.GetOrSet("key1", func() (int, time.Duration) {
			return 99, time.Hour // Should not be used
		})
		
		if value != 42 {
			t.Errorf("Expected cached value 42, got %d", value)
		}
		if !wasLoaded {
			t.Error("Expected value to be loaded from cache")
		}
	})
	
	t.Run("Keys enumeration", func(t *testing.T) {
		cache := NewCache[string, int](10)
		
		cache.Set("key1", 100, time.Hour)
		cache.Set("key2", 200, time.Hour)
		cache.Set("key3", 300, time.Hour)
		
		keys := cache.Keys()
		if len(keys) != 3 {
			t.Errorf("Expected 3 keys, got %d", len(keys))
		}
		
		keySet := make(map[string]bool)
		for _, key := range keys {
			keySet[key] = true
		}
		
		expectedKeys := []string{"key1", "key2", "key3"}
		for _, expected := range expectedKeys {
			if !keySet[expected] {
				t.Errorf("Expected to find key %s", expected)
			}
		}
	})
}

func TestCacheWithCleanup(t *testing.T) {
	t.Run("Automatic cleanup", func(t *testing.T) {
		cache := NewCacheWithCleanup[string, int](10, 50*time.Millisecond)
		defer cache.Stop()
		
		cache.Set("short1", 100, 25*time.Millisecond)
		cache.Set("short2", 200, 25*time.Millisecond)
		cache.Set("long", 300, time.Hour)
		
		if cache.Size() != 3 {
			t.Errorf("Expected initial size 3, got %d", cache.Size())
		}
		
		// Wait for cleanup to run
		time.Sleep(100 * time.Millisecond)
		
		if cache.Size() != 1 {
			t.Errorf("Expected size 1 after cleanup, got %d", cache.Size())
		}
		
		_, found := cache.Get("long")
		if !found {
			t.Error("Expected long-lived item to remain")
		}
	})
}

func TestLoadingCache(t *testing.T) {
	t.Run("Automatic loading", func(t *testing.T) {
		loadCount := 0
		cache := NewLoadingCache[int, string](10, func(key int) (string, error) {
			loadCount++
			return fmt.Sprintf("loaded-%d", key), nil
		})
		
		// First access should trigger load
		value, err := cache.Load(42)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if value != "loaded-42" {
			t.Errorf("Expected 'loaded-42', got %s", value)
		}
		if loadCount != 1 {
			t.Errorf("Expected 1 load, got %d", loadCount)
		}
		
		// Second access should use cache
		value, err = cache.Load(42)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if value != "loaded-42" {
			t.Errorf("Expected 'loaded-42', got %s", value)
		}
		if loadCount != 1 {
			t.Errorf("Expected still 1 load, got %d", loadCount)
		}
	})
	
	t.Run("Concurrent loading", func(t *testing.T) {
		var loadCount int64
		cache := NewLoadingCache[int, string](10, func(key int) (string, error) {
			atomic.AddInt64(&loadCount, 1)
			time.Sleep(50 * time.Millisecond) // Simulate slow load
			return fmt.Sprintf("loaded-%d", key), nil
		})
		
		var wg sync.WaitGroup
		results := make([]string, 5)
		
		// Multiple goroutines try to load the same key
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				value, err := cache.Load(1)
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				results[idx] = value
			}(i)
		}
		
		wg.Wait()
		
		// Should only load once despite concurrent access
		if loadCount != 1 {
			t.Errorf("Expected 1 load for concurrent access, got %d", loadCount)
		}
		
		// All results should be the same
		for i, result := range results {
			if result != "loaded-1" {
				t.Errorf("Result %d: expected 'loaded-1', got %s", i, result)
			}
		}
	})
}

// Benchmark tests
func BenchmarkCacheGet(b *testing.B) {
	cache := NewCache[int, string](1000)
	
	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i), time.Hour)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Get(42)
		}
	})
}

func BenchmarkCacheSet(b *testing.B) {
	cache := NewCache[int, string](10000)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(i, fmt.Sprintf("value-%d", i), time.Hour)
			i++
		}
	})
}

func BenchmarkCacheMixed(b *testing.B) {
	cache := NewCache[int, string](1000)
	
	// Pre-populate
	for i := 0; i < 500; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i), time.Hour)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				cache.Set(i, fmt.Sprintf("value-%d", i), time.Hour)
			} else {
				cache.Get(i % 500)
			}
			i++
		}
	})
}