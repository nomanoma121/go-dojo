package main

import (
	"bytes"
	"sync"
	"testing"
)

const (
	smallData = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 64 bytes
	largeData = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" +
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" +
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" +
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" // 256 bytes
)

// ---- BufferPoolの実装 ----
type BufferPool struct {
	pool sync.Pool
}

func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// ★プールが空のとき、128バイトの容量を持つバッファを生成
				return bytes.NewBuffer(make([]byte, 0, 128))
			},
		},
	}
}

func (bp *BufferPool) Get() *bytes.Buffer {
	return bp.pool.Get().(*bytes.Buffer)
}

func (bp *BufferPool) Put(b *bytes.Buffer) {
	b.Reset()
	bp.pool.Put(b)
}

// ---- ベンチマーク関数 ----

func BenchmarkWithPool_SmallWrite(b *testing.B) {
	pool := NewBufferPool()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer := pool.Get()
		buffer.WriteString(smallData) // 64バイト書き込み
		pool.Put(buffer)
	}
}

func BenchmarkWithPool_LargeWrite(b *testing.B) {
	pool := NewBufferPool()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer := pool.Get()
		buffer.WriteString(largeData) // 256バイト書き込み (容量オーバー)
		pool.Put(buffer)
	}
}
