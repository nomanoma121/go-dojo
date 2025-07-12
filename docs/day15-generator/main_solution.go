//go:build ignore

package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Generator represents a generator that produces values of type T
type Generator[T any] struct {
	ch     <-chan T
	cancel context.CancelFunc
	ctx    context.Context
}

// GeneratorFunc is a function that generates values
type GeneratorFunc[T any] func(ctx context.Context, yield func(T) bool)

// NewGenerator creates a new generator from a generator function
func NewGenerator[T any](fn GeneratorFunc[T]) Generator[T] {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan T)
	
	go func() {
		defer close(ch)
		fn(ctx, func(value T) bool {
			select {
			case ch <- value:
				return true
			case <-ctx.Done():
				return false
			}
		})
	}()
	
	return Generator[T]{
		ch:     ch,
		cancel: cancel,
		ctx:    ctx,
	}
}

// Next returns the next value from the generator
func (g Generator[T]) Next() (T, bool) {
	select {
	case value, ok := <-g.ch:
		return value, ok
	case <-g.ctx.Done():
		var zero T
		return zero, false
	}
}

// ToSlice collects all values from the generator into a slice
func (g Generator[T]) ToSlice() []T {
	var result []T
	for value := range g.ch {
		result = append(result, value)
	}
	return result
}

// ForEach applies a function to each value in the generator
func (g Generator[T]) ForEach(fn func(T)) {
	for value := range g.ch {
		fn(value)
	}
}

// Cancel stops the generator
func (g Generator[T]) Cancel() {
	if g.cancel != nil {
		g.cancel()
	}
}

// Chan returns the underlying channel
func (g Generator[T]) Chan() <-chan T {
	return g.ch
}

// Basic generators

// Range generates integers from start to end (inclusive)
func Range(start, end int) Generator[int] {
	return NewGenerator(func(ctx context.Context, yield func(int) bool) {
		for i := start; i <= end; i++ {
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
}

// Repeat generates the same value infinitely
func Repeat[T any](value T) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if !yield(value) {
					return
				}
			}
		}
	})
}

// FromSlice creates a generator from a slice
func FromSlice[T any](slice []T) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		for _, item := range slice {
			select {
			case <-ctx.Done():
				return
			default:
				if !yield(item) {
					return
				}
			}
		}
	})
}

// Fibonacci generates Fibonacci numbers
func Fibonacci() Generator[int] {
	return NewGenerator(func(ctx context.Context, yield func(int) bool) {
		a, b := 0, 1
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if !yield(a) {
					return
				}
				a, b = b, a+b
			}
		}
	})
}

// Timer generates timestamps at regular intervals
func Timer(interval time.Duration) Generator[time.Time] {
	return NewGenerator(func(ctx context.Context, yield func(time.Time) bool) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				if !yield(t) {
					return
				}
			}
		}
	})
}

// Transformation functions

// Map transforms each value using the provided function
func Map[T, U any](gen Generator[T], fn func(T) U) Generator[U] {
	return NewGenerator(func(ctx context.Context, yield func(U) bool) {
		for {
			select {
			case <-ctx.Done():
				return
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				transformed := fn(value)
				if !yield(transformed) {
					return
				}
			}
		}
	})
}

// Filter keeps only values that match the predicate
func Filter[T any](gen Generator[T], predicate func(T) bool) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		for {
			select {
			case <-ctx.Done():
				return
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				if predicate(value) {
					if !yield(value) {
						return
					}
				}
			}
		}
	})
}

// Take takes the first n values from the generator
func Take[T any](gen Generator[T], n int) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		count := 0
		for {
			if count >= n {
				return
			}
			select {
			case <-ctx.Done():
				return
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				if !yield(value) {
					return
				}
				count++
			}
		}
	})
}

// Skip skips the first n values from the generator
func Skip[T any](gen Generator[T], n int) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		skipped := 0
		for {
			select {
			case <-ctx.Done():
				return
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				if skipped < n {
					skipped++
					continue
				}
				if !yield(value) {
					return
				}
			}
		}
	})
}

// TakeWhile takes values while the predicate is true
func TakeWhile[T any](gen Generator[T], predicate func(T) bool) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		for {
			select {
			case <-ctx.Done():
				return
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				if !predicate(value) {
					return
				}
				if !yield(value) {
					return
				}
			}
		}
	})
}

// Chain concatenates multiple generators
func Chain[T any](generators ...Generator[T]) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		for _, gen := range generators {
			for value := range gen.ch {
				select {
				case <-ctx.Done():
					return
				default:
					if !yield(value) {
						return
					}
				}
			}
		}
	})
}

// Zip combines two generators into pairs
func Zip[T, U any](gen1 Generator[T], gen2 Generator[U]) Generator[Pair[T, U]] {
	return NewGenerator(func(ctx context.Context, yield func(Pair[T, U]) bool) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				value1, ok1 := <-gen1.ch
				if !ok1 {
					return
				}
				value2, ok2 := <-gen2.ch
				if !ok2 {
					return
				}
				pair := Pair[T, U]{First: value1, Second: value2}
				if !yield(pair) {
					return
				}
			}
		}
	})
}

// Pair represents a pair of values
type Pair[T, U any] struct {
	First  T
	Second U
}

// Aggregate functions

// Reduce reduces the generator to a single value
func Reduce[T, U any](gen Generator[T], initial U, fn func(U, T) U) U {
	accumulator := initial
	for value := range gen.ch {
		accumulator = fn(accumulator, value)
	}
	return accumulator
}

// Count counts the number of values in the generator
func Count[T any](gen Generator[T]) int {
	count := 0
	for range gen.ch {
		count++
	}
	return count
}

// Any checks if any value matches the predicate
func Any[T any](gen Generator[T], predicate func(T) bool) bool {
	for value := range gen.ch {
		if predicate(value) {
			return true
		}
	}
	return false
}

// All checks if all values match the predicate
func All[T any](gen Generator[T], predicate func(T) bool) bool {
	for value := range gen.ch {
		if !predicate(value) {
			return false
		}
	}
	return true
}

// Advanced generators

// Batch groups values into batches of specified size
func Batch[T any](gen Generator[T], size int) Generator[[]T] {
	return NewGenerator(func(ctx context.Context, yield func([]T) bool) {
		batch := make([]T, 0, size)
		for {
			select {
			case <-ctx.Done():
				if len(batch) > 0 {
					yield(batch)
				}
				return
			case value, ok := <-gen.ch:
				if !ok {
					if len(batch) > 0 {
						yield(batch)
					}
					return
				}
				batch = append(batch, value)
				if len(batch) == size {
					if !yield(batch) {
						return
					}
					batch = make([]T, 0, size)
				}
			}
		}
	})
}

// Distinct removes duplicate values
func Distinct[T comparable](gen Generator[T]) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		seen := make(map[T]bool)
		for {
			select {
			case <-ctx.Done():
				return
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				if !seen[value] {
					seen[value] = true
					if !yield(value) {
						return
					}
				}
			}
		}
	})
}

// Parallel processes values in parallel
func Parallel[T, U any](gen Generator[T], fn func(T) U, workers int) Generator[U] {
	return NewGenerator(func(ctx context.Context, yield func(U) bool) {
		input := make(chan T, workers)
		output := make(chan U, workers)
		
		// Start workers
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for value := range input {
					select {
					case <-ctx.Done():
						return
					case output <- fn(value):
					}
				}
			}()
		}
		
		// Send input values
		go func() {
			defer close(input)
			for {
				select {
				case <-ctx.Done():
					return
				case value, ok := <-gen.ch:
					if !ok {
						return
					}
					select {
					case input <- value:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
		
		// Close output when all workers are done
		go func() {
			wg.Wait()
			close(output)
		}()
		
		// Yield results
		for result := range output {
			if !yield(result) {
				return
			}
		}
	})
}

// Buffer buffers values to improve throughput
func Buffer[T any](gen Generator[T], size int) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		buffer := make(chan T, size)
		
		// Fill buffer
		go func() {
			defer close(buffer)
			for {
				select {
				case <-ctx.Done():
					return
				case value, ok := <-gen.ch:
					if !ok {
						return
					}
					select {
					case buffer <- value:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
		
		// Yield from buffer
		for value := range buffer {
			if !yield(value) {
				return
			}
		}
	})
}

func main() {
	fmt.Println("=== Generator Pattern Demo ===")
	
	// 基本的なジェネレータの使用例
	fmt.Println("Range generator:")
	rangeGen := Range(1, 5)
	rangeGen.ForEach(func(x int) {
		fmt.Printf("%d ", x)
	})
	fmt.Println()
	
	// 変換操作の例
	fmt.Println("\nMap transformation:")
	squaredGen := Map(Range(1, 5), func(x int) int {
		return x * x
	})
	squares := squaredGen.ToSlice()
	fmt.Printf("Squares: %v\n", squares)
	
	// フィルタリングの例
	fmt.Println("\nFilter even numbers:")
	evenGen := Filter(Range(1, 10), func(x int) bool {
		return x%2 == 0
	})
	evens := evenGen.ToSlice()
	fmt.Printf("Evens: %v\n", evens)
	
	// 無限ジェネレータの例（最初の10個を取得）
	fmt.Println("\nFibonacci sequence (first 10):")
	fibGen := Take(Fibonacci(), 10)
	fibs := fibGen.ToSlice()
	fmt.Printf("Fibonacci: %v\n", fibs)
	
	// チェイニングの例
	fmt.Println("\nChained operations:")
	result := Map(
		Filter(Range(1, 20), func(x int) bool {
			return x%3 == 0
		}),
		func(x int) string {
			return fmt.Sprintf("num-%d", x)
		},
	)
	fmt.Printf("Multiples of 3 as strings: %v\n", result.ToSlice())
	
	// バッチ処理の例
	fmt.Println("\nBatch processing:")
	batchGen := Batch(Range(1, 15), 4)
	batchGen.ForEach(func(batch []int) {
		fmt.Printf("Batch: %v\n", batch)
	})
	
	// リデュース操作の例
	fmt.Println("\nReduce operation:")
	sum := Reduce(Range(1, 100), 0, func(acc, x int) int {
		return acc + x
	})
	fmt.Printf("Sum of 1-100: %d\n", sum)
	
	// 並列処理の例
	fmt.Println("\nParallel processing:")
	
	start := time.Now()
	parallelResult := Parallel(Range(1, 5), func(x int) int {
		time.Sleep(100 * time.Millisecond)
		return x * x
	}, 3)
	results := parallelResult.ToSlice()
	elapsed := time.Since(start)
	
	fmt.Printf("Parallel results: %v (took %v)\n", results, elapsed)
	
	// タイマージェネレータの例
	fmt.Println("\nTimer generator (5 ticks):")
	timerGen := Take(Timer(200*time.Millisecond), 5)
	timerGen.ForEach(func(t time.Time) {
		fmt.Printf("Tick at %s\n", t.Format("15:04:05.000"))
	})
	
	fmt.Println("\nDemo completed!")
}