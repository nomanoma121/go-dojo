# Day 18: Conditional Middleware

## 🎯 本日の目標 (Today's Goal)

条件に基づいてミドルウェアを動的に適用・除外するシステムを実装し、柔軟でスケーラブルなHTTPミドルウェアアーキテクチャを習得する。環境、パス、ヘッダー、ユーザー権限などの条件に応じた適応的な処理を実現する。

## 📖 解説 (Explanation)

### Conditional Middlewareとは

Conditional Middleware（条件付きミドルウェア）は、特定の条件が満たされた場合にのみ実行されるミドルウェアパターンです。これにより、リクエストの特性に応じて異なる処理チェーンを動的に構築できます。

### 実用的なユースケース

#### 1. 環境固有の処理

```go
// 開発環境でのみデバッグ情報を出力
func DevOnlyMiddleware(handler http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if os.Getenv("ENV") == "development" {
            log.Printf("DEBUG: %s %s", r.Method, r.URL.Path)
        }
        handler.ServeHTTP(w, r)
    })
}
```

#### 2. パス固有の認証

```go
// 特定のパスでのみ認証を要求
func ConditionalAuthMiddleware(authPaths []string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            for _, path := range authPaths {
                if strings.HasPrefix(r.URL.Path, path) {
                    // 認証チェック
                    if !isAuthenticated(r) {
                        http.Error(w, "Unauthorized", http.StatusUnauthorized)
                        return
                    }
                    break
                }
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

#### 3. ユーザー権限ベース

```go
// 管理者権限が必要なエンドポイント
func AdminOnlyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := getUserFromContext(r.Context())
        if user == nil || !user.IsAdmin {
            http.Error(w, "Admin access required", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### 高度なConditional Middlewareパターン

#### 1. ルールベースミドルウェア

```go
type MiddlewareRule struct {
    Condition func(*http.Request) bool
    Middleware func(http.Handler) http.Handler
}

type ConditionalMiddleware struct {
    rules []MiddlewareRule
}

func NewConditionalMiddleware() *ConditionalMiddleware {
    return &ConditionalMiddleware{
        rules: make([]MiddlewareRule, 0),
    }
}

func (cm *ConditionalMiddleware) AddRule(condition func(*http.Request) bool, middleware func(http.Handler) http.Handler) {
    cm.rules = append(cm.rules, MiddlewareRule{
        Condition: condition,
        Middleware: middleware,
    })
}

func (cm *ConditionalMiddleware) Apply(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 適用可能なミドルウェアを収集
        var applicableMiddlewares []func(http.Handler) http.Handler
        
        for _, rule := range cm.rules {
            if rule.Condition(r) {
                applicableMiddlewares = append(applicableMiddlewares, rule.Middleware)
            }
        }
        
        // ミドルウェアチェーンを構築
        handler := next
        for i := len(applicableMiddlewares) - 1; i >= 0; i-- {
            handler = applicableMiddlewares[i](handler)
        }
        
        handler.ServeHTTP(w, r)
    })
}
```

#### 2. 複雑な条件式

```go
type RequestCondition interface {
    Matches(*http.Request) bool
}

type PathCondition struct {
    Pattern string
}

func (pc *PathCondition) Matches(r *http.Request) bool {
    matched, _ := regexp.MatchString(pc.Pattern, r.URL.Path)
    return matched
}

type MethodCondition struct {
    Methods []string
}

func (mc *MethodCondition) Matches(r *http.Request) bool {
    for _, method := range mc.Methods {
        if r.Method == method {
            return true
        }
    }
    return false
}

type HeaderCondition struct {
    Header string
    Value  string
}

func (hc *HeaderCondition) Matches(r *http.Request) bool {
    return r.Header.Get(hc.Header) == hc.Value
}

type AndCondition struct {
    Conditions []RequestCondition
}

func (ac *AndCondition) Matches(r *http.Request) bool {
    for _, condition := range ac.Conditions {
        if !condition.Matches(r) {
            return false
        }
    }
    return true
}

type OrCondition struct {
    Conditions []RequestCondition
}

func (oc *OrCondition) Matches(r *http.Request) bool {
    for _, condition := range oc.Conditions {
        if condition.Matches(r) {
            return true
        }
    }
    return false
}
```

#### 3. 動的ミドルウェア設定

```go
type DynamicMiddlewareConfig struct {
    mu          sync.RWMutex
    configs     map[string]MiddlewareConfig
    subscribers []chan<- MiddlewareUpdate
}

type MiddlewareConfig struct {
    Enabled    bool                          `json:"enabled"`
    Conditions map[string]interface{}        `json:"conditions"`
    Settings   map[string]interface{}        `json:"settings"`
}

type MiddlewareUpdate struct {
    Type   string
    Config MiddlewareConfig
}

func (dmc *DynamicMiddlewareConfig) UpdateConfig(middlewareType string, config MiddlewareConfig) {
    dmc.mu.Lock()
    defer dmc.mu.Unlock()
    
    dmc.configs[middlewareType] = config
    
    // 設定変更を通知
    update := MiddlewareUpdate{
        Type:   middlewareType,
        Config: config,
    }
    
    for _, subscriber := range dmc.subscribers {
        select {
        case subscriber <- update:
        default:
            // ノンブロッキング送信
        }
    }
}

func (dmc *DynamicMiddlewareConfig) GetConfig(middlewareType string) (MiddlewareConfig, bool) {
    dmc.mu.RLock()
    defer dmc.mu.RUnlock()
    
    config, exists := dmc.configs[middlewareType]
    return config, exists
}

func (dmc *DynamicMiddlewareConfig) Subscribe() <-chan MiddlewareUpdate {
    dmc.mu.Lock()
    defer dmc.mu.Unlock()
    
    ch := make(chan MiddlewareUpdate, 10)
    dmc.subscribers = append(dmc.subscribers, ch)
    return ch
}
```

#### 4. A/Bテスト用ミドルウェア

```go
type ABTestMiddleware struct {
    testName     string
    variantRatio float64 // 0.0-1.0
    variantMiddleware func(http.Handler) http.Handler
    controlMiddleware func(http.Handler) http.Handler
}

func NewABTestMiddleware(testName string, variantRatio float64, 
    variant, control func(http.Handler) http.Handler) *ABTestMiddleware {
    return &ABTestMiddleware{
        testName:          testName,
        variantRatio:      variantRatio,
        variantMiddleware: variant,
        controlMiddleware: control,
    }
}

func (ab *ABTestMiddleware) Apply(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ユーザーIDベースでバリアント決定
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

func hashString(s string) float64 {
    h := fnv.New32a()
    h.Write([]byte(s))
    return float64(h.Sum32()) / float64(^uint32(0))
}
```

#### 5. 機能フラグとの統合

```go
type FeatureFlaggedMiddleware struct {
    flagName   string
    flagClient FeatureFlagClient
    middleware func(http.Handler) http.Handler
    fallback   func(http.Handler) http.Handler
}

type FeatureFlagClient interface {
    IsEnabled(flagName, userID string) bool
}

func NewFeatureFlaggedMiddleware(flagName string, client FeatureFlagClient,
    middleware, fallback func(http.Handler) http.Handler) *FeatureFlaggedMiddleware {
    return &FeatureFlaggedMiddleware{
        flagName:   flagName,
        flagClient: client,
        middleware: middleware,
        fallback:   fallback,
    }
}

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
```

### 統合例：高度なミドルウェアルーター

```go
type AdvancedMiddlewareRouter struct {
    conditionalMiddleware *ConditionalMiddleware
    dynamicConfig        *DynamicMiddlewareConfig
    abTests              map[string]*ABTestMiddleware
    featureFlags         map[string]*FeatureFlaggedMiddleware
}

func NewAdvancedMiddlewareRouter() *AdvancedMiddlewareRouter {
    router := &AdvancedMiddlewareRouter{
        conditionalMiddleware: NewConditionalMiddleware(),
        dynamicConfig:        NewDynamicMiddlewareConfig(),
        abTests:              make(map[string]*ABTestMiddleware),
        featureFlags:         make(map[string]*FeatureFlaggedMiddleware),
    }
    
    // 基本的なルールを設定
    router.setupDefaultRules()
    
    return router
}

func (amr *AdvancedMiddlewareRouter) setupDefaultRules() {
    // API エンドポイントでは認証が必要
    amr.conditionalMiddleware.AddRule(
        func(r *http.Request) bool {
            return strings.HasPrefix(r.URL.Path, "/api/")
        },
        AuthenticationMiddleware,
    )
    
    // 管理者エンドポイントでは管理者権限が必要
    amr.conditionalMiddleware.AddRule(
        func(r *http.Request) bool {
            return strings.HasPrefix(r.URL.Path, "/admin/")
        },
        AdminOnlyMiddleware,
    )
    
    // 開発環境でのみデバッグミドルウェア
    amr.conditionalMiddleware.AddRule(
        func(r *http.Request) bool {
            return os.Getenv("ENV") == "development"
        },
        DebugMiddleware,
    )
    
    // HTTPS必須エンドポイント
    amr.conditionalMiddleware.AddRule(
        func(r *http.Request) bool {
            sensitivePathPattern := `^/(payment|auth|admin)/`
            matched, _ := regexp.MatchString(sensitivePathPattern, r.URL.Path)
            return matched
        },
        HTTPSRequiredMiddleware,
    )
}

func (amr *AdvancedMiddlewareRouter) AddABTest(name string, variantRatio float64,
    variant, control func(http.Handler) http.Handler) {
    amr.abTests[name] = NewABTestMiddleware(name, variantRatio, variant, control)
}

func (amr *AdvancedMiddlewareRouter) AddFeatureFlag(flagName string, client FeatureFlagClient,
    middleware, fallback func(http.Handler) http.Handler) {
    amr.featureFlags[flagName] = NewFeatureFlaggedMiddleware(flagName, client, middleware, fallback)
}

func (amr *AdvancedMiddlewareRouter) Handler(next http.Handler) http.Handler {
    // ミドルウェアチェーンを構築
    handler := next
    
    // 機能フラグ付きミドルウェア
    for _, ffm := range amr.featureFlags {
        handler = ffm.Apply(handler)
    }
    
    // A/Bテストミドルウェア
    for _, abm := range amr.abTests {
        handler = abm.Apply(handler)
    }
    
    // 条件付きミドルウェア
    handler = amr.conditionalMiddleware.Apply(handler)
    
    return handler
}
```

## 📝 課題 (The Problem)

以下の機能を持つ高度なConditional Middlewareシステムを実装してください：

### 1. ConditionalMiddleware の実装

```go
type ConditionalMiddleware struct {
    rules []MiddlewareRule
}

type MiddlewareRule struct {
    Name       string
    Condition  func(*http.Request) bool
    Middleware func(http.Handler) http.Handler
    Priority   int
}
```

### 2. 必要な機能

- **ルールベース条件判定**: パス、メソッド、ヘッダー、ユーザー属性での条件分岐
- **優先度制御**: ミドルウェアの実行順序制御
- **動的設定更新**: 実行時での条件・ミドルウェア設定変更
- **A/Bテスト対応**: ユーザーセグメント別処理分岐
- **機能フラグ統合**: フィーチャーフラグによる機能ON/OFF

### 3. 条件式の実装

- Path Pattern Matching
- HTTP Method Filtering  
- Header Value Checking
- User Role/Permission Based
- Time-based Conditions
- Geographic Conditions

### 4. パフォーマンス最適化

- 条件評価の効率化
- ミドルウェアチェーン構築コスト削減
- メモリ使用量最適化

### 5. 監視機能

- ミドルウェア実行メトリクス
- 条件マッチング統計
- パフォーマンス監視

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestConditionalMiddleware_BasicRules
    main_test.go:45: Path-based rule applied correctly
--- PASS: TestConditionalMiddleware_BasicRules (0.01s)

=== RUN   TestConditionalMiddleware_Priority
    main_test.go:65: Middleware executed in correct priority order
--- PASS: TestConditionalMiddleware_Priority (0.01s)

=== RUN   TestConditionalMiddleware_DynamicUpdate
    main_test.go:85: Dynamic rule update working correctly
--- PASS: TestConditionalMiddleware_DynamicUpdate (0.02s)

=== RUN   TestABTestMiddleware
    main_test.go:105: A/B test variant distribution correct
--- PASS: TestABTestMiddleware (0.03s)

=== RUN   TestFeatureFlagMiddleware
    main_test.go:125: Feature flag toggle working correctly
--- PASS: TestFeatureFlagMiddleware (0.01s)

PASS
ok      day18-conditional-middleware   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 基本的な条件判定

```go
func PathMatches(pattern string) func(*http.Request) bool {
    return func(r *http.Request) bool {
        matched, _ := regexp.MatchString(pattern, r.URL.Path)
        return matched
    }
}

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

func HasHeader(key, value string) func(*http.Request) bool {
    return func(r *http.Request) bool {
        return r.Header.Get(key) == value
    }
}
```

### ミドルウェア実行順制御

```go
func (cm *ConditionalMiddleware) sortRulesByPriority() {
    sort.Slice(cm.rules, func(i, j int) bool {
        return cm.rules[i].Priority > cm.rules[j].Priority
    })
}
```

### 動的設定更新

```go
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
    
    cm.rules = append(cm.rules, rule)
    cm.sortRulesByPriority()
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **プラグインシステム**: 外部からミドルウェアを動的ロード
2. **機械学習統合**: ユーザー行動ベースの動的ルーティング
3. **分散設定管理**: Consul/etcdでの設定同期
4. **リアルタイム解析**: ミドルウェア実行の可視化
5. **セキュリティスキャン**: 条件式の安全性検証

Conditional Middlewareの実装を通じて、柔軟で拡張可能なWebアプリケーションアーキテクチャの構築手法を習得しましょう！