//go:build ignore

package main

import (
	"context"
	"errors"
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

	state         CircuitBreakerState
	failures      int
	lastFailTime  time.Time
	halfOpenCalls int
	successCount  int
	totalCalls    int

	mutex sync.RWMutex
}

// Counts represents the current statistics of the circuit breaker
type Counts struct {
	Requests        uint64
	TotalSuccesses  uint64
	TotalFailures   uint64
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
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. CircuitBreakerを初期化
	// 2. 設定値を設定
	// 3. 初期状態はStateClosed
	// 4. カウンターを初期化
	panic("Not yet implemented")
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 現在の状態を確認
	// 2. 状態に応じて処理を分岐:
	//    - StateClosed: そのまま関数を実行
	//    - StateOpen: タイムアウトをチェックしてHalf-Openか判定
	//    - StateHalfOpen: 試行回数をチェック
	// 3. 関数を実行して結果を記録
	// 4. 成功・失敗に応じて状態を更新
	panic("Not yet implemented")
}

// GetState returns current state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. RLockで読み取り専用ロックを取得
	// 2. 現在の状態を返す
	panic("Not yet implemented")
}

// GetCounts returns current statistics
func (cb *CircuitBreaker) GetCounts() Counts {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. RLockで読み取り専用ロックを取得
	// 2. 現在の統計情報を Counts 構造体にまとめて返す
	panic("Not yet implemented")
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. Lockで書き込みロックを取得
	// 2. 状態をStateClosedにリセット
	// 3. すべてのカウンターをリセット
	panic("Not yet implemented")
}

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 現在の状態を確認
	// 2. StateClosed: true
	// 3. StateOpen: タイムアウトをチェックしてfalseかHalf-Openに移行
	// 4. StateHalfOpen: 試行回数をチェック
	panic("Not yet implemented")
}

// Private helper methods (実装のヒント)

// onSuccess is called when a call succeeds
func (cb *CircuitBreaker) onSuccess() {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 成功カウンターを更新
	// 2. StateHalfOpenの場合はStateClosedに移行を検討
	// 3. 失敗カウンターをリセット
	panic("Not yet implemented")
}

// onFailure is called when a call fails
func (cb *CircuitBreaker) onFailure() {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 失敗カウンターを更新
	// 2. lastFailTimeを更新
	// 3. 失敗回数が閾値を超えたらStateOpenに移行
	panic("Not yet implemented")
}

// shouldAttemptReset checks if circuit breaker should attempt reset
func (cb *CircuitBreaker) shouldAttemptReset() bool {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. StateOpenかつ十分な時間が経過している場合にtrue
	// 2. time.Since(cb.lastFailTime) >= cb.resetTimeout
	panic("Not yet implemented")
}

// Utility functions for testing and demonstration

// SimulateExternalService simulates an external service call
func SimulateExternalService(shouldFail bool, delay time.Duration) func() (interface{}, error) {
	return func() (interface{}, error) {
		time.Sleep(delay)
		if shouldFail {
			return nil, errors.New("external service error")
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