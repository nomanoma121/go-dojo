package main

import (
	"context"
	"hash/fnv"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// ConditionalMiddleware は条件に基づいてミドルウェアを適用するシステム
type ConditionalMiddleware struct {
	mu    sync.RWMutex
	rules []MiddlewareRule
}

// MiddlewareRule はミドルウェア適用ルールを定義
type MiddlewareRule struct {
	Name       string                              `json:"name"`
	Condition  func(*http.Request) bool            `json:"-"`
	Middleware func(http.Handler) http.Handler    `json:"-"`
	Priority   int                                 `json:"priority"`
	Enabled    bool                                `json:"enabled"`
}

// NewConditionalMiddleware は新しいConditionalMiddlewareを作成
func NewConditionalMiddleware() *ConditionalMiddleware {
	return &ConditionalMiddleware{
		rules: make([]MiddlewareRule, 0),
	}
}

// AddRule はルールを追加
func (cm *ConditionalMiddleware) AddRule(rule MiddlewareRule) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.rules = append(cm.rules, rule)
	cm.sortRulesByPriority()
}

// UpdateRule は既存ルールを更新、なければ追加
func (cm *ConditionalMiddleware) UpdateRule(name string, rule MiddlewareRule) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for i, existingRule := range cm.rules {
		if existingRule.Name == name {
			cm.rules[i] = rule
			cm.sortRulesByPriority()
			return
		}
	}
	
	// 見つからなかった場合は追加
	cm.rules = append(cm.rules, rule)
	cm.sortRulesByPriority()
}

// RemoveRule は名前でルールを削除
func (cm *ConditionalMiddleware) RemoveRule(name string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for i, rule := range cm.rules {
		if rule.Name == name {
			cm.rules = append(cm.rules[:i], cm.rules[i+1:]...)
			break
		}
	}
}

// Apply はミドルウェアチェーンを適用
func (cm *ConditionalMiddleware) Apply(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cm.mu.RLock()
		defer cm.mu.RUnlock()
		
		// 適用可能なミドルウェアを収集
		var applicableMiddlewares []func(http.Handler) http.Handler
		
		for _, rule := range cm.rules {
			if rule.Enabled && rule.Condition(r) {
				applicableMiddlewares = append(applicableMiddlewares, rule.Middleware)
			}
		}
		
		// ミドルウェアチェーンを構築（逆順で適用）
		handler := next
		for i := len(applicableMiddlewares) - 1; i >= 0; i-- {
			handler = applicableMiddlewares[i](handler)
		}
		
		handler.ServeHTTP(w, r)
	})
}

// sortRulesByPriority は優先度でルールをソート（高い順）
func (cm *ConditionalMiddleware) sortRulesByPriority() {
	sort.Slice(cm.rules, func(i, j int) bool {
		return cm.rules[i].Priority > cm.rules[j].Priority
	})
}

// 条件判定用のヘルパー関数群

// PathMatches はパスパターンマッチング
func PathMatches(pattern string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		matched, err := regexp.MatchString(pattern, r.URL.Path)
		if err != nil {
			return false
		}
		return matched
	}
}

// MethodIs はHTTPメソッドチェック
func MethodIs(methods ...string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		for _, method := range methods {
			if r.Method == method {
				return true
			}
		}
		return false
	}
}

// HasHeader はヘッダー値チェック
func HasHeader(key, value string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return r.Header.Get(key) == value
	}
}

// HasRole はユーザーロールチェック
func HasRole(role string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		user := getUserFromContext(r.Context())
		if user == nil {
			return false
		}
		
		for _, userRole := range user.Roles {
			if userRole == role {
				return true
			}
		}
		return false
	}
}

// A/Bテスト用ミドルウェア

// ABTestMiddleware はA/Bテスト用のミドルウェア
type ABTestMiddleware struct {
	testName          string
	variantRatio      float64
	variantMiddleware func(http.Handler) http.Handler
	controlMiddleware func(http.Handler) http.Handler
}

// NewABTestMiddleware はA/Bテストミドルウェアを初期化
func NewABTestMiddleware(testName string, variantRatio float64,
	variant, control func(http.Handler) http.Handler) *ABTestMiddleware {
	return &ABTestMiddleware{
		testName:          testName,
		variantRatio:      variantRatio,
		variantMiddleware: variant,
		controlMiddleware: control,
	}
}

// Apply はA/Bテストを適用
func (ab *ABTestMiddleware) Apply(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		hash := hashString(userID + ab.testName)
		
		var middleware func(http.Handler) http.Handler
		if hash < ab.variantRatio {
			middleware = ab.variantMiddleware
			w.Header().Set("X-AB-Test-Variant", "B")
		} else {
			middleware = ab.controlMiddleware
			w.Header().Set("X-AB-Test-Variant", "A")
		}
		
		w.Header().Set("X-AB-Test-Name", ab.testName)
		middleware(next).ServeHTTP(w, r)
	})
}

// 機能フラグ用ミドルウェア

// FeatureFlagClient は機能フラグクライアントのインターフェース
type FeatureFlagClient interface {
	IsEnabled(flagName, userID string) bool
}

// FeatureFlaggedMiddleware は機能フラグ付きミドルウェア
type FeatureFlaggedMiddleware struct {
	flagName   string
	flagClient FeatureFlagClient
	middleware func(http.Handler) http.Handler
	fallback   func(http.Handler) http.Handler
}

// NewFeatureFlaggedMiddleware は機能フラグミドルウェアを初期化
func NewFeatureFlaggedMiddleware(flagName string, client FeatureFlagClient,
	middleware, fallback func(http.Handler) http.Handler) *FeatureFlaggedMiddleware {
	return &FeatureFlaggedMiddleware{
		flagName:   flagName,
		flagClient: client,
		middleware: middleware,
		fallback:   fallback,
	}
}

// Apply は機能フラグを適用
func (ffm *FeatureFlaggedMiddleware) Apply(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		
		var middleware func(http.Handler) http.Handler
		if ffm.flagClient.IsEnabled(ffm.flagName, userID) {
			middleware = ffm.middleware
			w.Header().Set("X-Feature-Flag", "enabled")
		} else {
			middleware = ffm.fallback
			w.Header().Set("X-Feature-Flag", "disabled")
		}
		
		middleware(next).ServeHTTP(w, r)
	})
}

// 統合ミドルウェアルーター

// AdvancedMiddlewareRouter は高度なミドルウェアルーター
type AdvancedMiddlewareRouter struct {
	conditionalMiddleware *ConditionalMiddleware
	abTests              map[string]*ABTestMiddleware
	featureFlags         map[string]*FeatureFlaggedMiddleware
	mu                   sync.RWMutex
}

// NewAdvancedMiddlewareRouter は高度なミドルウェアルーターを初期化
func NewAdvancedMiddlewareRouter() *AdvancedMiddlewareRouter {
	amr := &AdvancedMiddlewareRouter{
		conditionalMiddleware: NewConditionalMiddleware(),
		abTests:              make(map[string]*ABTestMiddleware),
		featureFlags:         make(map[string]*FeatureFlaggedMiddleware),
	}
	
	// デフォルトルールを設定
	amr.setupDefaultRules()
	
	return amr
}

// setupDefaultRules はデフォルトのルールを設定
func (amr *AdvancedMiddlewareRouter) setupDefaultRules() {
	// API エンドポイントでは認証が必要
	amr.conditionalMiddleware.AddRule(MiddlewareRule{
		Name:       "api_auth",
		Condition:  PathMatches("^/api/"),
		Middleware: AuthenticationMiddleware,
		Priority:   100,
		Enabled:    true,
	})
	
	// 管理者エンドポイントでは管理者権限が必要
	amr.conditionalMiddleware.AddRule(MiddlewareRule{
		Name:       "admin_auth",
		Condition:  PathMatches("^/admin/"),
		Middleware: AdminOnlyMiddleware,
		Priority:   90,
		Enabled:    true,
	})
	
	// 全てのリクエストでCORSを有効化
	amr.conditionalMiddleware.AddRule(MiddlewareRule{
		Name:       "cors",
		Condition:  func(r *http.Request) bool { return true },
		Middleware: CORSMiddleware,
		Priority:   10,
		Enabled:    true,
	})
	
	// ログ出力
	amr.conditionalMiddleware.AddRule(MiddlewareRule{
		Name:       "logging",
		Condition:  func(r *http.Request) bool { return true },
		Middleware: LoggingMiddleware,
		Priority:   5,
		Enabled:    true,
	})
}

// AddABTest はA/Bテストを追加
func (amr *AdvancedMiddlewareRouter) AddABTest(name string, variantRatio float64,
	variant, control func(http.Handler) http.Handler) {
	amr.mu.Lock()
	defer amr.mu.Unlock()
	
	amr.abTests[name] = NewABTestMiddleware(name, variantRatio, variant, control)
}

// AddFeatureFlag は機能フラグを追加
func (amr *AdvancedMiddlewareRouter) AddFeatureFlag(flagName string, client FeatureFlagClient,
	middleware, fallback func(http.Handler) http.Handler) {
	amr.mu.Lock()
	defer amr.mu.Unlock()
	
	amr.featureFlags[flagName] = NewFeatureFlaggedMiddleware(flagName, client, middleware, fallback)
}

// Handler は統合ハンドラー
func (amr *AdvancedMiddlewareRouter) Handler(next http.Handler) http.Handler {
	handler := next
	
	amr.mu.RLock()
	defer amr.mu.RUnlock()
	
	// 機能フラグミドルウェアを適用
	for _, ffm := range amr.featureFlags {
		handler = ffm.Apply(handler)
	}
	
	// A/Bテストミドルウェアを適用
	for _, abm := range amr.abTests {
		handler = abm.Apply(handler)
	}
	
	// 条件付きミドルウェアを適用
	handler = amr.conditionalMiddleware.Apply(handler)
	
	return handler
}

// ユーティリティ関数

// getUserID はユーザーIDを取得
func getUserID(r *http.Request) string {
	// ヘッダーから取得を試行
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}
	
	// コンテキストから取得を試行
	if user := getUserFromContext(r.Context()); user != nil {
		return user.ID
	}
	
	// デフォルト値
	return "anonymous"
}

// hashString は文字列のハッシュ値を計算
func hashString(s string) float64 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return float64(h.Sum32()) / float64(^uint32(0))
}

// getUserFromContext はコンテキストからユーザー情報を取得
func getUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value("user").(*User); ok {
		return user
	}
	return nil
}

// User はユーザー情報を表す構造体
type User struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Roles   []string `json:"roles"`
	IsAdmin bool     `json:"is_admin"`
}

// SimpleFeatureFlagClient は簡単な機能フラグクライアント
type SimpleFeatureFlagClient struct {
	flags map[string]bool
	mu    sync.RWMutex
}

// NewSimpleFeatureFlagClient は新しいクライアントを作成
func NewSimpleFeatureFlagClient() *SimpleFeatureFlagClient {
	return &SimpleFeatureFlagClient{
		flags: make(map[string]bool),
	}
}

// SetFlag は機能フラグを設定
func (sfc *SimpleFeatureFlagClient) SetFlag(flagName string, enabled bool) {
	sfc.mu.Lock()
	defer sfc.mu.Unlock()
	
	sfc.flags[flagName] = enabled
}

// IsEnabled は機能フラグが有効かチェック
func (sfc *SimpleFeatureFlagClient) IsEnabled(flagName, userID string) bool {
	sfc.mu.RLock()
	defer sfc.mu.RUnlock()
	
	enabled, exists := sfc.flags[flagName]
	return exists && enabled
}

// サンプルミドルウェア

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("Request: %s %s\n", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 認証チェックのロジック
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		
		// 簡単な認証チェック（実際の実装では適切な認証を行う）
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Invalid authentication format", http.StatusUnauthorized)
			return
		}
		
		// ユーザー情報をコンテキストに追加
		user := &User{
			ID:      "user123",
			Name:    "Test User",
			Roles:   []string{"user"},
			IsAdmin: false,
		}
		
		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AdminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := getUserFromContext(r.Context())
		if user == nil {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		
		// 管理者権限チェック
		if !user.IsAdmin && !contains(user.Roles, "admin") {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// contains はスライスに特定の値が含まれているかチェック
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// メイン関数（テスト用）
func main() {
	router := NewAdvancedMiddlewareRouter()
	flagClient := NewSimpleFeatureFlagClient()
	
	// 機能フラグを設定
	flagClient.SetFlag("new_feature", true)
	
	// 機能フラグ付きミドルウェアを追加
	router.AddFeatureFlag("new_feature", flagClient,
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-New-Feature", "enabled")
				next.ServeHTTP(w, r)
			})
		},
		func(next http.Handler) http.Handler {
			return next // 何もしない
		},
	)
	
	// A/Bテストを追加
	router.AddABTest("ui_experiment", 0.5,
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-UI-Version", "new")
				next.ServeHTTP(w, r)
			})
		},
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-UI-Version", "old")
				next.ServeHTTP(w, r)
			})
		},
	)
	
	// 簡単なハンドラー
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from %s %s", r.Method, r.URL.Path)
	})
	
	// ミドルウェアを適用
	finalHandler := router.Handler(handler)
	
	fmt.Println("Starting server on :8080")
	fmt.Println("Try:")
	fmt.Println("  curl http://localhost:8080/")
	fmt.Println("  curl http://localhost:8080/api/users")
	fmt.Println("  curl -H 'Authorization: Bearer token' http://localhost:8080/api/users")
	fmt.Println("  curl -H 'Authorization: Bearer token' http://localhost:8080/admin/settings")
	
	http.ListenAndServe(":8080", finalHandler)
}