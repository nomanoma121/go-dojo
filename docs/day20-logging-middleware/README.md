# Day 20: 構造化ロギングミドルウェア

🎯 **本日の目標**
slogを使用した構造化ロギングミドルウェアを実装し、リクエストID、ユーザー情報、応答時間などの詳細なHTTPアクセスログを出力できるようになる。

## 📖 解説

### 構造化ロギングとは

```go
// 【構造化ロギングの重要性】運用監視とセキュリティインシデント対応の基盤
// ❌ 問題例：非構造化ログによる障害調査の長期化と情報漏洩
func catastrophicUnstructuredLogging() {
    // 🚨 災害例：フラットなテキストログで重大インシデントの原因特定が不可能
    
    http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
        username := r.FormValue("username")
        password := r.FormValue("password")
        userAgent := r.UserAgent()
        clientIP := r.RemoteAddr
        
        // ❌ 非構造化ログ出力（解析不可能）
        log.Printf("Login attempt from %s with agent %s for user %s", clientIP, userAgent, username)
        
        // 認証処理
        if authenticateUser(username, password) {
            // ❌ 重要な成功ログに構造がない
            log.Printf("User %s logged in successfully from %s", username, clientIP)
            
            // ❌ セッション情報がバラバラに散在
            sessionID := generateSessionID()
            log.Printf("Session created: %s", sessionID)
            
            w.WriteHeader(http.StatusOK)
        } else {
            // ❌ セキュリティ違反の詳細が不明
            log.Printf("Failed login for %s from %s", username, clientIP)
            
            // ❌ 攻撃パターンが検知不可能
            // - 複数回失敗 → ブルートフォース攻撃
            // - 異常なUser-Agent → ボット攻撃  
            // - 地理的に分散したIP → 分散攻撃
            // これらの検知が全て困難
            
            w.WriteHeader(http.StatusUnauthorized)
        }
    })
    
    // 【運用時の災害シナリオ】
    // 1. セキュリティインシデント発生時
    //    → ログから攻撃元や手法の特定が困難
    //    → 被害範囲の調査に数日〜数週間
    //    → 対策が遅れて被害拡大
    
    // 2. パフォーマンス問題発生時
    //    → レスポンス時間の傾向分析が不可能
    //    → どのエンドポイントが遅いか不明
    //    → 根本原因の特定に長期間を要する
    
    // 3. 監査・コンプライアンス対応時
    //    → 特定ユーザーの行動追跡が困難
    //    → データアクセスログの抽出が不可能
    //    → 法的要件への対応が不十分
    
    log.Println("❌ Starting server with unstructured logging...")
    http.ListenAndServe(":8080", nil)
    // 結果：セキュリティインシデント対応遅延、コンプライアンス違反、運用効率低下
}

// ✅ 正解：エンタープライズ級構造化ロギングシステム
type EnterpriseLogger struct {
    // 【基本設定】
    baseLogger      *slog.Logger            // slogベースロガー
    level           slog.Level              // ログレベル
    
    // 【高度な機能】
    contextEnricher *ContextEnricher        // コンテキスト情報付加
    formatter       *LogFormatter           // ログフォーマッター
    sampler         *LogSampler             // ログサンプリング
    
    // 【セキュリティ】
    sensitiveFilter *SensitiveDataFilter    // 機密情報マスキング
    anomalyDetector *LogAnomalyDetector     // 異常ログ検知
    
    // 【出力先管理】
    outputs         []LogOutput             // 複数出力先
    failover        *FailoverManager        // 出力先障害時の切り替え
    
    // 【パフォーマンス】
    asyncWriter     *AsyncLogWriter         // 非同期書き込み
    bufferManager   *BufferManager          // バッファ管理
    
    // 【監視・メトリクス】
    metrics         *LoggingMetrics         // ログメトリクス
    healthChecker   *LogHealthChecker       // ログシステム健全性監視
    
    // 【分散トレーシング】
    tracer          *DistributedTracer      // 分散トレーシング統合
    correlationID   *CorrelationIDManager   // リクエスト追跡ID管理
    
    mu              sync.RWMutex            // スレッドセーフティ
}

// 【重要関数】エンタープライズロガー初期化
func NewEnterpriseLogger(config *LoggerConfig) *EnterpriseLogger {
    // slogハンドラー設定
    handlerOpts := &slog.HandlerOptions{
        Level:     config.Level,
        AddSource: config.AddSource,
        ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
            // タイムスタンプをISO8601形式に統一
            if attr.Key == slog.TimeKey {
                return slog.String(slog.TimeKey, time.Now().UTC().Format(time.RFC3339Nano))
            }
            return attr
        },
    }
    
    var handler slog.Handler
    switch config.Format {
    case "json":
        handler = slog.NewJSONHandler(config.Output, handlerOpts)
    case "text":
        handler = slog.NewTextHandler(config.Output, handlerOpts)
    default:
        handler = slog.NewJSONHandler(config.Output, handlerOpts)
    }
    
    logger := &EnterpriseLogger{
        baseLogger:      slog.New(handler),
        level:           config.Level,
        contextEnricher: NewContextEnricher(),
        formatter:       NewLogFormatter(config.Format),
        sampler:         NewLogSampler(config.SamplingRate),
        sensitiveFilter: NewSensitiveDataFilter(),
        anomalyDetector: NewLogAnomalyDetector(),
        outputs:         config.Outputs,
        failover:        NewFailoverManager(config.Outputs),
        asyncWriter:     NewAsyncLogWriter(config.BufferSize),
        bufferManager:   NewBufferManager(config.BufferSize),
        metrics:         NewLoggingMetrics(),
        healthChecker:   NewLogHealthChecker(),
        tracer:          NewDistributedTracer(),
        correlationID:   NewCorrelationIDManager(),
    }
    
    // 【重要】バックグラウンド処理開始
    go logger.startAsyncProcessing()
    go logger.startHealthMonitoring()
    go logger.startMetricsCollection()
    
    logger.Info("Enterprise logger initialized",
        "format", config.Format,
        "level", config.Level,
        "outputs", len(config.Outputs),
        "sampling_rate", config.SamplingRate)
    
    return logger
}

// 【核心メソッド】高度な構造化ログ出力
func (l *EnterpriseLogger) LogWithContext(
    ctx context.Context,
    level slog.Level,
    message string,
    fields ...slog.Attr,
) {
    // 【サンプリングチェック】
    if !l.sampler.ShouldLog(level, message) {
        l.metrics.RecordSampledLog()
        return
    }
    
    // 【コンテキスト情報の付加】
    enrichedFields := l.contextEnricher.EnrichWithContext(ctx, fields...)
    
    // 【機密情報フィルタリング】
    filteredFields := l.sensitiveFilter.FilterSensitiveData(enrichedFields)
    
    // 【分散トレーシング情報付加】
    traceFields := l.tracer.AddTraceContext(ctx, filteredFields)
    
    // 【相関ID付加】
    correlationFields := l.correlationID.AddCorrelationID(ctx, traceFields)
    
    // 【ログエントリ作成】
    logEntry := &StructuredLogEntry{
        Timestamp:     time.Now().UTC(),
        Level:         level,
        Message:       message,
        Fields:        correlationFields,
        Source:        l.getCallerInfo(),
        RequestID:     getRequestIDFromContext(ctx),
        UserID:        getUserIDFromContext(ctx),
        SessionID:     getSessionIDFromContext(ctx),
        ClientIP:      getClientIPFromContext(ctx),
        UserAgent:     getUserAgentFromContext(ctx),
        ServiceName:   getServiceNameFromContext(ctx),
        Version:       getBuildVersionFromContext(ctx),
        Environment:   getEnvironmentFromContext(ctx),
    }
    
    // 【異常検知】
    if l.anomalyDetector.IsAnomalous(logEntry) {
        l.metrics.RecordAnomalousLog()
        // 異常ログの場合はアラート送信
        go l.sendAlert(logEntry, "ANOMALOUS_LOG_DETECTED")
    }
    
    // 【非同期書き込み】
    l.asyncWriter.WriteAsync(logEntry, func(err error) {
        if err != nil {
            l.metrics.RecordWriteError()
            l.failover.HandleWriteFailure(logEntry, err)
        } else {
            l.metrics.RecordSuccessWrite()
        }
    })
}

// 【HTTPミドルウェア】包括的なリクエストログ
func (l *EnterpriseLogger) HTTPLoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        startTime := time.Now()
        
        // 【リクエストID生成】
        requestID := generateRequestID()
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        r = r.WithContext(ctx)
        
        // 【レスポンスライター拡張】
        wrappedWriter := &ResponseWriterWrapper{
            ResponseWriter: w,
            statusCode:     200,  // デフォルト
            responseSize:   0,
        }
        
        // 【リクエスト開始ログ】
        l.LogWithContext(ctx, slog.LevelInfo, "HTTP request started",
            slog.String("method", r.Method),
            slog.String("url", r.URL.String()),
            slog.String("path", r.URL.Path),
            slog.String("query", r.URL.RawQuery),
            slog.String("user_agent", r.UserAgent()),
            slog.String("client_ip", getClientIP(r)),
            slog.String("referer", r.Referer()),
            slog.String("request_id", requestID),
            slog.Int64("content_length", r.ContentLength),
            slog.String("content_type", r.Header.Get("Content-Type")),
            slog.String("accept", r.Header.Get("Accept")),
            slog.String("accept_encoding", r.Header.Get("Accept-Encoding")),
            slog.String("accept_language", r.Header.Get("Accept-Language")),
            slog.Any("headers", sanitizeHeaders(r.Header)),
        )
        
        // 【パニック回復】
        defer func() {
            if recovered := recover(); recovered != nil {
                duration := time.Since(startTime)
                
                l.LogWithContext(ctx, slog.LevelError, "HTTP request panic",
                    slog.String("method", r.Method),
                    slog.String("url", r.URL.String()),
                    slog.String("request_id", requestID),
                    slog.Any("panic", recovered),
                    slog.String("stack_trace", string(debug.Stack())),
                    slog.Duration("duration", duration),
                    slog.Int("status_code", 500),
                )
                
                // パニック時はInternal Server Errorを返す
                if !wrappedWriter.written {
                    wrappedWriter.WriteHeader(http.StatusInternalServerError)
                }
            }
        }()
        
        // 【次のハンドラー実行】
        next.ServeHTTP(wrappedWriter, r)
        
        // 【リクエスト完了ログ】
        duration := time.Since(startTime)
        
        logLevel := slog.LevelInfo
        if wrappedWriter.statusCode >= 400 {
            logLevel = slog.LevelWarn
        }
        if wrappedWriter.statusCode >= 500 {
            logLevel = slog.LevelError
        }
        
        l.LogWithContext(ctx, logLevel, "HTTP request completed",
            slog.String("method", r.Method),
            slog.String("url", r.URL.String()),
            slog.String("path", r.URL.Path),
            slog.String("request_id", requestID),
            slog.Int("status_code", wrappedWriter.statusCode),
            slog.String("status_text", http.StatusText(wrappedWriter.statusCode)),
            slog.Duration("duration", duration),
            slog.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
            slog.Int64("response_size", wrappedWriter.responseSize),
            slog.String("client_ip", getClientIP(r)),
            slog.String("user_agent", r.UserAgent()),
            slog.Float64("requests_per_second", calculateRPS(duration)),
        )
        
        // 【パフォーマンス分析】
        if duration > 1*time.Second {
            l.LogWithContext(ctx, slog.LevelWarn, "Slow HTTP request detected",
                slog.String("method", r.Method),
                slog.String("url", r.URL.String()),
                slog.String("request_id", requestID),
                slog.Duration("duration", duration),
                slog.String("performance_category", categorizePerformance(duration)),
            )
        }
        
        // 【セキュリティ分析】
        if wrappedWriter.statusCode == 401 || wrappedWriter.statusCode == 403 {
            l.LogWithContext(ctx, slog.LevelWarn, "Security event detected",
                slog.String("event_type", "UNAUTHORIZED_ACCESS"),
                slog.String("method", r.Method),
                slog.String("url", r.URL.String()),
                slog.String("request_id", requestID),
                slog.Int("status_code", wrappedWriter.statusCode),
                slog.String("client_ip", getClientIP(r)),
                slog.String("user_agent", r.UserAgent()),
                slog.String("threat_level", assessThreatLevel(r, wrappedWriter.statusCode)),
            )
        }
    })
}

// 【実用例】高度なWebアプリケーションログ
func ProductionWebApplicationLogging() {
    // 【ロガー設定】
    config := &LoggerConfig{
        Level:        slog.LevelInfo,
        Format:       "json",
        Output:       os.Stdout,
        AddSource:    true,
        SamplingRate: 1.0,  // 本番環境では0.1など調整
        BufferSize:   10000,
        Outputs: []LogOutput{
            &FileOutput{Path: "/var/log/app/application.log"},
            &ElasticsearchOutput{URL: "https://elastic:9200"},
            &SyslogOutput{Network: "udp", Address: "syslog:514"},
        },
    }
    
    logger := NewEnterpriseLogger(config)
    
    // 【ルーター設定】
    mux := http.NewServeMux()
    
    // 【ユーザー登録エンドポイント】
    mux.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        
        var user struct {
            Name     string `json:"name"`
            Email    string `json:"email"`
            Password string `json:"password"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
            logger.LogWithContext(ctx, slog.LevelError, "Invalid JSON in registration request",
                slog.String("error", err.Error()),
                slog.String("endpoint", "/api/register"),
            )
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        // 【業務ログ】
        logger.LogWithContext(ctx, slog.LevelInfo, "User registration attempt",
            slog.String("email", maskEmail(user.Email)),
            slog.String("name_length", fmt.Sprintf("%d", len(user.Name))),
            slog.Bool("has_password", user.Password != ""),
        )
        
        // ユーザー作成処理（仮想）
        userID := createUser(user)
        
        // 【成功ログ】
        logger.LogWithContext(ctx, slog.LevelInfo, "User registration successful",
            slog.String("user_id", userID),
            slog.String("email", maskEmail(user.Email)),
        )
        
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status":  "success",
            "user_id": userID,
        })
    })
    
    // 【ミドルウェア適用】
    handler := logger.HTTPLoggingMiddleware(mux)
    
    // 【サーバー起動】
    server := &http.Server{
        Addr:    ":8080",
        Handler: handler,
    }
    
    logger.Info("Production web application starting",
        slog.String("addr", server.Addr),
        slog.String("log_level", config.Level.String()),
        slog.Int("output_count", len(config.Outputs)),
    )
    
    log.Fatal(server.ListenAndServe())
}
```

構造化ロギングは、以下の利点を提供します：

- **検索・フィルタリングが容易**：JSONフィールドで条件検索可能
- **パースが簡単**：ログ分析ツールで自動解析可能  
- **一貫性のある形式**：標準化されたフィールド名とデータ型
- **セキュリティ監視**：異常パターンの自動検知とアラート
- **パフォーマンス分析**：レスポンス時間とスループットの追跡
- **分散トレーシング**：マイクロサービス間のリクエスト追跡

### slog パッケージ

Go 1.21で追加された`log/slog`パッケージは、構造化ロギングの標準ライブラリです：

```go
import "log/slog"

// JSON形式でログ出力
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// 構造化ログの出力
logger.Info("User logged in",
    "user_id", "12345",
    "ip_address", "192.168.1.1",
    "timestamp", time.Now())
```

### HTTPミドルウェアでのロギング

Webアプリケーションでは、すべてのHTTPリクエストのログを統一的に記録することが重要です：

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // リクエスト情報をログ
        slog.Info("request started",
            "method", r.Method,
            "url", r.URL.Path,
            "user_agent", r.UserAgent())
            
        next.ServeHTTP(w, r)
        
        // レスポンス情報をログ
        slog.Info("request completed",
            "method", r.Method,
            "url", r.URL.Path,
            "duration", time.Since(start))
    })
}
```

### リクエストIDの生成と追跡

分散システムでは、単一のリクエストを複数のサービス間で追跡できることが重要です：

```go
func generateRequestID() string {
    bytes := make([]byte, 8)
    rand.Read(bytes)
    return hex.EncodeToString(bytes)
}

func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := generateRequestID()
        
        // Contextに保存
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        r = r.WithContext(ctx)
        
        // レスポンスヘッダーに設定
        w.Header().Set("X-Request-ID", requestID)
        
        next.ServeHTTP(w, r)
    })
}
```

### レスポンスライターのラッピング

HTTPレスポンスの詳細（ステータスコード、レスポンスサイズ）をログに記録するには、`http.ResponseWriter`をラップする必要があります：

```go
type responseWriter struct {
    http.ResponseWriter
    statusCode   int
    bytesWritten int64
}

func (rw *responseWriter) WriteHeader(statusCode int) {
    rw.statusCode = statusCode
    rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
    n, err := rw.ResponseWriter.Write(data)
    rw.bytesWritten += int64(n)
    return n, err
}
```

### エラーログとパニックリカバリ

アプリケーションエラーとパニックを適切にログ記録し、アプリケーションの安定性を保つことも重要です：

```go
func ErrorMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                slog.Error("panic recovered",
                    "error", rec,
                    "request_id", r.Context().Value("request_id"))
                http.Error(w, "Internal Server Error", 500)
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

### ログレベルとフィルタリング

本番環境では、ログレベルを適切に設定して、必要な情報のみを出力します：

- **Debug**: 開発時のデバッグ情報
- **Info**: 一般的な情報（リクエストログなど）
- **Warn**: 警告（遅いレスポンスなど）
- **Error**: エラー（4xx/5xxレスポンス、例外など）

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **構造化ロギングの設定**
   - JSON形式でログを出力するslogロガーの設定

2. **リクエストIDミドルウェア**
   - 16文字のランダムなリクエストIDを生成
   - ContextとX-Request-IDヘッダーに設定

3. **ロギングミドルウェア**
   - リクエスト開始と完了をログ記録
   - HTTPメソッド、URL、User-Agent、ステータスコード、レスポンスサイズ、処理時間を含む

4. **レスポンスライターラッピング**
   - ステータスコードとレスポンスサイズをキャプチャ

5. **エラーミドルウェア**
   - パニックをキャッチして500エラーを返す
   - エラー情報をログ記録

6. **ユーザーコンテキストミドルウェア**
   - X-User-IDヘッダーからユーザー情報を取得
   - 未設定の場合は"anonymous"として扱う

## ✅ 期待される挙動

テストが成功すると、以下のような構造化ログが出力されます：

```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "request_start",
  "method": "GET",
  "url": "/api/users",
  "user_agent": "Go-http-client/1.1",
  "request_id": "a1b2c3d4e5f67890",
  "user_id": "user123"
}

{
  "time": "2024-01-15T10:30:00.125Z",
  "level": "INFO", 
  "msg": "request_complete",
  "method": "GET",
  "url": "/api/users",
  "status_code": 200,
  "bytes_written": 1024,
  "duration_ms": 125,
  "request_id": "a1b2c3d4e5f67890",
  "user_id": "user123"
}
```

パニックが発生した場合：

```json
{
  "time": "2024-01-15T10:30:05Z",
  "level": "ERROR",
  "msg": "panic_recovered",
  "error": "simulated panic",
  "request_id": "f6e5d4c3b2a10987"
}
```

## 💡 ヒント

1. **slog.JSONHandler**: JSON形式のログ出力に使用
2. **crypto/rand**: 安全なランダム値生成
3. **encoding/hex**: バイト配列を16進文字列に変換
4. **context.WithValue**: Contextにカスタム値を保存
5. **http.ResponseWriter embedding**: インターフェースを満たしながら機能を拡張
6. **recover()**: パニックをキャッチして回復
7. **time.Since()**: 経過時間の測定

### コンテキストキーの型安全性

Contextキーには専用の型を使用して、キーの衝突を防ぎます：

```go
type contextKey string

const (
    RequestIDKey contextKey = "request_id"
    UserIDKey    contextKey = "user_id"
)

// 使用例
ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
requestID := ctx.Value(RequestIDKey).(string)
```

### ログフィールドの標準化

一貫したログ分析のため、フィールド名は標準化します：

- `request_id`: リクエスト識別子
- `user_id`: ユーザー識別子  
- `method`: HTTPメソッド
- `url`: リクエストURL
- `status_code`: HTTPステータスコード
- `bytes_written`: レスポンスサイズ
- `duration_ms`: 処理時間（ミリ秒）

これらの実装により、プロダクションレベルの構造化ロギングシステムを構築できます。
