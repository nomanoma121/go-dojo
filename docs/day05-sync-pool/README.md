# Day 05: sync.Poolによるオブジェクト再利用

## 🎯 本日の目標 (Today's Goal)

このチャレンジを通して、以下のスキルを身につけることができます：

- **sync.Poolを使ってオブジェクトの再利用システムを実装できるようになる**
- **ガベージコレクション（GC）の負荷を軽減する方法を理解できるようになる**
- **メモリアロケーションのパフォーマンス最適化技術を習得できるようになる**
- **高頻度で作成・破棄されるオブジェクトの効率的な管理方法をマスターする**

## 📖 解説 (Explanation)

### なぜsync.Poolが必要なのか？

高パフォーマンスなGoアプリケーションでは、以下のような場面で大量のオブジェクトが短時間で作成・破棄されます：

- HTTPリクエスト処理でのレスポンスバッファ
- JSON/XMLの パース処理での一時的な構造体
- 画像処理でのピクセルデータスライス
- ログ出力での文字列フォーマット処理

```go
// ❌ 【問題のある例】：大量のオブジェクト作成 - パフォーマンス問題の原因
func processRequests() {
    for i := 0; i < 10000; i++ {
        // 【致命的問題】毎回新しいスライスを作成
        buffer := make([]byte, 1024)
        
        // 【問題点の詳細】：
        // 1. make()呼び出し: ヒープメモリアロケーション（コストが高い）
        // 2. 10,000回 × 1KB = 10MB のメモリ確保が繰り返される
        // 3. 各bufferは短時間で不要になる（短命オブジェクト）
        // 4. GCが頻繁に発生し、アプリケーション全体が一時停止
        
        // 処理...（実際のビジネスロジック）
        processData(buffer)
        
        // 【問題】bufferは使用後に即座にGCの対象になる
        // スコープを抜けると参照が失われ、ガベージコレクションの対象となる
        // 結果：メモリアロケーション→使用→GC のサイクルが高頻度で発生
    }
    
    // 【パフォーマンス影響】：
    // - アロケーション時間: 10,000 × ~100ns = ~1ms
    // - GC停止時間: 数ms～数十ms（アプリケーション全体が停止）
    // - 総合的なスループット低下: 20-50%
}
```

この場合、以下の問題が発生します：

1. **メモリアロケーションのオーバーヘッド**: `make()`や`new()`の呼び出しコスト
2. **ガベージコレクションの負荷**: 大量の短命オブジェクトがGCを頻発させる
3. **メモリフラグメンテーション**: 細かいオブジェクトの断片化

### sync.Poolの基本概念

`sync.Pool`は**一時的なオブジェクトの再利用**を可能にする仕組みです：

```go
import "sync"

// 【正しい実装】sync.Poolによる効率的なオブジェクト再利用
var bufferPool = sync.Pool{
    // 【New関数】プールが空の時に新しいオブジェクトを作成
    New: func() interface{} {
        // 【重要】この関数は以下の場合にのみ呼ばれる：
        // 1. プールが完全に空の場合
        // 2. GC後にプール内容がクリアされた場合
        // 3. 並行性が高く、既存オブジェクトがすべて使用中の場合
        return make([]byte, 1024)
    },
}

func processWithPool() {
    // 【Step 1】プールからオブジェクトを取得
    // Get()の内部動作：
    // 1. 現在のGoroutineのローカルプールをチェック
    // 2. ローカルが空なら他のプールから「盗取」
    // 3. 全体が空ならNew()で新規作成
    buffer := bufferPool.Get().([]byte)
    
    // 【重要】型アサーション([]byte)が必要
    // sync.Poolはinterface{}を返すため
    
    // 【Step 2】オブジェクトを使用
    // この時点でbufferは完全に再利用可能な状態
    processData(buffer)
    
    // 【Step 3】プールに戻す（最重要：状態をリセット）
    buffer = buffer[:0]  // スライスの長さをリセット（容量は保持）
    bufferPool.Put(buffer)
    
    // 【注意】Put()後は絶対にbufferを使用してはいけない
    // 他のGoroutineが同じオブジェクトを取得する可能性がある
}

// 【使用パターンの比較】：
// 従来方式: make() → 使用 → GC
// Pool方式: Get() → 使用 → Put() → 再利用
//
// 【パフォーマンス向上効果】：
// - アロケーション削減: 90-99%
// - GC圧力軽減: 50-80%
// - 全体スループット向上: 20-100%（ワークロード依存）
```

**sync.Poolの特徴：**
- オブジェクトの作成コストを削減
- GCの負荷を軽減
- スレッドセーフな実装
- GC時にプール内容が自動的にクリアされる

### bytes.Bufferプールの実装例

最も一般的な使用例の一つがbytes.Bufferのプーリングです：

```go
type BufferPool struct {
    pool sync.Pool
}

func NewBufferPool() *BufferPool {
    return &BufferPool{
        pool: sync.Pool{
            New: func() interface{} {
                return &bytes.Buffer{}
            },
        },
    }
}

func (bp *BufferPool) Get() *bytes.Buffer {
    return bp.pool.Get().(*bytes.Buffer)
}

func (bp *BufferPool) Put(buf *bytes.Buffer) {
    buf.Reset() // 内容をクリア
    bp.pool.Put(buf)
}

// 使用例
func formatMessage(data string) string {
    buf := bufferPool.Get()
    defer bufferPool.Put(buf)
    
    buf.WriteString("Message: ")
    buf.WriteString(data)
    buf.WriteString("\n")
    
    return buf.String()
}
```

### 構造体プールの実装

重い構造体もプールで管理できます：

```go
type WorkerData struct {
    ID       int
    Payload  []byte
    Metadata map[string]string
    Results  []string
}

type WorkerDataPool struct {
    pool sync.Pool
}

func NewWorkerDataPool() *WorkerDataPool {
    return &WorkerDataPool{
        pool: sync.Pool{
            New: func() interface{} {
                return &WorkerData{
                    Payload:  make([]byte, 0, 1024),
                    Metadata: make(map[string]string),
                    Results:  make([]string, 0, 10),
                }
            },
        },
    }
}

func (wdp *WorkerDataPool) Get() *WorkerData {
    return wdp.pool.Get().(*WorkerData)
}

func (wdp *WorkerDataPool) Put(wd *WorkerData) {
    // 状態をリセット
    wd.ID = 0
    wd.Payload = wd.Payload[:0]
    
    // マップをクリア
    for k := range wd.Metadata {
        delete(wd.Metadata, k)
    }
    
    wd.Results = wd.Results[:0]
    
    wdp.pool.Put(wd)
}
```

### 可変サイズスライスプールの実装

異なるサイズのスライスを効率的に管理：

```go
type SlicePool struct {
    pools map[int]*sync.Pool // capacity -> pool
    mu    sync.RWMutex
}

func NewSlicePool() *SlicePool {
    return &SlicePool{
        pools: make(map[int]*sync.Pool),
    }
}

func (sp *SlicePool) GetSlice(capacity int) []byte {
    // 2の累乗に丸める（メモリ効率のため）
    roundedCap := roundUpToPowerOf2(capacity)
    
    sp.mu.RLock()
    pool, exists := sp.pools[roundedCap]
    sp.mu.RUnlock()
    
    if !exists {
        sp.mu.Lock()
        // ダブルチェック
        if pool, exists = sp.pools[roundedCap]; !exists {
            pool = &sync.Pool{
                New: func() interface{} {
                    return make([]byte, roundedCap)
                },
            }
            sp.pools[roundedCap] = pool
        }
        sp.mu.Unlock()
    }
    
    slice := pool.Get().([]byte)
    return slice[:capacity] // 要求されたサイズに調整
}

func (sp *SlicePool) PutSlice(slice []byte) {
    capacity := cap(slice)
    roundedCap := roundUpToPowerOf2(capacity)
    
    sp.mu.RLock()
    pool, exists := sp.pools[roundedCap]
    sp.mu.RUnlock()
    
    if exists {
        slice = slice[:cap(slice)] // 容量まで復元
        pool.Put(slice)
    }
}

func roundUpToPowerOf2(n int) int {
    if n <= 0 {
        return 1
    }
    n--
    n |= n >> 1
    n |= n >> 2
    n |= n >> 4
    n |= n >> 8
    n |= n >> 16
    n++
    return n
}
```

### パフォーマンス測定の例

プールの効果を測定する方法：

```go
func BenchmarkWithoutPool(b *testing.B) {
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        buffer := &bytes.Buffer{}
        buffer.WriteString("Hello, World!")
        _ = buffer.String()
    }
}

func BenchmarkWithPool(b *testing.B) {
    pool := NewBufferPool()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        buffer := pool.Get()
        buffer.WriteString("Hello, World!")
        _ = buffer.String()
        pool.Put(buffer)
    }
}
```

典型的な結果：
```
BenchmarkWithoutPool-8   5000000   300 ns/op   32 B/op   2 allocs/op
BenchmarkWithPool-8     10000000   150 ns/op    0 B/op   0 allocs/op
```

### 注意すべきポイント

1. **状態のリセット**: プールに戻す前に必ずオブジェクトの状態をリセット
2. **型アサーション**: `Get()`の戻り値は`interface{}`なので型アサーションが必要
3. **GCタイミング**: GC実行時にプール内容が削除される可能性
4. **適切なサイズ設計**: 大きすぎるオブジェクトはプールの利点を相殺

### 高度な使用例：ワーカープールとの組み合わせ

```go
type TaskProcessor struct {
    bufferPool *BufferPool
    workerPool *WorkerDataPool
}

func (tp *TaskProcessor) ProcessTask(task Task) Result {
    // プールからリソースを取得
    buffer := tp.bufferPool.Get()
    workerData := tp.workerPool.Get()
    
    defer func() {
        // 必ずプールに戻す
        tp.bufferPool.Put(buffer)
        tp.workerPool.Put(workerData)
    }()
    
    // 処理を実行
    return tp.executeTask(task, buffer, workerData)
}
```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **`NewBufferPool()`**: bytes.Bufferのプールを初期化する
2. **`(bp *BufferPool) Get() *bytes.Buffer`**: バッファをプールから取得する
3. **`(bp *BufferPool) Put(buf *bytes.Buffer)`**: バッファをプールに戻す
4. **`NewWorkerDataPool()`**: WorkerDataのプールを初期化する
5. **`(wdp *WorkerDataPool) Get() *WorkerData`**: WorkerDataをプールから取得する
6. **`(wdp *WorkerDataPool) Put(wd *WorkerData)`**: WorkerDataをプールに戻す
7. **`NewSlicePool()`**: 可変サイズスライスプールを初期化する
8. **`(sp *SlicePool) GetSlice(size int) []byte`**: 指定サイズのスライスを取得する
9. **`(sp *SlicePool) PutSlice(slice []byte)`**: スライスをプールに戻す

**重要な実装要件：**
- プールからのオブジェクト取得・返却が正しく動作すること
- オブジェクトの状態が適切にリセットされること
- 複数のGoroutineから安全にアクセスできること
- メモリアロケーションが大幅に削減されること
- ベンチマークでプール使用時の性能向上が確認できること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestBufferPool
=== RUN   TestBufferPool/Basic_operations
=== RUN   TestBufferPool/Concurrent_access
=== RUN   TestBufferPool/State_reset
--- PASS: TestBufferPool (0.10s)
=== RUN   TestWorkerDataPool
=== RUN   TestWorkerDataPool/Basic_operations
=== RUN   TestWorkerDataPool/State_reset
--- PASS: TestWorkerDataPool (0.05s)
=== RUN   TestSlicePool
=== RUN   TestSlicePool/Various_sizes
=== RUN   TestSlicePool/Concurrent_access
--- PASS: TestSlicePool (0.08s)
PASS
```

### ベンチマーク実行例
```bash
$ go test -bench=. -benchmem
BenchmarkBufferWithoutPool-8    	2000000	   800 ns/op	  64 B/op	   2 allocs/op
BenchmarkBufferWithPool-8       	5000000	   300 ns/op	   0 B/op	   0 allocs/op
BenchmarkWorkerDataWithoutPool-8	1000000	  1500 ns/op	 256 B/op	   4 allocs/op
BenchmarkWorkerDataWithPool-8   	3000000	   400 ns/op	   0 B/op	   0 allocs/op
BenchmarkSliceWithoutPool-8     	3000000	   500 ns/op	1024 B/op	   1 allocs/op
BenchmarkSliceWithPool-8        	8000000	   200 ns/op	   0 B/op	   0 allocs/op
```
プール使用時にメモリアロケーションが0になり、大幅な性能向上が確認できます。

### プログラム実行例
```bash
$ go run main.go
=== sync.Pool Object Reuse Demo ===

1. Buffer Pool Test:
Processing 1000 requests...
Without pool: 1000 buffers allocated
With pool: 50 buffers allocated (95% reduction!)

2. Worker Data Pool Test:
Processing 500 tasks...
Memory usage without pool: 2.5 MB
Memory usage with pool: 0.3 MB (88% reduction!)

3. Slice Pool Test:
Requesting various slice sizes...
Size 1024: reused from pool
Size 2048: reused from pool  
Size 512: reused from pool
Pool efficiency: 90% reuse rate

Performance improvement: 2.5x faster with pools!
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的なプール実装パターン
```go
type BufferPool struct {
    pool sync.Pool
}

func NewBufferPool() *BufferPool {
    return &BufferPool{
        pool: sync.Pool{
            New: func() interface{} {
                return &bytes.Buffer{}
            },
        },
    }
}

func (bp *BufferPool) Get() *bytes.Buffer {
    return bp.pool.Get().(*bytes.Buffer)
}

func (bp *BufferPool) Put(buf *bytes.Buffer) {
    buf.Reset() // 重要：状態をリセット
    bp.pool.Put(buf)
}
```

### 構造体の状態リセット
```go
func (wdp *WorkerDataPool) Put(wd *WorkerData) {
    // プリミティブ型のリセット
    wd.ID = 0
    
    // スライスの長さリセット（容量は保持）
    wd.Payload = wd.Payload[:0]
    wd.Results = wd.Results[:0]
    
    // マップのクリア
    for k := range wd.Metadata {
        delete(wd.Metadata, k)
    }
    
    wdp.pool.Put(wd)
}
```

### スライスサイズの最適化
```go
func (sp *SlicePool) GetSlice(size int) []byte {
    // 効率的なサイズに丸める
    poolSize := nextPowerOf2(size)
    
    // 対応するプールから取得
    slice := sp.getPoolForSize(poolSize).Get().([]byte)
    
    // 要求されたサイズに調整
    return slice[:size]
}
```

### 使用する主要なパッケージ
- `sync.Pool` - オブジェクトプール
- `bytes.Buffer` - バッファプールでの使用
- `sync.RWMutex` - 複数プール管理での排他制御

### デバッグのコツ
1. `go test -bench=. -benchmem`でメモリアロケーションを確認
2. プールのNew関数が適切に設定されているかチェック
3. Put時の状態リセットが完全に行われているか確認
4. 型アサーションでpanicが発生していないか確認

### よくある間違い
- Put時の状態リセット忘れ → 前回の状態が残ってしまう
- New関数の設定忘れ → プールが空の時にpanicが発生
- 型アサーション忘れ → `interface{}`のまま使用してしまう
- 大きすぎるオブジェクトのプール化 → メモリ効率が悪化

### パフォーマンス測定のコツ
```go
func BenchmarkWithPool(b *testing.B) {
    pool := NewBufferPool()
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            buf := pool.Get()
            // 処理...
            pool.Put(buf)
        }
    })
}
```

## 実行方法

```bash
# テスト実行
go test -v

# ベンチマーク測定（メモリ使用量込み）
go test -bench=. -benchmem

# 長時間ベンチマーク（より正確な測定）
go test -bench=. -benchtime=10s

# プログラム実行
go run main.go
```

## 参考資料

- [Go sync.Pool](https://pkg.go.dev/sync#Pool)
- [Pool Performance Tips](https://golang.org/doc/gc_guide#Pool)
- [Go GC Guide](https://go.dev/doc/gc-guide)
- [Memory Management Best Practices](https://golang.org/doc/gc_guide)