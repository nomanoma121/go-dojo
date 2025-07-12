package main

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	t.Run("Basic rate limiting", func(t *testing.T) {
		limiter := NewRateLimiter(5, 5) // 5 tokens per second, capacity 5
		if limiter == nil {
			t.Fatal("NewRateLimiter returned nil")
		}
		defer limiter.Stop()
		
		// Should initially allow requests up to capacity
		for i := 0; i < 5; i++ {
			if !limiter.Allow() {
				t.Errorf("Request %d should have been allowed", i)
			}
		}
		
		// Next request should be denied (bucket empty)
		if limiter.Allow() {
			t.Error("Request should have been denied when bucket is empty")
		}
	})

	t.Run("Token refill", func(t *testing.T) {
		limiter := NewRateLimiter(10, 3) // 10 tokens/sec, capacity 3
		if limiter == nil {
			t.Fatal("NewRateLimiter returned nil")
		}
		defer limiter.Stop()
		
		// Drain the bucket
		for i := 0; i < 3; i++ {
			limiter.Allow()
		}
		
		// Should be denied
		if limiter.Allow() {
			t.Error("Request should be denied after draining bucket")
		}
		
		// Wait for refill (at 10 tokens/sec, should get 1 token in 100ms)
		time.Sleep(150 * time.Millisecond)
		
		// Should be allowed now
		if !limiter.Allow() {
			t.Error("Request should be allowed after token refill")
		}
	})

	t.Run("Wait functionality", func(t *testing.T) {
		limiter := NewRateLimiter(10, 2) // 10 tokens/sec, capacity 2
		if limiter == nil {
			t.Fatal("NewRateLimiter returned nil")
		}
		defer limiter.Stop()
		
		// Drain the bucket
		limiter.Allow()
		limiter.Allow()
		
		start := time.Now()
		limiter.Wait() // Should wait for next token
		elapsed := time.Since(start)
		
		// Should have waited approximately 100ms (1/10 second)
		if elapsed < 50*time.Millisecond || elapsed > 200*time.Millisecond {
			t.Errorf("Wait took %v, expected around 100ms", elapsed)
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		limiter := NewRateLimiter(100, 10) // High rate for testing
		if limiter == nil {
			t.Fatal("NewRateLimiter returned nil")
		}
		defer limiter.Stop()
		
		const numGoroutines = 20
		const requestsPerGoroutine = 5
		
		var wg sync.WaitGroup
		allowed := make(chan bool, numGoroutines*requestsPerGoroutine)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < requestsPerGoroutine; j++ {
					allowed <- limiter.Allow()
				}
			}()
		}
		
		wg.Wait()
		close(allowed)
		
		allowedCount := 0
		for isAllowed := range allowed {
			if isAllowed {
				allowedCount++
			}
		}
		
		// Should allow at least the initial capacity
		if allowedCount < 10 {
			t.Errorf("Expected at least 10 allowed requests, got %d", allowedCount)
		}
	})

	t.Run("Burst handling", func(t *testing.T) {
		limiter := NewRateLimiter(1, 5) // 1 token/sec, burst of 5
		if limiter == nil {
			t.Fatal("NewRateLimiter returned nil")
		}
		defer limiter.Stop()
		
		// Should handle burst up to capacity
		burstAllowed := 0
		for i := 0; i < 10; i++ {
			if limiter.Allow() {
				burstAllowed++
			}
		}
		
		if burstAllowed != 5 {
			t.Errorf("Expected burst of 5 requests, got %d", burstAllowed)
		}
		
		// Wait for one token to refill
		time.Sleep(1100 * time.Millisecond) // Slightly more than 1 second
		
		// Should allow one more request
		if !limiter.Allow() {
			t.Error("Should allow one request after token refill")
		}
	})

	t.Run("Zero capacity", func(t *testing.T) {
		limiter := NewRateLimiter(10, 0) // No burst capacity
		if limiter == nil {
			t.Fatal("NewRateLimiter returned nil")
		}
		defer limiter.Stop()
		
		// Should not allow any requests with zero capacity
		if limiter.Allow() {
			t.Error("Should not allow requests with zero capacity")
		}
	})

	t.Run("High rate limiting", func(t *testing.T) {
		limiter := NewRateLimiter(1000, 100) // Very high rate
		if limiter == nil {
			t.Fatal("NewRateLimiter returned nil")
		}
		defer limiter.Stop()
		
		// Should allow many requests quickly
		allowed := 0
		for i := 0; i < 100; i++ {
			if limiter.Allow() {
				allowed++
			}
		}
		
		if allowed < 90 { // Allow some variance
			t.Errorf("Expected at least 90 allowed requests with high rate, got %d", allowed)
		}
	})
}

func TestRateLimiterEdgeCases(t *testing.T) {
	t.Run("Negative rate", func(t *testing.T) {
		limiter := NewRateLimiter(-1, 5)
		if limiter != nil {
			limiter.Stop()
			t.Error("Should not create limiter with negative rate")
		}
	})

	t.Run("Negative capacity", func(t *testing.T) {
		limiter := NewRateLimiter(5, -1)
		if limiter != nil {
			limiter.Stop()
			t.Error("Should not create limiter with negative capacity")
		}
	})

	t.Run("Stop multiple times", func(t *testing.T) {
		limiter := NewRateLimiter(5, 5)
		if limiter == nil {
			t.Fatal("NewRateLimiter returned nil")
		}
		
		// Should not panic when stopping multiple times
		limiter.Stop()
		limiter.Stop() // Should be safe to call multiple times
	})
}

// Benchmark tests
func BenchmarkRateLimiterAllow(b *testing.B) {
	limiter := NewRateLimiter(1000000, 1000) // Very high rate for benchmarking
	if limiter == nil {
		b.Fatal("NewRateLimiter returned nil")
	}
	defer limiter.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

func BenchmarkRateLimiterWait(b *testing.B) {
	limiter := NewRateLimiter(1000, 100) // High rate for benchmarking
	if limiter == nil {
		b.Fatal("NewRateLimiter returned nil")
	}
	defer limiter.Stop()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Wait()
	}
}

func BenchmarkConcurrentRateLimiter(b *testing.B) {
	limiter := NewRateLimiter(10000, 1000) // High rate for benchmarking
	if limiter == nil {
		b.Fatal("NewRateLimiter returned nil")
	}
	defer limiter.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}