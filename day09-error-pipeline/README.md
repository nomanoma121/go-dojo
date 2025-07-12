# Day 09: エラーハンドリング付きパイプライン

## 学習目標
並行処理パイプライン内で発生したエラーを適切に処理し、堅牢なデータ処理システムを構築する。

## 課題説明

データ処理パイプラインにおいて、各ステージで発生するエラーを適切にキャッチし、システム全体の安定性を保ちながらエラー情報を伝播させる仕組みを実装してください。

### 要件

1. **エラー伝播**: パイプラインの各ステージで発生したエラーを上位に伝播
2. **部分的失敗の処理**: 一部のデータが失敗しても他のデータの処理を継続
3. **エラー回復**: 一時的なエラーからの自動回復機能
4. **エラー集約**: 複数のエラーをまとめて報告する機能

### 実装すべき構造体と関数

```go
// ErrorPipeline represents a pipeline with error handling
type ErrorPipeline struct {
    stages     []PipelineStage
    errorChan  chan PipelineError
    ctx        context.Context
    cancel     context.CancelFunc
}

// PipelineStage represents a single stage in the pipeline
type PipelineStage func(context.Context, <-chan DataItem) <-chan DataItem

// PipelineError represents an error that occurred in the pipeline
type PipelineError struct {
    Stage     string
    Error     error
    Data      DataItem
    Timestamp time.Time
}
```

## ヒント

1. `errgroup`パッケージを使用してGoroutineのエラー管理を簡素化
2. `select`文でエラーチャネルとデータチャネルを並行監視
3. コンテキストを使ってエラー発生時の早期終了を実現
4. エラーログを構造化して詳細な情報を記録

## スコアカード

- ✅ 基本実装: エラーが適切に伝播される
- ✅ 継続処理: 部分的失敗でもパイプラインが継続する
- ✅ エラー情報: 詳細なエラー情報が提供される
- ✅ リソース管理: エラー時でもリソースリークが発生しない

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```

## 参考資料

- [Go Error Handling Best Practices](https://go.dev/blog/error-handling-and-go)
- [errgroup Package](https://pkg.go.dev/golang.org/x/sync/errgroup)