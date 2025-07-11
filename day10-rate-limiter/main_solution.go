package main

import (
	"context"
	"sync"
	"time"
)

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int, capacity int) *RateLimiter {
	if rate <= 0 || capacity < 0 {
		return nil
	}
	
	rl := &RateLimiter{
		rate:     rate,
		capacity: capacity,
		tokens:   capacity, // Start with full bucket
		stopCh:   make(chan struct{}),
	}
	
	if rate > 0 {
		interval := time.Second / time.Duration(rate)
		rl.ticker = time.NewTicker(interval)
		
		// Start the token refill goroutine
		go rl.refillTokens()
	}
	
	return rl
}

// refillTokens periodically adds tokens to the bucket
func (rl *RateLimiter) refillTokens() {
	for {
		select {
		case <-rl.ticker.C:
			rl.mu.Lock()
			if rl.tokens < rl.capacity {
				rl.tokens++
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Allow checks if an operation is allowed
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

// Wait waits until an operation is allowed
func (rl *RateLimiter) Wait() {
	for {
		if rl.Allow() {
			return
		}
		// Wait for next token (approximately)
		time.Sleep(time.Second / time.Duration(rl.rate))
	}
}

// WaitWithContext waits until an operation is allowed or context is cancelled
func (rl *RateLimiter) WaitWithContext(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second / time.Duration(rl.rate)):
			// Continue waiting
		}
	}
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	if rl.ticker != nil {
		rl.ticker.Stop()
	}
	
	select {
	case <-rl.stopCh:
		// Already stopped
	default:
		close(rl.stopCh)
	}
}

// GetTokenCount returns the current number of available tokens
func (rl *RateLimiter) GetTokenCount() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.tokens
}