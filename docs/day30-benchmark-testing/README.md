# Day 30: ベンチマークテスト

🎯 **本日の目標**
Go言語のベンチマークテスト機能を使用して、関数やアルゴリズムの性能を正確に測定し、メモリ使用量の分析、CPUプロファイリング、性能最適化の手法を習得できるようになる。

## 📖 解説

### ベンチマークテストとは

ベンチマークテストは、コードの性能を定量的に測定するためのテスト手法です。実行時間やメモリ使用量を測定し、異なる実装の性能を比較したり、最適化の効果を検証したりします：

```go
func BenchmarkStringBuilder(b *testing.B) {
    for i := 0; i < b.N; i++ {
        var builder strings.Builder
        for j := 0; j < 100; j++ {
            builder.WriteString("hello")
        }
        _ = builder.String()
    }
}
```

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