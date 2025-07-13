# Day 31: 高度なトランザクション管理

🎯 **本日の目標**

複数のDB操作を単一のトランザクションにまとめ、エラー時に適切にロールバックする高度なトランザクション制御を実装できるようになる。

📖 **解説**

## トランザクション管理の基礎

データベースのトランザクションは、複数の操作を一つの論理的な単位として扱い、ACID特性を保証する仕組みです。

### ACID特性
- **Atomicity（原子性）**: すべての操作が成功するか、すべて失敗するか
- **Consistency（一貫性）**: データベースの整合性が保たれる
- **Isolation（分離性）**: 並行実行される他のトランザクションから影響を受けない
- **Durability（永続性）**: コミット後のデータは永続的に保存される

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