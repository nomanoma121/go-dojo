package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testCacheClient *CacheClient
var redisAddr string

func TestMain(m *testing.M) {
	// Docker でテスト用 Redis を起動
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7",
		Env: []string{
			"REDIS_PASSWORD=",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	// Redis が起動するまで待機
	redisAddr = fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp"))
	if err := pool.Retry(func() error {
		var err error
		testCacheClient, err = NewCacheClient(redisAddr)
		if err != nil {
			return err
		}
		return testCacheClient.HealthCheck(context.Background())
	}); err != nil {
		log.Fatalf("Could not connect to redis: %s", err)
	}

	code := m.Run()

	// クリーンアップ
	if testCacheClient != nil {
		testCacheClient.Close()
	}
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}

func TestCacheClient_NewCacheClient(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "Valid Redis address",
			addr:    redisAddr,
			wantErr: false,
		},
		{
			name:    "Invalid Redis address",
			addr:    "localhost:9999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewCacheClient(tt.addr)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				if client != nil {
					client.Close()
				}
			}
		})
	}
}

func TestCacheClient_BasicOperations(t *testing.T) {
	ctx := context.Background()
	
	// テスト前にキャッシュをクリア
	err := testCacheClient.FlushAll(ctx)
	require.NoError(t, err)

	// Set 操作のテスト
	err = testCacheClient.Set(ctx, "test_key", "test_value", time.Hour)
	require.NoError(t, err)
	t.Log("Set operation successful")

	// Get 操作のテスト
	value, err := testCacheClient.Get(ctx, "test_key")
	require.NoError(t, err)
	assert.Equal(t, "test_value", value)
	t.Logf("Retrieved value: %s", value)

	// Exists 操作のテスト
	exists, err := testCacheClient.Exists(ctx, "test_key")
	require.NoError(t, err)
	assert.True(t, exists)
	t.Logf("Key exists: %t", exists)

	// Delete 操作のテスト
	err = testCacheClient.Delete(ctx, "test_key")
	require.NoError(t, err)
	t.Log("Key deleted successfully")

	// 削除後の存在確認
	exists, err = testCacheClient.Exists(ctx, "test_key")
	require.NoError(t, err)
	assert.False(t, exists)
	t.Logf("Key no longer exists: %t", exists)
}

func TestCacheClient_TTL(t *testing.T) {
	ctx := context.Background()
	
	// テスト前にキャッシュをクリア
	err := testCacheClient.FlushAll(ctx)
	require.NoError(t, err)

	// 短い TTL でデータを設定
	err = testCacheClient.Set(ctx, "ttl_key", "ttl_value", 1*time.Second)
	require.NoError(t, err)
	t.Log("Value set with TTL")

	// すぐに取得（存在するはず）
	value, err := testCacheClient.Get(ctx, "ttl_key")
	require.NoError(t, err)
	assert.Equal(t, "ttl_value", value)
	t.Logf("Value retrieved before expiration: %s", value)

	// TTL の確認
	ttl, err := testCacheClient.GetTTL(ctx, "ttl_key")
	require.NoError(t, err)
	assert.True(t, ttl > 0 && ttl <= time.Second)

	// 有効期限まで待機
	time.Sleep(1500 * time.Millisecond)

	// 有効期限後の取得（存在しないはず）
	_, err = testCacheClient.Get(ctx, "ttl_key")
	assert.Equal(t, ErrCacheMiss, err)
	t.Log("Value expired and no longer accessible")
}

func TestCacheClient_Stats(t *testing.T) {
	ctx := context.Background()
	
	// テスト前にキャッシュをクリア
	err := testCacheClient.FlushAll(ctx)
	require.NoError(t, err)

	// 初期統計をリセット（新しいクライアントを作成）
	client, err := NewCacheClient(redisAddr)
	require.NoError(t, err)
	defer client.Close()

	// データを設定
	err = client.Set(ctx, "stats_key1", "value1", time.Hour)
	require.NoError(t, err)
	err = client.Set(ctx, "stats_key2", "value2", time.Hour)
	require.NoError(t, err)

	// ヒット（存在するキー）
	_, err = client.Get(ctx, "stats_key1")
	require.NoError(t, err)
	_, err = client.Get(ctx, "stats_key2")
	require.NoError(t, err)

	// ミス（存在しないキー）
	_, err = client.Get(ctx, "nonexistent_key")
	assert.Equal(t, ErrCacheMiss, err)

	// 統計を確認
	stats := client.GetStats()
	assert.Equal(t, int64(2), stats.Hits)
	assert.Equal(t, int64(1), stats.Misses)
	assert.Equal(t, int64(0), stats.Errors)
	t.Logf("Cache stats - Hits: %d, Misses: %d, Errors: %d", 
		stats.Hits, stats.Misses, stats.Errors)
}

func TestCacheClient_HealthCheck(t *testing.T) {
	ctx := context.Background()
	
	err := testCacheClient.HealthCheck(ctx)
	require.NoError(t, err)
	t.Log("Health check passed")
}

func TestCacheClient_JSONOperations(t *testing.T) {
	ctx := context.Background()
	
	// テスト前にキャッシュをクリア
	err := testCacheClient.FlushAll(ctx)
	require.NoError(t, err)

	// 複雑なデータ構造のテスト
	type TestStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Tags []string `json:"tags"`
	}

	original := TestStruct{
		ID:   123,
		Name: "Test Object",
		Tags: []string{"tag1", "tag2", "tag3"},
	}

	// JSON で設定
	err = testCacheClient.SetJSON(ctx, "json_key", original, time.Hour)
	require.NoError(t, err)

	// JSON で取得
	var retrieved TestStruct
	err = testCacheClient.GetJSON(ctx, "json_key", &retrieved)
	require.NoError(t, err)

	assert.Equal(t, original, retrieved)
	t.Logf("JSON object stored and retrieved successfully: %+v", retrieved)
}

func TestCacheClient_MultiOperations(t *testing.T) {
	ctx := context.Background()
	
	// テスト前にキャッシュをクリア
	err := testCacheClient.FlushAll(ctx)
	require.NoError(t, err)

	// 複数のキー・値ペアを設定
	pairs := map[string]interface{}{
		"multi_key1": "value1",
		"multi_key2": "value2",
		"multi_key3": "value3",
	}

	err = testCacheClient.SetMulti(ctx, pairs, time.Hour)
	require.NoError(t, err)

	// 複数のキーを取得
	keys := []string{"multi_key1", "multi_key2", "multi_key3", "nonexistent"}
	results, err := testCacheClient.GetMulti(ctx, keys)
	require.NoError(t, err)

	// 結果を検証
	assert.Equal(t, "\"value1\"", results["multi_key1"]) // JSON エンコードされた文字列
	assert.Equal(t, "\"value2\"", results["multi_key2"])
	assert.Equal(t, "\"value3\"", results["multi_key3"])
	_, exists := results["nonexistent"]
	assert.False(t, exists)

	t.Logf("Multi-operations completed successfully. Retrieved %d values", len(results))
}

func TestCacheClient_ExpireOperations(t *testing.T) {
	ctx := context.Background()
	
	// テスト前にキャッシュをクリア
	err := testCacheClient.FlushAll(ctx)
	require.NoError(t, err)

	// データを設定（長い TTL）
	err = testCacheClient.Set(ctx, "expire_key", "expire_value", time.Hour)
	require.NoError(t, err)

	// 初期 TTL を確認
	ttl, err := testCacheClient.GetTTL(ctx, "expire_key")
	require.NoError(t, err)
	assert.True(t, ttl > 59*time.Minute) // 約1時間

	// TTL を短く更新
	err = testCacheClient.Expire(ctx, "expire_key", 2*time.Second)
	require.NoError(t, err)

	// 更新された TTL を確認
	ttl, err = testCacheClient.GetTTL(ctx, "expire_key")
	require.NoError(t, err)
	assert.True(t, ttl <= 2*time.Second && ttl > 0)

	t.Logf("TTL updated successfully. New TTL: %v", ttl)
}

func TestCacheClient_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	
	// テスト前にキャッシュをクリア
	err := testCacheClient.FlushAll(ctx)
	require.NoError(t, err)

	// 並行アクセステスト
	const numGoroutines = 10
	const numOperations = 100

	// データを事前に設定
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("concurrent_key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		err := testCacheClient.Set(ctx, key, value, time.Hour)
		require.NoError(t, err)
	}

	// 並行で読み取り操作を実行
	done := make(chan bool, numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer func() { done <- true }()
			
			for i := 0; i < numOperations; i++ {
				key := fmt.Sprintf("concurrent_key_%d", i)
				expectedValue := fmt.Sprintf("value_%d", i)
				
				value, err := testCacheClient.Get(ctx, key)
				if err != nil {
					t.Errorf("Goroutine %d: Failed to get key %s: %v", 
						goroutineID, key, err)
					return
				}
				
				if value != expectedValue {
					t.Errorf("Goroutine %d: Expected %s, got %s", 
						goroutineID, expectedValue, value)
					return
				}
			}
		}(g)
	}

	// すべてのゴルーチンの完了を待機
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	stats := testCacheClient.GetStats()
	expectedHits := int64(numGoroutines * numOperations)
	assert.True(t, stats.Hits >= expectedHits) // 他のテストからのヒットも含まれる可能性
	
	t.Logf("Concurrent access test completed. Stats: Hits=%d, Misses=%d, Errors=%d", 
		stats.Hits, stats.Misses, stats.Errors)
}

func TestCacheClient_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	
	// 存在しないキーの取得
	_, err := testCacheClient.Get(ctx, "nonexistent_key")
	assert.Equal(t, ErrCacheMiss, err)

	// 存在しないキーの削除（エラーにならない）
	err = testCacheClient.Delete(ctx, "nonexistent_key")
	assert.NoError(t, err)

	// 存在しないキーの存在確認
	exists, err := testCacheClient.Exists(ctx, "nonexistent_key")
	assert.NoError(t, err)
	assert.False(t, exists)

	t.Log("Error handling tests completed successfully")
}

// ベンチマークテスト
func BenchmarkCacheClient_Set(b *testing.B) {
	ctx := context.Background()
	
	// テスト前にキャッシュをクリア
	testCacheClient.FlushAll(ctx)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench_set_key_%d", i)
			value := fmt.Sprintf("bench_value_%d", i)
			err := testCacheClient.Set(ctx, key, value, time.Hour)
			if err != nil {
				b.Fatalf("Set failed: %v", err)
			}
			i++
		}
	})
}

func BenchmarkCacheClient_Get(b *testing.B) {
	ctx := context.Background()
	
	// テスト前にキャッシュをクリア
	testCacheClient.FlushAll(ctx)

	// ベンチマーク用データを事前に設定
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("bench_get_key_%d", i)
		value := fmt.Sprintf("bench_value_%d", i)
		err := testCacheClient.Set(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Setup failed: %v", err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench_get_key_%d", i%1000)
			_, err := testCacheClient.Get(ctx, key)
			if err != nil {
				b.Fatalf("Get failed: %v", err)
			}
			i++
		}
	})
}

func BenchmarkCacheClient_SetJSON(b *testing.B) {
	ctx := context.Background()
	
	type BenchStruct struct {
		ID    int      `json:"id"`
		Name  string   `json:"name"`
		Tags  []string `json:"tags"`
		Score float64  `json:"score"`
	}

	testData := BenchStruct{
		ID:    12345,
		Name:  "Benchmark Test Object",
		Tags:  []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
		Score: 99.99,
	}

	// テスト前にキャッシュをクリア
	testCacheClient.FlushAll(ctx)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench_json_key_%d", i)
			err := testCacheClient.SetJSON(ctx, key, testData, time.Hour)
			if err != nil {
				b.Fatalf("SetJSON failed: %v", err)
			}
			i++
		}
	})
}

func ExampleCacheClient_basicUsage() {
	// Redis キャッシュクライアントの作成
	client, err := NewCacheClient("localhost:6379")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// データの設定
	err = client.Set(ctx, "user:123", "John Doe", 30*time.Minute)
	if err != nil {
		log.Fatal(err)
	}

	// データの取得
	value, err := client.Get(ctx, "user:123")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Retrieved value: %s\n", value)

	// キャッシュ統計の確認
	stats := client.GetStats()
	fmt.Printf("Cache hits: %d, misses: %d\n", stats.Hits, stats.Misses)

	// Output:
	// Retrieved value: John Doe
	// Cache hits: 1, misses: 0
}