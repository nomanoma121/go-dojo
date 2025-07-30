# Day 50: gRPC Unary Interceptor

## 🎯 本日の目標 (Today's Goal)

gRPCのUnaryインターセプタを実装し、全てのUnary RPCで共通の処理（ログ、認証、メトリクス収集）を挟み込む仕組みを習得する。プロダクションレベルのgRPCサービスにおける横断的関心事の実装方法を学ぶ。

## 📖 解説 (Explanation)

### Unaryインターセプタとは

```go
// 【gRPC Unaryインターセプタの重要性】エンタープライズAPIの横断的関心事の実装
// ❌ 問題例：インターセプタ実装ミスによるセキュリティ侵害と大規模障害
func unaryInterceptorDisasters() {
    // 🚨 災害例：不適切なインターセプタ実装による壊滅的セキュリティ侵害
    
    // ❌ 最悪の実装1：認証バイパス可能なインターセプタ
    func BadAuthInterceptor() grpc.UnaryServerInterceptor {
        return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
            // ❌ 認証スキップ可能なパス - セキュリティホール
            if strings.Contains(info.FullMethod, "/Health") {
                return handler(ctx, req) // OK、これは正常
            }
            
            // ❌ メタデータ取得エラーを無視 - 認証完全バイパス
            md, ok := metadata.FromIncomingContext(ctx)
            if !ok {
                return handler(ctx, req) // ❌ 認証なしで実行！
            }
            
            // ❌ トークン検証なし - 偽造トークンでも通過
            tokens := md.Get("authorization")
            if len(tokens) == 0 {
                return handler(ctx, req) // ❌ トークンなしでも実行！
            }
            
            // ❌ トークン形式チェックなし
            token := tokens[0]
            if token == "" {
                return handler(ctx, req) // ❌ 空文字でも実行！
            }
            
            // ❌ 実際のトークン検証なし - 「Bearer invalid」でも通過
            return handler(ctx, req)
        }
        
        // 【災害的結果】
        // - 攻撃者が空のAuthorizationヘッダーで全API呼び出し可能
        // - 顧客データベースへの無制限アクセス
        // - 機密情報流出、GDPR違反で制裁金10億円
    }
    
    // ❌ 最悪の実装2：メモリリークを引き起こすメトリクスインターセプタ
    func BadMetricsInterceptor() grpc.UnaryServerInterceptor {
        // ❌ グローバル変数でメトリクス保存 - メモリリーク
        var allRequests []RequestMetric // 削除されない！
        
        return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
            start := time.Now()
            
            resp, err := handler(ctx, req)
            
            // ❌ 全リクエストを永続保存 - メモリ使用量無限増加
            metric := RequestMetric{
                Method:    info.FullMethod,
                Duration:  time.Since(start),
                Error:     err,
                Timestamp: time.Now(),
                Request:   req,         // ❌ リクエスト全体を保存！
                Response:  resp,        // ❌ レスポンス全体を保存！
            }
            allRequests = append(allRequests, metric) // 無限に増加
            
            return resp, err
        }
        
        // 【災害的結果】
        // - 1日で10万リクエスト → メモリ使用量50GB
        // - 1週間後: サーバーOOM、全サービス停止
        // - アプリケーション再起動で一時的復旧、再度メモリリーク
    }
    
    // ❌ 最悪の実装3：DoS攻撃を増幅するレート制限インターセプタ
    func BadRateLimitInterceptor() grpc.UnaryServerInterceptor {
        // ❌ 同期マップ使用 - 競合状態でデッドロック
        requestCounts := make(map[string]int)
        
        return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
            // ❌ クライアントIP取得方法が脆弱
            clientIP := getClientIP(ctx) // X-Forwarded-For偽装可能
            
            // ❌ 排他制御なしでマップアクセス - レースコンディション
            requestCounts[clientIP]++
            
            // ❌ レート制限チェックが後 - リソース消費済み
            if requestCounts[clientIP] > 100 {
                return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
            }
            
            // ❌ 実際のハンドラは実行済み - CPU/メモリ消費後に制限
            resp, err := handler(ctx, req)
            
            return resp, err
        }
        
        // 【災害的結果】
        // - 攻撃者がX-Forwarded-Forを偽装して制限回避
        // - 競合状態によりレート制限が効かない
        // - 大量リクエストでCPU使用率100%、全API応答不能
    }
    
    // ❌ 最悪の実装4：機密情報を漏洩するログインターセプタ
    func BadLoggingInterceptor() grpc.UnaryServerInterceptor {
        return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
            // ❌ リクエスト全体をログ出力 - 機密情報流出
            log.Printf("Request: %s - Data: %+v", info.FullMethod, req)
            
            resp, err := handler(ctx, req)
            
            // ❌ レスポンス全体をログ出力 - 個人情報流出
            log.Printf("Response: %s - Data: %+v", info.FullMethod, resp)
            
            return resp, err
        }
        
        // 【災害的結果】
        // - パスワード、クレジットカード番号、個人情報がログに記録
        // - ログ監視システム経由で機密情報が開発チーム全員に配信
        // - 内部監査で発覚、GDPR違反、顧客信頼失墜
    }
    
    // 【実際の被害例】
    // - 金融API：認証バイパスで口座情報流出、監査法人から業務停止命令
    // - 医療システム：患者データ流出、プライバシー侵害で集団訴訟
    // - ECサイト：レート制限不備でクレデン情報流出、売上99%減
    // - SaaSサービス：メモリリークで全サービス停止、顧客離れ80%
    
    fmt.Println("❌ Unary interceptor disasters caused security breaches and service collapse!")
    // 結果：セキュリティ侵害、メモリリーク、DoS攻撃成功、信頼失墜
}

// ✅ 正解：エンタープライズ級Unaryインターセプタシステム
type EnterpriseUnaryInterceptorSystem struct {
    // 【認証・認可】
    authManager          *AuthManager                // 認証管理
    authorizationEngine  *AuthorizationEngine        // 認可エンジン
    tokenValidator       *TokenValidator             // トークン検証
    
    // 【セキュリティ】
    rateLimiter          *DistributedRateLimiter     // 分散レート制限
    ddosProtector        *DDoSProtector              // DDoS攻撃防御
    firewallManager      *FirewallManager            // ファイアウォール管理
    
    // 【監視・ログ】
    metricsCollector     *MetricsCollector           // メトリクス収集
    structuredLogger     *StructuredLogger           // 構造化ログ
    auditLogger          *AuditLogger                // 監査ログ
    
    // 【パフォーマンス】
    circuitBreaker       *CircuitBreaker             // サーキットブレーカー
    bulkheadManager      *BulkheadManager            // バルクヘッド分離
    timeoutManager       *TimeoutManager             // タイムアウト管理
    
    // 【データ保護】
    encryptionManager    *EncryptionManager          // 暗号化管理
    dataClassifier       *DataClassifier             // データ分類
    privacyProtector     *PrivacyProtector           // プライバシー保護
    
    // 【監査・コンプライアンス】
    complianceChecker    *ComplianceChecker          // コンプライアンスチェック
    gdprManager          *GDPRManager                // GDPR対応
    pciManager           *PCIManager                 // PCI-DSS対応
    
    // 【エラーハンドリング】
    errorEnricher        *ErrorEnricher              // エラー詳細化
    retryManager         *RetryManager               // リトライ管理
    
    config               *InterceptorConfig          // 設定管理
    mu                   sync.RWMutex                // 並行アクセス制御
}

// 【重要関数】エンタープライズUnaryインターセプタシステム初期化
func NewEnterpriseUnaryInterceptorSystem(config *InterceptorConfig) *EnterpriseUnaryInterceptorSystem {
    return &EnterpriseUnaryInterceptorSystem{
        config:               config,
        authManager:          NewAuthManager(),
        authorizationEngine:  NewAuthorizationEngine(),
        tokenValidator:       NewTokenValidator(),
        rateLimiter:          NewDistributedRateLimiter(),
        ddosProtector:        NewDDoSProtector(),
        firewallManager:      NewFirewallManager(),
        metricsCollector:     NewMetricsCollector(),
        structuredLogger:     NewStructuredLogger(),
        auditLogger:          NewAuditLogger(),
        circuitBreaker:       NewCircuitBreaker(),
        bulkheadManager:      NewBulkheadManager(),
        timeoutManager:       NewTimeoutManager(),
        encryptionManager:    NewEncryptionManager(),
        dataClassifier:       NewDataClassifier(),
        privacyProtector:     NewPrivacyProtector(),
        complianceChecker:    NewComplianceChecker(),
        gdprManager:          NewGDPRManager(),
        pciManager:           NewPCIManager(),
        errorEnricher:        NewErrorEnricher(),
        retryManager:         NewRetryManager(),
    }
}

// 【実用例】エンタープライズ級認証インターセプタ
func (euis *EnterpriseUnaryInterceptorSystem) CreateSecureAuthInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        
        // 【STEP 1】セキュリティ前チェック
        if err := euis.firewallManager.CheckRequest(ctx, info); err != nil {
            return nil, status.Errorf(codes.PermissionDenied, "firewall blocked: %v", err)
        }
        
        // 【STEP 2】認証が不要なメソッドをチェック
        if euis.authManager.IsPublicMethod(info.FullMethod) {
            return euis.executeWithMonitoring(ctx, req, info, handler)
        }
        
        // 【STEP 3】メタデータ取得
        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            euis.auditLogger.LogAuthFailure(ctx, info.FullMethod, "missing metadata")
            return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
        }
        
        // 【STEP 4】Authorizationヘッダー取得
        authHeaders := md.Get("authorization")
        if len(authHeaders) == 0 {
            euis.auditLogger.LogAuthFailure(ctx, info.FullMethod, "missing authorization header")
            return nil, status.Errorf(codes.Unauthenticated, "missing authorization header")
        }
        
        // 【STEP 5】トークン検証
        token := authHeaders[0]
        claims, err := euis.tokenValidator.ValidateToken(token)
        if err != nil {
            euis.auditLogger.LogAuthFailure(ctx, info.FullMethod, fmt.Sprintf("invalid token: %v", err))
            return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
        }
        
        // 【STEP 6】認可チェック
        if !euis.authorizationEngine.IsAuthorized(claims, info.FullMethod, req) {
            euis.auditLogger.LogAuthzFailure(ctx, claims.UserID, info.FullMethod, "insufficient permissions")
            return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions")
        }
        
        // 【STEP 7】コンテキストに認証情報を追加
        ctx = context.WithValue(ctx, "user_claims", claims)
        ctx = context.WithValue(ctx, "user_id", claims.UserID)
        
        // 【STEP 8】成功ログ
        euis.auditLogger.LogAuthSuccess(ctx, claims.UserID, info.FullMethod)
        
        return euis.executeWithMonitoring(ctx, req, info, handler)
    }
}

// 【核心メソッド】監視付き実行
func (euis *EnterpriseUnaryInterceptorSystem) executeWithMonitoring(
    ctx context.Context,
    req interface{},
    info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler,
) (interface{}, error) {
    
    // メトリクス開始
    startTime := time.Now()
    
    // サーキットブレーカーチェック
    if !euis.circuitBreaker.AllowRequest(info.FullMethod) {
        euis.metricsCollector.RecordCircuitBreakerOpen(info.FullMethod)
        return nil, status.Errorf(codes.Unavailable, "circuit breaker open")
    }
    
    // タイムアウト設定
    ctx, cancel := euis.timeoutManager.SetTimeout(ctx, info.FullMethod)
    defer cancel()
    
    // 実際のハンドラ実行
    resp, err := handler(ctx, req)
    
    // メトリクス記録
    duration := time.Since(startTime)
    euis.metricsCollector.RecordRequest(info.FullMethod, duration, err)
    
    // サーキットブレーカー結果記録
    if err != nil {
        euis.circuitBreaker.RecordFailure(info.FullMethod)
    } else {
        euis.circuitBreaker.RecordSuccess(info.FullMethod)
    }
    
    return resp, err
}

// 【実用例】エンタープライズ級レート制限インターセプタ
func (euis *EnterpriseUnaryInterceptorSystem) CreateDistributedRateLimitInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        
        // 【STEP 1】クライアント識別（複数方式）
        clientID := euis.identifyClient(ctx)
        
        // 【STEP 2】分散レート制限チェック
        allowed, remainingQuota, resetTime, err := euis.rateLimiter.CheckRate(
            clientID, 
            info.FullMethod,
        )
        if err != nil {
            euis.structuredLogger.Error("rate limit check failed", 
                map[string]interface{}{
                    "client_id": clientID,
                    "method": info.FullMethod,
                    "error": err.Error(),
                })
            // エラー時はレート制限を適用しない（フェイルオープン）
        } else if !allowed {
            // レート制限ヘッダーを設定
            grpc.SetHeader(ctx, metadata.Pairs(
                "X-RateLimit-Remaining", fmt.Sprintf("%d", remainingQuota),
                "X-RateLimit-Reset", fmt.Sprintf("%d", resetTime.Unix()),
            ))
            
            euis.metricsCollector.RecordRateLimitExceeded(clientID, info.FullMethod)
            return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
        }
        
        // 【STEP 3】正常処理
        return handler(ctx, req)
    }
}

// 【高度機能】クライアント識別
func (euis *EnterpriseUnaryInterceptorSystem) identifyClient(ctx context.Context) string {
    // 1. 認証済みユーザーID
    if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
        return "user:" + userID
    }
    
    // 2. APIキー
    if md, ok := metadata.FromIncomingContext(ctx); ok {
        if apiKeys := md.Get("x-api-key"); len(apiKeys) > 0 {
            return "api_key:" + euis.hashAPIKey(apiKeys[0])
        }
    }
    
    // 3. クライアントIP（最後の手段）
    return "ip:" + euis.getClientIP(ctx)
}

// 【実用例】プライバシー保護ログインターセプタ
func (euis *EnterpriseUnaryInterceptorSystem) CreatePrivacyAwareLoggingInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        startTime := time.Now()
        
        // 【重要】機密データの除去
        safeRequest := euis.privacyProtector.SanitizeForLogging(req)
        
        // 構造化ログ
        euis.structuredLogger.Info("request started", map[string]interface{}{
            "method":    info.FullMethod,
            "request":   safeRequest,
            "timestamp": startTime.UTC(),
            "trace_id":  euis.getTraceID(ctx),
        })
        
        resp, err := handler(ctx, req)
        
        duration := time.Since(startTime)
        
        // レスポンスも機密データ除去
        safeResponse := euis.privacyProtector.SanitizeForLogging(resp)
        
        logLevel := "info"
        if err != nil {
            logLevel = "error"
        }
        
        euis.structuredLogger.Log(logLevel, "request completed", map[string]interface{}{
            "method":     info.FullMethod,
            "duration":   duration.Milliseconds(),
            "response":   safeResponse,
            "error":      euis.errorEnricher.SafeErrorMessage(err),
            "timestamp":  time.Now().UTC(),
            "trace_id":   euis.getTraceID(ctx),
        })
        
        return resp, err
    }
}
```

Unaryインターセプタは、gRPCのUnary RPC（1リクエスト-1レスポンス）の前後で共通処理を実行するためのミドルウェア機能です。

### 主な用途

1. **ログ出力**: リクエスト/レスポンスのログ
2. **認証・認可**: トークン検証やアクセス制御
3. **メトリクス収集**: レスポンス時間やエラー率の測定
4. **エラーハンドリング**: 統一されたエラー処理
5. **レート制限**: リクエスト頻度の制御

### 実装パターン

```go
// サーバーサイドインターセプタ
func LoggingInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        start := time.Now()
        
        // リクエストログ
        log.Printf("Request: %s", info.FullMethod)
        
        // 実際のハンドラを実行
        resp, err := handler(ctx, req)
        
        // レスポンスログ
        duration := time.Since(start)
        log.Printf("Response: %s (duration: %v, error: %v)", info.FullMethod, duration, err)
        
        return resp, err
    }
}

// クライアントサイドインターセプタ
func AuthInterceptor(token string) grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        // 認証ヘッダーを追加
        ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
        
        // 実際のRPCを実行
        return invoker(ctx, method, req, reply, cc, opts...)
    }
}

// インターセプタの登録
server := grpc.NewServer(
    grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
        LoggingInterceptor(),
        AuthInterceptor(),
        MetricsInterceptor(),
    )),
)
```

## 📝 課題 (The Problem)

Unaryインターセプタを使用して以下の機能を実装してください：

1. **ログインターセプタ**: リクエスト/レスポンスの詳細ログ
2. **認証インターセプタ**: JWTトークンによる認証
3. **メトリクスインターセプタ**: レスポンス時間とエラー率の収集
4. **レート制限インターセプタ**: IPベースのレート制限
5. **インターセプタチェーン**: 複数のインターセプタの組み合わせ

## 💡 ヒント (Hints)

- `grpc.UnaryServerInterceptor`と`grpc.UnaryClientInterceptor`の使用
- `context.Context`を使ったメタデータの伝播
- `grpc.UnaryHandler`による実際のRPC実行
- エラーハンドリングとメトリクス収集の組み合わせ