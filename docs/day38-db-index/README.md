# Day 38: DBインデックスの活用

🎯 **本日の目標**

クエリを高速化するためのインデックスの効果を`EXPLAIN`で確認し、効率的なデータベース設計ができるようになる。

📖 **解説**

## データベースインデックスとは

インデックスは、データベースのテーブルから素早くデータを検索するための仕組みです。本のように、目次があることで目的のページを素早く見つけられるのと同じ原理です。

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

### Go でのEXPLAIN分析

```go
package main

import (
    "database/sql"
    "fmt"
    "strings"
)

// QueryAnalyzer analyzes SQL queries using EXPLAIN
type QueryAnalyzer struct {
    db *sql.DB
}

func NewQueryAnalyzer(db *sql.DB) *QueryAnalyzer {
    return &QueryAnalyzer{db: db}
}

// ExplainQuery executes EXPLAIN on a query
func (qa *QueryAnalyzer) ExplainQuery(query string, args ...interface{}) ([]ExplainResult, error) {
    explainQuery := "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + query
    
    var jsonResult string
    err := qa.db.QueryRow(explainQuery, args...).Scan(&jsonResult)
    if err != nil {
        return nil, fmt.Errorf("failed to execute EXPLAIN: %w", err)
    }
    
    return parseExplainResult(jsonResult)
}

// ExplainResult holds the result of EXPLAIN analysis
type ExplainResult struct {
    NodeType           string  `json:"Node Type"`
    Relation           string  `json:"Relation Name,omitempty"`
    Alias              string  `json:"Alias,omitempty"`
    StartupCost        float64 `json:"Startup Cost"`
    TotalCost          float64 `json:"Total Cost"`
    PlanRows           int     `json:"Plan Rows"`
    PlanWidth          int     `json:"Plan Width"`
    ActualStartupTime  float64 `json:"Actual Startup Time,omitempty"`
    ActualTotalTime    float64 `json:"Actual Total Time,omitempty"`
    ActualRows         int     `json:"Actual Rows,omitempty"`
    IndexName          string  `json:"Index Name,omitempty"`
    IndexCondition     string  `json:"Index Cond,omitempty"`
    Filter             string  `json:"Filter,omitempty"`
    BuffersHit         int     `json:"Buffers Hit,omitempty"`
    BuffersRead        int     `json:"Buffers Read,omitempty"`
    Plans              []ExplainResult `json:"Plans,omitempty"`
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