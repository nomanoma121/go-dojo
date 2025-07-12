# Day 12: Future / Promiseパターン

## 学習目標
非同期処理の結果を後から受け取れるFutureパターンを実装し、並行プログラミングの抽象化を理解する。

## 課題説明

JavaScriptのPromiseやJavaのFutureのように、非同期処理の結果を表現するパターンを実装してください。処理の開始と結果の取得を分離することで、より柔軟な並行プログラミングが可能になります。

### 要件

1. **非同期実行**: タスクをバックグラウンドで実行
2. **結果の遅延取得**: 処理完了を待って結果を取得
3. **エラーハンドリング**: 非同期処理中のエラーを適切に処理
4. **タイムアウト**: 結果取得時のタイムアウト機能
5. **チェイニング**: 複数のFutureを連鎖させる機能

### 実装すべき構造体と関数

```go
// Future represents a future result of an asynchronous operation
type Future[T any] struct {
    result chan Result[T]
    done   chan struct{}
}

// Result represents the result of an operation (success or error)
type Result[T any] struct {
    Value T
    Error error
}

// Promise allows setting the result of a Future
type Promise[T any] struct {
    future *Future[T]
    once   sync.Once
}
```

## ヒント

1. ジェネリクスを使用して型安全なFutureを実装
2. `sync.Once`を使用して結果の設定を一度だけに制限
3. `context`を使用してタイムアウトやキャンセレーションを実装
4. チャネルを使用して結果の非同期通信を実現

## スコアカード

- ✅ 基本実装: Future/Promiseパターンが動作する
- ✅ エラーハンドリング: エラーが適切に伝播される
- ✅ タイムアウト: 結果取得でタイムアウトが機能する
- ✅ チェイニング: 複数のFutureを連鎖できる

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```

## 参考資料

- [Go Generics](https://go.dev/doc/tutorial/generics)
- [sync.Once Documentation](https://pkg.go.dev/sync#Once)