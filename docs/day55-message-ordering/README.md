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

```go
// 【メッセージ順序保証の重要性】分散システムでの一貫性維持
// ❌ 問題例：順序保証なしによる壊滅的データ不整合
func catastrophicUnorderedProcessing() {
    // 🚨 災害例：銀行取引システムでの順序無視処理
    
    accountID := "ACC-12345"
    initialBalance := 1000.0
    
    // 【問題のシナリオ】顧客の一連の取引が並行処理される
    transactions := []Transaction{
        {ID: "TXN-001", Type: "deposit",    Amount: 500.0,  Timestamp: time.Now().Add(-3*time.Minute)}, // +500
        {ID: "TXN-002", Type: "withdrawal", Amount: 800.0,  Timestamp: time.Now().Add(-2*time.Minute)}, // -800
        {ID: "TXN-003", Type: "deposit",    Amount: 300.0,  Timestamp: time.Now().Add(-1*time.Minute)}, // +300
        {ID: "TXN-004", Type: "withdrawal", Amount: 200.0,  Timestamp: time.Now()},                    // -200
    }
    
    // 【正しい処理順序】時系列順：1000 → 1500 → 700 → 1000 → 800
    // 最終残高：800（正常な取引すべてが実行可能）
    
    // 【致命的問題】並行処理で順序がランダムに
    var wg sync.WaitGroup
    for _, txn := range transactions {
        wg.Add(1)
        
        // 【災害発生】各取引が並行実行され、順序が保証されない
        go func(t Transaction) {
            defer wg.Done()
            
            // 【レースコンディション】同時実行で残高計算が破綻
            // 可能な実行順序パターン：
            // パターン1: TXN-004, TXN-002, TXN-001, TXN-003
            // パターン2: TXN-002, TXN-004, TXN-003, TXN-001
            // → 各パターンで全く異なる最終残高
            
            processTransactionUnsafe(accountID, t)
            
            // 【実際の被害例】：
            // - 残高不整合：顧客Aの1000円がマイナス2000円に
            // - 二重支払い：同じ商品代金を複数回決済
            // - 在庫過剰減算：在庫100個が-50個になる
            // - 監査不可能：取引履歴の時系列が破綻
            
        }(txn)
    }
    
    wg.Wait()
    
    // 【結果】：
    // - 顧客の正当な取引が拒否される（顧客満足度低下）
    // - システムの整合性が破綻（監査で発覚、法的責任）
    // - 修復作業で莫大なコスト（全取引の手動調査・修正）
    // - 信頼失墜（金融ライセンス剥奪の可能性）
    
    log.Printf("Final balance: %.2f (INCONSISTENT!)", getCurrentBalance(accountID))
    // 実行のたびに異なる値が出力される
}

// ✅ 正解：エンタープライズ級メッセージ順序保証システム
type EnterpriseOrderedMessageSystem struct {
    // 【基本順序保証】
    partitionManager    *PartitionManager       // パーティション管理
    sequencer          *MessageSequencer       // メッセージ順序付け
    orderValidator     *OrderValidator         // 順序検証
    conflictResolver   *ConflictResolver       // 競合解決
    
    // 【高度順序制御】
    logicalClock       *VectorClock           // ベクタークロック
    timestampOracle    *TimestampOracle       // タイムスタンプ生成
    causalityTracker   *CausalityTracker      // 因果関係追跡
    dependencyGraph    *DependencyGraph       // 依存関係グラフ
    
    // 【パーティション戦略】
    shardingStrategy   ShardingStrategy       // シャーディング戦略
    rebalancer         *PartitionRebalancer   // パーティション再配分
    consistentHashing  *ConsistentHashRing    // 一貫性ハッシュ
    affinityManager    *AffinityManager       // パーティション親和性
    
    // 【パフォーマンス最適化】
    batchProcessor     *BatchOrderProcessor   // バッチ順序処理
    pipelineManager    *PipelineManager       // パイプライン管理
    bufferManager      *OrderedBufferManager  // 順序付きバッファ
    backpressure       *BackpressureController // バックプレッシャー制御
    
    // 【障害処理・復旧】
    recoveryManager    *OrderRecoveryManager  // 順序復旧
    checkpointer       *OrderCheckpointer     // 順序チェックポイント
    replicationManager *OrderReplication      // 順序複製
    auditLogger        *OrderAuditLogger      // 順序監査ログ
}

// 【包括的順序保証処理】企業レベルの順序制御
func (oms *EnterpriseOrderedMessageSystem) ProcessOrderedMessage(ctx context.Context, message *OrderedMessage) error {
    startTime := time.Now()
    processingID := generateProcessingID()
    
    // 【STEP 1】メッセージ順序検証
    orderInfo := &OrderInfo{
        MessageID:     message.ID,
        PartitionKey:  message.PartitionKey,
        SequenceNum:   message.SequenceNumber,
        Timestamp:     message.Timestamp,
        ProcessingID:  processingID,
        Dependencies:  message.Dependencies,
    }
    
    if !oms.orderValidator.ValidateOrder(orderInfo) {
        return oms.handleOrderViolation(message, orderInfo)
    }
    
    // 【STEP 2】パーティション選択と親和性確保
    partition := oms.partitionManager.SelectPartition(message.PartitionKey)
    if partition.IsRebalancing() {
        // パーティション再配分中は一時待機
        if err := oms.waitForRebalanceCompletion(ctx, partition); err != nil {
            return fmt.Errorf("partition rebalancing timeout: %w", err)
        }
    }
    
    // 【STEP 3】因果関係と依存関係の確認
    if len(message.Dependencies) > 0 {
        if err := oms.causalityTracker.WaitForDependencies(ctx, message.Dependencies); err != nil {
            return fmt.Errorf("dependency wait failed: %w", err)
        }
    }
    
    // 【STEP 4】順序付きバッファへの格納
    bufferSlot := oms.bufferManager.AcquireSlot(partition.ID, message.SequenceNumber)
    defer bufferSlot.Release()
    
    // 【並行性制御】同一パーティション内での順序保証
    partitionLock := oms.getPartitionOrderLock(partition.ID)
    partitionLock.Lock()
    defer partitionLock.Unlock()
    
    // 【STEP 5】順序チェックと待機
    expectedSequence := oms.sequencer.GetExpectedSequence(partition.ID)
    if message.SequenceNumber != expectedSequence {
        // 【順序待機】期待される順序番号まで待機
        log.Printf("⏳ Message %s waiting for sequence %d (current: %d)", 
            message.ID, expectedSequence, message.SequenceNumber)
        
        if err := oms.waitForPrecedingMessages(ctx, partition.ID, expectedSequence, message.SequenceNumber); err != nil {
            return fmt.Errorf("sequence wait failed: %w", err)
        }
    }
    
    // 【STEP 6】ビジネスロジック実行
    processingResult, err := oms.executeBusinessLogic(ctx, message, partition)
    if err != nil {
        // 失敗時の順序状態復旧
        oms.recoveryManager.HandleProcessingFailure(partition.ID, message.SequenceNumber, err)
        return fmt.Errorf("business logic failed: %w", err)
    }
    
    // 【STEP 7】順序状態更新
    oms.sequencer.AdvanceSequence(partition.ID, message.SequenceNumber)
    
    // 【STEP 8】後続メッセージの通知
    oms.notifyWaitingMessages(partition.ID, message.SequenceNumber+1)
    
    // 【STEP 9】監査ログ記録
    auditEntry := &OrderAuditEntry{
        MessageID:         message.ID,
        PartitionID:       partition.ID,
        SequenceNumber:    message.SequenceNumber,
        ProcessingTime:    time.Since(startTime),
        ProcessingResult:  processingResult,
        PredecessorID:     oms.getPredecessorMessageID(partition.ID, message.SequenceNumber-1),
        SuccessorID:       "", // 後で設定
    }
    
    oms.auditLogger.LogOrderedProcessing(auditEntry)
    
    log.Printf("✅ Message %s processed in order (seq: %d, partition: %s)", 
        message.ID, message.SequenceNumber, partition.ID)
    
    return nil
}

// 【順序違反処理】順序エラー時の詳細対応
func (oms *EnterpriseOrderedMessageSystem) handleOrderViolation(message *OrderedMessage, orderInfo *OrderInfo) error {
    violation := &OrderViolation{
        MessageID:       message.ID,
        ExpectedSeq:     orderInfo.ExpectedSequence,
        ActualSeq:       message.SequenceNumber,
        PartitionID:     orderInfo.PartitionID,
        ViolationType:   oms.classifyViolation(orderInfo),
        Timestamp:       time.Now(),
        Severity:        oms.assessViolationSeverity(message, orderInfo),
    }
    
    // 【違反タイプ別処理】
    switch violation.ViolationType {
    case ViolationTypeSequenceGap:
        // 【シーケンス番号の欠落】前のメッセージが未到着
        return oms.handleSequenceGap(message, violation)
        
    case ViolationTypeDuplicateSequence:
        // 【重複シーケンス】同じ番号のメッセージが複数
        return oms.handleDuplicateSequence(message, violation)
        
    case ViolationTypeOutOfOrder:
        // 【順序逆転】後続メッセージが先に到着
        return oms.handleOutOfOrder(message, violation)
        
    case ViolationTypePartitionMismatch:
        // 【パーティション不整合】予期しないパーティション
        return oms.handlePartitionMismatch(message, violation)
        
    case ViolationTypeTimestampAnomaly:
        // 【タイムスタンプ異常】時計の狂いやネットワーク遅延
        return oms.handleTimestampAnomaly(message, violation)
        
    default:
        // 【未知の違反】新しいタイプの順序違反
        return oms.handleUnknownViolation(message, violation)
    }
}

// 【シーケンス欠落処理】メッセージ欠落時の対応
func (oms *EnterpriseOrderedMessageSystem) handleSequenceGap(message *OrderedMessage, violation *OrderViolation) error {
    log.Printf("🚨 Sequence gap detected: expected %d, got %d for partition %s", 
        violation.ExpectedSeq, violation.ActualSeq, violation.PartitionID)
    
    // 【欠落検出】どのメッセージが欠落しているかを特定
    missingSequences := make([]int64, 0)
    for seq := violation.ExpectedSeq; seq < violation.ActualSeq; seq++ {
        missingSequences = append(missingSequences, seq)
    }
    
    // 【重要度評価】
    impact := oms.assessGapImpact(violation.PartitionID, missingSequences)
    
    if impact.Severity >= ImpactSeverityCritical {
        // 【緊急対応】クリティカルなメッセージ欠落
        alert := &CriticalOrderAlert{
            Type:           AlertTypeSequenceGap,
            PartitionID:    violation.PartitionID,
            MissingSequences: missingSequences,
            BusinessImpact: impact,
            RequiredActions: []string{
                "Immediate investigation of message loss",
                "Check producer system health",
                "Verify network infrastructure",
                "Consider system rollback if data corruption suspected",
            },
        }
        
        oms.sendCriticalAlert(alert)
        
        // 【データ整合性保護】クリティカル時は処理停止
        if oms.config.StrictOrderingMode {
            return fmt.Errorf("critical sequence gap detected, halting processing to prevent data corruption")
        }
    }
    
    // 【欠落回復戦略】
    recoveryStrategy := oms.selectRecoveryStrategy(violation.PartitionID, missingSequences, impact)
    
    switch recoveryStrategy {
    case RecoveryStrategyWaitAndRetry:
        // 【待機・再試行】短時間待機してメッセージ到着を期待
        return oms.waitForMissingMessages(violation.PartitionID, missingSequences, 30*time.Second)
        
    case RecoveryStrategySkipAndContinue:
        // 【スキップ・継続】非クリティカルメッセージは欠落を許容
        oms.logSkippedMessages(violation.PartitionID, missingSequences)
        return oms.advanceSequenceWithGap(violation.PartitionID, violation.ActualSeq)
        
    case RecoveryStrategyRequestRedelivery:
        // 【再配信要求】プロデューサーに欠落メッセージの再送要求
        return oms.requestMessageRedelivery(violation.PartitionID, missingSequences)
        
    case RecoveryStrategyFailsafeMode:
        // 【セーフモード】システム保護のため処理を一時停止
        return oms.enterFailsafeMode(violation.PartitionID, "sequence gap detected")
        
    default:
        return fmt.Errorf("unknown recovery strategy: %v", recoveryStrategy)
    }
}
```

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