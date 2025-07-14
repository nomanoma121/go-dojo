# Day 03: sync.Mutex vs RWMutex実装

## 🎯 本日の目標 (Today's Goal)

Goの並行プログラミングにおける排他制御の核心技術であるsync.MutexとRWMutexを深く理解し、実装する。読み取り主体のワークロードにおけるパフォーマンス最適化技術を習得し、レースコンディションを完全に防ぐ安全で効率的な並行データ構造を構築できるようになる。

## 📖 解説 (Explanation)

### なぜミューテックスが必要なのか？

Goの並行プログラミングでは、複数のGoroutineが同じメモリ領域（変数、スライス、マップなど）に同時にアクセスする状況が頻繁に発生します。これが**レースコンディション**と呼ばれる深刻な問題を引き起こします。

#### レースコンディションの実例分析

```go
// ❌ 危険な例：レースコンディションが発生
var counter int

func increment() {
    counter++ // この操作は原子的ではない！
}

func problematicExample() {
    var wg sync.WaitGroup
    
    // 1000個のGoroutineが同時にcounterを変更
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            increment()
        }()
    }
    
    wg.Wait()
    fmt.Printf("Final counter: %d\n", counter) 
    // 期待値: 1000
    // 実際の結果: 500-1000の間のランダムな値（毎回異なる）
}
```

**なぜこの問題が発生するのか：**

`counter++`操作は、CPUレベルでは以下の3つのステップに分解されます：

```assembly
// counter++の実際の機械語レベル処理
1. LOAD  counter → register    // メモリから現在値を読み込み
2. INC   register             // レジスタの値を1増加
3. STORE register → counter   // 新しい値をメモリに書き戻し
```

**レースコンディションの発生パターン：**

```
時刻 | Goroutine A        | Goroutine B        | counter の値
-----|-------------------|-------------------|-------------
t1   | LOAD counter (0)  |                   | 0
t2   |                   | LOAD counter (0)  | 0  
t3   | INC register (1)  |                   | 0
t4   |                   | INC register (1)  | 0
t5   | STORE 1 → counter |                   | 1
t6   |                   | STORE 1 → counter | 1  ← 本来は2になるべき
```

#### メモリ可視性の問題

レースコンディション以外にも、メモリ可視性の問題があります：

```go
var ready bool
var message string

func writer() {
    message = "Hello, World!"  // 1. メッセージを設定
    ready = true              // 2. 準備完了フラグを設定
}

func reader() {
    for !ready {              // 3. フラグを待機
        time.Sleep(time.Millisecond)
    }
    fmt.Println(message)      // 4. メッセージを表示
}

// CPUキャッシュやコンパイラ最適化により、
// 1と2の順序が入れ替わる可能性がある
```

### sync.Mutex：基本的な排他制御

`sync.Mutex`は、**一度に一つのGoroutineだけ**が特定のコードセクション（クリティカルセクション）を実行できるようにする排他制御機構です。

#### 基本的な使用パターン

```go
import "sync"

type SafeCounter struct {
    mu    sync.Mutex
    value int
}

func (c *SafeCounter) Increment() {
    c.mu.Lock()         // クリティカルセクション開始
    defer c.mu.Unlock() // 関数終了時に自動解除
    
    c.value++           // 安全に値を変更
}

func (c *SafeCounter) Value() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    return c.value      // 安全に値を読み取り
}
```

#### より実用的なMutex活用例

```go
// スレッドセーフなキャッシュシステム
type SafeCache struct {
    mu    sync.Mutex
    items map[string]CacheItem
}

type CacheItem struct {
    Value     interface{}
    ExpiresAt time.Time
}

func NewSafeCache() *SafeCache {
    return &SafeCache{
        items: make(map[string]CacheItem),
    }
}

func (c *SafeCache) Set(key string, value interface{}, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.items[key] = CacheItem{
        Value:     value,
        ExpiresAt: time.Now().Add(ttl),
    }
}

func (c *SafeCache) Get(key string) (interface{}, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    item, exists := c.items[key]
    if !exists {
        return nil, false
    }
    
    // 有効期限チェック
    if time.Now().After(item.ExpiresAt) {
        delete(c.items, key)
        return nil, false
    }
    
    return item.Value, true
}

func (c *SafeCache) Delete(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    delete(c.items, key)
}

// バックグラウンドでの期限切れアイテム清掃
func (c *SafeCache) StartCleanup(interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        
        for range ticker.C {
            c.cleanup()
        }
    }()
}

func (c *SafeCache) cleanup() {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    now := time.Now()
    for key, item := range c.items {
        if now.After(item.ExpiresAt) {
            delete(c.items, key)
        }
    }
}
```

### sync.RWMutex：読み取り最適化型排他制御

`sync.RWMutex`（Reader-Writer Mutex）は、**読み取り処理は並行実行を許可し、書き込み処理のみ排他制御**を行う高度なミューテックスです。

#### RWMutexが解決する問題

通常のMutexでは、読み取り専用のアクセスであっても排他制御されるため、パフォーマンスのボトルネックになります：

```go
// ❌ Mutexによる非効率な読み取り制御
type ConfigManager struct {
    mu     sync.Mutex
    config map[string]string
}

func (cm *ConfigManager) GetConfig(key string) string {
    cm.mu.Lock()
    defer cm.mu.Unlock()
    
    return cm.config[key]
    // 読み取りだけなのに排他制御され、並行性が失われる
}

// 以下の処理は逐次実行される（非効率）
func inefficientConcurrentReads(cm *ConfigManager) {
    var wg sync.WaitGroup
    
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _ = cm.GetConfig("database_url") // 1つずつ順番に実行
        }()
    }
    wg.Wait()
}
```

#### RWMutexによる最適化

```go
// ✅ RWMutexによる効率的な読み書き制御
type OptimizedConfigManager struct {
    rwmu   sync.RWMutex
    config map[string]string
}

func NewOptimizedConfigManager() *OptimizedConfigManager {
    return &OptimizedConfigManager{
        config: make(map[string]string),
    }
}

// 読み取り操作：並行実行可能
func (cm *OptimizedConfigManager) GetConfig(key string) string {
    cm.rwmu.RLock()         // 読み取りロック（並行実行OK）
    defer cm.rwmu.RUnlock()
    
    return cm.config[key]
}

// 複数の設定を一度に取得：並行実行可能
func (cm *OptimizedConfigManager) GetConfigs(keys []string) map[string]string {
    cm.rwmu.RLock()
    defer cm.rwmu.RUnlock()
    
    result := make(map[string]string)
    for _, key := range keys {
        result[key] = cm.config[key]
    }
    return result
}

// 書き込み操作：排他実行
func (cm *OptimizedConfigManager) SetConfig(key, value string) {
    cm.rwmu.Lock()          // 書き込みロック（排他実行）
    defer cm.rwmu.Unlock()
    
    cm.config[key] = value
}

// 設定の一括更新：排他実行
func (cm *OptimizedConfigManager) UpdateConfigs(updates map[string]string) {
    cm.rwmu.Lock()
    defer cm.rwmu.Unlock()
    
    for key, value := range updates {
        cm.config[key] = value
    }
}

// 設定のバックアップ：読み取り中は書き込み不可
func (cm *OptimizedConfigManager) Backup() map[string]string {
    cm.rwmu.RLock()
    defer cm.rwmu.RUnlock()
    
    backup := make(map[string]string)
    for key, value := range cm.config {
        backup[key] = value
    }
    return backup
}
```

#### 実用的なRWMutex活用例：統計情報収集システム

```go
// 高頻度読み取り、低頻度書き込みの統計システム
type MetricsCollector struct {
    rwmu    sync.RWMutex
    metrics map[string]MetricData
}

type MetricData struct {
    Count       int64
    Sum         float64
    Min         float64
    Max         float64
    LastUpdated time.Time
}

func NewMetricsCollector() *MetricsCollector {
    return &MetricsCollector{
        metrics: make(map[string]MetricData),
    }
}

// 高頻度の読み取り操作（並行実行）
func (mc *MetricsCollector) GetMetric(name string) (MetricData, bool) {
    mc.rwmu.RLock()
    defer mc.rwmu.RUnlock()
    
    metric, exists := mc.metrics[name]
    return metric, exists
}

// 全メトリクスの取得（並行実行可能）
func (mc *MetricsCollector) GetAllMetrics() map[string]MetricData {
    mc.rwmu.RLock()
    defer mc.rwmu.RUnlock()
    
    result := make(map[string]MetricData)
    for name, metric := range mc.metrics {
        result[name] = metric
    }
    return result
}

// メトリクスの平均値計算（読み取り専用、並行実行可能）
func (mc *MetricsCollector) GetAverage(name string) float64 {
    mc.rwmu.RLock()
    defer mc.rwmu.RUnlock()
    
    metric, exists := mc.metrics[name]
    if !exists || metric.Count == 0 {
        return 0
    }
    
    return metric.Sum / float64(metric.Count)
}

// 低頻度の書き込み操作（排他実行）
func (mc *MetricsCollector) RecordValue(name string, value float64) {
    mc.rwmu.Lock()
    defer mc.rwmu.Unlock()
    
    metric, exists := mc.metrics[name]
    if !exists {
        mc.metrics[name] = MetricData{
            Count:       1,
            Sum:         value,
            Min:         value,
            Max:         value,
            LastUpdated: time.Now(),
        }
        return
    }
    
    // 既存メトリクスの更新
    metric.Count++
    metric.Sum += value
    if value < metric.Min {
        metric.Min = value
    }
    if value > metric.Max {
        metric.Max = value
    }
    metric.LastUpdated = time.Now()
    
    mc.metrics[name] = metric
}

// 古いメトリクスの削除（書き込み操作）
func (mc *MetricsCollector) CleanupOldMetrics(maxAge time.Duration) int {
    mc.rwmu.Lock()
    defer mc.rwmu.Unlock()
    
    cutoff := time.Now().Add(-maxAge)
    deletedCount := 0
    
    for name, metric := range mc.metrics {
        if metric.LastUpdated.Before(cutoff) {
            delete(mc.metrics, name)
            deletedCount++
        }
    }
    
    return deletedCount
}
```

#### RWMutexのパフォーマンス特性

```go
// パフォーマンステスト用のデータ構造
type PerformanceTestData struct {
    mutex   sync.Mutex
    rwMutex sync.RWMutex
    data    map[int]string
}

func NewPerformanceTestData() *PerformanceTestData {
    data := make(map[int]string)
    for i := 0; i < 1000; i++ {
        data[i] = fmt.Sprintf("value_%d", i)
    }
    
    return &PerformanceTestData{
        data: data,
    }
}

// Mutexを使った読み取り（すべて排他実行）
func (ptd *PerformanceTestData) ReadWithMutex(key int) string {
    ptd.mutex.Lock()
    defer ptd.mutex.Unlock()
    
    return ptd.data[key]
}

// RWMutexを使った読み取り（並行実行可能）
func (ptd *PerformanceTestData) ReadWithRWMutex(key int) string {
    ptd.rwMutex.RLock()
    defer ptd.rwMutex.RUnlock()
    
    return ptd.data[key]
}

// パフォーマンス比較テスト
func BenchmarkMutexReads(b *testing.B) {
    data := NewPerformanceTestData()
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _ = data.ReadWithMutex(rand.Intn(1000))
        }
    })
}

func BenchmarkRWMutexReads(b *testing.B) {
    data := NewPerformanceTestData()
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _ = data.ReadWithRWMutex(rand.Intn(1000))
        }
    })
}

// 期待される結果:
// BenchmarkMutexReads-8      1000000    1500 ns/op
// BenchmarkRWMutexReads-8   10000000     150 ns/op
// → RWMutexが約10倍高速（読み取り専用ワークロード）
```

### Mutex vs RWMutex の使い分け指針

#### Mutexを選ぶべき場合

1. **書き込み頻度が高い**: 読み取りと書き込みが同程度の頻度
2. **クリティカルセクションが短い**: ロック時間が非常に短い
3. **シンプルな実装が優先**: 可読性とメンテナンス性重視

```go
// 書き込み頻度が高い場合はMutexの方が効率的
type Counter struct {
    mu    sync.Mutex
    value int64
}

func (c *Counter) Increment() {
    c.mu.Lock()
    c.value++
    c.mu.Unlock()
}

func (c *Counter) Value() int64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.value
}
```

#### RWMutexを選ぶべき場合

1. **読み取り頻度が圧倒的に高い**: 読み取り：書き込み = 10：1以上
2. **クリティカルセクションが長い**: データベースアクセスやファイルI/O
3. **並行性が重要**: 高いスループットが要求される

```go
// 設定管理、キャッシュ、統計データなど
type ReadHeavyCache struct {
    rwmu sync.RWMutex
    data map[string]interface{}
}
```

### デッドロック防止パターン

#### ロック順序の統一

```go
type BankAccount struct {
    mu      sync.Mutex
    id      int
    balance float64
}

// ❌ デッドロックが発生する可能性
func dangerousTransfer(from, to *BankAccount, amount float64) {
    from.mu.Lock()
    to.mu.Lock()     // ロック順序が一定でない
    
    from.balance -= amount
    to.balance += amount
    
    to.mu.Unlock()
    from.mu.Unlock()
}

// ✅ デッドロックを防ぐ安全な実装
func safeTransfer(from, to *BankAccount, amount float64) {
    // IDの小さい順にロックを取得（順序の統一）
    first, second := from, to
    if from.id > to.id {
        first, second = to, from
    }
    
    first.mu.Lock()
    second.mu.Lock()
    
    from.balance -= amount
    to.balance += amount
    
    second.mu.Unlock()
    first.mu.Unlock()
}
```

#### タイムアウト付きロック（context使用）

```go
type TimeoutMutex struct {
    ch chan struct{}
}

func NewTimeoutMutex() *TimeoutMutex {
    return &TimeoutMutex{
        ch: make(chan struct{}, 1),
    }
}

func (tm *TimeoutMutex) TryLock(ctx context.Context) error {
    select {
    case tm.ch <- struct{}{}:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (tm *TimeoutMutex) Unlock() {
    select {
    case <-tm.ch:
    default:
        panic("unlock of unlocked mutex")
    }
}

// 使用例
func safeOperationWithTimeout() error {
    tm := NewTimeoutMutex()
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := tm.TryLock(ctx); err != nil {
        return fmt.Errorf("failed to acquire lock: %w", err)
    }
    defer tm.Unlock()
    
    // クリティカルセクション
    time.Sleep(2 * time.Second)
    
    return nil
}
```

```go
import "sync"

var (
    counter int
    mu      sync.Mutex
)

func safeIncrement() {
    mu.Lock()   // ロックを取得（他のGoroutineをブロック）
    counter++   // クリティカルセクション
    mu.Unlock() // ロックを解放
}
```

**Mutexの特徴：**
- 読み取りも書き込みも排他的
- シンプルで理解しやすい
- 書き込みが多い場合に適している

### sync.RWMutex：読み書き分離の排他制御

`sync.RWMutex`（読み書きミューテックス）は、**複数の読み取りは同時に許可し、書き込みは排他的に制御**する高度な仕組みです。

```go
import "sync"

var (
    data map[string]int
    rwMu sync.RWMutex
)

func read(key string) int {
    rwMu.RLock()         // 読み取りロック（他の読み取りと並行可能）
    defer rwMu.RUnlock()
    return data[key]
}

func write(key string, value int) {
    rwMu.Lock()          // 書き込みロック（完全に排他的）
    defer rwMu.Unlock()
    data[key] = value
}
```

**RWMutexの特徴：**
- 複数の読み取りGoroutineが同時実行可能
- 書き込み時は完全に排他的
- 読み取りが多い場合に大幅な性能向上
- Mutexより若干のオーバーヘッドあり

### パフォーマンス比較の実例

読み取りが多いワークロードでは、RWMutexが圧倒的に有利になります：

```go
// 読み取り90%、書き込み10%のワークロード
func benchmarkMutex() {
    var mu sync.Mutex
    data := make(map[string]int)
    
    // 90%が読み取り操作
    for i := 0; i < 9; i++ {
        go func() {
            for j := 0; j < 1000; j++ {
                mu.Lock()
                _ = data["key"]
                mu.Unlock()
            }
        }()
    }
    
    // 10%が書き込み操作
    go func() {
        for j := 0; j < 100; j++ {
            mu.Lock()
            data["key"] = j
            mu.Unlock()
        }
    }()
}
```

この場合、Mutexでは読み取りも1つずつしか実行できませんが、RWMutexなら9つの読み取りGoroutineが並列実行できます。

### 実際の使用場面

**Mutexを選ぶべき場面：**
- 書き込み操作が頻繁（読み書きの比率が1:1に近い）
- シンプルな排他制御で十分
- パフォーマンスよりも保守性を重視

**RWMutexを選ぶべき場面：**
- 読み取り操作が圧倒的に多い（設定情報、キャッシュなど）
- 高い並行性能が必要
- 複数の読み取りGoroutineを活用したい

### ベストプラクティス

1. **deferを使った確実なUnlock**
   ```go
   mu.Lock()
   defer mu.Unlock() // 必ずUnlockが実行される
   ```

2. **適切な粒度でのロック**
   ```go
   // 悪い例：粒度が粗すぎる
   func processAll() {
       mu.Lock()
       defer mu.Unlock()
       for i := 0; i < 1000000; i++ {
           // 長時間のロック
       }
   }
   
   // 良い例：必要な部分のみロック
   func processItem(item Item) {
       // 重い処理はロック外で
       result := heavyComputation(item)
       
       mu.Lock()
       updateSharedData(result) // 短時間のロック
       mu.Unlock()
   }
   ```

3. **デッドロックの回避**
   ```go
   // 悪い例：デッドロックの可能性
   func transferMoney(from, to *Account, amount int) {
       from.mu.Lock()
       to.mu.Lock()   // デッドロックリスク
       // 処理...
       to.mu.Unlock()
       from.mu.Unlock()
   }
   
   // 良い例：一貫した順序でロック
   func transferMoney(from, to *Account, amount int) {
       if from.id < to.id {
           from.mu.Lock()
           to.mu.Lock()
       } else {
           to.mu.Lock()
           from.mu.Lock()
       }
       // 処理...
       to.mu.Unlock()
       from.mu.Unlock()
   }
   ```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **`NewMutexCache()`**: sync.Mutexを使ったキャッシュを初期化する
2. **`NewRWMutexCache()`**: sync.RWMutexを使ったキャッシュを初期化する  
3. **`(c *MutexCache) Get(key string) (string, bool)`**: 値を安全に取得する
4. **`(c *MutexCache) Set(key, value string)`**: キーと値を安全に設定する
5. **`(c *MutexCache) Delete(key string)`**: キーを安全に削除する
6. **`(c *MutexCache) Len() int`**: キャッシュのサイズを安全に取得する
7. **同様のメソッドをRWMutexCacheにも実装**

**重要な実装要件：**
- MutexCacheは`sync.Mutex`を使用
- RWMutexCacheは`sync.RWMutex`を使用し、読み取り操作で`RLock()`を使用
- レースコンディションが発生しないこと
- 1000個のGoroutineが並行してアクセスしても正確に動作すること
- パフォーマンステストで読み取り中心のワークロードでRWMutexが高速であること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestMutexCache
=== RUN   TestMutexCache/Sequential_operations
=== RUN   TestMutexCache/Concurrent_operations
=== RUN   TestMutexCache/Race_condition_test
--- PASS: TestMutexCache (0.15s)
=== RUN   TestRWMutexCache  
=== RUN   TestRWMutexCache/Sequential_operations
=== RUN   TestRWMutexCache/Concurrent_reads
=== RUN   TestRWMutexCache/Mixed_read_write
--- PASS: TestRWMutexCache (0.20s)
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
BenchmarkMutexCacheRead-8        	2000000	   800 ns/op
BenchmarkRWMutexCacheRead-8      	10000000	   150 ns/op  
BenchmarkMutexCacheWrite-8       	5000000	   300 ns/op
BenchmarkRWMutexCacheWrite-8     	4500000	   350 ns/op
BenchmarkMutexCacheMixed-8       	1500000	   1200 ns/op
BenchmarkRWMutexCacheMixed-8     	6000000	   400 ns/op
```
読み取り中心のワークロードでRWMutexの方が5倍程度高速になることが確認できます。

### プログラム実行例
```bash
$ go run main.go
=== Mutex vs RWMutex Performance Comparison ===

Testing with 100 goroutines, 1000 operations each...

Mutex Cache Results:
- Cache size: 100 entries
- Read operations took: 45.2ms
- Write operations took: 12.8ms
- Total time: 58.0ms

RWMutex Cache Results:  
- Cache size: 100 entries
- Read operations took: 8.9ms
- Write operations took: 15.1ms
- Total time: 24.0ms

RWMutex is 2.4x faster for mixed read-heavy workloads!
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的な実装パターン
```go
type MutexCache struct {
    data  map[string]string
    mutex sync.Mutex
}

func (c *MutexCache) Get(key string) (string, bool) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    value, exists := c.data[key]
    return value, exists
}
```

### RWMutexの読み取り操作
```go
func (c *RWMutexCache) Get(key string) (string, bool) {
    c.rwmutex.RLock()  // 読み取りロック
    defer c.rwmutex.RUnlock()
    
    value, exists := c.data[key]
    return value, exists
}
```

### RWMutexの書き込み操作
```go
func (c *RWMutexCache) Set(key, value string) {
    c.rwmutex.Lock()   // 書き込みロック（排他的）
    defer c.rwmutex.Unlock()
    
    c.data[key] = value
}
```

### 使用する主要なパッケージ
- `sync.Mutex` - 基本的な排他制御
- `sync.RWMutex` - 読み書き分離の排他制御  
- `sync.WaitGroup` - Goroutineの完了待機（テストで使用）

### デバッグのコツ
1. `go test -race`でレースコンディションを検出
2. `go test -v`で詳細なテスト結果を確認
3. `go test -bench=.`でパフォーマンスを測定
4. 必要に応じて`time.Sleep()`でタイミングを調整してテスト

### よくある間違い
- Unlockし忘れ → `defer`を使って確実に解放
- 読み取り操作で書き込みロックを使用 → RWMutexでは`RLock()`を使用
- ロック範囲が広すぎる → 必要最小限の範囲でロック
- nilマップへのアクセス → 初期化を忘れずに

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

- [Go Memory Model](https://golang.org/ref/mem)
- [sync package documentation](https://pkg.go.dev/sync)
- [Effective Go - Concurrency](https://golang.org/doc/effective_go#concurrency)
- [Go sync.Mutex](https://pkg.go.dev/sync#Mutex)
- [Go sync.RWMutex](https://pkg.go.dev/sync#RWMutex)