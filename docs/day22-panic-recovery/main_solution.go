package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"time"
)

// contextKey is used for context keys to avoid collisions
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
)

// ErrorResponse represents the JSON error response structure
type ErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	RequestID string `json:"request_id,omitempty"`
}

// RecoveryMiddleware provides panic recovery functionality
type RecoveryMiddleware struct {
	logger       *slog.Logger
	includeStack bool
	customMsg    string
}

// RecoveryOptions configures the recovery middleware
type RecoveryOptions struct {
	Logger       *slog.Logger
	IncludeStack bool
	CustomMsg    string
}

// NewRecoveryMiddleware creates a new recovery middleware
func NewRecoveryMiddleware(opts *RecoveryOptions) *RecoveryMiddleware {
	rm := &RecoveryMiddleware{
		includeStack: false,
		customMsg:    "An internal error occurred",
	}

	// Set default logger if none provided
	if opts == nil {
		rm.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))
		return rm
	}

	if opts.Logger != nil {
		rm.logger = opts.Logger
	} else {
		rm.logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))
	}

	rm.includeStack = opts.IncludeStack
	if opts.CustomMsg != "" {
		rm.customMsg = opts.CustomMsg
	}

	return rm
}

// Recover is the main recovery middleware
func (rm *RecoveryMiddleware) Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with detailed information
				rm.logPanic(r, err)

				// Send error response to client
				rm.sendErrorResponse(w, r, http.StatusInternalServerError, rm.customMsg)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// formatPanicValue converts panic value to string
func (rm *RecoveryMiddleware) formatPanicValue(panicValue interface{}) string {
	switch v := panicValue.(type) {
	case error:
		return v.Error()
	case string:
		return v
	case nil:
		return "<nil>"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// logPanic logs the panic with detailed information
func (rm *RecoveryMiddleware) logPanic(r *http.Request, panicValue interface{}) {
	errorMsg := rm.formatPanicValue(panicValue)
	requestID := getRequestID(r.Context())

	// Prepare log attributes
	attrs := []slog.Attr{
		slog.String("error", errorMsg),
		slog.String("method", r.Method),
		slog.String("url", r.URL.String()),
		slog.String("user_agent", r.UserAgent()),
		slog.String("remote_addr", r.RemoteAddr),
	}

	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}

	if rm.includeStack {
		attrs = append(attrs, slog.String("stack_trace", string(debug.Stack())))
	}

	// Log the panic
	rm.logger.LogAttrs(r.Context(), slog.LevelError, "panic recovered", attrs...)
}

// sendErrorResponse sends a JSON error response to the client
func (rm *RecoveryMiddleware) sendErrorResponse(w http.ResponseWriter, r *http.Request, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	response := ErrorResponse{
		Error:     http.StatusText(code),
		Message:   message,
		Timestamp: time.Now().Unix(),
	}

	// Include request ID if available
	if requestID := getRequestID(r.Context()); requestID != "" {
		response.RequestID = requestID
	}

	json.NewEncoder(w).Encode(response)
}

// getRequestID extracts request ID from context if available
func getRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

func main() {
	// Create recovery middleware with options
	recovery := NewRecoveryMiddleware(&RecoveryOptions{
		IncludeStack: true,
		CustomMsg:    "An internal error occurred",
	})

	mux := http.NewServeMux()

	// Normal endpoint
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})

	// Endpoint that panics with string
	mux.HandleFunc("/panic-string", func(w http.ResponseWriter, r *http.Request) {
		panic("This is a string panic!")
	})

	// Endpoint that panics with error
	mux.HandleFunc("/panic-error", func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})

	// Endpoint that panics with other type
	mux.HandleFunc("/panic-int", func(w http.ResponseWriter, r *http.Request) {
		panic(42)
	})

	// Endpoint that causes nil pointer dereference
	mux.HandleFunc("/panic-nil", func(w http.ResponseWriter, r *http.Request) {
		var m map[string]string
		_ = m["key"] // This will panic
	})

	// Add recovery middleware
	handler := recovery.Recover(mux)

	slog.Info("Server starting on :8080")
	slog.Info("Test endpoints:")
	slog.Info("  GET /hello - Normal endpoint")
	slog.Info("  GET /panic-string - String panic")
	slog.Info("  GET /panic-error - Error panic")
	slog.Info("  GET /panic-int - Integer panic")
	slog.Info("  GET /panic-nil - Nil pointer panic")

	http.ListenAndServe(":8080", handler)
}