package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRetryManager_Execute(t *testing.T) {
	tests := []struct {
		name        string
		config      RetryConfig
		fn          RetryableFunc
		wantErr     bool
		wantRetries int
	}{
		{
			name: "immediate success",
			config: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  100 * time.Millisecond,
				Multiplier: 2.0,
				Jitter:     false,
			},
			fn: func() error {
				return nil
			},
			wantErr:     false,
			wantRetries: 1,
		},
		{
			name: "success after retries",
			config: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  10 * time.Millisecond,
				Multiplier: 2.0,
				Jitter:     false,
			},
			fn: func() func() error {
				attempt := 0
				return func() error {
					attempt++
					if attempt < 3 {
						return errors.New("temporary failure")
					}
					return nil
				}
			}(),
			wantErr:     false,
			wantRetries: 3,
		},
		{
			name: "max retries exceeded",
			config: RetryConfig{
				MaxRetries: 2,
				BaseDelay:  10 * time.Millisecond,
				Multiplier: 2.0,
				Jitter:     false,
			},
			fn: func() error {
				return errors.New("persistent failure")
			},
			wantErr:     true,
			wantRetries: 3, // 初回 + 2回のリトライ
		},
		{
			name: "non-retryable error",
			config: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  10 * time.Millisecond,
				Multiplier: 2.0,
				Jitter:     false,
			},
			fn: func() error {
				return errors.New("authentication failed")
			},
			wantErr:     true,
			wantRetries: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := NewRetryManager(tt.config)
			err := rm.Execute(tt.fn)

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			stats := rm.GetStatistics()
			if stats.TotalAttempts != int64(tt.wantRetries) {
				t.Errorf("Expected %d attempts, got %d", tt.wantRetries, stats.TotalAttempts)
			}
		})
	}
}

func TestRetryManager_ExecuteWithContext(t *testing.T) {
	tests := []struct {
		name           string
		config         RetryConfig
		contextTimeout time.Duration
		fn             RetryableFunc
		wantErr        bool
		expectTimeout  bool
	}{
		{
			name: "success before timeout",
			config: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  10 * time.Millisecond,
				Multiplier: 2.0,
				Jitter:     false,
			},
			contextTimeout: 1 * time.Second,
			fn: func() error {
				return nil
			},
			wantErr:       false,
			expectTimeout: false,
		},
		{
			name: "context timeout during retry",
			config: RetryConfig{
				MaxRetries: 5,
				BaseDelay:  100 * time.Millisecond,
				Multiplier: 2.0,
				Jitter:     false,
			},
			contextTimeout: 150 * time.Millisecond,
			fn: func() error {
				return errors.New("temporary failure")
			},
			wantErr:       true,
			expectTimeout: true,
		},
		{
			name: "context timeout during operation",
			config: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  10 * time.Millisecond,
				Multiplier: 2.0,
				Jitter:     false,
			},
			contextTimeout: 50 * time.Millisecond,
			fn: func() error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			wantErr:       true,
			expectTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := NewRetryManager(tt.config)
			ctx, cancel := context.WithTimeout(context.Background(), tt.contextTimeout)
			defer cancel()

			err := rm.ExecuteWithContext(ctx, tt.fn)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteWithContext() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.expectTimeout && err != context.DeadlineExceeded && err != context.Canceled {
				t.Errorf("Expected timeout error, got %v", err)
			}
		})
	}
}

func TestRetryManager_ExponentialBackoff(t *testing.T) {
	config := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   1 * time.Second,
		Multiplier: 2.0,
		Jitter:     false,
	}

	rm := NewRetryManager(config)

	expectedDelays := []time.Duration{
		100 * time.Millisecond, // 2^0 * 100ms
		200 * time.Millisecond, // 2^1 * 100ms
		400 * time.Millisecond, // 2^2 * 100ms
	}

	for i, expected := range expectedDelays {
		actual := rm.calculateDelay(i)
		if actual != expected {
			t.Errorf("calculateDelay(%d) = %v, want %v", i, actual, expected)
		}
	}
}

func TestRetryManager_ExponentialBackoffWithMaxDelay(t *testing.T) {
	config := RetryConfig{
		MaxRetries: 5,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   300 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     false,
	}

	rm := NewRetryManager(config)

	// MaxDelayを超える場合は制限される
	delay := rm.calculateDelay(3) // 2^3 * 100ms = 800ms だが MaxDelay=300ms
	if delay != 300*time.Millisecond {
		t.Errorf("calculateDelay(3) = %v, want %v", delay, 300*time.Millisecond)
	}
}

func TestRetryManager_Jitter(t *testing.T) {
	config := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     true,
	}

	rm := NewRetryManager(config)

	// ジッターが有効な場合、同じ試行回数でも異なる遅延時間が返される可能性がある
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = rm.calculateDelay(1)
	}

	// 全て同じ値でないことを確認（ジッターが効いている）
	allSame := true
	firstDelay := delays[0]
	for _, delay := range delays[1:] {
		if delay != firstDelay {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected jitter to produce different delays, but all delays were the same")
	}
}

func TestRetryManager_Statistics(t *testing.T) {
	config := RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     false,
	}

	rm := NewRetryManager(config)

	// 成功ケース
	err := rm.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// 失敗ケース
	err = rm.Execute(func() error {
		return errors.New("persistent failure")
	})
	if err == nil {
		t.Fatal("Expected failure, got success")
	}

	stats := rm.GetStatistics()

	if stats.TotalAttempts == 0 {
		t.Error("Expected TotalAttempts > 0")
	}

	if stats.TotalSuccesses != 1 {
		t.Errorf("Expected TotalSuccesses = 1, got %d", stats.TotalSuccesses)
	}

	if stats.TotalFailures != 1 {
		t.Errorf("Expected TotalFailures = 1, got %d", stats.TotalFailures)
	}

	if stats.AverageAttempts == 0 {
		t.Error("Expected AverageAttempts > 0")
	}

	// 統計リセット
	rm.ResetStatistics()
	stats = rm.GetStatistics()

	if stats.TotalAttempts != 0 {
		t.Errorf("Expected TotalAttempts = 0 after reset, got %d", stats.TotalAttempts)
	}
}

func TestDatabaseRetryManager_ExecuteQuery(t *testing.T) {
	drm := NewDatabaseRetryManager()

	tests := []struct {
		name    string
		query   func() error
		wantErr bool
	}{
		{
			name: "successful query",
			query: func() error {
				return nil
			},
			wantErr: false,
		},
		{
			name: "retryable database error",
			query: func() func() error {
				attempt := 0
				return func() error {
					attempt++
					if attempt < 3 {
						return errors.New("deadlock detected")
					}
					return nil
				}
			}(),
			wantErr: false,
		},
		{
			name: "non-retryable database error",
			query: func() error {
				return errors.New("syntax error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := drm.ExecuteQuery(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		want  bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "connection refused",
			err:  errors.New("connection refused"),
			want: true,
		},
		{
			name: "timeout error",
			err:  errors.New("operation timeout"),
			want: true,
		},
		{
			name: "temporary failure",
			err:  errors.New("temporary failure in name resolution"),
			want: true,
		},
		{
			name: "authentication error",
			err:  errors.New("authentication failed"),
			want: false,
		},
		{
			name: "invalid parameter",
			err:  errors.New("invalid parameter value"),
			want: false,
		},
		{
			name: "permission denied",
			err:  errors.New("permission denied"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableError(tt.err); got != tt.want {
				t.Errorf("isRetryableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRetryableDBError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "deadlock detected",
			err:  errors.New("deadlock detected"),
			want: true,
		},
		{
			name: "lock wait timeout",
			err:  errors.New("lock wait timeout exceeded"),
			want: true,
		},
		{
			name: "connection timeout",
			err:  errors.New("connection timeout"),
			want: true,
		},
		{
			name: "syntax error",
			err:  errors.New("syntax error near 'FROM'"),
			want: false,
		},
		{
			name: "constraint violation",
			err:  errors.New("foreign key constraint violation"),
			want: false,
		},
		{
			name: "table not found",
			err:  errors.New("table 'users' doesn't exist"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryableDBError(tt.err); got != tt.want {
				t.Errorf("isRetryableDBError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetryWithBackoff(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		baseDelay  time.Duration
		fn         RetryableFunc
		wantErr    bool
	}{
		{
			name:       "immediate success",
			maxRetries: 3,
			baseDelay:  10 * time.Millisecond,
			fn: func() error {
				return nil
			},
			wantErr: false,
		},
		{
			name:       "success after retries",
			maxRetries: 3,
			baseDelay:  10 * time.Millisecond,
			fn: func() func() error {
				attempt := 0
				return func() error {
					attempt++
					if attempt < 3 {
						return errors.New("temporary failure")
					}
					return nil
				}
			}(),
			wantErr: false,
		},
		{
			name:       "max retries exceeded",
			maxRetries: 2,
			baseDelay:  10 * time.Millisecond,
			fn: func() error {
				return errors.New("persistent failure")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RetryWithBackoff(tt.maxRetries, tt.baseDelay, tt.fn)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryWithBackoff() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRetryWithTimeout(t *testing.T) {
	config := RetryConfig{
		MaxRetries: 5,
		BaseDelay:  100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     false,
	}

	tests := []struct {
		name           string
		timeout        time.Duration
		fn             RetryableFunc
		wantErr        bool
		expectTimeout  bool
	}{
		{
			name:    "success before timeout",
			timeout: 1 * time.Second,
			fn: func() error {
				return nil
			},
			wantErr:       false,
			expectTimeout: false,
		},
		{
			name:    "timeout during retries",
			timeout: 150 * time.Millisecond,
			fn: func() error {
				return errors.New("temporary failure")
			},
			wantErr:       true,
			expectTimeout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RetryWithTimeout(tt.timeout, config, tt.fn)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryWithTimeout() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.expectTimeout && err != context.DeadlineExceeded {
				t.Errorf("Expected timeout error, got %v", err)
			}
		})
	}
}

func TestCircuitBreakerRetry_Execute(t *testing.T) {
	config := RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     false,
	}

	cbr := NewCircuitBreakerRetry(config, 3, 100*time.Millisecond)

	// 3回失敗させてサーキットを開く
	for i := 0; i < 3; i++ {
		err := cbr.Execute(func() error {
			return errors.New("service unavailable")
		})
		if err == nil {
			t.Errorf("Expected error on attempt %d", i+1)
		}
	}

	// サーキットが開いている間は即座にエラー
	err := cbr.Execute(func() error {
		return nil // この関数は実行されない
	})
	if err == nil {
		t.Error("Expected circuit breaker to be open")
	}

	// リセット時間経過後にサーキットが閉じることを確認
	time.Sleep(150 * time.Millisecond)
	err = cbr.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected circuit breaker to be closed after reset timeout, got error: %v", err)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  RetryConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  100 * time.Millisecond,
				MaxDelay:   1 * time.Second,
				Multiplier: 2.0,
				Jitter:     true,
			},
			wantErr: false,
		},
		{
			name: "negative max retries",
			config: RetryConfig{
				MaxRetries: -1,
				BaseDelay:  100 * time.Millisecond,
				Multiplier: 2.0,
			},
			wantErr: true,
		},
		{
			name: "zero base delay",
			config: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  0,
				Multiplier: 2.0,
			},
			wantErr: true,
		},
		{
			name: "invalid multiplier",
			config: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  100 * time.Millisecond,
				Multiplier: 0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddJitter(t *testing.T) {
	delay := 100 * time.Millisecond
	jitterPercent := 0.25 // ±25%

	// 複数回実行して、ジッターが適用されていることを確認
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = addJitter(delay, jitterPercent)
	}

	// 全ての値が基本遅延時間の75%〜125%の範囲内であることを確認
	minDelay := time.Duration(float64(delay) * 0.75)
	maxDelay := time.Duration(float64(delay) * 1.25)

	for i, d := range delays {
		if d < minDelay || d > maxDelay {
			t.Errorf("delays[%d] = %v, want between %v and %v", i, d, minDelay, maxDelay)
		}
	}

	// 異なる値が含まれていることを確認（ジッターが効いている）
	allSame := true
	firstDelay := delays[0]
	for _, d := range delays[1:] {
		if d != firstDelay {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected jitter to produce different delays, but all delays were the same")
	}
}

// ベンチマークテスト

func BenchmarkRetryManager_Execute(b *testing.B) {
	config := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Microsecond, // 短い遅延でベンチマーク
		Multiplier: 2.0,
		Jitter:     false,
	}

	rm := NewRetryManager(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rm.Execute(func() error {
			return nil
		})
	}
}

func BenchmarkRetryManager_CalculateDelay(b *testing.B) {
	config := RetryConfig{
		MaxRetries: 10,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   1 * time.Second,
		Multiplier: 2.0,
		Jitter:     true,
	}

	rm := NewRetryManager(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rm.calculateDelay(i % 5)
	}
}

func BenchmarkIsRetryableError(b *testing.B) {
	err := errors.New("connection refused")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isRetryableError(err)
	}
}

// Example Test
func ExampleRetryManager_Execute() {
	config := RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     false,
	}

	rm := NewRetryManager(config)

	err := rm.Execute(func() error {
		// 何らかの処理
		fmt.Println("Attempting operation...")
		return nil
	})

	if err != nil {
		fmt.Printf("Operation failed: %v\n", err)
	} else {
		fmt.Println("Operation succeeded")
	}

	// Output:
	// Attempting operation...
	// Operation succeeded
}