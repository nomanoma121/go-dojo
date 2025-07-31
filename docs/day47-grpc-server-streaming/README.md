# Day 47: gRPC Server-side Streaming

## 🎯 本日の目標 (Today's Goal)

gRPCのサーバーサイドストリーミングを完全に理解し、実装する。リアルタイム通知、ログストリーミング、大量データの分割送信、ファイル転送などの実用的なユースケースを通じて、高性能で堅牢なストリーミングシステムを構築できるようになる。

## 📖 解説 (Explanation)

```go
// 【gRPCサーバーサイドストリーミングの重要性】大規模データ配信システムの中核技術
// ❌ 問題例：不適切なサーバーストリーミング実装による壊滅的システム障害
func serverStreamingDisasters() {
    // 🚨 災害例：不正実装によるメモリリーク、DoS攻撃、セキュリティ侵害
    
    // ❌ 最悪の実装1：メモリリークを引き起こすログストリーミング
    func BadLogStreaming(req *pb.LogRequest, stream pb.LogService_StreamLogsServer) error {
        // ❌ 全ログを一度にメモリに読み込み - OOM確実
        allLogs := loadAllLogsFromDatabase() // 100GB のログデータ
        
        // ❌ バッファリングなしで送信 - メモリ使用量爆発
        for _, log := range allLogs {
            logEntry := &pb.LogEntry{
                Timestamp: log.Timestamp,
                Message:   log.Message,
                RawData:   log.RawData, // ❌ 生データを全て含める
            }
            
            // ❌ エラーハンドリングなし - 接続切断でリークする
            stream.Send(logEntry) // 失敗時の処理なし
        }
        return nil // ❌ リソースクリーンアップなし
        
        // 【災害的結果】
        // - 1時間後: メモリ使用量200GB、スワップ発生
        // - 2時間後: サーバーOOM Kill、全サービス停止
        // - 顧客からの苦情殺到、SLA違反で損害賠償
    }
    
    // ❌ 最悪の実装2：認証なしファイル配信で機密情報流出
    func BadFileStreaming(req *pb.FileRequest, stream pb.FileService_StreamFileServer) error {
        // ❌ 認証チェックなし - 誰でもアクセス可能
        filePath := req.GetFilePath()
        
        // ❌ パストラバーサル攻撃に対する防御なし
        // 攻撃者: "../../../etc/passwd" で機密ファイル取得可能
        file, err := os.Open(filePath)
        if err != nil {
            return err
        }
        defer file.Close()
        
        // ❌ レート制限なし - 攻撃者が大量ダウンロード可能
        buffer := make([]byte, 1024*1024) // 1MB chunks
        for {
            n, err := file.Read(buffer)
            if err == io.EOF {
                break
            }
            
            chunk := &pb.FileChunk{
                Data: buffer[:n], // ❌ 暗号化なしで機密データ送信
            }
            
            // ❌ 送信エラーを無視 - データ破損の可能性
            stream.Send(chunk)
        }
        
        // 【災害的結果】
        // - 攻撃者が全顧客データベースをダウンロード
        // - 機密情報流出、GDPR違反で制裁金50億円
        // - 企業の信頼失墜、株価50%下落
    }
    
    // ❌ 最悪の実装3：DoS攻撃を増幅するリアルタイム通知
    func BadNotificationStreaming(req *pb.NotificationRequest, stream pb.NotificationService_StreamNotificationsServer) error {
        // ❌ 購読者管理なし - 無制限接続受け入れ
        userID := req.GetUserId()
        
        // ❌ 通知キューに制限なし - メモリ爆発
        notificationChan := make(chan *pb.Notification) // unbuffered!
        
        // ❌ ゴルーチンリークを引き起こす実装
        go func() {
            for notification := range globalNotificationChannel {
                // ❌ バックプレッシャー制御なし
                // クライアントが遅い場合、全体がブロック
                notificationChan <- notification // デッドロック発生
            }
        }()
        
        // ❌ コンテキストキャンセルを無視
        for notification := range notificationChan {
            // ❌ クライアント切断を検知せず永続送信
            stream.Send(notification) // 幽霊ストリーム生成
        }
        
        // 【災害的結果】
        // - 100万接続で各1MB通知 → 1TB メモリ使用
        // - ゴルーチン500万個でCPU使用率100%
        // - 全サービス応答不能、ビジネス完全停止
    }
    
    // ❌ 最悪の実装4：レースコンディションだらけのメトリクス配信
    func BadMetricsStreaming(req *pb.MetricsRequest, stream pb.MetricsService_StreamMetricsServer) error {
        // ❌ 排他制御なしでグローバル変数操作
        activeStreams++ // レースコンディション発生
        
        // ❌ 同期化なしでマップアクセス
        metricsMap := globalMetricsMap // 複数ゴルーチンから同時アクセス
        
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                // ❌ データ競合でクラッシュ確実
                for name, value := range metricsMap { // panic: concurrent map read and map write
                    metric := &pb.Metric{
                        Name:  name,
                        Value: value,
                    }
                    
                    // ❌ 送信失敗時の処理なし
                    stream.Send(metric)
                }
            }
        }
        
        // 【災害的結果】
        // - 5分後: panic でプロセス全体クラッシュ
        // - 監視システム停止で障害検知不能
        // - カスケード障害で全インフラダウン
    }
    
    // 【実際の被害例】
    // - 動画配信企業：ストリーミングバグで同時視聴者全員切断、売上99%減
    // - 金融システム：リアルタイム取引配信障害で誤注文多発、損失100億円
    // - 医療システム：患者監視データ流出、プライバシー侵害で集団訴訟
    // - ECサイト：商品画像配信停止で注文不能、ブラックフライデー売上ゼロ
    
    fmt.Println("❌ Server-side streaming disasters caused complete business shutdown!")
    // 結果：メモリリーク、セキュリティ侵害、DoS攻撃成功、企業倒産危機
}

// ✅ 正解：エンタープライズ級サーバーサイドストリーミングシステム
type EnterpriseServerStreamingSystem struct {
    // 【接続管理】
    connectionManager    *ConnectionManager       // 接続プール管理
    sessionManager       *SessionManager         // セッション管理
    streamRegistry       *StreamRegistry         // アクティブストリーム登録
    
    // 【セキュリティ】
    authManager          *AuthManager            // 認証・認可
    encryptionManager    *EncryptionManager      // データ暗号化
    auditLogger          *AuditLogger            // セキュリティ監査
    
    // 【パフォーマンス】
    loadBalancer         *LoadBalancer           // 負荷分散
    rateLimiter          *RateLimiter            // レート制限
    backpressureManager  *BackpressureManager    // バックプレッシャー制御
    
    // 【リソース管理】
    memoryManager        *MemoryManager          // メモリ使用量制御
    bufferPool           *BufferPool             // バッファプール
    compressionManager   *CompressionManager     // データ圧縮
    
    // 【監視・診断】
    metricsCollector     *MetricsCollector       // メトリクス収集
    healthChecker        *HealthChecker          // ヘルスチェック
    performanceAnalyzer  *PerformanceAnalyzer    // パフォーマンス分析
    
    // 【障害対応】
    circuitBreaker       *CircuitBreaker         // サーキットブレーカー
    retryManager         *RetryManager           // リトライ管理
    failoverManager      *FailoverManager        // フェイルオーバー
    
    config               *StreamingConfig        // 設定管理
    mu                   sync.RWMutex            // 並行アクセス制御
}

// 【重要関数】エンタープライズストリーミングシステム初期化
func NewEnterpriseServerStreamingSystem(config *StreamingConfig) *EnterpriseServerStreamingSystem {
    return &EnterpriseServerStreamingSystem{
        config:               config,
        connectionManager:    NewConnectionManager(),
        sessionManager:       NewSessionManager(),
        streamRegistry:       NewStreamRegistry(),
        authManager:          NewAuthManager(),
        encryptionManager:    NewEncryptionManager(),
        auditLogger:          NewAuditLogger(),
        loadBalancer:         NewLoadBalancer(),
        rateLimiter:          NewRateLimiter(),
        backpressureManager:  NewBackpressureManager(),
        memoryManager:        NewMemoryManager(),
        bufferPool:           NewBufferPool(),
        compressionManager:   NewCompressionManager(),
        metricsCollector:     NewMetricsCollector(),
        healthChecker:        NewHealthChecker(),
        performanceAnalyzer:  NewPerformanceAnalyzer(),
        circuitBreaker:       NewCircuitBreaker(),
        retryManager:         NewRetryManager(),
        failoverManager:      NewFailoverManager(),
    }
}
```

### gRPCストリーミングの種類と特徴

gRPCには4つの通信パターンがあります：

1. **Unary（単項）**: 1リクエスト → 1レスポンス（通常のRPC）
2. **Server Streaming**: 1リクエスト → Nレスポンス（本日の課題）
3. **Client Streaming**: Nリクエスト → 1レスポンス
4. **Bidirectional Streaming**: Nリクエスト ↔ Nレスポンス

### サーバーサイドストリーミングとは

サーバーサイドストリーミングは、クライアントが一つのリクエストを送信し、サーバーが複数のレスポンスを**順次・連続的に**返すgRPCの通信パターンです。

```
Client                    Server
   |                         |
   |-------- Request ------->|
   |                         |
   |<------- Response 1 -----|
   |<------- Response 2 -----|
   |<------- Response 3 -----|
   |<------- ... ------------|
   |<------- Response N -----|
   |<------- EOF ------------|
```

**従来のHTTP APIとの比較：**

```go
// HTTP REST API（問題のあるアプローチ）
func GetLargeDataHTTP(w http.ResponseWriter, r *http.Request) {
    // すべてのデータを一度にメモリに読み込み
    data := fetchAllData() // 10GB のデータ
    
    // JSON に変換（メモリ使用量が2倍に）
    jsonData, _ := json.Marshal(data)
    
    // 一度に送信（クライアントも一度に受信待ち）
    w.Write(jsonData)
}

// gRPC Server Streaming（効率的なアプローチ）
func (s *Server) StreamLargeData(req *pb.DataRequest, stream pb.DataService_StreamLargeDataServer) error {
    // データを小さなチャンクで順次送信
    for chunk := range fetchDataInChunks(req.GetQuery()) {
        response := &pb.DataChunk{
            Data: chunk,
            ChunkId: chunkID,
            TotalChunks: totalChunks,
        }
        
        if err := stream.Send(response); err != nil {
            return err
        }
        // メモリ使用量は常に一定
    }
    return nil
}
```

### 主な用途と実際の活用例

#### 1. リアルタイム通知システム

```go
// プロトコルバッファ定義
syntax = "proto3";

package notification;

service NotificationService {
    rpc SubscribeToNotifications(SubscriptionRequest) returns (stream Notification);
}

message SubscriptionRequest {
    string user_id = 1;
    repeated string event_types = 2;
    int32 priority_level = 3;
}

message Notification {
    string id = 1;
    string type = 2;
    string title = 3;
    string message = 4;
    int64 timestamp = 5;
    int32 priority = 6;
    map<string, string> metadata = 7;
}

// サーバー実装
type NotificationServer struct {
    pb.UnimplementedNotificationServiceServer
    subscribers map[string]chan *pb.Notification
    mu          sync.RWMutex
}

func (s *NotificationServer) SubscribeToNotifications(
    req *pb.SubscriptionRequest,
    stream pb.NotificationService_SubscribeToNotificationsServer,
) error {
    userID := req.GetUserId()
    
    // サブスクリプション登録
    notifyChan := make(chan *pb.Notification, 100)
    s.mu.Lock()
    s.subscribers[userID] = notifyChan
    s.mu.Unlock()
    
    // クリーンアップ
    defer func() {
        s.mu.Lock()
        delete(s.subscribers, userID)
        close(notifyChan)
        s.mu.Unlock()
    }()
    
    // 通知をストリーミング
    for {
        select {
        case notification := <-notifyChan:
            if err := stream.Send(notification); err != nil {
                return err
            }
        case <-stream.Context().Done():
            return stream.Context().Err()
        }
    }
}

// 通知の配信
func (s *NotificationServer) BroadcastNotification(notification *pb.Notification) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    for userID, ch := range s.subscribers {
        select {
        case ch <- notification:
            // 送信成功
        default:
            // チャネルがフル→古い通知を破棄
            log.Printf("Notification queue full for user %s", userID)
        }
    }
}
```

#### 2. ログストリーミングシステム

```go
// ログストリーミング用のプロトコル定義
service LogService {
    rpc StreamLogs(LogStreamRequest) returns (stream LogEntry);
}

message LogStreamRequest {
    string application = 1;
    string log_level = 2;
    int64 start_timestamp = 3;
    repeated string filters = 4;
    bool follow = 5; // tail -f のような機能
}

message LogEntry {
    int64 timestamp = 1;
    string level = 2;
    string application = 3;
    string message = 4;
    string source_file = 5;
    int32 line_number = 6;
    map<string, string> fields = 7;
}

// ログストリーミング実装
type LogStreamServer struct {
    pb.UnimplementedLogServiceServer
    logBuffer *CircularBuffer
    tailMode  map[string]chan *pb.LogEntry
    mu        sync.RWMutex
}

func (s *LogStreamServer) StreamLogs(
    req *pb.LogStreamRequest,
    stream pb.LogService_StreamLogsServer,
) error {
    // 過去のログを送信
    historicalLogs := s.getHistoricalLogs(req)
    for _, logEntry := range historicalLogs {
        if err := stream.Send(logEntry); err != nil {
            return err
        }
    }
    
    // リアルタイムフォローモードが有効な場合
    if req.GetFollow() {
        return s.followLogs(req, stream)
    }
    
    return nil
}

func (s *LogStreamServer) followLogs(
    req *pb.LogStreamRequest,
    stream pb.LogService_StreamLogsServer,
) error {
    logChan := make(chan *pb.LogEntry, 1000)
    streamID := generateStreamID()
    
    s.mu.Lock()
    s.tailMode[streamID] = logChan
    s.mu.Unlock()
    
    defer func() {
        s.mu.Lock()
        delete(s.tailMode, streamID)
        close(logChan)
        s.mu.Unlock()
    }()
    
    for {
        select {
        case logEntry := <-logChan:
            if s.matchesFilter(logEntry, req) {
                if err := stream.Send(logEntry); err != nil {
                    return err
                }
            }
        case <-stream.Context().Done():
            return stream.Context().Err()
        }
    }
}

// 新しいログエントリを配信
func (s *LogStreamServer) DistributeLogEntry(entry *pb.LogEntry) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    for _, ch := range s.tailMode {
        select {
        case ch <- entry:
        default:
            // バッファフル時はスキップ
        }
    }
}
```

#### 3. ファイル転送システム

```go
// ファイル転送用プロトコル
service FileService {
    rpc DownloadFile(FileRequest) returns (stream FileChunk);
    rpc GetFileInfo(FileRequest) returns (FileInfo);
}

message FileRequest {
    string file_path = 1;
    int32 chunk_size = 2;
    int64 offset = 3;
    int64 limit = 4;
}

message FileChunk {
    string file_path = 1;
    int64 offset = 2;
    bytes data = 3;
    int64 total_size = 4;
    string checksum = 5;
    bool is_last_chunk = 6;
}

message FileInfo {
    string file_path = 1;
    int64 size = 2;
    int64 modified_time = 3;
    string mime_type = 4;
    string checksum = 5;
}

// ファイル転送実装
type FileServer struct {
    pb.UnimplementedFileServiceServer
    maxChunkSize int
    rateLimiter  *rate.Limiter
}

func NewFileServer(maxChunkSize int, rateLimit rate.Limit) *FileServer {
    return &FileServer{
        maxChunkSize: maxChunkSize,
        rateLimiter:  rate.NewLimiter(rateLimit, int(rateLimit)),
    }
}

func (s *FileServer) DownloadFile(
    req *pb.FileRequest,
    stream pb.FileService_DownloadFileServer,
) error {
    filePath := req.GetFilePath()
    
    // セキュリティチェック
    if err := s.validateFilePath(filePath); err != nil {
        return status.Errorf(codes.InvalidArgument, "Invalid file path: %v", err)
    }
    
    file, err := os.Open(filePath)
    if err != nil {
        return status.Errorf(codes.NotFound, "File not found: %v", err)
    }
    defer file.Close()
    
    fileInfo, err := file.Stat()
    if err != nil {
        return status.Errorf(codes.Internal, "Failed to get file info: %v", err)
    }
    
    // チャンクサイズの決定
    chunkSize := req.GetChunkSize()
    if chunkSize <= 0 || chunkSize > int32(s.maxChunkSize) {
        chunkSize = int32(s.maxChunkSize)
    }
    
    // オフセットとリミットの処理
    offset := req.GetOffset()
    limit := req.GetLimit()
    if limit <= 0 {
        limit = fileInfo.Size() - offset
    }
    
    if _, err := file.Seek(offset, 0); err != nil {
        return status.Errorf(codes.InvalidArgument, "Invalid offset: %v", err)
    }
    
    buffer := make([]byte, chunkSize)
    bytesRemaining := limit
    currentOffset := offset
    hasher := sha256.New()
    
    for bytesRemaining > 0 {
        // レート制限
        if err := s.rateLimiter.Wait(stream.Context()); err != nil {
            return err
        }
        
        // 読み取りサイズの調整
        readSize := int64(chunkSize)
        if bytesRemaining < readSize {
            readSize = bytesRemaining
            buffer = buffer[:readSize]
        }
        
        n, err := file.Read(buffer[:readSize])
        if err != nil && err != io.EOF {
            return status.Errorf(codes.Internal, "Failed to read file: %v", err)
        }
        
        if n == 0 {
            break
        }
        
        data := buffer[:n]
        hasher.Write(data)
        
        chunk := &pb.FileChunk{
            FilePath:     filePath,
            Offset:       currentOffset,
            Data:         data,
            TotalSize:    fileInfo.Size(),
            IsLastChunk:  bytesRemaining-int64(n) <= 0,
        }
        
        if chunk.IsLastChunk {
            chunk.Checksum = fmt.Sprintf("%x", hasher.Sum(nil))
        }
        
        if err := stream.Send(chunk); err != nil {
            return err
        }
        
        currentOffset += int64(n)
        bytesRemaining -= int64(n)
        
        if err == io.EOF {
            break
        }
    }
    
    return nil
}

func (s *FileServer) validateFilePath(path string) error {
    // パストラバーサル攻撃を防ぐ
    if strings.Contains(path, "..") {
        return fmt.Errorf("path traversal detected")
    }
    
    // 許可されたディレクトリのみアクセス可能
    allowedDir := "/var/files/"
    absPath, err := filepath.Abs(path)
    if err != nil {
        return err
    }
    
    if !strings.HasPrefix(absPath, allowedDir) {
        return fmt.Errorf("access denied")
    }
    
    return nil
}
```

#### 4. メトリクス監視システム

```go
// メトリクス監視用プロトコル
service MetricsService {
    rpc StreamMetrics(MetricsRequest) returns (stream MetricPoint);
}

message MetricsRequest {
    repeated string metric_names = 1;
    int32 interval_seconds = 2;
    map<string, string> labels = 3;
}

message MetricPoint {
    string name = 1;
    double value = 2;
    int64 timestamp = 3;
    map<string, string> labels = 4;
    string metric_type = 5; // counter, gauge, histogram
}

// メトリクス監視実装
type MetricsServer struct {
    pb.UnimplementedMetricsServiceServer
    collector *MetricsCollector
    streams   map[string]*MetricStream
    mu        sync.RWMutex
}

type MetricStream struct {
    ctx     context.Context
    cancel  context.CancelFunc
    stream  pb.MetricsService_StreamMetricsServer
    config  *pb.MetricsRequest
    lastSent map[string]int64
}

func (s *MetricsServer) StreamMetrics(
    req *pb.MetricsRequest,
    stream pb.MetricsService_StreamMetricsServer,
) error {
    ctx, cancel := context.WithCancel(stream.Context())
    defer cancel()
    
    streamID := generateStreamID()
    metricStream := &MetricStream{
        ctx:      ctx,
        cancel:   cancel,
        stream:   stream,
        config:   req,
        lastSent: make(map[string]int64),
    }
    
    s.mu.Lock()
    s.streams[streamID] = metricStream
    s.mu.Unlock()
    
    defer func() {
        s.mu.Lock()
        delete(s.streams, streamID)
        s.mu.Unlock()
    }()
    
    interval := time.Duration(req.GetIntervalSeconds()) * time.Second
    if interval < time.Second {
        interval = time.Second
    }
    
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if err := s.sendMetrics(metricStream); err != nil {
                return err
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (s *MetricsServer) sendMetrics(ms *MetricStream) error {
    metrics := s.collector.GetMetrics(ms.config.GetMetricNames(), ms.config.GetLabels())
    
    for _, metric := range metrics {
        // 重複送信を避けるため、タイムスタンプをチェック
        if lastSent, exists := ms.lastSent[metric.Name]; exists && metric.Timestamp <= lastSent {
            continue
        }
        
        if err := ms.stream.Send(metric); err != nil {
            return err
        }
        
        ms.lastSent[metric.Name] = metric.Timestamp
    }
    
    return nil
}
```

### 高度なストリーミング機能

#### 1. フロー制御とバックプレッシャー

```go
type FlowControlledStream struct {
    stream     pb.DataService_StreamDataServer
    windowSize int
    pending    int
    mu         sync.Mutex
    cond       *sync.Cond
}

func NewFlowControlledStream(stream pb.DataService_StreamDataServer, windowSize int) *FlowControlledStream {
    fcs := &FlowControlledStream{
        stream:     stream,
        windowSize: windowSize,
    }
    fcs.cond = sync.NewCond(&fcs.mu)
    return fcs
}

func (fcs *FlowControlledStream) Send(data *pb.DataResponse) error {
    fcs.mu.Lock()
    
    // ウィンドウサイズに達したら待機
    for fcs.pending >= fcs.windowSize {
        select {
        case <-fcs.stream.Context().Done():
            fcs.mu.Unlock()
            return fcs.stream.Context().Err()
        default:
            fcs.cond.Wait()
        }
    }
    
    fcs.pending++
    fcs.mu.Unlock()
    
    // 非同期で送信
    go func() {
        defer func() {
            fcs.mu.Lock()
            fcs.pending--
            fcs.cond.Signal()
            fcs.mu.Unlock()
        }()
        
        fcs.stream.Send(data)
    }()
    
    return nil
}
```

#### 2. エラー処理と再接続

```go
// エラー処理を含むストリーミング
func (s *Server) RobustStreamData(
    req *pb.StreamRequest,
    stream pb.DataService_RobustStreamDataServer,
) error {
    ctx := stream.Context()
    
    // ハートビート送信
    heartbeatTicker := time.NewTicker(30 * time.Second)
    defer heartbeatTicker.Stop()
    
    errorChan := make(chan error, 1)
    
    // データ送信Goroutine
    go func() {
        defer close(errorChan)
        
        for data := range s.generateData(ctx, req) {
            response := &pb.DataResponse{
                Data:      data,
                Timestamp: time.Now().Unix(),
            }
            
            if err := stream.Send(response); err != nil {
                errorChan <- err
                return
            }
        }
    }()
    
    // ハートビートとエラー監視
    for {
        select {
        case <-heartbeatTicker.C:
            // ハートビート送信
            heartbeat := &pb.DataResponse{
                Data:        "",
                Timestamp:   time.Now().Unix(),
                IsHeartbeat: true,
            }
            if err := stream.Send(heartbeat); err != nil {
                return err
            }
            
        case err := <-errorChan:
            if err != nil {
                // エラー詳細をログに記録
                log.Printf("Stream error: %v", err)
                return err
            }
            return nil // 正常終了
            
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

#### 3. クライアント実装例

```go
// クライアント側の実装
type StreamingClient struct {
    client pb.DataServiceClient
    conn   *grpc.ClientConn
}

func NewStreamingClient(address string) (*StreamingClient, error) {
    conn, err := grpc.Dial(address, grpc.WithInsecure())
    if err != nil {
        return nil, err
    }
    
    return &StreamingClient{
        client: pb.NewDataServiceClient(conn),
        conn:   conn,
    }, nil
}

func (c *StreamingClient) ReceiveStream(ctx context.Context, req *pb.StreamRequest) error {
    stream, err := c.client.StreamData(ctx, req)
    if err != nil {
        return err
    }
    
    for {
        response, err := stream.Recv()
        if err == io.EOF {
            log.Println("Stream ended normally")
            break
        }
        if err != nil {
            return fmt.Errorf("stream error: %w", err)
        }
        
        // ハートビートはスキップ
        if response.GetIsHeartbeat() {
            continue
        }
        
        // データ処理
        if err := c.processData(response); err != nil {
            log.Printf("Error processing data: %v", err)
            continue
        }
    }
    
    return nil
}

func (c *StreamingClient) ReceiveStreamWithRetry(ctx context.Context, req *pb.StreamRequest) error {
    maxRetries := 3
    backoff := time.Second
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := c.ReceiveStream(ctx, req)
        if err == nil {
            return nil
        }
        
        // 致命的でないエラーの場合は再試行
        if isRetryableError(err) && attempt < maxRetries-1 {
            log.Printf("Stream failed (attempt %d/%d): %v. Retrying in %v...", 
                attempt+1, maxRetries, err, backoff)
            
            select {
            case <-time.After(backoff):
                backoff *= 2 // 指数バックオフ
            case <-ctx.Done():
                return ctx.Err()
            }
            continue
        }
        
        return err
    }
    
    return fmt.Errorf("stream failed after %d attempts", maxRetries)
}

func isRetryableError(err error) bool {
    if err == nil {
        return false
    }
    
    // gRPCエラーコードによる判定
    st, ok := status.FromError(err)
    if !ok {
        return false
    }
    
    switch st.Code() {
    case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
        return true
    default:
        return false
    }
}
```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の機能を実装してください：

### 1. ログストリーミングサービス
- リアルタイムでログエントリを配信
- ログレベル、アプリケーション、時間範囲でのフィルタリング
- `tail -f`のようなフォローモード

### 2. ファイル転送サービス
- 大きなファイルを効率的に分割送信
- レート制限とフロー制御
- チェックサム検証
- 部分ダウンロード対応

### 3. メトリクス監視サービス
- システムメトリクスのリアルタイム配信
- 設定可能な監視間隔
- ラベルベースのフィルタリング

### 4. 通知配信サービス
- ユーザー別通知のリアルタイム配信
- 優先度別フィルタリング
- 通知キューの管理

**実装すべき関数：**

```go
// ログストリーミング
func (s *LogServer) StreamLogs(req *pb.LogStreamRequest, stream pb.LogService_StreamLogsServer) error

// ファイル転送
func (s *FileServer) DownloadFile(req *pb.FileRequest, stream pb.FileService_DownloadFileServer) error

// メトリクス監視
func (s *MetricsServer) StreamMetrics(req *pb.MetricsRequest, stream pb.MetricsService_StreamMetricsServer) error

// 通知配信
func (s *NotificationServer) SubscribeToNotifications(req *pb.SubscriptionRequest, stream pb.NotificationService_SubscribeToNotificationsServer) error
```

**重要な実装要件：**
- 適切なエラーハンドリングとリソース管理
- クライアント切断の検出と清理処理
- メモリ効率的なストリーミング
- レースコンディションの回避
- ハートビート機能による接続監視

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestLogStreaming
=== RUN   TestLogStreaming/Historical_logs
=== RUN   TestLogStreaming/Follow_mode
=== RUN   TestLogStreaming/Filter_by_level
--- PASS: TestLogStreaming (0.15s)

=== RUN   TestFileTransfer
=== RUN   TestFileTransfer/Complete_download
=== RUN   TestFileTransfer/Partial_download
=== RUN   TestFileTransfer/Checksum_verification
--- PASS: TestFileTransfer (0.25s)

=== RUN   TestMetricsStreaming
=== RUN   TestMetricsStreaming/Real_time_metrics
=== RUN   TestMetricsStreaming/Custom_interval
--- PASS: TestMetricsStreaming (0.20s)

PASS
```

### プログラム実行例
```bash
$ go run main.go
=== gRPC Server-side Streaming Demo ===

Starting log streaming server on :8080...
Log streaming ready - connect with: grpcurl -d '{"application":"myapp","follow":true}' localhost:8080 LogService/StreamLogs

File transfer server ready - download with: grpcurl -d '{"file_path":"test.txt","chunk_size":1024}' localhost:8080 FileService/DownloadFile

Metrics streaming ready - monitor with: grpcurl -d '{"metric_names":["cpu","memory"],"interval_seconds":5}' localhost:8080 MetricsService/StreamMetrics

Press Ctrl+C to stop...
```

### ベンチマーク結果例
```bash
$ go test -bench=.
BenchmarkLogStreaming-8          1000    1500000 ns/op    150 B/op    3 allocs/op
BenchmarkFileTransfer-8           500    3000000 ns/op   1024 B/op    2 allocs/op
BenchmarkMetricsStreaming-8      2000    1000000 ns/op    200 B/op    4 allocs/op
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的なストリーミング実装
```go
func (s *Server) StreamData(req *pb.Request, stream pb.Service_StreamDataServer) error {
    for i := 0; i < 10; i++ {
        response := &pb.Response{
            Data: fmt.Sprintf("Message %d", i),
        }
        
        if err := stream.Send(response); err != nil {
            return err
        }
        
        time.Sleep(100 * time.Millisecond)
    }
    return nil
}
```

### コンテキストキャンセルの処理
```go
func (s *Server) StreamWithCancellation(req *pb.Request, stream pb.Service_StreamDataServer) error {
    ctx := stream.Context()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // データ送信
            if err := stream.Send(response); err != nil {
                return err
            }
        }
    }
}
```

### エラー処理とクリーンアップ
```go
func (s *Server) StreamWithCleanup(req *pb.Request, stream pb.Service_StreamDataServer) error {
    // リソース初期化
    resource := acquireResource()
    defer func() {
        // 必ずクリーンアップ
        releaseResource(resource)
    }()
    
    // ストリーミング処理
    for data := range generateData() {
        if err := stream.Send(data); err != nil {
            log.Printf("Stream error: %v", err)
            return err
        }
    }
    
    return nil
}
```

## 実行方法

```bash
# プロトコルバッファのコンパイル
protoc --go_out=. --go-grpc_out=. *.proto

# テスト実行
go test -v

# ベンチマーク測定
go test -bench=.

# サーバー起動
go run main.go

# クライアントテスト（別ターミナルで）
grpcurl -d '{"application":"test"}' localhost:8080 LogService/StreamLogs
```

## 参考資料

- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)
- [Protocol Buffers Guide](https://developers.google.com/protocol-buffers)
- [gRPC Streaming Examples](https://github.com/grpc/grpc-go/tree/master/examples)
- [gRPC Error Handling](https://grpc.io/docs/guides/error/)