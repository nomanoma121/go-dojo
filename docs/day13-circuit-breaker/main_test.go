package main

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreakerBasic(t *testing.T) {
	t.Run("Initial state is closed", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      3,
			ResetTimeout:     5 * time.Second,
			HalfOpenMaxCalls: 2,
		}
		cb := NewCircuitBreaker(settings)
		
		if cb.GetState() != StateClosed {
			t.Errorf("Expected initial state to be Closed, got %v", cb.GetState())
		}
	})
	
	t.Run("Successful calls keep circuit closed", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      3,
			ResetTimeout:     5 * time.Second,
			HalfOpenMaxCalls: 2,
		}
		cb := NewCircuitBreaker(settings)
		
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
			if cb.GetState() != StateClosed {
				t.Errorf("Expected state to remain Closed, got %v", cb.GetState())
			}
		}
	})
	
	t.Run("Circuit opens after max failures", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      3,
			ResetTimeout:     5 * time.Second,
			HalfOpenMaxCalls: 2,
		}
		cb := NewCircuitBreaker(settings)
		
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
		if cb.GetState() != StateOpen {
			t.Errorf("Expected state to be Open after %d failures, got %v", 3, cb.GetState())
		}
	})
	
	t.Run("Open circuit rejects calls immediately", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      2,
			ResetTimeout:     5 * time.Second,
			HalfOpenMaxCalls: 1,
		}
		cb := NewCircuitBreaker(settings)
		
		// Trip the circuit
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		// Verify circuit is open
		if cb.GetState() != StateOpen {
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
		
		// Check that it's a CircuitBreakerOpenError
		if _, ok := err.(*CircuitBreakerOpenError); !ok {
			t.Errorf("Expected CircuitBreakerOpenError, got %T", err)
		}
	})
	
	t.Run("Circuit transitions to half-open after timeout", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      2,
			ResetTimeout:     100 * time.Millisecond,
			HalfOpenMaxCalls: 2,
		}
		cb := NewCircuitBreaker(settings)
		
		// Trip the circuit
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		if cb.GetState() != StateOpen {
			t.Fatal("Circuit should be open")
		}
		
		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)
		
		// Next call should transition to half-open
		callCount := 0
		result, err := cb.Call(func() (interface{}, error) {
			callCount++
			return "probe", nil
		})
		
		if err != nil {
			t.Errorf("Expected no error during probe, got %v", err)
		}
		if result != "probe" {
			t.Errorf("Expected 'probe', got %v", result)
		}
		if callCount != 1 {
			t.Error("Probe function should have been called exactly once")
		}
	})
	
	t.Run("Successful call in half-open closes circuit", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      2,
			ResetTimeout:     100 * time.Millisecond,
			HalfOpenMaxCalls: 1,
		}
		cb := NewCircuitBreaker(settings)
		
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
		if cb.GetState() != StateClosed {
			t.Errorf("Expected state to be Closed after successful recovery, got %v", cb.GetState())
		}
	})
	
	t.Run("Failed call in half-open reopens circuit", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      2,
			ResetTimeout:     100 * time.Millisecond,
			HalfOpenMaxCalls: 1,
		}
		cb := NewCircuitBreaker(settings)
		
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
		if cb.GetState() != StateOpen {
			t.Errorf("Expected state to be Open after failed recovery, got %v", cb.GetState())
		}
	})
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	t.Run("Half-open allows probe calls", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      2,
			ResetTimeout:     100 * time.Millisecond,
			HalfOpenMaxCalls: 3,
		}
		cb := NewCircuitBreaker(settings)
		
		// Trip the circuit
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)
		
		// First call in half-open should succeed and close circuit
		result, err := cb.Call(func() (interface{}, error) {
			return "success", nil
		})
		
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("Expected 'success', got %v", result)
		}
		
		// Should now be closed
		if cb.GetState() != StateClosed {
			t.Errorf("Expected state to be Closed after successful half-open call, got %v", cb.GetState())
		}
		
		// Subsequent calls should work normally (circuit is closed)
		for i := 0; i < 2; i++ {
			result, err := cb.Call(func() (interface{}, error) {
				return "normal", nil
			})
			
			if err != nil {
				t.Errorf("Expected no error in closed state, got %v", err)
			}
			if result != "normal" {
				t.Errorf("Expected 'normal', got %v", result)
			}
		}
	})
	
	t.Run("Half-open transitions to closed on success", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      1,
			ResetTimeout:     100 * time.Millisecond,
			HalfOpenMaxCalls: 2,
		}
		cb := NewCircuitBreaker(settings)
		
		// Trip the circuit
		cb.Call(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
		
		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)
		
		// First successful call in half-open should close the circuit
		result, err := cb.Call(func() (interface{}, error) {
			return "success", nil
		})
		
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "success" {
			t.Errorf("Expected 'success', got %v", result)
		}
		
		// Circuit should now be closed
		if cb.GetState() != StateClosed {
			t.Errorf("Expected state to be Closed after successful half-open call, got %v", cb.GetState())
		}
		
		// Additional calls should work normally (circuit is closed)
		result, err = cb.Call(func() (interface{}, error) {
			return "normal operation", nil
		})
		
		if err != nil {
			t.Errorf("Expected no error in closed state, got %v", err)
		}
		if result != "normal operation" {
			t.Errorf("Expected 'normal operation', got %v", result)
		}
	})
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	t.Run("Concurrent access safety", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      10,
			ResetTimeout:     100 * time.Millisecond,
			HalfOpenMaxCalls: 3,
		}
		cb := NewCircuitBreaker(settings)
		
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
		state := cb.GetState()
		if state != StateClosed && state != StateOpen && state != StateHalfOpen {
			t.Errorf("Circuit is in invalid state: %v", state)
		}
	})
}

func TestCircuitBreakerCounts(t *testing.T) {
	t.Run("Counts tracking", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      5,
			ResetTimeout:     1 * time.Second,
			HalfOpenMaxCalls: 2,
		}
		cb := NewCircuitBreaker(settings)
		
		initialCounts := cb.GetCounts()
		if initialCounts.Requests != 0 {
			t.Error("Initial request count should be 0")
		}
		
		// Make some successful calls
		for i := 0; i < 3; i++ {
			cb.Call(func() (interface{}, error) {
				return "success", nil
			})
		}
		
		counts := cb.GetCounts()
		if counts.TotalSuccesses != 3 {
			t.Errorf("Expected 3 successes, got %d", counts.TotalSuccesses)
		}
		if counts.ConsecutiveSuccesses != 3 {
			t.Errorf("Expected 3 consecutive successes, got %d", counts.ConsecutiveSuccesses)
		}
		
		// Make some failures
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		counts = cb.GetCounts()
		if counts.TotalFailures != 2 {
			t.Errorf("Expected 2 failures, got %d", counts.TotalFailures)
		}
		if counts.ConsecutiveFailures != 2 {
			t.Errorf("Expected 2 consecutive failures, got %d", counts.ConsecutiveFailures)
		}
		if counts.ConsecutiveSuccesses != 0 {
			t.Errorf("Expected 0 consecutive successes after failure, got %d", counts.ConsecutiveSuccesses)
		}
	})
}

func TestCircuitBreakerReset(t *testing.T) {
	t.Run("Manual reset", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      2,
			ResetTimeout:     1 * time.Hour, // Very long timeout
			HalfOpenMaxCalls: 1,
		}
		cb := NewCircuitBreaker(settings)
		
		// Trip the circuit
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		if cb.GetState() != StateOpen {
			t.Fatal("Circuit should be open")
		}
		
		// Manual reset
		cb.Reset()
		
		if cb.GetState() != StateClosed {
			t.Errorf("Expected state to be Closed after reset, got %v", cb.GetState())
		}
		
		// Should work normally after reset
		result, err := cb.Call(func() (interface{}, error) {
			return "after reset", nil
		})
		
		if err != nil {
			t.Errorf("Expected no error after reset, got %v", err)
		}
		if result != "after reset" {
			t.Errorf("Expected 'after reset', got %v", result)
		}
	})
}

func TestCircuitBreakerCanExecute(t *testing.T) {
	t.Run("CanExecute states", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      2,
			ResetTimeout:     100 * time.Millisecond,
			HalfOpenMaxCalls: 2,
		}
		cb := NewCircuitBreaker(settings)
		
		// Initially should allow execution
		if !cb.CanExecute() {
			t.Error("Should allow execution in closed state")
		}
		
		// Trip the circuit
		for i := 0; i < 2; i++ {
			cb.Call(func() (interface{}, error) {
				return nil, errors.New("failure")
			})
		}
		
		// Should not allow execution when open
		if cb.CanExecute() {
			t.Error("Should not allow execution in open state")
		}
		
		// Wait for reset timeout
		time.Sleep(150 * time.Millisecond)
		
		// Should allow limited execution in half-open
		if !cb.CanExecute() {
			t.Error("Should allow execution in half-open state")
		}
	})
}

func TestCircuitBreakerEdgeCases(t *testing.T) {
	t.Run("Zero max failures", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      0,
			ResetTimeout:     1 * time.Second,
			HalfOpenMaxCalls: 1,
		}
		cb := NewCircuitBreaker(settings)
		
		// First failure should trip the circuit immediately
		_, err := cb.Call(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
		
		if err == nil {
			t.Error("Expected error")
		}
		
		if cb.GetState() != StateOpen {
			t.Errorf("Expected circuit to be open immediately with maxFailures=0, got %v", cb.GetState())
		}
	})
	
	t.Run("Very short reset timeout", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      1,
			ResetTimeout:     1 * time.Millisecond,
			HalfOpenMaxCalls: 1,
		}
		cb := NewCircuitBreaker(settings)
		
		// Trip the circuit
		cb.Call(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
		
		if cb.GetState() != StateOpen {
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
}

func TestCircuitBreakerFallback(t *testing.T) {
	t.Run("Fallback pattern", func(t *testing.T) {
		settings := Settings{
			MaxFailures:      2,
			ResetTimeout:     100 * time.Millisecond,
			HalfOpenMaxCalls: 1,
		}
		cb := NewCircuitBreaker(settings)
		
		// Helper function with fallback
		callWithFallback := func() (string, error) {
			result, err := cb.Call(func() (interface{}, error) {
				return nil, errors.New("service unavailable")
			})
			
			if err != nil {
				// Check if it's a circuit breaker error (service is down)
				if _, ok := err.(*CircuitBreakerOpenError); ok {
					// Fast fallback - circuit is open
					return "fast fallback response", nil
				}
				// Slower fallback - service error but circuit still trying
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
			if result != "fallback response" && result != "fast fallback response" {
				t.Errorf("Expected fallback response, got %s", result)
			}
		}
		
		// Circuit should now be open
		if cb.GetState() != StateOpen {
			t.Error("Circuit should be open")
		}
		
		// Subsequent calls should use fast fallback
		result, err := callWithFallback()
		if err != nil {
			t.Errorf("Expected no error with fallback, got %v", err)
		}
		if result != "fast fallback response" {
			t.Errorf("Expected fast fallback response, got %s", result)
		}
	})
}

// Benchmark tests
func BenchmarkCircuitBreakerClosed(b *testing.B) {
	settings := Settings{
		MaxFailures:      100,
		ResetTimeout:     1 * time.Second,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker(settings)
	
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
	settings := Settings{
		MaxFailures:      1,
		ResetTimeout:     1 * time.Hour, // Long timeout to keep open
		HalfOpenMaxCalls: 1,
	}
	cb := NewCircuitBreaker(settings)
	
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
	settings := Settings{
		MaxFailures:      10,
		ResetTimeout:     100 * time.Millisecond,
		HalfOpenMaxCalls: 2,
	}
	cb := NewCircuitBreaker(settings)
	
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