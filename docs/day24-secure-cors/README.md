# Day 24: セキュアなCORS設定

🎯 **本日の目標**
Cross-Origin Resource Sharing (CORS) の安全な設定を実装し、ブラウザベースのWebアプリケーションでのAPIアクセス制御を学ぶ。

## 📖 解説

### CORS の基礎知識

```go
// 【CORSの重要性】クロスオリジン攻撃防御とセキュアなリソース共有
// ❌ 問題例：CORS設定なしでの壊滅的セキュリティホール
func catastrophicNoCORSProtection() {
    // 🚨 災害例：CORS制限なしで悪意あるサイトからの攻撃が可能
    
    http.HandleFunc("/api/user-data", func(w http.ResponseWriter, r *http.Request) {
        // ❌ Origin検証なし→どんなサイトからでもアクセス可能
        userID := r.Header.Get("X-User-ID")
        
        // ❌ 機密情報を無制限公開
        sensitiveData := getUserSensitiveData(userID)
        
        // ❌ CORSヘッダーなし→ブラウザはリクエストをブロック
        // しかし、攻撃者は直接HTTPクライアントでアクセス可能
        json.NewEncoder(w).Encode(sensitiveData)
        
        // 【攻撃シナリオ】
        // 1. 悪意のあるサイト evil.com が被害者ページに埋め込まれる
        // 2. ユーザーが正規サイトにログイン済み（Cookieあり）
        // 3. evil.com のJavaScriptが被害者の認証情報で機密APIにアクセス
        // 4. 個人情報、金融データ、企業機密が漏洩
    })
    
    http.HandleFunc("/api/transfer-money", func(w http.ResponseWriter, r *http.Request) {
        var transfer struct {
            To     string  `json:"to"`
            Amount float64 `json:"amount"`
        }
        
        json.NewDecoder(r.Body).Decode(&transfer)
        
        // ❌ CORS制限なし→CSRF攻撃が成功
        userID := getUserIDFromSession(r)
        err := transferMoney(userID, transfer.To, transfer.Amount)
        if err != nil {
            http.Error(w, "Transfer failed", http.StatusInternalServerError)
            return
        }
        
        // 【CSRF攻撃成功例】
        // 1. 攻撃者が偽サイトに被害者を誘導
        // 2. 隠しフォームで被害者の銀行口座から送金実行
        // 3. 被害者の認証Cookie使用で送金成功
        // 4. 全財産が攻撃者口座に移動
        
        json.NewEncoder(w).Encode(map[string]string{"status": "success"})
    })
    
    http.HandleFunc("/api/admin/delete-user", func(w http.ResponseWriter, r *http.Request) {
        userID := r.URL.Query().Get("user_id")
        
        // ❌ 管理者機能への無制限アクセス
        err := deleteUser(userID)
        if err != nil {
            http.Error(w, "Deletion failed", http.StatusInternalServerError)
            return
        }
        
        // 【管理者権限悪用】
        // 1. 管理者が悪意サイトを閲覧
        // 2. 隠しスクリプトが全ユーザー削除APIを実行
        // 3. 数秒で全顧客データが消失
        // 4. 事業継続不可能
        
        json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
    })
    
    // 【災害結果】
    // 1. 顧客の個人情報・金融情報大量漏洩
    // 2. 不正送金による金銭被害
    // 3. データ削除による事業停止
    // 4. 法的責任・賠償問題
    // 5. 企業信用失墜・株価暴落
    
    log.Println("❌ Starting server WITHOUT CORS protection...")
    http.ListenAndServe(":8080", nil)
    // 結果：数時間でセキュリティ侵害、企業存続危機
}

// ✅ 正解：エンタープライズ級CORS保護システム
type EnterpriseSecureCORSSystem struct {
    // 【基本CORS設定】
    allowedOrigins   []string                    // 許可オリジンリスト
    allowedMethods   []string                    // 許可HTTPメソッド
    allowedHeaders   []string                    // 許可ヘッダー
    exposedHeaders   []string                    // 公開ヘッダー
    
    // 【高度な制御】
    originValidator  *OriginValidator            // オリジン検証エンジン
    methodWhitelist  *MethodWhitelist            // メソッド制限
    headerSanitizer  *HeaderSanitizer            // ヘッダーサニタイズ
    
    // 【セキュリティ強化】
    csrfProtector    *CSRFProtector              // CSRF攻撃防御
    rateLimiter      *CORSRateLimiter            // CORS制限
    threatDetector   *ThreatDetector             // 脅威検知
    
    // 【認証統合】
    authValidator    *AuthValidator              // 認証検証
    sessionManager   *SessionManager             // セッション管理
    tokenValidator   *TokenValidator             // トークン検証
    
    // 【監視・ログ】
    accessLogger     *AccessLogger               // アクセスログ
    auditLogger      *AuditLogger                // 監査ログ
    securityMonitor  *SecurityMonitor           // セキュリティ監視
    
    // 【動的制御】
    dynamicRules     *DynamicRuleEngine          // 動的ルール
    geoRestriction   *GeoRestriction             // 地理的制限
    timeRestriction  *TimeRestriction            // 時間制限
    
    // 【パフォーマンス】
    cacheManager     *CORSCacheManager           // キャッシュ管理
    prefetchManager  *PrefetchManager            // プリフェッチ
    
    // 【障害回復】
    failoverHandler  *FailoverHandler            // フェイルオーバー
    backupRules      *BackupRules                // バックアップルール
    
    config           *SecureCORSConfig           // 設定管理
    mu               sync.RWMutex                // 安全な設定変更
}
```

CORS（Cross-Origin Resource Sharing）は、Webブラウザが実装するセキュリティ機能で、異なるオリジン（ドメイン、プロトコル、ポート）間でのリソース共有を制御します。

#### Same-Origin Policy

ブラウザはデフォルトで「同一オリジンポリシー」を適用し、異なるオリジンからのリクエストをブロックします：

```javascript
// https://example.com から実行される JavaScript
fetch('https://api.other-domain.com/data') // ブロックされる
```

#### CORS ヘッダーによる許可

```go
// 【エンタープライズCORS実装の核心】包括的セキュリティ検証とヘッダー設定
func (cors *EnterpriseSecureCORSSystem) ComprehensiveCORSHandler(w http.ResponseWriter, r *http.Request) {
    requestID := getRequestID(r.Context())
    clientIP := getClientIP(r)
    origin := r.Header.Get("Origin")
    
    // 【STEP 1】Origin検証（最重要セキュリティチェック）
    if !cors.validateOriginSecurely(origin, r) {
        cors.securityMonitor.LogSuspiciousOrigin(origin, clientIP, requestID)
        cors.auditLogger.LogSecurityViolation("invalid_origin", origin, clientIP)
        
        // 攻撃者に情報を与えない
        http.Error(w, "Access Denied", http.StatusForbidden)
        return
    }
    
    // 【STEP 2】メソッド検証
    if !cors.isMethodAllowed(r.Method, origin) {
        cors.auditLogger.LogSecurityViolation("invalid_method", r.Method, clientIP)
        w.Header().Set("Allow", strings.Join(cors.getAllowedMethods(origin), ", "))
        http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // 【STEP 3】ヘッダー検証とサニタイズ
    requestedHeaders := r.Header.Get("Access-Control-Request-Headers")
    if requestedHeaders != "" {
        if !cors.validateRequestedHeaders(requestedHeaders, origin) {
            cors.auditLogger.LogSecurityViolation("invalid_headers", requestedHeaders, clientIP)
            http.Error(w, "Invalid Headers", http.StatusBadRequest)
            return
        }
    }
    
    // 【STEP 4】レート制限チェック
    if !cors.rateLimiter.AllowRequest(clientIP, origin) {
        cors.securityMonitor.LogRateLimitExceeded(clientIP, origin)
        http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
        return
    }
    
    // 【STEP 5】セキュアなCORSヘッダー設定
    cors.setSecureCORSHeaders(w, origin, r)
    
    // 【STEP 6】アクセスログ記録
    cors.accessLogger.LogCORSAccess(origin, r.Method, r.URL.Path, clientIP, requestID)
}

// 【重要メソッド】セキュアなOrigin検証
func (cors *EnterpriseSecureCORSSystem) validateOriginSecurely(origin string, r *http.Request) bool {
    if origin == "" {
        // Same-Origin リクエストは許可
        return true
    }
    
    // 【基本検証】許可リスト確認
    if !cors.originValidator.IsOriginAllowed(origin) {
        return false
    }
    
    // 【高度検証】地理的制限
    if cors.geoRestriction != nil {
        clientIP := getClientIP(r)
        if !cors.geoRestriction.IsLocationAllowed(clientIP, origin) {
            return false
        }
    }
    
    // 【時間制限】営業時間制限
    if cors.timeRestriction != nil {
        if !cors.timeRestriction.IsTimeAllowed(origin) {
            return false
        }
    }
    
    // 【脅威検知】不審なアクセスパターン検出
    if cors.threatDetector.IsSuspiciousOrigin(origin, r) {
        return false
    }
    
    return true
}

// 【核心メソッド】セキュアなCORSヘッダー設定
func (cors *EnterpriseSecureCORSSystem) setSecureCORSHeaders(w http.ResponseWriter, origin string, r *http.Request) {
    // 【重要】Origin明示的指定（ワイルドカード禁止）
    if origin != "" {
        w.Header().Set("Access-Control-Allow-Origin", origin)
        // キャッシュポイズニング防止
        w.Header().Set("Vary", "Origin")
    }
    
    // 【メソッド制限】オリジン別許可メソッド
    allowedMethods := cors.getAllowedMethods(origin)
    w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
    
    // 【ヘッダー制限】サニタイズ済みヘッダー
    allowedHeaders := cors.getSanitizedHeaders(origin)
    w.Header().Set("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
    
    // 【公開ヘッダー】必要最小限
    if len(cors.exposedHeaders) > 0 {
        w.Header().Set("Access-Control-Expose-Headers", strings.Join(cors.exposedHeaders, ", "))
    }
    
    // 【認証情報制御】細かな制御
    if cors.shouldAllowCredentials(origin, r) {
        w.Header().Set("Access-Control-Allow-Credentials", "true")
    }
    
    // 【プリフライトキャッシュ】適切な期間設定
    maxAge := cors.getOptimalMaxAge(origin)
    w.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
    
    // 【セキュリティヘッダー追加】
    w.Header().Set("X-Content-Type-Options", "nosniff")
    w.Header().Set("X-Frame-Options", "DENY")
    w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

// 【実用例】プロダクション環境でのセキュアCORS運用
func ProductionSecureCORSUsage() {
    // 【設定】エンタープライズCORS設定
    corsConfig := &SecureCORSConfig{
        // 本番環境：厳格なオリジン制限
        AllowedOrigins: []string{
            "https://app.company.com",
            "https://admin.company.com", 
            "https://*.trusted-partner.com", // サブドメイン許可
        },
        
        // セキュアなメソッド制限
        AllowedMethods: []string{
            http.MethodGet,
            http.MethodPost,
            http.MethodPut,
            http.MethodDelete,
            http.MethodOptions, // プリフライト用
        },
        
        // 最小限ヘッダー許可
        AllowedHeaders: []string{
            "Content-Type",
            "Authorization",
            "X-Requested-With",
            "X-API-Key",
            "X-Client-Version",
        },
        
        // レスポンスヘッダー公開制限
        ExposedHeaders: []string{
            "X-Rate-Limit-Remaining",
            "X-Request-ID",
        },
        
        // 認証情報許可（厳格制御）
        AllowCredentials: true,
        
        // プリフライトキャッシュ最適化
        MaxAge: 3600, // 1時間
        
        // セキュリティ機能有効化
        EnableGeoRestriction:  true,
        EnableTimeRestriction: false, // 24時間サービス
        EnableThreatDetection: true,
        EnableRateLimit:      true,
        
        // 監視設定
        EnableAccessLog: true,
        EnableAuditLog:  true,
        EnableMetrics:   true,
    }
    
    corsSystem := NewEnterpriseSecureCORSSystem(corsConfig)
    
    // 【ルーター設定】
    mux := http.NewServeMux()
    
    // 【公開API】基本CORS適用
    mux.HandleFunc("/api/public/status", func(w http.ResponseWriter, r *http.Request) {
        corsSystem.ComprehensiveCORSHandler(w, r)
        
        if r.Method == http.MethodOptions {
            return // プリフライト完了
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status": "ok",
            "timestamp": time.Now().Format(time.RFC3339),
        })
    })
    
    // 【認証必須API】厳格CORS適用
    mux.HandleFunc("/api/user/profile", func(w http.ResponseWriter, r *http.Request) {
        corsSystem.ComprehensiveCORSHandler(w, r)
        
        if r.Method == http.MethodOptions {
            return
        }
        
        // 認証チェック
        if !corsSystem.authValidator.ValidateToken(r) {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        userProfile := getUserProfile(r)
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(userProfile)
    })
    
    // 【管理者API】最高レベルCORS保護
    mux.HandleFunc("/api/admin/users", func(w http.ResponseWriter, r *http.Request) {
        // 管理者専用CORS設定適用
        corsSystem.ApplyAdminCORS(w, r)
        
        if r.Method == http.MethodOptions {
            return
        }
        
        // 管理者権限チェック
        if !corsSystem.authValidator.ValidateAdminToken(r) {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }
        
        users := getAllUsers()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(users)
    })
    
    log.Printf("🔒 Enterprise CORS protection server starting on :8080")
    log.Printf("   Origin validation: ENABLED")
    log.Printf("   Geo restriction: %t", corsConfig.EnableGeoRestriction)
    log.Printf("   Threat detection: %t", corsConfig.EnableThreatDetection)
    log.Printf("   Rate limiting: %t", corsConfig.EnableRateLimit)
    
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

サーバーは適切なCORSヘッダーを送信することで、特定のクロスオリジンリクエストを許可できます：

### CORS のリクエストタイプ

#### Simple Requests

以下の条件を満たすリクエストは「Simple Request」として直接送信されます：

- メソッド: GET, HEAD, POST
- ヘッダー: 標準的なヘッダーのみ
- Content-Type: application/x-www-form-urlencoded, multipart/form-data, text/plain

```go
// Simple Request の例
fetch('https://api.example.com/data', {
    method: 'GET',
    headers: {
        'Content-Type': 'text/plain'
    }
})
```

#### Preflight Requests

Simple Request の条件を満たさない場合、ブラウザは事前にOPTIONSリクエストを送信します：

```http
OPTIONS /api/data HTTP/1.1
Host: api.example.com
Origin: https://webapp.example.com
Access-Control-Request-Method: PUT
Access-Control-Request-Headers: Content-Type, X-Custom-Header
```

サーバーの応答例：

```http
HTTP/1.1 200 OK
Access-Control-Allow-Origin: https://webapp.example.com
Access-Control-Allow-Methods: GET, POST, PUT, DELETE
Access-Control-Allow-Headers: Content-Type, X-Custom-Header
Access-Control-Max-Age: 86400
```

### セキュリティ考慮事項

#### 1. Origin検証の重要性

```go
type CORSConfig struct {
    AllowedOrigins     []string
    AllowAllOrigins    bool // 危険：本番環境では使用禁止
    AllowedMethods     []string
    AllowedHeaders     []string
    ExposedHeaders     []string
    AllowCredentials   bool
    MaxAge             int
}

func (c *CORSConfig) isOriginAllowed(origin string) bool {
    if c.AllowAllOrigins {
        return true // 危険
    }
    
    for _, allowed := range c.AllowedOrigins {
        if origin == allowed {
            return true
        }
        
        // ワイルドカードサブドメインのサポート
        if strings.HasPrefix(allowed, "*.") {
            domain := allowed[2:]
            if strings.HasSuffix(origin, "."+domain) {
                return true
            }
        }
    }
    
    return false
}
```

#### 2. 認証情報付きリクエストの制限

```go
func (cors *CORS) handleCredentials(w http.ResponseWriter, origin string) {
    if cors.config.AllowCredentials {
        // 認証情報を許可する場合、Origin を明示的に指定
        w.Header().Set("Access-Control-Allow-Origin", origin)
        w.Header().Set("Access-Control-Allow-Credentials", "true")
        
        // Vary ヘッダーでキャッシュ制御
        w.Header().Set("Vary", "Origin")
    }
}
```

#### 3. ヘッダーの適切な制限

```go
var dangerousHeaders = map[string]bool{
    "host":               true,
    "connection":         true,
    "upgrade":            true,
    "proxy-authorization": true,
}

func (cors *CORS) isHeaderAllowed(header string) bool {
    header = strings.ToLower(header)
    
    // 危険なヘッダーを拒否
    if dangerousHeaders[header] {
        return false
    }
    
    // 許可リストをチェック
    for _, allowed := range cors.config.AllowedHeaders {
        if strings.ToLower(allowed) == header {
            return true
        }
    }
    
    return false
}
```

### プリフライト最適化

#### キャッシュ戦略

```go
func (cors *CORS) setPreflightCache(w http.ResponseWriter) {
    // 適切なキャッシュ時間を設定（1時間〜24時間）
    maxAge := strconv.Itoa(cors.config.MaxAge)
    w.Header().Set("Access-Control-Max-Age", maxAge)
    
    // プロキシでのキャッシュも制御
    w.Header().Set("Cache-Control", "public, max-age="+maxAge)
}
```

#### 動的Origin許可

```go
func (cors *CORS) checkDynamicOrigin(origin string) bool {
    // 開発環境での動的許可
    if cors.isDevelopment() && strings.HasPrefix(origin, "http://localhost:") {
        return true
    }
    
    // データベースからの動的許可リスト
    return cors.isOriginInDatabase(origin)
}
```

### 高度なCORS設定

#### 条件付きCORS

```go
func (cors *CORS) ConditionalMiddleware(condition func(*http.Request) bool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if condition(r) {
                cors.Middleware(next).ServeHTTP(w, r)
            } else {
                next.ServeHTTP(w, r)
            }
        })
    }
}

// 使用例：API エンドポイントのみに CORS を適用
apiOnlyCORS := cors.ConditionalMiddleware(func(r *http.Request) bool {
    return strings.HasPrefix(r.URL.Path, "/api/")
})
```

#### ルート別CORS設定

```go
type RouteCORSConfig struct {
    Path       string
    CORSConfig CORSConfig
}

func (cors *CORS) RouteSpecificMiddleware(routes []RouteCORSConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            for _, route := range routes {
                if strings.HasPrefix(r.URL.Path, route.Path) {
                    corsHandler := NewCORS(route.CORSConfig)
                    corsHandler.Middleware(next).ServeHTTP(w, r)
                    return
                }
            }
            
            // デフォルトCORS設定
            cors.Middleware(next).ServeHTTP(w, r)
        })
    }
}
```

### セキュリティベストプラクティス

1. **最小権限の原則**: 必要最小限のオリジン、メソッド、ヘッダーのみ許可
2. **ワイルドカードの制限**: `*` の使用は認証情報なしの場合のみ
3. **HTTPS強制**: 本番環境では HTTPS オリジンのみ許可
4. **定期的な監査**: 許可されたオリジンの定期見直し
5. **ログ記録**: CORS エラーのログ収集と分析

### テスト戦略

```go
func TestCORS(t *testing.T) {
    tests := []struct {
        name           string
        origin         string
        method         string
        headers        map[string]string
        expectedStatus int
        expectedCORS   map[string]string
    }{
        {
            name:   "Allowed origin",
            origin: "https://trusted.example.com",
            method: "GET",
            expectedStatus: 200,
            expectedCORS: map[string]string{
                "Access-Control-Allow-Origin": "https://trusted.example.com",
            },
        },
        {
            name:   "Blocked origin",
            origin: "https://malicious.com",
            method: "GET",
            expectedStatus: 403,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // テスト実装
        })
    }
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **CORS構造体**
   - 設定可能なオリジン、メソッド、ヘッダーリスト
   - 認証情報許可フラグ
   - プリフライトキャッシュ時間

2. **Origin検証機能**
   - 許可されたオリジンとのマッチング
   - ワイルドカードサブドメインサポート
   - 大文字小文字を区別しない比較

3. **プリフライト処理**
   - OPTIONSリクエストの適切な処理
   - リクエストメソッドとヘッダーの検証
   - 適切なCORSヘッダーの設定

4. **Simple Request処理**
   - GETやPOSTリクエストの処理
   - Origin検証とヘッダー設定
   - エラー時の適切な応答

5. **セキュリティ機能**
   - 認証情報付きリクエストの制限
   - 危険なヘッダーのブロック
   - 不正なオリジンの拒否

6. **設定管理**
   - 柔軟な CORS 設定
   - 開発・本番環境の切り替え
   - ルート別設定サポート

## ✅ 期待される挙動

### 成功パターン

#### 許可されたオリジンからのSimple Request：
```bash
curl -H "Origin: https://trusted.example.com" http://localhost:8080/api/data
```
```http
HTTP/1.1 200 OK
Access-Control-Allow-Origin: https://trusted.example.com
Access-Control-Allow-Credentials: true
Vary: Origin
Content-Type: application/json

{
  "data": "success",
  "timestamp": "2023-12-31T23:59:59Z"
}
```

#### プリフライトリクエスト：
```bash
curl -X OPTIONS \
     -H "Origin: https://trusted.example.com" \
     -H "Access-Control-Request-Method: PUT" \
     -H "Access-Control-Request-Headers: Content-Type, X-Custom-Header" \
     http://localhost:8080/api/data
```
```http
HTTP/1.1 200 OK
Access-Control-Allow-Origin: https://trusted.example.com
Access-Control-Allow-Methods: GET, POST, PUT, DELETE
Access-Control-Allow-Headers: Content-Type, X-Custom-Header, Authorization
Access-Control-Max-Age: 86400
Access-Control-Allow-Credentials: true
Vary: Origin
```

#### ワイルドカードサブドメイン：
```bash
curl -H "Origin: https://app.trusted.example.com" http://localhost:8080/api/data
```
許可設定 `*.trusted.example.com` で許可される

### エラーパターン

#### 許可されていないオリジン（403 Forbidden）：
```bash
curl -H "Origin: https://malicious.com" http://localhost:8080/api/data
```
```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": "Origin not allowed",
  "origin": "https://malicious.com"
}
```

#### 許可されていないメソッド（405 Method Not Allowed）：
```bash
curl -X OPTIONS \
     -H "Origin: https://trusted.example.com" \
     -H "Access-Control-Request-Method: PATCH" \
     http://localhost:8080/api/data
```
```http
HTTP/1.1 405 Method Not Allowed
Content-Type: application/json

{
  "error": "Method not allowed",
  "method": "PATCH",
  "allowed_methods": ["GET", "POST", "PUT", "DELETE"]
}
```

## 💡 ヒント

1. **strings.HasPrefix/HasSuffix**: ワイルドカードマッチング
2. **http.MethodOptions**: プリフライトリクエストの検出
3. **r.Header.Get("Origin")**: オリジンヘッダーの取得
4. **strings.ToLower()**: 大文字小文字を区別しない比較
5. **http.StatusForbidden**: オリジン拒否時のステータス
6. **Vary: Origin**: キャッシュ制御ヘッダー

### CORS設定例

```go
config := CORSConfig{
    AllowedOrigins: []string{
        "https://example.com",
        "*.trusted.example.com",
        "http://localhost:3000", // 開発環境
    },
    AllowedMethods: []string{
        "GET", "POST", "PUT", "DELETE", "OPTIONS",
    },
    AllowedHeaders: []string{
        "Content-Type", "Authorization", "X-Requested-With",
    },
    ExposedHeaders: []string{
        "X-Total-Count", "X-Page-Number",
    },
    AllowCredentials: true,
    MaxAge:          86400, // 24時間
}
```

### セキュリティチェックリスト

- [ ] Origin の厳密な検証
- [ ] 認証情報付きリクエストでのワイルドカード禁止
- [ ] 危険なヘッダーのブロック
- [ ] プリフライトキャッシュの適切な設定
- [ ] HTTPS 環境での設定確認

### ブラウザテスト例

```html
<!DOCTYPE html>
<html>
<head>
    <title>CORS Test</title>
</head>
<body>
    <script>
        // Simple Request
        fetch('http://localhost:8080/api/data', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            }
        })
        .then(response => response.json())
        .then(data => console.log('Simple request:', data));

        // Preflight Request
        fetch('http://localhost:8080/api/data', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'X-Custom-Header': 'value'
            },
            body: JSON.stringify({test: 'data'})
        })
        .then(response => response.json())
        .then(data => console.log('Preflight request:', data));
    </script>
</body>
</html>
```

これらの実装により、セキュアで柔軟なCORS制御システムの基礎を学ぶことができます。