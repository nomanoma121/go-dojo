# Day 09: エラーハンドリング付きパイプライン

## 🎯 本日の目標 (Today's Goal)

このチャレンジを通して、以下のスキルを身につけることができます：

- **パイプライン処理でのエラーを適切に伝播・管理できるようになる**
- **部分的な失敗に対する堅牢な処理システムを構築できるようになる**
- **一時的なエラーからの自動回復機能を実装できるようになる**
- **エラー情報を集約して運用に有用な情報を提供できるようになる**

## 📖 解説 (Explanation)

### なぜエラーハンドリング付きパイプラインが重要なのか？

並行処理パイプラインでは、複数のステージが並列に実行され、各ステージで様々なエラーが発生する可能性があります：

```go
// 問題のある例：エラー処理が不十分
func processDataUnsafely(data []DataItem) {
    input := make(chan DataItem)
    
    // データを投入
    go func() {
        defer close(input)
        for _, item := range data {
            input <- item
        }
    }()
    
    // 各ステージで処理
    stage1 := processStage1(input)  // ネットワークエラー発生可能
    stage2 := processStage2(stage1) // DB接続エラー発生可能
    stage3 := processStage3(stage2) // ファイルI/Oエラー発生可能
    
    // エラーが発生してもわからない！
    for result := range stage3 {
        fmt.Println(result)
    }
}
```

この方法の問題点：
1. **エラーの見落とし**: エラーが発生しても検知できない
2. **部分的失敗の処理困難**: 一部のデータが失敗した場合の対応不能
3. **デバッグの困難さ**: どのステージでエラーが発生したかわからない
4. **リソースリーク**: エラー時にGoroutineやチャネルが残る

### エラーハンドリング付きパイプラインの基本設計

堅牢なエラー処理を組み込んだパイプライン：

```go
import (
    "context"
    "sync"
    "time"
    "golang.org/x/sync/errgroup"
)

type DataItem struct {
    ID   int
    Data interface{}
}

type Result struct {
    DataItem
    Error error
    Stage string
}

type PipelineError struct {
    Stage     string
    Error     error
    Data      DataItem
    Timestamp time.Time
    Retryable bool
}

type ErrorPipeline struct {
    stages    []PipelineStage
    errorChan chan PipelineError
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
}

type PipelineStage struct {
    Name    string
    Process func(context.Context, DataItem) (DataItem, error)
    Workers int
    Retry   RetryConfig
}

type RetryConfig struct {
    MaxAttempts int
    BackoffTime time.Duration
    Retryable   func(error) bool
}

func NewErrorPipeline() *ErrorPipeline {
    ctx, cancel := context.WithCancel(context.Background())
    return &ErrorPipeline{
        errorChan: make(chan PipelineError, 100),
        ctx:       ctx,
        cancel:    cancel,
    }
}
```

### エラー伝播とResult型パターン

各ステージでエラーを適切に処理し、後続ステージに伝播：

```go
// Result型でエラーを包含
func (ep *ErrorPipeline) ProcessStage(
    stageName string,
    input <-chan Result,
    processFn func(DataItem) (DataItem, error),
    workers int,
) <-chan Result {
    output := make(chan Result, workers)
    
    var wg sync.WaitGroup
    
    // 複数ワーカーで並列処理
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for {
                select {
                case result, ok := <-input:
                    if !ok {
                        return
                    }
                    
                    // 前のステージでエラーが発生していた場合はそのまま流す
                    if result.Error != nil {
                        select {
                        case output <- result:
                        case <-ep.ctx.Done():
                            return
                        }
                        continue
                    }
                    
                    // 処理を実行
                    processed, err := processFn(result.DataItem)
                    newResult := Result{
                        DataItem: processed,
                        Error:    err,
                        Stage:    stageName,
                    }
                    
                    // エラーが発生した場合はエラーチャネルに送信
                    if err != nil {
                        pipelineErr := PipelineError{
                            Stage:     stageName,
                            Error:     err,
                            Data:      result.DataItem,
                            Timestamp: time.Now(),
                            Retryable: isRetryableError(err),
                        }
                        
                        select {
                        case ep.errorChan <- pipelineErr:
                        case <-ep.ctx.Done():
                            return
                        }
                    }
                    
                    select {
                    case output <- newResult:
                    case <-ep.ctx.Done():
                        return
                    }
                    
                case <-ep.ctx.Done():
                    return
                }
            }
        }(i)
    }
    
    // 全ワーカー完了後にチャネルをクローズ
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}

func isRetryableError(err error) bool {
    // ネットワークエラー、一時的なDBエラーなど
    // 実装は具体的なエラー型に応じて調整
    return strings.Contains(err.Error(), "timeout") ||
           strings.Contains(err.Error(), "connection refused") ||
           strings.Contains(err.Error(), "temporary")
}
```

### リトライ機能付きステージ

一時的なエラーに対する自動回復機能：

```go
func (ep *ErrorPipeline) ProcessStageWithRetry(
    stage PipelineStage,
    input <-chan Result,
) <-chan Result {
    output := make(chan Result, stage.Workers)
    
    var wg sync.WaitGroup
    
    for i := 0; i < stage.Workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            
            for {
                select {
                case result, ok := <-input:
                    if !ok {
                        return
                    }
                    
                    // 前のステージでエラーが発生していた場合
                    if result.Error != nil {
                        select {
                        case output <- result:
                        case <-ep.ctx.Done():
                            return
                        }
                        continue
                    }
                    
                    // リトライ付きで処理を実行
                    processed, err := ep.executeWithRetry(stage, result.DataItem)
                    newResult := Result{
                        DataItem: processed,
                        Error:    err,
                        Stage:    stage.Name,
                    }
                    
                    select {
                    case output <- newResult:
                    case <-ep.ctx.Done():
                        return
                    }
                    
                case <-ep.ctx.Done():
                    return
                }
            }
        }()
    }
    
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}

func (ep *ErrorPipeline) executeWithRetry(stage PipelineStage, data DataItem) (DataItem, error) {
    var lastError error
    
    for attempt := 1; attempt <= stage.Retry.MaxAttempts; attempt++ {
        // コンテキストキャンセルのチェック
        select {
        case <-ep.ctx.Done():
            return data, ep.ctx.Err()
        default:
        }
        
        processed, err := stage.Process(ep.ctx, data)
        if err == nil {
            return processed, nil
        }
        
        lastError = err
        
        // リトライ可能なエラーかチェック
        if stage.Retry.Retryable != nil && !stage.Retry.Retryable(err) {
            break
        }
        
        // 最終試行でなければ待機
        if attempt < stage.Retry.MaxAttempts {
            backoff := time.Duration(attempt) * stage.Retry.BackoffTime
            timer := time.NewTimer(backoff)
            
            select {
            case <-timer.C:
                // 待機完了
            case <-ep.ctx.Done():
                timer.Stop()
                return data, ep.ctx.Err()
            }
        }
    }
    
    return data, lastError
}
```

### エラー集約とレポート

複数のエラーを効率的に収集・分析：

```go
type ErrorCollector struct {
    errors    []PipelineError
    mu        sync.RWMutex
    stats     map[string]*ErrorStats
    maxErrors int
}

type ErrorStats struct {
    Count         int
    FirstSeen     time.Time
    LastSeen      time.Time
    RetryableRate float64
}

func NewErrorCollector(maxErrors int) *ErrorCollector {
    return &ErrorCollector{
        maxErrors: maxErrors,
        stats:     make(map[string]*ErrorStats),
    }
}

func (ec *ErrorCollector) CollectError(pipelineErr PipelineError) {
    ec.mu.Lock()
    defer ec.mu.Unlock()
    
    // エラーを保存（上限チェック）
    if len(ec.errors) < ec.maxErrors {
        ec.errors = append(ec.errors, pipelineErr)
    }
    
    // 統計情報を更新
    stageName := pipelineErr.Stage
    if stats, exists := ec.stats[stageName]; exists {
        stats.Count++
        stats.LastSeen = pipelineErr.Timestamp
        if pipelineErr.Retryable {
            stats.RetryableRate = (stats.RetryableRate*float64(stats.Count-1) + 1) / float64(stats.Count)
        } else {
            stats.RetryableRate = stats.RetryableRate * float64(stats.Count-1) / float64(stats.Count)
        }
    } else {
        retryableRate := 0.0
        if pipelineErr.Retryable {
            retryableRate = 1.0
        }
        ec.stats[stageName] = &ErrorStats{
            Count:         1,
            FirstSeen:     pipelineErr.Timestamp,
            LastSeen:      pipelineErr.Timestamp,
            RetryableRate: retryableRate,
        }
    }
}

func (ec *ErrorCollector) GetErrorSummary() ErrorSummary {
    ec.mu.RLock()
    defer ec.mu.RUnlock()
    
    summary := ErrorSummary{
        TotalErrors: len(ec.errors),
        StageStats:  make(map[string]ErrorStats),
    }
    
    for stage, stats := range ec.stats {
        summary.StageStats[stage] = *stats
    }
    
    // 最も頻繁なエラーを特定
    maxCount := 0
    for stage, stats := range ec.stats {
        if stats.Count > maxCount {
            maxCount = stats.Count
            summary.MostProblematicStage = stage
        }
    }
    
    return summary
}

type ErrorSummary struct {
    TotalErrors           int
    StageStats           map[string]ErrorStats
    MostProblematicStage string
}
```

### サーキットブレーカー付きパイプライン

高エラー率時の自動保護機能：

```go
type CircuitBreaker struct {
    state         State
    failures      int
    successes     int
    lastFailTime  time.Time
    timeout       time.Duration
    maxFailures   int
    mu            sync.RWMutex
}

type State int

const (
    Closed State = iota  // 正常状態
    Open                 // 回路開放（失敗状態）
    HalfOpen            // 半開状態（回復テスト中）
)

func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        state:       Closed,
        maxFailures: maxFailures,
        timeout:     timeout,
    }
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    // 状態チェック
    if cb.state == Open {
        if time.Since(cb.lastFailTime) > cb.timeout {
            cb.state = HalfOpen
            cb.successes = 0
        } else {
            return errors.New("circuit breaker is open")
        }
    }
    
    // 関数実行
    err := fn()
    
    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()
        
        if cb.state == HalfOpen || cb.failures >= cb.maxFailures {
            cb.state = Open
        }
    } else {
        if cb.state == HalfOpen {
            cb.successes++
            if cb.successes >= 3 { // 3回成功で回復
                cb.state = Closed
                cb.failures = 0
            }
        } else {
            cb.failures = 0
        }
    }
    
    return err
}

// サーキットブレーカー付きステージ
func (ep *ErrorPipeline) ProcessStageWithCircuitBreaker(
    stage PipelineStage,
    input <-chan Result,
    cb *CircuitBreaker,
) <-chan Result {
    output := make(chan Result, stage.Workers)
    
    var wg sync.WaitGroup
    
    for i := 0; i < stage.Workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            
            for {
                select {
                case result, ok := <-input:
                    if !ok {
                        return
                    }
                    
                    if result.Error != nil {
                        select {
                        case output <- result:
                        case <-ep.ctx.Done():
                            return
                        }
                        continue
                    }
                    
                    // サーキットブレーカー経由で処理
                    var processed DataItem
                    var err error
                    
                    circuitErr := cb.Call(func() error {
                        processed, err = stage.Process(ep.ctx, result.DataItem)
                        return err
                    })
                    
                    if circuitErr != nil {
                        err = circuitErr
                    }
                    
                    newResult := Result{
                        DataItem: processed,
                        Error:    err,
                        Stage:    stage.Name,
                    }
                    
                    select {
                    case output <- newResult:
                    case <-ep.ctx.Done():
                        return
                    }
                    
                case <-ep.ctx.Done():
                    return
                }
            }
        }()
    }
    
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}
```

### 実用的なパイプライン例

データ処理パイプラインの完全な実装例：

```go
// データ処理パイプラインの実装例
func CreateDataProcessingPipeline() *ErrorPipeline {
    pipeline := NewErrorPipeline()
    
    // Stage 1: データ検証
    validationStage := PipelineStage{
        Name:    "validation",
        Process: validateData,
        Workers: 2,
        Retry: RetryConfig{
            MaxAttempts: 1, // 検証は再試行しない
            BackoffTime: 0,
        },
    }
    
    // Stage 2: 外部API呼び出し
    apiStage := PipelineStage{
        Name:    "api_call",
        Process: callExternalAPI,
        Workers: 4,
        Retry: RetryConfig{
            MaxAttempts: 3,
            BackoffTime: time.Second,
            Retryable:   isNetworkError,
        },
    }
    
    // Stage 3: データベース保存
    dbStage := PipelineStage{
        Name:    "database_save",
        Process: saveToDatabase,
        Workers: 2,
        Retry: RetryConfig{
            MaxAttempts: 2,
            BackoffTime: 500 * time.Millisecond,
            Retryable:   isDatabaseError,
        },
    }
    
    pipeline.stages = []PipelineStage{validationStage, apiStage, dbStage}
    return pipeline
}

func validateData(ctx context.Context, data DataItem) (DataItem, error) {
    // データの妥当性をチェック
    if data.Data == nil {
        return data, errors.New("data is nil")
    }
    return data, nil
}

func callExternalAPI(ctx context.Context, data DataItem) (DataItem, error) {
    // 外部API呼び出しをシミュレート
    select {
    case <-time.After(100 * time.Millisecond):
        // 10%の確率で失敗
        if rand.Float64() < 0.1 {
            return data, errors.New("api timeout")
        }
        return data, nil
    case <-ctx.Done():
        return data, ctx.Err()
    }
}

func saveToDatabase(ctx context.Context, data DataItem) (DataItem, error) {
    // データベース保存をシミュレート
    select {
    case <-time.After(50 * time.Millisecond):
        // 5%の確率で失敗
        if rand.Float64() < 0.05 {
            return data, errors.New("database connection error")
        }
        return data, nil
    case <-ctx.Done():
        return data, ctx.Err()
    }
}

func isNetworkError(err error) bool {
    return strings.Contains(err.Error(), "timeout") ||
           strings.Contains(err.Error(), "connection")
}

func isDatabaseError(err error) bool {
    return strings.Contains(err.Error(), "database")
}
```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **`NewErrorPipeline() *ErrorPipeline`**: エラーハンドリング付きパイプラインを初期化する
2. **`(ep *ErrorPipeline) ProcessStage(name string, input <-chan Result, fn ProcessFunc, workers int) <-chan Result`**: エラー処理付きステージを実行する
3. **`(ep *ErrorPipeline) ProcessStageWithRetry(stage PipelineStage, input <-chan Result) <-chan Result`**: リトライ機能付きステージを実行する
4. **`NewErrorCollector(maxErrors int) *ErrorCollector`**: エラー収集器を作成する
5. **`(ec *ErrorCollector) CollectError(err PipelineError)`**: エラーを収集する
6. **`(ec *ErrorCollector) GetErrorSummary() ErrorSummary`**: エラー統計を取得する
7. **`NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker`**: サーキットブレーカーを作成する

**重要な実装要件：**
- エラーが発生しても他のデータの処理を継続すること
- エラー情報を詳細に記録し、運用に役立つ統計を提供すること
- リトライ機能で一時的なエラーから自動回復すること
- サーキットブレーカーで高エラー率時にシステムを保護すること
- レースコンディションが発生しないこと
- 大量のデータ（10,000件以上）を効率的に処理できること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestErrorPipeline
=== RUN   TestErrorPipeline/Error_propagation
=== RUN   TestErrorPipeline/Partial_failure_handling
=== RUN   TestErrorPipeline/Retry_functionality
--- PASS: TestErrorPipeline (0.30s)
=== RUN   TestCircuitBreaker
=== RUN   TestCircuitBreaker/State_transitions
=== RUN   TestCircuitBreaker/Auto_recovery
--- PASS: TestCircuitBreaker (0.15s)
=== RUN   TestErrorCollector
=== RUN   TestErrorCollector/Error_statistics
--- PASS: TestErrorCollector (0.08s)
PASS
```

### プログラム実行例
```bash
$ go run main.go
=== Error Handling Pipeline Demo ===

Processing 1000 data items through error-resilient pipeline...

Stage 1 (Validation): 2 workers
Stage 2 (API Call): 4 workers with retry
Stage 3 (Database): 2 workers with circuit breaker

Processing Results:
- Total items processed: 1000
- Successful items: 945 (94.5%)
- Failed items: 55 (5.5%)

Error Statistics:
Stage "validation":
- Errors: 12 (1.2%)
- Retryable rate: 0%
- First error: 09:15:32
- Last error: 09:15:45

Stage "api_call":
- Errors: 28 (2.8%)
- Retryable rate: 85.7%
- First error: 09:15:33
- Last error: 09:15:47
- Retries performed: 42

Stage "database_save":
- Errors: 15 (1.5%)
- Retryable rate: 100%
- First error: 09:15:34
- Last error: 09:15:46
- Circuit breaker triggered: 2 times

Most problematic stage: api_call

Recovery Statistics:
- Successful retries: 38/42 (90.5%)
- Circuit breaker recoveries: 2/2 (100%)

Pipeline efficiency: 94.5%
Total processing time: 8.2s
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的なエラー処理パターン
```go
func (ep *ErrorPipeline) ProcessStage(name string, input <-chan Result, fn ProcessFunc, workers int) <-chan Result {
    output := make(chan Result, workers)
    
    for i := 0; i < workers; i++ {
        go func() {
            for result := range input {
                if result.Error != nil {
                    output <- result
                    continue
                }
                
                processed, err := fn(result.DataItem)
                output <- Result{
                    DataItem: processed,
                    Error:    err,
                    Stage:    name,
                }
            }
        }()
    }
    
    return output
}
```

### リトライロジック
```go
func executeWithRetry(fn func() error, maxAttempts int, backoff time.Duration) error {
    var lastErr error
    for i := 0; i < maxAttempts; i++ {
        if err := fn(); err == nil {
            return nil
        } else {
            lastErr = err
            if i < maxAttempts-1 {
                time.Sleep(time.Duration(i+1) * backoff)
            }
        }
    }
    return lastErr
}
```

### エラー統計計算
```go
func (ec *ErrorCollector) GetErrorSummary() ErrorSummary {
    ec.mu.RLock()
    defer ec.mu.RUnlock()
    
    summary := ErrorSummary{
        TotalErrors: len(ec.errors),
        StageStats:  make(map[string]ErrorStats),
    }
    
    for stage, stats := range ec.stats {
        summary.StageStats[stage] = *stats
    }
    
    return summary
}
```

### 使用する主要なパッケージ
- `golang.org/x/sync/errgroup` - エラー付きGoroutine管理
- `context` - キャンセレーション制御
- `sync` - 排他制御
- `time` - リトライ間隔制御

### デバッグのコツ
1. `go test -race`でレースコンディションを検出
2. エラー統計の計算ロジックを丁寧にテスト
3. リトライの指数バックオフが正しく動作するか確認
4. サーキットブレーカーの状態遷移をログで追跡

### よくある間違い
- エラー時のリソースリーク → deferでクリーンアップ
- リトライの無限ループ → 最大試行回数を設定
- 統計の競合状態 → 適切な排他制御
- コンテキストキャンセルの無視 → select文でチェック

## 実行方法

```bash
# テスト実行
go test -v

# レースコンディション検出
go test -race

# ベンチマーク測定
go test -bench=.

# カバレッジ測定
go test -cover

# プログラム実行
go run main.go
```

## 参考資料

- [Go Error Handling Best Practices](https://go.dev/blog/error-handling-and-go)
- [errgroup Package](https://pkg.go.dev/golang.org/x/sync/errgroup)
- [Circuit Breaker Pattern](https://martinfowler.com/bliki/CircuitBreaker.html)
- [Go Context Package](https://pkg.go.dev/context)