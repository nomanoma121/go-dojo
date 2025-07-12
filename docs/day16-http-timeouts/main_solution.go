package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	Port              string
}

// TimeoutServer represents an HTTP server with proper timeout configuration
type TimeoutServer struct {
	server *http.Server
	config *ServerConfig
}

// NewServerConfig creates default server configuration
func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		Port:              ":8080",
	}
}

// NewTimeoutServer creates a new server with timeout configuration
func NewTimeoutServer(config *ServerConfig) *TimeoutServer {
	ts := &TimeoutServer{
		config: config,
	}
	
	// Create HTTP server with timeout configuration
	ts.server = &http.Server{
		Addr:              config.Port,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
	}
	
	ts.setupRoutes()
	return ts
}

// setupRoutes sets up HTTP routes
func (ts *TimeoutServer) setupRoutes() {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/health", ts.healthHandler)
	mux.HandleFunc("/slow", ts.slowHandler)
	mux.HandleFunc("/api/data", ts.dataHandler)
	mux.HandleFunc("/api/timeout-test", ts.timeoutTestHandler)
	
	ts.server.Handler = mux
}

// Start starts the server
func (ts *TimeoutServer) Start() error {
	return ts.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (ts *TimeoutServer) Shutdown(ctx context.Context) error {
	return ts.server.Shutdown(ctx)
}

// healthHandler handles health check requests
func (ts *TimeoutServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"timeouts": map[string]string{
			"read":        ts.config.ReadTimeout.String(),
			"write":       ts.config.WriteTimeout.String(),
			"idle":        ts.config.IdleTimeout.String(),
			"read_header": ts.config.ReadHeaderTimeout.String(),
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// dataHandler handles normal data requests
func (ts *TimeoutServer) dataHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ts.handleGetData(w, r)
	case http.MethodPost:
		ts.handlePostData(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetData handles GET requests for data
func (ts *TimeoutServer) handleGetData(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"message": "Data retrieved successfully",
		"items": []map[string]interface{}{
			{"id": 1, "name": "Item 1"},
			{"id": 2, "name": "Item 2"},
			{"id": 3, "name": "Item 3"},
		},
		"timestamp": time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// handlePostData handles POST requests for data
func (ts *TimeoutServer) handlePostData(w http.ResponseWriter, r *http.Request) {
	var requestData map[string]interface{}
	
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	response := map[string]interface{}{
		"message":     "Data processed successfully",
		"received":    requestData,
		"processed_at": time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// slowHandler simulates a slow endpoint
func (ts *TimeoutServer) slowHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate slow processing
	delay := 5 * time.Second
	
	// Check if client specified a delay
	if delayParam := r.URL.Query().Get("delay"); delayParam != "" {
		if parsedDelay, err := time.ParseDuration(delayParam); err == nil {
			delay = parsedDelay
		}
	}
	
	// Use context to respect request timeout
	ctx := r.Context()
	select {
	case <-time.After(delay):
		// Normal processing completed
		response := map[string]interface{}{
			"message": "Slow processing completed",
			"delay":   delay.String(),
			"time":    time.Now().Unix(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		
	case <-ctx.Done():
		// Request was cancelled or timed out
		http.Error(w, "Request timeout", http.StatusRequestTimeout)
	}
}

// timeoutTestHandler tests various timeout scenarios
func (ts *TimeoutServer) timeoutTestHandler(w http.ResponseWriter, r *http.Request) {
	testType := r.URL.Query().Get("type")
	
	switch testType {
	case "read":
		ts.testReadTimeout(w, r)
	case "write":
		ts.testWriteTimeout(w, r)
	case "header":
		ts.testHeaderTimeout(w, r)
	default:
		ts.sendTimeoutTestInfo(w, r)
	}
}

// testReadTimeout tests read timeout
func (ts *TimeoutServer) testReadTimeout(w http.ResponseWriter, r *http.Request) {
	// This would typically be triggered by a client sending data very slowly
	response := map[string]interface{}{
		"test":    "read_timeout",
		"message": "Read timeout test - send data slowly to trigger",
		"timeout": ts.config.ReadTimeout.String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// testWriteTimeout tests write timeout
func (ts *TimeoutServer) testWriteTimeout(w http.ResponseWriter, r *http.Request) {
	// Simulate slow write by sending large amount of data
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	largeData := make([]byte, 1024*1024) // 1MB
	for i := range largeData {
		largeData[i] = 'A'
	}
	
	// Try to write data in chunks with delays
	chunkSize := 1024
	for i := 0; i < len(largeData); i += chunkSize {
		end := i + chunkSize
		if end > len(largeData) {
			end = len(largeData)
		}
		
		_, err := w.Write(largeData[i:end])
		if err != nil {
			return // Connection closed or timeout
		}
		
		// Small delay to potentially trigger write timeout
		time.Sleep(10 * time.Millisecond)
		
		// Flush if possible
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}

// testHeaderTimeout tests header read timeout
func (ts *TimeoutServer) testHeaderTimeout(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"test":    "header_timeout",
		"message": "Header timeout test - send headers slowly to trigger",
		"timeout": ts.config.ReadHeaderTimeout.String(),
		"headers": r.Header,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendTimeoutTestInfo sends information about available timeout tests
func (ts *TimeoutServer) sendTimeoutTestInfo(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message": "Timeout test endpoint",
		"tests": map[string]string{
			"read":   "/api/timeout-test?type=read",
			"write":  "/api/timeout-test?type=write", 
			"header": "/api/timeout-test?type=header",
		},
		"config": map[string]string{
			"read_timeout":        ts.config.ReadTimeout.String(),
			"write_timeout":       ts.config.WriteTimeout.String(),
			"read_header_timeout": ts.config.ReadHeaderTimeout.String(),
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}