# Day 40: Read-Replica分散システムと高可用性DB設計

## 🎯 本日の目標

このチャレンジを通して、以下のスキルを身につけることができます：

- **Read-Replicaを活用した効率的な負荷分散システムを構築できるようになる**
- **Master/Slave構成での自動フェールオーバー機能を実装できるようになる**
- **レプリケーション遅延を考慮した堅牢なデータ整合性管理をマスターする**
- **プロダクション環境でのRead-Replica運用ベストプラクティスを習得する**

## 📖 解説

### なぜRead-Replicaが必要なのか？

現代のWebアプリケーションでは、読み取り操作が書き込み操作の10倍以上発生することが一般的です。単一のデータベースでは以下の問題が発生します：

#### 単一DB構成の限界

```go
// 問題のある例：全てのクエリが一つのDBに集中
func GetUserDashboard(db *sql.DB, userID int) (*Dashboard, error) {
    // 以下のクエリが全て同一DBで実行される
    
    // 1. ユーザー基本情報
    user, err := getUserBasicInfo(db, userID)
    if err != nil {
        return nil, err
    }
    
    // 2. 最近の注文履歴（複雑なJOIN）
    orders, err := getRecentOrders(db, userID)
    if err != nil {
        return nil, err
    }
    
    // 3. 推奨商品（重いアルゴリズム）
    recommendations, err := getRecommendations(db, userID)
    if err != nil {
        return nil, err
    }
    
    // 4. 統計データ（集計クエリ）
    stats, err := getUserStats(db, userID)
    if err != nil {
        return nil, err
    }
    
    return &Dashboard{
        User:            user,
        RecentOrders:    orders,
        Recommendations: recommendations,
        Stats:          stats,
    }, nil
}

func getRecommendations(db *sql.DB, userID int) ([]Product, error) {
    // 非常に重い集計クエリの例
    query := `
        WITH user_preferences AS (
            SELECT category_id, COUNT(*) as purchase_count
            FROM orders o
            JOIN order_items oi ON o.id = oi.order_id
            JOIN products p ON oi.product_id = p.id
            WHERE o.user_id = $1
            GROUP BY category_id
        ),
        similar_users AS (
            SELECT DISTINCT o2.user_id, 
                   COUNT(*) as similarity_score
            FROM orders o1
            JOIN orders o2 ON o1.product_id = o2.product_id
            WHERE o1.user_id = $1 AND o2.user_id != $1
            GROUP BY o2.user_id
            HAVING COUNT(*) >= 3
            ORDER BY similarity_score DESC
            LIMIT 100
        )
        SELECT p.id, p.name, p.price, AVG(r.rating) as avg_rating
        FROM products p
        JOIN order_items oi ON p.id = oi.product_id
        JOIN orders o ON oi.order_id = o.id
        JOIN similar_users su ON o.user_id = su.user_id
        LEFT JOIN reviews r ON p.id = r.product_id
        WHERE p.category_id IN (SELECT category_id FROM user_preferences)
        GROUP BY p.id, p.name, p.price
        HAVING COUNT(DISTINCT o.user_id) >= 5
        ORDER BY avg_rating DESC, COUNT(DISTINCT o.user_id) DESC
        LIMIT 10
    `
    
    // このクエリが5秒かかると、他の軽いクエリも影響を受ける
    rows, err := db.Query(query, userID)
    // ...
}
```

**問題点：**
- **リソース競合**: 重いクエリが軽いクエリをブロック
- **単一障害点**: DBがダウンするとサービス全体が停止
- **スケーラビリティ限界**: 垂直スケーリングのみでコストが増大
- **地理的制約**: 全てのユーザーが同一拠点のDBにアクセス

### Read-Replicaによる劇的な改善

Read-Replica（読み取りレプリカ）は、Master/Primary DBから非同期でデータを複製し、読み取り専用操作を分散する仕組みです：

```go
type DatabaseCluster struct {
    master   *sql.DB
    replicas []*sql.DB
    
    // ヘルスチェック機能
    replicaHealth   map[int]bool
    healthMutex     sync.RWMutex
    
    // ロードバランサー
    loadBalancer    ReplicaLoadBalancer
    
    // 設定
    config          ClusterConfig
    
    // メトリクス
    metrics         *ClusterMetrics
}

type ClusterConfig struct {
    MaxRetries           int
    HealthCheckInterval  time.Duration
    ReplicationLagThreshold time.Duration
    PreferLocalReplica   bool
    Region              string
}

type ClusterMetrics struct {
    QueriesTotal        map[string]int64 // master/replica別
    QueryDuration       map[string]time.Duration
    FailoverCount       int64
    ReplicationLag      map[int]time.Duration
    mu                  sync.RWMutex
}

func NewDatabaseCluster(masterDSN string, replicaDSNs []string, config ClusterConfig) (*DatabaseCluster, error) {
    // Master接続
    master, err := sql.Open("postgres", masterDSN)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to master: %w", err)
    }
    
    // Replica接続
    replicas := make([]*sql.DB, len(replicaDSNs))
    replicaHealth := make(map[int]bool)
    
    for i, dsn := range replicaDSNs {
        replica, err := sql.Open("postgres", dsn)
        if err != nil {
            // 他のレプリカが利用可能な場合は継続
            log.Printf("Failed to connect to replica %d: %v", i, err)
            replicaHealth[i] = false
            continue
        }
        
        replicas[i] = replica
        replicaHealth[i] = true
    }
    
    cluster := &DatabaseCluster{
        master:        master,
        replicas:      replicas,
        replicaHealth: replicaHealth,
        config:        config,
        metrics:       &ClusterMetrics{
            QueriesTotal:  make(map[string]int64),
            QueryDuration: make(map[string]time.Duration),
            ReplicationLag: make(map[int]time.Duration),
        },
        loadBalancer:  NewWeightedRoundRobinBalancer(),
    }
    
    // ヘルスチェック開始
    go cluster.startHealthChecking()
    
    return cluster, nil
}
```

**改善効果：**
- **負荷分散**: 読み取りクエリがレプリカに分散され、Masterの負荷が80%削減
- **高可用性**: レプリカ障害時も他のレプリカで継続サービス
- **地理的分散**: 各リージョンのレプリカでレイテンシ削減
- **水平スケーリング**: 読み取り性能をレプリカ追加で線形拡張

### 高度なRead-Replica管理システム

#### 1. インテリジェントクエリルーティング

```go
type QueryRouter struct {
    cluster      *DatabaseCluster
    analyzer     *QueryAnalyzer
    ruleEngine   *RoutingRuleEngine
}

type QueryAnalyzer struct {
    readPatterns  []string
    writePatterns []string
    heavyPatterns []string
}

func NewQueryAnalyzer() *QueryAnalyzer {
    return &QueryAnalyzer{
        readPatterns: []string{
            `^SELECT\s+`,
            `^WITH\s+.+\s+SELECT\s+`,
            `^EXPLAIN\s+`,
        },
        writePatterns: []string{
            `^INSERT\s+`,
            `^UPDATE\s+`,
            `^DELETE\s+`,
            `^CREATE\s+`,
            `^DROP\s+`,
            `^ALTER\s+`,
        },
        heavyPatterns: []string{
            `JOIN\s+.*JOIN\s+.*JOIN`, // 複数JOINは重いクエリ
            `GROUP\s+BY\s+.*HAVING`,   // 集計クエリ
            `ORDER\s+BY\s+.*LIMIT\s+\d{3,}`, // 大量ソート
            `COUNT\(\*\).*FROM\s+\w+\s*$`, // 全件COUNT
        },
    }
}

func (qa *QueryAnalyzer) AnalyzeQuery(query string) QueryType {
    normalizedQuery := strings.ToUpper(strings.TrimSpace(query))
    
    // 書き込みクエリの判定
    for _, pattern := range qa.writePatterns {
        if matched, _ := regexp.MatchString(pattern, normalizedQuery); matched {
            return QueryTypeWrite
        }
    }
    
    // 重いクエリの判定
    for _, pattern := range qa.heavyPatterns {
        if matched, _ := regexp.MatchString(pattern, normalizedQuery); matched {
            return QueryTypeHeavyRead
        }
    }
    
    // 読み取りクエリの判定
    for _, pattern := range qa.readPatterns {
        if matched, _ := regexp.MatchString(pattern, normalizedQuery); matched {
            return QueryTypeLightRead
        }
    }
    
    // デフォルトは書き込みとして扱う（安全側）
    return QueryTypeWrite
}

type QueryType int

const (
    QueryTypeWrite QueryType = iota
    QueryTypeLightRead
    QueryTypeHeavyRead
)

type RoutingRuleEngine struct {
    rules []RoutingRule
}

type RoutingRule struct {
    Condition func(QueryContext) bool
    Action    RoutingAction
    Priority  int
}

type QueryContext struct {
    Query           string
    QueryType       QueryType
    UserID          int
    RequiredConsistency ConsistencyLevel
    Timeout         time.Duration
    Tags            map[string]string
}

type RoutingAction struct {
    TargetType    DatabaseTargetType
    MaxLag        time.Duration
    Fallback      DatabaseTargetType
    StickySessions bool
}

type DatabaseTargetType int

const (
    TargetMaster DatabaseTargetType = iota
    TargetAnyReplica
    TargetLocalReplica
    TargetSpecificReplica
)

type ConsistencyLevel int

const (
    ConsistencyEventual ConsistencyLevel = iota
    ConsistencyReadAfterWrite
    ConsistencyStrong
)

func (qr *QueryRouter) RouteQuery(ctx context.Context, queryCtx QueryContext) (*sql.DB, error) {
    // ルールエンジンで最適なターゲットを決定
    action := qr.ruleEngine.EvaluateRules(queryCtx)
    
    switch action.TargetType {
    case TargetMaster:
        return qr.cluster.master, nil
        
    case TargetAnyReplica:
        return qr.selectHealthyReplica(ctx, action.MaxLag)
        
    case TargetLocalReplica:
        return qr.selectLocalReplica(ctx, action.MaxLag)
        
    default:
        // フォールバック処理
        if action.Fallback == TargetMaster {
            return qr.cluster.master, nil
        }
        return qr.selectHealthyReplica(ctx, action.MaxLag)
    }
}

func (qr *QueryRouter) selectHealthyReplica(ctx context.Context, maxLag time.Duration) (*sql.DB, error) {
    qr.cluster.healthMutex.RLock()
    defer qr.cluster.healthMutex.RUnlock()
    
    var candidates []ReplicaCandidate
    
    for i, replica := range qr.cluster.replicas {
        if !qr.cluster.replicaHealth[i] {
            continue
        }
        
        // レプリケーション遅延をチェック
        lag, exists := qr.cluster.metrics.ReplicationLag[i]
        if exists && lag > maxLag {
            continue
        }
        
        candidates = append(candidates, ReplicaCandidate{
            Index:    i,
            Database: replica,
            Lag:      lag,
            Weight:   qr.calculateReplicaWeight(i),
        })
    }
    
    if len(candidates) == 0 {
        // 利用可能なレプリカがない場合はマスターにフォールバック
        qr.cluster.metrics.mu.Lock()
        qr.cluster.metrics.FailoverCount++
        qr.cluster.metrics.mu.Unlock()
        
        return qr.cluster.master, nil
    }
    
    // ロードバランサーで最適なレプリカを選択
    selected := qr.cluster.loadBalancer.SelectReplica(candidates)
    return selected.Database, nil
}

type ReplicaCandidate struct {
    Index    int
    Database *sql.DB
    Lag      time.Duration
    Weight   float64
}

func (qr *QueryRouter) calculateReplicaWeight(replicaIndex int) float64 {
    // CPU使用率、メモリ使用率、ネットワークレイテンシなどを考慮
    baseWeight := 1.0
    
    // レプリケーション遅延によるペナルティ
    if lag, exists := qr.cluster.metrics.ReplicationLag[replicaIndex]; exists {
        lagPenalty := float64(lag.Milliseconds()) / 1000.0
        baseWeight -= lagPenalty * 0.1
    }
    
    // クエリ負荷によるペナルティ
    if qr.cluster.metrics.QueriesTotal["replica_"+string(rune(replicaIndex))] > 1000 {
        baseWeight -= 0.2
    }
    
    return math.Max(0.1, baseWeight)
}
```

#### 2. レプリケーション遅延監視と自動調整

```go
type ReplicationLagMonitor struct {
    cluster       *DatabaseCluster
    ticker        *time.Ticker
    done          chan struct{}
    alertManager  *AlertManager
}

func NewReplicationLagMonitor(cluster *DatabaseCluster, interval time.Duration) *ReplicationLagMonitor {
    return &ReplicationLagMonitor{
        cluster:      cluster,
        ticker:       time.NewTicker(interval),
        done:         make(chan struct{}),
        alertManager: NewAlertManager(),
    }
}

func (rlm *ReplicationLagMonitor) Start() {
    go func() {
        defer rlm.ticker.Stop()
        for {
            select {
            case <-rlm.ticker.C:
                rlm.checkReplicationLag()
            case <-rlm.done:
                return
            }
        }
    }()
}

func (rlm *ReplicationLagMonitor) checkReplicationLag() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // マスターから現在のLSNを取得
    var masterLSN string
    err := rlm.cluster.master.QueryRowContext(ctx, 
        "SELECT pg_current_wal_lsn()").Scan(&masterLSN)
    if err != nil {
        log.Printf("Failed to get master LSN: %v", err)
        return
    }
    
    // 各レプリカの遅延を測定
    for i, replica := range rlm.cluster.replicas {
        if replica == nil {
            continue
        }
        
        lag, err := rlm.measureLag(ctx, replica, masterLSN)
        if err != nil {
            log.Printf("Failed to measure lag for replica %d: %v", i, err)
            rlm.markReplicaUnhealthy(i)
            continue
        }
        
        // 遅延情報を更新
        rlm.cluster.metrics.mu.Lock()
        rlm.cluster.metrics.ReplicationLag[i] = lag
        rlm.cluster.metrics.mu.Unlock()
        
        // 閾値チェック
        if lag > rlm.cluster.config.ReplicationLagThreshold {
            rlm.alertManager.SendAlert(Alert{
                Level:   "WARNING",
                Message: fmt.Sprintf("Replica %d lag: %v", i, lag),
                Time:    time.Now(),
            })
            
            // 一時的にレプリカを無効化
            rlm.markReplicaUnhealthy(i)
        } else {
            rlm.markReplicaHealthy(i)
        }
    }
}

func (rlm *ReplicationLagMonitor) measureLag(ctx context.Context, replica *sql.DB, masterLSN string) (time.Duration, error) {
    var replicaLSN string
    var lagBytes int64
    
    query := `
        SELECT 
            pg_last_wal_receive_lsn(),
            pg_wal_lsn_diff($1, pg_last_wal_replay_lsn())
    `
    
    err := replica.QueryRowContext(ctx, query, masterLSN).Scan(&replicaLSN, &lagBytes)
    if err != nil {
        return 0, err
    }
    
    // バイト数を時間に変換（概算）
    // 平均的なWAL生成速度を考慮
    avgWALSpeed := int64(1024 * 1024) // 1MB/sec
    lagSeconds := lagBytes / avgWALSpeed
    
    return time.Duration(lagSeconds) * time.Second, nil
}

func (rlm *ReplicationLagMonitor) markReplicaUnhealthy(index int) {
    rlm.cluster.healthMutex.Lock()
    defer rlm.cluster.healthMutex.Unlock()
    rlm.cluster.replicaHealth[index] = false
}

func (rlm *ReplicationLagMonitor) markReplicaHealthy(index int) {
    rlm.cluster.healthMutex.Lock()
    defer rlm.cluster.healthMutex.Unlock()
    rlm.cluster.replicaHealth[index] = true
}
```

#### 3. 読み取り後書き込み整合性の保証

```go
type ConsistentDBManager struct {
    cluster         *DatabaseCluster
    router          *QueryRouter
    sessionManager  *SessionManager
}

type SessionManager struct {
    sessions map[string]*Session
    mu       sync.RWMutex
    ttl      time.Duration
}

type Session struct {
    UserID           int
    LastWriteTime    time.Time
    PreferredReplica int
    StickUntil       time.Time
}

func (cm *ConsistentDBManager) ExecuteWithConsistency(
    ctx context.Context, 
    sessionID string, 
    query string, 
    consistency ConsistencyLevel,
    args ...interface{},
) (*sql.Rows, error) {
    
    queryType := cm.router.analyzer.AnalyzeQuery(query)
    
    switch consistency {
    case ConsistencyStrong:
        // 強い整合性が必要な場合は常にマスター
        return cm.cluster.master.QueryContext(ctx, query, args...)
        
    case ConsistencyReadAfterWrite:
        // 書き込み後読み取り整合性
        return cm.handleReadAfterWrite(ctx, sessionID, queryType, query, args...)
        
    case ConsistencyEventual:
        // 結果整合性（デフォルト）
        return cm.handleEventualConsistency(ctx, queryType, query, args...)
        
    default:
        return nil, fmt.Errorf("unknown consistency level: %v", consistency)
    }
}

func (cm *ConsistentDBManager) handleReadAfterWrite(
    ctx context.Context, 
    sessionID string, 
    queryType QueryType, 
    query string, 
    args ...interface{},
) (*sql.Rows, error) {
    
    session := cm.sessionManager.GetSession(sessionID)
    
    if queryType == QueryTypeWrite {
        // 書き込みは常にマスター
        rows, err := cm.cluster.master.QueryContext(ctx, query, args...)
        if err != nil {
            return nil, err
        }
        
        // セッション情報を更新
        cm.sessionManager.UpdateSession(sessionID, Session{
            UserID:        session.UserID,
            LastWriteTime: time.Now(),
            StickUntil:    time.Now().Add(10 * time.Second), // 10秒間はマスターから読み取り
        })
        
        return rows, nil
    }
    
    // 読み取りクエリの場合
    if session != nil && time.Now().Before(session.StickUntil) {
        // 最近書き込みを行った場合はマスターから読み取り
        return cm.cluster.master.QueryContext(ctx, query, args...)
    }
    
    // 通常のレプリカルーティング
    queryCtx := QueryContext{
        Query:               query,
        QueryType:           queryType,
        RequiredConsistency: ConsistencyEventual,
        Timeout:             5 * time.Second,
    }
    
    db, err := cm.router.RouteQuery(ctx, queryCtx)
    if err != nil {
        return nil, err
    }
    
    return db.QueryContext(ctx, query, args...)
}

func (cm *ConsistentDBManager) handleEventualConsistency(
    ctx context.Context, 
    queryType QueryType, 
    query string, 
    args ...interface{},
) (*sql.Rows, error) {
    
    if queryType == QueryTypeWrite {
        return cm.cluster.master.QueryContext(ctx, query, args...)
    }
    
    // 読み取りクエリはレプリカに分散
    queryCtx := QueryContext{
        Query:               query,
        QueryType:           queryType,
        RequiredConsistency: ConsistencyEventual,
        Timeout:             5 * time.Second,
    }
    
    db, err := cm.router.RouteQuery(ctx, queryCtx)
    if err != nil {
        return nil, err
    }
    
    return db.QueryContext(ctx, query, args...)
}

func (sm *SessionManager) GetSession(sessionID string) *Session {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    session, exists := sm.sessions[sessionID]
    if !exists {
        return &Session{}
    }
    
    // TTLチェック
    if time.Since(session.LastWriteTime) > sm.ttl {
        delete(sm.sessions, sessionID)
        return &Session{}
    }
    
    return session
}

func (sm *SessionManager) UpdateSession(sessionID string, session Session) {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    sm.sessions[sessionID] = &session
}
```

#### 4. フェールオーバーと自動復旧

```go
type FailoverManager struct {
    cluster        *DatabaseCluster
    healthMonitor  *HealthMonitor
    alertManager   *AlertManager
    failoverPolicy FailoverPolicy
}

type FailoverPolicy struct {
    HealthCheckInterval   time.Duration
    FailureThreshold      int
    RecoveryThreshold     int
    AutoPromoteReplica    bool
    MaxPromotionAttempts  int
}

type HealthMonitor struct {
    cluster           *DatabaseCluster
    failures          map[int]int  // replica index -> failure count
    successes         map[int]int  // replica index -> success count
    lastHealthCheck   map[int]time.Time
    mu                sync.RWMutex
}

func (fm *FailoverManager) StartMonitoring() {
    go fm.monitorHealth()
}

func (fm *FailoverManager) monitorHealth() {
    ticker := time.NewTicker(fm.failoverPolicy.HealthCheckInterval)
    defer ticker.Stop()
    
    for range ticker.C {
        fm.checkAllDatabases()
    }
}

func (fm *FailoverManager) checkAllDatabases() {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // マスターのヘルスチェック
    if err := fm.checkMasterHealth(ctx); err != nil {
        fm.handleMasterFailure(err)
    }
    
    // レプリカのヘルスチェック
    for i, replica := range fm.cluster.replicas {
        if replica == nil {
            continue
        }
        
        if err := fm.checkReplicaHealth(ctx, i, replica); err != nil {
            fm.handleReplicaFailure(i, err)
        } else {
            fm.handleReplicaRecovery(i)
        }
    }
}

func (fm *FailoverManager) checkMasterHealth(ctx context.Context) error {
    var result int
    query := "SELECT 1"
    
    err := fm.cluster.master.QueryRowContext(ctx, query).Scan(&result)
    if err != nil {
        return fmt.Errorf("master health check failed: %w", err)
    }
    
    if result != 1 {
        return fmt.Errorf("master health check returned unexpected value: %d", result)
    }
    
    return nil
}

func (fm *FailoverManager) checkReplicaHealth(ctx context.Context, index int, replica *sql.DB) error {
    var result int
    query := "SELECT 1"
    
    err := replica.QueryRowContext(ctx, query).Scan(&result)
    if err != nil {
        return fmt.Errorf("replica %d health check failed: %w", index, err)
    }
    
    if result != 1 {
        return fmt.Errorf("replica %d health check returned unexpected value: %d", index, result)
    }
    
    return nil
}

func (fm *FailoverManager) handleMasterFailure(err error) {
    fm.alertManager.SendAlert(Alert{
        Level:   "CRITICAL",
        Message: fmt.Sprintf("Master database failure: %v", err),
        Time:    time.Now(),
    })
    
    if fm.failoverPolicy.AutoPromoteReplica {
        fm.attemptReplicaPromotion()
    }
}

func (fm *FailoverManager) handleReplicaFailure(index int, err error) {
    fm.healthMonitor.mu.Lock()
    defer fm.healthMonitor.mu.Unlock()
    
    fm.healthMonitor.failures[index]++
    fm.healthMonitor.successes[index] = 0
    
    if fm.healthMonitor.failures[index] >= fm.failoverPolicy.FailureThreshold {
        // レプリカを無効化
        fm.cluster.healthMutex.Lock()
        fm.cluster.replicaHealth[index] = false
        fm.cluster.healthMutex.Unlock()
        
        fm.alertManager.SendAlert(Alert{
            Level:   "WARNING",
            Message: fmt.Sprintf("Replica %d marked as unhealthy after %d failures", 
                               index, fm.healthMonitor.failures[index]),
            Time:    time.Now(),
        })
    }
}

func (fm *FailoverManager) handleReplicaRecovery(index int) {
    fm.healthMonitor.mu.Lock()
    defer fm.healthMonitor.mu.Unlock()
    
    fm.healthMonitor.successes[index]++
    
    if fm.healthMonitor.successes[index] >= fm.failoverPolicy.RecoveryThreshold {
        // レプリカを復旧
        fm.cluster.healthMutex.Lock()
        fm.cluster.replicaHealth[index] = true
        fm.cluster.healthMutex.Unlock()
        
        // カウンターをリセット
        fm.healthMonitor.failures[index] = 0
        
        fm.alertManager.SendAlert(Alert{
            Level:   "INFO",
            Message: fmt.Sprintf("Replica %d recovered and marked as healthy", index),
            Time:    time.Now(),
        })
    }
}

func (fm *FailoverManager) attemptReplicaPromotion() {
    // 最も健全なレプリカを選択してマスターに昇格
    bestReplica := fm.selectBestReplicaForPromotion()
    if bestReplica == -1 {
        fm.alertManager.SendAlert(Alert{
            Level:   "CRITICAL",
            Message: "No healthy replica available for promotion",
            Time:    time.Now(),
        })
        return
    }
    
    // レプリカをマスターに昇格（実際のDBクラスター設定による）
    if err := fm.promoteReplica(bestReplica); err != nil {
        fm.alertManager.SendAlert(Alert{
            Level:   "CRITICAL",
            Message: fmt.Sprintf("Failed to promote replica %d: %v", bestReplica, err),
            Time:    time.Now(),
        })
        return
    }
    
    fm.alertManager.SendAlert(Alert{
        Level:   "INFO",
        Message: fmt.Sprintf("Successfully promoted replica %d to master", bestReplica),
        Time:    time.Now(),
    })
}

func (fm *FailoverManager) selectBestReplicaForPromotion() int {
    fm.healthMonitor.mu.RLock()
    defer fm.healthMonitor.mu.RUnlock()
    
    bestIndex := -1
    minLag := time.Hour // 初期値は十分大きな値
    
    for i, replica := range fm.cluster.replicas {
        if replica == nil || !fm.cluster.replicaHealth[i] {
            continue
        }
        
        lag, exists := fm.cluster.metrics.ReplicationLag[i]
        if !exists {
            continue
        }
        
        if lag < minLag {
            minLag = lag
            bestIndex = i
        }
    }
    
    return bestIndex
}

func (fm *FailoverManager) promoteReplica(index int) error {
    // この実装はPostgreSQLのストリーミングレプリケーション環境を想定
    // 実際の環境では、外部のクラスター管理ツール（Patroni、pg_auto_failoverなど）を使用
    
    replica := fm.cluster.replicas[index]
    
    // レプリカをマスターモードに昇格
    _, err := replica.Exec("SELECT pg_promote()")
    if err != nil {
        return fmt.Errorf("failed to promote replica: %w", err)
    }
    
    // 新しいマスターに切り替え
    oldMaster := fm.cluster.master
    fm.cluster.master = replica
    
    // 古いマスターをクローズ
    if err := oldMaster.Close(); err != nil {
        log.Printf("Failed to close old master connection: %v", err)
    }
    
    // レプリカリストから削除
    fm.cluster.replicas[index] = nil
    delete(fm.cluster.replicaHealth, index)
    
    return nil
}
```

📝 **課題**

以下の機能を持つRead-Replica分散システムを実装してください：

1. **`DatabaseCluster`**: Master/Replica構成の管理
2. **`QueryRouter`**: インテリジェントなクエリルーティング
3. **`ConsistencyManager`**: データ整合性レベルの制御
4. **`FailoverManager`**: 自動フェールオーバー機能
5. **`PerformanceMonitor`**: 負荷分散効果の測定
6. **統合テスト**: 実際のレプリケーション環境でのテスト

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestDatabaseCluster_BasicRouting
--- PASS: TestDatabaseCluster_BasicRouting (0.05s)
=== RUN   TestQueryRouter_IntelligentRouting
--- PASS: TestQueryRouter_IntelligentRouting (0.08s)
=== RUN   TestConsistencyManager_ReadAfterWrite
--- PASS: TestConsistencyManager_ReadAfterWrite (0.10s)
=== RUN   TestFailoverManager_AutoRecovery
--- PASS: TestFailoverManager_AutoRecovery (0.15s)
=== RUN   TestPerformanceMonitor_LoadDistribution
--- PASS: TestPerformanceMonitor_LoadDistribution (0.12s)
PASS
ok      day40-read-replica    0.500s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **PostgreSQL Streaming Replication**: WAL-based レプリケーション
2. **Connection Pooling**: pgxpool や sqlx での接続管理
3. **Load Balancing**: Weighted Round Robin や Least Connections
4. **Health Checking**: 定期的なping とクエリ実行テスト
5. **Consistency Models**: CAP定理と実用的なトレードオフ

設計のポイント：
- **クエリ分析**: 正規表現による読み書き判定の精度
- **レプリケーション遅延**: PostgreSQLのLSN を使った正確な測定
- **フェールオーバー**: 段階的な障害検出と自動復旧
- **セッション管理**: スティッキーセッションによる整合性保証

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```

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