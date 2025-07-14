# Day 0: Go道場入門前準備 - 必要な基礎知識の確認

## 🎯 本日の目標 (Today's Goal)

Go道場60日間のカリキュラムを効果的に学習するために必要な基礎知識とスキルを確認し、不足している部分を補強する。プロフェッショナルレベルのGo開発を行うための土台を固める。

## 📚 前提知識チェックリスト (Prerequisites Checklist)

### ✅ 基本的なGo言語知識

以下の項目について理解していることを確認してください：

#### 1. Go言語の基本文法

```go
// 変数宣言と型
var name string = "Go Developer"
age := 30
var isActive bool = true

// 関数定義
func calculateSum(a, b int) int {
    return a + b
}

// 構造体
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// メソッド
func (u *User) GetDisplayName() string {
    return fmt.Sprintf("%s (%s)", u.Name, u.Email)
}

// インターフェース
type Writer interface {
    Write([]byte) (int, error)
}

// エラーハンドリング
func readFile(filename string) ([]byte, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %w", err)
    }
    return data, nil
}
```

#### 2. Go Modules理解

```bash
# モジュール初期化
go mod init myproject

# 依存関係追加
go get github.com/gorilla/mux@v1.8.0

# 依存関係整理
go mod tidy

# ベンダリング
go mod vendor
```

#### 3. 基本的な並行処理

```go
// Goroutine
go func() {
    fmt.Println("Hello from goroutine")
}()

// Channel
ch := make(chan string, 1)
ch <- "message"
msg := <-ch

// Select文
select {
case msg := <-ch:
    fmt.Println("Received:", msg)
case <-time.After(1 * time.Second):
    fmt.Println("Timeout")
}

// WaitGroup
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // 処理
}()
wg.Wait()
```

### ✅ 開発環境とツール

#### 1. 必要なソフトウェア

**Go言語環境**
```bash
# Go 1.21以上のインストール確認
go version

# GOPATH, GOROOT確認
go env GOPATH
go env GOROOT
```

**エディタ/IDE**
- VS Code with Go extension
- GoLand
- Vim/Neovim with vim-go

**データベース**
```bash
# PostgreSQL
sudo apt-get install postgresql postgresql-contrib

# Redis
sudo apt-get install redis-server

# Docker (推奨)
docker --version
docker-compose --version
```

#### 2. 開発ツール

```bash
# 静的解析
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/tools/cmd/goimports@latest

# テスト関連
go install github.com/onsi/ginkgo/v2/ginkgo@latest
go install gotest.tools/gotestsum@latest

# モック生成
go install github.com/golang/mock/mockgen@latest

# プロファイリング
go install github.com/google/pprof@latest
```

### ✅ ネットワーク・プロトコル知識

#### 1. HTTP/HTTPSの理解

```go
// 基本的なHTTPサーバー
func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Hello, World!"))
    })
    
    log.Fatal(http.ListenAndServe(":8080", nil))
}

// HTTPクライアント
func makeRequest() error {
    resp, err := http.Get("https://api.example.com/data")
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    
    fmt.Println(string(body))
    return nil
}
```

#### 2. JSON処理

```go
type Person struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

// JSON エンコード
person := Person{Name: "Alice", Age: 30}
data, err := json.Marshal(person)

// JSON デコード
var person Person
err := json.Unmarshal(data, &person)
```

### ✅ データベース基礎

#### 1. SQL基本操作

```sql
-- テーブル作成
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- データ操作
INSERT INTO users (name, email) VALUES ('John Doe', 'john@example.com');
SELECT * FROM users WHERE email = 'john@example.com';
UPDATE users SET name = 'Jane Doe' WHERE id = 1;
DELETE FROM users WHERE id = 1;

-- JOINとトランザクション
BEGIN;
INSERT INTO orders (user_id, total) VALUES (1, 100.00);
INSERT INTO order_items (order_id, product_id, quantity) VALUES (1, 1, 2);
COMMIT;
```

#### 2. Goでのデータベース操作

```go
import (
    "database/sql"
    _ "github.com/lib/pq"
)

func connectDB() (*sql.DB, error) {
    db, err := sql.Open("postgres", "postgres://user:pass@localhost/dbname?sslmode=disable")
    if err != nil {
        return nil, err
    }
    
    if err = db.Ping(); err != nil {
        return nil, err
    }
    
    return db, nil
}

func getUser(db *sql.DB, id int) (*User, error) {
    query := "SELECT id, name, email FROM users WHERE id = $1"
    row := db.QueryRow(query, id)
    
    var user User
    err := row.Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        return nil, err
    }
    
    return &user, nil
}
```

### ✅ テスト知識

#### 1. 基本的なテスト

```go
func TestCalculateSum(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive numbers", 2, 3, 5},
        {"with zero", 0, 5, 5},
        {"negative numbers", -2, -3, -5},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := calculateSum(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("calculateSum(%d, %d) = %d; want %d", 
                    tt.a, tt.b, result, tt.expected)
            }
        })
    }
}
```

#### 2. ベンチマークテスト

```go
func BenchmarkCalculateSum(b *testing.B) {
    for i := 0; i < b.N; i++ {
        calculateSum(1000, 2000)
    }
}
```

### ✅ Git・バージョン管理

```bash
# 基本的なGitコマンド
git init
git add .
git commit -m "Initial commit"
git branch feature/new-feature
git checkout feature/new-feature
git merge main
git rebase main

# リモートリポジトリ
git remote add origin https://github.com/user/repo.git
git push origin main
git pull origin main
```

## 🔧 環境構築手順 (Environment Setup)

### 1. Go言語のインストール

**macOS (Homebrew)**
```bash
brew install go
```

**Ubuntu/Debian**
```bash
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

**Windows**
[https://golang.org/dl/](https://golang.org/dl/) からインストーラーをダウンロード

### 2. ワークスペース設定

```bash
# プロジェクトディレクトリ作成
mkdir -p ~/go-workspace/go-dojo
cd ~/go-workspace/go-dojo

# Go Modules初期化
go mod init go-dojo

# 基本的なディレクトリ構造
mkdir -p {cmd,internal,pkg,configs,scripts,docs}
```

### 3. 推奨VS Code拡張機能

```json
{
    "recommendations": [
        "golang.go",
        "ms-vscode.vscode-json",
        "redhat.vscode-yaml",
        "ms-vscode.test-adapter-converter",
        "streetsidesoftware.code-spell-checker"
    ]
}
```

### 4. Docker環境準備

```yaml
# docker-compose.yml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: go_dojo
      POSTGRES_USER: developer
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

## 📝 準備確認テスト (Readiness Assessment)

### 実践課題 1: 基本的なWebAPI

以下の仕様でHTTP APIを実装してください：

```go
// ユーザー管理API
// GET /users - ユーザー一覧取得
// POST /users - ユーザー作成
// GET /users/{id} - 特定ユーザー取得
// PUT /users/{id} - ユーザー更新
// DELETE /users/{id} - ユーザー削除

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// 要件:
// 1. メモリ内でのデータ保存（スライス使用）
// 2. 適切なHTTPステータスコード
// 3. JSON形式のレスポンス
// 4. エラーハンドリング
// 5. 基本的なバリデーション
```

**期待される実装例:**

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strconv"
    "strings"
    "sync"
)

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type UserStore struct {
    mu    sync.RWMutex
    users map[int]*User
    nextID int
}

func NewUserStore() *UserStore {
    return &UserStore{
        users: make(map[int]*User),
        nextID: 1,
    }
}

func (s *UserStore) CreateUser(user *User) *User {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    user.ID = s.nextID
    s.nextID++
    s.users[user.ID] = user
    return user
}

func (s *UserStore) GetUser(id int) (*User, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    user, exists := s.users[id]
    return user, exists
}

func (s *UserStore) GetAllUsers() []*User {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    users := make([]*User, 0, len(s.users))
    for _, user := range s.users {
        users = append(users, user)
    }
    return users
}

func (s *UserStore) UpdateUser(id int, user *User) (*User, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if _, exists := s.users[id]; !exists {
        return nil, false
    }
    
    user.ID = id
    s.users[id] = user
    return user, true
}

func (s *UserStore) DeleteUser(id int) bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if _, exists := s.users[id]; !exists {
        return false
    }
    
    delete(s.users, id)
    return true
}

type UserHandler struct {
    store *UserStore
}

func NewUserHandler() *UserHandler {
    return &UserHandler{
        store: NewUserStore(),
    }
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    path := strings.TrimPrefix(r.URL.Path, "/users")
    
    switch r.Method {
    case http.MethodGet:
        if path == "" || path == "/" {
            h.handleGetUsers(w, r)
        } else {
            h.handleGetUser(w, r, path)
        }
    case http.MethodPost:
        h.handleCreateUser(w, r)
    case http.MethodPut:
        h.handleUpdateUser(w, r, path)
    case http.MethodDelete:
        h.handleDeleteUser(w, r, path)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (h *UserHandler) handleGetUsers(w http.ResponseWriter, r *http.Request) {
    users := h.store.GetAllUsers()
    json.NewEncoder(w).Encode(users)
}

func (h *UserHandler) handleGetUser(w http.ResponseWriter, r *http.Request, path string) {
    id, err := strconv.Atoi(strings.Trim(path, "/"))
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }
    
    user, exists := h.store.GetUser(id)
    if !exists {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }
    
    json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
    var user User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    if user.Name == "" || user.Email == "" {
        http.Error(w, "Name and email are required", http.StatusBadRequest)
        return
    }
    
    createdUser := h.store.CreateUser(&user)
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(createdUser)
}

func (h *UserHandler) handleUpdateUser(w http.ResponseWriter, r *http.Request, path string) {
    id, err := strconv.Atoi(strings.Trim(path, "/"))
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }
    
    var user User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    if user.Name == "" || user.Email == "" {
        http.Error(w, "Name and email are required", http.StatusBadRequest)
        return
    }
    
    updatedUser, exists := h.store.UpdateUser(id, &user)
    if !exists {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }
    
    json.NewEncoder(w).Encode(updatedUser)
}

func (h *UserHandler) handleDeleteUser(w http.ResponseWriter, r *http.Request, path string) {
    id, err := strconv.Atoi(strings.Trim(path, "/"))
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }
    
    if !h.store.DeleteUser(id) {
        http.Error(w, "User not found", http.StatusNotFound)
        return
    }
    
    w.WriteHeader(http.StatusNoContent)
}

func main() {
    handler := NewUserHandler()
    http.Handle("/users", handler)
    http.Handle("/users/", handler)
    
    fmt.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### 実践課題 2: 並行処理

以下の仕様で並行処理プログラムを作成してください：

```go
// 要件:
// 1. 複数のURLに対して並行してHTTPリクエストを送信
// 2. 全てのレスポンスを待つ
// 3. 結果をまとめて返す
// 4. タイムアウト処理（5秒）
// 5. エラーハンドリング

type Result struct {
    URL      string
    Status   int
    Duration time.Duration
    Error    error
}

func fetchURLs(urls []string) []Result {
    // 実装してください
}
```

### 実践課題 3: テスト作成

課題1で作成したAPIに対して包括的なテストを作成してください：

```go
// 要件:
// 1. 各エンドポイントのテスト
// 2. 正常系とエラー系のテストケース
// 3. テーブル駆動テスト
// 4. httptest パッケージの使用
// 5. 適切なアサーション
```

## 📋 評価基準 (Assessment Criteria)

### ✅ 必須レベル（Go道場開始可能）

- [ ] 基本的なGo文法の理解（変数、関数、構造体、インターフェース）
- [ ] エラーハンドリングの理解
- [ ] 基本的な並行処理（goroutine、channel）
- [ ] HTTP API作成の基礎
- [ ] JSON処理
- [ ] 基本的なテスト作成
- [ ] Git操作の基礎

### ⭐ 推奨レベル（より効果的な学習）

- [ ] データベース操作の基礎
- [ ] ミドルウェアパターンの理解
- [ ] コンテキスト（context.Context）の使用
- [ ] Docker基礎知識
- [ ] 依存関係管理（Go Modules）
- [ ] 静的解析ツールの使用

### 🚀 理想レベル（高度な学習成果）

- [ ] 設計パターンの理解
- [ ] パフォーマンス測定
- [ ] セキュリティ基礎
- [ ] CI/CD基礎知識
- [ ] マイクロサービス概念

## 🔄 不足部分の学習リソース (Learning Resources)

### Go言語基礎が不足している場合

**推奨書籍:**
- 「プログラミング言語Go」（Alan Donovan, Brian Kernighan）
- 「Go言語による並行処理」（Katherine Cox-Buday）

**オンラインリソース:**
- [A Tour of Go](https://tour.golang.org/)
- [Go by Example](https://gobyexample.com/)
- [Effective Go](https://golang.org/doc/effective_go.html)

### HTTP・Web開発が不足している場合

**学習項目:**
```go
// HTTP基礎
- HTTPメソッド（GET, POST, PUT, DELETE）
- ステータスコード
- ヘッダー
- リクエスト/レスポンスボディ

// Go での Web開発
- net/http パッケージ
- ルーティング
- ミドルウェア
- テンプレート
```

### データベースが不足している場合

**学習項目:**
```sql
-- SQL基礎
SELECT, INSERT, UPDATE, DELETE
JOIN（INNER, LEFT, RIGHT）
トランザクション（BEGIN, COMMIT, ROLLBACK）
インデックス
制約（PRIMARY KEY, FOREIGN KEY, UNIQUE）
```

```go
// Go でのDB操作
database/sql パッケージ
準備済みステートメント
トランザクション処理
```

## 🎯 準備完了の判断基準 (Readiness Criteria)

以下の条件を満たしていればGo道場の学習を開始できます：

### 最低基準
✅ 実践課題1（基本的なWebAPI）を独力で実装できる
✅ 基本的なエラーハンドリングができる  
✅ Goroutineとチャネルの基本的な使用ができる
✅ JSONの処理ができる

### 推奨基準
✅ 実践課題2（並行処理）を実装できる
✅ 実践課題3（テスト作成）を実装できる
✅ データベースの基本操作ができる
✅ HTTP APIの設計思想を理解している

## 🚀 次のステップ (Next Steps)

準備が完了したら、[Day 1: Context Based Cancellation](../day01-context-cancellation/README.md) から Go道場の学習を開始してください。

60日間の充実した学習の旅が始まります！

---

**💡 重要なお知らせ**

不明な点や追加の質問がある場合は、各日の学習と並行して基礎知識を補強していくことも可能です。完璧な準備を待つよりも、学習を始めながら必要に応じて基礎に戻ることも効果的な学習方法です。