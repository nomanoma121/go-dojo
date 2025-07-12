package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAPICallWithTimeout(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		timeout         time.Duration
		simulatedDelay  time.Duration
		expectTimeout   bool
		expectError     bool
	}{
		{
			name:           "正常完了",
			url:            "https://api.example.com/fast",
			timeout:        2 * time.Second,
			simulatedDelay: 500 * time.Millisecond,
			expectTimeout:  false,
			expectError:    false,
		},
		{
			name:           "タイムアウト発生",
			url:            "https://api.example.com/slow",
			timeout:        1 * time.Second,
			simulatedDelay: 2 * time.Second,
			expectTimeout:  true,
			expectError:    true,
		},
		{
			name:           "ギリギリ完了",
			url:            "https://api.example.com/edge",
			timeout:        1 * time.Second,
			simulatedDelay: 900 * time.Millisecond,
			expectTimeout:  false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// テスト用にmockExternalAPIを直接使うためのバックアップ
			originalAPI := mockExternalAPI
			mockExternalAPI = func(url string, delay time.Duration) (*APIResponse, error) {
				return originalAPI(url, tt.simulatedDelay)
			}
			defer func() { mockExternalAPI = originalAPI }()

			start := time.Now()
			resp, err := APICallWithTimeout(context.Background(), tt.url, tt.timeout)
			elapsed := time.Since(start)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectTimeout {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("Expected timeout error, got: %v", err)
				}
				// タイムアウト時間付近で完了することを確認
				if elapsed > tt.timeout+200*time.Millisecond {
					t.Errorf("Timeout took too long: %v", elapsed)
				}
			} else {
				if resp == nil {
					t.Error("Expected response but got nil")
				}
			}
		})
	}
}

func TestAPICallWithDeadline(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		deadlineOffset  time.Duration  // 現在時刻からのオフセット
		simulatedDelay  time.Duration
		expectTimeout   bool
	}{
		{
			name:           "デッドライン内で完了",
			url:            "https://api.example.com/data",
			deadlineOffset: 2 * time.Second,
			simulatedDelay: 1 * time.Second,
			expectTimeout:  false,
		},
		{
			name:           "デッドライン超過",
			url:            "https://api.example.com/data",
			deadlineOffset: 1 * time.Second,
			simulatedDelay: 2 * time.Second,
			expectTimeout:  true,
		},
		{
			name:           "過去のデッドライン",
			url:            "https://api.example.com/data",
			deadlineOffset: -1 * time.Second,
			simulatedDelay: 100 * time.Millisecond,
			expectTimeout:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalAPI := mockExternalAPI
			mockExternalAPI = func(url string, delay time.Duration) (*APIResponse, error) {
				return originalAPI(url, tt.simulatedDelay)
			}
			defer func() { mockExternalAPI = originalAPI }()

			deadline := time.Now().Add(tt.deadlineOffset)
			resp, err := APICallWithDeadline(context.Background(), tt.url, deadline)

			if tt.expectTimeout {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("Expected deadline exceeded error, got: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("Expected response but got nil")
				}
			}
		})
	}
}

func TestAPICallWithRetry(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		timeout      time.Duration
		maxRetries   int
		failureRate  float32  // 失敗率（0.0-1.0）
		expectSuccess bool
	}{
		{
			name:         "即座に成功",
			url:          "https://api.example.com/reliable",
			timeout:      1 * time.Second,
			maxRetries:   3,
			failureRate:  0.0,
			expectSuccess: true,
		},
		{
			name:         "リトライで成功",
			url:          "https://api.example.com/unreliable",
			timeout:      1 * time.Second,
			maxRetries:   5,
			failureRate:  0.7,
			expectSuccess: true,  // リトライにより成功する可能性が高い
		},
		{
			name:         "全て失敗",
			url:          "https://api.example.com/broken",
			timeout:      200 * time.Millisecond,
			maxRetries:   2,
			failureRate:  1.0,
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts := 0
			originalAPI := mockExternalAPI
			mockExternalAPI = func(url string, delay time.Duration) (*APIResponse, error) {
				attempts++
				time.Sleep(50 * time.Millisecond) // 短い遅延
				
				// 指定された失敗率で失敗させる
				if float32(attempts) <= float32(tt.maxRetries)*tt.failureRate {
					return nil, &APIError{Message: "simulated failure", Code: 500}
				}
				
				return &APIResponse{
					Data:      "success after " + string(rune(attempts)) + " attempts",
					Status:    200,
					Timestamp: time.Now(),
				}, nil
			}
			defer func() { mockExternalAPI = originalAPI }()

			start := time.Now()
			resp, err := APICallWithRetry(context.Background(), tt.url, tt.timeout, tt.maxRetries)
			elapsed := time.Since(start)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
				if resp == nil {
					t.Error("Expected response but got nil")
				}
			} else {
				if err == nil {
					t.Error("Expected error but got success")
				}
			}

			// リトライの試行回数が期待値内かチェック
			if attempts > tt.maxRetries+1 {
				t.Errorf("Too many attempts: %d, max expected: %d", attempts, tt.maxRetries+1)
			}

			t.Logf("Test completed in %v with %d attempts", elapsed, attempts)
		})
	}
}

// ベンチマークテスト
func BenchmarkAPICallWithTimeout(b *testing.B) {
	originalAPI := mockExternalAPI
	mockExternalAPI = func(url string, delay time.Duration) (*APIResponse, error) {
		time.Sleep(10 * time.Millisecond) // 短い遅延
		return &APIResponse{Data: "benchmark", Status: 200}, nil
	}
	defer func() { mockExternalAPI = originalAPI }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := APICallWithTimeout(context.Background(), "https://api.example.com/bench", 1*time.Second)
		if err != nil {
			b.Fatalf("Benchmark failed: %v", err)
		}
	}
}

// コンテキストキャンセレーションのテスト
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// 500ms後にキャンセル
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	originalAPI := mockExternalAPI
	mockExternalAPI = func(url string, delay time.Duration) (*APIResponse, error) {
		time.Sleep(1 * time.Second) // 長い遅延
		return &APIResponse{Data: "should not reach here", Status: 200}, nil
	}
	defer func() { mockExternalAPI = originalAPI }()

	_, err := APICallWithTimeout(ctx, "https://api.example.com/slow", 2*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context canceled error, got: %v", err)
	}
}