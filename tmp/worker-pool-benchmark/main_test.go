package main

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// --- テスト対象のWorkerPoolの最小実装 ---

// Task はワーカーに渡す仕事の単位
type Task struct {
	ID   int
	Data func() // 実行する処理
}

// WorkerPool の実装
type WorkerPool struct {
	numWorkers int
	taskQueue  chan Task
	quit       chan struct{}
	wg         sync.WaitGroup
}

// NewWorkerPool はコンストラクタ
func NewWorkerPool(numWorkers, queueSize int) *WorkerPool {
	return &WorkerPool{
		numWorkers: numWorkers,
		taskQueue:  make(chan Task, queueSize),
		quit:       make(chan struct{}),
	}
}

// Start はワーカーを起動
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// worker は実際の処理を行うgoroutine
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for {
		select {
		case task, ok := <-wp.taskQueue:
			if !ok {
				return // キューが閉じられたら終了
			}
			task.Data() // タスクを実行
		case <-wp.quit:
			return // シャットダウン命令
		}
	}
}

// SubmitTask はタスクを投入
func (wp *WorkerPool) SubmitTask(task Task) {
	wp.taskQueue <- task
}

// GracefulShutdown は安全に停止
func (wp *WorkerPool) GracefulShutdown(timeout time.Duration) error {
	close(wp.taskQueue)
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		close(wp.quit)
		return errors.New("shutdown timeout")
	}
}

// heavyComputation は重い処理をシミュレート
func heavyComputation() {
	time.Sleep(1 * time.Millisecond)
}

// --- ユーザー提供のテストコード ---

// パフォーマンス測定
type PoolMetrics struct {
	TasksProcessed int64
	TotalDuration  time.Duration
	ErrorCount     int64
	AvgLatency     time.Duration
	Throughput     float64
}

func BenchmarkWorkerPool(b *testing.B) {
	pool := NewWorkerPool(10, 100)
	pool.Start()
	// このベンチマークではGracefulShutdownは結果に影響しないため省略
	// defer pool.GracefulShutdown(5 * time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// ベンチマークでは実際の処理はさせないのが一般的
		task := Task{ID: i, Data: func() {}}
		pool.SubmitTask(task)
	}
}

// 現実的な負荷テスト
// 現実的な負荷テスト
func TestWorkerPoolThroughput(t *testing.T) {
    numTasks := 10000
    pool := NewWorkerPool(20, 200)
    pool.Start()

    start := time.Now()

    // ★修正点1: WaitGroupを追加
    var wg sync.WaitGroup
    wg.Add(1) // これから1つのgoroutineを待つことを宣言

    // タスクを投入
    go func() {
        // ★修正点2: 処理が終わったらDoneを呼ぶ
        defer wg.Done()
        for i := 0; i < numTasks; i++ {
            task := Task{ID: i, Data: heavyComputation}
            pool.SubmitTask(task)
        }
    }()

    // ★修正点3: 全てのタスクが投入されるのを待つ
    wg.Wait()

    // 完了を待機
    err := pool.GracefulShutdown(30 * time.Second)
    if err != nil {
        t.Fatalf("Shutdown failed: %v", err)
    }

    duration := time.Since(start)
    throughput := float64(numTasks) / duration.Seconds()

    t.Logf("Processed %d tasks in %v (%.2f tasks/sec)",
        numTasks, duration.Round(time.Millisecond), throughput)
}
