package main

import (
	"bytes"
	"runtime"
	"sync"
	"testing"
)

func TestBufferPool(t *testing.T) {
	t.Run("Basic operations", func(t *testing.T) {
		pool := NewBufferPool()
		
		// Get buffer from pool
		buf := pool.Get()
		if buf == nil {
			t.Fatal("Got nil buffer from pool")
		}
		
		// Use buffer
		buf.WriteString("test data")
		if buf.String() != "test data" {
			t.Errorf("Expected 'test data', got '%s'", buf.String())
		}
		
		// Return to pool
		pool.Put(buf)
		
		// Get again - should be reused
		buf2 := pool.Get()
		if buf2 == nil {
			t.Fatal("Got nil buffer from pool on second get")
		}
		
		// Buffer should be reset
		if buf2.Len() != 0 {
			t.Errorf("Buffer not reset, length: %d", buf2.Len())
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		pool := NewBufferPool()
		const numGoroutines = 100
		const numOperations = 10
		
		var wg sync.WaitGroup
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					buf := pool.Get()
					if buf == nil {
						t.Errorf("Got nil buffer in goroutine %d", id)
						return
					}
					
					buf.WriteString("test")
					pool.Put(buf)
				}
			}(i)
		}
		
		wg.Wait()
	})

	t.Run("Large buffer handling", func(t *testing.T) {
		pool := NewBufferPool()
		
		buf := pool.Get()
		
		// Write large amount of data
		largeData := make([]byte, 1024*1024) // 1MB
		buf.Write(largeData)
		
		// Should handle large buffer appropriately
		pool.Put(buf)
		
		// Next buffer should still work
		buf2 := pool.Get()
		if buf2 == nil {
			t.Fatal("Got nil buffer after large buffer")
		}
		
		if buf2.Len() != 0 {
			t.Error("Buffer not reset after large buffer")
		}
	})
}

func TestWorkerDataPool(t *testing.T) {
	t.Run("Basic operations", func(t *testing.T) {
		pool := NewWorkerDataPool()
		
		// Get WorkerData from pool
		wd := pool.Get()
		if wd == nil {
			t.Fatal("Got nil WorkerData from pool")
		}
		
		// Use WorkerData
		wd.ID = 123
		wd.Payload = []byte("test payload")
		wd.Metadata["key"] = "value"
		wd.Results = append(wd.Results, 1.23, 4.56)
		
		// Verify data
		if wd.ID != 123 {
			t.Errorf("Expected ID 123, got %d", wd.ID)
		}
		
		// Return to pool
		pool.Put(wd)
		
		// Get again - should be reused and reset
		wd2 := pool.Get()
		if wd2 == nil {
			t.Fatal("Got nil WorkerData from pool on second get")
		}
		
		// Should be reset
		if wd2.ID != 0 {
			t.Errorf("WorkerData not reset, ID: %d", wd2.ID)
		}
		if len(wd2.Payload) != 0 {
			t.Errorf("Payload not reset, length: %d", len(wd2.Payload))
		}
		if len(wd2.Metadata) != 0 {
			t.Errorf("Metadata not reset, length: %d", len(wd2.Metadata))
		}
		if len(wd2.Results) != 0 {
			t.Errorf("Results not reset, length: %d", len(wd2.Results))
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		pool := NewWorkerDataPool()
		const numGoroutines = 50
		const numOperations = 20
		
		var wg sync.WaitGroup
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					wd := pool.Get()
					if wd == nil {
						t.Errorf("Got nil WorkerData in goroutine %d", id)
						return
					}
					
					wd.ID = id
					wd.Payload = make([]byte, 10)
					wd.Metadata["goroutine"] = string(rune(id))
					
					pool.Put(wd)
				}
			}(i)
		}
		
		wg.Wait()
	})
}

func TestSlicePool(t *testing.T) {
	t.Run("Basic operations", func(t *testing.T) {
		pool := NewSlicePool()
		
		// Get slice from pool
		slice := pool.GetSlice(64)
		if slice == nil {
			t.Fatal("Got nil slice from pool")
		}
		
		if cap(slice) < 64 {
			t.Errorf("Expected capacity >= 64, got %d", cap(slice))
		}
		
		// Use slice
		copy(slice, []byte("test data"))
		
		// Return to pool
		pool.PutSlice(slice)
		
		// Get again with same capacity
		slice2 := pool.GetSlice(64)
		if slice2 == nil {
			t.Fatal("Got nil slice from pool on second get")
		}
		
		// Slice should be cleared
		allZero := true
		for _, b := range slice2[:10] {
			if b != 0 {
				allZero = false
				break
			}
		}
		if !allZero {
			t.Error("Slice not cleared when returned to pool")
		}
	})

	t.Run("Different capacities", func(t *testing.T) {
		pool := NewSlicePool()
		
		capacities := []int{32, 64, 128, 256, 512}
		
		for _, capacity := range capacities {
			slice := pool.GetSlice(capacity)
			if slice == nil {
				t.Errorf("Got nil slice for capacity %d", capacity)
				continue
			}
			
			if cap(slice) < capacity {
				t.Errorf("Expected capacity >= %d, got %d", capacity, cap(slice))
			}
			
			pool.PutSlice(slice)
		}
	})

	t.Run("Capacity rounding", func(t *testing.T) {
		testCases := []struct {
			input    int
			expected int
		}{
			{10, 32},
			{33, 64},
			{65, 128},
			{129, 256},
		}
		
		for _, tc := range testCases {
			result := roundUpToPowerOf2(tc.input)
			if result < tc.expected {
				t.Errorf("roundUpToPowerOf2(%d) = %d, expected >= %d", tc.input, result, tc.expected)
			}
		}
	})
}

func TestProcessingService(t *testing.T) {
	t.Run("Process data with pools", func(t *testing.T) {
		service := NewProcessingService()
		
		testData := []byte("Hello, World!")
		result, err := service.ProcessData(testData)
		
		if err != nil {
			t.Fatalf("ProcessData failed: %v", err)
		}
		
		if result == "" {
			t.Error("Expected non-empty result")
		}
	})

	t.Run("Process without pools", func(t *testing.T) {
		testData := []byte("Hello, World!")
		result, err := ProcessWithoutPool(testData)
		
		if err != nil {
			t.Fatalf("ProcessWithoutPool failed: %v", err)
		}
		
		if result == "" {
			t.Error("Expected non-empty result")
		}
	})

	t.Run("Concurrent processing", func(t *testing.T) {
		service := NewProcessingService()
		const numGoroutines = 20
		const numOperations = 10
		
		var wg sync.WaitGroup
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					testData := []byte("test data")
					_, err := service.ProcessData(testData)
					if err != nil {
						t.Errorf("ProcessData failed in goroutine %d: %v", id, err)
						return
					}
				}
			}(i)
		}
		
		wg.Wait()
	})
}

// ベンチマークテスト: プール使用時 vs 非使用時の比較
func BenchmarkBufferPoolVsNew(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		pool := NewBufferPool()
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := pool.Get()
				buf.WriteString("benchmark test data")
				pool.Put(buf)
			}
		})
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := &bytes.Buffer{}
				buf.WriteString("benchmark test data")
			}
		})
	})
}

func BenchmarkWorkerDataPoolVsNew(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		pool := NewWorkerDataPool()
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				wd := pool.Get()
				wd.ID = 123
				wd.Payload = make([]byte, 100)
				wd.Metadata["key"] = "value"
				pool.Put(wd)
			}
		})
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				wd := &WorkerData{
					Metadata: make(map[string]string),
					Results:  make([]float64, 0),
				}
				wd.ID = 123
				wd.Payload = make([]byte, 100)
				wd.Metadata["key"] = "value"
			}
		})
	})
}

func BenchmarkSlicePoolVsNew(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		pool := NewSlicePool()
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				slice := pool.GetSlice(256)
				copy(slice, []byte("benchmark test data"))
				pool.PutSlice(slice)
			}
		})
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				slice := make([]byte, 256)
				copy(slice, []byte("benchmark test data"))
			}
		})
	})
}

func BenchmarkProcessingServiceVsWithoutPool(b *testing.B) {
	testData := []byte("Benchmark test data for processing service comparison")
	
	b.Run("WithPools", func(b *testing.B) {
		service := NewProcessingService()
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := service.ProcessData(testData)
				if err != nil {
					b.Fatalf("ProcessData failed: %v", err)
				}
			}
		})
	})

	b.Run("WithoutPools", func(b *testing.B) {
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := ProcessWithoutPool(testData)
				if err != nil {
					b.Fatalf("ProcessWithoutPool failed: %v", err)
				}
			}
		})
	})
}

// メモリ使用量の測定
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		pool := NewBufferPool()
		
		b.ResetTimer()
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		for i := 0; i < b.N; i++ {
			buf := pool.Get()
			buf.WriteString("memory usage test")
			pool.Put(buf)
		}
		
		runtime.GC()
		runtime.ReadMemStats(&m2)
		
		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ResetTimer()
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)
		
		for i := 0; i < b.N; i++ {
			buf := &bytes.Buffer{}
			buf.WriteString("memory usage test")
		}
		
		runtime.GC()
		runtime.ReadMemStats(&m2)
		
		b.ReportMetric(float64(m2.TotalAlloc-m1.TotalAlloc)/float64(b.N), "bytes/op")
	})
}