package main

import (
	"sync"
	"testing"
	"time"
)

func TestWorkerPool(t *testing.T) {
	t.Run("Basic functionality", func(t *testing.T) {
		pool := NewWorkerPool(2, 5)
		pool.Start()
		defer pool.Stop()
		
		// Submit a task
		task := Task{
			ID:      1,
			Data:    "test task",
			Created: time.Now(),
		}
		
		err := pool.SubmitTask(task)
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
		
		// Get result
		result, ok := pool.GetResult()
		if !ok {
			t.Fatal("Expected result but got none")
		}
		
		if result.TaskID != task.ID {
			t.Errorf("Expected TaskID %d, got %d", task.ID, result.TaskID)
		}
	})

	t.Run("Multiple tasks", func(t *testing.T) {
		const numTasks = 10
		pool := NewWorkerPool(3, numTasks)
		pool.Start()
		defer pool.Stop()
		
		// Submit multiple tasks
		for i := 0; i < numTasks; i++ {
			task := Task{
				ID:      i,
				Data:    i * 2,
				Created: time.Now(),
			}
			
			err := pool.SubmitTask(task)
			if err != nil {
				t.Fatalf("Failed to submit task %d: %v", i, err)
			}
		}
		
		// Collect results
		results := make(map[int]Result)
		for i := 0; i < numTasks; i++ {
			result, ok := pool.GetResult()
			if !ok {
				t.Fatalf("Expected result %d but got none", i)
			}
			results[result.TaskID] = result
		}
		
		// Verify all tasks were processed
		if len(results) != numTasks {
			t.Errorf("Expected %d results, got %d", numTasks, len(results))
		}
		
		for i := 0; i < numTasks; i++ {
			if _, exists := results[i]; !exists {
				t.Errorf("Missing result for task %d", i)
			}
		}
	})

	t.Run("Worker limit", func(t *testing.T) {
		const numWorkers = 2
		pool := NewWorkerPool(numWorkers, 10)
		pool.Start()
		defer pool.Stop()
		
		// Submit tasks that take some time
		const numTasks = 6
		start := time.Now()
		
		for i := 0; i < numTasks; i++ {
			task := Task{
				ID:   i,
				Data: "slow task",
			}
			pool.SubmitTask(task)
		}
		
		// Collect results
		for i := 0; i < numTasks; i++ {
			pool.GetResult()
		}
		
		elapsed := time.Since(start)
		
		// With 2 workers and 6 tasks that take ~100ms each,
		// it should take at least 300ms (3 rounds of execution)
		minExpected := 200 * time.Millisecond
		if elapsed < minExpected {
			t.Errorf("Tasks completed too quickly: %v (expected >= %v)", elapsed, minExpected)
		}
	})

	t.Run("Graceful shutdown", func(t *testing.T) {
		pool := NewWorkerPool(2, 5)
		pool.Start()
		
		// Submit some tasks
		for i := 0; i < 3; i++ {
			task := Task{
				ID:   i,
				Data: "shutdown test",
			}
			pool.SubmitTask(task)
		}
		
		// Stop pool (should wait for current tasks to complete)
		stopStart := time.Now()
		pool.Stop()
		stopDuration := time.Since(stopStart)
		
		// Should have taken some time to complete running tasks
		if stopDuration < 50*time.Millisecond {
			t.Errorf("Stop completed too quickly: %v", stopDuration)
		}
	})

	t.Run("Queue full handling", func(t *testing.T) {
		const queueSize = 2
		pool := NewWorkerPool(1, queueSize)
		pool.Start()
		defer pool.Stop()
		
		// Fill the queue and one more (being processed)
		submitted := 0
		for i := 0; i < queueSize+2; i++ {
			task := Task{
				ID:   i,
				Data: "queue test",
			}
			
			err := pool.SubmitTask(task)
			if err == nil {
				submitted++
			}
		}
		
		// Should have submitted at least queueSize tasks
		if submitted < queueSize {
			t.Errorf("Expected to submit at least %d tasks, submitted %d", queueSize, submitted)
		}
	})

	t.Run("Concurrent submission", func(t *testing.T) {
		pool := NewWorkerPool(3, 20)
		pool.Start()
		defer pool.Stop()
		
		const numGoroutines = 5
		const tasksPerGoroutine = 4
		
		var wg sync.WaitGroup
		submitted := make(chan int, numGoroutines)
		
		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				count := 0
				for i := 0; i < tasksPerGoroutine; i++ {
					task := Task{
						ID:   goroutineID*tasksPerGoroutine + i,
						Data: goroutineID,
					}
					
					err := pool.SubmitTask(task)
					if err == nil {
						count++
					}
				}
				submitted <- count
			}(g)
		}
		
		wg.Wait()
		close(submitted)
		
		totalSubmitted := 0
		for count := range submitted {
			totalSubmitted += count
		}
		
		// Collect results
		results := 0
		timeout := time.After(2 * time.Second)
		for results < totalSubmitted {
			select {
			case <-timeout:
				t.Fatalf("Timeout waiting for results. Expected %d, got %d", totalSubmitted, results)
			default:
				if _, ok := pool.GetResult(); ok {
					results++
				} else {
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
	})
}

func TestTaskProcessor(t *testing.T) {
	t.Run("SimpleTaskProcessor", func(t *testing.T) {
		processor := &SimpleTaskProcessor{}
		
		result, err := processor.Process("test data")
		if err != nil {
			t.Fatalf("SimpleTaskProcessor failed: %v", err)
		}
		
		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("HeavyTaskProcessor", func(t *testing.T) {
		processor := &HeavyTaskProcessor{}
		
		start := time.Now()
		result, err := processor.Process(123)
		duration := time.Since(start)
		
		if err != nil {
			t.Fatalf("HeavyTaskProcessor failed: %v", err)
		}
		
		if result == nil {
			t.Error("Expected non-nil result")
		}
		
		// Should take some time due to heavy processing
		if duration < 50*time.Millisecond {
			t.Errorf("Heavy processing completed too quickly: %v", duration)
		}
	})
}

func TestBatchProcessor(t *testing.T) {
	t.Run("Batch accumulation", func(t *testing.T) {
		const batchSize = 3
		processor := NewBatchProcessor(batchSize)
		
		// Add tasks one by one
		for i := 0; i < batchSize-1; i++ {
			task := Task{ID: i, Data: i}
			ready := processor.AddTask(task)
			if ready {
				t.Errorf("Batch should not be ready at task %d", i)
			}
		}
		
		// Add final task to complete batch
		finalTask := Task{ID: batchSize - 1, Data: batchSize - 1}
		ready := processor.AddTask(finalTask)
		if !ready {
			t.Error("Batch should be ready after adding final task")
		}
	})

	t.Run("Batch processing", func(t *testing.T) {
		const batchSize = 2
		processor := NewBatchProcessor(batchSize)
		
		// Fill batch
		for i := 0; i < batchSize; i++ {
			task := Task{ID: i, Data: i * 10}
			processor.AddTask(task)
		}
		
		// Process batch
		results := processor.ProcessBatch()
		if len(results) != batchSize {
			t.Errorf("Expected %d results, got %d", batchSize, len(results))
		}
		
		for i, result := range results {
			if result.TaskID != i {
				t.Errorf("Expected TaskID %d, got %d", i, result.TaskID)
			}
		}
	})
}

func TestWorkerPoolStats(t *testing.T) {
	pool := NewWorkerPool(2, 5)
	pool.Start()
	defer pool.Stop()
	
	stats := pool.GetStats()
	
	if stats.NumWorkers != 2 {
		t.Errorf("Expected 2 workers, got %d", stats.NumWorkers)
	}
	
	if stats.QueueSize != 5 {
		t.Errorf("Expected queue size 5, got %d", stats.QueueSize)
	}
	
	// Submit a task and check queue length
	task := Task{ID: 1, Data: "stats test"}
	pool.SubmitTask(task)
	
	stats = pool.GetStats()
	if stats.QueueLength < 0 || stats.QueueLength > stats.QueueSize {
		t.Errorf("Invalid queue length: %d", stats.QueueLength)
	}
}

// ベンチマークテスト
func BenchmarkWorkerPool(b *testing.B) {
	pool := NewWorkerPool(4, 100)
	pool.Start()
	defer pool.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		taskID := 0
		for pb.Next() {
			task := Task{
				ID:   taskID,
				Data: "benchmark task",
			}
			
			err := pool.SubmitTask(task)
			if err == nil {
				pool.GetResult()
			}
			taskID++
		}
	})
}

func BenchmarkTaskProcessors(b *testing.B) {
	b.Run("SimpleTaskProcessor", func(b *testing.B) {
		processor := &SimpleTaskProcessor{}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processor.Process("benchmark data")
		}
	})

	b.Run("HeavyTaskProcessor", func(b *testing.B) {
		processor := &HeavyTaskProcessor{}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			processor.Process(i)
		}
	})
}

func BenchmarkBatchProcessor(b *testing.B) {
	const batchSize = 10
	processor := NewBatchProcessor(batchSize)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < batchSize; j++ {
			task := Task{ID: i*batchSize + j, Data: j}
			if processor.AddTask(task) {
				processor.ProcessBatch()
			}
		}
	}
}

// Race condition test
func TestWorkerPoolRace(t *testing.T) {
	pool := NewWorkerPool(5, 20)
	pool.Start()
	defer pool.Stop()
	
	const numGoroutines = 10
	const numTasks = 50
	
	var wg sync.WaitGroup
	
	// Submit tasks concurrently
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for i := 0; i < numTasks; i++ {
				task := Task{
					ID:   goroutineID*numTasks + i,
					Data: goroutineID,
				}
				pool.SubmitTask(task)
			}
		}(g)
	}
	
	// Collect results concurrently
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for i := 0; i < numTasks; i++ {
				pool.GetResult()
			}
		}()
	}
	
	wg.Wait()
}