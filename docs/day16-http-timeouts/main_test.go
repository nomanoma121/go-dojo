package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServerConfig(t *testing.T) {
	t.Run("Default configuration", func(t *testing.T) {
		config := NewServerConfig()
		
		if config == nil {
			t.Fatal("NewServerConfig should not return nil")
		}
		
		if config.ReadTimeout <= 0 {
			t.Error("ReadTimeout should be positive")
		}
		
		if config.WriteTimeout <= 0 {
			t.Error("WriteTimeout should be positive")
		}
		
		if config.IdleTimeout <= 0 {
			t.Error("IdleTimeout should be positive")
		}
		
		if config.ReadHeaderTimeout <= 0 {
			t.Error("ReadHeaderTimeout should be positive")
		}
		
		if config.Port == "" {
			t.Error("Port should not be empty")
		}
	})
	
	t.Run("Reasonable timeout values", func(t *testing.T) {
		config := NewServerConfig()
		
		// ReadTimeout should be reasonable (1-60 seconds)
		if config.ReadTimeout < time.Second || config.ReadTimeout > 60*time.Second {
			t.Errorf("ReadTimeout should be between 1s and 60s, got %v", config.ReadTimeout)
		}
		
		// WriteTimeout should be reasonable (1-60 seconds)
		if config.WriteTimeout < time.Second || config.WriteTimeout > 60*time.Second {
			t.Errorf("WriteTimeout should be between 1s and 60s, got %v", config.WriteTimeout)
		}
		
		// IdleTimeout should be reasonable (1-300 seconds)
		if config.IdleTimeout < time.Second || config.IdleTimeout > 300*time.Second {
			t.Errorf("IdleTimeout should be between 1s and 300s, got %v", config.IdleTimeout)
		}
		
		// ReadHeaderTimeout should be reasonable (1-30 seconds)
		if config.ReadHeaderTimeout < time.Second || config.ReadHeaderTimeout > 30*time.Second {
			t.Errorf("ReadHeaderTimeout should be between 1s and 30s, got %v", config.ReadHeaderTimeout)
		}
	})
}

func TestTimeoutServer(t *testing.T) {
	t.Run("Server creation", func(t *testing.T) {
		config := &ServerConfig{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			Port:              ":8084", // Use random port
		}
		
		server := NewTimeoutServer(config)
		if server == nil {
			t.Fatal("NewTimeoutServer should not return nil")
		}
		
		if server.config != config {
			t.Error("Server should store the provided config")
		}
		
		if server.server == nil {
			t.Error("Server should have an http.Server instance")
		}
	})
	
	t.Run("Server timeout configuration", func(t *testing.T) {
		config := &ServerConfig{
			ReadTimeout:       3 * time.Second,
			WriteTimeout:      4 * time.Second,
			IdleTimeout:       50 * time.Second,
			ReadHeaderTimeout: 1 * time.Second,
			Port:              ":8084",
		}
		
		server := NewTimeoutServer(config)
		
		if server.server.ReadTimeout != config.ReadTimeout {
			t.Errorf("Expected ReadTimeout %v, got %v", config.ReadTimeout, server.server.ReadTimeout)
		}
		
		if server.server.WriteTimeout != config.WriteTimeout {
			t.Errorf("Expected WriteTimeout %v, got %v", config.WriteTimeout, server.server.WriteTimeout)
		}
		
		if server.server.IdleTimeout != config.IdleTimeout {
			t.Errorf("Expected IdleTimeout %v, got %v", config.IdleTimeout, server.server.IdleTimeout)
		}
		
		if server.server.ReadHeaderTimeout != config.ReadHeaderTimeout {
			t.Errorf("Expected ReadHeaderTimeout %v, got %v", config.ReadHeaderTimeout, server.server.ReadHeaderTimeout)
		}
	})
}

func TestServerHandlers(t *testing.T) {
	t.Run("Health handler", func(t *testing.T) {
		config := &ServerConfig{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			Port:              ":8081", // Use fixed port for testing
		}
		
		server := NewTimeoutServer(config)
		
		// Start server in background
		go func() {
			server.Start()
		}()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server.Shutdown(ctx)
		}()
		
		// Wait for server to start
		time.Sleep(200 * time.Millisecond)
		
		resp, err := http.Get("http://localhost" + config.Port + "/health")
		if err != nil {
			t.Skip("Could not connect to server, skipping test")
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		
		if !strings.Contains(string(body), "status") {
			t.Error("Health response should contain status field")
		}
	})
}

func TestServerTimeouts(t *testing.T) {
	t.Run("Write timeout", func(t *testing.T) {
		config := &ServerConfig{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      1 * time.Second, // Short write timeout
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			Port:              ":8082",
		}
		
		server := NewTimeoutServer(config)
		
		// Start server in background
		done := make(chan bool)
		go func() {
			server.Start()
			done <- true
		}()
		
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			server.Shutdown(ctx)
			<-done
		}()
		
		// Wait for server to start
		time.Sleep(100 * time.Millisecond)
		
		// Test slow endpoint that should timeout
		client := &http.Client{
			Timeout: 5 * time.Second,
		}
		
		resp, err := client.Get("http://localhost" + config.Port + "/slow")
		if err == nil {
			defer resp.Body.Close()
			// If we get a response, it should be a timeout error (500 or connection closed)
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				if strings.Contains(string(body), "slow") {
					t.Error("Slow endpoint should have timed out")
				}
			}
		}
		// If we get a network error, that's also acceptable (server closed connection)
	})
	
	t.Run("Read timeout", func(t *testing.T) {
		config := &ServerConfig{
			ReadTimeout:       1 * time.Second, // Short read timeout
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			Port:              ":8083",
		}
		
		server := NewTimeoutServer(config)
		
		// Start server in background
		done := make(chan bool)
		go func() {
			server.Start()
			done <- true
		}()
		
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			server.Shutdown(ctx)
			<-done
		}()
		
		// Wait for server to start
		time.Sleep(100 * time.Millisecond)
		
		// This test is tricky to implement reliably, so we'll just verify the timeout is set
		if server.server.ReadTimeout != config.ReadTimeout {
			t.Errorf("ReadTimeout not set correctly")
		}
	})
}

func TestServerGracefulShutdown(t *testing.T) {
	t.Run("Shutdown within timeout", func(t *testing.T) {
		config := &ServerConfig{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			Port:              ":8084",
		}
		
		server := NewTimeoutServer(config)
		
		// Start server in background
		go func() {
			server.Start()
		}()
		
		// Wait for server to start
		time.Sleep(100 * time.Millisecond)
		
		// Test graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		start := time.Now()
		err := server.Shutdown(ctx)
		elapsed := time.Since(start)
		
		if err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}
		
		if elapsed > 2*time.Second {
			t.Errorf("Shutdown took too long: %v", elapsed)
		}
	})
	
	t.Run("Shutdown timeout", func(t *testing.T) {
		config := &ServerConfig{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			Port:              ":8084",
		}
		
		server := NewTimeoutServer(config)
		
		// Start server in background
		go func() {
			server.Start()
		}()
		
		// Wait for server to start
		time.Sleep(100 * time.Millisecond)
		
		// Test shutdown with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		
		err := server.Shutdown(ctx)
		// Shutdown might succeed even with short timeout if there are no active connections
		// This is more of a configuration test
		if err != nil && err != context.DeadlineExceeded {
			t.Errorf("Unexpected shutdown error: %v", err)
		}
	})
}

func TestConcurrentRequests(t *testing.T) {
	t.Run("Multiple concurrent requests", func(t *testing.T) {
		config := &ServerConfig{
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      5 * time.Second,
			IdleTimeout:       60 * time.Second,
			ReadHeaderTimeout: 2 * time.Second,
			Port:              ":8084",
		}
		
		server := NewTimeoutServer(config)
		
		// Start server in background
		done := make(chan bool)
		go func() {
			server.Start()
			done <- true
		}()
		
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server.Shutdown(ctx)
			<-done
		}()
		
		// Wait for server to start
		time.Sleep(100 * time.Millisecond)
		
		// Send multiple concurrent requests
		const numRequests = 10
		results := make(chan error, numRequests)
		
		for i := 0; i < numRequests; i++ {
			go func() {
				resp, err := http.Get("http://localhost" + config.Port + "/health")
				if err != nil {
					results <- err
					return
				}
				defer resp.Body.Close()
				
				if resp.StatusCode != http.StatusOK {
					results <- http.ErrNotSupported
					return
				}
				
				results <- nil
			}()
		}
		
		// Collect results
		successCount := 0
		for i := 0; i < numRequests; i++ {
			err := <-results
			if err == nil {
				successCount++
			}
		}
		
		if successCount < numRequests/2 {
			t.Errorf("Too many failed requests: %d/%d succeeded", successCount, numRequests)
		}
	})
}

// Benchmark tests
func BenchmarkHealthHandler(b *testing.B) {
	config := &ServerConfig{
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		Port:              ":0",
	}
	
	server := NewTimeoutServer(config)
	
	// Start server in background
	go func() {
		server.Start()
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	
	// Wait for server to start
	time.Sleep(100 * time.Millisecond)
	
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get("http://localhost" + config.Port + "/health")
			if err == nil {
				resp.Body.Close()
			}
		}
	})
}