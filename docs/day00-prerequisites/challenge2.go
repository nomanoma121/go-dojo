//go:build ignore

package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// TODO: 実践課題2を実装してください
//
// 以下の仕様で並行処理プログラムを作成してください：
// 1. 複数のURLに対して並行してHTTPリクエストを送信
// 2. 全てのレスポンスを待つ
// 3. 結果をまとめて返す
// 4. タイムアウト処理（5秒）
// 5. エラーハンドリング

type Result struct {
	URL      string        `json:"url"`
	Status   int           `json:"status"`
	Duration time.Duration `json:"duration"`
	Error    error         `json:"error,omitempty"`
}

// TODO: この関数を実装してください
func fetchURLs(urls []string) []Result {
	// ヒント: 
	// 1. context.WithTimeout() でタイムアウト設定
	// 2. sync.WaitGroup で並行処理の完了を待つ
	// 3. チャネルまたはスライスで結果を収集
	// 4. Goroutineで各URLを並行処理
	
	return nil
}

// TODO: この関数を実装してください
func fetchURL(ctx context.Context, url string) Result {
	// ヒント:
	// 1. time.Now() で開始時刻を記録
	// 2. http.NewRequestWithContext() でコンテキスト付きリクエスト作成
	// 3. http.Client.Do() でリクエスト実行
	// 4. time.Since() で処理時間を計算
	// 5. エラーハンドリング
	
	return Result{}
}

func main() {
	// テスト用URL
	urls := []string{
		"https://httpbin.org/delay/1",
		"https://httpbin.org/delay/2", 
		"https://httpbin.org/status/200",
		"https://httpbin.org/status/404",
		"https://httpbin.org/status/500",
		"https://invalid-url-that-should-fail.com",
	}
	
	fmt.Println("Fetching URLs concurrently...")
	start := time.Now()
	
	results := fetchURLs(urls)
	
	fmt.Printf("Completed in %v\n\n", time.Since(start))
	
	// 結果表示
	for _, result := range results {
		if result.Error != nil {
			fmt.Printf("URL: %s\n  Error: %v\n  Duration: %v\n\n", 
				result.URL, result.Error, result.Duration)
		} else {
			fmt.Printf("URL: %s\n  Status: %d\n  Duration: %v\n\n", 
				result.URL, result.Status, result.Duration)
		}
	}
}