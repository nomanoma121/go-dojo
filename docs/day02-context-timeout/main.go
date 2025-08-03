//go:build ignore

package main

import (
	"context"
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

// APIError represents an API error
type APIError struct {
	Message string
	Code    int
}

func (e *APIError) Error() string {
	return e.Message
}

// APICallWithTimeout は外部APIを呼び出し、タイムアウトを設定する
func APICallWithTimeout(ctx context.Context, url string, timeout time.Duration) (*APIResponse, error) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. context.WithTimeout()でタイムアウト付きコンテキストを作成
	// 2. Goroutineで実際のAPI呼び出しを実行
	// 3. selectでタイムアウトと正常完了を監視
	// 4. 適切なエラーハンドリングを実装
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultChan := make(chan *APIResponse, 1)
	errorChan := make(chan error, 1)

	go func() {
		delay := time.Duration(rand.Intn(2000)) * time.Millisecond
		resp, err := mockExternalAPI(url, delay)
		if err != nil {
			errorChan <- err
		} else {
			resultChan <- resp
		}	
	}()

	select {
	case <-timeoutCtx.Done():
		return nil, timeoutCtx.Err() // タイムアウトエラーを返す
	case resp := <-resultChan:
		return resp, nil // 正常なレスポンスを返す
	case err := <-errorChan:
		return nil, err // API呼び出しのエラーを返す
	}
}

// APICallWithDeadline は絶対時刻でのデッドラインを設定してAPIを呼び出す
func APICallWithDeadline(ctx context.Context, url string, deadline time.Time) (*APIResponse, error) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. context.WithDeadline()でデッドライン付きコンテキストを作成
	// 2. デッドラインまでの残り時間を確認
	// 3. API呼び出しを実行
	// 4. デッドライン超過時の適切な処理
	deadlineCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	
	if time.Until(deadline) <= 0 {
		return nil, context.DeadlineExceeded // デッドライン超過エラーを返す
	}

	resp, err := APICallWithTimeout(deadlineCtx, url, time.Until(deadline))
	if err != nil {
		return nil, err // API呼び出しのエラーを返す
	}
	
	return resp, nil // 正常なレスポンスを返す
}

// APICallWithRetry はタイムアウト付きでリトライ機能を持つAPI呼び出し
func APICallWithRetry(ctx context.Context, url string, timeout time.Duration, maxRetries int) (*APIResponse, error) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 指定回数まで繰り返し試行
	// 2. 各試行でタイムアウトを設定
	// 3. 失敗時は指数バックオフで待機
	// 4. コンテキストキャンセルのチェック
	
	return nil, nil
}

func main() {
	ctx := context.Background()
	
	// テスト用のサンプル実行
	resp, err := APICallWithTimeout(ctx, "https://api.example.com/data", 2*time.Second)
	if err != nil {
		panic(err)
	}
	
	_ = resp
}
