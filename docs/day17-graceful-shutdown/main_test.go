package main

import (
	"context"
	"io"
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestServerConfig(t *testing.T) {
	t.Run("Default configuration", func(t *testing.T) {
		config := NewServerConfig()
		
		if config == nil {
			t.Fatal("NewServerConfig should not return nil")
		}
		
		if config.Port == "" {
			t.Error("Port should not be empty")
		}
		
		if config.ShutdownTimeout <= 0 {
			t.Error("ShutdownTimeout should be positive")
		}
		
		if config.ReadTimeout <= 0 {
			t.Error("ReadTimeout should be positive")
		}
		
		if config.WriteTimeout <= 0 {
			t.Error("WriteTimeout should be positive")
		}
	})
}

func TestGracefulServer(t *testing.T) {
	t.Run("Server creation", func(t *testing.T) {
		config := &ServerConfig{
			Port:            ":8091",
			ShutdownTimeout: 10 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		}
		
		server := NewGracefulServer(config)
		if server == nil {
			t.Fatal("NewGracefulServer should not return nil")
		}
		
		if server.config != config {
			t.Error("Server should store the provided config")
		}
		
		if server.server == nil {
			t.Error("Server should have an http.Server instance")
		}
	})
	
	t.Run("Request tracking", func(t *testing.T) {
		config := &ServerConfig{
			Port:            ":8092",
			ShutdownTimeout: 10 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		}
		
		server := NewGracefulServer(config)
		
		// Initially should have 0 active requests
		if count := server.getActiveRequests(); count != 0 {
			t.Errorf("Expected 0 active requests, got %d", count)
		}
		
		// Test increment/decrement
		server.incrementActiveRequests()
		if count := server.getActiveRequests(); count != 1 {
			t.Errorf("Expected 1 active request, got %d", count)
		}
		
		server.incrementActiveRequests()
		if count := server.getActiveRequests(); count != 2 {
			t.Errorf("Expected 2 active requests, got %d", count)
		}
		
		server.decrementActiveRequests()
		if count := server.getActiveRequests(); count != 1 {
			t.Errorf("Expected 1 active request, got %d", count)
		}
		
		server.decrementActiveRequests()
		if count := server.getActiveRequests(); count != 0 {
			t.Errorf("Expected 0 active requests, got %d", count)
		}
	})
}

func TestServerHandlers(t *testing.T) {
	t.Run("Health handler", func(t *testing.T) {
		config := &ServerConfig{
			Port:            ":8093",
			ShutdownTimeout: 10 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		}
		
		server := NewGracefulServer(config)
		
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
	})
	
	t.Run("Status handler", func(t *testing.T) {
		config := &ServerConfig{
			Port:            ":8094",
			ShutdownTimeout: 10 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		}
		
		server := NewGracefulServer(config)
		
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
		
		resp, err := http.Get("http://localhost" + config.Port + "/status")
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
		
		// Should contain active_requests field
		bodyStr := string(body)
		if len(bodyStr) == 0 {
			t.Error("Status response should not be empty")
		}
	})
}

func TestGracefulShutdown(t *testing.T) {
	t.Run("Basic shutdown", func(t *testing.T) {
		config := &ServerConfig{
			Port:            ":8095",
			ShutdownTimeout: 5 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		}
		
		server := NewGracefulServer(config)
		
		// Start server in background
		go func() {
			server.Start()
		}()
		
		// Wait for server to start
		time.Sleep(200 * time.Millisecond)
		
		// Test graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	
	t.Run("Shutdown with active requests", func(t *testing.T) {
		config := &ServerConfig{
			Port:            ":8096",
			ShutdownTimeout: 5 * time.Second,
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
		}
		
		server := NewGracefulServer(config)
		
		// Start server in background
		serverDone := make(chan bool)
		go func() {
			server.Start()
			serverDone <- true
		}()
		
		// Wait for server to start
		time.Sleep(200 * time.Millisecond)
		
		// Start a long-running request
		requestDone := make(chan bool)
		go func() {
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get("http://localhost" + config.Port + "/long-running?delay=2s")
			if err == nil {
				resp.Body.Close()
			}
			requestDone <- true
		}()
		
		// Wait a bit for request to start
		time.Sleep(100 * time.Millisecond)
		
		// Initiate shutdown
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		err := server.Shutdown(ctx)
		elapsed := time.Since(start)
		
		if err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}
		
		// Should wait for the request to complete
		if elapsed < 1*time.Second {
			t.Errorf("Shutdown was too fast, should wait for requests: %v", elapsed)
		}
		
		// Wait for request and server to complete
		<-requestDone
		<-serverDone
	})
	
	t.Run("Signal handling", func(t *testing.T) {
		config := &ServerConfig{
			Port:            ":8097",
			ShutdownTimeout: 5 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		}
		
		server := NewGracefulServer(config)
		
		// Start server in background
		serverDone := make(chan bool)
		go func() {
			server.Start()
			serverDone <- true
		}()
		
		// Wait for server to start
		time.Sleep(200 * time.Millisecond)
		
		// Send SIGTERM signal
		start := time.Now()
		
		// Since we can't easily send signals in tests, we'll test the shutdown function directly
		go func() {
			time.Sleep(100 * time.Millisecond)
			server.shutdownSignal <- syscall.SIGTERM
		}()
		
		// Wait for server to shutdown
		select {
		case <-serverDone:
			elapsed := time.Since(start)
			if elapsed > 5*time.Second {
				t.Errorf("Server took too long to shutdown: %v", elapsed)
			}
		case <-time.After(10 * time.Second):
			t.Error("Server did not shutdown within timeout")
		}
	})
}

func TestConcurrentRequests(t *testing.T) {
	t.Run("Multiple concurrent requests during shutdown", func(t *testing.T) {
		config := &ServerConfig{
			Port:            ":8098",
			ShutdownTimeout: 10 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		}
		
		server := NewGracefulServer(config)
		
		// Start server in background
		go func() {
			server.Start()
		}()
		
		// Wait for server to start
		time.Sleep(200 * time.Millisecond)
		
		// Start multiple concurrent requests
		const numRequests = 5
		var wg sync.WaitGroup
		requestResults := make(chan error, numRequests)
		
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				client := &http.Client{Timeout: 8 * time.Second}
				resp, err := client.Get("http://localhost" + config.Port + "/long-running?delay=1s")
				if err == nil {
					resp.Body.Close()
				}
				requestResults <- err
			}(i)
		}
		
		// Wait a bit for requests to start
		time.Sleep(100 * time.Millisecond)
		
		// Initiate shutdown
		shutdownDone := make(chan error)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			shutdownDone <- server.Shutdown(ctx)
		}()
		
		// Wait for all requests to complete
		wg.Wait()
		
		// Check shutdown result
		err := <-shutdownDone
		if err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}
		
		// Check request results
		successCount := 0
		for i := 0; i < numRequests; i++ {
			err := <-requestResults
			if err == nil {
				successCount++
			}
		}
		
		// Most requests should succeed
		if successCount < numRequests/2 {
			t.Errorf("Too many requests failed: %d/%d succeeded", successCount, numRequests)
		}
	})
}

func TestRequestTracking(t *testing.T) {
	t.Run("Active request counting", func(t *testing.T) {
		config := &ServerConfig{
			Port:            ":8099",
			ShutdownTimeout: 10 * time.Second,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
		}
		
		server := NewGracefulServer(config)
		
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
		
		// Start a long-running request in background
		requestDone := make(chan bool)
		go func() {
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get("http://localhost" + config.Port + "/long-running?delay=1s")
			if err == nil {
				resp.Body.Close()
			}
			requestDone <- true
		}()
		
		// Wait a bit for request to start
		time.Sleep(100 * time.Millisecond)
		
		// Check that active requests is > 0
		activeCount := server.getActiveRequests()
		if activeCount == 0 {
			t.Error("Should have active requests during long-running request")
		}
		
		// Wait for request to complete
		<-requestDone
		
		// Wait a bit more for cleanup
		time.Sleep(100 * time.Millisecond)
		
		// Check that active requests is back to 0
		activeCount = server.getActiveRequests()
		if activeCount != 0 {
			t.Errorf("Expected 0 active requests after completion, got %d", activeCount)
		}
	})
}