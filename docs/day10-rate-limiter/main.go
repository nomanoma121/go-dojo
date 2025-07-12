//go:build ignore

package main

import (
	"sync"
	"time"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	rate       int
	capacity   int
	tokens     int
	ticker     *time.Ticker
	mu         sync.Mutex
	stopCh     chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int, capacity int) *RateLimiter {
	// TODO: 実装してください
	return nil
}

// Allow checks if an operation is allowed
func (rl *RateLimiter) Allow() bool {
	// TODO: 実装してください
	return false
}

// Wait waits until an operation is allowed
func (rl *RateLimiter) Wait() {
	// TODO: 実装してください
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	// TODO: 実装してください
}

func main() {
	limiter := NewRateLimiter(10, 5)
	defer limiter.Stop()
	
	// テスト実行
	for i := 0; i < 3; i++ {
		if limiter.Allow() {
			println("Request", i, "allowed")
		} else {
			println("Request", i, "denied")
		}
	}
}