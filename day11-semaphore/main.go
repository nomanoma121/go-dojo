package main

import (
	"context"
	"sync"
	"time"
)

// Semaphore controls access to a limited number of resources
type Semaphore struct {
	permits chan struct{}
	mu      sync.Mutex
}

// NewSemaphore creates a new semaphore with the specified number of permits
func NewSemaphore(permits int) *Semaphore {
	// TODO: 実装してください
	return nil
}

// Acquire acquires a permit from the semaphore
func (s *Semaphore) Acquire() {
	// TODO: 実装してください
}

// TryAcquire tries to acquire a permit without blocking
func (s *Semaphore) TryAcquire() bool {
	// TODO: 実装してください
	return false
}

// AcquireWithTimeout acquires a permit with timeout
func (s *Semaphore) AcquireWithTimeout(timeout time.Duration) bool {
	// TODO: 実装してください
	return false
}

// Release releases a permit back to the semaphore
func (s *Semaphore) Release() {
	// TODO: 実装してください
}

// AvailablePermits returns the number of available permits
func (s *Semaphore) AvailablePermits() int {
	// TODO: 実装してください
	return 0
}

func main() {
	sem := NewSemaphore(2)
	
	// テスト実行
	sem.Acquire()
	println("Acquired permit 1")
	
	if sem.TryAcquire() {
		println("Acquired permit 2")
		sem.Release()
		println("Released permit 2")
	}
	
	sem.Release()
	println("Released permit 1")
}