package main

import (
	"context"
	"fmt"
	"log"
	"sync"
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
	mutex    sync.RWMutex
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
		done := make(chan struct{})
		go func() {
			q.cond.Wait()
			close(done)
		}()
		
		q.mutex.Unlock()
		select {
		case <-ctx.Done():
			q.mutex.Lock()
			return nil, ctx.Err()
		case <-done:
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
	q.mutex.RLock()
	defer q.mutex.RUnlock()
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

// TODO: Consumer構造体を実装してください
type Consumer struct {
	// TODO: consumerに必要なフィールド
	// - ID: consumer の識別子
	// - queue: メッセージキュー
	// - processor: メッセージ処理ロジック
	// - stats: 統計情報
	// - done: 停止シグナル
}

// TODO: MessageProcessor型を定義してください
// メッセージ処理関数の型定義
type MessageProcessor func(msg Message) error

// TODO: ConsumerStats構造体を実装してください
type ConsumerStats struct {
	// TODO: consumer統計に必要なフィールド
	ProcessedCount       int64         // 処理済みメッセージ数
	ErrorCount          int64         // エラー発生数
	TotalProcessingTime time.Duration // 総処理時間
	LastProcessedAt     time.Time     // 最後に処理した時刻
}

// TODO: NewConsumer関数を実装してください
func NewConsumer(id string, queue Queue, processor MessageProcessor) *Consumer {
	// ここに実装
	return nil
}

// TODO: Start メソッドを実装してください
// consumerを開始し、メッセージの処理を始める
func (c *Consumer) Start(ctx context.Context) {
	// TODO: 以下の処理を実装
	// 1. ゴルーチンでメッセージ処理ループを開始
	// 2. キューからメッセージを継続的に取得
	// 3. メッセージを処理し、統計情報を更新
	// 4. エラーハンドリング
	// 5. コンテキストキャンセレーション時の正常終了
}

// TODO: Stop メソッドを実装してください
func (c *Consumer) Stop() {
	// ここに実装
}

// TODO: GetStats メソッドを実装してください
func (c *Consumer) GetStats() ConsumerStats {
	// ここに実装
	return ConsumerStats{}
}

// TODO: ConsumerGroup構造体を実装してください
type ConsumerGroup struct {
	// TODO: consumer群の管理に必要なフィールド
	// - consumers: 複数のconsumer
	// - queue: 共有キュー
	// - wg: consumer群の終了待機用
	// - stats: 群全体の統計
}

// TODO: NewConsumerGroup関数を実装してください
func NewConsumerGroup(queue Queue, consumerCount int, processor MessageProcessor) *ConsumerGroup {
	// ここに実装
	return nil
}

// TODO: Start メソッドを実装してください（ConsumerGroup用）
func (cg *ConsumerGroup) Start(ctx context.Context) {
	// TODO: 全てのconsumerを開始
}

// TODO: Stop メソッドを実装してください（ConsumerGroup用）
func (cg *ConsumerGroup) Stop() {
	// TODO: 全てのconsumerを停止し、終了を待機
}

// TODO: GetAggregatedStats メソッドを実装してください
func (cg *ConsumerGroup) GetAggregatedStats() map[string]ConsumerStats {
	// TODO: 全consumerの統計情報を集約
	return nil
}

// TODO: Producer構造体を実装してください
type Producer struct {
	// TODO: producerに必要なフィールド
	// - queue: メッセージキュー
	// - messageID: メッセージIDカウンター
}

// TODO: NewProducer関数を実装してください
func NewProducer(queue Queue) *Producer {
	// ここに実装
	return nil
}

// TODO: Produce メソッドを実装してください
func (p *Producer) Produce(data string) error {
	// TODO: メッセージを生成してキューに送信
	return nil
}

// TODO: ProduceBatch メソッドを実装してください
func (p *Producer) ProduceBatch(dataList []string) error {
	// TODO: 複数のメッセージを一括で送信
	return nil
}

// TODO: LoadBalancer構造体を実装してください
type LoadBalancer struct {
	// TODO: 負荷分散に必要なフィールド
	// - queues: 複数のキュー
	// - strategy: 負荷分散戦略
	// - roundRobinIndex: ラウンドロビン用インデックス
}

// TODO: LoadBalanceStrategy型を定義してください
type LoadBalanceStrategy int

const (
	// TODO: 負荷分散戦略の定数を定義
	// RoundRobin, LeastQueue, Random など
)

// TODO: NewLoadBalancer関数を実装してください
func NewLoadBalancer(queues []Queue, strategy LoadBalanceStrategy) *LoadBalancer {
	// ここに実装
	return nil
}

// TODO: SelectQueue メソッドを実装してください
func (lb *LoadBalancer) SelectQueue() Queue {
	// TODO: 戦略に基づいてキューを選択
	return nil
}

// ===============================
// 以下は動作確認用のサンプル関数
// ===============================

// サンプルのメッセージ処理関数
func sampleProcessor(msg Message) error {
	// 処理時間の模擬
	time.Sleep(time.Duration(50+msg.ID%100) * time.Millisecond)
	
	// ランダムなエラー発生（5%の確率）
	if msg.ID%20 == 0 {
		return fmt.Errorf("processing failed for message %d", msg.ID)
	}
	
	log.Printf("Consumer processed message ID: %d, Data: %s", msg.ID, msg.Data)
	return nil
}

// サンプルの統計表示関数
func displayStats(cg *ConsumerGroup) {
	stats := cg.GetAggregatedStats()
	for consumerID, stat := range stats {
		log.Printf("Consumer %s: Processed=%d, Errors=%d, AvgTime=%.2fms",
			consumerID, stat.ProcessedCount, stat.ErrorCount,
			float64(stat.TotalProcessingTime.Nanoseconds())/1000000.0)
	}
}

func main() {
	// TODO: 実装完了後、以下の動作確認コードが正常に動作するはず
	
	// キューの作成
	queue := NewInMemoryQueue()
	if queue == nil {
		log.Println("InMemoryQueue implementation is not ready")
		return
	}
	defer queue.Close()

	// Consumer群の作成
	consumerGroup := NewConsumerGroup(queue, 3, sampleProcessor)
	if consumerGroup == nil {
		log.Println("ConsumerGroup implementation is not ready")
		return
	}

	// Producerの作成
	producer := NewProducer(queue)
	if producer == nil {
		log.Println("Producer implementation is not ready")
		return
	}

	// Consumer群を開始
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	consumerGroup.Start(ctx)
	defer consumerGroup.Stop()

	// メッセージを生産
	go func() {
		for i := 0; i < 50; i++ {
			data := fmt.Sprintf("Message data %d", i)
			if err := producer.Produce(data); err != nil {
				log.Printf("Failed to produce message: %v", err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// 定期的に統計を表示
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			displayStats(consumerGroup)
			log.Printf("Queue size: %d", queue.Size())
		case <-ctx.Done():
			log.Println("Application shutting down...")
			displayStats(consumerGroup)
			return
		}
	}
}