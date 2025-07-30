# Day 38: DBインデックス最適化とクエリ分析

## 🎯 本日の目標

このチャレンジを通して、以下のスキルを身につけることができます：

- **EXPLAINを使ったクエリ実行計画の詳細分析ができるようになる**
- **インデックス戦略の立案と効果測定を実践できるようになる**
- **クエリパフォーマンスのボトルネック特定と改善ができるようになる**
- **プロダクション環境でのインデックス運用管理をマスターする**

## 📖 解説

### データベースインデックスとは何か？

```go
// 【DBインデックス最適化の重要性】大規模システムでのパフォーマンス生死を分ける技術
// ❌ 問題例：インデックス設計ミスによる本番システム完全停止と業務麻痺
func catastrophicIndexMismanagement() {
    // 🚨 災害例：インデックス不備による壊滅的システム障害とビジネス停止
    
    // ❌ 最悪のテーブル設計：1億件ユーザーテーブルでのインデックス地獄
    /*
    CREATE TABLE users (
        id SERIAL PRIMARY KEY,           -- 唯一のインデックス
        email VARCHAR(255),              -- インデックスなし！
        username VARCHAR(100),           -- インデックスなし！
        phone VARCHAR(20),               -- インデックスなし！
        created_at TIMESTAMP,            -- インデックスなし！
        last_login TIMESTAMP,            -- インデックスなし！
        profile_data JSONB,              -- インデックスなし！
        tags TEXT[]                      -- インデックスなし！
    );
    
    -- 100,000,000行のデータが既に存在
    */
    
    // ❌ 災害的クエリ1：メール検索で全テーブルスキャン
    func LoginByEmailDisaster(db *sql.DB, email string) (*User, error) {
        query := `
            SELECT id, email, username, profile_data 
            FROM users 
            WHERE email = $1  -- インデックスなし！
        `
        
        // 【災害的実行計画】
        // Seq Scan on users (cost=0.00..2500000.00 rows=1 width=256)
        //   Filter: (email = 'user@example.com')
        //   Rows Removed by Filter: 99999999
        //   Execution Time: 67834.521 ms (67秒！)
        
        var user User
        err := db.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.Username, &user.ProfileData)
        // 結果：1回のログイン試行で67秒、全システム応答不能
        return &user, err
    }
    
    // ❌ 災害的クエリ2：範囲検索で完全死亡
    func GetRecentActiveUsersDisaster(db *sql.DB) ([]*User, error) {
        query := `
            SELECT id, email, username, last_login
            FROM users 
            WHERE last_login >= NOW() - INTERVAL '7 days'  -- インデックスなし！
            ORDER BY last_login DESC                        -- インデックスなし！
            LIMIT 1000
        `
        
        // 【災害的実行計画】
        // Sort (cost=3500000.00..3750000.00 rows=1000000 width=128)
        //   Sort Key: last_login DESC
        //   ->  Seq Scan on users (cost=0.00..2500000.00 rows=1000000 width=128)
        //         Filter: (last_login >= (now() - '7 days'::interval))
        //         Rows Removed by Filter: 99000000
        //   Execution Time: 123456.789 ms (123秒！)
        
        rows, err := db.Query(query)
        if err != nil {
            return nil, err
        }
        defer rows.Close()
        
        var users []*User
        for rows.Next() {
            var user User
            err := rows.Scan(&user.ID, &user.Email, &user.Username, &user.LastLogin)
            if err != nil {
                continue
            }
            users = append(users, &user)
        }
        
        // 結果：管理画面のアクティブユーザー表示に2分3秒、運用チーム業務停止
        return users, nil
    }
    
    // ❌ 災害的クエリ3：複合条件でシステム完全崩壊
    func SearchUsersDisaster(db *sql.DB, username string, startDate, endDate time.Time) ([]*User, error) {
        query := `
            SELECT u.id, u.email, u.username, u.created_at,
                   COUNT(o.id) as order_count
            FROM users u
            LEFT JOIN orders o ON u.id = o.user_id  -- orders.user_idにもインデックスなし！
            WHERE u.username ILIKE $1               -- インデックスなし！
            AND u.created_at BETWEEN $2 AND $3     -- インデックスなし！
            GROUP BY u.id, u.email, u.username, u.created_at
            ORDER BY order_count DESC               -- 計算結果のソート
            LIMIT 100
        `
        
        // 【災害的実行計画】
        // Sort (cost=15000000.00..15500000.00 rows=10000000 width=256)
        //   Sort Key: (count(o.id)) DESC
        //   ->  HashAggregate (cost=12000000.00..13000000.00 rows=10000000 width=256)
        //         Group Key: u.id, u.email, u.username, u.created_at
        //         ->  Hash Left Join (cost=5000000.00..10000000.00 rows=50000000 width=128)
        //               Hash Cond: (u.id = o.user_id)
        //               ->  Seq Scan on users u (cost=0.00..2500000.00 rows=1000000 width=64)
        //                     Filter: ((username ~~* $1) AND (created_at >= $2) AND (created_at <= $3))
        //                     Rows Removed by Filter: 99000000
        //               ->  Hash (cost=1500000.00..1500000.00 rows=50000000 width=8)
        //                     ->  Seq Scan on orders o (cost=0.00..1500000.00 rows=50000000 width=8)
        //   Execution Time: 456789.123 ms (456秒 = 7分36秒！)
        
        rows, err := db.Query(query, username+"%", startDate, endDate)
        if err != nil {
            return nil, err
        }
        defer rows.Close()
        
        var users []*User
        for rows.Next() {
            var user User
            var orderCount int
            err := rows.Scan(&user.ID, &user.Email, &user.Username, &user.CreatedAt, &orderCount)
            if err != nil {
                continue
            }
            user.OrderCount = orderCount
            users = append(users, &user)
        }
        
        // 結果：管理者の顧客検索に7分36秒、顧客サポート業務完全停止
        return users, nil
    }
    
    // 【本番環境での実際の災害】
    // 1. ECサイト：商品検索に5分→売上80%減少、顧客離脱
    // 2. 銀行システム：口座残高照会に3分→ATM全台停止、顧客苦情殺到
    // 3. 医療システム：患者検索に8分→診療予約システム麻痺、病院業務停止
    // 4. 物流システム：配送状況確認に4分→配送追跡不能、顧客対応破綻
    
    // 【連鎖的被害】
    // - データベース接続プール枯渇
    // - アプリケーションサーバー応答タイムアウト
    // - ロードバランサー健全性チェック失敗
    // - 全システムサービス停止判定
    // - 緊急事態宣言、全社対策本部設置
    
    fmt.Println("❌ Index disaster caused 7+ minute queries and complete business shutdown!")
    // 結果：1クエリ456秒、全システム停止、損失数億円
}

// ✅ 正解：エンタープライズ級インデックス最適化システム
type EnterpriseIndexOptimizationSystem struct {
    // 【基本インデックス管理】
    indexAnalyzer    *IndexAnalyzer           // インデックス解析エンジン
    queryOptimizer   *QueryOptimizer          // クエリ最適化エンジン
    explainParser    *ExplainParser           // EXPLAIN結果パーサー
    
    // 【インデックス戦略】
    indexStrategy    *IndexStrategy           // インデックス戦略エンジン
    coveringAnalyzer *CoveringIndexAnalyzer   // カバリングインデックス解析
    compositeBuilder *CompositeIndexBuilder   // 複合インデックス構築
    
    // 【パフォーマンス監視】
    performanceMonitor *PerformanceMonitor    // パフォーマンス監視
    slowQueryDetector  *SlowQueryDetector     // スロークエリ検出
    indexUsageTracker  *IndexUsageTracker     // インデックス使用状況追跡
    
    // 【自動最適化】
    autoOptimizer     *AutoOptimizer          // 自動最適化エンジン
    recommendationEngine *RecommendationEngine // 推奨エンジン
    impactAnalyzer    *ImpactAnalyzer         // 影響度分析
    
    // 【メンテナンス】
    maintenanceScheduler *MaintenanceScheduler // メンテナンススケジューラー
    fragmentationAnalyzer *FragmentationAnalyzer // 断片化解析
    
    // 【マルチ環境対応】
    environmentManager *EnvironmentManager     // 環境管理
    migrationPlanner   *MigrationPlanner       // マイグレーション計画
    
    config           *IndexConfig             // 設定管理
    mu               sync.RWMutex             // 並行アクセス制御
}

// 【重要関数】包括的インデックス最適化システム初期化
func NewEnterpriseIndexOptimizationSystem(config *IndexConfig) *EnterpriseIndexOptimizationSystem {
    return &EnterpriseIndexOptimizationSystem{
        config:                config,
        indexAnalyzer:         NewIndexAnalyzer(),
        queryOptimizer:        NewQueryOptimizer(),
        explainParser:         NewExplainParser(),
        indexStrategy:         NewIndexStrategy(),
        coveringAnalyzer:      NewCoveringIndexAnalyzer(),
        compositeBuilder:      NewCompositeIndexBuilder(),
        performanceMonitor:    NewPerformanceMonitor(),
        slowQueryDetector:     NewSlowQueryDetector(),
        indexUsageTracker:     NewIndexUsageTracker(),
        autoOptimizer:         NewAutoOptimizer(),
        recommendationEngine:  NewRecommendationEngine(),
        impactAnalyzer:        NewImpactAnalyzer(),
        maintenanceScheduler:  NewMaintenanceScheduler(),
        fragmentationAnalyzer: NewFragmentationAnalyzer(),
        environmentManager:    NewEnvironmentManager(),
        migrationPlanner:      NewMigrationPlanner(),
    }
}

// 【実用例】最適化されたユーザー検索システム
func (eios *EnterpriseIndexOptimizationSystem) CreateOptimalIndexes(
    ctx context.Context,
    db *sql.DB,
) error {
    
    // 【STEP 1】既存インデックス状況分析
    currentIndexes, err := eios.indexAnalyzer.AnalyzeCurrentIndexes(ctx, db)
    if err != nil {
        return fmt.Errorf("failed to analyze current indexes: %w", err)
    }
    
    // 【STEP 2】クエリパターン分析
    queryPatterns, err := eios.slowQueryDetector.AnalyzeQueryPatterns(ctx, db)
    if err != nil {
        return fmt.Errorf("failed to analyze query patterns: %w", err)
    }
    
    // 【STEP 3】最適インデックス推奨
    recommendations := eios.recommendationEngine.GenerateIndexRecommendations(
        currentIndexes, queryPatterns)
    
    // 【STEP 4】影響度分析と安全性確認
    for _, recommendation := range recommendations {
        impact, err := eios.impactAnalyzer.AnalyzeImpact(ctx, db, recommendation)
        if err != nil {
            continue
        }
        
        if impact.RiskLevel > AcceptableRiskLevel {
            continue // 高リスクは スキップ
        }
        
        // 【STEP 5】段階的インデックス作成
        err = eios.createIndexSafely(ctx, db, recommendation)
        if err != nil {
            return fmt.Errorf("failed to create index %s: %w", recommendation.IndexName, err)
        }
    }
    
    return nil
}

// 【核心メソッド】安全なインデックス作成
func (eios *EnterpriseIndexOptimizationSystem) createIndexSafely(
    ctx context.Context,
    db *sql.DB,
    recommendation *IndexRecommendation,
) error {
    
    // 【安全対策1】CONCURRENTLY オプション使用
    createSQL := fmt.Sprintf(
        "CREATE INDEX CONCURRENTLY %s ON %s (%s)",
        recommendation.IndexName,
        recommendation.TableName,
        strings.Join(recommendation.Columns, ", "),
    )
    
    // 【安全対策2】タイムアウト設定
    ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Minute)
    defer cancel()
    
    // 【安全対策3】進捗監視
    go eios.monitorIndexCreation(ctx, db, recommendation.IndexName)
    
    // 【安全対策4】インデックス作成実行
    _, err := db.ExecContext(ctxWithTimeout, createSQL)
    if err != nil {
        return fmt.Errorf("index creation failed: %w", err)
    }
    
    // 【安全対策5】作成後検証
    isValid, err := eios.validateIndexCreation(ctx, db, recommendation.IndexName)
    if err != nil || !isValid {
        // インデックス削除
        dropSQL := fmt.Sprintf("DROP INDEX CONCURRENTLY %s", recommendation.IndexName)
        db.ExecContext(ctx, dropSQL)
        return fmt.Errorf("index validation failed, rolled back")
    }
    
    return nil
}

// 【実用例】最適化後のクエリ実行
func OptimizedUserLogin(db *sql.DB, email string) (*User, error) {
    // 事前作成インデックス: CREATE INDEX idx_users_email ON users(email);
    
    query := `
        SELECT id, email, username, profile_data 
        FROM users 
        WHERE email = $1  -- インデックスヒット！
    `
    
    // 【最適化後実行計画】
    // Index Scan using idx_users_email on users (cost=0.43..8.45 rows=1 width=256)
    //   Index Cond: (email = 'user@example.com')
    //   Execution Time: 0.123 ms (0.123ミリ秒！)
    
    var user User
    err := db.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.Username, &user.ProfileData)
    
    // 【結果】
    // - 従来: 67秒のフルテーブルスキャン
    // - 最適化後: 0.123ミリ秒のインデックス検索
    // - 改善率: 544,715倍の高速化
    
    return &user, err
}
```

インデックスは、データベースのテーブルから素早くデータを検索するための**データ構造**です。辞書の見出しのように、データの位置を効率的に特定できます。

#### インデックスなしでのデータ検索の問題

```go
// 1億件のユーザーテーブルからemailで検索する場合
// CREATE TABLE users (id SERIAL PRIMARY KEY, email VARCHAR(255), name VARCHAR(255), created_at TIMESTAMP);

func FindUserByEmailWithoutIndex(db *sql.DB, email string) (*User, error) {
    query := `
        SELECT id, email, name, created_at 
        FROM users 
        WHERE email = $1
    `
    // インデックスがない場合：
    // - テーブル全体をスキャン（Sequential Scan）
    // - 1億件全てをチェック = 数十秒かかる
    // - CPUとI/Oリソースを大量消費
    
    var user User
    err := db.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
    return &user, err
}
```

**問題点の詳細分析：**
- **時間計算量**: O(n) - データ量に比例して検索時間が増加
- **I/O負荷**: 全データブロックの読み込みが必要
- **リソース競合**: 他のクエリも同時に遅延
- **スケーラビリティ**: データ増加で指数的に性能劣化

#### インデックスによる劇的な改善

```sql
-- emailカラムにB-treeインデックスを作成
CREATE INDEX idx_users_email ON users(email);
```

```go
func FindUserByEmailWithIndex(db *sql.DB, email string) (*User, error) {
    query := `
        SELECT id, email, name, created_at 
        FROM users 
        WHERE email = $1
    `
    // インデックスがある場合：
    // - Index Scan使用
    // - O(log n)の時間計算量 = 数ミリ秒で完了
    // - 必要最小限のデータブロックのみアクセス
    
    var user User
    err := db.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
    return &user, err
}
```

**改善効果：**
- **検索時間**: 数十秒 → 数ミリ秒（10,000倍高速化）
- **I/O負荷**: 99.9%削減
- **同時実行性**: 大幅向上
- **リソース効率**: CPU使用率激減

### インデックスの種類

#### 1. B-tree インデックス（最も一般的）
```sql
-- 単一カラムインデックス
CREATE INDEX idx_users_email ON users(email);

-- 複合インデックス
CREATE INDEX idx_orders_user_date ON orders(user_id, created_at);

-- 部分インデックス
CREATE INDEX idx_active_users ON users(email) WHERE active = true;
```

#### 2. ハッシュインデックス
```sql
-- 等価検索に最適
CREATE INDEX idx_users_id_hash ON users USING HASH(id);
```

#### 3. GINインデックス（配列・JSON用）
```sql
-- 配列検索用
CREATE INDEX idx_post_tags ON posts USING GIN(tags);

-- JSONB検索用
CREATE INDEX idx_user_metadata ON users USING GIN(metadata);
```

### 高度なEXPLAIN分析システム

EXPLAINを使った包括的なクエリ分析システムを構築します：

```go
package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "strings"
    "time"
    "math"
)

// QueryAnalyzer analyzes SQL queries using EXPLAIN
type QueryAnalyzer struct {
    db *sql.DB
    cache map[string]*CachedExplainResult
    mu    sync.RWMutex
}

func NewQueryAnalyzer(db *sql.DB) *QueryAnalyzer {
    return &QueryAnalyzer{
        db:    db,
        cache: make(map[string]*CachedExplainResult),
    }
}

// CachedExplainResult holds cached analysis results
type CachedExplainResult struct {
    Result    *DetailedExplainResult
    Timestamp time.Time
    TTL       time.Duration
}

// DetailedExplainResult holds comprehensive analysis
type DetailedExplainResult struct {
    Query                string                    `json:"query"`
    ExecutionPlan        *ExecutionPlan           `json:"execution_plan"`
    PerformanceMetrics   *PerformanceMetrics      `json:"performance_metrics"`
    IndexUsage           []IndexUsageInfo         `json:"index_usage"`
    Recommendations      []OptimizationSuggestion `json:"recommendations"`
    BottleneckAnalysis   *BottleneckAnalysis      `json:"bottleneck_analysis"`
    CostBreakdown        *CostBreakdown           `json:"cost_breakdown"`
}

type ExecutionPlan struct {
    NodeType           string             `json:"Node Type"`
    Relation           string             `json:"Relation Name,omitempty"`
    Alias              string             `json:"Alias,omitempty"`
    StartupCost        float64            `json:"Startup Cost"`
    TotalCost          float64            `json:"Total Cost"`
    PlanRows           int                `json:"Plan Rows"`
    PlanWidth          int                `json:"Plan Width"`
    ActualStartupTime  float64            `json:"Actual Startup Time,omitempty"`
    ActualTotalTime    float64            `json:"Actual Total Time,omitempty"`
    ActualRows         int                `json:"Actual Rows,omitempty"`
    IndexName          string             `json:"Index Name,omitempty"`
    IndexCondition     string             `json:"Index Cond,omitempty"`
    Filter             string             `json:"Filter,omitempty"`
    BuffersHit         int                `json:"Buffers Hit,omitempty"`
    BuffersRead        int                `json:"Buffers Read,omitempty"`
    ChildPlans         []*ExecutionPlan   `json:"Plans,omitempty"`
    JoinType           string             `json:"Join Type,omitempty"`
    HashCondition      string             `json:"Hash Cond,omitempty"`
    SortKey            []string           `json:"Sort Key,omitempty"`
    SortMethod         string             `json:"Sort Method,omitempty"`
    WorkMemUsed        int                `json:"Sort Space Used,omitempty"`
}

type PerformanceMetrics struct {
    ExecutionTime      time.Duration  `json:"execution_time"`
    PlanningTime       time.Duration  `json:"planning_time"`
    TotalCost          float64        `json:"total_cost"`
    RowsReturned       int            `json:"rows_returned"`
    RowsExamined       int            `json:"rows_examined"`
    SelectivityRatio   float64        `json:"selectivity_ratio"`
    BufferHitRatio     float64        `json:"buffer_hit_ratio"`
    IOTime             time.Duration  `json:"io_time"`
    CPUTime            time.Duration  `json:"cpu_time"`
}

type IndexUsageInfo struct {
    IndexName        string  `json:"index_name"`
    TableName        string  `json:"table_name"`
    Columns          []string `json:"columns"`
    UsageType        string   `json:"usage_type"` // "scan", "seek", "lookup"
    SelectivityGain  float64  `json:"selectivity_gain"`
    CostReduction    float64  `json:"cost_reduction"`
}

type OptimizationSuggestion struct {
    Type           string  `json:"type"` // "create_index", "drop_index", "modify_query"
    Priority       string  `json:"priority"` // "high", "medium", "low"
    Description    string  `json:"description"`
    SQLCommand     string  `json:"sql_command,omitempty"`
    ExpectedGain   float64 `json:"expected_gain"` // パフォーマンス改善率（%）
    Reason         string  `json:"reason"`
    Impact         string  `json:"impact"`
}

type BottleneckAnalysis struct {
    PrimaryBottleneck   string             `json:"primary_bottleneck"`
    BottleneckDetails   map[string]float64 `json:"bottleneck_details"`
    TimeBreakdown       map[string]float64 `json:"time_breakdown"`
    ResourceUsage       map[string]float64 `json:"resource_usage"`
}

type CostBreakdown struct {
    SeqScanCost      float64 `json:"seq_scan_cost"`
    IndexScanCost    float64 `json:"index_scan_cost"`
    JoinCost         float64 `json:"join_cost"`
    SortCost         float64 `json:"sort_cost"`
    HashCost         float64 `json:"hash_cost"`
    FilterCost       float64 `json:"filter_cost"`
}

// ComprehensiveAnalyzeQuery performs detailed query analysis
func (qa *QueryAnalyzer) ComprehensiveAnalyzeQuery(query string, args ...interface{}) (*DetailedExplainResult, error) {
    // キャッシュチェック
    cacheKey := qa.generateCacheKey(query, args...)
    if cached := qa.getCachedResult(cacheKey); cached != nil {
        return cached, nil
    }
    
    // EXPLAIN ANALYZE実行
    explainQuery := "EXPLAIN (ANALYZE true, BUFFERS true, FORMAT JSON, TIMING true, VERBOSE true) " + query
    
    var jsonResult string
    start := time.Now()
    err := qa.db.QueryRow(explainQuery, args...).Scan(&jsonResult)
    if err != nil {
        return nil, fmt.Errorf("failed to execute EXPLAIN: %w", err)
    }
    executionTime := time.Since(start)
    
    // JSON結果をパース
    var rawResult []map[string]interface{}
    if err := json.Unmarshal([]byte(jsonResult), &rawResult); err != nil {
        return nil, fmt.Errorf("failed to parse EXPLAIN result: %w", err)
    }
    
    if len(rawResult) == 0 {
        return nil, fmt.Errorf("empty EXPLAIN result")
    }
    
    planData := rawResult[0]["Plan"].(map[string]interface{})
    
    // 詳細分析を実行
    result := &DetailedExplainResult{
        Query: query,
    }
    
    result.ExecutionPlan = qa.parseExecutionPlan(planData)
    result.PerformanceMetrics = qa.calculatePerformanceMetrics(planData, executionTime)
    result.IndexUsage = qa.analyzeIndexUsage(result.ExecutionPlan)
    result.Recommendations = qa.generateRecommendations(result)
    result.BottleneckAnalysis = qa.analyzeBottlenecks(result)
    result.CostBreakdown = qa.calculateCostBreakdown(result.ExecutionPlan)
    
    // 結果をキャッシュ
    qa.cacheResult(cacheKey, result, 5*time.Minute)
    
    return result, nil
}

func (qa *QueryAnalyzer) parseExecutionPlan(planData map[string]interface{}) *ExecutionPlan {
    plan := &ExecutionPlan{}
    
    // 基本情報の抽出
    if nodeType, ok := planData["Node Type"].(string); ok {
        plan.NodeType = nodeType
    }
    if relation, ok := planData["Relation Name"].(string); ok {
        plan.Relation = relation
    }
    if alias, ok := planData["Alias"].(string); ok {
        plan.Alias = alias
    }
    
    // コスト情報
    if startupCost, ok := planData["Startup Cost"].(float64); ok {
        plan.StartupCost = startupCost
    }
    if totalCost, ok := planData["Total Cost"].(float64); ok {
        plan.TotalCost = totalCost
    }
    if planRows, ok := planData["Plan Rows"].(float64); ok {
        plan.PlanRows = int(planRows)
    }
    if planWidth, ok := planData["Plan Width"].(float64); ok {
        plan.PlanWidth = int(planWidth)
    }
    
    // 実行時統計
    if actualStartupTime, ok := planData["Actual Startup Time"].(float64); ok {
        plan.ActualStartupTime = actualStartupTime
    }
    if actualTotalTime, ok := planData["Actual Total Time"].(float64); ok {
        plan.ActualTotalTime = actualTotalTime
    }
    if actualRows, ok := planData["Actual Rows"].(float64); ok {
        plan.ActualRows = int(actualRows)
    }
    
    // インデックス情報
    if indexName, ok := planData["Index Name"].(string); ok {
        plan.IndexName = indexName
    }
    if indexCond, ok := planData["Index Cond"].(string); ok {
        plan.IndexCondition = indexCond
    }
    if filter, ok := planData["Filter"].(string); ok {
        plan.Filter = filter
    }
    
    // バッファ情報
    if buffersHit, ok := planData["Buffers Hit"].(float64); ok {
        plan.BuffersHit = int(buffersHit)
    }
    if buffersRead, ok := planData["Buffers Read"].(float64); ok {
        plan.BuffersRead = int(buffersRead)
    }
    
    // JOIN情報
    if joinType, ok := planData["Join Type"].(string); ok {
        plan.JoinType = joinType
    }
    if hashCond, ok := planData["Hash Cond"].(string); ok {
        plan.HashCondition = hashCond
    }
    
    // ソート情報
    if sortKey, ok := planData["Sort Key"].([]interface{}); ok {
        plan.SortKey = make([]string, len(sortKey))
        for i, key := range sortKey {
            plan.SortKey[i] = key.(string)
        }
    }
    if sortMethod, ok := planData["Sort Method"].(string); ok {
        plan.SortMethod = sortMethod
    }
    if workMemUsed, ok := planData["Sort Space Used"].(float64); ok {
        plan.WorkMemUsed = int(workMemUsed)
    }
    
    // 子プランの再帰的パース
    if plans, ok := planData["Plans"].([]interface{}); ok {
        plan.ChildPlans = make([]*ExecutionPlan, len(plans))
        for i, childPlan := range plans {
            plan.ChildPlans[i] = qa.parseExecutionPlan(childPlan.(map[string]interface{}))
        }
    }
    
    return plan
}

func (qa *QueryAnalyzer) calculatePerformanceMetrics(planData map[string]interface{}, executionTime time.Duration) *PerformanceMetrics {
    metrics := &PerformanceMetrics{
        ExecutionTime: executionTime,
    }
    
    // 基本メトリクス
    if totalCost, ok := planData["Total Cost"].(float64); ok {
        metrics.TotalCost = totalCost
    }
    if actualRows, ok := planData["Actual Rows"].(float64); ok {
        metrics.RowsReturned = int(actualRows)
    }
    
    // バッファヒット率計算
    totalBuffers := 0
    hitBuffers := 0
    qa.calculateBufferStats(planData, &totalBuffers, &hitBuffers)
    
    if totalBuffers > 0 {
        metrics.BufferHitRatio = float64(hitBuffers) / float64(totalBuffers)
    }
    
    // 選択性計算（概算）
    if planRows, ok := planData["Plan Rows"].(float64); ok {
        if actualRows, ok := planData["Actual Rows"].(float64); ok {
            if planRows > 0 {
                metrics.SelectivityRatio = actualRows / planRows
            }
        }
    }
    
    return metrics
}

func (qa *QueryAnalyzer) calculateBufferStats(planData map[string]interface{}, totalBuffers, hitBuffers *int) {
    if hit, ok := planData["Buffers Hit"].(float64); ok {
        *hitBuffers += int(hit)
        *totalBuffers += int(hit)
    }
    if read, ok := planData["Buffers Read"].(float64); ok {
        *totalBuffers += int(read)
    }
    
    // 子プランの統計も計算
    if plans, ok := planData["Plans"].([]interface{}); ok {
        for _, childPlan := range plans {
            qa.calculateBufferStats(childPlan.(map[string]interface{}), totalBuffers, hitBuffers)
        }
    }
}
```

### インデックス効果の測定

```go
package main

import (
    "context"
    "database/sql"
    "time"
)

// IndexPerformanceTest tests index performance
type IndexPerformanceTest struct {
    db    *sql.DB
    table string
}

// BenchmarkQuery measures query performance
func (ipt *IndexPerformanceTest) BenchmarkQuery(ctx context.Context, query string, iterations int, args ...interface{}) (QueryBenchmark, error) {
    var totalDuration time.Duration
    var successCount int
    
    for i := 0; i < iterations; i++ {
        start := time.Now()
        
        rows, err := ipt.db.QueryContext(ctx, query, args...)
        if err != nil {
            continue
        }
        
        // Consume all rows to ensure full execution
        for rows.Next() {
            // Do nothing, just consume
        }
        rows.Close()
        
        totalDuration += time.Since(start)
        successCount++
    }
    
    return QueryBenchmark{
        Query:           query,
        Iterations:      iterations,
        SuccessCount:    successCount,
        TotalDuration:   totalDuration,
        AverageDuration: totalDuration / time.Duration(successCount),
    }, nil
}

type QueryBenchmark struct {
    Query           string
    Iterations      int
    SuccessCount    int
    TotalDuration   time.Duration
    AverageDuration time.Duration
}
```

### インデックス推奨システム

```go
// IndexRecommendation suggests indexes based on query patterns
type IndexRecommendation struct {
    TableName   string
    Columns     []string
    IndexType   string
    Reason      string
    ExpectedGain float64
}

// IndexAdvisor analyzes queries and suggests indexes
type IndexAdvisor struct {
    db           *sql.DB
    analyzer     *QueryAnalyzer
    queries      []string
    recommendations []IndexRecommendation
}

func NewIndexAdvisor(db *sql.DB) *IndexAdvisor {
    return &IndexAdvisor{
        db:       db,
        analyzer: NewQueryAnalyzer(db),
        queries:  make([]string, 0),
        recommendations: make([]IndexRecommendation, 0),
    }
}

// AnalyzeQuery analyzes a query and suggests indexes
func (ia *IndexAdvisor) AnalyzeQuery(query string, args ...interface{}) error {
    results, err := ia.analyzer.ExplainQuery(query, args...)
    if err != nil {
        return err
    }
    
    // Analyze for sequential scans
    for _, result := range results {
        if result.NodeType == "Seq Scan" && result.ActualTotalTime > 10.0 {
            recommendation := IndexRecommendation{
                TableName:    result.Relation,
                Columns:      extractColumnsFromFilter(result.Filter),
                IndexType:    "btree",
                Reason:       "Sequential scan detected on large table",
                ExpectedGain: result.ActualTotalTime * 0.8, // Estimate 80% improvement
            }
            ia.recommendations = append(ia.recommendations, recommendation)
        }
    }
    
    return nil
}
```

### 実践的なインデックス戦略

#### WHERE句のインデックス化
```go
// Bad: No index on email
// SELECT * FROM users WHERE email = 'user@example.com'

// Good: Index on email
// CREATE INDEX idx_users_email ON users(email);

func FindUserByEmail(db *sql.DB, email string) (*User, error) {
    query := `
        SELECT id, name, email, created_at 
        FROM users 
        WHERE email = $1
    `
    
    var user User
    err := db.QueryRow(query, email).Scan(
        &user.ID, &user.Name, &user.Email, &user.CreatedAt,
    )
    if err != nil {
        return nil, err
    }
    
    return &user, nil
}
```

#### 複合インデックスの活用
```go
// 複合インデックス: (user_id, created_at)
// CREATE INDEX idx_orders_user_date ON orders(user_id, created_at);

func GetUserOrdersInDateRange(db *sql.DB, userID int, start, end time.Time) ([]Order, error) {
    query := `
        SELECT id, user_id, amount, created_at
        FROM orders 
        WHERE user_id = $1 
          AND created_at BETWEEN $2 AND $3
        ORDER BY created_at DESC
    `
    
    rows, err := db.Query(query, userID, start, end)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var orders []Order
    for rows.Next() {
        var order Order
        err := rows.Scan(&order.ID, &order.UserID, &order.Amount, &order.CreatedAt)
        if err != nil {
            return nil, err
        }
        orders = append(orders, order)
    }
    
    return orders, nil
}
```

### インデックスメンテナンス

```go
// IndexMaintenance handles index maintenance tasks
type IndexMaintenance struct {
    db *sql.DB
}

// GetIndexUsageStats returns index usage statistics
func (im *IndexMaintenance) GetIndexUsageStats() ([]IndexUsage, error) {
    query := `
        SELECT 
            schemaname,
            tablename,
            indexname,
            idx_tup_read,
            idx_tup_fetch,
            idx_scan
        FROM pg_stat_user_indexes
        ORDER BY idx_scan ASC
    `
    
    rows, err := im.db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var usage []IndexUsage
    for rows.Next() {
        var idx IndexUsage
        err := rows.Scan(
            &idx.SchemaName,
            &idx.TableName, 
            &idx.IndexName,
            &idx.TupRead,
            &idx.TupFetch,
            &idx.Scans,
        )
        if err != nil {
            return nil, err
        }
        usage = append(usage, idx)
    }
    
    return usage, nil
}

type IndexUsage struct {
    SchemaName string
    TableName  string
    IndexName  string
    TupRead    int64
    TupFetch   int64
    Scans      int64
}

// FindUnusedIndexes identifies potentially unused indexes
func (im *IndexMaintenance) FindUnusedIndexes() ([]string, error) {
    query := `
        SELECT indexname 
        FROM pg_stat_user_indexes 
        WHERE idx_scan = 0
          AND indexname != tablename || '_pkey'
    `
    
    rows, err := im.db.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var unused []string
    for rows.Next() {
        var indexName string
        if err := rows.Scan(&indexName); err != nil {
            return nil, err
        }
        unused = append(unused, indexName)
    }
    
    return unused, nil
}
```

📝 **課題**

以下の機能を持つデータベースインデックス分析システムを実装してください：

1. **`QueryAnalyzer`**: EXPLAIN結果の分析
2. **`IndexAdvisor`**: インデックス推奨システム
3. **`PerformanceTester`**: インデックス効果の測定
4. **`IndexMaintenance`**: インデックス保守管理
5. **`QueryOptimizer`**: クエリ最適化支援
6. **統計レポート**: パフォーマンス改善レポート

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestQueryAnalyzer_ExplainQuery
--- PASS: TestQueryAnalyzer_ExplainQuery (0.02s)
=== RUN   TestIndexAdvisor_Recommendations
--- PASS: TestIndexAdvisor_Recommendations (0.05s)
=== RUN   TestPerformanceTester_IndexComparison
--- PASS: TestPerformanceTester_IndexComparison (0.10s)
=== RUN   TestIndexMaintenance_UsageStats
--- PASS: TestIndexMaintenance_UsageStats (0.03s)
=== RUN   TestQueryOptimizer_Integration
--- PASS: TestQueryOptimizer_Integration (0.15s)
PASS
ok      day38-db-index    0.350s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **EXPLAIN ANALYZE**: 実際の実行統計を取得
2. **pg_stat_user_indexes**: インデックス使用状況の監視
3. **JSONB処理**: PostgreSQLのEXPLAIN結果パース
4. **ベンチマーク**: クエリ性能の定量的測定
5. **インデックス戦略**: 適切なインデックス設計

インデックス設計のポイント：
- **選択性の高いカラム**: ユニークな値が多いカラムを優先
- **WHERE句の頻度**: よく使われる検索条件をインデックス化
- **複合インデックスの順序**: 選択性の高い順に配置
- **メンテナンスコスト**: 更新頻度とのバランスを考慮

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```