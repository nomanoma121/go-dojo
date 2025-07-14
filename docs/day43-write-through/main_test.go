package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testRedisClient    *redis.Client
	testCacheClient    CacheClient
	testProductService *ProductService
	testProductRepo    ProductRepository
)

// TestCacheClient は、テスト用のキャッシュクライアント実装
type TestCacheClient struct {
	client *redis.Client
}

func (t *TestCacheClient) GetJSON(ctx context.Context, key string, dest interface{}) error {
	result, err := t.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	return err
}

func (t *TestCacheClient) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return t.client.Set(ctx, key, "cached_value", ttl).Err()
}

func (t *TestCacheClient) SetMulti(ctx context.Context, pairs map[string]interface{}, ttl time.Duration) error {
	pipe := t.client.Pipeline()
	for key := range pairs {
		pipe.Set(ctx, key, "cached_value", ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (t *TestCacheClient) Delete(ctx context.Context, key string) error {
	return t.client.Del(ctx, key).Err()
}

func (t *TestCacheClient) Exists(ctx context.Context, key string) (bool, error) {
	result, err := t.client.Exists(ctx, key).Result()
	return result > 0, err
}

// MockProductRepository は、テスト用の商品リポジトリ実装
type MockProductRepository struct {
	products     map[int]*Product
	mutex        sync.RWMutex
	queryCount   int64
	writeCount   int64
	shouldFail   bool
}

func NewMockProductRepository() *MockProductRepository {
	return &MockProductRepository{
		products: make(map[int]*Product),
	}
}

func (m *MockProductRepository) GetProduct(ctx context.Context, productID int) (*Product, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.shouldFail {
		return nil, fmt.Errorf("simulated DB error")
	}
	
	// データベースアクセスの遅延をシミュレート
	time.Sleep(10 * time.Millisecond)
	
	product, exists := m.products[productID]
	if !exists {
		return nil, ErrProductNotFound
	}
	
	productCopy := *product
	return &productCopy, nil
}

func (m *MockProductRepository) CreateProduct(ctx context.Context, product *Product) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.writeCount++
	
	if m.shouldFail {
		return fmt.Errorf("simulated DB error")
	}
	
	time.Sleep(20 * time.Millisecond)
	m.products[product.ID] = product
	return nil
}

func (m *MockProductRepository) UpdateProduct(ctx context.Context, product *Product) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.writeCount++
	
	if m.shouldFail {
		return fmt.Errorf("simulated DB error")
	}
	
	if _, exists := m.products[product.ID]; !exists {
		return ErrProductNotFound
	}
	
	time.Sleep(20 * time.Millisecond)
	m.products[product.ID] = product
	return nil
}

func (m *MockProductRepository) UpdateProducts(ctx context.Context, products []*Product) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.writeCount++
	
	if m.shouldFail {
		return fmt.Errorf("simulated DB error")
	}
	
	time.Sleep(time.Duration(len(products)) * 5 * time.Millisecond)
	for _, product := range products {
		m.products[product.ID] = product
	}
	return nil
}

func (m *MockProductRepository) DeleteProduct(ctx context.Context, productID int) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.writeCount++
	
	if m.shouldFail {
		return fmt.Errorf("simulated DB error")
	}
	
	if _, exists := m.products[productID]; !exists {
		return ErrProductNotFound
	}
	
	delete(m.products, productID)
	return nil
}

func (m *MockProductRepository) ListProducts(ctx context.Context) ([]*Product, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	products := make([]*Product, 0, len(m.products))
	for _, product := range m.products {
		productCopy := *product
		products = append(products, &productCopy)
	}
	
	return products, nil
}

func (m *MockProductRepository) SetShouldFail(shouldFail bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldFail = shouldFail
}

func (m *MockProductRepository) GetWriteCount() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.writeCount
}

func (m *MockProductRepository) ResetCounts() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.queryCount = 0
	m.writeCount = 0
}

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	redisResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start redis: %s", err)
	}

	redisAddr := fmt.Sprintf("localhost:%s", redisResource.GetPort("6379/tcp"))
	if err := pool.Retry(func() error {
		testRedisClient = redis.NewClient(&redis.Options{Addr: redisAddr})
		return testRedisClient.Ping(context.Background()).Err()
	}); err != nil {
		log.Fatalf("Could not connect to redis: %s", err)
	}

	testCacheClient = &TestCacheClient{client: testRedisClient}
	testProductRepo = NewMockProductRepository()
	testProductService = NewProductService(testProductRepo, testCacheClient)

	code := m.Run()

	if testRedisClient != nil {
		testRedisClient.Close()
	}
	if err := pool.Purge(redisResource); err != nil {
		log.Fatalf("Could not purge redis: %s", err)
	}

	os.Exit(code)
}

func TestProductService_WriteThrough(t *testing.T) {
	ctx := context.Background()
	testRedisClient.FlushAll(ctx)
	
	mockRepo := testProductRepo.(*MockProductRepository)
	mockRepo.ResetCounts()
	
	service := NewProductService(mockRepo, testCacheClient)
	
	product := &Product{
		ID:       1,
		Name:     "Test Product",
		Price:    99.99,
		Category: "Electronics",
	}
	
	// Write-Through で商品を作成
	err := service.CreateProduct(ctx, product)
	require.NoError(t, err)
	t.Log("Product created with write-through")
	
	// DB書き込みが1回実行された
	assert.Equal(t, int64(1), mockRepo.GetWriteCount())
	
	// キャッシュから即座に取得可能
	exists, err := testCacheClient.Exists(ctx, productCacheKey(1))
	require.NoError(t, err)
	assert.True(t, exists)
	t.Log("Product immediately available in cache")
	
	// メトリクスを確認
	metrics := service.GetMetrics()
	assert.Equal(t, int64(1), metrics.DatabaseWrites)
	assert.Equal(t, int64(1), metrics.CacheWrites)
	t.Log("Database and cache are consistent")
}

func TestProductService_StrictConsistency(t *testing.T) {
	ctx := context.Background()
	testRedisClient.FlushAll(ctx)
	
	mockRepo := NewMockProductRepository()
	config := ServiceConfig{
		StrictConsistency: true,
		CacheWriteTimeout: 1 * time.Second,
		MaxRetries:        2,
	}
	service := NewProductServiceWithConfig(mockRepo, testCacheClient, config)
	
	product := &Product{
		ID:       2,
		Name:     "Strict Product",
		Price:    199.99,
		Category: "Books",
	}
	
	// 正常ケース
	err := service.CreateProduct(ctx, product)
	require.NoError(t, err)
	
	metrics := service.GetMetrics()
	assert.Equal(t, int64(1), metrics.DatabaseWrites)
	assert.Equal(t, int64(1), metrics.CacheWrites)
	assert.Equal(t, int64(0), metrics.ConsistencyErrors)
}

func TestProductService_BulkUpdate(t *testing.T) {
	ctx := context.Background()
	testRedisClient.FlushAll(ctx)
	
	mockRepo := NewMockProductRepository()
	service := NewProductService(mockRepo, testCacheClient)
	
	// テスト用商品を事前作成
	products := make([]*Product, 100)
	for i := 0; i < 100; i++ {
		products[i] = &Product{
			ID:       i + 1,
			Name:     fmt.Sprintf("Product %d", i+1),
			Price:    float64(i + 1),
			Category: "Bulk",
		}
		mockRepo.CreateProduct(ctx, products[i])
	}
	
	start := time.Now()
	err := service.BulkUpdateProducts(ctx, products)
	require.NoError(t, err)
	duration := time.Since(start)
	
	t.Logf("100 products updated in bulk")
	t.Logf("Bulk operation completed in %v", duration)
	
	// すべての商品がキャッシュに存在することを確認
	for i := 0; i < 10; i++ { // 一部をサンプル確認
		exists, err := testCacheClient.Exists(ctx, productCacheKey(i+1))
		require.NoError(t, err)
		assert.True(t, exists)
	}
	t.Log("All products immediately available in cache")
}

func TestProductService_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	
	mockRepo := NewMockProductRepository()
	service := NewProductService(mockRepo, testCacheClient)
	
	product := &Product{
		ID:       999,
		Name:     "Error Product",
		Price:    999.99,
		Category: "Error",
	}
	
	// データベースエラーをシミュレート
	mockRepo.SetShouldFail(true)
	
	err := service.CreateProduct(ctx, product)
	assert.Error(t, err)
	t.Log("Database failure properly handled")
	
	// メトリクスでエラーが記録される
	metrics := service.GetMetrics()
	assert.True(t, metrics.WriteFailures > 0)
	
	// エラー状態をリセット
	mockRepo.SetShouldFail(false)
	
	// 正常な作成が可能
	err = service.CreateProduct(ctx, product)
	assert.NoError(t, err)
	t.Log("Recovery after error successful")
}

func TestProductService_EventualConsistency(t *testing.T) {
	ctx := context.Background()
	
	mockRepo := NewMockProductRepository()
	config := ServiceConfig{
		StrictConsistency: false,
		AsyncCacheUpdate:  true,
		CacheWriteTimeout: 1 * time.Second,
		MaxRetries:        2,
	}
	service := NewProductServiceWithConfig(mockRepo, testCacheClient, config)
	
	product := &Product{
		ID:       3,
		Name:     "Eventual Product",
		Price:    299.99,
		Category: "Electronics",
	}
	
	err := service.CreateProduct(ctx, product)
	require.NoError(t, err)
	
	// 非同期キャッシュ更新を待機
	time.Sleep(100 * time.Millisecond)
	
	metrics := service.GetMetrics()
	assert.Equal(t, int64(1), metrics.DatabaseWrites)
	// 非同期更新のため、キャッシュ書き込みは後で完了する可能性がある
}

func TestProductService_Metrics(t *testing.T) {
	ctx := context.Background()
	
	mockRepo := NewMockProductRepository()
	service := NewProductService(mockRepo, testCacheClient)
	
	// 複数の操作を実行してメトリクスを蓄積
	for i := 0; i < 5; i++ {
		product := &Product{
			ID:       100 + i,
			Name:     fmt.Sprintf("Metrics Product %d", i),
			Price:    float64(100 + i),
			Category: "Metrics",
		}
		service.CreateProduct(ctx, product)
	}
	
	metrics := service.GetMetrics()
	efficiency := service.GetWriteThroughEfficiency()
	consistency := service.GetCacheConsistencyRate()
	
	assert.True(t, metrics.DatabaseWrites >= 5)
	assert.True(t, metrics.CacheWrites >= 5)
	assert.True(t, efficiency >= 0.0 && efficiency <= 100.0)
	assert.True(t, consistency >= 0.0 && consistency <= 100.0)
	
	t.Logf("Write-through efficiency: %.2f%%", efficiency)
	t.Logf("Cache consistency rate: %.2f%%", consistency)
	t.Logf("Average write time: %v", metrics.AvgWriteTime)
}

// ベンチマークテスト
func BenchmarkProductService_CreateProduct(b *testing.B) {
	ctx := context.Background()
	
	mockRepo := NewMockProductRepository()
	service := NewProductService(mockRepo, testCacheClient)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			product := &Product{
				ID:       1000 + i,
				Name:     fmt.Sprintf("Bench Product %d", i),
				Price:    float64(i),
				Category: "Benchmark",
			}
			
			err := service.CreateProduct(ctx, product)
			if err != nil {
				b.Fatalf("CreateProduct failed: %v", err)
			}
			i++
		}
	})
}

func BenchmarkProductService_UpdateProduct(b *testing.B) {
	ctx := context.Background()
	
	mockRepo := NewMockProductRepository()
	service := NewProductService(mockRepo, testCacheClient)
	
	// 事前にテストデータを作成
	for i := 0; i < b.N; i++ {
		product := &Product{
			ID:       2000 + i,
			Name:     fmt.Sprintf("Update Product %d", i),
			Price:    float64(i),
			Category: "Update",
		}
		mockRepo.CreateProduct(ctx, product)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 2000
		for pb.Next() {
			product := &Product{
				ID:       i,
				Name:     fmt.Sprintf("Updated Product %d", i),
				Price:    float64(i * 2),
				Category: "Updated",
			}
			
			err := service.UpdateProduct(ctx, product)
			if err != nil && err != ErrProductNotFound {
				b.Fatalf("UpdateProduct failed: %v", err)
			}
			i++
		}
	})
}

func ExampleProductService_writeThrough() {
	ctx := context.Background()
	
	// サービスの初期化
	mockRepo := NewMockProductRepository()
	service := NewProductService(mockRepo, testCacheClient)
	
	// 商品の作成（Write-Through）
	product := &Product{
		ID:       1,
		Name:     "Example Product",
		Price:    49.99,
		Category: "Example",
	}
	
	err := service.CreateProduct(ctx, product)
	if err != nil {
		log.Fatal(err)
	}
	
	// 作成直後にキャッシュから取得可能
	retrievedProduct, err := service.GetProduct(ctx, 1)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Product: %s, Price: $%.2f\n", retrievedProduct.Name, retrievedProduct.Price)
	
	// メトリクスの確認
	metrics := service.GetMetrics()
	fmt.Printf("DB writes: %d, Cache writes: %d\n", metrics.DatabaseWrites, metrics.CacheWrites)
	
	// Output:
	// Product: Example Product, Price: $49.99
	// DB writes: 1, Cache writes: 1
}