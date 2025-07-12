package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		rl := NewRateLimiter(3, time.Minute)
		
		assert.True(t, rl.Allow("192.168.1.1"))
		assert.True(t, rl.Allow("192.168.1.1"))
		assert.True(t, rl.Allow("192.168.1.1"))
	})
	
	t.Run("blocks requests over limit", func(t *testing.T) {
		rl := NewRateLimiter(2, time.Minute)
		
		assert.True(t, rl.Allow("192.168.1.2"))
		assert.True(t, rl.Allow("192.168.1.2"))
		assert.False(t, rl.Allow("192.168.1.2"))
	})
	
	t.Run("different IPs have separate limits", func(t *testing.T) {
		rl := NewRateLimiter(1, time.Minute)
		
		assert.True(t, rl.Allow("192.168.1.3"))
		assert.False(t, rl.Allow("192.168.1.3"))
		assert.True(t, rl.Allow("192.168.1.4")) // different IP
	})
	
	t.Run("resets after window", func(t *testing.T) {
		rl := NewRateLimiter(1, 10*time.Millisecond)
		
		assert.True(t, rl.Allow("192.168.1.5"))
		assert.False(t, rl.Allow("192.168.1.5"))
		
		time.Sleep(15 * time.Millisecond)
		assert.True(t, rl.Allow("192.168.1.5"))
	})
}

func TestCORSMiddleware(t *testing.T) {
	allowedOrigins := []string{"https://example.com", "https://test.com"}
	allowedMethods := []string{"GET", "POST", "PUT", "DELETE"}
	allowedHeaders := []string{"Content-Type", "Authorization"}
	
	middleware := CORSMiddleware(allowedOrigins, allowedMethods, allowedHeaders)
	
	t.Run("sets CORS headers for allowed origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, "https://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE", rr.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type, Authorization", rr.Header().Get("Access-Control-Allow-Headers"))
	})
	
	t.Run("does not set origin for disallowed origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://malicious.com")
		
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		handler.ServeHTTP(rr, req)
		
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})
	
	t.Run("handles preflight OPTIONS request", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Should not reach next handler for OPTIONS")
		}))
		
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "https://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestAuthMiddleware(t *testing.T) {
	validTokens := map[string]string{
		"valid-token": "user123",
		"admin-token": "admin",
	}
	
	middleware := AuthMiddleware(validTokens)
	
	t.Run("allows valid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		
		var receivedUserID string
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedUserID = getUserID(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
		
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "user123", receivedUserID)
	})
	
	t.Run("rejects missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Should not reach next handler")
		}))
		
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "missing authorization header")
	})
	
	t.Run("rejects invalid token format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Invalid format")
		
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Should not reach next handler")
		}))
		
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid authorization format")
	})
	
	t.Run("rejects invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Should not reach next handler")
		}))
		
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid token")
	})
}

func TestLoggingMiddleware(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	middleware := LoggingMiddleware(logger)
	
	t.Run("logs request and response", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("User-Agent", "test-agent")
		
		// Set request ID in context
		ctx := context.WithValue(req.Context(), RequestIDKey, "test-request-id")
		req = req.WithContext(ctx)
		
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		}))
		
		handler.ServeHTTP(rr, req)
		
		logOutput := logBuffer.String()
		assert.Contains(t, logOutput, "request_start")
		assert.Contains(t, logOutput, "request_complete")
		assert.Contains(t, logOutput, "GET")
		assert.Contains(t, logOutput, "/test")
		assert.Contains(t, logOutput, "test-request-id")
		assert.Contains(t, logOutput, "200")
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	rateLimiter := NewRateLimiter(2, time.Minute)
	middleware := RateLimitMiddleware(rateLimiter)
	
	t.Run("allows requests under limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		
		// Second request should also pass
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
	
	t.Run("blocks requests over limit", func(t *testing.T) {
		rl := NewRateLimiter(1, time.Minute)
		mw := RateLimitMiddleware(rl)
		
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		
		handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		
		// First request should pass
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		
		// Second request should be rate limited
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
		assert.Contains(t, rr.Body.String(), "rate limit exceeded")
	})
}

func TestRequestIDMiddleware(t *testing.T) {
	middleware := RequestIDMiddleware()
	
	t.Run("generates and sets request ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		
		var receivedRequestID string
		rr := httptest.NewRecorder()
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedRequestID = getRequestID(r.Context())
			w.WriteHeader(http.StatusOK)
		}))
		
		handler.ServeHTTP(rr, req)
		
		assert.NotEmpty(t, receivedRequestID)
		assert.Len(t, receivedRequestID, 16) // Should be 16 characters
		assert.Equal(t, receivedRequestID, rr.Header().Get("X-Request-ID"))
	})
	
	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			
			var requestID string
			rr := httptest.NewRecorder()
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestID = getRequestID(r.Context())
			}))
			
			handler.ServeHTTP(rr, req)
			
			assert.False(t, ids[requestID], "Request ID should be unique: %s", requestID)
			ids[requestID] = true
		}
	})
}

func TestMiddlewareChain(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug}))
	rateLimiter := NewRateLimiter(10, time.Minute)
	validTokens := map[string]string{
		"valid-token": "user123",
	}
	
	chain := CreateMiddlewareChain(logger, rateLimiter, validTokens)
	handler := chain(TestHandler())
	
	t.Run("successful request through chain", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/data", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("User-Agent", "test-client")
		
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		
		// Check CORS headers
		assert.Equal(t, "https://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
		
		// Check request ID header
		assert.NotEmpty(t, rr.Header().Get("X-Request-ID"))
		
		// Check response content
		var response map[string]interface{}
		err := json.Unmarshal(rr.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "success", response["message"])
		assert.Equal(t, "user123", response["user_id"])
		assert.NotEmpty(t, response["request_id"])
		
		// Check logs
		logOutput := logBuffer.String()
		assert.Contains(t, logOutput, "request_start")
		assert.Contains(t, logOutput, "request_complete")
	})
	
	t.Run("unauthorized request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/data", nil)
		req.Header.Set("Origin", "https://example.com")
		// No Authorization header
		
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		
		// Should still have CORS headers for error responses
		assert.Equal(t, "https://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	})
	
	t.Run("disallowed origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/data", nil)
		req.Header.Set("Origin", "https://malicious.com")
		req.Header.Set("Authorization", "Bearer valid-token")
		
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		
		// Should not have CORS origin header
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
	})
	
	t.Run("rate limit exceeded", func(t *testing.T) {
		rl := NewRateLimiter(1, time.Minute)
		limitedChain := CreateMiddlewareChain(logger, rl, validTokens)
		limitedHandler := limitedChain(TestHandler())
		
		req := httptest.NewRequest("GET", "/api/data", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		req.RemoteAddr = "192.168.1.100:12345"
		
		// First request should succeed
		rr := httptest.NewRecorder()
		limitedHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		
		// Second request should be rate limited
		rr = httptest.NewRecorder()
		limitedHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	})
	
	t.Run("OPTIONS preflight request", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/api/data", nil)
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "https://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", rr.Header().Get("Access-Control-Allow-Methods"))
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code and response size", func(t *testing.T) {
		originalWriter := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: originalWriter,
			statusCode:     http.StatusOK, // default
		}
		
		// Test WriteHeader
		rw.WriteHeader(http.StatusCreated)
		assert.Equal(t, http.StatusCreated, rw.statusCode)
		
		// Test Write
		data := []byte("test response")
		n, err := rw.Write(data)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, int64(len(data)), rw.bytesWritten)
		
		// Check original writer received the data
		assert.Equal(t, http.StatusCreated, originalWriter.Code)
		assert.Equal(t, "test response", originalWriter.Body.String())
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("getClientIP", func(t *testing.T) {
		// Test X-Forwarded-For header
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1, 192.168.1.1")
		assert.Equal(t, "203.0.113.1", getClientIP(req))
		
		// Test X-Real-IP header
		req = httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Real-IP", "203.0.113.2")
		assert.Equal(t, "203.0.113.2", getClientIP(req))
		
		// Test RemoteAddr fallback
		req = httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.3:12345"
		assert.Equal(t, "192.168.1.3", getClientIP(req))
	})
	
	t.Run("generateRequestID", func(t *testing.T) {
		id1 := generateRequestID()
		id2 := generateRequestID()
		
		assert.NotEqual(t, id1, id2)
		assert.Len(t, id1, 16)
		assert.Len(t, id2, 16)
		
		// Should only contain hexadecimal characters
		for _, char := range id1 {
			assert.True(t, 
				(char >= '0' && char <= '9') || 
				(char >= 'a' && char <= 'f'),
				"Invalid character in request ID: %c", char)
		}
	})
	
	t.Run("context helpers", func(t *testing.T) {
		ctx := context.Background()
		
		// Test with values
		ctx = context.WithValue(ctx, RequestIDKey, "test-request-id")
		ctx = context.WithValue(ctx, UserIDKey, "test-user")
		
		assert.Equal(t, "test-request-id", getRequestID(ctx))
		assert.Equal(t, "test-user", getUserID(ctx))
		
		// Test with missing values
		emptyCtx := context.Background()
		assert.Equal(t, "", getRequestID(emptyCtx))
		assert.Equal(t, "", getUserID(emptyCtx))
	})
}

func BenchmarkMiddlewareChain(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rateLimiter := NewRateLimiter(1000, time.Minute)
	validTokens := map[string]string{
		"bench-token": "bench-user",
	}
	
	chain := CreateMiddlewareChain(logger, rateLimiter, validTokens)
	handler := chain(TestHandler())
	
	req := httptest.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Authorization", "Bearer bench-token")
	req.RemoteAddr = "192.168.1.200:12345"
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkIndividualMiddlewares(b *testing.B) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Authorization", "Bearer valid-token")
	req.RemoteAddr = "192.168.1.201:12345"
	
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	b.Run("CORS", func(b *testing.B) {
		middleware := CORSMiddleware(
			[]string{"https://example.com"},
			[]string{"GET", "POST"},
			[]string{"Content-Type"},
		)
		handler := middleware(dummyHandler)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}
	})
	
	b.Run("Auth", func(b *testing.B) {
		middleware := AuthMiddleware(map[string]string{"valid-token": "user"})
		handler := middleware(dummyHandler)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}
	})
	
	b.Run("RateLimit", func(b *testing.B) {
		rateLimiter := NewRateLimiter(1000, time.Minute)
		middleware := RateLimitMiddleware(rateLimiter)
		handler := middleware(dummyHandler)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
		}
	})
	
	b.Run("Logging", func(b *testing.B) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		middleware := LoggingMiddleware(logger)
		handler := middleware(dummyHandler)
		
		// Add request ID to context for logging
		ctx := context.WithValue(req.Context(), RequestIDKey, "bench-id")
		benchReq := req.WithContext(ctx)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, benchReq)
		}
	})
}