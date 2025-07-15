# Day 39: sqlxによる効率的なDB操作と高度なクエリパターン

## 🎯 本日の目標

このチャレンジを通して、以下のスキルを身につけることができます：

- **sqlxライブラリを活用した効率的なデータベース操作ができるようになる**
- **構造体マッピングと名前付きパラメータで保守性の高いコードを書けるようになる**
- **複雑なクエリパターンを型安全かつ効率的に実装できるようになる**
- **プロダクション環境でのsqlx運用ベストプラクティスをマスターする**

## 📖 解説

### なぜsqlxが必要なのか？

標準の`database/sql`パッケージは強力ですが、実際の開発では以下の課題があります：

#### 標準database/sqlの課題

```go
// 従来のdatabase/sql：冗長で保守性が低い
func GetUsersByAgeRange(db *sql.DB, minAge, maxAge int) ([]User, error) {
    query := `
        SELECT id, name, email, age, created_at, updated_at, status, profile_json
        FROM users 
        WHERE age BETWEEN ? AND ? 
        ORDER BY created_at DESC
    `
    
    rows, err := db.Query(query, minAge, maxAge)
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }
    defer rows.Close()
    
    var users []User
    for rows.Next() {
        var user User
        var profileJSON sql.NullString
        
        // 多数のフィールドを個別にScan - エラーが起きやすい
        err := rows.Scan(
            &user.ID,
            &user.Name,
            &user.Email,
            &user.Age,
            &user.CreatedAt,
            &user.UpdatedAt,
            &user.Status,
            &profileJSON,
        )
        if err != nil {
            return nil, fmt.Errorf("scan failed: %w", err)
        }
        
        // NULL値の手動処理
        if profileJSON.Valid {
            if err := json.Unmarshal([]byte(profileJSON.String), &user.Profile); err != nil {
                return nil, fmt.Errorf("profile unmarshal failed: %w", err)
            }
        }
        
        users = append(users, user)
    }
    
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows iteration error: %w", err)
    }
    
    return users, nil
}
```

**問題点：**
- **冗長性**: 大量のボイラープレートコード
- **エラーリスク**: フィールド順序の間違いやタイプミスが頻発
- **保守性**: 構造体変更時に多数の箇所を修正
- **NULL処理**: sql.NullStringなどの手動処理が煩雑

### sqlxによる劇的な改善

同じ機能をsqlxで実装すると：

```go
import "github.com/jmoiron/sqlx"

func GetUsersByAgeRange(db *sqlx.DB, minAge, maxAge int) ([]User, error) {
    query := `
        SELECT id, name, email, age, created_at, updated_at, status, profile_json
        FROM users 
        WHERE age BETWEEN :min_age AND :max_age 
        ORDER BY created_at DESC
    `
    
    var users []User
    err := db.Select(&users, query, map[string]interface{}{
        "min_age": minAge,
        "max_age": maxAge,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to get users: %w", err)
    }
    
    return users, nil
}
```

**改善効果：**
- **行数**: 50行 → 15行（70%削減）
- **エラーリスク**: フィールドマッピングの自動化
- **可読性**: 名前付きパラメータで意図が明確
- **保守性**: 構造体変更への自動対応

### sqlxの高度な機能

#### 1. 構造体タグによる柔軟なマッピング

```go
type User struct {
    ID        int       `db:"user_id" json:"id"`
    Name      string    `db:"full_name" json:"name"`
    Email     string    `db:"email_address" json:"email"`
    Age       int       `db:"age" json:"age"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
    UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
    IsActive  bool      `db:"is_active" json:"is_active"`
    
    // 埋め込み構造体のサポート
    Profile   UserProfile `db:"profile_json" json:"profile"`
    
    // 計算フィールド（データベースには存在しない）
    FullDisplayName string `db:"-" json:"full_display_name"`
}

type UserProfile struct {
    Bio       string   `json:"bio"`
    Interests []string `json:"interests"`
    Location  string   `json:"location"`
}

// カスタムスキャナーの実装
func (p *UserProfile) Scan(src interface{}) error {
    if src == nil {
        return nil
    }
    
    switch s := src.(type) {
    case string:
        return json.Unmarshal([]byte(s), p)
    case []byte:
        return json.Unmarshal(s, p)
    default:
        return fmt.Errorf("cannot scan %T into UserProfile", src)
    }
}

func (p UserProfile) Value() (driver.Value, error) {
    return json.Marshal(p)
}
```

#### 2. 名前付きパラメータとINクエリ

```go
type UserFilter struct {
    IDs          []int     `db:"ids"`
    AgeMin       *int      `db:"age_min"`
    AgeMax       *int      `db:"age_max"`
    NamePattern  string    `db:"name_pattern"`
    CreatedAfter time.Time `db:"created_after"`
    IsActive     *bool     `db:"is_active"`
    Limit        int       `db:"limit"`
    Offset       int       `db:"offset"`
}

func GetUsersWithFilter(db *sqlx.DB, filter UserFilter) ([]User, error) {
    var conditions []string
    var args = make(map[string]interface{})
    
    // 動的WHERE句の構築
    if len(filter.IDs) > 0 {
        conditions = append(conditions, "id IN (:ids)")
        args["ids"] = filter.IDs
    }
    
    if filter.AgeMin != nil {
        conditions = append(conditions, "age >= :age_min")
        args["age_min"] = *filter.AgeMin
    }
    
    if filter.AgeMax != nil {
        conditions = append(conditions, "age <= :age_max")
        args["age_max"] = *filter.AgeMax
    }
    
    if filter.NamePattern != "" {
        conditions = append(conditions, "name ILIKE :name_pattern")
        args["name_pattern"] = "%" + filter.NamePattern + "%"
    }
    
    if !filter.CreatedAfter.IsZero() {
        conditions = append(conditions, "created_at > :created_after")
        args["created_after"] = filter.CreatedAfter
    }
    
    if filter.IsActive != nil {
        conditions = append(conditions, "is_active = :is_active")
        args["is_active"] = *filter.IsActive
    }
    
    // クエリの組み立て
    baseQuery := `
        SELECT id, name, email, age, created_at, updated_at, is_active, profile_json
        FROM users
    `
    
    if len(conditions) > 0 {
        baseQuery += " WHERE " + strings.Join(conditions, " AND ")
    }
    
    baseQuery += " ORDER BY created_at DESC LIMIT :limit OFFSET :offset"
    
    args["limit"] = filter.Limit
    args["offset"] = filter.Offset
    
    // 名前付きクエリの実行
    query, args, err := sqlx.Named(baseQuery, args)
    if err != nil {
        return nil, fmt.Errorf("failed to build named query: %w", err)
    }
    
    // INクエリの展開
    query, args, err = sqlx.In(query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to expand IN query: %w", err)
    }
    
    // プレースホルダーの変換
    query = db.Rebind(query)
    
    var users []User
    err = db.Select(&users, query, args...)
    if err != nil {
        return nil, fmt.Errorf("failed to execute query: %w", err)
    }
    
    return users, nil
}
```

#### 3. トランザクション処理の改善

```go
type UserService struct {
    db *sqlx.DB
}

func (s *UserService) CreateUserWithProfile(ctx context.Context, req CreateUserRequest) (*User, error) {
    // トランザクション開始
    tx, err := s.db.BeginTxx(ctx, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to begin transaction: %w", err)
    }
    
    // ロールバック用のdefer
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p)
        } else if err != nil {
            tx.Rollback()
        } else {
            err = tx.Commit()
        }
    }()
    
    // ユーザー作成
    userQuery := `
        INSERT INTO users (name, email, age, created_at, updated_at)
        VALUES (:name, :email, :age, :created_at, :updated_at)
        RETURNING id
    `
    
    now := time.Now()
    userParams := map[string]interface{}{
        "name":       req.Name,
        "email":      req.Email,
        "age":        req.Age,
        "created_at": now,
        "updated_at": now,
    }
    
    // NamedQueryでINSERT+RETURNING
    rows, err := tx.NamedQuery(userQuery, userParams)
    if err != nil {
        return nil, fmt.Errorf("failed to create user: %w", err)
    }
    defer rows.Close()
    
    if !rows.Next() {
        return nil, fmt.Errorf("failed to get user ID from insert")
    }
    
    var userID int
    if err := rows.Scan(&userID); err != nil {
        return nil, fmt.Errorf("failed to scan user ID: %w", err)
    }
    
    // プロフィール作成
    if req.Profile != nil {
        profileQuery := `
            INSERT INTO user_profiles (user_id, bio, location, interests, created_at)
            VALUES (:user_id, :bio, :location, :interests, :created_at)
        `
        
        profileParams := map[string]interface{}{
            "user_id":    userID,
            "bio":        req.Profile.Bio,
            "location":   req.Profile.Location,
            "interests":  pq.Array(req.Profile.Interests),
            "created_at": now,
        }
        
        _, err = tx.NamedExec(profileQuery, profileParams)
        if err != nil {
            return nil, fmt.Errorf("failed to create profile: %w", err)
        }
    }
    
    // 作成されたユーザーを取得
    var user User
    getUserQuery := `
        SELECT u.id, u.name, u.email, u.age, u.created_at, u.updated_at,
               p.bio, p.location, p.interests
        FROM users u
        LEFT JOIN user_profiles p ON u.id = p.user_id
        WHERE u.id = $1
    `
    
    err = tx.Get(&user, getUserQuery, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get created user: %w", err)
    }
    
    return &user, nil
}
```

#### 4. バッチ操作とプリペアードステートメント

```go
type BatchUserProcessor struct {
    db         *sqlx.DB
    insertStmt *sqlx.NamedStmt
    updateStmt *sqlx.NamedStmt
}

func NewBatchUserProcessor(db *sqlx.DB) (*BatchUserProcessor, error) {
    // プリペアードステートメントの作成
    insertStmt, err := db.PrepareNamed(`
        INSERT INTO users (name, email, age, created_at, updated_at)
        VALUES (:name, :email, :age, :created_at, :updated_at)
    `)
    if err != nil {
        return nil, fmt.Errorf("failed to prepare insert statement: %w", err)
    }
    
    updateStmt, err := db.PrepareNamed(`
        UPDATE users 
        SET name = :name, email = :email, age = :age, updated_at = :updated_at
        WHERE id = :id
    `)
    if err != nil {
        insertStmt.Close()
        return nil, fmt.Errorf("failed to prepare update statement: %w", err)
    }
    
    return &BatchUserProcessor{
        db:         db,
        insertStmt: insertStmt,
        updateStmt: updateStmt,
    }, nil
}

func (bp *BatchUserProcessor) Close() error {
    if err := bp.insertStmt.Close(); err != nil {
        return err
    }
    return bp.updateStmt.Close()
}

func (bp *BatchUserProcessor) ProcessBatch(ctx context.Context, users []UserBatchItem) (*BatchResult, error) {
    tx, err := bp.db.BeginTxx(ctx, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to begin transaction: %w", err)
    }
    
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p)
        } else if err != nil {
            tx.Rollback()
        } else {
            err = tx.Commit()
        }
    }()
    
    result := &BatchResult{
        Processed: 0,
        Errors:    make([]BatchError, 0),
    }
    
    now := time.Now()
    
    for i, user := range users {
        var operationErr error
        
        if user.ID == 0 {
            // INSERT操作
            user.CreatedAt = now
            user.UpdatedAt = now
            _, operationErr = tx.NamedStmt(bp.insertStmt).Exec(user)
        } else {
            // UPDATE操作
            user.UpdatedAt = now
            _, operationErr = tx.NamedStmt(bp.updateStmt).Exec(user)
        }
        
        if operationErr != nil {
            result.Errors = append(result.Errors, BatchError{
                Index: i,
                User:  user,
                Error: operationErr,
            })
        } else {
            result.Processed++
        }
    }
    
    return result, nil
}

type UserBatchItem struct {
    ID        int       `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    Age       int       `db:"age"`
    CreatedAt time.Time `db:"created_at"`
    UpdatedAt time.Time `db:"updated_at"`
}

type BatchResult struct {
    Processed int
    Errors    []BatchError
}

type BatchError struct {
    Index int
    User  UserBatchItem
    Error error
}
```
### プロダクション環境でのsqlx最適化

#### 1. コネクションプールとsqlx

```go
import (
    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
)

func InitDatabase(config DatabaseConfig) (*sqlx.DB, error) {
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode)
    
    db, err := sqlx.Open("postgres", dsn)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    // コネクションプール設定の最適化
    db.SetMaxOpenConns(config.MaxOpenConns)    // 最大同時接続数
    db.SetMaxIdleConns(config.MaxIdleConns)    // 最大アイドル接続数
    db.SetConnMaxLifetime(config.ConnMaxLifetime) // 接続の最大生存時間
    db.SetConnMaxIdleTime(config.ConnMaxIdleTime) // アイドル接続の最大時間
    
    // 接続テスト
    if err := db.Ping(); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }
    
    return db, nil
}

type DatabaseConfig struct {
    Host            string
    Port            int
    User            string
    Password        string
    DBName          string
    SSLMode         string
    MaxOpenConns    int
    MaxIdleConns    int
    ConnMaxLifetime time.Duration
    ConnMaxIdleTime time.Duration
}
```

#### 2. クエリパフォーマンスの最適化

```go
type OptimizedUserRepository struct {
    db           *sqlx.DB
    getStmt      *sqlx.Stmt
    getUsersStmt *sqlx.NamedStmt
    statsCache   *sync.Map
}

func NewOptimizedUserRepository(db *sqlx.DB) (*OptimizedUserRepository, error) {
    // 頻繁に使用されるクエリをプリペア
    getStmt, err := db.Preparex(`
        SELECT id, name, email, age, created_at, updated_at, is_active
        FROM users WHERE id = $1
    `)
    if err != nil {
        return nil, fmt.Errorf("failed to prepare get statement: %w", err)
    }
    
    getUsersStmt, err := db.PrepareNamed(`
        SELECT id, name, email, age, created_at, updated_at, is_active
        FROM users 
        WHERE created_at > :from_date 
        AND is_active = :is_active
        ORDER BY created_at DESC
        LIMIT :limit OFFSET :offset
    `)
    if err != nil {
        getStmt.Close()
        return nil, fmt.Errorf("failed to prepare getUsers statement: %w", err)
    }
    
    return &OptimizedUserRepository{
        db:           db,
        getStmt:      getStmt,
        getUsersStmt: getUsersStmt,
        statsCache:   &sync.Map{},
    }, nil
}

func (r *OptimizedUserRepository) GetUser(ctx context.Context, id int) (*User, error) {
    var user User
    err := r.getStmt.GetContext(ctx, &user, id)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, ErrUserNotFound
        }
        return nil, fmt.Errorf("failed to get user %d: %w", id, err)
    }
    return &user, nil
}

func (r *OptimizedUserRepository) GetUsersPaginated(ctx context.Context, req PaginationRequest) (*PaginatedUsers, error) {
    params := map[string]interface{}{
        "from_date": req.FromDate,
        "is_active": req.IsActive,
        "limit":     req.Limit,
        "offset":    req.Offset,
    }
    
    var users []User
    err := r.getUsersStmt.SelectContext(ctx, &users, params)
    if err != nil {
        return nil, fmt.Errorf("failed to get users: %w", err)
    }
    
    // 総数をキャッシュから取得または計算
    total, err := r.getTotalCount(ctx, req.FromDate, req.IsActive)
    if err != nil {
        return nil, fmt.Errorf("failed to get total count: %w", err)
    }
    
    return &PaginatedUsers{
        Users:      users,
        Total:      total,
        Page:       req.Offset/req.Limit + 1,
        PerPage:    req.Limit,
        TotalPages: (total + req.Limit - 1) / req.Limit,
    }, nil
}

// 統計情報のキャッシュ付き取得
func (r *OptimizedUserRepository) getTotalCount(ctx context.Context, fromDate time.Time, isActive bool) (int, error) {
    cacheKey := fmt.Sprintf("total_count_%s_%t", fromDate.Format("2006-01-02"), isActive)
    
    if cached, ok := r.statsCache.Load(cacheKey); ok {
        if entry, ok := cached.(*CacheEntry); ok && time.Since(entry.Timestamp) < 5*time.Minute {
            return entry.Count, nil
        }
    }
    
    var count int
    query := `SELECT COUNT(*) FROM users WHERE created_at > $1 AND is_active = $2`
    err := r.db.GetContext(ctx, &count, query, fromDate, isActive)
    if err != nil {
        return 0, err
    }
    
    r.statsCache.Store(cacheKey, &CacheEntry{
        Count:     count,
        Timestamp: time.Now(),
    })
    
    return count, nil
}

type CacheEntry struct {
    Count     int
    Timestamp time.Time
}

type PaginationRequest struct {
    FromDate time.Time
    IsActive bool
    Limit    int
    Offset   int
}

type PaginatedUsers struct {
    Users      []User `json:"users"`
    Total      int    `json:"total"`
    Page       int    `json:"page"`
    PerPage    int    `json:"per_page"`
    TotalPages int    `json:"total_pages"`
}
```

#### 3. エラーハンドリングとリトライ機能

```go
type ResilientUserService struct {
    repo    *OptimizedUserRepository
    retrier *Retrier
    metrics *Metrics
}

type Retrier struct {
    maxRetries int
    baseDelay  time.Duration
}

func (r *Retrier) Execute(ctx context.Context, operation func() error) error {
    var lastErr error
    
    for attempt := 0; attempt <= r.maxRetries; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // リトライ可能なエラーかチェック
        if !isRetryableError(err) {
            return err
        }
        
        if attempt < r.maxRetries {
            delay := r.baseDelay * time.Duration(1<<attempt) // 指数バックオフ
            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }
    
    return fmt.Errorf("operation failed after %d retries: %w", r.maxRetries, lastErr)
}

func isRetryableError(err error) bool {
    // PostgreSQLの一時的エラーをチェック
    if pgErr, ok := err.(*pq.Error); ok {
        switch pgErr.Code {
        case "53300": // too_many_connections
        case "53400": // configuration_limit_exceeded
        case "08000": // connection_exception
        case "08003": // connection_does_not_exist
        case "08006": // connection_failure
            return true
        }
    }
    
    // ネットワークエラーやタイムアウトもリトライ対象
    if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
        return true
    }
    
    if errors.Is(err, context.DeadlineExceeded) {
        return true
    }
    
    return false
}

func (s *ResilientUserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    var user *User
    var err error
    
    start := time.Now()
    defer func() {
        s.metrics.RecordOperation("create_user", time.Since(start), err == nil)
    }()
    
    err = s.retrier.Execute(ctx, func() error {
        user, err = s.repo.CreateUserWithProfile(ctx, req)
        return err
    })
    
    if err != nil {
        s.metrics.RecordError("create_user", err)
        return nil, fmt.Errorf("failed to create user: %w", err)
    }
    
    return user, nil
}

type Metrics struct {
    operationDurations *prometheus.HistogramVec
    operationCounts    *prometheus.CounterVec
    errorCounts        *prometheus.CounterVec
}

func (m *Metrics) RecordOperation(operation string, duration time.Duration, success bool) {
    status := "success"
    if !success {
        status = "error"
    }
    
    m.operationDurations.WithLabelValues(operation, status).Observe(duration.Seconds())
    m.operationCounts.WithLabelValues(operation, status).Inc()
}

func (m *Metrics) RecordError(operation string, err error) {
    errorType := "unknown"
    if pgErr, ok := err.(*pq.Error); ok {
        errorType = string(pgErr.Code)
    } else if errors.Is(err, sql.ErrNoRows) {
        errorType = "not_found"
    } else if errors.Is(err, context.DeadlineExceeded) {
        errorType = "timeout"
    }
    
    m.errorCounts.WithLabelValues(operation, errorType).Inc()
}
```

📝 **課題**

以下の機能を持つsqlx活用システムを実装してください：

1. **`UserRepository`**: 基本的なCRUD操作のsqlx実装
2. **`QueryBuilder`**: 動的クエリ生成システム
3. **`BatchProcessor`**: 大量データの効率的な処理
4. **`TransactionManager`**: 複雑なトランザクション管理
5. **`PerformanceMonitor`**: クエリパフォーマンス監視
6. **統合テスト**: 実際のデータベースを使用したテストスイート

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestUserRepository_CRUD
--- PASS: TestUserRepository_CRUD (0.05s)
=== RUN   TestQueryBuilder_DynamicQuery
--- PASS: TestQueryBuilder_DynamicQuery (0.03s)
=== RUN   TestBatchProcessor_BulkOperations
--- PASS: TestBatchProcessor_BulkOperations (0.10s)
=== RUN   TestTransactionManager_ComplexTransaction
--- PASS: TestTransactionManager_ComplexTransaction (0.08s)
=== RUN   TestPerformanceMonitor_QueryAnalysis
--- PASS: TestPerformanceMonitor_QueryAnalysis (0.12s)
PASS
ok      day39-sqlx    0.380s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **sqlxライブラリ**: 構造体マッピングと名前付きパラメータ
2. **PostgreSQLドライバ**: 配列やJSONBの効率的な処理
3. **プリペアードステートメント**: 繰り返し実行されるクエリの最適化
4. **IN句の展開**: sqlx.Inによる動的配列クエリ
5. **エラーハンドリング**: データベース固有のエラー分類と処理

設定のポイント：
- **構造体タグ**: dbタグによるフィールドマッピング
- **名前付きクエリ**: 可読性と保守性の向上
- **バッチ処理**: 大量データの効率的な処理
- **パフォーマンス監視**: クエリ実行時間とリソース使用量の追跡

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```

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