# Day 48: gRPC Client-side Streaming

## 🎯 本日の目標 (Today's Goal)

gRPCのクライアントサイドストリーミングを完全に理解し、実装する。大量データの効率的なアップロード、バッチ処理、リアルタイムデータ収集、ログ集約などの実用的なユースケースを通じて、高性能で堅牢なデータ送信システムを構築できるようになる。

## 📖 解説 (Explanation)

### クライアントサイドストリーミングとは

クライアントサイドストリーミングは、クライアントが**複数のリクエストを順次送信**し、サーバーが**単一のレスポンス**を返すgRPCの通信パターンです。これにより、大量のデータを効率的にサーバーに送信できます。

```
Client                    Server
   |                         |
   |-------- Request 1 ----->|
   |-------- Request 2 ----->|
   |-------- Request 3 ----->|
   |-------- ... ----------->|
   |-------- Request N ----->|
   |-------- EOF ----------->|
   |                         |
   |<------- Response -------|
```

**従来のHTTP APIとの比較：**

```go
// HTTP REST API（非効率なアプローチ）
func UploadDataHTTP(data []DataPoint) error {
    for _, point := range data {
        // 各データポイントで個別のHTTPリクエスト
        jsonData, _ := json.Marshal(point)
        resp, err := http.Post("/api/data", "application/json", bytes.NewReader(jsonData))
        if err != nil {
            return err
        }
        resp.Body.Close()
        // 1000個のデータポイント = 1000回のHTTPリクエスト
    }
    return nil
}

// gRPC Client Streaming（効率的なアプローチ）
func UploadDataGRPC(client pb.DataServiceClient, data []DataPoint) error {
    stream, err := client.CollectData(context.Background())
    if err != nil {
        return err
    }
    
    // 単一の接続で大量データを送信
    for _, point := range data {
        if err := stream.Send(&point); err != nil {
            return err
        }
    }
    
    // 単一のレスポンスを受信
    result, err := stream.CloseAndRecv()
    if err != nil {
        return err
    }
    
    log.Printf("Successfully uploaded %d data points", result.TotalProcessed)
    return nil
}
```

### 主な用途と実際の活用例

#### 1. ファイルアップロードシステム

```go
// プロトコルバッファ定義
syntax = "proto3";

package fileupload;

service FileUploadService {
    rpc UploadFile(stream FileChunk) returns (UploadResult);
}

message FileChunk {
    string file_name = 1;
    bytes data = 2;
    int64 offset = 3;
    int64 total_size = 4;
    bool is_last_chunk = 5;
    string checksum = 6;
}

message UploadResult {
    string file_id = 1;
    string file_name = 2;
    int64 total_size = 3;
    string checksum = 4;
    string status = 5;
    repeated string errors = 6;
}

// サーバー実装
type FileUploadServer struct {
    pb.UnimplementedFileUploadServiceServer
    uploadDir string
    maxFileSize int64
}

func (s *FileUploadServer) UploadFile(stream pb.FileUploadService_UploadFileServer) error {
    var (
        fileName    string
        totalSize   int64
        receivedSize int64
        file        *os.File
        hasher      hash.Hash
        chunks      []FileChunkInfo
    )
    
    hasher = sha256.New()
    
    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            // ファイル転送完了
            break
        }
        if err != nil {
            return status.Errorf(codes.Internal, "receive error: %v", err)
        }
        
        // 初回チャンクの処理
        if file == nil {
            fileName = chunk.GetFileName()
            totalSize = chunk.GetTotalSize()
            
            // セキュリティチェック
            if err := s.validateUpload(fileName, totalSize); err != nil {
                return status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
            }
            
            // 一時ファイル作成
            tempPath := filepath.Join(s.uploadDir, fmt.Sprintf("upload_%s_%d", fileName, time.Now().Unix()))
            file, err = os.Create(tempPath)
            if err != nil {
                return status.Errorf(codes.Internal, "failed to create file: %v", err)
            }
            defer file.Close()
        }
        
        // チャンクの検証
        if chunk.GetOffset() != receivedSize {
            return status.Errorf(codes.InvalidArgument, 
                "chunk offset mismatch: expected %d, got %d", receivedSize, chunk.GetOffset())
        }
        
        // データ書き込み
        data := chunk.GetData()
        n, err := file.Write(data)
        if err != nil {
            return status.Errorf(codes.Internal, "write error: %v", err)
        }
        
        receivedSize += int64(n)
        hasher.Write(data)
        
        // チャンク情報を記録
        chunks = append(chunks, FileChunkInfo{
            Offset: chunk.GetOffset(),
            Size:   int64(len(data)),
            Checksum: fmt.Sprintf("%x", sha256.Sum256(data)),
        })
        
        // サイズ制限チェック
        if receivedSize > s.maxFileSize {
            return status.Errorf(codes.ResourceExhausted, 
                "file size exceeds limit: %d > %d", receivedSize, s.maxFileSize)
        }
        
        // 最終チャンクの処理
        if chunk.GetIsLastChunk() {
            break
        }
    }
    
    // ファイル整合性チェック
    if receivedSize != totalSize {
        return status.Errorf(codes.DataLoss, 
            "size mismatch: expected %d, received %d", totalSize, receivedSize)
    }
    
    // チェックサム検証
    finalChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
    
    // ファイルの最終処理
    fileID := s.processUploadedFile(fileName, file.Name(), chunks)
    
    // レスポンス送信
    result := &pb.UploadResult{
        FileId:    fileID,
        FileName:  fileName,
        TotalSize: receivedSize,
        Checksum:  finalChecksum,
        Status:    "SUCCESS",
    }
    
    return stream.SendAndClose(result)
}

type FileChunkInfo struct {
    Offset   int64
    Size     int64
    Checksum string
}

func (s *FileUploadServer) validateUpload(fileName string, size int64) error {
    // ファイル名の検証
    if fileName == "" {
        return fmt.Errorf("file name is required")
    }
    
    // パストラバーサル攻撃を防ぐ
    if strings.Contains(fileName, "..") || strings.Contains(fileName, "/") {
        return fmt.Errorf("invalid file name")
    }
    
    // サイズ制限
    if size > s.maxFileSize {
        return fmt.Errorf("file size too large: %d > %d", size, s.maxFileSize)
    }
    
    // 拡張子チェック
    allowedExts := []string{".txt", ".csv", ".json", ".xml"}
    ext := strings.ToLower(filepath.Ext(fileName))
    for _, allowed := range allowedExts {
        if ext == allowed {
            return nil
        }
    }
    
    return fmt.Errorf("file type not allowed: %s", ext)
}

func (s *FileUploadServer) processUploadedFile(fileName, tempPath string, chunks []FileChunkInfo) string {
    fileID := fmt.Sprintf("file_%s_%d", fileName, time.Now().Unix())
    
    // 最終的なファイルパスに移動
    finalPath := filepath.Join(s.uploadDir, fileID)
    os.Rename(tempPath, finalPath)
    
    // メタデータを保存
    metadata := FileMetadata{
        FileID:    fileID,
        FileName:  fileName,
        Path:      finalPath,
        Chunks:    chunks,
        UploadTime: time.Now(),
    }
    s.saveMetadata(metadata)
    
    return fileID
}
```

#### 2. バッチデータ収集システム

```go
// バッチデータ収集用プロトコル
service DataCollectionService {
    rpc CollectDataPoints(stream DataPoint) returns (CollectionResult);
    rpc CollectLogs(stream LogEntry) returns (LogCollectionResult);
}

message DataPoint {
    string source = 1;
    int64 timestamp = 2;
    map<string, double> metrics = 3;
    map<string, string> tags = 4;
}

message LogEntry {
    string application = 1;
    string level = 2;
    int64 timestamp = 3;
    string message = 4;
    map<string, string> fields = 5;
}

message CollectionResult {
    int32 total_processed = 1;
    int32 successful_count = 2;
    int32 error_count = 3;
    repeated string errors = 4;
    int64 processing_time_ms = 5;
}

// データ収集サーバー実装
type DataCollectionServer struct {
    pb.UnimplementedDataCollectionServiceServer
    storage DataStorage
    validator DataValidator
    aggregator MetricsAggregator
}

func (s *DataCollectionServer) CollectDataPoints(stream pb.DataCollectionService_CollectDataPointsServer) error {
    startTime := time.Now()
    
    var (
        totalProcessed = 0
        successCount   = 0
        errorCount     = 0
        errors         []string
        batch          []*pb.DataPoint
        batchSize      = 100 // バッチ処理サイズ
    )
    
    for {
        dataPoint, err := stream.Recv()
        if err == io.EOF {
            // 最後のバッチを処理
            if len(batch) > 0 {
                batchResults := s.processBatch(batch)
                successCount += batchResults.SuccessCount
                errorCount += batchResults.ErrorCount
                errors = append(errors, batchResults.Errors...)
            }
            break
        }
        if err != nil {
            return status.Errorf(codes.Internal, "receive error: %v", err)
        }
        
        totalProcessed++
        
        // バリデーション
        if err := s.validator.ValidateDataPoint(dataPoint); err != nil {
            errorCount++
            errors = append(errors, fmt.Sprintf("validation error for point %d: %v", totalProcessed, err))
            continue
        }
        
        batch = append(batch, dataPoint)
        
        // バッチサイズに達したら処理
        if len(batch) >= batchSize {
            batchResults := s.processBatch(batch)
            successCount += batchResults.SuccessCount
            errorCount += batchResults.ErrorCount
            errors = append(errors, batchResults.Errors...)
            batch = batch[:0] // バッチをクリア
        }
    }
    
    processingTime := time.Since(startTime)
    
    result := &pb.CollectionResult{
        TotalProcessed:   int32(totalProcessed),
        SuccessfulCount:  int32(successCount),
        ErrorCount:       int32(errorCount),
        Errors:          errors,
        ProcessingTimeMs: processingTime.Milliseconds(),
    }
    
    return stream.SendAndClose(result)
}

type BatchResult struct {
    SuccessCount int
    ErrorCount   int
    Errors       []string
}

func (s *DataCollectionServer) processBatch(batch []*pb.DataPoint) BatchResult {
    var result BatchResult
    
    // バッチ単位でデータベースに保存
    if err := s.storage.SaveDataPointsBatch(batch); err != nil {
        result.ErrorCount = len(batch)
        result.Errors = append(result.Errors, fmt.Sprintf("batch save error: %v", err))
        return result
    }
    
    // メトリクス集約
    for _, point := range batch {
        if err := s.aggregator.AggregateMetrics(point); err != nil {
            result.ErrorCount++
            result.Errors = append(result.Errors, fmt.Sprintf("aggregation error: %v", err))
        } else {
            result.SuccessCount++
        }
    }
    
    return result
}
```

#### 3. リアルタイムログ収集システム

```go
// ログ収集用実装
func (s *DataCollectionServer) CollectLogs(stream pb.DataCollectionService_CollectLogsServer) error {
    var (
        logBuffer   []*pb.LogEntry
        flushTimer  = time.NewTicker(5 * time.Second) // 5秒毎にフラッシュ
        totalLogs   = 0
        indexedLogs = 0
    )
    defer flushTimer.Stop()
    
    // 非同期でバッファをフラッシュ
    flushChan := make(chan bool, 1)
    go func() {
        for {
            select {
            case <-flushTimer.C:
                if len(logBuffer) > 0 {
                    select {
                    case flushChan <- true:
                    default: // フラッシュ中の場合はスキップ
                    }
                }
            }
        }
    }()
    
    for {
        select {
        case <-flushChan:
            // バッファのフラッシュ
            if len(logBuffer) > 0 {
                indexed := s.indexLogs(logBuffer)
                indexedLogs += indexed
                logBuffer = logBuffer[:0]
            }
            
        default:
            logEntry, err := stream.Recv()
            if err == io.EOF {
                // 最後のバッファをフラッシュ
                if len(logBuffer) > 0 {
                    indexed := s.indexLogs(logBuffer)
                    indexedLogs += indexed
                }
                
                result := &pb.LogCollectionResult{
                    TotalReceived: int32(totalLogs),
                    IndexedCount:  int32(indexedLogs),
                    Status:       "SUCCESS",
                }
                return stream.SendAndClose(result)
            }
            if err != nil {
                return status.Errorf(codes.Internal, "receive error: %v", err)
            }
            
            totalLogs++
            logBuffer = append(logBuffer, logEntry)
            
            // バッファサイズ制限
            if len(logBuffer) >= 1000 {
                indexed := s.indexLogs(logBuffer)
                indexedLogs += indexed
                logBuffer = logBuffer[:0]
            }
        }
    }
}

func (s *DataCollectionServer) indexLogs(logs []*pb.LogEntry) int {
    indexed := 0
    for _, log := range logs {
        if err := s.storage.IndexLogEntry(log); err == nil {
            indexed++
        }
    }
    return indexed
}
```

#### 4. クライアント実装例

```go
// 高度なクライアント実装
type StreamingClient struct {
    client     pb.DataCollectionServiceClient
    conn       *grpc.ClientConn
    rateLimiter *rate.Limiter
}

func NewStreamingClient(address string, rateLimit rate.Limit) (*StreamingClient, error) {
    conn, err := grpc.Dial(address, 
        grpc.WithInsecure(),
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time:    30 * time.Second,
            Timeout: 5 * time.Second,
        }),
    )
    if err != nil {
        return nil, err
    }
    
    return &StreamingClient{
        client:      pb.NewDataCollectionServiceClient(conn),
        conn:        conn,
        rateLimiter: rate.NewLimiter(rateLimit, int(rateLimit)),
    }, nil
}

// 並行ファイルアップロード
func (c *StreamingClient) UploadFilesConcurrently(files []string, maxConcurrency int) error {
    semaphore := make(chan struct{}, maxConcurrency)
    errChan := make(chan error, len(files))
    
    var wg sync.WaitGroup
    
    for _, filePath := range files {
        wg.Add(1)
        go func(path string) {
            defer wg.Done()
            
            semaphore <- struct{}{} // セマフォ取得
            defer func() { <-semaphore }() // セマフォ解放
            
            if err := c.UploadFile(path); err != nil {
                errChan <- fmt.Errorf("failed to upload %s: %w", path, err)
            }
        }(filePath)
    }
    
    wg.Wait()
    close(errChan)
    
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("upload errors: %v", errors)
    }
    
    return nil
}

// レート制限付きファイルアップロード
func (c *StreamingClient) UploadFile(filePath string) error {
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    fileInfo, err := file.Stat()
    if err != nil {
        return err
    }
    
    stream, err := c.client.UploadFile(context.Background())
    if err != nil {
        return err
    }
    
    buffer := make([]byte, 32*1024) // 32KB chunks
    offset := int64(0)
    totalSize := fileInfo.Size()
    fileName := filepath.Base(filePath)
    
    for {
        // レート制限適用
        if err := c.rateLimiter.Wait(context.Background()); err != nil {
            return err
        }
        
        n, err := file.Read(buffer)
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
        
        chunk := &pb.FileChunk{
            FileName:     fileName,
            Data:         buffer[:n],
            Offset:       offset,
            TotalSize:    totalSize,
            IsLastChunk:  offset+int64(n) >= totalSize,
        }
        
        if err := stream.Send(chunk); err != nil {
            return err
        }
        
        offset += int64(n)
    }
    
    result, err := stream.CloseAndRecv()
    if err != nil {
        return err
    }
    
    log.Printf("Upload completed: %s (ID: %s)", result.FileName, result.FileId)
    return nil
}

// データポイントのストリーミング送信
func (c *StreamingClient) SendDataPoints(ctx context.Context, dataPoints <-chan *pb.DataPoint) (*pb.CollectionResult, error) {
    stream, err := c.client.CollectDataPoints(ctx)
    if err != nil {
        return nil, err
    }
    
    // 送信統計
    var sentCount int
    startTime := time.Now()
    
    // 進行状況レポート
    progressTicker := time.NewTicker(10 * time.Second)
    defer progressTicker.Stop()
    
    go func() {
        for {
            select {
            case <-progressTicker.C:
                log.Printf("Sent %d data points in %v", sentCount, time.Since(startTime))
            case <-ctx.Done():
                return
            }
        }
    }()
    
    // データポイントを送信
    for {
        select {
        case dataPoint, ok := <-dataPoints:
            if !ok {
                // チャネルクローズ = 送信完了
                result, err := stream.CloseAndRecv()
                if err != nil {
                    return nil, err
                }
                
                log.Printf("Stream completed: sent %d, processed %d", 
                    sentCount, result.TotalProcessed)
                return result, nil
            }
            
            if err := stream.Send(dataPoint); err != nil {
                return nil, err
            }
            sentCount++
            
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }
}
```

### エラーハンドリングと再接続

```go
// 堅牢なストリーミングクライアント
func (c *StreamingClient) UploadWithRetry(filePath string, maxRetries int) error {
    backoff := time.Second
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := c.UploadFile(filePath)
        if err == nil {
            return nil
        }
        
        // リトライ可能なエラーか判定
        if !isRetryableError(err) {
            return err
        }
        
        if attempt < maxRetries-1 {
            log.Printf("Upload failed (attempt %d/%d): %v. Retrying in %v...", 
                attempt+1, maxRetries, err, backoff)
            
            time.Sleep(backoff)
            backoff *= 2
        }
    }
    
    return fmt.Errorf("upload failed after %d attempts", maxRetries)
}

func isRetryableError(err error) bool {
    if err == nil {
        return false
    }
    
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

// 接続状態監視
func (c *StreamingClient) MonitorConnection() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            state := c.conn.GetState()
            log.Printf("Connection state: %v", state)
            
            if state == connectivity.TransientFailure {
                log.Println("Connection in failure state, attempting to reconnect...")
                c.conn.Connect()
            }
        }
    }
}
```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の機能を実装してください：

### 1. ファイルアップロードサービス
- 大きなファイルを効率的に分割アップロード
- チェックサム検証とデータ整合性確認
- アップロード進行状況の監視
- セキュリティチェック（ファイル名、サイズ、タイプ）

### 2. データ収集サービス
- 大量データポイントのバッチ収集
- リアルタイムデータの効率的な処理
- メトリクス集約と統計情報生成
- エラー処理とデータ検証

### 3. ログ集約サービス
- 複数のログエントリを一括収集
- ログフィルタリングと分類
- インデックス作成と検索準備
- バッファリングと定期フラッシュ

### 4. バッチ処理システム
- 大量データの効率的な処理
- レート制限とフロー制御
- 並行アップロード機能
- 進行状況監視とレポート

**実装すべき関数：**

```go
// ファイルアップロード
func (s *FileUploadServer) UploadFile(stream pb.FileUploadService_UploadFileServer) error

// データ収集
func (s *DataCollectionServer) CollectDataPoints(stream pb.DataCollectionService_CollectDataPointsServer) error

// ログ集約
func (s *DataCollectionServer) CollectLogs(stream pb.DataCollectionService_CollectLogsServer) error

// クライアント実装
func (c *StreamingClient) UploadFile(filePath string) error
func (c *StreamingClient) SendDataPoints(ctx context.Context, dataPoints <-chan *pb.DataPoint) (*pb.CollectionResult, error)
```

**重要な実装要件：**
- メモリ効率的なストリーミング処理
- 適切なエラーハンドリングとバリデーション
- レート制限とフロー制御
- データ整合性の保証
- 並行処理の安全性

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestFileUpload
=== RUN   TestFileUpload/Small_file_upload
=== RUN   TestFileUpload/Large_file_upload
=== RUN   TestFileUpload/Checksum_verification
--- PASS: TestFileUpload (0.30s)

=== RUN   TestDataCollection
=== RUN   TestDataCollection/Batch_data_points
=== RUN   TestDataCollection/Real_time_streaming
--- PASS: TestDataCollection (0.20s)

=== RUN   TestLogAggregation
=== RUN   TestLogAggregation/Log_collection
=== RUN   TestLogAggregation/Filtering_and_indexing
--- PASS: TestLogAggregation (0.15s)

PASS
```

### プログラム実行例
```bash
$ go run main.go
=== gRPC Client-side Streaming Demo ===

Starting servers on :8080...

File upload server ready
Data collection server ready
Log aggregation server ready

Testing file upload...
Uploading test_file.txt (10MB)...
Upload progress: 32KB/10MB (0.3%)
Upload progress: 1024KB/10MB (10.2%)
Upload progress: 5120KB/10MB (51.2%)
Upload completed: test_file.txt (ID: file_test_file.txt_1642597800)

Testing data collection...
Sending 10000 data points...
Batch 1: 100 points processed
Batch 50: 5000 points processed
Batch 100: 10000 points processed
Collection completed: 10000 total, 9985 successful, 15 errors

Press Ctrl+C to stop...
```

### ベンチマーク結果例
```bash
$ go test -bench=.
BenchmarkFileUpload-8        100    15000000 ns/op   1024 B/op    5 allocs/op
BenchmarkDataCollection-8   2000     1000000 ns/op    200 B/op    3 allocs/op
BenchmarkLogCollection-8    1500     1200000 ns/op    150 B/op    2 allocs/op
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的なクライアントストリーミング
```go
func (c *Client) SendData(data []DataPoint) error {
    stream, err := c.client.CollectData(context.Background())
    if err != nil {
        return err
    }
    
    for _, point := range data {
        if err := stream.Send(&point); err != nil {
            return err
        }
    }
    
    result, err := stream.CloseAndRecv()
    if err != nil {
        return err
    }
    
    log.Printf("Processed: %d", result.TotalProcessed)
    return nil
}
```

### サーバーサイドの受信処理
```go
func (s *Server) CollectData(stream pb.Service_CollectDataServer) error {
    var count int
    
    for {
        data, err := stream.Recv()
        if err == io.EOF {
            // ストリーム終了
            return stream.SendAndClose(&pb.Result{
                TotalProcessed: int32(count),
                Status:        "SUCCESS",
            })
        }
        if err != nil {
            return err
        }
        
        // データ処理
        if err := s.processData(data); err != nil {
            return err
        }
        count++
    }
}
```

### ファイルチャンクの送信
```go
func (c *Client) UploadFile(filePath string) error {
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    stream, err := c.client.UploadFile(context.Background())
    if err != nil {
        return err
    }
    
    buffer := make([]byte, 32*1024) // 32KB chunks
    for {
        n, err := file.Read(buffer)
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
        
        chunk := &pb.FileChunk{
            Data: buffer[:n],
        }
        
        if err := stream.Send(chunk); err != nil {
            return err
        }
    }
    
    result, err := stream.CloseAndRecv()
    return err
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
grpcurl -d '{"file_name":"test.txt"}' localhost:8080 FileUploadService/UploadFile
```

## 参考資料

- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)
- [Protocol Buffers Guide](https://developers.google.com/protocol-buffers)
- [gRPC Client-side Streaming](https://grpc.io/docs/what-is-grpc/core-concepts/#client-streaming-rpc)
- [gRPC Error Handling](https://grpc.io/docs/guides/error/)