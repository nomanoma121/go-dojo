# Day 15: Generatorパターン

## 学習目標
チャネルを使い、値を逐次生成するジェネレータ関数を実装し、遅延評価とメモリ効率の良いデータ処理を理解する。

## 課題説明

大量のデータを一度にメモリに読み込むのではなく、必要に応じて値を生成するGeneratorパターンを実装してください。このパターンは、無限シーケンスや大きなデータセットを効率的に処理するために重要です。

### 要件

1. **遅延評価**: 値は要求されたときに生成される
2. **メモリ効率**: 大量のデータを一度にメモリに保持しない
3. **組み合わせ可能**: 複数のジェネレータを組み合わせられる
4. **キャンセレーション**: コンテキストによる中断をサポート
5. **型安全**: ジェネリクスを使用した型安全な実装

### 実装すべき構造体と関数

```go
// Generator represents a generator that produces values of type T
type Generator[T any] struct {
    ch     <-chan T
    cancel context.CancelFunc
}

// GeneratorFunc is a function that generates values
type GeneratorFunc[T any] func(ctx context.Context, yield func(T) bool)

// Transformer transforms one generator into another
type Transformer[T, U any] func(Generator[T]) Generator[U]
```

## 実装例

```go
// Range generates numbers from start to end
func Range(start, end int) Generator[int] {
    return NewGenerator(func(ctx context.Context, yield func(int) bool) {
        for i := start; i <= end; i++ {
            if !yield(i) {
                return
            }
        }
    })
}

// Map transforms each value using the provided function
func Map[T, U any](gen Generator[T], fn func(T) U) Generator[U] {
    return NewGenerator(func(ctx context.Context, yield func(U) bool) {
        for value := range gen.ch {
            if !yield(fn(value)) {
                return
            }
        }
    })
}
```

## ヒント

1. チャネルを使用してジェネレータの値を伝達
2. `context.Context`を使用してキャンセレーションを実装
3. 関数型プログラミングのパターン（Map、Filter、Reduce）を実装
4. 遅延評価のためにGoroutineを適切に管理

## スコアカード

- ✅ 基本実装: ジェネレータが値を遅延生成する
- ✅ 変換操作: Map、Filter、Takeなどの操作が動作する
- ✅ 組み合わせ: 複数のジェネレータを組み合わせられる
- ✅ リソース管理: 適切にGoroutineとチャネルを管理する

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```

## 参考資料

- [Go Channels](https://go.dev/tour/concurrency/2)
- [Context Package](https://pkg.go.dev/context)
- [Generator Pattern](https://en.wikipedia.org/wiki/Generator_(computer_programming))