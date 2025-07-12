package main

import (
	"context"
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

// Start starts the pipeline
func (p *Pipeline) Start() {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. workers分のGoroutineを起動
	// 2. fan-out: inputから複数のワーカーにデータを分散
	// 3. fan-in: 複数のワーカーからの出力を一つのoutputに集約
}

// Submit submits data to the pipeline
func (p *Pipeline) Submit(item DataItem) error {
	// TODO: 実装してください
	select {
	case p.input <- item:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// GetOutput gets processed data from the pipeline
func (p *Pipeline) GetOutput() (DataItem, bool) {
	// TODO: 実装してください
	select {
	case item := <-p.output:
		return item, true
	case <-p.ctx.Done():
		return DataItem{}, false
	default:
		return DataItem{}, false
	}
}

// Stop stops the pipeline
func (p *Pipeline) Stop() {
	// TODO: 実装してください
	p.cancel()
	close(p.input)
	p.wg.Wait()
	close(p.output)
}

// worker processes data items
func (p *Pipeline) worker(workerID int) {
	defer p.wg.Done()
	
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. inputからデータを受信
	// 2. データを処理
	// 3. outputに結果を送信
	// 4. コンテキストキャンセルを監視
}

// processItem processes a single data item
func (p *Pipeline) processItem(item DataItem, workerID int) DataItem {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. データの変換処理
	// 2. ワーカーIDを追加
	// 3. ステージ情報を更新
	
	return DataItem{
		ID:    item.ID,
		Value: item.Value,
		Stage: "processed",
	}
}

// FanOut distributes data from one channel to multiple channels
func FanOut(ctx context.Context, input <-chan DataItem, numOutputs int) []<-chan DataItem {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. numOutputs分のチャネルを作成
	// 2. 入力データを複数のチャネルに分散
	// 3. ラウンドロビンまたはランダムで分散
	
	outputs := make([]<-chan DataItem, numOutputs)
	return outputs
}

// FanIn merges multiple channels into one channel
func FanIn(ctx context.Context, inputs ...<-chan DataItem) <-chan DataItem {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 出力用チャネルを作成
	// 2. 各入力チャネルからGoroutineで読み取り
	// 3. すべてを一つの出力チャネルにマージ
	
	output := make(chan DataItem)
	return output
}

// MultiStagePipeline represents a multi-stage processing pipeline
type MultiStagePipeline struct {
	stages []func(DataItem) DataItem
	input  chan DataItem
	output chan DataItem
}

// NewMultiStagePipeline creates a multi-stage pipeline
func NewMultiStagePipeline(stages ...func(DataItem) DataItem) *MultiStagePipeline {
	// TODO: 実装してください
	return nil
}

// Start starts the multi-stage pipeline
func (msp *MultiStagePipeline) Start() {
	// TODO: 実装してください
	//
	// 実装の流れ:
	// 1. 各ステージ間をチャネルで接続
	// 2. 各ステージをGoroutineで実行
	// 3. データの流れを制御
}

func main() {
	// テスト用のサンプル実行
	pipeline := NewPipeline(3, 10)
	pipeline.Start()
	
	// データを送信
	for i := 0; i < 5; i++ {
		item := DataItem{
			ID:    i,
			Value: i * 2,
			Stage: "input",
		}
		pipeline.Submit(item)
	}
	
	// 結果を受信
	for i := 0; i < 5; i++ {
		if result, ok := pipeline.GetOutput(); ok {
			println("Processed:", result.ID, result.Stage)
		}
	}
	
	pipeline.Stop()
}