# Day 18: リクエストボディのサイズ制限

## 🎯 本日の目標 (Today's Goal)

HTTPリクエストボディのサイズを制限するミドルウェアを実装し、メモリ枯渇攻撃やDoS攻撃からサーバーを保護する。動的サイズ制限、Content-Type別制限、進捗モニタリング、グレースフルな処理を含む包括的なボディサイズ制御システムを構築する。

## 📖 解説 (Explanation)

### リクエストボディサイズ制限の重要性

```go
// 【リクエストボディサイズ制限の重要性】DoS攻撃とメモリ枯渇からの保護
// ❌ 問題例：サイズ制限なしでの壊滅的なDoS攻撃被害
func catastrophicNoBodySizeLimit() {
    // 🚨 災害例：無制限なリクエストボディ受信でサーバー崩壊
    
    http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
        log.Printf("Receiving upload request from %s", r.RemoteAddr)
        
        // ❌ サイズ制限なしでリクエストボディを読み取り
        bodyBytes, err := ioutil.ReadAll(r.Body)
        if err != nil {
            http.Error(w, "Failed to read body", http.StatusInternalServerError)
            return
        }
        
        // ❌ 100GB のファイルでもメモリに全て読み込む
        // メモリ使用量: 100GB × 同時接続数 = サーバークラッシュ
        
        log.Printf("Received %d bytes from %s", len(bodyBytes), r.RemoteAddr)
        
        // ❌ 攻撃者が100個の接続で10GBずつ送信
        // 合計1TB のメモリ消費 → OOM Killer発動
        // ❌ 正常なユーザーも巻き込まれてサービス全停止
        // ❌ インフラコストが爆発的に増大
        
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Upload processed"))
    })
    
    log.Println("❌ Starting server without body size limits...")
    http.ListenAndServe(":8080", nil)
    // 結果：メモリ枯渇攻撃により数分でサーバーダウン、全サービス停止
}

// ✅ 正解：エンタープライズ級ボディサイズ制限システム
type EnterpriseBodySizeLimiter struct {
    // 【基本設定】
    globalMaxSize     int64                    // グローバル最大サイズ
    contentTypeLimits map[string]int64         // Content-Type別制限
    
    // 【高度な機能】
    dynamicLimiter    *DynamicSizeLimiter      // 動的制限調整
    progressTracker   *ProgressTracker         // 進捗追跡
    rateLimiter       *UploadRateLimiter       // アップロード速度制限
    
    // 【セキュリティ】
    blacklist         *IPBlacklist             // 悪意IPブラックリスト
    anomalyDetector   *AnomalyDetector         // 異常検知システム
    
    // 【監視・ログ】
    metrics           *DetailedMetrics         // 詳細メトリクス
    logger            *log.Logger              // 構造化ログ
    alertManager      *AlertManager            // アラート管理
    
    // 【リソース管理】
    memoryMonitor     *MemoryMonitor           // メモリ使用量監視
    connectionLimiter *ConnectionLimiter       // 同時接続数制限
    
    // 【設定管理】
    configManager     *ConfigManager           // 動的設定管理
    mu                sync.RWMutex             // 設定変更用ミューテックス
}

// 【重要関数】エンタープライズ級ボディサイズ制限システム初期化
func NewEnterpriseBodySizeLimiter(config *LimiterConfig) *EnterpriseBodySizeLimiter {
    limiter := &EnterpriseBodySizeLimiter{
        globalMaxSize: config.GlobalMaxSize,
        contentTypeLimits: map[string]int64{
            "application/json":       1 << 20,     // 1MB - API calls
            "application/xml":        2 << 20,     // 2MB - structured data
            "multipart/form-data":    50 << 20,    // 50MB - file uploads
            "image/jpeg":             10 << 20,    // 10MB - image files
            "image/png":              10 << 20,    // 10MB - image files
            "video/mp4":              500 << 20,   // 500MB - video files
            "application/octet-stream": 100 << 20,  // 100MB - binary data
        },
        
        dynamicLimiter:    NewDynamicSizeLimiter(config.BaseLimit),
        progressTracker:   NewProgressTracker(config.MaxConcurrentUploads),
        rateLimiter:       NewUploadRateLimiter(config.MaxUploadRate),
        blacklist:         NewIPBlacklist(),
        anomalyDetector:   NewAnomalyDetector(),
        metrics:           NewDetailedMetrics(),
        logger:            log.New(os.Stdout, "[BODY-LIMITER] ", log.LstdFlags),
        alertManager:      NewAlertManager(),
        memoryMonitor:     NewMemoryMonitor(),
        connectionLimiter: NewConnectionLimiter(config.MaxConnections),
        configManager:     NewConfigManager(),
    }
    
    // 【重要】監視とアラートの開始
    go limiter.startMonitoring()
    go limiter.startAnomalyDetection()
    go limiter.startMemoryMonitoring()
    
    limiter.logger.Printf("🚀 Enterprise body size limiter initialized")
    limiter.logger.Printf("   Global limit: %.2f MB", float64(config.GlobalMaxSize)/1024/1024)
    limiter.logger.Printf("   Content-type limits: %d configured", len(limiter.contentTypeLimits))
    limiter.logger.Printf("   Max concurrent uploads: %d", config.MaxConcurrentUploads)
    
    return limiter
}

// 【核심メソッド】HTTPミドルウェア実装
func (limiter *EnterpriseBodySizeLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        startTime := time.Now()
        requestID := generateRequestID()
        
        // 【STEP 1】事前セキュリティチェック
        if blocked, reason := limiter.blacklist.IsBlocked(getClientIP(r)); blocked {
            limiter.metrics.RecordBlocked(reason)
            limiter.logger.Printf("❌ Blocked request from %s: %s", getClientIP(r), reason)
            http.Error(w, "Request blocked", http.StatusForbidden)
            return
        }
        
        // 【STEP 2】同時接続数制限チェック
        if !limiter.connectionLimiter.AllowConnection() {
            limiter.metrics.RecordRejection("max_connections_exceeded")
            limiter.logger.Printf("⚠️  Connection limit exceeded from %s", getClientIP(r))
            http.Error(w, "Too many connections", http.StatusTooManyRequests)
            return
        }
        defer limiter.connectionLimiter.ReleaseConnection()
        
        // 【STEP 3】Content-Type別制限取得
        contentType := r.Header.Get("Content-Type")
        mediaType, _, _ := mime.ParseMediaType(contentType)
        
        limiter.mu.RLock()
        typeLimit, exists := limiter.contentTypeLimits[mediaType]
        if !exists {
            typeLimit = limiter.globalMaxSize
        }
        limiter.mu.RUnlock()
        
        // 動的制限との比較
        dynamicLimit := limiter.dynamicLimiter.GetCurrentLimit()
        effectiveLimit := min(typeLimit, dynamicLimit)
        
        limiter.logger.Printf("📊 Request %s: Content-Type=%s, Limit=%.2fMB", 
            requestID, mediaType, float64(effectiveLimit)/1024/1024)
        
        // 【STEP 4】Content-Length事前チェック
        if r.ContentLength > effectiveLimit {
            limiter.metrics.RecordRejection("content_length_exceeded")
            limiter.anomalyDetector.ReportSuspiciousActivity(getClientIP(r), "oversized_request", r.ContentLength)
            
            limiter.logger.Printf("❌ Content-Length exceeded: %d > %d (client: %s)", 
                r.ContentLength, effectiveLimit, getClientIP(r))
            
            http.Error(w, fmt.Sprintf("Request body too large (limit: %.2f MB)", 
                float64(effectiveLimit)/1024/1024), http.StatusRequestEntityTooLarge)
            return
        }
        
        // 【STEP 5】プログレストラッキング開始
        if err := limiter.progressTracker.StartTracking(requestID, mediaType, r.ContentLength); err != nil {
            limiter.metrics.RecordRejection("tracking_failed")
            limiter.logger.Printf("❌ Failed to start progress tracking: %v", err)
            http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
            return
        }
        defer limiter.progressTracker.FinishTracking(requestID)
        
        // 【STEP 6】レート制限チェック
        if !limiter.rateLimiter.AllowUpload(getClientIP(r), r.ContentLength) {
            limiter.metrics.RecordRejection("rate_limit_exceeded")
            limiter.logger.Printf("⚠️  Upload rate limit exceeded for %s", getClientIP(r))
            http.Error(w, "Upload rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        
        // 【STEP 7】ボディリーダーのラップ
        originalBody := r.Body
        r.Body = &EnterpriseBodyReader{
            reader:          originalBody,
            maxSize:         effectiveLimit,
            requestID:       requestID,
            progressTracker: limiter.progressTracker,
            rateLimiter:     limiter.rateLimiter,
            metrics:         limiter.metrics,
            logger:          limiter.logger,
            clientIP:        getClientIP(r),
            startTime:       startTime,
            anomalyDetector: limiter.anomalyDetector,
        }
        
        // 【STEP 8】次のハンドラーへ
        limiter.metrics.RecordAccepted(mediaType)
        next.ServeHTTP(w, r)
        
        // 【STEP 9】完了時の統計更新
        duration := time.Since(startTime)
        limiter.metrics.RecordProcessingTime(duration)
        
        limiter.logger.Printf("✅ Request %s completed in %v", requestID, duration)
    })
}

// 【高度な機能】エンタープライズ級ボディリーダー
type EnterpriseBodyReader struct {
    reader          io.ReadCloser
    maxSize         int64
    bytesRead       int64
    requestID       string
    progressTracker *ProgressTracker
    rateLimiter     *UploadRateLimiter
    metrics         *DetailedMetrics
    logger          *log.Logger
    clientIP        string
    startTime       time.Time
    anomalyDetector *AnomalyDetector
    lastProgressTime time.Time
}

// 【重要メソッド】高度なRead実装
func (reader *EnterpriseBodyReader) Read(p []byte) (n int, err error) {
    // 【制限チェック】
    if reader.bytesRead >= reader.maxSize {
        reader.metrics.RecordRejection("stream_size_exceeded")
        reader.anomalyDetector.ReportSuspiciousActivity(reader.clientIP, "stream_size_exceeded", reader.bytesRead)
        reader.logger.Printf("❌ Stream size exceeded for request %s: %d bytes", reader.requestID, reader.bytesRead)
        return 0, &BodySizeExceededError{
            RequestID: reader.requestID,
            BytesRead: reader.bytesRead,
            MaxSize:   reader.maxSize,
        }
    }
    
    // 【読み取り可能サイズ計算】
    remaining := reader.maxSize - reader.bytesRead
    if int64(len(p)) > remaining {
        p = p[:remaining]
    }
    
    // 【タイムアウト付き読み取り】
    readDeadline := time.Now().Add(30 * time.Second)
    if conn, ok := reader.reader.(interface{ SetReadDeadline(time.Time) error }); ok {
        conn.SetReadDeadline(readDeadline)
    }
    
    // 【実際の読み取り】
    n, err = reader.reader.Read(p)
    reader.bytesRead += int64(n)
    
    // 【進捗更新】
    now := time.Now()
    if now.Sub(reader.lastProgressTime) > 100*time.Millisecond {
        reader.progressTracker.UpdateProgress(reader.requestID, int64(n))
        reader.lastProgressTime = now
        
        // 転送速度計算
        duration := now.Sub(reader.startTime)
        if duration > 0 {
            rate := float64(reader.bytesRead) / duration.Seconds()
            reader.metrics.RecordTransferRate(rate)
            
            // 異常に遅い転送の検知（Slowloris攻撃対策）
            if rate < 1024 && duration > 10*time.Second { // 1KB/s未満が10秒以上
                reader.anomalyDetector.ReportSuspiciousActivity(reader.clientIP, "slow_transfer", int64(rate))
                reader.logger.Printf("⚠️  Slow transfer detected from %s: %.2f bytes/sec", reader.clientIP, rate)
            }
        }
    }
    
    // 【レート制限適用】
    reader.rateLimiter.ApplyRateLimit(reader.clientIP, int64(n))
    
    // 【サイズ超過の最終チェック】
    if reader.bytesRead > reader.maxSize {
        reader.metrics.RecordRejection("stream_size_exceeded")
        reader.logger.Printf("❌ Final size check failed for request %s: %d > %d", 
            reader.requestID, reader.bytesRead, reader.maxSize)
        return n, &BodySizeExceededError{
            RequestID: reader.requestID,
            BytesRead: reader.bytesRead,
            MaxSize:   reader.maxSize,
        }
    }
    
    return n, err
}

// 【カスタムエラー型】詳細なエラー情報
type BodySizeExceededError struct {
    RequestID string
    BytesRead int64
    MaxSize   int64
}

func (e *BodySizeExceededError) Error() string {
    return fmt.Sprintf("body size exceeded: %d bytes read, limit: %d bytes (request: %s)", 
        e.BytesRead, e.MaxSize, e.RequestID)
}

// 【実用例】高負荷環境での実際の使用
func ProductionBodySizeLimitingUsage() {
    // 【初期化】本番環境設定
    config := &LimiterConfig{
        GlobalMaxSize:          100 << 20,  // 100MB
        BaseLimit:              50 << 20,   // 50MB (動的調整ベース)
        MaxConcurrentUploads:   50,         // 同時アップロード数
        MaxUploadRate:          10 << 20,   // 10MB/s per IP
        MaxConnections:         1000,       // 最大同時接続数
    }
    
    limiter := NewEnterpriseBodySizeLimiter(config)
    
    // 【ルート設定】
    mux := http.NewServeMux()
    
    // アップロードエンドポイント
    mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        // リクエストボディ処理（制限が適用済み）
        body, err := ioutil.ReadAll(r.Body)
        if err != nil {
            http.Error(w, "Failed to read body", http.StatusBadRequest)
            return
        }
        
        log.Printf("✅ Successfully processed %d bytes upload", len(body))
        
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status":       "success",
            "bytes_received": len(body),
            "timestamp":    time.Now().Unix(),
        })
    })
    
    // 管理用エンドポイント（メトリクス表示）
    mux.HandleFunc("/admin/metrics", func(w http.ResponseWriter, r *http.Request) {
        metrics := limiter.metrics.GetSummary()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(metrics)
    })
    
    // 【ミドルウェア適用】
    handler := limiter.Middleware(mux)
    
    // 【サーバー起動】
    server := &http.Server{
        Addr:           ":8080",
        Handler:        handler,
        ReadTimeout:    30 * time.Second,
        WriteTimeout:   30 * time.Second,
        IdleTimeout:    60 * time.Second,
        MaxHeaderBytes: 1 << 20, // 1MB
    }
    
    log.Printf("🚀 Production server starting on :8080")
    log.Printf("   Body size limits: Global=%.2fMB, Dynamic adjustment enabled", 
        float64(config.GlobalMaxSize)/1024/1024)
    log.Printf("   Security features: IP blacklist, anomaly detection, rate limiting")
    
    log.Fatal(server.ListenAndServe())
}
```

### 基本的なサイズ制限実装

```go
type BodySizeLimitMiddleware struct {
    maxSize     int64
    errorWriter ErrorWriter
    metrics     *Metrics
}

func NewBodySizeLimitMiddleware(maxSize int64) *BodySizeLimitMiddleware {
    return &BodySizeLimitMiddleware{
        maxSize:     maxSize,
        errorWriter: &DefaultErrorWriter{},
        metrics:     NewMetrics(),
    }
}

func (m *BodySizeLimitMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Content-Lengthヘッダーをチェック
        if r.ContentLength > m.maxSize {
            m.metrics.RecordRejection("content_length_exceeded")
            m.errorWriter.WriteError(w, ErrRequestTooLarge)
            return
        }
        
        // リーダーをラップしてストリーミング制限
        r.Body = &limitedReader{
            reader:  r.Body,
            maxSize: m.maxSize,
            metrics: m.metrics,
        }
        
        next.ServeHTTP(w, r)
    })
}

type limitedReader struct {
    reader   io.ReadCloser
    maxSize  int64
    readSize int64
    metrics  *Metrics
}

func (lr *limitedReader) Read(p []byte) (n int, err error) {
    if lr.readSize >= lr.maxSize {
        lr.metrics.RecordRejection("stream_size_exceeded")
        return 0, ErrRequestTooLarge
    }
    
    // 読み込み可能サイズを計算
    remaining := lr.maxSize - lr.readSize
    if int64(len(p)) > remaining {
        p = p[:remaining]
    }
    
    n, err = lr.reader.Read(p)
    lr.readSize += int64(n)
    
    if lr.readSize > lr.maxSize {
        lr.metrics.RecordRejection("stream_size_exceeded")
        return n, ErrRequestTooLarge
    }
    
    return n, err
}

func (lr *limitedReader) Close() error {
    return lr.reader.Close()
}
```

### Content-Type別サイズ制限

```go
type ContentTypeLimits struct {
    limits map[string]int64
    defaultLimit int64
}

func NewContentTypeLimits() *ContentTypeLimits {
    return &ContentTypeLimits{
        limits: map[string]int64{
            "application/json":       1 << 20,    // 1MB
            "application/xml":        2 << 20,    // 2MB
            "text/plain":            512 << 10,   // 512KB
            "multipart/form-data":   10 << 20,    // 10MB
            "image/jpeg":            5 << 20,     // 5MB
            "image/png":             5 << 20,     // 5MB
            "video/mp4":             100 << 20,   // 100MB
        },
        defaultLimit: 1 << 20, // 1MB
    }
}

func (ctl *ContentTypeLimits) GetLimit(contentType string) int64 {
    // Content-Type からメディアタイプを抽出
    mediaType, _, err := mime.ParseMediaType(contentType)
    if err != nil {
        return ctl.defaultLimit
    }
    
    if limit, exists := ctl.limits[mediaType]; exists {
        return limit
    }
    
    return ctl.defaultLimit
}

type AdvancedBodySizeLimitMiddleware struct {
    contentTypeLimits *ContentTypeLimits
    globalMaxSize     int64
    progressTracker   *ProgressTracker
    metrics          *Metrics
}

func (m *AdvancedBodySizeLimitMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        contentType := r.Header.Get("Content-Type")
        typeLimit := m.contentTypeLimits.GetLimit(contentType)
        
        // グローバル制限と型別制限の小さい方を採用
        effectiveLimit := min(m.globalMaxSize, typeLimit)
        
        if r.ContentLength > effectiveLimit {
            m.metrics.RecordRejection("content_length_exceeded", contentType)
            http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
            return
        }
        
        // プログレストラッカー付きリーダー
        r.Body = &progressTrackingReader{
            reader:    r.Body,
            maxSize:   effectiveLimit,
            tracker:   m.progressTracker,
            requestID: getRequestID(r),
            metrics:   m.metrics,
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### プログレストラッキング

```go
type ProgressTracker struct {
    activeReads map[string]*ReadProgress
    mu          sync.RWMutex
    maxConcurrent int
}

type ReadProgress struct {
    RequestID   string
    TotalSize   int64
    ReadSize    int64
    StartTime   time.Time
    LastUpdate  time.Time
    ContentType string
    Rate        *RateCalculator
}

func NewProgressTracker(maxConcurrent int) *ProgressTracker {
    return &ProgressTracker{
        activeReads:   make(map[string]*ReadProgress),
        maxConcurrent: maxConcurrent,
    }
}

func (pt *ProgressTracker) StartTracking(requestID, contentType string, totalSize int64) error {
    pt.mu.Lock()
    defer pt.mu.Unlock()
    
    if len(pt.activeReads) >= pt.maxConcurrent {
        return errors.New("too many concurrent reads")
    }
    
    pt.activeReads[requestID] = &ReadProgress{
        RequestID:   requestID,
        TotalSize:   totalSize,
        ReadSize:    0,
        StartTime:   time.Now(),
        LastUpdate:  time.Now(),
        ContentType: contentType,
        Rate:        NewRateCalculator(),
    }
    
    return nil
}

func (pt *ProgressTracker) UpdateProgress(requestID string, bytesRead int64) {
    pt.mu.Lock()
    defer pt.mu.Unlock()
    
    if progress, exists := pt.activeReads[requestID]; exists {
        progress.ReadSize += bytesRead
        progress.LastUpdate = time.Now()
        progress.Rate.Update(bytesRead)
        
        // 進捗ログ
        if progress.TotalSize > 0 {
            percentage := float64(progress.ReadSize) / float64(progress.TotalSize) * 100
            log.Printf("Request %s: %.1f%% complete (%d/%d bytes)", 
                requestID, percentage, progress.ReadSize, progress.TotalSize)
        }
    }
}

type progressTrackingReader struct {
    reader    io.ReadCloser
    maxSize   int64
    readSize  int64
    tracker   *ProgressTracker
    requestID string
    metrics   *Metrics
}

func (ptr *progressTrackingReader) Read(p []byte) (n int, err error) {
    if ptr.readSize >= ptr.maxSize {
        ptr.metrics.RecordRejection("stream_size_exceeded")
        return 0, ErrRequestTooLarge
    }
    
    n, err = ptr.reader.Read(p)
    ptr.readSize += int64(n)
    
    // プログレス更新
    ptr.tracker.UpdateProgress(ptr.requestID, int64(n))
    
    if ptr.readSize > ptr.maxSize {
        ptr.metrics.RecordRejection("stream_size_exceeded") 
        return n, ErrRequestTooLarge
    }
    
    return n, err
}
```

### 動的サイズ制限

```go
type DynamicSizeLimiter struct {
    baseLimit     int64
    scaleFactor   float64
    metrics       *SystemMetrics
    loadThreshold float64
    mu            sync.RWMutex
}

func NewDynamicSizeLimiter(baseLimit int64) *DynamicSizeLimiter {
    return &DynamicSizeLimiter{
        baseLimit:     baseLimit,
        scaleFactor:   1.0,
        metrics:       NewSystemMetrics(),
        loadThreshold: 0.8,
    }
}

func (dsl *DynamicSizeLimiter) GetCurrentLimit() int64 {
    dsl.mu.RLock()
    defer dsl.mu.RUnlock()
    
    return int64(float64(dsl.baseLimit) * dsl.scaleFactor)
}

func (dsl *DynamicSizeLimiter) AdjustLimit() {
    memUsage := dsl.metrics.GetMemoryUsage()
    cpuUsage := dsl.metrics.GetCPUUsage()
    avgLoad := (memUsage + cpuUsage) / 2
    
    dsl.mu.Lock()
    defer dsl.mu.Unlock()
    
    if avgLoad > dsl.loadThreshold {
        // 負荷が高い場合は制限を厳しく
        dsl.scaleFactor = max(0.1, dsl.scaleFactor*0.9)
    } else if avgLoad < dsl.loadThreshold*0.5 {
        // 負荷が低い場合は制限を緩和
        dsl.scaleFactor = min(1.0, dsl.scaleFactor*1.1)
    }
}

// 定期的な調整
func (dsl *DynamicSizeLimiter) StartAutoAdjustment(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            dsl.AdjustLimit()
        case <-ctx.Done():
            return
        }
    }
}
```

### ストリーミング処理とバックプレッシャー

```go
type StreamingBodyHandler struct {
    chunkSize     int
    timeout       time.Duration
    backpressure  *BackpressureController
}

func NewStreamingBodyHandler(chunkSize int, timeout time.Duration) *StreamingBodyHandler {
    return &StreamingBodyHandler{
        chunkSize:    chunkSize,
        timeout:      timeout,
        backpressure: NewBackpressureController(),
    }
}

func (sbh *StreamingBodyHandler) ProcessStream(r io.Reader, processor func([]byte) error) error {
    buffer := make([]byte, sbh.chunkSize)
    
    for {
        // バックプレッシャーチェック
        if err := sbh.backpressure.WaitIfNeeded(context.Background()); err != nil {
            return err
        }
        
        n, err := r.Read(buffer[:])
        if n > 0 {
            if err := processor(buffer[:n]); err != nil {
                return err
            }
        }
        
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
    }
    
    return nil
}

type BackpressureController struct {
    currentLoad int64
    maxLoad     int64
    throttle    chan struct{}
}

func NewBackpressureController() *BackpressureController {
    return &BackpressureController{
        maxLoad:  1000,
        throttle: make(chan struct{}, 100),
    }
}

func (bc *BackpressureController) WaitIfNeeded(ctx context.Context) error {
    if atomic.LoadInt64(&bc.currentLoad) > bc.maxLoad {
        select {
        case <-bc.throttle:
            return nil
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    return nil
}
```

## 📝 課題 (The Problem)

以下の機能を持つ包括的なリクエストボディサイズ制限システムを実装してください：

### 1. 基本サイズ制限
- グローバル最大サイズ制限
- Content-Length ヘッダーチェック
- ストリーミング読み込み制限
- 適切なエラーレスポンス

### 2. Content-Type別制限
- メディアタイプ別サイズ制限
- MIME タイプ解析
- デフォルト制限の適用
- 動的制限設定

### 3. プログレストラッキング
- 読み込み進捗の監視
- 並行読み込み数制限
- 転送速度計算
- タイムアウト制御

### 4. 動的制限調整
- システム負荷に基づく調整
- メモリ・CPU使用率監視
- 自動スケーリング
- 制限履歴の記録

### 5. セキュリティ機能
- スローポスト攻撃対策
- バックプレッシャー制御
- リソース枯渇防止
- 攻撃検知とログ記録

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestBodySizeLimit_BasicLimiting
    main_test.go:45: Basic size limiting working correctly
--- PASS: TestBodySizeLimit_BasicLimiting (0.01s)

=== RUN   TestBodySizeLimit_ContentTypeSpecific
    main_test.go:65: Content-type specific limits applied
--- PASS: TestBodySizeLimit_ContentTypeSpecific (0.02s)

=== RUN   TestBodySizeLimit_ProgressTracking
    main_test.go:85: Progress tracking functioning properly
--- PASS: TestBodySizeLimit_ProgressTracking (0.03s)

=== RUN   TestBodySizeLimit_DynamicAdjustment
    main_test.go:105: Dynamic limit adjustment working
--- PASS: TestBodySizeLimit_DynamicAdjustment (0.04s)

PASS
ok      day18-request-body-size-limit   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 基本的なサイズチェック

```go
func checkContentLength(r *http.Request, maxSize int64) error {
    if r.ContentLength < 0 {
        return nil // Content-Length 不明
    }
    
    if r.ContentLength > maxSize {
        return ErrRequestTooLarge
    }
    
    return nil
}
```

### ストリーミングリーダー

```go
type LimitedReader struct {
    R       io.Reader
    N       int64 // 最大読み込み可能バイト数
    read    int64 // 既に読み込んだバイト数
}

func (l *LimitedReader) Read(p []byte) (n int, err error) {
    if l.read >= l.N {
        return 0, io.EOF
    }
    
    if int64(len(p)) > l.N-l.read {
        p = p[0:l.N-l.read]
    }
    
    n, err = l.R.Read(p)
    l.read += int64(n)
    return
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **分散制限**: Redis を使った複数インスタンス間での制限共有
2. **機械学習予測**: 過去のトラフィックパターンに基づく動的制限
3. **WebSocket サポート**: リアルタイム通信での制限適用
4. **圧縮対応**: gzip 圧縮されたボディの効率的な処理
5. **監視ダッシュボード**: Grafana を使った制限状況の可視化

リクエストボディサイズ制限の実装を通じて、Webアプリケーションのセキュリティとパフォーマンスを向上させる重要な技術を習得しましょう！