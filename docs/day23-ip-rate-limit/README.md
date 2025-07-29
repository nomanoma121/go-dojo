# Day 23: IPベースレートリミットミドルウェア

🎯 **本日の目標**
IPアドレス単位でリクエスト頻度を制限するレートリミットミドルウェアを実装し、DDoS攻撃や過負荷からサーバーを保護する手法を学ぶ。

## 📖 解説

### レートリミットの重要性

```go
// 【レートリミットの重要性】DDoS攻撃とリソース枯渇攻撃からの防御
// ❌ 問題例：レートリミットなしでの壊滅的サービス障害
func catastrophicNoRateLimit() {
    // 🚨 災害例：レートリミットなしでDDoS攻撃により完全サービス停止
    
    http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
        // ❌ 認証試行に制限なし→ブルートフォース攻撃が可能
        username := r.FormValue("username")
        password := r.FormValue("password")
        
        // ❌ データベース照会を無制限実行
        user, err := authenticateUser(username, password)
        if err != nil {
            // 毎回重いクエリが実行される
            log.Printf("Authentication failed for %s from %s", username, r.RemoteAddr)
            http.Error(w, "Invalid credentials", http.StatusUnauthorized)
            return
        }
        
        // ❌ 攻撃者が自動化ツールで毎秒1000回のログイン試行
        // → データベース接続プール枯渇
        // → 正常ユーザーもログイン不可能
        
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "success",
            "token":  generateJWT(user),
        })
    })
    
    http.HandleFunc("/api/data-export", func(w http.ResponseWriter, r *http.Request) {
        // ❌ 重いクエリへの制限なし→リソース枯渇攻撃
        format := r.URL.Query().Get("format")
        
        // ❌ CPUとメモリ集約的な処理を無制限実行
        data, err := exportAllData(format) // 100GBのデータ処理
        if err != nil {
            http.Error(w, "Export failed", http.StatusInternalServerError)
            return
        }
        
        // ❌ 攻撃者が同時に50個の並列エクスポート実行
        // → CPU使用率100%、メモリ枯渇
        // → サーバークラッシュ、全サービス停止
        
        w.Header().Set("Content-Type", "application/octet-stream")
        w.Write(data)
    })
    
    http.HandleFunc("/api/send-email", func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            To      string `json:"to"`
            Subject string `json:"subject"`
            Body    string `json:"body"`
        }
        
        json.NewDecoder(r.Body).Decode(&req)
        
        // ❌ メール送信に制限なし→スパム攻撃
        err := sendEmail(req.To, req.Subject, req.Body)
        if err != nil {
            http.Error(w, "Email send failed", http.StatusInternalServerError)
            return
        }
        
        // ❌ 攻撃者が毎分10000通のスパムメール送信
        // → メールサービスプロバイダーからブラックリスト登録
        // → 正常業務メールも送信不可能
        // → 顧客への重要通知が届かない
        
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
    })
    
    // 【攻撃シナリオ】協調分散攻撃による完全サービス停止
    // 1. ボットネット（10000台）が同時攻撃開始
    // 2. 毎秒100万リクエストでサーバー負荷急上昇
    // 3. データベース接続プール完全枯渇
    // 4. メモリとCPU使用率100%継続
    // 5. 正常ユーザー完全アクセス不可
    // 6. 売上ゼロ、顧客離れ、事業停止
    
    log.Println("❌ Starting server WITHOUT rate limiting...")
    http.ListenAndServe(":8080", nil)
    // 結果：数分でサービス完全停止、事業継続不可能、競合他社に顧客流出
}

// ✅ 正解：エンタープライズ級レートリミットシステム
type EnterpriseRateLimiterSystem struct {
    // 【基本機能】
    algorithms      map[AlgorithmType]RateLimitAlgorithm // 複数アルゴリズム対応
    ipStorage       *DistributedIPStorage                // 分散IP管理
    configManager   *DynamicConfigManager                // 動的設定管理
    
    // 【高度な制御】
    adaptiveEngine  *AdaptiveRateEngine                  // 適応的レート調整
    geolocationAPI  *GeolocationAPI                      // 地理的位置制御
    behaviorAnalyzer *BehaviorAnalyzer                   // 行動パターン分析
    
    // 【攻撃対策】
    ddosProtector   *DDoSProtector                       // DDoS攻撃検知・防御
    botDetector     *BotDetector                         // ボット検知
    vpnDetector     *VPNDetector                         // VPN/プロキシ検知
    
    // 【管理・監視】
    metrics         *RateLimitMetrics                    // 詳細メトリクス
    alertManager    *AlertManager                        // アラート管理
    auditLogger     *AuditLogger                         // 監査ログ
    
    // 【性能最適化】
    cacheLayer      *CacheLayer                          // キャッシュ層
    loadBalancer    *LoadBalancer                        // 負荷分散
    
    // 【ホワイトリスト/ブラックリスト】
    whitelistManager *WhitelistManager                   // ホワイトリスト管理
    blacklistManager *BlacklistManager                   // ブラックリスト管理
    graylistManager  *GraylistManager                    // グレーリスト管理
    
    // 【分散・冗長化】
    redisCluster    *RedisCluster                        // Redis分散クラスター
    nodeCoordinator *NodeCoordinator                     // ノード間協調
    
    mu              sync.RWMutex                         // 設定変更保護
}

// 【重要関数】エンタープライズレートリミッター初期化
func NewEnterpriseRateLimiterSystem(config *RateLimiterConfig) *EnterpriseRateLimiterSystem {
    system := &EnterpriseRateLimiterSystem{
        algorithms: map[AlgorithmType]RateLimitAlgorithm{
            TokenBucket:    NewTokenBucketAlgorithm(config.TokenBucket),
            SlidingWindow:  NewSlidingWindowAlgorithm(config.SlidingWindow),
            FixedWindow:    NewFixedWindowAlgorithm(config.FixedWindow),
            LeakyBucket:    NewLeakyBucketAlgorithm(config.LeakyBucket),
        },
        ipStorage:        NewDistributedIPStorage(config.RedisConfig),
        configManager:    NewDynamicConfigManager(config.ConfigSource),
        adaptiveEngine:   NewAdaptiveRateEngine(config.AdaptiveConfig),
        geolocationAPI:   NewGeolocationAPI(config.GeoAPIKey),
        behaviorAnalyzer: NewBehaviorAnalyzer(),
        ddosProtector:    NewDDoSProtector(config.DDoSConfig),
        botDetector:      NewBotDetector(config.BotDetectionConfig),
        vpnDetector:      NewVPNDetector(config.VPNDetectionConfig),
        metrics:          NewRateLimitMetrics(),
        alertManager:     NewAlertManager(config.AlertConfig),
        auditLogger:      NewAuditLogger(config.AuditConfig),
        cacheLayer:       NewCacheLayer(config.CacheConfig),
        loadBalancer:     NewLoadBalancer(config.LoadBalancerConfig),
        whitelistManager: NewWhitelistManager(config.WhitelistRules),
        blacklistManager: NewBlacklistManager(config.BlacklistSources),
        graylistManager:  NewGraylistManager(),
        redisCluster:     NewRedisCluster(config.RedisClusterConfig),
        nodeCoordinator:  NewNodeCoordinator(config.NodeConfig),
    }
    
    // 【重要】バックグラウンド処理開始
    go system.startAdaptiveAdjustment()
    go system.startBehaviorAnalysis()
    go system.startDDoSMonitoring()
    go system.startGeoBasedUpdates()
    go system.startMetricsCollection()
    
    log.Printf("🛡️  Enterprise rate limiter system initialized")
    log.Printf("   Algorithms: %d types configured", len(system.algorithms))
    log.Printf("   DDoS protection: ENABLED")
    log.Printf("   Adaptive engine: ENABLED")
    log.Printf("   Geographic filtering: ENABLED")
    
    return system
}

// 【核心メソッド】包括的レートリミットミドルウェア
func (system *EnterpriseRateLimiterSystem) ComprehensiveRateLimitMiddleware(
    endpointConfig *EndpointRateLimitConfig,
) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            startTime := time.Now()
            requestID := generateRequestID()
            
            // 【STEP 1】IP アドレス抽出と検証
            clientIP, ipInfo := system.extractAndValidateIP(r)
            
            // 【STEP 2】ブラックリストチェック
            if system.blacklistManager.IsBlacklisted(clientIP) {
                system.metrics.RecordBlacklistedRequest(clientIP)
                system.auditLogger.LogBlacklistedAccess(requestID, clientIP, r)
                http.Error(w, "Access denied", http.StatusForbidden)
                return
            }
            
            // 【STEP 3】ホワイトリストチェック
            if system.whitelistManager.IsWhitelisted(clientIP) {
                system.metrics.RecordWhitelistedRequest(clientIP)
                system.auditLogger.LogWhitelistedAccess(requestID, clientIP, r)
                next.ServeHTTP(w, r)
                return
            }
            
            // 【STEP 4】地理的位置制限チェック
            if !system.checkGeographicRestrictions(clientIP, ipInfo, endpointConfig) {
                system.metrics.RecordGeoBlockedRequest(clientIP, ipInfo.Country)
                system.auditLogger.LogGeoBlocked(requestID, clientIP, ipInfo.Country, r)
                http.Error(w, "Geographic access restricted", http.StatusForbidden)
                return
            }
            
            // 【STEP 5】VPN/プロキシ検知
            if endpointConfig.BlockVPN && system.vpnDetector.IsVPN(clientIP) {
                system.metrics.RecordVPNBlockedRequest(clientIP)
                system.auditLogger.LogVPNBlocked(requestID, clientIP, r)
                http.Error(w, "VPN/Proxy access not allowed", http.StatusForbidden)
                return
            }
            
            // 【STEP 6】ボット検知
            if system.botDetector.IsBot(r, ipInfo) {
                system.metrics.RecordBotBlockedRequest(clientIP)
                system.auditLogger.LogBotBlocked(requestID, clientIP, r)
                
                // CAPTCHAチャレンジ
                if endpointConfig.RequireCAPTCHA {
                    system.sendCAPTCHAChallenge(w, clientIP)
                    return
                }
                
                http.Error(w, "Bot access detected", http.StatusForbidden)
                return
            }
            
            // 【STEP 7】行動パターン分析
            behaviorScore := system.behaviorAnalyzer.AnalyzeBehavior(clientIP, r)
            if behaviorScore > endpointConfig.SuspiciousThreshold {
                system.graylistManager.AddToGraylist(clientIP, behaviorScore)
                system.auditLogger.LogSuspiciousBehavior(requestID, clientIP, behaviorScore, r)
            }
            
            // 【STEP 8】DDoS攻撃検知
            if system.ddosProtector.IsUnderAttack(clientIP, r) {
                system.metrics.RecordDDoSAttack(clientIP)
                system.alertManager.TriggerDDoSAlert(clientIP, r)
                http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
                return
            }
            
            // 【STEP 9】適応的レート制限の適用
            adaptiveConfig := system.adaptiveEngine.GetAdaptiveConfig(clientIP, endpointConfig)
            
            // 【STEP 10】複数アルゴリズムによるレート制限判定
            rateLimitResult := system.checkRateLimits(clientIP, adaptiveConfig, r)
            
            if !rateLimitResult.Allowed {
                system.handleRateLimitExceeded(w, r, clientIP, rateLimitResult, requestID)
                return
            }
            
            // 【STEP 11】リクエストの実行
            system.recordRequestExecution(clientIP, endpointConfig)
            
            // レスポンスヘッダー設定
            system.setRateLimitHeaders(w, rateLimitResult)
            
            next.ServeHTTP(w, r)
            
            // 【STEP 12】完了後処理
            system.recordRequestCompletion(clientIP, time.Since(startTime), endpointConfig)
        })
    }
}

// 【重要メソッド】IP アドレス抽出と検証
func (system *EnterpriseRateLimiterSystem) extractAndValidateIP(r *http.Request) (string, *IPInfo) {
    var clientIP string
    
    // 【信頼できるプロキシヘッダーの優先順位チェック】
    trustedHeaders := []string{
        "CF-Connecting-IP",      // Cloudflare
        "X-Forwarded-For",       // 標準プロキシヘッダー
        "X-Real-IP",             // nginx標準
        "X-Client-IP",           // Apache標準
        "X-Forwarded",           // RFC 7239
        "Forwarded-For",         // 旧式
        "Forwarded",             // RFC 7239
    }
    
    for _, header := range trustedHeaders {
        if value := r.Header.Get(header); value != "" {
            // 複数IPの場合は最初のIPを使用
            ips := strings.Split(value, ",")
            for _, ip := range ips {
                cleanIP := strings.TrimSpace(ip)
                if system.isValidPublicIP(cleanIP) {
                    clientIP = cleanIP
                    break
                }
            }
            if clientIP != "" {
                break
            }
        }
    }
    
    // フォールバック：RemoteAddr
    if clientIP == "" {
        host, _, err := net.SplitHostPort(r.RemoteAddr)
        if err == nil {
            clientIP = host
        } else {
            clientIP = r.RemoteAddr
        }
    }
    
    // IP情報の取得
    ipInfo := system.geolocationAPI.GetIPInfo(clientIP)
    
    return clientIP, ipInfo
}

// 【重要メソッド】複数アルゴリズムによるレート制限チェック
func (system *EnterpriseRateLimiterSystem) checkRateLimits(
    clientIP string,
    config *AdaptiveRateLimitConfig,
    r *http.Request,
) *RateLimitResult {
    
    // 【アルゴリズム別チェック】
    results := make(map[AlgorithmType]*AlgorithmResult)
    
    for algType, algorithm := range system.algorithms {
        if config.EnabledAlgorithms[algType] {
            result := algorithm.CheckLimit(clientIP, config.Limits[algType], r)
            results[algType] = result
        }
    }
    
    // 【複合判定】最も厳しい制限を採用
    finalResult := &RateLimitResult{
        Allowed:   true,
        Algorithm: "composite",
    }
    
    var minRemaining int64 = math.MaxInt64
    var maxRetryAfter time.Duration
    
    for algType, result := range results {
        if !result.Allowed {
            finalResult.Allowed = false
            finalResult.RejectedBy = append(finalResult.RejectedBy, algType)
        }
        
        if result.Remaining < minRemaining {
            minRemaining = result.Remaining
            finalResult.Remaining = result.Remaining
            finalResult.Limit = result.Limit
            finalResult.Algorithm = string(algType)
        }
        
        if result.RetryAfter > maxRetryAfter {
            maxRetryAfter = result.RetryAfter
            finalResult.RetryAfter = result.RetryAfter
        }
    }
    
    return finalResult
}

// 【重要メソッド】レート制限超過時の処理
func (system *EnterpriseRateLimiterSystem) handleRateLimitExceeded(
    w http.ResponseWriter,
    r *http.Request,
    clientIP string,
    result *RateLimitResult,
    requestID string,
) {
    // メトリクス記録
    system.metrics.RecordRateLimitExceeded(clientIP, result.Algorithm)
    
    // 監査ログ
    system.auditLogger.LogRateLimitExceeded(requestID, clientIP, result, r)
    
    // 段階的制裁措置
    violations := system.getViolationCount(clientIP)
    
    switch {
    case violations >= 100:
        // 重度違反：長期ブラックリスト
        system.blacklistManager.AddToBlacklist(clientIP, 24*time.Hour, "repeated_violations")
        system.alertManager.TriggerSevereViolationAlert(clientIP, violations)
        
    case violations >= 20:
        // 中度違反：一時的ブラックリスト
        system.blacklistManager.AddToBlacklist(clientIP, 1*time.Hour, "moderate_violations")
        
    case violations >= 5:
        // 軽度違反：グレーリスト
        system.graylistManager.AddToGraylist(clientIP, violations)
    }
    
    // レスポンスヘッダー設定
    system.setRateLimitHeaders(w, result)
    w.Header().Set("Retry-After", fmt.Sprintf("%.0f", result.RetryAfter.Seconds()))
    
    // JSON エラーレスポンス
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusTooManyRequests)
    
    errorResponse := map[string]interface{}{
        "error":       "Rate limit exceeded",
        "message":     fmt.Sprintf("Too many requests from IP %s", clientIP),
        "limit":       result.Limit,
        "remaining":   result.Remaining,
        "retry_after": result.RetryAfter.Seconds(),
        "algorithm":   result.Algorithm,
        "request_id":  requestID,
        "timestamp":   time.Now().Unix(),
        "violated_by": result.RejectedBy,
    }
    
    json.NewEncoder(w).Encode(errorResponse)
}

// 【実用例】プロダクション環境での包括的レートリミット
func ProductionRateLimitingUsage() {
    // 【設定】エンタープライズレートリミッター設定
    config := &RateLimiterConfig{
        TokenBucket: &TokenBucketConfig{
            Capacity:    100,
            RefillRate:  10, // 10 tokens/second
            RefillPeriod: time.Second,
        },
        SlidingWindow: &SlidingWindowConfig{
            WindowSize: time.Minute,
            MaxRequests: 60,
        },
        FixedWindow: &FixedWindowConfig{
            WindowSize: time.Minute,
            MaxRequests: 100,
        },
        LeakyBucket: &LeakyBucketConfig{
            Capacity:   50,
            LeakRate:   5, // 5 requests/second
            LeakPeriod: time.Second,
        },
        RedisConfig: &RedisConfig{
            Addresses: []string{
                "redis-cluster-1:6379",
                "redis-cluster-2:6379", 
                "redis-cluster-3:6379",
            },
            Password: getEnv("REDIS_PASSWORD"),
            DB:       0,
        },
        AdaptiveConfig: &AdaptiveConfig{
            Enabled:          true,
            LearningPeriod:   24 * time.Hour,
            AdjustmentFactor: 0.1,
            MinLimit:         10,
            MaxLimit:         1000,
        },
        GeoAPIKey: getEnv("GEOLOCATION_API_KEY"),
        DDoSConfig: &DDoSConfig{
            DetectionThreshold: 1000, // requests/minute
            MitigationDuration: 10 * time.Minute,
            AlertThreshold:     500,
        },
        BotDetectionConfig: &BotDetectionConfig{
            UserAgentChecking: true,
            BehaviorAnalysis:  true,
            ChallengeResponse: true,
        },
        VPNDetectionConfig: &VPNDetectionConfig{
            Enabled:     true,
            DatabaseURL: getEnv("VPN_DB_URL"),
            CacheExpiry: 1 * time.Hour,
        },
        WhitelistRules: []WhitelistRule{
            {CIDR: "10.0.0.0/8", Description: "Internal network"},
            {CIDR: "192.168.0.0/16", Description: "Private network"},
            {CIDR: "172.16.0.0/12", Description: "Docker networks"},
        },
        BlacklistSources: []BlacklistSource{
            {URL: "https://blocklist.example.com/ips.txt", UpdateInterval: time.Hour},
            {URL: "https://tor-exit-nodes.example.com/list.txt", UpdateInterval: 30 * time.Minute},
        },
    }
    
    rateLimiter := NewEnterpriseRateLimiterSystem(config)
    
    // 【ルーター設定】
    mux := http.NewServeMux()
    
    // 【認証エンドポイント】厳格な制限
    authConfig := &EndpointRateLimitConfig{
        RequestsPerMinute:    5,   // 認証は1分に5回まで
        BurstAllowed:        2,   // バースト許可
        BlockVPN:            true, // VPN接続ブロック
        RequireCAPTCHA:      true, // CAPTCHA必須
        SuspiciousThreshold: 0.8,  // 疑わしい行動の閾値
        GeographicRestrictions: []string{"CN", "RU", "KP"}, // 特定国ブロック
        EnabledAlgorithms: map[AlgorithmType]bool{
            TokenBucket:   true,
            SlidingWindow: true,
        },
    }
    
    authHandler := rateLimiter.ComprehensiveRateLimitMiddleware(authConfig)(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 認証処理
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]string{
                "message": "Authentication endpoint",
                "status":  "success",
            })
        }))
    mux.Handle("/api/auth/login", authHandler)
    
    // 【API エンドポイント】標準制限
    apiConfig := &EndpointRateLimitConfig{
        RequestsPerMinute:   60,   // 1分に60回
        BurstAllowed:       10,   // バースト許可
        BlockVPN:           false, // VPN許可
        RequireCAPTCHA:     false,
        SuspiciousThreshold: 0.9,
        EnabledAlgorithms: map[AlgorithmType]bool{
            TokenBucket:   true,
            FixedWindow:   true,
        },
    }
    
    apiHandler := rateLimiter.ComprehensiveRateLimitMiddleware(apiConfig)(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]interface{}{
                "message": "API endpoint accessed",
                "timestamp": time.Now().Unix(),
            })
        }))
    mux.Handle("/api/users", apiHandler)
    
    // 【エクスポートエンドポイント】重い処理用制限
    exportConfig := &EndpointRateLimitConfig{
        RequestsPerMinute:   2,    // 1分に2回のみ
        BurstAllowed:       1,    // バーストなし
        BlockVPN:           true,  // VPN ブロック
        RequireCAPTCHA:     true,  // CAPTCHA必須
        SuspiciousThreshold: 0.5,  // 低い閾値
        GeographicRestrictions: []string{"CN", "RU", "IR", "KP"},
        EnabledAlgorithms: map[AlgorithmType]bool{
            LeakyBucket:   true,
            SlidingWindow: true,
        },
    }
    
    exportHandler := rateLimiter.ComprehensiveRateLimitMiddleware(exportConfig)(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 重い処理のシミュレーション
            time.Sleep(2 * time.Second)
            
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]string{
                "message": "Export completed",
                "status":  "success",
            })
        }))
    mux.Handle("/api/export", exportHandler)
    
    // 【管理エンドポイント】メトリクス
    mux.HandleFunc("/admin/metrics", func(w http.ResponseWriter, r *http.Request) {
        metrics := rateLimiter.metrics.GetDetailedMetrics()
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(metrics)
    })
    
    // 【ヘルスチェック】制限なし
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "status": "healthy",
            "timestamp": time.Now().Format(time.RFC3339),
        })
    })
    
    // 【サーバー起動】
    server := &http.Server{
        Addr:    ":8080",
        Handler: mux,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    log.Printf("🚀 Enterprise rate limiting server starting on :8080")
    log.Printf("   Rate limiting algorithms: %d configured", len(config.algorithms))
    log.Printf("   DDoS protection: %t", config.DDoSConfig.Enabled)
    log.Printf("   Adaptive limiting: %t", config.AdaptiveConfig.Enabled)
    log.Printf("   Geographic filtering: ENABLED")
    log.Printf("   Bot detection: %t", config.BotDetectionConfig.Enabled)
    log.Printf("   VPN detection: %t", config.VPNDetectionConfig.Enabled)
    
    log.Fatal(server.ListenAndServe())
}
```

### レートリミットの重要性

レートリミットは、特定の時間内にクライアントが送信できるリクエストの数を制限するセキュリティ機能です。これにより以下の脅威を防ぐことができます：

- **DDoS攻撃**: 大量のリクエストによるサービス妨害
- **ブルートフォース攻撃**: パスワード総当たり攻撃
- **API乱用**: 過度なAPIコールによるリソース枯渇
- **スクレイピング攻撃**: 大量データ取得の悪用

### Sliding Windowアルゴリズム

レートリミットの実装には複数のアルゴリズムがありますが、今回はSliding Window（滑動窓）方式を使用します：

```go
type SlidingWindow struct {
    mu        sync.Mutex
    requests  []time.Time
    window    time.Duration
    limit     int
}

func (sw *SlidingWindow) Allow() bool {
    sw.mu.Lock()
    defer sw.mu.Unlock()
    
    now := time.Now()
    cutoff := now.Add(-sw.window)
    
    // 期限切れのリクエストを削除
    for len(sw.requests) > 0 && sw.requests[0].Before(cutoff) {
        sw.requests = sw.requests[1:]
    }
    
    // 制限チェック
    if len(sw.requests) >= sw.limit {
        return false
    }
    
    // 新しいリクエストを記録
    sw.requests = append(sw.requests, now)
    return true
}
```

### IPアドレスの取得

リバースプロキシ環境では、実際のクライアントIPは`X-Forwarded-For`や`X-Real-IP`ヘッダーに含まれます：

```go
func getRealIP(r *http.Request) string {
    // プロキシ経由の場合、X-Forwarded-Forを優先
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        ips := strings.Split(xff, ",")
        if len(ips) > 0 {
            return strings.TrimSpace(ips[0])
        }
    }
    
    // X-Real-IPを確認
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }
    
    // 直接接続の場合
    ip, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    
    return ip
}
```

### メモリ効率的なクリーンアップ

時間が経過した古いエントリを定期的に削除してメモリ使用量を制御します：

```go
func (rl *RateLimiter) cleanup() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            rl.mu.Lock()
            now := time.Now()
            for ip, window := range rl.clients {
                window.mu.Lock()
                cutoff := now.Add(-window.window)
                
                // 期限切れリクエストをすべて削除
                newRequests := make([]time.Time, 0)
                for _, req := range window.requests {
                    if !req.Before(cutoff) {
                        newRequests = append(newRequests, req)
                    }
                }
                window.requests = newRequests
                
                // 空になったウィンドウを削除
                if len(window.requests) == 0 {
                    delete(rl.clients, ip)
                }
                
                window.mu.Unlock()
            }
            rl.mu.Unlock()
        case <-rl.done:
            return
        }
    }
}
```

### HTTPレスポンスヘッダー

レートリミット情報をクライアントに通知するための標準的なヘッダー：

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1640995200
Retry-After: 60
```

### ホワイトリスト機能

特定のIPアドレスをレートリミットから除外する機能：

```go
type RateLimiter struct {
    // ... 他のフィールド
    whitelist map[string]bool
}

func (rl *RateLimiter) IsWhitelisted(ip string) bool {
    rl.mu.RLock()
    defer rl.mu.RUnlock()
    return rl.whitelist[ip]
}

func (rl *RateLimiter) AddToWhitelist(ip string) {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    rl.whitelist[ip] = true
}
```

### エンドポイント別の制限設定

異なるエンドポイントに異なる制限を適用：

```go
type EndpointConfig struct {
    RequestsPerMinute int
    Window           time.Duration
}

func (rl *RateLimiter) MiddlewareWithConfig(config EndpointConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := getRealIP(r)
            
            if rl.IsWhitelisted(ip) {
                next.ServeHTTP(w, r)
                return
            }
            
            if !rl.allowWithConfig(ip, config) {
                rl.sendRateLimitResponse(w)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### セキュリティ考慮事項

1. **IP偽装対策**: プロキシ設定の検証
2. **分散レートリミット**: Redis等を使った複数サーバー間での制限
3. **適応的制限**: 攻撃パターンに応じた動的な制限調整
4. **ログ記録**: 制限に達したリクエストのログ
5. **監視**: レートリミット状況のメトリクス収集

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **RateLimiter構造体**
   - IPごとのSlidingWindowを管理
   - 制限値と時間窓の設定
   - ホワイトリスト機能

2. **Sliding Window実装**
   - 時間窓内のリクエスト数カウント
   - 期限切れエントリの自動削除
   - スレッドセーフな操作

3. **ミドルウェア関数**
   - IPアドレスの適切な取得
   - レート制限の判定
   - 適切なHTTPレスポンスの送信

4. **レスポンスヘッダー**
   - X-RateLimit-Limit: 制限値
   - X-RateLimit-Remaining: 残り回数
   - X-RateLimit-Reset: リセット時刻
   - Retry-After: 再試行可能時間

5. **管理機能**
   - ホワイトリストへの追加/削除
   - 制限設定の動的変更
   - メモリクリーンアップ

6. **エラーハンドリング**
   - 429 Too Many Requests応答
   - 適切なJSON形式のエラーメッセージ

## ✅ 期待される挙動

### 成功パターン

#### 制限内のリクエスト：
```bash
curl -v http://localhost:8080/api
```
```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1640995260
Content-Type: application/json

{
  "message": "Request successful",
  "timestamp": "2023-12-31T23:59:59Z"
}
```

#### ホワイトリストIP：
```bash
curl -H "X-Real-IP: 127.0.0.1" http://localhost:8080/api
```
```json
{
  "message": "Request successful (whitelisted)",
  "ip": "127.0.0.1"
}
```

### エラーパターン

#### レート制限超過（429 Too Many Requests）：
```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1640995320
Retry-After: 60
Content-Type: application/json

{
  "error": "Rate limit exceeded",
  "message": "Too many requests from IP 192.168.1.100",
  "retry_after": 60,
  "limit": 10,
  "window": "1m"
}
```

#### プロキシ経由のIPアドレス：
```bash
curl -H "X-Forwarded-For: 203.0.113.195, 70.41.3.18, 150.172.238.178" http://localhost:8080/api
```
実際のクライアントIP（203.0.113.195）で制限が適用される

## 💡 ヒント

1. **sync.RWMutex**: 読み取り頻度が高い場合の最適化
2. **time.NewTicker**: 定期的なクリーンアップタスク
3. **net.SplitHostPort**: IPアドレスとポートの分離
4. **strings.Split**: X-Forwarded-Forの複数IP処理
5. **HTTP Status 429**: レート制限専用のステータスコード
6. **time.Unix()**: UNIXタイムスタンプでのリセット時刻表現

### レート制限アルゴリズムの選択

```go
// Sliding Window: 正確だがメモリ使用量が多い
type SlidingWindow struct {
    requests []time.Time
}

// Token Bucket: メモリ効率的で突発的トラフィックに対応
type TokenBucket struct {
    tokens     float64
    lastRefill time.Time
}

// Fixed Window: 実装が簡単だが境界問題あり
type FixedWindow struct {
    count  int
    window time.Time
}
```

### プロダクション考慮事項

```go
// Redis を使った分散レートリミット
func (rl *RateLimiter) checkRedisLimit(ip string) bool {
    key := fmt.Sprintf("rate_limit:%s", ip)
    count, err := rl.redis.Incr(key).Result()
    if err != nil {
        return true // フェイルオープン
    }
    
    if count == 1 {
        rl.redis.Expire(key, rl.window)
    }
    
    return count <= int64(rl.limit)
}
```

### テスト戦略

- 並行リクエストでの競合状態テスト
- 時間境界でのウィンドウ動作テスト
- メモリリーク確認のための長時間テスト
- 異なるIPからの同時アクセステスト

これらの実装により、プロダクション環境で使用できる堅牢なレートリミットミドルウェアの基礎を学ぶことができます。