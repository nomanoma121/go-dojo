package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Port            string
	ShutdownTimeout time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
}

// GracefulServer represents a server with graceful shutdown capability
type GracefulServer struct {
	server          *http.Server
	config          *ServerConfig
	shutdownSignal  chan os.Signal
	activeRequests  int64
	requestsMu      sync.RWMutex
	shutdownOnce    sync.Once
	isShuttingDown  bool
}

// NewServerConfig creates default server configuration
func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		Port:            ":8080",
		ShutdownTimeout: 30 * time.Second,
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
	}
}

// NewGracefulServer creates a new server with graceful shutdown capability
func NewGracefulServer(config *ServerConfig) *GracefulServer {
	gs := &GracefulServer{
		config:         config,
		shutdownSignal: make(chan os.Signal, 1),
	}
	
	gs.server = &http.Server{
		Addr:         config.Port,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}
	
	gs.setupRoutes()
	return gs
}

// setupRoutes sets up HTTP routes
func (gs *GracefulServer) setupRoutes() {
	mux := http.NewServeMux()
	
	// Wrap all handlers with request tracking middleware
	mux.Handle("/health", gs.requestTrackingMiddleware(http.HandlerFunc(gs.healthHandler)))
	mux.Handle("/status", gs.requestTrackingMiddleware(http.HandlerFunc(gs.statusHandler)))
	mux.Handle("/long-running", gs.requestTrackingMiddleware(http.HandlerFunc(gs.longRunningHandler)))
	mux.Handle("/shutdown", gs.requestTrackingMiddleware(http.HandlerFunc(gs.shutdownHandler)))
	
	gs.server.Handler = mux
}

// Start starts the server and sets up signal handling
func (gs *GracefulServer) Start() error {
	gs.setupSignalHandling()
	
	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Server starting on %s", gs.config.Port)
		if err := gs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()
	
	// Wait for shutdown signal
	select {
	case err := <-serverErr:
		return err
	case sig := <-gs.shutdownSignal:
		log.Printf("Received signal: %v", sig)
		
		// Perform graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), gs.config.ShutdownTimeout)
		defer cancel()
		
		return gs.Shutdown(ctx)
	}
}

// setupSignalHandling sets up signal handling for graceful shutdown
func (gs *GracefulServer) setupSignalHandling() {
	signal.Notify(gs.shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
}

// Shutdown gracefully shuts down the server
func (gs *GracefulServer) Shutdown(ctx context.Context) error {
	var shutdownErr error
	
	gs.shutdownOnce.Do(func() {
		log.Println("Initiating graceful shutdown...")
		
		gs.requestsMu.Lock()
		gs.isShuttingDown = true
		gs.requestsMu.Unlock()
		
		// Stop accepting new connections
		shutdownErr = gs.server.Shutdown(ctx)
		
		// Wait for active requests to complete
		for {
			activeCount := atomic.LoadInt64(&gs.activeRequests)
			if activeCount == 0 {
				break
			}
			
			log.Printf("Waiting for %d active requests to complete...", activeCount)
			
			select {
			case <-ctx.Done():
				log.Printf("Shutdown timeout reached, forcefully closing with %d active requests", activeCount)
				return
			case <-time.After(100 * time.Millisecond):
				// Continue waiting
			}
		}
		
		log.Println("All requests completed, server stopped gracefully")
	})
	
	return shutdownErr
}

// requestTrackingMiddleware tracks active requests
func (gs *GracefulServer) requestTrackingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if server is shutting down
		gs.requestsMu.RLock()
		if gs.isShuttingDown {
			gs.requestsMu.RUnlock()
			http.Error(w, "Server is shutting down", http.StatusServiceUnavailable)
			return
		}
		gs.requestsMu.RUnlock()
		
		// Increment active request counter
		gs.incrementActiveRequests()
		defer gs.decrementActiveRequests()
		
		// Process request
		next.ServeHTTP(w, r)
	})
}

// incrementActiveRequests safely increments the active request counter
func (gs *GracefulServer) incrementActiveRequests() {
	atomic.AddInt64(&gs.activeRequests, 1)
}

// decrementActiveRequests safely decrements the active request counter
func (gs *GracefulServer) decrementActiveRequests() {
	atomic.AddInt64(&gs.activeRequests, -1)
}

// getActiveRequests safely gets the active request count
func (gs *GracefulServer) getActiveRequests() int64 {
	return atomic.LoadInt64(&gs.activeRequests)
}

// healthHandler handles health check requests
func (gs *GracefulServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(time.Now()).String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// statusHandler returns server status including active requests
func (gs *GracefulServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"active_requests": gs.getActiveRequests(),
		"is_shutting_down": gs.isShuttingDown,
		"server_config": map[string]interface{}{
			"port":             gs.config.Port,
			"shutdown_timeout": gs.config.ShutdownTimeout.String(),
			"read_timeout":     gs.config.ReadTimeout.String(),
			"write_timeout":    gs.config.WriteTimeout.String(),
		},
		"timestamp": time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// longRunningHandler simulates a long-running request
func (gs *GracefulServer) longRunningHandler(w http.ResponseWriter, r *http.Request) {
	// Get delay from query parameter, default to 2 seconds
	delayStr := r.URL.Query().Get("delay")
	delay := 2 * time.Second
	
	if delayStr != "" {
		if parsedDelay, err := time.ParseDuration(delayStr); err == nil {
			delay = parsedDelay
		}
	}
	
	log.Printf("Starting long-running request with delay: %v", delay)
	
	// Use request context to handle cancellation
	ctx := r.Context()
	
	select {
	case <-time.After(delay):
		response := map[string]interface{}{
			"message": "Long-running operation completed",
			"delay":   delay.String(),
			"time":    time.Now().Unix(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		
		log.Printf("Long-running request completed successfully")
		
	case <-ctx.Done():
		log.Printf("Long-running request cancelled: %v", ctx.Err())
		http.Error(w, "Request cancelled", http.StatusRequestTimeout)
	}
}

// shutdownHandler initiates graceful shutdown (for testing)
func (gs *GracefulServer) shutdownHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message": "Shutdown initiated",
		"time":    time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	
	// Trigger shutdown in a separate goroutine
	go func() {
		time.Sleep(100 * time.Millisecond) // Give time for response to be sent
		gs.shutdownSignal <- syscall.SIGTERM
	}()
}

func main() {
	config := NewServerConfig()
	
	// Allow port override from environment
	if port := os.Getenv("PORT"); port != "" {
		config.Port = ":" + port
	}
	
	// Allow timeout override from environment
	if timeoutStr := os.Getenv("SHUTDOWN_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			config.ShutdownTimeout = time.Duration(timeout) * time.Second
		}
	}
	
	server := NewGracefulServer(config)
	
	fmt.Printf("Starting server on %s\n", config.Port)
	fmt.Printf("Shutdown timeout: %v\n", config.ShutdownTimeout)
	fmt.Println("Send SIGINT (Ctrl+C) or SIGTERM to gracefully shutdown")
	fmt.Println("Endpoints:")
	fmt.Println("  GET  /health        - Health check")
	fmt.Println("  GET  /status        - Server status")
	fmt.Println("  GET  /long-running  - Simulate long request (use ?delay=5s)")
	fmt.Println("  POST /shutdown      - Trigger graceful shutdown")
	
	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	
	fmt.Println("Server stopped gracefully")
}