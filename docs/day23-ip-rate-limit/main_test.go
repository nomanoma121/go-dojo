package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rateLimiter := NewRateLimiter(10, time.Minute)
	if rateLimiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	defer rateLimiter.Close()

	// 初期化の確認
	if rateLimiter.limit != 10 {
		t.Errorf("Expected limit 10, got %d", rateLimiter.limit)
	}
	if rateLimiter.window != time.Minute {
		t.Errorf("Expected window 1 minute, got %v", rateLimiter.window)
	}
}

func TestSlidingWindowAllow(t *testing.T) {
	window := &SlidingWindow{
		window: time.Second,
		limit:  3,
	}

	// 制限内のリクエスト
	for i := 0; i < 3; i++ {
		if !window.Allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 制限を超えるリクエスト
	if window.Allow() {
		t.Error("4th request should be denied")
	}

	// 時間経過後のリクエスト
	time.Sleep(time.Second + 100*time.Millisecond)
	if !window.Allow() {
		t.Error("Request after window should be allowed")
	}
}

func TestGetRealIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remoteAddr string
		expected string
	}{
		{
			name:     "Direct connection",
			headers:  map[string]string{},
			remoteAddr: "192.168.1.100:8080",
			expected: "192.168.1.100",
		},
		{
			name: "X-Forwarded-For single IP",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.195",
			},
			remoteAddr: "192.168.1.100:8080",
			expected: "203.0.113.195",
		},
		{
			name: "X-Forwarded-For multiple IPs",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.195, 70.41.3.18, 150.172.238.178",
			},
			remoteAddr: "192.168.1.100:8080",
			expected: "203.0.113.195",
		},
		{
			name: "X-Real-IP",
			headers: map[string]string{
				"X-Real-IP": "198.51.100.178",
			},
			remoteAddr: "192.168.1.100:8080",
			expected: "198.51.100.178",
		},
		{
			name: "X-Forwarded-For takes precedence",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.195",
				"X-Real-IP":       "198.51.100.178",
			},
			remoteAddr: "192.168.1.100:8080",
			expected: "203.0.113.195",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			ip := getRealIP(req)
			if ip != tt.expected {
				t.Errorf("Expected IP %s, got %s", tt.expected, ip)
			}
		})
	}
}

func TestRateLimiterIsAllowed(t *testing.T) {
	rateLimiter := NewRateLimiter(3, time.Second)
	defer rateLimiter.Close()

	ip := "192.168.1.100"

	// 制限内のリクエスト
	for i := 0; i < 3; i++ {
		allowed, remaining := rateLimiter.IsAllowed(ip)
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
		expectedRemaining := 3 - i - 1
		if remaining != expectedRemaining {
			t.Errorf("Expected remaining %d, got %d", expectedRemaining, remaining)
		}
	}

	// 制限を超えるリクエスト
	allowed, remaining := rateLimiter.IsAllowed(ip)
	if allowed {
		t.Error("4th request should be denied")
	}
	if remaining != 0 {
		t.Errorf("Expected remaining 0, got %d", remaining)
	}
}

func TestRateLimiterWhitelist(t *testing.T) {
	rateLimiter := NewRateLimiter(1, time.Minute)
	defer rateLimiter.Close()

	ip := "192.168.1.100"
	
	// ホワイトリストに追加
	rateLimiter.AddToWhitelist(ip)
	
	if !rateLimiter.IsWhitelisted(ip) {
		t.Error("IP should be whitelisted")
	}

	// ホワイトリストのIPは制限なし
	for i := 0; i < 10; i++ {
		allowed, _ := rateLimiter.IsAllowed(ip)
		if !allowed {
			t.Errorf("Whitelisted IP should always be allowed (request %d)", i+1)
		}
	}

	// ホワイトリストから削除
	rateLimiter.RemoveFromWhitelist(ip)
	
	if rateLimiter.IsWhitelisted(ip) {
		t.Error("IP should not be whitelisted after removal")
	}
}

func TestMiddleware(t *testing.T) {
	rateLimiter := NewRateLimiter(2, time.Second)
	defer rateLimiter.Close()

	handler := rateLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// 制限内のリクエスト
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.100:8080"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should return 200, got %d", i+1, w.Code)
		}

		// レートリミットヘッダーの確認
		if limit := w.Header().Get("X-RateLimit-Limit"); limit != "2" {
			t.Errorf("Expected X-RateLimit-Limit 2, got %s", limit)
		}

		expectedRemaining := strconv.Itoa(2 - i - 1)
		if remaining := w.Header().Get("X-RateLimit-Remaining"); remaining != expectedRemaining {
			t.Errorf("Expected X-RateLimit-Remaining %s, got %s", expectedRemaining, remaining)
		}
	}

	// 制限を超えるリクエスト
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.100:8080"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Rate limited request should return 429, got %d", w.Code)
	}

	// Retry-Afterヘッダーの確認
	if retryAfter := w.Header().Get("Retry-After"); retryAfter == "" {
		t.Error("Retry-After header should be set")
	}

	// エラーレスポンスの確認
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal("Failed to parse JSON response")
	}

	if response["error"] != "Rate limit exceeded" {
		t.Errorf("Expected error message, got %v", response["error"])
	}
}

func TestMiddlewareWithWhitelistedIP(t *testing.T) {
	rateLimiter := NewRateLimiter(1, time.Second)
	defer rateLimiter.Close()

	ip := "127.0.0.1"
	rateLimiter.AddToWhitelist(ip)

	handler := rateLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	// ホワイトリストIPは制限なし
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Real-IP", ip)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Whitelisted IP request %d should return 200, got %d", i+1, w.Code)
		}
	}
}

func TestConcurrentRequests(t *testing.T) {
	rateLimiter := NewRateLimiter(10, time.Second)
	defer rateLimiter.Close()

	handler := rateLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const numGoroutines = 5
	const requestsPerGoroutine = 3
	
	var wg sync.WaitGroup
	results := make(chan int, numGoroutines*requestsPerGoroutine)

	// 複数のゴルーチンから同時にリクエスト
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = "192.168.1.100:8080"
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)
				results <- w.Code
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// 結果の集計
	successCount := 0
	rateLimitedCount := 0
	
	for code := range results {
		switch code {
		case http.StatusOK:
			successCount++
		case http.StatusTooManyRequests:
			rateLimitedCount++
		default:
			t.Errorf("Unexpected status code: %d", code)
		}
	}

	// 制限内のリクエストが成功していることを確認
	if successCount > 10 {
		t.Errorf("Too many successful requests: %d (limit: 10)", successCount)
	}
	
	if successCount < 5 {
		t.Errorf("Too few successful requests: %d", successCount)
	}
}

func TestDifferentIPs(t *testing.T) {
	rateLimiter := NewRateLimiter(2, time.Second)
	defer rateLimiter.Close()

	handler := rateLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ips := []string{"192.168.1.100", "192.168.1.101", "192.168.1.102"}

	// 各IPごとに制限内のリクエスト
	for _, ip := range ips {
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = ip + ":8080"
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Request from IP %s should be allowed, got %d", ip, w.Code)
			}
		}
	}

	// 各IPで制限を超える
	for _, ip := range ips {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip + ":8080"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("Rate limited request from IP %s should return 429, got %d", ip, w.Code)
		}
	}
}

func TestTimeWindowReset(t *testing.T) {
	rateLimiter := NewRateLimiter(2, 500*time.Millisecond)
	defer rateLimiter.Close()

	handler := rateLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "192.168.1.100:8080"

	// 制限まで使い切る
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed, got %d", i+1, w.Code)
		}
	}

	// 制限超過を確認
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Should be rate limited, got %d", w.Code)
	}

	// ウィンドウリセット後に再び許可される
	time.Sleep(600 * time.Millisecond)

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ip
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Request after window reset should be allowed, got %d", w.Code)
	}
}

func TestJSONErrorResponse(t *testing.T) {
	rateLimiter := NewRateLimiter(1, time.Minute)
	defer rateLimiter.Close()

	handler := rateLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "192.168.1.100:8080"

	// 最初のリクエストは成功
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ip
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("First request should be allowed, got %d", w.Code)
	}

	// 2回目は制限
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ip
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited, got %d", w.Code)
	}

	// Content-Typeの確認
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// JSONレスポンスの確認
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal("Failed to parse JSON response")
	}

	expectedFields := []string{"error", "message", "retry_after", "limit", "window"}
	for _, field := range expectedFields {
		if _, exists := response[field]; !exists {
			t.Errorf("Response should contain field: %s", field)
		}
	}
}

func BenchmarkRateLimiter(b *testing.B) {
	rateLimiter := NewRateLimiter(1000, time.Second)
	defer rateLimiter.Close()

	handler := rateLimiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.100:8080"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})
}

func BenchmarkGetRealIP(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.100:8080"
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getRealIP(req)
	}
}