package main

import (
	"context"
	"testing"
	"time"
)

func TestResultCollector(t *testing.T) {
	t.Run("Unordered collection", func(t *testing.T) {
		ctx := context.Background()
		collector := NewResultCollector(ctx, 10, false)
		
		// Collect some results
		results := []Result{
			{TaskID: 1, Output: "result1", Duration: 100 * time.Millisecond},
			{TaskID: 2, Output: "result2", Duration: 200 * time.Millisecond},
			{TaskID: 3, Output: "result3", Duration: 150 * time.Millisecond},
		}
		
		for _, result := range results {
			collector.CollectResult(result)
		}
		
		// Retrieve results
		collected := collector.GetResults(3)
		if len(collected) != 3 {
			t.Errorf("Expected 3 results, got %d", len(collected))
		}
	})

	t.Run("Ordered collection", func(t *testing.T) {
		ctx := context.Background()
		collector := NewResultCollector(ctx, 10, true)
		
		// Collect results out of order
		collector.CollectResult(Result{TaskID: 2, Output: "result2"})
		collector.CollectResult(Result{TaskID: 1, Output: "result1"})
		collector.CollectResult(Result{TaskID: 3, Output: "result3"})
		
		// Should get them in order
		result1, ok := collector.GetResult()
		if !ok || result1.TaskID != 1 {
			t.Errorf("Expected TaskID 1, got %d", result1.TaskID)
		}
		
		result2, ok := collector.GetResult()
		if !ok || result2.TaskID != 2 {
			t.Errorf("Expected TaskID 2, got %d", result2.TaskID)
		}
	})
}

func TestResultAggregator(t *testing.T) {
	t.Run("Basic aggregation", func(t *testing.T) {
		aggregator := NewResultAggregator(func(results []Result) interface{} {
			total := 0
			for _, r := range results {
				if val, ok := r.Output.(int); ok {
					total += val
				}
			}
			return total
		})
		
		results := []Result{
			{TaskID: 1, Output: 10, Error: nil},
			{TaskID: 2, Output: 20, Error: nil},
			{TaskID: 3, Output: 30, Error: nil},
		}
		
		aggregated := aggregator.Aggregate(results)
		
		if aggregated.TotalTasks != 3 {
			t.Errorf("Expected 3 total tasks, got %d", aggregated.TotalTasks)
		}
		
		if aggregated.SuccessCount != 3 {
			t.Errorf("Expected 3 successful tasks, got %d", aggregated.SuccessCount)
		}
		
		if aggregated.AggregateData.(int) != 60 {
			t.Errorf("Expected aggregate sum 60, got %v", aggregated.AggregateData)
		}
	})
}