# Day 01: Contextによるキャンセル伝播

## 学習目標
Goroutineのツリーにキャンセルのシグナルを正しく伝播させる方法を理解し、実装する。

## 課題説明

親のGoroutineから子のGoroutineに対してキャンセルシグナルを送り、すべての子Goroutineが適切に停止することを確認するプログラムを実装してください。

### 要件

1. **親Goroutine**: 複数の子Goroutineを起動し、一定時間後にキャンセルシグナルを送信
2. **子Goroutine**: 親からのキャンセルシグナルを受け取り、作業を停止
3. **リソースリーク防止**: すべてのGoroutineが適切に終了すること
4. **タイムアウト処理**: 指定時間内にすべての処理が完了すること

### 実装すべき関数

```go
// ProcessWithCancellation は複数のワーカーGoroutineを起動し、
// 指定時間後にキャンセルシグナルを送信して全ワーカーを停止させる
func ProcessWithCancellation(numWorkers int, workDuration time.Duration, cancelAfter time.Duration) error

// Worker は与えられたcontextをチェックして作業を行う
// キャンセルシグナルを受け取ったら即座に停止する
func Worker(ctx context.Context, id int, results chan<- WorkResult) error
```

## ヒント

1. `context.WithCancel()`を使ってキャンセル可能なコンテキストを作成
2. `context.Done()`チャネルを使ってキャンセルシグナルを検知
3. `sync.WaitGroup`を使ってすべてのGoroutineの完了を待機
4. `select`文を使ってキャンセルシグナルと通常の処理を並行して監視

## スコアカード

- ✅ 基本実装: キャンセルシグナルが正しく伝播される
- ✅ エラーハンドリング: 適切なエラーメッセージが返される  
- ✅ リソース管理: Goroutineリークが発生しない
- ✅ パフォーマンス: 不要な遅延なくキャンセルが実行される

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
```

## 参考資料

- [Go Context Package](https://pkg.go.dev/context)
- [Go Concurrency Patterns: Context](https://blog.golang.org/context)