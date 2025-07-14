# Day 18: リクエストボディのサイズ制限

## 🎯 本日の目標 (Today's Goal)

HTTPリクエストボディのサイズを制限するミドルウェアを実装し、メモリ枯渇攻撃やDoS攻撃からサーバーを保護する。動的サイズ制限、Content-Type別制限、進捗モニタリング、グレースフルな処理を含む包括的なボディサイズ制御システムを構築する。

## 📖 解説 (Explanation)

### リクエストボディサイズ制限の重要性

Web アプリケーションでは、悪意のあるクライアントが巨大なリクエストボディを送信することで、サーバーのメモリを枯渇させたり、ネットワーク帯域を占有する攻撃が可能です。適切なサイズ制限により、これらの攻撃からサーバーを保護できます。

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