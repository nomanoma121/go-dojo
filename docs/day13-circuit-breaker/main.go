package main

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	maxFailures  int
	resetTimeout time.Duration
	state        State
	failures     int
	lastFailTime time.Time
	mu           sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	// TODO: 実装してください
	return nil
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
	// TODO: 実装してください
	return nil, nil
}

// State returns current state
func (cb *CircuitBreaker) State() State {
	// TODO: 実装してください
	return StateClosed
}

func main() {
	cb := NewCircuitBreaker(3, 5*time.Second)
	
	result, err := cb.Call(func() (interface{}, error) {
		return "success", nil
	})
	
	if err != nil {
		println("Error:", err.Error())
	} else {
		println("Result:", result.(string))
	}
}