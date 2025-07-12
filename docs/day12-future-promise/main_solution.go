package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

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
	result := <-f.result
	return result.Value, result.Error
}

// GetWithTimeout waits for the result with a timeout
func (f *Future[T]) GetWithTimeout(timeout time.Duration) (T, error) {
	select {
	case result := <-f.result:
		return result.Value, result.Error
	case <-time.After(timeout):
		var zero T
		return zero, fmt.Errorf("timeout after %v", timeout)
	}
}

// GetWithContext waits for the result with context cancellation support
func (f *Future[T]) GetWithContext(ctx context.Context) (T, error) {
	select {
	case result := <-f.result:
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
		result := <-f.result
		if result.Error != nil {
			newPromise.Reject(result.Error)
			return
		}
		
		newValue, err := fn(result.Value)
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
			result := <-future.result
			if result.Error != nil {
				promise.Reject(result.Error)
				return
			}
			results[i] = result.Value
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
			result := <-f.result
			if result.Error != nil {
				promise.Reject(result.Error)
			} else {
				promise.Resolve(result.Value)
			}
		}(future)
	}
	
	return promise.GetFuture()
}

// Advanced Future combinators

// Sequence executes Futures sequentially, each depending on the previous result
func Sequence[T any](initial T, operations ...func(T) *Future[T]) *Future[T] {
	current := Completed(initial)
	
	for _, op := range operations {
		current = current.Then(func(value T) (any, error) {
			nextFuture := op(value)
			return nextFuture.Get()
		}).Then(func(value any) (any, error) {
			return value.(T), nil
		})
	}
	
	return current.(*Future[T])
}

// Retry retries a Future-producing function up to maxAttempts times
func Retry[T any](fn func() *Future[T], maxAttempts int, delay time.Duration) *Future[T] {
	promise := NewPromise[T]()
	
	go func() {
		var lastError error
		
		for attempt := 0; attempt < maxAttempts; attempt++ {
			future := fn()
			value, err := future.Get()
			
			if err == nil {
				promise.Resolve(value)
				return
			}
			
			lastError = err
			if attempt < maxAttempts-1 {
				time.Sleep(delay)
			}
		}
		
		promise.Reject(fmt.Errorf("failed after %d attempts, last error: %w", maxAttempts, lastError))
	}()
	
	return promise.GetFuture()
}

// Timeout wraps a Future with a timeout
func Timeout[T any](future *Future[T], timeout time.Duration) *Future[T] {
	promise := NewPromise[T]()
	
	go func() {
		select {
		case result := <-future.result:
			if result.Error != nil {
				promise.Reject(result.Error)
			} else {
				promise.Resolve(result.Value)
			}
		case <-time.After(timeout):
			promise.Reject(fmt.Errorf("timeout after %v", timeout))
		}
	}()
	
	return promise.GetFuture()
}

// Cache caches the result of a Future so multiple Gets return the same result quickly
type CachedFuture[T any] struct {
	*Future[T]
	cached    bool
	cachedVal T
	cachedErr error
	mu        sync.RWMutex
}

func Cache[T any](future *Future[T]) *CachedFuture[T] {
	cached := &CachedFuture[T]{Future: future}
	
	// Start caching in background
	go func() {
		val, err := future.Get()
		cached.mu.Lock()
		cached.cached = true
		cached.cachedVal = val
		cached.cachedErr = err
		cached.mu.Unlock()
	}()
	
	return cached
}

func (cf *CachedFuture[T]) Get() (T, error) {
	cf.mu.RLock()
	if cf.cached {
		val, err := cf.cachedVal, cf.cachedErr
		cf.mu.RUnlock()
		return val, err
	}
	cf.mu.RUnlock()
	
	return cf.Future.Get()
}

// FutureGroup manages a group of Futures with various completion strategies
type FutureGroup[T any] struct {
	futures [](*Future[T])
}

func NewFutureGroup[T any]() *FutureGroup[T] {
	return &FutureGroup[T]{
		futures: make([]*Future[T], 0),
	}
}

func (fg *FutureGroup[T]) Add(future *Future[T]) {
	fg.futures = append(fg.futures, future)
}

func (fg *FutureGroup[T]) WaitAll() *Future[[]T] {
	return AllOf(fg.futures...)
}

func (fg *FutureGroup[T]) WaitAny() *Future[T] {
	return AnyOf(fg.futures...)
}

func (fg *FutureGroup[T]) WaitAtLeast(n int) *Future[[]T] {
	promise := NewPromise[[]T]()
	
	go func() {
		results := make([]T, 0, n)
		completed := 0
		
		for _, future := range fg.futures {
			go func(f *Future[T]) {
				if value, err := f.Get(); err == nil {
					results = append(results, value)
					completed++
					if completed >= n {
						promise.Resolve(results[:n])
					}
				}
			}(future)
		}
	}()
	
	return promise.GetFuture()
}