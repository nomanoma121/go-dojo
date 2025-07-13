package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxRetries int           // 最大リトライ回数
	BaseDelay  time.Duration // 基本遅延時間
	MaxDelay   time.Duration // 最大遅延時間
	Multiplier float64       // 指数の倍数（通常は2.0）
	Jitter     bool          // ジッターの有効/無効
}

// RetryableFunc represents a function that can be retried
type RetryableFunc func() error

// RetryStatistics holds statistics about retry operations
type RetryStatistics struct {
	TotalAttempts    int64
	TotalSuccesses   int64
	TotalFailures    int64
	AverageAttempts  float64
	TotalRetryTime   time.Duration
	LastError        error
	mutex            sync.RWMutex
}

// RetryManager manages retry operations with exponential backoff
type RetryManager struct {
	config RetryConfig
	stats  *RetryStatistics
}

// NewRetryManager creates a new RetryManager with the given configuration
func NewRetryManager(config RetryConfig) *RetryManager {
	if err := validateConfig(config); err != nil {
		panic(fmt.Sprintf("invalid retry config: %v", err))
	}

	return &RetryManager{
		config: config,
		stats: &RetryStatistics{
			mutex: sync.RWMutex{},
		},
	}
}

// Execute executes the given function with retry logic
func (rm *RetryManager) Execute(fn RetryableFunc) error {
	return rm.ExecuteWithContext(context.Background(), fn)
}

// ExecuteWithContext executes the given function with retry logic and context support
func (rm *RetryManager) ExecuteWithContext(ctx context.Context, fn RetryableFunc) error {
	start := time.Now()
	var lastErr error
	attempts := int64(0)

	for attempt := 0; attempt <= rm.config.MaxRetries; attempt++ {
		// コンテキストのキャンセレーションをチェック
		select {
		case <-ctx.Done():
			rm.updateStats(attempts, false, time.Since(start), lastErr)
			return ctx.Err()
		default:
		}

		attempts++
		lastErr = fn()

		if lastErr == nil {
			// 成功
			rm.updateStats(attempts, true, time.Since(start), nil)
			return nil
		}

		// 再試行不可能なエラーの場合は即座に終了
		if !isRetryableError(lastErr) {
			rm.updateStats(attempts, false, time.Since(start), lastErr)
			return lastErr
		}

		// 最大試行回数に達した場合は終了
		if attempt >= rm.config.MaxRetries {
			break
		}

		// 次の試行前に待機
		delay := rm.calculateDelay(attempt)
		select {
		case <-ctx.Done():
			rm.updateStats(attempts, false, time.Since(start), lastErr)
			return ctx.Err()
		case <-time.After(delay):
			// 続行
		}
	}

	// 全ての試行が失敗
	rm.updateStats(attempts, false, time.Since(start), lastErr)
	return fmt.Errorf("operation failed after %d retries: %w", rm.config.MaxRetries, lastErr)
}

// calculateDelay calculates the delay for the given attempt number
func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
	delay := float64(rm.config.BaseDelay) * math.Pow(rm.config.Multiplier, float64(attempt))

	// 最大遅延時間の制限
	if rm.config.MaxDelay > 0 && time.Duration(delay) > rm.config.MaxDelay {
		delay = float64(rm.config.MaxDelay)
	}

	// ジッターの追加
	if rm.config.Jitter {
		return addJitter(time.Duration(delay), 0.25) // ±25%のジッター
	}

	return time.Duration(delay)
}

// updateStats updates retry statistics
func (rm *RetryManager) updateStats(attempts int64, success bool, duration time.Duration, err error) {
	rm.stats.mutex.Lock()
	defer rm.stats.mutex.Unlock()

	rm.stats.TotalAttempts += attempts
	rm.stats.TotalRetryTime += duration
	rm.stats.LastError = err

	if success {
		rm.stats.TotalSuccesses++
	} else {
		rm.stats.TotalFailures++
	}

	// 平均試行回数を計算
	totalOps := rm.stats.TotalSuccesses + rm.stats.TotalFailures
	if totalOps > 0 {
		rm.stats.AverageAttempts = float64(rm.stats.TotalAttempts) / float64(totalOps)
	}
}

// GetStatistics returns current retry statistics
func (rm *RetryManager) GetStatistics() RetryStatistics {
	rm.stats.mutex.RLock()
	defer rm.stats.mutex.RUnlock()

	// コピーを返す
	return RetryStatistics{
		TotalAttempts:   rm.stats.TotalAttempts,
		TotalSuccesses:  rm.stats.TotalSuccesses,
		TotalFailures:   rm.stats.TotalFailures,
		AverageAttempts: rm.stats.AverageAttempts,
		TotalRetryTime:  rm.stats.TotalRetryTime,
		LastError:       rm.stats.LastError,
	}
}

// ResetStatistics resets all statistics
func (rm *RetryManager) ResetStatistics() {
	rm.stats.mutex.Lock()
	defer rm.stats.mutex.Unlock()

	rm.stats.TotalAttempts = 0
	rm.stats.TotalSuccesses = 0
	rm.stats.TotalFailures = 0
	rm.stats.AverageAttempts = 0
	rm.stats.TotalRetryTime = 0
	rm.stats.LastError = nil
}

// Database-specific retry functionality

// DatabaseRetryManager provides database-specific retry logic
type DatabaseRetryManager struct {
	*RetryManager
}

// NewDatabaseRetryManager creates a RetryManager optimized for database operations
func NewDatabaseRetryManager() *DatabaseRetryManager {
	config := RetryConfig{
		MaxRetries: 5,
		BaseDelay:  50 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		Multiplier: 2.0,
		Jitter:     true,
	}

	return &DatabaseRetryManager{
		RetryManager: NewRetryManager(config),
	}
}

// ExecuteQuery executes a database query with database-specific retry logic
func (drm *DatabaseRetryManager) ExecuteQuery(query func() error) error {
	return drm.ExecuteWithContext(context.Background(), func() error {
		err := query()
		if err != nil && !isRetryableDBError(err) {
			// データベース固有の再試行不可能エラーは即座に返す
			return err
		}
		return err
	})
}

// Error classification functions

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// 再試行可能なエラーパターン
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"server is not ready",
		"service unavailable",
		"too many requests",
		"rate limit",
		"network is unreachable",
		"no route to host",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// 非再試行エラーパターン
	nonRetryablePatterns := []string{
		"authentication",
		"authorization",
		"permission denied",
		"invalid parameter",
		"bad request",
		"not found",
		"forbidden",
		"syntax error",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// デフォルトでは再試行可能とする（保守的なアプローチ）
	return true
}

// isRetryableDBError determines if a database error should trigger a retry
func isRetryableDBError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// データベース特有の再試行可能エラー
	retryableDBPatterns := []string{
		"deadlock detected",
		"lock wait timeout",
		"connection timeout",
		"connection refused",
		"connection reset",
		"server has gone away",
		"too many connections",
		"database is locked",
		"temporary failure",
		"serialization failure",
		"could not serialize access",
	}

	for _, pattern := range retryableDBPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// データベース特有の非再試行エラー
	nonRetryableDBPatterns := []string{
		"syntax error",
		"column",
		"table",
		"constraint",
		"violation",
		"duplicate",
		"foreign key",
		"unique",
		"not null",
		"data too long",
		"out of range",
		"division by zero",
	}

	for _, pattern := range nonRetryableDBPatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// デフォルトでは一般的なリトライ判定に委ねる
	return isRetryableError(err)
}

// Utility functions

// validateConfig validates the retry configuration
func validateConfig(config RetryConfig) error {
	if config.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries must be >= 0, got %d", config.MaxRetries)
	}

	if config.BaseDelay <= 0 {
		return fmt.Errorf("BaseDelay must be > 0, got %v", config.BaseDelay)
	}

	if config.Multiplier <= 1.0 {
		return fmt.Errorf("Multiplier must be > 1.0, got %f", config.Multiplier)
	}

	if config.MaxDelay > 0 && config.MaxDelay < config.BaseDelay {
		return fmt.Errorf("MaxDelay (%v) must be >= BaseDelay (%v)", config.MaxDelay, config.BaseDelay)
	}

	return nil
}

// addJitter adds random jitter to the delay
func addJitter(delay time.Duration, jitterPercent float64) time.Duration {
	if jitterPercent <= 0 {
		return delay
	}

	// ±jitterPercent%の範囲でランダムな値を生成
	jitterRange := float64(delay) * jitterPercent
	jitter := (rand.Float64() - 0.5) * 2 * jitterRange

	newDelay := float64(delay) + jitter
	if newDelay < 0 {
		newDelay = float64(delay) * (1 - jitterPercent) // 最小値を保証
	}

	return time.Duration(newDelay)
}

// Advanced retry patterns

// RetryWithBackoff is a standalone function for simple exponential backoff retry
func RetryWithBackoff(maxRetries int, baseDelay time.Duration, fn RetryableFunc) error {
	config := RetryConfig{
		MaxRetries: maxRetries,
		BaseDelay:  baseDelay,
		Multiplier: 2.0,
		Jitter:     false,
	}

	rm := NewRetryManager(config)
	return rm.Execute(fn)
}

// RetryWithTimeout executes function with both retry and total timeout
func RetryWithTimeout(timeout time.Duration, config RetryConfig, fn RetryableFunc) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rm := NewRetryManager(config)
	return rm.ExecuteWithContext(ctx, fn)
}

// CircuitBreakerRetry combines circuit breaker pattern with retry
type CircuitBreakerRetry struct {
	retryManager    *RetryManager
	failureCount    int64
	lastFailureTime int64 // Unix nano
	threshold       int64
	resetTimeout    time.Duration
	mutex           sync.RWMutex
}

// NewCircuitBreakerRetry creates a new circuit breaker with retry
func NewCircuitBreakerRetry(config RetryConfig, threshold int64, resetTimeout time.Duration) *CircuitBreakerRetry {
	return &CircuitBreakerRetry{
		retryManager: NewRetryManager(config),
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

// Execute executes function with circuit breaker and retry logic
func (cbr *CircuitBreakerRetry) Execute(fn RetryableFunc) error {
	if cbr.isCircuitOpen() {
		return fmt.Errorf("circuit breaker is open")
	}

	err := cbr.retryManager.Execute(fn)
	if err != nil {
		cbr.recordFailure()
	} else {
		cbr.recordSuccess()
	}

	return err
}

// isCircuitOpen checks if the circuit breaker is open
func (cbr *CircuitBreakerRetry) isCircuitOpen() bool {
	cbr.mutex.RLock()
	defer cbr.mutex.RUnlock()

	if atomic.LoadInt64(&cbr.failureCount) < cbr.threshold {
		return false
	}

	// 閾値を超えている場合、リセット時間を確認
	lastFailure := time.Unix(0, atomic.LoadInt64(&cbr.lastFailureTime))
	return time.Since(lastFailure) < cbr.resetTimeout
}

// recordFailure records a failure
func (cbr *CircuitBreakerRetry) recordFailure() {
	atomic.AddInt64(&cbr.failureCount, 1)
	atomic.StoreInt64(&cbr.lastFailureTime, time.Now().UnixNano())
}

// recordSuccess records a success and resets failure count
func (cbr *CircuitBreakerRetry) recordSuccess() {
	atomic.StoreInt64(&cbr.failureCount, 0)
}

func main() {
	// デモンストレーション用のコード

	// 基本的なリトライの例
	fmt.Println("=== Basic Retry Example ===")
	config := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     true,
	}

	rm := NewRetryManager(config)

	// 失敗する関数の例
	attempt := 0
	err := rm.Execute(func() error {
		attempt++
		fmt.Printf("Attempt %d\n", attempt)
		if attempt < 3 {
			return fmt.Errorf("temporary failure")
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Failed: %v\n", err)
	} else {
		fmt.Println("Success!")
	}

	// 統計情報の表示
	stats := rm.GetStatistics()
	fmt.Printf("Statistics: Attempts=%d, Successes=%d, Failures=%d, AvgAttempts=%.2f\n",
		stats.TotalAttempts, stats.TotalSuccesses, stats.TotalFailures, stats.AverageAttempts)

	// データベースリトライの例
	fmt.Println("\n=== Database Retry Example ===")
	drm := NewDatabaseRetryManager()

	dbAttempt := 0
	err = drm.ExecuteQuery(func() error {
		dbAttempt++
		fmt.Printf("DB Query Attempt %d\n", dbAttempt)
		if dbAttempt < 2 {
			return fmt.Errorf("deadlock detected")
		}
		return nil
	})

	if err != nil {
		fmt.Printf("DB Query Failed: %v\n", err)
	} else {
		fmt.Println("DB Query Success!")
	}

	// サーキットブレーカー付きリトライの例
	fmt.Println("\n=== Circuit Breaker Retry Example ===")
	cbr := NewCircuitBreakerRetry(config, 2, 1*time.Second)

	for i := 0; i < 5; i++ {
		err := cbr.Execute(func() error {
			return fmt.Errorf("service unavailable")
		})
		fmt.Printf("Circuit Breaker Attempt %d: %v\n", i+1, err)
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("Exponential backoff retry demo completed!")
}