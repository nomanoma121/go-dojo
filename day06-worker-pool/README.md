# Day 06: Worker Poolパターン

## 学習目標
決まった数のGoroutineで大量のタスクを効率的に処理するWorker Poolパターンを実装し、並行処理の制御方法を理解する。

## 課題説明

固定数のワーカーGoroutineを起動し、チャネルを通じてタスクを分散処理するシステムを実装してください。システムリソースを効率的に使用し、大量のタスクを安定して処理できるようにします。

### 要件

1. **ワーカープール**: 固定数のワーカーGoroutineでタスクを処理
2. **タスクキュー**: バッファ付きチャネルでタスクをキューイング
3. **グレースフルシャットダウン**: 進行中のタスクを完了してから停止
4. **負荷制御**: システムリソースの過負荷を防ぐ

### 実装すべき構造体と関数

```go
// Task represents a unit of work to be processed
type Task struct {
    ID       int
    Data     interface{}
    Priority int
}

// WorkerPool manages a fixed number of worker goroutines
type WorkerPool struct {
    numWorkers int
    taskQueue  chan Task
    quit       chan bool
    wg         sync.WaitGroup
}

// Result represents the result of processing a task
type Result struct {
    TaskID int
    Output interface{}
    Error  error
}
```

## ヒント

1. `make(chan Task, bufferSize)`でバッファ付きチャネルを作成
2. ワーカーは無限ループでタスクを待機し処理
3. `sync.WaitGroup`ですべてのワーカーの完了を待機
4. `select`文でシャットダウンシグナルとタスクを監視
5. チャネルのクローズでワーカーに停止を通知

## スコアカード

- ✅ 基本実装: ワーカープールが正しく動作し、タスクが並列処理される
- ✅ リソース制御: 指定された数のワーカーでリソース使用量が制限される
- ✅ グレースフルシャットダウン: 進行中のタスクが完了してから停止
- ✅ パフォーマンス: 適切なスループットとレイテンシが達成される

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```

## 参考資料

- [Go Concurrency Patterns: Worker Pool](https://gobyexample.com/worker-pools)
- [Effective Go: Concurrency](https://golang.org/doc/effective_go#concurrency)