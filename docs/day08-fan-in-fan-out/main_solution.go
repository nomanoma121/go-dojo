package main

import (
	"context"
	"fmt"
	"sync"
)

// DataItem represents a piece of data flowing through the pipeline
type DataItem struct {
	ID    int
	Value interface{}
	Stage string
}

// Pipeline represents a data processing pipeline
type Pipeline struct {
	input   chan DataItem
	output  chan DataItem
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewPipeline creates a new pipeline
func NewPipeline(workers int, bufferSize int) *Pipeline {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pipeline{
		input:   make(chan DataItem, bufferSize),
		output:  make(chan DataItem, bufferSize),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Submit submits a data item to the pipeline
func (p *Pipeline) Submit(item DataItem) error {
	select {
	case p.input <- item:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("pipeline is stopped")
	}
}

// GetOutput gets an output item from the pipeline
func (p *Pipeline) GetOutput() (DataItem, bool) {
	select {
	case item, ok := <-p.output:
		return item, ok
	case <-p.ctx.Done():
		return DataItem{}, false
	}
}

// Stop stops the pipeline
func (p *Pipeline) Stop() {
	p.cancel()
	close(p.input)
	p.wg.Wait()
	close(p.output)
}

// MultiStagePipeline represents a multi-stage pipeline
type MultiStagePipeline struct {
	stages []Stage
	input  chan DataItem
	output chan DataItem
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Stage represents a processing stage
type Stage struct {
	Name      string
	Workers   int
	Input     chan DataItem
	Output    chan DataItem
	Transform func(DataItem) DataItem
}


// Start starts the pipeline
func (p *Pipeline) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// worker processes data items
func (p *Pipeline) worker(workerID int) {
	defer p.wg.Done()
	
	for {
		select {
		case item, ok := <-p.input:
			if !ok {
				return // Input channel closed
			}
			
			// Process the item
			processed := p.processItem(item, workerID)
			
			// Send to output
			select {
			case p.output <- processed:
			case <-p.ctx.Done():
				return
			}
			
		case <-p.ctx.Done():
			return
		}
	}
}

// processItem processes a single data item
func (p *Pipeline) processItem(item DataItem, workerID int) DataItem {
	// Simulate processing work
	switch v := item.Value.(type) {
	case string:
		item.Value = v + "-processed"
	case int:
		item.Value = v * 2
	default:
		item.Value = "processed"
	}
	
	item.Stage = "processed"
	return item
}

// FanOut distributes data from one channel to multiple channels
func FanOut(ctx context.Context, input <-chan DataItem, numOutputs int) []<-chan DataItem {
	outputs := make([]chan DataItem, numOutputs)
	readOnlyOutputs := make([]<-chan DataItem, numOutputs)
	
	// Create output channels
	for i := 0; i < numOutputs; i++ {
		outputs[i] = make(chan DataItem)
		readOnlyOutputs[i] = outputs[i]
	}
	
	// Start distribution goroutine
	go func() {
		defer func() {
			for _, output := range outputs {
				close(output)
			}
		}()
		
		index := 0
		for {
			select {
			case item, ok := <-input:
				if !ok {
					return // Input closed
				}
				
				// Round-robin distribution
				targetOutput := outputs[index%numOutputs]
				index++
				
				select {
				case targetOutput <- item:
				case <-ctx.Done():
					return
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return readOnlyOutputs
}

// FanIn merges multiple channels into one channel
func FanIn(ctx context.Context, inputs ...<-chan DataItem) <-chan DataItem {
	output := make(chan DataItem)
	
	var wg sync.WaitGroup
	
	// Start a goroutine for each input channel
	for _, input := range inputs {
		wg.Add(1)
		go func(in <-chan DataItem) {
			defer wg.Done()
			for {
				select {
				case item, ok := <-in:
					if !ok {
						return // Channel closed
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
		}(input)
	}
	
	// Close output when all inputs are done
	go func() {
		wg.Wait()
		close(output)
	}()
	
	return output
}

// NewMultiStagePipeline creates a multi-stage pipeline
func NewMultiStagePipeline(transforms ...func(DataItem) DataItem) *MultiStagePipeline {
	ctx, cancel := context.WithCancel(context.Background())
	
	stages := make([]Stage, len(transforms))
	for i, transform := range transforms {
		stages[i] = Stage{
			Name:      fmt.Sprintf("Stage-%d", i),
			Transform: transform,
		}
	}
	
	return &MultiStagePipeline{
		stages: stages,
		input:  make(chan DataItem),
		output: make(chan DataItem),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the multi-stage pipeline
func (msp *MultiStagePipeline) Start() {
	if len(msp.stages) == 0 {
		// No stages, just pass through
		go func() {
			for item := range msp.input {
				msp.output <- item
			}
			close(msp.output)
		}()
		return
	}
	
	// Create intermediate channels
	channels := make([]chan DataItem, len(msp.stages)+1)
	channels[0] = msp.input
	channels[len(msp.stages)] = msp.output
	
	for i := 1; i < len(msp.stages); i++ {
		channels[i] = make(chan DataItem)
	}
	
	// Start stage processors
	for i, stage := range msp.stages {
		go func(stageFunc func(DataItem) DataItem, in <-chan DataItem, out chan<- DataItem) {
			defer close(out)
			for item := range in {
				processed := stageFunc(item)
				out <- processed
			}
		}(stage.Transform, channels[i], channels[i+1])
	}
}

// Submit submits data to the multi-stage pipeline
func (msp *MultiStagePipeline) Submit(item DataItem) error {
	select {
	case msp.input <- item:
		return nil
	case <-msp.ctx.Done():
		return fmt.Errorf("multi-stage pipeline is stopped")
	}
}

// GetOutput gets processed data from the pipeline
func (msp *MultiStagePipeline) GetOutput() (DataItem, bool) {
	item, ok := <-msp.output
	return item, ok
}

// Stop stops the multi-stage pipeline
func (msp *MultiStagePipeline) Stop() {
	close(msp.input)
}

// LoadBalancedFanOut distributes work based on worker load
func LoadBalancedFanOut(ctx context.Context, input <-chan DataItem, numWorkers int) []<-chan DataItem {
	outputs := make([]chan DataItem, numWorkers)
	readOnlyOutputs := make([]<-chan DataItem, numWorkers)
	loads := make([]int, numWorkers) // Track load per worker
	var loadMutex sync.Mutex
	
	// Create output channels
	for i := 0; i < numWorkers; i++ {
		outputs[i] = make(chan DataItem, 10) // Buffered for load balancing
		readOnlyOutputs[i] = outputs[i]
	}
	
	// Distribution logic
	go func() {
		defer func() {
			for _, output := range outputs {
				close(output)
			}
		}()
		
		for {
			select {
			case item, ok := <-input:
				if !ok {
					return
				}
				
				// Find worker with minimum load
				loadMutex.Lock()
				minLoadWorker := 0
				minLoad := loads[0]
				for i := 1; i < numWorkers; i++ {
					if loads[i] < minLoad {
						minLoad = loads[i]
						minLoadWorker = i
					}
				}
				loads[minLoadWorker]++
				loadMutex.Unlock()
				
				// Send to selected worker
				select {
				case outputs[minLoadWorker] <- item:
					// Decrement load when item is processed (simplified)
					go func(workerID int) {
						loadMutex.Lock()
						loads[workerID]--
						loadMutex.Unlock()
					}(minLoadWorker)
				case <-ctx.Done():
					return
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return readOnlyOutputs
}

// PriorityFanOut distributes work based on priority
func PriorityFanOut(ctx context.Context, input <-chan DataItem, numWorkers int) []<-chan DataItem {
	outputs := make([]chan DataItem, numWorkers)
	readOnlyOutputs := make([]<-chan DataItem, numWorkers)
	
	for i := 0; i < numWorkers; i++ {
		outputs[i] = make(chan DataItem)
		readOnlyOutputs[i] = outputs[i]
	}
	
	go func() {
		defer func() {
			for _, output := range outputs {
				close(output)
			}
		}()
		
		for {
			select {
			case item, ok := <-input:
				if !ok {
					return
				}
				
				// Determine worker based on item priority or ID
				workerID := item.ID % numWorkers
				
				select {
				case outputs[workerID] <- item:
				case <-ctx.Done():
					return
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return readOnlyOutputs
}