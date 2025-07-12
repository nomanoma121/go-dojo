//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// User represents an authenticated user
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Roles []string `json:"roles"`
}

// AuthMiddleware provides authentication functionality
type AuthMiddleware struct {
	jwtSecret string
	apiKeys   map[string]User
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtSecret string) *AuthMiddleware {
	// TODO: 実装してください
	return nil
}

// JWT authentication middleware
func (am *AuthMiddleware) JWTAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: 実装してください
		//
		// 実装の流れ:
		// 1. Authorization ヘッダーを取得
		// 2. "Bearer "プレフィックスを確認
		// 3. JWTトークンを検証
		// 4. ユーザー情報をcontextに格納
		// 5. 次のハンドラーを呼び出し
		
		next.ServeHTTP(w, r)
	})
}

// API Key authentication middleware
func (am *AuthMiddleware) APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: 実装してください
		//
		// 実装の流れ:
		// 1. X-API-Key ヘッダーを取得
		// 2. APIキーが登録されているか確認
		// 3. ユーザー情報をcontextに格納
		// 4. 次のハンドラーを呼び出し
		
		next.ServeHTTP(w, r)
	})
}

// Optional authentication (doesn't fail if no auth provided)
func (am *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	// TODO: 実装してください
	return next
}

// Role-based authorization middleware
func (am *AuthMiddleware) RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: 実装してください
			//
			// 実装の流れ:
			// 1. contextからユーザー情報を取得
			// 2. ユーザーが必要な役割を持っているか確認
			// 3. 権限がない場合は403を返す
			
			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions
func getUserFromContext(ctx context.Context) (*User, bool) {
	// TODO: 実装してください
	return nil, false
}

func (am *AuthMiddleware) validateJWT(token string) (*User, error) {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. JWTトークンの形式をチェック
	// 2. 署名を検証
	// 3. ペイロードからユーザー情報を抽出
	
	return nil, nil
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
			"user": user,
		})
	})
	mux.Handle("/protected", auth.JWTAuth(protected))
	
	// Admin endpoint with role requirement
	admin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "admin endpoint"})
	})
	mux.Handle("/admin", auth.JWTAuth(auth.RequireRoles("admin")(admin)))
	
	println("Server starting on :8080")
	http.ListenAndServe(":8080", mux)
}