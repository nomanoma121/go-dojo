package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestMutexCache(t *testing.T) {
	cache := NewMutexCache()
	
	// 基本的な操作のテスト
	t.Run("Basic Operations", func(t *testing.T) {
		// Set and Get
		cache.Set("key1", "value1")
		if val, ok := cache.Get("key1"); !ok || val != "value1" {
			t.Errorf("Expected value1, got %s, exists: %v", val, ok)
		}
		
		// Check length
		if cache.Len() != 1 {
			t.Errorf("Expected length 1, got %d", cache.Len())
		}
		
		// Delete
		cache.Delete("key1")
		if _, ok := cache.Get("key1"); ok {
			t.Error("Expected key1 to be deleted")
		}
		
		if cache.Len() != 0 {
			t.Errorf("Expected length 0 after delete, got %d", cache.Len())
		}
	})
	
	// 並行アクセスのテスト
	t.Run("Concurrent Access", func(t *testing.T) {
		const numGoroutines = 10
		const numOperations = 100
		
		var wg sync.WaitGroup
		
		// 同時書き込み
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("key_%d_%d", id, j)
					value := fmt.Sprintf("value_%d_%d", id, j)
					cache.Set(key, value)
				}
			}(i)
		}
		
		// 同時読み取り
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("key_%d_%d", id, j)
					cache.Get(key)
				}
			}(i)
		}
		
		wg.Wait()
	})
}

func TestRWMutexCache(t *testing.T) {
	cache := NewRWMutexCache()
	
	// 基本的な操作のテスト（MutexCacheと同じ）
	t.Run("Basic Operations", func(t *testing.T) {
		cache.Set("key1", "value1")
		if val, ok := cache.Get("key1"); !ok || val != "value1" {
			t.Errorf("Expected value1, got %s, exists: %v", val, ok)
		}
		
		if cache.Len() != 1 {
			t.Errorf("Expected length 1, got %d", cache.Len())
		}
		
		cache.Delete("key1")
		if _, ok := cache.Get("key1"); ok {
			t.Error("Expected key1 to be deleted")
		}
		
		if cache.Len() != 0 {
			t.Errorf("Expected length 0 after delete, got %d", cache.Len())
		}
	})
	
	// 大量の同時読み取りテスト
	t.Run("Massive Concurrent Reads", func(t *testing.T) {
		// 事前にデータを設定
		for i := 0; i < 100; i++ {
			cache.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
		}
		
		const numReaders = 50
		const numReads = 1000
		
		var wg sync.WaitGroup
		start := time.Now()
		
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numReads; j++ {
					key := fmt.Sprintf("key%d", rand.Intn(100))
					cache.Get(key)
				}
			}()
		}
		
		wg.Wait()
		elapsed := time.Since(start)
		t.Logf("RWMutex massive reads completed in: %v", elapsed)
	})
}

// インターフェースの適合性テスト
func TestCacheInterface(t *testing.T) {
	caches := []Cache{
		NewMutexCache(),
		NewRWMutexCache(),
	}
	
	for i, cache := range caches {
		t.Run(fmt.Sprintf("Cache Implementation %d", i), func(t *testing.T) {
			// インターフェースを通じた操作
			cache.Set("test", "value")
			if val, ok := cache.Get("test"); !ok || val != "value" {
				t.Errorf("Interface test failed: got %s, exists: %v", val, ok)
			}
			
			if cache.Len() != 1 {
				t.Errorf("Interface test failed: expected length 1, got %d", cache.Len())
			}
			
			cache.Delete("test")
			if cache.Len() != 0 {
				t.Errorf("Interface test failed: expected length 0 after delete, got %d", cache.Len())
			}
		})
	}
}

// ベンチマークテスト: 読み取り専用ワークロード
func BenchmarkMutexCacheReadOnly(b *testing.B) {
	cache := NewMutexCache()
	
	// 事前にデータを準備
	for i := 0; i < 1000; i++ {
		cache.Set(strconv.Itoa(i), fmt.Sprintf("value%d", i))
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := strconv.Itoa(rand.Intn(1000))
			cache.Get(key)
		}
	})
}

func BenchmarkRWMutexCacheReadOnly(b *testing.B) {
	cache := NewRWMutexCache()
	
	// 事前にデータを準備
	for i := 0; i < 1000; i++ {
		cache.Set(strconv.Itoa(i), fmt.Sprintf("value%d", i))
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := strconv.Itoa(rand.Intn(1000))
			cache.Get(key)
		}
	})
}

// ベンチマークテスト: 書き込み専用ワークロード
func BenchmarkMutexCacheWriteOnly(b *testing.B) {
	cache := NewMutexCache()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(strconv.Itoa(i), fmt.Sprintf("value%d", i))
			i++
		}
	})
}

func BenchmarkRWMutexCacheWriteOnly(b *testing.B) {
	cache := NewRWMutexCache()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(strconv.Itoa(i), fmt.Sprintf("value%d", i))
			i++
		}
	})
}

// ベンチマークテスト: 混合ワークロード (90% 読み取り, 10% 書き込み)
func BenchmarkMutexCacheMixed(b *testing.B) {
	cache := NewMutexCache()
	
	// 事前にデータを準備
	for i := 0; i < 1000; i++ {
		cache.Set(strconv.Itoa(i), fmt.Sprintf("value%d", i))
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 1000
		for pb.Next() {
			if rand.Intn(10) < 9 { // 90% の確率で読み取り
				key := strconv.Itoa(rand.Intn(1000))
				cache.Get(key)
			} else { // 10% の確率で書き込み
				cache.Set(strconv.Itoa(i), fmt.Sprintf("value%d", i))
				i++
			}
		}
	})
}

func BenchmarkRWMutexCacheMixed(b *testing.B) {
	cache := NewRWMutexCache()
	
	// 事前にデータを準備
	for i := 0; i < 1000; i++ {
		cache.Set(strconv.Itoa(i), fmt.Sprintf("value%d", i))
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 1000
		for pb.Next() {
			if rand.Intn(10) < 9 { // 90% の確率で読み取り
				key := strconv.Itoa(rand.Intn(1000))
				cache.Get(key)
			} else { // 10% の確率で書き込み
				cache.Set(strconv.Itoa(i), fmt.Sprintf("value%d", i))
				i++
			}
		}
	})
}

// パフォーマンス比較テスト
func TestPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance comparison in short mode")
	}
	
	scenarios := []struct {
		name          string
		readers       int
		writers       int
		operations    int
	}{
		{"Read Heavy", 10, 1, 10000},
		{"Write Heavy", 1, 10, 1000},
		{"Balanced", 5, 5, 5000},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Run("Mutex", func(t *testing.T) {
				cache := NewMutexCache()
				start := time.Now()
				ConcurrentReadWrite(cache, scenario.readers, scenario.writers, scenario.operations)
				elapsed := time.Since(start)
				t.Logf("Mutex %s: %v", scenario.name, elapsed)
			})
			
			t.Run("RWMutex", func(t *testing.T) {
				cache := NewRWMutexCache()
				start := time.Now()
				ConcurrentReadWrite(cache, scenario.readers, scenario.writers, scenario.operations)
				elapsed := time.Since(start)
				t.Logf("RWMutex %s: %v", scenario.name, elapsed)
			})
		})
	}
}