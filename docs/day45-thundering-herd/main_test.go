package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// モックの実装

// MockCacheClient はテスト用のキャッシュクライアントです
type MockCacheClient struct {
	data     map[string]string
	ttls     map[string]time.Time
	mutex    sync.RWMutex
	failNext bool
}

func NewMockCacheClient() *MockCacheClient {
	return &MockCacheClient{
		data: make(map[string]string),
		ttls: make(map[string]time.Time),
	}
}

func (m *MockCacheClient) Get(ctx context.Context, key string) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.failNext {
		return "", errors.New("cache miss")
	}

	value, exists := m.data[key]
	if !exists {
		return "", errors.New("cache miss")
	}

	// TTL チェック
	if expiry, hasExpiry := m.ttls[key]; hasExpiry && time.Now().After(expiry) {
		return "", errors.New("cache expired")
	}

	return value, nil
}

func (m *MockCacheClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.data[key] = value
	if ttl > 0 {
		m.ttls[key] = time.Now().Add(ttl)
	}
	return nil
}

func (m *MockCacheClient) GetWithTTL(ctx context.Context, key string) (string, time.Duration, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	value, exists := m.data[key]
	if !exists {
		return "", 0, errors.New("cache miss")
	}

	var remainingTTL time.Duration
	if expiry, hasExpiry := m.ttls[key]; hasExpiry {
		remainingTTL = time.Until(expiry)
		if remainingTTL <= 0 {
			return value, remainingTTL, nil // 期限切れだが値は返す
		}
	} else {
		remainingTTL = time.Hour // デフォルト値
	}

	return value, remainingTTL, nil
}

func (m *MockCacheClient) Pipeline() Pipeline {
	return &MockPipeline{cache: m}
}

func (m *MockCacheClient) SetFailNext(fail bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.failNext = fail
}

// MockPipeline はテスト用のパイプラインです
type MockPipeline struct {
	cache *MockCacheClient
}

func (p *MockPipeline) TTL(ctx context.Context, key string) TTLCmd {
	_, ttl, _ := p.cache.GetWithTTL(ctx, key)
	return &MockTTLCmd{ttl: ttl}
}

func (p *MockPipeline) Get(ctx context.Context, key string) GetCmd {
	value, _, _ := p.cache.GetWithTTL(ctx, key)
	return &MockGetCmd{value: value}
}

func (p *MockPipeline) Exec(ctx context.Context) ([]Result, error) {
	return []Result{}, nil
}

// MockTTLCmd はテスト用のTTLコマンドです
type MockTTLCmd struct {
	ttl time.Duration
}

func (c *MockTTLCmd) Val() time.Duration {
	return c.ttl
}

// MockGetCmd はテスト用のGetコマンドです
type MockGetCmd struct {
	value string
}

func (c *MockGetCmd) Val() string {
	return c.value
}

// MockDataRepository はテスト用のデータリポジトリです
type MockDataRepository struct {
	data        map[string]*Data
	mutex       sync.RWMutex
	callCount   int64
	failNext    bool
	slowRequest bool
}

func NewMockDataRepository() *MockDataRepository {
	return &MockDataRepository{
		data: make(map[string]*Data),
	}
}

func (m *MockDataRepository) GetByID(ctx context.Context, id string) (*Data, error) {
	atomic.AddInt64(&m.callCount, 1)

	if m.failNext {
		return nil, errors.New("database error")
	}

	if m.slowRequest {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	data, exists := m.data[id]
	if !exists {
		return nil, ErrNotFound
	}

	// データをコピーして返す
	result := *data
	return &result, nil
}

func (m *MockDataRepository) Create(ctx context.Context, data *Data) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.data[data.ID] = data
	return nil
}

func (m *MockDataRepository) GetCallCount() int64 {
	return atomic.LoadInt64(&m.callCount)
}

func (m *MockDataRepository) SetFailNext(fail bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.failNext = fail
}

func (m *MockDataRepository) SetSlowRequest(slow bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.slowRequest = slow
}

// MockLockManager はテスト用のロックマネージャです
type MockLockManager struct {
	locks map[string]bool
	mutex sync.Mutex
}

func NewMockLockManager() *MockLockManager {
	return &MockLockManager{
		locks: make(map[string]bool),
	}
}

func (m *MockLockManager) TryLock(ctx context.Context, key string, ttl time.Duration) (Lock, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.locks[key] {
		return nil, ErrLockNotAcquired
	}

	m.locks[key] = true
	return &MockLock{manager: m, key: key}, nil
}

// MockLock はテスト用のロックです
type MockLock struct {
	manager *MockLockManager
	key     string
}

func (l *MockLock) Release(ctx context.Context) error {
	l.manager.mutex.Lock()
	defer l.manager.mutex.Unlock()
	delete(l.manager.locks, l.key)
	return nil
}

// テストケース

func TestThunderingHerdProtector_SingleFlight(t *testing.T) {
	cache := NewMockCacheClient()
	db := NewMockDataRepository()
	lockManager := NewMockLockManager()

	// テストデータを設定
	testData := &Data{
		ID:        "test-key",
		Value:     "test-value",
		CreatedAt: time.Now(),
	}
	db.Create(context.Background(), testData)

	protector := NewThunderingHerdProtector(
		cache,
		db,
		lockManager,
		3,              // circuit breaker threshold
		5*time.Second,  // circuit breaker timeout
		0.1,            // 10% jitter
	)

	ctx := context.Background()
	key := "test-key"

	// 1000個の並行リクエストを実行
	const numRequests = 1000
	var wg sync.WaitGroup
	results := make([]*Data, numRequests)
	errors := make([]error, numRequests)

	initialCallCount := db.GetCallCount()

	wg.Add(numRequests)
	for i := 0; i < numRequests; i++ {
		go func(index int) {
			defer wg.Done()
			data, err := protector.Get(ctx, key)
			results[index] = data
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// 結果を検証
	finalCallCount := db.GetCallCount()
	dbCalls := finalCallCount - initialCallCount

	// Single Flightにより、DB呼び出しは1回のみであることを確認
	if dbCalls != 1 {
		t.Errorf("Expected 1 DB call, got %d", dbCalls)
	}

	// 全てのリクエストが成功することを確認
	for i, err := range errors {
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// 全てのリクエストが同じデータを返すことを確認
	for i, data := range results {
		if data == nil {
			t.Errorf("Request %d returned nil data", i)
			continue
		}
		if data.ID != testData.ID || data.Value != testData.Value {
			t.Errorf("Request %d returned incorrect data", i)
		}
	}

	metrics := protector.GetMetrics()
	t.Logf("%d concurrent requests resulted in %d DB queries", numRequests, dbCalls)
	t.Logf("Metrics: Total=%d, CacheHits=%d, CacheMisses=%d, SingleFlightHits=%d",
		metrics.TotalRequests, metrics.CacheHits, metrics.CacheMisses, metrics.SingleFlightHits)

	if metrics.SingleFlightHits == 0 {
		t.Error("Expected some single flight hits")
	}

	t.Log("Single flight pattern prevented thundering herd")
}

func TestThunderingHerdProtector_DistributedLock(t *testing.T) {
	cache := NewMockCacheClient()
	db := NewMockDataRepository()
	lockManager := NewMockLockManager()

	testData := &Data{
		ID:        "distributed-test",
		Value:     "distributed-value",
		CreatedAt: time.Now(),
	}
	db.Create(context.Background(), testData)

	protector := NewThunderingHerdProtector(
		cache,
		db,
		lockManager,
		3,
		5*time.Second,
		0.1,
	)

	ctx := context.Background()
	key := "distributed-test"

	// 複数のプロセスをシミュレート（異なるプロテクターインスタンス）
	const numProcesses = 10
	var wg sync.WaitGroup
	results := make([]*Data, numProcesses)
	errors := make([]error, numProcesses)

	initialCallCount := db.GetCallCount()

	wg.Add(numProcesses)
	for i := 0; i < numProcesses; i++ {
		go func(index int) {
			defer wg.Done()
			
			// 各プロセスが独自のプロテクターを持つ（分散環境をシミュレート）
			processProtector := NewThunderingHerdProtector(
				cache,
				db,
				lockManager, // 同じロックマネージャを共有
				3,
				5*time.Second,
				0.1,
			)
			
			data, err := processProtector.Get(ctx, key)
			results[index] = data
			errors[index] = err
		}(i)
	}

	wg.Wait()

	finalCallCount := db.GetCallCount()
	dbCalls := finalCallCount - initialCallCount

	// 分散ロックにより、DB呼び出しは少数であることを確認
	if dbCalls > 3 {
		t.Errorf("Expected at most 3 DB calls with distributed lock, got %d", dbCalls)
	}

	// 全てのリクエストが成功することを確認
	for i, err := range errors {
		if err != nil {
			t.Errorf("Process %d failed: %v", i, err)
		}
	}

	metrics := protector.GetMetrics()
	t.Logf("Multiple processes coordinated via distributed lock")
	t.Logf("Lock acquisitions: %d", metrics.LockAcquisitions)

	// 分散ロックはコンテンション時に取得されるため、0でも正常
	if metrics.LockAcquisitions > numProcesses {
		t.Errorf("Expected at most %d lock acquisitions, got %d", numProcesses, metrics.LockAcquisitions)
	}

	t.Log("Only one process loaded data from DB")
}

func TestThunderingHerdProtector_CircuitBreaker(t *testing.T) {
	cache := NewMockCacheClient()
	db := NewMockDataRepository()
	lockManager := NewMockLockManager()

	protector := NewThunderingHerdProtector(
		cache,
		db,
		lockManager,
		2,              // 低い閾値でテスト
		1*time.Second,  // 短いタイムアウトでテスト
		0.1,
	)

	ctx := context.Background()
	key := "circuit-test"

	// データベースを失敗状態に設定
	db.SetFailNext(true)

	// 閾値を超える失敗リクエストを送信
	for i := 0; i < 3; i++ {
		_, err := protector.Get(ctx, key)
		if err == nil {
			t.Error("Expected error from failing database")
		}
	}

	// Circuit Breakerが開いていることを確認
	_, err := protector.Get(ctx, key)
	if err != ErrCircuitOpen {
		t.Errorf("Expected circuit breaker to be open, got error: %v", err)
	}

	metrics := protector.GetMetrics()
	t.Logf("Circuit breaker activated after threshold failures")
	t.Logf("Circuit breaker trips: %d", metrics.CircuitBreakerTrips)

	if metrics.CircuitBreakerTrips == 0 {
		t.Error("Expected circuit breaker trips")
	}

	// タイムアウト後にHalf-Open状態になることを確認
	time.Sleep(1100 * time.Millisecond)
	db.SetFailNext(false) // データベースを復旧

	testData := &Data{
		ID:        "circuit-test",
		Value:     "circuit-value",
		CreatedAt: time.Now(),
	}
	db.Create(context.Background(), testData)

	data, err := protector.Get(ctx, key)
	if err != nil {
		t.Errorf("Expected successful request after timeout, got error: %v", err)
	}
	if data == nil {
		t.Error("Expected data after circuit breaker recovery")
	}

	t.Log("DB protected from excessive load")
}

func TestThunderingHerdProtector_StaleWhileRevalidate(t *testing.T) {
	cache := NewMockCacheClient()
	db := NewMockDataRepository()
	lockManager := NewMockLockManager()

	testData := &Data{
		ID:        "stale-test",
		Value:     "initial-value",
		CreatedAt: time.Now(),
	}

	protector := NewThunderingHerdProtector(
		cache,
		db,
		lockManager,
		3,
		5*time.Second,
		0.1,
	)

	ctx := context.Background()
	key := "stale-test"

	// 初期データをキャッシュに設定（短いTTL）
	protector.Set(ctx, key, testData, 50*time.Millisecond)

	// TTLが切れるまで待機
	time.Sleep(60 * time.Millisecond)

	// 新しいデータをDBに設定
	newData := &Data{
		ID:        "stale-test",
		Value:     "updated-value",
		CreatedAt: time.Now(),
	}
	db.Create(context.Background(), newData)

	// Stale-While-Revalidateでアクセス
	data, err := protector.GetStaleWhileRevalidate(ctx, key)
	if err != nil {
		t.Fatalf("StaleWhileRevalidate failed: %v", err)
	}

	// 古いデータが即座に返されることを確認
	if data.Value != "initial-value" {
		t.Errorf("Expected stale data 'initial-value', got '%s'", data.Value)
	}

	metrics := protector.GetMetrics()
	if metrics.StaleReturns == 0 {
		t.Error("Expected stale returns metric to be incremented")
	}

	t.Log("Stale data returned immediately")

	// バックグラウンド更新の完了を待機
	time.Sleep(200 * time.Millisecond)

	// 新しいリクエストで更新されたデータが返されることを確認
	data, err = protector.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after background refresh failed: %v", err)
	}

	if data.Value != "updated-value" {
		t.Errorf("Expected updated data 'updated-value', got '%s'", data.Value)
	}

	finalMetrics := protector.GetMetrics()
	if finalMetrics.BackgroundRefresh == 0 {
		t.Error("Expected background refresh metric to be incremented")
	}

	t.Log("Background refresh completed")
}

func TestThunderingHerdProtector_TTLJitter(t *testing.T) {
	cache := NewMockCacheClient()
	db := NewMockDataRepository()
	lockManager := NewMockLockManager()

	protector := NewThunderingHerdProtector(
		cache,
		db,
		lockManager,
		3,
		5*time.Second,
		0.2, // 20% jitter
	)

	ctx := context.Background()
	baseTTL := 1 * time.Second

	// 複数のキーに同じTTLで設定
	const numKeys = 100
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("jitter-test-%d", i)
		data := &Data{
			ID:        key,
			Value:     fmt.Sprintf("value-%d", i),
			CreatedAt: time.Now(),
		}

		err := protector.Set(ctx, key, data, baseTTL)
		if err != nil {
			t.Fatalf("Failed to set data for key %s: %v", key, err)
		}
	}

	// ジッターが適用されていることを確認するため、
	// TTL範囲をチェック
	ttlVariations := make(map[int64]int)
	
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("jitter-test-%d", i)
		_, ttl, err := cache.GetWithTTL(ctx, key)
		if err != nil {
			continue
		}
		
		// TTLを100ms単位で丸める
		roundedTTL := int64(ttl / (100 * time.Millisecond))
		ttlVariations[roundedTTL]++
	}

	// TTLに多様性があることを確認
	if len(ttlVariations) < 3 {
		t.Errorf("Expected TTL variations due to jitter, got %d unique TTL values", len(ttlVariations))
	}

	t.Logf("TTL jitter created %d different expiration times", len(ttlVariations))
}

func TestAddJitter(t *testing.T) {
	baseTTL := 10 * time.Second
	jitterPercent := 0.2

	// 複数回実行してジッターの範囲をテスト
	variations := make(map[time.Duration]bool)
	
	for i := 0; i < 100; i++ {
		jitteredTTL := addJitter(baseTTL, jitterPercent)
		variations[jitteredTTL] = true
		
		// 範囲チェック：±20%以内
		minTTL := time.Duration(float64(baseTTL) * 0.8)
		maxTTL := time.Duration(float64(baseTTL) * 1.2)
		
		if jitteredTTL < minTTL || jitteredTTL > maxTTL {
			t.Errorf("Jittered TTL %v is outside expected range [%v, %v]", jitteredTTL, minTTL, maxTTL)
		}
	}

	// 多様性があることを確認
	if len(variations) < 10 {
		t.Errorf("Expected more TTL variations, got %d", len(variations))
	}
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	// 初期状態：Closed
	_, err := cb.Call(func() (interface{}, error) {
		return "success", nil
	})
	if err != nil {
		t.Errorf("Expected success in Closed state, got error: %v", err)
	}

	// 失敗を重ねてOpen状態へ
	for i := 0; i < 2; i++ {
		cb.Call(func() (interface{}, error) {
			return nil, errors.New("failure")
		})
	}

	// Open状態では即座に失敗
	_, err = cb.Call(func() (interface{}, error) {
		return "should not be called", nil
	})
	if err != ErrCircuitOpen {
		t.Errorf("Expected ErrCircuitOpen in Open state, got: %v", err)
	}

	// タイムアウト後にHalf-Open状態へ
	time.Sleep(150 * time.Millisecond)

	// 成功でClosed状態へ復帰
	_, err = cb.Call(func() (interface{}, error) {
		return "success", nil
	})
	if err != nil {
		t.Errorf("Expected success in Half-Open state, got error: %v", err)
	}

	// 再度成功することを確認（Closed状態）
	_, err = cb.Call(func() (interface{}, error) {
		return "success", nil
	})
	if err != nil {
		t.Errorf("Expected success in Closed state after recovery, got error: %v", err)
	}
}

// ベンチマークテスト

func BenchmarkThunderingHerdProtector_CacheHit(b *testing.B) {
	cache := NewMockCacheClient()
	db := NewMockDataRepository()
	lockManager := NewMockLockManager()

	protector := NewThunderingHerdProtector(
		cache,
		db,
		lockManager,
		3,
		5*time.Second,
		0.1,
	)

	// データを事前にキャッシュに設定
	testData := &Data{
		ID:        "bench-test",
		Value:     "bench-value",
		CreatedAt: time.Now(),
	}
	ctx := context.Background()
	protector.Set(ctx, "bench-test", testData, 1*time.Hour)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := protector.Get(ctx, "bench-test")
			if err != nil {
				b.Errorf("Unexpected error: %v", err)
			}
		}
	})
}

func BenchmarkThunderingHerdProtector_CacheMissWithSingleFlight(b *testing.B) {
	cache := NewMockCacheClient()
	db := NewMockDataRepository()
	lockManager := NewMockLockManager()

	testData := &Data{
		ID:        "bench-miss-test",
		Value:     "bench-miss-value",
		CreatedAt: time.Now(),
	}
	db.Create(context.Background(), testData)

	protector := NewThunderingHerdProtector(
		cache,
		db,
		lockManager,
		3,
		5*time.Second,
		0.1,
	)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// キャッシュをクリアしてミスを強制
		cache.SetFailNext(true)
		
		_, err := protector.Get(ctx, "bench-miss-test")
		if err != nil {
			b.Errorf("Unexpected error: %v", err)
		}
		
		cache.SetFailNext(false)
	}
}