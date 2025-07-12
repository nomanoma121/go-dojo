package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// contextKey is used for context keys to avoid collisions
type contextKey string

const UserContextKey contextKey = "user"

// User represents an authenticated user
type User struct {
	ID    string   `json:"id"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// JWTClaims represents JWT payload claims
type JWTClaims struct {
	Sub   string   `json:"sub"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
	Exp   int64    `json:"exp"`
	Iat   int64    `json:"iat"`
}

// AuthMiddleware provides authentication functionality
type AuthMiddleware struct {
	jwtSecret string
	apiKeys   map[string]User
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtSecret string) *AuthMiddleware {
	// Initialize with some test API keys
	apiKeys := map[string]User{
		"test-api-key-123": {
			ID:    "api-user-1",
			Email: "api@example.com",
			Roles: []string{"user"},
		},
		"admin-api-key-456": {
			ID:    "api-admin-1",
			Email: "admin@example.com",
			Roles: []string{"admin", "user"},
		},
	}

	return &AuthMiddleware{
		jwtSecret: jwtSecret,
		apiKeys:   apiKeys,
	}
}

// JWT authentication middleware
func (am *AuthMiddleware) JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			am.sendErrorResponse(w, http.StatusUnauthorized, "Missing authorization header")
			return
		}

		// Check for Bearer prefix
		if !strings.HasPrefix(authHeader, "Bearer ") {
			am.sendErrorResponse(w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			am.sendErrorResponse(w, http.StatusUnauthorized, "Missing token")
			return
		}

		// Validate JWT token
		user, err := am.validateJWT(token)
		if err != nil {
			am.sendErrorResponse(w, http.StatusUnauthorized, "Invalid token: "+err.Error())
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// API Key authentication middleware
func (am *AuthMiddleware) APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get API key from header
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			am.sendErrorResponse(w, http.StatusUnauthorized, "Missing API key")
			return
		}

		// Check if API key is valid
		user, exists := am.apiKeys[apiKey]
		if !exists {
			am.sendErrorResponse(w, http.StatusUnauthorized, "Invalid API key")
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), UserContextKey, &user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Optional authentication (doesn't fail if no auth provided)
func (am *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try JWT first
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if user, err := am.validateJWT(token); err == nil {
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Try API key
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			if user, exists := am.apiKeys[apiKey]; exists {
				ctx := context.WithValue(r.Context(), UserContextKey, &user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// No auth provided or invalid - continue without user
		next.ServeHTTP(w, r)
	})
}

// Role-based authorization middleware
func (am *AuthMiddleware) RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			user, exists := getUserFromContext(r.Context())
			if !exists {
				am.sendErrorResponse(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Check if user has any of the required roles
			hasRole := false
			for _, requiredRole := range roles {
				for _, userRole := range user.Roles {
					if userRole == requiredRole {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}

			if !hasRole {
				am.sendErrorResponse(w, http.StatusForbidden, "Insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions
func getUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserContextKey).(*User)
	return user, ok
}

func (am *AuthMiddleware) validateJWT(token string) (*User, error) {
	// For this demo, we'll use a simple JWT implementation
	// In production, use a proper JWT library like golang-jwt
	
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	// Decode header
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid header encoding")
	}

	var header map[string]interface{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, errors.New("invalid header JSON")
	}

	// Check algorithm
	if header["alg"] != "HS256" {
		return nil, errors.New("unsupported algorithm")
	}

	// Verify signature
	expectedSig := am.generateSignature(parts[0] + "." + parts[1])
	actualSig := parts[2]
	
	if !hmac.Equal([]byte(expectedSig), []byte(actualSig)) {
		return nil, errors.New("invalid signature")
	}

	// Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid payload encoding")
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("invalid payload JSON")
	}

	// Check expiration
	if claims.Exp < time.Now().Unix() {
		return nil, errors.New("token expired")
	}

	return &User{
		ID:    claims.Sub,
		Email: claims.Email,
		Roles: claims.Roles,
	}, nil
}

func (am *AuthMiddleware) generateSignature(data string) string {
	h := hmac.New(sha256.New, []byte(am.jwtSecret))
	h.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// GenerateJWT creates a JWT token for testing
func (am *AuthMiddleware) GenerateJWT(userID, email string, roles []string, expiration time.Duration) string {
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}

	claims := JWTClaims{
		Sub:   userID,
		Email: email,
		Roles: roles,
		Exp:   time.Now().Add(expiration).Unix(),
		Iat:   time.Now().Unix(),
	}

	headerBytes, _ := json.Marshal(header)
	claimsBytes, _ := json.Marshal(claims)

	headerEncoded := base64.RawURLEncoding.EncodeToString(headerBytes)
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsBytes)

	signature := am.generateSignature(headerEncoded + "." + claimsEncoded)

	return headerEncoded + "." + claimsEncoded + "." + signature
}

func (am *AuthMiddleware) sendErrorResponse(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func main() {
	auth := NewAuthMiddleware("secret-key")

	mux := http.NewServeMux()

	// Public endpoint
	mux.HandleFunc("/public", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "public endpoint"})
	})

	// Protected endpoint with JWT
	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := getUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "protected endpoint",
			"user":    user,
		})
	})
	mux.Handle("/protected", auth.JWTAuth(protected))

	// API key protected endpoint
	apiProtected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := getUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "api protected endpoint",
			"user":    user,
		})
	})
	mux.Handle("/api/protected", auth.APIKeyAuth(apiProtected))

	// Admin endpoint with role requirement
	admin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "admin endpoint"})
	})
	mux.Handle("/admin", auth.JWTAuth(auth.RequireRoles("admin")(admin)))

	// Optional auth endpoint
	optional := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, exists := getUserFromContext(r.Context())
		response := map[string]interface{}{
			"message": "optional auth endpoint",
		}
		if exists {
			response["user"] = user
		} else {
			response["user"] = "anonymous"
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})
	mux.Handle("/optional", auth.OptionalAuth(optional))

	// Demo token generation endpoint
	mux.HandleFunc("/generate-token", func(w http.ResponseWriter, r *http.Request) {
		userToken := auth.GenerateJWT("user123", "user@example.com", []string{"user"}, time.Hour)
		adminToken := auth.GenerateJWT("admin456", "admin@example.com", []string{"admin", "user"}, time.Hour)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"user_token":  userToken,
			"admin_token": adminToken,
		})
	})

	fmt.Println("Server starting on :8080")
	fmt.Println("Endpoints:")
	fmt.Println("- GET /public (no auth)")
	fmt.Println("- GET /protected (JWT required)")
	fmt.Println("- GET /api/protected (API key required)")
	fmt.Println("- GET /admin (JWT + admin role required)")
	fmt.Println("- GET /optional (optional auth)")
	fmt.Println("- GET /generate-token (get demo tokens)")
	http.ListenAndServe(":8080", mux)
}