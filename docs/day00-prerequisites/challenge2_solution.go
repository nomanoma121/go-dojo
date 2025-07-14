package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Result struct {
	URL      string        `json:"url"`
	Status   int           `json:"status"`
	Duration time.Duration `json:"duration"`
	Error    error         `json:"error,omitempty"`
}

func fetchURLs(urls []string) []Result {
	// 5秒のタイムアウト設定
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	results := make([]Result, len(urls))
	var wg sync.WaitGroup
	
	// 各URLを並行処理
	for i, url := range urls {
		wg.Add(1)
		go func(index int, u string) {
			defer wg.Done()
			results[index] = fetchURL(ctx, u)
		}(i, url)
	}
	
	// 全ての処理完了を待機
	wg.Wait()
	
	return results
}

func fetchURL(ctx context.Context, url string) Result {
	start := time.Now()
	
	// コンテキスト付きHTTPリクエスト作成
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Result{
			URL:      url,
			Duration: time.Since(start),
			Error:    err,
		}
	}
	
	// HTTPクライアント作成（タイムアウト設定）
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	
	// リクエスト実行
	resp, err := client.Do(req)
	duration := time.Since(start)
	
	if err != nil {
		return Result{
			URL:      url,
			Duration: duration,
			Error:    err,
		}
	}
	defer resp.Body.Close()
	
	return Result{
		URL:      url,
		Status:   resp.StatusCode,
		Duration: duration,
	}
}

// Alternative implementation using channels
func fetchURLsWithChannels(urls []string) []Result {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	resultChan := make(chan Result, len(urls))
	var wg sync.WaitGroup
	
	// 各URLを並行処理
	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			result := fetchURL(ctx, u)
			resultChan <- result
		}(url)
	}
	
	// Goroutineでチャネルをクローズ
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// 結果を収集
	var results []Result
	for result := range resultChan {
		results = append(results, result)
	}
	
	return results
}

// Rate-limited version
func fetchURLsWithRateLimit(urls []string, maxConcurrency int) []Result {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	results := make([]Result, len(urls))
	var wg sync.WaitGroup
	
	// セマフォでコンカレンシー制限
	semaphore := make(chan struct{}, maxConcurrency)
	
	for i, url := range urls {
		wg.Add(1)
		go func(index int, u string) {
			defer wg.Done()
			
			// セマフォ取得
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			results[index] = fetchURL(ctx, u)
		}(i, url)
	}
	
	wg.Wait()
	return results
}

func main() {
	// テスト用URL
	urls := []string{
		"https://httpbin.org/delay/1",
		"https://httpbin.org/delay/2",
		"https://httpbin.org/status/200",
		"https://httpbin.org/status/404",
		"https://httpbin.org/status/500",
		"https://google.com",
		"https://github.com",
		"https://stackoverflow.com",
	}
	
	fmt.Println("=== Basic concurrent fetch ===")
	start := time.Now()
	results := fetchURLs(urls)
	fmt.Printf("Completed in %v\n\n", time.Since(start))
	
	for _, result := range results {
		if result.Error != nil {
			fmt.Printf("❌ URL: %s\n   Error: %v\n   Duration: %v\n\n",
				result.URL, result.Error, result.Duration)
		} else {
			fmt.Printf("✅ URL: %s\n   Status: %d\n   Duration: %v\n\n",
				result.URL, result.Status, result.Duration)
		}
	}
	
	fmt.Println("=== Channel-based fetch ===")
	start = time.Now()
	results2 := fetchURLsWithChannels(urls)
	fmt.Printf("Completed in %v\n\n", time.Since(start))
	
	fmt.Printf("Results count: %d\n", len(results2))
	
	fmt.Println("=== Rate-limited fetch (max 3 concurrent) ===")
	start = time.Now()
	results3 := fetchURLsWithRateLimit(urls, 3)
	fmt.Printf("Completed in %v\n\n", time.Since(start))
	
	fmt.Printf("Results count: %d\n", len(results3))
}