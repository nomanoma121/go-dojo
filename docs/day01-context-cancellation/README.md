# Day 01: Contextによるキャンセル伝播

## 🎯 本日の目標 (Today's Goal)

Goの並行プログラミングにおいて最も重要な概念の一つである`context.Context`を使ったキャンセル伝播を完全に理解し、実装する。Goroutineのツリー構造全体に対してグレースフルなシャットダウンを実現し、リソースリークを防ぐ安全で効率的な並行処理システムを構築する。

## 📖 解説 (Explanation)

### なぜContextによるキャンセル伝播が必要なのか

現代のGoアプリケーションでは、多数のGoroutineが協調して動作します。しかし、適切なキャンセル機能がないと以下の問題が発生します：

#### 1. Goroutineリークの問題

```go
// 【問題のある例】：キャンセル機能なし - この実装は絶対に避けるべき
func badExample() {
    for i := 0; i < 1000; i++ {
        go func(id int) {
            // 【危険】無限ループするGoroutine
            for {
                // 【問題点1】停止条件が存在しない
                // この処理は永続的に実行され続け、プログラム終了後も残存する
                time.Sleep(time.Second)
                fmt.Printf("Worker %d is working...\n", id)
                
                // 【問題点2】外部からの制御が不可能
                // このGoroutineを停止する手段が一切提供されていない
                // シグナル処理、タイムアウト、手動停止のすべてが不可能
            }
        }(i) // 【重要】ループ変数をキャプチャしてGoroutineに渡す
    }
    
    // 【致命的問題】main関数が終了してもGoroutineは残り続ける
    // - 1000個のGoroutineが永続的にシステムリソースを消費
    // - プロセス終了時にGoroutineは強制終了されるが、グレースフルではない
    // - 実際のサーバーアプリケーションでは重大なメモリリークの原因となる
}
```

この例では、1000個のGoroutineが作成され、それらを停止する方法がありません。これにより：
- **メモリリーク**: Goroutineスタックが蓄積
- **CPU使用率増加**: 不要な処理が継続
- **システム不安定**: リソース枯渇によるクラッシュ

#### 2. 級連停止の必要性

実際のアプリケーションでは、Goroutineが階層構造を持ちます：

```
Main Goroutine
├── HTTP Server Goroutine
│   ├── Request Handler 1
│   ├── Request Handler 2
│   └── Request Handler 3
├── Background Worker Pool
│   ├── Worker 1
│   ├── Worker 2
│   └── Worker 3
└── Database Connection Monitor
```

上位のコンポーネントが停止する時、配下のすべてのGoroutineも連鎖的に停止する必要があります。

### Contextパッケージの基本概念

#### 1. Context.Contextインターフェース

```go
type Context interface {
    // Done returns a channel that's closed when work done on behalf of this
    // context should be canceled.
    Done() <-chan struct{}
    
    // Err returns a non-nil error value after Done is closed.
    Err() error
    
    // Deadline returns the time when work done on behalf of this context
    // should be canceled.
    Deadline() (deadline time.Time, ok bool)
    
    // Value returns the value associated with this context for key.
    Value(key interface{}) interface{}
}
```

#### 2. キャンセル可能なContextの作成

```go
import (
    "context"
    "fmt"
    "sync"
    "time"
)

// 【正しい実装】キャンセル可能なContextを作成
// この関数はContextパッケージの基本的な使用パターンを示しています
func createCancellableContext() {
    // 【Step 1】親Context（通常はcontext.Background()）
    // context.Background()は以下の特徴を持つルートContext：
    // - キャンセルされない（Done()チャネルはnil）
    // - デッドラインを持たない
    // - 値を保持しない
    // - すべてのContextツリーのルートとして使用される
    parentCtx := context.Background()
    
    // 【Step 2】キャンセル機能付きContextを作成
    // context.WithCancel()は以下を返す：
    // - ctx: 新しいキャンセル可能なContext
    // - cancel: キャンセル実行関数（context.CancelFunc型）
    ctx, cancel := context.WithCancel(parentCtx)
    
    // 【重要】cancel()の役割：
    // 1. ctx.Done()チャネルをクローズする
    // 2. ctx.Err()がcontext.Canceledを返すようにする
    // 3. このContextから派生したすべての子Contextもキャンセルする
    
    // 【Step 3】5秒後にキャンセルするGoroutineを起動
    go func() {
        time.Sleep(5 * time.Second)
        fmt.Println("Cancelling context...")
        
        // 【キャンセル実行】これによりctx.Done()チャネルがクローズされる
        // このタイミングで、このContextを監視しているすべてのGoroutineに
        // キャンセルシグナルが伝播される
        cancel()
    }()
    
    // 【Step 4】Contextのキャンセルを待機
    // <-ctx.Done() はキャンセルされるまでブロックする
    // Done()チャネルがクローズされると、この行の実行が継続される
    <-ctx.Done()
    
    // 【Step 5】キャンセル理由の確認
    // ctx.Err()はキャンセル理由を返す：
    // - context.Canceled: cancel()関数が呼ばれた場合
    // - context.DeadlineExceeded: タイムアウトが発生した場合
    // - nil: まだキャンセルされていない場合
    fmt.Printf("Context cancelled: %v\n", ctx.Err())
}
```

### 実践的なキャンセル伝播パターン

#### 1. Worker Pool with Cancellation

```go
// 【実践的なパターン】Worker Pool with Cancellation
// 本格的なプロダクションシステムで使用される設計パターン

// WorkerPool は複数のワーカーGoroutineを管理する構造体
type WorkerPool struct {
    workerCount int                    // 【設定】ワーカー数
    workQueue   chan Work             // 【キュー】作業待ちのタスク
    ctx         context.Context       // 【制御】キャンセル用Context
    cancel      context.CancelFunc    // 【制御】キャンセル実行関数
    wg          sync.WaitGroup        // 【同期】ワーカー終了待機用
}

// Work はワーカーが処理するタスクを表現
type Work struct {
    ID   int    // タスクの一意識別子
    Data string // 処理対象のデータ
}

// 【コンストラクタ】新しいWorkerPoolを作成
func NewWorkerPool(workerCount int) *WorkerPool {
    // 【Context作成】キャンセル可能なContextを生成
    ctx, cancel := context.WithCancel(context.Background())
    
    return &WorkerPool{
        workerCount: workerCount,
        // 【バッファ付きチャネル】100件まで作業をキューイング可能
        // バッファサイズにより、プロデューサー側のブロッキングを軽減
        workQueue:   make(chan Work, 100),
        ctx:         ctx,
        cancel:      cancel,
    }
}

// 【起動】指定数のワーカーGoroutineを開始
func (wp *WorkerPool) Start() {
    for i := 0; i < wp.workerCount; i++ {
        wp.wg.Add(1)  // WaitGroupカウンターを増加
        go wp.worker(i)  // 各ワーカーを独立したGoroutineで起動
    }
}

// 【ワーカー実装】各Goroutineで実行されるメインロジック
func (wp *WorkerPool) worker(id int) {
    // 【重要】defer wp.wg.Done()でWaitGroupカウンターを確実に減算
    // パニック発生時でも必ず実行される
    defer wp.wg.Done()
    
    // 【メインループ】キャンセルまたは作業受信を監視
    for {
        select {
        // 【作業処理】キューから作業を受信
        case work := <-wp.workQueue:
            // 【実際の作業実行】
            fmt.Printf("Worker %d processing work %d: %s\n", id, work.ID, work.Data)
            
            // 【作業時間シミュレーション】100ms の処理時間
            // 実際のアプリケーションでは、ここにビジネスロジックを実装
            time.Sleep(time.Millisecond * 100)
            
        // 【キャンセル処理】Context.Done()チャネル監視
        case <-wp.ctx.Done():
            // キャンセルシグナル受信時の処理
            fmt.Printf("Worker %d shutting down: %v\n", id, wp.ctx.Err())
            return  // Goroutineを終了（defer文が実行される）
        }
    }
}

// 【作業追加】新しい作業をキューに追加（スレッドセーフ）
func (wp *WorkerPool) AddWork(work Work) {
    select {
    // 【正常ケース】キューに空きがある場合
    case wp.workQueue <- work:
        // 作業をキューに追加成功
        
    // 【シャットダウン中】すでにキャンセルされている場合
    case <-wp.ctx.Done():
        // 新しい作業の受け付けを拒否
        fmt.Println("Cannot add work: pool is shutting down")
        
    // 【注意】この実装では、キューが満杯の場合のタイムアウト処理は省略
    // プロダクション環境では、time.Afterを使ったタイムアウト処理も検討
    }
}

// 【シャットダウン】グレースフルな停止処理
func (wp *WorkerPool) Shutdown() {
    // 【Step 1】キャンセルシグナルを全ワーカーに送信
    wp.cancel()
    
    // 【Step 2】すべてのワーカーの終了を待機
    // この呼び出しにより、実行中の作業が完了するまで待機
    wp.wg.Wait()
    
    // 【Step 3】リソースのクリーンアップ
    close(wp.workQueue)  // チャネルクローズでリソース解放
    fmt.Println("Worker pool shutdown complete")
    
    // 【設計のポイント】
    // 1. 新しい作業の受け付けを即座に停止
    // 2. 実行中の作業は完了まで待機（データ損失防止）
    // 3. すべてのリソースを確実に解放
}
```

#### 2. 階層的なキャンセル伝播

```go
type ServiceManager struct {
    httpServer    *HTTPServer
    workerPool    *WorkerPool
    dbMonitor     *DatabaseMonitor
    ctx           context.Context
    cancel        context.CancelFunc
}

func NewServiceManager() *ServiceManager {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &ServiceManager{
        ctx:    ctx,
        cancel: cancel,
    }
}

func (sm *ServiceManager) Start() error {
    // 各コンポーネントに子Contextを渡す
    httpCtx, _ := context.WithCancel(sm.ctx)
    workerCtx, _ := context.WithCancel(sm.ctx)
    dbCtx, _ := context.WithCancel(sm.ctx)
    
    // コンポーネントを起動
    sm.httpServer = NewHTTPServer(httpCtx)
    sm.workerPool = NewWorkerPoolWithContext(workerCtx)
    sm.dbMonitor = NewDatabaseMonitor(dbCtx)
    
    go sm.httpServer.Start()
    go sm.workerPool.Start()
    go sm.dbMonitor.Start()
    
    return nil
}

func (sm *ServiceManager) Shutdown() {
    // トップレベルのキャンセルを実行
    // これにより、すべての子Contextも自動的にキャンセルされる
    sm.cancel()
    
    // 各コンポーネントの終了を待機
    sm.httpServer.Wait()
    sm.workerPool.Wait()
    sm.dbMonitor.Wait()
    
    fmt.Println("All services shutdown complete")
}
```

### 高度なキャンセルパターン

#### 1. タイムアウト付きキャンセル

```go
func processWithTimeout(work func(context.Context) error) error {
    // 10秒でタイムアウト
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // 結果チャネル
    resultChan := make(chan error, 1)
    
    go func() {
        resultChan <- work(ctx)
    }()
    
    select {
    case err := <-resultChan:
        return err
    case <-ctx.Done():
        return fmt.Errorf("operation timed out: %w", ctx.Err())
    }
}
```

#### 2. マルチプル待機パターン

```go
func waitForMultipleOperations(ctx context.Context) error {
    var wg sync.WaitGroup
    errChan := make(chan error, 3)
    
    operations := []func(context.Context) error{
        operationA,
        operationB,
        operationC,
    }
    
    for _, op := range operations {
        wg.Add(1)
        go func(operation func(context.Context) error) {
            defer wg.Done()
            if err := operation(ctx); err != nil {
                errChan <- err
            }
        }(op)
    }
    
    // 完了通知用チャネル
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()
    
    select {
    case err := <-errChan:
        // いずれかの操作でエラー発生
        return err
    case <-done:
        // すべての操作が正常完了
        return nil
    case <-ctx.Done():
        // 外部からのキャンセル
        return ctx.Err()
    }
}
```

### パフォーマンスとメモリ考慮事項

#### 1. Context作成のオーバーヘッド

```go
// 効率的：単一のContextを再利用
func efficientPattern(parentCtx context.Context, tasks []Task) {
    for _, task := range tasks {
        processTask(parentCtx, task) // 同じContextを再利用
    }
}

// 非効率：各タスクで新しいContextを作成
func inefficientPattern(parentCtx context.Context, tasks []Task) {
    for _, task := range tasks {
        ctx, cancel := context.WithCancel(parentCtx)
        processTask(ctx, task)
        cancel() // 毎回新しいContextを作成・破棄
    }
}
```

#### 2. Done()チャネルの効率的な監視

```go
func efficientCancellationCheck(ctx context.Context) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // 定期的なタスク実行
            doWork()
        case <-ctx.Done():
            // キャンセル時の即座な停止
            return
        }
    }
}
```

## 📝 課題 (The Problem)

以下の要件を満たすキャンセル伝播システムを実装してください：

### 1. 基本機能
- 複数のワーカーGoroutineの作成と管理
- 親Contextからのキャンセル伝播
- すべてのGoroutineのグレースフル停止
- リソースリークの防止

### 2. 高度な機能
- タイムアウト付きキャンセル
- 部分的なキャンセル（特定のワーカーのみ停止）
- キャンセル理由の追跡
- 統計情報の収集

### 3. エラーハンドリング
- キャンセル時の適切なエラー報告
- リソースクリーンアップの保証
- デッドロック防止

### 実装すべき関数

```go
// ProcessWithCancellation は複数のワーカーGoroutineを起動し、
// 指定時間後にキャンセルシグナルを送信して全ワーカーを停止させる
func ProcessWithCancellation(numWorkers int, workDuration time.Duration, cancelAfter time.Duration) error

// Worker は与えられたcontextをチェックして作業を行う
// キャンセルシグナルを受け取ったら即座に停止する
func Worker(ctx context.Context, id int, results chan<- WorkResult) error

// WorkerPool は複数のワーカーを管理し、効率的なタスク分散を行う
type WorkerPool struct {
    // 実装詳細
}

// ManagerService は複数のサービスを管理し、階層的キャンセルを実現
type ManagerService struct {
    // 実装詳細
}
```

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestBasicCancellation
    main_test.go:45: All workers cancelled successfully
--- PASS: TestBasicCancellation (0.01s)

=== RUN   TestTimeoutCancellation
    main_test.go:65: Timeout cancellation working correctly
--- PASS: TestTimeoutCancellation (0.02s)

=== RUN   TestHierarchicalCancellation
    main_test.go:85: Hierarchical cancellation propagated
--- PASS: TestHierarchicalCancellation (0.03s)

=== RUN   TestNoGoroutineLeaks
    main_test.go:105: No goroutine leaks detected
--- PASS: TestNoGoroutineLeaks (0.04s)

PASS
ok      day01-context-cancellation   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 基本的なキャンセル実装

```go
func basicWorker(ctx context.Context, id int) {
    for {
        select {
        case <-ctx.Done():
            fmt.Printf("Worker %d cancelled: %v\n", id, ctx.Err())
            return
        default:
            // 通常の作業
            doWork()
        }
    }
}
```

### WaitGroupを使った同期

```go
func manageWorkers(ctx context.Context, numWorkers int) {
    var wg sync.WaitGroup
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            basicWorker(ctx, id)
        }(i)
    }
    
    wg.Wait() // すべてのワーカーの完了を待機
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **動的ワーカー管理**: 実行時のワーカー数調整
2. **優先度付きキャンセル**: 重要度に応じた停止順序制御
3. **分散キャンセル**: 複数プロセス間でのキャンセル伝播
4. **メトリクス収集**: キャンセル統計の記録と分析
5. **リトライ機能**: キャンセル後の自動再起動

Contextによるキャンセル伝播の実装を通じて、Goの並行プログラミングの基礎となる重要な概念を完全に習得しましょう！