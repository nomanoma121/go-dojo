# Day 37: DBコネクションプールの設定

🎯 **本日の目標**

`sql.DB`のコネクションプール設定を調整し、パフォーマンスを最適化できるようになる。

📖 **解説**

## コネクションプールとは

コネクションプールは、データベースへの接続を事前に作成して管理する仕組みです。新しい接続を都度作成するオーバーヘッドを削減し、データベースの性能を最大化します。

### Goのsql.DBコネクションプール

Goの`database/sql`パッケージは、内部で自動的にコネクションプールを管理します。以下の設定項目があります：

#### 主要な設定項目

1. **MaxOpenConns**: 最大オープン接続数
2. **MaxIdleConns**: 最大アイドル接続数  
3. **ConnMaxLifetime**: 接続の最大生存時間
4. **ConnMaxIdleTime**: アイドル接続の最大生存時間

### 基本的なコネクションプール設定

```go
package main

import (
    "database/sql"
    "time"
    _ "github.com/lib/pq"
)

func setupConnectionPool(dsn string) (*sql.DB, error) {
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }

    // 最大オープン接続数を設定
    db.SetMaxOpenConns(25)
    
    // 最大アイドル接続数を設定
    db.SetMaxIdleConns(5)
    
    // 接続の最大生存時間を設定
    db.SetConnMaxLifetime(5 * time.Minute)
    
    // アイドル接続の最大生存時間を設定
    db.SetConnMaxIdleTime(30 * time.Second)

    return db, nil
}
```

### 設定項目の詳細解説

#### MaxOpenConns（最大オープン接続数）

同時に開ける最大接続数を制限します。

```go
// デフォルトは無制限（0）
db.SetMaxOpenConns(25)

// 無制限に設定
db.SetMaxOpenConns(0)
```

**設定のポイント:**
- 高すぎる値: データベースの接続制限を超える可能性
- 低すぎる値: 接続待機によるパフォーマンス低下
- 推奨値: CPU数 × 2〜4 または データベースの接続制限の70-80%

#### MaxIdleConns（最大アイドル接続数）

プールに保持するアイドル接続の最大数です。

```go
// デフォルトは2
db.SetMaxIdleConns(5)

// アイドル接続を無効化
db.SetMaxIdleConns(0)
```

**設定のポイント:**
- 高すぎる値: 不要な接続によるリソース消費
- 低すぎる値: 接続作成のオーバーヘッド増加
- 推奨値: MaxOpenConnsの20-50%

#### ConnMaxLifetime（接続の最大生存時間）

接続が作成されてから自動的に閉じられるまでの時間です。

```go
// デフォルトは無制限
db.SetConnMaxLifetime(5 * time.Minute)

// 無制限に設定
db.SetConnMaxLifetime(0)
```

**設定のポイント:**
- データベース側のタイムアウトより短く設定
- ロードバランサーのタイムアウトより短く設定
- 推奨値: 1-30分

#### ConnMaxIdleTime（アイドル接続の最大生存時間）

アイドル状態の接続が閉じられるまでの時間です。

```go
// デフォルトは無制限
db.SetConnMaxIdleTime(30 * time.Second)

// 無制限に設定
db.SetConnMaxIdleTime(0)
```

### コネクションプール監視

```go
package main

import (
    "database/sql"
    "fmt"
    "time"
)

type PoolMonitor struct {
    db     *sql.DB
    name   string
    ticker *time.Ticker
    done   chan bool
}

func NewPoolMonitor(db *sql.DB, name string, interval time.Duration) *PoolMonitor {
    return &PoolMonitor{
        db:     db,
        name:   name,
        ticker: time.NewTicker(interval),
        done:   make(chan bool),
    }
}

func (pm *PoolMonitor) Start() {
    go func() {
        for {
            select {
            case <-pm.ticker.C:
                pm.logStats()
            case <-pm.done:
                return
            }
        }
    }()
}

func (pm *PoolMonitor) Stop() {
    pm.ticker.Stop()
    pm.done <- true
}

func (pm *PoolMonitor) logStats() {
    stats := pm.db.Stats()
    fmt.Printf("[%s] Pool Stats: Open=%d, InUse=%d, Idle=%d, WaitCount=%d, WaitDuration=%v\n",
        pm.name,
        stats.OpenConnections,
        stats.InUse,
        stats.Idle,
        stats.WaitCount,
        stats.WaitDuration,
    )
}
```

### 環境別の最適化設定

#### 開発環境

```go
func setupDevelopmentPool(db *sql.DB) {
    db.SetMaxOpenConns(5)
    db.SetMaxIdleConns(2)
    db.SetConnMaxLifetime(1 * time.Minute)
    db.SetConnMaxIdleTime(30 * time.Second)
}
```

#### ステージング環境

```go
func setupStagingPool(db *sql.DB) {
    db.SetMaxOpenConns(15)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(3 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)
}
```

#### 本番環境

```go
func setupProductionPool(db *sql.DB) {
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(10)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(2 * time.Minute)
}
```

### パフォーマンステスト

```go
func BenchmarkConnectionPool(b *testing.B) {
    db := setupTestDB()
    defer db.Close()

    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            var count int
            err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
            if err != nil {
                b.Error(err)
            }
        }
    })
}
```

📝 **課題**

以下の機能を持つコネクションプール管理システムを実装してください：

1. **`PoolConfig`**: コネクションプール設定構造体
2. **`ConnectionManager`**: 接続管理システム
3. **`HealthChecker`**: データベース接続ヘルスチェック
4. **`PoolMonitor`**: リアルタイム統計監視
5. **`LoadTester`**: コネクションプール負荷テスト
6. **動的設定変更**: 実行時の設定調整機能

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestPoolConfig_Apply
--- PASS: TestPoolConfig_Apply (0.01s)
=== RUN   TestConnectionManager_BasicOperations
--- PASS: TestConnectionManager_BasicOperations (0.02s)
=== RUN   TestHealthChecker_Integration
--- PASS: TestHealthChecker_Integration (0.05s)
=== RUN   TestPoolMonitor_Statistics
--- PASS: TestPoolMonitor_Statistics (0.10s)
=== RUN   TestLoadTester_ConcurrentAccess
--- PASS: TestLoadTester_ConcurrentAccess (0.15s)
PASS
ok      day37-connection-pool    0.330s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **database/sql**パッケージ: コネクションプール設定
2. **time**パッケージ: タイムアウト設定とタイマー
3. **context**パッケージ: タイムアウト付きクエリ実行
4. **sync**パッケージ: 統計データの並行安全性
5. **testing**パッケージ: ベンチマークテストの実装

設定のポイント：
- **環境に応じた最適化**: 開発/ステージング/本番で異なる設定
- **監視とアラート**: プール使用率の監視
- **グレースフル設定変更**: 既存接続に影響を与えない更新
- **負荷テスト**: 設定の妥当性検証

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```