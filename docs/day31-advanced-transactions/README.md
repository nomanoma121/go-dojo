# Day 31: 高度なトランザクション管理実装

## 🎯 本日の目標

複数のDB操作を単一のトランザクションにまとめ、エラー時に適切にロールバックする高度なトランザクション制御を実装する。セーブポイント、楽観的ロック、デッドロック対策など、実用的なシナリオを通じて、本格的なデータベースアプリケーションに必要な技術を習得する。

## 📖 解説

### トランザクション管理の重要性

データベースのトランザクションは、**複数の操作を一つの論理的な単位として扱い、ACID特性を保証する仕組み**です。これにより、システム障害やアプリケーションエラーが発生しても、データの整合性を維持できます。

**トランザクションなしの問題例：**

```go
// ❌ トランザクションなしの危険な実装
func transferMoneyUnsafe(db *sql.DB, fromAccountID, toAccountID int, amount decimal.Decimal) error {
    // 1. 送金元から引出
    _, err := db.Exec("UPDATE accounts SET balance = balance - ? WHERE id = ?", amount, fromAccountID)
    if err != nil {
        return err
    }
    
    // ここでシステム障害が発生すると...
    // 送金元からお金が消え、送金先には届かない！
    
    // 2. 送金先に入金
    _, err = db.Exec("UPDATE accounts SET balance = balance + ? WHERE id = ?", amount, toAccountID)
    if err != nil {
        // ロールバックできない！データ不整合が発生
        return err
    }
    
    return nil
}
```

**トランザクションによる改善：**

```go
// ✅ トランザクションによる安全な実装
func transferMoneySafe(db *sql.DB, fromAccountID, toAccountID int, amount decimal.Decimal) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p) // re-throw panic after Rollback
        } else if err != nil {
            tx.Rollback() // エラー時は自動ロールバック
        } else {
            err = tx.Commit() // 成功時はコミット
        }
    }()
    
    // 1. 送金元から引出
    _, err = tx.Exec("UPDATE accounts SET balance = balance - ? WHERE id = ?", amount, fromAccountID)
    if err != nil {
        return err
    }
    
    // 2. 送金先に入金
    _, err = tx.Exec("UPDATE accounts SET balance = balance + ? WHERE id = ?", amount, toAccountID)
    if err != nil {
        return err
    }
    
    // 両方成功時のみコミット
    return nil
}
```

### ACID特性の詳細理解

#### **Atomicity（原子性）**

すべての操作が成功するか、すべて失敗するかの「オールオアナッシング」原則：

```go
func demonstrateAtomicity(db *sql.DB) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback() // 明示的にCommitされない限り、常にRollback
    
    // 複数の関連操作
    operations := []string{
        "INSERT INTO orders (customer_id, total) VALUES (1, 100.00)",
        "INSERT INTO order_items (order_id, product_id, quantity) VALUES (1, 1, 2)", 
        "UPDATE inventory SET quantity = quantity - 2 WHERE product_id = 1",
        "INSERT INTO audit_log (action, timestamp) VALUES ('ORDER_CREATED', NOW())",
    }
    
    for i, operation := range operations {
        _, err := tx.Exec(operation)
        if err != nil {
            // どこか1つでも失敗したら、全て取り消される
            return fmt.Errorf("operation %d failed: %w", i, err)
        }
    }
    
    // 全て成功した場合のみコミット
    return tx.Commit()
}
```

#### **Consistency（一貫性）**

データベースの制約や業務ルールが常に保たれる状態：

```go
func demonstrateConsistency(db *sql.DB) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 業務制約: アカウント残高は必ず0以上でなければならない
    var currentBalance decimal.Decimal
    err = tx.QueryRow("SELECT balance FROM accounts WHERE id = ? FOR UPDATE", 1).Scan(&currentBalance)
    if err != nil {
        return err
    }
    
    withdrawAmount := decimal.NewFromFloat(150.00)
    
    // 制約チェック
    if currentBalance.LessThan(withdrawAmount) {
        return fmt.Errorf("insufficient balance: current=%v, requested=%v", 
            currentBalance, withdrawAmount)
    }
    
    // 制約を満たす場合のみ実行
    _, err = tx.Exec("UPDATE accounts SET balance = balance - ? WHERE id = ?", 
        withdrawAmount, 1)
    if err != nil {
        return err
    }
    
    return tx.Commit()
}
```

#### **Isolation（分離性）**

並行実行される他のトランザクションからの分離：

```go
func demonstrateIsolation(db *sql.DB) {
    // 分離レベルの設定例
    isolationLevels := []sql.IsolationLevel{
        sql.LevelReadUncommitted, // ダーティリード可能
        sql.LevelReadCommitted,   // ダーティリード不可、ファントムリード可能  
        sql.LevelRepeatableRead,  // ファントムリード不可、MySQL InnoDBのデフォルト
        sql.LevelSerializable,    // 最も厳格、性能低下あり
    }
    
    for _, level := range isolationLevels {
        err := demonstrateIsolationLevel(db, level)
        if err != nil {
            log.Printf("Isolation level %v failed: %v", level, err)
        }
    }
}

func demonstrateIsolationLevel(db *sql.DB, level sql.IsolationLevel) error {
    tx, err := db.BeginTx(context.Background(), &sql.TxOptions{
        Isolation: level,
    })
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 分離レベルに応じて異なる動作を示す
    var count int
    err = tx.QueryRow("SELECT COUNT(*) FROM accounts WHERE balance > 1000").Scan(&count)
    if err != nil {
        return err
    }
    
    fmt.Printf("Isolation %v: Found %d accounts with balance > 1000\n", level, count)
    return tx.Commit()
}
```

#### **Durability（永続性）**

コミット後のデータは永続的に保存される：

```go
func demonstrateDurability(db *sql.DB) error {
    // WAL（Write-Ahead Logging）の確認
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 重要なデータの永続化
    result, err := tx.Exec(`
        INSERT INTO critical_transactions (id, amount, timestamp, checksum) 
        VALUES (?, ?, ?, ?)
    `, uuid.New(), 1000.00, time.Now(), generateChecksum())
    
    if err != nil {
        return err
    }
    
    // コミット時にディスクに書き込み保証
    err = tx.Commit()
    if err != nil {
        return err
    }
    
    // コミット成功 = データの永続化保証
    rowsAffected, _ := result.RowsAffected()
    fmt.Printf("Durability guaranteed: %d rows permanently stored\n", rowsAffected)
    
    return nil
}

func generateChecksum() string {
    // データ整合性チェック用のチェックサム
    h := sha256.New()
    h.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
    return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
```

### Goでのトランザクション制御

```go
package main

import (
    "database/sql"
    "fmt"
    _ "github.com/lib/pq"
)

// 基本的なトランザクション
func basicTransaction(db *sql.DB) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer func() {
        if err != nil {
            tx.Rollback()
        }
    }()

    // 複数の操作
    _, err = tx.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", "Alice", "alice@example.com")
    if err != nil {
        return err
    }

    _, err = tx.Exec("UPDATE accounts SET balance = balance - 100 WHERE user_id = $1", 1)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

### セーブポイント（Savepoint）

PostgreSQLなどではネストしたトランザクションを擬似的に実現するためにセーブポイントを使用できます：

```go
func withSavepoint(tx *sql.Tx) error {
    // セーブポイントを作成
    _, err := tx.Exec("SAVEPOINT sp1")
    if err != nil {
        return err
    }

    // 危険な操作
    _, err = tx.Exec("INSERT INTO sensitive_data ...")
    if err != nil {
        // セーブポイントまでロールバック
        tx.Exec("ROLLBACK TO SAVEPOINT sp1")
        return err
    }

    // セーブポイントを解放
    _, err = tx.Exec("RELEASE SAVEPOINT sp1")
    return err
}
```

### 楽観的ロック（Optimistic Locking）

バージョン番号を使用した競合制御：

```go
type User struct {
    ID      int
    Name    string
    Version int
}

func updateUserOptimistic(tx *sql.Tx, user *User, newName string) error {
    result, err := tx.Exec(
        "UPDATE users SET name = $1, version = version + 1 WHERE id = $2 AND version = $3",
        newName, user.ID, user.Version,
    )
    if err != nil {
        return err
    }

    affected, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if affected == 0 {
        return fmt.Errorf("optimistic lock error: data was modified by another transaction")
    }

    user.Version++
    return nil
}
```

### デッドロック対策

デッドロックが発生した場合の検出と再試行：

```go
func executeWithDeadlockRetry(db *sql.DB, operation func(*sql.Tx) error) error {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        err := executeInTransaction(db, operation)
        if err == nil {
            return nil
        }

        // PostgreSQLのデッドロックエラーコード
        if isDeadlockError(err) && i < maxRetries-1 {
            time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
            continue
        }
        return err
    }
    return fmt.Errorf("transaction failed after %d retries", maxRetries)
}

func isDeadlockError(err error) bool {
    // PostgreSQLのデッドロックエラーコード: 40P01
    return strings.Contains(err.Error(), "40P01") || 
           strings.Contains(err.Error(), "deadlock detected")
}
```

📝 **課題**

以下の機能を持つ高度なトランザクション管理システムを実装してください：

1. **`TransactionManager`構造体**: 複数のDB操作を管理
2. **`ExecuteInTransaction`メソッド**: トランザクション内での操作実行
3. **`WithSavepoint`メソッド**: セーブポイントを使ったネストトランザクション
4. **`TransferMoney`関数**: 楽観的ロックを使った資金移動
5. **`BulkOperation`関数**: 大量データの一括処理とバッチコミット

具体的な実装要件：
- PostgreSQLを使用したトランザクション制御
- エラー時の適切なロールバック処理
- セーブポイントによるネストしたトランザクション
- 楽観的ロックによる競合制御
- デッドロック検出と再試行機能

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestTransactionManager_ExecuteInTransaction
--- PASS: TestTransactionManager_ExecuteInTransaction (0.01s)
=== RUN   TestTransactionManager_WithSavepoint
--- PASS: TestTransactionManager_WithSavepoint (0.01s)
=== RUN   TestTransferMoney_OptimisticLock
--- PASS: TestTransferMoney_OptimisticLock (0.02s)
=== RUN   TestBulkOperation_BatchCommit
--- PASS: TestBulkOperation_BatchCommit (0.05s)
=== RUN   TestDeadlockRetry
--- PASS: TestDeadlockRetry (0.10s)
PASS
ok      day31-advanced-transactions    0.182s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **database/sql**: Goの標準SQLドライバ
2. **github.com/lib/pq**: PostgreSQLドライバ
3. **sql.Tx**: トランザクションオブジェクト
4. **defer**文でのリソース管理パターン
5. **context.Context**: タイムアウト付きトランザクション
6. **sync.Mutex**: 並行アクセス制御（必要に応じて）

データベーススキーマ例：
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    version INTEGER DEFAULT 0
);

CREATE TABLE accounts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    balance DECIMAL(10, 2) DEFAULT 0,
    version INTEGER DEFAULT 0
);
```

## 実行方法

```bash
# PostgreSQLコンテナを起動
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=testdb postgres:15

# テスト実行
go test -v
go test -race  # レースコンディションの検出
```