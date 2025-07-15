# Day 41: Redis高性能キャッシュシステムとメモリ最適化

## 🎯 本日の目標

このチャレンジを通して、以下のスキルを身につけることができます：

- **Redisを活用した高性能キャッシュシステムを設計・実装できるようになる**
- **メモリ効率とパフォーマンスを両立したキャッシュ戦略をマスターする**
- **Redis Cluster構成での分散キャッシュ管理を実装できるようになる**
- **プロダクション環境でのRedis運用ベストプラクティスを習得する**

## 📖 解説

### なぜRedisキャッシュが必要なのか？

現代のWebアプリケーションでは、データベースへのクエリが性能のボトルネックになることが一般的です。適切なキャッシュ戦略なしでは以下の問題が発生します：

#### キャッシュなしの性能問題

```go
// 問題のある例：毎回データベースアクセス
func GetUserProfile(db *sql.DB, userID int) (*UserProfile, error) {
    // 毎回重いJOINクエリを実行
    query := `
        SELECT u.id, u.name, u.email, u.avatar_url,
               p.bio, p.website, p.location,
               COUNT(f.follower_id) as follower_count,
               COUNT(po.id) as post_count,
               AVG(r.rating) as avg_rating
        FROM users u
        LEFT JOIN profiles p ON u.id = p.user_id
        LEFT JOIN followers f ON u.id = f.user_id
        LEFT JOIN posts po ON u.id = po.author_id
        LEFT JOIN reviews r ON u.id = r.reviewer_id
        WHERE u.id = $1
        GROUP BY u.id, p.bio, p.website, p.location
    `
    
    // このクエリが毎回300msかかる場合
    // 100同時リクエスト = 30秒の総処理時間
    start := time.Now()
    row := db.QueryRow(query, userID)
    
    var profile UserProfile
    err := row.Scan(
        &profile.ID, &profile.Name, &profile.Email, &profile.AvatarURL,
        &profile.Bio, &profile.Website, &profile.Location,
        &profile.FollowerCount, &profile.PostCount, &profile.AvgRating,
    )
    if err != nil {
        return nil, err
    }
    
    log.Printf("Database query took: %v", time.Since(start))
    return &profile, nil
}

func GetPopularPosts(db *sql.DB, limit int) ([]Post, error) {
    // 毎回重い集計クエリ
    query := `
        SELECT p.id, p.title, p.content, p.created_at,
               u.name as author_name,
               COUNT(l.id) as like_count,
               COUNT(c.id) as comment_count
        FROM posts p
        JOIN users u ON p.author_id = u.id
        LEFT JOIN likes l ON p.id = l.post_id
        LEFT JOIN comments c ON p.id = c.post_id
        WHERE p.created_at > NOW() - INTERVAL '24 hours'
        GROUP BY p.id, u.name
        ORDER BY like_count DESC, comment_count DESC
        LIMIT $1
    `
    
    // 人気投稿の計算に毎回5秒かかる
    rows, err := db.Query(query, limit)
    // ...処理
}
```

**問題点の分析：**
- **レスポンス時間**: データベースクエリによる300ms-5秒の遅延
- **データベース負荷**: 同じクエリが何度も実行されCPU/IOを圧迫
- **スケーラビリティ**: アクセス数増加で指数的に性能劣化
- **ユーザー体験**: 遅いレスポンスによる離脱率増加

### Redisによる劇的な性能改善

同じ機能をRedisキャッシュで最適化すると：

```go
import (
    "github.com/redis/go-redis/v9"
    "encoding/json"
    "time"
    "context"
)

type CacheManager struct {
    rdb           *redis.Client
    db            *sql.DB
    defaultTTL    time.Duration
    clusterClient *redis.ClusterClient
    metrics       *CacheMetrics
}

type CacheMetrics struct {
    Hits          int64
    Misses        int64
    Evictions     int64
    TotalRequests int64
    AvgLatency    time.Duration
    mu            sync.RWMutex
}

func NewCacheManager(redisAddr, dbDSN string) (*CacheManager, error) {
    // Redis接続の最適化設定
    rdb := redis.NewClient(&redis.Options{
        Addr:            redisAddr,
        Password:        "",
        DB:              0,
        PoolSize:        100,         // 接続プール最大数
        PoolTimeout:     30 * time.Second,
        IdleTimeout:     5 * time.Minute,
        IdleCheckFrequency: 1 * time.Minute,
        
        // 接続タイムアウト設定
        DialTimeout:  10 * time.Second,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 5 * time.Second,
        
        // 再試行設定
        MaxRetries:      3,
        MinRetryBackoff: 100 * time.Millisecond,
        MaxRetryBackoff: 2 * time.Second,
    })
    
    // 接続テスト
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := rdb.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to Redis: %w", err)
    }
    
    db, err := sql.Open("postgres", dbDSN)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    
    return &CacheManager{
        rdb:        rdb,
        db:         db,
        defaultTTL: 15 * time.Minute,
        metrics:    &CacheMetrics{},
    }, nil
}

func (cm *CacheManager) GetUserProfile(ctx context.Context, userID int) (*UserProfile, error) {
    start := time.Now()
    defer func() {
        cm.recordLatency(time.Since(start))
    }()
    
    cacheKey := fmt.Sprintf("user_profile:%d", userID)
    
    // 1. キャッシュから試行
    cached, err := cm.rdb.Get(ctx, cacheKey).Result()
    if err == nil {
        cm.recordHit()
        
        var profile UserProfile
        if err := json.Unmarshal([]byte(cached), &profile); err != nil {
            return nil, fmt.Errorf("failed to unmarshal cached profile: %w", err)
        }
        
        log.Printf("Cache hit for user %d (took: %v)", userID, time.Since(start))
        return &profile, nil
    }
    
    // 2. キャッシュミス：データベースから取得
    if err != redis.Nil {
        log.Printf("Redis error for user %d: %v", userID, err)
    }
    
    cm.recordMiss()
    
    profile, err := cm.getUserProfileFromDB(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // 3. キャッシュに保存（非同期）
    go func() {
        cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := cm.cacheUserProfile(cacheCtx, cacheKey, profile); err != nil {
            log.Printf("Failed to cache user profile %d: %v", userID, err)
        }
    }()
    
    log.Printf("Database fetch for user %d (took: %v)", userID, time.Since(start))
    return profile, nil
}

func (cm *CacheManager) getUserProfileFromDB(ctx context.Context, userID int) (*UserProfile, error) {
    query := `
        SELECT u.id, u.name, u.email, u.avatar_url,
               COALESCE(p.bio, '') as bio, 
               COALESCE(p.website, '') as website, 
               COALESCE(p.location, '') as location,
               COALESCE(stats.follower_count, 0) as follower_count,
               COALESCE(stats.post_count, 0) as post_count,
               COALESCE(stats.avg_rating, 0) as avg_rating
        FROM users u
        LEFT JOIN profiles p ON u.id = p.user_id
        LEFT JOIN (
            SELECT user_id,
                   COUNT(DISTINCT f.follower_id) as follower_count,
                   COUNT(DISTINCT po.id) as post_count,
                   AVG(r.rating) as avg_rating
            FROM users u2
            LEFT JOIN followers f ON u2.id = f.user_id
            LEFT JOIN posts po ON u2.id = po.author_id
            LEFT JOIN reviews r ON u2.id = r.reviewer_id
            WHERE u2.id = $1
            GROUP BY user_id
        ) stats ON u.id = stats.user_id
        WHERE u.id = $1
    `
    
    var profile UserProfile
    err := cm.db.QueryRowContext(ctx, query, userID).Scan(
        &profile.ID, &profile.Name, &profile.Email, &profile.AvatarURL,
        &profile.Bio, &profile.Website, &profile.Location,
        &profile.FollowerCount, &profile.PostCount, &profile.AvgRating,
    )
    
    if err != nil {
        return nil, fmt.Errorf("failed to get user profile from DB: %w", err)
    }
    
    return &profile, nil
}

func (cm *CacheManager) cacheUserProfile(ctx context.Context, key string, profile *UserProfile) error {
    data, err := json.Marshal(profile)
    if err != nil {
        return fmt.Errorf("failed to marshal profile: %w", err)
    }
    
    return cm.rdb.Set(ctx, key, data, cm.defaultTTL).Err()
}
```

**改善効果：**
- **レスポンス時間**: 300ms → 2ms（150倍高速化）
- **データベース負荷**: 95%削減（キャッシュヒット率90%想定）
- **同時処理能力**: 10リクエスト/秒 → 5000リクエスト/秒
- **ユーザー体験**: 瞬時のレスポンスによる満足度向上

### 高度なRedisキャッシュパターン

#### 1. 多層キャッシュとプリロード戦略

```go
type MultiLayerCache struct {
    l1Cache    *sync.Map          // アプリケーション内メモリキャッシュ
    l2Cache    *redis.Client      // Redis分散キャッシュ
    db         *sql.DB            // データベース
    preloader  *CachePreloader
}

type CachePreloader struct {
    cache     *MultiLayerCache
    scheduler *time.Ticker
    patterns  []PreloadPattern
}

type PreloadPattern struct {
    KeyPattern    string
    LoadFunction  func(ctx context.Context) (map[string]interface{}, error)
    Schedule      time.Duration
    Priority      int
}

func (mlc *MultiLayerCache) Get(ctx context.Context, key string) (interface{}, error) {
    // L1キャッシュ（メモリ）から試行
    if value, ok := mlc.l1Cache.Load(key); ok {
        return value, nil
    }
    
    // L2キャッシュ（Redis）から試行
    redisValue, err := mlc.l2Cache.Get(ctx, key).Result()
    if err == nil {
        var data interface{}
        if err := json.Unmarshal([]byte(redisValue), &data); err == nil {
            // L1キャッシュにも保存
            mlc.l1Cache.Store(key, data)
            return data, nil
        }
    }
    
    // データベースから取得とキャッシュ
    return mlc.loadFromDatabase(ctx, key)
}

func (mlc *MultiLayerCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    // L1キャッシュに保存
    mlc.l1Cache.Store(key, value)
    
    // L2キャッシュ（Redis）に保存
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    
    return mlc.l2Cache.Set(ctx, key, data, ttl).Err()
}

func (cp *CachePreloader) StartPreloading() {
    go func() {
        for range cp.scheduler.C {
            for _, pattern := range cp.patterns {
                go cp.preloadPattern(pattern)
            }
        }
    }()
}

func (cp *CachePreloader) preloadPattern(pattern PreloadPattern) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    data, err := pattern.LoadFunction(ctx)
    if err != nil {
        log.Printf("Failed to preload pattern %s: %v", pattern.KeyPattern, err)
        return
    }
    
    for key, value := range data {
        if err := cp.cache.Set(ctx, key, value, pattern.Schedule*2); err != nil {
            log.Printf("Failed to cache preloaded data %s: %v", key, err)
        }
    }
    
    log.Printf("Preloaded %d items for pattern %s", len(data), pattern.KeyPattern)
}

// 人気コンテンツのプリロード例
func (cm *CacheManager) PreloadPopularContent(ctx context.Context) (map[string]interface{}, error) {
    popularPosts, err := cm.getPopularPostsFromDB(ctx, 100)
    if err != nil {
        return nil, err
    }
    
    trendingUsers, err := cm.getTrendingUsersFromDB(ctx, 50)
    if err != nil {
        return nil, err
    }
    
    result := make(map[string]interface{})
    
    for _, post := range popularPosts {
        key := fmt.Sprintf("post:%d", post.ID)
        result[key] = post
    }
    
    for _, user := range trendingUsers {
        key := fmt.Sprintf("user_profile:%d", user.ID)
        result[key] = user
    }
    
    return result, nil
}
```

#### 2. Redis Pipelineとバッチ処理

```go
type BatchCacheManager struct {
    rdb         *redis.Client
    batchSize   int
    flushTimer  *time.Timer
    pending     map[string]CacheOperation
    mu          sync.Mutex
}

type CacheOperation struct {
    Type      string      // "SET", "GET", "DEL"
    Key       string
    Value     interface{}
    TTL       time.Duration
    Callback  func(interface{}, error)
    CreatedAt time.Time
}

func (bcm *BatchCacheManager) BatchSet(key string, value interface{}, ttl time.Duration) {
    bcm.mu.Lock()
    defer bcm.mu.Unlock()
    
    bcm.pending[key] = CacheOperation{
        Type:      "SET",
        Key:       key,
        Value:     value,
        TTL:       ttl,
        CreatedAt: time.Now(),
    }
    
    if len(bcm.pending) >= bcm.batchSize {
        go bcm.flushPending()
    }
}

func (bcm *BatchCacheManager) flushPending() {
    bcm.mu.Lock()
    operations := make(map[string]CacheOperation)
    for k, v := range bcm.pending {
        operations[k] = v
        delete(bcm.pending, k)
    }
    bcm.mu.Unlock()
    
    if len(operations) == 0 {
        return
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // Redisパイプラインで一括実行
    pipe := bcm.rdb.Pipeline()
    
    for _, op := range operations {
        switch op.Type {
        case "SET":
            data, err := json.Marshal(op.Value)
            if err != nil {
                log.Printf("Failed to marshal value for key %s: %v", op.Key, err)
                continue
            }
            pipe.Set(ctx, op.Key, data, op.TTL)
            
        case "DEL":
            pipe.Del(ctx, op.Key)
        }
    }
    
    // 一括実行
    cmds, err := pipe.Exec(ctx)
    if err != nil {
        log.Printf("Failed to execute batch operations: %v", err)
        return
    }
    
    log.Printf("Executed %d cache operations in batch", len(cmds))
}

func (bcm *BatchCacheManager) StartBatchProcessor() {
    bcm.flushTimer = time.NewTimer(100 * time.Millisecond)
    go func() {
        for range bcm.flushTimer.C {
            bcm.flushPending()
            bcm.flushTimer.Reset(100 * time.Millisecond)
        }
    }()
}
```

#### 3. Redis Cluster分散キャッシュ

```go
type ClusterCacheManager struct {
    cluster     *redis.ClusterClient
    hashRing    *ConsistentHash
    nodeManager *NodeManager
    metrics     *ClusterMetrics
}

type ConsistentHash struct {
    nodes    map[uint32]string
    keys     []uint32
    replicas int
    mu       sync.RWMutex
}

type NodeManager struct {
    nodes       []string
    healthCheck map[string]bool
    mu          sync.RWMutex
}

type ClusterMetrics struct {
    NodeLatency    map[string]time.Duration
    NodeLoad       map[string]int64
    RedirectCount  int64
    ClusterErrors  int64
    mu             sync.RWMutex
}

func NewClusterCacheManager(nodes []string) (*ClusterCacheManager, error) {
    // Redis Clusterクライアント設定
    cluster := redis.NewClusterClient(&redis.ClusterOptions{
        Addrs:              nodes,
        MaxRedirects:       3,
        ReadOnly:           true,
        RouteByLatency:     true,
        RouteRandomly:      false,
        
        // 接続プール設定
        PoolSize:           100,
        PoolTimeout:        30 * time.Second,
        IdleTimeout:        5 * time.Minute,
        IdleCheckFrequency: 1 * time.Minute,
        
        // タイムアウト設定
        DialTimeout:  10 * time.Second,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 5 * time.Second,
    })
    
    // 接続テスト
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    if err := cluster.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to Redis cluster: %w", err)
    }
    
    hashRing := NewConsistentHash(3, nodes)
    nodeManager := &NodeManager{
        nodes:       nodes,
        healthCheck: make(map[string]bool),
    }
    
    // 全ノードを初期状態では健全とマーク
    for _, node := range nodes {
        nodeManager.healthCheck[node] = true
    }
    
    ccm := &ClusterCacheManager{
        cluster:     cluster,
        hashRing:    hashRing,
        nodeManager: nodeManager,
        metrics: &ClusterMetrics{
            NodeLatency: make(map[string]time.Duration),
            NodeLoad:    make(map[string]int64),
        },
    }
    
    // ヘルスチェック開始
    go ccm.startHealthMonitoring()
    
    return ccm, nil
}

func (ccm *ClusterCacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return fmt.Errorf("failed to marshal value: %w", err)
    }
    
    start := time.Now()
    err = ccm.cluster.Set(ctx, key, data, ttl).Err()
    
    // メトリクス記録
    node := ccm.hashRing.GetNode(key)
    ccm.recordNodeLatency(node, time.Since(start))
    
    if err != nil {
        ccm.recordClusterError()
        return fmt.Errorf("failed to set cache: %w", err)
    }
    
    return nil
}

func (ccm *ClusterCacheManager) Get(ctx context.Context, key string) (interface{}, error) {
    start := time.Now()
    result, err := ccm.cluster.Get(ctx, key).Result()
    
    node := ccm.hashRing.GetNode(key)
    ccm.recordNodeLatency(node, time.Since(start))
    
    if err == redis.Nil {
        return nil, ErrCacheMiss
    }
    
    if err != nil {
        ccm.recordClusterError()
        return nil, fmt.Errorf("failed to get cache: %w", err)
    }
    
    var data interface{}
    if err := json.Unmarshal([]byte(result), &data); err != nil {
        return nil, fmt.Errorf("failed to unmarshal cached data: %w", err)
    }
    
    return data, nil
}

func (ccm *ClusterCacheManager) startHealthMonitoring() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        ccm.checkClusterHealth()
    }
}

func (ccm *ClusterCacheManager) checkClusterHealth() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // クラスターノード情報を取得
    clusterNodes, err := ccm.cluster.ClusterNodes(ctx).Result()
    if err != nil {
        log.Printf("Failed to get cluster nodes: %v", err)
        return
    }
    
    ccm.updateNodeHealth(clusterNodes)
}

func (ccm *ClusterCacheManager) updateNodeHealth(nodesInfo string) {
    ccm.nodeManager.mu.Lock()
    defer ccm.nodeManager.mu.Unlock()
    
    lines := strings.Split(nodesInfo, "\n")
    for _, line := range lines {
        if line == "" {
            continue
        }
        
        parts := strings.Fields(line)
        if len(parts) < 8 {
            continue
        }
        
        nodeID := parts[0]
        address := strings.Split(parts[1], "@")[0] // Remove port for cluster
        flags := parts[2]
        
        // ノードの健全性をフラグから判定
        isHealthy := !strings.Contains(flags, "fail") && 
                    !strings.Contains(flags, "handshake") &&
                    (strings.Contains(flags, "master") || strings.Contains(flags, "slave"))
        
        ccm.nodeManager.healthCheck[address] = isHealthy
        
        log.Printf("Node %s (%s): healthy=%v", nodeID[:8], address, isHealthy)
    }
}

func (ccm *ClusterCacheManager) recordNodeLatency(node string, latency time.Duration) {
    ccm.metrics.mu.Lock()
    defer ccm.metrics.mu.Unlock()
    ccm.metrics.NodeLatency[node] = latency
}

func (ccm *ClusterCacheManager) recordClusterError() {
    ccm.metrics.mu.Lock()
    defer ccm.metrics.mu.Unlock()
    ccm.metrics.ClusterErrors++
}

func NewConsistentHash(replicas int, nodes []string) *ConsistentHash {
    ch := &ConsistentHash{
        nodes:    make(map[uint32]string),
        replicas: replicas,
    }
    
    for _, node := range nodes {
        ch.AddNode(node)
    }
    
    return ch
}

func (ch *ConsistentHash) AddNode(node string) {
    ch.mu.Lock()
    defer ch.mu.Unlock()
    
    for i := 0; i < ch.replicas; i++ {
        hash := ch.hashKey(fmt.Sprintf("%s:%d", node, i))
        ch.nodes[hash] = node
        ch.keys = append(ch.keys, hash)
    }
    
    sort.Slice(ch.keys, func(i, j int) bool {
        return ch.keys[i] < ch.keys[j]
    })
}

func (ch *ConsistentHash) GetNode(key string) string {
    ch.mu.RLock()
    defer ch.mu.RUnlock()
    
    if len(ch.keys) == 0 {
        return ""
    }
    
    hash := ch.hashKey(key)
    
    // Find the first node with hash >= key hash
    idx := sort.Search(len(ch.keys), func(i int) bool {
        return ch.keys[i] >= hash
    })
    
    // Wrap around if necessary
    if idx == len(ch.keys) {
        idx = 0
    }
    
    return ch.nodes[ch.keys[idx]]
}

func (ch *ConsistentHash) hashKey(key string) uint32 {
    h := fnv.New32a()
    h.Write([]byte(key))
    return h.Sum32()
}
```

📝 **課題**

以下の機能を持つRedis高性能キャッシュシステムを実装してください：

1. **`CacheManager`**: 基本的なRedisキャッシュ操作
2. **`MultiLayerCache`**: L1/L2多層キャッシュシステム
3. **`BatchCacheManager`**: パイプライン処理とバッチ操作
4. **`ClusterCacheManager`**: Redis Cluster分散キャッシュ
5. **`CacheMetrics`**: パフォーマンス監視とメトリクス
6. **統合テスト**: 実際のRedis環境でのテストスイート

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestCacheManager_BasicOperations
--- PASS: TestCacheManager_BasicOperations (0.05s)
=== RUN   TestMultiLayerCache_L1L2Performance
--- PASS: TestMultiLayerCache_L1L2Performance (0.08s)
=== RUN   TestBatchCacheManager_PipelineOps
--- PASS: TestBatchCacheManager_PipelineOps (0.10s)
=== RUN   TestClusterCacheManager_DistributedOps
--- PASS: TestClusterCacheManager_DistributedOps (0.15s)
=== RUN   TestCacheMetrics_PerformanceTracking
--- PASS: TestCacheMetrics_PerformanceTracking (0.12s)
PASS
ok      day41-redis-caching    0.500s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **go-redis/redis**: 最新のRedis Go クライアント
2. **Redis Pipeline**: バッチ処理による性能向上
3. **Consistent Hashing**: 分散キャッシュでの均等な負荷分散
4. **TTL Management**: 適切なキャッシュ期限管理
5. **Error Handling**: Redis接続エラーとフォールバック処理

設計のポイント：
- **接続プール**: Redis接続の効率的な管理
- **キャッシュ戦略**: Write-Through, Write-Behind, Cache-Aside
- **メモリ効率**: JSON vs MessagePack vs Protocol Buffers
- **監視**: レイテンシ、ヒット率、エラー率の追跡

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```
})
```

### TTL (Time To Live) 管理

キャッシュデータには適切な有効期限を設定することが重要です：

```go
// TTL 付きでデータを設定
rdb.Set(ctx, "session:12345", userData, 30*time.Minute)

// TTL を確認
ttl := rdb.TTL(ctx, "session:12345").Val()

// TTL を更新
rdb.Expire(ctx, "session:12345", time.Hour)
```

### ヘルスチェック

Redis の接続状態を監視することは重要です：

```go
// Ping でヘルスチェック
pong, err := rdb.Ping(ctx).Result()
if err != nil {
    log.Printf("Redis connection failed: %v", err)
}
```

### エラーハンドリング

Redis 特有のエラーを適切に処理する必要があります：

```go
val, err := rdb.Get(ctx, "key").Result()
if err == redis.Nil {
    // キーが存在しない場合
    fmt.Println("Key does not exist")
} else if err != nil {
    // その他のエラー
    log.Printf("Redis error: %v", err)
}
```

## 📝 課題 (The Problem)

以下の機能を持つ Redis キャッシュクライアントを実装してください：

### 1. CacheClient 構造体の実装

```go
type CacheClient struct {
    client *redis.Client
    stats  *CacheStats
}

type CacheStats struct {
    Hits   int64
    Misses int64
    Errors int64
}
```

### 2. 必要なメソッドの実装

- `NewCacheClient(addr string) (*CacheClient, error)`: クライアントの初期化
- `Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error`: データの設定
- `Get(ctx context.Context, key string) (string, error)`: データの取得
- `Delete(ctx context.Context, key string) error`: データの削除
- `Exists(ctx context.Context, key string) (bool, error)`: キーの存在確認
- `GetStats() CacheStats`: キャッシュ統計の取得
- `HealthCheck(ctx context.Context) error`: ヘルスチェック
- `Close() error`: 接続のクリーンアップ

### 3. 統計情報の管理

キャッシュのヒット率、ミス率、エラー率を追跡してください。

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestCacheClient_BasicOperations
    main_test.go:45: Set operation successful
    main_test.go:52: Retrieved value: test_value
    main_test.go:59: Key exists: true
    main_test.go:66: Key deleted successfully
    main_test.go:73: Key no longer exists: false
--- PASS: TestCacheClient_BasicOperations (0.02s)

=== RUN   TestCacheClient_TTL
    main_test.go:95: Value set with TTL
    main_test.go:102: Value retrieved before expiration: ttl_value
    main_test.go:109: Value expired and no longer accessible
--- PASS: TestCacheClient_TTL (1.51s)

=== RUN   TestCacheClient_Stats
    main_test.go:135: Cache stats - Hits: 2, Misses: 1, Errors: 0
--- PASS: TestCacheClient_Stats (0.01s)

=== RUN   TestCacheClient_HealthCheck
    main_test.go:150: Health check passed
--- PASS: TestCacheClient_HealthCheck (0.01s)

PASS
ok      day41-redis-caching     1.672s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### パッケージのインポート

```go
import (
    "context"
    "encoding/json"
    "sync/atomic"
    "time"
    
    "github.com/go-redis/redis/v8"
)
```

### 依存関係

```bash
go mod init day41-redis-caching
go get github.com/go-redis/redis/v8
go get github.com/ory/dockertest/v3
```

### 統計情報の原子的操作

```go
// ヒット数の増加
atomic.AddInt64(&c.stats.Hits, 1)

// ミス数の増加
atomic.AddInt64(&c.stats.Misses, 1)
```

### JSON エンコーディング

複雑なデータ構造をキャッシュする場合：

```go
data, err := json.Marshal(value)
if err != nil {
    return err
}
return c.client.Set(ctx, key, data, ttl).Err()
```

### エラー分類

```go
if err == redis.Nil {
    // キーが存在しない
    atomic.AddInt64(&c.stats.Misses, 1)
    return "", ErrCacheMiss
} else if err != nil {
    // その他のエラー
    atomic.AddInt64(&c.stats.Errors, 1)
    return "", err
}
```

### Docker テスト環境

テストで Redis コンテナを使用する場合：

```go
func setupRedis(t *testing.T) (*redis.Client, func()) {
    pool, err := dockertest.NewPool("")
    require.NoError(t, err)
    
    resource, err := pool.Run("redis", "7", nil)
    require.NoError(t, err)
    
    // 接続確認とクリーンアップ関数を返す
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **バルク操作**: 複数のキーを一度に操作する機能
2. **キー名前空間**: アプリケーション別にキーを分離する機能
3. **圧縮**: 大きなデータを圧縮してキャッシュする機能
4. **メトリクス**: Prometheus メトリクスの出力
5. **フェイルオーバー**: Redis サーバーダウン時の対処

実装を通じて、Redis の基本的な使用方法と、プロダクション環境でのキャッシュ設計の基礎を学びましょう！