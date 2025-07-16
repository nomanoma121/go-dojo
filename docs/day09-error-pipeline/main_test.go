package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestErrorPipeline(t *testing.T) {
	t.Run("Basic pipeline functionality", func(t *testing.T) {
		pipeline := NewErrorPipeline()
		defer pipeline.Stop()
		
		// Add stages
		pipeline.AddStage(ValidationStage)
		pipeline.AddStage(TransformStage)
		
		// Create input
		input := make(chan DataItem)
		go func() {
			defer close(input)
			for i := 0; i < 5; i++ {
				input <- DataItem{
					ID:   i,
					Data: fmt.Sprintf("data-%d", i),
					Metadata: map[string]string{
						"source": "test",
					},
				}
			}
		}()
		
		// Process
		output := pipeline.Process(input)
		
		// Collect results
		var results []DataItem
		for result := range output {
			results = append(results, result)
		}
		
		if len(results) != 5 {
			t.Errorf("Expected 5 results, got %d", len(results))
		}
	})
	
	t.Run("Error propagation", func(t *testing.T) {
		pipeline := NewErrorPipeline()
		defer pipeline.Stop()
		
		// Add a stage that will cause errors
		errorStage := func(ctx context.Context, input <-chan DataItem) <-chan DataItem {
			output := make(chan DataItem)
			go func() {
				defer close(output)
				for item := range input {
					if item.ID%2 == 0 {
						// Simulate error for even IDs
						panic(fmt.Sprintf("error processing item %d", item.ID))
					}
					output <- item
				}
			}()
			return output
		}
		
		pipeline.AddStage(errorStage)
		
		// Create input
		input := make(chan DataItem)
		go func() {
			defer close(input)
			for i := 0; i < 4; i++ {
				input <- DataItem{ID: i, Data: fmt.Sprintf("data-%d", i)}
			}
		}()
		
		// Process and collect errors
		output := pipeline.Process(input)
		var errors []PipelineError
		
		done := make(chan bool)
		go func() {
			for {
				pipelineErrors := pipeline.GetErrors()
				if len(pipelineErrors) > 0 {
					errors = append(errors, pipelineErrors...)
				}
				if len(errors) > 0 {
					done <- true
					return
				}
			}
		}()
		
		// Drain output
		for range output {
		}
		
		pipeline.Stop()
		<-done
		
		// Should have errors for even IDs
		if len(errors) < 2 {
			t.Errorf("Expected at least 2 errors, got %d", len(errors))
		}
	})
	
	t.Run("Partial failure handling", func(t *testing.T) {
		pipeline := NewErrorPipeline()
		defer pipeline.Stop()
		
		// Add a stage that fails for some items
		selectiveErrorStage := func(ctx context.Context, input <-chan DataItem) <-chan DataItem {
			output := make(chan DataItem)
			go func() {
				defer close(output)
				for item := range input {
					if item.ID == 2 {
						// Skip item 2 (simulate error)
						continue
					}
					output <- item
				}
			}()
			return output
		}
		
		pipeline.AddStage(selectiveErrorStage)
		
		// Create input
		input := make(chan DataItem)
		go func() {
			defer close(input)
			for i := 0; i < 5; i++ {
				input <- DataItem{ID: i, Data: fmt.Sprintf("data-%d", i)}
			}
		}()
		
		// Process
		output := pipeline.Process(input)
		
		// Collect results
		var results []DataItem
		for result := range output {
			results = append(results, result)
		}
		
		// Should have 4 results (excluding item 2)
		if len(results) != 4 {
			t.Errorf("Expected 4 results, got %d", len(results))
		}
		
		// Verify that item 2 is missing
		for _, result := range results {
			if result.ID == 2 {
				t.Error("Item 2 should have been filtered out")
			}
		}
	})
}

func TestRetryableStage(t *testing.T) {
	t.Run("Successful processing", func(t *testing.T) {
		processor := func(item DataItem) (DataItem, error) {
			item.Data = fmt.Sprintf("processed-%v", item.Data)
			return item, nil
		}
		
		stage := RetryableStage(processor, 3, "test-stage")
		
		input := make(chan DataItem)
		go func() {
			defer close(input)
			input <- DataItem{ID: 1, Data: "test"}
		}()
		
		output := stage(context.Background(), input)
		
		result := <-output
		if result.Data != "processed-test" {
			t.Errorf("Expected 'processed-test', got %v", result.Data)
		}
	})
	
	t.Run("Retry on failure", func(t *testing.T) {
		attempts := 0
		processor := func(item DataItem) (DataItem, error) {
			attempts++
			if attempts < 3 {
				return item, fmt.Errorf("temporary error")
			}
			item.Data = fmt.Sprintf("processed-%v", item.Data)
			return item, nil
		}
		
		stage := RetryableStage(processor, 3, "test-stage")
		
		input := make(chan DataItem)
		go func() {
			defer close(input)
			input <- DataItem{ID: 1, Data: "test"}
		}()
		
		output := stage(context.Background(), input)
		
		result := <-output
		if result.Data != "processed-test" {
			t.Errorf("Expected 'processed-test', got %v", result.Data)
		}
		
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})
	
	t.Run("Max retries exceeded", func(t *testing.T) {
		processor := func(item DataItem) (DataItem, error) {
			return item, fmt.Errorf("persistent error")
		}
		
		stage := RetryableStage(processor, 2, "test-stage")
		
		input := make(chan DataItem)
		go func() {
			defer close(input)
			input <- DataItem{ID: 1, Data: "test"}
		}()
		
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		
		output := stage(ctx, input)
		
		// Should not receive any output as all retries failed
		select {
		case <-output:
			t.Error("Should not receive output when all retries fail")
		case <-time.After(100 * time.Millisecond):
			// Expected - no output
		}
	})
}

func TestErrorCollector(t *testing.T) {
	t.Run("Basic error collection", func(t *testing.T) {
		collector := NewErrorCollector(10)
		
		err1 := PipelineError{
			Stage:     "stage1",
			Error:     fmt.Errorf("error 1"),
			Timestamp: time.Now(),
			Retryable: true,
		}
		
		err2 := PipelineError{
			Stage:     "stage2",
			Error:     fmt.Errorf("error 2"),
			Timestamp: time.Now(),
			Retryable: false,
		}
		
		collector.Collect(err1)
		collector.Collect(err2)
		
		errors := collector.GetErrors()
		if len(errors) != 2 {
			t.Errorf("Expected 2 errors, got %d", len(errors))
		}
	})
	
	t.Run("Filter by stage", func(t *testing.T) {
		collector := NewErrorCollector(10)
		
		collector.Collect(PipelineError{
			Stage: "stage1",
			Error: fmt.Errorf("error 1"),
		})
		collector.Collect(PipelineError{
			Stage: "stage2",
			Error: fmt.Errorf("error 2"),
		})
		collector.Collect(PipelineError{
			Stage: "stage1",
			Error: fmt.Errorf("error 3"),
		})
		
		stage1Errors := collector.GetErrorsByStage("stage1")
		if len(stage1Errors) != 2 {
			t.Errorf("Expected 2 stage1 errors, got %d", len(stage1Errors))
		}
		
		stage2Errors := collector.GetErrorsByStage("stage2")
		if len(stage2Errors) != 1 {
			t.Errorf("Expected 1 stage2 error, got %d", len(stage2Errors))
		}
	})
	
	t.Run("Max errors limit", func(t *testing.T) {
		collector := NewErrorCollector(2)
		
		for i := 0; i < 5; i++ {
			collector.Collect(PipelineError{
				Stage: "test",
				Error: fmt.Errorf("error %d", i),
			})
		}
		
		errors := collector.GetErrors()
		if len(errors) > 2 {
			t.Errorf("Expected at most 2 errors, got %d", len(errors))
		}
	})
	
	t.Run("Clear errors", func(t *testing.T) {
		collector := NewErrorCollector(10)
		
		collector.Collect(PipelineError{
			Stage: "test",
			Error: fmt.Errorf("error"),
		})
		
		if len(collector.GetErrors()) != 1 {
			t.Error("Should have 1 error before clear")
		}
		
		collector.Clear()
		
		if len(collector.GetErrors()) != 0 {
			t.Error("Should have 0 errors after clear")
		}
	})
}

func TestSampleStages(t *testing.T) {
	t.Run("ValidationStage", func(t *testing.T) {
		input := make(chan DataItem)
		go func() {
			defer close(input)
			input <- DataItem{ID: 1, Data: "valid"}
			input <- DataItem{ID: 2, Data: nil}
		}()
		
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		
		output := ValidationStage(ctx, input)
		
		var results []DataItem
		for result := range output {
			results = append(results, result)
		}
		
		// All items should pass through (validation logic not implemented in skeleton)
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})
	
	t.Run("TransformStage", func(t *testing.T) {
		input := make(chan DataItem)
		go func() {
			defer close(input)
			input <- DataItem{ID: 1, Data: "test"}
		}()
		
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		
		output := TransformStage(ctx, input)
		
		var results []DataItem
		for result := range output {
			results = append(results, result)
		}
		
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})
	
	t.Run("EnrichmentStage", func(t *testing.T) {
		input := make(chan DataItem)
		go func() {
			defer close(input)
			input <- DataItem{ID: 1, Data: "test"}
		}()
		
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		
		output := EnrichmentStage(ctx, input)
		
		var results []DataItem
		for result := range output {
			results = append(results, result)
		}
		
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})
}

func TestConcurrentPipelineProcessing(t *testing.T) {
	t.Run("Concurrent error handling", func(t *testing.T) {
		pipeline := NewErrorPipeline()
		defer pipeline.Stop()
		
		// Add multiple stages
		pipeline.AddStage(ValidationStage)
		pipeline.AddStage(TransformStage)
		pipeline.AddStage(EnrichmentStage)
		
		// Process multiple batches concurrently
		var wg sync.WaitGroup
		numBatches := 3
		itemsPerBatch := 10
		
		for batch := 0; batch < numBatches; batch++ {
			wg.Add(1)
			go func(batchID int) {
				defer wg.Done()
				
				input := make(chan DataItem)
				go func() {
					defer close(input)
					for i := 0; i < itemsPerBatch; i++ {
						input <- DataItem{
							ID:   batchID*itemsPerBatch + i,
							Data: fmt.Sprintf("batch-%d-item-%d", batchID, i),
						}
					}
				}()
				
				output := pipeline.Process(input)
				count := 0
				for range output {
					count++
				}
				
				if count != itemsPerBatch {
					t.Errorf("Batch %d: expected %d items, got %d", batchID, itemsPerBatch, count)
				}
			}(batch)
		}
		
		wg.Wait()
	})
}

// Benchmark tests
func BenchmarkErrorPipeline(b *testing.B) {
	pipeline := NewErrorPipeline()
	defer pipeline.Stop()
	
	pipeline.AddStage(ValidationStage)
	pipeline.AddStage(TransformStage)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			input := make(chan DataItem)
			go func() {
				defer close(input)
				input <- DataItem{ID: 1, Data: "test"}
			}()
			
			output := pipeline.Process(input)
			for range output {
			}
		}
	})
}

func BenchmarkRetryableStage(b *testing.B) {
	processor := func(item DataItem) (DataItem, error) {
		item.Data = fmt.Sprintf("processed-%v", item.Data)
		return item, nil
	}
	
	stage := RetryableStage(processor, 3, "benchmark")
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			input := make(chan DataItem)
			go func() {
				defer close(input)
				input <- DataItem{ID: 1, Data: "test"}
			}()
			
			output := stage(context.Background(), input)
			for range output {
			}
		}
	})
}

func BenchmarkErrorCollector(b *testing.B) {
	collector := NewErrorCollector(1000)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			collector.Collect(PipelineError{
				Stage: "benchmark",
				Error: fmt.Errorf("test error"),
			})
		}
	})
}