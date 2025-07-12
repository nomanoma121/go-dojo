package main

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// APIResponse represents a response from an external API
type APIResponse struct {
	Data      string
	Status    int
	Timestamp time.Time
	Duration  time.Duration
}

// APIError represents an API error
type APIError struct {
	Message string
	Code    int
}

func (e *APIError) Error() string {
	return e.Message
}

// mockExternalAPI simulates an external API call with variable response time
var mockExternalAPI = func(url string, simulatedDelay time.Duration) (*APIResponse, error) {
	// Simulate network delay
	time.Sleep(simulatedDelay)
	
	// Simulate occasional failures
	if rand.Float32() < 0.1 { // 10% chance of failure
		return nil, &APIError{Message: "simulated API failure", Code: 500}
	}
	
	return &APIResponse{
		Data:      "response from " + url,
		Status:    200,
		Timestamp: time.Now(),
		Duration:  simulatedDelay,
	}, nil
}

// APICallWithTimeout は外部APIを呼び出し、タイムアウトを設定する
func APICallWithTimeout(ctx context.Context, url string, timeout time.Duration) (*APIResponse, error) {
	// 1. context.WithTimeout()でタイムアウト付きコンテキストを作成
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// 2. Goroutineで実際のAPI呼び出しを実行
	resultChan := make(chan *APIResponse, 1)
	errorChan := make(chan error, 1)
	
	go func() {
		// 実際のAPI呼び出し（モック）
		delay := time.Duration(rand.Intn(2000)) * time.Millisecond
		resp, err := mockExternalAPI(url, delay)
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- resp
		}
	}()
	
	// 3. selectでタイムアウトと正常完了を監視
	select {
	case <-timeoutCtx.Done():
		return nil, timeoutCtx.Err()
	case resp := <-resultChan:
		return resp, nil
	case err := <-errorChan:
		return nil, err
	}
}

// APICallWithDeadline は絶対時刻でのデッドラインを設定してAPIを呼び出す
func APICallWithDeadline(ctx context.Context, url string, deadline time.Time) (*APIResponse, error) {
	// 1. context.WithDeadline()でデッドライン付きコンテキストを作成
	deadlineCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	
	// 2. デッドラインまでの残り時間を確認
	if time.Until(deadline) <= 0 {
		return nil, context.DeadlineExceeded
	}
	
	// 3. API呼び出しを実行
	return APICallWithTimeout(deadlineCtx, url, time.Until(deadline))
}

// APICallWithRetry はタイムアウト付きでリトライ機能を持つAPI呼び出し
func APICallWithRetry(ctx context.Context, url string, timeout time.Duration, maxRetries int) (*APIResponse, error) {
	var lastErr error
	baseDelay := 100 * time.Millisecond
	
	// maxRetries回の試行（1回目 + (maxRetries-1)回のリトライ）
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 1. 各試行でタイムアウトを設定
		resp, err := APICallWithTimeout(ctx, url, timeout)
		if err == nil {
			return resp, nil
		}
		
		lastErr = err
		
		// コンテキストがキャンセルされた場合は即座に終了
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		
		// 最後の試行の場合はリトライしない
		if attempt >= maxRetries {
			break
		}
		
		// 2. 指数バックオフで待機
		delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
		
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
			// 次の試行に進む
		}
	}
	
	return nil, lastErr
}