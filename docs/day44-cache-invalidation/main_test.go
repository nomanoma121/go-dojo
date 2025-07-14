package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCacheClient は、テスト用のキャッシュクライアント実装
type MockCacheClient struct {
	data    map[string]bool
	deleted map[string]bool
}

func NewMockCacheClient() *MockCacheClient {
	return &MockCacheClient{
		data:    make(map[string]bool),
		deleted: make(map[string]bool),
	}
}

func (m *MockCacheClient) SetKey(key string) {
	m.data[key] = true
}

func (m *MockCacheClient) Delete(ctx context.Context, key string) error {
	m.deleted[key] = true
	delete(m.data, key)
	return nil
}

func (m *MockCacheClient) DeleteMulti(ctx context.Context, keys []string) error {
	for _, key := range keys {
		m.deleted[key] = true
		delete(m.data, key)
	}
	return nil
}

func (m *MockCacheClient) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := m.data[key]
	return exists, nil
}

func (m *MockCacheClient) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

func (m *MockCacheClient) Scan(ctx context.Context, pattern string) ([]string, error) {
	keys := make([]string, 0)
	
	// 簡単なパターンマッチング（* のみサポート）
	for key := range m.data {
		if matchPattern(pattern, key) {
			keys = append(keys, key)
		}
	}
	
	return keys, nil
}

func (m *MockCacheClient) IsDeleted(key string) bool {
	return m.deleted[key]
}

func (m *MockCacheClient) GetDeletedCount() int {
	return len(m.deleted)
}

func (m *MockCacheClient) Reset() {
	m.data = make(map[string]bool)
	m.deleted = make(map[string]bool)
}

// 簡単なパターンマッチング関数
func matchPattern(pattern, str string) bool {
	if pattern == "*" {
		return true
	}
	
	if len(pattern) == 0 {
		return len(str) == 0
	}
	
	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix
	}
	
	return pattern == str
}

func TestCacheInvalidator_InvalidateByKey(t *testing.T) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// テストデータを準備
	cache.SetKey("user:123")
	tagStore.AddTag(ctx, "user:123", "user")
	
	// キーを無効化
	err := invalidator.InvalidateByKey(ctx, "user:123")
	require.NoError(t, err)
	
	// キーが削除されたことを確認
	assert.True(t, cache.IsDeleted("user:123"))
	
	// メトリクスを確認
	metrics := invalidator.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalInvalidations)
	assert.Equal(t, int64(0), metrics.FailedInvalidations)
}

func TestCacheInvalidator_InvalidateByTag(t *testing.T) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// テストデータを準備
	testKeys := []string{"user:123", "user:456", "user:789"}
	for _, key := range testKeys {
		cache.SetKey(key)
		tagStore.AddTag(ctx, key, "user")
	}
	
	// 他のタグのデータも追加
	cache.SetKey("product:100")
	tagStore.AddTag(ctx, "product:100", "product")
	
	// タグベースで無効化
	err := invalidator.InvalidateByTag(ctx, "user")
	require.NoError(t, err)
	t.Log("Tagged cache invalidation successful")
	
	// ユーザー関連のキーがすべて削除されたことを確認
	for _, key := range testKeys {
		assert.True(t, cache.IsDeleted(key))
	}
	
	// 他のタグのデータは削除されていないことを確認
	assert.False(t, cache.IsDeleted("product:100"))
	
	// メトリクスを確認
	metrics := invalidator.GetMetrics()
	assert.Equal(t, int64(3), metrics.TotalInvalidations)
	assert.Equal(t, int64(1), metrics.TagInvalidations)
	t.Logf("All related items invalidated: %d", len(testKeys))
}

func TestCacheInvalidator_InvalidateByPattern(t *testing.T) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// テストデータを準備
	sessionKeys := []string{"session:123", "session:456", "session:789"}
	for _, key := range sessionKeys {
		cache.SetKey(key)
	}
	
	// 他のキーも追加
	cache.SetKey("user:123")
	cache.SetKey("product:100")
	
	// パターンマッチで無効化
	err := invalidator.InvalidateByPattern(ctx, "session:*")
	require.NoError(t, err)
	
	// セッション関連のキーがすべて削除されたことを確認
	for _, key := range sessionKeys {
		assert.True(t, cache.IsDeleted(key))
	}
	
	// 他のキーは削除されていないことを確認
	assert.False(t, cache.IsDeleted("user:123"))
	assert.False(t, cache.IsDeleted("product:100"))
	
	// メトリクスを確認
	metrics := invalidator.GetMetrics()
	assert.Equal(t, int64(3), metrics.TotalInvalidations)
	assert.Equal(t, int64(1), metrics.PatternInvalidations)
}

func TestCacheInvalidator_InvalidationRules(t *testing.T) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// テストデータを準備
	cache.SetKey("user:123")
	cache.SetKey("user:123:profile")
	cache.SetKey("user:123:settings")
	cache.SetKey("users:list")
	
	// 無効化ルールを追加
	rule := InvalidationRule{
		Trigger: "user:123",
		Targets: []string{"user:123:*", "users:list"},
		Delay:   0,
		Condition: func() bool { return true },
	}
	
	err := invalidator.AddInvalidationRule(rule)
	require.NoError(t, err)
	
	// トリガーキーを無効化
	err = invalidator.InvalidateByKey(ctx, "user:123")
	require.NoError(t, err)
	
	// 少し待機してルールが実行されるのを待つ
	time.Sleep(100 * time.Millisecond)
	
	// 関連キーが削除されたことを確認
	assert.True(t, cache.IsDeleted("user:123"))
	assert.True(t, cache.IsDeleted("user:123:profile"))
	assert.True(t, cache.IsDeleted("user:123:settings"))
	assert.True(t, cache.IsDeleted("users:list"))
}

func TestCacheInvalidator_DependencyChain(t *testing.T) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// 依存関係チェーンのテストデータを準備
	keys := []string{
		"user:123",
		"user:123:profile",
		"user:123:settings",
		"user:123:posts",
		"user:123:posts:1",
		"user:123:posts:2",
		"posts:list",
		"users:list",
	}
	
	for _, key := range keys {
		cache.SetKey(key)
	}
	
	// 依存関係を無効化
	err := invalidator.InvalidateRelated(ctx, "user:123")
	require.NoError(t, err)
	t.Log("Dependency chain invalidation completed")
	
	// 関連キーが削除されたことを確認
	deletedCount := 0
	for _, key := range keys {
		if cache.IsDeleted(key) {
			deletedCount++
		}
	}
	
	assert.True(t, deletedCount >= 5) // 少なくとも5つのキーが削除される
	t.Logf("Cascaded invalidation affected %d keys", deletedCount)
}

func TestCacheInvalidator_BatchInvalidate(t *testing.T) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// テストデータを準備
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keys[i] = fmt.Sprintf("batch:key:%d", i)
		cache.SetKey(keys[i])
	}
	
	// バッチで無効化
	start := time.Now()
	err := invalidator.BatchInvalidate(ctx, keys)
	require.NoError(t, err)
	duration := time.Since(start)
	
	// すべてのキーが削除されたことを確認
	assert.Equal(t, 100, cache.GetDeletedCount())
	
	// メトリクスを確認
	metrics := invalidator.GetMetrics()
	assert.Equal(t, int64(100), metrics.TotalInvalidations)
	
	t.Logf("Batch invalidation of 100 keys completed in %v", duration)
}

func TestCacheInvalidator_DelayedInvalidation(t *testing.T) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// テストデータを準備
	cache.SetKey("delayed:key")
	cache.SetKey("target:key")
	
	// 遅延無効化ルールを追加
	rule := InvalidationRule{
		Trigger: "delayed:key",
		Targets: []string{"target:key"},
		Delay:   100 * time.Millisecond,
		Condition: func() bool { return true },
	}
	
	err := invalidator.AddInvalidationRule(rule)
	require.NoError(t, err)
	
	// トリガーキーを無効化
	err = invalidator.InvalidateByKey(ctx, "delayed:key")
	require.NoError(t, err)
	
	// すぐにはターゲットキーは削除されていない
	assert.False(t, cache.IsDeleted("target:key"))
	
	// 遅延時間後に削除される
	time.Sleep(200 * time.Millisecond)
	assert.True(t, cache.IsDeleted("target:key"))
}

func TestCacheInvalidator_Metrics(t *testing.T) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// 複数の無効化操作を実行
	
	// 個別無効化
	cache.SetKey("key1")
	invalidator.InvalidateByKey(ctx, "key1")
	
	// タグベース無効化
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("tag:key:%d", i)
		cache.SetKey(key)
		tagStore.AddTag(ctx, key, "test_tag")
	}
	invalidator.InvalidateByTag(ctx, "test_tag")
	
	// パターンベース無効化
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("pattern:key:%d", i)
		cache.SetKey(key)
	}
	invalidator.InvalidateByPattern(ctx, "pattern:*")
	
	// メトリクスを確認
	metrics := invalidator.GetMetrics()
	efficiency := invalidator.GetInvalidationEfficiency()
	
	assert.Equal(t, int64(9), metrics.TotalInvalidations) // 1 + 5 + 3
	assert.Equal(t, int64(1), metrics.TagInvalidations)
	assert.Equal(t, int64(1), metrics.PatternInvalidations)
	assert.Equal(t, int64(0), metrics.FailedInvalidations)
	assert.Equal(t, 100.0, efficiency)
	
	t.Logf("Invalidation efficiency: %.2f%%", efficiency)
	t.Logf("Average invalidation time: %v", metrics.AvgInvalidationTime)
}

func TestCacheInvalidator_HierarchicalInvalidation(t *testing.T) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// 階層的なデータを準備
	hierarchyKeys := []string{
		"organization:123",
		"organization:123:departments",
		"organization:123:department:1",
		"organization:123:department:1:users",
		"organization:123:department:2",
		"organization:123:department:2:users",
		"organizations:list",
	}
	
	for _, key := range hierarchyKeys {
		cache.SetKey(key)
	}
	
	// 階層的無効化を実行
	err := invalidator.InvalidateHierarchy(ctx, "organization:123")
	require.NoError(t, err)
	
	// 関連する階層データが削除されたことを確認
	deletedCount := 0
	for _, key := range hierarchyKeys {
		if cache.IsDeleted(key) {
			deletedCount++
		}
	}
	
	assert.True(t, deletedCount >= 6) // ほとんどのキーが削除される
	t.Logf("Hierarchical invalidation affected %d keys", deletedCount)
}

// ベンチマークテスト
func BenchmarkCacheInvalidator_InvalidateByKey(b *testing.B) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// テストデータを準備
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench:key:%d", i)
		cache.SetKey(key)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench:key:%d", i)
			invalidator.InvalidateByKey(ctx, key)
			i++
		}
	})
}

func BenchmarkCacheInvalidator_InvalidateByTag(b *testing.B) {
	cache := NewMockCacheClient()
	tagStore := NewMemoryTagStore()
	ruleEngine := NewMemoryRuleEngine()
	invalidator := NewCacheInvalidator(cache, tagStore, ruleEngine)
	
	ctx := context.Background()
	
	// 各タグに10個のキーを関連付け
	for i := 0; i < b.N; i++ {
		tag := fmt.Sprintf("bench:tag:%d", i)
		for j := 0; j < 10; j++ {
			key := fmt.Sprintf("bench:key:%d:%d", i, j)
			cache.SetKey(key)
			tagStore.AddTag(ctx, key, tag)
		}
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			tag := fmt.Sprintf("bench:tag:%d", i)
			invalidator.InvalidateByTag(ctx, tag)
			i++
		}
	})
}