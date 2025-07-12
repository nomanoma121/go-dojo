//go:build ignore
package main

import (
	"sync"
)

// Cache インターフェース - 共通のキャッシュ操作を定義
type Cache interface {
	Get(key string) (string, bool)
	Set(key, value string)
	Delete(key string)
	Len() int
}

// MutexCache は sync.Mutex を使ったキャッシュ実装
type MutexCache struct {
	data  map[string]string
	mutex sync.Mutex
}

// NewMutexCache creates a new MutexCache
func NewMutexCache() *MutexCache {
	return &MutexCache{
		data: make(map[string]string),
	}
}

// Get retrieves a value from the cache
func (c *MutexCache) Get(key string) (string, bool) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. mutexをロック
	// 2. mapから値を取得
	// 3. defer文でアンロック
	// 4. 値と存在フラグを返す
	
	return "", false
}

// Set stores a value in the cache
func (c *MutexCache) Set(key, value string) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. mutexをロック
	// 2. mapに値を設定
	// 3. defer文でアンロック
}

// Delete removes a value from the cache
func (c *MutexCache) Delete(key string) {
	// TODO: ここに実装を追加してください
}

// Len returns the number of items in the cache
func (c *MutexCache) Len() int {
	// TODO: ここに実装を追加してください
	return 0
}

// RWMutexCache は sync.RWMutex を使ったキャッシュ実装
type RWMutexCache struct {
	data    map[string]string
	rwmutex sync.RWMutex
}

// NewRWMutexCache creates a new RWMutexCache
func NewRWMutexCache() *RWMutexCache {
	return &RWMutexCache{
		data: make(map[string]string),
	}
}

// Get retrieves a value from the cache (読み取り専用ロック使用)
func (c *RWMutexCache) Get(key string) (string, bool) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. RLock()で読み取り専用ロック
	// 2. mapから値を取得
	// 3. defer文でRUnlock()
	// 4. 値と存在フラグを返す
	
	return "", false
}

// Set stores a value in the cache (書き込み専用ロック使用)
func (c *RWMutexCache) Set(key, value string) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. Lock()で書き込み専用ロック
	// 2. mapに値を設定
	// 3. defer文でUnlock()
}

// Delete removes a value from the cache
func (c *RWMutexCache) Delete(key string) {
	// TODO: ここに実装を追加してください
}

// Len returns the number of items in the cache
func (c *RWMutexCache) Len() int {
	// TODO: ここに実装を追加してください
	return 0
}

// ConcurrentReadWrite performs concurrent read and write operations on a cache
func ConcurrentReadWrite(cache Cache, readers, writers int, operations int) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. WaitGroupでgoroutineの完了を待機
	// 2. readersの数だけ読み取り専用のgoroutineを起動
	// 3. writersの数だけ書き込み専用のgoroutineを起動
	// 4. 各goroutineで指定回数の操作を実行
	// 5. すべてのgoroutineの完了を待機
}

func main() {
	// テスト用のサンプル実行
	mutexCache := NewMutexCache()
	rwmutexCache := NewRWMutexCache()
	
	// 基本的な動作確認
	mutexCache.Set("key1", "value1")
	if val, ok := mutexCache.Get("key1"); ok {
		println("MutexCache:", val)
	}
	
	rwmutexCache.Set("key1", "value1")
	if val, ok := rwmutexCache.Get("key1"); ok {
		println("RWMutexCache:", val)
	}
}
