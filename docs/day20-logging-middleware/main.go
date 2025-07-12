package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// contextKey is used for context keys to avoid collisions
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
)

// LoggingMiddleware provides structured logging for HTTP requests
type LoggingMiddleware struct {
	logger *slog.Logger
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware() *LoggingMiddleware {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. slog.JSONハンドラーでJSON形式のログを設定
	// 2. ログレベルを設定
	// 3. LoggingMiddlewareを作成
	
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	
	return &LoggingMiddleware{
		logger: logger,
	}
}

// RequestIDMiddleware generates and adds request ID to context
func (lm *LoggingMiddleware) RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: 実装してください
		//
		// 実装の流れ:
		// 1. リクエストIDを生成（UUID形式またはランダム文字列）
		// 2. リクエストIDをcontextに追加
		// 3. レスポンスヘッダーにリクエストIDを追加
		// 4. 次のハンドラーを呼び出し
		
		requestID := generateRequestID()
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		
		// Add to response header for client tracking
		w.Header().Set("X-Request-ID", requestID)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggingMiddleware logs HTTP request details
func (lm *LoggingMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// TODO: 実装してください
		//
		// 実装の流れ:
		// 1. レスポンスライターをラップして詳細情報を記録
		// 2. リクエスト開始ログを出力
		// 3. 次のハンドラーを実行
		// 4. レスポンス完了ログを出力
		
		// Wrap response writer to capture status and size
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		
		// Log request start
		lm.logRequest(r, "request_start")
		
		// Process request
		next.ServeHTTP(wrapped, r)
		
		// Log request completion
		duration := time.Since(start)
		lm.logRequestComplete(r, wrapped.statusCode, wrapped.bytesWritten, duration)
	})
}

// logRequest logs request information
func (lm *LoggingMiddleware) logRequest(r *http.Request, event string) {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. contextからリクエストIDを取得
	// 2. 構造化ログでリクエスト情報を記録
	// 3. URL、メソッド、User-Agent、IP等を含める
	
	requestID, _ := r.Context().Value(RequestIDKey).(string)
	userID, _ := r.Context().Value(UserIDKey).(string)
	
	lm.logger.InfoContext(r.Context(), event,
		"request_id", requestID,
		"method", r.Method,
		"url", r.URL.String(),
		"user_agent", r.UserAgent(),
		"remote_addr", r.RemoteAddr,
		"user_id", userID,
	)
}

// logRequestComplete logs request completion
func (lm *LoggingMiddleware) logRequestComplete(r *http.Request, statusCode int, bytesWritten int64, duration time.Duration) {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. リクエスト完了の詳細情報をログ出力
	// 2. ステータスコード、レスポンスサイズ、処理時間を記録
	// 3. エラー時は詳細情報も記録
	
	requestID, _ := r.Context().Value(RequestIDKey).(string)
	userID, _ := r.Context().Value(UserIDKey).(string)
	
	logLevel := slog.LevelInfo
	if statusCode >= 400 {
		logLevel = slog.LevelWarn
	}
	if statusCode >= 500 {
		logLevel = slog.LevelError
	}
	
	lm.logger.Log(r.Context(), logLevel, "request_complete",
		"request_id", requestID,
		"method", r.Method,
		"url", r.URL.String(),
		"status_code", statusCode,
		"response_size_bytes", bytesWritten,
		"duration_ms", duration.Milliseconds(),
		"user_id", userID,
	)
}

// ErrorMiddleware logs errors with detailed context
func (lm *LoggingMiddleware) ErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// TODO: 実装してください
				//
				// 実装の流れ:
				// 1. パニックをキャッチしてエラーログを出力
				// 2. スタックトレースを記録
				// 3. 500エラーをクライアントに返す
				
				requestID, _ := r.Context().Value(RequestIDKey).(string)
				
				lm.logger.ErrorContext(r.Context(), "panic_recovered",
					"request_id", requestID,
					"error", err,
					"method", r.Method,
					"url", r.URL.String(),
				)
				
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		
		next.ServeHTTP(w, r)
	})
}

// UserContextMiddleware adds user information to context (for demonstration)
func (lm *LoggingMiddleware) UserContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: 実装してください
		//
		// 実装の流れ:
		// 1. 認証ヘッダーまたはセッションからユーザーIDを取得
		// 2. ユーザー情報をcontextに追加
		// 3. 次のハンドラーを呼び出し
		
		// Simulate getting user ID from header
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			userID = "anonymous"
		}
		
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// responseWriter wraps http.ResponseWriter to capture response details
type responseWriter struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int64
	headerWritten bool
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	// TODO: 実装してください
	if !rw.headerWritten {
		rw.statusCode = statusCode
		rw.headerWritten = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	// TODO: 実装してください
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += int64(n)
	return n, err
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. crypto/randで安全な乱数を生成
	// 2. 16進数文字列に変換
	// 3. 適切な長さ（8-16文字）に調整
	
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func main() {
	logging := NewLoggingMiddleware()
	
	mux := http.NewServeMux()
	
	// Sample handlers
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		users := []map[string]interface{}{
			{"id": 1, "name": "Alice"},
			{"id": 2, "name": "Bob"},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})
	
	mux.HandleFunc("/api/error", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
	})
	
	mux.HandleFunc("/api/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("simulated panic")
	})
	
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// Chain middlewares
	handler := logging.RequestIDMiddleware(
		logging.UserContextMiddleware(
			logging.ErrorMiddleware(
				logging.Middleware(mux),
			),
		),
	)
	
	slog.Info("Server starting on :8080")
	http.ListenAndServe(":8080", handler)
}