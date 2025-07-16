// Day 54: Dead-Letter Queue (DLQ)
// 処理に失敗し続けるメッセージを隔離する仕組みを実装

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// Message メッセージ構造体
type Message struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Data        interface{}            `json:"data"`
	Headers     map[string]string      `json:"headers"`
	Timestamp   time.Time              `json:"timestamp"`
	RetryCount  int                    `json:"retry_count"`
	MaxRetries  int                    `json:"max_retries"`
	LastError   string                 `json:"last_error,omitempty"`
	FailureTime time.Time              `json:"failure_time,omitempty"`
}

// DeadLetterMessage DLQに送信されるメッセージ
type DeadLetterMessage struct {
	*Message
	Reason         string    `json:"reason"`
	OriginalQueue  string    `json:"original_queue"`
	DeadLetterTime time.Time `json:"dead_letter_time"`
	FailureHistory []FailureRecord `json:"failure_history"`
}

// FailureRecord 失敗記録
type FailureRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error"`
	RetryNumber int      `json:"retry_number"`
}

// MessageProcessor メッセージ処理インターフェース
type MessageProcessor interface {
	Process(ctx context.Context, msg *Message) error
	GetProcessorType() string
}

// Queue キューインターフェース
type Queue interface {
	Send(ctx context.Context, msg *Message) error
	Receive(ctx context.Context) (*Message, error)
	Ack(ctx context.Context, msg *Message) error
	Nack(ctx context.Context, msg *Message) error
	Size() int
}

// DeadLetterQueue Dead Letter Queueインターフェース
type DeadLetterQueue interface {
	SendToDeadLetter(ctx context.Context, msg *Message, reason string, originalQueue string) error
	GetDeadLetterMessages(ctx context.Context) ([]*DeadLetterMessage, error)
	ReprocessMessage(ctx context.Context, dlqMsg *DeadLetterMessage, targetQueue string) error
	PurgeExpiredMessages(ctx context.Context, expiration time.Duration) error
}

// RetryPolicy 再試行ポリシー
type RetryPolicy struct {
	MaxRetries      int
	InitialDelay    time.Duration
	BackoffFactor   float64
	MaxDelay        time.Duration
	RetryableErrors map[string]bool
}

func NewDefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:    3,
		InitialDelay:  1 * time.Second,
		BackoffFactor: 2.0,
		MaxDelay:      30 * time.Second,
		RetryableErrors: map[string]bool{
			"temporary_error":   true,
			"network_error":     true,
			"resource_busy":     true,
			"service_unavailable": true,
		},
	}
}

func (p *RetryPolicy) CalculateDelay(retryCount int) time.Duration {
	delay := float64(p.InitialDelay)
	for i := 0; i < retryCount; i++ {
		delay *= p.BackoffFactor
	}
	
	if time.Duration(delay) > p.MaxDelay {
		return p.MaxDelay
	}
	
	return time.Duration(delay)
}

func (p *RetryPolicy) IsRetryable(errorType string) bool {
	return p.RetryableErrors[errorType]
}

// InMemoryQueue メモリベースのキュー実装
type InMemoryQueue struct {
	name     string
	messages []*Message
	mu       sync.RWMutex
}

func NewInMemoryQueue(name string) *InMemoryQueue {
	return &InMemoryQueue{
		name:     name,
		messages: make([]*Message, 0),
	}
}

func (q *InMemoryQueue) Send(ctx context.Context, msg *Message) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	q.messages = append(q.messages, msg)
	log.Printf("Message %s sent to queue %s", msg.ID, q.name)
	return nil
}

func (q *InMemoryQueue) Receive(ctx context.Context) (*Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if len(q.messages) == 0 {
		return nil, fmt.Errorf("queue empty")
	}
	
	msg := q.messages[0]
	q.messages = q.messages[1:]
	
	log.Printf("Message %s received from queue %s", msg.ID, q.name)
	return msg, nil
}

func (q *InMemoryQueue) Ack(ctx context.Context, msg *Message) error {
	log.Printf("Message %s acknowledged", msg.ID)
	return nil
}

func (q *InMemoryQueue) Nack(ctx context.Context, msg *Message) error {
	log.Printf("Message %s not acknowledged", msg.ID)
	return nil
}

func (q *InMemoryQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.messages)
}

// InMemoryDeadLetterQueue メモリベースのDLQ実装
type InMemoryDeadLetterQueue struct {
	messages map[string]*DeadLetterMessage
	mu       sync.RWMutex
}

func NewInMemoryDeadLetterQueue() *InMemoryDeadLetterQueue {
	return &InMemoryDeadLetterQueue{
		messages: make(map[string]*DeadLetterMessage),
	}
}

func (dlq *InMemoryDeadLetterQueue) SendToDeadLetter(ctx context.Context, msg *Message, reason string, originalQueue string) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	
	dlqMsg := &DeadLetterMessage{
		Message:        msg,
		Reason:         reason,
		OriginalQueue:  originalQueue,
		DeadLetterTime: time.Now(),
		FailureHistory: make([]FailureRecord, 0),
	}
	
	// 失敗履歴を追加
	if msg.LastError != "" {
		dlqMsg.FailureHistory = append(dlqMsg.FailureHistory, FailureRecord{
			Timestamp:   msg.FailureTime,
			Error:       msg.LastError,
			RetryNumber: msg.RetryCount,
		})
	}
	
	dlq.messages[msg.ID] = dlqMsg
	log.Printf("Message %s sent to dead letter queue. Reason: %s", msg.ID, reason)
	return nil
}

func (dlq *InMemoryDeadLetterQueue) GetDeadLetterMessages(ctx context.Context) ([]*DeadLetterMessage, error) {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	
	messages := make([]*DeadLetterMessage, 0, len(dlq.messages))
	for _, msg := range dlq.messages {
		messages = append(messages, msg)
	}
	
	return messages, nil
}

func (dlq *InMemoryDeadLetterQueue) ReprocessMessage(ctx context.Context, dlqMsg *DeadLetterMessage, targetQueue string) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	
	// DLQからメッセージを削除
	delete(dlq.messages, dlqMsg.Message.ID)
	
	// 再処理用にメッセージをリセット
	dlqMsg.Message.RetryCount = 0
	dlqMsg.Message.LastError = ""
	
	log.Printf("Message %s reprocessed from DLQ to queue %s", dlqMsg.Message.ID, targetQueue)
	return nil
}

func (dlq *InMemoryDeadLetterQueue) PurgeExpiredMessages(ctx context.Context, expiration time.Duration) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	
	cutoff := time.Now().Add(-expiration)
	expired := make([]string, 0)
	
	for id, msg := range dlq.messages {
		if msg.DeadLetterTime.Before(cutoff) {
			expired = append(expired, id)
		}
	}
	
	for _, id := range expired {
		delete(dlq.messages, id)
	}
	
	log.Printf("Purged %d expired messages from DLQ", len(expired))
	return nil
}

func (dlq *InMemoryDeadLetterQueue) Size() int {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	return len(dlq.messages)
}

// MessageProcessorWithRetry 再試行機能付きメッセージプロセッサー
type MessageProcessorWithRetry struct {
	processor    MessageProcessor
	retryPolicy  *RetryPolicy
	deadLetterQ  DeadLetterQueue
	originalQueue string
}

func NewMessageProcessorWithRetry(processor MessageProcessor, retryPolicy *RetryPolicy, dlq DeadLetterQueue, originalQueue string) *MessageProcessorWithRetry {
	return &MessageProcessorWithRetry{
		processor:     processor,
		retryPolicy:   retryPolicy,
		deadLetterQ:   dlq,
		originalQueue: originalQueue,
	}
}

func (p *MessageProcessorWithRetry) ProcessWithRetry(ctx context.Context, msg *Message) error {
	for {
		err := p.processor.Process(ctx, msg)
		if err == nil {
			log.Printf("Message %s processed successfully", msg.ID)
			return nil
		}
		
		// エラータイプを判定
		errorType := p.classifyError(err)
		msg.LastError = err.Error()
		msg.FailureTime = time.Now()
		
		// 再試行可能かチェック
		if !p.retryPolicy.IsRetryable(errorType) {
			log.Printf("Non-retryable error for message %s: %v", msg.ID, err)
			return p.sendToDeadLetter(ctx, msg, fmt.Sprintf("non-retryable error: %s", errorType))
		}
		
		// 最大再試行回数チェック
		if msg.RetryCount >= p.retryPolicy.MaxRetries {
			log.Printf("Max retries exceeded for message %s", msg.ID)
			return p.sendToDeadLetter(ctx, msg, "max retries exceeded")
		}
		
		// 再試行
		msg.RetryCount++
		delay := p.retryPolicy.CalculateDelay(msg.RetryCount - 1)
		
		log.Printf("Retrying message %s in %v (attempt %d/%d)", 
			msg.ID, delay, msg.RetryCount, p.retryPolicy.MaxRetries)
		
		select {
		case <-time.After(delay):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *MessageProcessorWithRetry) classifyError(err error) string {
	errorMsg := err.Error()
	
	// 簡単なエラー分類（実際の実装ではより詳細な分類が必要）
	switch {
	case contains(errorMsg, "network"):
		return "network_error"
	case contains(errorMsg, "timeout"):
		return "timeout_error"
	case contains(errorMsg, "busy"):
		return "resource_busy"
	case contains(errorMsg, "unavailable"):
		return "service_unavailable"
	case contains(errorMsg, "temporary"):
		return "temporary_error"
	default:
		return "unknown_error"
	}
}

func (p *MessageProcessorWithRetry) sendToDeadLetter(ctx context.Context, msg *Message, reason string) error {
	return p.deadLetterQ.SendToDeadLetter(ctx, msg, reason, p.originalQueue)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// OrderProcessor 注文処理プロセッサーの例
type OrderProcessor struct {
	failureRate float64 // テスト用の失敗率
}

func NewOrderProcessor(failureRate float64) *OrderProcessor {
	return &OrderProcessor{
		failureRate: failureRate,
	}
}

func (p *OrderProcessor) Process(ctx context.Context, msg *Message) error {
	// ランダムに失敗をシミュレート
	if p.failureRate > 0 && rand.Float64() < p.failureRate {
		errorTypes := []string{
			"network timeout",
			"service unavailable", 
			"resource busy",
			"temporary database error",
			"payment service down",
		}
		
		errorType := errorTypes[rand.Intn(len(errorTypes))]
		return fmt.Errorf(errorType)
	}
	
	// 成功時の処理
	orderData, ok := msg.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid order data format")
	}
	
	orderID, ok := orderData["order_id"].(string)
	if !ok {
		return fmt.Errorf("missing order_id")
	}
	
	log.Printf("Order %s processed successfully", orderID)
	return nil
}

func (p *OrderProcessor) GetProcessorType() string {
	return "order_processor"
}

// DLQManager DLQ管理システム
type DLQManager struct {
	deadLetterQ DeadLetterQueue
	queues      map[string]Queue
	processors  map[string]MessageProcessor
	mu          sync.RWMutex
}

func NewDLQManager(dlq DeadLetterQueue) *DLQManager {
	return &DLQManager{
		deadLetterQ: dlq,
		queues:      make(map[string]Queue),
		processors:  make(map[string]MessageProcessor),
	}
}

func (m *DLQManager) RegisterQueue(name string, queue Queue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queues[name] = queue
}

func (m *DLQManager) RegisterProcessor(queueName string, processor MessageProcessor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processors[queueName] = processor
}

func (m *DLQManager) GetDLQStatistics(ctx context.Context) (*DLQStatistics, error) {
	messages, err := m.deadLetterQ.GetDeadLetterMessages(ctx)
	if err != nil {
		return nil, err
	}
	
	stats := &DLQStatistics{
		TotalMessages: len(messages),
		MessagesByQueue: make(map[string]int),
		MessagesByReason: make(map[string]int),
		OldestMessage: time.Now(),
	}
	
	for _, msg := range messages {
		stats.MessagesByQueue[msg.OriginalQueue]++
		stats.MessagesByReason[msg.Reason]++
		
		if msg.DeadLetterTime.Before(stats.OldestMessage) {
			stats.OldestMessage = msg.DeadLetterTime
		}
	}
	
	return stats, nil
}

func (m *DLQManager) ReprocessFromDLQ(ctx context.Context, messageID string, targetQueue string) error {
	messages, err := m.deadLetterQ.GetDeadLetterMessages(ctx)
	if err != nil {
		return err
	}
	
	for _, dlqMsg := range messages {
		if dlqMsg.Message.ID == messageID {
			m.mu.RLock()
			queue, exists := m.queues[targetQueue]
			m.mu.RUnlock()
			
			if !exists {
				return fmt.Errorf("target queue %s not found", targetQueue)
			}
			
			// DLQから削除
			err = m.deadLetterQ.ReprocessMessage(ctx, dlqMsg, targetQueue)
			if err != nil {
				return err
			}
			
			// ターゲットキューに送信
			return queue.Send(ctx, dlqMsg.Message)
		}
	}
	
	return fmt.Errorf("message %s not found in DLQ", messageID)
}

func (m *DLQManager) StartCleanupWorker(ctx context.Context, interval time.Duration, expiration time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			err := m.deadLetterQ.PurgeExpiredMessages(ctx, expiration)
			if err != nil {
				log.Printf("Failed to purge expired DLQ messages: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// DLQStatistics DLQ統計情報
type DLQStatistics struct {
	TotalMessages    int               `json:"total_messages"`
	MessagesByQueue  map[string]int    `json:"messages_by_queue"`
	MessagesByReason map[string]int    `json:"messages_by_reason"`
	OldestMessage    time.Time         `json:"oldest_message"`
}

// PoisonMessageDetector 毒メッセージ検出器
type PoisonMessageDetector struct {
	patterns     []PoisonPattern
	deadLetterQ  DeadLetterQueue
}

type PoisonPattern struct {
	Name        string
	Condition   func(*Message) bool
	Description string
}

func NewPoisonMessageDetector(dlq DeadLetterQueue) *PoisonMessageDetector {
	detector := &PoisonMessageDetector{
		deadLetterQ: dlq,
		patterns:    make([]PoisonPattern, 0),
	}
	
	// デフォルトのパターンを追加
	detector.AddPattern(PoisonPattern{
		Name: "malformed_json",
		Condition: func(msg *Message) bool {
			// JSONデータの検証
			_, ok := msg.Data.(string)
			return ok && len(msg.Data.(string)) > 1000000 // 1MB以上
		},
		Description: "Message contains malformed or oversized JSON",
	})
	
	detector.AddPattern(PoisonPattern{
		Name: "circular_reference",
		Condition: func(msg *Message) bool {
			// 循環参照の検出（簡略化）
			if dataMap, ok := msg.Data.(map[string]interface{}); ok {
				return contains(fmt.Sprintf("%v", dataMap), "circular")
			}
			return false
		},
		Description: "Message contains circular references",
	})
	
	return detector
}

func (d *PoisonMessageDetector) AddPattern(pattern PoisonPattern) {
	d.patterns = append(d.patterns, pattern)
}

func (d *PoisonMessageDetector) DetectPoisonMessage(ctx context.Context, msg *Message) (bool, string) {
	for _, pattern := range d.patterns {
		if pattern.Condition(msg) {
			reason := fmt.Sprintf("poison message detected: %s - %s", pattern.Name, pattern.Description)
			
			// 毒メッセージをDLQに送信
			d.deadLetterQ.SendToDeadLetter(ctx, msg, reason, "poison_detection")
			
			return true, reason
		}
	}
	
	return false, ""
}

// ErrorClassification エラー分類
type ErrorClassification int

const (
	TemporaryError ErrorClassification = iota
	PermanentError
	ValidationError
	SecurityError
	TimeoutError
)

// DLQMessage テスト用のDLQメッセージ構造体
type DLQMessage struct {
	OriginalMessage *Message             `json:"original_message"`
	FailureReason   string               `json:"failure_reason"`
	ErrorClass      ErrorClassification  `json:"error_class"`
	FailureCount    int                  `json:"failure_count"`
	FirstFailure    time.Time            `json:"first_failure"`
	LastFailure     time.Time            `json:"last_failure"`
}

// ExponentialBackoffReprocessing 指数バックオフ再処理戦略
type ExponentialBackoffReprocessing struct {
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	MaxAttempts int
	Multiplier  float64
}

func (s *ExponentialBackoffReprocessing) ShouldReprocess(dlqMsg *DLQMessage) bool {
	// 永続エラーは再処理しない
	if dlqMsg.ErrorClass == PermanentError {
		return false
	}
	
	// 最大試行回数チェック
	if dlqMsg.FailureCount >= s.MaxAttempts {
		return false
	}
	
	return true
}

func (s *ExponentialBackoffReprocessing) NextAttemptTime(dlqMsg *DLQMessage) time.Time {
	delay := time.Duration(float64(s.BaseDelay) * pow(s.Multiplier, float64(dlqMsg.FailureCount)))
	if delay > s.MaxDelay {
		delay = s.MaxDelay
	}
	return dlqMsg.LastFailure.Add(delay)
}

func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	result := base
	for i := 1; i < int(exp); i++ {
		result *= base
	}
	return result
}

// DeadLetterQueueTest テスト用のDLQ実装
type DeadLetterQueueTest struct {
	messages map[string]*DLQMessage
	strategy *ExponentialBackoffReprocessing
	mu       sync.RWMutex
}

func NewDeadLetterQueue(strategy *ExponentialBackoffReprocessing) *DeadLetterQueueTest {
	return &DeadLetterQueueTest{
		messages: make(map[string]*DLQMessage),
		strategy: strategy,
	}
}

func (dlq *DeadLetterQueueTest) Send(ctx context.Context, dlqMsg *DLQMessage) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	dlq.messages[dlqMsg.OriginalMessage.ID] = dlqMsg
	return nil
}

func (dlq *DeadLetterQueueTest) GetMessage(messageID string) (*DLQMessage, bool) {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	msg, exists := dlq.messages[messageID]
	return msg, exists
}

func (dlq *DeadLetterQueueTest) RemoveMessage(messageID string) error {
	dlq.mu.Lock()
	defer dlq.mu.Unlock()
	delete(dlq.messages, messageID)
	return nil
}

func (dlq *DeadLetterQueueTest) GetAnalytics() *DLQAnalytics {
	dlq.mu.RLock()
	defer dlq.mu.RUnlock()
	
	analytics := &DLQAnalytics{
		TotalMessages:   len(dlq.messages),
		ErrorBreakdown:  make(map[ErrorClassification]int),
		TopicBreakdown:  make(map[string]int),
		AverageRetries:  0,
		OldestMessage:   nil,
	}
	
	if len(dlq.messages) == 0 {
		return analytics
	}
	
	totalRetries := 0
	var oldestTime time.Time
	var oldestMsg *DLQMessage
	first := true
	
	for _, msg := range dlq.messages {
		analytics.ErrorBreakdown[msg.ErrorClass]++
		analytics.TopicBreakdown[msg.OriginalMessage.Topic]++
		totalRetries += msg.FailureCount
		
		if first || msg.FirstFailure.Before(oldestTime) {
			oldestTime = msg.FirstFailure
			oldestMsg = msg
			first = false
		}
	}
	
	analytics.AverageRetries = float64(totalRetries) / float64(len(dlq.messages))
	analytics.OldestMessage = oldestMsg
	
	return analytics
}

// DLQAnalytics DLQ分析データ
type DLQAnalytics struct {
	TotalMessages   int                              `json:"total_messages"`
	ErrorBreakdown  map[ErrorClassification]int      `json:"error_breakdown"`
	TopicBreakdown  map[string]int                   `json:"topic_breakdown"`
	AverageRetries  float64                          `json:"average_retries"`
	OldestMessage   *DLQMessage                      `json:"oldest_message"`
}

// Message テスト用のメッセージ構造体（簡略版）
type Message struct {
	ID    string `json:"id"`
	Topic string `json:"topic"`
	Data  []byte `json:"data"`
}

// ClassifyError エラー分類関数
func ClassifyError(err error) ErrorClassification {
	errMsg := err.Error()
	
	if contains(errMsg, "deadline exceeded") || contains(errMsg, "timeout") {
		return TimeoutError
	}
	if contains(errMsg, "validation") || contains(errMsg, "invalid") {
		return ValidationError
	}
	if contains(errMsg, "unauthorized") || contains(errMsg, "security") {
		return SecurityError
	}
	if contains(errMsg, "connection") || contains(errMsg, "network") {
		return TemporaryError
	}
	
	return PermanentError
}

// SimplePublisher テスト用のシンプルパブリッシャー
type SimplePublisher struct {
	published []PublishedMessage
	mu        sync.RWMutex
}

type PublishedMessage struct {
	Topic   string   `json:"topic"`
	Message *Message `json:"message"`
}

func NewSimplePublisher() *SimplePublisher {
	return &SimplePublisher{
		published: make([]PublishedMessage, 0),
	}
}

func (p *SimplePublisher) Publish(ctx context.Context, topic string, message *Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, PublishedMessage{
		Topic:   topic,
		Message: message,
	})
	return nil
}

func (p *SimplePublisher) GetPublishedMessages() []PublishedMessage {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]PublishedMessage, len(p.published))
	copy(result, p.published)
	return result
}

// BatchReprocessor バッチ再処理器
type BatchReprocessor struct {
	dlq         *DeadLetterQueueTest
	publisher   *SimplePublisher
	batchSize   int
	strategy    *ExponentialBackoffReprocessing
}

func NewBatchReprocessor(dlq *DeadLetterQueueTest, publisher *SimplePublisher, batchSize int, strategy *ExponentialBackoffReprocessing) *BatchReprocessor {
	return &BatchReprocessor{
		dlq:       dlq,
		publisher: publisher,
		batchSize: batchSize,
		strategy:  strategy,
	}
}

func (r *BatchReprocessor) ReprocessBatch(ctx context.Context, filter func(*DLQMessage) bool) error {
	r.dlq.mu.RLock()
	var toReprocess []*DLQMessage
	for _, msg := range r.dlq.messages {
		if filter(msg) && len(toReprocess) < r.batchSize {
			toReprocess = append(toReprocess, msg)
		}
	}
	r.dlq.mu.RUnlock()
	
	// 再処理
	for _, msg := range toReprocess {
		err := r.publisher.Publish(ctx, msg.OriginalMessage.Topic, msg.OriginalMessage)
		if err != nil {
			return err
		}
		
		// DLQから削除
		r.dlq.RemoveMessage(msg.OriginalMessage.ID)
	}
	
	return nil
}

// DLQMonitor DLQ監視器
type DLQMonitor struct {
	dlq      *DeadLetterQueueTest
	alerting *SimpleAlertingService
	config   MonitorConfig
}

type MonitorConfig struct {
	MaxMessages     int
	MaxMessageAge   time.Duration
	MaxSecurityErrs int
	CheckInterval   time.Duration
}

func NewDLQMonitor(dlq *DeadLetterQueueTest, alerting *SimpleAlertingService, config MonitorConfig) *DLQMonitor {
	return &DLQMonitor{
		dlq:      dlq,
		alerting: alerting,
		config:   config,
	}
}

func (m *DLQMonitor) StartMonitoring(ctx context.Context) {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			analytics := m.dlq.GetAnalytics()
			
			// 高ボリュームアラート
			if analytics.TotalMessages > m.config.MaxMessages {
				m.alerting.SendCriticalAlert("HIGH_VOLUME", fmt.Sprintf("DLQ has %d messages", analytics.TotalMessages))
			}
			
			// 古いメッセージアラート
			if analytics.OldestMessage != nil {
				age := time.Since(analytics.OldestMessage.FirstFailure)
				if age > m.config.MaxMessageAge {
					m.alerting.SendWarningAlert("OLD_MESSAGES", fmt.Sprintf("Oldest message is %v old", age))
				}
			}
			
			// セキュリティエラーアラート
			if securityCount := analytics.ErrorBreakdown[SecurityError]; securityCount > m.config.MaxSecurityErrs {
				m.alerting.SendSecurityAlert("SECURITY", fmt.Sprintf("Too many security errors: %d", securityCount))
			}
			
		case <-ctx.Done():
			return
		}
	}
}

// SimpleAlertingService テスト用のシンプルアラートサービス
type SimpleAlertingService struct {
	alerts []Alert
	mu     sync.RWMutex
}

type Alert struct {
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func NewSimpleAlertingService() *SimpleAlertingService {
	return &SimpleAlertingService{
		alerts: make([]Alert, 0),
	}
}

func (s *SimpleAlertingService) SendWarningAlert(alertType, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, Alert{
		Type:      alertType,
		Level:     "warning",
		Message:   message,
		Timestamp: time.Now(),
	})
	return nil
}

func (s *SimpleAlertingService) SendCriticalAlert(alertType, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, Alert{
		Type:      alertType,
		Level:     "critical",
		Message:   message,
		Timestamp: time.Now(),
	})
	return nil
}

func (s *SimpleAlertingService) SendSecurityAlert(alertType, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, Alert{
		Type:      alertType,
		Level:     "security",
		Message:   message,
		Timestamp: time.Now(),
	})
	return nil
}

func (s *SimpleAlertingService) GetAlerts() []Alert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Alert, len(s.alerts))
	copy(result, s.alerts)
	return result
}

func main() {
	fmt.Println("Day 54: Dead-Letter Queue (DLQ)")
	fmt.Println("Run 'go test -v' to see the dead letter queue system in action")
}