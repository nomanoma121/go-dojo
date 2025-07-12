package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AddStage adds a processing stage to the pipeline
func (ep *ErrorPipeline) AddStage(stage PipelineStage) {
	// Wrap the stage with error handling
	wrappedStage := ep.wrapStageWithErrorHandling(stage, fmt.Sprintf("stage-%d", len(ep.stages)))
	ep.stages = append(ep.stages, wrappedStage)
}

// Process processes data through the pipeline
func (ep *ErrorPipeline) Process(input <-chan DataItem) <-chan DataItem {
	if len(ep.stages) == 0 {
		// No stages, just pass through
		output := make(chan DataItem)
		go func() {
			defer close(output)
			for item := range input {
				select {
				case output <- item:
				case <-ep.ctx.Done():
					return
				}
			}
		}()
		return output
	}
	
	// Chain stages together
	current := input
	for _, stage := range ep.stages {
		current = stage(ep.ctx, current)
	}
	
	return current
}

// Stop stops the pipeline
func (ep *ErrorPipeline) Stop() {
	ep.cancel()
	ep.wg.Wait()
	close(ep.errorChan)
}

// wrapStageWithErrorHandling wraps a stage with error handling
func (ep *ErrorPipeline) wrapStageWithErrorHandling(stage PipelineStage, stageName string) PipelineStage {
	return func(ctx context.Context, input <-chan DataItem) <-chan DataItem {
		output := make(chan DataItem)
		
		ep.wg.Add(1)
		go func() {
			defer close(output)
			defer ep.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					select {
					case ep.errorChan <- PipelineError{
						Stage:     stageName,
						Error:     fmt.Errorf("panic recovered: %v", r),
						Timestamp: time.Now(),
						Retryable: false,
					}:
					case <-ctx.Done():
					}
				}
			}()
			
			// Execute the original stage and handle its output
			stageOutput := stage(ctx, input)
			for {
				select {
				case item, ok := <-stageOutput:
					if !ok {
						return
					}
					select {
					case output <- item:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
		
		return output
	}
}

// RetryableStage creates a stage that can retry on failure
func RetryableStage(processor func(DataItem) (DataItem, error), maxRetries int, stageName string) PipelineStage {
	return func(ctx context.Context, input <-chan DataItem) <-chan DataItem {
		output := make(chan DataItem)
		
		go func() {
			defer close(output)
			
			for {
				select {
				case item, ok := <-input:
					if !ok {
						return
					}
					
					var processed DataItem
					var err error
					
					// Retry logic
					for attempt := 0; attempt <= maxRetries; attempt++ {
						processed, err = processor(item)
						if err == nil {
							break
						}
						
						// If this was the last attempt, give up
						if attempt == maxRetries {
							// Could send error to error channel here if we had access to it
							continue
						}
						
						// Wait before retry with exponential backoff
						backoff := time.Duration(attempt+1) * 100 * time.Millisecond
						select {
						case <-time.After(backoff):
						case <-ctx.Done():
							return
						}
					}
					
					// Send successful result
					if err == nil {
						select {
						case output <- processed:
						case <-ctx.Done():
							return
						}
					}
					
				case <-ctx.Done():
					return
				}
			}
		}()
		
		return output
	}
}

// Collect collects an error
func (ec *ErrorCollector) Collect(err PipelineError) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	
	ec.errors = append(ec.errors, err)
	
	// Remove oldest errors if we exceed max
	if len(ec.errors) > ec.maxErrors {
		ec.errors = ec.errors[len(ec.errors)-ec.maxErrors:]
	}
}

// GetErrors returns all collected errors
func (ec *ErrorCollector) GetErrors() []PipelineError {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	result := make([]PipelineError, len(ec.errors))
	copy(result, ec.errors)
	return result
}

// GetErrorsByStage returns errors filtered by stage
func (ec *ErrorCollector) GetErrorsByStage(stage string) []PipelineError {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	
	var result []PipelineError
	for _, err := range ec.errors {
		if err.Stage == stage {
			result = append(result, err)
		}
	}
	return result
}

// Clear clears all collected errors
func (ec *ErrorCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	
	ec.errors = ec.errors[:0]
}

// Enhanced ValidationStage with actual validation logic
func ValidationStage(ctx context.Context, input <-chan DataItem) <-chan DataItem {
	output := make(chan DataItem)
	
	go func() {
		defer close(output)
		for {
			select {
			case item, ok := <-input:
				if !ok {
					return
				}
				
				// Validation logic
				if item.Data == nil {
					// Invalid data - could report error here
					continue
				}
				
				if str, ok := item.Data.(string); ok && str == "" {
					// Empty string is invalid
					continue
				}
				
				// Data is valid
				select {
				case output <- item:
				case <-ctx.Done():
					return
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return output
}

// Enhanced TransformStage with actual transformation logic
func TransformStage(ctx context.Context, input <-chan DataItem) <-chan DataItem {
	output := make(chan DataItem)
	
	go func() {
		defer close(output)
		for {
			select {
			case item, ok := <-input:
				if !ok {
					return
				}
				
				// Transform data
				switch v := item.Data.(type) {
				case string:
					item.Data = "transformed-" + v
				case int:
					item.Data = v * 2
				default:
					item.Data = "transformed"
				}
				
				// Add transform metadata
				if item.Metadata == nil {
					item.Metadata = make(map[string]string)
				}
				item.Metadata["transformed"] = "true"
				item.Metadata["transform_time"] = time.Now().Format(time.RFC3339)
				
				select {
				case output <- item:
				case <-ctx.Done():
					return
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return output
}

// Enhanced EnrichmentStage with actual enrichment logic
func EnrichmentStage(ctx context.Context, input <-chan DataItem) <-chan DataItem {
	output := make(chan DataItem)
	
	go func() {
		defer close(output)
		for {
			select {
			case item, ok := <-input:
				if !ok {
					return
				}
				
				// Simulate enrichment (adding metadata)
				if item.Metadata == nil {
					item.Metadata = make(map[string]string)
				}
				
				// Add enrichment data
				item.Metadata["enriched"] = "true"
				item.Metadata["enrichment_time"] = time.Now().Format(time.RFC3339)
				item.Metadata["enrichment_source"] = "external_api"
				
				// Simulate external API call delay
				select {
				case <-time.After(10 * time.Millisecond):
				case <-ctx.Done():
					return
				}
				
				// Simulate occasional enrichment failure
				if item.ID%10 == 9 {
					// Skip this item (simulate enrichment failure)
					continue
				}
				
				select {
				case output <- item:
				case <-ctx.Done():
					return
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return output
}

// Advanced error recovery pipeline
func NewErrorRecoveryPipeline() *ErrorPipeline {
	ctx, cancel := context.WithCancel(context.Background())
	return &ErrorPipeline{
		stages:    make([]PipelineStage, 0),
		errorChan: make(chan PipelineError, 1000), // Larger buffer for error bursts
		ctx:       ctx,
		cancel:    cancel,
	}
}

// CircuitBreakerStage implements circuit breaker pattern for error recovery
func CircuitBreakerStage(processor func(DataItem) (DataItem, error), threshold int, timeout time.Duration) PipelineStage {
	var (
		failures   int
		lastFailure time.Time
		state      = "closed" // closed, open, half-open
		mu         sync.Mutex
	)
	
	return func(ctx context.Context, input <-chan DataItem) <-chan DataItem {
		output := make(chan DataItem)
		
		go func() {
			defer close(output)
			
			for {
				select {
				case item, ok := <-input:
					if !ok {
						return
					}
					
					mu.Lock()
					
					// Check circuit breaker state
					if state == "open" {
						if time.Since(lastFailure) > timeout {
							state = "half-open"
						} else {
							mu.Unlock()
							continue // Skip processing
						}
					}
					
					mu.Unlock()
					
					// Try to process
					processed, err := processor(item)
					
					mu.Lock()
					if err != nil {
						failures++
						lastFailure = time.Now()
						
						if failures >= threshold {
							state = "open"
						}
						mu.Unlock()
						continue
					}
					
					// Success
					if state == "half-open" {
						state = "closed"
						failures = 0
					}
					mu.Unlock()
					
					select {
					case output <- processed:
					case <-ctx.Done():
						return
					}
					
				case <-ctx.Done():
					return
				}
			}
		}()
		
		return output
	}
}