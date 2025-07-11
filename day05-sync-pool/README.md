# Day 05: sync.Poolによるオブジェクト再利用

## 学習目標
GCの負荷を軽減するためのオブジェクトプーリングを実装し、sync.Poolの効果的な使用方法を理解する。

## 課題説明

頻繁に作成・破棄されるオブジェクト（バッファ、構造体、スライスなど）をプールして再利用することで、ガベージコレクションの負荷を軽減し、メモリ使用量を最適化するプログラムを実装してください。

### 要件

1. **バッファプール**: bytes.Bufferの効率的な再利用
2. **構造体プール**: 重い構造体インスタンスのプーリング
3. **スライスプール**: 動的サイズのスライスの再利用
4. **パフォーマンス測定**: プール使用時と非使用時の比較

### 実装すべき構造体と関数

```go
// BufferPool manages a pool of bytes.Buffer
type BufferPool struct {
    pool sync.Pool
}

// WorkerData represents data processed by workers
type WorkerData struct {
    ID       int
    Payload  []byte
    Metadata map[string]string
}

// WorkerDataPool manages a pool of WorkerData
type WorkerDataPool struct {
    pool sync.Pool
}

// SlicePool manages pools of slices with different capacities
type SlicePool struct {
    pools map[int]*sync.Pool // key: capacity, value: pool
    mu    sync.RWMutex
}
```

## ヒント

1. `sync.Pool.New`フィールドでプールが空の時に新しいオブジェクトを作成する関数を設定
2. `Get()`でオブジェクトを取得、`Put()`でプールに戻す
3. オブジェクトをプールに戻す前に状態をリセット
4. プールからのオブジェクトは型アサーションが必要
5. GCが実行されるとプールの内容は削除される可能性がある

## スコアカード

- ✅ 基本実装: sync.Poolが正しく動作し、オブジェクトが再利用される
- ✅ メモリ効率: メモリ使用量とGC負荷が軽減される
- ✅ パフォーマンス: 処理速度が向上する
- ✅ 並行安全性: 複数のGoroutineから安全にアクセスできる

## 実行方法

```bash
go test -v
go test -bench=.
go test -bench=. -benchmem  # メモリ使用量も測定
```

## 参考資料

- [Go sync.Pool](https://pkg.go.dev/sync#Pool)
- [Pool Performance Tips](https://golang.org/doc/gc_guide#Pool)