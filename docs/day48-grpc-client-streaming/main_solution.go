// Day 48: gRPC Client-side Streaming
// クライアントからサーバーへの連続データ送信の実装

package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// データ構造定義
type DataPoint struct {
	ID        string  `json:"id"`
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
	Source    string  `json:"source"`
}

type CollectionResult struct {
	TotalPoints   int32  `json:"total_points"`
	ProcessedAt   int64  `json:"processed_at"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

type LogEntry struct {
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	Service   string `json:"service"`
}

type LogCollectionResult struct {
	TotalLogs   int32  `json:"total_logs"`
	ProcessedAt int64  `json:"processed_at"`
	Status      string `json:"status"`
}

type FileChunk struct {
	ChunkID  int32  `json:"chunk_id"`
	Data     []byte `json:"data"`
	Filename string `json:"filename"`
	IsLast   bool   `json:"is_last"`
}

type FileUploadResult struct {
	Filename    string `json:"filename"`
	TotalSize   int64  `json:"total_size"`
	TotalChunks int32  `json:"total_chunks"`
	ProcessedAt int64  `json:"processed_at"`
	Status      string `json:"status"`
}

// ストリームインターフェース（モック実装）
type DataCollectorStreamClient interface {
	Send(*DataPoint) error
	CloseAndRecv() (*CollectionResult, error)
	Context() context.Context
}

type LogCollectorStreamClient interface {
	Send(*LogEntry) error
	CloseAndRecv() (*LogCollectionResult, error)
	Context() context.Context
}

type FileUploaderStreamClient interface {
	Send(*FileChunk) error
	CloseAndRecv() (*FileUploadResult, error)
	Context() context.Context
}

// サーバー実装
type StreamingServer struct {
	dataPoints    []*DataPoint
	logs          []*LogEntry
	uploadedFiles map[string][]byte
	mu            sync.RWMutex
}

func NewStreamingServer() *StreamingServer {
	return &StreamingServer{
		dataPoints:    make([]*DataPoint, 0),
		logs:          make([]*LogEntry, 0),
		uploadedFiles: make(map[string][]byte),
	}
}

// CollectData クライアントからのデータポイントストリームを受信し、処理
func (s *StreamingServer) CollectData(stream DataCollectorStreamClient) (*CollectionResult, error) {
	var count int32
	var dataPoints []*DataPoint

	// MockDataCollectorStreamの場合は、既に蓄積されたデータを処理
	if mockStream, ok := stream.(*MockDataCollectorStream); ok {
		dataPoints = mockStream.GetDataPoints()
	} else {
		// 実際のストリームからデータを受信
		for {
			dataPoint, err := s.receiveDataPoint(stream)
			if err == io.EOF {
				break
			}
			if err != nil {
				return &CollectionResult{
					TotalPoints:  count,
					ProcessedAt:  time.Now().Unix(),
					Status:       "ERROR",
					ErrorMessage: err.Error(),
				}, err
			}
			dataPoints = append(dataPoints, dataPoint)
		}
	}

	// データポイントの検証と保存
	for _, dataPoint := range dataPoints {
		if err := validateDataPoint(dataPoint); err != nil {
			return &CollectionResult{
				TotalPoints:  count,
				ProcessedAt:  time.Now().Unix(),
				Status:       "ERROR",
				ErrorMessage: fmt.Sprintf("validation failed: %v", err),
			}, err
		}

		s.mu.Lock()
		s.dataPoints = append(s.dataPoints, dataPoint)
		s.mu.Unlock()
		count++
	}

	return &CollectionResult{
		TotalPoints: count,
		ProcessedAt: time.Now().Unix(),
		Status:      "SUCCESS",
	}, nil
}

// CollectLogs クライアントからのログストリームを受信し、処理
func (s *StreamingServer) CollectLogs(stream LogCollectorStreamClient) (*LogCollectionResult, error) {
	var count int32
	var logs []*LogEntry

	// MockLogCollectorStreamの場合は、既に蓄積されたデータを処理
	if mockStream, ok := stream.(*MockLogCollectorStream); ok {
		logs = mockStream.GetLogs()
	} else {
		// 実際のストリームからログを受信
		for {
			log, err := s.receiveLog(stream)
			if err == io.EOF {
				break
			}
			if err != nil {
				return &LogCollectionResult{
					TotalLogs:   count,
					ProcessedAt: time.Now().Unix(),
					Status:      "ERROR",
				}, err
			}
			logs = append(logs, log)
		}
	}

	// ログの保存
	s.mu.Lock()
	for _, log := range logs {
		s.logs = append(s.logs, log)
		count++
	}
	s.mu.Unlock()

	return &LogCollectionResult{
		TotalLogs:   count,
		ProcessedAt: time.Now().Unix(),
		Status:      "SUCCESS",
	}, nil
}

// UploadFile クライアントからのファイルチャンクストリームを受信し、ファイルを再構築
func (s *StreamingServer) UploadFile(stream FileUploaderStreamClient) (*FileUploadResult, error) {
	var chunks []*FileChunk
	var filename string
	var totalChunks int32

	// MockFileUploaderStreamの場合は、既に蓄積されたチャンクを処理
	if mockStream, ok := stream.(*MockFileUploaderStream); ok {
		chunks = mockStream.GetChunks()
	} else {
		// 実際のストリームからチャンクを受信
		for {
			chunk, err := s.receiveFileChunk(stream)
			if err == io.EOF {
				break
			}
			if err != nil {
				return &FileUploadResult{
					Filename:    filename,
					TotalChunks: totalChunks,
					ProcessedAt: time.Now().Unix(),
					Status:      "ERROR",
				}, err
			}
			chunks = append(chunks, chunk)
		}
	}

	if len(chunks) == 0 {
		return &FileUploadResult{
			TotalChunks: 0,
			ProcessedAt: time.Now().Unix(),
			Status:      "ERROR",
		}, fmt.Errorf("no chunks received")
	}

	// ファイル再構築
	filename = chunks[0].Filename
	var fileData []byte

	for i, chunk := range chunks {
		if chunk.ChunkID != int32(i) {
			return &FileUploadResult{
				Filename:    filename,
				TotalChunks: int32(len(chunks)),
				ProcessedAt: time.Now().Unix(),
				Status:      "ERROR",
			}, fmt.Errorf("chunk sequence error: expected %d, got %d", i, chunk.ChunkID)
		}
		fileData = append(fileData, chunk.Data...)
		totalChunks++
	}

	// ファイル保存
	s.mu.Lock()
	s.uploadedFiles[filename] = fileData
	s.mu.Unlock()

	return &FileUploadResult{
		Filename:    filename,
		TotalSize:   int64(len(fileData)),
		TotalChunks: totalChunks,
		ProcessedAt: time.Now().Unix(),
		Status:      "SUCCESS",
	}, nil
}

// GetDataPoints 収集されたデータポイントを返す
func (s *StreamingServer) GetDataPoints() []*DataPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]*DataPoint, len(s.dataPoints))
	copy(result, s.dataPoints)
	return result
}

// GetLogs 収集されたログを返す
func (s *StreamingServer) GetLogs() []*LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make([]*LogEntry, len(s.logs))
	copy(result, s.logs)
	return result
}

// GetUploadedFile アップロードされたファイルを返す
func (s *StreamingServer) GetUploadedFile(filename string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	data, exists := s.uploadedFiles[filename]
	if !exists {
		return nil, false
	}
	
	result := make([]byte, len(data))
	copy(result, data)
	return result, true
}

// ヘルパーメソッド（実際のgRPCストリーム用）
func (s *StreamingServer) receiveDataPoint(stream DataCollectorStreamClient) (*DataPoint, error) {
	// 実際の実装では stream.Recv() を使用
	return nil, io.EOF
}

func (s *StreamingServer) receiveLog(stream LogCollectorStreamClient) (*LogEntry, error) {
	// 実際の実装では stream.Recv() を使用
	return nil, io.EOF
}

func (s *StreamingServer) receiveFileChunk(stream FileUploaderStreamClient) (*FileChunk, error) {
	// 実際の実装では stream.Recv() を使用
	return nil, io.EOF
}

// クライアント実装
type StreamingClient struct {
	server *StreamingServer
}

func NewStreamingClient(server *StreamingServer) *StreamingClient {
	return &StreamingClient{server: server}
}

// SendDataPoints データポイントの配列をストリームで送信
func (c *StreamingClient) SendDataPoints(ctx context.Context, dataPoints []*DataPoint) (*CollectionResult, error) {
	stream := NewMockDataCollectorStream(ctx, c.server)
	
	for _, dataPoint := range dataPoints {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if err := stream.Send(dataPoint); err != nil {
				return nil, fmt.Errorf("failed to send data point: %w", err)
			}
		}
	}
	
	return stream.CloseAndRecv()
}

// SendLogs ログエントリの配列をストリームで送信
func (c *StreamingClient) SendLogs(ctx context.Context, logs []*LogEntry) (*LogCollectionResult, error) {
	stream := NewMockLogCollectorStream(ctx, c.server)
	
	for _, log := range logs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if err := stream.Send(log); err != nil {
				return nil, fmt.Errorf("failed to send log: %w", err)
			}
		}
	}
	
	return stream.CloseAndRecv()
}

// UploadFile ファイルをチャンクに分割してストリームで送信
func (c *StreamingClient) UploadFile(ctx context.Context, filename string, data []byte, chunkSize int) (*FileUploadResult, error) {
	stream := NewMockFileUploaderStream(ctx, c.server)
	
	chunks := createFileChunks(filename, data, chunkSize)
	
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if err := stream.Send(chunk); err != nil {
				return nil, fmt.Errorf("failed to send chunk: %w", err)
			}
		}
	}
	
	return stream.CloseAndRecv()
}

// SendDataPointsWithCallback 送信進捗をコールバックで通知しながらデータを送信
func (c *StreamingClient) SendDataPointsWithCallback(ctx context.Context, dataPoints []*DataPoint, callback func(int, int)) (*CollectionResult, error) {
	stream := NewMockDataCollectorStream(ctx, c.server)
	
	total := len(dataPoints)
	for i, dataPoint := range dataPoints {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if err := stream.Send(dataPoint); err != nil {
				return nil, fmt.Errorf("failed to send data point: %w", err)
			}
			
			// 進捗を通知
			if callback != nil {
				callback(i+1, total)
			}
		}
	}
	
	return stream.CloseAndRecv()
}

// モックストリーム実装
type MockDataCollectorStream struct {
	dataPoints []*DataPoint
	ctx        context.Context
	server     *StreamingServer
	closed     bool
	mu         sync.Mutex
}

func NewMockDataCollectorStream(ctx context.Context, server *StreamingServer) *MockDataCollectorStream {
	return &MockDataCollectorStream{
		dataPoints: make([]*DataPoint, 0),
		ctx:        ctx,
		server:     server,
	}
}

func (m *MockDataCollectorStream) Send(dataPoint *DataPoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return fmt.Errorf("stream is closed")
	}
	
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	default:
		m.dataPoints = append(m.dataPoints, dataPoint)
		return nil
	}
}

func (m *MockDataCollectorStream) CloseAndRecv() (*CollectionResult, error) {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	
	return m.server.CollectData(m)
}

func (m *MockDataCollectorStream) Context() context.Context {
	return m.ctx
}

func (m *MockDataCollectorStream) GetDataPoints() []*DataPoint {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*DataPoint, len(m.dataPoints))
	copy(result, m.dataPoints)
	return result
}

// MockLogCollectorStream ログ収集ストリームのモック実装
type MockLogCollectorStream struct {
	logs   []*LogEntry
	ctx    context.Context
	server *StreamingServer
	closed bool
	mu     sync.Mutex
}

func NewMockLogCollectorStream(ctx context.Context, server *StreamingServer) *MockLogCollectorStream {
	return &MockLogCollectorStream{
		logs:   make([]*LogEntry, 0),
		ctx:    ctx,
		server: server,
	}
}

func (m *MockLogCollectorStream) Send(log *LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return fmt.Errorf("stream is closed")
	}
	
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	default:
		m.logs = append(m.logs, log)
		return nil
	}
}

func (m *MockLogCollectorStream) CloseAndRecv() (*LogCollectionResult, error) {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	
	return m.server.CollectLogs(m)
}

func (m *MockLogCollectorStream) Context() context.Context {
	return m.ctx
}

func (m *MockLogCollectorStream) GetLogs() []*LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*LogEntry, len(m.logs))
	copy(result, m.logs)
	return result
}

// MockFileUploaderStream ファイルアップロードストリームのモック実装
type MockFileUploaderStream struct {
	chunks []*FileChunk
	ctx    context.Context
	server *StreamingServer
	closed bool
	mu     sync.Mutex
}

func NewMockFileUploaderStream(ctx context.Context, server *StreamingServer) *MockFileUploaderStream {
	return &MockFileUploaderStream{
		chunks: make([]*FileChunk, 0),
		ctx:    ctx,
		server: server,
	}
}

func (m *MockFileUploaderStream) Send(chunk *FileChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return fmt.Errorf("stream is closed")
	}
	
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	default:
		m.chunks = append(m.chunks, chunk)
		return nil
	}
}

func (m *MockFileUploaderStream) CloseAndRecv() (*FileUploadResult, error) {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	
	return m.server.UploadFile(m)
}

func (m *MockFileUploaderStream) Context() context.Context {
	return m.ctx
}

func (m *MockFileUploaderStream) GetChunks() []*FileChunk {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*FileChunk, len(m.chunks))
	copy(result, m.chunks)
	return result
}

// ユーティリティ関数

// generateDataPoints テスト用のデータポイントを生成
func generateDataPoints(count int, source string) []*DataPoint {
	dataPoints := make([]*DataPoint, count)
	
	for i := 0; i < count; i++ {
		dataPoints[i] = &DataPoint{
			ID:        fmt.Sprintf("%s_point_%d", source, i+1),
			Value:     float64(i+1) * 10.5,
			Timestamp: time.Now().Unix(),
			Source:    source,
		}
	}
	
	return dataPoints
}

// generateLogs テスト用のログエントリを生成
func generateLogs(count int, service string) []*LogEntry {
	levels := []string{"INFO", "WARN", "ERROR", "DEBUG"}
	logs := make([]*LogEntry, count)
	
	for i := 0; i < count; i++ {
		level := levels[i%len(levels)]
		logs[i] = &LogEntry{
			Level:     level,
			Message:   fmt.Sprintf("Log message %d from %s", i+1, service),
			Timestamp: time.Now().Unix(),
			Service:   service,
		}
	}
	
	return logs
}

// createFileChunks ファイルデータをチャンクに分割
func createFileChunks(filename string, data []byte, chunkSize int) []*FileChunk {
	if chunkSize <= 0 {
		chunkSize = 1024 // デフォルト 1KB
	}
	
	var chunks []*FileChunk
	
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		
		chunk := &FileChunk{
			ChunkID:  int32(len(chunks)),
			Data:     data[i:end],
			Filename: filename,
			IsLast:   end == len(data),
		}
		chunks = append(chunks, chunk)
	}
	
	if len(chunks) == 0 {
		// 空ファイルの場合
		chunks = append(chunks, &FileChunk{
			ChunkID:  0,
			Data:     []byte{},
			Filename: filename,
			IsLast:   true,
		})
	}
	
	return chunks
}

// validateDataPoint データポイントの妥当性を検証
func validateDataPoint(dataPoint *DataPoint) error {
	if dataPoint == nil {
		return fmt.Errorf("data point is nil")
	}
	
	if strings.TrimSpace(dataPoint.ID) == "" {
		return fmt.Errorf("data point ID cannot be empty")
	}
	
	if strings.TrimSpace(dataPoint.Source) == "" {
		return fmt.Errorf("data point source cannot be empty")
	}
	
	if dataPoint.Timestamp <= 0 {
		return fmt.Errorf("data point timestamp must be positive")
	}
	
	return nil
}

func main() {
	fmt.Println("Day 48: gRPC Client-side Streaming")
	fmt.Println("Run 'go test -v' to see the streaming system in action")
}