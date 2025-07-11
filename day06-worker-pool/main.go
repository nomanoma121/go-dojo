package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents a unit of work to be processed
type Task struct {
	ID       int
	Data     interface{}
	Priority int
	Created  time.Time
}

// Result represents the result of processing a task
type Result struct {
	TaskID    int
	Output    interface{}
	Error     error
	Duration  time.Duration
	WorkerID  int
}

// WorkerPool manages a fixed number of worker goroutines
type WorkerPool struct {
	numWorkers int
	taskQueue  chan Task
	resultChan chan Result
	quit       chan struct{}
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWorkerPool creates a new WorkerPool
func NewWorkerPool(numWorkers int, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WorkerPool{
		numWorkers: numWorkers,
		taskQueue:  make(chan Task, queueSize),
		resultChan: make(chan Result, queueSize),
		quit:       make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. numWorkers分のワーカーGoroutineを起動
	// 2. 各ワーカーに固有のIDを付与
	// 3. WaitGroupに追加
	// 4. worker関数を呼び出し
}

// worker is the main worker function that processes tasks
func (wp *WorkerPool) worker(workerID int) {
	defer wp.wg.Done()
	
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 無限ループでタスクを待機
	// 2. selectでタスクとシャットダウンシグナルを監視
	// 3. タスクを受信したら処理を実行
	// 4. 結果をresultChanに送信
	// 5. コンテキストがキャンセルされたら終了
}

// processTask processes a single task
func (wp *WorkerPool) processTask(task Task, workerID int) Result {
	start := time.Now()
	
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. タスクの種類に応じて処理を実行
	// 2. 処理時間を測定
	// 3. エラーハンドリング
	// 4. Resultを作成して返す
	
	return Result{
		TaskID:   task.ID,
		Output:   nil,
		Error:    nil,
		Duration: time.Since(start),
		WorkerID: workerID,
	}
}

// SubmitTask submits a task to the worker pool
func (wp *WorkerPool) SubmitTask(task Task) error {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. コンテキストがキャンセルされていないかチェック
	// 2. selectでタスクキューへの送信とタイムアウトを監視
	// 3. キューが満杯の場合のハンドリング
	
	return nil
}

// GetResult gets a result from the result channel
func (wp *WorkerPool) GetResult() (Result, bool) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. selectでresultChanからの受信を監視
	// 2. 結果が利用可能かどうかを示すbool値も返す
	// 3. ノンブロッキングで実装
	
	return Result{}, false
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. taskQueueをクローズ
	// 2. コンテキストをキャンセル
	// 3. すべてのワーカーの完了を待機
	// 4. resultChanをクローズ
}

// WaitForCompletion waits for all submitted tasks to complete
func (wp *WorkerPool) WaitForCompletion() {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. taskQueueが空になるまで待機
	// 2. すべてのワーカーがアイドル状態になるまで待機
}

// GetStats returns statistics about the worker pool
type PoolStats struct {
	NumWorkers    int
	QueueSize     int
	QueueLength   int
	TasksComplete int64
	TasksError    int64
}

func (wp *WorkerPool) GetStats() PoolStats {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 現在のキューの長さを取得
	// 2. 完了したタスク数を取得
	// 3. エラーになったタスク数を取得
	
	return PoolStats{
		NumWorkers:  wp.numWorkers,
		QueueSize:   cap(wp.taskQueue),
		QueueLength: len(wp.taskQueue),
	}
}

// TaskProcessor defines the interface for processing different types of tasks
type TaskProcessor interface {
	Process(data interface{}) (interface{}, error)
}

// SimpleTaskProcessor implements basic task processing
type SimpleTaskProcessor struct{}

func (stp *SimpleTaskProcessor) Process(data interface{}) (interface{}, error) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. データの型をチェック
	// 2. 簡単な処理を実行（例：文字列の変換、数値の計算）
	// 3. 処理時間をシミュレート
	
	return nil, nil
}

// HeavyTaskProcessor implements CPU-intensive task processing
type HeavyTaskProcessor struct{}

func (htp *HeavyTaskProcessor) Process(data interface{}) (interface{}, error) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 重い計算処理をシミュレート
	// 2. time.Sleepで処理時間をシミュレート
	// 3. 途中でコンテキストキャンセルのチェック
	
	return nil, nil
}

// BatchProcessor processes multiple tasks as a batch
type BatchProcessor struct {
	batchSize int
	tasks     []Task
	mu        sync.Mutex
}

func NewBatchProcessor(batchSize int) *BatchProcessor {
	return &BatchProcessor{
		batchSize: batchSize,
		tasks:     make([]Task, 0, batchSize),
	}
}

func (bp *BatchProcessor) AddTask(task Task) bool {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. ミューテックスでロック
	// 2. tasksスライスにタスクを追加
	// 3. バッチサイズに達したかチェック
	
	return false
}

func (bp *BatchProcessor) ProcessBatch() []Result {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 現在のバッチを取得
	// 2. バッチ内のすべてのタスクを処理
	// 3. 結果をまとめて返す
	
	return nil
}

func main() {
	// テスト用のサンプル実行
	pool := NewWorkerPool(3, 10)
	pool.Start()
	
	// テストタスクを送信
	for i := 0; i < 5; i++ {
		task := Task{
			ID:      i,
			Data:    fmt.Sprintf("task-%d", i),
			Created: time.Now(),
		}
		
		err := pool.SubmitTask(task)
		if err != nil {
			fmt.Printf("Failed to submit task %d: %v\n", i, err)
		}
	}
	
	// 結果を受信
	time.Sleep(time.Second) // 処理完了を待機
	
	for i := 0; i < 5; i++ {
		if result, ok := pool.GetResult(); ok {
			fmt.Printf("Result: TaskID=%d, WorkerID=%d, Duration=%v\n", 
				result.TaskID, result.WorkerID, result.Duration)
		}
	}
	
	pool.Stop()
}