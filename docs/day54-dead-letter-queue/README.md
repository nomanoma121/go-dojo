# Day 54: Dead-Letter Queue (DLQ)

## 🎯 本日の目標 (Today's Goal)

Dead-Letter Queue パターンを実装し、処理に失敗したメッセージの適切な管理と分析を行う。エラー分類、再処理機能、監視機能を含む包括的なDLQシステムを構築する。

## 📖 解説 (Explanation)

### Dead-Letter Queue とは

Dead-Letter Queue（DLQ）は、正常に処理できなかったメッセージを一時的に保存するためのキューです。システムの信頼性向上と障害分析に重要な役割を果たします。

### DLQの主要機能

#### 1. エラー分類と処理

```go
type ErrorClassification string

const (
    TemporaryError   ErrorClassification = "temporary"
    PermanentError   ErrorClassification = "permanent"
    ValidationError  ErrorClassification = "validation"
    TimeoutError     ErrorClassification = "timeout"
    SecurityError    ErrorClassification = "security"
)

type DLQMessage struct {
    OriginalMessage *Message              `json:"original_message"`
    FailureReason   string                `json:"failure_reason"`
    ErrorClass      ErrorClassification   `json:"error_class"`
    FailureCount    int                   `json:"failure_count"`
    FirstFailure    time.Time             `json:"first_failure"`
    LastFailure     time.Time             `json:"last_failure"`
    Metadata        map[string]interface{} `json:"metadata"`
}

func ClassifyError(err error) ErrorClassification {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        return TimeoutError
    case strings.Contains(err.Error(), "validation"):
        return ValidationError
    case strings.Contains(err.Error(), "unauthorized"):
        return SecurityError
    case strings.Contains(err.Error(), "connection"):
        return TemporaryError
    default:
        return PermanentError
    }
}
```

#### 2. 自動再処理機能

```go
type ReprocessingStrategy interface {
    ShouldReprocess(dlqMsg *DLQMessage) bool
    NextAttemptTime(dlqMsg *DLQMessage) time.Time
    MaxAttempts() int
}

type ExponentialBackoffReprocessing struct {
    BaseDelay    time.Duration
    MaxDelay     time.Duration
    MaxAttempts  int
    Multiplier   float64
}

func (ebr *ExponentialBackoffReprocessing) ShouldReprocess(dlqMsg *DLQMessage) bool {
    if dlqMsg.ErrorClass == PermanentError || dlqMsg.ErrorClass == SecurityError {
        return false
    }
    
    return dlqMsg.FailureCount < ebr.MaxAttempts
}

func (ebr *ExponentialBackoffReprocessing) NextAttemptTime(dlqMsg *DLQMessage) time.Time {
    delay := time.Duration(float64(ebr.BaseDelay) * math.Pow(ebr.Multiplier, float64(dlqMsg.FailureCount)))
    if delay > ebr.MaxDelay {
        delay = ebr.MaxDelay
    }
    
    return dlqMsg.LastFailure.Add(delay)
}
```

#### 3. メッセージ分析と監視

```go
type DLQAnalytics struct {
    TotalMessages     int64                            `json:"total_messages"`
    ErrorBreakdown    map[ErrorClassification]int64    `json:"error_breakdown"`
    TopicBreakdown    map[string]int64                 `json:"topic_breakdown"`
    HourlyStats       map[string]int64                 `json:"hourly_stats"`
    AverageRetries    float64                          `json:"average_retries"`
    OldestMessage     *time.Time                       `json:"oldest_message"`
}

func (dlq *DeadLetterQueue) GetAnalytics() *DLQAnalytics {
    analytics := &DLQAnalytics{
        ErrorBreakdown: make(map[ErrorClassification]int64),
        TopicBreakdown: make(map[string]int64),
        HourlyStats:    make(map[string]int64),
    }
    
    dlq.mu.RLock()
    defer dlq.mu.RUnlock()
    
    totalRetries := int64(0)
    var oldestTime *time.Time
    
    for _, dlqMsg := range dlq.messages {
        analytics.TotalMessages++
        analytics.ErrorBreakdown[dlqMsg.ErrorClass]++
        analytics.TopicBreakdown[dlqMsg.OriginalMessage.Topic]++
        
        hour := dlqMsg.FirstFailure.Format("2006-01-02-15")
        analytics.HourlyStats[hour]++
        
        totalRetries += int64(dlqMsg.FailureCount)
        
        if oldestTime == nil || dlqMsg.FirstFailure.Before(*oldestTime) {
            oldestTime = &dlqMsg.FirstFailure
        }
    }
    
    if analytics.TotalMessages > 0 {
        analytics.AverageRetries = float64(totalRetries) / float64(analytics.TotalMessages)
    }
    analytics.OldestMessage = oldestTime
    
    return analytics
}
```

#### 4. バッチ再処理

```go
type BatchReprocessor struct {
    dlq           *DeadLetterQueue
    publisher     Publisher
    batchSize     int
    strategy      ReprocessingStrategy
    semaphore     chan struct{}
}

func (br *BatchReprocessor) ReprocessBatch(ctx context.Context, filter func(*DLQMessage) bool) error {
    messages := br.dlq.GetMessagesForReprocessing(filter)
    
    for i := 0; i < len(messages); i += br.batchSize {
        end := i + br.batchSize
        if end > len(messages) {
            end = len(messages)
        }
        
        batch := messages[i:end]
        if err := br.processBatch(ctx, batch); err != nil {
            return fmt.Errorf("batch processing failed: %w", err)
        }
    }
    
    return nil
}

func (br *BatchReprocessor) processBatch(ctx context.Context, batch []*DLQMessage) error {
    var wg sync.WaitGroup
    errChan := make(chan error, len(batch))
    
    for _, dlqMsg := range batch {
        wg.Add(1)
        go func(msg *DLQMessage) {
            defer wg.Done()
            
            select {
            case br.semaphore <- struct{}{}:
                defer func() { <-br.semaphore }()
                
                if err := br.publisher.Publish(ctx, msg.OriginalMessage.Topic, msg.OriginalMessage); err != nil {
                    errChan <- fmt.Errorf("failed to republish message %s: %w", msg.OriginalMessage.ID, err)
                } else {
                    br.dlq.RemoveMessage(msg.OriginalMessage.ID)
                }
            case <-ctx.Done():
                errChan <- ctx.Err()
            }
        }(dlqMsg)
    }
    
    wg.Wait()
    close(errChan)
    
    var errors []error
    for err := range errChan {
        if err != nil {
            errors = append(errors, err)
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("batch had %d errors: %v", len(errors), errors[0])
    }
    
    return nil
}
```

### 実用的な統合例

```go
type EnhancedMessageProcessor struct {
    primaryQueue    Queue
    dlq            *DeadLetterQueue
    reprocessor    *BatchReprocessor
    analytics      *DLQAnalytics
    alerting       AlertingService
}

func (emp *EnhancedMessageProcessor) ProcessMessage(ctx context.Context, message *Message) error {
    maxRetries := 3
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        if err := emp.processMessageOnce(ctx, message); err != nil {
            errorClass := ClassifyError(err)
            
            // 永続的エラーまたはセキュリティエラーの場合は即座にDLQへ
            if errorClass == PermanentError || errorClass == SecurityError {
                return emp.sendToDLQ(message, err, errorClass, attempt+1)
            }
            
            // 最後の試行の場合はDLQへ
            if attempt == maxRetries-1 {
                return emp.sendToDLQ(message, err, errorClass, attempt+1)
            }
            
            // 指数バックオフで再試行
            delay := time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
            select {
            case <-time.After(delay):
                continue
            case <-ctx.Done():
                return emp.sendToDLQ(message, ctx.Err(), TimeoutError, attempt+1)
            }
        } else {
            return nil // 成功
        }
    }
    
    return nil
}

func (emp *EnhancedMessageProcessor) sendToDLQ(message *Message, err error, errorClass ErrorClassification, failureCount int) error {
    dlqMessage := &DLQMessage{
        OriginalMessage: message,
        FailureReason:   err.Error(),
        ErrorClass:      errorClass,
        FailureCount:    failureCount,
        FirstFailure:    time.Now(),
        LastFailure:     time.Now(),
        Metadata: map[string]interface{}{
            "processor_version": "1.0.0",
            "environment":      "production",
        },
    }
    
    if err := emp.dlq.Send(context.Background(), dlqMessage); err != nil {
        // DLQ送信失敗は重大な問題
        emp.alerting.SendCriticalAlert("DLQ_SEND_FAILED", fmt.Sprintf("Failed to send message %s to DLQ: %v", message.ID, err))
        return fmt.Errorf("failed to send to DLQ: %w", err)
    }
    
    // アラート送信
    if errorClass == SecurityError {
        emp.alerting.SendSecurityAlert("SECURITY_ERROR_DLQ", fmt.Sprintf("Security error for message %s: %s", message.ID, err.Error()))
    }
    
    return nil
}

// 定期的なDLQ監視とアラート
func (emp *EnhancedMessageProcessor) StartDLQMonitoring(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            analytics := emp.dlq.GetAnalytics()
            
            // 閾値チェック
            if analytics.TotalMessages > 1000 {
                emp.alerting.SendWarningAlert("DLQ_HIGH_VOLUME", fmt.Sprintf("DLQ has %d messages", analytics.TotalMessages))
            }
            
            if analytics.OldestMessage != nil && time.Since(*analytics.OldestMessage) > 24*time.Hour {
                emp.alerting.SendWarningAlert("DLQ_OLD_MESSAGES", fmt.Sprintf("Oldest DLQ message is %v old", time.Since(*analytics.OldestMessage)))
            }
            
            // セキュリティエラーの増加チェック
            if securityErrors, exists := analytics.ErrorBreakdown[SecurityError]; exists && securityErrors > 10 {
                emp.alerting.SendSecurityAlert("DLQ_SECURITY_SPIKE", fmt.Sprintf("High number of security errors in DLQ: %d", securityErrors))
            }
            
        case <-ctx.Done():
            return
        }
    }
}
```

## 📝 課題 (The Problem)

以下の機能を持つ包括的なDead-Letter Queueシステムを実装してください：

### 1. 基本DLQ機能
- メッセージの保存と取得
- エラー分類と管理
- メタデータ追跡

### 2. 再処理機能
- 自動再処理戦略
- バッチ再処理
- 条件付きフィルタリング

### 3. 分析機能
- エラー統計
- トレンド分析
- パフォーマンス監視

### 4. アラート機能
- 閾値ベースアラート
- セキュリティアラート
- 運用アラート

### 5. 管理機能
- メッセージの手動操作
- 設定の動的更新
- データエクスポート

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestDeadLetterQueue_BasicOperations
    main_test.go:45: DLQ basic operations working correctly
--- PASS: TestDeadLetterQueue_BasicOperations (0.01s)

=== RUN   TestDeadLetterQueue_ErrorClassification
    main_test.go:65: Error classification working correctly
--- PASS: TestDeadLetterQueue_ErrorClassification (0.01s)

=== RUN   TestDeadLetterQueue_Reprocessing
    main_test.go:85: Message reprocessing working correctly
--- PASS: TestDeadLetterQueue_Reprocessing (0.03s)

=== RUN   TestDeadLetterQueue_Analytics
    main_test.go:105: DLQ analytics working correctly
--- PASS: TestDeadLetterQueue_Analytics (0.02s)

PASS
ok      day54-dead-letter-queue   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### DLQ基本構造

```go
type DeadLetterQueue struct {
    messages    map[string]*DLQMessage
    mu          sync.RWMutex
    strategy    ReprocessingStrategy
    analytics   *DLQAnalytics
    storage     Storage
}

func NewDeadLetterQueue(strategy ReprocessingStrategy, storage Storage) *DeadLetterQueue {
    return &DeadLetterQueue{
        messages:  make(map[string]*DLQMessage),
        strategy:  strategy,
        analytics: NewDLQAnalytics(),
        storage:   storage,
    }
}
```

### エラー分類

```go
func ClassifyError(err error) ErrorClassification {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        return TimeoutError
    case isValidationError(err):
        return ValidationError
    case isSecurityError(err):
        return SecurityError
    case isTemporaryError(err):
        return TemporaryError
    default:
        return PermanentError
    }
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **分散DLQ**: 複数ノード間でのDLQ共有
2. **機械学習分析**: エラーパターンの自動検出
3. **GraphQL API**: DLQデータのクエリインターフェース
4. **ストリーミング分析**: リアルタイムエラー分析
5. **自動修復**: エラーパターンに基づく自動修正

Dead-Letter Queueの実装を通じて、堅牢なメッセージング系システムの構築手法を習得しましょう！