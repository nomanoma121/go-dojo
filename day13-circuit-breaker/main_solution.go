package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
		failures:     0,
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
	cb.mu.Lock()
	
	// Check if we should transition from Open to Half-Open
	if cb.state == StateOpen && time.Since(cb.lastFailTime) > cb.resetTimeout {
		cb.state = StateHalfOpen
		cb.failures = 0
	}
	
	// If circuit is open, fail fast
	if cb.state == StateOpen {
		cb.mu.Unlock()
		return nil, ErrCircuitOpen
	}
	
	cb.mu.Unlock()
	
	// Execute the function
	result, err := fn()
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if err != nil {
		cb.onFailure()
		return nil, err
	}
	
	cb.onSuccess()
	return result, nil
}

// onFailure handles a failure
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()
	
	if cb.failures >= cb.maxFailures {
		cb.state = StateOpen
	}
}

// onSuccess handles a success
func (cb *CircuitBreaker) onSuccess() {
	cb.failures = 0
	cb.state = StateClosed
}

// State returns current state
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Failures returns current failure count
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// CallWithFallback executes a function with fallback
func (cb *CircuitBreaker) CallWithFallback(fn func() (interface{}, error), fallback func() (interface{}, error)) (interface{}, error) {
	result, err := cb.Call(fn)
	if err != nil && (errors.Is(err, ErrCircuitOpen) || cb.state == StateOpen) {
		return fallback()
	}
	return result, err
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.state = StateClosed
	cb.failures = 0
}