package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPipeline(t *testing.T) {
	t.Run("Basic pipeline processing", func(t *testing.T) {
		pipeline := NewPipeline(3, 10)
		pipeline.Start()
		defer pipeline.Stop()
		
		// Submit test data
		testItems := []DataItem{
			{ID: 1, Value: "test1", Stage: "input"},
			{ID: 2, Value: "test2", Stage: "input"},
			{ID: 3, Value: "test3", Stage: "input"},
		}
		
		for _, item := range testItems {
			err := pipeline.Submit(item)
			if err != nil {
				t.Fatalf("Failed to submit item %d: %v", item.ID, err)
			}
		}
		
		// Collect results
		results := make(map[int]DataItem)
		for i := 0; i < len(testItems); i++ {
			if result, ok := pipeline.GetOutput(); ok {
				results[result.ID] = result
			} else {
				t.Fatalf("Failed to get result %d", i)
			}
		}
		
		// Verify all items were processed
		if len(results) != len(testItems) {
			t.Errorf("Expected %d results, got %d", len(testItems), len(results))
		}
		
		for _, original := range testItems {
			if result, exists := results[original.ID]; exists {
				if result.Stage != "processed" {
					t.Errorf("Item %d not processed correctly, stage: %s", original.ID, result.Stage)
				}
			} else {
				t.Errorf("Missing result for item %d", original.ID)
			}
		}
	})

	t.Run("Pipeline capacity handling", func(t *testing.T) {
		pipeline := NewPipeline(2, 5) // Small buffer
		pipeline.Start()
		defer pipeline.Stop()
		
		// Try to submit more items than buffer size
		submitted := 0
		for i := 0; i < 10; i++ {
			item := DataItem{ID: i, Value: i, Stage: "input"}
			err := pipeline.Submit(item)
			if err == nil {
				submitted++
			}
		}
		
		if submitted < 5 {
			t.Errorf("Expected to submit at least 5 items, submitted %d", submitted)
		}
	})

	t.Run("Concurrent submission", func(t *testing.T) {
		pipeline := NewPipeline(4, 20)
		pipeline.Start()
		defer pipeline.Stop()
		
		const numGoroutines = 5
		const itemsPerGoroutine = 4
		
		var wg sync.WaitGroup
		submitted := make(chan int, numGoroutines)
		
		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				count := 0
				
				for i := 0; i < itemsPerGoroutine; i++ {
					item := DataItem{
						ID:    goroutineID*itemsPerGoroutine + i,
						Value: goroutineID,
						Stage: "input",
					}
					
					err := pipeline.Submit(item)
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
		timeout := time.After(3 * time.Second)
		for results < totalSubmitted {
			select {
			case <-timeout:
				t.Fatalf("Timeout waiting for results. Expected %d, got %d", totalSubmitted, results)
			default:
				if _, ok := pipeline.GetOutput(); ok {
					results++
				} else {
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
		
		if results != totalSubmitted {
			t.Errorf("Expected %d results, got %d", totalSubmitted, results)
		}
	})
}

func TestFanOutFanIn(t *testing.T) {
	t.Run("Fan-out distribution", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		input := make(chan DataItem, 10)
		outputs := FanOut(ctx, input, 3)
		
		if len(outputs) != 3 {
			t.Errorf("Expected 3 output channels, got %d", len(outputs))
		}
		
		// Send test data
		testData := []DataItem{
			{ID: 1, Value: "data1"},
			{ID: 2, Value: "data2"},
			{ID: 3, Value: "data3"},
		}
		
		go func() {
			for _, item := range testData {
				input <- item
			}
			close(input)
		}()
		
		// Collect from all outputs
		var wg sync.WaitGroup
		collected := make([][]DataItem, 3)
		
		for i, output := range outputs {
			wg.Add(1)
			go func(index int, ch <-chan DataItem) {
				defer wg.Done()
				for item := range ch {
					collected[index] = append(collected[index], item)
				}
			}(i, output)
		}
		
		wg.Wait()
		
		// Verify distribution
		totalCollected := 0
		for i, items := range collected {
			totalCollected += len(items)
			t.Logf("Output %d collected %d items", i, len(items))
		}
		
		if totalCollected != len(testData) {
			t.Errorf("Expected %d total items, got %d", len(testData), totalCollected)
		}
	})

	t.Run("Fan-in aggregation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		// Create multiple input channels
		input1 := make(chan DataItem, 5)
		input2 := make(chan DataItem, 5)
		input3 := make(chan DataItem, 5)
		
		// Fan-in to single output
		output := FanIn(ctx, input1, input2, input3)
		
		// Send data to different inputs
		testData := map[string][]DataItem{
			"input1": {{ID: 1, Value: "from1"}, {ID: 2, Value: "from1"}},
			"input2": {{ID: 3, Value: "from2"}, {ID: 4, Value: "from2"}},
			"input3": {{ID: 5, Value: "from3"}},
		}
		
		go func() {
			for _, item := range testData["input1"] {
				input1 <- item
			}
			close(input1)
		}()
		
		go func() {
			for _, item := range testData["input2"] {
				input2 <- item
			}
			close(input2)
		}()
		
		go func() {
			for _, item := range testData["input3"] {
				input3 <- item
			}
			close(input3)
		}()
		
		// Collect all results
		var results []DataItem
		expectedCount := 5
		timeout := time.After(2 * time.Second)
		
		for len(results) < expectedCount {
			select {
			case item, ok := <-output:
				if ok {
					results = append(results, item)
				} else {
					goto done
				}
			case <-timeout:
				t.Fatalf("Timeout waiting for fan-in results. Got %d/%d", len(results), expectedCount)
			}
		}
		
	done:
		if len(results) != expectedCount {
			t.Errorf("Expected %d results from fan-in, got %d", expectedCount, len(results))
		}
		
		// Verify all IDs are present
		ids := make(map[int]bool)
		for _, result := range results {
			ids[result.ID] = true
		}
		
		for i := 1; i <= expectedCount; i++ {
			if !ids[i] {
				t.Errorf("Missing result with ID %d", i)
			}
		}
	})
}

func TestMultiStagePipeline(t *testing.T) {
	t.Run("Multi-stage processing", func(t *testing.T) {
		// Define processing stages
		stage1 := func(item DataItem) DataItem {
			item.Value = item.Value.(string) + "-stage1"
			item.Stage = "stage1"
			return item
		}
		
		stage2 := func(item DataItem) DataItem {
			item.Value = item.Value.(string) + "-stage2"
			item.Stage = "stage2"
			return item
		}
		
		stage3 := func(item DataItem) DataItem {
			item.Value = item.Value.(string) + "-stage3"
			item.Stage = "final"
			return item
		}
		
		pipeline := NewMultiStagePipeline(stage1, stage2, stage3)
		if pipeline != nil {
			pipeline.Start()
			
			// Test would continue with actual pipeline testing
			// This is a placeholder for the multi-stage pipeline implementation
		}
	})
}

// Benchmark tests
func BenchmarkPipelineProcessing(b *testing.B) {
	pipeline := NewPipeline(4, 100)
	pipeline.Start()
	defer pipeline.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			item := DataItem{
				ID:    id,
				Value: "benchmark data",
				Stage: "input",
			}
			
			err := pipeline.Submit(item)
			if err == nil {
				pipeline.GetOutput()
			}
			id++
		}
	})
}

func BenchmarkFanOutFanIn(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		input := make(chan DataItem, 10)
		outputs := FanOut(ctx, input, 4)
		output := FanIn(ctx, outputs...)
		
		// Send test data
		go func() {
			input <- DataItem{ID: i, Value: "test"}
			close(input)
		}()
		
		// Consume output
		<-output
	}
}