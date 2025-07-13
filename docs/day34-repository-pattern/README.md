# Day 34: Repositoryパターン

🎯 **本日の目標**

DB操作のロジックをカプセル化し、ビジネスロジックから分離するRepositoryパターンを実装できるようになる。データアクセス層の抽象化によりテスタビリティと保守性を向上させる。

📖 **解説**

## Repositoryパターンとは

Repositoryパターンは、データアクセス層を抽象化するデザインパターンです。ビジネスロジックからデータベースの詳細を隠蔽し、データアクセスロジックを一箇所に集約することで、保守性とテスタビリティを向上させます。

### Repositoryパターンの利点

1. **関心の分離**: ビジネスロジックとデータアクセスロジックを分離
2. **テスタビリティ**: インターフェースによりモック可能
3. **保守性**: データアクセス層の変更がビジネスロジックに影響しない
4. **再利用性**: 共通のデータアクセスロジックを再利用可能

### 基本的なRepositoryパターンの実装

```go
package main

import (
    "context"
    "database/sql"
)

// User represents a user entity
type User struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
    Created  time.Time `json:"created"`
}

// UserRepository defines the interface for user data access
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id int) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id int) error
    List(ctx context.Context, limit, offset int) ([]*User, error)
}

// PostgreSQLUserRepository implements UserRepository for PostgreSQL
type PostgreSQLUserRepository struct {
    db *sql.DB
}

// NewPostgreSQLUserRepository creates a new PostgreSQL user repository
func NewPostgreSQLUserRepository(db *sql.DB) UserRepository {
    return &PostgreSQLUserRepository{db: db}
}

// Create creates a new user
func (r *PostgreSQLUserRepository) Create(ctx context.Context, user *User) error {
    query := `
        INSERT INTO users (username, email, created) 
        VALUES ($1, $2, $3) 
        RETURNING id`
    
    err := r.db.QueryRowContext(ctx, query, user.Username, user.Email, time.Now()).
        Scan(&user.ID)
    return err
}

// GetByID retrieves a user by ID
func (r *PostgreSQLUserRepository) GetByID(ctx context.Context, id int) (*User, error) {
    query := `
        SELECT id, username, email, created 
        FROM users 
        WHERE id = $1`
    
    user := &User{}
    err := r.db.QueryRowContext(ctx, query, id).
        Scan(&user.ID, &user.Username, &user.Email, &user.Created)
    
    if err == sql.ErrNoRows {
        return nil, nil
    }
    return user, err
}
```

### トランザクション対応のRepository

```go
// TxRepository represents a repository that can work with transactions
type TxRepository interface {
    UserRepository
    WithTx(tx *sql.Tx) UserRepository
}

// PostgreSQLUserTxRepository extends PostgreSQL repository with transaction support
type PostgreSQLUserTxRepository struct {
    db *sql.DB
    tx *sql.Tx
}

// NewPostgreSQLUserTxRepository creates a transaction-aware repository
func NewPostgreSQLUserTxRepository(db *sql.DB) TxRepository {
    return &PostgreSQLUserTxRepository{db: db}
}

// WithTx returns a repository that uses the provided transaction
func (r *PostgreSQLUserTxRepository) WithTx(tx *sql.Tx) UserRepository {
    return &PostgreSQLUserTxRepository{db: r.db, tx: tx}
}

// getDB returns the appropriate database connection or transaction
func (r *PostgreSQLUserTxRepository) getDB() interface {
    QueryRowContext(context.Context, string, ...interface{}) *sql.Row
    ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
    QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
} {
    if r.tx != nil {
        return r.tx
    }
    return r.db
}

// Create with transaction support
func (r *PostgreSQLUserTxRepository) Create(ctx context.Context, user *User) error {
    query := `
        INSERT INTO users (username, email, created) 
        VALUES ($1, $2, $3) 
        RETURNING id`
    
    db := r.getDB()
    err := db.QueryRowContext(ctx, query, user.Username, user.Email, time.Now()).
        Scan(&user.ID)
    return err
}
```

### サービス層との統合

```go
// UserService provides business logic for user operations
type UserService struct {
    userRepo UserRepository
    db       *sql.DB
}

// NewUserService creates a new user service
func NewUserService(userRepo UserRepository, db *sql.DB) *UserService {
    return &UserService{
        userRepo: userRepo,
        db:       db,
    }
}

// CreateUserWithProfile creates a user and their profile in a single transaction
func (s *UserService) CreateUserWithProfile(ctx context.Context, user *User, profile *Profile) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // トランザクション対応リポジトリを使用
    txUserRepo := s.userRepo.(TxRepository).WithTx(tx)
    
    // ユーザー作成
    err = txUserRepo.Create(ctx, user)
    if err != nil {
        return err
    }

    // プロフィール作成（ここではダミー実装）
    profile.UserID = user.ID
    // profileRepo.WithTx(tx).Create(ctx, profile)

    return tx.Commit()
}
```

### Unit of Work パターン

```go
// UnitOfWork manages multiple repositories in a single transaction
type UnitOfWork struct {
    db       *sql.DB
    tx       *sql.Tx
    userRepo TxRepository
    postRepo TxRepository
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(db *sql.DB) *UnitOfWork {
    return &UnitOfWork{
        db:       db,
        userRepo: NewPostgreSQLUserTxRepository(db),
        postRepo: NewPostgreSQLPostTxRepository(db),
    }
}

// Begin starts a new transaction
func (uow *UnitOfWork) Begin(ctx context.Context) error {
    tx, err := uow.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    uow.tx = tx
    return nil
}

// Users returns the user repository within the transaction
func (uow *UnitOfWork) Users() UserRepository {
    if uow.tx != nil {
        return uow.userRepo.WithTx(uow.tx)
    }
    return uow.userRepo
}

// Posts returns the post repository within the transaction
func (uow *UnitOfWork) Posts() PostRepository {
    if uow.tx != nil {
        return uow.postRepo.WithTx(uow.tx)
    }
    return uow.postRepo
}

// Commit commits the transaction
func (uow *UnitOfWork) Commit() error {
    if uow.tx == nil {
        return fmt.Errorf("no active transaction")
    }
    err := uow.tx.Commit()
    uow.tx = nil
    return err
}

// Rollback rolls back the transaction
func (uow *UnitOfWork) Rollback() error {
    if uow.tx == nil {
        return nil
    }
    err := uow.tx.Rollback()
    uow.tx = nil
    return err
}
```

### テスト用モックRepository

```go
// MockUserRepository implements UserRepository for testing
type MockUserRepository struct {
    users map[int]*User
    nextID int
    mu     sync.RWMutex
}

// NewMockUserRepository creates a new mock repository
func NewMockUserRepository() UserRepository {
    return &MockUserRepository{
        users:  make(map[int]*User),
        nextID: 1,
    }
}

// Create creates a user in memory
func (m *MockUserRepository) Create(ctx context.Context, user *User) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    user.ID = m.nextID
    m.nextID++
    user.Created = time.Now()
    m.users[user.ID] = user
    return nil
}

// GetByID retrieves a user by ID from memory
func (m *MockUserRepository) GetByID(ctx context.Context, id int) (*User, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    user, exists := m.users[id]
    if !exists {
        return nil, nil
    }
    
    // Return a copy to avoid data races
    userCopy := *user
    return &userCopy, nil
}
```

### Specification パターンの組み合わせ

```go
// UserSpecification defines criteria for querying users
type UserSpecification interface {
    ToSQL() (string, []interface{})
}

// UserByEmailSpec specification for finding users by email
type UserByEmailSpec struct {
    Email string
}

func (s UserByEmailSpec) ToSQL() (string, []interface{}) {
    return "email = $1", []interface{}{s.Email}
}

// UserCreatedAfterSpec specification for finding users created after a date
type UserCreatedAfterSpec struct {
    After time.Time
}

func (s UserCreatedAfterSpec) ToSQL() (string, []interface{}) {
    return "created > $1", []interface{}{s.After}
}

// AndSpec combines specifications with AND
type AndSpec struct {
    Left, Right UserSpecification
}

func (s AndSpec) ToSQL() (string, []interface{}) {
    leftSQL, leftArgs := s.Left.ToSQL()
    rightSQL, rightArgs := s.Right.ToSQL()
    
    sql := fmt.Sprintf("(%s) AND (%s)", leftSQL, rightSQL)
    args := append(leftArgs, rightArgs...)
    return sql, args
}

// Enhanced repository with specification support
func (r *PostgreSQLUserRepository) FindBySpec(ctx context.Context, spec UserSpecification) ([]*User, error) {
    whereClause, args := spec.ToSQL()
    query := fmt.Sprintf("SELECT id, username, email, created FROM users WHERE %s", whereClause)
    
    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var users []*User
    for rows.Next() {
        user := &User{}
        err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Created)
        if err != nil {
            return nil, err
        }
        users = append(users, user)
    }
    
    return users, rows.Err()
}
```

📝 **課題**

以下の機能を持つRepositoryパターンシステムを実装してください：

1. **`UserRepository`インターフェース**: ユーザーデータアクセスの抽象化
2. **`PostgreSQLUserRepository`**: PostgreSQL実装
3. **`MockUserRepository`**: テスト用インメモリ実装
4. **`UserService`**: ビジネスロジック層
5. **`UnitOfWork`**: トランザクション管理

具体的な実装要件：
- CRUD操作の完全な実装
- トランザクション対応
- エラーハンドリング
- コンテキスト対応
- ページング機能
- 検索機能（Specification パターン）

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestUserRepository_Create
--- PASS: TestUserRepository_Create (0.01s)
=== RUN   TestUserRepository_GetByID
--- PASS: TestUserRepository_GetByID (0.01s)
=== RUN   TestUserRepository_Update
--- PASS: TestUserRepository_Update (0.01s)
=== RUN   TestUserRepository_Delete
--- PASS: TestUserRepository_Delete (0.01s)
=== RUN   TestUserRepository_List
--- PASS: TestUserRepository_List (0.01s)
=== RUN   TestUserRepository_FindBySpec
--- PASS: TestUserRepository_FindBySpec (0.02s)
=== RUN   TestUnitOfWork_Transaction
--- PASS: TestUnitOfWork_Transaction (0.01s)
=== RUN   TestUserService_CreateUserWithProfile
--- PASS: TestUserService_CreateUserWithProfile (0.02s)
PASS
ok      day34-repository-pattern    0.095s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **database/sql**: Goの標準SQL ドライバ
2. **context**: リクエストスコープの管理
3. **sync**: 並行安全性（モック実装で必要）
4. **time**: タイムスタンプ処理
5. **fmt**: SQLクエリの動的生成

Repository パターンのベストプラクティス：
- **インターフェース優先**: 具象型ではなくインターフェースに依存
- **単一責任**: 各Repositoryは一つのエンティティに責任を持つ
- **トランザクション透過性**: Repositoryはトランザクションの境界を知らない
- **エラーハンドリング**: データベース固有のエラーをアプリケーションエラーに変換

データベーススキーマ例：
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    title VARCHAR(255) NOT NULL,
    content TEXT,
    created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 実行方法

```bash
# PostgreSQLコンテナを起動
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=testdb postgres:15

# テスト実行
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```