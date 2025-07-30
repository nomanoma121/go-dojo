# Day 30: ベンチマークテスト

🎯 **本日の目標**
Go言語のベンチマークテスト機能を使用して、関数やアルゴリズムの性能を正確に測定し、メモリ使用量の分析、CPUプロファイリング、性能最適化の手法を習得できるようになる。

## 📖 解説

### ベンチマークテストとは

```go
// 【ベンチマークテストの重要性】性能測定と最適化による競争優位性確保
// ❌ 問題例：パフォーマンス測定なしでの性能劣化とユーザー離れ
func catastrophicNoPerformanceMeasurement() {
    // 🚨 災害例：性能測定なしでのパフォーマンス劣化による事業停止
    
    // ❌ 非効率な文字列結合（測定なし）
    func processLargeData(items []string) string {
        var result string
        
        // 10万件のデータを非効率に処理
        for _, item := range items { // 100,000 iterations
            // ❌ 文字列連結で毎回新しいメモリ割り当て
            result += item + ","
            // メモリ使用量: O(n²)、実行時間: O(n²)
        }
        
        // 【実際の被害】100,000件処理で：
        // - 実行時間: 45秒（ユーザー待機限界超過）
        // - メモリ使用量: 12GB（サーバー枯渇）
        // - GC停止時間: 8秒（アプリ無反応）
        
        return result
    }
    
    // ❌ 無駄なアロケーション（測定なし）
    func calculateStatistics(numbers []float64) Statistics {
        // 毎回新しいスライス作成
        var processedNumbers []float64
        
        for _, num := range numbers {
            processedNumbers = append(processedNumbers, num*2)
            // ❌ append毎にメモリ再割り当て
        }
        
        // ❌ 複数回の無駄なソート
        sort.Float64s(processedNumbers) // ソート1回目
        median := calculateMedian(processedNumbers)
        sort.Float64s(processedNumbers) // 同じデータを再ソート
        mean := calculateMean(processedNumbers)
        
        // 結果：1000万件のデータで30分のレスポンス時間
        return Statistics{Median: median, Mean: mean}
    }
    
    // ❌ 並行処理の性能問題（測定なし）
    func concurrentProcessing(data []DataItem) []ProcessedItem {
        var mu sync.Mutex
        var results []ProcessedItem
        
        // ❌ Goroutine数制限なし→リソース枯渇
        for _, item := range data { // 100万件
            go func(item DataItem) {
                processed := heavyProcessing(item) // 1秒/件
                
                // ❌ 粗粒度ロック→並行性ゼロ
                mu.Lock()
                results = append(results, processed)
                mu.Unlock()
            }(item)
        }
        
        // 【災害結果】
        // - 100万Goroutine起動→メモリ不足でクラッシュ
        // - ロック競合で実質シーケンシャル実行
        // - 処理時間: 100万秒（11.5日）
        
        return results
    }
    
    // 【実際の被害例】
    // - ECサイト：検索レスポンス30秒→顧客離脱率95%
    // - 金融システム：取引処理5分→監査法人から業務改善命令
    // - 動画配信：エンコード24時間→サービス停止
    // - ゲーム：マッチング15分→ユーザー激減
    
    fmt.Println("❌ No performance measurement led to business failure!")
    // 結果：売上90%減、競合他社に顧客流出、事業撤退
}

// ✅ 正解：エンタープライズ級ベンチマーク測定システム
type EnterpriseBenchmarkSystem struct {
    // 【基本測定機能】
    performanceMeter   *PerformanceMeter              // 性能測定
    memoryAnalyzer     *MemoryAnalyzer                // メモリ解析
    cpuProfiler        *CPUProfiler                   // CPU分析
    
    // 【高度測定機能】
    latencyTracker     *LatencyTracker                // レイテンシ追跡
    throughputMeter    *ThroughputMeter               // スループット測定
    concurrencyAnalyzer *ConcurrencyAnalyzer          // 並行性解析
    
    // 【マイクロベンチマーク】
    algorithmBench     *AlgorithmBenchmark            // アルゴリズム比較
    dataStructureBench *DataStructureBenchmark        // データ構造比較
    ioPerformanceBench *IOPerformanceBenchmark        // I/O性能測定
    
    // 【システム統合測定】
    endToEndBench      *EndToEndBenchmark             // E2E性能測定
    loadTestRunner     *LoadTestRunner                // 負荷テスト
    stressTestRunner   *StressTestRunner              // ストレステスト
    
    // 【自動最適化】
    optimizer          *PerformanceOptimizer          // 自動最適化
    suggestionEngine   *OptimizationSuggestionEngine  // 最適化提案
    
    // 【継続監視】
    regressionDetector *RegressionDetector            // 回帰検出
    alertSystem        *PerformanceAlertSystem        // パフォーマンスアラート
    
    // 【レポート生成】
    reportGenerator    *BenchmarkReportGenerator       // レポート生成
    visualizer         *PerformanceVisualizer         // 可視化
    
    config             *BenchmarkConfig               // 設定管理
    mu                 sync.RWMutex                   // 並行アクセス制御
}

// 【重要関数】包括的ベンチマークシステム初期化
func NewEnterpriseBenchmarkSystem(config *BenchmarkConfig) *EnterpriseBenchmarkSystem {
    return &EnterpriseBenchmarkSystem{
        config:              config,
        performanceMeter:    NewPerformanceMeter(),
        memoryAnalyzer:      NewMemoryAnalyzer(),
        cpuProfiler:         NewCPUProfiler(),
        latencyTracker:      NewLatencyTracker(),
        throughputMeter:     NewThroughputMeter(),
        concurrencyAnalyzer: NewConcurrencyAnalyzer(),
        algorithmBench:      NewAlgorithmBenchmark(),
        dataStructureBench:  NewDataStructureBenchmark(),
        ioPerformanceBench:  NewIOPerformanceBenchmark(),
        endToEndBench:       NewEndToEndBenchmark(),
        loadTestRunner:      NewLoadTestRunner(),
        stressTestRunner:    NewStressTestRunner(),
        optimizer:           NewPerformanceOptimizer(),
        suggestionEngine:    NewOptimizationSuggestionEngine(),
        regressionDetector:  NewRegressionDetector(),
        alertSystem:         NewPerformanceAlertSystem(),
        reportGenerator:     NewBenchmarkReportGenerator(),
        visualizer:          NewPerformanceVisualizer(),
    }
}

// 【核心メソッド】包括的パフォーマンス測定と最適化
func (ebs *EnterpriseBenchmarkSystem) ExecuteComprehensiveBenchmark(
    function interface{}, 
    testData interface{},
) (*BenchmarkResult, error) {
    
    // 【STEP 1】マイクロベンチマーク実行
    microResults, err := ebs.runMicroBenchmarks(function, testData)
    if err != nil {
        return nil, fmt.Errorf("micro benchmark failed: %w", err)
    }
    
    // 【STEP 2】メモリプロファイリング
    memoryProfile, err := ebs.memoryAnalyzer.ProfileMemoryUsage(function, testData)
    if err != nil {
        return nil, fmt.Errorf("memory profiling failed: %w", err)
    }
    
    // 【STEP 3】CPU プロファイリング
    cpuProfile, err := ebs.cpuProfiler.ProfileCPUUsage(function, testData)
    if err != nil {
        return nil, fmt.Errorf("CPU profiling failed: %w", err)
    }
    
    // 【STEP 4】並行性解析
    concurrencyMetrics, err := ebs.concurrencyAnalyzer.AnalyzeConcurrency(function, testData)
    if err != nil {
        return nil, fmt.Errorf("concurrency analysis failed: %w", err)
    }
    
    // 【STEP 5】レイテンシとスループット測定
    latencyMetrics := ebs.latencyTracker.TrackLatency(function, testData)
    throughputMetrics := ebs.throughputMeter.MeasureThroughput(function, testData)
    
    // 【STEP 6】最適化提案生成
    suggestions := ebs.suggestionEngine.GenerateOptimizationSuggestions(
        microResults, memoryProfile, cpuProfile, concurrencyMetrics,
    )
    
    return &BenchmarkResult{
        MicroBenchmarks:    microResults,
        MemoryProfile:      memoryProfile,
        CPUProfile:         cpuProfile,
        ConcurrencyMetrics: concurrencyMetrics,
        LatencyMetrics:     latencyMetrics,
        ThroughputMetrics:  throughputMetrics,
        Suggestions:        suggestions,
        Timestamp:          time.Now(),
    }, nil
}

// 【実用例】エンタープライズ級文字列処理ベンチマーク
func BenchmarkEnterpriseStringProcessing(b *testing.B) {
    benchmarkSystem := NewEnterpriseBenchmarkSystem(&BenchmarkConfig{
        EnableMemoryProfiling: true,
        EnableCPUProfiling:   true,
        EnableConcurrencyAnalysis: true,
        DetailedReporting:    true,
    })
    
    // 【テストデータ生成】実際のワークロード想定
    testCases := []struct {
        name     string
        dataSize int
        function func([]string) string
    }{
        {
            name:     "StringBuilder_最適化済み",
            dataSize: 100000,
            function: func(items []string) string {
                // ✅ 事前容量確保で効率的な文字列結合
                totalLen := 0
                for _, item := range items {
                    totalLen += len(item) + 1 // +1 for comma
                }
                
                var builder strings.Builder
                builder.Grow(totalLen) // 事前にメモリ確保
                
                for i, item := range items {
                    if i > 0 {
                        builder.WriteByte(',')
                    }
                    builder.WriteString(item)
                }
                
                return builder.String()
            },
        },
        {
            name:     "ByteBuffer_超最適化",
            dataSize: 100000,
            function: func(items []string) string {
                // ✅ bytes.Buffer使用でさらなる最適化
                totalLen := 0
                for _, item := range items {
                    totalLen += len(item) + 1
                }
                
                buf := make([]byte, 0, totalLen)
                buffer := bytes.NewBuffer(buf)
                
                for i, item := range items {
                    if i > 0 {
                        buffer.WriteByte(',')
                    }
                    buffer.WriteString(item)
                }
                
                return buffer.String()
            },
        },
        {
            name:     "SliceJoin_内蔵関数活用",
            dataSize: 100000,
            function: func(items []string) string {
                // ✅ strings.Join の内部最適化活用
                return strings.Join(items, ",")
            },
        },
        {
            name:     "ConcurrentProcessing_並列最適化",
            dataSize: 100000,
            function: func(items []string) string {
                // ✅ 並列処理による高速化
                const numWorkers = 8
                chunkSize := len(items) / numWorkers
                
                results := make([]string, numWorkers)
                var wg sync.WaitGroup
                
                for i := 0; i < numWorkers; i++ {
                    wg.Add(1)
                    go func(workerID int) {
                        defer wg.Done()
                        
                        start := workerID * chunkSize
                        end := start + chunkSize
                        if workerID == numWorkers-1 {
                            end = len(items) // 最後のワーカーは残り全て
                        }
                        
                        chunk := items[start:end]
                        results[workerID] = strings.Join(chunk, ",")
                    }(i)
                }
                
                wg.Wait()
                return strings.Join(results, ",")
            },
        },
    }
    
    // 【テスト実行】各実装の詳細性能測定
    for _, tc := range testCases {
        b.Run(tc.name, func(b *testing.B) {
            // テストデータ準備
            testData := make([]string, tc.dataSize)
            for i := 0; i < tc.dataSize; i++ {
                testData[i] = fmt.Sprintf("item_%d", i)
            }
            
            // 【詳細測定開始】
            b.ReportAllocs()    // メモリアロケーション測定
            b.ResetTimer()      // 準備時間除外
            
            var result string
            for i := 0; i < b.N; i++ {
                result = tc.function(testData)
            }
            
            // コンパイラ最適化防止
            _ = result
            
            // 【カスタムメトリクス記録】
            b.StopTimer()
            
            // エンタープライズベンチマーク実行
            benchResult, err := benchmarkSystem.ExecuteComprehensiveBenchmark(
                tc.function, testData,
            )
            if err != nil {
                b.Fatalf("Enterprise benchmark failed: %v", err)
            }
            
            // 詳細レポート生成
            report := benchmarkSystem.reportGenerator.GenerateDetailedReport(benchResult)
            b.Logf("Performance Report for %s:\n%s", tc.name, report)
            
            // 性能回帰チェック
            if regression := benchmarkSystem.regressionDetector.CheckRegression(
                tc.name, benchResult); regression != nil {
                b.Errorf("Performance regression detected: %v", regression)
            }
            
            b.StartTimer()
        })
    }
}

// 【高度ベンチマーク】リアルワールドシナリオ測定
func BenchmarkRealWorldPerformance(b *testing.B) {
    // 【シナリオ1】大規模データ処理システム
    b.Run("BigDataProcessing", func(b *testing.B) {
        dataSize := 10000000 // 1000万件
        data := generateRealWorldData(dataSize)
        
        b.ReportAllocs()
        b.SetBytes(int64(dataSize * 64)) // 1レコード64バイト想定
        
        for i := 0; i < b.N; i++ {
            result := processLargeDatasetOptimized(data)
            
            // 結果検証（正確性保証）
            if len(result) != dataSize {
                b.Fatalf("Result size mismatch: got %d, want %d", len(result), dataSize)
            }
        }
    })
    
    // 【シナリオ2】高頻度トランザクション処理
    b.Run("HighFrequencyTransactions", func(b *testing.B) {
        transactionCount := 1000000
        
        b.ReportAllocs()
        b.RunParallel(func(pb *testing.PB) {
            for pb.Next() {
                transactions := generateTransactionBatch(transactionCount)
                result := processTransactionsConcurrently(transactions)
                
                // トランザクション整合性チェック
                if !validateTransactionIntegrity(result) {
                    b.Error("Transaction integrity violation detected")
                }
            }
        })
    })
    
    // 【シナリオ3】リアルタイムAPI レスポンス
    b.Run("RealtimeAPIResponse", func(b *testing.B) {
        server := httptest.NewServer(createOptimizedAPIHandler())
        defer server.Close()
        
        client := &http.Client{
            Timeout: 100 * time.Millisecond, // SLA要件
        }
        
        b.ResetTimer()
        b.RunParallel(func(pb *testing.PB) {
            for pb.Next() {
                resp, err := client.Get(server.URL + "/api/v1/data")
                if err != nil {
                    b.Errorf("API request failed: %v", err)
                    continue
                }
                resp.Body.Close()
                
                // レスポンス時間SLAチェック
                if resp.Header.Get("X-Response-Time") > "50ms" {
                    b.Error("SLA violation: response time > 50ms")
                }
            }
        })
    })
}
```

ベンチマークテストは、コードの性能を定量的に測定するためのテスト手法です。実行時間やメモリ使用量を測定し、異なる実装の性能を比較したり、最適化の効果を検証したりします：

### 基本的なベンチマーク

ベンチマーク関数は`Benchmark`で始まり、`*testing.B`パラメーターを受け取ります：

```go
func BenchmarkFunction(b *testing.B) {
    // セットアップ（測定対象外）
    data := prepareTestData()
    
    b.ResetTimer() // タイマーをリセット
    
    for i := 0; i < b.N; i++ {
        // 測定対象の処理
        result := YourFunction(data)
        _ = result // コンパイラ最適化を防ぐ
    }
}
```

`b.N`は、Goのベンチマークフレームワークが自動的に調整する反復回数です。十分な精度が得られるまで自動的に増加します。

### メモリベンチマーク

`b.ReportAllocs()`を使用してメモリアロケーションを測定します：

```go
func BenchmarkStringConcatenation(b *testing.B) {
    b.ReportAllocs() // メモリアロケーションを報告
    
    for i := 0; i < b.N; i++ {
        var result string
        for j := 0; j < 100; j++ {
            result += "hello"
        }
        _ = result
    }
}

func BenchmarkStringBuilder(b *testing.B) {
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        var builder strings.Builder
        builder.Grow(500) // 事前にキャパシティを確保
        for j := 0; j < 100; j++ {
            builder.WriteString("hello")
        }
        _ = builder.String()
    }
}
```

実行結果：
```bash
BenchmarkStringConcatenation-8    10000    150000 ns/op    350000 B/op    99 allocs/op
BenchmarkStringBuilder-8         100000     15000 ns/op      1024 B/op     2 allocs/op
```

### サブベンチマークとパラメーター化

複数のパラメーターでベンチマークを実行：

```go
func BenchmarkSortAlgorithms(b *testing.B) {
    sizes := []int{10, 100, 1000, 10000}
    algorithms := map[string]func([]int){
        "Sort":      sort.Ints,
        "BubbleSort": BubbleSort,
        "QuickSort":  QuickSort,
    }
    
    for name, sortFunc := range algorithms {
        for _, size := range sizes {
            b.Run(fmt.Sprintf("%s-%d", name, size), func(b *testing.B) {
                data := generateRandomData(size)
                b.ResetTimer()
                
                for i := 0; i < b.N; i++ {
                    testData := make([]int, len(data))
                    copy(testData, data)
                    sortFunc(testData)
                }
            })
        }
    }
}
```

### CPUプロファイリング

ベンチマークと組み合わせてCPUプロファイルを生成：

```bash
# CPUプロファイル生成
go test -bench=BenchmarkFunction -cpuprofile=cpu.prof

# プロファイル分析
go tool pprof cpu.prof
(pprof) top10
(pprof) list functionName
(pprof) web
```

```go
func BenchmarkComplexOperation(b *testing.B) {
    // 複雑な処理のベンチマーク
    data := generateLargeDataset()
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        result := ComplexOperation(data)
        _ = result
    }
}
```

### メモリプロファイリング

メモリ使用パターンの分析：

```bash
# メモリプロファイル生成
go test -bench=BenchmarkFunction -memprofile=mem.prof

# メモリプロファイル分析
go tool pprof mem.prof
(pprof) top10
(pprof) list functionName
```

### 並行処理のベンチマーク

`b.RunParallel()`を使用して並行処理の性能を測定：

```go
func BenchmarkConcurrentMap(b *testing.B) {
    m := make(map[int]int)
    var mu sync.RWMutex
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            key := rand.Intn(1000)
            
            mu.RLock()
            _ = m[key]
            mu.RUnlock()
        }
    })
}

func BenchmarkSyncMap(b *testing.B) {
    var m sync.Map
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            key := rand.Intn(1000)
            m.Load(key)
        }
    })
}
```

### データ構造の性能比較

異なるデータ構造の性能特性を比較：

```go
func BenchmarkDataStructures(b *testing.B) {
    sizes := []int{100, 1000, 10000}
    
    for _, size := range sizes {
        b.Run(fmt.Sprintf("Slice-%d", size), func(b *testing.B) {
            data := make([]int, size)
            b.ResetTimer()
            
            for i := 0; i < b.N; i++ {
                for j := 0; j < size; j++ {
                    data[j] = j
                }
            }
        })
        
        b.Run(fmt.Sprintf("Map-%d", size), func(b *testing.B) {
            data := make(map[int]int, size)
            b.ResetTimer()
            
            for i := 0; i < b.N; i++ {
                for j := 0; j < size; j++ {
                    data[j] = j
                }
            }
        })
    }
}
```

### I/O性能のベンチマーク

ファイル読み書きやネットワーク通信の性能測定：

```go
func BenchmarkFileIO(b *testing.B) {
    // 一時ファイル作成
    tmpfile, err := os.CreateTemp("", "benchmark")
    if err != nil {
        b.Fatal(err)
    }
    defer os.Remove(tmpfile.Name())
    defer tmpfile.Close()
    
    data := make([]byte, 1024)
    rand.Read(data)
    
    b.Run("Write", func(b *testing.B) {
        b.SetBytes(1024) // バイト/秒を計算
        
        for i := 0; i < b.N; i++ {
            tmpfile.Seek(0, 0)
            tmpfile.Write(data)
        }
    })
    
    b.Run("Read", func(b *testing.B) {
        b.SetBytes(1024)
        buffer := make([]byte, 1024)
        
        for i := 0; i < b.N; i++ {
            tmpfile.Seek(0, 0)
            tmpfile.Read(buffer)
        }
    })
}
```

### HTTP性能ベンチマーク

HTTPハンドラーの性能測定：

```go
func BenchmarkHTTPHandler(b *testing.B) {
    handler := http.HandlerFunc(YourHTTPHandler)
    
    b.Run("GET", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            req := httptest.NewRequest("GET", "/api/data", nil)
            rr := httptest.NewRecorder()
            handler.ServeHTTP(rr, req)
        }
    })
    
    b.Run("POST", func(b *testing.B) {
        body := strings.NewReader(`{"key": "value"}`)
        
        for i := 0; i < b.N; i++ {
            req := httptest.NewRequest("POST", "/api/data", body)
            req.Header.Set("Content-Type", "application/json")
            rr := httptest.NewRecorder()
            handler.ServeHTTP(rr, req)
            
            // bodyをリセット
            body.Seek(0, 0)
        }
    })
}
```

### エスケープ分析

コンパイラのエスケープ分析を活用した最適化：

```go
// ヒープアロケーション版
func createUser() *User {
    return &User{Name: "John"} // ヒープに割り当て
}

// スタックアロケーション版  
func createUserValue() User {
    return User{Name: "John"} // スタックに割り当て
}

func BenchmarkUserCreation(b *testing.B) {
    b.Run("Heap", func(b *testing.B) {
        b.ReportAllocs()
        for i := 0; i < b.N; i++ {
            user := createUser()
            _ = user
        }
    })
    
    b.Run("Stack", func(b *testing.B) {
        b.ReportAllocs()
        for i := 0; i < b.N; i++ {
            user := createUserValue()
            _ = user
        }
    })
}
```

### ベンチマーク最適化のテクニック

```go
func BenchmarkOptimized(b *testing.B) {
    // 1. 事前準備をタイマー外で行う
    data := prepareData()
    
    // 2. プールを使用してアロケーションを削減
    var pool = sync.Pool{
        New: func() interface{} {
            return make([]byte, 1024)
        },
    }
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        // 3. プールからバッファを取得
        buffer := pool.Get().([]byte)
        
        // 4. 処理実行
        result := processData(data, buffer)
        _ = result
        
        // 5. プールに返却
        pool.Put(buffer)
    }
}
```

### 継続的性能監視

ベンチマーク結果をファイルに保存して継続監視：

```bash
# ベースライン保存
go test -bench=. > baseline.txt

# 比較実行
go test -bench=. > current.txt
benchcmp baseline.txt current.txt
```

```go
// CI/CDでの性能回帰検出
func TestPerformanceRegression(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test in short mode")
    }
    
    start := time.Now()
    result := criticalFunction()
    duration := time.Since(start)
    
    // 性能回帰の閾値チェック
    maxDuration := 100 * time.Millisecond
    if duration > maxDuration {
        t.Errorf("Performance regression detected: %v > %v", duration, maxDuration)
    }
    
    _ = result
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **データ構造とアルゴリズム**
   - 複数のソートアルゴリズム（Bubble, Quick, Merge, Heap Sort）
   - 文字列処理（連結、ビルダー、バイト操作）
   - 検索アルゴリズム（線形、二分探索）

2. **並行処理**
   - ミューテックス vs チャネル
   - sync.Map vs 通常のMap
   - ワーカープール実装

3. **メモリ最適化**
   - オブジェクトプールの活用
   - スライスの事前割り当て
   - インターフェース vs 具象型

4. **I/O操作**
   - ファイル読み書き
   - JSON エンコード/デコード
   - HTTP レスポンス処理

## ✅ 期待される挙動

### ベンチマーク実行結果
```bash
BenchmarkSortAlgorithms/BubbleSort-100-8         20000     75000 ns/op
BenchmarkSortAlgorithms/QuickSort-100-8         100000     12000 ns/op  
BenchmarkSortAlgorithms/MergeSort-100-8          80000     15000 ns/op

BenchmarkStringOperations/Concatenation-8          1000   1500000 ns/op   500000 B/op   99 allocs/op
BenchmarkStringOperations/Builder-8                10000    150000 ns/op     1024 B/op    2 allocs/op

BenchmarkConcurrency/Mutex-8                     500000      3000 ns/op
BenchmarkConcurrency/Channel-8                   300000      4500 ns/op
```

### メモリプロファイル
```bash
go test -bench=BenchmarkMemoryIntensive -memprofile=mem.prof
go tool pprof mem.prof
(pprof) top10
```

### CPU使用率分析
```bash
go test -bench=BenchmarkCPUIntensive -cpuprofile=cpu.prof  
go tool pprof cpu.prof
(pprof) web
```

## 💡 ヒント

1. **testing.B**: ベンチマークテスト用インターフェース
2. **b.N**: フレームワークが調整する反復回数
3. **b.ResetTimer()**: 計測タイマーのリセット
4. **b.ReportAllocs()**: メモリアロケーション報告
5. **b.SetBytes()**: 処理バイト数の設定
6. **b.RunParallel()**: 並列ベンチマーク
7. **sync.Pool**: オブジェクトプールによる最適化
8. **runtime.GC()**: 明示的ガベージコレクション

### ベンチマーク実行オプション

```bash
# 基本実行
go test -bench=.

# メモリアロケーション表示
go test -bench=. -benchmem

# 実行時間指定
go test -bench=. -benchtime=10s

# 特定のベンチマーク実行
go test -bench=BenchmarkSort

# CPU使用率1に制限
go test -bench=. -cpu=1

# 結果をファイルに保存
go test -bench=. | tee benchmark.txt
```

### 最適化のガイドライン

1. **計測前の最適化は悪**：まず計測してボトルネックを特定
2. **プロファイルガイド最適化**：CPU/メモリプロファイルを活用
3. **マイクロベンチマークの限界**：実際の使用パターンでも検証
4. **継続的な監視**：性能回帰を防ぐ自動テスト

これらの実装により、Go言語での効果的な性能測定と最適化のスキルを身につけることができます。