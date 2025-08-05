# Day 06: Worker Poolパターン

## 🎯 本日の目標 (Today's Goal)

このチャレンジを通して、以下のスキルを身につけることができます：

- **固定数のGoroutineを使って大量のタスクを効率的に処理できるようになる**
- **システムリソースの使用量を制御し、過負荷を防ぐ方法を理解できるようになる**
- **Goroutineプールによる並行処理の最適化技術をマスターする**
- **グレースフルシャットダウンによる安全な停止処理を実装できるようになる**

## 📖 解説 (Explanation)

### なぜWorker Poolパターンが必要なのか？

Webサーバーやバッチ処理システムでは、大量のタスクを並行処理する必要があります。しかし、タスクごとに新しいGoroutineを作成すると、以下の問題が発生します：

```go
// ❌ 【問題のある例】：無制限のGoroutine作成 - システム破綻の原因
func processTasksBadly(tasks []Task) {
    for _, task := range tasks {
        go func(t Task) {
            // 【致命的問題】各タスクに対して新しいGoroutineを作成
            // 問題の詳細：
            // 1. Goroutineスタック: 各Goroutine = 2-8KB
            // 2. 100万タスク = 2-8GB のメモリ消費
            // 3. CPUスケジューラーの負荷爆発
            // 4. OSレベルでのリソース制限に到達
            
            processTaskExpensive(t)  // 重い処理（DB呼び出し、API呼び出し等）
            
            // 【追加問題】：
            // - 同時DB接続数がDB接続プール上限を超過
            // - 外部API呼び出しが制限を超えてレート制限発動
            // - ファイルディスクリプタ枯渇
            // - メモリ枯渇によるOOM Kill
        }(task)
    }
    
    // 【災害シナリオ】：
    // 100万タスクがあれば100万個のGoroutineが作成される！
    // 結果：
    // - メモリ使用量: 数GB
    // - CPU使用率: 100%（コンテキストスイッチオーバーヘッド）
    // - アプリケーション応答停止
    // - 最悪の場合、サーバークラッシュ
}
```

この方法の問題点：
1. **メモリ使用量の爆発**: Goroutineスタックで大量のメモリを消費
2. **スケジューラの負荷**: 大量のGoroutineがCPUコンテキストスイッチを増加
3. **リソース枯渇**: ファイルディスクリプタやDB接続などの限界
4. **制御不能**: タスクの実行順序やスループットの制御が困難

### Worker Poolパターンの基本概念

Worker Poolは、**固定数のワーカーGoroutine**でタスクを処理する仕組みです：

```go
import (
    "sync"
    "context"
)

// 【正しい実装】Worker Poolパターン - 本格的なプロダクション対応設計
type WorkerPool struct {
    numWorkers int           // 【設定】ワーカーGoroutine数（固定）
    taskQueue  chan Task    // 【キュー】処理待ちタスクのバッファ
    resultChan chan Result  // 【出力】処理結果の収集チャネル
    quit       chan struct{} // 【制御】シャットダウンシグナル
    wg         sync.WaitGroup // 【同期】全ワーカーの終了待機
}

// 【コンストラクタ】適切なサイズ設定が重要
func NewWorkerPool(numWorkers, queueSize int) *WorkerPool {
    return &WorkerPool{
        numWorkers: numWorkers,
        
        // 【重要】バッファ付きチャネルでブロッキングを回避
        // queueSize設定の指針：
        // - 小さすぎる: プロデューサーがブロック
        // - 大きすぎる: メモリ使用量増加
        // - 推奨: ワーカー数の2-10倍
        taskQueue:  make(chan Task, queueSize),
        resultChan: make(chan Result, queueSize),
        
        quit:       make(chan struct{}),
    }
}

// 【起動】固定数のワーカーGoroutineを開始
func (wp *WorkerPool) Start() {
    for i := 0; i < wp.numWorkers; i++ {
        wp.wg.Add(1)
        go wp.worker(i)  // 各ワーカーに一意IDを付与
    }
}

// 【ワーカー実装】各Goroutineで実行されるメインループ
func (wp *WorkerPool) worker(id int) {
    defer wp.wg.Done()  // 【重要】終了時にWaitGroupを減算
    
    // 【メインループ】タスク処理またはシャットダウン待機
    for {
        select {
        // 【タスク処理】キューからタスクを受信
        case task := <-wp.taskQueue:
            // 【実際の処理】ここでビジネスロジックを実行
            result := processTask(task)
            
            // 【結果送信】処理結果を結果チャネルに送信
            // 注意：resultChanが満杯の場合はブロックする可能性
            wp.resultChan <- result
            
        // 【シャットダウン】quit チャネルからシグナル受信
        case <-wp.quit:
            // 【グレースフル終了】現在処理中のタスクを完了してから終了
            return
        }
    }
}

// 【パフォーマンス特性】：
// - メモリ使用量: O(numWorkers) - 固定で予測可能
// - CPU効率: CPUコア数に最適化されたワーカー数で最大効率
// - スループット: タスク処理能力 × ワーカー数
// - レイテンシ: キューサイズとワーカー数で調整可能

```

**Worker Poolの利点：**
- **リソース制御**: ワーカー数を固定することでメモリ・CPU使用量を制限
- **スループット調整**: ワーカー数を調整してパフォーマンスを最適化
- **安定性**: システム負荷の予測可能性
- **拡張性**: 水平スケーリングへの対応

### タスクキューの設計

効率的なタスクキューは、バッファ付きチャネルで実装します：

```go
type Task struct {
    ID       int
    Data     interface{}
    Priority int
    Timeout  time.Duration
}

// バッファサイズの考慮事項
func NewTaskQueue(bufferSize int) chan Task {
    // バッファサイズ = ワーカー数 × 2〜5 が一般的
    return make(chan Task, bufferSize)
}

// タスクの投入
func (wp *WorkerPool) SubmitTask(task Task) error {
    select {
    case wp.taskQueue <- task:
        return nil
    default:
        return errors.New("task queue is full")
    }
}

// ノンブロッキング投入（タイムアウト付き）
func (wp *WorkerPool) SubmitTaskWithTimeout(task Task, timeout time.Duration) error {
    select {
    case wp.taskQueue <- task:
        return nil
    case <-time.After(timeout):
        return errors.New("submit timeout")
    }
}
```

### 優先度付きタスクキュー

重要なタスクを優先的に処理する仕組み：

```go
import "container/heap"

type PriorityTaskQueue struct {
    tasks []Task
    mu    sync.Mutex
}

func (pq *PriorityTaskQueue) Len() int { return len(pq.tasks) }
func (pq *PriorityTaskQueue) Less(i, j int) bool {
    return pq.tasks[i].Priority > pq.tasks[j].Priority // 高優先度が先
}
func (pq *PriorityTaskQueue) Swap(i, j int) { pq.tasks[i], pq.tasks[j] = pq.tasks[j], pq.tasks[i] }

func (pq *PriorityTaskQueue) Push(x interface{}) {
    pq.tasks = append(pq.tasks, x.(Task))
}

func (pq *PriorityTaskQueue) Pop() interface{} {
    old := pq.tasks
    n := len(old)
    task := old[n-1]
    pq.tasks = old[0 : n-1]
    return task
}

type PriorityWorkerPool struct {
    workers    int
    taskQueue  *PriorityTaskQueue
    taskChan   chan Task
    quit       chan struct{}
    wg         sync.WaitGroup
}

func (pwp *PriorityWorkerPool) taskDispatcher() {
    ticker := time.NewTicker(10 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            pwp.taskQueue.mu.Lock()
            if pwp.taskQueue.Len() > 0 {
                task := heap.Pop(pwp.taskQueue).(Task)
                pwp.taskQueue.mu.Unlock()
                
                select {
                case pwp.taskChan <- task:
                case <-pwp.quit:
                    return
                }
            } else {
                pwp.taskQueue.mu.Unlock()
            }
        case <-pwp.quit:
            return
        }
    }
}
```

### グレースフルシャットダウンの実装

進行中のタスクを安全に完了してから停止する仕組み：

```go
func (wp *WorkerPool) GracefulShutdown(timeout time.Duration) error {
    // 新しいタスクの受付を停止
    close(wp.taskQueue)
    
    // 完了通知用チャネル
    done := make(chan struct{})
    
    go func() {
        wp.wg.Wait() // 全ワーカーの完了を待機
        close(done)
    }()
    
    // タイムアウト付きで完了を待機
    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        close(wp.quit) // 強制終了
        return errors.New("shutdown timeout")
    }
}

// より高度なシャットダウン（段階的）
func (wp *WorkerPool) AdvancedShutdown() error {
    // Phase 1: 新しいタスク受付停止
    close(wp.taskQueue)
    
    // Phase 2: 処理中タスクの完了を待機（短時間）
    done := make(chan struct{})
    go func() {
        wp.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-time.After(30 * time.Second):
        // Phase 3: 強制終了
        close(wp.quit)
        
        // Phase 4: 最終的な完了確認
        select {
        case <-done:
            return nil
        case <-time.After(5 * time.Second):
            return errors.New("force shutdown timeout")
        }
    }
}
```

### 結果収集パターン

タスクの処理結果を効率的に収集：

```go
type Result struct {
    TaskID    int
    Output    interface{}
    Error     error
    Duration  time.Duration
    WorkerID  int
}

type ResultCollector struct {
    results    chan Result
    collected  []Result
    mu         sync.Mutex
    wg         sync.WaitGroup
}

func NewResultCollector(bufferSize int) *ResultCollector {
    return &ResultCollector{
        results: make(chan Result, bufferSize),
    }
}

func (rc *ResultCollector) Start() {
    rc.wg.Add(1)
    go func() {
        defer rc.wg.Done()
        for result := range rc.results {
            rc.mu.Lock()
            rc.collected = append(rc.collected, result)
            rc.mu.Unlock()
        }
    }()
}

func (rc *ResultCollector) Submit(result Result) {
    rc.results <- result
}

func (rc *ResultCollector) GetResults() []Result {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    
    // コピーを返す（安全性のため）
    results := make([]Result, len(rc.collected))
    copy(results, rc.collected)
    return results
}

func (rc *ResultCollector) Close() []Result {
    close(rc.results)
    rc.wg.Wait()
    return rc.GetResults()
}
```

### 動的ワーカー調整

負荷に応じてワーカー数を動的に調整：

```go
type DynamicWorkerPool struct {
    minWorkers   int
    maxWorkers   int
    currentWorkers int
    taskQueue    chan Task
    quit         chan struct{}
    wg           sync.WaitGroup
    mu           sync.RWMutex
}

func (dwp *DynamicWorkerPool) MonitorLoad() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            queueLen := len(dwp.taskQueue)
            dwp.mu.RLock()
            currentWorkers := dwp.currentWorkers
            dwp.mu.RUnlock()
            
            // スケールアップの判定
            if queueLen > currentWorkers*2 && currentWorkers < dwp.maxWorkers {
                dwp.addWorker()
            }
            
            // スケールダウンの判定
            if queueLen < currentWorkers/2 && currentWorkers > dwp.minWorkers {
                dwp.removeWorker()
            }
            
        case <-dwp.quit:
            return
        }
    }
}

func (dwp *DynamicWorkerPool) addWorker() {
    dwp.mu.Lock()
    defer dwp.mu.Unlock()
    
    if dwp.currentWorkers < dwp.maxWorkers {
        dwp.wg.Add(1)
        go dwp.worker(dwp.currentWorkers)
        dwp.currentWorkers++
    }
}
```

### パフォーマンス測定

Worker Poolの効果を測定：

```go
type PoolMetrics struct {
    TasksProcessed int64
    TotalDuration  time.Duration
    ErrorCount     int64
    AvgLatency     time.Duration
    Throughput     float64
}

func BenchmarkWorkerPool(b *testing.B) {
    pool := NewWorkerPool(10, 100)
    pool.Start()
    defer pool.GracefulShutdown(5 * time.Second)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        task := Task{ID: i, Data: i * i}
        pool.SubmitTask(task)
    }
}

// 現実的な負荷テスト
func TestWorkerPoolThroughput(t *testing.T) {
    numTasks := 10000
    pool := NewWorkerPool(20, 200)
    pool.Start()
    
    start := time.Now()
    
    // タスクを投入
    for i := 0; i < numTasks; i++ {
        task := Task{ID: i, Data: heavyComputation}
        pool.SubmitTask(task)
    }
    
    // 完了を待機
    pool.GracefulShutdown(30 * time.Second)
    
    duration := time.Since(start)
    throughput := float64(numTasks) / duration.Seconds()
    
    t.Logf("Processed %d tasks in %v (%.2f tasks/sec)", 
           numTasks, duration, throughput)
}
```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **`NewWorkerPool(numWorkers, queueSize int) *WorkerPool`**: ワーカープールを初期化する
2. **`(wp *WorkerPool) Start()`**: ワーカーGoroutineを開始する
3. **`(wp *WorkerPool) SubmitTask(task Task) error`**: タスクをキューに投入する
4. **`(wp *WorkerPool) GetResult() <-chan Result`**: 結果チャネルを取得する
5. **`(wp *WorkerPool) GracefulShutdown(timeout time.Duration) error`**: 安全に停止する
6. **`NewTaskProcessor(fn ProcessFunc) *TaskProcessor`**: タスク処理関数を作成する
7. **`(tp *TaskProcessor) Process(task Task) Result`**: タスクを処理する

**重要な実装要件：**
- 指定された数のワーカーGoroutineでタスクを並列処理すること
- タスクキューがフルの場合は適切にエラーを返すこと
- グレースフルシャットダウンで進行中のタスクを完了してから停止すること
- レースコンディションが発生しないこと
- 大量のタスク（10,000件以上）を効率的に処理できること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestWorkerPool
=== RUN   TestWorkerPool/Basic_functionality
=== RUN   TestWorkerPool/Concurrent_processing
=== RUN   TestWorkerPool/Queue_overflow
=== RUN   TestWorkerPool/Graceful_shutdown
--- PASS: TestWorkerPool (0.25s)
=== RUN   TestTaskProcessor
=== RUN   TestTaskProcessor/Task_processing
=== RUN   TestTaskProcessor/Error_handling
--- PASS: TestTaskProcessor (0.15s)
PASS
```

### ベンチマーク実行例
```bash
$ go test -bench=.
BenchmarkWorkerPool-8           	   10000	    120000 ns/op
BenchmarkSequential-8           	    2000	    800000 ns/op
BenchmarkLargeTaskSet-8         	     100	  12000000 ns/op
```
Worker Poolが順次処理より6倍以上高速であることが確認できます。

### プログラム実行例
```bash
$ go run main.go
=== Worker Pool Pattern Demo ===

Creating worker pool with 5 workers...
Processing 1000 tasks...

Worker 1 processing task 1 (compute: 15)
Worker 2 processing task 2 (compute: 12)
Worker 3 processing task 3 (compute: 8) 
Worker 4 processing task 4 (compute: 20)
Worker 5 processing task 5 (compute: 11)

Results Summary:
- Total tasks processed: 1000
- Total time: 2.5s
- Throughput: 400 tasks/sec
- Average latency: 12ms
- Success rate: 100%
- Peak workers active: 5

Graceful shutdown completed in 0.3s
All workers stopped cleanly.
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的なワーカー実装
```go
func (wp *WorkerPool) worker(id int) {
    defer wp.wg.Done()
    
    for {
        select {
        case task, ok := <-wp.taskQueue:
            if !ok {
                return // チャネルがクローズされた
            }
            
            result := wp.processTask(task)
            wp.resultChan <- result
            
        case <-wp.quit:
            return
        }
    }
}
```

### タスク投入パターン
```go
func (wp *WorkerPool) SubmitTask(task Task) error {
    select {
    case wp.taskQueue <- task:
        return nil
    default:
        return errors.New("task queue is full")
    }
}
```

### グレースフルシャットダウン
```go
func (wp *WorkerPool) GracefulShutdown(timeout time.Duration) error {
    close(wp.taskQueue) // 新しいタスクを受け付けない
    
    done := make(chan struct{})
    go func() {
        wp.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        close(wp.quit) // 強制終了
        return errors.New("shutdown timeout")
    }
}
```

### 使用する主要なパッケージ
- `sync.WaitGroup` - ワーカーGoroutineの完了待機
- `time.Duration` - タイムアウト処理
- `context.Context` - キャンセレーション処理
- `chan` - タスクキューと結果チャネル

### デバッグのコツ
1. `go test -race`でレースコンディションを検出
2. ワーカー数とキューサイズのバランスを調整
3. ログでワーカーの動作を追跡
4. ベンチマークでスループットを測定

### よくある間違い
- チャネルのクローズタイミング → close()の順序に注意
- WaitGroupの使い方 → Add()とDone()の対応
- デッドロック → select文での適切なケース処理
- ゴルーチンリーク → 必ず停止シグナルを送信

## 実行方法

```bash
# テスト実行
go test -v

# レースコンディション検出
go test -race

# ベンチマーク測定
go test -bench=.

# ロングランニングテスト
go test -timeout=30s

# プログラム実行
go run main.go
```

## 参考資料

- [Go Concurrency Patterns: Worker Pool](https://gobyexample.com/worker-pools)
- [Effective Go: Concurrency](https://golang.org/doc/effective_go#concurrency)
- [Go sync package](https://pkg.go.dev/sync)
- [Worker Pool Pattern](https://golang.org/doc/codewalk/sharemem/)
