package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoveryMiddleware(t *testing.T) {
	t.Run("Normal request handling", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))

		recovery := NewRecoveryMiddleware(&RecoveryOptions{
			Logger:       logger,
			IncludeStack: true,
		})

		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		if body := rr.Body.String(); body != "Success" {
			t.Errorf("Expected 'Success', got '%s'", body)
		}

		// Should not log anything for normal requests
		if logBuffer.Len() > 0 {
			t.Error("Expected no logs for normal request")
		}
	})

	t.Run("String panic recovery", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		recovery := NewRecoveryMiddleware(&RecoveryOptions{
			Logger:       logger,
			IncludeStack: true,
			CustomMsg:    "Custom error message",
		})

		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic message")
		}))

		req := httptest.NewRequest("GET", "/panic", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Check response status
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}

		// Check response content type
		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", ct)
		}

		// Check response body
		var response ErrorResponse
		if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Error != "Internal Server Error" {
			t.Errorf("Expected error 'Internal Server Error', got '%s'", response.Error)
		}

		if response.Message != "Custom error message" {
			t.Errorf("Expected message 'Custom error message', got '%s'", response.Message)
		}

		if response.Timestamp == 0 {
			t.Error("Expected timestamp to be set")
		}

		// Check logs
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "panic recovered") {
			t.Error("Expected log to contain 'panic recovered'")
		}

		if !strings.Contains(logOutput, "test panic message") {
			t.Error("Expected log to contain panic message")
		}
	})

	t.Run("Error panic recovery", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		recovery := NewRecoveryMiddleware(&RecoveryOptions{
			Logger: logger,
		})

		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(http.ErrAbortHandler)
		}))

		req := httptest.NewRequest("GET", "/panic", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}

		// Check logs contain error message
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, http.ErrAbortHandler.Error()) {
			t.Error("Expected log to contain error message")
		}
	})

	t.Run("Non-string/error panic recovery", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		recovery := NewRecoveryMiddleware(&RecoveryOptions{
			Logger: logger,
		})

		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(42)
		}))

		req := httptest.NewRequest("GET", "/panic", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}

		// Check logs contain formatted panic value
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "42") {
			t.Error("Expected log to contain '42'")
		}
	})

	t.Run("Request ID in context", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		recovery := NewRecoveryMiddleware(&RecoveryOptions{
			Logger: logger,
		})

		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test with request ID")
		}))

		req := httptest.NewRequest("GET", "/panic", nil)
		// Add request ID to context
		ctx := context.WithValue(req.Context(), RequestIDKey, "test-request-123")
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		// Check response includes request ID
		var response ErrorResponse
		json.NewDecoder(rr.Body).Decode(&response)
		if response.RequestID != "test-request-123" {
			t.Errorf("Expected request ID 'test-request-123', got '%s'", response.RequestID)
		}

		// Check logs include request ID
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "test-request-123") {
			t.Error("Expected log to contain request ID")
		}
	})

	t.Run("Stack trace inclusion", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		recovery := NewRecoveryMiddleware(&RecoveryOptions{
			Logger:       logger,
			IncludeStack: true,
		})

		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic for stack trace")
		}))

		req := httptest.NewRequest("GET", "/panic", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Check logs include stack trace
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "stack_trace") {
			t.Error("Expected log to contain stack trace when IncludeStack is true")
		}
	})

	t.Run("Stack trace exclusion", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		recovery := NewRecoveryMiddleware(&RecoveryOptions{
			Logger:       logger,
			IncludeStack: false,
		})

		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic without stack trace")
		}))

		req := httptest.NewRequest("GET", "/panic", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Check logs don't include stack trace
		logOutput := logBuffer.String()
		if strings.Contains(logOutput, "stack_trace") {
			t.Error("Expected log to not contain stack trace when IncludeStack is false")
		}
	})

	t.Run("Request information logging", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		recovery := NewRecoveryMiddleware(&RecoveryOptions{
			Logger: logger,
		})

		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic for request info")
		}))

		req := httptest.NewRequest("POST", "/api/test?param=value", nil)
		req.Header.Set("User-Agent", "test-agent/1.0")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Check logs include request information
		logOutput := logBuffer.String()
		if !strings.Contains(logOutput, "POST") {
			t.Error("Expected log to contain HTTP method")
		}
		if !strings.Contains(logOutput, "/api/test") {
			t.Error("Expected log to contain request URL")
		}
		if !strings.Contains(logOutput, "test-agent/1.0") {
			t.Error("Expected log to contain User-Agent")
		}
	})

	t.Run("Default options", func(t *testing.T) {
		recovery := NewRecoveryMiddleware(nil)
		
		if recovery == nil {
			t.Fatal("Expected recovery middleware to be created with nil options")
		}

		// Test that it works with default options
		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test with defaults")
		}))

		req := httptest.NewRequest("GET", "/panic", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}
	})

	t.Run("Multiple panics handling", func(t *testing.T) {
		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))

		recovery := NewRecoveryMiddleware(&RecoveryOptions{
			Logger: logger,
		})

		handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("first panic")
		}))

		// First panic
		req1 := httptest.NewRequest("GET", "/panic1", nil)
		rr1 := httptest.NewRecorder()
		handler.ServeHTTP(rr1, req1)

		// Second panic
		req2 := httptest.NewRequest("GET", "/panic2", nil)
		rr2 := httptest.NewRecorder()
		handler.ServeHTTP(rr2, req2)

		// Both should return 500
		if rr1.Code != http.StatusInternalServerError {
			t.Errorf("Expected first request status 500, got %d", rr1.Code)
		}
		if rr2.Code != http.StatusInternalServerError {
			t.Errorf("Expected second request status 500, got %d", rr2.Code)
		}

		// Should have logged both panics
		logOutput := logBuffer.String()
		panicCount := strings.Count(logOutput, "panic recovered")
		if panicCount != 2 {
			t.Errorf("Expected 2 panic logs, got %d", panicCount)
		}
	})
}

func TestFormatPanicValue(t *testing.T) {
	recovery := NewRecoveryMiddleware(nil)

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "String panic",
			input:    "test error",
			expected: "test error",
		},
		{
			name:     "Error panic",
			input:    http.ErrAbortHandler,
			expected: http.ErrAbortHandler.Error(),
		},
		{
			name:     "Integer panic",
			input:    42,
			expected: "42",
		},
		{
			name:     "Nil panic",
			input:    nil,
			expected: "<nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := recovery.formatPanicValue(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetRequestID(t *testing.T) {
	t.Run("With request ID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RequestIDKey, "test-123")
		result := getRequestID(ctx)
		if result != "test-123" {
			t.Errorf("Expected 'test-123', got '%s'", result)
		}
	})

	t.Run("Without request ID", func(t *testing.T) {
		ctx := context.Background()
		result := getRequestID(ctx)
		if result != "" {
			t.Errorf("Expected empty string, got '%s'", result)
		}
	})

	t.Run("Wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RequestIDKey, 123)
		result := getRequestID(ctx)
		if result != "" {
			t.Errorf("Expected empty string for wrong type, got '%s'", result)
		}
	})
}

// Benchmark tests
func BenchmarkRecoveryMiddleware_Normal(b *testing.B) {
	recovery := NewRecoveryMiddleware(nil)
	handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkRecoveryMiddleware_Panic(b *testing.B) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	recovery := NewRecoveryMiddleware(&RecoveryOptions{
		Logger:       logger,
		IncludeStack: false, // Disable stack trace for performance
	})

	handler := recovery.Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("benchmark panic")
	}))

	req := httptest.NewRequest("GET", "/panic", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logBuffer.Reset()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}