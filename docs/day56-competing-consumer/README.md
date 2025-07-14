# Day 56: 競合コンシューマーパターン

## 🎯 本日の目標 (Today's Goal)

競合コンシューマーパターンを実装し、複数のコンシューマーが同一キューからメッセージを安全に取得・処理する仕組みを構築する。負荷分散、フェイルオーバー、スケーラビリティを実現する包括的なシステムを習得する。

## 📖 解説 (Explanation)

### 競合コンシューマーパターンとは

競合コンシューマーパターン（Competing Consumer Pattern）は、複数のコンシューマーインスタンスが同一のメッセージキューから競合的にメッセージを取得し処理するパターンです。このパターンにより、スループットの向上と可用性の確保を実現できます。

### 主要な利点

1. **スケーラビリティ**: コンシューマー数を動的に調整可能
2. **負荷分散**: 処理負荷を複数インスタンスに分散
3. **高可用性**: 一部のコンシューマーが停止しても処理継続
4. **フォルトトレラント**: 障害からの自動復旧

### 基本アーキテクチャ

```go
type CompetingConsumerSystem struct {
    messageQueue    MessageQueue
    consumers       map[string]*Consumer
    coordinator     *ConsumerCoordinator
    loadBalancer    LoadBalancer
    healthChecker   *HealthChecker
    metrics         *SystemMetrics
}

type Consumer struct {
    ID              string
    Status          ConsumerStatus
    ProcessingChan  chan *Message
    ErrorChan       chan error
    HeartbeatTicker *time.Ticker
    Processor       MessageProcessor
}

type ConsumerStatus string

const (
    StatusIdle       ConsumerStatus = "idle"
    StatusProcessing ConsumerStatus = "processing"
    StatusFailed     ConsumerStatus = "failed"
    StatusStopped    ConsumerStatus = "stopped"
)
```

### メッセージキューの実装

```go
type SafeMessageQueue struct {
    messages        []*Message
    inFlight        map[string]*InFlightMessage
    mu              sync.RWMutex
    notifyConsumers chan struct{}
    maxRetries      int
    visibilityTimeout time.Duration
}

type InFlightMessage struct {
    Message       *Message
    ConsumerID    string
    StartTime     time.Time
    RetryCount    int
    Timeout       time.Time
}

func (smq *SafeMessageQueue) Dequeue(consumerID string) (*Message, error) {
    smq.mu.Lock()
    defer smq.mu.Unlock()
    
    if len(smq.messages) == 0 {
        return nil, ErrNoMessages
    }
    
    // FIFOでメッセージを取得
    message := smq.messages[0]
    smq.messages = smq.messages[1:]
    
    // インフライトメッセージとして追加
    inFlight := &InFlightMessage{
        Message:    message,
        ConsumerID: consumerID,
        StartTime:  time.Now(),
        Timeout:    time.Now().Add(smq.visibilityTimeout),
    }
    smq.inFlight[message.ID] = inFlight
    
    return message, nil
}

func (smq *SafeMessageQueue) Acknowledge(messageID, consumerID string) error {
    smq.mu.Lock()
    defer smq.mu.Unlock()
    
    inFlight, exists := smq.inFlight[messageID]
    if !exists {
        return ErrMessageNotFound
    }
    
    if inFlight.ConsumerID != consumerID {
        return ErrUnauthorizedAck
    }
    
    delete(smq.inFlight, messageID)
    return nil
}

func (smq *SafeMessageQueue) Nack(messageID, consumerID string) error {
    smq.mu.Lock()
    defer smq.mu.Unlock()
    
    inFlight, exists := smq.inFlight[messageID]
    if !exists {
        return ErrMessageNotFound
    }
    
    if inFlight.ConsumerID != consumerID {
        return ErrUnauthorizedNack
    }
    
    // 再試行回数をチェック
    if inFlight.RetryCount >= smq.maxRetries {
        // DLQに送信
        return smq.sendToDeadLetterQueue(inFlight.Message)
    }
    
    // メッセージをキューに戻す
    inFlight.RetryCount++
    smq.messages = append(smq.messages, inFlight.Message)
    delete(smq.inFlight, messageID)
    
    // 他のコンシューマーに通知
    select {
    case smq.notifyConsumers <- struct{}{}:
    default:
    }
    
    return nil
}
```

### 負荷分散戦略

```go
type LoadBalancer interface {
    SelectConsumer(consumers []*Consumer) *Consumer
}

type RoundRobinLoadBalancer struct {
    current int
    mu      sync.Mutex
}

func (rr *RoundRobinLoadBalancer) SelectConsumer(consumers []*Consumer) *Consumer {
    rr.mu.Lock()
    defer rr.mu.Unlock()
    
    availableConsumers := make([]*Consumer, 0)
    for _, consumer := range consumers {
        if consumer.Status == StatusIdle {
            availableConsumers = append(availableConsumers, consumer)
        }
    }
    
    if len(availableConsumers) == 0 {
        return nil
    }
    
    selected := availableConsumers[rr.current%len(availableConsumers)]
    rr.current++
    return selected
}

type WeightedLoadBalancer struct {
    weights map[string]int
    mu      sync.RWMutex
}

func (wlb *WeightedLoadBalancer) SelectConsumer(consumers []*Consumer) *Consumer {
    wlb.mu.RLock()
    defer wlb.mu.RUnlock()
    
    totalWeight := 0
    for _, consumer := range consumers {
        if consumer.Status == StatusIdle {
            totalWeight += wlb.weights[consumer.ID]
        }
    }
    
    if totalWeight == 0 {
        return nil
    }
    
    rand.Seed(time.Now().UnixNano())
    target := rand.Intn(totalWeight)
    
    current := 0
    for _, consumer := range consumers {
        if consumer.Status == StatusIdle {
            current += wlb.weights[consumer.ID]
            if current > target {
                return consumer
            }
        }
    }
    
    return nil
}

type LeastConnectionsLoadBalancer struct {
    connections map[string]int
    mu          sync.RWMutex
}

func (lcb *LeastConnectionsLoadBalancer) SelectConsumer(consumers []*Consumer) *Consumer {
    lcb.mu.RLock()
    defer lcb.mu.RUnlock()
    
    var selected *Consumer
    minConnections := int(^uint(0) >> 1) // max int
    
    for _, consumer := range consumers {
        if consumer.Status == StatusIdle {
            connections := lcb.connections[consumer.ID]
            if connections < minConnections {
                minConnections = connections
                selected = consumer
            }
        }
    }
    
    return selected
}
```

### ヘルスチェックとフェイルオーバー

```go
type HealthChecker struct {
    consumers       map[string]*Consumer
    checkInterval   time.Duration
    healthTimeout   time.Duration
    failureThreshold int
    failures        map[string]int
    mu              sync.RWMutex
}

func (hc *HealthChecker) StartMonitoring(ctx context.Context) {
    ticker := time.NewTicker(hc.checkInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            hc.performHealthChecks()
        case <-ctx.Done():
            return
        }
    }
}

func (hc *HealthChecker) performHealthChecks() {
    hc.mu.Lock()
    defer hc.mu.Unlock()
    
    for consumerID, consumer := range hc.consumers {
        if err := hc.checkConsumerHealth(consumer); err != nil {
            hc.failures[consumerID]++
            
            if hc.failures[consumerID] >= hc.failureThreshold {
                consumer.Status = StatusFailed
                hc.triggerFailover(consumer)
            }
        } else {
            hc.failures[consumerID] = 0
            if consumer.Status == StatusFailed {
                consumer.Status = StatusIdle
            }
        }
    }
}

func (hc *HealthChecker) checkConsumerHealth(consumer *Consumer) error {
    // ハートビートチェック
    if time.Since(consumer.LastHeartbeat) > hc.healthTimeout {
        return errors.New("heartbeat timeout")
    }
    
    // レスポンシブネスチェック
    healthCheck := make(chan error, 1)
    go func() {
        healthCheck <- consumer.Ping()
    }()
    
    select {
    case err := <-healthCheck:
        return err
    case <-time.After(hc.healthTimeout):
        return errors.New("health check timeout")
    }
}

func (hc *HealthChecker) triggerFailover(failedConsumer *Consumer) {
    // フェイルオーバーロジック
    // 1. 処理中のメッセージを他のコンシューマーに再配布
    // 2. 新しいコンシューマーインスタンスの起動
    // 3. アラート送信
}
```

### 動的スケーリング

```go
type AutoScaler struct {
    minConsumers    int
    maxConsumers    int
    scaleUpThreshold   float64
    scaleDownThreshold float64
    cooldownPeriod     time.Duration
    lastScaleAction    time.Time
    metrics           *SystemMetrics
    consumerFactory   ConsumerFactory
    mu               sync.RWMutex
}

func (as *AutoScaler) MonitorAndScale(ctx context.Context, system *CompetingConsumerSystem) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            as.evaluateScaling(system)
        case <-ctx.Done():
            return
        }
    }
}

func (as *AutoScaler) evaluateScaling(system *CompetingConsumerSystem) {
    as.mu.Lock()
    defer as.mu.Unlock()
    
    // クールダウン期間中はスケーリングしない
    if time.Since(as.lastScaleAction) < as.cooldownPeriod {
        return
    }
    
    currentConsumers := len(system.consumers)
    queueDepth := system.messageQueue.GetDepth()
    avgProcessingTime := as.metrics.GetAverageProcessingTime()
    
    // スケールアップの判定
    if queueDepth > 0 && avgProcessingTime > 0 {
        estimatedThroughput := float64(currentConsumers) / avgProcessingTime.Seconds()
        queueGrowthRate := float64(queueDepth) / estimatedThroughput
        
        if queueGrowthRate > as.scaleUpThreshold && currentConsumers < as.maxConsumers {
            as.scaleUp(system)
            return
        }
    }
    
    // スケールダウンの判定
    if queueDepth == 0 && currentConsumers > as.minConsumers {
        idleConsumers := 0
        for _, consumer := range system.consumers {
            if consumer.Status == StatusIdle {
                idleConsumers++
            }
        }
        
        idleRatio := float64(idleConsumers) / float64(currentConsumers)
        if idleRatio > as.scaleDownThreshold {
            as.scaleDown(system)
        }
    }
}

func (as *AutoScaler) scaleUp(system *CompetingConsumerSystem) {
    newConsumer := as.consumerFactory.CreateConsumer()
    system.AddConsumer(newConsumer)
    as.lastScaleAction = time.Now()
    
    log.Printf("Scaled up: added consumer %s", newConsumer.ID)
}

func (as *AutoScaler) scaleDown(system *CompetingConsumerSystem) {
    // 最もアイドル時間の長いコンシューマーを停止
    var longestIdle *Consumer
    var maxIdleTime time.Duration
    
    for _, consumer := range system.consumers {
        if consumer.Status == StatusIdle {
            idleTime := time.Since(consumer.LastActivity)
            if idleTime > maxIdleTime {
                maxIdleTime = idleTime
                longestIdle = consumer
            }
        }
    }
    
    if longestIdle != nil {
        system.RemoveConsumer(longestIdle.ID)
        as.lastScaleAction = time.Now()
        
        log.Printf("Scaled down: removed consumer %s", longestIdle.ID)
    }
}
```

### メッセージ処理の実装

```go
type MessageProcessor interface {
    Process(ctx context.Context, message *Message) error
}

type AsyncMessageProcessor struct {
    workerPool  *WorkerPool
    timeout     time.Duration
    retryPolicy RetryPolicy
}

func (amp *AsyncMessageProcessor) Process(ctx context.Context, message *Message) error {
    processCtx, cancel := context.WithTimeout(ctx, amp.timeout)
    defer cancel()
    
    return amp.workerPool.Submit(processCtx, func(ctx context.Context) error {
        return amp.processWithRetry(ctx, message)
    })
}

func (amp *AsyncMessageProcessor) processWithRetry(ctx context.Context, message *Message) error {
    var lastErr error
    
    for attempt := 0; attempt < amp.retryPolicy.MaxAttempts; attempt++ {
        if attempt > 0 {
            delay := amp.retryPolicy.CalculateDelay(attempt)
            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return ctx.Err()
            }
        }
        
        if err := amp.processMessage(ctx, message); err != nil {
            lastErr = err
            if !amp.retryPolicy.ShouldRetry(err) {
                break
            }
            continue
        }
        
        return nil
    }
    
    return lastErr
}
```

## 📝 課題 (The Problem)

以下の機能を持つ包括的な競合コンシューマーシステムを実装してください：

### 1. 基本機能
- 安全なメッセージキュー
- 複数コンシューマーの管理
- メッセージの可視性制御

### 2. 負荷分散
- ラウンドロビン方式
- 重み付き分散
- 最小接続数方式

### 3. 障害処理
- ヘルスチェック
- 自動フェイルオーバー
- メッセージの再処理

### 4. スケーラビリティ
- 動的スケーリング
- パフォーマンス監視
- 負荷予測

### 5. 運用機能
- メトリクス収集
- ログ記録
- アラート機能

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestCompetingConsumer_BasicOperation
    main_test.go:45: Multiple consumers processing messages correctly
--- PASS: TestCompetingConsumer_BasicOperation (0.02s)

=== RUN   TestCompetingConsumer_LoadBalancing
    main_test.go:65: Load balancing distributing work evenly
--- PASS: TestCompetingConsumer_LoadBalancing (0.03s)

=== RUN   TestCompetingConsumer_Failover
    main_test.go:85: Failover handling working correctly
--- PASS: TestCompetingConsumer_Failover (0.04s)

PASS
ok      day56-competing-consumer   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 基本的なコンシューマー実装

```go
func (c *Consumer) Start(ctx context.Context, queue MessageQueue) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            default:
                message, err := queue.Dequeue(c.ID)
                if err != nil {
                    time.Sleep(100 * time.Millisecond)
                    continue
                }
                
                c.processMessage(ctx, message, queue)
            }
        }
    }()
}
```

### メッセージ可視性制御

```go
func (smq *SafeMessageQueue) startVisibilityTimeoutChecker() {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        
        for range ticker.C {
            smq.checkTimeouts()
        }
    }()
}

func (smq *SafeMessageQueue) checkTimeouts() {
    smq.mu.Lock()
    defer smq.mu.Unlock()
    
    now := time.Now()
    for messageID, inFlight := range smq.inFlight {
        if now.After(inFlight.Timeout) {
            // タイムアウトしたメッセージをキューに戻す
            smq.messages = append(smq.messages, inFlight.Message)
            delete(smq.inFlight, messageID)
        }
    }
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **優先度キュー**: メッセージ優先度に基づく処理
2. **バッチ処理**: 複数メッセージの一括処理
3. **分散コーディネーション**: 複数ノード間での調整
4. **機械学習予測**: 負荷パターンの学習と予測
5. **ゼロダウンタイム更新**: 無停止でのコンシューマー更新

競合コンシューマーパターンの実装を通じて、スケーラブルで堅牢な分散システムの構築手法を習得しましょう！