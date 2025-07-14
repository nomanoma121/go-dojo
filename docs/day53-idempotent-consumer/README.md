# Day 53: 冪等なメッセージコンシューマー

## 🎯 本日の目標 (Today's Goal)

同じメッセージを複数回受信しても結果が変わらない冪等なコンシューマーを設計・実装できるようになる。重複配信への対策、状態管理、エラーハンドリングを習得する。

## 📖 解説 (Explanation)

### 冪等性とは

冪等性（Idempotency）とは、同じ操作を何回実行しても結果が変わらない性質のことです。メッセージングシステムでは、ネットワーク障害や再試行により同じメッセージが複数回配信される可能性があります。

### メッセージの重複配信が発生する理由

#### 1. ネットワーク障害
```
Producer → [Network Error] → Message Queue
Producer → [Retry] → Message Queue (Duplicate)
```

#### 2. アクノリッジメント失敗
```
Message Queue → Consumer → [Process Success]
Message Queue ← [Ack Failed] ← Consumer
Message Queue → Consumer (Redelivery)
```

#### 3. システム復旧時の再処理
```
Consumer Crash → System Recovery → Reprocess Messages
```

### 冪等性の実装パターン

#### 1. メッセージIDベースの重複検出

```go
type IdempotentConsumer struct {
    processedMessages map[string]bool
    mu               sync.RWMutex
}

func (c *IdempotentConsumer) ProcessMessage(msg *Message) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if c.processedMessages[msg.ID] {
        log.Printf("Message %s already processed, skipping", msg.ID)
        return nil
    }
    
    err := c.doProcess(msg)
    if err == nil {
        c.processedMessages[msg.ID] = true
    }
    
    return err
}
```

#### 2. データベーススキーマ設計

```sql
CREATE TABLE processed_messages (
    message_id VARCHAR(255) PRIMARY KEY,
    processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    result_data JSONB
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    message_id VARCHAR(255) UNIQUE, -- 重複防止
    user_id INTEGER,
    amount DECIMAL(10,2),
    status VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### 3. 分散環境での冪等性

```go
type DistributedIdempotentConsumer struct {
    storage IdempotencyStorage
    lockManager LockManager
}

func (c *DistributedIdempotentConsumer) ProcessMessage(ctx context.Context, msg *Message) error {
    // 分散ロックを取得
    lock, err := c.lockManager.AcquireLock(ctx, "msg:"+msg.ID, 30*time.Second)
    if err != nil {
        return fmt.Errorf("failed to acquire lock: %w", err)
    }
    defer lock.Release()
    
    // 処理済みチェック
    if processed, err := c.storage.IsProcessed(ctx, msg.ID); err != nil {
        return err
    } else if processed {
        return nil
    }
    
    // メッセージ処理
    result, err := c.processMessage(ctx, msg)
    if err != nil {
        return err
    }
    
    // 処理済みマークを保存
    return c.storage.MarkProcessed(ctx, msg.ID, result)
}
```

### 高度な冪等性パターン

#### 1. バージョンベース冪等性

```go
type VersionedMessage struct {
    ID      string `json:"id"`
    Version int    `json:"version"`
    Data    interface{} `json:"data"`
}

func (c *VersionedConsumer) ProcessMessage(msg *VersionedMessage) error {
    currentVersion, err := c.storage.GetMessageVersion(msg.ID)
    if err != nil {
        return err
    }
    
    if msg.Version <= currentVersion {
        log.Printf("Message %s version %d already processed (current: %d)", 
            msg.ID, msg.Version, currentVersion)
        return nil
    }
    
    err = c.doProcess(msg)
    if err == nil {
        c.storage.SetMessageVersion(msg.ID, msg.Version)
    }
    
    return err
}
```

#### 2. タイムスタンプベース冪等性

```go
type TimestampBasedConsumer struct {
    storage TimestampStorage
}

func (c *TimestampBasedConsumer) ProcessMessage(msg *TimestampedMessage) error {
    lastProcessed, err := c.storage.GetLastProcessedTime(msg.ID)
    if err != nil {
        return err
    }
    
    if msg.Timestamp.Before(lastProcessed) || msg.Timestamp.Equal(lastProcessed) {
        log.Printf("Message %s with timestamp %v already processed", 
            msg.ID, msg.Timestamp)
        return nil
    }
    
    err = c.doProcess(msg)
    if err == nil {
        c.storage.SetLastProcessedTime(msg.ID, msg.Timestamp)
    }
    
    return err
}
```

### 状態管理とクリーンアップ

#### 1. TTLベースのクリーンアップ

```go
type TTLIdempotencyStorage struct {
    data map[string]ProcessedRecord
    mu   sync.RWMutex
}

type ProcessedRecord struct {
    ProcessedAt time.Time
    Result      interface{}
}

func (s *TTLIdempotencyStorage) Cleanup(ttl time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    cutoff := time.Now().Add(-ttl)
    for id, record := range s.data {
        if record.ProcessedAt.Before(cutoff) {
            delete(s.data, id)
        }
    }
}
```

#### 2. LRUキャッシュ実装

```go
type LRUIdempotencyCache struct {
    capacity int
    cache    map[string]*LRUNode
    head     *LRUNode
    tail     *LRUNode
    mu       sync.RWMutex
}

func (c *LRUIdempotencyCache) IsProcessed(messageID string) bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if node, exists := c.cache[messageID]; exists {
        c.moveToHead(node)
        return true
    }
    return false
}
```

### エラー処理と再試行

#### 1. 部分失敗の処理

```go
func (c *IdempotentConsumer) ProcessBatchMessage(msg *BatchMessage) error {
    for _, item := range msg.Items {
        itemID := fmt.Sprintf("%s:%s", msg.ID, item.ID)
        
        if c.isItemProcessed(itemID) {
            continue
        }
        
        if err := c.processItem(item); err != nil {
            return fmt.Errorf("failed to process item %s: %w", item.ID, err)
        }
        
        c.markItemProcessed(itemID)
    }
    
    return nil
}
```

#### 2. 補償トランザクション

```go
func (c *CompensatingConsumer) ProcessMessage(msg *Message) error {
    if c.isProcessed(msg.ID) {
        return nil
    }
    
    // 補償可能な操作を記録
    compensations := make([]func() error, 0)
    
    err := c.doProcessWithCompensation(msg, &compensations)
    if err != nil {
        // 失敗時は補償処理を実行
        for i := len(compensations) - 1; i >= 0; i-- {
            if compErr := compensations[i](); compErr != nil {
                log.Printf("Compensation failed: %v", compErr)
            }
        }
        return err
    }
    
    c.markProcessed(msg.ID)
    return nil
}
```

## 📝 課題 (The Problem)

以下の機能を持つ冪等なメッセージコンシューマーシステムを実装してください：

### 1. IdempotentConsumer の実装

```go
type IdempotentConsumer struct {
    storage IdempotencyStorage
    processor MessageProcessor
    metrics IdempotencyMetrics
}
```

### 2. 必要なコンポーネントの実装

- `IdempotencyStorage`: 処理済みメッセージの管理
- `MessageProcessor`: 実際のメッセージ処理ロジック  
- `IdempotencyMetrics`: 重複検出メトリクス
- `DistributedLock`: 分散環境での競合制御

### 3. 複数の冪等性戦略

- メッセージIDベース
- バージョンベース
- タイムスタンプベース
- ハッシュベース

### 4. 状態管理機能

- TTLベースクリーンアップ
- LRUキャッシュ
- 永続化ストレージ

### 5. エラーハンドリング

- 部分失敗の処理
- 再試行メカニズム
- 補償トランザクション

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestIdempotentConsumer_DuplicateMessages
    main_test.go:45: Processing message: msg-001
    main_test.go:48: Duplicate message msg-001 detected and skipped
--- PASS: TestIdempotentConsumer_DuplicateMessages (0.01s)

=== RUN   TestVersionBasedIdempotency
    main_test.go:75: Version 2 processed, skipping version 1
--- PASS: TestVersionBasedIdempotency (0.01s)

=== RUN   TestDistributedIdempotency
    main_test.go:105: Multiple consumers handled correctly with locks
--- PASS: TestDistributedIdempotency (0.05s)

PASS
ok      day53-idempotent-consumer   0.123s
```

## 💡 ヒント (Hints)

### IdempotencyStorage インターフェース

```go
type IdempotencyStorage interface {
    IsProcessed(ctx context.Context, messageID string) (bool, error)
    MarkProcessed(ctx context.Context, messageID string, result interface{}) error
    GetProcessedResult(ctx context.Context, messageID string) (interface{}, error)
    Cleanup(ctx context.Context, olderThan time.Time) error
}
```

### メッセージハッシュによる冪等性

```go
func generateMessageHash(msg *Message) string {
    h := sha256.New()
    h.Write([]byte(msg.ID))
    h.Write([]byte(msg.Data))
    return hex.EncodeToString(h.Sum(nil))
}
```

### 分散ロック実装

```go
type DistributedLock interface {
    AcquireLock(ctx context.Context, key string, ttl time.Duration) (Lock, error)
}

type Lock interface {
    Release() error
    Extend(duration time.Duration) error
}
```

## 🚀 発展課題 (Advanced Features)

基本実装完了後、以下の追加機能にもチャレンジしてください：

1. **ブルームフィルター**: メモリ効率的な重複検出
2. **分散キャッシュ**: Redis/Hazelcastでの冪等性状態共有
3. **イベントソーシング**: イベントストリームでの冪等性
4. **サーキットブレーカー**: 障害時の自動停止機能
5. **メトリクス監視**: 重複率やパフォーマンスの監視

冪等なコンシューマーの実装を通じて、信頼性の高いメッセージ処理システムの構築方法を習得しましょう！