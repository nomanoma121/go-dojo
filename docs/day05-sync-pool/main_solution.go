package main

import (
	"bytes"
	"sync"
	"time"
)

// BufferPool manages a pool of bytes.Buffer for efficient reuse
type BufferPool struct {
	pool sync.Pool
}

// WorkerData represents data processed by workers
type WorkerData struct {
	ID       int
	Payload  []byte
	Metadata map[string]string
	Results  []float64
}

// WorkerDataPool manages a pool of WorkerData structs
type WorkerDataPool struct {
	pool sync.Pool
}

// SlicePool manages pools of slices with different capacities
type SlicePool struct {
	pools map[int]*sync.Pool // key: capacity range, value: pool
	mu    sync.RWMutex
}

// ProcessingService demonstrates object pooling in a service
type ProcessingService struct {
	bufferPool     *BufferPool
	workerDataPool *WorkerDataPool
	slicePool      *SlicePool
}

// PoolStats provides statistics about pool usage
type PoolStats struct {
	BufferPoolHits   int64
	WorkerPoolHits   int64
	SlicePoolHits    int64
	TotalAllocations int64
}

// BufferPool の実装
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}
}

func (bp *BufferPool) Get() *bytes.Buffer {
	buf := bp.pool.Get().(*bytes.Buffer)
	buf.Reset() // バッファをクリア
	return buf
}

func (bp *BufferPool) Put(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	
	// 大きすぎるバッファは破棄
	const maxSize = 1 << 20 // 1MB
	if buf.Cap() > maxSize {
		return
	}
	
	buf.Reset()
	bp.pool.Put(buf)
}

// WorkerData の実装
func (wd *WorkerData) Reset() {
	wd.ID = 0
	wd.Payload = nil
	
	// マップをクリア
	for k := range wd.Metadata {
		delete(wd.Metadata, k)
	}
	
	// スライスをクリア
	wd.Results = wd.Results[:0]
}

// WorkerDataPool の実装
func NewWorkerDataPool() *WorkerDataPool {
	return &WorkerDataPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &WorkerData{
					Metadata: make(map[string]string),
					Results:  make([]float64, 0, 10),
				}
			},
		},
	}
}

func (wdp *WorkerDataPool) Get() *WorkerData {
	wd := wdp.pool.Get().(*WorkerData)
	wd.Reset()
	return wd
}

func (wdp *WorkerDataPool) Put(wd *WorkerData) {
	if wd == nil {
		return
	}
	wd.Reset()
	wdp.pool.Put(wd)
}

// SlicePool の実装
func NewSlicePool() *SlicePool {
	return &SlicePool{
		pools: make(map[int]*sync.Pool),
	}
}

func (sp *SlicePool) getPoolForCapacity(capacity int) *sync.Pool {
	bucketSize := roundUpToPowerOf2(capacity)
	
	sp.mu.RLock()
	pool, exists := sp.pools[bucketSize]
	sp.mu.RUnlock()
	
	if exists {
		return pool
	}
	
	sp.mu.Lock()
	defer sp.mu.Unlock()
	
	// Double-checked locking
	if pool, exists := sp.pools[bucketSize]; exists {
		return pool
	}
	
	pool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, bucketSize)
		},
	}
	sp.pools[bucketSize] = pool
	return pool
}

func roundUpToPowerOf2(n int) int {
	if n <= 32 {
		return 32
	}
	
	// Find next power of 2
	power := 1
	for power < n {
		power <<= 1
	}
	return power
}

func (sp *SlicePool) GetSlice(capacity int) []byte {
	pool := sp.getPoolForCapacity(capacity)
	slice := pool.Get().([]byte)
	
	if cap(slice) < capacity {
		return make([]byte, 0, capacity)
	}
	
	return slice[:0] // Reset length but keep capacity
}

func (sp *SlicePool) PutSlice(slice []byte) {
	if slice == nil || cap(slice) > 1<<20 { // 1MB limit
		return
	}
	
	// Clear slice for security
	for i := range slice {
		slice[i] = 0
	}
	
	pool := sp.getPoolForCapacity(cap(slice))
	pool.Put(slice[:0])
}

// ProcessingService の実装
func NewProcessingService() *ProcessingService {
	return &ProcessingService{
		bufferPool:     NewBufferPool(),
		workerDataPool: NewWorkerDataPool(),
		slicePool:      NewSlicePool(),
	}
}

func (ps *ProcessingService) ProcessData(inputData []byte) (string, error) {
	// バッファプールからバッファを取得
	buf := ps.bufferPool.Get()
	defer ps.bufferPool.Put(buf)
	
	// ワーカーデータプールからWorkerDataを取得
	wd := ps.workerDataPool.Get()
	defer ps.workerDataPool.Put(wd)
	
	// スライスプールから作業用スライスを取得
	workSlice := ps.slicePool.GetSlice(len(inputData) * 2)
	defer ps.slicePool.PutSlice(workSlice)
	
	// データ処理を実行
	wd.ID = 1
	wd.Payload = inputData
	wd.Metadata["processing_time"] = time.Now().Format(time.RFC3339)
	
	// 実際の処理
	processed := simulateHeavyProcessing(inputData, workSlice)
	
	buf.Write(processed)
	buf.WriteString("-processed")
	
	return buf.String(), nil
}

// ProcessWithoutPool processes data without using object pools (for comparison)
func ProcessWithoutPool(inputData []byte) (string, error) {
	// 毎回新しいオブジェクトを作成
	buf := &bytes.Buffer{}
	wd := &WorkerData{
		Metadata: make(map[string]string),
		Results:  make([]float64, 0),
	}
	workSlice := make([]byte, len(inputData)*2)
	
	// 同じ処理を実行
	wd.ID = 1
	wd.Payload = inputData
	wd.Metadata["processing_time"] = time.Now().Format(time.RFC3339)
	
	processed := simulateHeavyProcessing(inputData, workSlice)
	
	buf.Write(processed)
	buf.WriteString("-processed")
	
	return buf.String(), nil
}

// GetStats returns current pool statistics (stub for demonstration)
func (ps *ProcessingService) GetStats() PoolStats {
	// 実際の実装では適切な統計情報を収集
	return PoolStats{}
}

// simulateHeavyProcessing simulates CPU-intensive work
func simulateHeavyProcessing(data []byte, workSlice []byte) []byte {
	// データ変換の模擬
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ 0xFF // 簡単な変換
	}
	return result
}