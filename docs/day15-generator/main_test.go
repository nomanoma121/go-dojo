package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestBasicGenerators(t *testing.T) {
	t.Run("Range generator", func(t *testing.T) {
		gen := Range(1, 5)
		values := gen.ToSlice()
		
		expected := []int{1, 2, 3, 4, 5}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
	
	t.Run("Empty range", func(t *testing.T) {
		gen := Range(5, 1) // Invalid range
		values := gen.ToSlice()
		
		if len(values) != 0 {
			t.Errorf("Expected empty slice for invalid range, got %v", values)
		}
	})
	
	t.Run("FromSlice generator", func(t *testing.T) {
		input := []string{"a", "b", "c"}
		gen := FromSlice(input)
		values := gen.ToSlice()
		
		if len(values) != len(input) {
			t.Errorf("Expected %d values, got %d", len(input), len(values))
		}
		
		for i, v := range values {
			if v != input[i] {
				t.Errorf("Expected %s at index %d, got %s", input[i], i, v)
			}
		}
	})
	
	t.Run("Repeat generator with Take", func(t *testing.T) {
		gen := Take(Repeat("hello"), 3)
		values := gen.ToSlice()
		
		expected := []string{"hello", "hello", "hello"}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %s at index %d, got %s", expected[i], i, v)
			}
		}
	})
	
	t.Run("Fibonacci generator", func(t *testing.T) {
		gen := Take(Fibonacci(), 10)
		values := gen.ToSlice()
		
		expected := []int{0, 1, 1, 2, 3, 5, 8, 13, 21, 34}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
}

func TestGeneratorIteration(t *testing.T) {
	t.Run("Next method", func(t *testing.T) {
		gen := Range(1, 3)
		
		val1, ok1 := gen.Next()
		if !ok1 || val1 != 1 {
			t.Errorf("Expected (1, true), got (%d, %t)", val1, ok1)
		}
		
		val2, ok2 := gen.Next()
		if !ok2 || val2 != 2 {
			t.Errorf("Expected (2, true), got (%d, %t)", val2, ok2)
		}
		
		val3, ok3 := gen.Next()
		if !ok3 || val3 != 3 {
			t.Errorf("Expected (3, true), got (%d, %t)", val3, ok3)
		}
		
		val4, ok4 := gen.Next()
		if ok4 {
			t.Errorf("Expected (0, false), got (%d, %t)", val4, ok4)
		}
	})
	
	t.Run("ForEach method", func(t *testing.T) {
		gen := Range(1, 5)
		var collected []int
		
		gen.ForEach(func(x int) {
			collected = append(collected, x)
		})
		
		expected := []int{1, 2, 3, 4, 5}
		if len(collected) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(collected))
		}
		
		for i, v := range collected {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
	
	t.Run("Chan method", func(t *testing.T) {
		gen := Range(1, 3)
		ch := gen.Chan()
		
		var values []int
		for v := range ch {
			values = append(values, v)
		}
		
		expected := []int{1, 2, 3}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
}

func TestTransformations(t *testing.T) {
	t.Run("Map transformation", func(t *testing.T) {
		gen := Map(Range(1, 5), func(x int) int {
			return x * 2
		})
		values := gen.ToSlice()
		
		expected := []int{2, 4, 6, 8, 10}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
	
	t.Run("Map type transformation", func(t *testing.T) {
		gen := Map(Range(1, 3), func(x int) string {
			return fmt.Sprintf("num-%d", x)
		})
		values := gen.ToSlice()
		
		expected := []string{"num-1", "num-2", "num-3"}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %s at index %d, got %s", expected[i], i, v)
			}
		}
	})
	
	t.Run("Filter transformation", func(t *testing.T) {
		gen := Filter(Range(1, 10), func(x int) bool {
			return x%2 == 0
		})
		values := gen.ToSlice()
		
		expected := []int{2, 4, 6, 8, 10}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
	
	t.Run("Take transformation", func(t *testing.T) {
		gen := Take(Range(1, 100), 5)
		values := gen.ToSlice()
		
		expected := []int{1, 2, 3, 4, 5}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
	
	t.Run("Skip transformation", func(t *testing.T) {
		gen := Skip(Range(1, 10), 5)
		values := gen.ToSlice()
		
		expected := []int{6, 7, 8, 9, 10}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
	
	t.Run("TakeWhile transformation", func(t *testing.T) {
		gen := TakeWhile(Range(1, 10), func(x int) bool {
			return x < 6
		})
		values := gen.ToSlice()
		
		expected := []int{1, 2, 3, 4, 5}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
}

func TestComposition(t *testing.T) {
	t.Run("Chained transformations", func(t *testing.T) {
		gen := Map(
			Filter(Range(1, 20), func(x int) bool {
				return x%3 == 0
			}),
			func(x int) string {
				return fmt.Sprintf("num-%d", x)
			},
		)
		values := gen.ToSlice()
		
		expected := []string{"num-3", "num-6", "num-9", "num-12", "num-15", "num-18"}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %s at index %d, got %s", expected[i], i, v)
			}
		}
	})
	
	t.Run("Chain multiple generators", func(t *testing.T) {
		gen1 := Range(1, 3)
		gen2 := Range(10, 12)
		gen3 := Range(20, 21)
		
		chained := Chain(gen1, gen2, gen3)
		values := chained.ToSlice()
		
		expected := []int{1, 2, 3, 10, 11, 12, 20, 21}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
	
	t.Run("Zip generators", func(t *testing.T) {
		gen1 := Range(1, 3)
		gen2 := FromSlice([]string{"a", "b", "c"})
		
		zipped := Zip(gen1, gen2)
		values := zipped.ToSlice()
		
		if len(values) != 3 {
			t.Errorf("Expected 3 pairs, got %d", len(values))
		}
		
		expected := []Pair[int, string]{
			{1, "a"}, {2, "b"}, {3, "c"},
		}
		
		for i, pair := range values {
			if pair.First != expected[i].First || pair.Second != expected[i].Second {
				t.Errorf("Expected {%d, %s} at index %d, got {%d, %s}",
					expected[i].First, expected[i].Second, i, pair.First, pair.Second)
			}
		}
	})
	
	t.Run("Zip unequal length generators", func(t *testing.T) {
		gen1 := Range(1, 5)
		gen2 := FromSlice([]string{"a", "b"})
		
		zipped := Zip(gen1, gen2)
		values := zipped.ToSlice()
		
		// Should stop when shorter generator ends
		if len(values) != 2 {
			t.Errorf("Expected 2 pairs, got %d", len(values))
		}
	})
}

func TestAggregations(t *testing.T) {
	t.Run("Reduce", func(t *testing.T) {
		sum := Reduce(Range(1, 10), 0, func(acc, x int) int {
			return acc + x
		})
		
		expected := 55 // Sum of 1-10
		if sum != expected {
			t.Errorf("Expected %d, got %d", expected, sum)
		}
	})
	
	t.Run("Count", func(t *testing.T) {
		count := Count(Range(1, 100))
		
		if count != 100 {
			t.Errorf("Expected 100, got %d", count)
		}
	})
	
	t.Run("Any", func(t *testing.T) {
		hasEven := Any(Range(1, 10), func(x int) bool {
			return x%2 == 0
		})
		
		if !hasEven {
			t.Error("Expected to find even number")
		}
		
		hasLarge := Any(Range(1, 5), func(x int) bool {
			return x > 10
		})
		
		if hasLarge {
			t.Error("Should not find number > 10 in range 1-5")
		}
	})
	
	t.Run("All", func(t *testing.T) {
		allPositive := All(Range(1, 10), func(x int) bool {
			return x > 0
		})
		
		if !allPositive {
			t.Error("Expected all numbers to be positive")
		}
		
		allEven := All(Range(1, 10), func(x int) bool {
			return x%2 == 0
		})
		
		if allEven {
			t.Error("Not all numbers should be even")
		}
	})
}

func TestAdvancedFeatures(t *testing.T) {
	t.Run("Batch", func(t *testing.T) {
		gen := Batch(Range(1, 10), 3)
		batches := gen.ToSlice()
		
		expected := [][]int{
			{1, 2, 3},
			{4, 5, 6},
			{7, 8, 9},
			{10},
		}
		
		if len(batches) != len(expected) {
			t.Errorf("Expected %d batches, got %d", len(expected), len(batches))
		}
		
		for i, batch := range batches {
			if len(batch) != len(expected[i]) {
				t.Errorf("Batch %d: expected length %d, got %d", i, len(expected[i]), len(batch))
			}
			
			for j, v := range batch {
				if v != expected[i][j] {
					t.Errorf("Batch %d[%d]: expected %d, got %d", i, j, expected[i][j], v)
				}
			}
		}
	})
	
	t.Run("Distinct", func(t *testing.T) {
		input := []int{1, 2, 2, 3, 1, 4, 3, 5}
		gen := Distinct(FromSlice(input))
		values := gen.ToSlice()
		
		// Should contain each unique value once (order may vary)
		expected := map[int]bool{1: true, 2: true, 3: true, 4: true, 5: true}
		
		if len(values) != len(expected) {
			t.Errorf("Expected %d unique values, got %d", len(expected), len(values))
		}
		
		for _, v := range values {
			if !expected[v] {
				t.Errorf("Unexpected value %d in distinct result", v)
			}
		}
	})
	
	t.Run("Buffer", func(t *testing.T) {
		gen := Buffer(Range(1, 5), 2)
		values := gen.ToSlice()
		
		expected := []int{1, 2, 3, 4, 5}
		if len(values) != len(expected) {
			t.Errorf("Expected %d values, got %d", len(expected), len(values))
		}
		
		for i, v := range values {
			if v != expected[i] {
				t.Errorf("Expected %d at index %d, got %d", expected[i], i, v)
			}
		}
	})
}

func TestCancellation(t *testing.T) {
	t.Run("Cancel generator", func(t *testing.T) {
		gen := Repeat(1)
		
		// Start consuming in goroutine
		var count int
		done := make(chan bool)
		
		go func() {
			for {
				_, ok := gen.Next()
				if !ok {
					break
				}
				count++
				if count >= 5 {
					gen.Cancel()
				}
			}
			done <- true
		}()
		
		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Error("Generator should have been cancelled")
		}
		
		if count < 5 {
			t.Errorf("Expected at least 5 values before cancellation, got %d", count)
		}
	})
	
	t.Run("Context cancellation", func(t *testing.T) {
		_, cancel := context.WithCancel(context.Background())
		
		gen := NewGenerator(func(ctx context.Context, yield func(int) bool) {
			for i := 1; ; i++ {
				select {
				case <-ctx.Done():
					return
				default:
					if !yield(i) {
						return
					}
				}
			}
		})
		
		var values []int
		go func() {
			for v := range gen.Chan() {
				values = append(values, v)
				if len(values) >= 5 {
					cancel()
					break
				}
			}
		}()
		
		time.Sleep(100 * time.Millisecond)
		
		if len(values) < 5 {
			t.Errorf("Expected at least 5 values, got %d", len(values))
		}
	})
}

func TestTimer(t *testing.T) {
	t.Run("Timer generator", func(t *testing.T) {
		start := time.Now()
		gen := Take(Timer(50*time.Millisecond), 3)
		
		var timestamps []time.Time
		gen.ForEach(func(ts time.Time) {
			timestamps = append(timestamps, ts)
		})
		
		elapsed := time.Since(start)
		
		if len(timestamps) != 3 {
			t.Errorf("Expected 3 timestamps, got %d", len(timestamps))
		}
		
		// Should take at least 100ms (2 intervals)
		if elapsed < 100*time.Millisecond {
			t.Errorf("Timer took %v, expected at least 100ms", elapsed)
		}
		
		// Check intervals
		for i := 1; i < len(timestamps); i++ {
			interval := timestamps[i].Sub(timestamps[i-1])
			if interval < 40*time.Millisecond || interval > 70*time.Millisecond {
				t.Errorf("Interval %d was %v, expected around 50ms", i, interval)
			}
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("Multiple consumers", func(t *testing.T) {
		gen := Range(1, 100)
		ch := gen.Chan()
		
		var wg sync.WaitGroup
		var mu sync.Mutex
		var allValues []int
		
		// Start multiple consumers
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for v := range ch {
					mu.Lock()
					allValues = append(allValues, v)
					mu.Unlock()
				}
			}()
		}
		
		wg.Wait()
		
		// Should have consumed all values exactly once
		if len(allValues) != 100 {
			t.Errorf("Expected 100 values total, got %d", len(allValues))
		}
		
		// Check that all values 1-100 are present
		valueMap := make(map[int]bool)
		for _, v := range allValues {
			valueMap[v] = true
		}
		
		for i := 1; i <= 100; i++ {
			if !valueMap[i] {
				t.Errorf("Value %d was not consumed", i)
			}
		}
	})
	
	t.Run("Parallel processing", func(t *testing.T) {
		gen := Parallel(Range(1, 10), func(x int) int {
			return x * x
		}, 3)
		
		values := gen.ToSlice()
		
		if len(values) != 10 {
			t.Errorf("Expected 10 values, got %d", len(values))
		}
		
		// Values should be squares of 1-10 (order may vary)
		expectedSet := make(map[int]bool)
		for i := 1; i <= 10; i++ {
			expectedSet[i*i] = true
		}
		
		for _, v := range values {
			if !expectedSet[v] {
				t.Errorf("Unexpected value %d", v)
			}
		}
	})
}

// Benchmark tests
func BenchmarkRange(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gen := Range(1, 1000)
		Count(gen)
	}
}

func BenchmarkMap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gen := Map(Range(1, 1000), func(x int) int {
			return x * 2
		})
		Count(gen)
	}
}

func BenchmarkFilter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gen := Filter(Range(1, 1000), func(x int) bool {
			return x%2 == 0
		})
		Count(gen)
	}
}

func BenchmarkChained(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gen := Map(
			Filter(Range(1, 1000), func(x int) bool {
				return x%3 == 0
			}),
			func(x int) int {
				return x * x
			},
		)
		Count(gen)
	}
}

func BenchmarkParallel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gen := Parallel(Range(1, 100), func(x int) int {
			return x * x
		}, 4)
		Count(gen)
	}
}