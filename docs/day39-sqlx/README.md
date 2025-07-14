# Day 39: sqlxによる効率的なDB操作

🎯 **本日の目標**

`database/sql`の煩雑さを`sqlx`で解消し、より効率的で可読性の高いデータベース操作ができるようになる。

📖 **解説**

## sqlxとは

sqlxは、Goの標準`database/sql`パッケージを拡張したライブラリです。構造体への直接マッピング、名前付きパラメータ、プリペアードステートメントの改善など、多くの便利機能を提供します。

### sqlxの主な機能

#### 1. 構造体への直接マッピング
```go
// 標準database/sql
rows, err := db.Query("SELECT id, name, email FROM users")
for rows.Next() {
    var user User
    err := rows.Scan(&user.ID, &user.Name, &user.Email)
    // エラーハンドリング...
}

// sqlxを使用
var users []User
err := db.Select(&users, "SELECT id, name, email FROM users")
```

#### 2. 名前付きパラメータ
```go
// 標準database/sql
result, err := db.Exec("INSERT INTO users (name, email, age) VALUES ($1, $2, $3)", 
    user.Name, user.Email, user.Age)

// sqlxを使用
result, err := db.NamedExec("INSERT INTO users (name, email, age) VALUES (:name, :email, :age)", 
    user)
```

### 基本的なsqlx使用例

```go
package main

import (
    "database/sql"
    "log"
    
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

// User represents a user entity
type User struct {
    ID       int    `db:"id"`
    Name     string `db:"name"`
    Email    string `db:"email"`
    Age      *int   `db:"age"` // NULL許可のためpointer使用
    Created  time.Time `db:"created_at"`
}

// UserRepository handles user database operations
type UserRepository struct {
    db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
    return &UserRepository{db: db}
}

// GetByID retrieves a user by ID
func (ur *UserRepository) GetByID(id int) (*User, error) {
    var user User
    err := ur.db.Get(&user, "SELECT * FROM users WHERE id = $1", id)
    if err != nil {
        return nil, err
    }
    return &user, nil
}

// GetAll retrieves all users
func (ur *UserRepository) GetAll() ([]User, error) {
    var users []User
    err := ur.db.Select(&users, "SELECT * FROM users ORDER BY created_at DESC")
    return users, err
}

// Create creates a new user
func (ur *UserRepository) Create(user *User) error {
    query := `
        INSERT INTO users (name, email, age) 
        VALUES (:name, :email, :age) 
        RETURNING id, created_at`
    
    stmt, err := ur.db.PrepareNamed(query)
    if err != nil {
        return err
    }
    defer stmt.Close()
    
    return stmt.Get(user, user)
}

// Update updates an existing user
func (ur *UserRepository) Update(user *User) error {
    query := `
        UPDATE users 
        SET name = :name, email = :email, age = :age 
        WHERE id = :id`
    
    _, err := ur.db.NamedExec(query, user)
    return err
}

// Delete deletes a user
func (ur *UserRepository) Delete(id int) error {
    _, err := ur.db.Exec("DELETE FROM users WHERE id = $1", id)
    return err
}
```

### 高度なsqlx機能

#### In句の展開
```go
// In clause with slice
func (ur *UserRepository) GetByIDs(ids []int) ([]User, error) {
    query, args, err := sqlx.In("SELECT * FROM users WHERE id IN (?)", ids)
    if err != nil {
        return nil, err
    }
    
    // PostgreSQL用にリバインド
    query = ur.db.Rebind(query)
    
    var users []User
    err = ur.db.Select(&users, query, args...)
    return users, err
}
```

#### バッチ操作
```go
// BatchInsert inserts multiple users efficiently
func (ur *UserRepository) BatchInsert(users []User) error {
    tx, err := ur.db.Beginx()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    stmt, err := tx.PrepareNamed(`
        INSERT INTO users (name, email, age) 
        VALUES (:name, :email, :age)`)
    if err != nil {
        return err
    }
    defer stmt.Close()
    
    for _, user := range users {
        if _, err := stmt.Exec(user); err != nil {
            return err
        }
    }
    
    return tx.Commit()
}
```

### sqlxを使ったトランザクション処理

```go
// TransferService handles money transfer between users
type TransferService struct {
    db *sqlx.DB
}

func (ts *TransferService) Transfer(fromUserID, toUserID int, amount decimal.Decimal) error {
    tx, err := ts.db.Beginx()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // 送金者の残高確認
    var fromBalance decimal.Decimal
    err = tx.Get(&fromBalance, 
        "SELECT balance FROM accounts WHERE user_id = $1 FOR UPDATE", 
        fromUserID)
    if err != nil {
        return err
    }
    
    if fromBalance.LessThan(amount) {
        return errors.New("insufficient balance")
    }
    
    // 送金者から減額
    _, err = tx.Exec(
        "UPDATE accounts SET balance = balance - $1 WHERE user_id = $2",
        amount, fromUserID)
    if err != nil {
        return err
    }
    
    // 受取者に加算
    _, err = tx.Exec(
        "UPDATE accounts SET balance = balance + $1 WHERE user_id = $2",
        amount, toUserID)
    if err != nil {
        return err
    }
    
    // トランザクション履歴を記録
    _, err = tx.NamedExec(`
        INSERT INTO transfers (from_user_id, to_user_id, amount, created_at)
        VALUES (:from_user_id, :to_user_id, :amount, NOW())`,
        map[string]interface{}{
            "from_user_id": fromUserID,
            "to_user_id":   toUserID,
            "amount":       amount,
        })
    if err != nil {
        return err
    }
    
    return tx.Commit()
}
```

### カスタムタイプとスキャナー

```go
// JSONB型のカスタムスキャナー
type JSONB map[string]interface{}

func (j *JSONB) Scan(value interface{}) error {
    if value == nil {
        return nil
    }
    
    bytes, ok := value.([]byte)
    if !ok {
        return errors.New("cannot scan into JSONB")
    }
    
    return json.Unmarshal(bytes, j)
}

func (j JSONB) Value() (driver.Value, error) {
    if j == nil {
        return nil, nil
    }
    return json.Marshal(j)
}

// カスタムタイプを使った構造体
type UserProfile struct {
    ID       int    `db:"id"`
    UserID   int    `db:"user_id"`
    Metadata JSONB  `db:"metadata"`
    Settings JSONB  `db:"settings"`
}
```

### sqlxを使ったテスト支援

```go
// TestRepository provides test helper methods
type TestRepository struct {
    db *sqlx.DB
}

func NewTestRepository(db *sqlx.DB) *TestRepository {
    return &TestRepository{db: db}
}

// TruncateAll truncates all tables for testing
func (tr *TestRepository) TruncateAll() error {
    tables := []string{"users", "orders", "accounts", "transfers"}
    
    tx, err := tr.db.Beginx()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    for _, table := range tables {
        if _, err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)); err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

// SeedTestData inserts test data
func (tr *TestRepository) SeedTestData() error {
    users := []User{
        {Name: "Alice", Email: "alice@example.com", Age: intPtr(25)},
        {Name: "Bob", Email: "bob@example.com", Age: intPtr(30)},
        {Name: "Charlie", Email: "charlie@example.com", Age: intPtr(35)},
    }
    
    tx, err := tr.db.Beginx()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    for _, user := range users {
        if _, err := tx.NamedExec(`
            INSERT INTO users (name, email, age) 
            VALUES (:name, :email, :age)`, user); err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

func intPtr(i int) *int {
    return &i
}
```

📝 **課題**

以下の機能を持つsqlxベースのデータベース操作システムを実装してください：

1. **`UserRepository`**: ユーザーのCRUD操作
2. **`OrderRepository`**: 注文データの管理
3. **`TransactionService`**: トランザクション処理
4. **`QueryBuilder`**: 動的クエリ構築
5. **`MigrationRunner`**: スキーママイグレーション
6. **`TestHelper`**: テスト支援機能

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestUserRepository_CRUD
--- PASS: TestUserRepository_CRUD (0.02s)
=== RUN   TestOrderRepository_Advanced
--- PASS: TestOrderRepository_Advanced (0.03s)
=== RUN   TestTransactionService_Transfer
--- PASS: TestTransactionService_Transfer (0.05s)
=== RUN   TestQueryBuilder_Dynamic
--- PASS: TestQueryBuilder_Dynamic (0.02s)
=== RUN   TestMigrationRunner_Schema
--- PASS: TestMigrationRunner_Schema (0.10s)
PASS
ok      day39-sqlx    0.220s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **github.com/jmoiron/sqlx**: 標準パッケージの拡張
2. **構造体タグ**: `db`タグでカラムマッピング
3. **Named queries**: `:name`形式の名前付きパラメータ
4. **Batch operations**: 効率的な一括処理
5. **Custom types**: database/sql/driverインターフェース

sqlxの利点：
- **コード削減**: Scanの記述量削減
- **型安全性**: 構造体への直接マッピング
- **可読性向上**: 名前付きパラメータ
- **エラー削減**: 手動Scanによるミス防止

## 実行方法

```bash
go mod tidy  # sqlx依存関係をインストール
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```