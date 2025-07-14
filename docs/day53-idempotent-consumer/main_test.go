package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestIdempotentConsumer_ProcessMessage(t *testing.T) {
	retryStrategy := &ExponentialBackoffStrategy{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		MaxRetries: 2,
	}
	dlq := NewSimpleDeadLetterQueue()
	pubsub := NewInMemoryPubSub()
	consumer := NewIdempotentConsumer(pubsub, retryStrategy, dlq)
	
	callCount := 0
	handler := func(ctx context.Context, message *Message) error {
		callCount++
		return nil
	}
	
	message := &Message{
		ID:        "test-msg-1",
		Topic:     "test",
		Data:      []byte("test data"),
		Timestamp: time.Now(),
	}
	
	ctx := context.Background()
	
	// 最初の処理
	err := consumer.ProcessMessage(ctx, message, handler)
	if err != nil {
		t.Errorf("First processing should succeed: %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Handler should be called once, called %d times", callCount)
	}
	
	// 重複メッセージの処理
	err = consumer.ProcessMessage(ctx, message, handler)
	if err != nil {
		t.Errorf("Duplicate processing should succeed: %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Handler should not be called for duplicate, called %d times", callCount)
	}
	
	t.Log("Idempotent consumer working correctly")
}

func TestIdempotentConsumer_RetryLogic(t *testing.T) {
	retryStrategy := &ExponentialBackoffStrategy{
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		MaxRetries: 3,
	}
	dlq := NewSimpleDeadLetterQueue()
	pubsub := NewInMemoryPubSub()
	consumer := NewIdempotentConsumer(pubsub, retryStrategy, dlq)
	
	callCount := 0
	handler := func(ctx context.Context, message *Message) error {
		callCount++
		if callCount < 3 {
			return fmt.Errorf("temporary error")
		}
		return nil
	}
	
	message := &Message{
		ID:        "retry-msg-1",
		Topic:     "test",
		Data:      []byte("retry test"),
		Timestamp: time.Now(),
	}
	
	ctx := context.Background()
	
	err := consumer.ProcessMessage(ctx, message, handler)
	if err != nil {
		t.Errorf("Processing should eventually succeed: %v", err)
	}
	
	if callCount != 3 {
		t.Errorf("Handler should be called 3 times, called %d times", callCount)
	}
	
	t.Log("Retry logic working correctly")
}

func TestIdempotentConsumer_DeadLetterQueue(t *testing.T) {
	retryStrategy := &ExponentialBackoffStrategy{
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		MaxRetries: 2,
	}
	dlq := NewSimpleDeadLetterQueue()
	pubsub := NewInMemoryPubSub()
	consumer := NewIdempotentConsumer(pubsub, retryStrategy, dlq)
	
	handler := func(ctx context.Context, message *Message) error {
		return fmt.Errorf("persistent error")
	}
	
	message := &Message{
		ID:        "dlq-msg-1",
		Topic:     "test",
		Data:      []byte("dlq test"),
		Timestamp: time.Now(),
	}
	
	ctx := context.Background()
	
	err := consumer.ProcessMessage(ctx, message, handler)
	if err == nil {
		t.Error("Processing should fail and send to DLQ")
	}
	
	dlqMessages := dlq.GetMessages()
	if len(dlqMessages) != 1 {
		t.Errorf("Expected 1 message in DLQ, got %d", len(dlqMessages))
	}
	
	if dlqMessages[0].Message.ID != message.ID {
		t.Errorf("DLQ message ID mismatch: expected %s, got %s", 
			message.ID, dlqMessages[0].Message.ID)
	}
	
	t.Log("Dead letter queue working correctly")
}

func TestExponentialBackoffStrategy(t *testing.T) {
	strategy := &ExponentialBackoffStrategy{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		MaxRetries: 3,
	}
	
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 10 * time.Millisecond},
		{1, 20 * time.Millisecond},
		{2, 40 * time.Millisecond},
		{3, 80 * time.Millisecond},
		{4, 100 * time.Millisecond}, // capped at MaxDelay
	}
	
	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			delay := strategy.NextDelay(tt.attempt)
			if delay != tt.expected {
				t.Errorf("Expected delay %v, got %v", tt.expected, delay)
			}
		})
	}
	
	// Test retry limit
	for attempt := 0; attempt <= 5; attempt++ {
		shouldRetry := strategy.ShouldRetry(attempt, fmt.Errorf("test error"))
		expected := attempt < strategy.MaxRetries
		
		if shouldRetry != expected {
			t.Errorf("Attempt %d: expected shouldRetry=%v, got %v", 
				attempt, expected, shouldRetry)
		}
	}
}

func TestInMemoryPubSub(t *testing.T) {
	pubsub := NewInMemoryPubSub()
	defer pubsub.Close()
	
	receivedMessages := make([]string, 0)
	var mu sync.Mutex
	
	handler := func(ctx context.Context, message *Message) error {
		mu.Lock()
		receivedMessages = append(receivedMessages, message.ID)
		mu.Unlock()
		return nil
	}
	
	ctx := context.Background()
	
	// Subscribe to topic
	err := pubsub.Subscribe(ctx, "test-topic", handler)
	if err != nil {
		t.Errorf("Subscribe failed: %v", err)
	}
	
	// Publish messages
	messages := []*Message{
		{ID: "msg-1", Topic: "test-topic", Data: []byte("data-1")},
		{ID: "msg-2", Topic: "test-topic", Data: []byte("data-2")},
		{ID: "msg-3", Topic: "test-topic", Data: []byte("data-3")},
	}
	
	for _, msg := range messages {
		err := pubsub.Publish(ctx, "test-topic", msg)
		if err != nil {
			t.Errorf("Publish failed: %v", err)
		}
	}
	
	// Wait for message processing
	time.Sleep(100 * time.Millisecond)
	
	mu.Lock()
	if len(receivedMessages) != len(messages) {
		t.Errorf("Expected %d messages, received %d", len(messages), len(receivedMessages))
	}
	
	for i, msg := range messages {
		if receivedMessages[i] != msg.ID {
			t.Errorf("Message %d: expected ID %s, got %s", 
				i, msg.ID, receivedMessages[i])
		}
	}
	mu.Unlock()
	
	t.Log("PubSub working correctly")
}

func TestInMemoryPubSub_MultipleSubscribers(t *testing.T) {
	pubsub := NewInMemoryPubSub()
	defer pubsub.Close()
	
	var receivedCount1, receivedCount2 int
	var mu1, mu2 sync.Mutex
	
	handler1 := func(ctx context.Context, message *Message) error {
		mu1.Lock()
		receivedCount1++
		mu1.Unlock()
		return nil
	}
	
	handler2 := func(ctx context.Context, message *Message) error {
		mu2.Lock()
		receivedCount2++
		mu2.Unlock()
		return nil
	}
	
	ctx := context.Background()
	
	// Subscribe with multiple handlers
	pubsub.Subscribe(ctx, "multi-topic", handler1)
	pubsub.Subscribe(ctx, "multi-topic", handler2)
	
	// Publish a message
	message := &Message{
		ID:    "multi-msg-1",
		Topic: "multi-topic",
		Data:  []byte("multi data"),
	}
	
	err := pubsub.Publish(ctx, "multi-topic", message)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}
	
	// Wait for processing
	time.Sleep(100 * time.Millisecond)
	
	mu1.Lock()
	mu2.Lock()
	defer mu1.Unlock()
	defer mu2.Unlock()
	
	if receivedCount1 != 1 {
		t.Errorf("Handler1 should receive 1 message, got %d", receivedCount1)
	}
	
	if receivedCount2 != 1 {
		t.Errorf("Handler2 should receive 1 message, got %d", receivedCount2)
	}
	
	t.Log("Multiple subscribers working correctly")
}

func TestSimpleDeadLetterQueue(t *testing.T) {
	dlq := NewSimpleDeadLetterQueue()
	
	message := &Message{
		ID:        "dlq-test-1",
		Topic:     "test",
		Data:      []byte("test data"),
		Timestamp: time.Now(),
	}
	
	ctx := context.Background()
	
	// Send message to DLQ
	err := dlq.Send(ctx, message, "processing failed")
	if err != nil {
		t.Errorf("Send to DLQ failed: %v", err)
	}
	
	// Get messages from DLQ
	messages := dlq.GetMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 message in DLQ, got %d", len(messages))
	}
	
	dlqMsg := messages[0]
	if dlqMsg.Message.ID != message.ID {
		t.Errorf("Message ID mismatch: expected %s, got %s", 
			message.ID, dlqMsg.Message.ID)
	}
	
	if dlqMsg.Reason != "processing failed" {
		t.Errorf("Reason mismatch: expected 'processing failed', got '%s'", 
			dlqMsg.Reason)
	}
	
	t.Log("Dead letter queue working correctly")
}

func TestConsumerMetrics(t *testing.T) {
	retryStrategy := &ExponentialBackoffStrategy{
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		MaxRetries: 1,
	}
	dlq := NewSimpleDeadLetterQueue()
	pubsub := NewInMemoryPubSub()
	consumer := NewIdempotentConsumer(pubsub, retryStrategy, dlq)
	
	ctx := context.Background()
	
	// Successful processing
	successHandler := func(ctx context.Context, message *Message) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	}
	
	successMsg := &Message{
		ID:    "success-msg",
		Topic: "test",
		Data:  []byte("success"),
	}
	
	consumer.ProcessMessage(ctx, successMsg, successHandler)
	
	// Failed processing
	failHandler := func(ctx context.Context, message *Message) error {
		return fmt.Errorf("test error")
	}
	
	failMsg := &Message{
		ID:    "fail-msg",
		Topic: "test",
		Data:  []byte("fail"),
	}
	
	consumer.ProcessMessage(ctx, failMsg, failHandler)
	
	// Check metrics
	if consumer.metrics.messagesProcessed < 1 {
		t.Error("Should have at least 1 processed message")
	}
	
	if consumer.metrics.messagesFailed < 1 {
		t.Error("Should have at least 1 failed message")
	}
	
	if consumer.metrics.avgProcessingTime <= 0 {
		t.Error("Average processing time should be positive")
	}
	
	t.Log("Consumer metrics working correctly")
}

// ベンチマークテスト
func BenchmarkIdempotentConsumer_ProcessMessage(b *testing.B) {
	retryStrategy := &ExponentialBackoffStrategy{
		BaseDelay:  1 * time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		MaxRetries: 1,
	}
	dlq := NewSimpleDeadLetterQueue()
	pubsub := NewInMemoryPubSub()
	consumer := NewIdempotentConsumer(pubsub, retryStrategy, dlq)
	
	handler := func(ctx context.Context, message *Message) error {
		return nil
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			message := &Message{
				ID:        fmt.Sprintf("bench-msg-%d", i),
				Topic:     "bench",
				Data:      []byte("bench data"),
				Timestamp: time.Now(),
			}
			
			consumer.ProcessMessage(ctx, message, handler)
			i++
		}
	})
}

func BenchmarkInMemoryPubSub_Publish(b *testing.B) {
	pubsub := NewInMemoryPubSub()
	defer pubsub.Close()
	
	handler := func(ctx context.Context, message *Message) error {
		return nil
	}
	
	ctx := context.Background()
	pubsub.Subscribe(ctx, "bench-topic", handler)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			message := &Message{
				ID:        fmt.Sprintf("bench-msg-%d", i),
				Topic:     "bench-topic",
				Data:      []byte("bench data"),
				Timestamp: time.Now(),
			}
			
			pubsub.Publish(ctx, "bench-topic", message)
			i++
		}
	})
}