# Day 27: `dockertest`による統合テスト

🎯 **本日の目標**
`dockertest`ライブラリを使用して実際のPostgreSQLコンテナを起動し、データベースと連携するWebAPIの本格的なエンドツーエンドテストを実装できるようになる。

## 📖 解説

### dockertestとは

`dockertest`は、Go言語のテストコードから直接Dockerコンテナを起動・管理できるライブラリです。統合テストで実際のデータベースやRedis、Message Queueなどの外部依存を使用する際に非常に有用です：

```go
import "github.com/ory/dockertest/v3"

pool, err := dockertest.NewPool("")
if err != nil {
    log.Fatalf("Could not create pool: %s", err)
}

resource, err := pool.Run("postgres", "13", []string{
    "POSTGRES_PASSWORD=secret",
    "POSTGRES_DB=testdb",
})
```

### 統合テストの重要性

単体テストではモックを使用しますが、統合テストでは実際の依存システムを使用します：

- **単体テスト**: 各関数・メソッドの動作を検証
- **統合テスト**: システム全体の連携を検証
- **E2Eテスト**: ユーザーの操作フローを検証

```go
// 単体テスト（モック使用）
func TestUserService_GetUser(t *testing.T) {
    mockDB := &MockDatabase{}
    mockDB.On("FindUser", 1).Return(&User{ID: 1, Name: "Test"}, nil)
    
    service := NewUserService(mockDB)
    user, err := service.GetUser(1)
    
    assert.NoError(t, err)
    assert.Equal(t, "Test", user.Name)
}

// 統合テスト（実際のDB使用）
func TestUserAPI_Integration(t *testing.T) {
    db := setupTestDatabase(t)
    defer db.Close()
    
    server := setupTestServer(db)
    defer server.Close()
    
    resp, err := http.Post(server.URL+"/users", "application/json", 
        strings.NewReader(`{"name": "Test User"}`))
    assert.NoError(t, err)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
```

### PostgreSQLコンテナの起動

dockertestを使用してPostgreSQLコンテナを起動し、接続を確立します：

```go
func setupTestDB(t *testing.T) (*sql.DB, func()) {
    pool, err := dockertest.NewPool("")
    require.NoError(t, err)
    
    resource, err := pool.Run("postgres", "13", []string{
        "POSTGRES_PASSWORD=testpass",
        "POSTGRES_DB=testdb",
        "POSTGRES_USER=testuser",
    })
    require.NoError(t, err)
    
    // コンテナの起動を待機
    var db *sql.DB
    err = pool.Retry(func() error {
        var err error
        db, err = sql.Open("postgres", fmt.Sprintf(
            "postgres://testuser:testpass@localhost:%s/testdb?sslmode=disable",
            resource.GetPort("5432/tcp")))
        if err != nil {
            return err
        }
        return db.Ping()
    })
    require.NoError(t, err)
    
    // クリーンアップ関数を返す
    cleanup := func() {
        db.Close()
        pool.Purge(resource)
    }
    
    return db, cleanup
}
```

### データベーススキーマの初期化

テスト実行前にテーブル作成とテストデータの投入を行います：

```go
func initSchema(db *sql.DB) error {
    schema := `
    CREATE TABLE IF NOT EXISTS users (
        id SERIAL PRIMARY KEY,
        name VARCHAR(100) NOT NULL,
        email VARCHAR(100) UNIQUE NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    
    CREATE TABLE IF NOT EXISTS posts (
        id SERIAL PRIMARY KEY,
        user_id INTEGER REFERENCES users(id),
        title VARCHAR(200) NOT NULL,
        content TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`
    
    _, err := db.Exec(schema)
    return err
}

func seedTestData(db *sql.DB) error {
    users := []struct {
        name, email string
    }{
        {"Alice", "alice@example.com"},
        {"Bob", "bob@example.com"},
    }
    
    for _, user := range users {
        _, err := db.Exec(
            "INSERT INTO users (name, email) VALUES ($1, $2)",
            user.name, user.email)
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### HTTPサーバーのテスト起動

テスト用のHTTPサーバーを起動し、実際のHTTPリクエストでテストします：

```go
func setupTestServer(db *sql.DB) *httptest.Server {
    userRepo := NewUserRepository(db)
    userService := NewUserService(userRepo)
    handler := NewUserHandler(userService)
    
    mux := http.NewServeMux()
    mux.HandleFunc("/users", handler.CreateUser)
    mux.HandleFunc("/users/", handler.GetUser)
    
    return httptest.NewServer(mux)
}

func TestUserAPI_CreateAndGet(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()
    
    require.NoError(t, initSchema(db))
    
    server := setupTestServer(db)
    defer server.Close()
    
    // ユーザー作成
    payload := `{"name": "Test User", "email": "test@example.com"}`
    resp, err := http.Post(server.URL+"/users", "application/json", 
        strings.NewReader(payload))
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
    
    var created map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&created)
    require.NoError(t, err)
    
    userID := int(created["id"].(float64))
    
    // ユーザー取得
    resp, err = http.Get(fmt.Sprintf("%s/users/%d", server.URL, userID))
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    
    var retrieved map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&retrieved)
    require.NoError(t, err)
    
    assert.Equal(t, "Test User", retrieved["name"])
    assert.Equal(t, "test@example.com", retrieved["email"])
}
```

### テストの並列実行

複数のテストケースを並列実行する際は、各テストで独立したデータベースを使用します：

```go
func TestUserAPI_Parallel(t *testing.T) {
    tests := []struct {
        name     string
        testFunc func(t *testing.T, server *httptest.Server)
    }{
        {"CreateUser", testCreateUser},
        {"GetUser", testGetUser},
        {"UpdateUser", testUpdateUser},
        {"DeleteUser", testDeleteUser},
    }
    
    for _, tt := range tests {
        tt := tt // capture loop variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // 並列実行を有効化
            
            db, cleanup := setupTestDB(t)
            defer cleanup()
            
            require.NoError(t, initSchema(db))
            require.NoError(t, seedTestData(db))
            
            server := setupTestServer(db)
            defer server.Close()
            
            tt.testFunc(t, server)
        })
    }
}
```

### トランザクションテスト

データベーストランザクションの動作を検証します：

```go
func TestUserService_Transaction(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()
    
    require.NoError(t, initSchema(db))
    
    service := NewUserService(NewUserRepository(db))
    
    // トランザクション内でエラーが発生した場合のロールバックテスト
    err := service.CreateUserWithPosts(&User{
        Name:  "Test User",
        Email: "invalid-email", // バリデーションエラーを意図的に発生
    }, []Post{
        {Title: "Post 1", Content: "Content 1"},
    })
    
    assert.Error(t, err)
    
    // ユーザーが作成されていないことを確認
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM users WHERE name = 'Test User'").Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 0, count)
}
```

### クリーンアップの重要性

テスト実行後は必ずリソースをクリーンアップします：

```go
func TestMain(m *testing.M) {
    // テスト実行前の準備
    pool, err := dockertest.NewPool("")
    if err != nil {
        log.Fatalf("Could not create pool: %s", err)
    }
    
    // テスト実行
    code := m.Run()
    
    // 全体的なクリーンアップ
    // 残ったコンテナがあれば削除
    
    os.Exit(code)
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **データベース接続**
   - PostgreSQLコンテナとの接続確立
   - 接続プールの適切な管理

2. **ユーザー管理API**
   - ユーザーの作成、取得、更新、削除
   - バリデーション処理とエラーハンドリング

3. **投稿管理API**
   - 投稿の作成、取得、一覧表示
   - ユーザーとの関連性管理

4. **トランザクション処理**
   - 複数のデータ操作を一括実行
   - エラー時の適切なロールバック

5. **テストヘルパー**
   - dockertestによるDB起動
   - スキーマ初期化とテストデータ投入
   - HTTPサーバーのテスト起動

## ✅ 期待される挙動

### ユーザー作成API
```bash
POST /api/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com"
}

Response:
HTTP/1.1 201 Created
Content-Type: application/json

{
  "id": 1,
  "name": "John Doe", 
  "email": "john@example.com",
  "created_at": "2024-01-15T10:30:00Z"
}
```

### ユーザー取得API
```bash
GET /api/users/1

Response:
HTTP/1.1 200 OK
Content-Type: application/json

{
  "id": 1,
  "name": "John Doe",
  "email": "john@example.com",
  "created_at": "2024-01-15T10:30:00Z",
  "posts": [
    {
      "id": 1,
      "title": "First Post",
      "content": "Hello World",
      "created_at": "2024-01-15T11:00:00Z"
    }
  ]
}
```

### エラーレスポンス
```bash
POST /api/users
Content-Type: application/json

{
  "name": "",
  "email": "invalid-email"
}

Response:
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "validation failed",
  "details": {
    "name": "name is required",
    "email": "invalid email format"
  }
}
```

## 💡 ヒント

1. **dockertest.NewPool()**: Dockerプールの作成
2. **pool.Run()**: コンテナの起動
3. **pool.Retry()**: 接続試行のリトライ
4. **pool.Purge()**: コンテナの削除
5. **sql.Open()**: データベース接続
6. **httptest.NewServer()**: テスト用HTTPサーバー
7. **t.Parallel()**: テストの並列実行
8. **t.Cleanup()**: テスト終了時の自動クリーンアップ

### dockertestの最適化

大量のテストを実行する際のパフォーマンス向上：

```go
var (
    testDB   *sql.DB
    testPool *dockertest.Pool
    testResource *dockertest.Resource
)

func TestMain(m *testing.M) {
    var err error
    testPool, err = dockertest.NewPool("")
    if err != nil {
        log.Fatalf("Could not create pool: %s", err)
    }
    
    // 全テストで共有するDBコンテナを起動
    testResource, err = testPool.Run("postgres", "13", []string{
        "POSTGRES_PASSWORD=testpass",
        "POSTGRES_DB=testdb", 
        "POSTGRES_USER=testuser",
    })
    if err != nil {
        log.Fatalf("Could not start resource: %s", err)
    }
    
    // 接続確立
    if err = testPool.Retry(func() error {
        var err error
        testDB, err = sql.Open("postgres", fmt.Sprintf(
            "postgres://testuser:testpass@localhost:%s/testdb?sslmode=disable",
            testResource.GetPort("5432/tcp")))
        if err != nil {
            return err
        }
        return testDB.Ping()
    }); err != nil {
        log.Fatalf("Could not connect to database: %s", err)
    }
    
    code := m.Run()
    
    // クリーンアップ
    testDB.Close()
    testPool.Purge(testResource)
    
    os.Exit(code)
}
```

### テストデータの分離

並列テストでのデータ競合を防ぐため、テストごとに独立したデータセットを使用：

```go
func createTestUser(t *testing.T, db *sql.DB) *User {
    user := &User{
        Name:  fmt.Sprintf("test-user-%d", time.Now().UnixNano()),
        Email: fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
    }
    
    err := db.QueryRow(
        "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, created_at",
        user.Name, user.Email).Scan(&user.ID, &user.CreatedAt)
    require.NoError(t, err)
    
    return user
}
```

これらの実装により、本格的な統合テスト環境を構築し、データベースと連携するWebAPIの品質を保証できます。