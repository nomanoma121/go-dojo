# Day 41: Redisによるキャッシュ層の実装

## 🎯 本日の目標 (Today's Goal)

go-redis クライアントを使用して Redis に接続し、基本的なキャッシュ操作を実装できるようになる。Redis の接続プールとヘルスチェック機能を理解し、プロダクション環境で使用可能なキャッシュ層を構築できる。

## 📖 解説 (Explanation)

### Redis とは

Redis (Remote Dictionary Server) は、インメモリの高速データ構造ストアです。データベース、キャッシュ、メッセージブローカーとして使用できます。

### なぜキャッシュが必要なのか？

1. **データベース負荷軽減**: 頻繁にアクセスされるデータをメモリに保存
2. **応答速度向上**: メモリアクセスはディスクアクセスより圧倒的に高速
3. **スケーラビリティ**: 読み取り負荷を分散

### Redis の特徴

- **高速性**: メモリベースで非常に高速
- **豊富なデータ構造**: String, Hash, List, Set, Sorted Set をサポート
- **永続化**: RDB と AOF による永続化オプション
- **レプリケーション**: マスター・スレーブ構成対応
- **クラスタリング**: 分散構成で高可用性を実現

### go-redis クライアント

Go で Redis を使用する際の標準的なクライアントライブラリです。

```go
// 基本的な接続
rdb := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
})

// 基本操作
err := rdb.Set(ctx, "key", "value", time.Hour).Err()
val, err := rdb.Get(ctx, "key").Result()
```

### 接続プールの重要性

Redis クライアントは内部的に接続プールを管理します：

```go
rdb := redis.NewClient(&redis.Options{
    Addr:         "localhost:6379",
    PoolSize:     10,           // 最大接続数
    MinIdleConns: 5,            // 最小アイドル接続数
    PoolTimeout:  30 * time.Second, // 接続待機タイムアウト
    IdleTimeout:  time.Minute,  // アイドル接続のタイムアウト
})
```

### TTL (Time To Live) 管理

キャッシュデータには適切な有効期限を設定することが重要です：

```go
// TTL 付きでデータを設定
rdb.Set(ctx, "session:12345", userData, 30*time.Minute)

// TTL を確認
ttl := rdb.TTL(ctx, "session:12345").Val()

// TTL を更新
rdb.Expire(ctx, "session:12345", time.Hour)
```

### ヘルスチェック

Redis の接続状態を監視することは重要です：

```go
// Ping でヘルスチェック
pong, err := rdb.Ping(ctx).Result()
if err != nil {
    log.Printf("Redis connection failed: %v", err)
}
```

### エラーハンドリング

Redis 特有のエラーを適切に処理する必要があります：

```go
val, err := rdb.Get(ctx, "key").Result()
if err == redis.Nil {
    // キーが存在しない場合
    fmt.Println("Key does not exist")
} else if err != nil {
    // その他のエラー
    log.Printf("Redis error: %v", err)
}
```

## 📝 課題 (The Problem)

以下の機能を持つ Redis キャッシュクライアントを実装してください：

### 1. CacheClient 構造体の実装

```go
type CacheClient struct {
    client *redis.Client
    stats  *CacheStats
}

type CacheStats struct {
    Hits   int64
    Misses int64
    Errors int64
}
```

### 2. 必要なメソッドの実装

- `NewCacheClient(addr string) (*CacheClient, error)`: クライアントの初期化
- `Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error`: データの設定
- `Get(ctx context.Context, key string) (string, error)`: データの取得
- `Delete(ctx context.Context, key string) error`: データの削除
- `Exists(ctx context.Context, key string) (bool, error)`: キーの存在確認
- `GetStats() CacheStats`: キャッシュ統計の取得
- `HealthCheck(ctx context.Context) error`: ヘルスチェック
- `Close() error`: 接続のクリーンアップ

### 3. 統計情報の管理

キャッシュのヒット率、ミス率、エラー率を追跡してください。

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestCacheClient_BasicOperations
    main_test.go:45: Set operation successful
    main_test.go:52: Retrieved value: test_value
    main_test.go:59: Key exists: true
    main_test.go:66: Key deleted successfully
    main_test.go:73: Key no longer exists: false
--- PASS: TestCacheClient_BasicOperations (0.02s)

=== RUN   TestCacheClient_TTL
    main_test.go:95: Value set with TTL
    main_test.go:102: Value retrieved before expiration: ttl_value
    main_test.go:109: Value expired and no longer accessible
--- PASS: TestCacheClient_TTL (1.51s)

=== RUN   TestCacheClient_Stats
    main_test.go:135: Cache stats - Hits: 2, Misses: 1, Errors: 0
--- PASS: TestCacheClient_Stats (0.01s)

=== RUN   TestCacheClient_HealthCheck
    main_test.go:150: Health check passed
--- PASS: TestCacheClient_HealthCheck (0.01s)

PASS
ok      day41-redis-caching     1.672s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### パッケージのインポート

```go
import (
    "context"
    "encoding/json"
    "sync/atomic"
    "time"
    
    "github.com/go-redis/redis/v8"
)
```

### 依存関係

```bash
go mod init day41-redis-caching
go get github.com/go-redis/redis/v8
go get github.com/ory/dockertest/v3
```

### 統計情報の原子的操作

```go
// ヒット数の増加
atomic.AddInt64(&c.stats.Hits, 1)

// ミス数の増加
atomic.AddInt64(&c.stats.Misses, 1)
```

### JSON エンコーディング

複雑なデータ構造をキャッシュする場合：

```go
data, err := json.Marshal(value)
if err != nil {
    return err
}
return c.client.Set(ctx, key, data, ttl).Err()
```

### エラー分類

```go
if err == redis.Nil {
    // キーが存在しない
    atomic.AddInt64(&c.stats.Misses, 1)
    return "", ErrCacheMiss
} else if err != nil {
    // その他のエラー
    atomic.AddInt64(&c.stats.Errors, 1)
    return "", err
}
```

### Docker テスト環境

テストで Redis コンテナを使用する場合：

```go
func setupRedis(t *testing.T) (*redis.Client, func()) {
    pool, err := dockertest.NewPool("")
    require.NoError(t, err)
    
    resource, err := pool.Run("redis", "7", nil)
    require.NoError(t, err)
    
    // 接続確認とクリーンアップ関数を返す
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **バルク操作**: 複数のキーを一度に操作する機能
2. **キー名前空間**: アプリケーション別にキーを分離する機能
3. **圧縮**: 大きなデータを圧縮してキャッシュする機能
4. **メトリクス**: Prometheus メトリクスの出力
5. **フェイルオーバー**: Redis サーバーダウン時の対処

実装を通じて、Redis の基本的な使用方法と、プロダクション環境でのキャッシュ設計の基礎を学びましょう！