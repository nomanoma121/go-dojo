// Day 53: 冪等なメッセージコンシューマー
// 同じメッセージを複数回受信しても結果が変わらないコンシューマーを実装

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// Message メッセージ構造体
type Message struct {
	ID        string      `json:"id"`
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	Version   int         `json:"version"`
}

// ProcessResult 処理結果
type ProcessResult struct {
	Success     bool        `json:"success"`
	Result      interface{} `json:"result"`
	ProcessedAt time.Time   `json:"processed_at"`
	Error       string      `json:"error,omitempty"`
}

// IdempotencyStorage 冪等性のための状態管理インターフェース
type IdempotencyStorage interface {
	IsProcessed(ctx context.Context, messageID string) (bool, error)
	MarkProcessed(ctx context.Context, messageID string, result *ProcessResult) error
	GetProcessedResult(ctx context.Context, messageID string) (*ProcessResult, error)
	Cleanup(ctx context.Context, olderThan time.Time) error
}

// MessageProcessor メッセージ処理インターフェース
type MessageProcessor interface {
	Process(ctx context.Context, msg *Message) (*ProcessResult, error)
	GetProcessorType() string
}

// DistributedLock 分散ロックインターフェース
type DistributedLock interface {
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (Lock, error)
}

type Lock interface {
	Release() error
	Extend(duration time.Duration) error
}

// IdempotencyMetrics 冪等性メトリクス
type IdempotencyMetrics struct {
	TotalMessages     int64
	DuplicateMessages int64
	ProcessedMessages int64
	FailedMessages    int64
	mu                sync.RWMutex
}

func (m *IdempotencyMetrics) IncrementTotal() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalMessages++
}

func (m *IdempotencyMetrics) IncrementDuplicate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DuplicateMessages++
}

func (m *IdempotencyMetrics) IncrementProcessed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProcessedMessages++
}

func (m *IdempotencyMetrics) IncrementFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedMessages++
}

func (m *IdempotencyMetrics) GetStats() (int64, int64, int64, int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.TotalMessages, m.DuplicateMessages, m.ProcessedMessages, m.FailedMessages
}

// InMemoryIdempotencyStorage メモリベースの冪等性ストレージ
type InMemoryIdempotencyStorage struct {
	data map[string]*ProcessResult
	mu   sync.RWMutex
}

func NewInMemoryIdempotencyStorage() *InMemoryIdempotencyStorage {
	return &InMemoryIdempotencyStorage{
		data: make(map[string]*ProcessResult),
	}
}

func (s *InMemoryIdempotencyStorage) IsProcessed(ctx context.Context, messageID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	_, exists := s.data[messageID]
	return exists, nil
}

func (s *InMemoryIdempotencyStorage) MarkProcessed(ctx context.Context, messageID string, result *ProcessResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.data[messageID] = result
	return nil
}

func (s *InMemoryIdempotencyStorage) GetProcessedResult(ctx context.Context, messageID string) (*ProcessResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if result, exists := s.data[messageID]; exists {
		return result, nil
	}
	return nil, fmt.Errorf("message not found")
}

func (s *InMemoryIdempotencyStorage) Cleanup(ctx context.Context, olderThan time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for messageID, result := range s.data {
		if result.ProcessedAt.Before(olderThan) {
			delete(s.data, messageID)
		}
	}
	return nil
}

// TTLIdempotencyStorage TTL対応の冪等性ストレージ
type TTLIdempotencyStorage struct {
	data map[string]*ProcessResult
	ttl  time.Duration
	mu   sync.RWMutex
}

func NewTTLIdempotencyStorage(ttl time.Duration) *TTLIdempotencyStorage {
	storage := &TTLIdempotencyStorage{
		data: make(map[string]*ProcessResult),
		ttl:  ttl,
	}
	
	// 定期的なクリーンアップを開始
	go storage.startCleanupWorker()
	
	return storage
}

func (s *TTLIdempotencyStorage) startCleanupWorker() {
	ticker := time.NewTicker(s.ttl / 2)
	defer ticker.Stop()
	
	for range ticker.C {
		s.Cleanup(context.Background(), time.Now().Add(-s.ttl))
	}
}

func (s *TTLIdempotencyStorage) IsProcessed(ctx context.Context, messageID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if result, exists := s.data[messageID]; exists {
		// TTLチェック
		if time.Since(result.ProcessedAt) > s.ttl {
			return false, nil
		}
		return true, nil
	}
	return false, nil
}

func (s *TTLIdempotencyStorage) MarkProcessed(ctx context.Context, messageID string, result *ProcessResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.data[messageID] = result
	return nil
}

func (s *TTLIdempotencyStorage) GetProcessedResult(ctx context.Context, messageID string) (*ProcessResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if result, exists := s.data[messageID]; exists {
		if time.Since(result.ProcessedAt) <= s.ttl {
			return result, nil
		}
	}
	return nil, fmt.Errorf("message not found or expired")
}

func (s *TTLIdempotencyStorage) Cleanup(ctx context.Context, olderThan time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for messageID, result := range s.data {
		if result.ProcessedAt.Before(olderThan) {
			delete(s.data, messageID)
		}
	}
	return nil
}

// MockDistributedLock 分散ロックのモック実装
type MockDistributedLock struct {
	locks map[string]bool
	mu    sync.Mutex
}

func NewMockDistributedLock() *MockDistributedLock {
	return &MockDistributedLock{
		locks: make(map[string]bool),
	}
}

func (l *MockDistributedLock) AcquireLock(ctx context.Context, key string, ttl time.Duration) (Lock, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.locks[key] {
		return nil, fmt.Errorf("lock already held")
	}
	
	l.locks[key] = true
	
	return &MockLock{
		lock: l,
		key:  key,
		ttl:  ttl,
	}, nil
}

type MockLock struct {
	lock *MockDistributedLock
	key  string
	ttl  time.Duration
}

func (l *MockLock) Release() error {
	l.lock.mu.Lock()
	defer l.lock.mu.Unlock()
	
	delete(l.lock.locks, l.key)
	return nil
}

func (l *MockLock) Extend(duration time.Duration) error {
	l.ttl += duration
	return nil
}

// OrderProcessor 注文処理の例
type OrderProcessor struct {
	orders map[string]bool
	mu     sync.RWMutex
}

func NewOrderProcessor() *OrderProcessor {
	return &OrderProcessor{
		orders: make(map[string]bool),
	}
}

func (p *OrderProcessor) Process(ctx context.Context, msg *Message) (*ProcessResult, error) {
	orderData, ok := msg.Data.(map[string]interface{})
	if !ok {
		return &ProcessResult{
			Success:     false,
			ProcessedAt: time.Now(),
			Error:       "invalid order data",
		}, fmt.Errorf("invalid order data")
	}
	
	orderID, ok := orderData["order_id"].(string)
	if !ok {
		return &ProcessResult{
			Success:     false,
			ProcessedAt: time.Now(),
			Error:       "missing order_id",
		}, fmt.Errorf("missing order_id")
	}
	
	// 注文処理をシミュレート
	time.Sleep(10 * time.Millisecond)
	
	p.mu.Lock()
	p.orders[orderID] = true
	p.mu.Unlock()
	
	result := &ProcessResult{
		Success:     true,
		Result:      map[string]interface{}{"order_id": orderID, "status": "processed"},
		ProcessedAt: time.Now(),
	}
	
	log.Printf("Processed order: %s", orderID)
	return result, nil
}

func (p *OrderProcessor) GetProcessorType() string {
	return "order_processor"
}

func (p *OrderProcessor) IsOrderProcessed(orderID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.orders[orderID]
}

// IdempotentConsumer 冪等なメッセージコンシューマー
type IdempotentConsumer struct {
	storage          IdempotencyStorage
	processor        MessageProcessor
	metrics          *IdempotencyMetrics
	distributedLock  DistributedLock
	strategy         IdempotencyStrategy
	enableLocking    bool
}

// IdempotencyStrategy 冪等性戦略
type IdempotencyStrategy int

const (
	StrategyMessageID IdempotencyStrategy = iota
	StrategyVersion
	StrategyTimestamp
	StrategyHash
)

func NewIdempotentConsumer(storage IdempotencyStorage, processor MessageProcessor, strategy IdempotencyStrategy) *IdempotentConsumer {
	return &IdempotentConsumer{
		storage:   storage,
		processor: processor,
		metrics:   &IdempotencyMetrics{},
		strategy:  strategy,
	}
}

func (c *IdempotentConsumer) WithDistributedLock(lock DistributedLock) *IdempotentConsumer {
	c.distributedLock = lock
	c.enableLocking = true
	return c
}

func (c *IdempotentConsumer) ProcessMessage(ctx context.Context, msg *Message) error {
	c.metrics.IncrementTotal()
	
	// 冪等性キーを生成
	idempotencyKey := c.generateIdempotencyKey(msg)
	
	// 分散ロックが有効な場合
	if c.enableLocking && c.distributedLock != nil {
		return c.processWithLock(ctx, idempotencyKey, msg)
	}
	
	return c.processMessage(ctx, idempotencyKey, msg)
}

func (c *IdempotentConsumer) processWithLock(ctx context.Context, key string, msg *Message) error {
	lock, err := c.distributedLock.AcquireLock(ctx, "idempotent:"+key, 30*time.Second)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Release()
	
	return c.processMessage(ctx, key, msg)
}

func (c *IdempotentConsumer) processMessage(ctx context.Context, key string, msg *Message) error {
	// 処理済みチェック
	processed, err := c.storage.IsProcessed(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if processed: %w", err)
	}
	
	if processed {
		c.metrics.IncrementDuplicate()
		log.Printf("Message %s already processed, skipping", key)
		
		// 既存の結果を取得
		result, err := c.storage.GetProcessedResult(ctx, key)
		if err == nil && !result.Success {
			// 前回失敗した場合は再処理
			log.Printf("Previous processing failed, retrying message %s", key)
		} else {
			return nil
		}
	}
	
	// メッセージ処理
	result, err := c.processor.Process(ctx, msg)
	if err != nil {
		c.metrics.IncrementFailed()
		
		// 失敗した場合も記録（再処理のため）
		failureResult := &ProcessResult{
			Success:     false,
			ProcessedAt: time.Now(),
			Error:       err.Error(),
		}
		c.storage.MarkProcessed(ctx, key, failureResult)
		
		return fmt.Errorf("message processing failed: %w", err)
	}
	
	// 成功結果を記録
	c.metrics.IncrementProcessed()
	err = c.storage.MarkProcessed(ctx, key, result)
	if err != nil {
		return fmt.Errorf("failed to mark as processed: %w", err)
	}
	
	return nil
}

func (c *IdempotentConsumer) generateIdempotencyKey(msg *Message) string {
	switch c.strategy {
	case StrategyMessageID:
		return msg.ID
	case StrategyVersion:
		return fmt.Sprintf("%s:v%d", msg.ID, msg.Version)
	case StrategyTimestamp:
		return fmt.Sprintf("%s:%d", msg.ID, msg.Timestamp.Unix())
	case StrategyHash:
		return c.generateMessageHash(msg)
	default:
		return msg.ID
	}
}

func (c *IdempotentConsumer) generateMessageHash(msg *Message) string {
	h := sha256.New()
	h.Write([]byte(msg.ID))
	h.Write([]byte(msg.Type))
	h.Write([]byte(fmt.Sprintf("%v", msg.Data)))
	return hex.EncodeToString(h.Sum(nil))[:16] // 最初の16文字を使用
}

func (c *IdempotentConsumer) GetMetrics() *IdempotencyMetrics {
	return c.metrics
}

func (c *IdempotentConsumer) Cleanup(ctx context.Context, olderThan time.Time) error {
	return c.storage.Cleanup(ctx, olderThan)
}

// BatchIdempotentConsumer バッチ処理対応の冪等コンシューマー
type BatchIdempotentConsumer struct {
	*IdempotentConsumer
}

func NewBatchIdempotentConsumer(storage IdempotencyStorage, processor MessageProcessor) *BatchIdempotentConsumer {
	return &BatchIdempotentConsumer{
		IdempotentConsumer: NewIdempotentConsumer(storage, processor, StrategyMessageID),
	}
}

func (c *BatchIdempotentConsumer) ProcessBatch(ctx context.Context, messages []*Message) error {
	for _, msg := range messages {
		if err := c.ProcessMessage(ctx, msg); err != nil {
			log.Printf("Failed to process message %s: %v", msg.ID, err)
			// バッチ処理では個別のエラーを記録するが処理を続行
		}
	}
	return nil
}

// VersionBasedIdempotentConsumer バージョンベースの冪等コンシューマー
type VersionBasedIdempotentConsumer struct {
	*IdempotentConsumer
}

func NewVersionBasedIdempotentConsumer(storage IdempotencyStorage, processor MessageProcessor) *VersionBasedIdempotentConsumer {
	return &VersionBasedIdempotentConsumer{
		IdempotentConsumer: NewIdempotentConsumer(storage, processor, StrategyVersion),
	}
}

func (c *VersionBasedIdempotentConsumer) ProcessMessage(ctx context.Context, msg *Message) error {
	// より新しいバージョンのメッセージのみを処理
	for version := 1; version <= msg.Version; version++ {
		versionedKey := fmt.Sprintf("%s:v%d", msg.ID, version)
		processed, _ := c.storage.IsProcessed(ctx, versionedKey)
		
		if version == msg.Version && !processed {
			return c.IdempotentConsumer.ProcessMessage(ctx, msg)
		} else if version == msg.Version && processed {
			c.metrics.IncrementDuplicate()
			log.Printf("Version %d of message %s already processed", version, msg.ID)
			return nil
		}
	}
	
	return nil
}

func main() {
	fmt.Println("Day 53: 冪等なメッセージコンシューマー")
	fmt.Println("Run 'go test -v' to see the idempotent consumer system in action")
}