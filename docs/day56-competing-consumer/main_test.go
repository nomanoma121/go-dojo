package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestInMemoryQueue_BasicOperations(t *testing.T) {
	queue := NewInMemoryQueue()
	if queue == nil {
		t.Skip("InMemoryQueue not implemented yet")
	}
	defer queue.Close()

	// 初期状態のテスト
	if queue.Size() != 0 {
		t.Errorf("Expected empty queue, got size %d", queue.Size())
	}

	// Enqueue テスト
	msg := Message{
		ID:        1,
		Data:      "test message",
		Timestamp: time.Now(),
	}

	err := queue.Enqueue(msg)
	if err != nil {
		t.Errorf("Failed to enqueue message: %v", err)
	}

	if queue.Size() != 1 {
		t.Errorf("Expected queue size 1, got %d", queue.Size())
	}

	// Dequeue テスト
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	dequeuedMsg, err := queue.Dequeue(ctx)
	if err != nil {
		t.Errorf("Failed to dequeue message: %v", err)
	}

	if dequeuedMsg == nil {
		t.Error("Dequeued message is nil")
		return
	}

	if dequeuedMsg.ID != msg.ID || dequeuedMsg.Data != msg.Data {
		t.Errorf("Dequeued message doesn't match. Expected ID=%d, Data=%s, got ID=%d, Data=%s",
			msg.ID, msg.Data, dequeuedMsg.ID, dequeuedMsg.Data)
	}

	if queue.Size() != 0 {
		t.Errorf("Expected empty queue after dequeue, got size %d", queue.Size())
	}
}

func TestInMemoryQueue_ConcurrentAccess(t *testing.T) {
	queue := NewInMemoryQueue()
	if queue == nil {
		t.Skip("InMemoryQueue not implemented yet")
	}
	defer queue.Close()

	const numProducers = 5
	const numConsumers = 3
	const messagesPerProducer = 10

	var wg sync.WaitGroup
	processedMessages := make(map[int]bool)
	var mu sync.Mutex

	// Producers
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			for j := 0; j < messagesPerProducer; j++ {
				msg := Message{
					ID:        producerID*1000 + j,
					Data:      fmt.Sprintf("Producer %d Message %d", producerID, j),
					Timestamp: time.Now(),
				}
				if err := queue.Enqueue(msg); err != nil {
					t.Errorf("Producer %d failed to enqueue: %v", producerID, err)
				}
			}
		}(i)
	}

	// Consumers
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := 0; i < numConsumers; i++ {
		wg.Add(1)
		go func(consumerID int) {
			defer wg.Done()
			for {
				msg, err := queue.Dequeue(ctx)
				if err != nil {
					// Context cancelled or queue closed
					return
				}
				if msg == nil {
					continue
				}

				mu.Lock()
				if processedMessages[msg.ID] {
					t.Errorf("Message %d processed multiple times", msg.ID)
				}
				processedMessages[msg.ID] = true
				mu.Unlock()

				// Check if all messages are processed
				mu.Lock()
				if len(processedMessages) >= numProducers*messagesPerProducer {
					mu.Unlock()
					cancel()
					return
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Verify all messages were processed
	expectedMessages := numProducers * messagesPerProducer
	if len(processedMessages) != expectedMessages {
		t.Errorf("Expected %d messages to be processed, got %d",
			expectedMessages, len(processedMessages))
	}
}

func TestInMemoryQueue_ContextCancellation(t *testing.T) {
	queue := NewInMemoryQueue()
	if queue == nil {
		t.Skip("InMemoryQueue not implemented yet")
	}
	defer queue.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Try to dequeue from empty queue with timeout
	start := time.Now()
	_, err := queue.Dequeue(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	// Should timeout around 100ms
	if elapsed < 50*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Errorf("Expected timeout around 100ms, got %v", elapsed)
	}
}

func TestConsumer_BasicProcessing(t *testing.T) {
	queue := NewInMemoryQueue()
	if queue == nil {
		t.Skip("InMemoryQueue not implemented yet")
	}
	defer queue.Close()

	processedMessages := make([]Message, 0)
	var mu sync.Mutex

	processor := func(msg Message) error {
		mu.Lock()
		processedMessages = append(processedMessages, msg)
		mu.Unlock()
		return nil
	}

	consumer := NewConsumer("test-consumer", queue, processor)
	if consumer == nil {
		t.Skip("Consumer not implemented yet")
	}

	// Add test messages
	testMessages := []Message{
		{ID: 1, Data: "Message 1", Timestamp: time.Now()},
		{ID: 2, Data: "Message 2", Timestamp: time.Now()},
		{ID: 3, Data: "Message 3", Timestamp: time.Now()},
	}

	for _, msg := range testMessages {
		queue.Enqueue(msg)
	}

	// Start consumer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	consumer.Start(ctx)

	// Wait for processing
	time.Sleep(500 * time.Millisecond)
	consumer.Stop()

	// Verify processed messages
	mu.Lock()
	processedCount := len(processedMessages)
	mu.Unlock()

	if processedCount != len(testMessages) {
		t.Errorf("Expected %d messages to be processed, got %d",
			len(testMessages), processedCount)
	}

	// Check stats
	stats := consumer.GetStats()
	if stats.ProcessedCount != int64(len(testMessages)) {
		t.Errorf("Expected processed count %d, got %d",
			len(testMessages), stats.ProcessedCount)
	}
}

func TestConsumer_ErrorHandling(t *testing.T) {
	queue := NewInMemoryQueue()
	if queue == nil {
		t.Skip("InMemoryQueue not implemented yet")
	}
	defer queue.Close()

	errorProcessor := func(msg Message) error {
		if msg.ID%2 == 0 {
			return fmt.Errorf("simulated error for message %d", msg.ID)
		}
		return nil
	}

	consumer := NewConsumer("error-test-consumer", queue, errorProcessor)
	if consumer == nil {
		t.Skip("Consumer not implemented yet")
	}

	// Add test messages
	for i := 1; i <= 5; i++ {
		msg := Message{
			ID:        i,
			Data:      fmt.Sprintf("Message %d", i),
			Timestamp: time.Now(),
		}
		queue.Enqueue(msg)
	}

	// Start consumer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	consumer.Start(ctx)
	time.Sleep(500 * time.Millisecond)
	consumer.Stop()

	// Check error stats
	stats := consumer.GetStats()
	expectedErrors := int64(2) // Messages with ID 2 and 4 should error
	if stats.ErrorCount != expectedErrors {
		t.Errorf("Expected %d errors, got %d", expectedErrors, stats.ErrorCount)
	}
}

func TestConsumerGroup_LoadDistribution(t *testing.T) {
	queue := NewInMemoryQueue()
	if queue == nil {
		t.Skip("InMemoryQueue not implemented yet")
	}
	defer queue.Close()

	processor := func(msg Message) error {
		time.Sleep(10 * time.Millisecond) // Simulate processing time
		return nil
	}

	const numConsumers = 3
	const numMessages = 30

	consumerGroup := NewConsumerGroup(queue, numConsumers, processor)
	if consumerGroup == nil {
		t.Skip("ConsumerGroup not implemented yet")
	}

	// Add messages
	for i := 1; i <= numMessages; i++ {
		msg := Message{
			ID:        i,
			Data:      fmt.Sprintf("Message %d", i),
			Timestamp: time.Now(),
		}
		queue.Enqueue(msg)
	}

	// Start consumer group
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	consumerGroup.Start(ctx)
	time.Sleep(2 * time.Second)
	consumerGroup.Stop()

	// Check aggregated stats
	stats := consumerGroup.GetAggregatedStats()
	if len(stats) != numConsumers {
		t.Errorf("Expected %d consumers in stats, got %d", numConsumers, len(stats))
	}

	totalProcessed := int64(0)
	for _, stat := range stats {
		totalProcessed += stat.ProcessedCount
	}

	if totalProcessed != numMessages {
		t.Errorf("Expected %d total messages processed, got %d",
			numMessages, totalProcessed)
	}

	// Verify load distribution (each consumer should process similar amounts)
	minProcessed := int64(numMessages)
	maxProcessed := int64(0)
	for _, stat := range stats {
		if stat.ProcessedCount < minProcessed {
			minProcessed = stat.ProcessedCount
		}
		if stat.ProcessedCount > maxProcessed {
			maxProcessed = stat.ProcessedCount
		}
	}

	// The difference shouldn't be too large
	if maxProcessed-minProcessed > int64(numMessages/numConsumers) {
		t.Errorf("Load distribution is uneven: min=%d, max=%d, diff=%d",
			minProcessed, maxProcessed, maxProcessed-minProcessed)
	}
}

func TestProducer_BasicOperations(t *testing.T) {
	queue := NewInMemoryQueue()
	if queue == nil {
		t.Skip("InMemoryQueue not implemented yet")
	}
	defer queue.Close()

	producer := NewProducer(queue)
	if producer == nil {
		t.Skip("Producer not implemented yet")
	}

	// Test single message production
	err := producer.Produce("test message")
	if err != nil {
		t.Errorf("Failed to produce message: %v", err)
	}

	if queue.Size() != 1 {
		t.Errorf("Expected queue size 1, got %d", queue.Size())
	}

	// Test batch production
	batch := []string{"msg1", "msg2", "msg3"}
	err = producer.ProduceBatch(batch)
	if err != nil {
		t.Errorf("Failed to produce batch: %v", err)
	}

	expectedSize := 1 + len(batch)
	if queue.Size() != expectedSize {
		t.Errorf("Expected queue size %d, got %d", expectedSize, queue.Size())
	}
}

func TestLoadBalancer_RoundRobinStrategy(t *testing.T) {
	// Create multiple queues
	queues := make([]Queue, 3)
	for i := range queues {
		queues[i] = NewInMemoryQueue()
		if queues[i] == nil {
			t.Skip("InMemoryQueue not implemented yet")
		}
		defer queues[i].Close()
	}

	loadBalancer := NewLoadBalancer(queues, 0) // Assuming 0 is RoundRobin
	if loadBalancer == nil {
		t.Skip("LoadBalancer not implemented yet")
	}

	// Test round-robin selection
	selectedQueues := make([]Queue, 6)
	for i := range selectedQueues {
		selectedQueues[i] = loadBalancer.SelectQueue()
	}

	// In round-robin, we should cycle through queues
	// Check that we get each queue twice in sequence
	for i := 0; i < 3; i++ {
		if selectedQueues[i] != selectedQueues[i+3] {
			t.Errorf("Round-robin pattern broken at index %d", i)
		}
	}
}

// ベンチマークテスト
func BenchmarkInMemoryQueue_EnqueueDequeue(b *testing.B) {
	queue := NewInMemoryQueue()
	if queue == nil {
		b.Skip("InMemoryQueue not implemented yet")
	}
	defer queue.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			msg := Message{
				ID:        1,
				Data:      "benchmark message",
				Timestamp: time.Now(),
			}
			queue.Enqueue(msg)
			queue.Dequeue(ctx)
		}
	})
}

func BenchmarkConsumerGroup_Throughput(b *testing.B) {
	queue := NewInMemoryQueue()
	if queue == nil {
		b.Skip("InMemoryQueue not implemented yet")
	}
	defer queue.Close()

	processor := func(msg Message) error {
		// Minimal processing
		return nil
	}

	consumerGroup := NewConsumerGroup(queue, 4, processor)
	if consumerGroup == nil {
		b.Skip("ConsumerGroup not implemented yet")
	}

	ctx := context.Background()
	consumerGroup.Start(ctx)
	defer consumerGroup.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			msg := Message{
				ID:        1,
				Data:      "benchmark message",
				Timestamp: time.Now(),
			}
			queue.Enqueue(msg)
		}
	})
}