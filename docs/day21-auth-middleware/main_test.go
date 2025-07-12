package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthMiddleware(t *testing.T) {
	auth := NewAuthMiddleware("test-secret")

	t.Run("JWT Authentication", func(t *testing.T) {
		// Create a test handler
		handler := auth.JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, exists := getUserFromContext(r.Context())
			if !exists {
				t.Error("Expected user in context")
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"user_id": user.ID})
		}))

		t.Run("Valid JWT token", func(t *testing.T) {
			token := auth.GenerateJWT("test123", "test@example.com", []string{"user"}, time.Hour)
			
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			
			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)
			if response["user_id"] != "test123" {
				t.Errorf("Expected user_id 'test123', got %v", response["user_id"])
			}
		})

		t.Run("Missing Authorization header", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})

		t.Run("Invalid Bearer format", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Basic sometoken")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})

		t.Run("Invalid JWT token", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer invalid.token.here")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})

		t.Run("Expired JWT token", func(t *testing.T) {
			// Generate an expired token
			expiredToken := auth.GenerateJWT("test123", "test@example.com", []string{"user"}, -time.Hour)
			
			req := httptest.NewRequest("GET", "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+expiredToken)
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})
	})

	t.Run("API Key Authentication", func(t *testing.T) {
		handler := auth.APIKeyAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, exists := getUserFromContext(r.Context())
			if !exists {
				t.Error("Expected user in context")
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{"user_id": user.ID})
		}))

		t.Run("Valid API key", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/protected", nil)
			req.Header.Set("X-API-Key", "test-api-key-123")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			
			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)
			if response["user_id"] != "api-user-1" {
				t.Errorf("Expected user_id 'api-user-1', got %v", response["user_id"])
			}
		})

		t.Run("Missing API key", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/protected", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})

		t.Run("Invalid API key", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/protected", nil)
			req.Header.Set("X-API-Key", "invalid-key")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})
	})

	t.Run("Optional Authentication", func(t *testing.T) {
		handler := auth.OptionalAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, exists := getUserFromContext(r.Context())
			response := map[string]interface{}{"authenticated": exists}
			if exists {
				response["user_id"] = user.ID
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		}))

		t.Run("With valid JWT", func(t *testing.T) {
			token := auth.GenerateJWT("test123", "test@example.com", []string{"user"}, time.Hour)
			
			req := httptest.NewRequest("GET", "/optional", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			
			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)
			if response["authenticated"] != true {
				t.Error("Expected authenticated to be true")
			}
		})

		t.Run("With valid API key", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/optional", nil)
			req.Header.Set("X-API-Key", "test-api-key-123")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			
			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)
			if response["authenticated"] != true {
				t.Error("Expected authenticated to be true")
			}
		})

		t.Run("Without authentication", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/optional", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
			
			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)
			if response["authenticated"] != false {
				t.Error("Expected authenticated to be false")
			}
		})
	})

	t.Run("Role-based Authorization", func(t *testing.T) {
		// First apply JWT auth, then role auth
		handler := auth.JWTAuth(auth.RequireRoles("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "admin access granted"})
		})))

		t.Run("User with admin role", func(t *testing.T) {
			token := auth.GenerateJWT("admin123", "admin@example.com", []string{"admin", "user"}, time.Hour)
			
			req := httptest.NewRequest("GET", "/admin", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})

		t.Run("User without admin role", func(t *testing.T) {
			token := auth.GenerateJWT("user123", "user@example.com", []string{"user"}, time.Hour)
			
			req := httptest.NewRequest("GET", "/admin", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusForbidden {
				t.Errorf("Expected status 403, got %d", w.Code)
			}
		})

		t.Run("Multiple required roles - has one", func(t *testing.T) {
			multiRoleHandler := auth.JWTAuth(auth.RequireRoles("admin", "moderator")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})))

			token := auth.GenerateJWT("mod123", "mod@example.com", []string{"moderator"}, time.Hour)
			
			req := httptest.NewRequest("GET", "/admin", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			
			w := httptest.NewRecorder()
			multiRoleHandler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	})

	t.Run("JWT Token Generation and Validation", func(t *testing.T) {
		t.Run("Generate and validate token", func(t *testing.T) {
			token := auth.GenerateJWT("test123", "test@example.com", []string{"user", "admin"}, time.Hour)
			
			// Token should have 3 parts
			parts := strings.Split(token, ".")
			if len(parts) != 3 {
				t.Errorf("Expected 3 token parts, got %d", len(parts))
			}
			
			// Validate the token
			user, err := auth.validateJWT(token)
			if err != nil {
				t.Errorf("Token validation failed: %v", err)
			}
			
			if user.ID != "test123" {
				t.Errorf("Expected user ID 'test123', got '%s'", user.ID)
			}
			
			if user.Email != "test@example.com" {
				t.Errorf("Expected email 'test@example.com', got '%s'", user.Email)
			}
			
			if len(user.Roles) != 2 || user.Roles[0] != "user" || user.Roles[1] != "admin" {
				t.Errorf("Expected roles ['user', 'admin'], got %v", user.Roles)
			}
		})
	})

	t.Run("Error Response Format", func(t *testing.T) {
		handler := auth.JWTAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		var response map[string]string
		json.NewDecoder(w.Body).Decode(&response)
		
		if _, exists := response["error"]; !exists {
			t.Error("Expected error field in response")
		}
		
		if w.Header().Get("Content-Type") != "application/json" {
			t.Error("Expected JSON content type")
		}
	})
}