// Day 47: gRPC Server-side Streaming
// サーバーからクライアントへの連続データ送信の実装

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
type StreamRequest struct {
	Query string `json:"query"`
	Limit int32  `json:"limit"`
}

type DataResponse struct {
	ID        string `json:"id"`
	Data      string `json:"data"`
	Timestamp int64  `json:"timestamp"`
	SeqNum    int32  `json:"seq_num"`
}

type LogEntry struct {
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	Source    string `json:"source"`
}

type FileChunk struct {
	ChunkID   int32  `json:"chunk_id"`
	Data      []byte `json:"data"`
	TotalSize int64  `json:"total_size"`
	IsLast    bool   `json:"is_last"`
}

// ストリームインターフェース（モック実装）
type DataServiceStreamServer interface {
	Send(*DataResponse) error
	Context() context.Context
}

type LogStreamServer interface {
	Send(*LogEntry) error
	Context() context.Context
}

type FileStreamServer interface {
	Send(*FileChunk) error
	Context() context.Context
}

// サーバー実装
type StreamingServer struct {
	data     []*DataResponse
	logs     []*LogEntry
	files    map[string][]byte
	mu       sync.RWMutex
	logSubs  map[string]chan *LogEntry
	subsMu   sync.RWMutex
}

func NewStreamingServer() *StreamingServer {
	return &StreamingServer{
		data:    make([]*DataResponse, 0),
		logs:    make([]*LogEntry, 0),
		files:   make(map[string][]byte),
		logSubs: make(map[string]chan *LogEntry),
	}
}

// StreamData リクエストに基づいてデータを連続的にストリーミング
func (s *StreamingServer) StreamData(req *StreamRequest, stream DataServiceStreamServer) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// フィルタリングされたデータを取得
	filteredData := filterData(s.data, req.Query)

	// リミットを適用
	limit := req.Limit
	if limit <= 0 || limit > int32(len(filteredData)) {
		limit = int32(len(filteredData))
	}

	// データを順次送信
	for i := int32(0); i < limit; i++ {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		default:
			data := filteredData[i]
			data.SeqNum = i + 1 // シーケンス番号を設定

			if err := stream.Send(data); err != nil {
				return fmt.Errorf("failed to send data: %w", err)
			}

			// リアルタイム感を演出するため少し待機
			time.Sleep(10 * time.Millisecond)
		}
	}

	return nil
}

// StreamLogs リアルタイムでログをストリーミング
func (s *StreamingServer) StreamLogs(req *StreamRequest, stream LogStreamServer) error {
	subscriptionID := fmt.Sprintf("sub_%d", time.Now().UnixNano())
	
	// ログ購読を開始
	logChan := s.SubscribeToLogs(subscriptionID)
	defer s.UnsubscribeFromLogs(subscriptionID)

	// 既存のログを最初に送信
	s.mu.RLock()
	existingLogs := make([]*LogEntry, len(s.logs))
	copy(existingLogs, s.logs)
	s.mu.RUnlock()

	// 既存ログの送信
	for _, log := range existingLogs {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		default:
			if err := stream.Send(log); err != nil {
				return fmt.Errorf("failed to send existing log: %w", err)
			}
		}
	}

	// リアルタイムログの送信
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case log, ok := <-logChan:
			if !ok {
				return nil // チャネルが閉じられた
			}
			if err := stream.Send(log); err != nil {
				return fmt.Errorf("failed to send real-time log: %w", err)
			}
		}
	}
}

// StreamFile ファイルを分割してストリーミング
func (s *StreamingServer) StreamFile(filename string, stream FileStreamServer) error {
	s.mu.RLock()
	fileData, exists := s.files[filename]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("file not found: %s", filename)
	}

	// ファイルをチャンクに分割
	chunkSize := 1024 // 1KB chunks
	chunks := chunkFile(fileData, chunkSize)

	// チャンクを順次送信
	for i, chunk := range chunks {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		default:
			chunk.ChunkID = int32(i)
			chunk.TotalSize = int64(len(fileData))
			chunk.IsLast = (i == len(chunks)-1)

			if err := stream.Send(chunk); err != nil {
				return fmt.Errorf("failed to send chunk %d: %w", i, err)
			}

			// 転送速度制限のため少し待機
			time.Sleep(5 * time.Millisecond)
		}
	}

	return nil
}

// AddData データを追加
func (s *StreamingServer) AddData(data *DataResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	data.Timestamp = time.Now().Unix()
	s.data = append(s.data, data)
}

// AddLog ログを追加し、サブスクライバーに通知
func (s *StreamingServer) AddLog(log *LogEntry) {
	log.Timestamp = time.Now().Unix()
	
	s.mu.Lock()
	s.logs = append(s.logs, log)
	s.mu.Unlock()

	// 全サブスクライバーに通知
	s.subsMu.RLock()
	defer s.subsMu.RUnlock()
	
	for _, ch := range s.logSubs {
		select {
		case ch <- log:
		default:
			// チャネルが満杯の場合はスキップ
		}
	}
}

// AddFile ファイルを追加
func (s *StreamingServer) AddFile(filename string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.files[filename] = data
}

// SubscribeToLogs ログ購読を開始
func (s *StreamingServer) SubscribeToLogs(subscriptionID string) <-chan *LogEntry {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	
	ch := make(chan *LogEntry, 100) // バッファ付きチャネル
	s.logSubs[subscriptionID] = ch
	
	return ch
}

// UnsubscribeFromLogs ログ購読を停止
func (s *StreamingServer) UnsubscribeFromLogs(subscriptionID string) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	
	if ch, exists := s.logSubs[subscriptionID]; exists {
		close(ch)
		delete(s.logSubs, subscriptionID)
	}
}

// クライアント実装
type StreamingClient struct {
	server *StreamingServer
}

func NewStreamingClient(server *StreamingServer) *StreamingClient {
	return &StreamingClient{server: server}
}

// ReceiveData データストリームを受信
func (c *StreamingClient) ReceiveData(ctx context.Context, req *StreamRequest) ([]*DataResponse, error) {
	stream := NewMockDataStream(ctx)
	
	err := c.server.StreamData(req, stream)
	if err != nil {
		return nil, err
	}
	
	return stream.GetResponses(), nil
}

// ReceiveLogs ログストリームを受信
func (c *StreamingClient) ReceiveLogs(ctx context.Context, req *StreamRequest, callback func(*LogEntry)) error {
	stream := NewMockLogStream(ctx)
	
	// バックグラウンドでストリーミングを開始
	errChan := make(chan error, 1)
	go func() {
		errChan <- c.server.StreamLogs(req, stream)
	}()

	// ログを受信してコールバックを呼び出し
	done := false
	for !done {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			done = true
			if err != nil {
				return err
			}
		default:
			logs := stream.GetLogs()
			for _, log := range logs {
				callback(log)
			}
			time.Sleep(10 * time.Millisecond) // ポーリング間隔
		}
	}

	return nil
}

// ReceiveFile ファイルストリームを受信
func (c *StreamingClient) ReceiveFile(ctx context.Context, filename string) ([]byte, error) {
	stream := NewMockFileStream(ctx)
	
	err := c.server.StreamFile(filename, stream)
	if err != nil {
		return nil, err
	}

	// チャンクを結合
	chunks := stream.GetChunks()
	if len(chunks) == 0 {
		return []byte{}, nil
	}

	totalSize := chunks[0].TotalSize
	result := make([]byte, 0, totalSize)
	
	for _, chunk := range chunks {
		result = append(result, chunk.Data...)
	}

	return result, nil
}

// モックストリーム実装
type MockDataStream struct {
	responses []*DataResponse
	ctx       context.Context
	mu        sync.Mutex
}

func NewMockDataStream(ctx context.Context) *MockDataStream {
	return &MockDataStream{
		responses: make([]*DataResponse, 0),
		ctx:       ctx,
	}
}

func (m *MockDataStream) Send(response *DataResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	default:
		m.responses = append(m.responses, response)
		return nil
	}
}

func (m *MockDataStream) Context() context.Context {
	return m.ctx
}

func (m *MockDataStream) GetResponses() []*DataResponse {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*DataResponse, len(m.responses))
	copy(result, m.responses)
	return result
}

type MockLogStream struct {
	logs []*LogEntry
	ctx  context.Context
	mu   sync.Mutex
}

func NewMockLogStream(ctx context.Context) *MockLogStream {
	return &MockLogStream{
		logs: make([]*LogEntry, 0),
		ctx:  ctx,
	}
}

func (m *MockLogStream) Send(log *LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	default:
		m.logs = append(m.logs, log)
		return nil
	}
}

func (m *MockLogStream) Context() context.Context {
	return m.ctx
}

func (m *MockLogStream) GetLogs() []*LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*LogEntry, len(m.logs))
	copy(result, m.logs)
	return result
}

type MockFileStream struct {
	chunks []*FileChunk
	ctx    context.Context
	mu     sync.Mutex
}

func NewMockFileStream(ctx context.Context) *MockFileStream {
	return &MockFileStream{
		chunks: make([]*FileChunk, 0),
		ctx:    ctx,
	}
}

func (m *MockFileStream) Send(chunk *FileChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	default:
		m.chunks = append(m.chunks, chunk)
		return nil
	}
}

func (m *MockFileStream) Context() context.Context {
	return m.ctx
}

func (m *MockFileStream) GetChunks() []*FileChunk {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*FileChunk, len(m.chunks))
	copy(result, m.chunks)
	return result
}

// ユーティリティ関数

// chunkFile ファイルを指定サイズのチャンクに分割
func chunkFile(data []byte, chunkSize int) []*FileChunk {
	if chunkSize <= 0 {
		chunkSize = 1024
	}

	chunks := make([]*FileChunk, 0)
	
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		
		chunk := &FileChunk{
			Data: data[i:end],
		}
		chunks = append(chunks, chunk)
	}
	
	return chunks
}

// generateData テスト用のデータを生成
func generateData(count int) []*DataResponse {
	data := make([]*DataResponse, count)
	
	for i := 0; i < count; i++ {
		data[i] = &DataResponse{
			ID:        fmt.Sprintf("data_%d", i+1),
			Data:      fmt.Sprintf("Sample data %d", i+1),
			Timestamp: time.Now().Unix(),
		}
	}
	
	return data
}

// filterData クエリに基づいてデータをフィルタリング
func filterData(data []*DataResponse, query string) []*DataResponse {
	if query == "" {
		return data
	}
	
	filtered := make([]*DataResponse, 0)
	query = strings.ToLower(query)
	
	for _, item := range data {
		if strings.Contains(strings.ToLower(item.Data), query) ||
		   strings.Contains(strings.ToLower(item.ID), query) {
			filtered = append(filtered, item)
		}
	}
	
	return filtered
}

func main() {
	fmt.Println("Day 47: gRPC Server-side Streaming")
	fmt.Println("Run 'go test -v' to see the streaming system in action")
}