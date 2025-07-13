# Day 33: デッドロックの再現と対策

🎯 **本日の目標**

データベースにおけるデッドロックが発生する状況を理解し、デッドロックを検出・回避・解決する実践的な対策を実装できるようになる。

📖 **解説**

## デッドロックとは

デッドロック（Deadlock）は、2つ以上のトランザクションが互いに相手が保持するリソースの解放を無限に待ち続ける状態です。データベースシステムでは、適切な対策なしには避けられない重要な問題です。

### デッドロックが発生する条件

デッドロックが発生するには、以下の4つの条件が同時に満たされる必要があります：

1. **相互排除（Mutual Exclusion）**: リソースが同時に複数のプロセスで使用できない
2. **占有と待機（Hold and Wait）**: プロセスがリソースを保持しながら、他のリソースを待機
3. **非搾取（No Preemption）**: 他のプロセスがリソースを強制的に奪えない
4. **循環待機（Circular Wait）**: プロセス間でリソースの待機が循環している

### デッドロックの典型例

```sql
-- トランザクション1
BEGIN;
UPDATE accounts SET balance = balance - 100 WHERE id = 1; -- アカウント1をロック
-- 少し待機...
UPDATE accounts SET balance = balance + 100 WHERE id = 2; -- アカウント2のロック待ち
COMMIT;

-- トランザクション2（同時実行）
BEGIN;
UPDATE accounts SET balance = balance - 50 WHERE id = 2;  -- アカウント2をロック
-- 少し待機...
UPDATE accounts SET balance = balance + 50 WHERE id = 1;  -- アカウント1のロック待ち
COMMIT;
```

### Goでのデッドロック再現と検出

```go
package main

import (
    "database/sql"
    "fmt"
    "sync"
    "time"
    _ "github.com/lib/pq"
)

// DeadlockScenario simulates a classic deadlock situation
func DeadlockScenario(db *sql.DB) error {
    var wg sync.WaitGroup
    errors := make(chan error, 2)

    // トランザクション1: アカウント1→2の順でロック
    wg.Add(1)
    go func() {
        defer wg.Done()
        err := transferMoney(db, 1, 2, 100)
        errors <- err
    }()

    // トランザクション2: アカウント2→1の順でロック
    wg.Add(1)
    go func() {
        defer wg.Done()
        time.Sleep(50 * time.Millisecond) // わずかな遅延
        err := transferMoney(db, 2, 1, 50)
        errors <- err
    }()

    wg.Wait()
    close(errors)

    // エラーを確認
    for err := range errors {
        if err != nil && isDeadlockError(err) {
            return fmt.Errorf("deadlock detected: %w", err)
        }
    }
    return nil
}

func transferMoney(db *sql.DB, fromID, toID int, amount float64) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 送金元をロック
    _, err = tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)
    if err != nil {
        return err
    }

    // 意図的な遅延でデッドロックを誘発
    time.Sleep(100 * time.Millisecond)

    // 送金先をロック
    _, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

### デッドロック対策1: 順序付きロック

リソースアクセスの順序を統一することでデッドロックを防ぐ：

```go
func transferMoneyOrdered(db *sql.DB, fromID, toID int, amount float64) error {
    // 常に小さいIDから大きいIDの順でロック
    firstID, secondID := fromID, toID
    firstAmount, secondAmount := -amount, amount
    
    if fromID > toID {
        firstID, secondID = toID, fromID
        firstAmount, secondAmount = amount, -amount
    }

    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 順序付きでロック
    _, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", firstAmount, firstID)
    if err != nil {
        return err
    }

    _, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", secondAmount, secondID)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

### デッドロック対策2: タイムアウト

トランザクションにタイムアウトを設定：

```go
func transferMoneyWithTimeout(db *sql.DB, fromID, toID int, amount float64, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // コンテキスト付きでクエリ実行
    _, err = tx.ExecContext(ctx, "UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)
    if err != nil {
        return err
    }

    _, err = tx.ExecContext(ctx, "UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

### デッドロック対策3: リトライメカニズム

デッドロック検出時の自動リトライ：

```go
func executeWithDeadlockRetry(db *sql.DB, operation func(*sql.Tx) error, maxRetries int) error {
    for attempt := 0; attempt < maxRetries; attempt++ {
        tx, err := db.Begin()
        if err != nil {
            return err
        }

        err = operation(tx)
        if err != nil {
            tx.Rollback()
            
            if isDeadlockError(err) && attempt < maxRetries-1 {
                // 指数バックオフでリトライ
                delay := time.Duration(math.Pow(2, float64(attempt))) * 50 * time.Millisecond
                time.Sleep(delay)
                continue
            }
            return err
        }

        if err := tx.Commit(); err != nil {
            if isDeadlockError(err) && attempt < maxRetries-1 {
                delay := time.Duration(math.Pow(2, float64(attempt))) * 50 * time.Millisecond
                time.Sleep(delay)
                continue
            }
            return err
        }

        return nil
    }
    
    return fmt.Errorf("operation failed after %d retries", maxRetries)
}

func isDeadlockError(err error) bool {
    if err == nil {
        return false
    }
    
    errStr := strings.ToLower(err.Error())
    return strings.Contains(errStr, "deadlock") || 
           strings.Contains(errStr, "40P01") // PostgreSQL deadlock code
}
```

### デッドロック対策4: 分散ロック

外部の分散ロックシステムを使用：

```go
type DistributedLock interface {
    Lock(ctx context.Context, resource string, ttl time.Duration) (bool, error)
    Unlock(ctx context.Context, resource string) error
}

func transferMoneyWithDistributedLock(db *sql.DB, lock DistributedLock, fromID, toID int, amount float64) error {
    ctx := context.Background()
    
    // リソース名を順序付け
    lockKey := fmt.Sprintf("account_lock_%d_%d", min(fromID, toID), max(fromID, toID))
    
    acquired, err := lock.Lock(ctx, lockKey, 30*time.Second)
    if !acquired || err != nil {
        return fmt.Errorf("failed to acquire lock: %w", err)
    }
    defer lock.Unlock(ctx, lockKey)

    // 分散ロック内でトランザクション実行
    return transferMoney(db, fromID, toID, amount)
}
```

### デッドロック検出と監視

```go
type DeadlockMonitor struct {
    deadlockCount int64
    lastDeadlock  time.Time
    mutex         sync.RWMutex
}

func (dm *DeadlockMonitor) RecordDeadlock() {
    dm.mutex.Lock()
    defer dm.mutex.Unlock()
    
    dm.deadlockCount++
    dm.lastDeadlock = time.Now()
}

func (dm *DeadlockMonitor) GetStats() (count int64, lastTime time.Time) {
    dm.mutex.RLock()
    defer dm.mutex.RUnlock()
    
    return dm.deadlockCount, dm.lastDeadlock
}
```

📝 **課題**

以下の機能を持つデッドロック対策システムを実装してください：

1. **`DeadlockSimulator`**: デッドロックを意図的に発生させる
2. **`DeadlockPreventer`**: 順序付きロックによる予防機能
3. **`DeadlockDetector`**: デッドロック検出とリトライ機能
4. **`ResourceLockManager`**: リソースの順序付き管理
5. **`DeadlockMonitor`**: デッドロック統計の監視

具体的な実装要件：
- 確実にデッドロックを再現するシミュレーション
- 順序付きロックによるデッドロック予防
- デッドロック検出時の自動リトライ
- タイムアウト機能の実装
- デッドロック発生統計の記録

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestDeadlockSimulator_ReproduceDeadlock
--- PASS: TestDeadlockSimulator_ReproduceDeadlock (0.15s)
=== RUN   TestDeadlockPreventer_OrderedLocking
--- PASS: TestDeadlockPreventer_OrderedLocking (0.05s)
=== RUN   TestDeadlockDetector_RetryOnDeadlock
--- PASS: TestDeadlockDetector_RetryOnDeadlock (0.10s)
=== RUN   TestResourceLockManager_LockOrdering
--- PASS: TestResourceLockManager_LockOrdering (0.02s)
=== RUN   TestDeadlockMonitor_Statistics
--- PASS: TestDeadlockMonitor_Statistics (0.01s)
PASS
ok      day33-deadlock-prevention    0.332s
```

デッドロック検出ログの例：
```
2024/07/13 10:30:00 Deadlock detected in transaction, attempt 1/3
2024/07/13 10:30:00 Retrying after 50ms backoff...
2024/07/13 10:30:00 Transaction succeeded on retry attempt 2
2024/07/13 10:30:00 Deadlock statistics: 1 total, last occurred at 10:30:00
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **sync**パッケージ: Goレベルの並行制御（`sync.Mutex`, `sync.RWMutex`）
2. **context**パッケージ: タイムアウト制御
3. **time**パッケージ: リトライ遅延処理
4. **sort**パッケージ: リソースID順序付け
5. **strings**パッケージ: エラーメッセージ判定

デッドロック予防のベストプラクティス：
- **リソース順序付け**: 常に同じ順序でリソースにアクセス
- **タイムアウト設定**: 長時間のロック待機を防ぐ
- **トランザクションの短縮**: ロック時間を最小限に抑える
- **分離レベルの調整**: 必要以上に厳しい分離レベルを避ける

PostgreSQLでのデッドロック関連エラーコード：
- **40P01**: deadlock_detected
- **40001**: serialization_failure

## 実行方法

```bash
# PostgreSQLコンテナを起動
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=testdb postgres:15

# テスト実行
go test -v
go test -race  # レースコンディションの検出
```