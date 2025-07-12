package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Result represents the result of an operation (success or error)
type Result[T any] struct {
	Value T
	Error error
}

// Future represents a future result of an asynchronous operation
type Future[T any] struct {
	result chan Result[T]
	done   chan struct{}
	cached *Result[T]
	mu     sync.RWMutex
}

// Promise allows setting the result of a Future
type Promise[T any] struct {
	future *Future[T]
	once   sync.Once
}

// NewPromise creates a new Promise and its associated Future
func NewPromise[T any]() *Promise[T] {
	future := &Future[T]{
		result: make(chan Result[T], 1),
		done:   make(chan struct{}),
	}
	
	return &Promise[T]{
		future: future,
	}
}

// GetFuture returns the Future associated with this Promise
func (p *Promise[T]) GetFuture() *Future[T] {
	return p.future
}

// Resolve sets a successful result for the Future
func (p *Promise[T]) Resolve(value T) {
	p.once.Do(func() {
		p.future.result <- Result[T]{Value: value, Error: nil}
		close(p.future.done)
	})
}

// Reject sets an error result for the Future
func (p *Promise[T]) Reject(err error) {
	p.once.Do(func() {
		var zero T
		p.future.result <- Result[T]{Value: zero, Error: err}
		close(p.future.done)
	})
}

// Get waits for the result and returns it
func (f *Future[T]) Get() (T, error) {
	// Check if result is already cached
	f.mu.RLock()
	if f.cached != nil {
		result := *f.cached
		f.mu.RUnlock()
		return result.Value, result.Error
	}
	f.mu.RUnlock()
	
	// Wait for result and cache it
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Double-check in case another goroutine cached it
	if f.cached != nil {
		return f.cached.Value, f.cached.Error
	}
	
	result := <-f.result
	f.cached = &result
	return result.Value, result.Error
}

// GetWithTimeout waits for the result with a timeout
func (f *Future[T]) GetWithTimeout(timeout time.Duration) (T, error) {
	// Check if result is already cached
	f.mu.RLock()
	if f.cached != nil {
		result := *f.cached
		f.mu.RUnlock()
		return result.Value, result.Error
	}
	f.mu.RUnlock()
	
	// Wait for result with timeout
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Double-check in case another goroutine cached it
	if f.cached != nil {
		return f.cached.Value, f.cached.Error
	}
	
	select {
	case result := <-f.result:
		f.cached = &result
		return result.Value, result.Error
	case <-time.After(timeout):
		var zero T
		return zero, fmt.Errorf("timeout after %v", timeout)
	}
}

// GetWithContext waits for the result with context cancellation support
func (f *Future[T]) GetWithContext(ctx context.Context) (T, error) {
	// Check if result is already cached
	f.mu.RLock()
	if f.cached != nil {
		result := *f.cached
		f.mu.RUnlock()
		return result.Value, result.Error
	}
	f.mu.RUnlock()
	
	// Wait for result with context
	f.mu.Lock()
	defer f.mu.Unlock()
	
	// Double-check in case another goroutine cached it
	if f.cached != nil {
		return f.cached.Value, f.cached.Error
	}
	
	select {
	case result := <-f.result:
		f.cached = &result
		return result.Value, result.Error
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

// IsDone returns true if the Future has completed
func (f *Future[T]) IsDone() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// Then creates a new Future by applying a function to the result
func (f *Future[T]) Then(fn func(T) (any, error)) *Future[any] {
	newPromise := NewPromise[any]()
	
	go func() {
		value, err := f.Get()
		if err != nil {
			newPromise.Reject(err)
			return
		}
		
		newValue, err := fn(value)
		if err != nil {
			newPromise.Reject(err)
		} else {
			newPromise.Resolve(newValue)
		}
	}()
	
	return newPromise.GetFuture()
}

// Map creates a new Future by applying a transformation function
func (f *Future[T]) Map(fn func(T) any) *Future[any] {
	return f.Then(func(value T) (any, error) {
		return fn(value), nil
	})
}

// Utility functions

// Completed creates a Future that is already completed with a value
func Completed[T any](value T) *Future[T] {
	promise := NewPromise[T]()
	promise.Resolve(value)
	return promise.GetFuture()
}

// Failed creates a Future that is already completed with an error
func Failed[T any](err error) *Future[T] {
	promise := NewPromise[T]()
	promise.Reject(err)
	return promise.GetFuture()
}

// RunAsync runs a function asynchronously and returns a Future
func RunAsync[T any](fn func() (T, error)) *Future[T] {
	promise := NewPromise[T]()
	
	go func() {
		value, err := fn()
		if err != nil {
			promise.Reject(err)
		} else {
			promise.Resolve(value)
		}
	}()
	
	return promise.GetFuture()
}

// Delay creates a Future that completes after a specified duration
func Delay[T any](value T, delay time.Duration) *Future[T] {
	promise := NewPromise[T]()
	
	go func() {
		time.Sleep(delay)
		promise.Resolve(value)
	}()
	
	return promise.GetFuture()
}

// AllOf waits for all Futures to complete
func AllOf[T any](futures ...*Future[T]) *Future[[]T] {
	promise := NewPromise[[]T]()
	
	go func() {
		results := make([]T, len(futures))
		
		for i, future := range futures {
			value, err := future.Get()
			if err != nil {
				promise.Reject(err)
				return
			}
			results[i] = value
		}
		
		promise.Resolve(results)
	}()
	
	return promise.GetFuture()
}

// AnyOf waits for any Future to complete
func AnyOf[T any](futures ...*Future[T]) *Future[T] {
	promise := NewPromise[T]()
	
	for _, future := range futures {
		go func(f *Future[T]) {
			value, err := f.Get()
			if err != nil {
				promise.Reject(err)
			} else {
				promise.Resolve(value)
			}
		}(future)
	}
	
	return promise.GetFuture()
}

// Sample usage and testing functions

func simulateAPICall(id int, delay time.Duration) *Future[string] {
	return RunAsync(func() (string, error) {
		time.Sleep(delay)
		if id%5 == 0 {
			return "", fmt.Errorf("API error for ID %d", id)
		}
		return fmt.Sprintf("API response for ID %d", id), nil
	})
}

func main() {
	fmt.Println("=== Future/Promise Pattern Demo ===")
	
	// 基本的な使用例
	promise := NewPromise[string]()
	future := promise.GetFuture()
	
	// 非同期でPromiseを解決
	go func() {
		time.Sleep(100 * time.Millisecond)
		promise.Resolve("Hello, Future!")
	}()
	
	// 結果を取得
	result, err := future.Get()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s\n", result)
	}
	
	// 非同期実行の例
	fmt.Println("\n=== Async Execution ===")
	asyncFuture := RunAsync(func() (int, error) {
		time.Sleep(200 * time.Millisecond)
		return 42, nil
	})
	
	value, err := asyncFuture.GetWithTimeout(1 * time.Second)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Async result: %d\n", value)
	}
	
	// チェイニングの例
	fmt.Println("\n=== Future Chaining ===")
	chainedFuture := RunAsync(func() (int, error) {
		return 10, nil
	}).Then(func(x int) (any, error) {
		return x * 2, nil
	}).Then(func(x any) (any, error) {
		return fmt.Sprintf("Result: %v", x), nil
	})
	
	finalResult, err := chainedFuture.Get()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Chained result: %v\n", finalResult)
	}
	
	// 複数のFutureを組み合わせる例
	fmt.Println("\n=== Multiple Futures ===")
	future1 := simulateAPICall(1, 100*time.Millisecond)
	future2 := simulateAPICall(2, 150*time.Millisecond)
	future3 := simulateAPICall(3, 80*time.Millisecond)
	
	allResults := AllOf(future1, future2, future3)
	results, err := allResults.GetWithTimeout(1 * time.Second)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("All results: %v\n", results)
	}
}