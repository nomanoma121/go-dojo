// Day 56: 競合コンシューマーパターン
// 同じキューを複数のコンシューマーで処理させ、スループットを向上させる

package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Message メッセージ構造体
type Message struct {
	ID        int       // メッセージの一意識別子
	Data      string    // メッセージの内容
	RetryCount int      // 再試行回数
	Timestamp time.Time // メッセージ生成時刻
	Priority  string    // 優先度（オプション）
}

// Queue インターフェース
type Queue interface {
	Enqueue(msg Message) error                        // メッセージをキューに追加
	Dequeue(ctx context.Context) (*Message, error)    // メッセージをキューから取得
	Size() int                                        // キューのサイズを取得
	Close() error                                     // キューをクローズ
}

// InMemoryQueue インメモリキュー実装
type InMemoryQueue struct {
	messages []Message
	mutex    sync.Mutex
	cond     *sync.Cond
	closed   bool
}

// NewInMemoryQueue 新しいインメモリキューを作成
func NewInMemoryQueue() *InMemoryQueue {
	q := &InMemoryQueue{
		messages: make([]Message, 0),
	}
	q.cond = sync.NewCond(&q.mutex)
	return q
}

// Enqueue メッセージをキューに追加
func (q *InMemoryQueue) Enqueue(msg Message) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	if q.closed {
		return fmt.Errorf("queue is closed")
	}
	
	q.messages = append(q.messages, msg)
	q.cond.Signal()
	return nil
}

// Dequeue メッセージをキューから取得
func (q *InMemoryQueue) Dequeue(ctx context.Context) (*Message, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	for len(q.messages) == 0 && !q.closed {
		// check for context cancellation before waiting
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		// Use a timeout-based approach instead of cond.Wait()
		q.mutex.Unlock()
		
		// Wait for either a signal or context cancellation
		select {
		case <-ctx.Done():
			q.mutex.Lock()
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Reacquire lock and check again
			q.mutex.Lock()
		}
	}
	
	if q.closed && len(q.messages) == 0 {
		return nil, fmt.Errorf("queue is closed")
	}
	
	msg := q.messages[0]
	q.messages = q.messages[1:]
	return &msg, nil
}

// Size キューのサイズを取得
func (q *InMemoryQueue) Size() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.messages)
}

// Close キューをクローズ
func (q *InMemoryQueue) Close() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.closed = true
	q.cond.Broadcast()
	return nil
}

// Consumer コンシューマー構造体
type Consumer struct {
	ID        string
	queue     Queue
	processor MessageProcessor
	stats     ConsumerStats
	done      chan struct{}
	mutex     sync.RWMutex
}

// MessageProcessor メッセージ処理関数の型定義
type MessageProcessor func(msg Message) error

// ConsumerStats コンシューマー統計
type ConsumerStats struct {
	ProcessedCount       int64         // 処理済みメッセージ数
	ErrorCount          int64         // エラー発生数
	TotalProcessingTime time.Duration // 総処理時間
	LastProcessedAt     time.Time     // 最後に処理した時刻
}

// NewConsumer 新しいコンシューマーを作成
func NewConsumer(id string, queue Queue, processor MessageProcessor) *Consumer {
	return &Consumer{
		ID:        id,
		queue:     queue,
		processor: processor,
		done:      make(chan struct{}),
	}
}

// Start コンシューマーを開始
func (c *Consumer) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-c.done:
				return
			case <-ctx.Done():
				return
			default:
				msg, err := c.queue.Dequeue(ctx)
				if err != nil {
					if err == context.Canceled || err == context.DeadlineExceeded {
						return
					}
					continue
				}
				
				start := time.Now()
				err = c.processor(*msg)
				duration := time.Since(start)
				
				c.mutex.Lock()
				c.stats.ProcessedCount++
				c.stats.TotalProcessingTime += duration
				c.stats.LastProcessedAt = time.Now()
				if err != nil {
					c.stats.ErrorCount++
				}
				c.mutex.Unlock()
			}
		}
	}()
}

// Stop コンシューマーを停止
func (c *Consumer) Stop() {
	close(c.done)
}

// GetStats 統計情報を取得
func (c *Consumer) GetStats() ConsumerStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.stats
}

// ConsumerGroup コンシューマー群
type ConsumerGroup struct {
	consumers []*Consumer
	queue     Queue
	wg        sync.WaitGroup
}

// NewConsumerGroup 新しいコンシューマー群を作成
func NewConsumerGroup(queue Queue, consumerCount int, processor MessageProcessor) *ConsumerGroup {
	cg := &ConsumerGroup{
		consumers: make([]*Consumer, consumerCount),
		queue:     queue,
	}
	
	for i := 0; i < consumerCount; i++ {
		id := fmt.Sprintf("consumer-%d", i)
		cg.consumers[i] = NewConsumer(id, queue, processor)
	}
	
	return cg
}

// Start 全てのコンシューマーを開始
func (cg *ConsumerGroup) Start(ctx context.Context) {
	for _, consumer := range cg.consumers {
		cg.wg.Add(1)
		go func(c *Consumer) {
			defer cg.wg.Done()
			c.Start(ctx)
		}(consumer)
	}
}

// Stop 全てのコンシューマーを停止
func (cg *ConsumerGroup) Stop() {
	for _, consumer := range cg.consumers {
		consumer.Stop()
	}
	cg.wg.Wait()
}

// GetAggregatedStats 全コンシューマーの統計情報を集約
func (cg *ConsumerGroup) GetAggregatedStats() map[string]ConsumerStats {
	stats := make(map[string]ConsumerStats)
	for _, consumer := range cg.consumers {
		stats[consumer.ID] = consumer.GetStats()
	}
	return stats
}

// Producer プロデューサー構造体
type Producer struct {
	queue     Queue
	messageID int64
}

// NewProducer 新しいプロデューサーを作成
func NewProducer(queue Queue) *Producer {
	return &Producer{
		queue: queue,
	}
}

// Produce メッセージを生成してキューに送信
func (p *Producer) Produce(data string) error {
	id := atomic.AddInt64(&p.messageID, 1)
	msg := Message{
		ID:        int(id),
		Data:      data,
		Timestamp: time.Now(),
		Priority:  "normal",
	}
	return p.queue.Enqueue(msg)
}

// ProduceBatch 複数のメッセージを一括送信
func (p *Producer) ProduceBatch(dataList []string) error {
	for _, data := range dataList {
		if err := p.Produce(data); err != nil {
			return err
		}
	}
	return nil
}

// LoadBalanceStrategy 負荷分散戦略
type LoadBalanceStrategy int

const (
	RoundRobin LoadBalanceStrategy = iota
	LeastQueue
	Random
)

// LoadBalancer 負荷分散器
type LoadBalancer struct {
	queues            []Queue
	strategy          LoadBalanceStrategy
	roundRobinIndex   int64
	mutex             sync.RWMutex
}

// NewLoadBalancer 新しいロードバランサーを作成
func NewLoadBalancer(queues []Queue, strategy LoadBalanceStrategy) *LoadBalancer {
	return &LoadBalancer{
		queues:   queues,
		strategy: strategy,
	}
}

// SelectQueue 戦略に基づいてキューを選択
func (lb *LoadBalancer) SelectQueue() Queue {
	if len(lb.queues) == 0 {
		return nil
	}
	
	switch lb.strategy {
	case RoundRobin:
		index := atomic.AddInt64(&lb.roundRobinIndex, 1) % int64(len(lb.queues))
		return lb.queues[index]
	case LeastQueue:
		minSize := lb.queues[0].Size()
		minIndex := 0
		for i, queue := range lb.queues {
			if size := queue.Size(); size < minSize {
				minSize = size
				minIndex = i
			}
		}
		return lb.queues[minIndex]
	default:
		return lb.queues[0]
	}
}

func main() {
	fmt.Println("Day 56: 競合コンシューマーパターン")
	fmt.Println("Run 'go test -v' to see the competing consumer system in action")
}