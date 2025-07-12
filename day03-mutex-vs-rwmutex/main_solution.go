package main

import (
	"fmt"
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

// MutexCache は sync.Mutex を使ったキャッシュ実装
func (c *MutexCache) Get(key string) (string, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	value, exists := c.data[key]
	return value, exists
}

func (c *MutexCache) Set(key, value string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.data[key] = value
}

func (c *MutexCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	delete(c.data, key)
}

func (c *MutexCache) Len() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	return len(c.data)
}

// RWMutexCache は sync.RWMutex を使ったキャッシュ実装
func (c *RWMutexCache) Get(key string) (string, bool) {
	c.rwmutex.RLock()
	defer c.rwmutex.RUnlock()
	
	value, exists := c.data[key]
	return value, exists
}

func (c *RWMutexCache) Set(key, value string) {
	c.rwmutex.Lock()
	defer c.rwmutex.Unlock()
	
	c.data[key] = value
}

func (c *RWMutexCache) Delete(key string) {
	c.rwmutex.Lock()
	defer c.rwmutex.Unlock()
	
	delete(c.data, key)
}

func (c *RWMutexCache) Len() int {
	c.rwmutex.RLock()
	defer c.rwmutex.RUnlock()
	
	return len(c.data)
}

// ConcurrentReadWrite performs concurrent read and write operations on a cache
func ConcurrentReadWrite(cache Cache, readers, writers int, operations int) {
	var wg sync.WaitGroup
	
	// 読み取り専用のGoroutineを起動
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key-%d", j%10)
				cache.Get(key)
			}
		}(i)
	}
	
	// 書き込み専用のGoroutineを起動
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key-%d", j%10)
				value := fmt.Sprintf("writer-%d-value-%d", writerID, j)
				cache.Set(key, value)
			}
		}(i)
	}
	
	wg.Wait()
}