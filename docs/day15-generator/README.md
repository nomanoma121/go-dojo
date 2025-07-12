# Day 15: Generatorパターン

## 🎯 本日の目標 (Today's Goal)

チャネルとジェネリクスを使った遅延評価のGeneratorパターンを実装し、メモリ効率の良いデータ処理とストリーミング処理の概念を理解する。

## 📖 解説 (Explanation)

### Generatorパターンとは

Generatorパターンは、値を逐次生成して提供するデザインパターンです。大量のデータを一度にメモリに読み込むのではなく、必要に応じて値を生成することで、メモリ効率の良いプログラムを実現できます。

### 従来の処理との比較

**従来の一括処理:**
```go
// 大量のデータを一度にメモリに読み込む
func processRange(start, end int) []int {
    var results []int
    for i := start; i <= end; i++ {
        results = append(results, i*i) // 全てメモリに保持
    }
    return results
}

// 1000万個の要素を一度にメモリに保持
data := processRange(1, 10_000_000) // メモリ使用量が膨大
```

**Generatorパターン:**
```go
// 必要な時に値を生成
func squareGenerator(start, end int) Generator[int] {
    return NewGenerator(func(ctx context.Context, yield func(int) bool) {
        for i := start; i <= end; i++ {
            if !yield(i * i) { // 一つずつ生成
                return
            }
        }
    })
}

// メモリ使用量は常に一定
gen := squareGenerator(1, 10_000_000)
for value := range gen.Chan() {
    process(value) // 一つずつ処理
}
```

### Goでの実装アプローチ

Goでは、チャネルとGoroutineを使ってGeneratorパターンを実装します：

#### 1. 基本構造

```go
type Generator[T any] struct {
    ch     <-chan T
    cancel context.CancelFunc
    ctx    context.Context
}

type GeneratorFunc[T any] func(ctx context.Context, yield func(T) bool)
```

#### 2. 基本的なGenerator

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
    
    return Generator[T]{
        ch:     ch,
        cancel: cancel,
        ctx:    ctx,
    }
}
```

#### 3. 数値範囲Generator

```go
func Range(start, end int) Generator[int] {
    return NewGenerator(func(ctx context.Context, yield func(int) bool) {
        for i := start; i <= end; i++ {
            select {
            case <-ctx.Done():
                return
            default:
                if !yield(i) {
                    return
                }
            }
        }
    })
}
```

### 変換操作（Transformation）

Generatorパターンの強力な点は、関数型プログラミングの操作を組み合わせられることです：

#### Map変換
```go
func Map[T, U any](gen Generator[T], fn func(T) U) Generator[U] {
    return NewGenerator(func(ctx context.Context, yield func(U) bool) {
        for value := range gen.ch {
            select {
            case <-ctx.Done():
                return
            default:
                transformed := fn(value)
                if !yield(transformed) {
                    return
                }
            }
        }
    })
}

// 使用例
squares := Map(Range(1, 10), func(x int) int { return x * x })
```

#### Filter操作
```go
func Filter[T any](gen Generator[T], predicate func(T) bool) Generator[T] {
    return NewGenerator(func(ctx context.Context, yield func(T) bool) {
        for value := range gen.ch {
            if predicate(value) {
                if !yield(value) {
                    return
                }
            }
        }
    })
}

// 使用例
evens := Filter(Range(1, 20), func(x int) bool { return x%2 == 0 })
```

#### Take操作
```go
func Take[T any](gen Generator[T], n int) Generator[T] {
    return NewGenerator(func(ctx context.Context, yield func(T) bool) {
        count := 0
        for value := range gen.ch {
            if count >= n {
                return
            }
            if !yield(value) {
                return
            }
            count++
        }
    })
}

// 使用例：無限シーケンスから最初の10個を取得
firstTen := Take(Fibonacci(), 10)
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