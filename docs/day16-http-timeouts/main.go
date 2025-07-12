//go:build ignore

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

// TimeoutServer represents an HTTP server with proper timeout configuration
type TimeoutServer struct {
	server *http.Server
	config *ServerConfig
}

// NewTimeoutServer creates a new server with timeout configuration
func NewTimeoutServer(config *ServerConfig) *TimeoutServer {
	mux := http.NewServeMux()
	
	server := &http.Server{
		Addr:              config.Port,
		Handler:           mux,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
	}
	
	ts := &TimeoutServer{
		server: server,
		config: config,
	}
	
	ts.setupRoutes()
	return ts
}

// Start starts the server
func (ts *TimeoutServer) Start() error {
	return ts.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (ts *TimeoutServer) Shutdown(ctx context.Context) error {
	return ts.server.Shutdown(ctx)
}

// setupRoutes sets up HTTP routes
func (ts *TimeoutServer) setupRoutes() {
	mux := ts.server.Handler.(*http.ServeMux)
	mux.HandleFunc("/health", ts.healthHandler)
	mux.HandleFunc("/slow", ts.slowHandler)
}

// healthHandler handles health check requests
func (ts *TimeoutServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "ok",
		"timestamp": time.Now().Unix(),
		"server_config": map[string]string{
			"read_timeout":        ts.config.ReadTimeout.String(),
			"write_timeout":       ts.config.WriteTimeout.String(),
			"idle_timeout":        ts.config.IdleTimeout.String(),
			"read_header_timeout": ts.config.ReadHeaderTimeout.String(),
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// slowHandler simulates a slow endpoint
func (ts *TimeoutServer) slowHandler(w http.ResponseWriter, r *http.Request) {
	// Parse delay parameter from query string
	delay := 15 * time.Second // Default delay longer than write timeout
	if delayParam := r.URL.Query().Get("delay"); delayParam != "" {
		if d, err := time.ParseDuration(delayParam); err == nil {
			delay = d
		}
	}
	
	// Simulate slow processing
	select {
	case <-time.After(delay):
		response := map[string]interface{}{
			"message": "Request completed successfully",
			"delay":   delay.String(),
			"timestamp": time.Now().Unix(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		
	case <-r.Context().Done():
		// Request was cancelled (likely due to timeout)
		http.Error(w, "Request timeout", http.StatusRequestTimeout)
	}
}

func main() {
	config := NewServerConfig()
	server := NewTimeoutServer(config)
	
	println("Starting server on", config.Port)
	if err := server.Start(); err != nil {
		println("Server error:", err.Error())
	}
}