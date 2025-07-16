// Day 55: メッセージ順序保証
// メッセージの処理順序が重要なケースとその対策を実装

package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Message 順序付きメッセージ
type Message struct {
	ID           string      `json:"id"`
	PartitionKey string      `json:"partition_key"`
	SequenceNum  int64       `json:"sequence_num"`
	Timestamp    time.Time   `json:"timestamp"`
	Type         string      `json:"type"`
	Data         interface{} `json:"data"`
	Dependencies []string    `json:"dependencies,omitempty"`
}

// OrderedMessageProcessor 順序保証メッセージプロセッサー
type OrderedMessageProcessor interface {
	Process(ctx context.Context, msg *Message) error
	GetProcessorType() string
}

// SequentialProcessor 順次処理プロセッサー
type SequentialProcessor struct {
	name           string
	partitionState map[string]*PartitionState
	mu             sync.RWMutex
}

type PartitionState struct {
	lastProcessedSeq int64
	pendingMessages  []*Message
	processing       bool
	mu               sync.RWMutex
}

func NewSequentialProcessor(name string) *SequentialProcessor {
	return &SequentialProcessor{
		name:           name,
		partitionState: make(map[string]*PartitionState),
	}
}

func (p *SequentialProcessor) Process(ctx context.Context, msg *Message) error {
	partitionKey := msg.PartitionKey
	if partitionKey == "" {
		partitionKey = "default"
	}
	
	// パーティション状態を取得または作成
	state := p.getPartitionState(partitionKey)
	
	state.mu.Lock()
	defer state.mu.Unlock()
	
	// 既に処理中の場合は待機
	if state.processing {
		state.pendingMessages = append(state.pendingMessages, msg)
		p.sortPendingMessages(state)
		return nil
	}
	
	// 順序チェック
	if msg.SequenceNum == state.lastProcessedSeq+1 {
		// 順序が正しい場合は即座に処理
		return p.processMessage(ctx, msg, state)
	} else if msg.SequenceNum > state.lastProcessedSeq+1 {
		// 未来のメッセージの場合は保留
		state.pendingMessages = append(state.pendingMessages, msg)
		p.sortPendingMessages(state)
		log.Printf("Message %s queued for future processing (seq: %d, expected: %d)", 
			msg.ID, msg.SequenceNum, state.lastProcessedSeq+1)
		return nil
	} else {
		// 過去のメッセージの場合は重複として処理
		log.Printf("Duplicate or out-of-order message %s (seq: %d, last processed: %d)", 
			msg.ID, msg.SequenceNum, state.lastProcessedSeq)
		return nil
	}
}

func (p *SequentialProcessor) processMessage(ctx context.Context, msg *Message, state *PartitionState) error {
	state.processing = true
	defer func() { state.processing = false }()
	
	// 実際のメッセージ処理
	err := p.doProcess(ctx, msg)
	if err != nil {
		return err
	}
	
	state.lastProcessedSeq = msg.SequenceNum
	log.Printf("Processed message %s (seq: %d)", msg.ID, msg.SequenceNum)
	
	// 保留中のメッセージを処理
	p.processNextPendingMessages(ctx, state)
	
	return nil
}

func (p *SequentialProcessor) processNextPendingMessages(ctx context.Context, state *PartitionState) {
	for len(state.pendingMessages) > 0 {
		nextMsg := state.pendingMessages[0]
		
		if nextMsg.SequenceNum == state.lastProcessedSeq+1 {
			// 次のメッセージを処理
			state.pendingMessages = state.pendingMessages[1:]
			
			err := p.doProcess(ctx, nextMsg)
			if err != nil {
				log.Printf("Failed to process pending message %s: %v", nextMsg.ID, err)
				return
			}
			
			state.lastProcessedSeq = nextMsg.SequenceNum
			log.Printf("Processed pending message %s (seq: %d)", nextMsg.ID, nextMsg.SequenceNum)
		} else {
			// まだ順序が来ていない
			break
		}
	}
}

func (p *SequentialProcessor) doProcess(ctx context.Context, msg *Message) error {
	// 実際の処理ロジック（例：注文処理）
	time.Sleep(10 * time.Millisecond) // 処理時間をシミュレート
	
	if orderData, ok := msg.Data.(map[string]interface{}); ok {
		orderID := orderData["order_id"]
		log.Printf("Processing order %v in sequence %d", orderID, msg.SequenceNum)
	}
	
	return nil
}

func (p *SequentialProcessor) getPartitionState(partitionKey string) *PartitionState {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if state, exists := p.partitionState[partitionKey]; exists {
		return state
	}
	
	state := &PartitionState{
		lastProcessedSeq: 0,
		pendingMessages:  make([]*Message, 0),
		processing:       false,
	}
	p.partitionState[partitionKey] = state
	return state
}

func (p *SequentialProcessor) sortPendingMessages(state *PartitionState) {
	sort.Slice(state.pendingMessages, func(i, j int) bool {
		return state.pendingMessages[i].SequenceNum < state.pendingMessages[j].SequenceNum
	})
}

func (p *SequentialProcessor) GetProcessorType() string {
	return p.name
}

func (p *SequentialProcessor) GetPartitionStats() map[string]PartitionStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	stats := make(map[string]PartitionStats)
	for key, state := range p.partitionState {
		state.mu.RLock()
		stats[key] = PartitionStats{
			LastProcessedSeq: state.lastProcessedSeq,
			PendingCount:     len(state.pendingMessages),
			Processing:       state.processing,
		}
		state.mu.RUnlock()
	}
	
	return stats
}

type PartitionStats struct {
	LastProcessedSeq int64 `json:"last_processed_seq"`
	PendingCount     int   `json:"pending_count"`
	Processing       bool  `json:"processing"`
}

// DependencyOrderProcessor 依存関係ベースの順序プロセッサー
type DependencyOrderProcessor struct {
	name               string
	processedMessages  map[string]bool
	dependencyGraph    map[string]*DependencyNode
	pendingByDependency map[string][]*Message
	mu                 sync.RWMutex
}

type DependencyNode struct {
	MessageID    string
	Dependencies []string
	Dependents   []string
	Processed    bool
}

func NewDependencyOrderProcessor(name string) *DependencyOrderProcessor {
	return &DependencyOrderProcessor{
		name:                name,
		processedMessages:   make(map[string]bool),
		dependencyGraph:     make(map[string]*DependencyNode),
		pendingByDependency: make(map[string][]*Message),
	}
}

func (p *DependencyOrderProcessor) Process(ctx context.Context, msg *Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// 依存関係チェック
	if !p.areDependenciesSatisfied(msg) {
		// 依存関係が満たされていない場合は保留
		return p.addToPendingList(msg)
	}
	
	// 依存関係が満たされている場合は処理
	return p.processMessageWithDependents(ctx, msg)
}

func (p *DependencyOrderProcessor) areDependenciesSatisfied(msg *Message) bool {
	for _, depID := range msg.Dependencies {
		if !p.processedMessages[depID] {
			return false
		}
	}
	return true
}

func (p *DependencyOrderProcessor) addToPendingList(msg *Message) error {
	for _, depID := range msg.Dependencies {
		if !p.processedMessages[depID] {
			p.pendingByDependency[depID] = append(p.pendingByDependency[depID], msg)
		}
	}
	
	log.Printf("Message %s added to pending list due to unmet dependencies: %v", 
		msg.ID, msg.Dependencies)
	return nil
}

func (p *DependencyOrderProcessor) processMessageWithDependents(ctx context.Context, msg *Message) error {
	// メッセージを処理
	err := p.doProcess(ctx, msg)
	if err != nil {
		return err
	}
	
	// 処理済みとしてマーク
	p.processedMessages[msg.ID] = true
	log.Printf("Processed message %s with dependencies %v", msg.ID, msg.Dependencies)
	
	// このメッセージに依存していた保留中のメッセージを処理
	p.processDependentMessages(ctx, msg.ID)
	
	return nil
}

func (p *DependencyOrderProcessor) processDependentMessages(ctx context.Context, processedMsgID string) {
	pendingMessages := p.pendingByDependency[processedMsgID]
	delete(p.pendingByDependency, processedMsgID)
	
	for _, msg := range pendingMessages {
		if p.areDependenciesSatisfied(msg) {
			go func(m *Message) {
				if err := p.processMessageWithDependents(ctx, m); err != nil {
					log.Printf("Failed to process dependent message %s: %v", m.ID, err)
				}
			}(msg)
		} else {
			// まだ他の依存関係が満たされていない
			p.addToPendingList(msg)
		}
	}
}

func (p *DependencyOrderProcessor) doProcess(ctx context.Context, msg *Message) error {
	// 実際の処理ロジック
	time.Sleep(5 * time.Millisecond)
	
	if data, ok := msg.Data.(map[string]interface{}); ok {
		entityID := data["entity_id"]
		log.Printf("Processing entity %v with dependencies resolved", entityID)
	}
	
	return nil
}

func (p *DependencyOrderProcessor) GetProcessorType() string {
	return p.name
}

func (p *DependencyOrderProcessor) GetPendingStats() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	stats := make(map[string]int)
	for depID, messages := range p.pendingByDependency {
		stats[depID] = len(messages)
	}
	
	return stats
}

// TimestampOrderProcessor タイムスタンプベースの順序プロセッサー
type TimestampOrderProcessor struct {
	name            string
	window          time.Duration
	messageBuffer   []*Message
	lastProcessed   time.Time
	mu              sync.RWMutex
	ticker          *time.Ticker
	stopCh          chan struct{}
}

func NewTimestampOrderProcessor(name string, window time.Duration) *TimestampOrderProcessor {
	p := &TimestampOrderProcessor{
		name:          name,
		window:        window,
		messageBuffer: make([]*Message, 0),
		lastProcessed: time.Now(),
		stopCh:        make(chan struct{}),
	}
	
	// 定期的な処理を開始
	p.startPeriodicProcessing()
	
	return p
}

func (p *TimestampOrderProcessor) startPeriodicProcessing() {
	p.ticker = time.NewTicker(p.window / 2)
	
	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.processBufferedMessages()
			case <-p.stopCh:
				return
			}
		}
	}()
}

func (p *TimestampOrderProcessor) Process(ctx context.Context, msg *Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// バッファに追加
	p.messageBuffer = append(p.messageBuffer, msg)
	
	// タイムスタンプでソート
	sort.Slice(p.messageBuffer, func(i, j int) bool {
		return p.messageBuffer[i].Timestamp.Before(p.messageBuffer[j].Timestamp)
	})
	
	log.Printf("Message %s buffered for timestamp ordering", msg.ID)
	return nil
}

func (p *TimestampOrderProcessor) processBufferedMessages() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if len(p.messageBuffer) == 0 {
		return
	}
	
	cutoff := time.Now().Add(-p.window)
	processableMessages := make([]*Message, 0)
	remainingMessages := make([]*Message, 0)
	
	for _, msg := range p.messageBuffer {
		if msg.Timestamp.Before(cutoff) || msg.Timestamp.Equal(cutoff) {
			processableMessages = append(processableMessages, msg)
		} else {
			remainingMessages = append(remainingMessages, msg)
		}
	}
	
	// 処理可能なメッセージを処理
	for _, msg := range processableMessages {
		err := p.doProcess(context.Background(), msg)
		if err != nil {
			log.Printf("Failed to process buffered message %s: %v", msg.ID, err)
		}
	}
	
	p.messageBuffer = remainingMessages
	
	if len(processableMessages) > 0 {
		log.Printf("Processed %d buffered messages", len(processableMessages))
	}
}

func (p *TimestampOrderProcessor) doProcess(ctx context.Context, msg *Message) error {
	// 実際の処理ロジック
	time.Sleep(2 * time.Millisecond)
	
	log.Printf("Processed message %s at timestamp %v", msg.ID, msg.Timestamp)
	return nil
}

func (p *TimestampOrderProcessor) GetProcessorType() string {
	return p.name
}

func (p *TimestampOrderProcessor) Stop() {
	if p.ticker != nil {
		p.ticker.Stop()
	}
	close(p.stopCh)
	
	// 残りのメッセージを処理
	p.processBufferedMessages()
}

func (p *TimestampOrderProcessor) GetBufferStats() BufferStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	return BufferStats{
		BufferedCount: len(p.messageBuffer),
		OldestMessage: p.getOldestMessageTime(),
		WindowSize:    p.window,
	}
}

func (p *TimestampOrderProcessor) getOldestMessageTime() time.Time {
	if len(p.messageBuffer) == 0 {
		return time.Time{}
	}
	return p.messageBuffer[0].Timestamp
}

type BufferStats struct {
	BufferedCount int           `json:"buffered_count"`
	OldestMessage time.Time     `json:"oldest_message"`
	WindowSize    time.Duration `json:"window_size"`
}

// PartitionedOrderManager パーティション分割による順序管理
type PartitionedOrderManager struct {
	processors map[string]*SequentialProcessor
	mu         sync.RWMutex
}

func NewPartitionedOrderManager() *PartitionedOrderManager {
	return &PartitionedOrderManager{
		processors: make(map[string]*SequentialProcessor),
	}
}

func (m *PartitionedOrderManager) ProcessMessage(ctx context.Context, msg *Message) error {
	partitionKey := msg.PartitionKey
	if partitionKey == "" {
		partitionKey = "default"
	}
	
	processor := m.getOrCreateProcessor(partitionKey)
	return processor.Process(ctx, msg)
}

func (m *PartitionedOrderManager) getOrCreateProcessor(partitionKey string) *SequentialProcessor {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if processor, exists := m.processors[partitionKey]; exists {
		return processor
	}
	
	processor := NewSequentialProcessor(fmt.Sprintf("partition_%s", partitionKey))
	m.processors[partitionKey] = processor
	return processor
}

func (m *PartitionedOrderManager) GetAllPartitionStats() map[string]map[string]PartitionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	allStats := make(map[string]map[string]PartitionStats)
	for partitionKey, processor := range m.processors {
		allStats[partitionKey] = processor.GetPartitionStats()
	}
	
	return allStats
}

// OrderViolationDetector 順序違反検出器
type OrderViolationDetector struct {
	expectedSeq map[string]int64
	violations  []OrderViolation
	mu          sync.RWMutex
}

type OrderViolation struct {
	PartitionKey string    `json:"partition_key"`
	MessageID    string    `json:"message_id"`
	Expected     int64     `json:"expected_seq"`
	Actual       int64     `json:"actual_seq"`
	Timestamp    time.Time `json:"timestamp"`
}

func NewOrderViolationDetector() *OrderViolationDetector {
	return &OrderViolationDetector{
		expectedSeq: make(map[string]int64),
		violations:  make([]OrderViolation, 0),
	}
}

func (d *OrderViolationDetector) CheckOrder(msg *Message) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	partitionKey := msg.PartitionKey
	if partitionKey == "" {
		partitionKey = "default"
	}
	
	expected := d.expectedSeq[partitionKey] + 1
	
	if msg.SequenceNum == expected {
		d.expectedSeq[partitionKey] = msg.SequenceNum
		return true
	}
	
	// 順序違反を記録
	violation := OrderViolation{
		PartitionKey: partitionKey,
		MessageID:    msg.ID,
		Expected:     expected,
		Actual:       msg.SequenceNum,
		Timestamp:    time.Now(),
	}
	
	d.violations = append(d.violations, violation)
	log.Printf("Order violation detected: %+v", violation)
	
	// 期待値を更新（ギャップを認識）
	if msg.SequenceNum > expected {
		d.expectedSeq[partitionKey] = msg.SequenceNum
	}
	
	return false
}

func (d *OrderViolationDetector) GetViolations() []OrderViolation {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	violations := make([]OrderViolation, len(d.violations))
	copy(violations, d.violations)
	return violations
}

func (d *OrderViolationDetector) GetViolationCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.violations)
}

// OrderedMessage テスト用の順序付きメッセージ
type OrderedMessage struct {
	ID           string `json:"id"`
	PartitionKey string `json:"partition_key"`
	Data         []byte `json:"data"`
	Sequence     int64  `json:"sequence"`
}

// HashPartitioner ハッシュベースパーティショナー
type HashPartitioner struct {
	partitions int
}

func NewHashPartitioner(partitions int) *HashPartitioner {
	return &HashPartitioner{
		partitions: partitions,
	}
}

func (h *HashPartitioner) GetPartition(key string) int {
	if h.partitions <= 0 {
		return 0
	}
	hash := 0
	for _, c := range key {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash % h.partitions
}

// PartitionedQueue パーティション分割キュー
type PartitionedQueue struct {
	partitions  int
	partitioner *HashPartitioner
	queues      []chan *OrderedMessage
	consumers   map[int]*OrderedConsumer
	mu          sync.RWMutex
}

func NewPartitionedQueue(partitions int, partitioner *HashPartitioner) *PartitionedQueue {
	queues := make([]chan *OrderedMessage, partitions)
	for i := range queues {
		queues[i] = make(chan *OrderedMessage, 100)
	}
	
	return &PartitionedQueue{
		partitions:  partitions,
		partitioner: partitioner,
		queues:      queues,
		consumers:   make(map[int]*OrderedConsumer),
	}
}

func (pq *PartitionedQueue) Send(message *OrderedMessage) error {
	partition := pq.partitioner.GetPartition(message.PartitionKey)
	select {
	case pq.queues[partition] <- message:
		return nil
	default:
		return fmt.Errorf("queue full for partition %d", partition)
	}
}

func (pq *PartitionedQueue) AddConsumer(partition int, consumer *OrderedConsumer) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.consumers[partition] = consumer
	consumer.queue = pq.queues[partition]
}

// OrderedConsumer 順序保証コンシューマー
type OrderedConsumer struct {
	name    string
	handler func(context.Context, *OrderedMessage) error
	queue   chan *OrderedMessage
}

func NewOrderedConsumer(name string, handler func(context.Context, *OrderedMessage) error) *OrderedConsumer {
	return &OrderedConsumer{
		name:    name,
		handler: handler,
	}
}

func (oc *OrderedConsumer) Start(ctx context.Context) {
	for {
		select {
		case msg := <-oc.queue:
			if msg != nil {
				oc.handler(ctx, msg)
			}
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	fmt.Println("Day 55: メッセージ順序保証")
	fmt.Println("Run 'go test -v' to see the message ordering system in action")
}