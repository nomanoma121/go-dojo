# Day 15: Generatorパターン

## 🎯 本日の目標 (Today's Goal)

チャネルとジェネリクスを使った遅延評価のGeneratorパターンを実装し、メモリ効率の良いデータ処理とストリーミング処理の概念を理解する。

## 📖 解説 (Explanation)

### Generatorパターンとは

Generatorパターンは、値を逐次生成して提供するデザインパターンです。大量のデータを一度にメモリに読み込むのではなく、必要に応じて値を生成することで、メモリ効率の良いプログラムを実現できます。

### 従来の処理との比較

```go
// 【Generatorパターンの重要性】メモリ効率と遅延評価による最適化
// ❌ 問題例：一括処理によるメモリ枯渇とシステム障害
func disastrousEagerProcessing() {
    // 🚨 災害例：大量データの一括読み込みでシステム崩壊
    log.Printf("Processing 10 million records eagerly...")
    
    // ❌ 全データを一度にメモリに読み込み
    var allUsers []User
    
    // データベースから1000万件を一度に取得
    rows, err := db.Query("SELECT id, name, email, profile FROM users")
    if err != nil {
        log.Fatal("Query failed:", err)
    }
    defer rows.Close()
    
    for rows.Next() {
        var user User
        rows.Scan(&user.ID, &user.Name, &user.Email, &user.Profile)
        allUsers = append(allUsers, user)
        
        // ❌ メモリ使用量が指数関数的に増大
        // 1000万件 × 平均1KB = 10GB以上のメモリ使用
        // ❌ ガベージコレクションが頻発してCPU使用率100%
        // ❌ スワップ発生でシステム全体が応答不能
    }
    
    log.Printf("❌ Loaded %d users into memory (%.2f GB used)", 
        len(allUsers), float64(len(allUsers)*1024)/1024/1024/1024)
    
    // 【処理開始】すでに手遅れ
    for i, user := range allUsers {
        processUser(user) // CPU集約的な処理
        
        if i%100000 == 0 {
            // ❌ この時点でメモリ不足でプロセス強制終了
            log.Printf("Processed %d users...", i)
            // OOM Killer発動、サービス停止
        }
    }
    // 結果：サーバークラッシュ、サービス停止、顧客影響、インフラコスト増大
}

// ✅ 正解：プロダクション品質のGeneratorパターン実装
// 【遅延評価でメモリ効率を最大化】
type EnterpriseGenerator[T any] struct {
    // 【基本構成】
    ch          <-chan T             // データストリーム
    cancel      context.CancelFunc   // キャンセレーション制御
    ctx         context.Context      // コンテキスト管理
    
    // 【高度な機能】
    buffer      chan T               // バッファリング機能
    bufferSize  int                  // バッファサイズ
    
    // 【監視・メトリクス】
    generated   int64                // 生成済み要素数
    consumed    int64                // 消費済み要素数
    startTime   time.Time            // 生成開始時刻
    
    // 【エラー処理】
    errorChan   chan error           // エラー通知チャネル
    logger      *log.Logger          // ログ出力
    
    // 【パフォーマンス最適化】
    batchSize   int                  // バッチ処理サイズ
    prefetch    bool                 // プリフェッチ有効化
}

// 【重要関数】メモリ効率的な大量データ処理Generator
func NewMemoryEfficientUserGenerator(ctx context.Context, batchSize int) *EnterpriseGenerator[User] {
    generatorCtx, cancel := context.WithCancel(ctx)
    
    gen := &EnterpriseGenerator[User]{
        cancel:     cancel,
        ctx:        generatorCtx,
        bufferSize: 1000,               // 1000件のバッファ
        batchSize:  batchSize,          // バッチサイズ
        startTime:  time.Now(),
        logger:     log.New(os.Stdout, "[GENERATOR] ", log.LstdFlags),
        errorChan:  make(chan error, 10),
    }
    
    // バッファ付きチャネル作成
    gen.buffer = make(chan User, gen.bufferSize)
    gen.ch = gen.buffer
    
    // 【重要】非同期でデータ生成開始
    go gen.generateUsersLazily()
    
    gen.logger.Printf("🚀 Memory-efficient generator started (batch: %d, buffer: %d)", 
        batchSize, gen.bufferSize)
    
    return gen
}

// 【核心メソッド】遅延評価によるメモリ効率的データ生成
func (g *EnterpriseGenerator[User]) generateUsersLazily() {
    defer close(g.buffer)
    defer close(g.errorChan)
    
    offset := 0
    
    for {
        select {
        case <-g.ctx.Done():
            g.logger.Printf("🛑 Generator cancelled after %d items", 
                atomic.LoadInt64(&g.generated))
            return
            
        default:
            // 【バッチクエリ】少量ずつデータベースから取得
            query := `SELECT id, name, email, profile FROM users 
                     ORDER BY id LIMIT ? OFFSET ?`
            
            rows, err := db.QueryContext(g.ctx, query, g.batchSize, offset)
            if err != nil {
                g.errorChan <- fmt.Errorf("batch query failed at offset %d: %w", offset, err)
                return
            }
            
            batchCount := 0
            batchStart := time.Now()
            
            for rows.Next() {
                var user User
                if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Profile); err != nil {
                    rows.Close()
                    g.errorChan <- fmt.Errorf("scan failed: %w", err)
                    return
                }
                
                // 【重要】バッファがフルの場合は消費者を待機
                select {
                case g.buffer <- user:
                    atomic.AddInt64(&g.generated, 1)
                    batchCount++
                    
                case <-g.ctx.Done():
                    rows.Close()
                    return
                }
            }
            rows.Close()
            
            batchDuration := time.Since(batchStart)
            
            if batchCount == 0 {
                // データ終了
                g.logger.Printf("✅ Generation completed: %d total items in %v", 
                    atomic.LoadInt64(&g.generated), time.Since(g.startTime))
                return
            }
            
            g.logger.Printf("📦 Batch loaded: %d items (offset: %d, took: %v, rate: %.0f items/sec)", 
                batchCount, offset, batchDuration, float64(batchCount)/batchDuration.Seconds())
            
            offset += g.batchSize
            
            // 【メモリ効率】少し待機してGCに余裕を与える
            if offset%10000 == 0 {
                runtime.GC()
                time.Sleep(10 * time.Millisecond)
            }
        }
    }
}

// 【重要メソッド】メモリ使用量監視付き要素取得
func (g *EnterpriseGenerator[User]) Next() (User, bool) {
    select {
    case user, ok := <-g.ch:
        if ok {
            atomic.AddInt64(&g.consumed, 1)
            
            // 【定期レポート】
            consumed := atomic.LoadInt64(&g.consumed)
            if consumed%10000 == 0 {
                generated := atomic.LoadInt64(&g.generated)
                var m runtime.MemStats
                runtime.ReadMemStats(&m)
                
                g.logger.Printf("📊 Progress: consumed=%d, generated=%d, memory=%.2fMB", 
                    consumed, generated, float64(m.Alloc)/1024/1024)
            }
        }
        return user, ok
        
    case <-g.ctx.Done():
        var zero User
        return zero, false
    }
}

// 【実用例】メモリ効率的な大量データ処理
func EfficientBigDataProcessing() {
    ctx := context.Background()
    
    // 【初期化】1000件ずつバッチ処理
    generator := NewMemoryEfficientUserGenerator(ctx, 1000)
    defer generator.Cancel()
    
    processedCount := 0
    errorCount := 0
    startTime := time.Now()
    
    log.Printf("🚀 Starting memory-efficient processing of millions of users")
    
    // 【エラー監視】
    go func() {
        for err := range generator.GetErrors() {
            log.Printf("❌ Generator error: %v", err)
            errorCount++
        }
    }()
    
    // 【メモリ効率的処理】一度に1件ずつ処理
    for {
        user, ok := generator.Next()
        if !ok {
            break
        }
        
        // 【実際の処理】CPUを使う重い処理も安全
        if err := processUserWithMLAnalysis(user); err != nil {
            log.Printf("❌ Processing failed for user %d: %v", user.ID, err)
            errorCount++
            continue
        }
        
        processedCount++
        
        // 【定期レポート】
        if processedCount%50000 == 0 {
            elapsed := time.Since(startTime)
            rate := float64(processedCount) / elapsed.Seconds()
            
            var m runtime.MemStats
            runtime.ReadMemStats(&m)
            
            log.Printf("✅ Processed %d users (%.0f/sec, memory: %.2fMB, errors: %d)", 
                processedCount, rate, float64(m.Alloc)/1024/1024, errorCount)
        }
    }
    
    totalTime := time.Since(startTime)
    finalRate := float64(processedCount) / totalTime.Seconds()
    
    log.Printf("🎯 Processing completed:")
    log.Printf("   Total processed: %d users", processedCount)
    log.Printf("   Total errors: %d", errorCount)
    log.Printf("   Processing time: %v", totalTime)
    log.Printf("   Processing rate: %.0f users/sec", finalRate)
    log.Printf("   Success rate: %.2f%%", float64(processedCount)/(float64(processedCount+errorCount))*100)
    
    // 【メモリ使用量確認】
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    log.Printf("   Final memory usage: %.2fMB (peak: %.2fMB)", 
        float64(m.Alloc)/1024/1024, float64(m.Sys)/1024/1024)
}

// 【高度な機能】並列処理対応Generator
func processUserWithMLAnalysis(user User) error {
    // 機械学習による分析処理（CPU集約的）
    time.Sleep(100 * time.Millisecond) // 重い処理をシミュレート
    
    // 10%の確率でエラー
    if rand.Float64() < 0.1 {
        return fmt.Errorf("ML analysis failed for user %d", user.ID)
    }
    
    return nil
}
```

### Goでの実装アプローチ

Goでは、チャネルとGoroutineを使ってGeneratorパターンを実装します：

#### 1. プロダクション品質の基本構造

```go
// 【プロダクション品質】型安全で高性能なGenerator
type Generator[T any] struct {
    // 【基本機能】
    ch          <-chan T              // 読み取り専用チャネル
    cancel      context.CancelFunc    // キャンセレーション制御
    ctx         context.Context       // コンテキスト管理
    
    // 【高度な機能】
    buffer      int                   // バッファサイズ
    generated   *atomic.Int64         // 生成数カウンタ（thread-safe）
    consumed    *atomic.Int64         // 消費数カウンタ（thread-safe）
    
    // 【エラー処理】
    errorChan   <-chan error          // エラー通知チャネル
    onError     func(error)           // エラーハンドラ
    
    // 【状態管理】
    state       *atomic.Int32         // 0:running, 1:completed, 2:cancelled
    startTime   time.Time             // 開始時刻
    
    // 【監視機能】
    metrics     *GeneratorMetrics     // パフォーマンスメトリクス
    logger      *log.Logger           // ログ出力
}

// GeneratorFunc defines the function signature for generator functions
type GeneratorFunc[T any] func(ctx context.Context, yield func(T) bool) error

// GeneratorMetrics tracks performance statistics
type GeneratorMetrics struct {
    ItemsGenerated   int64         // 生成されたアイテム数
    ItemsConsumed    int64         // 消費されたアイテム数  
    GenerationRate   float64       // 生成レート（items/sec）
    ConsumptionRate  float64       // 消費レート（items/sec）
    BufferUtilization float64      // バッファ利用率
    ErrorCount       int64         // エラー発生回数
    LastActivity     time.Time     // 最終活動時刻
}
```

#### 2. エンタープライズレベルGenerator作成

```go
// 【重要関数】高性能でエラー耐性のあるGenerator作成
func NewEnterpriseGenerator[T any](
    ctx context.Context, 
    fn GeneratorFunc[T], 
    bufferSize int,
) *Generator[T] {
    if bufferSize <= 0 {
        bufferSize = 100 // デフォルトバッファサイズ
    }
    
    generatorCtx, cancel := context.WithCancel(ctx)
    
    // チャネル作成（バッファ付き）
    dataChan := make(chan T, bufferSize)
    errorChan := make(chan error, 10)
    
    // メトリクス初期化
    metrics := &GeneratorMetrics{
        LastActivity: time.Now(),
    }
    
    generator := &Generator[T]{
        ch:          dataChan,
        cancel:      cancel,
        ctx:         generatorCtx,
        buffer:      bufferSize,
        generated:   &atomic.Int64{},
        consumed:    &atomic.Int64{},
        errorChan:   errorChan,
        state:       &atomic.Int32{}, // 0 = running
        startTime:   time.Now(),
        metrics:     metrics,
        logger:      log.New(os.Stdout, "[GENERATOR] ", log.LstdFlags),
    }
    
    // 【重要】非同期で生成処理開始
    go generator.runGeneratorWithRecovery(fn, dataChan, errorChan)
    
    // 【重要】メトリクス更新goroutine開始  
    go generator.updateMetrics()
    
    generator.logger.Printf("🚀 Enterprise generator started (buffer: %d)", bufferSize)
    return generator
}

// 【核心メソッド】パニック回復付きGenerator実行
func (g *Generator[T]) runGeneratorWithRecovery(
    fn GeneratorFunc[T], 
    dataChan chan<- T,
    errorChan chan<- error,
) {
    defer func() {
        close(dataChan)
        close(errorChan)
        g.state.Store(1) // completed
        
        // パニック回復
        if r := recover(); r != nil {
            g.logger.Printf("❌ Generator panic recovered: %v", r)
            if g.onError != nil {
                g.onError(fmt.Errorf("generator panic: %v", r))
            }
        }
        
        generated := g.generated.Load()
        duration := time.Since(g.startTime)
        rate := float64(generated) / duration.Seconds()
        
        g.logger.Printf("✅ Generator completed: %d items in %v (%.0f items/sec)", 
            generated, duration, rate)
    }()
    
    // yield関数の定義（thread-safe）
    yield := func(value T) bool {
        select {
        case dataChan <- value:
            count := g.generated.Add(1)
            g.metrics.LastActivity = time.Now()
            
            // 定期的なプログレスレポート
            if count%10000 == 0 {
                g.logger.Printf("📊 Generated %d items", count)
            }
            return true
            
        case <-g.ctx.Done():
            g.state.Store(2) // cancelled
            return false
        }
    }
    
    // ユーザー定義の生成関数実行
    if err := fn(g.ctx, yield); err != nil {
        select {
        case errorChan <- err:
            g.logger.Printf("❌ Generator function error: %v", err)
        case <-g.ctx.Done():
        }
    }
}

// 【監視機能】リアルタイムメトリクス更新
func (g *Generator[T]) updateMetrics() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    var lastGenerated, lastConsumed int64
    var lastTime = time.Now()
    
    for {
        select {
        case <-ticker.C:
            now := time.Now()
            elapsed := now.Sub(lastTime).Seconds()
            
            currentGenerated := g.generated.Load()
            currentConsumed := g.consumed.Load()
            
            // レート計算
            generationRate := float64(currentGenerated-lastGenerated) / elapsed
            consumptionRate := float64(currentConsumed-lastConsumed) / elapsed
            
            // バッファ利用率計算
            bufferUtilization := float64(currentGenerated-currentConsumed) / float64(g.buffer) * 100
            if bufferUtilization > 100 {
                bufferUtilization = 100
            }
            
            // メトリクス更新
            g.metrics.GenerationRate = generationRate
            g.metrics.ConsumptionRate = consumptionRate
            g.metrics.BufferUtilization = bufferUtilization
            g.metrics.ItemsGenerated = currentGenerated
            g.metrics.ItemsConsumed = currentConsumed
            
            // アラート条件チェック
            if bufferUtilization > 90 {
                g.logger.Printf("⚠️  High buffer utilization: %.1f%%", bufferUtilization)
            }
            
            if generationRate > 0 && consumptionRate > 0 && generationRate > consumptionRate*2 {
                g.logger.Printf("⚠️  Generation outpacing consumption: gen=%.0f/sec, cons=%.0f/sec", 
                    generationRate, consumptionRate)
            }
            
            lastGenerated = currentGenerated
            lastConsumed = currentConsumed  
            lastTime = now
            
        case <-g.ctx.Done():
            return
        }
    }
}
```

#### 3. 高性能な数値範囲Generator

```go
// 【実用例】高性能な数値範囲Generator（並列化対応）
func NewParallelRangeGenerator(ctx context.Context, start, end int, workers int) *Generator[int] {
    if workers <= 0 {
        workers = runtime.NumCPU()
    }
    
    return NewEnterpriseGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
        // 【並列化】範囲を複数ワーカーに分割
        chunkSize := (end - start + 1) / workers
        if chunkSize == 0 {
            chunkSize = 1
        }
        
        var wg sync.WaitGroup
        resultChan := make(chan int, workers*100)
        
        for i := 0; i < workers; i++ {
            wg.Add(1)
            go func(workerID int) {
                defer wg.Done()
                
                workerStart := start + workerID*chunkSize
                workerEnd := workerStart + chunkSize - 1
                if workerID == workers-1 {
                    workerEnd = end // 最後のワーカーは残り全て
                }
                
                log.Printf("🔧 Worker %d processing range [%d, %d]", workerID, workerStart, workerEnd)
                
                for num := workerStart; num <= workerEnd; num++ {
                    select {
                    case resultChan <- num:
                    case <-ctx.Done():
                        return
                    }
                }
            }(i)
        }
        
        // 結果を順序保証で出力
        go func() {
            wg.Wait()
            close(resultChan)
        }()
        
        // ソート用の一時バッファ
        var buffer []int
        for num := range resultChan {
            buffer = append(buffer, num)
        }
        
        // ソートして順序保証
        sort.Ints(buffer)
        
        // 順序通りにyield
        for _, num := range buffer {
            if !yield(num) {
                return nil
            }
        }
        
        return nil
    }, 1000)
}

// 【実用例】無限フィボナッチGenerator（メモリ効率的）
func NewFibonacciGenerator(ctx context.Context) *Generator[int] {
    return NewEnterpriseGenerator(ctx, func(ctx context.Context, yield func(int) bool) error {
        a, b := 0, 1
        
        // 最初の2つの値
        if !yield(a) || !yield(b) {
            return nil
        }
        
        for {
            select {
            case <-ctx.Done():
                return ctx.Err()
            default:
                next := a + b
                
                // オーバーフロー検出
                if next < 0 {
                    return fmt.Errorf("fibonacci overflow detected at %d + %d", a, b)
                }
                
                if !yield(next) {
                    return nil
                }
                
                a, b = b, next
                
                // 定期的にGCを呼び出してメモリ効率化
                if next%1000000 == 0 {
                    runtime.GC()
                }
            }
        }
    }, 100)
}
```

### 変換操作（Transformation）

Generatorパターンの強力な点は、関数型プログラミングの操作を組み合わせられることです：

#### プロダクション品質のMap変換

```go
// 【高性能Map変換】並列処理対応とエラーハンドリング
func Map[T, U any](gen *Generator[T], fn func(T) (U, error), workers int) *Generator[U] {
    if workers <= 0 {
        workers = runtime.NumCPU()
    }
    
    return NewEnterpriseGenerator(gen.ctx, func(ctx context.Context, yield func(U) bool) error {
        // 【並列処理】複数ワーカーで変換処理
        inputChan := make(chan T, workers*2)
        resultChan := make(chan transformResult[U], workers*2)
        
        var wg sync.WaitGroup
        
        // ワーカー起動
        for i := 0; i < workers; i++ {
            wg.Add(1)
            go func(workerID int) {
                defer wg.Done()
                
                for {
                    select {
                    case input, ok := <-inputChan:
                        if !ok {
                            return
                        }
                        
                        // 変換処理実行
                        result, err := fn(input)
                        
                        select {
                        case resultChan <- transformResult[U]{
                            Value:    result,
                            Error:    err,
                            WorkerID: workerID,
                        }:
                        case <-ctx.Done():
                            return
                        }
                        
                    case <-ctx.Done():
                        return
                    }
                }
            }(i)
        }
        
        // 入力データの供給
        go func() {
            defer close(inputChan)
            for {
                select {
                case value, ok := <-gen.ch:
                    if !ok {
                        return
                    }
                    gen.consumed.Add(1)
                    
                    select {
                    case inputChan <- value:
                    case <-ctx.Done():
                        return
                    }
                    
                case <-ctx.Done():
                    return
                }
            }
        }()
        
        // 結果の収集と出力
        go func() {
            wg.Wait()
            close(resultChan)
        }()
        
        successCount := 0
        errorCount := 0
        
        for result := range resultChan {
            if result.Error != nil {
                errorCount++
                gen.logger.Printf("❌ Map transformation error (worker %d): %v", 
                    result.WorkerID, result.Error)
                continue
            }
            
            if !yield(result.Value) {
                break
            }
            
            successCount++
            
            // 定期的なパフォーマンスレポート
            if (successCount+errorCount)%10000 == 0 {
                gen.logger.Printf("📊 Map progress: %d success, %d errors", 
                    successCount, errorCount)
            }
        }
        
        gen.logger.Printf("✅ Map completed: %d success, %d errors", successCount, errorCount)
        return nil
    }, 1000)
}

type transformResult[T any] struct {
    Value    T
    Error    error
    WorkerID int
}

// 【実用例】CPU集約的な変換処理
func CPUIntensiveMapExample() {
    ctx := context.Background()
    
    // 1万から10万までの数値に対してCPU集約的処理
    numbers := NewParallelRangeGenerator(ctx, 10000, 100000, 4)
    
    // 各数値に対して素数判定（CPU集約的）
    primeCheck := Map(numbers, func(n int) (bool, error) {
        if n < 2 {
            return false, nil
        }
        
        // 素数判定（CPU集約的処理）
        for i := 2; i*i <= n; i++ {
            if n%i == 0 {
                return false, nil
            }
        }
        return true, nil
    }, 8) // 8並列で処理
    
    primeCount := 0
    for isPrime, ok := primeCheck.Next(); ok; isPrime, ok = primeCheck.Next() {
        if isPrime {
            primeCount++
        }
    }
    
    log.Printf("Found %d prime numbers", primeCount)
}
```

#### 高性能Filter操作

```go
// 【高性能Filter】条件分岐最適化とバッチ処理
func Filter[T any](gen *Generator[T], predicate func(T) bool, batchSize int) *Generator[T] {
    if batchSize <= 0 {
        batchSize = 1000
    }
    
    return NewEnterpriseGenerator(gen.ctx, func(ctx context.Context, yield func(T) bool) error {
        batch := make([]T, 0, batchSize)
        filteredCount := 0
        totalProcessed := 0
        
        for {
            select {
            case value, ok := <-gen.ch:
                if !ok {
                    // 最後のバッチを処理
                    for _, item := range batch {
                        if predicate(item) {
                            if !yield(item) {
                                return nil
                            }
                            filteredCount++
                        }
                    }
                    
                    gen.logger.Printf("✅ Filter completed: %d/%d items passed (%.2f%%)", 
                        filteredCount, totalProcessed, 
                        float64(filteredCount)/float64(totalProcessed)*100)
                    return nil
                }
                
                gen.consumed.Add(1)
                batch = append(batch, value)
                totalProcessed++
                
                // バッチが満杯になったら処理
                if len(batch) >= batchSize {
                    batchStart := time.Now()
                    batchFiltered := 0
                    
                    for _, item := range batch {
                        if predicate(item) {
                            if !yield(item) {
                                return nil
                            }
                            batchFiltered++
                            filteredCount++
                        }
                    }
                    
                    batchDuration := time.Since(batchStart)
                    processingRate := float64(batchSize) / batchDuration.Seconds()
                    
                    gen.logger.Printf("📦 Batch filtered: %d/%d passed (rate: %.0f items/sec)", 
                        batchFiltered, batchSize, processingRate)
                    
                    // バッチをリセット
                    batch = batch[:0]
                }
                
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }, 1000)
}

// 【実用例】複雑な条件でのフィルタリング
func ComplexFilterExample() {
    ctx := context.Background()
    
    // ユーザーデータのGenerator（仮想）
    users := NewMemoryEfficientUserGenerator(ctx, 1000)
    
    // 複雑な条件でフィルタリング
    activeUsers := Filter(users, func(user User) bool {
        // 複数の条件を組み合わせ
        return user.Active && 
               user.LastLoginDays <= 30 && 
               user.AccountType == "premium" &&
               len(user.Email) > 0
    }, 500) // 500件バッチ処理
    
    count := 0
    for user, ok := activeUsers.Next(); ok; user, ok = activeUsers.Next() {
        processActiveUser(user)
        count++
    }
    
    log.Printf("Processed %d active premium users", count)
}
```

#### スマートTake操作

```go
// 【高度なTake操作】動的制限とメモリ効率化
func Take[T any](gen *Generator[T], n int) *Generator[T] {
    return NewEnterpriseGenerator(gen.ctx, func(ctx context.Context, yield func(T) bool) error {
        if n <= 0 {
            gen.logger.Printf("⚠️  Take with n=%d, no items will be yielded", n)
            return nil
        }
        
        count := 0
        startTime := time.Now()
        
        for {
            select {
            case value, ok := <-gen.ch:
                if !ok {
                    gen.logger.Printf("✅ Take completed early: %d/%d items (source exhausted)", 
                        count, n)
                    return nil
                }
                
                gen.consumed.Add(1)
                
                if !yield(value) {
                    gen.logger.Printf("✅ Take completed: %d/%d items (consumer stopped)", 
                        count, n)
                    return nil
                }
                
                count++
                
                // 目標数に達したら終了
                if count >= n {
                    duration := time.Since(startTime)
                    rate := float64(count) / duration.Seconds()
                    
                    gen.logger.Printf("✅ Take completed: %d items in %v (%.0f items/sec)", 
                        count, duration, rate)
                    return nil
                }
                
                // 進捗レポート
                if count%10000 == 0 {
                    progress := float64(count) / float64(n) * 100
                    gen.logger.Printf("📊 Take progress: %d/%d (%.1f%%)", count, n, progress)
                }
                
            case <-ctx.Done():
                gen.logger.Printf("⚠️  Take cancelled: %d/%d items", count, n)
                return ctx.Err()
            }
        }
    }, min(n, 1000)) // バッファサイズを適切に設定
}

// 【実用例】無限ストリームからの効率的な抽出
func InfiniteStreamSamplingExample() {
    ctx := context.Background()
    
    // 無限フィボナッチ数列
    fibonacci := NewFibonacciGenerator(ctx)
    
    // 最初の10万個を取得
    first100k := Take(fibonacci, 100000)
    
    // さらに100で割り切れるものだけをフィルタ
    divisibleBy100 := Filter(first100k, func(n int) bool {
        return n%100 == 0
    }, 1000)
    
    // 最初の1000個を最終的に取得
    final1000 := Take(divisibleBy100, 1000)
    
    results := make([]int, 0, 1000)
    for value, ok := final1000.Next(); ok; value, ok = final1000.Next() {
        results = append(results, value)
    }
    
    log.Printf("Collected %d fibonacci numbers divisible by 100", len(results))
    log.Printf("First few: %v", results[:min(len(results), 10)])
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
```

### 組み合わせ（Composition）

複数のGeneratorを組み合わせて複雑な処理を構築できます：

```go
// 1から100までの数字から、3で割り切れる数の平方を文字列として取得
result := Map(
    Filter(Range(1, 100), func(x int) bool {
        return x%3 == 0
    }),
    func(x int) string {
        return fmt.Sprintf("square:%d", x*x)
    },
)

strings := result.ToSlice()
// ["square:9", "square:36", "square:81", ...]
```

### 並列処理

Generatorパターンは並列処理とも相性が良いです：

```go
func Parallel[T, U any](gen Generator[T], fn func(T) U, workers int) Generator[U] {
    return NewGenerator(func(ctx context.Context, yield func(U) bool) {
        input := make(chan T, workers)
        output := make(chan U, workers)
        
        // ワーカー起動
        var wg sync.WaitGroup
        for i := 0; i < workers; i++ {
            wg.Add(1)
            go func() {
                defer wg.Done()
                for value := range input {
                    output <- fn(value)
                }
            }()
        }
        
        // 入力をワーカーに分散
        go func() {
            defer close(input)
            for value := range gen.ch {
                input <- value
            }
        }()
        
        // 結果を出力
        go func() {
            wg.Wait()
            close(output)
        }()
        
        for result := range output {
            if !yield(result) {
                return
            }
        }
    })
}
```

### 実践的な使用例

#### 1. ファイル処理
```go
func ReadLines(filename string) Generator[string] {
    return NewGenerator(func(ctx context.Context, yield func(string) bool) {
        file, err := os.Open(filename)
        if err != nil {
            return
        }
        defer file.Close()
        
        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
            if !yield(scanner.Text()) {
                return
            }
        }
    })
}
```

#### 2. HTTP APIの大量データ取得
```go
func FetchPages(baseURL string) Generator[APIResponse] {
    return NewGenerator(func(ctx context.Context, yield func(APIResponse) bool) {
        page := 1
        for {
            resp, err := fetchPage(baseURL, page)
            if err != nil || resp.IsEmpty() {
                return
            }
            if !yield(resp) {
                return
            }
            page++
        }
    })
}
```

## 📝 課題 (The Problem)

`main_test.go`に書かれているテストをパスするように、以下のGeneratorパターンを実装してください：

### 実装すべき構造体と関数

```go
// Generator represents a generator that produces values of type T
type Generator[T any] struct {
    ch     <-chan T
    cancel context.CancelFunc
    ctx    context.Context
}

// GeneratorFunc is a function that generates values
type GeneratorFunc[T any] func(ctx context.Context, yield func(T) bool)

// 基本関数
func NewGenerator[T any](fn GeneratorFunc[T]) Generator[T]
func (g Generator[T]) Next() (T, bool)
func (g Generator[T]) ToSlice() []T
func (g Generator[T]) ForEach(fn func(T))
func (g Generator[T]) Cancel()

// 基本Generator
func Range(start, end int) Generator[int]
func Repeat[T any](value T) Generator[T]
func FromSlice[T any](slice []T) Generator[T]
func Fibonacci() Generator[int]
func Timer(interval time.Duration) Generator[time.Time]

// 変換操作
func Map[T, U any](gen Generator[T], fn func(T) U) Generator[U]
func Filter[T any](gen Generator[T], predicate func(T) bool) Generator[T]
func Take[T any](gen Generator[T], n int) Generator[T]
func Skip[T any](gen Generator[T], n int) Generator[T]
func TakeWhile[T any](gen Generator[T], predicate func(T) bool) Generator[T]

// 組み合わせ操作
func Chain[T any](generators ...Generator[T]) Generator[T]
func Zip[T, U any](gen1 Generator[T], gen2 Generator[U]) Generator[Pair[T, U]]

// 集約操作
func Reduce[T, U any](gen Generator[T], initial U, fn func(U, T) U) U
func Count[T any](gen Generator[T]) int
func Any[T any](gen Generator[T], predicate func(T) bool) bool
func All[T any](gen Generator[T], predicate func(T) bool) bool

// 高度な機能
func Batch[T any](gen Generator[T], size int) Generator[[]T]
func Distinct[T comparable](gen Generator[T]) Generator[T]
func Parallel[T, U any](gen Generator[T], fn func(T) U, workers int) Generator[U]
func Buffer[T any](gen Generator[T], size int) Generator[T]
```

## ✅ 期待される挙動 (Expected Behavior)

実装が完了すると、以下のような動作が期待されます：

### 1. 基本的な使用
```go
// 数値範囲の生成
gen := Range(1, 5)
values := gen.ToSlice()
// [1, 2, 3, 4, 5]
```

### 2. 変換操作
```go
// Map: 各値を2倍に
doubled := Map(Range(1, 5), func(x int) int { return x * 2 })
// [2, 4, 6, 8, 10]

// Filter: 偶数のみ
evens := Filter(Range(1, 10), func(x int) bool { return x%2 == 0 })
// [2, 4, 6, 8, 10]
```

### 3. 無限シーケンス
```go
// フィボナッチ数列の最初の10個
fibs := Take(Fibonacci(), 10).ToSlice()
// [0, 1, 1, 2, 3, 5, 8, 13, 21, 34]
```

### 4. 組み合わせ処理
```go
// 複雑な変換パイプライン
result := Map(
    Filter(Range(1, 20), func(x int) bool { return x%3 == 0 }),
    func(x int) string { return fmt.Sprintf("num-%d", x) },
).ToSlice()
// ["num-3", "num-6", "num-9", "num-12", "num-15", "num-18"]
```

### 5. テスト結果
```bash
$ go test -v
=== RUN   TestBasicGenerators
--- PASS: TestBasicGenerators (0.00s)
=== RUN   TestTransformations
--- PASS: TestTransformations (0.00s)
=== RUN   TestComposition
--- PASS: TestComposition (0.00s)
=== RUN   TestAggregations
--- PASS: TestAggregations (0.00s)
PASS
```

## 💡 ヒント (Hints)

詰まった場合は、以下のヒントを参考にしてください：

### 1. 基本的なGenerator実装
```go
func NewGenerator[T any](fn GeneratorFunc[T]) Generator[T] {
    ctx, cancel := context.WithCancel(context.Background())
    ch := make(chan T)
    
    go func() {
        defer close(ch)
        fn(ctx, func(value T) bool {
            select {
            case ch <- value:
                return true
            case <-ctx.Done():
                return false
            }
        })
    }()
    
    return Generator[T]{ch: ch, cancel: cancel, ctx: ctx}
}
```

### 2. 役立つパッケージ
- `context`: キャンセレーション制御
- `sync`: 並列処理制御
- `time`: タイマーとチャネル操作
- `container/list`: バッファ管理

### 3. チャネル操作のパターン
```go
// チャネルからの読み取り
for value := range gen.ch {
    // 処理
}

// コンテキストキャンセレーションの監視
select {
case <-ctx.Done():
    return
case value := <-ch:
    // 処理
}
```

### 4. Goroutineリーク防止
```go
// 必ずGoroutineを適切に終了
defer close(ch)

// キャンセレーション時の処理
func (g Generator[T]) Cancel() {
    if g.cancel != nil {
        g.cancel()
    }
}
```

### 5. 段階的な実装順序

1. **基本構造**: `Generator`構造体と`NewGenerator`関数
2. **基本操作**: `Range`, `FromSlice`, `Next`, `ToSlice`
3. **変換操作**: `Map`, `Filter`, `Take`
4. **組み合わせ**: `Chain`, `Zip`
5. **集約操作**: `Reduce`, `Count`
6. **高度な機能**: `Parallel`, `Batch`, `Distinct`

これらのヒントを参考に、段階的に実装を進めてください。まずは最も基本的な`Range`と`ToSlice`から始めて、徐々に複雑な操作を追加していくのがおすすめです。