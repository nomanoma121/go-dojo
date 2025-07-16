# Day 04: sync.Onceによる安全な初期化

## 🎯 本日の目標 (Today's Goal)

このチャレンジを通して、以下のスキルを身につけることができます：

- **sync.Onceを使って一度だけ実行される初期化処理を実装できるようになる**
- **複数のGoroutineから安全にシングルトンパターンを実装できるようになる**
- **初期化エラーの適切な処理方法を理解できるようになる**
- **遅延初期化（Lazy Initialization）のパフォーマンス利点を活用できるようになる**

## 📖 解説 (Explanation)

### なぜsync.Onceが必要なのか？

アプリケーション開発では、以下のような「一度だけ実行したい処理」が頻繁に発生します：

- 設定ファイルの読み込み
- データベース接続プールの初期化
- 外部APIクライアントの設定
- ログシステムの初期化
- キャッシュシステムの準備

これらの処理を複数回実行してしまうと、以下の問題が発生する可能性があります：

```go
// ❌ 【問題のある例】：複数回初期化される可能性 - 本番環境で避けるべき
var config *Config  // 【問題】グローバル変数への非同期アクセス

func GetConfig() *Config {
    // 【致命的問題】レースコンディションが発生
    if config == nil {
        // 【危険地帯】複数のGoroutineが同時にここに到達する可能性
        // シナリオ：
        // - Goroutine A: config == nil を確認 → loadConfigFromFile()開始
        // - Goroutine B: config == nil を確認 → loadConfigFromFile()開始
        // - 結果: 同じ設定ファイルを2回読み込み
        
        config = loadConfigFromFile() // 【問題】重い処理が複数回実行される
        
        // 【追加問題】：
        // 1. ファイルI/Oが重複実行される（パフォーマンス劣化）
        // 2. 異なるタイミングで読み込まれた設定が混在する可能性
        // 3. メモリリークの危険性（古いconfigオブジェクトが残存）
        // 4. 設定の一貫性が保証されない
    }
    return config
}

// 【レースコンディションの具体例】：
// 時刻 | Goroutine A              | Goroutine B              | config状態
// -----|-------------------------|-------------------------|----------
// t1   | if config == nil (true) |                        | nil
// t2   |                        | if config == nil (true) | nil
// t3   | loadConfig開始          |                        | nil  
// t4   |                        | loadConfig開始          | nil
// t5   | config = configA       |                        | configA
// t6   |                        | config = configB       | configB (上書き!)
```

上記のコードは**レースコンディション**を引き起こし、以下の問題が発生します：

1. 設定の読み込みが複数回実行される（パフォーマンス問題）
2. 異なるGoroutineが異なる設定を取得する可能性
3. リソースリークの危険性

### sync.Onceの基本的な使い方

`sync.Once`は、指定された関数を**プログラムの実行中に一度だけ**実行することを保証します：

```go
import "sync"

// 【正しい実装】sync.Onceによる安全な一度限りの初期化
var (
    config *Config      // 【保護対象】設定データ
    once   sync.Once    // 【制御機構】一度限りの実行を保証
)

func GetConfig() *Config {
    // 【核心機能】once.Do()が一度限りの実行を保証
    once.Do(func() {
        // 【重要】この関数は以下を保証する：
        // 1. プログラム実行中に一度だけ呼ばれる
        // 2. 複数のGoroutineが同時に呼び出しても安全
        // 3. 最初のGoroutineが実行中、他は完了まで待機
        // 4. 実行完了後、他のGoroutineは即座にreturn
        
        config = loadConfigFromFile()
        fmt.Println("Configuration loaded!")
        
        // 【内部動作の詳細】：
        // sync.Onceは内部で以下の仕組みを使用：
        // - atomic操作による高速な「実行済みチェック」
        // - Mutexによる排他制御（初回実行時のみ）
        // - Memory Barrierによる可視性保証
    })
    
    // 【パフォーマンス特性】：
    // - 初回呼び出し: Mutex + 関数実行 + 状態更新
    // - 2回目以降: atomic load のみ（非常に高速）
    return config
}

// 【使用例】複数Goroutineからの安全な呼び出し
func demonstrateSafeUsage() {
    var wg sync.WaitGroup
    
    // 100個のGoroutineが同時にGetConfig()を呼び出し
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            cfg := GetConfig()  // 【安全】どのGoroutineも同じ設定を取得
            fmt.Printf("Goroutine %d got config: %v\n", id, cfg != nil)
        }(i)
    }
    
    wg.Wait()
    // 結果: "Configuration loaded!" は一度だけ出力される
}
```

**sync.Onceの特徴：**
- 最初に`Do()`が呼ばれたときのみ関数を実行
- 他のGoroutineは初期化の完了を待つ
- 初期化完了後は追加のロックオーバーヘッドなし
- パニックが発生しても「実行済み」と記録される

### 遅延初期化（Lazy Initialization）パターン

sync.Onceは遅延初期化パターンの実装に最適です：

```go
type ConfigManager struct {
    config *Config
    once   sync.Once
    err    error
}

func (cm *ConfigManager) GetConfig() (*Config, error) {
    cm.once.Do(func() {
        // 実際に必要になった時点で初期化
        cm.config, cm.err = loadConfigFromFile()
    })
    return cm.config, cm.err
}
```

**遅延初期化の利点：**
- アプリケーション起動時間の短縮
- 使用されないリソースの初期化を回避
- メモリ使用量の最適化

### シングルトンパターンの安全な実装

データベース接続プールなどのシングルトンも安全に実装できます：

```go
type DatabasePool struct {
    connections []*sql.DB
}

var (
    dbPool *DatabasePool
    dbOnce sync.Once
)

func GetDatabasePool() *DatabasePool {
    dbOnce.Do(func() {
        dbPool = &DatabasePool{
            connections: make([]*sql.DB, 0, 10),
        }
        // 接続プールの初期化
        for i := 0; i < 10; i++ {
            conn, err := sql.Open("postgres", "connection_string")
            if err != nil {
                panic("Failed to create database connection")
            }
            dbPool.connections = append(dbPool.connections, conn)
        }
        fmt.Println("Database pool initialized with 10 connections")
    })
    return dbPool
}
```

### エラーハンドリングの注意点

sync.Onceは一度実行されると、たとえ関数内でエラーが発生しても再実行されません：

```go
// 問題のある例：エラー時の再試行ができない
type BadConfigManager struct {
    config *Config
    once   sync.Once
}

func (cm *BadConfigManager) GetConfig() (*Config, error) {
    var err error
    cm.once.Do(func() {
        cm.config, err = loadConfigFromFile()
        // errが設定されてもDo()は再実行されない
    })
    return cm.config, err // errは常にnilになってしまう
}
```

正しいエラーハンドリング：

```go
// 正しい例：エラーを構造体に保存
type ConfigManager struct {
    config *Config
    once   sync.Once
    err    error
}

func (cm *ConfigManager) GetConfig() (*Config, error) {
    cm.once.Do(func() {
        cm.config, cm.err = loadConfigFromFile()
    })
    return cm.config, cm.err
}
```

### 高度な使用例：再試行可能な初期化

エラー時に再試行を可能にしたい場合は、Onceを動的に作り直します：

```go
type RetryableConfigManager struct {
    config *Config
    once   *sync.Once
    mu     sync.Mutex
}

func (cm *RetryableConfigManager) GetConfig() (*Config, error) {
    cm.mu.Lock()
    if cm.once == nil {
        cm.once = &sync.Once{}
    }
    once := cm.once
    cm.mu.Unlock()
    
    var err error
    once.Do(func() {
        cm.config, err = loadConfigFromFile()
        if err != nil {
            cm.mu.Lock()
            cm.once = nil // エラー時はOnceをリセット
            cm.mu.Unlock()
        }
    })
    return cm.config, err
}
```

### パフォーマンス特性

sync.Onceは初期化後の呼び出しで非常に高速です：

```go
// ベンチマーク例
func BenchmarkOnceAfterInit(b *testing.B) {
    var once sync.Once
    var value int
    
    // 初期化を完了
    once.Do(func() {
        value = 42
    })
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            once.Do(func() {
                value = 999 // この関数は実行されない
            })
            _ = value
        }
    })
}
```

初期化完了後は、`Do()`の呼び出しはほぼロックフリーで実行されます。

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **`NewConfigManager()`**: 設定マネージャーを初期化する
2. **`(cm *ConfigManager) GetConfig() (*Config, error)`**: 設定を遅延読み込みする
3. **`GetDatabasePool() *DatabasePool`**: シングルトンのDBプールを取得する
4. **`(dp *DatabasePool) GetConnection() string`**: 接続文字列を取得する
5. **`NewLazyResource(initFunc func() (interface{}, error))`**: 汎用的な遅延初期化リソースを作成する
6. **`(lr *LazyResource) Get() (interface{}, error)`**: リソースを遅延取得する

**重要な実装要件：**
- すべての初期化は一度だけ実行されること
- 複数のGoroutineから同時アクセスしても安全であること  
- 初期化エラーが適切に処理されること
- 初期化完了後のアクセスは高速であること
- 1000個のGoroutineが並行してアクセスしても正確に動作すること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestConfigManager
=== RUN   TestConfigManager/Single_initialization
=== RUN   TestConfigManager/Concurrent_access
=== RUN   TestConfigManager/Error_handling
--- PASS: TestConfigManager (0.10s)
=== RUN   TestDatabasePool
=== RUN   TestDatabasePool/Singleton_behavior
=== RUN   TestDatabasePool/Concurrent_initialization
--- PASS: TestDatabasePool (0.05s)
=== RUN   TestLazyResource
=== RUN   TestLazyResource/Successful_initialization
=== RUN   TestLazyResource/Error_propagation
--- PASS: TestLazyResource (0.08s)
PASS
```

### レース検出テスト
```bash
$ go test -race
PASS
```
レースコンディションが検出されないことを確認できます。

### ベンチマーク実行例
```bash
$ go test -bench=.
BenchmarkConfigManagerAfterInit-8    	50000000	    25.4 ns/op
BenchmarkDatabasePoolAccess-8        	100000000	    10.1 ns/op
BenchmarkLazyResourceAfterInit-8     	30000000	    35.2 ns/op
```
初期化完了後のアクセスが非常に高速であることが確認できます。

### プログラム実行例
```bash
$ go run main.go
=== sync.Once Initialization Demo ===

1. First GetConfig() call:
Configuration loaded from file!
Config: &{DatabaseURL:postgres://localhost:5432/mydb APIKey:secret-api-key LogLevel:info}

2. Second GetConfig() call:
Config: &{DatabaseURL:postgres://localhost:5432/mydb APIKey:secret-api-key LogLevel:info}
(Note: No loading message - cached result used)

3. Database Pool Access:
Database pool initialized with 5 connections
First connection: connection-0
Second access: connection-0
(Note: Same singleton instance)

4. Concurrent access test:
Testing with 100 goroutines...
All goroutines received the same config instance: true
Initialization count: 1
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的な実装パターン
```go
type ConfigManager struct {
    config *Config
    once   sync.Once
    err    error
}

func (cm *ConfigManager) GetConfig() (*Config, error) {
    cm.once.Do(func() {
        cm.config, cm.err = loadConfigFromFile()
    })
    return cm.config, cm.err
}
```

### シングルトンパターン
```go
var (
    dbPool *DatabasePool
    dbOnce sync.Once
)

func GetDatabasePool() *DatabasePool {
    dbOnce.Do(func() {
        dbPool = &DatabasePool{
            connections: []string{"conn-0", "conn-1", "conn-2"},
        }
    })
    return dbPool
}
```

### 汎用的な遅延初期化
```go
type LazyResource struct {
    resource interface{}
    once     sync.Once
    err      error
    initFunc func() (interface{}, error)
}

func (lr *LazyResource) Get() (interface{}, error) {
    lr.once.Do(func() {
        lr.resource, lr.err = lr.initFunc()
    })
    return lr.resource, lr.err
}
```

### 使用する主要なパッケージ
- `sync.Once` - 一度だけ実行する仕組み
- `sync.Mutex` - 必要に応じた排他制御
- `sync.WaitGroup` - Goroutineの完了待機（テストで使用）

### デバッグのコツ
1. `go test -race`でレースコンディションを検出
2. 初期化関数内でログ出力して実行回数を確認
3. `go test -bench=.`でパフォーマンスを測定
4. 初期化エラーのテストケースを忘れずに

### よくある間違い
- Onceをポインタで渡してしまう → 値で保持する
- エラーハンドリングを忘れる → 構造体にエラーを保存
- パニック時の挙動を考慮しない → recover()で適切に処理
- 初期化関数が重すぎる → 必要最小限の処理にとどめる

### エラーハンドリングのパターン
```go
// パニック時の安全な処理
func (cm *ConfigManager) GetConfig() (*Config, error) {
    cm.once.Do(func() {
        defer func() {
            if r := recover(); r != nil {
                cm.err = fmt.Errorf("initialization panic: %v", r)
            }
        }()
        cm.config, cm.err = loadConfigFromFile()
    })
    return cm.config, cm.err
}
```

## 実行方法

```bash
# テスト実行
go test -v

# レースコンディション検出
go test -race

# ベンチマーク測定
go test -bench=.

# メモリ使用量も測定
go test -bench=. -benchmem

# プログラム実行
go run main.go
```

## 参考資料

- [Go sync.Once](https://pkg.go.dev/sync#Once)
- [Singleton Pattern in Go](https://golang.org/doc/faq#closures_and_goroutines)
- [Go Memory Model](https://golang.org/ref/mem)
- [Effective Go - Initialization](https://golang.org/doc/effective_go#init)