package main

import (
	"context"
	"sync"
	"time"
)

// NewGenerator creates a new generator from a generator function
func NewGenerator[T any](fn GeneratorFunc[T]) Generator[T] {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan T)
	
	go func() {
		defer close(ch)
		
		yield := func(value T) bool {
			select {
			case ch <- value:
				return true
			case <-ctx.Done():
				return false
			}
		}
		
		fn(ctx, yield)
	}()
	
	return Generator[T]{
		ch:     ch,
		cancel: cancel,
		ctx:    ctx,
	}
}

// Next returns the next value from the generator
func (g Generator[T]) Next() (T, bool) {
	value, ok := <-g.ch
	return value, ok
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

// Range generates integers from start to end (inclusive)
func Range(start, end int) Generator[int] {
	return NewGenerator(func(ctx context.Context, yield func(int) bool) {
		for i := start; i <= end; i++ {
			if !yield(i) {
				return
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
		for _, value := range slice {
			if !yield(value) {
				return
			}
		}
	})
}

// Fibonacci generates Fibonacci numbers
func Fibonacci() Generator[int] {
	return NewGenerator(func(ctx context.Context, yield func(int) bool) {
		a, b := 0, 1
		
		if !yield(a) {
			return
		}
		
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if !yield(b) {
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
			case t := <-ticker.C:
				if !yield(t) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	})
}

// Map transforms each value using the provided function
func Map[T, U any](gen Generator[T], fn func(T) U) Generator[U] {
	return NewGenerator(func(ctx context.Context, yield func(U) bool) {
		for {
			select {
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				transformed := fn(value)
				if !yield(transformed) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	})
}

// Filter keeps only values that match the predicate
func Filter[T any](gen Generator[T], predicate func(T) bool) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		for {
			select {
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				if predicate(value) {
					if !yield(value) {
						return
					}
				}
			case <-ctx.Done():
				return
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
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				if !yield(value) {
					return
				}
				count++
			case <-ctx.Done():
				return
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
			case <-ctx.Done():
				return
			}
		}
	})
}

// TakeWhile takes values while the predicate is true
func TakeWhile[T any](gen Generator[T], predicate func(T) bool) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		for {
			select {
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
			case <-ctx.Done():
				return
			}
		}
	})
}

// Chain concatenates multiple generators
func Chain[T any](generators ...Generator[T]) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		for _, gen := range generators {
			for {
				select {
				case value, ok := <-gen.ch:
					if !ok {
						break
					}
					if !yield(value) {
						return
					}
				case <-ctx.Done():
					return
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
			case value1, ok1 := <-gen1.ch:
				if !ok1 {
					return
				}
				select {
				case value2, ok2 := <-gen2.ch:
					if !ok2 {
						return
					}
					pair := Pair[T, U]{First: value1, Second: value2}
					if !yield(pair) {
						return
					}
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	})
}

// Reduce reduces the generator to a single value
func Reduce[T, U any](gen Generator[T], initial U, fn func(U, T) U) U {
	acc := initial
	for value := range gen.ch {
		acc = fn(acc, value)
	}
	return acc
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

// Batch groups values into batches of specified size
func Batch[T any](gen Generator[T], size int) Generator[[]T] {
	return NewGenerator(func(ctx context.Context, yield func([]T) bool) {
		batch := make([]T, 0, size)
		
		for {
			select {
			case value, ok := <-gen.ch:
				if !ok {
					// Send remaining batch if not empty
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
			case <-ctx.Done():
				return
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
			case <-ctx.Done():
				return
			}
		}
	})
}

// Parallel processes values in parallel
func Parallel[T, U any](gen Generator[T], fn func(T) U, workers int) Generator[U] {
	return NewGenerator(func(ctx context.Context, yield func(U) bool) {
		input := make(chan T, workers)
		output := make(chan U, workers)
		
		var wg sync.WaitGroup
		
		// Start workers
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for value := range input {
					result := fn(value)
					select {
					case output <- result:
					case <-ctx.Done():
						return
					}
				}
			}()
		}
		
		// Feed input
		go func() {
			defer close(input)
			for {
				select {
				case value, ok := <-gen.ch:
					if !ok {
						return
					}
					select {
					case input <- value:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
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
		
		// Buffer input
		go func() {
			defer close(buffer)
			for {
				select {
				case value, ok := <-gen.ch:
					if !ok {
						return
					}
					select {
					case buffer <- value:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
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

// Advanced generators and utilities

// Merge merges multiple generators into one (values may be interleaved)
func Merge[T any](generators ...Generator[T]) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		var wg sync.WaitGroup
		merged := make(chan T)
		
		for _, gen := range generators {
			wg.Add(1)
			go func(g Generator[T]) {
				defer wg.Done()
				for {
					select {
					case value, ok := <-g.ch:
						if !ok {
							return
						}
						select {
						case merged <- value:
						case <-ctx.Done():
							return
						}
					case <-ctx.Done():
						return
					}
				}
			}(gen)
		}
		
		go func() {
			wg.Wait()
			close(merged)
		}()
		
		for value := range merged {
			if !yield(value) {
				return
			}
		}
	})
}

// Throttle limits the rate of value emission
func Throttle[T any](gen Generator[T], rate time.Duration) Generator[T] {
	return NewGenerator(func(ctx context.Context, yield func(T) bool) {
		ticker := time.NewTicker(rate)
		defer ticker.Stop()
		
		for {
			select {
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				
				// Wait for ticker
				select {
				case <-ticker.C:
					if !yield(value) {
						return
					}
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	})
}

// WithIndex adds an index to each value
func WithIndex[T any](gen Generator[T]) Generator[Pair[int, T]] {
	return NewGenerator(func(ctx context.Context, yield func(Pair[int, T]) bool) {
		index := 0
		for {
			select {
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				pair := Pair[int, T]{First: index, Second: value}
				if !yield(pair) {
					return
				}
				index++
			case <-ctx.Done():
				return
			}
		}
	})
}

// Scan accumulates values (like Reduce but emits intermediate results)
func Scan[T, U any](gen Generator[T], initial U, fn func(U, T) U) Generator[U] {
	return NewGenerator(func(ctx context.Context, yield func(U) bool) {
		acc := initial
		if !yield(acc) {
			return
		}
		
		for {
			select {
			case value, ok := <-gen.ch:
				if !ok {
					return
				}
				acc = fn(acc, value)
				if !yield(acc) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	})
}