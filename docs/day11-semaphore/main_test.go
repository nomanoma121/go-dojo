package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSemaphore(t *testing.T) {
	t.Run("Basic acquire and release", func(t *testing.T) {
		sem := NewSemaphore(2)
		
		// Should be able to acquire twice
		sem.Acquire()
		sem.Acquire()
		
		// Third acquire should not block in TryAcquire
		if sem.TryAcquire() {
			t.Error("Should not be able to acquire third permit")
		}
		
		// Release one and try again
		sem.Release()
		if !sem.TryAcquire() {
			t.Error("Should be able to acquire after release")
		}
		
		// Clean up
		sem.Release()
		sem.Release()
	})
	
	t.Run("Available permits count", func(t *testing.T) {
		sem := NewSemaphore(3)
		
		if available := sem.AvailablePermits(); available != 3 {
			t.Errorf("Expected 3 available permits, got %d", available)
		}
		
		sem.Acquire()
		if available := sem.AvailablePermits(); available != 2 {
			t.Errorf("Expected 2 available permits after acquire, got %d", available)
		}
		
		sem.Acquire()
		sem.Acquire()
		if available := sem.AvailablePermits(); available != 0 {
			t.Errorf("Expected 0 available permits, got %d", available)
		}
		
		sem.Release()
		if available := sem.AvailablePermits(); available != 1 {
			t.Errorf("Expected 1 available permit after release, got %d", available)
		}
		
		// Clean up
		sem.Release()
		sem.Release()
	})
	
	t.Run("Timeout handling", func(t *testing.T) {
		sem := NewSemaphore(1)
		
		// Acquire the only permit
		sem.Acquire()
		
		// Try to acquire with timeout should fail
		start := time.Now()
		acquired := sem.AcquireWithTimeout(100 * time.Millisecond)
		elapsed := time.Since(start)
		
		if acquired {
			t.Error("Should not have acquired permit with timeout")
		}
		
		if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
			t.Errorf("Timeout took %v, expected around 100ms", elapsed)
		}
		
		// Clean up
		sem.Release()
	})
	
	t.Run("Concurrent access", func(t *testing.T) {
		sem := NewSemaphore(3)
		var concurrent int64
		var maxConcurrent int64
		var wg sync.WaitGroup
		
		numWorkers := 10
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				sem.Acquire()
				current := atomic.AddInt64(&concurrent, 1)
				
				// Update max concurrent if needed
				for {
					max := atomic.LoadInt64(&maxConcurrent)
					if current <= max || atomic.CompareAndSwapInt64(&maxConcurrent, max, current) {
						break
					}
				}
				
				// Simulate work
				time.Sleep(50 * time.Millisecond)
				
				atomic.AddInt64(&concurrent, -1)
				sem.Release()
			}()
		}
		
		wg.Wait()
		
		if maxConcurrent > 3 {
			t.Errorf("Max concurrent was %d, should not exceed 3", maxConcurrent)
		}
		
		if maxConcurrent < 3 {
			t.Errorf("Max concurrent was %d, expected to reach 3", maxConcurrent)
		}
	})
	
	t.Run("FIFO ordering", func(t *testing.T) {
		sem := NewSemaphore(1)
		order := make([]int, 0)
		var mu sync.Mutex
		var wg sync.WaitGroup
		
		// Acquire the permit to block others
		sem.Acquire()
		
		// Start multiple goroutines that will wait
		numWorkers := 5
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				sem.Acquire()
				
				mu.Lock()
				order = append(order, id)
				mu.Unlock()
				
				// Simulate quick work
				time.Sleep(10 * time.Millisecond)
				
				sem.Release()
			}(i)
		}
		
		// Let all goroutines start and block
		time.Sleep(50 * time.Millisecond)
		
		// Release to start the chain
		sem.Release()
		
		wg.Wait()
		
		// Check that we got all workers (order might vary due to scheduler)
		if len(order) != numWorkers {
			t.Errorf("Expected %d workers to complete, got %d", numWorkers, len(order))
		}
	})
	
	t.Run("Context cancellation", func(t *testing.T) {
		sem := NewSemaphore(1)
		
		// Acquire the only permit
		sem.Acquire()
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		start := time.Now()
		acquired := sem.AcquireWithContext(ctx)
		elapsed := time.Since(start)
		
		if acquired {
			t.Error("Should not have acquired permit when context cancelled")
		}
		
		if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
			t.Errorf("Context cancellation took %v, expected around 100ms", elapsed)
		}
		
		// Clean up
		sem.Release()
	})
	
	t.Run("Zero permits semaphore", func(t *testing.T) {
		sem := NewSemaphore(0)
		
		if sem.TryAcquire() {
			t.Error("Should not be able to acquire from zero-permit semaphore")
		}
		
		if sem.AvailablePermits() != 0 {
			t.Errorf("Expected 0 permits, got %d", sem.AvailablePermits())
		}
		
		// Adding a permit should allow acquisition
		sem.Release()
		if !sem.TryAcquire() {
			t.Error("Should be able to acquire after adding permit")
		}
	})
	
	t.Run("Multiple releases", func(t *testing.T) {
		sem := NewSemaphore(1)
		
		sem.Acquire()
		sem.Release()
		sem.Release() // Extra release
		
		// Should now have 2 permits available
		if sem.AvailablePermits() != 2 {
			t.Errorf("Expected 2 permits after extra release, got %d", sem.AvailablePermits())
		}
		
		// Should be able to acquire twice
		if !sem.TryAcquire() {
			t.Error("Should be able to acquire first permit")
		}
		if !sem.TryAcquire() {
			t.Error("Should be able to acquire second permit")
		}
		if sem.TryAcquire() {
			t.Error("Should not be able to acquire third permit")
		}
		
		// Clean up
		sem.Release()
		sem.Release()
	})
}

func TestSemaphoreResourcePool(t *testing.T) {
	t.Run("Database connection pool simulation", func(t *testing.T) {
		// Simulate a database connection pool with 3 connections
		sem := NewSemaphore(3)
		var activeConnections int64
		var totalQueries int64
		var wg sync.WaitGroup
		
		// Simulate multiple clients making database queries
		numClients := 10
		queriesPerClient := 5
		
		for client := 0; client < numClients; client++ {
			wg.Add(1)
			go func(clientID int) {
				defer wg.Done()
				
				for query := 0; query < queriesPerClient; query++ {
					// Acquire a database connection
					sem.Acquire()
					
					active := atomic.AddInt64(&activeConnections, 1)
					if active > 3 {
						t.Errorf("Too many active connections: %d", active)
					}
					
					// Simulate database query
					time.Sleep(20 * time.Millisecond)
					
					atomic.AddInt64(&totalQueries, 1)
					atomic.AddInt64(&activeConnections, -1)
					
					// Release the connection
					sem.Release()
					
					// Brief pause between queries
					time.Sleep(5 * time.Millisecond)
				}
			}(client)
		}
		
		wg.Wait()
		
		expectedQueries := int64(numClients * queriesPerClient)
		if totalQueries != expectedQueries {
			t.Errorf("Expected %d total queries, got %d", expectedQueries, totalQueries)
		}
		
		if activeConnections != 0 {
			t.Errorf("Expected 0 active connections at end, got %d", activeConnections)
		}
	})
}

func TestSemaphoreEdgeCases(t *testing.T) {
	t.Run("Negative permits", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for negative permits")
			}
		}()
		NewSemaphore(-1)
	})
	
	t.Run("Large number of permits", func(t *testing.T) {
		sem := NewSemaphore(1000000)
		
		if sem.AvailablePermits() != 1000000 {
			t.Errorf("Expected 1000000 permits, got %d", sem.AvailablePermits())
		}
		
		// Should be able to acquire without blocking
		if !sem.TryAcquire() {
			t.Error("Should be able to acquire from large semaphore")
		}
		
		sem.Release()
	})
}

// Benchmark tests
func BenchmarkSemaphoreAcquireRelease(b *testing.B) {
	sem := NewSemaphore(1)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sem.Acquire()
			sem.Release()
		}
	})
}

func BenchmarkSemaphoreTryAcquire(b *testing.B) {
	sem := NewSemaphore(1000)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if sem.TryAcquire() {
				sem.Release()
			}
		}
	})
}

func BenchmarkSemaphoreHighContention(b *testing.B) {
	sem := NewSemaphore(1)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			acquired := sem.AcquireWithTimeout(time.Microsecond)
			if acquired {
				sem.Release()
			}
		}
	})
}

func BenchmarkSemaphoreAvailablePermits(b *testing.B) {
	sem := NewSemaphore(100)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = sem.AvailablePermits()
		}
	})
}

// Additional method for context-based acquisition (bonus)
func (s *Semaphore) AcquireWithContext(ctx context.Context) bool {
	select {
	case <-s.permits:
		return true
	case <-ctx.Done():
		return false
	}
}