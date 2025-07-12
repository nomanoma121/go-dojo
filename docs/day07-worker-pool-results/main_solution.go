package main

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Task represents a unit of work
type Task struct {
	ID       int
	Data     interface{}
	Priority int
}

// Result represents the result of processing a task
type Result struct {
	TaskID   int
	Output   interface{}
	Error    error
	Duration time.Duration
}

// ResultCollector collects and manages results from workers
type ResultCollector struct {
	results     map[int]Result
	resultChan  chan Result
	orderedMode bool
	mu          sync.RWMutex
	ctx         context.Context
}

// AggregatedResult represents aggregated results
type AggregatedResult struct {
	TotalTasks    int
	SuccessCount  int
	ErrorCount    int
	Results       []Result
	AggregateData interface{}
}

// ResultAggregator aggregates multiple results
type ResultAggregator struct {
	aggregateFunc func([]Result) interface{}
	mu            sync.Mutex
}

// NewResultAggregator creates a new ResultAggregator
func NewResultAggregator(aggregateFunc func([]Result) interface{}) *ResultAggregator {
	return &ResultAggregator{
		aggregateFunc: aggregateFunc,
	}
}

// NewResultCollector creates a new ResultCollector
func NewResultCollector(ctx context.Context, bufferSize int, orderedMode bool) *ResultCollector {
	return &ResultCollector{
		results:     make(map[int]Result),
		resultChan:  make(chan Result, bufferSize),
		orderedMode: orderedMode,
		ctx:         ctx,
	}
}

// CollectResult collects a result from a worker
func (rc *ResultCollector) CollectResult(result Result) {
	if rc.orderedMode {
		// Store in map for ordered retrieval
		rc.mu.Lock()
		rc.results[result.TaskID] = result
		rc.mu.Unlock()
	} else {
		// Send directly to channel for unordered retrieval
		select {
		case rc.resultChan <- result:
		case <-rc.ctx.Done():
		}
	}
}

// GetResult retrieves a result (ordered or unordered based on mode)
func (rc *ResultCollector) GetResult() (Result, bool) {
	if rc.orderedMode {
		return rc.getOrderedResult()
	}
	
	select {
	case result, ok := <-rc.resultChan:
		return result, ok
	default:
		return Result{}, false
	}
}

// getOrderedResult retrieves results in order
func (rc *ResultCollector) getOrderedResult() (Result, bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	
	if len(rc.results) == 0 {
		return Result{}, false
	}
	
	// Find the lowest TaskID
	var minID int
	first := true
	for id := range rc.results {
		if first || id < minID {
			minID = id
			first = false
		}
	}
	
	result := rc.results[minID]
	delete(rc.results, minID)
	return result, true
}

// GetResults retrieves multiple results
func (rc *ResultCollector) GetResults(count int) []Result {
	results := make([]Result, 0, count)
	
	for i := 0; i < count; i++ {
		if result, ok := rc.GetResult(); ok {
			results = append(results, result)
		} else {
			break
		}
	}
	
	if rc.orderedMode {
		// Sort by TaskID to ensure order
		sort.Slice(results, func(i, j int) bool {
			return results[i].TaskID < results[j].TaskID
		})
	}
	
	return results
}

// GetAllResults retrieves all available results
func (rc *ResultCollector) GetAllResults() []Result {
	var results []Result
	
	if rc.orderedMode {
		rc.mu.Lock()
		for _, result := range rc.results {
			results = append(results, result)
		}
		rc.results = make(map[int]Result) // Clear the map
		rc.mu.Unlock()
		
		// Sort by TaskID
		sort.Slice(results, func(i, j int) bool {
			return results[i].TaskID < results[j].TaskID
		})
	} else {
		// Drain the channel
		for {
			select {
			case result := <-rc.resultChan:
				results = append(results, result)
			default:
				return results
			}
		}
	}
	
	return results
}

// Aggregate aggregates results
func (ra *ResultAggregator) Aggregate(results []Result) AggregatedResult {
	ra.mu.Lock()
	defer ra.mu.Unlock()
	
	totalTasks := len(results)
	successCount := 0
	errorCount := 0
	
	for _, result := range results {
		if result.Error == nil {
			successCount++
		} else {
			errorCount++
		}
	}
	
	var aggregateData interface{}
	if ra.aggregateFunc != nil {
		aggregateData = ra.aggregateFunc(results)
	}
	
	return AggregatedResult{
		TotalTasks:    totalTasks,
		SuccessCount:  successCount,
		ErrorCount:    errorCount,
		Results:       results,
		AggregateData: aggregateData,
	}
}

// AggregateByStatus aggregates results by their status
func (ra *ResultAggregator) AggregateByStatus(results []Result) map[string][]Result {
	statusMap := make(map[string][]Result)
	
	for _, result := range results {
		if result.Error != nil {
			statusMap["error"] = append(statusMap["error"], result)
		} else {
			statusMap["success"] = append(statusMap["success"], result)
		}
	}
	
	return statusMap
}

// CalculateStatistics calculates basic statistics from results
func (ra *ResultAggregator) CalculateStatistics(results []Result) map[string]interface{} {
	if len(results) == 0 {
		return map[string]interface{}{
			"count": 0,
		}
	}
	
	var totalDuration time.Duration
	var minDuration, maxDuration time.Duration
	successCount := 0
	
	for i, result := range results {
		totalDuration += result.Duration
		
		if i == 0 {
			minDuration = result.Duration
			maxDuration = result.Duration
		} else {
			if result.Duration < minDuration {
				minDuration = result.Duration
			}
			if result.Duration > maxDuration {
				maxDuration = result.Duration
			}
		}
		
		if result.Error == nil {
			successCount++
		}
	}
	
	avgDuration := totalDuration / time.Duration(len(results))
	successRate := float64(successCount) / float64(len(results)) * 100
	
	return map[string]interface{}{
		"count":         len(results),
		"success_count": successCount,
		"error_count":   len(results) - successCount,
		"success_rate":  successRate,
		"avg_duration":  avgDuration,
		"min_duration":  minDuration,
		"max_duration":  maxDuration,
		"total_duration": totalDuration,
	}
}

// BatchProcessor processes results in batches
type BatchProcessor struct {
	batchSize int
	processor func([]Result) interface{}
	mu        sync.Mutex
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(batchSize int, processor func([]Result) interface{}) *BatchProcessor {
	return &BatchProcessor{
		batchSize: batchSize,
		processor: processor,
	}
}

// ProcessInBatches processes results in batches
func (bp *BatchProcessor) ProcessInBatches(results []Result) []interface{} {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	var batchResults []interface{}
	
	for i := 0; i < len(results); i += bp.batchSize {
		end := i + bp.batchSize
		if end > len(results) {
			end = len(results)
		}
		
		batch := results[i:end]
		if bp.processor != nil {
			batchResult := bp.processor(batch)
			batchResults = append(batchResults, batchResult)
		}
	}
	
	return batchResults
}

// StreamingResultCollector handles streaming results
type StreamingResultCollector struct {
	output chan Result
	done   chan struct{}
	ctx    context.Context
}

// NewStreamingResultCollector creates a streaming result collector
func NewStreamingResultCollector(ctx context.Context) *StreamingResultCollector {
	return &StreamingResultCollector{
		output: make(chan Result),
		done:   make(chan struct{}),
		ctx:    ctx,
	}
}

// Stream returns the output channel for streaming results
func (src *StreamingResultCollector) Stream() <-chan Result {
	return src.output
}

// CollectResult adds a result to the stream
func (src *StreamingResultCollector) CollectResult(result Result) {
	select {
	case src.output <- result:
	case <-src.ctx.Done():
	case <-src.done:
	}
}

// Close closes the streaming collector
func (src *StreamingResultCollector) Close() {
	close(src.done)
	close(src.output)
}