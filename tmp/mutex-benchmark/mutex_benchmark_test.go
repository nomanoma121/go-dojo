package main

import (
	"fmt"
	"sync"
	"testing"

	"golang.org/x/exp/rand"
)

// パフォーマンステスト用のデータ構造
type PerformanceTestData struct {
	mutex   sync.Mutex
	rwMutex sync.RWMutex
	data    map[int]string
}

func NewPerformanceTestData() *PerformanceTestData {
	data := make(map[int]string)
	for i := 0; i < 1000; i++ {
		data[i] = fmt.Sprintf("value_%d", i)
	}

	return &PerformanceTestData{
		data: data,
	}
}

// Mutexを使った読み取り（すべて排他実行）
func (ptd *PerformanceTestData) ReadWithMutex(key int) string {
	ptd.mutex.Lock()
	defer ptd.mutex.Unlock()

	return ptd.data[key]
}

// RWMutexを使った読み取り（並行実行可能）
func (ptd *PerformanceTestData) ReadWithRWMutex(key int) string {
	ptd.rwMutex.RLock()
	defer ptd.rwMutex.RUnlock()

	return ptd.data[key]
}

// パフォーマンス比較テスト
func BenchmarkMutexReads(b *testing.B) {
	data := NewPerformanceTestData()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = data.ReadWithMutex(rand.Intn(1000))
		}
	})
}

func BenchmarkRWMutexReads(b *testing.B) {
	data := NewPerformanceTestData()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = data.ReadWithRWMutex(rand.Intn(1000))
		}
	})
}

// 期待される結果:
// BenchmarkMutexReads-8      1000000    1500 ns/op
// BenchmarkRWMutexReads-8   10000000     150 ns/op
// → RWMutexが約10倍高速（読み取り専用ワークロード）
