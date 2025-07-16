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

// PoolStats represents statistics about the worker pool
type PoolStats struct {
	NumWorkers    int
	QueueSize     int
	QueueLength   int
	TasksComplete int64
	TasksInFlight int
}

// TaskProcessor interface for processing different types of tasks
type TaskProcessor interface {
	Process(data interface{}) (interface{}, error)
}

// SimpleTaskProcessor processes simple tasks
type SimpleTaskProcessor struct{}

// HeavyTaskProcessor simulates CPU-intensive work
type HeavyTaskProcessor struct {
	ProcessingTime time.Duration
}

// BatchProcessor processes multiple items in batches
type BatchProcessor struct {
	BatchSize int
	buffer    []interface{}
	tasks     []Task
	mu        sync.Mutex
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

// NewBatchProcessor creates a new BatchProcessor
func NewBatchProcessor(batchSize int) *BatchProcessor {
	return &BatchProcessor{
		BatchSize: batchSize,
		buffer:    make([]interface{}, 0, batchSize),
		tasks:     make([]Task, 0, batchSize),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// worker is the main worker function that processes tasks
func (wp *WorkerPool) worker(workerID int) {
	defer wp.wg.Done()
	
	for {
		select {
		case task, ok := <-wp.taskQueue:
			if !ok {
				// Channel closed, worker should exit
				return
			}
			
			// Process the task
			result := wp.processTask(task, workerID)
			
			// Send result
			select {
			case wp.resultChan <- result:
			case <-wp.ctx.Done():
				return
			}
			
		case <-wp.ctx.Done():
			// Context cancelled, worker should exit
			return
		}
	}
}

// processTask processes a single task
func (wp *WorkerPool) processTask(task Task, workerID int) Result {
	start := time.Now()
	
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
		// Number processing
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
	select {
	case result, ok := <-wp.resultChan:
		return result, ok
	case <-time.After(1 * time.Second):
		return Result{}, false
	}
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.taskQueue)
	wp.cancel()
	wp.wg.Wait()
	close(wp.resultChan)
}

// WaitForCompletion waits for all submitted tasks to complete
func (wp *WorkerPool) WaitForCompletion() {
	// Wait for task queue to be empty
	for len(wp.taskQueue) > 0 {
		time.Sleep(10 * time.Millisecond)
	}
	
	// Wait a bit more for workers to finish current tasks
	time.Sleep(100 * time.Millisecond)
}

// GetStats returns statistics about the worker pool
func (wp *WorkerPool) GetStats() PoolStats {
	return PoolStats{
		NumWorkers:  wp.numWorkers,
		QueueSize:   cap(wp.taskQueue),
		QueueLength: len(wp.taskQueue),
	}
}

// SimpleTaskProcessor implementation
func (stp *SimpleTaskProcessor) Process(data interface{}) (interface{}, error) {
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

// HeavyTaskProcessor implementation
func (htp *HeavyTaskProcessor) Process(data interface{}) (interface{}, error) {
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

// BatchProcessor implementation
func (bp *BatchProcessor) AddTask(task Task) bool {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	bp.tasks = append(bp.tasks, task)
	return len(bp.tasks) >= bp.BatchSize
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