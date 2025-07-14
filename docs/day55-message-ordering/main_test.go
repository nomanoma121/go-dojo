package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestPartitionedQueue_BasicOrdering(t *testing.T) {
	partitioner := NewHashPartitioner(2)
	queue := NewPartitionedQueue(2, partitioner)
	
	var processedMessages []*OrderedMessage
	var mu sync.Mutex
	
	handler := func(ctx context.Context, message *OrderedMessage) error {
		mu.Lock()
		processedMessages = append(processedMessages, message)
		mu.Unlock()
		return nil
	}
	
	// コンシューマーを追加
	consumer := NewOrderedConsumer("test-consumer", handler)
	queue.AddConsumer(0, consumer)
	
	ctx := context.Background()
	go consumer.Start(ctx)
	
	// 同じパーティションキーでメッセージを送信
	messages := []*OrderedMessage{
		{ID: "msg-1", PartitionKey: "user-1", Data: []byte("data-1")},
		{ID: "msg-2", PartitionKey: "user-1", Data: []byte("data-2")},
		{ID: "msg-3", PartitionKey: "user-1", Data: []byte("data-3")},
	}
	
	for _, message := range messages {
		err := queue.Send(message)
		if err != nil {
			t.Errorf("Failed to send message: %v", err)
		}
	}
	
	// 処理を待つ
	time.Sleep(500 * time.Millisecond)
	
	mu.Lock()
	defer mu.Unlock()
	
	if len(processedMessages) != len(messages) {
		t.Errorf("Expected %d processed messages, got %d", 
			len(messages), len(processedMessages))
	}
	
	// 順序をチェック
	for i, processed := range processedMessages {
		if processed.SequenceNo != int64(i+1) {
			t.Errorf("Message %d: expected sequence %d, got %d", 
				i, i+1, processed.SequenceNo)
		}
	}
	
	t.Log("Message ordering preserved correctly")
}

func TestPartitionedQueue_MultiplePartitions(t *testing.T) {
	partitioner := NewHashPartitioner(2)
	queue := NewPartitionedQueue(2, partitioner)
	
	var processedByPartition = make(map[int][]*OrderedMessage)
	var mu sync.Mutex
	
	// パーティション別のコンシューマーを作成
	for partitionID := 0; partitionID < 2; partitionID++ {
		pid := partitionID // クロージャーのため
		handler := func(ctx context.Context, message *OrderedMessage) error {
			mu.Lock()
			processedByPartition[pid] = append(processedByPartition[pid], message)
			mu.Unlock()
			return nil
		}
		
		consumer := NewOrderedConsumer(fmt.Sprintf("consumer-%d", pid), handler)
		queue.AddConsumer(pid, consumer)
		
		ctx := context.Background()
		go consumer.Start(ctx)
	}
	
	// 異なるパーティションキーでメッセージを送信
	messages := []*OrderedMessage{
		{ID: "msg-1", PartitionKey: "user-1", Data: []byte("data-1")},
		{ID: "msg-2", PartitionKey: "user-2", Data: []byte("data-2")},
		{ID: "msg-3", PartitionKey: "user-1", Data: []byte("data-3")},
		{ID: "msg-4", PartitionKey: "user-2", Data: []byte("data-4")},
	}
	
	for _, message := range messages {
		err := queue.Send(message)
		if err != nil {
			t.Errorf("Failed to send message: %v", err)
		}
	}
	
	// 処理を待つ
	time.Sleep(500 * time.Millisecond)
	
	mu.Lock()
	defer mu.Unlock()
	
	// 各パーティション内での順序をチェック
	for partitionID, messages := range processedByPartition {
		if len(messages) == 0 {
			continue
		}
		
		for i, msg := range messages {
			if msg.SequenceNo != int64(i+1) {
				t.Errorf("Partition %d, message %d: expected sequence %d, got %d", 
					partitionID, i, i+1, msg.SequenceNo)
			}
		}
	}
	
	t.Log("Partition-based ordering working correctly")
}

func TestOrderingBuffer_SequenceHandling(t *testing.T) {
	buffer := NewOrderingBuffer(10)
	
	var deliveredMessages []*OrderedMessage
	
	// 配信チャネルを監視
	go func() {
		for message := range buffer.GetDeliveryChannel() {
			deliveredMessages = append(deliveredMessages, message)
		}
	}()
	
	// 順序が入れ替わったメッセージを追加
	messages := []*OrderedMessage{
		{ID: "msg-1", SequenceNo: 1},
		{ID: "msg-3", SequenceNo: 3}, // 順序が飛んでいる
		{ID: "msg-2", SequenceNo: 2}, // 後から到着
		{ID: "msg-4", SequenceNo: 4},
	}
	
	for _, message := range messages {
		err := buffer.AddMessage(message)
		if err != nil {
			t.Errorf("Failed to add message to buffer: %v", err)
		}
	}
	
	// 処理を待つ
	time.Sleep(100 * time.Millisecond)
	
	// 順序通りに配信されているかチェック
	expectedOrder := []string{"msg-1", "msg-2", "msg-3", "msg-4"}
	
	if len(deliveredMessages) != len(expectedOrder) {
		t.Errorf("Expected %d delivered messages, got %d", 
			len(expectedOrder), len(deliveredMessages))
	}
	
	for i, expected := range expectedOrder {
		if i >= len(deliveredMessages) {
			t.Errorf("Missing message at position %d", i)
			continue
		}
		
		if deliveredMessages[i].ID != expected {
			t.Errorf("Position %d: expected %s, got %s", 
				i, expected, deliveredMessages[i].ID)
		}
	}
	
	t.Log("Ordering buffer working correctly")
}

func TestBackpressureController_ThrottleControl(t *testing.T) {
	controller := NewBackpressureController(5)
	
	ctx := context.Background()
	
	// 通常の状態では スロットルしない
	if controller.ShouldThrottle() {
		t.Error("Should not throttle under normal conditions")
	}
	
	// キューサイズを上限まで増やす
	for i := 0; i < 6; i++ {
		controller.currentQueueSize++
	}
	
	if !controller.ShouldThrottle() {
		t.Error("Should throttle when queue is full")
	}
	
	// 待機をテスト（タイムアウトで確認）
	start := time.Now()
	
	done := make(chan bool)
	go func() {
		controller.WaitIfNeeded(ctx)
		done <- true
	}()
	
	// メッセージ処理をシミュレート
	go func() {
		time.Sleep(50 * time.Millisecond)
		controller.MessageProcessed()
		if controller.throttle != nil {
			select {
			case controller.throttle <- struct{}{}:
			default:
			}
		}
	}()
	
	select {
	case <-done:
		duration := time.Since(start)
		if duration < 40*time.Millisecond {
			t.Error("Should have waited for throttle release")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("WaitIfNeeded should have returned")
	}
	
	t.Log("Backpressure control working correctly")
}

func TestHashPartitioner_Distribution(t *testing.T) {
	partitioner := NewHashPartitioner(4)
	
	keys := []string{"user-1", "user-2", "user-3", "user-4", "user-5", "user-6", "user-7", "user-8"}
	distribution := make(map[int]int)
	
	for _, key := range keys {
		partition := partitioner.GetPartition(key)
		if partition < 0 || partition >= 4 {
			t.Errorf("Invalid partition %d for key %s", partition, key)
		}
		distribution[partition]++
	}
	
	// 分散が適切かチェック（完全に均等である必要はないが、極端に偏ってはいけない）
	for partition, count := range distribution {
		if count == 0 {
			t.Errorf("Partition %d has no keys assigned", partition)
		}
	}
	
	t.Log("Hash partitioner distributing keys correctly")
}

func TestOrderViolationDetector_Detection(t *testing.T) {
	detector := NewOrderViolationDetector()
	
	// 正常なシーケンス
	normalMessages := []*OrderedMessage{
		{ID: "msg-1", PartitionKey: "user-1", SequenceNo: 1},
		{ID: "msg-2", PartitionKey: "user-1", SequenceNo: 2},
		{ID: "msg-3", PartitionKey: "user-1", SequenceNo: 3},
	}
	
	for _, message := range normalMessages {
		violation := detector.CheckMessage(message)
		if violation != nil {
			t.Errorf("Unexpected violation for normal sequence: %+v", violation)
		}
	}
	
	// 順序違反メッセージ
	violationMessage := &OrderedMessage{
		ID:           "msg-violation",
		PartitionKey: "user-1",
		SequenceNo:   5, // 4をスキップ
	}
	
	violation := detector.CheckMessage(violationMessage)
	if violation == nil {
		t.Error("Expected violation for skipped sequence")
	}
	
	if violation.ExpectedSequence != 4 {
		t.Errorf("Expected sequence 4, got %d", violation.ExpectedSequence)
	}
	
	if violation.ActualSequence != 5 {
		t.Errorf("Expected actual sequence 5, got %d", violation.ActualSequence)
	}
	
	// 違反一覧をチェック
	violations := detector.GetViolations()
	if len(violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(violations))
	}
	
	t.Log("Order violation detection working correctly")
}

func TestRateCalculator_Calculation(t *testing.T) {
	calculator := NewRateCalculator(100.0) // 100 msg/sec target
	
	// 初期状態
	if calculator.GetCurrentRate() != 0 {
		t.Error("Initial rate should be 0")
	}
	
	if calculator.GetTargetRate() != 100.0 {
		t.Error("Target rate should be 100.0")
	}
	
	// 処理を記録
	start := time.Now()
	for i := 0; i < 10; i++ {
		calculator.RecordProcessing()
		time.Sleep(10 * time.Millisecond)
	}
	duration := time.Since(start)
	
	currentRate := calculator.GetCurrentRate()
	expectedRate := 10.0 / duration.Seconds()
	
	// 誤差範囲内かチェック（±20%）
	if currentRate < expectedRate*0.8 || currentRate > expectedRate*1.2 {
		t.Errorf("Rate calculation seems incorrect: expected ~%.2f, got %.2f", 
			expectedRate, currentRate)
	}
	
	t.Log("Rate calculation working correctly")
}

func TestDistributedOrderingCoordinator_VectorClock(t *testing.T) {
	coord1 := NewDistributedOrderingCoordinator("node-1")
	coord2 := NewDistributedOrderingCoordinator("node-2")
	
	// ノード1からメッセージ送信
	message1 := &OrderedMessage{
		ID:           "msg-1",
		PartitionKey: "test",
		Data:         []byte("data-1"),
	}
	
	err := coord1.SendMessage(message1)
	if err != nil {
		t.Errorf("Failed to send message from node-1: %v", err)
	}
	
	// ベクタークロックが更新されているかチェック
	if coord1.vectorClock["node-1"] != 1 {
		t.Errorf("Node-1 vector clock should be 1, got %d", 
			coord1.vectorClock["node-1"])
	}
	
	// ノード2でメッセージ受信
	err = coord2.ReceiveMessage(message1)
	if err != nil {
		t.Errorf("Failed to receive message at node-2: %v", err)
	}
	
	// ノード2のベクタークロックが更新されているかチェック
	if coord2.vectorClock["node-1"] == 0 {
		t.Error("Node-2 should have updated vector clock for node-1")
	}
	
	t.Log("Distributed ordering coordinator working correctly")
}

// 統合テスト
func TestIntegratedOrderingSystem(t *testing.T) {
	partitioner := NewHashPartitioner(2)
	queue := NewPartitionedQueue(2, partitioner)
	detector := NewOrderViolationDetector()
	
	var allProcessedMessages []*OrderedMessage
	var mu sync.Mutex
	
	handler := func(ctx context.Context, message *OrderedMessage) error {
		// 順序違反チェック
		violation := detector.CheckMessage(message)
		if violation != nil {
			t.Logf("Order violation: %+v", violation)
		}
		
		mu.Lock()
		allProcessedMessages = append(allProcessedMessages, message)
		mu.Unlock()
		
		return nil
	}
	
	// 各パーティションにコンシューマーを追加
	for i := 0; i < 2; i++ {
		consumer := NewOrderedConsumer(fmt.Sprintf("consumer-%d", i), handler)
		queue.AddConsumer(i, consumer)
		
		ctx := context.Background()
		go consumer.Start(ctx)
	}
	
	// 複数のパーティションキーでメッセージを送信
	testMessages := []*OrderedMessage{
		{ID: "msg-1", PartitionKey: "user-1", Data: []byte("data-1")},
		{ID: "msg-2", PartitionKey: "user-2", Data: []byte("data-2")},
		{ID: "msg-3", PartitionKey: "user-1", Data: []byte("data-3")},
		{ID: "msg-4", PartitionKey: "user-2", Data: []byte("data-4")},
		{ID: "msg-5", PartitionKey: "user-1", Data: []byte("data-5")},
	}
	
	for _, message := range testMessages {
		err := queue.Send(message)
		if err != nil {
			t.Errorf("Failed to send message %s: %v", message.ID, err)
		}
		time.Sleep(10 * time.Millisecond) // 少し間隔を開ける
	}
	
	// 処理完了を待つ
	time.Sleep(1 * time.Second)
	
	mu.Lock()
	defer mu.Unlock()
	
	if len(allProcessedMessages) != len(testMessages) {
		t.Errorf("Expected %d processed messages, got %d", 
			len(testMessages), len(allProcessedMessages))
	}
	
	// パーティション別にグループ化して順序をチェック
	partitionGroups := make(map[string][]*OrderedMessage)
	for _, msg := range allProcessedMessages {
		partitionGroups[msg.PartitionKey] = append(partitionGroups[msg.PartitionKey], msg)
	}
	
	for partitionKey, messages := range partitionGroups {
		for i, msg := range messages {
			if msg.SequenceNo != int64(i+1) {
				t.Errorf("Partition %s, message %d: expected sequence %d, got %d", 
					partitionKey, i, i+1, msg.SequenceNo)
			}
		}
	}
	
	violations := detector.GetViolations()
	if len(violations) > 0 {
		t.Errorf("Unexpected violations in integrated test: %d", len(violations))
	}
	
	t.Log("Integrated ordering system working correctly")
}

// ベンチマークテスト
func BenchmarkPartitionedQueue_Send(b *testing.B) {
	partitioner := NewHashPartitioner(4)
	queue := NewPartitionedQueue(4, partitioner)
	
	handler := func(ctx context.Context, message *OrderedMessage) error {
		return nil
	}
	
	// コンシューマーを設定
	for i := 0; i < 4; i++ {
		consumer := NewOrderedConsumer(fmt.Sprintf("consumer-%d", i), handler)
		queue.AddConsumer(i, consumer)
		go consumer.Start(context.Background())
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			message := &OrderedMessage{
				ID:           fmt.Sprintf("msg-%d", i),
				PartitionKey: fmt.Sprintf("user-%d", i%100),
				Data:         []byte("benchmark data"),
			}
			
			queue.Send(message)
			i++
		}
	})
}

func BenchmarkOrderingBuffer_AddMessage(b *testing.B) {
	buffer := NewOrderingBuffer(1000)
	
	// 配信チャネルを消費
	go func() {
		for range buffer.GetDeliveryChannel() {
			// 何もしない
		}
	}()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		message := &OrderedMessage{
			ID:         fmt.Sprintf("msg-%d", i),
			SequenceNo: int64(i + 1),
		}
		buffer.AddMessage(message)
	}
}