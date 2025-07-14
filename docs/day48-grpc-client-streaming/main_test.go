package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStreamingClient_SendDataPoints_Success(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)

	// テストデータを生成
	dataPoints := generateDataPoints(5, "sensor1")

	ctx := context.Background()
	result, err := client.SendDataPoints(ctx, dataPoints)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Status != "SUCCESS" {
		t.Errorf("Expected SUCCESS status, got %s", result.Status)
	}

	if result.TotalPoints != 5 {
		t.Errorf("Expected 5 points, got %d", result.TotalPoints)
	}

	// サーバーに保存されたデータを確認
	savedData := server.GetDataPoints()
	if len(savedData) != 5 {
		t.Errorf("Expected 5 saved points, got %d", len(savedData))
	}

	for i, data := range savedData {
		if data.Source != "sensor1" {
			t.Errorf("Expected source 'sensor1', got %s", data.Source)
		}
		if data.Value != float64(i+1)*10.5 {
			t.Errorf("Expected value %f, got %f", float64(i+1)*10.5, data.Value)
		}
	}

	t.Logf("Successfully sent %d data points", result.TotalPoints)
}

func TestStreamingClient_SendDataPoints_WithCallback(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)

	dataPoints := generateDataPoints(10, "sensor2")

	var progress []string
	var mu sync.Mutex

	callback := func(current, total int) {
		mu.Lock()
		defer mu.Unlock()
		progress = append(progress, fmt.Sprintf("%d/%d", current, total))
	}

	ctx := context.Background()
	result, err := client.SendDataPointsWithCallback(ctx, dataPoints, callback)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.TotalPoints != 10 {
		t.Errorf("Expected 10 points, got %d", result.TotalPoints)
	}

	// 進捗コールバックが呼ばれたことを確認
	mu.Lock()
	defer mu.Unlock()
	
	if len(progress) != 10 {
		t.Errorf("Expected 10 progress updates, got %d", len(progress))
	}

	// 最後の進捗が "10/10" であることを確認
	if len(progress) > 0 && progress[len(progress)-1] != "10/10" {
		t.Errorf("Expected last progress to be '10/10', got %s", progress[len(progress)-1])
	}

	t.Logf("Progress tracking worked: %v", progress)
}

func TestStreamingClient_SendDataPoints_ContextCancellation(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)

	// 大量のデータポイントを生成
	dataPoints := generateDataPoints(100, "sensor3")

	// 短いタイムアウトのコンテキスト
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// 短時間でキャンセルされることを期待
	result, err := client.SendDataPoints(ctx, dataPoints)

	if err == nil {
		t.Error("Expected context cancellation error")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}

	if result != nil {
		t.Error("Expected nil result on context cancellation")
	}

	t.Logf("Correctly handled context cancellation: %v", err)
}

func TestStreamingClient_SendLogs_Success(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)

	// テストログを生成
	logs := generateLogs(8, "api-server")

	ctx := context.Background()
	result, err := client.SendLogs(ctx, logs)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Status != "SUCCESS" {
		t.Errorf("Expected SUCCESS status, got %s", result.Status)
	}

	if result.TotalLogs != 8 {
		t.Errorf("Expected 8 logs, got %d", result.TotalLogs)
	}

	// サーバーに保存されたログを確認
	savedLogs := server.GetLogs()
	if len(savedLogs) != 8 {
		t.Errorf("Expected 8 saved logs, got %d", len(savedLogs))
	}

	// ログレベルの分布を確認
	levelCounts := make(map[string]int)
	for _, log := range savedLogs {
		levelCounts[log.Level]++
		if log.Service != "api-server" {
			t.Errorf("Expected service 'api-server', got %s", log.Service)
		}
	}

	// 各レベルのログが存在することを確認
	expectedLevels := []string{"INFO", "WARN", "ERROR", "DEBUG"}
	for _, level := range expectedLevels {
		if levelCounts[level] == 0 {
			t.Errorf("Expected at least one %s log", level)
		}
	}

	t.Logf("Successfully sent %d logs with levels: %v", result.TotalLogs, levelCounts)
}

func TestStreamingClient_UploadFile_Success(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)

	// テストファイルデータを作成
	filename := "test-document.txt"
	fileContent := "This is a test file for gRPC client-side streaming upload. " +
		"It contains multiple lines of text to test chunking functionality. " +
		"The file should be successfully transmitted and reconstructed on the server side."
	fileData := []byte(fileContent)

	ctx := context.Background()
	chunkSize := 50 // 小さなチャンクサイズでテスト
	
	result, err := client.UploadFile(ctx, filename, fileData, chunkSize)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Status != "SUCCESS" {
		t.Errorf("Expected SUCCESS status, got %s", result.Status)
	}

	if result.Filename != filename {
		t.Errorf("Expected filename %s, got %s", filename, result.Filename)
	}

	if result.TotalSize != int64(len(fileData)) {
		t.Errorf("Expected total size %d, got %d", len(fileData), result.TotalSize)
	}

	expectedChunks := (len(fileData) + chunkSize - 1) / chunkSize
	if int(result.TotalChunks) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, result.TotalChunks)
	}

	// サーバーに保存されたファイルを確認
	savedData, exists := server.GetUploadedFile(filename)
	if !exists {
		t.Fatal("File was not saved on server")
	}

	if string(savedData) != fileContent {
		t.Error("Saved file content doesn't match original")
	}

	t.Logf("Successfully uploaded file: %s (%d bytes in %d chunks)", 
		result.Filename, result.TotalSize, result.TotalChunks)
}

func TestStreamingClient_UploadFile_EmptyFile(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)

	filename := "empty.txt"
	fileData := []byte{}

	ctx := context.Background()
	result, err := client.UploadFile(ctx, filename, fileData, 1024)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Status != "SUCCESS" {
		t.Errorf("Expected SUCCESS status, got %s", result.Status)
	}

	if result.TotalSize != 0 {
		t.Errorf("Expected total size 0, got %d", result.TotalSize)
	}

	if result.TotalChunks != 1 {
		t.Errorf("Expected 1 chunk for empty file, got %d", result.TotalChunks)
	}

	// 空ファイルがサーバーに保存されたことを確認
	savedData, exists := server.GetUploadedFile(filename)
	if !exists {
		t.Fatal("Empty file was not saved on server")
	}

	if len(savedData) != 0 {
		t.Errorf("Expected empty file data, got %d bytes", len(savedData))
	}

	t.Log("Successfully uploaded empty file")
}

func TestStreamingClient_UploadFile_LargeFile(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)

	// 大きなファイルを作成（10KB）
	filename := "large-file.bin"
	fileData := make([]byte, 10240)
	for i := range fileData {
		fileData[i] = byte(i % 256)
	}

	ctx := context.Background()
	chunkSize := 1024 // 1KB chunks
	
	result, err := client.UploadFile(ctx, filename, fileData, chunkSize)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Status != "SUCCESS" {
		t.Errorf("Expected SUCCESS status, got %s", result.Status)
	}

	if result.TotalSize != int64(len(fileData)) {
		t.Errorf("Expected total size %d, got %d", len(fileData), result.TotalSize)
	}

	if result.TotalChunks != 10 {
		t.Errorf("Expected 10 chunks, got %d", result.TotalChunks)
	}

	// ファイル内容の検証
	savedData, exists := server.GetUploadedFile(filename)
	if !exists {
		t.Fatal("Large file was not saved on server")
	}

	if len(savedData) != len(fileData) {
		t.Errorf("Saved data length mismatch: expected %d, got %d", len(fileData), len(savedData))
	}

	// バイト単位での比較
	for i, b := range savedData {
		if b != fileData[i] {
			t.Errorf("Data mismatch at byte %d: expected %d, got %d", i, fileData[i], b)
			break
		}
	}

	t.Logf("Successfully uploaded large file: %d bytes in %d chunks", 
		result.TotalSize, result.TotalChunks)
}

func TestStreamingServer_CollectData_ValidationError(t *testing.T) {
	server := NewStreamingServer()

	// 無効なデータポイントを含むストリーム
	ctx := context.Background()
	stream := NewMockDataCollectorStream(ctx, server)

	// 有効なデータポイント
	validPoint := &DataPoint{
		ID:        "valid_point",
		Value:     100.0,
		Timestamp: time.Now().Unix(),
		Source:    "sensor",
	}

	// 無効なデータポイント（空のID）
	invalidPoint := &DataPoint{
		ID:        "",
		Value:     200.0,
		Timestamp: time.Now().Unix(),
		Source:    "sensor",
	}

	// データを送信
	stream.Send(validPoint)
	stream.Send(invalidPoint)

	result, err := stream.CloseAndRecv()

	if err == nil {
		t.Error("Expected validation error")
	}

	if result.Status != "ERROR" {
		t.Errorf("Expected ERROR status, got %s", result.Status)
	}

	if !strings.Contains(result.ErrorMessage, "validation failed") {
		t.Errorf("Expected validation error message, got %s", result.ErrorMessage)
	}

	t.Logf("Correctly handled validation error: %s", result.ErrorMessage)
}

func TestConcurrentDataStreaming(t *testing.T) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalPoints int32

	// 複数のゴルーチンで同時にデータを送信
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			source := fmt.Sprintf("sensor%d", id)
			dataPoints := generateDataPoints(10, source)

			ctx := context.Background()
			result, err := client.SendDataPoints(ctx, dataPoints)

			if err != nil {
				t.Errorf("Goroutine %d failed: %v", id, err)
				return
			}

			mu.Lock()
			totalPoints += result.TotalPoints
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	if totalPoints != 50 {
		t.Errorf("Expected 50 total points, got %d", totalPoints)
	}

	// サーバーのデータ確認
	savedData := server.GetDataPoints()
	if len(savedData) != 50 {
		t.Errorf("Expected 50 saved points, got %d", len(savedData))
	}

	// ソース別の分布確認
	sourceCounts := make(map[string]int)
	for _, data := range savedData {
		sourceCounts[data.Source]++
	}

	for i := 0; i < 5; i++ {
		source := fmt.Sprintf("sensor%d", i)
		if sourceCounts[source] != 10 {
			t.Errorf("Expected 10 points from %s, got %d", source, sourceCounts[source])
		}
	}

	t.Logf("Concurrent streaming completed: %d total points from %d sources", 
		totalPoints, len(sourceCounts))
}

func TestUtilityFunctions(t *testing.T) {
	// generateDataPoints のテスト
	dataPoints := generateDataPoints(3, "test_sensor")
	if len(dataPoints) != 3 {
		t.Errorf("Expected 3 data points, got %d", len(dataPoints))
	}

	for i, point := range dataPoints {
		expectedID := fmt.Sprintf("test_sensor_point_%d", i+1)
		if point.ID != expectedID {
			t.Errorf("Expected ID %s, got %s", expectedID, point.ID)
		}
		
		if point.Source != "test_sensor" {
			t.Errorf("Expected source 'test_sensor', got %s", point.Source)
		}
		
		expectedValue := float64(i+1) * 10.5
		if point.Value != expectedValue {
			t.Errorf("Expected value %f, got %f", expectedValue, point.Value)
		}
	}

	// generateLogs のテスト
	logs := generateLogs(4, "test_service")
	if len(logs) != 4 {
		t.Errorf("Expected 4 logs, got %d", len(logs))
	}

	expectedLevels := []string{"INFO", "WARN", "ERROR", "DEBUG"}
	for i, log := range logs {
		if log.Level != expectedLevels[i] {
			t.Errorf("Expected level %s, got %s", expectedLevels[i], log.Level)
		}
		
		if log.Service != "test_service" {
			t.Errorf("Expected service 'test_service', got %s", log.Service)
		}
	}

	// createFileChunks のテスト
	testData := []byte("Hello, World! This is a test.")
	chunks := createFileChunks("test.txt", testData, 10)

	expectedChunks := 3 // 29 bytes / 10 = 3 chunks
	if len(chunks) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, len(chunks))
	}

	// チャンクの内容確認
	var reconstructed []byte
	for i, chunk := range chunks {
		if chunk.ChunkID != int32(i) {
			t.Errorf("Expected ChunkID %d, got %d", i, chunk.ChunkID)
		}
		
		if chunk.Filename != "test.txt" {
			t.Errorf("Expected filename 'test.txt', got %s", chunk.Filename)
		}
		
		reconstructed = append(reconstructed, chunk.Data...)
		
		// 最後のチャンクのみ IsLast = true
		expectedIsLast := (i == len(chunks)-1)
		if chunk.IsLast != expectedIsLast {
			t.Errorf("Expected IsLast=%t for chunk %d, got %t", expectedIsLast, i, chunk.IsLast)
		}
	}

	if string(reconstructed) != string(testData) {
		t.Error("Reconstructed data doesn't match original")
	}

	// validateDataPoint のテスト
	validPoint := &DataPoint{
		ID:        "valid",
		Value:     10.0,
		Timestamp: time.Now().Unix(),
		Source:    "sensor",
	}

	if err := validateDataPoint(validPoint); err != nil {
		t.Errorf("Expected valid point to pass validation, got error: %v", err)
	}

	invalidPoints := []*DataPoint{
		nil,
		{ID: "", Value: 10.0, Timestamp: time.Now().Unix(), Source: "sensor"},
		{ID: "test", Value: 10.0, Timestamp: time.Now().Unix(), Source: ""},
		{ID: "test", Value: 10.0, Timestamp: 0, Source: "sensor"},
	}

	for i, point := range invalidPoints {
		if err := validateDataPoint(point); err == nil {
			t.Errorf("Expected invalid point %d to fail validation", i)
		}
	}

	t.Log("All utility functions work correctly")
}

// ベンチマークテスト
func BenchmarkStreamingClient_SendDataPoints(b *testing.B) {
	server := NewStreamingServer()
	client := NewStreamingClient(server)
	
	dataPoints := generateDataPoints(100, "benchmark_sensor")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.SendDataPoints(ctx, dataPoints)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateFileChunks(b *testing.B) {
	data := make([]byte, 10240) // 10KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunks := createFileChunks("test.bin", data, 1024)
		if len(chunks) == 0 {
			b.Fatal("No chunks created")
		}
	}
}

func BenchmarkValidateDataPoint(b *testing.B) {
	point := &DataPoint{
		ID:        "benchmark_point",
		Value:     100.0,
		Timestamp: time.Now().Unix(),
		Source:    "benchmark_sensor",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validateDataPoint(point)
		if err != nil {
			b.Fatal(err)
		}
	}
}