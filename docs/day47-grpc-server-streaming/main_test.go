package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamingServer_StreamData_Success(t *testing.T) {
	server := NewStreamingServer()
	
	// テストデータを追加
	testData := generateData(10)
	for _, data := range testData {
		server.AddData(data)
	}

	ctx := context.Background()
	stream := NewMockDataStream(ctx)
	
	req := &StreamRequest{
		Query: "",
		Limit: 5,
	}

	err := server.StreamData(req, stream)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	responses := stream.GetResponses()
	if len(responses) != 5 {
		t.Errorf("Expected 5 responses, got %d", len(responses))
	}

	// シーケンス番号の確認
	for i, resp := range responses {
		if resp.SeqNum != int32(i+1) {
			t.Errorf("Expected SeqNum %d, got %d", i+1, resp.SeqNum)
		}
	}

	t.Logf("Successfully streamed %d data items", len(responses))
}

func TestStreamingServer_StreamData_WithQuery(t *testing.T) {
	server := NewStreamingServer()
	
	// 様々なデータを追加
	testData := []*DataResponse{
		{ID: "user_1", Data: "User data 1"},
		{ID: "product_1", Data: "Product information"},
		{ID: "user_2", Data: "User data 2"},
		{ID: "order_1", Data: "Order details"},
	}
	
	for _, data := range testData {
		server.AddData(data)
	}

	ctx := context.Background()
	stream := NewMockDataStream(ctx)
	
	req := &StreamRequest{
		Query: "user",
		Limit: 10,
	}

	err := server.StreamData(req, stream)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	responses := stream.GetResponses()
	if len(responses) != 2 {
		t.Errorf("Expected 2 filtered responses, got %d", len(responses))
	}

	// フィルタリング結果の確認
	for _, resp := range responses {
		if !strings.Contains(strings.ToLower(resp.ID), "user") {
			t.Errorf("Expected filtered response to contain 'user', got %s", resp.ID)
		}
	}

	t.Logf("Successfully filtered and streamed %d items", len(responses))
}

func TestStreamingServer_StreamData_ContextCancellation(t *testing.T) {
	server := NewStreamingServer()
	
	// 大量のテストデータを追加
	testData := generateData(100)
	for _, data := range testData {
		server.AddData(data)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	stream := NewMockDataStream(ctx)
	
	req := &StreamRequest{
		Query: "",
		Limit: 100,
	}

	err := server.StreamData(req, stream)
	
	// コンテキストキャンセレーションが発生することを期待
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}

	t.Logf("Correctly handled context cancellation: %v", err)
}

func TestStreamingServer_StreamLogs_ExistingLogs(t *testing.T) {
	server := NewStreamingServer()
	
	// 既存ログを追加
	existingLogs := []*LogEntry{
		{Level: "INFO", Message: "Server started", Source: "main"},
		{Level: "DEBUG", Message: "Debug message", Source: "handler"},
		{Level: "ERROR", Message: "Error occurred", Source: "database"},
	}
	
	for _, log := range existingLogs {
		server.AddLog(log)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	stream := NewMockLogStream(ctx)
	
	req := &StreamRequest{}

	// バックグラウンドでストリーミングを開始
	go func() {
		server.StreamLogs(req, stream)
	}()

	// 少し待機してログを受信
	time.Sleep(50 * time.Millisecond)

	logs := stream.GetLogs()
	if len(logs) < 3 {
		t.Errorf("Expected at least 3 logs, got %d", len(logs))
	}

	// 既存ログが含まれることを確認
	foundInfo := false
	for _, log := range logs {
		if log.Level == "INFO" && log.Message == "Server started" {
			foundInfo = true
			break
		}
	}
	
	if !foundInfo {
		t.Error("Expected to find existing INFO log")
	}

	t.Logf("Successfully received %d logs", len(logs))
}

func TestStreamingServer_StreamLogs_RealTime(t *testing.T) {
	server := NewStreamingServer()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	
	stream := NewMockLogStream(ctx)
	
	req := &StreamRequest{}

	// バックグラウンドでストリーミングを開始
	go func() {
		server.StreamLogs(req, stream)
	}()

	// 少し待機してからログを追加
	time.Sleep(50 * time.Millisecond)
	
	newLog := &LogEntry{
		Level:   "WARN",
		Message: "Real-time warning",
		Source:  "monitor",
	}
	server.AddLog(newLog)

	// ログが配信されるまで待機
	time.Sleep(100 * time.Millisecond)

	logs := stream.GetLogs()
	
	// リアルタイムログが含まれることを確認
	foundRealTime := false
	for _, log := range logs {
		if log.Level == "WARN" && log.Message == "Real-time warning" {
			foundRealTime = true
			break
		}
	}
	
	if !foundRealTime {
		t.Error("Expected to receive real-time log")
	}

	t.Logf("Successfully received real-time log among %d total logs", len(logs))
}

func TestStreamingServer_StreamFile_Success(t *testing.T) {
	server := NewStreamingServer()
	
	// テストファイルを追加
	filename := "test.txt"
	fileData := []byte("This is a test file content that will be chunked for streaming transfer.")
	server.AddFile(filename, fileData)

	ctx := context.Background()
	stream := NewMockFileStream(ctx)

	err := server.StreamFile(filename, stream)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	chunks := stream.GetChunks()
	if len(chunks) == 0 {
		t.Fatal("Expected file chunks, got none")
	}

	// チャンクの内容を結合
	var reconstructed []byte
	for i, chunk := range chunks {
		if chunk.ChunkID != int32(i) {
			t.Errorf("Expected ChunkID %d, got %d", i, chunk.ChunkID)
		}
		
		if chunk.TotalSize != int64(len(fileData)) {
			t.Errorf("Expected TotalSize %d, got %d", len(fileData), chunk.TotalSize)
		}
		
		if (i == len(chunks)-1) != chunk.IsLast {
			t.Errorf("Expected IsLast=%t for chunk %d, got %t", i == len(chunks)-1, i, chunk.IsLast)
		}
		
		reconstructed = append(reconstructed, chunk.Data...)
	}

	// 再構築されたデータが元のデータと一致することを確認
	if string(reconstructed) != string(fileData) {
		t.Errorf("Reconstructed data doesn't match original")
	}

	t.Logf("Successfully streamed file in %d chunks", len(chunks))
}

func TestStreamingServer_StreamFile_NotFound(t *testing.T) {
	server := NewStreamingServer()

	ctx := context.Background()
	stream := NewMockFileStream(ctx)

	err := server.StreamFile("nonexistent.txt", stream)
	
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
	
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("Expected 'file not found' error, got %v", err)
	}

	t.Logf("Correctly handled non-existent file: %v", err)
}

func TestStreamingClient_ReceiveData(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)
	
	// テストデータを追加
	testData := generateData(5)
	for _, data := range testData {
		server.AddData(data)
	}

	ctx := context.Background()
	req := &StreamRequest{
		Query: "",
		Limit: 3,
	}

	responses, err := client.ReceiveData(ctx, req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(responses) != 3 {
		t.Errorf("Expected 3 responses, got %d", len(responses))
	}

	t.Logf("Client successfully received %d data items", len(responses))
}

func TestStreamingClient_ReceiveFile(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)
	
	// テストファイルを追加
	filename := "test.bin"
	originalData := make([]byte, 2048) // 2KB のテストデータ
	for i := range originalData {
		originalData[i] = byte(i % 256)
	}
	server.AddFile(filename, originalData)

	ctx := context.Background()
	
	receivedData, err := client.ReceiveFile(ctx, filename)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(receivedData) != len(originalData) {
		t.Errorf("Expected data length %d, got %d", len(originalData), len(receivedData))
	}

	// データの内容を確認
	for i, b := range receivedData {
		if b != originalData[i] {
			t.Errorf("Data mismatch at index %d: expected %d, got %d", i, originalData[i], b)
			break
		}
	}

	t.Logf("Client successfully received file of %d bytes", len(receivedData))
}

func TestLogSubscription_MultipleSubscribers(t *testing.T) {
	server := NewStreamingServer()
	
	// 複数のサブスクライバーを作成
	sub1 := server.SubscribeToLogs("sub1")
	sub2 := server.SubscribeToLogs("sub2")
	
	var wg sync.WaitGroup
	var logs1, logs2 []*LogEntry
	var mu1, mu2 sync.Mutex

	// サブスクライバー1
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeout := time.After(200 * time.Millisecond)
		for {
			select {
			case log := <-sub1:
				mu1.Lock()
				logs1 = append(logs1, log)
				mu1.Unlock()
			case <-timeout:
				return
			}
		}
	}()

	// サブスクライバー2
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeout := time.After(200 * time.Millisecond)
		for {
			select {
			case log := <-sub2:
				mu2.Lock()
				logs2 = append(logs2, log)
				mu2.Unlock()
			case <-timeout:
				return
			}
		}
	}()

	// ログを追加
	time.Sleep(50 * time.Millisecond)
	testLog := &LogEntry{
		Level:   "INFO",
		Message: "Broadcast message",
		Source:  "test",
	}
	server.AddLog(testLog)

	wg.Wait()

	// 購読解除
	server.UnsubscribeFromLogs("sub1")
	server.UnsubscribeFromLogs("sub2")

	// 両方のサブスクライバーがログを受信したことを確認
	mu1.Lock()
	found1 := len(logs1) > 0
	mu1.Unlock()

	mu2.Lock()
	found2 := len(logs2) > 0
	mu2.Unlock()

	if !found1 {
		t.Error("Subscriber 1 didn't receive log")
	}
	
	if !found2 {
		t.Error("Subscriber 2 didn't receive log")
	}

	t.Logf("Multiple subscribers successfully received logs")
}

func TestUtilityFunctions(t *testing.T) {
	// chunkFile のテスト
	data := make([]byte, 2500) // 2.5KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	chunks := chunkFile(data, 1024)
	expectedChunks := 3 // 1024 + 1024 + 452
	if len(chunks) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, len(chunks))
	}

	// チャンクサイズの確認
	if len(chunks[0].Data) != 1024 {
		t.Errorf("Expected first chunk size 1024, got %d", len(chunks[0].Data))
	}
	
	if len(chunks[1].Data) != 1024 {
		t.Errorf("Expected second chunk size 1024, got %d", len(chunks[1].Data))
	}
	
	if len(chunks[2].Data) != 452 {
		t.Errorf("Expected third chunk size 452, got %d", len(chunks[2].Data))
	}

	// generateData のテスト
	generated := generateData(5)
	if len(generated) != 5 {
		t.Errorf("Expected 5 generated items, got %d", len(generated))
	}

	for i, item := range generated {
		expectedID := fmt.Sprintf("data_%d", i+1)
		if item.ID != expectedID {
			t.Errorf("Expected ID %s, got %s", expectedID, item.ID)
		}
	}

	// filterData のテスト
	testData := []*DataResponse{
		{ID: "item1", Data: "apple data"},
		{ID: "item2", Data: "banana info"},
		{ID: "apple3", Data: "orange data"},
	}

	filtered := filterData(testData, "apple")
	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered items, got %d", len(filtered))
	}

	// 空のクエリでのフィルタリング
	allFiltered := filterData(testData, "")
	if len(allFiltered) != len(testData) {
		t.Errorf("Expected all items with empty query, got %d", len(allFiltered))
	}

	t.Log("All utility functions work correctly")
}

func TestConcurrentStreaming(t *testing.T) {
	server := NewStreamingServer()
	
	// テストデータを追加
	testData := generateData(20)
	for _, data := range testData {
		server.AddData(data)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	totalResponses := 0

	// 複数のゴルーチンで同時にストリーミング
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			ctx := context.Background()
			stream := NewMockDataStream(ctx)
			
			req := &StreamRequest{
				Query: "",
				Limit: 4,
			}

			err := server.StreamData(req, stream)
			if err != nil {
				t.Errorf("Goroutine %d failed: %v", id, err)
				return
			}

			responses := stream.GetResponses()
			
			mu.Lock()
			totalResponses += len(responses)
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	expectedTotal := 5 * 4 // 5 goroutines × 4 responses each
	if totalResponses != expectedTotal {
		t.Errorf("Expected %d total responses, got %d", expectedTotal, totalResponses)
	}

	t.Logf("Concurrent streaming completed: %d total responses", totalResponses)
}

// ベンチマークテスト
func BenchmarkStreamingServer_StreamData(b *testing.B) {
	server := NewStreamingServer()
	
	// テストデータを追加
	testData := generateData(100)
	for _, data := range testData {
		server.AddData(data)
	}

	req := &StreamRequest{
		Query: "",
		Limit: 10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		stream := NewMockDataStream(ctx)
		
		err := server.StreamData(req, stream)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkChunkFile(b *testing.B) {
	data := make([]byte, 10240) // 10KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunks := chunkFile(data, 1024)
		if len(chunks) == 0 {
			b.Fatal("No chunks generated")
		}
	}
}

func BenchmarkFilterData(b *testing.B) {
	testData := generateData(1000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filtered := filterData(testData, "data")
		if len(filtered) == 0 {
			b.Fatal("No filtered data")
		}
	}
}