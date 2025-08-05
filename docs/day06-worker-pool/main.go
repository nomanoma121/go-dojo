//go:build ignore

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
	TaskID   int
	Output   interface{}
	Error    error
	Duration time.Duration
	WorkerID int
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
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
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

	for {
		select {
		case task, ok := <-wp.taskQueue:
			if !ok {
				return
			}

			result := wp.processTask(task, workerID)

			select {
			case wp.resultChan <- result:
			case <-wp.ctx.Done():
				return
			}

		case <-wp.ctx.Done():
			return
		}
	}
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

	// Simulate work based on task data
	var output interface{}
	var err error

	switch data := task.Data.(type) {
	case string:
		// String processing
		if data == "slow task" {
			time.Sleep(100 * time.Millisecond) // Simulate slow work
		} else {
			time.Sleep(50 * time.Millisecond) // Simulate work
		}
		output = "processed: " + data
	case int:
		// Number processings
		time.Sleep(100 * time.Millisecond) // Simulate work
		output = data * 2
	default:
		// Default processing
		time.Sleep(30 * time.Millisecond)
		output = fmt.Sprintf("processed_%v", data)
	}

	return Result{
		TaskID:   task.ID,
		Output:   output,
		Error:    err,
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

	select {
	case wp.taskQueue <- task:
		return nil
	case <-wp.ctx.Done():
		return wp.ctx.Err()
	default:
		// Queue is full, try with timeout
		select {
		case wp.taskQueue <- task:
			return nil
		case <-time.After(100 * time.Millisecond):
			return fmt.Errorf("task queue is full")
		case <-wp.ctx.Done():
			return wp.ctx.Err()
		}
	}
}

// GetResult gets a result from the result channel
func (wp *WorkerPool) GetResult() (Result, bool) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. selectでresultChanからの受信を監視
	// 2. 結果が利用可能かどうかを示すbool値も返す
	// 3. ノンブロッキングで実装
	select {
	case result, ok := <-wp.resultChan:
		return result, ok
	case <-time.After(1 * time.Second):
		return Result{}, false
	}
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

	close(wp.taskQueue)
	wp.cancel()
	wp.wg.Wait()
	close(wp.resultChan)
}

// WaitForCompletion waits for all submitted tasks to complete
func (wp *WorkerPool) WaitForCompletion() {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. taskQueueが空になるまで待機
	// 2. すべてのワーカーがアイドル状態になるまで待機
	// Wait for task queue to be empty
	for len(wp.taskQueue) > 0 {
		time.Sleep(10 * time.Millisecond)
	}

	// Wait a bit more for workers to finish current tasks
	time.Sleep(100 * time.Millisecond)
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
	switch v := data.(type) {
	case string:
		// Simple string transformation
		time.Sleep(10 * time.Millisecond)
		return "processed_" + v, nil
	case int:
		// Simple number calculation
		time.Sleep(5 * time.Millisecond)
		return v * 10, nil
	default:
		return fmt.Sprintf("unknown_type_%v", v), nil
	}
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
	// Simulate CPU-intensive work
	time.Sleep(200 * time.Millisecond)

	switch v := data.(type) {
	case int:
		// Heavy computation simulation
		result := 0
		for i := 0; i < v*1000; i++ {
			result += i
		}
		return result, nil
	default:
		return fmt.Sprintf("heavy_processed_%v", v), nil
	}
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
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.tasks = append(bp.tasks, task)
	if len(bp.tasks) >= bp.batchSize {
		return true
	}
	return false
}

func (bp *BatchProcessor) ProcessBatch() []Result {
	bp.mu.Lock()
	batch := make([]Task, len(bp.tasks))
	copy(batch, bp.tasks)
	bp.tasks = bp.tasks[:0] // Clear the batch
	bp.mu.Unlock()

	results := make([]Result, len(batch))
	start := time.Now()

	// Process all tasks in the batch
	for i, task := range batch {
		// Simulate batch processing (more efficient than individual)
		var output interface{}
		switch data := task.Data.(type) {
		case int:
			output = data * 5 // Batch processing multiplier
		default:
			output = fmt.Sprintf("batch_processed_%v", data)
		}

		results[i] = Result{
			TaskID:   task.ID,
			Output:   output,
			Duration: time.Since(start) / time.Duration(len(batch)), // Average time
		}
	}

	return results
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
