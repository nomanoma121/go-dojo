package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SlidingWindow IPごとのスライディングウィンドウを管理
type SlidingWindow struct {
	mu       sync.Mutex
	requests []time.Time
	window   time.Duration
	limit    int
}

// RateLimiter IP ベースのレートリミッター
type RateLimiter struct {
	mu        sync.RWMutex
	clients   map[string]*SlidingWindow
	whitelist map[string]bool
	limit     int
	window    time.Duration
	done      chan struct{}
}

// NewRateLimiter 新しいレートリミッターを作成
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:   make(map[string]*SlidingWindow),
		whitelist: make(map[string]bool),
		limit:     limit,
		window:    window,
		done:      make(chan struct{}),
	}
	
	// バックグラウンドクリーンアップを開始
	go rl.cleanup()
	
	return rl
}

// Allow リクエストが許可されるかチェック
func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	
	now := time.Now()
	cutoff := now.Add(-sw.window)
	
	// ウィンドウ外の古いリクエストを削除
	validRequests := make([]time.Time, 0, len(sw.requests))
	for _, req := range sw.requests {
		if !req.Before(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	sw.requests = validRequests
	
	// 制限チェック
	if len(sw.requests) >= sw.limit {
		return false
	}
	
	// 新しいリクエストを記録
	sw.requests = append(sw.requests, now)
	return true
}

// remaining 残り回数を計算
func (sw *SlidingWindow) remaining() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	
	now := time.Now()
	cutoff := now.Add(-sw.window)
	
	// 有効なリクエスト数をカウント
	validCount := 0
	for _, req := range sw.requests {
		if !req.Before(cutoff) {
			validCount++
		}
	}
	
	remaining := sw.limit - validCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

// nextReset 次のリセット時刻を計算
func (sw *SlidingWindow) nextReset() time.Time {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	
	if len(sw.requests) == 0 {
		return time.Now().Add(sw.window)
	}
	
	// 最古のリクエストから window 分後がリセット時刻
	return sw.requests[0].Add(sw.window)
}

// IsAllowed 指定されたIPからのリクエストが許可されるかチェック
func (rl *RateLimiter) IsAllowed(ip string) (bool, int) {
	// ホワイトリストチェック
	if rl.IsWhitelisted(ip) {
		return true, rl.limit // ホワイトリストは常に満額の残り回数
	}
	
	rl.mu.Lock()
	window, exists := rl.clients[ip]
	if !exists {
		window = &SlidingWindow{
			window: rl.window,
			limit:  rl.limit,
		}
		rl.clients[ip] = window
	}
	rl.mu.Unlock()
	
	allowed := window.Allow()
	remaining := window.remaining()
	
	return allowed, remaining
}

// AddToWhitelist IPをホワイトリストに追加
func (rl *RateLimiter) AddToWhitelist(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.whitelist[ip] = true
}

// RemoveFromWhitelist IPをホワイトリストから削除
func (rl *RateLimiter) RemoveFromWhitelist(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.whitelist, ip)
}

// IsWhitelisted IPがホワイトリストに含まれているかチェック
func (rl *RateLimiter) IsWhitelisted(ip string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.whitelist[ip]
}

// Close リソースをクリーンアップ
func (rl *RateLimiter) Close() {
	close(rl.done)
}

// getRealIP リクエストから実際の IP アドレスを取得
func getRealIP(r *http.Request) string {
	// X-Forwarded-For ヘッダーをチェック（プロキシ経由の場合）
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			// 最初のIPアドレスが実際のクライアントIP
			return strings.TrimSpace(ips[0])
		}
	}
	
	// X-Real-IP ヘッダーをチェック
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// 直接接続の場合、RemoteAddrから取得
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	
	return ip
}

// Middleware レートリミットミドルウェア
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getRealIP(r)
		
		allowed, remaining := rl.IsAllowed(ip)
		
		// レートリミットヘッダーを設定
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		
		// リセット時刻を設定（ホワイトリストの場合を除く）
		if !rl.IsWhitelisted(ip) {
			rl.mu.RLock()
			if window, exists := rl.clients[ip]; exists {
				resetTime := window.nextReset()
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
			}
			rl.mu.RUnlock()
		}
		
		if !allowed {
			rl.sendRateLimitResponse(w, ip, int(rl.window.Seconds()))
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// sendRateLimitResponse レート制限応答を送信
func (rl *RateLimiter) sendRateLimitResponse(w http.ResponseWriter, ip string, retryAfter int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	w.WriteHeader(http.StatusTooManyRequests)
	
	response := map[string]interface{}{
		"error":      "Rate limit exceeded",
		"message":    fmt.Sprintf("Too many requests from IP %s", ip),
		"retry_after": retryAfter,
		"limit":      rl.limit,
		"window":     rl.window.String(),
	}
	
	json.NewEncoder(w).Encode(response)
}

// cleanup 期限切れエントリを定期的にクリーンアップ
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			
			for ip, window := range rl.clients {
				window.mu.Lock()
				cutoff := now.Add(-window.window)
				
				// 期限切れリクエストをすべて削除
				validRequests := make([]time.Time, 0)
				for _, req := range window.requests {
					if !req.Before(cutoff) {
						validRequests = append(validRequests, req)
					}
				}
				window.requests = validRequests
				
				// 空になったウィンドウを削除
				if len(window.requests) == 0 {
					delete(rl.clients, ip)
				}
				
				window.mu.Unlock()
			}
			
			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// テスト用のサンプルハンドラー
func sampleHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getRealIP(r)
		response := map[string]interface{}{
			"message":   "Request successful",
			"ip":        ip,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

func main() {
	// 10リクエスト/分の制限でレートリミッターを作成
	rateLimiter := NewRateLimiter(10, time.Minute)
	defer rateLimiter.Close()

	// ローカルホストをホワイトリストに追加
	rateLimiter.AddToWhitelist("127.0.0.1")
	rateLimiter.AddToWhitelist("::1")

	// ハンドラーにミドルウェアを適用
	handler := rateLimiter.Middleware(sampleHandler())

	fmt.Println("Server starting on :8080")
	fmt.Println("Rate limit: 10 requests per minute")
	fmt.Println("Test with: curl http://localhost:8080/")
	
	http.Handle("/", handler)
	http.ListenAndServe(":8080", nil)
}