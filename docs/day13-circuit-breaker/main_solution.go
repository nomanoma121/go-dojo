//go:build ignore

package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// String returns string representation of the state
func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "Closed"
	case StateOpen:
		return "Open"
	case StateHalfOpen:
		return "Half-Open"
	default:
		return "Unknown"
	}
}

// Settings for circuit breaker configuration
type Settings struct {
	MaxFailures      int           // 失敗回数の閾値
	ResetTimeout     time.Duration // Open状態からHalf-Openに移行する時間
	HalfOpenMaxCalls int           // Half-Open状態での最大試行回数
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	maxFailures      int
	resetTimeout     time.Duration
	halfOpenMaxCalls int

	state              CircuitBreakerState
	failures           int
	lastFailTime       time.Time
	halfOpenCalls      int
	totalRequests      uint64
	totalSuccesses     uint64
	totalFailures      uint64
	consecutiveSuccesses uint64
	consecutiveFailures  uint64

	mutex sync.RWMutex
}

// Counts represents the current statistics of the circuit breaker
type Counts struct {
	Requests             uint64
	TotalSuccesses       uint64
	TotalFailures        uint64
	ConsecutiveSuccesses uint64
	ConsecutiveFailures  uint64
}

// CircuitBreakerOpenError is returned when the circuit breaker is open
type CircuitBreakerOpenError struct {
	State CircuitBreakerState
}

func (e *CircuitBreakerOpenError) Error() string {
	return fmt.Sprintf("circuit breaker is %s", e.State)
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(settings Settings) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:      settings.MaxFailures,
		resetTimeout:     settings.ResetTimeout,
		halfOpenMaxCalls: settings.HalfOpenMaxCalls,
		state:            StateClosed,
		failures:         0,
		halfOpenCalls:    0,
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
	cb.mutex.Lock()
	
	// Check if we should transition from Open to Half-Open
	if cb.state == StateOpen && cb.shouldAttemptReset() {
		cb.state = StateHalfOpen
		cb.halfOpenCalls = 0
	}
	
	// Check if call is allowed
	if !cb.canExecuteUnsafe() {
		cb.mutex.Unlock()
		return nil, &CircuitBreakerOpenError{State: cb.state}
	}
	
	// Increment counters
	cb.totalRequests++
	if cb.state == StateHalfOpen {
		cb.halfOpenCalls++
	}
	
	cb.mutex.Unlock()
	
	// Execute the function
	result, err := fn()
	
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	if err != nil {
		cb.onFailure()
		return nil, err
	}
	
	cb.onSuccess()
	return result, nil
}

// GetState returns current state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetCounts returns current statistics
func (cb *CircuitBreaker) GetCounts() Counts {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	return Counts{
		Requests:             cb.totalRequests,
		TotalSuccesses:       cb.totalSuccesses,
		TotalFailures:        cb.totalFailures,
		ConsecutiveSuccesses: cb.consecutiveSuccesses,
		ConsecutiveFailures:  cb.consecutiveFailures,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.state = StateClosed
	cb.failures = 0
	cb.halfOpenCalls = 0
	cb.consecutiveSuccesses = 0
	cb.consecutiveFailures = 0
}

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	
	return cb.canExecuteUnsafe()
}

// Private helper methods

// canExecuteUnsafe checks if execution is allowed (assumes lock is held)
func (cb *CircuitBreaker) canExecuteUnsafe() bool {
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if cb.shouldAttemptReset() {
			return true // Will transition to half-open
		}
		return false
	case StateHalfOpen:
		return cb.halfOpenCalls < cb.halfOpenMaxCalls
	default:
		return false
	}
}

// onSuccess is called when a call succeeds
func (cb *CircuitBreaker) onSuccess() {
	cb.totalSuccesses++
	cb.consecutiveSuccesses++
	cb.consecutiveFailures = 0
	
	switch cb.state {
	case StateHalfOpen:
		// Any success in half-open state closes the circuit
		cb.state = StateClosed
		cb.failures = 0
		cb.halfOpenCalls = 0
	case StateClosed:
		// Reset failure count on any success in closed state
		cb.failures = 0
	}
}

// onFailure is called when a call fails
func (cb *CircuitBreaker) onFailure() {
	cb.totalFailures++
	cb.consecutiveFailures++
	cb.consecutiveSuccesses = 0
	cb.failures++
	cb.lastFailTime = time.Now()
	
	switch cb.state {
	case StateClosed:
		// Check if we should open the circuit
		if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
		}
	case StateHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.state = StateOpen
		cb.halfOpenCalls = 0
	}
}

// shouldAttemptReset checks if circuit breaker should attempt reset
func (cb *CircuitBreaker) shouldAttemptReset() bool {
	return cb.state == StateOpen && time.Since(cb.lastFailTime) >= cb.resetTimeout
}

// Utility functions for testing and demonstration

// SimulateExternalService simulates an external service call
func SimulateExternalService(shouldFail bool, delay time.Duration) func() (interface{}, error) {
	return func() (interface{}, error) {
		time.Sleep(delay)
		if shouldFail {
			return nil, fmt.Errorf("external service error")
		}
		return "service response", nil
	}
}

// CallWithTimeout wraps a circuit breaker call with context timeout
func CallWithTimeout(cb *CircuitBreaker, fn func() (interface{}, error), timeout time.Duration) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		value interface{}
		err   error
	}

	ch := make(chan result, 1)
	
	go func() {
		val, err := cb.Call(fn)
		ch <- result{val, err}
	}()

	select {
	case res := <-ch:
		return res.value, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CallWithFallback executes a function with circuit breaker and fallback
func CallWithFallback(cb *CircuitBreaker, fn func() (interface{}, error), fallback func() (interface{}, error)) (interface{}, error) {
	result, err := cb.Call(fn)
	if err != nil {
		// Check if it's a circuit breaker error
		if _, ok := err.(*CircuitBreakerOpenError); ok {
			// Circuit is open, use fallback
			return fallback()
		}
		// Function error, use fallback
		return fallback()
	}
	return result, nil
}

// Advanced functionality

// Sequence executes multiple operations with circuit breaker protection
func Sequence(cb *CircuitBreaker, operations ...func() (interface{}, error)) ([]interface{}, error) {
	results := make([]interface{}, 0, len(operations))
	
	for _, op := range operations {
		result, err := cb.Call(op)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	
	return results, nil
}

// Retry retries an operation with circuit breaker protection
func Retry(cb *CircuitBreaker, fn func() (interface{}, error), maxAttempts int, delay time.Duration) (interface{}, error) {
	var lastErr error
	
	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err := cb.Call(fn)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		
		// If circuit breaker is open, don't retry
		if _, ok := err.(*CircuitBreakerOpenError); ok {
			return nil, err
		}
		
		if attempt < maxAttempts-1 {
			time.Sleep(delay)
		}
	}
	
	return nil, lastErr
}

// Monitor provides circuit breaker monitoring capabilities
type Monitor struct {
	cb       *CircuitBreaker
	interval time.Duration
	stopCh   chan struct{}
	mu       sync.Mutex
	running  bool
}

// NewMonitor creates a new circuit breaker monitor
func NewMonitor(cb *CircuitBreaker, interval time.Duration) *Monitor {
	return &Monitor{
		cb:       cb,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins monitoring the circuit breaker
func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.running {
		return
	}
	
	m.running = true
	
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				state := m.cb.GetState()
				counts := m.cb.GetCounts()
				fmt.Printf("[Circuit Breaker Monitor] State: %s, Requests: %d, Successes: %d, Failures: %d\n",
					state, counts.Requests, counts.TotalSuccesses, counts.TotalFailures)
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop stops monitoring the circuit breaker
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.running {
		return
	}
	
	m.running = false
	close(m.stopCh)
}

func main() {
	fmt.Println("=== Circuit Breaker Pattern Demo ===")
	
	// Circuit Breakerの設定
	settings := Settings{
		MaxFailures:      3,
		ResetTimeout:     5 * time.Second,
		HalfOpenMaxCalls: 2,
	}
	
	cb := NewCircuitBreaker(settings)
	
	// 正常なサービス呼び出しのテスト
	fmt.Println("\n--- Testing Normal Operation ---")
	for i := 0; i < 3; i++ {
		result, err := cb.Call(SimulateExternalService(false, 100*time.Millisecond))
		fmt.Printf("Call %d: State=%s, Result=%v, Error=%v\n", 
			i+1, cb.GetState(), result, err)
	}
	
	// 失敗を発生させてCircuit Breakerを開く
	fmt.Println("\n--- Testing Failure Cases ---")
	for i := 0; i < 5; i++ {
		result, err := cb.Call(SimulateExternalService(true, 50*time.Millisecond))
		fmt.Printf("Failure %d: State=%s, Result=%v, Error=%v\n", 
			i+1, cb.GetState(), result, err)
	}
	
	// Open状態での即座の失敗を確認
	fmt.Println("\n--- Testing Open State ---")
	for i := 0; i < 3; i++ {
		result, err := cb.Call(SimulateExternalService(false, 10*time.Millisecond))
		fmt.Printf("Open call %d: State=%s, Result=%v, Error=%v\n", 
			i+1, cb.GetState(), result, err)
	}
	
	// Half-Open状態への移行待ち
	fmt.Println("\n--- Waiting for Half-Open State ---")
	fmt.Printf("Waiting %v for circuit breaker to attempt reset...\n", settings.ResetTimeout)
	time.Sleep(settings.ResetTimeout + 100*time.Millisecond)
	
	// Half-Open状態での試行
	fmt.Println("\n--- Testing Half-Open State ---")
	for i := 0; i < 3; i++ {
		result, err := cb.Call(SimulateExternalService(false, 50*time.Millisecond))
		fmt.Printf("Half-open call %d: State=%s, Result=%v, Error=%v\n", 
			i+1, cb.GetState(), result, err)
		time.Sleep(100 * time.Millisecond)
	}
	
	// 最終状態の確認
	fmt.Printf("\nFinal State: %s\n", cb.GetState())
	counts := cb.GetCounts()
	fmt.Printf("Final Counts: %+v\n", counts)
}