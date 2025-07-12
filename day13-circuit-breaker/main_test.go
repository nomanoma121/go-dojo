package main

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("Initial state is closed", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 5*time.Second)
		
		if cb.State() != StateClosed {
			t.Errorf("Expected initial state to be Closed, got %v", cb.State())
		}
	})
	
	t.Run("Successful calls keep circuit closed", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 5*time.Second)
		
		for i := 0; i < 10; i++ {
			result, err := cb.Call(func() (interface{}, error) {
				return "success", nil
			})
			
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if result != "success" {
				t.Errorf("Expected 'success', got %v", result)
			}
			if cb.State() != StateClosed {
				t.Errorf("Expected state to remain Closed, got %v", cb.State())
			}
		}
	})
	
	t.Run("Circuit opens after max failures", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 5*time.Second)
		
		// Cause failures to reach the threshold
		for i := 0; i < 3; i++ {
			_, err := cb.Call(func() (interface{}, error) {
				return nil, errors.New("service error")
			})
			
			if err == nil {
				t.Error("Expected error from failing function")
			}
		}
		
		// Circuit should now be open
		if cb.State() != StateOpen {
			t.Errorf("Expected state to be Open after %d failures, got %v", 3, cb.State())
		}
	})
	
	t.Run("Open circuit rejects calls immediately", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 5*time.Second)
		
		// Trip the circuit
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		// Verify circuit is open
		if cb.State() != StateOpen {
			t.Fatal("Circuit should be open")
		}
		
		// Calls should be rejected immediately
		callCount := 0
		result, err := cb.Call(func() (interface{}, error) {
			callCount++
			return "should not execute", nil
		})
		
		if err == nil {
			t.Error("Expected error from open circuit")
		}
		if result != nil {
			t.Errorf("Expected nil result from open circuit, got %v", result)
		}
		if callCount != 0 {
			t.Error("Function should not have been called when circuit is open")
		}
	})
	
	t.Run("Circuit transitions to half-open after timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 100*time.Millisecond)
		
		// Trip the circuit
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		if cb.State() != StateOpen {
			t.Fatal("Circuit should be open")
		}
		
		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)
		
		// Next call should transition to half-open
		callCount := 0
		cb.Call(func() (interface{}, error) {
			callCount++
			if cb.State() != StateHalfOpen {
				t.Error("Expected state to be HalfOpen during probe call")
			}
			return "probe", nil
		})
		
		if callCount != 1 {
			t.Error("Probe function should have been called exactly once")
		}
	})
	
	t.Run("Successful call in half-open closes circuit", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 100*time.Millisecond)
		
		// Trip the circuit
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)
		
		// Successful call should close the circuit
		result, err := cb.Call(func() (interface{}, error) {
			return "recovery", nil
		})
		
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "recovery" {
			t.Errorf("Expected 'recovery', got %v", result)
		}
		if cb.State() != StateClosed {
			t.Errorf("Expected state to be Closed after successful recovery, got %v", cb.State())
		}
	})
	
	t.Run("Failed call in half-open reopens circuit", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 100*time.Millisecond)
		
		// Trip the circuit
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)
		
		// Failed call should reopen the circuit
		_, err := cb.Call(func() (interface{}, error) {
			return nil, errors.New("still failing")
		})
		
		if err == nil {
			t.Error("Expected error from failing function")
		}
		if cb.State() != StateOpen {
			t.Errorf("Expected state to be Open after failed recovery, got %v", cb.State())
		}
	})
	
	t.Run("Concurrent access safety", func(t *testing.T) {
		cb := NewCircuitBreaker(10, 100*time.Millisecond)
		
		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64
		numGoroutines := 50
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < 10; j++ {
					_, err := cb.Call(func() (interface{}, error) {
						// Simulate intermittent failures
						if (id*10+j)%7 == 0 {
							return nil, errors.New("simulated failure")
						}
						return "success", nil
					})
					
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
					
					time.Sleep(time.Millisecond)
				}
			}(i)
		}
		
		wg.Wait()
		
		total := successCount + errorCount
		if total != int64(numGoroutines*10) {
			t.Errorf("Expected %d total operations, got %d", numGoroutines*10, total)
		}
		
		// Circuit should be in a consistent state
		state := cb.State()
		if state != StateClosed && state != StateOpen && state != StateHalfOpen {
			t.Errorf("Circuit is in invalid state: %v", state)
		}
	})
	
	t.Run("Failure count resets on success", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 100*time.Millisecond)
		
		// Cause some failures (but not enough to trip)
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		// Successful call should reset failure count
		cb.Call(func() (interface{}, error) {
			return "success", nil
		})
		
		// Should be able to have more failures before tripping
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		// Circuit should still be closed
		if cb.State() != StateClosed {
			t.Errorf("Expected circuit to remain closed, got %v", cb.State())
		}
		
		// One more failure should trip it
		cb.Call(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
		
		if cb.State() != StateOpen {
			t.Errorf("Expected circuit to be open after threshold reached, got %v", cb.State())
		}
	})
	
	t.Run("Error types and messages", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 100*time.Millisecond)
		
		// Trip the circuit
		originalErr := errors.New("original service error")
		_, err := cb.Call(func() (interface{}, error) {
			return nil, originalErr
		})
		
		if err != originalErr {
			t.Errorf("Expected original error to be returned, got %v", err)
		}
		
		// Now circuit should be open
		_, err = cb.Call(func() (interface{}, error) {
			return "should not execute", nil
		})
		
		if err == nil {
			t.Error("Expected circuit breaker error")
		}
		
		// Error should indicate circuit is open
		if err.Error() == "" {
			t.Error("Circuit breaker error should have meaningful message")
		}
	})
}

func TestCircuitBreakerConfiguration(t *testing.T) {
	t.Run("Zero max failures", func(t *testing.T) {
		cb := NewCircuitBreaker(0, 1*time.Second)
		
		// First call should trip the circuit immediately
		_, err := cb.Call(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
		
		if err == nil {
			t.Error("Expected error")
		}
		
		if cb.State() != StateOpen {
			t.Errorf("Expected circuit to be open immediately with maxFailures=0, got %v", cb.State())
		}
	})
	
	t.Run("Very short reset timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 1*time.Millisecond)
		
		// Trip the circuit
		cb.Call(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
		
		if cb.State() != StateOpen {
			t.Fatal("Circuit should be open")
		}
		
		// Wait for reset
		time.Sleep(5 * time.Millisecond)
		
		// Should allow probe call
		callCount := 0
		cb.Call(func() (interface{}, error) {
			callCount++
			return "probe", nil
		})
		
		if callCount != 1 {
			t.Error("Probe call should have been executed")
		}
	})
	
	t.Run("Very long reset timeout", func(t *testing.T) {
		cb := NewCircuitBreaker(1, 1*time.Hour)
		
		// Trip the circuit
		cb.Call(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
		
		if cb.State() != StateOpen {
			t.Fatal("Circuit should be open")
		}
		
		// Should still be open after short wait
		time.Sleep(10 * time.Millisecond)
		
		callCount := 0
		cb.Call(func() (interface{}, error) {
			callCount++
			return "should not execute", nil
		})
		
		if callCount != 0 {
			t.Error("Function should not have been called")
		}
		if cb.State() != StateOpen {
			t.Error("Circuit should still be open")
		}
	})
}

func TestCircuitBreakerFallback(t *testing.T) {
	t.Run("Fallback pattern", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 100*time.Millisecond)
		
		// Helper function with fallback
		callWithFallback := func() (string, error) {
			result, err := cb.Call(func() (interface{}, error) {
				return nil, errors.New("service unavailable")
			})
			
			if err != nil {
				// Fallback to cached or default value
				return "fallback response", nil
			}
			
			return result.(string), nil
		}
		
		// First few calls should execute function and use fallback
		for i := 0; i < 3; i++ {
			result, err := callWithFallback()
			if err != nil {
				t.Errorf("Expected no error with fallback, got %v", err)
			}
			if result != "fallback response" {
				t.Errorf("Expected fallback response, got %s", result)
			}
		}
		
		// Circuit should now be open
		if cb.State() != StateOpen {
			t.Error("Circuit should be open")
		}
		
		// Subsequent calls should still work with fallback
		result, err := callWithFallback()
		if err != nil {
			t.Errorf("Expected no error with fallback, got %v", err)
		}
		if result != "fallback response" {
			t.Errorf("Expected fallback response, got %s", result)
		}
	})
}

// Benchmark tests
func BenchmarkCircuitBreakerClosed(b *testing.B) {
	cb := NewCircuitBreaker(100, 1*time.Second)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Call(func() (interface{}, error) {
				return "success", nil
			})
		}
	})
}

func BenchmarkCircuitBreakerOpen(b *testing.B) {
	cb := NewCircuitBreaker(1, 1*time.Hour) // Long timeout to keep open
	
	// Trip the circuit
	cb.Call(func() (interface{}, error) {
		return nil, errors.New("failure")
	})
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Call(func() (interface{}, error) {
				return "should not execute", nil
			})
		}
	})
}

func BenchmarkCircuitBreakerMixed(b *testing.B) {
	cb := NewCircuitBreaker(10, 100*time.Millisecond)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			counter++
			cb.Call(func() (interface{}, error) {
				// Simulate 10% failure rate
				if counter%10 == 0 {
					return nil, errors.New("occasional failure")
				}
				return "success", nil
			})
		}
	})
}

// Helper function to simulate state transitions for testing
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

func TestCircuitBreakerInternals(t *testing.T) {
	t.Run("Failure count tracking", func(t *testing.T) {
		cb := NewCircuitBreaker(5, 1*time.Second)
		
		if cb.GetFailureCount() != 0 {
			t.Error("Initial failure count should be 0")
		}
		
		// Cause some failures
		for i := 1; i <= 3; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
			
			if cb.GetFailureCount() != i {
				t.Errorf("Expected failure count %d, got %d", i, cb.GetFailureCount())
			}
		}
		
		// Success should reset count
		cb.Call(func() (interface{}, error) {
			return "success", nil
		})
		
		if cb.GetFailureCount() != 0 {
			t.Errorf("Expected failure count to reset to 0, got %d", cb.GetFailureCount())
		}
	})
}