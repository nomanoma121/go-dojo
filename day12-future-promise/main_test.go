package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBasicPromiseFuture(t *testing.T) {
	t.Run("Promise resolve", func(t *testing.T) {
		promise := NewPromise[string]()
		future := promise.GetFuture()
		
		go func() {
			time.Sleep(50 * time.Millisecond)
			promise.Resolve("test value")
		}()
		
		result, err := future.Get()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "test value" {
			t.Errorf("Expected 'test value', got '%s'", result)
		}
	})
	
	t.Run("Promise reject", func(t *testing.T) {
		promise := NewPromise[string]()
		future := promise.GetFuture()
		
		expectedError := errors.New("test error")
		go func() {
			time.Sleep(50 * time.Millisecond)
			promise.Reject(expectedError)
		}()
		
		result, err := future.Get()
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if err != expectedError {
			t.Errorf("Expected error '%v', got '%v'", expectedError, err)
		}
		if result != "" {
			t.Errorf("Expected empty result on error, got '%s'", result)
		}
	})
	
	t.Run("Promise resolve only once", func(t *testing.T) {
		promise := NewPromise[int]()
		future := promise.GetFuture()
		
		// Try to resolve multiple times
		promise.Resolve(1)
		promise.Resolve(2)
		promise.Reject(errors.New("should not happen"))
		
		result, err := future.Get()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != 1 {
			t.Errorf("Expected 1, got %d", result)
		}
	})
}

func TestFutureTimeout(t *testing.T) {
	t.Run("Timeout before completion", func(t *testing.T) {
		promise := NewPromise[string]()
		future := promise.GetFuture()
		
		// Don't resolve the promise
		start := time.Now()
		result, err := future.GetWithTimeout(100 * time.Millisecond)
		elapsed := time.Since(start)
		
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
		if result != "" {
			t.Errorf("Expected empty result on timeout, got '%s'", result)
		}
		if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
			t.Errorf("Timeout took %v, expected around 100ms", elapsed)
		}
	})
	
	t.Run("Completion before timeout", func(t *testing.T) {
		promise := NewPromise[string]()
		future := promise.GetFuture()
		
		go func() {
			time.Sleep(50 * time.Millisecond)
			promise.Resolve("fast result")
		}()
		
		result, err := future.GetWithTimeout(200 * time.Millisecond)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "fast result" {
			t.Errorf("Expected 'fast result', got '%s'", result)
		}
	})
}

func TestFutureContext(t *testing.T) {
	t.Run("Context cancellation", func(t *testing.T) {
		promise := NewPromise[string]()
		future := promise.GetFuture()
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		start := time.Now()
		result, err := future.GetWithContext(ctx)
		elapsed := time.Since(start)
		
		if err == nil {
			t.Error("Expected context cancellation error, got nil")
		}
		if result != "" {
			t.Errorf("Expected empty result on cancellation, got '%s'", result)
		}
		if elapsed < 90*time.Millisecond || elapsed > 200*time.Millisecond {
			t.Errorf("Cancellation took %v, expected around 100ms", elapsed)
		}
	})
	
	t.Run("Completion before context cancellation", func(t *testing.T) {
		promise := NewPromise[string]()
		future := promise.GetFuture()
		
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		
		go func() {
			time.Sleep(50 * time.Millisecond)
			promise.Resolve("context result")
		}()
		
		result, err := future.GetWithContext(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "context result" {
			t.Errorf("Expected 'context result', got '%s'", result)
		}
	})
}

func TestFutureIsDone(t *testing.T) {
	t.Run("Initially not done", func(t *testing.T) {
		promise := NewPromise[string]()
		future := promise.GetFuture()
		
		if future.IsDone() {
			t.Error("Future should not be done initially")
		}
	})
	
	t.Run("Done after resolve", func(t *testing.T) {
		promise := NewPromise[string]()
		future := promise.GetFuture()
		
		promise.Resolve("test")
		
		if !future.IsDone() {
			t.Error("Future should be done after resolve")
		}
	})
	
	t.Run("Done after reject", func(t *testing.T) {
		promise := NewPromise[string]()
		future := promise.GetFuture()
		
		promise.Reject(errors.New("test error"))
		
		if !future.IsDone() {
			t.Error("Future should be done after reject")
		}
	})
}

func TestFutureChaining(t *testing.T) {
	t.Run("Then chaining", func(t *testing.T) {
		promise := NewPromise[int]()
		future := promise.GetFuture()
		
		chainedFuture := future.Then(func(x int) (any, error) {
			return x * 2, nil
		}).Then(func(x any) (any, error) {
			if val, ok := x.(int); ok {
				return fmt.Sprintf("result: %d", val), nil
			}
			return nil, errors.New("type conversion failed")
		})
		
		promise.Resolve(21)
		
		result, err := chainedFuture.Get()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "result: 42" {
			t.Errorf("Expected 'result: 42', got '%v'", result)
		}
	})
	
	t.Run("Then with error", func(t *testing.T) {
		promise := NewPromise[int]()
		future := promise.GetFuture()
		
		chainedFuture := future.Then(func(x int) (any, error) {
			return nil, errors.New("chain error")
		})
		
		promise.Resolve(10)
		
		result, err := chainedFuture.Get()
		if err == nil {
			t.Error("Expected error from chain, got nil")
		}
		if result != nil {
			t.Errorf("Expected nil result on error, got %v", result)
		}
	})
	
	t.Run("Map transformation", func(t *testing.T) {
		promise := NewPromise[int]()
		future := promise.GetFuture()
		
		mappedFuture := future.Map(func(x int) any {
			return x * 3
		})
		
		promise.Resolve(10)
		
		result, err := mappedFuture.Get()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != 30 {
			t.Errorf("Expected 30, got %v", result)
		}
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("Completed future", func(t *testing.T) {
		future := Completed("immediate value")
		
		if !future.IsDone() {
			t.Error("Completed future should be done immediately")
		}
		
		result, err := future.Get()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "immediate value" {
			t.Errorf("Expected 'immediate value', got '%s'", result)
		}
	})
	
	t.Run("Failed future", func(t *testing.T) {
		expectedError := errors.New("immediate error")
		future := Failed[string](expectedError)
		
		if !future.IsDone() {
			t.Error("Failed future should be done immediately")
		}
		
		result, err := future.Get()
		if err != expectedError {
			t.Errorf("Expected error '%v', got '%v'", expectedError, err)
		}
		if result != "" {
			t.Errorf("Expected empty result, got '%s'", result)
		}
	})
	
	t.Run("RunAsync", func(t *testing.T) {
		future := RunAsync(func() (int, error) {
			time.Sleep(50 * time.Millisecond)
			return 100, nil
		})
		
		result, err := future.GetWithTimeout(200 * time.Millisecond)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != 100 {
			t.Errorf("Expected 100, got %d", result)
		}
	})
	
	t.Run("Delay", func(t *testing.T) {
		start := time.Now()
		future := Delay("delayed value", 100*time.Millisecond)
		
		result, err := future.Get()
		elapsed := time.Since(start)
		
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "delayed value" {
			t.Errorf("Expected 'delayed value', got '%s'", result)
		}
		if elapsed < 90*time.Millisecond {
			t.Errorf("Delay was too short: %v", elapsed)
		}
	})
}

func TestAllOf(t *testing.T) {
	t.Run("All succeed", func(t *testing.T) {
		future1 := RunAsync(func() (int, error) {
			time.Sleep(50 * time.Millisecond)
			return 1, nil
		})
		future2 := RunAsync(func() (int, error) {
			time.Sleep(100 * time.Millisecond)
			return 2, nil
		})
		future3 := RunAsync(func() (int, error) {
			time.Sleep(30 * time.Millisecond)
			return 3, nil
		})
		
		allFuture := AllOf(future1, future2, future3)
		results, err := allFuture.GetWithTimeout(300 * time.Millisecond)
		
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(results))
		}
		
		// Results should be in order
		expected := []int{1, 2, 3}
		for i, result := range results {
			if result != expected[i] {
				t.Errorf("Expected result[%d] = %d, got %d", i, expected[i], result)
			}
		}
	})
	
	t.Run("One fails", func(t *testing.T) {
		future1 := RunAsync(func() (int, error) {
			return 1, nil
		})
		future2 := RunAsync(func() (int, error) {
			return 0, errors.New("failure")
		})
		future3 := RunAsync(func() (int, error) {
			return 3, nil
		})
		
		allFuture := AllOf(future1, future2, future3)
		results, err := allFuture.GetWithTimeout(300 * time.Millisecond)
		
		if err == nil {
			t.Error("Expected error when one future fails, got nil")
		}
		if results != nil {
			t.Errorf("Expected nil results on failure, got %v", results)
		}
	})
}

func TestAnyOf(t *testing.T) {
	t.Run("First completes wins", func(t *testing.T) {
		future1 := RunAsync(func() (string, error) {
			time.Sleep(100 * time.Millisecond)
			return "slow", nil
		})
		future2 := RunAsync(func() (string, error) {
			time.Sleep(50 * time.Millisecond)
			return "fast", nil
		})
		future3 := RunAsync(func() (string, error) {
			time.Sleep(200 * time.Millisecond)
			return "slower", nil
		})
		
		anyFuture := AnyOf(future1, future2, future3)
		result, err := anyFuture.GetWithTimeout(300 * time.Millisecond)
		
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != "fast" {
			t.Errorf("Expected 'fast', got '%s'", result)
		}
	})
	
	t.Run("First error wins", func(t *testing.T) {
		future1 := RunAsync(func() (string, error) {
			time.Sleep(100 * time.Millisecond)
			return "success", nil
		})
		future2 := RunAsync(func() (string, error) {
			time.Sleep(50 * time.Millisecond)
			return "", errors.New("fast error")
		})
		
		anyFuture := AnyOf(future1, future2)
		result, err := anyFuture.GetWithTimeout(300 * time.Millisecond)
		
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if result != "" {
			t.Errorf("Expected empty result on error, got '%s'", result)
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("Multiple goroutines accessing same future", func(t *testing.T) {
		promise := NewPromise[int]()
		future := promise.GetFuture()
		
		var wg sync.WaitGroup
		var successCount int64
		numGoroutines := 10
		
		// Start multiple goroutines trying to get the result
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				result, err := future.GetWithTimeout(1 * time.Second)
				if err == nil && result == 42 {
					atomic.AddInt64(&successCount, 1)
				}
			}()
		}
		
		// Resolve after a short delay
		time.Sleep(50 * time.Millisecond)
		promise.Resolve(42)
		
		wg.Wait()
		
		if successCount != int64(numGoroutines) {
			t.Errorf("Expected %d successful gets, got %d", numGoroutines, successCount)
		}
	})
}

// Benchmark tests
func BenchmarkPromiseResolve(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			promise := NewPromise[int]()
			promise.Resolve(42)
			future := promise.GetFuture()
			future.Get()
		}
	})
}

func BenchmarkFutureChaining(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		future := Completed(10).
			Then(func(x int) (any, error) { return x * 2, nil }).
			Then(func(x any) (any, error) { return x.(int) + 1, nil })
		future.Get()
	}
}

func BenchmarkAllOf(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		futures := make([]*Future[int], 10)
		for j := 0; j < 10; j++ {
			futures[j] = Completed(j)
		}
		allFuture := AllOf(futures...)
		allFuture.Get()
	}
}