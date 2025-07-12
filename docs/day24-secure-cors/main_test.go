package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewCORS(t *testing.T) {
	config := DefaultCORSConfig()
	cors := NewCORS(config)
	
	if cors == nil {
		t.Fatal("NewCORS returned nil")
	}
}

func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()
	
	if config.AllowAllOrigins {
		t.Error("Default config should not allow all origins")
	}
	
	if config.AllowCredentials {
		t.Error("Default config should not allow credentials")
	}
	
	if len(config.AllowedMethods) == 0 {
		t.Error("Default config should have allowed methods")
	}
	
	if config.MaxAge != 86400 {
		t.Errorf("Expected MaxAge 86400, got %d", config.MaxAge)
	}
}

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		allowAll       bool
		origin         string
		expected       bool
	}{
		{
			name:           "Exact match",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://example.com",
			expected:       true,
		},
		{
			name:           "Case insensitive match",
			allowedOrigins: []string{"https://EXAMPLE.com"},
			origin:         "https://example.com",
			expected:       true,
		},
		{
			name:           "Not allowed origin",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://malicious.com",
			expected:       false,
		},
		{
			name:           "Wildcard subdomain match",
			allowedOrigins: []string{"*.example.com"},
			origin:         "https://app.example.com",
			expected:       true,
		},
		{
			name:           "Wildcard subdomain no match",
			allowedOrigins: []string{"*.example.com"},
			origin:         "https://example.com",
			expected:       false,
		},
		{
			name:           "Allow all origins",
			allowedOrigins: []string{},
			allowAll:       true,
			origin:         "https://any-domain.com",
			expected:       true,
		},
		{
			name:           "Multiple origins",
			allowedOrigins: []string{"https://example.com", "https://trusted.com"},
			origin:         "https://trusted.com",
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CORSConfig{
				AllowedOrigins:  tt.allowedOrigins,
				AllowAllOrigins: tt.allowAll,
			}
			cors := NewCORS(config)
			
			result := cors.isOriginAllowed(tt.origin)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsMethodAllowed(t *testing.T) {
	config := CORSConfig{
		AllowedMethods: []string{"GET", "POST", "PUT"},
	}
	cors := NewCORS(config)

	tests := []struct {
		method   string
		expected bool
	}{
		{"GET", true},
		{"POST", true},
		{"PUT", true},
		{"DELETE", false},
		{"PATCH", false},
		{"get", true}, // Case insensitive
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := cors.isMethodAllowed(tt.method)
			if result != tt.expected {
				t.Errorf("Method %s: expected %v, got %v", tt.method, tt.expected, result)
			}
		})
	}
}

func TestIsHeaderAllowed(t *testing.T) {
	config := CORSConfig{
		AllowedHeaders: []string{"Content-Type", "Authorization", "X-Custom"},
	}
	cors := NewCORS(config)

	tests := []struct {
		header   string
		expected bool
	}{
		{"Content-Type", true},
		{"Authorization", true},
		{"X-Custom", true},
		{"content-type", true}, // Case insensitive
		{"X-Forbidden", false},
		{"Host", false},        // Dangerous header
		{"Connection", false},  // Dangerous header
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			result := cors.isHeaderAllowed(tt.header)
			if result != tt.expected {
				t.Errorf("Header %s: expected %v, got %v", tt.header, tt.expected, result)
			}
		})
	}
}

func TestIsDangerousHeader(t *testing.T) {
	dangerousHeaders := []string{
		"Host", "Connection", "Upgrade", "Proxy-Authorization",
		"host", "connection", "upgrade", // Case variations
	}

	for _, header := range dangerousHeaders {
		t.Run(header, func(t *testing.T) {
			if !isDangerousHeader(header) {
				t.Errorf("Header %s should be considered dangerous", header)
			}
		})
	}

	safeHeaders := []string{
		"Content-Type", "Authorization", "X-Custom-Header",
	}

	for _, header := range safeHeaders {
		t.Run(header, func(t *testing.T) {
			if isDangerousHeader(header) {
				t.Errorf("Header %s should not be considered dangerous", header)
			}
		})
	}
}

func TestMatchesWildcard(t *testing.T) {
	tests := []struct {
		pattern  string
		origin   string
		expected bool
	}{
		{"*.example.com", "https://app.example.com", true},
		{"*.example.com", "https://api.example.com", true},
		{"*.example.com", "https://example.com", false},
		{"*.example.com", "https://app.notexample.com", false},
		{"example.com", "https://example.com", false}, // Not a wildcard
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.origin, func(t *testing.T) {
			result := matchesWildcard(tt.pattern, tt.origin)
			if result != tt.expected {
				t.Errorf("Pattern %s with origin %s: expected %v, got %v",
					tt.pattern, tt.origin, tt.expected, result)
			}
		})
	}
}

func TestIsSimpleRequest(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "Simple GET request",
			method:   "GET",
			headers:  map[string]string{},
			expected: true,
		},
		{
			name:     "Simple POST with form data",
			method:   "POST",
			headers:  map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
			expected: true,
		},
		{
			name:     "Simple POST with text",
			method:   "POST",
			headers:  map[string]string{"Content-Type": "text/plain"},
			expected: true,
		},
		{
			name:     "Not simple - JSON content",
			method:   "POST",
			headers:  map[string]string{"Content-Type": "application/json"},
			expected: false,
		},
		{
			name:     "Not simple - custom header",
			method:   "GET",
			headers:  map[string]string{"X-Custom-Header": "value"},
			expected: false,
		},
		{
			name:     "Not simple - PUT method",
			method:   "PUT",
			headers:  map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := isSimpleRequest(req)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSimpleRequestCORS(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowCredentials: true,
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	tests := []struct {
		name           string
		origin         string
		expectedStatus int
		expectedOrigin string
	}{
		{
			name:           "Allowed origin",
			origin:         "https://example.com",
			expectedStatus: http.StatusOK,
			expectedOrigin: "https://example.com",
		},
		{
			name:           "Blocked origin",
			origin:         "https://malicious.com",
			expectedStatus: http.StatusForbidden,
			expectedOrigin: "",
		},
		{
			name:           "No origin header",
			origin:         "",
			expectedStatus: http.StatusOK,
			expectedOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if allowOrigin != tt.expectedOrigin {
				t.Errorf("Expected Access-Control-Allow-Origin %s, got %s",
					tt.expectedOrigin, allowOrigin)
			}

			if tt.expectedStatus == http.StatusOK && tt.origin != "" {
				// Credentials header should be set for allowed origins
				credentials := w.Header().Get("Access-Control-Allow-Credentials")
				if credentials != "true" {
					t.Errorf("Expected Access-Control-Allow-Credentials true, got %s", credentials)
				}
			}
		})
	}
}

func TestPreflightRequest(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "X-Custom-Header"},
		MaxAge:         3600,
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("should not reach here for preflight"))
	}))

	tests := []struct {
		name                    string
		origin                  string
		requestMethod           string
		requestHeaders          string
		expectedStatus          int
		expectAllowMethods      bool
		expectAllowHeaders      bool
		expectMaxAge            bool
	}{
		{
			name:               "Valid preflight",
			origin:             "https://example.com",
			requestMethod:      "PUT",
			requestHeaders:     "Content-Type, Authorization",
			expectedStatus:     http.StatusOK,
			expectAllowMethods: true,
			expectAllowHeaders: true,
			expectMaxAge:       true,
		},
		{
			name:           "Invalid origin",
			origin:         "https://malicious.com",
			requestMethod:  "PUT",
			requestHeaders: "Content-Type",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Invalid method",
			origin:         "https://example.com",
			requestMethod:  "PATCH",
			requestHeaders: "Content-Type",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Invalid header",
			origin:         "https://example.com",
			requestMethod:  "PUT",
			requestHeaders: "X-Forbidden-Header",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("OPTIONS", "/", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", tt.requestMethod)
			if tt.requestHeaders != "" {
				req.Header.Set("Access-Control-Request-Headers", tt.requestHeaders)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectAllowMethods {
				allowMethods := w.Header().Get("Access-Control-Allow-Methods")
				if allowMethods == "" {
					t.Error("Expected Access-Control-Allow-Methods header")
				}
				if !strings.Contains(allowMethods, tt.requestMethod) {
					t.Errorf("Access-Control-Allow-Methods should contain %s", tt.requestMethod)
				}
			}

			if tt.expectAllowHeaders {
				allowHeaders := w.Header().Get("Access-Control-Allow-Headers")
				if allowHeaders == "" {
					t.Error("Expected Access-Control-Allow-Headers header")
				}
			}

			if tt.expectMaxAge {
				maxAge := w.Header().Get("Access-Control-Max-Age")
				if maxAge != "3600" {
					t.Errorf("Expected Access-Control-Max-Age 3600, got %s", maxAge)
				}
			}
		})
	}
}

func TestCORSErrorResponse(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://malicious.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal("Failed to parse JSON response")
	}

	if response["error"] == nil {
		t.Error("Response should contain error field")
	}
}

func TestCORSWithCredentials(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins:   []string{"https://example.com"},
		AllowCredentials: true,
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	credentials := w.Header().Get("Access-Control-Allow-Credentials")
	if credentials != "true" {
		t.Errorf("Expected Access-Control-Allow-Credentials true, got %s", credentials)
	}

	vary := w.Header().Get("Vary")
	if !strings.Contains(vary, "Origin") {
		t.Errorf("Vary header should contain Origin, got %s", vary)
	}
}

func TestCORSExposedHeaders(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"https://example.com"},
		ExposedHeaders: []string{"X-Total-Count", "X-Page-Number"},
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Count", "100")
		w.Header().Set("X-Page-Number", "1")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	exposedHeaders := w.Header().Get("Access-Control-Expose-Headers")
	if !strings.Contains(exposedHeaders, "X-Total-Count") {
		t.Error("Access-Control-Expose-Headers should contain X-Total-Count")
	}
	if !strings.Contains(exposedHeaders, "X-Page-Number") {
		t.Error("Access-Control-Expose-Headers should contain X-Page-Number")
	}
}

func TestCORSAllowAllOrigins(t *testing.T) {
	config := CORSConfig{
		AllowAllOrigins:  true,
		AllowCredentials: false, // Should not be true when allowing all origins
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://any-domain.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin *, got %s", allowOrigin)
	}
}

func TestCORSComplexPreflight(t *testing.T) {
	config := CORSConfig{
		AllowedOrigins: []string{"https://app.example.com"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders: []string{
			"Accept", "Accept-Language", "Content-Language", "Content-Type",
			"Authorization", "X-Requested-With", "X-Custom-Header",
		},
		ExposedHeaders:   []string{"X-Total-Count"},
		AllowCredentials: true,
		MaxAge:           86400,
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Complex preflight request
	req := httptest.NewRequest("OPTIONS", "/api/data", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization, X-Custom-Header")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check all required headers are present
	headers := map[string]string{
		"Access-Control-Allow-Origin":      "https://app.example.com",
		"Access-Control-Allow-Methods":     "",
		"Access-Control-Allow-Headers":     "",
		"Access-Control-Allow-Credentials": "true",
		"Access-Control-Max-Age":           "86400",
	}

	for header, expectedValue := range headers {
		value := w.Header().Get(header)
		if value == "" {
			t.Errorf("Missing required header: %s", header)
		}
		if expectedValue != "" && value != expectedValue {
			t.Errorf("Header %s: expected %s, got %s", header, expectedValue, value)
		}
	}
}

func BenchmarkCORSMiddleware(b *testing.B) {
	config := CORSConfig{
		AllowedOrigins: []string{"https://example.com", "https://app.example.com"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}
	cors := NewCORS(config)

	handler := cors.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
		}
	})
}

func BenchmarkOriginMatching(b *testing.B) {
	config := CORSConfig{
		AllowedOrigins: []string{
			"https://example.com",
			"https://app.example.com",
			"*.trusted.example.com",
			"https://api.example.com",
		},
	}
	cors := NewCORS(config)

	origin := "https://sub.trusted.example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cors.isOriginAllowed(origin)
	}
}