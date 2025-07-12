# Day 14: スレッドセーフなキャッシュ

## 学習目標
sync.MapまたはRWMutexを使い、並行アクセス可能なインメモリキャッシュを実装する。

## 課題説明

高パフォーマンスなWebアプリケーションでは、データベースやAPIへのアクセス回数を減らすためにインメモリキャッシュが重要です。複数のGoroutineから安全にアクセスできるキャッシュシステムを実装してください。

### 要件

1. **並行安全**: 複数のGoroutineからの同時アクセスに対応
2. **TTL(Time To Live)**: データの有効期限管理
3. **容量制限**: メモリ使用量の制限とLRU(Least Recently Used)削除
4. **統計情報**: ヒット率、ミス率などの統計収集
5. **型安全**: ジェネリクスを使用した型安全なインターフェース

### 実装すべき構造体と関数

```go
// Cache represents a thread-safe cache with TTL and LRU eviction
type Cache[K comparable, V any] struct {
    maxSize int
    items   map[K]*cacheItem[V]
    lruList *list.List
    mu      sync.RWMutex
    stats   *CacheStats
}

// CacheItem represents a cached item with metadata
type cacheItem[V any] struct {
    key        K
    value      V
    expiration time.Time
    element    *list.Element
}

// CacheStats tracks cache performance metrics
type CacheStats struct {
    hits       int64
    misses     int64
    evictions  int64
}
```

## ヒント

1. `sync.RWMutex`を使用して読み取り操作を並行化
2. `container/list`を使用してLRU機能を実装
3. `sync/atomic`を使用して統計情報を並行安全に更新
4. ジェネリクスを使用して型安全なキャッシュを実装

## スコアカード

- ✅ 基本実装: Get/Set/Delete操作が並行安全に動作
- ✅ TTL機能: 有効期限切れのデータが自動削除される
- ✅ LRU削除: 容量上限時に最も古いデータが削除される
- ✅ 統計情報: ヒット率などの統計が正確に収集される

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```

## 参考資料

- [sync.RWMutex Documentation](https://pkg.go.dev/sync#RWMutex)
- [container/list Documentation](https://pkg.go.dev/container/list)
- [Go Generics](https://go.dev/doc/tutorial/generics)