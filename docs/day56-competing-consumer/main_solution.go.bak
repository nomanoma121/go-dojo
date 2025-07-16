// Day 56: 競合コンシューマーパターン
// 同じキューを複数のコンシューマーで処理させ、スループットを向上させる

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Message メッセージ構造体
type Message struct {
	ID           string      `json:"id"`
	Type         string      `json:"type"`
	Data         interface{} `json:"data"`
	Priority     int         `json:"priority"`
	Timestamp    time.Time   `json:"timestamp"`
	PartitionKey string      `json:"partition_key,omitempty"`
	RetryCount   int         `json:"retry_count"`
}

// MessageProcessor メッセージ処理インターフェース
type MessageProcessor interface {
	Process(ctx context.Context, msg *Message) error
	GetProcessorID() string
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

// Consumer コンシューマーインターフェース
type Consumer interface {
	Start(ctx context.Context) error
	Stop() error
	GetStats() ConsumerStats
	IsHealthy() bool
}

// ConsumerStats コンシューマー統計
type ConsumerStats struct {
	ProcessedCount int64     `json:"processed_count"`
	ErrorCount     int64     `json:"error_count"`
	LastProcessed  time.Time `json:"last_processed"`
	AverageLatency time.Duration `json:"average_latency"`
	IsActive       bool      `json:"is_active"`
}

// CompetingConsumer 競合コンシューマー実装
type CompetingConsumer struct {
	id                string
	processor         MessageProcessor
	queue             Queue
	concurrency       int
	pollInterval      time.Duration
	maxIdleTime       time.Duration
	stats             *ConsumerStats
	running           int64
	stopCh            chan struct{}
	workerWg          sync.WaitGroup
	mu                sync.RWMutex
	healthCheck       HealthChecker
	lastHealthCheck   time.Time
	errorThreshold    int64
	consecutiveErrors int64
}

type HealthChecker interface {
	CheckHealth(consumer *CompetingConsumer) bool
}

func NewCompetingConsumer(id string, processor MessageProcessor, queue Queue, concurrency int) *CompetingConsumer {
	return &CompetingConsumer{
		id:             id,
		processor:      processor,
		queue:          queue,
		concurrency:    concurrency,
		pollInterval:   100 * time.Millisecond,
		maxIdleTime:    5 * time.Second,
		stats:          &ConsumerStats{},
		stopCh:         make(chan struct{}),
		errorThreshold: 10,
		healthCheck:    &DefaultHealthChecker{},
	}
}

func (c *CompetingConsumer) SetPollInterval(interval time.Duration) *CompetingConsumer {
	c.pollInterval = interval
	return c
}

func (c *CompetingConsumer) SetHealthChecker(checker HealthChecker) *CompetingConsumer {
	c.healthCheck = checker
	return c
}

func (c *CompetingConsumer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt64(&c.running, 0, 1) {
		return fmt.Errorf("consumer %s is already running", c.id)
	}
	
	log.Printf("Starting consumer %s with %d workers", c.id, c.concurrency)
	
	c.mu.Lock()
	c.stats.IsActive = true
	c.mu.Unlock()
	
	// ワーカーゴルーチンを起動
	for i := 0; i < c.concurrency; i++ {
		c.workerWg.Add(1)
		go c.worker(ctx, i)
	}
	
	// ヘルスチェックゴルーチンを起動
	c.workerWg.Add(1)
	go c.healthCheckWorker(ctx)
	
	return nil
}

func (c *CompetingConsumer) worker(ctx context.Context, workerID int) {
	defer c.workerWg.Done()
	
	log.Printf("Worker %d started for consumer %s", workerID, c.id)
	
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping due to context cancellation", workerID)
			return
		case <-c.stopCh:
			log.Printf("Worker %d stopping due to stop signal", workerID)
			return
		case <-ticker.C:
			c.processMessages(ctx, workerID)
		}
	}
}

func (c *CompetingConsumer) processMessages(ctx context.Context, workerID int) {
	// キューからメッセージを取得
	msg, err := c.queue.Receive(ctx)
	if err != nil {
		// キューが空の場合は正常なのでログ出力しない
		return
	}
	
	start := time.Now()
	
	// メッセージを処理
	err = c.processor.Process(ctx, msg)
	
	processingTime := time.Since(start)
	
	if err != nil {
		// 処理失敗
		c.handleProcessingError(ctx, msg, err, workerID)
		atomic.AddInt64(&c.consecutiveErrors, 1)
	} else {
		// 処理成功
		c.queue.Ack(ctx, msg)
		c.updateSuccessStats(processingTime)
		atomic.StoreInt64(&c.consecutiveErrors, 0)
		
		log.Printf("Worker %d processed message %s in %v", workerID, msg.ID, processingTime)
	}
}

func (c *CompetingConsumer) handleProcessingError(ctx context.Context, msg *Message, err error, workerID int) {
	c.mu.Lock()
	c.stats.ErrorCount++
	c.mu.Unlock()
	
	log.Printf("Worker %d failed to process message %s: %v", workerID, msg.ID, err)
	
	// エラーの種類によって対応を変える
	if c.isRetryableError(err) && msg.RetryCount < 3 {
		// 再試行可能なエラーの場合は再キューに戻す
		msg.RetryCount++
		c.queue.Send(ctx, msg)
		log.Printf("Message %s requeued for retry (attempt %d)", msg.ID, msg.RetryCount)
	} else {
		// 再試行不可能または最大再試行回数に達した場合は NACK
		c.queue.Nack(ctx, msg)
		log.Printf("Message %s rejected after %d retries", msg.ID, msg.RetryCount)
	}
}

func (c *CompetingConsumer) isRetryableError(err error) bool {
	// 簡単なエラー分類（実際の実装ではより詳細な分類が必要）
	errorMsg := err.Error()
	retryableErrors := []string{"timeout", "network", "temporary", "busy"}
	
	for _, retryable := range retryableErrors {
		if contains(errorMsg, retryable) {
			return true
		}
	}
	return false
}

func (c *CompetingConsumer) updateSuccessStats(processingTime time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.stats.ProcessedCount++
	c.stats.LastProcessed = time.Now()
	
	// 移動平均でレイテンシを計算
	if c.stats.ProcessedCount == 1 {
		c.stats.AverageLatency = processingTime
	} else {
		alpha := 0.1 // 平滑化係数
		c.stats.AverageLatency = time.Duration(
			float64(c.stats.AverageLatency)*(1-alpha) + float64(processingTime)*alpha,
		)
	}
}

func (c *CompetingConsumer) healthCheckWorker(ctx context.Context) {
	defer c.workerWg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.performHealthCheck()
		}
	}
}

func (c *CompetingConsumer) performHealthCheck() {
	c.mu.Lock()
	c.lastHealthCheck = time.Now()
	c.mu.Unlock()
	
	healthy := c.healthCheck.CheckHealth(c)
	
	if !healthy {
		log.Printf("Consumer %s failed health check", c.id)
		// 必要に応じて自動回復処理やアラートを実装
	}
}

func (c *CompetingConsumer) Stop() error {
	if !atomic.CompareAndSwapInt64(&c.running, 1, 0) {
		return fmt.Errorf("consumer %s is not running", c.id)
	}
	
	log.Printf("Stopping consumer %s", c.id)
	
	close(c.stopCh)
	c.workerWg.Wait()
	
	c.mu.Lock()
	c.stats.IsActive = false
	c.mu.Unlock()
	
	log.Printf("Consumer %s stopped", c.id)
	return nil
}

func (c *CompetingConsumer) GetStats() ConsumerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return ConsumerStats{
		ProcessedCount: c.stats.ProcessedCount,
		ErrorCount:     c.stats.ErrorCount,
		LastProcessed:  c.stats.LastProcessed,
		AverageLatency: c.stats.AverageLatency,
		IsActive:       c.stats.IsActive,
	}
}

func (c *CompetingConsumer) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// 連続エラー数と最終処理時刻をチェック
	consecutiveErrors := atomic.LoadInt64(&c.consecutiveErrors)
	timeSinceLastProcessed := time.Since(c.stats.LastProcessed)
	
	return consecutiveErrors < c.errorThreshold && 
		   timeSinceLastProcessed < c.maxIdleTime &&
		   c.stats.IsActive
}

// DefaultHealthChecker デフォルトヘルスチェッカー
type DefaultHealthChecker struct{}

func (h *DefaultHealthChecker) CheckHealth(consumer *CompetingConsumer) bool {
	return consumer.IsHealthy()
}

// ConsumerGroup 複数のコンシューマーを管理するグループ
type ConsumerGroup struct {
	name           string
	consumers      map[string]*CompetingConsumer
	loadBalancer   LoadBalancer
	monitor        GroupMonitor
	mu             sync.RWMutex
	rebalancer     *ConsumerRebalancer
}

type LoadBalancer interface {
	SelectConsumer(consumers []*CompetingConsumer) *CompetingConsumer
}

type GroupMonitor interface {
	MonitorGroup(group *ConsumerGroup)
}

func NewConsumerGroup(name string, loadBalancer LoadBalancer) *ConsumerGroup {
	group := &ConsumerGroup{
		name:         name,
		consumers:    make(map[string]*CompetingConsumer),
		loadBalancer: loadBalancer,
		monitor:      &DefaultGroupMonitor{},
	}
	
	group.rebalancer = NewConsumerRebalancer(group)
	return group
}

func (g *ConsumerGroup) AddConsumer(consumer *CompetingConsumer) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	g.consumers[consumer.id] = consumer
	log.Printf("Consumer %s added to group %s", consumer.id, g.name)
}

func (g *ConsumerGroup) RemoveConsumer(consumerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if consumer, exists := g.consumers[consumerID]; exists {
		consumer.Stop()
		delete(g.consumers, consumerID)
		log.Printf("Consumer %s removed from group %s", consumerID, g.name)
	}
}

func (g *ConsumerGroup) StartAll(ctx context.Context) error {
	g.mu.RLock()
	consumers := make([]*CompetingConsumer, 0, len(g.consumers))
	for _, consumer := range g.consumers {
		consumers = append(consumers, consumer)
	}
	g.mu.RUnlock()
	
	for _, consumer := range consumers {
		if err := consumer.Start(ctx); err != nil {
			return fmt.Errorf("failed to start consumer %s: %w", consumer.id, err)
		}
	}
	
	// グループ監視を開始
	go g.monitor.MonitorGroup(g)
	
	// リバランサーを開始
	go g.rebalancer.Start(ctx)
	
	return nil
}

func (g *ConsumerGroup) StopAll() error {
	g.mu.RLock()
	consumers := make([]*CompetingConsumer, 0, len(g.consumers))
	for _, consumer := range g.consumers {
		consumers = append(consumers, consumer)
	}
	g.mu.RUnlock()
	
	for _, consumer := range consumers {
		if err := consumer.Stop(); err != nil {
			log.Printf("Failed to stop consumer %s: %v", consumer.id, err)
		}
	}
	
	return nil
}

func (g *ConsumerGroup) GetGroupStats() GroupStats {
	g.mu.RLock()
	defer g.mu.RUnlock()
	
	stats := GroupStats{
		GroupName:     g.name,
		ConsumerCount: len(g.consumers),
		ConsumerStats: make(map[string]ConsumerStats),
	}
	
	var totalProcessed, totalErrors int64
	var totalLatency time.Duration
	activeCount := 0
	
	for id, consumer := range g.consumers {
		consumerStats := consumer.GetStats()
		stats.ConsumerStats[id] = consumerStats
		
		totalProcessed += consumerStats.ProcessedCount
		totalErrors += consumerStats.ErrorCount
		totalLatency += consumerStats.AverageLatency
		
		if consumerStats.IsActive {
			activeCount++
		}
	}
	
	stats.TotalProcessed = totalProcessed
	stats.TotalErrors = totalErrors
	stats.ActiveConsumers = activeCount
	
	if activeCount > 0 {
		stats.AverageLatency = totalLatency / time.Duration(activeCount)
	}
	
	return stats
}

type GroupStats struct {
	GroupName       string                    `json:"group_name"`
	ConsumerCount   int                       `json:"consumer_count"`
	ActiveConsumers int                       `json:"active_consumers"`
	TotalProcessed  int64                     `json:"total_processed"`
	TotalErrors     int64                     `json:"total_errors"`
	AverageLatency  time.Duration             `json:"average_latency"`
	ConsumerStats   map[string]ConsumerStats  `json:"consumer_stats"`
}

// DefaultGroupMonitor デフォルトグループモニター
type DefaultGroupMonitor struct{}

func (m *DefaultGroupMonitor) MonitorGroup(group *ConsumerGroup) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		stats := group.GetGroupStats()
		log.Printf("Group %s stats: %d consumers, %d active, %d processed, %d errors", 
			stats.GroupName, stats.ConsumerCount, stats.ActiveConsumers, 
			stats.TotalProcessed, stats.TotalErrors)
	}
}

// RoundRobinLoadBalancer ラウンドロビン負荷分散器
type RoundRobinLoadBalancer struct {
	lastIndex int64
}

func NewRoundRobinLoadBalancer() *RoundRobinLoadBalancer {
	return &RoundRobinLoadBalancer{}
}

func (lb *RoundRobinLoadBalancer) SelectConsumer(consumers []*CompetingConsumer) *CompetingConsumer {
	if len(consumers) == 0 {
		return nil
	}
	
	index := atomic.AddInt64(&lb.lastIndex, 1) % int64(len(consumers))
	return consumers[index]
}

// WeightedLoadBalancer 重み付き負荷分散器
type WeightedLoadBalancer struct{}

func NewWeightedLoadBalancer() *WeightedLoadBalancer {
	return &WeightedLoadBalancer{}
}

func (lb *WeightedLoadBalancer) SelectConsumer(consumers []*CompetingConsumer) *CompetingConsumer {
	if len(consumers) == 0 {
		return nil
	}
	
	// パフォーマンスに基づいてコンシューマーを選択
	var bestConsumer *CompetingConsumer
	bestScore := -1.0
	
	for _, consumer := range consumers {
		if !consumer.IsHealthy() {
			continue
		}
		
		stats := consumer.GetStats()
		
		// スコア計算（レイテンシが低く、エラー率が低いほど高スコア）
		score := 1.0
		if stats.AverageLatency > 0 {
			score /= float64(stats.AverageLatency.Milliseconds())
		}
		if stats.ProcessedCount > 0 {
			errorRate := float64(stats.ErrorCount) / float64(stats.ProcessedCount)
			score *= (1.0 - errorRate)
		}
		
		if score > bestScore {
			bestScore = score
			bestConsumer = consumer
		}
	}
	
	if bestConsumer == nil && len(consumers) > 0 {
		// 全てのコンシューマーが不健全な場合はランダム選択
		return consumers[rand.Intn(len(consumers))]
	}
	
	return bestConsumer
}

// ConsumerRebalancer コンシューマーリバランサー
type ConsumerRebalancer struct {
	group            *ConsumerGroup
	rebalanceInterval time.Duration
	lastRebalance     time.Time
}

func NewConsumerRebalancer(group *ConsumerGroup) *ConsumerRebalancer {
	return &ConsumerRebalancer{
		group:             group,
		rebalanceInterval: 5 * time.Minute,
	}
}

func (r *ConsumerRebalancer) Start(ctx context.Context) {
	ticker := time.NewTicker(r.rebalanceInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.performRebalancing()
		}
	}
}

func (r *ConsumerRebalancer) performRebalancing() {
	r.group.mu.RLock()
	consumers := make([]*CompetingConsumer, 0, len(r.group.consumers))
	for _, consumer := range r.group.consumers {
		consumers = append(consumers, consumer)
	}
	r.group.mu.RUnlock()
	
	// 不健全なコンシューマーを特定
	unhealthyConsumers := make([]*CompetingConsumer, 0)
	for _, consumer := range consumers {
		if !consumer.IsHealthy() {
			unhealthyConsumers = append(unhealthyConsumers, consumer)
		}
	}
	
	if len(unhealthyConsumers) > 0 {
		log.Printf("Rebalancing: found %d unhealthy consumers", len(unhealthyConsumers))
		
		// 不健全なコンシューマーを再起動
		for _, consumer := range unhealthyConsumers {
			log.Printf("Restarting unhealthy consumer %s", consumer.id)
			consumer.Stop()
			time.Sleep(1 * time.Second)
			consumer.Start(context.Background())
		}
	}
	
	r.lastRebalance = time.Now()
}

// InMemoryQueue シンプルなメモリキュー実装
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
	
	return msg, nil
}

func (q *InMemoryQueue) Ack(ctx context.Context, msg *Message) error {
	// ACK処理（この実装では何もしない）
	return nil
}

func (q *InMemoryQueue) Nack(ctx context.Context, msg *Message) error {
	// NACK処理（この実装では再キューイング）
	return q.Send(ctx, msg)
}

func (q *InMemoryQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.messages)
}

// OrderProcessor 注文処理プロセッサーの例
type OrderProcessor struct {
	id           string
	processingTime time.Duration
	errorRate    float64
}

func NewOrderProcessor(id string, processingTime time.Duration, errorRate float64) *OrderProcessor {
	return &OrderProcessor{
		id:             id,
		processingTime: processingTime,
		errorRate:      errorRate,
	}
}

func (p *OrderProcessor) Process(ctx context.Context, msg *Message) error {
	// 処理時間をシミュレート
	time.Sleep(p.processingTime)
	
	// エラーをランダムに発生
	if p.errorRate > 0 && rand.Float64() < p.errorRate {
		return fmt.Errorf("random processing error")
	}
	
	// 成功時の処理
	if orderData, ok := msg.Data.(map[string]interface{}); ok {
		orderID := orderData["order_id"]
		log.Printf("Processor %s processed order %v", p.id, orderID)
	}
	
	return nil
}

func (p *OrderProcessor) GetProcessorID() string {
	return p.id
}

func (p *OrderProcessor) GetProcessorType() string {
	return "order_processor"
}

// ユーティリティ関数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

func main() {
	fmt.Println("Day 56: 競合コンシューマーパターン")
	fmt.Println("Run 'go test -v' to see the competing consumer system in action")
}