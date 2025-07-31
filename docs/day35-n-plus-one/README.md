# Day 35: N+1問題の解決

## 🎯 本日の目標 (Today's Goal)

データベースアクセスにおける最も深刻なパフォーマンス問題の一つである「N+1問題」を理解し、効果的な解決手法を習得する。Eager Loading、Batch Loading、DataLoaderパターンなどの技術を駆使して、大規模アプリケーションでも高速なデータアクセスを実現できるようになる。

## 📖 解説 (Explanation)

### N+1問題とは？

```go
// 【N+1問題の重要性】データベースパフォーマンス最適化と大規模システム対応
// ❌ 問題例：N+1問題によるサービス停止とユーザー離脱の大災害
func catastrophicNPlusOneProblem() {
    // 🚨 災害例：N+1問題による深刻なパフォーマンス問題とサービス麻痺
    
    // ❌ 最悪の実装：N+1問題が発生するソーシャルメディアAPI
    func getTimelineBadly(userID int) (*Timeline, error) {
        // 1. フォローしているユーザーを取得（1回目のクエリ）
        following, err := getFollowingUsers(userID) // 10,000人フォロー中
        if err != nil {
            return nil, err
        }
        
        var posts []*Post
        
        // ❌ 各フォローユーザーの投稿を個別取得（N回のクエリ）
        for _, followedUser := range following { // 10,000回ループ
            // データベースに毎回アクセス
            userPosts, err := getPostsByUserID(followedUser.ID)
            if err != nil {
                continue // エラー時も処理継続
            }
            
            // 各ユーザーの最新5投稿を取得
            for _, post := range userPosts[:5] {
                // さらに投稿の詳細情報を取得（いいね数、コメント数など）
                postDetails, err := getPostDetails(post.ID) // さらにN回
                if err != nil {
                    continue
                }
                
                // コメントを取得
                comments, err := getCommentsByPostID(post.ID) // さらにN回
                if err != nil {
                    continue
                }
                
                // 各コメントのユーザー情報を取得
                for _, comment := range comments {
                    commentUser, err := getUserByID(comment.UserID) // さらにN回
                    if err != nil {
                        continue
                    }
                    comment.User = commentUser
                }
                
                post.Details = postDetails
                post.Comments = comments
                posts = append(posts, post)
            }
        }
        
        // 【災害的結果】
        // - 初期クエリ: 1回（フォローユーザー取得）
        // - 投稿取得: 10,000回
        // - 投稿詳細: 50,000回（各ユーザー5投稿）
        // - コメント取得: 50,000回
        // - コメントユーザー: 500,000回（1投稿10コメント想定）
        // 合計: 610,001回のクエリ！
        
        return &Timeline{Posts: posts}, nil
    }
    
    // ❌ ECサイトでの商品一覧表示
    func getProductsWithDetailsBadly() ([]*Product, error) {
        // 100商品を取得
        products, err := getAllProducts() // 1回目
        if err != nil {
            return nil, err
        }
        
        for _, product := range products { // 100回ループ
            // 各商品の詳細を個別取得
            details, err := getProductDetails(product.ID) // 100回
            if err != nil {
                continue
            }
            product.Details = details
            
            // 在庫情報を取得
            inventory, err := getInventory(product.ID) // 100回
            if err != nil {
                continue
            }
            product.Inventory = inventory
            
            // レビューを取得
            reviews, err := getReviews(product.ID) // 100回
            if err != nil {
                continue
            }
            
            // 各レビューのユーザー情報
            for _, review := range reviews {
                user, err := getUserByID(review.UserID) // さらに500回（1商品5レビュー想定）
                if err != nil {
                    continue
                }
                review.User = user
            }
            
            product.Reviews = reviews
            
            // 関連商品を取得
            related, err := getRelatedProducts(product.ID) // 100回
            if err != nil {
                continue
            }
            product.Related = related
        }
        
        // 【実際の被害】100商品の場合：
        // - 基本クエリ: 1回
        // - 商品詳細: 100回
        // - 在庫情報: 100回
        // - レビュー: 100回
        // - レビューユーザー: 500回
        // - 関連商品: 100回
        // 合計: 901回のクエリ
        // レスポンス時間: 45秒（ユーザー離脱）
        
        return products, nil
    }
    
    // 【実際の被害例】
    // - Twitter風SNS：タイムライン表示に3分→ユーザー99%離脱
    // - ECサイト：商品一覧が30秒→売上90%減
    // - ニュースサイト：記事一覧が60秒→PV激減
    // - 企業システム：レポート生成に2時間→業務停止
    
    fmt.Println("❌ N+1 problem caused complete service failure!")
    // 結果：データベースサーバークラッシュ、全サービス停止、顧客離れ
}

// ✅ 正解：エンタープライズ級N+1問題解決システム
type EnterpriseNPlusOneResolver struct {
    // 【基本解決手法】
    eagerLoader     *EagerLoader                  // Eager Loading
    batchLoader     *BatchLoader                  // Batch Loading
    dataLoaderPool  *DataLoaderPool               // DataLoader Pool
    
    // 【高度最適化】
    queryOptimizer  *QueryOptimizer               // クエリ最適化
    cacheManager    *CacheManager                 // キャッシュ管理
    indexAdvisor    *IndexAdvisor                 // インデックス提案
    
    // 【パフォーマンス監視】
    queryTracker    *QueryTracker                 // クエリ追跡
    performanceMonitor *PerformanceMonitor        // 性能監視
    alertManager    *AlertManager                 // アラート管理
    
    // 【自動化機能】
    autoOptimizer   *AutoOptimizer                // 自動最適化
    patternDetector *PatternDetector              // パターン検出
    
    // 【スケーラビリティ】
    shardingManager *ShardingManager              // シャーディング
    readReplica     *ReadReplicaManager           // 読み取りレプリカ
    
    db              *sql.DB                       // データベース接続
    config          *ResolverConfig               // 設定管理
    mu              sync.RWMutex                  // 並行アクセス制御
}

// 【重要関数】包括的N+1問題解決システム初期化
func NewEnterpriseNPlusOneResolver(db *sql.DB, config *ResolverConfig) *EnterpriseNPlusOneResolver {
    resolver := &EnterpriseNPlusOneResolver{
        db:              db,
        config:          config,
        eagerLoader:     NewEagerLoader(db),
        batchLoader:     NewBatchLoader(db),
        dataLoaderPool:  NewDataLoaderPool(db, config.PoolSize),
        queryOptimizer:  NewQueryOptimizer(),
        cacheManager:    NewCacheManager(config.CacheConfig),
        indexAdvisor:    NewIndexAdvisor(db),
        queryTracker:    NewQueryTracker(),
        performanceMonitor: NewPerformanceMonitor(),
        alertManager:    NewAlertManager(config.AlertConfig),
        autoOptimizer:   NewAutoOptimizer(),
        patternDetector: NewPatternDetector(),
        shardingManager: NewShardingManager(config.ShardingConfig),
        readReplica:     NewReadReplicaManager(config.ReplicaConfig),
    }
    
    // 【自動監視開始】
    go resolver.startPerformanceMonitoring()
    go resolver.startPatternDetection()
    go resolver.startAutoOptimization()
    
    return resolver
}

// 【核心メソッド】インテリジェントなデータ取得
func (resolver *EnterpriseNPlusOneResolver) LoadUsersWithPosts(
    ctx context.Context,
    userIDs []int,
) ([]*UserWithPosts, error) {
    
    startTime := time.Now()
    
    // 【STEP 1】最適な解決手法を自動選択
    strategy := resolver.selectOptimalStrategy(len(userIDs))
    
    var result []*UserWithPosts
    var err error
    
    switch strategy {
    case EagerLoadingStrategy:
        result, err = resolver.loadWithEagerLoading(ctx, userIDs)
    case BatchLoadingStrategy:
        result, err = resolver.loadWithBatchLoading(ctx, userIDs)
    case DataLoaderStrategy:
        result, err = resolver.loadWithDataLoader(ctx, userIDs)
    case HybridStrategy:
        result, err = resolver.loadWithHybridApproach(ctx, userIDs)
    }
    
    if err != nil {
        return nil, fmt.Errorf("data loading failed: %w", err)
    }
    
    // 【STEP 2】パフォーマンスメトリクス記録
    duration := time.Since(startTime)
    resolver.performanceMonitor.RecordQuery("LoadUsersWithPosts", duration, len(userIDs))
    
    // 【STEP 3】N+1問題検出
    if resolver.queryTracker.DetectNPlusOnePattern() {
        resolver.alertManager.SendAlert("N+1 problem detected", AlertLevelWarning)
    }
    
    return result, nil
}

// 【高度メソッド】Eager Loading最適化実装
func (resolver *EnterpriseNPlusOneResolver) loadWithEagerLoading(
    ctx context.Context,
    userIDs []int,
) ([]*UserWithPosts, error) {
    
    // 【最適化されたJOINクエリ】
    query := `
        WITH user_filter AS (
            SELECT unnest($1::int[]) as user_id
        ),
        ranked_posts AS (
            SELECT 
                p.*,
                ROW_NUMBER() OVER (PARTITION BY p.user_id ORDER BY p.created_at DESC) as rn
            FROM posts p
            INNER JOIN user_filter uf ON p.user_id = uf.user_id
        )
        SELECT 
            u.id, u.name, u.email, u.created_at,
            p.id, p.user_id, p.title, p.content, p.created_at,
            COALESCE(pc.comment_count, 0) as comment_count,
            COALESCE(pl.like_count, 0) as like_count
        FROM users u
        INNER JOIN user_filter uf ON u.id = uf.user_id
        LEFT JOIN ranked_posts p ON u.id = p.user_id AND p.rn <= 10  -- 最新10投稿のみ
        LEFT JOIN (
            SELECT post_id, COUNT(*) as comment_count
            FROM comments
            GROUP BY post_id
        ) pc ON p.id = pc.post_id
        LEFT JOIN (
            SELECT post_id, COUNT(*) as like_count
            FROM likes
            GROUP BY post_id
        ) pl ON p.id = pl.post_id
        ORDER BY u.id, p.created_at DESC
    `
    
    // PostgreSQLの配列パラメータを使用
    pq_array := pq.Array(userIDs)
    
    rows, err := resolver.db.QueryContext(ctx, query, pq_array)
    if err != nil {
        return nil, fmt.Errorf("eager loading query failed: %w", err)
    }
    defer rows.Close()
    
    return resolver.buildUserWithPostsFromRows(rows)
}

// 【高度メソッド】DataLoader実装
func (resolver *EnterpriseNPlusOneResolver) loadWithDataLoader(
    ctx context.Context,
    userIDs []int,
) ([]*UserWithPosts, error) {
    
    // 【並列データロード】
    userLoader := resolver.dataLoaderPool.GetUserLoader()
    postLoader := resolver.dataLoaderPool.GetPostLoader()
    
    // ユーザーとポストを並列取得
    var wg sync.WaitGroup
    var users []*User
    var postsMap map[int][]*Post
    var userErr, postErr error
    
    wg.Add(2)
    
    // ユーザー情報を並列取得
    go func() {
        defer wg.Done()
        users, userErr = userLoader.LoadMany(ctx, userIDs)
    }()
    
    // 投稿情報を並列取得
    go func() {
        defer wg.Done()
        postsMap, postErr = postLoader.LoadManyByUserIDs(ctx, userIDs)
    }()
    
    wg.Wait()
    
    if userErr != nil {
        return nil, fmt.Errorf("user loading failed: %w", userErr)
    }
    if postErr != nil {
        return nil, fmt.Errorf("post loading failed: %w", postErr)
    }
    
    // 結果を組み合わせ
    result := make([]*UserWithPosts, len(users))
    for i, user := range users {
        posts, exists := postsMap[user.ID]
        if !exists {
            posts = []*Post{}
        }
        
        result[i] = &UserWithPosts{
            User:  user,
            Posts: posts,
        }
    }
    
    return result, nil
}

// 【実用例】SNSタイムライン最適化実装
func BenchmarkTimelineGeneration(b *testing.B) {
    resolver := setupEnterpriseResolver()
    userIDs := generateTestUserIDs(10000) // 1万人のフォロー
    
    b.Run("N+1Problem_Disaster", func(b *testing.B) {
        queryCount := 0
        
        for i := 0; i < b.N; i++ {
            // 災害的実装
            timeline := generateTimelineBadly(userIDs)
            queryCount += len(userIDs) * 5 // 各ユーザーあたり5クエリ
            
            if len(timeline.Posts) == 0 {
                b.Error("Timeline generation failed")
            }
        }
        
        b.Logf("Total queries executed: %d", queryCount)
        b.Logf("Queries per user: %.2f", float64(queryCount)/float64(len(userIDs)))
    })
    
    b.Run("EagerLoading_Optimized", func(b *testing.B) {
        queryCount := 0
        
        for i := 0; i < b.N; i++ {
            // 最適化実装
            timeline, err := resolver.GenerateTimelineOptimized(context.Background(), userIDs)
            if err != nil {
                b.Fatal(err)
            }
            queryCount += 1 // 1つのJOINクエリのみ
            
            if len(timeline.Posts) == 0 {
                b.Error("Timeline generation failed")
            }
        }
        
        b.Logf("Total queries executed: %d", queryCount)
        b.Logf("Query reduction: %.2f%%", 
            (1.0 - float64(queryCount)/float64(len(userIDs)*5))*100)
    })
    
    b.Run("HybridApproach_Enterprise", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            // エンタープライズ級ハイブリッド実装
            timeline, metrics, err := resolver.GenerateTimelineWithMetrics(
                context.Background(), userIDs,
            )
            if err != nil {
                b.Fatal(err)
            }
            
            if len(timeline.Posts) == 0 {
                b.Error("Timeline generation failed")
            }
            
            // パフォーマンスメトリクス記録
            b.Logf("Cache hit rate: %.2f%%", metrics.CacheHitRate*100)
            b.Logf("Average query time: %v", metrics.AvgQueryTime)
            b.Logf("Total database operations: %d", metrics.DatabaseOps)
        }
    })
}
```

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