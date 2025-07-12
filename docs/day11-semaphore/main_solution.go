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
	if permits < 0 {
		panic("Semaphore permits cannot be negative")
	}
	
	sem := &Semaphore{
		permits: make(chan struct{}, permits),
	}
	
	// Fill the semaphore with permits
	for i := 0; i < permits; i++ {
		sem.permits <- struct{}{}
	}
	
	return sem
}

// Acquire acquires a permit from the semaphore
func (s *Semaphore) Acquire() {
	<-s.permits
}

// TryAcquire tries to acquire a permit without blocking
func (s *Semaphore) TryAcquire() bool {
	select {
	case <-s.permits:
		return true
	default:
		return false
	}
}

// AcquireWithTimeout acquires a permit with timeout
func (s *Semaphore) AcquireWithTimeout(timeout time.Duration) bool {
	select {
	case <-s.permits:
		return true
	case <-time.After(timeout):
		return false
	}
}

// AcquireWithContext acquires a permit with context
func (s *Semaphore) AcquireWithContext(ctx context.Context) error {
	select {
	case <-s.permits:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a permit back to the semaphore
func (s *Semaphore) Release() {
	select {
	case s.permits <- struct{}{}:
		// Successfully released
	default:
		// Channel is full, create a goroutine to avoid blocking
		go func() {
			s.permits <- struct{}{}
		}()
	}
}

// AvailablePermits returns the number of available permits
func (s *Semaphore) AvailablePermits() int {
	return len(s.permits)
}

// TryAcquireN tries to acquire n permits at once
func (s *Semaphore) TryAcquireN(n int) bool {
	if n <= 0 {
		return true
	}
	
	acquired := 0
	for i := 0; i < n; i++ {
		if s.TryAcquire() {
			acquired++
		} else {
			// Release the permits we already acquired
			for j := 0; j < acquired; j++ {
				s.Release()
			}
			return false
		}
	}
	return true
}

// ReleaseN releases n permits
func (s *Semaphore) ReleaseN(n int) {
	for i := 0; i < n; i++ {
		s.Release()
	}
}