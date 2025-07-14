# Day 40: Read-Replicaへの分散

🎯 **本日の目標**

更新系と参照系のクエリを別のDBに振り分けるロジックを実装し、データベースの負荷分散ができるようになる。

📖 **解説**

## Read-Replicaとは

Read-Replica（読み取りレプリカ）は、メインのデータベース（Master/Primary）から非同期でデータを複製したデータベースです。読み取り専用の操作をレプリカに分散することで、システム全体のパフォーマンスを向上させます。

### Read-Replicaの利点

1. **負荷分散**: 読み取りクエリをレプリカに分散
2. **高可用性**: メインDBがダウンしても読み取りは継続可能
3. **地理的分散**: 異なる地域にレプリカを配置
4. **スケーラビリティ**: 読み取り性能の水平スケーリング

### 基本的なRead-Replica実装

```go
package main

import (
    "context"
    "database/sql"
    "errors"
    "sync"
    
    "github.com/jmoiron/sqlx"
)

// DBCluster manages primary and replica databases
type DBCluster struct {
    primary  *sqlx.DB
    replicas []*sqlx.DB
    mu       sync.RWMutex
    current  int // Round-robin counter for replicas
}

func NewDBCluster(primaryDSN string, replicaDSNs []string) (*DBCluster, error) {
    primary, err := sqlx.Open("postgres", primaryDSN)
    if err != nil {
        return nil, err
    }
    
    replicas := make([]*sqlx.DB, len(replicaDSNs))
    for i, dsn := range replicaDSNs {
        replica, err := sqlx.Open("postgres", dsn)
        if err != nil {
            return nil, err
        }
        replicas[i] = replica
    }
    
    return &DBCluster{
        primary:  primary,
        replicas: replicas,
    }, nil
}

// GetPrimary returns the primary database for write operations
func (cluster *DBCluster) GetPrimary() *sqlx.DB {
    return cluster.primary
}

// GetReplica returns a replica database for read operations
func (cluster *DBCluster) GetReplica() *sqlx.DB {
    if len(cluster.replicas) == 0 {
        return cluster.primary // Fallback to primary
    }
    
    cluster.mu.Lock()
    defer cluster.mu.Unlock()
    
    replica := cluster.replicas[cluster.current]
    cluster.current = (cluster.current + 1) % len(cluster.replicas)
    
    return replica
}
```

### 操作の分離

```go
// UserService demonstrates read-write splitting
type UserService struct {
    cluster *DBCluster
}

func NewUserService(cluster *DBCluster) *UserService {
    return &UserService{cluster: cluster}
}

// CreateUser performs write operation on primary
func (us *UserService) CreateUser(ctx context.Context, user *User) error {
    db := us.cluster.GetPrimary()
    
    query := `
        INSERT INTO users (name, email, age) 
        VALUES ($1, $2, $3) 
        RETURNING id, created_at`
    
    return db.GetContext(ctx, user, query, user.Name, user.Email, user.Age)
}

// GetUser performs read operation on replica
func (us *UserService) GetUser(ctx context.Context, id int) (*User, error) {
    db := us.cluster.GetReplica()
    
    var user User
    err := db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = $1", id)
    return &user, err
}

// UpdateUser performs write operation on primary
func (us *UserService) UpdateUser(ctx context.Context, user *User) error {
    db := us.cluster.GetPrimary()
    
    query := `
        UPDATE users 
        SET name = $1, email = $2, age = $3, updated_at = NOW() 
        WHERE id = $4`
    
    _, err := db.ExecContext(ctx, query, user.Name, user.Email, user.Age, user.ID)
    return err
}

// SearchUsers performs read operation on replica
func (us *UserService) SearchUsers(ctx context.Context, filter UserFilter) ([]User, error) {
    db := us.cluster.GetReplica()
    
    // Complex search query that benefits from replica
    query := `
        SELECT * FROM users 
        WHERE ($1 = '' OR name ILIKE '%' || $1 || '%')
          AND ($2 = '' OR email ILIKE '%' || $2 || '%')
          AND ($3 = 0 OR age >= $3)
        ORDER BY created_at DESC
        LIMIT $4 OFFSET $5`
    
    var users []User
    err := db.SelectContext(ctx, &users, query, 
        filter.Name, filter.Email, filter.MinAge, filter.Limit, filter.Offset)
    return users, err
}
```

### 高度なルーティング戦略

```go
// ReadWriteRouter provides intelligent query routing
type ReadWriteRouter struct {
    cluster    *DBCluster
    health     *HealthChecker
    metrics    *RoutingMetrics
    strategy   RoutingStrategy
}

type RoutingStrategy interface {
    SelectReplica(replicas []*sqlx.DB, metrics *RoutingMetrics) *sqlx.DB
}

// RoundRobinStrategy implements round-robin replica selection
type RoundRobinStrategy struct {
    current int
    mu      sync.Mutex
}

func (rr *RoundRobinStrategy) SelectReplica(replicas []*sqlx.DB, metrics *RoutingMetrics) *sqlx.DB {
    if len(replicas) == 0 {
        return nil
    }
    
    rr.mu.Lock()
    defer rr.mu.Unlock()
    
    replica := replicas[rr.current]
    rr.current = (rr.current + 1) % len(replicas)
    
    return replica
}

// WeightedStrategy implements weighted replica selection
type WeightedStrategy struct {
    weights []int
    mu      sync.RWMutex
}

func (ws *WeightedStrategy) SelectReplica(replicas []*sqlx.DB, metrics *RoutingMetrics) *sqlx.DB {
    ws.mu.RLock()
    defer ws.mu.RUnlock()
    
    // Implementation of weighted selection based on replica performance
    totalWeight := 0
    for _, weight := range ws.weights {
        totalWeight += weight
    }
    
    // Simplified weighted selection
    if totalWeight > 0 && len(ws.weights) == len(replicas) {
        return replicas[0] // Simplified for example
    }
    
    return replicas[0]
}
```

### ヘルスチェックとフェイルオーバー

```go
// HealthChecker monitors database health
type HealthChecker struct {
    cluster     *DBCluster
    healthMap   map[*sqlx.DB]bool
    mu          sync.RWMutex
    checkInterval time.Duration
}

func NewHealthChecker(cluster *DBCluster, checkInterval time.Duration) *HealthChecker {
    hc := &HealthChecker{
        cluster:       cluster,
        healthMap:     make(map[*sqlx.DB]bool),
        checkInterval: checkInterval,
    }
    
    // Initialize health status
    hc.healthMap[cluster.primary] = true
    for _, replica := range cluster.replicas {
        hc.healthMap[replica] = true
    }
    
    return hc
}

func (hc *HealthChecker) Start() {
    go func() {
        ticker := time.NewTicker(hc.checkInterval)
        defer ticker.Stop()
        
        for range ticker.C {
            hc.checkHealth()
        }
    }()
}

func (hc *HealthChecker) checkHealth() {
    hc.mu.Lock()
    defer hc.mu.Unlock()
    
    // Check primary
    hc.healthMap[hc.cluster.primary] = hc.pingDatabase(hc.cluster.primary)
    
    // Check replicas
    for _, replica := range hc.cluster.replicas {
        hc.healthMap[replica] = hc.pingDatabase(replica)
    }
}

func (hc *HealthChecker) pingDatabase(db *sqlx.DB) bool {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    return db.PingContext(ctx) == nil
}

func (hc *HealthChecker) GetHealthyReplicas() []*sqlx.DB {
    hc.mu.RLock()
    defer hc.mu.RUnlock()
    
    healthy := make([]*sqlx.DB, 0)
    for _, replica := range hc.cluster.replicas {
        if hc.healthMap[replica] {
            healthy = append(healthy, replica)
        }
    }
    
    return healthy
}
```

### レプリケーションラグの考慮

```go
// ReplicationLagDetector monitors replication lag
type ReplicationLagDetector struct {
    cluster *DBCluster
    maxLag  time.Duration
}

func NewReplicationLagDetector(cluster *DBCluster, maxLag time.Duration) *ReplicationLagDetector {
    return &ReplicationLagDetector{
        cluster: cluster,
        maxLag:  maxLag,
    }
}

func (rld *ReplicationLagDetector) CheckReplicationLag(ctx context.Context) (map[*sqlx.DB]time.Duration, error) {
    lagMap := make(map[*sqlx.DB]time.Duration)
    
    // Get current time from primary
    var primaryTime time.Time
    err := rld.cluster.GetPrimary().GetContext(ctx, &primaryTime, "SELECT NOW()")
    if err != nil {
        return nil, err
    }
    
    // Check lag for each replica
    for _, replica := range rld.cluster.replicas {
        var replicaTime time.Time
        err := replica.GetContext(ctx, &replicaTime, "SELECT NOW()")
        if err != nil {
            lagMap[replica] = time.Hour // Mark as severely lagged
            continue
        }
        
        lag := primaryTime.Sub(replicaTime)
        if lag < 0 {
            lag = 0 // Replica might be ahead due to clock differences
        }
        
        lagMap[replica] = lag
    }
    
    return lagMap, nil
}

func (rld *ReplicationLagDetector) GetLowLagReplicas(ctx context.Context) ([]*sqlx.DB, error) {
    lagMap, err := rld.CheckReplicationLag(ctx)
    if err != nil {
        return nil, err
    }
    
    lowLagReplicas := make([]*sqlx.DB, 0)
    for replica, lag := range lagMap {
        if lag <= rld.maxLag {
            lowLagReplicas = append(lowLagReplicas, replica)
        }
    }
    
    return lowLagReplicas, nil
}
```

### トランザクション処理

```go
// TransactionManager handles transactions with read-replica awareness
type TransactionManager struct {
    cluster *DBCluster
}

func NewTransactionManager(cluster *DBCluster) *TransactionManager {
    return &TransactionManager{cluster: cluster}
}

// WithReadOnlyTransaction executes read-only operations in a transaction
func (tm *TransactionManager) WithReadOnlyTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
    db := tm.cluster.GetReplica()
    
    tx, err := db.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    if err := fn(tx); err != nil {
        return err
    }
    
    return tx.Commit()
}

// WithWriteTransaction executes write operations in a transaction
func (tm *TransactionManager) WithWriteTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
    db := tm.cluster.GetPrimary()
    
    tx, err := db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    if err := fn(tx); err != nil {
        return err
    }
    
    return tx.Commit()
}
```

📝 **課題**

以下の機能を持つRead-Replica分散システムを実装してください：

1. **`DBCluster`**: Primary/Replicaクラスター管理
2. **`RoutingManager`**: 読み書き操作のルーティング
3. **`HealthMonitor`**: データベースヘルスチェック
4. **`LagDetector`**: レプリケーションラグ監視
5. **`LoadBalancer`**: レプリカ間の負荷分散
6. **`FailoverManager`**: 自動フェイルオーバー機能

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestDBCluster_BasicOperations
--- PASS: TestDBCluster_BasicOperations (0.02s)
=== RUN   TestRoutingManager_ReadWriteSplit
--- PASS: TestRoutingManager_ReadWriteSplit (0.05s)
=== RUN   TestHealthMonitor_FailureDetection
--- PASS: TestHealthMonitor_FailureDetection (0.10s)
=== RUN   TestLagDetector_ReplicationMonitoring
--- PASS: TestLagDetector_ReplicationMonitoring (0.08s)
=== RUN   TestLoadBalancer_ReplicaSelection
--- PASS: TestLoadBalancer_ReplicaSelection (0.03s)
PASS
ok      day40-read-replica    0.280s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **Context**: タイムアウト付きデータベース操作
2. **sync**パッケージ: 並行安全なルーティング管理
3. **time**パッケージ: ヘルスチェック間隔とラグ測定
4. **database/sql**: 読み取り専用トランザクション
5. **Load balancing**: ラウンドロビン、重み付け選択

Read-Replica設計のポイント：
- **読み書き分離**: 明確な操作の分類
- **フェイルオーバー**: Primary障害時のレプリカ昇格
- **ラグ監視**: データ整合性の管理
- **ヘルスチェック**: 障害の早期検出

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```