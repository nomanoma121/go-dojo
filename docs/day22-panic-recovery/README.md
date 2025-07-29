# Day 22: パニックリカバリミドルウェア

🎯 **本日の目標**
ハンドラ内で発生したパニックを捕捉し、アプリケーションクラッシュを防ぐリカバリミドルウェアを実装し、安定性の高いWebアプリケーションの構築方法を学ぶ。

## 📖 解説

### パニックリカバリミドルウェアの重要性

```go
// 【パニックリカバリミドルウェアの重要性】システム全体崩壊防止とサービス継続性確保
// ❌ 問題例：パニック処理なしでの壊滅的システム障害
func catastrophicNoPanicRecovery() {
    // 🚨 災害例：ハンドラでパニック発生→サービス全停止
    
    http.HandleFunc("/api/calculate", func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Dividend int `json:"dividend"`
            Divisor  int `json:"divisor"`
        }
        
        json.NewDecoder(r.Body).Decode(&req)
        
        // ❌ ゼロ除算でパニック発生（リカバリなし）
        result := req.Dividend / req.Divisor // panic: runtime error: integer divide by zero
        
        // この行は実行されない（パニックで即座に停止）
        json.NewEncoder(w).Encode(map[string]int{"result": result})
        // ❌ ここでプロセス全体が終了→全ユーザーに影響
    })
    
    http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        userID := r.URL.Query().Get("id")
        
        // ❌ 不正なuser ID→データベースアクセスでパニック
        user := getUserByID(userID) // panic: invalid user ID format
        
        // ❌ 1つのリクエストのパニックで全サービス停止
        json.NewEncoder(w).Encode(user)
    })
    
    http.HandleFunc("/api/orders", func(w http.ResponseWriter, r *http.Request) {
        var orders []Order
        
        // ❌ メモリ不足、配列アクセス違反等でパニック多発
        for i := 0; i < 1000000; i++ {
            orders = append(orders, Order{ID: i})
        }
        
        // ❌ スライス範囲外アクセス
        lastOrder := orders[len(orders)+1] // panic: index out of range
        
        json.NewEncoder(w).Encode(lastOrder)
        // 結果：1つの不正リクエストで全システムダウン
    })
    
    // 【災害シナリオ】連鎖的システム障害
    // 1. 1人のユーザーが不正データでAPIアクセス
    // 2. ハンドラでパニック発生→プロセス終了
    // 3. 全サービス停止→数千人のユーザーに影響
    // 4. データベース接続プール破綻→復旧困難
    // 5. 売上損失、信用失墜、SLA違反
    
    log.Println("❌ Starting server WITHOUT panic recovery...")
    http.ListenAndServe(":8080", nil)
    // 結果：1つのパニックで全システム停止、業務継続不可能
    
    // 【実際の被害例】
    // - EC サイト：決済処理中のパニックで全注文停止
    // - 銀行API：残高照会パニックでATM全停止
    // - 配送システム：配送追跡パニックで物流麻痺
    // - 医療システム：患者情報アクセスパニックで診療停止
}

// ✅ 正解：エンタープライズ級パニックリカバリシステム
type EnterpriseRecoverySystem struct {
    // 【基本機能】
    logger          *slog.Logger            // 構造化ログ
    stackTraceEnabled bool                  // スタックトレース有効化
    
    // 【高度な機能】
    panicAnalyzer   *PanicAnalyzer          // パニック分析エンジン
    alertManager    *AlertManager           // アラート管理
    circuitBreaker  *CircuitBreaker         // サーキットブレーカー
    
    // 【障害対策】
    fallbackHandler *FallbackHandler        // フォールバック処理
    retryManager    *RetryManager           // リトライ管理
    bulkheadPattern *BulkheadPattern        // バルクヘッドパターン
    
    // 【監視・メトリクス】 
    metrics         *PanicMetrics           // パニックメトリクス
    healthChecker   *HealthChecker          // ヘルスチェック
    
    // 【セキュリティ】
    sanitizer       *PanicSanitizer         // パニック情報サニタイズ
    rateLimiter     *PanicRateLimiter       // パニック頻度制限
    
    // 【復旧機能】
    autoRecovery    *AutoRecoveryManager    // 自動復旧管理
    resourceMonitor *ResourceMonitor        // リソース監視
    
    // 【分散対応】
    nodeCoordinator *NodeCoordinator        // ノード協調機能
    stateReplicator *StateReplicator        // 状態複製
    
    mu              sync.RWMutex            // 設定変更保護
}

// 【重要関数】エンタープライズリカバリシステム初期化
func NewEnterpriseRecoverySystem(config *RecoveryConfig) *EnterpriseRecoverySystem {
    recovery := &EnterpriseRecoverySystem{
        logger:          slog.New(slog.NewJSONHandler(os.Stdout, nil)),
        stackTraceEnabled: config.StackTraceEnabled,
        panicAnalyzer:   NewPanicAnalyzer(),
        alertManager:    NewAlertManager(config.AlertConfig),
        circuitBreaker:  NewCircuitBreaker(config.CircuitBreakerConfig),
        fallbackHandler: NewFallbackHandler(config.FallbackConfig),
        retryManager:    NewRetryManager(config.RetryConfig),
        bulkheadPattern: NewBulkheadPattern(config.BulkheadConfig),
        metrics:         NewPanicMetrics(),
        healthChecker:   NewHealthChecker(),
        sanitizer:       NewPanicSanitizer(),
        rateLimiter:     NewPanicRateLimiter(config.RateLimit),
        autoRecovery:    NewAutoRecoveryManager(),
        resourceMonitor: NewResourceMonitor(),
        nodeCoordinator: NewNodeCoordinator(config.NodeConfig),
        stateReplicator: NewStateReplicator(config.ReplicationConfig),
    }
    
    // 【重要】バックグラウンド監視開始
    go recovery.startPanicAnalysis()
    go recovery.startHealthMonitoring()
    go recovery.startAutoRecovery()
    go recovery.startResourceMonitoring()
    
    recovery.logger.Info("Enterprise panic recovery system initialized",
        "stack_trace_enabled", config.StackTraceEnabled,
        "circuit_breaker_enabled", config.CircuitBreakerConfig.Enabled,
        "auto_recovery_enabled", config.AutoRecoveryEnabled)
    
    return recovery
}

// 【核心メソッド】包括的パニックリカバリミドルウェア
func (recovery *EnterpriseRecoverySystem) ComprehensivePanicRecovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        startTime := time.Now()
        requestID := getRequestID(r.Context())
        clientIP := getClientIP(r)
        endpoint := r.URL.Path
        
        // 【STEP 1】事前チェック（サーキットブレーカー）
        if recovery.circuitBreaker.IsOpen(endpoint) {
            recovery.logger.Warn("Circuit breaker is open",
                "endpoint", endpoint,
                "request_id", requestID)
            
            // フォールバック処理
            recovery.fallbackHandler.HandleFallback(w, r, "service_unavailable")
            return
        }
        
        // 【STEP 2】パニックリカバリの設定
        defer func() {
            if panicValue := recover(); panicValue != nil {
                duration := time.Since(startTime)
                
                // 【重要】包括的パニック処理
                recovery.handlePanicComprehensively(
                    w, r, panicValue, requestID, clientIP, endpoint, duration)
            }
        }()
        
        // 【STEP 3】リソース監視
        recovery.resourceMonitor.CheckResourceLimits(r)
        
        // 【STEP 4】リクエスト実行
        next.ServeHTTP(w, r)
        
        // 【STEP 5】成功時処理
        duration := time.Since(startTime)
        recovery.circuitBreaker.RecordSuccess(endpoint)
        recovery.metrics.RecordSuccessfulRequest(endpoint, duration)
    })
}

// 【重要メソッド】包括的パニック處理
func (recovery *EnterpriseRecoverySystem) handlePanicComprehensively(
    w http.ResponseWriter,
    r *http.Request,
    panicValue interface{},
    requestID, clientIP, endpoint string,
    duration time.Duration,
) {
    // 【STEP 1】パニック情報の構造化
    panicInfo := recovery.panicAnalyzer.AnalyzePanic(panicValue, r)
    
    // 【STEP 2】スタックトレース取得
    stackTrace := string(debug.Stack())
    
    // 【STEP 3】パニック分類と重要度判定
    severity := recovery.panicAnalyzer.ClassifySeverity(panicInfo)
    category := recovery.panicAnalyzer.CategorizePanic(panicInfo)
    
    // 【STEP 4】レート制限チェック
    if !recovery.rateLimiter.AllowPanic(clientIP, category) {
        recovery.logger.Warn("Panic rate limit exceeded",
            "client_ip", clientIP,
            "category", category,
            "request_id", requestID)
        
        http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
        return
    }
    
    // 【STEP 5】構造化ログ出力
    logFields := []slog.Attr{
        slog.String("request_id", requestID),
        slog.String("client_ip", clientIP),
        slog.String("endpoint", endpoint),
        slog.String("method", r.Method),
        slog.String("user_agent", r.UserAgent()),
        slog.String("panic_category", string(category)),
        slog.String("severity", string(severity)),
        slog.Duration("duration", duration),
        slog.String("panic_type", fmt.Sprintf("%T", panicValue)),
    }
    
    // パニック内容のサニタイズ
    sanitizedMessage := recovery.sanitizer.SanitizePanicMessage(panicValue)
    logFields = append(logFields, slog.String("panic_message", sanitizedMessage))
    
    // スタックトレース（設定に応じて）
    if recovery.stackTraceEnabled {
        logFields = append(logFields, slog.String("stack_trace", stackTrace))
    }
    
    recovery.logger.Error("Panic recovered", logFields...)
    
    // 【STEP 6】メトリクス記録
    recovery.metrics.RecordPanic(endpoint, category, severity, duration)
    
    // 【STEP 7】サーキットブレーカー更新
    recovery.circuitBreaker.RecordFailure(endpoint)
    
    // 【STEP 8】アラート送信（重要度に応じて）
    if severity >= SeverityHigh {
        recovery.alertManager.SendPanicAlert(AlertInfo{
            RequestID:    requestID,
            Endpoint:     endpoint,
            Category:     category,
            Severity:     severity,
            Message:      sanitizedMessage,
            ClientIP:     clientIP,
            Timestamp:    time.Now(),
        })
    }
    
    // 【STEP 9】自動復旧処理の開始（必要に応じて）
    if severity >= SeverityCritical {
        go recovery.autoRecovery.TriggerRecoveryProcess(endpoint, panicInfo)
    }
    
    // 【STEP 10】ノード間での状態共有
    recovery.nodeCoordinator.NotifyPanicEvent(endpoint, category, severity)
    
    // 【STEP 11】フォールバック処理
    if recovery.fallbackHandler.HasFallback(endpoint) {
        recovery.fallbackHandler.HandleFallback(w, r, "panic_recovery")
        return
    }
    
    // 【STEP 12】統一エラーレスポンス
    recovery.sendSecureErrorResponse(w, requestID, severity)
}

// 【重要メソッド】セキュアなエラーレスポンス送信
func (recovery *EnterpriseRecoverySystem) sendSecureErrorResponse(
    w http.ResponseWriter,
    requestID string,
    severity PanicSeverity,
) {
    // レスポンスヘッダー設定
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Request-ID", requestID)
    
    // ステータスコード決定
    statusCode := http.StatusInternalServerError
    if severity == SeverityLow {
        statusCode = http.StatusBadRequest
    }
    w.WriteHeader(statusCode)
    
    // セキュアなエラーレスポンス
    errorResponse := map[string]interface{}{
        "error": http.StatusText(statusCode),
        "message": "An internal error occurred. Please try again later.",
        "request_id": requestID,
        "timestamp": time.Now().Unix(),
        "support_info": map[string]string{
            "contact": "support@company.com",
            "documentation": "https://docs.company.com/errors",
        },
    }
    
    json.NewEncoder(w).Encode(errorResponse)
}

// 【高度な機能】パニック分析エンジン
type PanicAnalyzer struct {
    patterns []PanicPattern
    ml       *MachineLearningEngine
}

func (pa *PanicAnalyzer) AnalyzePanic(panicValue interface{}, r *http.Request) *PanicInfo {
    return &PanicInfo{
        Value:       panicValue,
        Type:        fmt.Sprintf("%T", panicValue),
        Message:     pa.extractMessage(panicValue),
        Endpoint:    r.URL.Path,
        Method:      r.Method,
        Headers:     r.Header,
        Timestamp:   time.Now(),
        Fingerprint: pa.generateFingerprint(panicValue, r),
    }
}

func (pa *PanicAnalyzer) ClassifySeverity(panicInfo *PanicInfo) PanicSeverity {
    // 機械学習による重要度分類
    if pa.ml != nil {
        return pa.ml.PredictSeverity(panicInfo)
    }
    
    // ルールベース分類
    message := strings.ToLower(panicInfo.Message)
    
    switch {
    case strings.Contains(message, "out of memory"):
        return SeverityCritical
    case strings.Contains(message, "database"):
        return SeverityHigh
    case strings.Contains(message, "index out of range"):
        return SeverityMedium
    case strings.Contains(message, "nil pointer"):
        return SeverityMedium
    default:
        return SeverityLow
    }
}

func (pa *PanicAnalyzer) CategorizePanic(panicInfo *PanicInfo) PanicCategory {
    message := strings.ToLower(panicInfo.Message)
    
    switch {
    case strings.Contains(message, "index out of range"):
        return CategoryIndexOutOfRange
    case strings.Contains(message, "nil pointer"):
        return CategoryNilPointer
    case strings.Contains(message, "divide by zero"):
        return CategoryDivisionByZero
    case strings.Contains(message, "type assertion"):
        return CategoryTypeAssertion
    case strings.Contains(message, "channel"):
        return CategoryChannelOperation
    case strings.Contains(message, "memory"):
        return CategoryMemoryError
    case strings.Contains(message, "database"):
        return CategoryDatabaseError
    default:
        return CategoryUnknown
    }
}

// 【実用例】プロダクション環境でのパニックリカバリ
func ProductionPanicRecoveryUsage() {
    // 【設定】エンタープライズリカバリ設定
    config := &RecoveryConfig{
        StackTraceEnabled: getEnvBool("ENABLE_STACK_TRACE", true),
        AlertConfig: &AlertConfig{
            SlackWebhookURL:  getEnv("SLACK_WEBHOOK_URL"),
            EmailRecipients:  []string{"oncall@company.com"},
            SMSNumbers:       []string{"+1234567890"},
        },
        CircuitBreakerConfig: &CircuitBreakerConfig{
            Enabled:         true,
            FailureThreshold: 5,
            RecoveryTimeout:  30 * time.Second,
            HalfOpenMax:     3,
        },
        FallbackConfig: &FallbackConfig{
            DefaultMessage: "Service temporarily unavailable",
            CacheEnabled:   true,
            CacheTTL:      5 * time.Minute,
        },
        RetryConfig: &RetryConfig{
            MaxRetries:    3,
            BackoffFactor: 2.0,
            InitialDelay:  100 * time.Millisecond,
        },
        BulkheadConfig: &BulkheadConfig{
            MaxConcurrentRequests: 100,
            QueueSize:            200,
            Timeout:              30 * time.Second,
        },
        RateLimit: &RateLimitConfig{
            MaxPanicsPerIP:    10,
            TimeWindow:        time.Hour,
            BlockDuration:     15 * time.Minute,
        },
        AutoRecoveryEnabled: true,
        NodeConfig: &NodeConfig{
            NodeID:      getEnv("NODE_ID"),
            ClusterName: getEnv("CLUSTER_NAME"),
        },
        ReplicationConfig: &ReplicationConfig{
            ReplicationFactor: 3,
            SyncTimeout:      5 * time.Second,
        },
    }
    
    recovery := NewEnterpriseRecoverySystem(config)
    
    // 【ルーター設定】
    mux := http.NewServeMux()
    
    // 【危険な計算エンドポイント】
    mux.HandleFunc("/api/calculate", func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Dividend int `json:"dividend"`
            Divisor  int `json:"divisor"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        // 意図的にパニックを発生させる可能性
        if req.Divisor == 0 {
            panic("division by zero detected")
        }
        
        result := req.Dividend / req.Divisor
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]int{"result": result})
    })
    
    // 【配列アクセステスト】
    mux.HandleFunc("/api/array-access", func(w http.ResponseWriter, r *http.Request) {
        data := []int{1, 2, 3, 4, 5}
        
        indexStr := r.URL.Query().Get("index")
        index, err := strconv.Atoi(indexStr)
        if err != nil {
            panic(fmt.Sprintf("invalid index: %s", indexStr))
        }
        
        // 意図的に範囲外アクセス
        value := data[index] // パニックの可能性
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]int{"value": value})
    })
    
    // 【メモリ集約的処理】
    mux.HandleFunc("/api/memory-intensive", func(w http.ResponseWriter, r *http.Request) {
        sizeStr := r.URL.Query().Get("size")
        size, err := strconv.Atoi(sizeStr)
        if err != nil {
            size = 1000
        }
        
        // 大量メモリ確保でパニックの可能性
        data := make([]byte, size*1024*1024) // MB単位
        
        // メモリ使用量情報
        var memStats runtime.MemStats
        runtime.ReadMemStats(&memStats)
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "allocated_mb": len(data) / 1024 / 1024,
            "system_memory_mb": memStats.Sys / 1024 / 1024,
        })
    })
    
    // 【ヘルスチェックエンドポイント】
    mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
        healthStatus := recovery.healthChecker.GetHealthStatus()
        
        w.Header().Set("Content-Type", "application/json")
        if healthStatus.Status == "healthy" {
            w.WriteHeader(http.StatusOK)
        } else {
            w.WriteHeader(http.StatusServiceUnavailable)
        }
        
        json.NewEncoder(w).Encode(healthStatus)
    })
    
    // 【メトリクスエンドポイント】
    mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
        metrics := recovery.metrics.GetMetricsSummary()
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(metrics)
    })
    
    // 【ミドルウェア適用】
    handler := recovery.ComprehensivePanicRecovery(mux)
    
    // 【サーバー起動】
    server := &http.Server{
        Addr:    ":8080",
        Handler: handler,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    log.Printf("🚀 Enterprise panic recovery server starting on :8080")
    log.Printf("   Panic recovery: ENABLED")
    log.Printf("   Circuit breaker: %t", config.CircuitBreakerConfig.Enabled)
    log.Printf("   Auto recovery: %t", config.AutoRecoveryEnabled)
    log.Printf("   Stack trace logging: %t", config.StackTraceEnabled)
    
    log.Fatal(server.ListenAndServe())
}
```

### パニックとは

Goにおけるパニックは、プログラムが回復不可能なエラー状態に陥った際に発生する実行時エラーです。パニックが発生すると、通常はプログラム全体が停止してしまいます。

```go
// パニックを発生させる例
func riskyFunction() {
    panic("Something went wrong!")
}

// 配列の範囲外アクセスもパニックの原因
func outOfBounds() {
    slice := []int{1, 2, 3}
    _ = slice[10] // panic: runtime error: index out of range
}
```

### recover()によるパニック捕捉

Goの`recover()`関数を使用することで、パニックを捕捉し、プログラムの実行を継続できます：

```go
func safeFunction() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Printf("Recovered from panic: %v\n", r)
        }
    }()
    
    panic("This will be caught!")
    fmt.Println("This won't be printed")
}
```

### HTTPミドルウェアでのパニックリカバリ

Webアプリケーションでは、個々のHTTPリクエストでパニックが発生しても、サーバー全体が停止しないようにすることが重要です：

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                // パニックをログに記録
                log.Printf("Panic recovered: %v", err)
                
                // クライアントに500エラーを返す
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

### スタックトレースの取得

デバッグのために、パニック発生時のスタックトレースを記録することが重要です：

```go
import (
    "runtime/debug"
)

func RecoveryWithStackTrace(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                // スタックトレースを取得
                stack := debug.Stack()
                
                log.Printf("Panic recovered: %v\nStack trace:\n%s", err, stack)
                
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

### 構造化ログでのパニック記録

`slog`を使用して、パニック情報を構造化形式で記録：

```go
func (rm *RecoveryMiddleware) Recover(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                requestID := getRequestID(r.Context())
                
                rm.logger.Error("panic recovered",
                    "error", err,
                    "request_id", requestID,
                    "method", r.Method,
                    "url", r.URL.String(),
                    "user_agent", r.UserAgent(),
                    "stack_trace", string(debug.Stack()),
                )
                
                // JSONエラーレスポンス
                rm.sendErrorResponse(w, http.StatusInternalServerError, "Internal Server Error")
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

### エラーレスポンスの統一

パニック発生時のエラーレスポンスを統一的に処理：

```go
type ErrorResponse struct {
    Error     string `json:"error"`
    Message   string `json:"message"`
    Timestamp int64  `json:"timestamp"`
    RequestID string `json:"request_id,omitempty"`
}

func (rm *RecoveryMiddleware) sendErrorResponse(w http.ResponseWriter, code int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    
    response := ErrorResponse{
        Error:     http.StatusText(code),
        Message:   message,
        Timestamp: time.Now().Unix(),
    }
    
    json.NewEncoder(w).Encode(response)
}
```

### パニック発生パターンの分類

一般的なパニック発生パターンと対策：

#### 1. Null Pointer Dereference
```go
var user *User
name := user.Name // panic: runtime error: invalid memory address
```

#### 2. Type Assertion Failed
```go
var val interface{} = "string"
num := val.(int) // panic: interface conversion
```

#### 3. Channel Operations
```go
ch := make(chan int)
close(ch)
ch <- 1 // panic: send on closed channel
```

#### 4. Slice/Map Access
```go
slice := []int{1, 2, 3}
val := slice[10] // panic: index out of range
```

### 本番環境での考慮事項

1. **セキュリティ**: スタックトレースはログにのみ記録し、クライアントには送信しない
2. **監視**: パニック発生率の監視とアラート設定
3. **ログレベル**: パニックは常にERRORレベルでログ記録
4. **メトリクス**: パニック発生回数のメトリクス収集

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、以下の機能を実装してください：

1. **RecoveryMiddleware構造体**
   - 構造化ログ用のloggerを保持
   - 設定可能なオプション

2. **パニックリカバリ機能**
   - defer/recover パターンでパニックを捕捉
   - 500 Internal Server Errorを返却
   - アプリケーションの継続実行を保証

3. **詳細ログ記録**
   - パニック内容の記録
   - リクエスト情報（URL、メソッド、User-Agent）
   - スタックトレースの取得と記録
   - リクエストIDがある場合は含める

4. **エラーレスポンス**
   - JSON形式での統一されたエラーレスポンス
   - タイムスタンプとリクエストIDを含む
   - セキュアな情報のみクライアントに送信

5. **異なるパニックタイプの処理**
   - 文字列パニック
   - error型パニック
   - その他の型のパニック

6. **設定可能な動作**
   - デバッグモードでのスタックトレース表示制御
   - カスタムエラーメッセージ
   - ログレベルの調整

## ✅ 期待される挙動

### パニック発生時のログ出力：
```json
{
  "time": "2024-01-15T10:30:05Z",
  "level": "ERROR",
  "msg": "panic recovered",
  "error": "division by zero",
  "request_id": "req_123456",
  "method": "GET",
  "url": "/api/calculate",
  "user_agent": "curl/7.68.0",
  "stack_trace": "goroutine 1 [running]:\n..."
}
```

### クライアントへのエラーレスポンス：
```json
{
  "error": "Internal Server Error",
  "message": "An internal error occurred",
  "timestamp": 1705317005,
  "request_id": "req_123456"
}
```

### 正常継続の確認：
パニック発生後も他のリクエストが正常に処理されることを確認できます。

## 💡 ヒント

1. **defer文**: 必ずdeferで実行されるrecover処理
2. **runtime/debug.Stack()**: スタックトレースの取得
3. **type assertion**: パニック値の型に応じた処理
4. **slog.Error()**: 構造化エラーログの出力
5. **http.StatusInternalServerError**: 500エラーの定数
6. **json.NewEncoder()**: JSONレスポンスの生成

### パニック処理のベストプラクティス

```go
defer func() {
    if r := recover(); r != nil {
        // 1. パニック値の型を確認
        var err string
        switch v := r.(type) {
        case error:
            err = v.Error()
        case string:
            err = v
        default:
            err = fmt.Sprintf("%v", v)
        }
        
        // 2. 詳細ログ記録
        logger.Error("panic recovered", 
            "error", err,
            "stack", string(debug.Stack()))
        
        // 3. セキュアなレスポンス
        sendErrorResponse(w, 500, "Internal Server Error")
    }
}()
```

### テスト時の注意点

- パニックリカバリのテストでは、実際にパニックを発生させる
- レスポンスコードとレスポンス内容の両方を検証
- ログ出力の内容も検証対象に含める
- 複数のパニックパターンをテスト

これらの実装により、障害に強い本番レベルのWebアプリケーションを構築できます。