

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

// NewBufferPool creates a new BufferPool
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// TODO: ここに実装を追加してください
				//
				// 実装の流れ:
				// 1. 新しいbytes.Bufferを作成
				// 2. 適切な初期容量を設定
				return &bytes.Buffer{}
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() *bytes.Buffer {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. poolからオブジェクトを取得
	// 2. *bytes.Bufferに型アサーション
	// 3. バッファをリセット
	return bp.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. バッファが大きすぎる場合は破棄
	// 2. バッファをリセット
	// 3. poolに戻す
	buf.Reset()
	bp.pool.Put(buf)
}

// WorkerData represents data processed by workers
type WorkerData struct {
	ID       int
	Payload  []byte
	Metadata map[string]string
	Results  []float64
}

// Reset resets the WorkerData to its zero state
func (wd *WorkerData) Reset() {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. IDをゼロに設定
	// 2. Payloadをnil化（大きなスライスは破棄）
	// 3. Metadataマップをクリア
	// 4. Resultsスライスをクリア
	wd.ID = 0
	wd.Payload = nil

	for k := range wd.Metadata {
		delete(wd.Metadata, k)
	}

	wd.Results = wd.Results[:0] // Clear the slice without reallocating
}

// WorkerDataPool manages a pool of WorkerData structs
type WorkerDataPool struct {
	pool sync.Pool
}

// NewWorkerDataPool creates a new WorkerDataPool
func NewWorkerDataPool() *WorkerDataPool {
	return &WorkerDataPool{
		pool: sync.Pool{
			New: func() interface{} {
				// TODO: ここに実装を追加してください
				//
				// 実装の流れ:
				// 1. 新しいWorkerDataを作成
				// 2. マップとスライスを初期化
				return &WorkerData{
					Payload:  make([]byte, 0, 1024),
					Metadata: make(map[string]string),
					Results:  make([]float64, 0, 10),
				}
			},
		},
	}
}

// Get retrieves a WorkerData from the pool
func (wdp *WorkerDataPool) Get() *WorkerData {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. poolからオブジェクトを取得
	// 2. *WorkerDataに型アサーション
	// 3. 必要に応じて初期化
	return wdp.pool.Get().(*WorkerData)
}

// Put returns a WorkerData to the pool
func (wdp *WorkerDataPool) Put(wd *WorkerData) {
	if wd == nil {
		return
	}

	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. オブジェクトの状態をリセット
	// 2. poolに戻す
	wd.Reset()
	wdp.pool.Put(wd)
}

// SlicePool manages pools of slices with different capacities
type SlicePool struct {
	pools map[int]*sync.Pool // key: capacity range, value: pool
	mu    sync.RWMutex
}

// NewSlicePool creates a new SlicePool
func NewSlicePool() *SlicePool {
	return &SlicePool{
		pools: make(map[int]*sync.Pool),
	}
}

// getPoolForCapacity returns the appropriate pool for the given capacity
func (sp *SlicePool) getPoolForCapacity(capacity int) *sync.Pool {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 容量を適切なバケットサイズに丸める（例：32, 64, 128, 256...）
	// 2. 該当するプールが存在しない場合は作成
	// 3. 読み取りロック→書き込みロックの適切な使い分け
	bucketSize := roundUpToPowerOf2(capacity)

	sp.mu.RLock()
	pool, exists := sp.pools[bucketSize]
	sp.mu.RUnlock()

	if exists {
		return pool
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	if exists {
		return pool
	}

	pool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, bucketSize)
		},
	}
	sp.pools[bucketSize] = pool
	return pool
}

// roundUpToPowerOf2 rounds up to the next power of 2
func roundUpToPowerOf2(n int) int {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. nが2の累乗かチェック
	// 2. 次の2の累乗を計算
	// 3. 最小サイズ（例：32）を保証
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

// GetSlice retrieves a slice from the appropriate pool
func (sp *SlicePool) GetSlice(capacity int) []byte {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 適切なプールを取得
	// 2. プールからスライスを取得
	// 3. 容量をチェックして調整
	pool := sp.getPoolForCapacity(capacity)
	slice := pool.Get().([]byte)

	return slice[:0] // Reset length but keep capacity
}

// PutSlice returns a slice to the appropriate pool
func (sp *SlicePool) PutSlice(slice []byte) {
	if slice == nil {
		return
	}

	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. スライスが大きすぎる場合は破棄
	// 2. スライスをクリア（セキュリティ上の理由）
	// 3. 適切なプールに戻す
	if cap(slice) > 1<<20 { // 1MB limit
		return
	}

	slice = slice[:0] // Reset length but keep capacity
	pool := sp.getPoolForCapacity(cap(slice))
	pool.Put(slice)
}

// ProcessingService demonstrates object pooling in a service
type ProcessingService struct {
	bufferPool     *BufferPool
	workerDataPool *WorkerDataPool
	slicePool      *SlicePool
}

// NewProcessingService creates a new ProcessingService
func NewProcessingService() *ProcessingService {
	return &ProcessingService{
		bufferPool:     NewBufferPool(),
		workerDataPool: NewWorkerDataPool(),
		slicePool:      NewSlicePool(),
	}
}

// ProcessData processes data using pooled objects
func (ps *ProcessingService) ProcessData(inputData []byte) (string, error) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. バッファプールからバッファを取得
	// 2. ワーカーデータプールからWorkerDataを取得
	// 3. スライスプールから作業用スライスを取得
	// 4. データ処理を実行
	// 5. すべてのオブジェクトをプールに戻す（defer使用）
	buf := ps.bufferPool.Get()
	defer ps.bufferPool.Put(buf)
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
	processed := simulateHeavyProcessing(wd.Payload, workSlice)

	buf.Write(processed)
	buf.WriteString("-processed")

	return buf.String(), nil
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

// ProcessWithoutPool processes data without using object pools (for comparison)
func ProcessWithoutPool(inputData []byte) (string, error) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 毎回新しいバッファを作成
	// 2. 毎回新しいWorkerDataを作成
	// 3. 毎回新しいスライスを作成
	// 4. 同じ処理を実行

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

// PoolStats provides statistics about pool usage
type PoolStats struct {
	BufferPoolHits   int64
	WorkerPoolHits   int64
	SlicePoolHits    int64
	TotalAllocations int64
}

// GetStats returns current pool statistics (stub for demonstration)
func (ps *ProcessingService) GetStats() PoolStats {
	// 実際の実装では適切な統計情報を収集
	return PoolStats{}
}

func main() {
	// テスト用のサンプル実行
	service := NewProcessingService()

	testData := []byte("Hello, World! This is test data for processing.")
	result, err := service.ProcessData(testData)
	if err != nil {
		println("Error:", err.Error())
	} else {
		println("Result:", result)
	}
}
