package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

// ErrCacheMiss は、キャッシュにキーが存在しない場合のエラー
var ErrCacheMiss = errors.New("cache miss")

// CacheStats は、キャッシュの統計情報を保持する構造体
type CacheStats struct {
	Hits   int64 // キャッシュヒット数
	Misses int64 // キャッシュミス数
	Errors int64 // エラー数
}

// CacheClient は、Redis キャッシュクライアントを表す構造体
type CacheClient struct {
	client *redis.Client
	stats  *CacheStats
}

// NewCacheClient は、新しい CacheClient を作成します
func NewCacheClient(addr string) (*CacheClient, error) {
	// Redis クライアントの設定
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     "",                   // パスワードなし
		DB:           0,                    // デフォルトDB
		PoolSize:     10,                   // 接続プールサイズ
		MinIdleConns: 5,                    // 最小アイドル接続数
		PoolTimeout:  30 * time.Second,     // プールタイムアウト
		IdleTimeout:  5 * time.Minute,      // アイドルタイムアウト
		ReadTimeout:  3 * time.Second,      // 読み取りタイムアウト
		WriteTimeout: 3 * time.Second,      // 書き込みタイムアウト
	})

	// 接続テスト
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &CacheClient{
		client: rdb,
		stats:  &CacheStats{},
	}, nil
}

// Set は、キーと値をキャッシュに設定します
func (c *CacheClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// 値を JSON でエンコード
	data, err := json.Marshal(value)
	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return err
	}

	// Redis に設定
	err = c.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return err
	}

	return nil
}

// Get は、キャッシュからキーに対応する値を取得します
func (c *CacheClient) Get(ctx context.Context, key string) (string, error) {
	result, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// キーが存在しない場合
		atomic.AddInt64(&c.stats.Misses, 1)
		return "", ErrCacheMiss
	} else if err != nil {
		// その他のエラー
		atomic.AddInt64(&c.stats.Errors, 1)
		return "", err
	}

	// キャッシュヒット
	atomic.AddInt64(&c.stats.Hits, 1)

	// JSON のデコードが必要な場合は、呼び出し側で処理
	// ここでは文字列として返す
	var value string
	if err := json.Unmarshal([]byte(result), &value); err != nil {
		// JSON でない場合は、そのまま返す
		return result, nil
	}
	return value, nil
}

// Delete は、キャッシュからキーを削除します
func (c *CacheClient) Delete(ctx context.Context, key string) error {
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return err
	}
	return nil
}

// Exists は、キーがキャッシュに存在するかチェックします
func (c *CacheClient) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return false, err
	}
	return result > 0, nil
}

// GetStats は、現在のキャッシュ統計を返します
func (c *CacheClient) GetStats() CacheStats {
	return CacheStats{
		Hits:   atomic.LoadInt64(&c.stats.Hits),
		Misses: atomic.LoadInt64(&c.stats.Misses),
		Errors: atomic.LoadInt64(&c.stats.Errors),
	}
}

// HealthCheck は、Redis への接続をチェックします
func (c *CacheClient) HealthCheck(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close は、Redis クライアントの接続をクローズします
func (c *CacheClient) Close() error {
	return c.client.Close()
}

// GetJSON は、JSON でエンコードされた値を取得してデコードします
func (c *CacheClient) GetJSON(ctx context.Context, key string, dest interface{}) error {
	result, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		atomic.AddInt64(&c.stats.Misses, 1)
		return ErrCacheMiss
	} else if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return err
	}

	atomic.AddInt64(&c.stats.Hits, 1)
	return json.Unmarshal([]byte(result), dest)
}

// SetJSON は、値を JSON でエンコードしてキャッシュに設定します
func (c *CacheClient) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.Set(ctx, key, value, ttl)
}

// GetMulti は、複数のキーを一度に取得します
func (c *CacheClient) GetMulti(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return make(map[string]string), nil
	}

	results, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return nil, err
	}

	resultMap := make(map[string]string)
	for i, result := range results {
		if result != nil {
			atomic.AddInt64(&c.stats.Hits, 1)
			resultMap[keys[i]] = result.(string)
		} else {
			atomic.AddInt64(&c.stats.Misses, 1)
		}
	}

	return resultMap, nil
}

// SetMulti は、複数のキー・値ペアを一度に設定します
func (c *CacheClient) SetMulti(ctx context.Context, pairs map[string]interface{}, ttl time.Duration) error {
	pipe := c.client.Pipeline()
	
	for key, value := range pairs {
		data, err := json.Marshal(value)
		if err != nil {
			atomic.AddInt64(&c.stats.Errors, 1)
			return err
		}
		pipe.Set(ctx, key, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return err
	}

	return nil
}

// GetTTL は、キーの残り TTL を取得します
func (c *CacheClient) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return 0, err
	}
	return ttl, nil
}

// Expire は、キーの TTL を更新します
func (c *CacheClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	err := c.client.Expire(ctx, key, ttl).Err()
	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return err
	}
	return nil
}

// FlushAll は、すべてのキーを削除します（テスト用）
func (c *CacheClient) FlushAll(ctx context.Context) error {
	err := c.client.FlushAll(ctx).Err()
	if err != nil {
		atomic.AddInt64(&c.stats.Errors, 1)
		return err
	}
	return nil
}