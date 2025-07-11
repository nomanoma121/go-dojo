# Day 07: Worker Pool (結果の受信)

## 学習目標
各ワーカーからの処理結果を安全に収集し、結果の順序保証や集約処理を効率的に実装する方法を理解する。

## 課題説明

Worker Poolパターンを拡張し、処理結果の効率的な収集、順序保証、集約処理を実装してください。大量の並列処理結果を適切に管理し、呼び出し元に返すシステムを構築します。

### 要件

1. **結果収集**: 並列処理の結果を効率的に収集
2. **順序保証**: 必要に応じてタスクの投入順序で結果を返す
3. **結果集約**: 複数の結果をまとめて一つの結果にする
4. **エラーハンドリング**: 部分的な失敗を適切に処理

### 実装すべき構造体と関数

```go
// ResultCollector collects and manages results from workers
type ResultCollector struct {
    results     map[int]Result
    resultChan  chan Result
    orderedMode bool
    mu          sync.RWMutex
}

// AggregatedResult represents aggregated results from multiple tasks
type AggregatedResult struct {
    TotalTasks    int
    SuccessCount  int
    ErrorCount    int
    Results       []Result
    AggregateData interface{}
}
```

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```