# Day 35: N+1問題の解決

## 🎯 本日の目標 (Today's Goal)

データベースアクセスにおける最も深刻なパフォーマンス問題の一つである「N+1問題」を理解し、効果的な解決手法を習得する。Eager Loading、Batch Loading、DataLoaderパターンなどの技術を駆使して、大規模アプリケーションでも高速なデータアクセスを実現できるようになる。

## 📖 解説 (Explanation)

### N+1問題とは？

N+1問題は、関連データを取得する際に発生する典型的なパフォーマンス問題です。「N個のメインデータを取得するために、1つの初期クエリ + N個の追加クエリ」が実行されることから、この名前がついています。

#### 問題のあるコード例

```go
// ユーザー一覧とその投稿を表示する例
type User struct {
    ID    int    `db:"id"`
    Name  string `db:"name"`
    Email string `db:"email"`
}

type Post struct {
    ID       int    `db:"id"`
    UserID   int    `db:"user_id"`
    Title    string `db:"title"`
    Content  string `db:"content"`
}

// N+1問題が発生する悪い実装
func GetUsersWithPostsBadly(db *sql.DB) ([]UserWithPosts, error) {
    // 1. ユーザー一覧を取得（1回のクエリ）
    users, err := getUserList(db)
    if err != nil {
        return nil, err
    }
    
    var result []UserWithPosts
    for _, user := range users { // Nユーザーに対して
        // 2. 各ユーザーの投稿を個別に取得（N回のクエリ）
        posts, err := getPostsByUserID(db, user.ID)
        if err != nil {
            return nil, err
        }
        
        result = append(result, UserWithPosts{
            User:  user,
            Posts: posts,
        })
    }
    
    return result, nil
}

func getUserList(db *sql.DB) ([]User, error) {
    // SQL: SELECT id, name, email FROM users;
    // この1回のクエリで100ユーザーを取得
    rows, err := db.Query("SELECT id, name, email FROM users")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var users []User
    for rows.Next() {
        var user User
        err := rows.Scan(&user.ID, &user.Name, &user.Email)
        if err != nil {
            return nil, err
        }
        users = append(users, user)
    }
    
    return users, nil
}

func getPostsByUserID(db *sql.DB, userID int) ([]Post, error) {
    // SQL: SELECT id, user_id, title, content FROM posts WHERE user_id = ?;
    // 100ユーザーなら、このクエリが100回実行される！
    rows, err := db.Query("SELECT id, user_id, title, content FROM posts WHERE user_id = $1", userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var posts []Post
    for rows.Next() {
        var post Post
        err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content)
        if err != nil {
            return nil, err
        }
        posts = append(posts, post)
    }
    
    return posts, nil
}
```

**この実装の問題点：**
- 100ユーザーの場合：1つの初期クエリ + 100個の追加クエリ = 合計101回のクエリ
- データベース接続のオーバーヘッドが101回発生
- ネットワークレイテンシが101回積み重なる
- データベースサーバーへの負荷が激増

### Eager Loading（JOIN使用）による解決

最も効果的な解決策の一つが、JOINを使った一括取得です：

```go
type UserWithPosts struct {
    User  User
    Posts []Post
}

// JOINを使った効率的な実装
func GetUsersWithPostsEagerly(db *sql.DB) ([]UserWithPosts, error) {
    // 1回のJOINクエリですべてのデータを取得
    query := `
        SELECT 
            u.id, u.name, u.email,
            p.id, p.user_id, p.title, p.content
        FROM users u
        LEFT JOIN posts p ON u.id = p.user_id
        ORDER BY u.id, p.id
    `
    
    rows, err := db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    return buildUserWithPostsFromRows(rows)
}

func buildUserWithPostsFromRows(rows *sql.Rows) ([]UserWithPosts, error) {
    userMap := make(map[int]*UserWithPosts)
    
    for rows.Next() {
        var (
            userID    int
            userName  string
            userEmail string
            postID    sql.NullInt32
            postUserID sql.NullInt32
            postTitle sql.NullString
            postContent sql.NullString
        )
        
        err := rows.Scan(
            &userID, &userName, &userEmail,
            &postID, &postUserID, &postTitle, &postContent,
        )
        if err != nil {
            return nil, err
        }
        
        // ユーザーがまだマップにない場合は追加
        if _, exists := userMap[userID]; !exists {
            userMap[userID] = &UserWithPosts{
                User: User{
                    ID:    userID,
                    Name:  userName,
                    Email: userEmail,
                },
                Posts: []Post{},
            }
        }
        
        // 投稿が存在する場合は追加
        if postID.Valid {
            post := Post{
                ID:      int(postID.Int32),
                UserID:  int(postUserID.Int32),
                Title:   postTitle.String,
                Content: postContent.String,
            }
            userMap[userID].Posts = append(userMap[userID].Posts, post)
        }
    }
    
    // マップから配列に変換
    var result []UserWithPosts
    for _, userWithPosts := range userMap {
        result = append(result, *userWithPosts)
    }
    
    // ID順でソート
    sort.Slice(result, func(i, j int) bool {
        return result[i].User.ID < result[j].User.ID
    })
    
    return result, nil
}
```

**Eager Loadingの利点：**
- クエリ数：101回 → 1回（99%減少）
- ネットワークラウンドトリップ：101回 → 1回
- データベース接続オーバーヘッド：大幅削減

### Batch Loading（IN句使用）による解決

JOINが複雑になる場合や、柔軟性が必要な場合はBatch Loadingを使用：

```go
// IN句を使ったバッチ読み込み
func GetUsersWithPostsBatch(db *sql.DB) ([]UserWithPosts, error) {
    // 1. ユーザー一覧を取得
    users, err := getUserList(db)
    if err != nil {
        return nil, err
    }
    
    if len(users) == 0 {
        return []UserWithPosts{}, nil
    }
    
    // 2. ユーザーIDを抽出
    userIDs := make([]int, len(users))
    for i, user := range users {
        userIDs[i] = user.ID
    }
    
    // 3. 全ユーザーの投稿を一括取得
    postsMap, err := getPostsByUserIDs(db, userIDs)
    if err != nil {
        return nil, err
    }
    
    // 4. ユーザーと投稿を組み合わせ
    var result []UserWithPosts
    for _, user := range users {
        posts, exists := postsMap[user.ID]
        if !exists {
            posts = []Post{}
        }
        
        result = append(result, UserWithPosts{
            User:  user,
            Posts: posts,
        })
    }
    
    return result, nil
}

func getPostsByUserIDs(db *sql.DB, userIDs []int) (map[int][]Post, error) {
    if len(userIDs) == 0 {
        return make(map[int][]Post), nil
    }
    
    // IN句のプレースホルダーを動的に生成
    placeholders := make([]string, len(userIDs))
    args := make([]interface{}, len(userIDs))
    for i, id := range userIDs {
        placeholders[i] = fmt.Sprintf("$%d", i+1)
        args[i] = id
    }
    
    query := fmt.Sprintf(`
        SELECT id, user_id, title, content 
        FROM posts 
        WHERE user_id IN (%s)
        ORDER BY user_id, id
    `, strings.Join(placeholders, ","))
    
    rows, err := db.Query(query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    postsMap := make(map[int][]Post)
    for rows.Next() {
        var post Post
        err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content)
        if err != nil {
            return nil, err
        }
        
        postsMap[post.UserID] = append(postsMap[post.UserID], post)
    }
    
    return postsMap, nil
}
```

### DataLoaderパターンの実装

GraphQLで人気のDataLoaderパターンをGoで実装：

```go
import (
    "context"
    "sync"
    "time"
)

type PostLoader struct {
    db           *sql.DB
    wait         time.Duration
    maxBatch     int
    batch        []batchItem
    mu           sync.Mutex
    pendingKeys  map[int][]chan []Post
}

type batchItem struct {
    userID int
    result chan []Post
}

func NewPostLoader(db *sql.DB) *PostLoader {
    return &PostLoader{
        db:          db,
        wait:        10 * time.Millisecond, // バッチ待機時間
        maxBatch:    100,                   // 最大バッチサイズ
        pendingKeys: make(map[int][]chan []Post),
    }
}

func (l *PostLoader) Load(ctx context.Context, userID int) ([]Post, error) {
    resultChan := make(chan []Post, 1)
    
    l.mu.Lock()
    // 既存のリクエストに追加
    l.pendingKeys[userID] = append(l.pendingKeys[userID], resultChan)
    
    // 初回リクエストの場合、バッチ処理をスケジュール
    if len(l.pendingKeys[userID]) == 1 {
        l.scheduleLoad()
    }
    l.mu.Unlock()
    
    // 結果を待機
    select {
    case result := <-resultChan:
        return result, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

func (l *PostLoader) scheduleLoad() {
    go func() {
        time.Sleep(l.wait)
        l.processBatch()
    }()
}

func (l *PostLoader) processBatch() {
    l.mu.Lock()
    currentBatch := l.pendingKeys
    l.pendingKeys = make(map[int][]chan []Post)
    l.mu.Unlock()
    
    if len(currentBatch) == 0 {
        return
    }
    
    // バッチで投稿を取得
    userIDs := make([]int, 0, len(currentBatch))
    for userID := range currentBatch {
        userIDs = append(userIDs, userID)
    }
    
    postsMap, err := getPostsByUserIDs(l.db, userIDs)
    
    // 結果を各リクエストに配信
    for userID, channels := range currentBatch {
        var posts []Post
        if err == nil {
            posts = postsMap[userID]
        }
        
        for _, ch := range channels {
            ch <- posts
        }
    }
}

// DataLoaderを使った実装
func GetUsersWithPostsDataLoader(db *sql.DB) ([]UserWithPosts, error) {
    users, err := getUserList(db)
    if err != nil {
        return nil, err
    }
    
    loader := NewPostLoader(db)
    ctx := context.Background()
    
    var result []UserWithPosts
    for _, user := range users {
        posts, err := loader.Load(ctx, user.ID)
        if err != nil {
            return nil, err
        }
        
        result = append(result, UserWithPosts{
            User:  user,
            Posts: posts,
        })
    }
    
    return result, nil
}
```

### パフォーマンス測定とベンチマーク

各手法のパフォーマンスを測定：

```go
import (
    "testing"
    "time"
)

type QueryCounter struct {
    count int
    mu    sync.Mutex
}

func (qc *QueryCounter) Increment() {
    qc.mu.Lock()
    qc.count++
    qc.mu.Unlock()
}

func (qc *QueryCounter) Count() int {
    qc.mu.Lock()
    defer qc.mu.Unlock()
    return qc.count
}

func BenchmarkNPlusOneProblem(b *testing.B) {
    db := setupTestDB()
    defer db.Close()
    
    // テストデータを挿入（100ユーザー、各5投稿）
    insertTestData(db, 100, 5)
    
    b.Run("BadImplementation", func(b *testing.B) {
        counter := &QueryCounter{}
        dbWithCounter := wrapDBWithCounter(db, counter)
        
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            _, err := GetUsersWithPostsBadly(dbWithCounter)
            if err != nil {
                b.Fatal(err)
            }
        }
        
        b.Logf("Total queries executed: %d", counter.Count())
    })
    
    b.Run("EagerLoading", func(b *testing.B) {
        counter := &QueryCounter{}
        dbWithCounter := wrapDBWithCounter(db, counter)
        
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            _, err := GetUsersWithPostsEagerly(dbWithCounter)
            if err != nil {
                b.Fatal(err)
            }
        }
        
        b.Logf("Total queries executed: %d", counter.Count())
    })
    
    b.Run("BatchLoading", func(b *testing.B) {
        counter := &QueryCounter{}
        dbWithCounter := wrapDBWithCounter(db, counter)
        
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            _, err := GetUsersWithPostsBatch(dbWithCounter)
            if err != nil {
                b.Fatal(err)
            }
        }
        
        b.Logf("Total queries executed: %d", counter.Count())
    })
    
    b.Run("DataLoader", func(b *testing.B) {
        counter := &QueryCounter{}
        dbWithCounter := wrapDBWithCounter(db, counter)
        
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
            _, err := GetUsersWithPostsDataLoader(dbWithCounter)
            if err != nil {
                b.Fatal(err)
            }
        }
        
        b.Logf("Total queries executed: %d", counter.Count())
    })
}

func TestQueryCounts(t *testing.T) {
    db := setupTestDB()
    defer db.Close()
    
    insertTestData(db, 10, 3) // 10ユーザー、各3投稿
    
    tests := []struct {
        name           string
        implementation func(*sql.DB) ([]UserWithPosts, error)
        expectedQueries int
    }{
        {"BadImplementation", GetUsersWithPostsBadly, 11}, // 1 + 10
        {"EagerLoading", GetUsersWithPostsEagerly, 1},
        {"BatchLoading", GetUsersWithPostsBatch, 2}, // users + posts
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            counter := &QueryCounter{}
            dbWithCounter := wrapDBWithCounter(db, counter)
            
            _, err := tt.implementation(dbWithCounter)
            if err != nil {
                t.Fatal(err)
            }
            
            actualQueries := counter.Count()
            if actualQueries != tt.expectedQueries {
                t.Errorf("Expected %d queries, got %d", tt.expectedQueries, actualQueries)
            }
        })
    }
}
```

### 実践的なN+1問題の検出

本番環境でN+1問題を検出するツール：

```go
type QueryTracker struct {
    queries      []QueryInfo
    mu           sync.Mutex
    threshold    int  // N+1問題とみなすクエリ数の閾値
    timeWindow   time.Duration
}

type QueryInfo struct {
    SQL       string
    Args      []interface{}
    Timestamp time.Time
    Duration  time.Duration
}

func NewQueryTracker(threshold int, timeWindow time.Duration) *QueryTracker {
    return &QueryTracker{
        threshold:  threshold,
        timeWindow: timeWindow,
    }
}

func (qt *QueryTracker) TrackQuery(sql string, args []interface{}, duration time.Duration) {
    qt.mu.Lock()
    defer qt.mu.Unlock()
    
    query := QueryInfo{
        SQL:       sql,
        Args:      args,
        Timestamp: time.Now(),
        Duration:  duration,
    }
    
    qt.queries = append(qt.queries, query)
    
    // 古いクエリを削除
    cutoff := time.Now().Add(-qt.timeWindow)
    for i, q := range qt.queries {
        if q.Timestamp.After(cutoff) {
            qt.queries = qt.queries[i:]
            break
        }
    }
    
    qt.detectNPlusOne()
}

func (qt *QueryTracker) detectNPlusOne() {
    patterns := make(map[string]int)
    
    for _, query := range qt.queries {
        // クエリパターンを正規化（パラメータを除去）
        pattern := normalizeQuery(query.SQL)
        patterns[pattern]++
    }
    
    for pattern, count := range patterns {
        if count > qt.threshold {
            log.Printf("Potential N+1 detected: %s executed %d times", pattern, count)
        }
    }
}

func normalizeQuery(sql string) string {
    // パラメータプレースホルダーを正規化
    re := regexp.MustCompile(`\$\d+|\?`)
    return re.ReplaceAllString(sql, "?")
}
```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **N+1問題の再現**: `GetUsersWithPostsBadly(db *sql.DB) ([]UserWithPosts, error)`
2. **Eager Loading実装**: `GetUsersWithPostsEagerly(db *sql.DB) ([]UserWithPosts, error)`
3. **Batch Loading実装**: `GetUsersWithPostsBatch(db *sql.DB) ([]UserWithPosts, error)`
4. **DataLoader実装**: `NewPostLoader(db *sql.DB) *PostLoader`と`Load`メソッド
5. **パフォーマンス測定**: 各手法のクエリ数とレスポンス時間を測定

**重要な実装要件：**
- 正確な結果：すべての手法で同じ結果が得られること
- パフォーマンス改善：Eager/Batch LoadingでN+1問題が解決されること
- エラーハンドリング：データベースエラーを適切に処理すること
- テスタビリティ：クエリ数の測定が可能であること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### ベンチマーク結果例
```bash
$ go test -bench=. -benchmem
BenchmarkNPlusOneProblem/BadImplementation-8    100    15000000 ns/op    101 queries
BenchmarkNPlusOneProblem/EagerLoading-8        2000      500000 ns/op      1 queries  
BenchmarkNPlusOneProblem/BatchLoading-8        1500      800000 ns/op      2 queries
BenchmarkNPlusOneProblem/DataLoader-8          1000     1200000 ns/op      1 queries
```

### テスト実行例
```bash
$ go test -v
=== RUN   TestQueryCounts
=== RUN   TestQueryCounts/BadImplementation
    Executed 11 queries for 10 users (N+1 problem confirmed)
=== RUN   TestQueryCounts/EagerLoading
    Executed 1 query for 10 users (99% reduction!)
=== RUN   TestQueryCounts/BatchLoading
    Executed 2 queries for 10 users (82% reduction!)
--- PASS: TestQueryCounts (0.05s)

=== RUN   TestDataIntegrity
    All implementations return identical results ✓
--- PASS: TestDataIntegrity (0.03s)
PASS
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### JOINクエリの書き方
```go
query := `
    SELECT 
        u.id, u.name, u.email,
        p.id, p.user_id, p.title, p.content
    FROM users u
    LEFT JOIN posts p ON u.id = p.user_id
    ORDER BY u.id, p.id
`
```

### IN句の動的生成
```go
placeholders := make([]string, len(userIDs))
args := make([]interface{}, len(userIDs))
for i, id := range userIDs {
    placeholders[i] = fmt.Sprintf("$%d", i+1)
    args[i] = id
}
query := fmt.Sprintf("SELECT * FROM posts WHERE user_id IN (%s)", 
    strings.Join(placeholders, ","))
```

### DataLoaderのバッチ処理
```go
func (l *PostLoader) Load(ctx context.Context, userID int) ([]Post, error) {
    // 1. リクエストをバッチに追加
    // 2. 一定時間後またはバッチサイズ到達でまとめて処理
    // 3. 結果を各リクエストに配信
}
```

## 実行方法

```bash
# テスト実行
go test -v

# ベンチマーク測定（クエリ数込み）
go test -bench=. -benchmem

# N+1問題の検出
go test -run=TestQueryCounts

# プログラム実行
go run main.go
```

## 参考資料

- [PostgreSQL JOIN Performance](https://www.postgresql.org/docs/current/performance-tips.html)
- [DataLoader Pattern](https://github.com/graphql/dataloader)
- [SQL Query Optimization](https://use-the-index-luke.com/)
- [Go database/sql Best Practices](https://go.dev/doc/database/sql-injection)