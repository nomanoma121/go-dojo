# Day 21: 認証ミドルウェア

🎯 **本日の目標**
HTTPヘッダーからトークンを読み取り、リクエストを認証するミドルウェアを実装し、JWTとAPIキーによる2つの認証方式とロールベースのアクセス制御を学ぶ。

## 📖 解説

### 認証ミドルウェアの重要性

```go
// 【認証ミドルウェアの重要性】セキュリティ侵害と不正アクセス防御
// ❌ 問題例：認証なしAPIによる壊滅的なデータ漏洩災害
func catastrophicNoAuthentication() {
    // 🚨 災害例：認証なしAPIエンドポイントで企業データ全漏洩
    
    http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        // ❌ 認証チェックなしでユーザーデータを返却
        users, err := getAllUsers() // 全ユーザーの個人情報
        if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        
        // ❌ 誰でも1000万人分の個人情報にアクセス可能
        // - 氏名、住所、電話番号、メールアドレス
        // - クレジットカード情報、銀行口座
        // - 機密ビジネスデータ、内部情報
        // - システム管理者パスワード
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(users)
        
        log.Printf("❌ Returned %d users without authentication", len(users))
        // 攻撃者による大量アクセス実行中
    })
    
    http.HandleFunc("/api/admin/delete-all", func(w http.ResponseWriter, r *http.Request) {
        // ❌ 管理者権限チェックなしで危険操作
        if r.Method == "DELETE" {
            err := deleteAllUserData() // 全データ削除
            if err != nil {
                http.Error(w, "Delete failed", http.StatusInternalServerError)
                return
            }
            
            // ❌ 誰でもクリック一つで全データ削除可能
            // - 10年分の顧客データ消失
            // - バックアップなし→復旧不可能
            // - 事業継続不可能、倒産危機
            
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("All data deleted"))
            
            log.Printf("❌ ALL USER DATA DELETED - No authentication required!")
        }
    })
    
    http.HandleFunc("/api/financial/transactions", func(w http.ResponseWriter, r *http.Request) {
        // ❌ 金融取引データも認証なしでアクセス可能
        transactions := getAllTransactions() // 全金融取引データ
        
        // ❌ 攻撃者が不正に取得可能
        // - 銀行取引履歴
        // - クレジットカード決済情報
        // - 投資ポートフォリオ
        // - 暗号通貨ウォレット情報
        
        json.NewEncoder(w).Encode(transactions)
        log.Printf("❌ Financial data exposed without authentication")
    })
    
    // 【攻撃シナリオ】自動化された大規模データ盗取
    // 1. 検索エンジンがAPIを発見→インデックス化
    // 2. セキュリティスキャナーがURLを発見
    // 3. 攻撃者が自動化ツールで数分で全データ盗取
    // 4. ダークウェブで個人情報販売
    // 5. 法的責任、罰金、信用失墜、廃業
    
    log.Println("❌ Starting server with NO authentication...")
    http.ListenAndServe(":8080", nil)
    // 結果：数時間でデータ全漏洩、法的責任、事業停止、倒産
}

// ✅ 正解：エンタープライズ級認証・認可システム
type EnterpriseAuthSystem struct {
    // 【基本認証機能】
    jwtValidator    *JWTValidator           // JWT検証システム
    apiKeyManager   *APIKeyManager          // APIキー管理
    sessionManager  *SessionManager         // セッション管理
    
    // 【高度なセキュリティ】
    mfaValidator    *MFAValidator           // 多要素認証
    oauth2Provider  *OAuth2Provider         // OAuth2プロバイダー
    samlProvider    *SAMLProvider           // SAML認証
    
    // 【ロール・権限管理】
    rbacManager     *RBACManager            // ロールベースアクセス制御
    abacEngine      *ABACEngine             // 属性ベースアクセス制御
    policyEngine    *PolicyEngine           // ポリシーエンジン
    
    // 【セキュリティ監視】
    auditLogger     *AuditLogger            // 監査ログ
    anomalyDetector *AnomalyDetector        // 異常検知
    threatAnalyzer  *ThreatAnalyzer         // 脅威分析
    
    // 【攻撃対策】
    rateLimiter     *AuthRateLimiter        // 認証試行制限
    bruteForceProtector *BruteForceProtector // ブルートフォース攻撃防御
    ipWhitelisting  *IPWhitelisting         // IP許可リスト
    
    // 【コンプライアンス】
    gdprCompliance  *GDPRCompliance         // GDPR準拠
    hipaaCompliance *HIPAACompliance        // HIPAA準拠
    pciCompliance   *PCICompliance          // PCI-DSS準拠
    
    // 【監視・メトリクス】
    metrics         *SecurityMetrics        // セキュリティメトリクス
    alertManager    *SecurityAlertManager   // セキュリティアラート
    
    mu              sync.RWMutex            // 設定変更保護
}

// 【重要関数】エンタープライズ認証システム初期化
func NewEnterpriseAuthSystem(config *AuthConfig) *EnterpriseAuthSystem {
    auth := &EnterpriseAuthSystem{
        jwtValidator:        NewJWTValidator(config.JWTSecret, config.JWTAlgorithm),
        apiKeyManager:       NewAPIKeyManager(config.APIKeys),
        sessionManager:      NewSessionManager(config.SessionConfig),
        mfaValidator:        NewMFAValidator(config.MFAConfig),
        oauth2Provider:      NewOAuth2Provider(config.OAuth2Config),
        samlProvider:        NewSAMLProvider(config.SAMLConfig),
        rbacManager:         NewRBACManager(config.RBACRules),
        abacEngine:          NewABACEngine(config.ABACPolicies),
        policyEngine:        NewPolicyEngine(config.Policies),
        auditLogger:         NewAuditLogger(config.AuditConfig),
        anomalyDetector:     NewAnomalyDetector(),
        threatAnalyzer:      NewThreatAnalyzer(),
        rateLimiter:         NewAuthRateLimiter(config.RateLimits),
        bruteForceProtector: NewBruteForceProtector(config.BruteForceConfig),
        ipWhitelisting:      NewIPWhitelisting(config.AllowedIPs),
        gdprCompliance:      NewGDPRCompliance(),
        hipaaCompliance:     NewHIPAACompliance(),
        pciCompliance:       NewPCICompliance(),
        metrics:            NewSecurityMetrics(),
        alertManager:       NewSecurityAlertManager(),
    }
    
    // 【重要】バックグラウンド監視開始
    go auth.startSecurityMonitoring()
    go auth.startThreatAnalysis()
    go auth.startComplianceChecks()
    
    log.Printf("🔐 Enterprise authentication system initialized")
    log.Printf("   JWT validation: %s algorithm", config.JWTAlgorithm)
    log.Printf("   API keys: %d registered", len(config.APIKeys))
    log.Printf("   RBAC roles: %d configured", len(config.RBACRules))
    log.Printf("   Security monitoring: ENABLED")
    
    return auth
}

// 【核心メソッド】多層認証ミドルウェア
func (auth *EnterpriseAuthSystem) ComprehensiveAuthMiddleware(
    authMethods []AuthMethod,
    requiredPermissions []Permission,
    complianceLevel ComplianceLevel,
) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            startTime := time.Now()
            requestID := generateSecureRequestID()
            clientIP := getClientIP(r)
            userAgent := r.UserAgent()
            
            // 【STEP 1】セキュリティ事前チェック
            if blocked, reason := auth.ipWhitelisting.IsBlocked(clientIP); blocked {
                auth.auditLogger.LogSecurityEvent("IP_BLOCKED", clientIP, reason)
                auth.metrics.RecordBlockedRequest("ip_blocked")
                http.Error(w, "Access denied", http.StatusForbidden)
                return
            }
            
            // 【STEP 2】レート制限チェック
            if !auth.rateLimiter.AllowAuthAttempt(clientIP) {
                auth.auditLogger.LogSecurityEvent("RATE_LIMIT_EXCEEDED", clientIP, "")
                auth.metrics.RecordBlockedRequest("rate_limited")
                http.Error(w, "Too many requests", http.StatusTooManyRequests)
                return
            }
            
            // 【STEP 3】複数認証方式の試行
            var authenticatedUser *User
            var authMethod AuthMethod
            var authError error
            
            for _, method := range authMethods {
                switch method {
                case JWTAuth:
                    if user, err := auth.tryJWTAuthentication(r); err == nil {
                        authenticatedUser = user
                        authMethod = JWTAuth
                        break
                    } else {
                        authError = err
                    }
                    
                case APIKeyAuth:
                    if user, err := auth.tryAPIKeyAuthentication(r); err == nil {
                        authenticatedUser = user
                        authMethod = APIKeyAuth
                        break
                    } else {
                        authError = err
                    }
                    
                case SessionAuth:
                    if user, err := auth.trySessionAuthentication(r); err == nil {
                        authenticatedUser = user
                        authMethod = SessionAuth
                        break
                    } else {
                        authError = err
                    }
                    
                case OAuth2Auth:
                    if user, err := auth.tryOAuth2Authentication(r); err == nil {
                        authenticatedUser = user
                        authMethod = OAuth2Auth
                        break
                    } else {
                        authError = err
                    }
                }
            }
            
            // 【STEP 4】認証失敗処理
            if authenticatedUser == nil {
                auth.bruteForceProtector.RecordFailedAttempt(clientIP, userAgent)
                auth.auditLogger.LogAuthenticationFailure(requestID, clientIP, userAgent, authError)
                auth.metrics.RecordAuthenticationFailure(string(authMethod))
                
                // 異常検知
                if auth.anomalyDetector.IsAnomalousAuth(clientIP, userAgent) {
                    auth.alertManager.TriggerSecurityAlert("SUSPICIOUS_AUTH_PATTERN", clientIP)
                }
                
                http.Error(w, "Authentication required", http.StatusUnauthorized)
                return
            }
            
            // 【STEP 5】多要素認証チェック（必要な場合）
            if auth.requiresMFA(authenticatedUser, r) {
                if !auth.validateMFA(r, authenticatedUser) {
                    auth.auditLogger.LogMFAFailure(requestID, authenticatedUser.ID, clientIP)
                    http.Error(w, "Multi-factor authentication required", http.StatusUnauthorized)
                    return
                }
            }
            
            // 【STEP 6】権限チェック（RBAC + ABAC）
            if len(requiredPermissions) > 0 {
                // ロールベースチェック
                if !auth.rbacManager.HasPermissions(authenticatedUser, requiredPermissions) {
                    auth.auditLogger.LogAuthorizationFailure(requestID, authenticatedUser.ID, requiredPermissions)
                    auth.metrics.RecordAuthorizationFailure("rbac")
                    http.Error(w, "Insufficient permissions", http.StatusForbidden)
                    return
                }
                
                // 属性ベースチェック
                context := auth.buildABACContext(r, authenticatedUser)
                if !auth.abacEngine.Evaluate(context, requiredPermissions) {
                    auth.auditLogger.LogAuthorizationFailure(requestID, authenticatedUser.ID, requiredPermissions)
                    auth.metrics.RecordAuthorizationFailure("abac")
                    http.Error(w, "Access denied by policy", http.StatusForbidden)
                    return
                }
            }
            
            // 【STEP 7】コンプライアンスチェック
            if complianceLevel != ComplianceNone {
                if err := auth.checkCompliance(authenticatedUser, r, complianceLevel); err != nil {
                    auth.auditLogger.LogComplianceViolation(requestID, authenticatedUser.ID, err)
                    http.Error(w, "Compliance requirements not met", http.StatusForbidden)
                    return
                }
            }
            
            // 【STEP 8】セッション管理
            sessionToken := auth.sessionManager.CreateSession(authenticatedUser, clientIP, userAgent)
            w.Header().Set("X-Session-Token", sessionToken)
            
            // 【STEP 9】リクエストコンテキスト作成
            authContext := &AuthenticationContext{
                User:            authenticatedUser,
                Method:          authMethod,
                RequestID:       requestID,
                ClientIP:        clientIP,
                UserAgent:       userAgent,
                Permissions:     auth.rbacManager.GetUserPermissions(authenticatedUser),
                SessionToken:    sessionToken,
                AuthTime:        startTime,
                ComplianceLevel: complianceLevel,
            }
            
            ctx := context.WithValue(r.Context(), "auth", authContext)
            
            // 【STEP 10】成功ログとメトリクス
            auth.auditLogger.LogSuccessfulAuthentication(requestID, authenticatedUser.ID, authMethod, clientIP)
            auth.metrics.RecordSuccessfulAuthentication(string(authMethod))
            
            // 【STEP 11】次のハンドラーへ
            next.ServeHTTP(w, r.WithContext(ctx))
            
            // 【STEP 12】完了後処理
            duration := time.Since(startTime)
            auth.metrics.RecordAuthenticationDuration(duration)
            auth.auditLogger.LogRequestCompletion(requestID, duration)
        })
    }
}

// 【重要メソッド】JWT認証試行
func (auth *EnterpriseAuthSystem) tryJWTAuthentication(r *http.Request) (*User, error) {
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return nil, errors.New("authorization header missing")
    }
    
    // Bearer プレフィックスチェック
    if !strings.HasPrefix(authHeader, "Bearer ") {
        return nil, errors.New("invalid authorization format")
    }
    
    tokenString := strings.TrimPrefix(authHeader, "Bearer ")
    
    // JWT検証
    claims, err := auth.jwtValidator.ValidateToken(tokenString)
    if err != nil {
        return nil, fmt.Errorf("invalid JWT: %w", err)
    }
    
    // ユーザー情報構築
    user := &User{
        ID:          claims.Subject,
        Email:       claims.Email,
        Roles:       claims.Roles,
        Permissions: claims.Permissions,
        TokenType:   "JWT",
        ExpiresAt:   time.Unix(claims.ExpiresAt, 0),
    }
    
    // トークン有効期限チェック
    if time.Now().After(user.ExpiresAt) {
        return nil, errors.New("token expired")
    }
    
    return user, nil
}

// 【重要メソッド】APIキー認証試行
func (auth *EnterpriseAuthSystem) tryAPIKeyAuthentication(r *http.Request) (*User, error) {
    apiKey := r.Header.Get("X-API-Key")
    if apiKey == "" {
        return nil, errors.New("API key header missing")
    }
    
    // APIキー検証
    keyInfo, err := auth.apiKeyManager.ValidateKey(apiKey)
    if err != nil {
        return nil, fmt.Errorf("invalid API key: %w", err)
    }
    
    // レート制限チェック（APIキー固有）
    if !auth.apiKeyManager.CheckRateLimit(apiKey) {
        return nil, errors.New("API key rate limit exceeded")
    }
    
    // ユーザー情報構築
    user := &User{
        ID:          keyInfo.UserID,
        Email:       keyInfo.Email,
        Roles:       keyInfo.Roles,
        Permissions: keyInfo.Permissions,
        TokenType:   "API_KEY",
        ExpiresAt:   keyInfo.ExpiresAt,
    }
    
    // APIキー有効期限チェック
    if time.Now().After(user.ExpiresAt) {
        return nil, errors.New("API key expired")
    }
    
    // 使用量記録
    auth.apiKeyManager.RecordUsage(apiKey, getClientIP(r))
    
    return user, nil
}

// 【実用例】プロダクション環境での包括的認証システム
func ProductionAuthenticationUsage() {
    // 【設定】エンタープライズ認証設定
    config := &AuthConfig{
        JWTSecret:    getEnvOrDefault("JWT_SECRET", ""),
        JWTAlgorithm: "HS256",
        APIKeys: map[string]*APIKeyInfo{
            "api-key-admin-001": {
                UserID:      "admin-001",
                Email:       "admin@company.com",
                Roles:       []string{"admin", "super-admin"},
                Permissions: []Permission{PermissionReadAll, PermissionWriteAll, PermissionDeleteAll},
                ExpiresAt:   time.Now().AddDate(1, 0, 0), // 1年後
                RateLimit:   1000, // 1000 req/hour
            },
            "api-key-service-001": {
                UserID:      "service-001",
                Email:       "service@company.com",
                Roles:       []string{"service"},
                Permissions: []Permission{PermissionReadUsers, PermissionWriteUsers},
                ExpiresAt:   time.Now().AddDate(0, 6, 0), // 6ヶ月後
                RateLimit:   10000, // 10000 req/hour
            },
        },
        SessionConfig: &SessionConfig{
            Timeout:    30 * time.Minute,
            SecureCookie: true,
            SameSite:   http.SameSiteStrictMode,
        },
        RBACRules: map[string][]Permission{
            "admin":      {PermissionReadAll, PermissionWriteAll, PermissionDeleteAll},
            "user":       {PermissionReadUsers, PermissionWriteUsers},
            "readonly":   {PermissionReadUsers},
            "service":    {PermissionReadUsers, PermissionWriteUsers},
        },
        RateLimits: &RateLimitConfig{
            MaxAttemptsPerIP:     10,
            MaxAttemptsPerUser:   5,
            TimeWindow:          time.Hour,
            BanDuration:         24 * time.Hour,
        },
        BruteForceConfig: &BruteForceConfig{
            MaxFailures:         5,
            LockoutDuration:     15 * time.Minute,
            ProgressiveLockout:  true,
        },
        AllowedIPs: []string{
            "10.0.0.0/8",      // 内部ネットワーク
            "192.168.0.0/16",  // プライベートネットワーク
            "172.16.0.0/12",   // プライベートネットワーク
        },
    }
    
    auth := NewEnterpriseAuthSystem(config)
    
    // 【ルーター設定】
    mux := http.NewServeMux()
    
    // 【パブリックエンドポイント】認証不要
    mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status": "healthy",
            "timestamp": time.Now().Format(time.RFC3339),
        })
    })
    
    // 【ユーザーエンドポイント】JWT or APIキー認証
    userHandler := auth.ComprehensiveAuthMiddleware(
        []AuthMethod{JWTAuth, APIKeyAuth},
        []Permission{PermissionReadUsers},
        ComplianceBasic,
    )(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authCtx := getAuthContext(r)
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "message": "User endpoint accessed",
            "user": map[string]interface{}{
                "id":          authCtx.User.ID,
                "email":       authCtx.User.Email,
                "roles":       authCtx.User.Roles,
                "method":      string(authCtx.Method),
                "permissions": authCtx.Permissions,
            },
        })
    }))
    mux.Handle("/api/users", userHandler)
    
    // 【管理者エンドポイント】JWT認証 + 管理者権限
    adminHandler := auth.ComprehensiveAuthMiddleware(
        []AuthMethod{JWTAuth},
        []Permission{PermissionReadAll, PermissionWriteAll},
        ComplianceStrict,
    )(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authCtx := getAuthContext(r)
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "message": "Admin endpoint accessed",
            "user": map[string]interface{}{
                "id":    authCtx.User.ID,
                "email": authCtx.User.Email,
                "roles": authCtx.User.Roles,
            },
            "compliance_level": string(authCtx.ComplianceLevel),
        })
    }))
    mux.Handle("/api/admin", adminHandler)
    
    // 【サーバー起動】
    server := &http.Server{
        Addr:    ":8080",
        Handler: mux,
        TLSConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
            CipherSuites: []uint16{
                tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
                tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
                tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
            },
        },
    }
    
    log.Printf("🚀 Enterprise authentication server starting on :8080")
    log.Printf("   JWT authentication: ENABLED")
    log.Printf("   API key authentication: ENABLED") 
    log.Printf("   RBAC authorization: ENABLED")
    log.Printf("   Compliance checking: ENABLED")
    log.Printf("   Security monitoring: ENABLED")
    
    // HTTPS起動
    log.Fatal(server.ListenAndServeTLS("server.crt", "server.key"))
}
```

### Web認証の基礎

Web APIにおける認証は、ユーザーが誰であるかを確認する重要なセキュリティ機能です。認証なしでは、すべてのAPIエンドポイントが公開されてしまいます。

#### 認証 vs 認可
- **認証（Authentication）**: ユーザーが本人かどうか確認する
- **認可（Authorization）**: 認証されたユーザーが特定のリソースにアクセスする権限があるか確認する

### JWTによる認証

JWT（JSON Web Token）は、情報を安全に送信するためのコンパクトなトークン形式です。

#### JWT の構造
```
header.payload.signature
```

- **Header**: アルゴリズムとトークンタイプを定義
- **Payload**: ユーザー情報やクレームを含む
- **Signature**: 改ざん検知のための署名

```go
// JWTの例
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.
eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.
SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

### HTTPヘッダーでの認証情報送信

#### Bearer Token方式
```http
Authorization: Bearer <token>
```

#### API Key方式
```http
X-API-Key: <api-key>
```

### Go での JWT 実装例

簡単なJWT検証の実装：

```go
func validateJWT(tokenString string, secret string) (*User, error) {
    // トークンの形式チェック
    parts := strings.Split(tokenString, ".")
    if len(parts) != 3 {
        return nil, errors.New("invalid token format")
    }
    
    // 署名検証（実際のプロダクションでは適切なライブラリを使用）
    header := parts[0]
    payload := parts[1]
    signature := parts[2]
    
    // ペイロードデコード
    payloadBytes, err := base64.URLEncoding.DecodeString(payload)
    if err != nil {
        return nil, err
    }
    
    var claims map[string]interface{}
    if err := json.Unmarshal(payloadBytes, &claims); err != nil {
        return nil, err
    }
    
    // ユーザー情報の抽出
    user := &User{
        ID:    claims["sub"].(string),
        Email: claims["email"].(string),
    }
    
    return user, nil
}
```

### ミドルウェアでの認証実装

```go
func (am *AuthMiddleware) JWTAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Authorization ヘッダー取得
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Authorization header required", http.StatusUnauthorized)
            return
        }
        
        // Bearer プレフィックス確認
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        if tokenString == authHeader {
            http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
            return
        }
        
        // JWT検証
        user, err := am.validateJWT(tokenString)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        // Context にユーザー情報を追加
        ctx := context.WithValue(r.Context(), "user", user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### ロールベースアクセス制御（RBAC）

ユーザーには複数の役割（ロール）を割り当て、エンドポイントごとに必要な役割を定義します：

```go
type User struct {
    ID    string   `json:"id"`
    Email string   `json:"email"`
    Roles []string `json:"roles"`
}

func (am *AuthMiddleware) RequireRoles(requiredRoles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user, ok := getUserFromContext(r.Context())
            if !ok {
                http.Error(w, "User not authenticated", http.StatusUnauthorized)
                return
            }
            
            // ユーザーの役割をチェック
            hasRole := false
            for _, userRole := range user.Roles {
                for _, reqRole := range requiredRoles {
                    if userRole == reqRole {
                        hasRole = true
                        break
                    }
                }
                if hasRole {
                    break
                }
            }
            
            if !hasRole {
                http.Error(w, "Insufficient permissions", http.StatusForbidden)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### セキュリティ考慮事項

1. **秘密鍵の管理**: JWT署名用の秘密鍵は環境変数で管理
2. **トークンの有効期限**: 短期間の有効期限を設定
3. **HTTPS必須**: 本番環境では必ずHTTPS通信
4. **レート制限**: 同一IPからの大量リクエストを制限
5. **ログ記録**: 認証失敗をログに記録（セキュリティ監査用）

### エラーハンドリングのベストプラクティス

```go
func (am *AuthMiddleware) sendErrorResponse(w http.ResponseWriter, code int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    
    response := map[string]interface{}{
        "error": message,
        "timestamp": time.Now().Unix(),
    }
    
    json.NewEncoder(w).Encode(response)
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **AuthMiddleware構造体**
   - JWT秘密鍵とAPIキーのマップを保持
   - 初期化時にサンプルのAPIキーとユーザーを設定

2. **JWT認証ミドルウェア**
   - Authorizationヘッダーの`Bearer <token>`形式を解析
   - 簡略化されたJWT検証（実際のプロダクションでは`jwt-go`等のライブラリを使用）
   - ユーザー情報をContextに格納

3. **APIキー認証ミドルウェア**
   - X-API-Keyヘッダーからキーを取得
   - 事前登録されたキーかチェック
   - 対応するユーザー情報をContextに格納

4. **オプショナル認証**
   - 認証情報があれば検証、なければスキップ
   - 失敗してもエラーにしない

5. **ロールベース認可**
   - 必要な役割を指定できるミドルウェア関数
   - ユーザーの役割をチェックして403または次のハンドラーへ

6. **ヘルパー関数**
   - Contextからユーザー情報を取得
   - エラーレスポンスの統一的な送信

## ✅ 期待される挙動

### 成功パターン

#### 有効なJWTトークンでのアクセス：
```bash
curl -H "Authorization: Bearer valid-jwt-token" http://localhost:8080/protected
```
```json
{
  "message": "protected endpoint",
  "user": {
    "id": "user123",
    "email": "user@example.com",
    "roles": ["user"]
  }
}
```

#### 有効なAPIキーでのアクセス：
```bash
curl -H "X-API-Key: api-key-123" http://localhost:8080/protected
```
```json
{
  "message": "protected endpoint",
  "user": {
    "id": "api-user",
    "email": "api@example.com",
    "roles": ["admin"]
  }
}
```

### エラーパターン

#### 認証情報なし（401 Unauthorized）：
```json
{
  "error": "Authorization header required"
}
```

#### 無効なトークン（401 Unauthorized）：
```json
{
  "error": "Invalid token"
}
```

#### 権限不足（403 Forbidden）：
```json
{
  "error": "Insufficient permissions"
}
```

## 💡 ヒント

1. **strings.TrimPrefix**: "Bearer "プレフィックスの除去
2. **context.WithValue**: Contextにユーザー情報を格納
3. **type assertion**: interface{}からの型変換
4. **HTTP Status Codes**: 
   - 401: 認証失敗
   - 403: 認可失敗（権限不足）
5. **JSON encoding**: エラーレスポンスのJSON形式での送信
6. **slice contains**: スライス内の要素検索

### サンプルのJWTトークン形式（テスト用）

```go
// テスト用の簡略化されたJWT
// Header: {"alg":"HS256","typ":"JWT"}
// Payload: {"sub":"user123","email":"user@example.com","roles":["user"]}
// 実際のプロダクションでは適切なライブラリを使用すること
```

### ミドルウェアチェーンの例

```go
// 複数のミドルウェアを組み合わせ
protected := auth.JWTAuth(auth.RequireRoles("admin")(handler))
```

### セキュリティのヒント

- 本番環境では適切なJWTライブラリ（`github.com/golang-jwt/jwt`）を使用
- 秘密鍵は環境変数から読み込み
- トークンの有効期限をチェック
- レート制限やブルートフォース攻撃対策を検討

これらの実装により、本格的なWebアプリケーションで使用できる認証・認可システムの基礎を学ぶことができます。