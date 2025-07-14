# Day 55: メッセージ順序保証

## 🎯 本日の目標 (Today's Goal)

分散メッセージングシステムにおけるメッセージ順序保証を実装し、パーティション分割、順序付きコンシューマー、バックプレッシャー制御を含む包括的な順序保証メカニズムを構築する。

## 📖 解説 (Explanation)

### メッセージ順序保証の重要性

分散システムでは、メッセージの順序が重要なビジネスロジックに影響する場合があります。例：
- 金融取引の処理順序
- ユーザーアクションの時系列
- 在庫更新の順序
- ログエントリの順序

### 順序保証の実装パターン

#### 1. パーティション分割による順序保証

```go
type PartitionedQueue struct {
    partitions    map[int]*OrderedPartition
    partitioner   Partitioner
    mu           sync.RWMutex
}

type OrderedPartition struct {
    id           int
    messages     []*OrderedMessage
    consumers    []*OrderedConsumer
    mu          sync.RWMutex
    sequenceNo  int64
}

type OrderedMessage struct {
    ID          string                 `json:"id"`
    PartitionKey string                `json:"partition_key"`
    SequenceNo  int64                 `json:"sequence_no"`
    Data        []byte                `json:"data"`
    Timestamp   time.Time             `json:"timestamp"`
    Metadata    map[string]interface{} `json:"metadata"`
}

func (pq *PartitionedQueue) Send(message *OrderedMessage) error {
    partitionID := pq.partitioner.GetPartition(message.PartitionKey)
    
    pq.mu.RLock()
    partition, exists := pq.partitions[partitionID]
    pq.mu.RUnlock()
    
    if !exists {
        return fmt.Errorf("partition %d not found", partitionID)
    }
    
    return partition.AddMessage(message)
}
```

#### 2. 順序付きコンシューマー

```go
type OrderedConsumer struct {
    id                string
    partition        *OrderedPartition
    lastProcessedSeq int64
    processingQueue  chan *OrderedMessage
    handler          MessageHandler
    backpressure     *BackpressureController
}

func (oc *OrderedConsumer) Start(ctx context.Context) error {
    go oc.processMessages(ctx)
    go oc.consumeFromPartition(ctx)
    return nil
}

func (oc *OrderedConsumer) processMessages(ctx context.Context) {
    for {
        select {
        case message := <-oc.processingQueue:
            if message.SequenceNo != oc.lastProcessedSeq+1 {
                // 順序が正しくない場合は待機
                oc.waitForCorrectSequence(ctx, message)
                continue
            }
            
            if err := oc.handler(ctx, message); err != nil {
                oc.handleProcessingError(message, err)
                continue
            }
            
            oc.lastProcessedSeq = message.SequenceNo
            oc.backpressure.MessageProcessed()
            
        case <-ctx.Done():
            return
        }
    }
}
```

#### 3. バックプレッシャー制御

```go
type BackpressureController struct {
    maxQueueSize     int
    currentQueueSize int64
    processingRate   *RateCalculator
    mu              sync.RWMutex
    throttle        chan struct{}
}

func (bp *BackpressureController) ShouldThrottle() bool {
    bp.mu.RLock()
    defer bp.mu.RUnlock()
    
    if bp.currentQueueSize >= int64(bp.maxQueueSize) {
        return true
    }
    
    // 処理速度に基づく動的制御
    currentRate := bp.processingRate.GetCurrentRate()
    targetRate := bp.processingRate.GetTargetRate()
    
    return currentRate < targetRate*0.8
}

func (bp *BackpressureController) WaitIfNeeded(ctx context.Context) error {
    if !bp.ShouldThrottle() {
        return nil
    }
    
    select {
    case <-bp.throttle:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

#### 4. 順序保証アルゴリズム

```go
type OrderingCoordinator struct {
    partitions       map[int]*PartitionState
    globalSequence   int64
    orderingWindow   time.Duration
    pendingMessages  *PriorityQueue
    mu              sync.RWMutex
}

type PartitionState struct {
    LastSequence     int64
    ExpectedSequence int64
    BufferedMessages map[int64]*OrderedMessage
    MaxBufferSize    int
}

func (oc *OrderingCoordinator) ProcessMessage(message *OrderedMessage) error {
    partitionID := message.PartitionKey
    
    oc.mu.Lock()
    defer oc.mu.Unlock()
    
    state, exists := oc.partitions[partitionID]
    if !exists {
        state = &PartitionState{
            BufferedMessages: make(map[int64]*OrderedMessage),
            MaxBufferSize:   1000,
        }
        oc.partitions[partitionID] = state
    }
    
    if message.SequenceNo == state.ExpectedSequence {
        // 期待されるシーケンス番号
        return oc.processInOrder(state, message)
    } else if message.SequenceNo > state.ExpectedSequence {
        // 未来のメッセージ - バッファに保存
        return oc.bufferMessage(state, message)
    } else {
        // 重複または古いメッセージ
        return fmt.Errorf("duplicate or old message: seq=%d, expected=%d", 
            message.SequenceNo, state.ExpectedSequence)
    }
}

func (oc *OrderingCoordinator) processInOrder(state *PartitionState, message *OrderedMessage) error {
    // メッセージを処理
    if err := oc.handleMessage(message); err != nil {
        return err
    }
    
    state.ExpectedSequence = message.SequenceNo + 1
    state.LastSequence = message.SequenceNo
    
    // バッファされた次のメッセージをチェック
    for {
        nextMessage, exists := state.BufferedMessages[state.ExpectedSequence]
        if !exists {
            break
        }
        
        delete(state.BufferedMessages, state.ExpectedSequence)
        
        if err := oc.handleMessage(nextMessage); err != nil {
            return err
        }
        
        state.ExpectedSequence++
        state.LastSequence = nextMessage.SequenceNo
    }
    
    return nil
}
```

### 高度な順序保証機能

#### 1. タイムスタンプベース順序保証

```go
type TimestampOrderingQueue struct {
    messages        *TimestampPriorityQueue
    watermark       time.Time
    maxDelay        time.Duration
    orderingWindow  time.Duration
    deliveryQueue   chan *OrderedMessage
}

func (toq *TimestampOrderingQueue) AddMessage(message *OrderedMessage) error {
    toq.messages.Push(message)
    
    // ウォーターマークを更新
    now := time.Now()
    if toq.watermark.IsZero() || now.Sub(toq.watermark) > toq.orderingWindow {
        toq.watermark = now.Add(-toq.maxDelay)
        toq.deliverReadyMessages()
    }
    
    return nil
}

func (toq *TimestampOrderingQueue) deliverReadyMessages() {
    for !toq.messages.IsEmpty() {
        message := toq.messages.Peek()
        if message.Timestamp.After(toq.watermark) {
            break
        }
        
        toq.messages.Pop()
        toq.deliveryQueue <- message
    }
}
```

#### 2. 分散順序保証

```go
type DistributedOrderingCoordinator struct {
    nodeID          string
    vectorClock     *VectorClock
    lamportClock    int64
    nodeClocks      map[string]int64
    orderingBuffer  *DistributedOrderingBuffer
    consensus       ConsensusService
}

type VectorClock map[string]int64

func (doc *DistributedOrderingCoordinator) SendMessage(message *OrderedMessage) error {
    // ベクタークロックを更新
    doc.vectorClock[doc.nodeID]++
    message.VectorClock = doc.copyVectorClock()
    
    // ランポートクロックを更新
    atomic.AddInt64(&doc.lamportClock, 1)
    message.LamportTimestamp = atomic.LoadInt64(&doc.lamportClock)
    
    return doc.broadcastMessage(message)
}

func (doc *DistributedOrderingCoordinator) ReceiveMessage(message *OrderedMessage) error {
    // ベクタークロックを更新
    doc.updateVectorClock(message.VectorClock)
    
    // ランポートクロックを更新
    currentLamport := atomic.LoadInt64(&doc.lamportClock)
    newLamport := max(currentLamport, message.LamportTimestamp) + 1
    atomic.StoreInt64(&doc.lamportClock, newLamport)
    
    // 因果順序に基づいて配信
    return doc.orderingBuffer.AddMessage(message)
}
```

## 📝 課題 (The Problem)

以下の機能を持つ包括的なメッセージ順序保証システムを実装してください：

### 1. パーティション分割
- パーティションキーベースの分割
- 動的パーティション追加/削除
- 負荷分散

### 2. 順序保証アルゴリズム
- シーケンス番号ベース
- タイムスタンプベース
- ベクタークロックベース

### 3. バックプレッシャー制御
- 動的スループット調整
- メモリ使用量制御
- 遅延制御

### 4. 監視機能
- 順序違反検出
- 遅延メトリクス
- スループット監視

### 5. 復旧機能
- 順序エラー修復
- メッセージ再送
- 状態復元

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestOrderedQueue_BasicOrdering
    main_test.go:45: Message ordering preserved correctly
--- PASS: TestOrderedQueue_BasicOrdering (0.01s)

=== RUN   TestOrderedQueue_PartitionHandling
    main_test.go:65: Partition-based ordering working
--- PASS: TestOrderedQueue_PartitionHandling (0.02s)

=== RUN   TestOrderedQueue_BackpressureControl
    main_test.go:85: Backpressure control functioning
--- PASS: TestOrderedQueue_BackpressureControl (0.03s)

PASS
ok      day55-message-ordering   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### パーティション分割

```go
type HashPartitioner struct {
    partitionCount int
}

func (hp *HashPartitioner) GetPartition(key string) int {
    hash := fnv.New32a()
    hash.Write([]byte(key))
    return int(hash.Sum32()) % hp.partitionCount
}
```

### 順序保証バッファ

```go
type OrderingBuffer struct {
    buffer          map[int64]*OrderedMessage
    expectedSeq     int64
    maxBufferSize   int
    deliveryChannel chan *OrderedMessage
}

func (ob *OrderingBuffer) AddMessage(message *OrderedMessage) error {
    if message.SequenceNo == ob.expectedSeq {
        return ob.deliverInOrder(message)
    }
    
    if len(ob.buffer) >= ob.maxBufferSize {
        return errors.New("buffer overflow")
    }
    
    ob.buffer[message.SequenceNo] = message
    return nil
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **分散順序保証**: 複数ノード間での全順序保証
2. **機械学習予測**: メッセージ到着パターンの学習
3. **動的パーティション**: 負荷に応じた自動分割・結合
4. **CRDT統合**: Conflict-free Replicated Data Types
5. **ゼロダウンタイム移行**: 順序保証を維持した設定更新

メッセージ順序保証の実装を通じて、分散システムにおける一貫性制御の重要な概念を習得しましょう！