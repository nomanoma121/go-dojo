# Day 07: Worker Pool (結果の受信)

## 🎯 本日の目標 (Today's Goal)

このチャレンジを通して、以下のスキルを身につけることができます：

- **並列処理の結果を効率的に収集・管理できるようになる**
- **タスクの順序保証機能を実装できるようになる**
- **複数の結果を集約して統計情報を作成できるようになる**
- **部分的な失敗を適切にハンドリングし、システムの堅牢性を高められるようになる**

## 📖 解説 (Explanation)

### なぜ結果収集が重要なのか？

Worker Poolでタスクを並列処理した後、結果を適切に収集・管理することは非常に重要です。単純に結果を受け取るだけでは、以下の問題が発生します：

```go
// 【結果収集の重要性】Worker Poolからの効率的な結果管理
// ❌ 問題例：不適切な結果管理によるカオス状態
func badResultManagement() {
    pool := NewWorkerPool(5, 100)
    pool.Start()
    
    // 🚨 災害例：タスクIDと結果の紐付けが不可能
    for i := 0; i < 1000; i++ {
        pool.SubmitTask(Task{ID: i, Data: i})
        // ❌ タスクの送信順序と結果の到着順序が異なる
        // ❌ どのタスクがどの結果を生成したか不明
    }
    
    // 🚨 災害例：結果の順序が保証されない混沌状態
    for result := range pool.GetResults() {
        fmt.Println(result) 
        // ❌ Task 1, Task 100, Task 5, Task 50... ランダムな順序
        // ❌ エラー処理が困難（どのタスクが失敗したか不明）
        // ❌ 進捗管理が不可能（完了したタスク数が不明）
    }
    // 結果：データの整合性がない、デバッグが困難、運用不可能
}

// ✅ 正解：プロダクション品質の結果収集システム
func properResultManagement() {
    // 【STEP 1】結果コレクターの初期化
    collector := NewResultCollector(1000, true) // 順序保証あり
    collector.Start()
    
    pool := NewWorkerPool(5, 100)
    pool.SetResultCollector(collector) // 結果の送信先を設定
    pool.Start()
    
    // 【STEP 2】タスクの投入（順序付きID付与）
    for i := 0; i < 1000; i++ {
        task := Task{
            ID:      i,
            Data:    i,
            Created: time.Now(),
        }
        pool.SubmitTask(task)
        // ✅ 各タスクに一意のIDを付与
        // ✅ 作成時刻を記録して処理時間を追跡
    }
    
    // 【STEP 3】順序保証付き結果収集
    results := collector.GetOrderedResults()
    for result := range results {
        // ✅ タスクID順（0, 1, 2, 3...）で結果を取得
        // ✅ エラーハンドリングが明確
        // ✅ 処理時間やワーカーIDなどの詳細情報が利用可能
        
        if result.Error != nil {
            log.Printf("Task %d failed: %v", result.TaskID, result.Error)
        } else {
            log.Printf("Task %d completed in %v by worker %d", 
                result.TaskID, result.Duration, result.WorkerID)
        }
    }
    
    // 【STEP 4】集約統計の取得
    stats := collector.GetStatistics()
    log.Printf("Success: %d, Errors: %d, Avg Duration: %v", 
        stats.SuccessCount, stats.ErrorCount, stats.AverageDuration)
}
```

この方法の問題点：
1. **順序の不保証**: 結果がタスクの投入順序と異なる順序で返ってくる
2. **結果の紐付け困難**: どの結果がどのタスクのものかわからない
3. **エラー処理の複雑化**: 部分的な失敗の処理が困難
4. **集約処理の欠如**: 全体の統計情報や集約結果が得られない

### ResultCollectorパターンの基本概念

`ResultCollector`は、Worker Poolからの結果を効率的に収集・管理する仕組みです：

```go
import (
    "sync"
    "sort"
    "context"
)

type Result struct {
    TaskID    int
    Output    interface{}
    Error     error
    Duration  time.Duration
    WorkerID  int
    Timestamp time.Time
}

type ResultCollector struct {
    results      map[int]Result
    resultChan   chan Result
    orderedMode  bool
    maxResults   int
    completed    int
    mu           sync.RWMutex
    done         chan struct{}
    ctx          context.Context
    cancel       context.CancelFunc
}

func NewResultCollector(maxResults int, ordered bool) *ResultCollector {
    ctx, cancel := context.WithCancel(context.Background())
    return &ResultCollector{
        results:     make(map[int]Result),
        resultChan:  make(chan Result, maxResults),
        orderedMode: ordered,
        maxResults:  maxResults,
        done:        make(chan struct{}),
        ctx:         ctx,
        cancel:      cancel,
    }
}
```

**ResultCollectorの特徴：**
- **結果の順序保証**: タスクIDに基づいた順序での結果取得
- **効率的な収集**: チャネルベースの非同期収集
- **集約機能**: 統計情報や集約結果の計算
- **エラー処理**: 部分的な失敗への対応

### 順序保証付き結果収集

タスクの投入順序で結果を取得する仕組み：

```go
func (rc *ResultCollector) Start() {
    go func() {
        defer close(rc.done)
        expectedID := 0
        
        for {
            select {
            case result := <-rc.resultChan:
                rc.mu.Lock()
                rc.results[result.TaskID] = result
                rc.completed++
                
                // 順序保証モードの場合
                if rc.orderedMode {
                    rc.flushOrderedResults(&expectedID)
                }
                
                // 全て完了した場合
                if rc.completed >= rc.maxResults {
                    rc.mu.Unlock()
                    return
                }
                rc.mu.Unlock()
                
            case <-rc.ctx.Done():
                return
            }
        }
    }()
}

func (rc *ResultCollector) flushOrderedResults(expectedID *int) {
    for {
        if result, exists := rc.results[*expectedID]; exists {
            // 順序通りに結果を処理
            rc.processOrderedResult(result)
            delete(rc.results, *expectedID)
            *expectedID++
        } else {
            break
        }
    }
}

func (rc *ResultCollector) processOrderedResult(result Result) {
    // 順序保証された結果の処理
    // ログ出力、ファイル書き込み、DBへの保存など
}
```

### 結果集約とバッチ処理

複数の結果をまとめて効率的に処理：

```go
type AggregatedResult struct {
    TotalTasks     int
    SuccessCount   int
    ErrorCount     int
    TotalDuration  time.Duration
    AvgDuration    time.Duration
    Results        []Result
    Errors         []error
    Summary        interface{}
}

type BatchProcessor struct {
    batchSize   int
    timeout     time.Duration
    processor   func([]Result) interface{}
    buffer      []Result
    lastFlush   time.Time
    mu          sync.Mutex
}

func NewBatchProcessor(batchSize int, timeout time.Duration, processor func([]Result) interface{}) *BatchProcessor {
    return &BatchProcessor{
        batchSize: batchSize,
        timeout:   timeout,
        processor: processor,
        buffer:    make([]Result, 0, batchSize),
        lastFlush: time.Now(),
    }
}

func (bp *BatchProcessor) AddResult(result Result) {
    bp.mu.Lock()
    defer bp.mu.Unlock()
    
    bp.buffer = append(bp.buffer, result)
    
    // バッチサイズに達した場合
    if len(bp.buffer) >= bp.batchSize {
        bp.flush()
    }
    
    // タイムアウトに達した場合
    if time.Since(bp.lastFlush) >= bp.timeout {
        bp.flush()
    }
}

func (bp *BatchProcessor) flush() {
    if len(bp.buffer) == 0 {
        return
    }
    
    // バッチ処理を実行
    summary := bp.processor(bp.buffer)
    
    // バッファをリセット
    bp.buffer = bp.buffer[:0]
    bp.lastFlush = time.Now()
    
    // 集約結果を処理
    bp.handleAggregatedResult(summary)
}
```

### 高度な結果フィルタリングと変換

結果の条件付きフィルタリングと変換：

```go
type ResultFilter func(Result) bool
type ResultTransformer func(Result) Result

type FilteredResultCollector struct {
    *ResultCollector
    filters      []ResultFilter
    transformers []ResultTransformer
}

func (frc *FilteredResultCollector) AddFilter(filter ResultFilter) {
    frc.filters = append(frc.filters, filter)
}

func (frc *FilteredResultCollector) AddTransformer(transformer ResultTransformer) {
    frc.transformers = append(frc.transformers, transformer)
}

func (frc *FilteredResultCollector) ProcessResult(result Result) bool {
    // フィルタ適用
    for _, filter := range frc.filters {
        if !filter(result) {
            return false // フィルタに引っかかった
        }
    }
    
    // 変換適用
    for _, transformer := range frc.transformers {
        result = transformer(result)
    }
    
    // 結果を収集
    frc.resultChan <- result
    return true
}

// 使用例
func setupFiltersAndTransformers() *FilteredResultCollector {
    collector := &FilteredResultCollector{
        ResultCollector: NewResultCollector(1000, true),
    }
    
    // 成功した結果のみを収集
    collector.AddFilter(func(r Result) bool {
        return r.Error == nil
    })
    
    // 処理時間が長いタスクのみを収集
    collector.AddFilter(func(r Result) bool {
        return r.Duration > 100*time.Millisecond
    })
    
    // 結果を正規化
    collector.AddTransformer(func(r Result) Result {
        if r.Output != nil {
            r.Output = normalizeOutput(r.Output)
        }
        return r
    })
    
    return collector
}
```

### エラー処理と回復戦略

部分的な失敗に対する堅牢な処理：

```go
type ErrorStrategy int

const (
    FailFast ErrorStrategy = iota  // 最初のエラーで停止
    FailSafe                      // エラーを記録して続行
    Retry                         // エラー時に再試行
)

type RobustResultCollector struct {
    *ResultCollector
    errorStrategy ErrorStrategy
    maxRetries    int
    retryDelay    time.Duration
    failedTasks   chan Task
}

func (rrc *RobustResultCollector) HandleResult(result Result) {
    if result.Error != nil {
        switch rrc.errorStrategy {
        case FailFast:
            rrc.failFast(result)
        case FailSafe:
            rrc.failSafe(result)
        case Retry:
            rrc.retryTask(result)
        }
        return
    }
    
    // 成功した結果を収集
    rrc.resultChan <- result
}

func (rrc *RobustResultCollector) failFast(result Result) {
    // 即座に処理を停止
    rrc.cancel()
}

func (rrc *RobustResultCollector) failSafe(result Result) {
    // エラーを記録して処理を続行
    errorResult := Result{
        TaskID: result.TaskID,
        Error:  result.Error,
        Output: nil,
    }
    rrc.resultChan <- errorResult
}

func (rrc *RobustResultCollector) retryTask(result Result) {
    // 再試行可能な場合は再試行キューに追加
    if result.TaskID < rrc.maxRetries {
        go func() {
            time.Sleep(rrc.retryDelay)
            // 元のタスクを再試行キューに送信
            // rrc.failedTasks <- originalTask
        }()
    } else {
        rrc.failSafe(result) // 最大再試行数に達した場合は記録
    }
}
```

### リアルタイム統計とモニタリング

処理の進行状況をリアルタイムで監視：

```go
type ResultStats struct {
    TotalProcessed  int64
    SuccessCount    int64
    ErrorCount      int64
    AvgProcessTime  time.Duration
    MinProcessTime  time.Duration
    MaxProcessTime  time.Duration
    Throughput      float64
    LastUpdated     time.Time
    mu              sync.RWMutex
}

func (rs *ResultStats) Update(result Result) {
    rs.mu.Lock()
    defer rs.mu.Unlock()
    
    rs.TotalProcessed++
    if result.Error != nil {
        rs.ErrorCount++
    } else {
        rs.SuccessCount++
    }
    
    // 処理時間の統計を更新
    if rs.MinProcessTime == 0 || result.Duration < rs.MinProcessTime {
        rs.MinProcessTime = result.Duration
    }
    if result.Duration > rs.MaxProcessTime {
        rs.MaxProcessTime = result.Duration
    }
    
    // 平均処理時間を更新（移動平均）
    alpha := 0.1 // 重み
    if rs.AvgProcessTime == 0 {
        rs.AvgProcessTime = result.Duration
    } else {
        rs.AvgProcessTime = time.Duration(float64(rs.AvgProcessTime)*(1-alpha) + float64(result.Duration)*alpha)
    }
    
    // スループットを計算
    elapsed := time.Since(rs.LastUpdated)
    if elapsed > time.Second {
        rs.Throughput = float64(rs.TotalProcessed) / elapsed.Seconds()
        rs.LastUpdated = time.Now()
    }
}

func (rs *ResultStats) GetSnapshot() ResultStats {
    rs.mu.RLock()
    defer rs.mu.RUnlock()
    
    snapshot := *rs
    return snapshot
}
```

### 結果のストリーミングとパイプライン

大量の結果を効率的にストリーミング処理：

```go
type ResultStream struct {
    input      <-chan Result
    output     chan Result
    processors []func(Result) Result
    filters    []func(Result) bool
    done       chan struct{}
}

func NewResultStream(input <-chan Result) *ResultStream {
    return &ResultStream{
        input:  input,
        output: make(chan Result),
        done:   make(chan struct{}),
    }
}

func (rs *ResultStream) AddProcessor(processor func(Result) Result) *ResultStream {
    rs.processors = append(rs.processors, processor)
    return rs
}

func (rs *ResultStream) AddFilter(filter func(Result) bool) *ResultStream {
    rs.filters = append(rs.filters, filter)
    return rs
}

func (rs *ResultStream) Start() <-chan Result {
    go func() {
        defer close(rs.output)
        
        for result := range rs.input {
            // フィルタを適用
            skip := false
            for _, filter := range rs.filters {
                if !filter(result) {
                    skip = true
                    break
                }
            }
            if skip {
                continue
            }
            
            // プロセッサを適用
            for _, processor := range rs.processors {
                result = processor(result)
            }
            
            rs.output <- result
        }
    }()
    
    return rs.output
}
```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **`NewResultCollector(maxResults int, ordered bool) *ResultCollector`**: 結果コレクターを初期化する
2. **`(rc *ResultCollector) Start()`**: 結果収集を開始する
3. **`(rc *ResultCollector) SubmitResult(result Result)`**: 結果を投入する
4. **`(rc *ResultCollector) GetResults() []Result`**: 収集した結果を取得する
5. **`(rc *ResultCollector) GetAggregatedResult() AggregatedResult`**: 集約結果を取得する
6. **`NewBatchProcessor(batchSize int, processor func([]Result) interface{}) *BatchProcessor`**: バッチ処理を作成する
7. **`(bp *BatchProcessor) ProcessBatch(results []Result) interface{}`**: バッチを処理する

**重要な実装要件：**
- 順序指定時はタスクIDの順序で結果を返すこと
- 結果の集約統計（成功数、エラー数、平均時間など）を正しく計算すること
- 大量の結果（10,000件以上）を効率的に処理できること
- レースコンディションが発生しないこと
- バッチ処理で複数の結果をまとめて効率的に処理できること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestResultCollector
=== RUN   TestResultCollector/Ordered_collection
=== RUN   TestResultCollector/Unordered_collection
=== RUN   TestResultCollector/Result_aggregation
=== RUN   TestResultCollector/Error_handling
--- PASS: TestResultCollector (0.20s)
=== RUN   TestBatchProcessor
=== RUN   TestBatchProcessor/Batch_processing
=== RUN   TestBatchProcessor/Statistics_aggregation
--- PASS: TestBatchProcessor (0.15s)
PASS
```

### ベンチマーク実行例
```bash
$ go test -bench=.
BenchmarkOrderedCollection-8    	   10000	    150000 ns/op
BenchmarkUnorderedCollection-8  	   50000	     30000 ns/op
BenchmarkBatchProcessing-8      	    5000	    250000 ns/op
```
順序保証なしの方が5倍高速で、バッチ処理により効率的な集約が可能なことが確認できます。

### プログラム実行例
```bash
$ go run main.go
=== Worker Pool Result Collection Demo ===

Processing 1000 tasks with 5 workers...

Result Collection (Ordered):
- Task 1: Result: 2 (1ms)
- Task 2: Result: 4 (2ms)
- Task 3: Result: 6 (1ms)
...
- Task 1000: Result: 2000 (2ms)

Aggregated Statistics:
- Total tasks: 1000
- Success rate: 98.5% (985/1000)
- Error rate: 1.5% (15/1000)
- Average processing time: 1.5ms
- Total processing time: 5.2s
- Throughput: 192 tasks/sec

Batch Processing Results:
- Batch 1 (100 results): Sum=5050, Avg=50.5
- Batch 2 (100 results): Sum=15150, Avg=151.5
...
- Final batch (85 results): Sum=85425, Avg=1004.4

Processing complete!
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的な結果収集
```go
func (rc *ResultCollector) Start() {
    go func() {
        for result := range rc.resultChan {
            rc.mu.Lock()
            rc.results[result.TaskID] = result
            rc.completed++
            rc.mu.Unlock()
        }
    }()
}
```

### 順序保証の実装
```go
func (rc *ResultCollector) GetOrderedResults() []Result {
    rc.mu.RLock()
    defer rc.mu.RUnlock()
    
    ordered := make([]Result, 0, len(rc.results))
    for i := 0; i < len(rc.results); i++ {
        if result, exists := rc.results[i]; exists {
            ordered = append(ordered, result)
        }
    }
    return ordered
}
```

### 集約統計の計算
```go
func (rc *ResultCollector) GetAggregatedResult() AggregatedResult {
    rc.mu.RLock()
    defer rc.mu.RUnlock()
    
    var totalDuration time.Duration
    var successCount, errorCount int
    
    for _, result := range rc.results {
        totalDuration += result.Duration
        if result.Error != nil {
            errorCount++
        } else {
            successCount++
        }
    }
    
    return AggregatedResult{
        TotalTasks:   len(rc.results),
        SuccessCount: successCount,
        ErrorCount:   errorCount,
        AvgDuration:  totalDuration / time.Duration(len(rc.results)),
    }
}
```

### 使用する主要なパッケージ
- `sync.RWMutex` - 結果マップの排他制御
- `sort` - 結果の順序保証
- `time` - 統計情報の計算
- `context` - キャンセレーション処理

### デバッグのコツ
1. `go test -race`でレースコンディションを検出
2. 順序保証のロジックをテストデータで確認
3. 集約統計の計算が正確か検証
4. バッチ処理のタイミングを調整

### よくある間違い
- 順序保証の実装漏れ → TaskIDでソート
- 統計計算の誤り → 分母がゼロの場合を考慮
- メモリリーク → 適切にチャネルをクローズ
- レースコンディション → 適切な排他制御

## 実行方法

```bash
# テスト実行
go test -v

# レースコンディション検出
go test -race

# ベンチマーク測定
go test -bench=.

# メモリプロファイル
go test -bench=. -memprofile=mem.prof

# プログラム実行
go run main.go
```

## 参考資料

- [Go sync package](https://pkg.go.dev/sync)
- [Channel Best Practices](https://golang.org/doc/effective_go#channels)
- [Go Memory Model](https://golang.org/ref/mem)
- [Concurrency Patterns](https://blog.golang.org/context)