package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingMiddleware(t *testing.T) {
	t.Run("Request ID generation and propagation", func(t *testing.T) {
		logging := NewLoggingMiddleware()
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Context().Value(RequestIDKey)
			if requestID == nil {
				t.Error("Request ID not found in context")
			}
			
			if requestIDStr, ok := requestID.(string); ok && len(requestIDStr) != 16 {
				t.Errorf("Request ID length should be 16, got %d", len(requestIDStr))
			}
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		
		logging.RequestIDMiddleware(handler).ServeHTTP(rr, req)
		
		// Check response header
		requestID := rr.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("X-Request-ID header not set")
		}
		
		if len(requestID) != 16 {
			t.Errorf("Request ID in header should be 16 chars, got %d", len(requestID))
		}
	})

	t.Run("Structured logging output", func(t *testing.T) {
		var logBuffer bytes.Buffer
		
		// Create custom logger that writes to buffer
		logging := &LoggingMiddleware{
			logger: createTestLogger(&logBuffer),
		}
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test response"))
		})
		
		req := httptest.NewRequest("GET", "/api/test?param=value", nil)
		req.Header.Set("User-Agent", "test-agent")
		rr := httptest.NewRecorder()
		
		// Chain middlewares
		middleware := logging.RequestIDMiddleware(logging.Middleware(handler))
		middleware.ServeHTTP(rr, req)
		
		// Check log output
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "request_start") {
			t.Error("Log should contain request_start event")
		}
		if !strings.Contains(logOutput, "request_complete") {
			t.Error("Log should contain request_complete event")
		}
		if !strings.Contains(logOutput, "GET") {
			t.Error("Log should contain HTTP method")
		}
		if !strings.Contains(logOutput, "/api/test") {
			t.Error("Log should contain request URL")
		}
	})

	t.Run("Response writer wrapping", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logging := &LoggingMiddleware{
			logger: createTestLogger(&logBuffer),
		}
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("created response"))
		})
		
		req := httptest.NewRequest("POST", "/api/create", nil)
		rr := httptest.NewRecorder()
		
		logging.RequestIDMiddleware(logging.Middleware(handler)).ServeHTTP(rr, req)
		
		// Check status code in logs
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "201") {
			t.Error("Log should contain status code 201")
		}
		
		// Check response
		if rr.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, rr.Code)
		}
	})

	t.Run("Error logging levels", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logging := &LoggingMiddleware{
			logger: createTestLogger(&logBuffer),
		}
		
		tests := []struct {
			name       string
			statusCode int
			handler    http.HandlerFunc
		}{
			{
				name:       "4xx error",
				statusCode: http.StatusBadRequest,
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "Bad request", http.StatusBadRequest)
				}),
			},
			{
				name:       "5xx error",
				statusCode: http.StatusInternalServerError,
				handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "Internal error", http.StatusInternalServerError)
				}),
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				logBuffer.Reset()
				
				req := httptest.NewRequest("GET", "/error", nil)
				rr := httptest.NewRecorder()
				
				logging.RequestIDMiddleware(logging.Middleware(tt.handler)).ServeHTTP(rr, req)
				
				logOutput := logBuffer.String()
				if !strings.Contains(logOutput, "request_complete") {
					t.Error("Should log request completion even on error")
				}
			})
		}
	})

	t.Run("Panic recovery", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logging := &LoggingMiddleware{
			logger: createTestLogger(&logBuffer),
		}
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})
		
		req := httptest.NewRequest("GET", "/panic", nil)
		rr := httptest.NewRecorder()
		
		// Should not panic
		logging.RequestIDMiddleware(
			logging.ErrorMiddleware(
				logging.Middleware(handler),
			),
		).ServeHTTP(rr, req)
		
		// Check response
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
		
		// Check logs
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "panic_recovered") {
			t.Error("Should log panic recovery")
		}
	})

	t.Run("User context middleware", func(t *testing.T) {
		logging := NewLoggingMiddleware()
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Context().Value(UserIDKey)
			if userID == nil {
				t.Error("User ID not found in context")
			}
			
			if userIDStr, ok := userID.(string); ok && userIDStr != "test-user" {
				t.Errorf("Expected user ID 'test-user', got '%s'", userIDStr)
			}
			
			w.WriteHeader(http.StatusOK)
		})
		
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-User-ID", "test-user")
		rr := httptest.NewRecorder()
		
		logging.UserContextMiddleware(handler).ServeHTTP(rr, req)
		
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})

	t.Run("Anonymous user handling", func(t *testing.T) {
		logging := NewLoggingMiddleware()
		
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := r.Context().Value(UserIDKey)
			if userIDStr, ok := userID.(string); ok && userIDStr != "anonymous" {
				t.Errorf("Expected anonymous user, got '%s'", userIDStr)
			}
			w.WriteHeader(http.StatusOK)
		})
		
		req := httptest.NewRequest("GET", "/test", nil)
		// No X-User-ID header
		rr := httptest.NewRecorder()
		
		logging.UserContextMiddleware(handler).ServeHTTP(rr, req)
		
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})
}

func TestResponseWriter(t *testing.T) {
	t.Run("Status code capture", func(t *testing.T) {
		rw := &responseWriter{
			ResponseWriter: httptest.NewRecorder(),
			statusCode:     http.StatusOK,
		}
		
		rw.WriteHeader(http.StatusCreated)
		if rw.statusCode != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, rw.statusCode)
		}
	})

	t.Run("Bytes written tracking", func(t *testing.T) {
		rw := &responseWriter{
			ResponseWriter: httptest.NewRecorder(),
			statusCode:     http.StatusOK,
		}
		
		data := []byte("test response data")
		n, err := rw.Write(data)
		
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if n != len(data) {
			t.Errorf("Expected %d bytes written, got %d", len(data), n)
		}
		if rw.bytesWritten != int64(len(data)) {
			t.Errorf("Expected %d bytes tracked, got %d", len(data), rw.bytesWritten)
		}
	})

	t.Run("Multiple writes", func(t *testing.T) {
		rw := &responseWriter{
			ResponseWriter: httptest.NewRecorder(),
			statusCode:     http.StatusOK,
		}
		
		data1 := []byte("first ")
		data2 := []byte("second")
		
		rw.Write(data1)
		rw.Write(data2)
		
		expectedBytes := int64(len(data1) + len(data2))
		if rw.bytesWritten != expectedBytes {
			t.Errorf("Expected %d total bytes, got %d", expectedBytes, rw.bytesWritten)
		}
	})
}

func TestGenerateRequestID(t *testing.T) {
	t.Run("ID uniqueness", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			id := generateRequestID()
			if ids[id] {
				t.Errorf("Duplicate request ID generated: %s", id)
			}
			ids[id] = true
		}
	})

	t.Run("ID length", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			id := generateRequestID()
			if len(id) != 16 {
				t.Errorf("Expected ID length 16, got %d", len(id))
			}
		}
	})

	t.Run("ID format", func(t *testing.T) {
		id := generateRequestID()
		// Should be hex characters only
		for _, char := range id {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
				t.Errorf("Invalid character in request ID: %c", char)
			}
		}
	})
}

// Helper function to create a test logger
func createTestLogger(buffer *bytes.Buffer) *slog.Logger {
	handler := slog.NewJSONHandler(buffer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}

// Benchmark tests
func BenchmarkRequestIDGeneration(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		generateRequestID()
	}
}

func BenchmarkLoggingMiddleware(b *testing.B) {
	logging := NewLoggingMiddleware()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	middleware := logging.RequestIDMiddleware(logging.Middleware(handler))
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)
		}
	})
}