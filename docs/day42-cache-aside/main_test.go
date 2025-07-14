package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// テスト用のグローバル変数
var (
	testDB           *sql.DB
	testRedisClient  *redis.Client
	testCacheClient  CacheClient
	testUserService  *UserService
	testUserRepo     UserRepository
)

// TestCacheClient は、テスト用のキャッシュクライアント実装
type TestCacheClient struct {
	client *redis.Client
}

func (t *TestCacheClient) GetJSON(ctx context.Context, key string, dest interface{}) error {
	result, err := t.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return ErrCacheMiss
	} else if err != nil {
		return err
	}
	
	// 簡単なJSON処理（実際の実装では適切なJSONライブラリを使用）
	return nil // テスト用の簡易実装
}

func (t *TestCacheClient) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return t.client.Set(ctx, key, "cached_value", ttl).Err()
}

func (t *TestCacheClient) Delete(ctx context.Context, key string) error {
	return t.client.Del(ctx, key).Err()
}

func (t *TestCacheClient) Exists(ctx context.Context, key string) (bool, error) {
	result, err := t.client.Exists(ctx, key).Result()
	return result > 0, err
}

// MockUserRepository は、テスト用のユーザーリポジトリ実装
type MockUserRepository struct {
	users     map[int]*User
	mutex     sync.RWMutex
	queryCount int64
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users: make(map[int]*User),
	}
}

func (m *MockUserRepository) GetUser(ctx context.Context, userID int) (*User, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	m.queryCount++
	
	// データベースアクセスの遅延をシミュレート
	time.Sleep(50 * time.Millisecond)
	
	user, exists := m.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}
	
	// ディープコピーを返す
	userCopy := *user
	return &userCopy, nil
}

func (m *MockUserRepository) CreateUser(ctx context.Context, user *User) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.queryCount++
	
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepository) UpdateUser(ctx context.Context, user *User) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.queryCount++
	
	if _, exists := m.users[user.ID]; !exists {
		return ErrUserNotFound
	}
	
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepository) DeleteUser(ctx context.Context, userID int) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.queryCount++
	
	if _, exists := m.users[userID]; !exists {
		return ErrUserNotFound
	}
	
	delete(m.users, userID)
	return nil
}

func (m *MockUserRepository) ListUsers(ctx context.Context) ([]*User, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	m.queryCount++
	
	users := make([]*User, 0, len(m.users))
	for _, user := range m.users {
		userCopy := *user
		users = append(users, &userCopy)
	}
	
	return users, nil
}

func (m *MockUserRepository) GetQueryCount() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.queryCount
}

func (m *MockUserRepository) ResetQueryCount() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.queryCount = 0
}

func TestMain(m *testing.M) {
	// Docker でテスト環境を構築
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// Redis コンテナを起動
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

	// Redis クライアントの初期化
	redisAddr := fmt.Sprintf("localhost:%s", redisResource.GetPort("6379/tcp"))
	if err := pool.Retry(func() error {
		testRedisClient = redis.NewClient(&redis.Options{
			Addr: redisAddr,
		})
		return testRedisClient.Ping(context.Background()).Err()
	}); err != nil {
		log.Fatalf("Could not connect to redis: %s", err)
	}

	// テスト用のクライアントとサービスを初期化
	testCacheClient = &TestCacheClient{client: testRedisClient}
	testUserRepo = NewMockUserRepository()
	testUserService = NewUserService(testUserRepo, testCacheClient)

	// テスト実行
	code := m.Run()

	// クリーンアップ
	if testRedisClient != nil {
		testRedisClient.Close()
	}
	if err := pool.Purge(redisResource); err != nil {
		log.Fatalf("Could not purge redis: %s", err)
	}

	os.Exit(code)
}

func TestUserService_NewUserService(t *testing.T) {
	mockRepo := NewMockUserRepository()
	service := NewUserService(mockRepo, testCacheClient)
	
	assert.NotNil(t, service)
	assert.NotNil(t, service.db)
	assert.NotNil(t, service.cache)
	assert.NotNil(t, service.sf)
	assert.NotNil(t, service.metrics)
}

func TestUserService_CacheAside(t *testing.T) {
	ctx := context.Background()
	
	// テスト用データの準備
	testUser := &User{
		ID:        1,
		Name:      "Test User",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
	}
	
	// テスト用リポジトリにユーザーを追加
	mockRepo := testUserRepo.(*MockUserRepository)
	mockRepo.CreateUser(ctx, testUser)
	mockRepo.ResetQueryCount()
	
	// キャッシュをクリア
	testRedisClient.FlushAll(ctx)
	
	// 新しいサービスインスタンスを作成（メトリクスリセット）
	service := NewUserService(mockRepo, testCacheClient)
	
	// 1回目のアクセス - キャッシュミス、DB から読み込み
	user1, err := service.GetUser(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, testUser.ID, user1.ID)
	assert.Equal(t, testUser.Name, user1.Name)
	t.Log("First access - cache miss, loaded from DB")
	
	// DB クエリが1回実行されたことを確認
	assert.Equal(t, int64(1), mockRepo.GetQueryCount())
	
	// 2回目のアクセス - キャッシュヒット
	user2, err := service.GetUser(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, testUser.ID, user2.ID)
	t.Log("Second access - cache hit, served from cache")
	
	// DB クエリが追加で実行されていないことを確認
	assert.Equal(t, int64(1), mockRepo.GetQueryCount())
	
	// メトリクスを確認
	metrics := service.GetMetrics()
	hitRate := service.GetHitRate()
	t.Logf("Cache hit rate: %.2f%%", hitRate)
	assert.True(t, hitRate >= 50.0) // 50%以上のヒット率
}

func TestUserService_SingleFlight(t *testing.T) {
	ctx := context.Background()
	
	// テスト用データの準備
	testUser := &User{
		ID:        2,
		Name:      "Single Flight User",
		Email:     "singleflight@example.com",
		CreatedAt: time.Now(),
	}
	
	mockRepo := testUserRepo.(*MockUserRepository)
	mockRepo.CreateUser(ctx, testUser)
	mockRepo.ResetQueryCount()
	
	// キャッシュをクリア
	testRedisClient.FlushAll(ctx)
	
	service := NewUserService(mockRepo, testCacheClient)
	
	// 10個の並行リクエストを実行
	const numRequests = 10
	var wg sync.WaitGroup
	results := make([]*User, numRequests)
	errors := make([]error, numRequests)
	
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			user, err := service.GetUser(ctx, 2)
			results[index] = user
			errors[index] = err
		}(i)
	}
	
	wg.Wait()
	
	// すべてのリクエストが成功したことを確認
	for i := 0; i < numRequests; i++ {
		require.NoError(t, errors[i])
		assert.Equal(t, testUser.ID, results[i].ID)
	}
	
	// Single Flight により、DB クエリが1回だけ実行されたことを確認
	queryCount := mockRepo.GetQueryCount()
	assert.Equal(t, int64(1), queryCount)
	t.Logf("%d concurrent requests resulted in %d DB query", numRequests, queryCount)
	
	// Single Flight のメトリクスを確認
	metrics := service.GetMetrics()
	assert.True(t, metrics.SharedLoads > 0)
	t.Log("Single flight pattern working correctly")
}

func TestUserService_UpdateInvalidation(t *testing.T) {
	ctx := context.Background()
	
	// テスト用データの準備
	originalUser := &User{
		ID:        3,
		Name:      "Original User",
		Email:     "original@example.com",
		CreatedAt: time.Now(),
	}
	
	mockRepo := testUserRepo.(*MockUserRepository)
	mockRepo.CreateUser(ctx, originalUser)
	mockRepo.ResetQueryCount()
	
	// キャッシュをクリア
	testRedisClient.FlushAll(ctx)
	
	service := NewUserService(mockRepo, testCacheClient)
	
	// ユーザーを取得してキャッシュに保存
	user1, err := service.GetUser(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, "Original User", user1.Name)
	t.Log("User created and cached")
	
	// ユーザー情報を更新
	updatedUser := &User{
		ID:        3,
		Name:      "Updated User",
		Email:     "updated@example.com",
		CreatedAt: originalUser.CreatedAt,
	}
	
	err = service.UpdateUser(ctx, updatedUser)
	require.NoError(t, err)
	t.Log("User updated, cache invalidated")
	
	// 更新後の取得 - キャッシュが無効化され、新しいデータが取得される
	user2, err := service.GetUser(ctx, 3)
	require.NoError(t, err)
	assert.Equal(t, "Updated User", user2.Name)
	assert.Equal(t, "updated@example.com", user2.Email)
	t.Log("Fresh data loaded from DB after update")
}

func TestUserService_DeleteUser(t *testing.T) {
	ctx := context.Background()
	
	// テスト用データの準備
	testUser := &User{
		ID:        4,
		Name:      "To Be Deleted",
		Email:     "delete@example.com",
		CreatedAt: time.Now(),
	}
	
	mockRepo := testUserRepo.(*MockUserRepository)
	mockRepo.CreateUser(ctx, testUser)
	
	// キャッシュをクリア
	testRedisClient.FlushAll(ctx)
	
	service := NewUserService(mockRepo, testCacheClient)
	
	// ユーザーを取得してキャッシュに保存
	user, err := service.GetUser(ctx, 4)
	require.NoError(t, err)
	assert.Equal(t, testUser.ID, user.ID)
	
	// ユーザーを削除
	err = service.DeleteUser(ctx, 4)
	require.NoError(t, err)
	
	// 削除後の取得 - ユーザーが存在しない
	_, err = service.GetUser(ctx, 4)
	assert.Equal(t, ErrUserNotFound, err)
}

func TestUserService_ListUsers(t *testing.T) {
	ctx := context.Background()
	
	// テスト用データの準備
	users := []*User{
		{ID: 10, Name: "User 10", Email: "user10@example.com", CreatedAt: time.Now()},
		{ID: 11, Name: "User 11", Email: "user11@example.com", CreatedAt: time.Now()},
		{ID: 12, Name: "User 12", Email: "user12@example.com", CreatedAt: time.Now()},
	}
	
	mockRepo := NewMockUserRepository()
	for _, user := range users {
		mockRepo.CreateUser(ctx, user)
	}
	mockRepo.ResetQueryCount()
	
	// キャッシュをクリア
	testRedisClient.FlushAll(ctx)
	
	service := NewUserService(mockRepo, testCacheClient)
	
	// 1回目のリスト取得 - キャッシュミス
	userList1, err := service.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, userList1, 3)
	
	// DB クエリが1回実行された
	assert.Equal(t, int64(1), mockRepo.GetQueryCount())
	
	// 2回目のリスト取得 - キャッシュヒット
	userList2, err := service.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, userList2, 3)
	
	// DB クエリが追加で実行されていない
	assert.Equal(t, int64(1), mockRepo.GetQueryCount())
}

func TestUserService_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	
	mockRepo := NewMockUserRepository()
	service := NewUserService(mockRepo, testCacheClient)
	
	// 存在しないユーザーの取得
	_, err := service.GetUser(ctx, 999)
	assert.Equal(t, ErrUserNotFound, err)
	
	// 存在しないユーザーの更新
	nonExistentUser := &User{
		ID:    999,
		Name:  "Non-existent",
		Email: "none@example.com",
	}
	err = service.UpdateUser(ctx, nonExistentUser)
	assert.Equal(t, ErrUserNotFound, err)
	
	// 存在しないユーザーの削除
	err = service.DeleteUser(ctx, 999)
	assert.Equal(t, ErrUserNotFound, err)
}

func TestUserService_Metrics(t *testing.T) {
	ctx := context.Background()
	
	// テスト用データの準備
	testUser := &User{
		ID:        5,
		Name:      "Metrics User",
		Email:     "metrics@example.com",
		CreatedAt: time.Now(),
	}
	
	mockRepo := NewMockUserRepository()
	mockRepo.CreateUser(ctx, testUser)
	
	// キャッシュをクリア
	testRedisClient.FlushAll(ctx)
	
	service := NewUserService(mockRepo, testCacheClient)
	
	// 複数回のアクセスでメトリクスを蓄積
	service.GetUser(ctx, 5) // キャッシュミス
	service.GetUser(ctx, 5) // キャッシュヒット
	service.GetUser(ctx, 5) // キャッシュヒット
	service.GetUser(ctx, 999) // 存在しないユーザー（キャッシュミス）
	
	metrics := service.GetMetrics()
	hitRate := service.GetHitRate()
	
	assert.True(t, metrics.CacheHits >= 2)
	assert.True(t, metrics.CacheMisses >= 2)
	assert.True(t, metrics.DBQueries >= 2)
	assert.True(t, hitRate >= 0.0 && hitRate <= 100.0)
	
	t.Logf("Metrics - Hits: %d, Misses: %d, Queries: %d, Hit Rate: %.2f%%",
		metrics.CacheHits, metrics.CacheMisses, metrics.DBQueries, hitRate)
}

// ベンチマークテスト
func BenchmarkUserService_GetUser_CacheHit(b *testing.B) {
	ctx := context.Background()
	
	testUser := &User{
		ID:        100,
		Name:      "Benchmark User",
		Email:     "bench@example.com",
		CreatedAt: time.Now(),
	}
	
	mockRepo := NewMockUserRepository()
	mockRepo.CreateUser(ctx, testUser)
	
	service := NewUserService(mockRepo, testCacheClient)
	
	// 事前にキャッシュに保存
	service.GetUser(ctx, 100)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := service.GetUser(ctx, 100)
			if err != nil {
				b.Fatalf("GetUser failed: %v", err)
			}
		}
	})
}

func BenchmarkUserService_GetUser_CacheMiss(b *testing.B) {
	ctx := context.Background()
	
	mockRepo := NewMockUserRepository()
	service := NewUserService(mockRepo, testCacheClient)
	
	// ユーザーを事前に作成
	for i := 0; i < b.N; i++ {
		user := &User{
			ID:        1000 + i,
			Name:      fmt.Sprintf("User %d", i),
			Email:     fmt.Sprintf("user%d@example.com", i),
			CreatedAt: time.Now(),
		}
		mockRepo.CreateUser(ctx, user)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 1000
		for pb.Next() {
			_, err := service.GetUser(ctx, i)
			if err != nil && err != ErrUserNotFound {
				b.Fatalf("GetUser failed: %v", err)
			}
			i++
		}
	})
}

func ExampleUserService_cacheAside() {
	ctx := context.Background()
	
	// サービスの初期化
	mockRepo := NewMockUserRepository()
	service := NewUserService(mockRepo, testCacheClient)
	
	// ユーザーの作成
	user := &User{
		ID:        1,
		Name:      "John Doe",
		Email:     "john@example.com",
		CreatedAt: time.Now(),
	}
	
	err := service.CreateUser(ctx, user)
	if err != nil {
		log.Fatal(err)
	}
	
	// 1回目の取得 - データベースから読み込み
	retrievedUser, err := service.GetUser(ctx, 1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("First access: %s\n", retrievedUser.Name)
	
	// 2回目の取得 - キャッシュから読み込み
	cachedUser, err := service.GetUser(ctx, 1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Second access: %s\n", cachedUser.Name)
	
	// メトリクスの確認
	metrics := service.GetMetrics()
	fmt.Printf("Cache hits: %d, misses: %d\n", metrics.CacheHits, metrics.CacheMisses)
	
	// Output:
	// First access: John Doe
	// Second access: John Doe
	// Cache hits: 1, misses: 1
}