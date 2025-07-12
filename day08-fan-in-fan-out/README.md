# Day 08: Fan-in / Fan-outパターン

## 🎯 本日の目標 (Today's Goal)

このチャレンジを通して、以下のスキルを身につけることができます：

- **Fan-outパターンで単一ストリームを複数ワーカーに効率的に分散できるようになる**
- **Fan-inパターンで複数ストリームを一つにマージする仕組みを実装できるようになる**
- **パイプライン処理による段階的なデータ変換システムを構築できるようになる**
- **バックプレッシャー制御による流量制御でシステムの安定性を高められるようになる**

## 📖 解説 (Explanation)

### Fan-in / Fan-outパターンとは？

Fan-in / Fan-outパターンは、並行処理でデータの流れを制御する重要なパターンです：

- **Fan-out**: 1つの入力ストリームを複数の並列処理ワーカーに分散
- **Fan-in**: 複数の処理結果ストリームを1つの出力ストリームにマージ

```go
// シンプルな例
//     Input
//       |
//   ┌───┴───┐     Fan-out (分散)
//   ▼       ▼
// Worker1  Worker2
//   |       |
//   └───┬───┘     Fan-in (集約)
//       ▼
//     Output
```

これにより、**処理能力のスケールアウト**と**効率的なリソース利用**が可能になります。

### なぜFan-in / Fan-outが必要なのか？

データ処理システムでは、以下のような課題があります：

```go
// 問題のある例：順次処理
func processDataSequentially(data []int) []int {
    var results []int
    for _, item := range data {
        // 重い処理が順番に実行される
        result := heavyProcessing(item) // 1秒かかる処理
        results = append(results, result)
    }
    return results // 1000件なら1000秒かかる！
}
```

この方法の問題点：
1. **処理時間の長大化**: CPUコアを1つしか使わない
2. **リソースの非効率利用**: 他のCPUコアが遊んでいる
3. **スケーラビリティの欠如**: 処理量が増えると線形に時間が増加
4. **障害の影響拡大**: 1つの処理が失敗すると全体が停止

### Fan-outパターンの基本実装

単一のストリームを複数のワーカーに分散する仕組み：

```go
import (
    "sync"
    "context"
)

// Fan-outの基本実装
func FanOut[T any](ctx context.Context, input <-chan T, workers int) []<-chan T {
    outputs := make([]<-chan T, workers)
    
    for i := 0; i < workers; i++ {
        ch := make(chan T)
        outputs[i] = ch
        
        // 各ワーカー用のチャネルを作成
        go func(output chan<- T) {
            defer close(output)
            for {
                select {
                case data, ok := <-input:
                    if !ok {
                        return
                    }
                    select {
                    case output <- data:
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
        }(ch)
    }
    
    return outputs
}

// ラウンドロビン方式のFan-out
func FanOutRoundRobin[T any](ctx context.Context, input <-chan T, workers int) []<-chan T {
    outputs := make([]chan T, workers)
    readOnlyOutputs := make([]<-chan T, workers)
    
    for i := 0; i < workers; i++ {
        outputs[i] = make(chan T)
        readOnlyOutputs[i] = outputs[i]
    }
    
    go func() {
        defer func() {
            for _, ch := range outputs {
                close(ch)
            }
        }()
        
        workerIndex := 0
        for {
            select {
            case data, ok := <-input:
                if !ok {
                    return
                }
                
                select {
                case outputs[workerIndex] <- data:
                    workerIndex = (workerIndex + 1) % workers
                case <-ctx.Done():
                    return
                }
            case <-ctx.Done():
                return
            }
        }
    }()
    
    return readOnlyOutputs
}
```

### Fan-inパターンの基本実装

複数のストリームを1つにマージする仕組み：

```go
// 基本的なFan-in実装
func FanIn[T any](ctx context.Context, inputs ...<-chan T) <-chan T {
    output := make(chan T)
    var wg sync.WaitGroup
    
    // 各入力チャネルから読み取り
    for _, input := range inputs {
        wg.Add(1)
        go func(ch <-chan T) {
            defer wg.Done()
            for {
                select {
                case data, ok := <-ch:
                    if !ok {
                        return
                    }
                    select {
                    case output <- data:
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
        }(input)
    }
    
    // 全ての入力が終了したら出力を閉じる
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}

// 順序保証付きのFan-in
func FanInOrdered[T any](ctx context.Context, inputs ...<-chan T) <-chan T {
    output := make(chan T)
    
    go func() {
        defer close(output)
        
        // 各チャネルから順番に読み取り
        for _, input := range inputs {
            for {
                select {
                case data, ok := <-input:
                    if !ok {
                        goto nextChannel
                    }
                    select {
                    case output <- data:
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
            nextChannel:
        }
    }()
    
    return output
}
```

### パイプライン処理の実装

複数の処理段階を組み合わせた効率的なパイプライン：

```go
type ProcessFunc[T, U any] func(T) U

// パイプライン段階の定義
type PipelineStage[T, U any] struct {
    Name     string
    Process  ProcessFunc[T, U]
    Workers  int
    BufferSize int
}

// パイプライン全体の管理
type Pipeline[T any] struct {
    stages []interface{}
    ctx    context.Context
    cancel context.CancelFunc
}

func NewPipeline[T any]() *Pipeline[T] {
    ctx, cancel := context.WithCancel(context.Background())
    return &Pipeline[T]{
        ctx:    ctx,
        cancel: cancel,
    }
}

// パイプライン段階を追加
func (p *Pipeline[T]) AddStage[U any](stage PipelineStage[T, U]) *Pipeline[U] {
    // 型安全性のため、新しいパイプラインを返す
    newPipeline := &Pipeline[U]{
        stages: append(p.stages, stage),
        ctx:    p.ctx,
        cancel: p.cancel,
    }
    return newPipeline
}

// パイプライン実行
func (p *Pipeline[T]) Run(input <-chan T) <-chan T {
    current := input
    
    for _, stageInterface := range p.stages {
        stage := stageInterface.(PipelineStage[T, T]) // 実際は型キャストが必要
        current = p.runStage(current, stage)
    }
    
    return current
}

func (p *Pipeline[T]) runStage(input <-chan T, stage PipelineStage[T, T]) <-chan T {
    // Fan-out: ワーカーに分散
    workerInputs := FanOutRoundRobin(p.ctx, input, stage.Workers)
    
    // 各ワーカーで処理
    var workerOutputs []<-chan T
    for _, workerInput := range workerInputs {
        workerOutput := make(chan T, stage.BufferSize)
        workerOutputs = append(workerOutputs, workerOutput)
        
        go func(in <-chan T, out chan<- T) {
            defer close(out)
            for data := range in {
                select {
                case out <- stage.Process(data):
                case <-p.ctx.Done():
                    return
                }
            }
        }(workerInput, workerOutput)
    }
    
    // Fan-in: 結果をマージ
    return FanIn(p.ctx, workerOutputs...)
}
```

### 高度なFan-out戦略

負荷に応じた動的な分散制御：

```go
// 負荷バランシング付きFan-out
type LoadBalancedFanOut[T any] struct {
    workers     []chan T
    loads       []int64  // 各ワーカーの負荷
    mu          sync.RWMutex
    selector    LoadBalanceStrategy
}

type LoadBalanceStrategy int

const (
    RoundRobin LoadBalanceStrategy = iota
    LeastLoaded
    Random
    Hash
)

func NewLoadBalancedFanOut[T any](workers int, strategy LoadBalanceStrategy) *LoadBalancedFanOut[T] {
    lb := &LoadBalancedFanOut[T]{
        workers:  make([]chan T, workers),
        loads:    make([]int64, workers),
        selector: strategy,
    }
    
    for i := 0; i < workers; i++ {
        lb.workers[i] = make(chan T)
    }
    
    return lb
}

func (lb *LoadBalancedFanOut[T]) SelectWorker(data T) int {
    switch lb.selector {
    case RoundRobin:
        return lb.roundRobinSelect()
    case LeastLoaded:
        return lb.leastLoadedSelect()
    case Random:
        return lb.randomSelect()
    case Hash:
        return lb.hashSelect(data)
    default:
        return 0
    }
}

func (lb *LoadBalancedFanOut[T]) leastLoadedSelect() int {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    
    minLoad := lb.loads[0]
    minIndex := 0
    
    for i, load := range lb.loads {
        if load < minLoad {
            minLoad = load
            minIndex = i
        }
    }
    
    return minIndex
}

func (lb *LoadBalancedFanOut[T]) IncrementLoad(workerIndex int) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    lb.loads[workerIndex]++
}

func (lb *LoadBalancedFanOut[T]) DecrementLoad(workerIndex int) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    lb.loads[workerIndex]--
}
```

### バックプレッシャー制御

処理能力に応じた流量制御の実装：

```go
// バックプレッシャー対応パイプライン
type BackpressurePipeline[T any] struct {
    maxQueueSize int
    dropPolicy   DropPolicy
    metrics      *PipelineMetrics
}

type DropPolicy int

const (
    DropOldest DropPolicy = iota  // 古いデータを破棄
    DropNewest                    // 新しいデータを破棄
    Block                         // ブロック（デフォルト）
)

type PipelineMetrics struct {
    ProcessedCount int64
    DroppedCount   int64
    QueueLength    int64
    mu             sync.RWMutex
}

func (bp *BackpressurePipeline[T]) ProcessWithBackpressure(input <-chan T, process func(T) T) <-chan T {
    output := make(chan T)
    queue := make(chan T, bp.maxQueueSize)
    
    // バックプレッシャー制御付きの入力処理
    go func() {
        defer close(queue)
        for data := range input {
            select {
            case queue <- data:
                bp.metrics.IncrementProcessed()
            default:
                // キューが満杯の場合の処理
                switch bp.dropPolicy {
                case DropOldest:
                    select {
                    case <-queue:  // 古いデータを破棄
                    default:
                    }
                    queue <- data
                case DropNewest:
                    bp.metrics.IncrementDropped()
                    continue  // 新しいデータを破棄
                case Block:
                    queue <- data  // ブロック（デフォルト動作）
                }
            }
        }
    }()
    
    // 実際の処理
    go func() {
        defer close(output)
        for data := range queue {
            result := process(data)
            output <- result
        }
    }()
    
    return output
}

func (pm *PipelineMetrics) IncrementProcessed() {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    pm.ProcessedCount++
}

func (pm *PipelineMetrics) IncrementDropped() {
    pm.mu.Lock()
    defer pm.mu.Unlock()
    pm.DroppedCount++
}

func (pm *PipelineMetrics) GetStats() (int64, int64, float64) {
    pm.mu.RLock()
    defer pm.mu.RUnlock()
    
    total := pm.ProcessedCount + pm.DroppedCount
    dropRate := 0.0
    if total > 0 {
        dropRate = float64(pm.DroppedCount) / float64(total)
    }
    
    return pm.ProcessedCount, pm.DroppedCount, dropRate
}
```

### 実用的なパイプライン例

画像処理パイプラインの実装：

```go
type ImageData struct {
    ID    int
    Data  []byte
    Format string
}

type ProcessedImage struct {
    ID        int
    Data      []byte
    Format    string
    Processed time.Time
}

// 画像処理パイプライン
func CreateImageProcessingPipeline() *Pipeline[ImageData] {
    pipeline := NewPipeline[ImageData]()
    
    // Stage 1: 画像検証
    validationStage := PipelineStage[ImageData, ImageData]{
        Name:    "validation",
        Process: validateImage,
        Workers: 2,
        BufferSize: 10,
    }
    
    // Stage 2: リサイズ
    resizeStage := PipelineStage[ImageData, ImageData]{
        Name:    "resize",
        Process: resizeImage,
        Workers: 4,  // CPU集約的なので多めに
        BufferSize: 5,
    }
    
    // Stage 3: 圧縮
    compressionStage := PipelineStage[ImageData, ProcessedImage]{
        Name:    "compression",
        Process: compressImage,
        Workers: 2,
        BufferSize: 10,
    }
    
    return pipeline.
        AddStage(validationStage).
        AddStage(resizeStage).
        AddStage(compressionStage)
}

func validateImage(img ImageData) ImageData {
    // 画像形式の検証、破損チェックなど
    time.Sleep(10 * time.Millisecond) // 模擬処理時間
    return img
}

func resizeImage(img ImageData) ImageData {
    // 画像のリサイズ処理
    time.Sleep(50 * time.Millisecond) // 重い処理
    return img
}

func compressImage(img ImageData) ProcessedImage {
    // 画像の圧縮処理
    time.Sleep(30 * time.Millisecond)
    return ProcessedImage{
        ID:        img.ID,
        Data:      img.Data,
        Format:    img.Format,
        Processed: time.Now(),
    }
}
```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **`FanOut[T any](input <-chan T, workers int) []<-chan T`**: 単一ストリームを複数ワーカーに分散する
2. **`FanIn[T any](inputs ...<-chan T) <-chan T`**: 複数ストリームを1つにマージする
3. **`NewPipeline[T any]() *Pipeline[T]`**: パイプラインを初期化する
4. **`(p *Pipeline[T]) AddStage[U any](stage PipelineStage[T, U]) *Pipeline[U]`**: 処理段階を追加する
5. **`(p *Pipeline[T]) Run(input <-chan T) <-chan T`**: パイプラインを実行する
6. **`CreateBalancedFanOut[T any](workers int, strategy LoadBalanceStrategy) *LoadBalancedFanOut[T]`**: 負荷分散Fan-outを作成する
7. **`ProcessWithBackpressure[T any](input <-chan T, maxQueue int, policy DropPolicy) <-chan T`**: バックプレッシャー制御付き処理を行う

**重要な実装要件：**
- Fan-outで複数ワーカーにデータを効率的に分散すること
- Fan-inで複数ストリームを正しくマージすること
- パイプライン処理で段階的な変換が動作すること
- レースコンディションが発生しないこと
- バックプレッシャー制御でシステムの安定性を保つこと
- 大量のデータ（10,000件以上）を効率的に処理できること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestFanOut
=== RUN   TestFanOut/Round_robin_distribution
=== RUN   TestFanOut/Load_balancing
--- PASS: TestFanOut (0.15s)
=== RUN   TestFanIn
=== RUN   TestFanIn/Multiple_streams_merge
=== RUN   TestFanIn/Ordered_merge
--- PASS: TestFanIn (0.12s)
=== RUN   TestPipeline
=== RUN   TestPipeline/Multi_stage_processing
=== RUN   TestPipeline/Backpressure_control
--- PASS: TestPipeline (0.25s)
PASS
```

### ベンチマーク実行例
```bash
$ go test -bench=.
BenchmarkSequentialProcessing-8    	    1000	   1500000 ns/op
BenchmarkFanOutProcessing-8         	    5000	    300000 ns/op
BenchmarkPipelineProcessing-8       	    8000	    180000 ns/op
```
Fan-outとパイプライン処理により5-8倍の性能向上が確認できます。

### プログラム実行例
```bash
$ go run main.go
=== Fan-in / Fan-out Pipeline Demo ===

Processing 1000 data items through 3-stage pipeline...

Stage 1 (Validation): 2 workers
Stage 2 (Transform): 4 workers  
Stage 3 (Aggregation): 2 workers

Processing Results:
- Stage 1 completed: 1000/1000 items (2.1s)
- Stage 2 completed: 1000/1000 items (1.8s)
- Stage 3 completed: 1000/1000 items (1.5s)

Total pipeline time: 2.3s
Sequential processing would take: 8.5s
Speedup: 3.7x

Load Balancing Stats:
- Worker 0: 251 items (25.1%)
- Worker 1: 248 items (24.8%)
- Worker 2: 252 items (25.2%)
- Worker 3: 249 items (24.9%)

Backpressure Stats:
- Items processed: 1000
- Items dropped: 5 (0.5%)
- Peak queue length: 87

Pipeline efficiency: 96.8%
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的なFan-out実装
```go
func FanOut[T any](input <-chan T, workers int) []<-chan T {
    outputs := make([]<-chan T, workers)
    
    for i := 0; i < workers; i++ {
        ch := make(chan T)
        outputs[i] = ch
        
        go func(output chan<- T, index int) {
            defer close(output)
            for data := range input {
                if hash(data) % workers == index {
                    output <- data
                }
            }
        }(ch, i)
    }
    
    return outputs
}
```

### 基本的なFan-in実装
```go
func FanIn[T any](inputs ...<-chan T) <-chan T {
    output := make(chan T)
    var wg sync.WaitGroup
    
    for _, input := range inputs {
        wg.Add(1)
        go func(ch <-chan T) {
            defer wg.Done()
            for data := range ch {
                output <- data
            }
        }(input)
    }
    
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}
```

### パイプライン段階の接続
```go
func (p *Pipeline[T]) RunStage(input <-chan T, stage PipelineStage[T, T]) <-chan T {
    // Fan-out
    workerInputs := FanOut(input, stage.Workers)
    
    // 処理
    var workerOutputs []<-chan T
    for _, workerInput := range workerInputs {
        workerOutput := processData(workerInput, stage.Process)
        workerOutputs = append(workerOutputs, workerOutput)
    }
    
    // Fan-in
    return FanIn(workerOutputs...)
}
```

### 使用する主要なパッケージ
- `sync.WaitGroup` - 複数Goroutineの完了待機
- `context` - キャンセレーション制御
- `time` - タイムアウト処理
- `sync.RWMutex` - 負荷統計の排他制御

### デバッグのコツ
1. `go test -race`でレースコンディションを検出
2. チャネルのクローズタイミングを確認
3. ワーカー数とバッファサイズのバランス調整
4. デッドロックを避けるためのselect文使用

### よくある間違い
- チャネルのクローズ忘れ → Goroutineリーク
- WaitGroupの使い方 → Add/Doneの不一致
- バッファサイズ不足 → デッドロック
- 負荷分散の偏り → パフォーマンス低下

## 実行方法

```bash
# テスト実行
go test -v

# レースコンディション検出
go test -race

# ベンチマーク測定
go test -bench=.

# CPUプロファイル
go test -bench=. -cpuprofile=cpu.prof

# プログラム実行
go run main.go
```

## 参考資料

- [Go Concurrency Patterns: Pipelines](https://golang.org/doc/codewalk/sharemem/)
- [Fan-in Fan-out Pattern](https://blog.golang.org/pipelines)
- [Go Channels Best Practices](https://golang.org/doc/effective_go#channels)
- [Concurrency in Go](https://www.oreilly.com/library/view/concurrency-in-go/9781491941195/)