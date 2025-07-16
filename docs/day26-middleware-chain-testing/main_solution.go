package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Context keys
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
)

// RateLimiter implements IP-based rate limiting
type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if request is allowed for given IP
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	cutoff := now.Add(-rl.window)
	
	// Get request times for this IP
	times := rl.requests[ip]
	
	// Remove expired entries
	validTimes := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			validTimes = append(validTimes, t)
		}
	}
	
	// Check if under limit
	if len(validTimes) >= rl.limit {
		rl.requests[ip] = validTimes
		return false
	}
	
	// Add current request time
	validTimes = append(validTimes, now)
	rl.requests[ip] = validTimes
	
	return true
}

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware(allowedOrigins []string, allowedMethods []string, allowedHeaders []string) func(http.Handler) http.Handler {
	methodsStr := strings.Join(allowedMethods, ", ")
	headersStr := strings.Join(allowedHeaders, ", ")
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			
			// Check if origin is allowed
			originAllowed := false
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					originAllowed = true
					break
				}
			}
			
			// Set CORS headers if origin is allowed
			if originAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", methodsStr)
				w.Header().Set("Access-Control-Allow-Headers", headersStr)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			
			// Handle preflight OPTIONS request
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// AuthMiddleware validates Bearer tokens
func AuthMiddleware(validTokens map[string]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error": "missing authorization header"}`, http.StatusUnauthorized)
				return
			}
			
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error": "invalid authorization format"}`, http.StatusUnauthorized)
				return
			}
			
			token := parts[1]
			userID, valid := validTokens[token]
			if !valid {
				http.Error(w, `{"error": "invalid token"}`, http.StatusUnauthorized)
				return
			}
			
			// Set user ID in context
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			r = r.WithContext(ctx)
			
			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware logs request and response information
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Get request ID from context
			requestID := getRequestID(r.Context())
			
			// Log request start
			logger.Info("request_start",
				"method", r.Method,
				"url", r.URL.Path,
				"user_agent", r.UserAgent(),
				"request_id", requestID,
			)
			
			// Wrap response writer to capture status and size
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			
			next.ServeHTTP(rw, r)
			
			duration := time.Since(start)
			
			// Log request completion
			logger.Info("request_complete",
				"method", r.Method,
				"url", r.URL.Path,
				"status_code", rw.statusCode,
				"bytes_written", rw.bytesWritten,
				"duration_ms", duration.Milliseconds(),
				"request_id", requestID,
			)
		})
	}
}

// RateLimitMiddleware applies rate limiting per IP
func RateLimitMiddleware(rateLimiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIP(r)
			
			if !rateLimiter.Allow(clientIP) {
				http.Error(w, `{"error": "rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware generates and sets request ID
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := generateRequestID()
			
			// Set in context
			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
			r = r.WithContext(ctx)
			
			// Set in response header
			w.Header().Set("X-Request-ID", requestID)
			
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status and size
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

// WriteHeader captures status code
func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write captures response size
func (rw *responseWriter) Write(data []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += int64(n)
	return n, err
}

// CreateMiddlewareChain creates the complete middleware chain
func CreateMiddlewareChain(logger *slog.Logger, rateLimiter *RateLimiter, validTokens map[string]string) func(http.Handler) http.Handler {
	allowedOrigins := []string{"https://example.com", "https://test.com"}
	allowedMethods := []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	allowedHeaders := []string{"Content-Type", "Authorization", "X-Request-ID"}
	
	return func(handler http.Handler) http.Handler {
		// Chain middlewares in order: CORS -> RequestID -> Logging -> Auth -> RateLimit
		handler = RateLimitMiddleware(rateLimiter)(handler)
		handler = AuthMiddleware(validTokens)(handler)
		handler = LoggingMiddleware(logger)(handler)
		handler = RequestIDMiddleware()(handler)
		handler = CORSMiddleware(allowedOrigins, allowedMethods, allowedHeaders)(handler)
		return handler
	}
}

// TestHandler is a simple handler for testing
func TestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get context values
		requestID := getRequestID(r.Context())
		userID := getUserID(r.Context())
		
		response := map[string]interface{}{
			"message":    "success",
			"request_id": requestID,
			"user_id":    userID,
			"path":       r.URL.Path,
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

// Helper functions to get context values
func getRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

func getUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}
	
	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	// Generate 8 random bytes
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(bytes)
}

func main() {
	// Example usage
	logger := slog.Default()
	rateLimiter := NewRateLimiter(10, time.Minute)
	validTokens := map[string]string{
		"valid-token": "user123",
		"admin-token": "admin",
	}
	
	chain := CreateMiddlewareChain(logger, rateLimiter, validTokens)
	handler := chain(TestHandler())
	
	http.Handle("/api/", handler)
	slog.Info("Server starting on :8080")
	http.ListenAndServe(":8080", nil)
}