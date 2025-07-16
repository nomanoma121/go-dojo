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
-- 【災害シナリオ】デッドロック発生の瞬間
-- 
-- 時刻 t=0: 両トランザクションが同時開始
-- ┌─────────────────────────────────┬─────────────────────────────────┐
-- │    トランザクション 1              │    トランザクション 2              │
-- ├─────────────────────────────────┼─────────────────────────────────┤
-- │ BEGIN;                          │ BEGIN;                          │
-- │                                 │                                 │
-- │ -- 【STEP 1】account_id=1 ロック │ -- 【STEP 1】account_id=2 ロック │
-- │ UPDATE accounts                 │ UPDATE accounts                 │
-- │ SET balance = balance - 100     │ SET balance = balance - 50      │
-- │ WHERE id = 1;                   │ WHERE id = 2;                   │
-- │ ✅ 成功: account_1 排他ロック取得│ ✅ 成功: account_2 排他ロック取得│
-- │                                 │                                 │
-- │ -- 【STEP 2】account_id=2 待機   │ -- 【STEP 2】account_id=1 待機   │
-- │ UPDATE accounts                 │ UPDATE accounts                 │
-- │ SET balance = balance + 100     │ SET balance = balance + 50      │
-- │ WHERE id = 2;                   │ WHERE id = 1;                   │
-- │ ⏳ 待機: account_2 ロック要求    │ ⏳ 待機: account_1 ロック要求    │
-- │    (TRX2が保持中)               │    (TRX1が保持中)               │
-- │                                 │                                 │
-- │ -- 【DEADLOCK】無限待機開始     │ -- 【DEADLOCK】無限待機開始     │
-- │ ❌ TRX2がaccount_2を解放待ち    │ ❌ TRX1がaccount_1を解放待ち    │
-- │ ❌ しかしTRX2はaccount_1待ち    │ ❌ しかしTRX1はaccount_2待ち    │
-- └─────────────────────────────────┴─────────────────────────────────┘
-- 
-- 【結果】: 循環待機により両トランザクションが永続的にブロック
-- 
-- 【システムへの影響】：
-- 1. アプリケーションレスポンス停止
-- 2. 接続プールの枯渇
-- 3. 他のトランザクションへの連鎖ブロック
-- 4. データベースリソースの無駄な消費
-- 5. ユーザーエクスペリエンスの悪化

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

// 【デッドロック再現】確実にデッドロックを発生させるシミュレーター
func DeadlockScenario(db *sql.DB) error {
    var wg sync.WaitGroup
    errors := make(chan error, 2)

    // 【トランザクション1】アカウント1→2の順でロック取得
    wg.Add(1)
    go func() {
        defer wg.Done()
        
        // 【戦略】先にaccount_id=1をロック、後でaccount_id=2をロック
        log.Printf("Transaction 1: Starting transfer 1->2")
        err := transferMoney(db, 1, 2, 100)
        if err != nil {
            log.Printf("Transaction 1: Failed with error: %v", err)
        } else {
            log.Printf("Transaction 1: Completed successfully")
        }
        errors <- err
    }()

    // 【トランザクション2】アカウント2→1の順でロック取得（逆順）
    wg.Add(1)
    go func() {
        defer wg.Done()
        
        // 【重要】わずかな遅延でタイミングを調整
        // この遅延により、TRX1が先にaccount_1をロックする確率を高める
        time.Sleep(50 * time.Millisecond)
        
        // 【戦略】先にaccount_id=2をロック、後でaccount_id=1をロック
        log.Printf("Transaction 2: Starting transfer 2->1")
        err := transferMoney(db, 2, 1, 50)
        if err != nil {
            log.Printf("Transaction 2: Failed with error: %v", err)
        } else {
            log.Printf("Transaction 2: Completed successfully")
        }
        errors <- err
    }()

    wg.Wait()
    close(errors)

    // 【デッドロック検出】エラー解析
    deadlockDetected := false
    for err := range errors {
        if err != nil && isDeadlockError(err) {
            deadlockDetected = true
            log.Printf("🚨 DEADLOCK DETECTED: %v", err)
        }
    }
    
    if deadlockDetected {
        return fmt.Errorf("deadlock successfully reproduced")
    }
    
    log.Printf("✅ Both transactions completed without deadlock")
    return nil
}

// 【危険な実装】デッドロックを誘発する資金移動関数
func transferMoney(db *sql.DB, fromID, toID int, amount float64) error {
    tx, err := db.Begin()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()

    log.Printf("TRX: Attempting to lock account %d (debit)", fromID)
    
    // 【STEP 1】送金元アカウントの排他ロック取得
    _, err = tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)
    if err != nil {
        return fmt.Errorf("failed to debit account %d: %w", fromID, err)
    }
    
    log.Printf("TRX: Successfully locked account %d, now waiting before locking %d", fromID, toID)

    // 【危険ゾーン】意図的な遅延でデッドロック確率を上げる
    // この間に他のトランザクションが次のリソースをロックする時間を与える
    time.Sleep(100 * time.Millisecond)

    log.Printf("TRX: Now attempting to lock account %d (credit)", toID)
    
    // 【STEP 2】送金先アカウントの排他ロック取得（デッドロック発生ポイント）
    _, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)
    if err != nil {
        // 【デッドロック検出】ここでデッドロックエラーが発生する
        log.Printf("TRX: Failed to lock account %d: %v", toID, err)
        return fmt.Errorf("failed to credit account %d: %w", toID, err)
    }
    
    log.Printf("TRX: Successfully completed transfer from %d to %d", fromID, toID)

    // 【コミット】両方のロックが取得できた場合のみ実行
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    
    return nil
}

// 【エラー判定】データベース固有のデッドロックエラー検出
func isDeadlockError(err error) bool {
    if err == nil {
        return false
    }
    
    errStr := strings.ToLower(err.Error())
    
    // 【PostgreSQL】デッドロック関連エラーパターン
    deadlockPatterns := []string{
        "deadlock detected",        // 直接的なデッドロックメッセージ
        "40p01",                   // PostgreSQL deadlock_detected エラーコード
        "deadlock",                // 一般的なデッドロック用語
        "lock wait timeout",       // MySQL/MariaDB のロック待機タイムアウト
        "lock timeout",            // SQL Server のロックタイムアウト
    }
    
    for _, pattern := range deadlockPatterns {
        if strings.Contains(errStr, pattern) {
            return true
        }
    }
    
    return false
}

// 【検証用】デッドロック発生条件の確認
func validateDeadlockConditions(db *sql.DB) error {
    // 【確認1】テーブルとデータの存在確認
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM accounts WHERE id IN (1, 2)").Scan(&count)
    if err != nil {
        return fmt.Errorf("failed to verify test accounts: %w", err)
    }
    
    if count < 2 {
        return fmt.Errorf("insufficient test accounts: found %d, need 2", count)
    }
    
    // 【確認2】分離レベルの確認（READ COMMITTEDまたはREPEATABLE READ推奨）
    var isolationLevel string
    err = db.QueryRow("SHOW transaction_isolation").Scan(&isolationLevel)
    if err != nil {
        log.Printf("Warning: Could not check isolation level: %v", err)
    } else {
        log.Printf("Current isolation level: %s", isolationLevel)
    }
    
    return nil
}
```

### デッドロック対策1: 順序付きロック

リソースアクセスの順序を統一することでデッドロックを防ぐ：

```go
// 【デッドロック予防】順序付きロックによる根本的解決
func transferMoneyOrdered(db *sql.DB, fromID, toID int, amount float64) error {
    // 【核心アイデア】常に一定の順序でリソースにアクセス
    // 
    // 【理論的背景】：
    // デッドロック発生の4条件のうち「循環待機」を破ることで、
    // デッドロックを根本的に防止する
    //
    // 【実装方針】：
    // - 全てのトランザクションで同じ順序でロックを取得
    // - ID順序付けにより一貫性を保証
    // - 循環待機の発生を物理的に不可能にする
    
    firstID, secondID := fromID, toID
    firstAmount, secondAmount := -amount, amount
    
    // 【順序統一】常に小さいIDから大きいIDの順でアクセス
    if fromID > toID {
        // 逆方向の送金でも順序を維持
        firstID, secondID = toID, fromID
        firstAmount, secondAmount = amount, -amount
    }

    log.Printf("Ordered transfer: Will lock ID %d first, then ID %d", firstID, secondID)

    tx, err := db.Begin()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()

    // 【STEP 1】最小IDのアカウントを必ず最初にロック
    log.Printf("Locking account %d (first in order)", firstID)
    _, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", firstAmount, firstID)
    if err != nil {
        return fmt.Errorf("failed to update account %d: %w", firstID, err)
    }

    // 【STEP 2】最大IDのアカウントを常に後でロック
    log.Printf("Locking account %d (second in order)", secondID)
    _, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", secondAmount, secondID)
    if err != nil {
        return fmt.Errorf("failed to update account %d: %w", secondID, err)
    }

    log.Printf("✅ Ordered lock strategy: Successfully completed transfer")
    
    // 【成功】循環待機が物理的に不可能なため、デッドロックなし
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    
    return nil
}

// 【汎用実装】複数リソースの順序付きロック
func lockResourcesInOrder(tx *sql.Tx, resourceIDs []int, operation func(*sql.Tx, int) error) error {
    // 【STEP 1】リソースIDをソートして順序を統一
    sortedIDs := make([]int, len(resourceIDs))
    copy(sortedIDs, resourceIDs)
    sort.Ints(sortedIDs)
    
    // 【STEP 2】ソート済み順序でリソースをロック
    for _, id := range sortedIDs {
        if err := operation(tx, id); err != nil {
            return fmt.Errorf("failed to lock resource %d: %w", id, err)
        }
        log.Printf("Successfully locked resource %d", id)
    }
    
    return nil
}

// 【複雑なケース】多方向送金での順序付きロック適用例
func transferMoneyMultiple(db *sql.DB, transfers []Transfer) error {
    // Transfer構造体: {FromID, ToID, Amount}
    
    // 【STEP 1】全関連アカウントIDを収集
    accountIDs := make(map[int]bool)
    for _, t := range transfers {
        accountIDs[t.FromID] = true
        accountIDs[t.ToID] = true
    }
    
    // 【STEP 2】ID順序でソート
    var sortedAccountIDs []int
    for id := range accountIDs {
        sortedAccountIDs = append(sortedAccountIDs, id)
    }
    sort.Ints(sortedAccountIDs)
    
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 【STEP 3】順序付きで全アカウントをロック（FOR UPDATEクエリ）
    for _, accountID := range sortedAccountIDs {
        var balance float64
        err := tx.QueryRow("SELECT balance FROM accounts WHERE id = $1 FOR UPDATE", accountID).Scan(&balance)
        if err != nil {
            return fmt.Errorf("failed to lock account %d: %w", accountID, err)
        }
        log.Printf("Locked account %d for update", accountID)
    }
    
    // 【STEP 4】全ロック取得後に安全に更新実行
    for _, transfer := range transfers {
        _, err = tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", transfer.Amount, transfer.FromID)
        if err != nil {
            return err
        }
        
        _, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", transfer.Amount, transfer.ToID)
        if err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

// 【設計原則】順序付きロックの重要ポイント
//
// 1. 【一貫性】：全てのトランザクションで同じ順序を使用
// 2. 【決定性】：ソート順序は決定的（通常は数値順、文字列辞書順など）
// 3. 【完全性】：必要なリソースを事前に特定し、全て同じ方法で順序付け
// 4. 【効率性】：不要なリソースのロックは避ける
// 5. 【保守性】：順序ルールを明確に文書化し、チーム全体で共有

type Transfer struct {
    FromID int
    ToID   int
    Amount float64
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