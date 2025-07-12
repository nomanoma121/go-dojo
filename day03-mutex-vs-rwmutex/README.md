# Day 03: sync.Mutex vs RWMutex

## 🎯 本日の目標 (Today's Goal)

このチャレンジを通して、以下のスキルを身につけることができます：

- **sync.Mutex と sync.RWMutex の違いを理解し、適切に使い分けられるようになる**
- **並行読み取りのパフォーマンス利点を測定・比較できるようになる**
- **共有リソースへの安全なアクセス制御を実装できるようになる**
- **レースコンディションを防ぐミューテックスの使い方をマスターする**

## 📖 解説 (Explanation)

### なぜミューテックスが必要なのか？

Goの並行プログラミングでは、複数のGoroutineが同じメモリ領域（変数、スライス、マップなど）に同時にアクセスする状況が頻繁に発生します。これが**レースコンディション**と呼ばれる問題を引き起こす可能性があります。

```go
// 危険な例：レースコンディションが発生する可能性
var counter int

func increment() {
    counter++ // この操作は原子的ではない！
}

func main() {
    for i := 0; i < 1000; i++ {
        go increment() // 1000個のGoroutineが同時にcounterを変更
    }
    // 結果は1000にならない可能性が高い
}
```

上記のコードで`counter++`は、実際には以下の3つのステップに分かれています：

1. メモリからcounterの値を読み取る
2. 値を1増加させる  
3. 新しい値をメモリに書き戻す

複数のGoroutineが同時にこれらのステップを実行すると、古い値を読み取って上書きしてしまう可能性があります。

### sync.Mutex：基本的な排他制御

`sync.Mutex`（ミューテックス）は、**一度に一つのGoroutineだけ**が特定のコードセクション（クリティカルセクション）を実行できるようにする仕組みです。

```go
import "sync"

var (
    counter int
    mu      sync.Mutex
)

func safeIncrement() {
    mu.Lock()   // ロックを取得（他のGoroutineをブロック）
    counter++   // クリティカルセクション
    mu.Unlock() // ロックを解放
}
```

**Mutexの特徴：**
- 読み取りも書き込みも排他的
- シンプルで理解しやすい
- 書き込みが多い場合に適している

### sync.RWMutex：読み書き分離の排他制御

`sync.RWMutex`（読み書きミューテックス）は、**複数の読み取りは同時に許可し、書き込みは排他的に制御**する高度な仕組みです。

```go
import "sync"

var (
    data map[string]int
    rwMu sync.RWMutex
)

func read(key string) int {
    rwMu.RLock()         // 読み取りロック（他の読み取りと並行可能）
    defer rwMu.RUnlock()
    return data[key]
}

func write(key string, value int) {
    rwMu.Lock()          // 書き込みロック（完全に排他的）
    defer rwMu.Unlock()
    data[key] = value
}
```

**RWMutexの特徴：**
- 複数の読み取りGoroutineが同時実行可能
- 書き込み時は完全に排他的
- 読み取りが多い場合に大幅な性能向上
- Mutexより若干のオーバーヘッドあり

### パフォーマンス比較の実例

読み取りが多いワークロードでは、RWMutexが圧倒的に有利になります：

```go
// 読み取り90%、書き込み10%のワークロード
func benchmarkMutex() {
    var mu sync.Mutex
    data := make(map[string]int)
    
    // 90%が読み取り操作
    for i := 0; i < 9; i++ {
        go func() {
            for j := 0; j < 1000; j++ {
                mu.Lock()
                _ = data["key"]
                mu.Unlock()
            }
        }()
    }
    
    // 10%が書き込み操作
    go func() {
        for j := 0; j < 100; j++ {
            mu.Lock()
            data["key"] = j
            mu.Unlock()
        }
    }()
}
```

この場合、Mutexでは読み取りも1つずつしか実行できませんが、RWMutexなら9つの読み取りGoroutineが並列実行できます。

### 実際の使用場面

**Mutexを選ぶべき場面：**
- 書き込み操作が頻繁（読み書きの比率が1:1に近い）
- シンプルな排他制御で十分
- パフォーマンスよりも保守性を重視

**RWMutexを選ぶべき場面：**
- 読み取り操作が圧倒的に多い（設定情報、キャッシュなど）
- 高い並行性能が必要
- 複数の読み取りGoroutineを活用したい

### ベストプラクティス

1. **deferを使った確実なUnlock**
   ```go
   mu.Lock()
   defer mu.Unlock() // 必ずUnlockが実行される
   ```

2. **適切な粒度でのロック**
   ```go
   // 悪い例：粒度が粗すぎる
   func processAll() {
       mu.Lock()
       defer mu.Unlock()
       for i := 0; i < 1000000; i++ {
           // 長時間のロック
       }
   }
   
   // 良い例：必要な部分のみロック
   func processItem(item Item) {
       // 重い処理はロック外で
       result := heavyComputation(item)
       
       mu.Lock()
       updateSharedData(result) // 短時間のロック
       mu.Unlock()
   }
   ```

3. **デッドロックの回避**
   ```go
   // 悪い例：デッドロックの可能性
   func transferMoney(from, to *Account, amount int) {
       from.mu.Lock()
       to.mu.Lock()   // デッドロックリスク
       // 処理...
       to.mu.Unlock()
       from.mu.Unlock()
   }
   
   // 良い例：一貫した順序でロック
   func transferMoney(from, to *Account, amount int) {
       if from.id < to.id {
           from.mu.Lock()
           to.mu.Lock()
       } else {
           to.mu.Lock()
           from.mu.Lock()
       }
       // 処理...
       to.mu.Unlock()
       from.mu.Unlock()
   }
   ```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **`NewMutexCache()`**: sync.Mutexを使ったキャッシュを初期化する
2. **`NewRWMutexCache()`**: sync.RWMutexを使ったキャッシュを初期化する  
3. **`(c *MutexCache) Get(key string) (string, bool)`**: 値を安全に取得する
4. **`(c *MutexCache) Set(key, value string)`**: キーと値を安全に設定する
5. **`(c *MutexCache) Delete(key string)`**: キーを安全に削除する
6. **`(c *MutexCache) Len() int`**: キャッシュのサイズを安全に取得する
7. **同様のメソッドをRWMutexCacheにも実装**

**重要な実装要件：**
- MutexCacheは`sync.Mutex`を使用
- RWMutexCacheは`sync.RWMutex`を使用し、読み取り操作で`RLock()`を使用
- レースコンディションが発生しないこと
- 1000個のGoroutineが並行してアクセスしても正確に動作すること
- パフォーマンステストで読み取り中心のワークロードでRWMutexが高速であること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestMutexCache
=== RUN   TestMutexCache/Sequential_operations
=== RUN   TestMutexCache/Concurrent_operations
=== RUN   TestMutexCache/Race_condition_test
--- PASS: TestMutexCache (0.15s)
=== RUN   TestRWMutexCache  
=== RUN   TestRWMutexCache/Sequential_operations
=== RUN   TestRWMutexCache/Concurrent_reads
=== RUN   TestRWMutexCache/Mixed_read_write
--- PASS: TestRWMutexCache (0.20s)
PASS
```

### レース検出テスト
```bash
$ go test -race
PASS
```
レースコンディションが検出されないことを確認できます。

### ベンチマーク実行例
```bash
$ go test -bench=.
BenchmarkMutexCacheRead-8        	2000000	   800 ns/op
BenchmarkRWMutexCacheRead-8      	10000000	   150 ns/op  
BenchmarkMutexCacheWrite-8       	5000000	   300 ns/op
BenchmarkRWMutexCacheWrite-8     	4500000	   350 ns/op
BenchmarkMutexCacheMixed-8       	1500000	   1200 ns/op
BenchmarkRWMutexCacheMixed-8     	6000000	   400 ns/op
```
読み取り中心のワークロードでRWMutexの方が5倍程度高速になることが確認できます。

### プログラム実行例
```bash
$ go run main.go
=== Mutex vs RWMutex Performance Comparison ===

Testing with 100 goroutines, 1000 operations each...

Mutex Cache Results:
- Cache size: 100 entries
- Read operations took: 45.2ms
- Write operations took: 12.8ms
- Total time: 58.0ms

RWMutex Cache Results:  
- Cache size: 100 entries
- Read operations took: 8.9ms
- Write operations took: 15.1ms
- Total time: 24.0ms

RWMutex is 2.4x faster for mixed read-heavy workloads!
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的な実装パターン
```go
type MutexCache struct {
    data  map[string]string
    mutex sync.Mutex
}

func (c *MutexCache) Get(key string) (string, bool) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    value, exists := c.data[key]
    return value, exists
}
```

### RWMutexの読み取り操作
```go
func (c *RWMutexCache) Get(key string) (string, bool) {
    c.rwmutex.RLock()  // 読み取りロック
    defer c.rwmutex.RUnlock()
    
    value, exists := c.data[key]
    return value, exists
}
```

### RWMutexの書き込み操作
```go
func (c *RWMutexCache) Set(key, value string) {
    c.rwmutex.Lock()   // 書き込みロック（排他的）
    defer c.rwmutex.Unlock()
    
    c.data[key] = value
}
```

### 使用する主要なパッケージ
- `sync.Mutex` - 基本的な排他制御
- `sync.RWMutex` - 読み書き分離の排他制御  
- `sync.WaitGroup` - Goroutineの完了待機（テストで使用）

### デバッグのコツ
1. `go test -race`でレースコンディションを検出
2. `go test -v`で詳細なテスト結果を確認
3. `go test -bench=.`でパフォーマンスを測定
4. 必要に応じて`time.Sleep()`でタイミングを調整してテスト

### よくある間違い
- Unlockし忘れ → `defer`を使って確実に解放
- 読み取り操作で書き込みロックを使用 → RWMutexでは`RLock()`を使用
- ロック範囲が広すぎる → 必要最小限の範囲でロック
- nilマップへのアクセス → 初期化を忘れずに

## 実行方法

```bash
# テスト実行
go test -v

# レースコンディション検出
go test -race

# ベンチマーク測定
go test -bench=.

# メモリ使用量も測定
go test -bench=. -benchmem

# プログラム実行
go run main.go
```

## 参考資料

- [Go Memory Model](https://golang.org/ref/mem)
- [sync package documentation](https://pkg.go.dev/sync)
- [Effective Go - Concurrency](https://golang.org/doc/effective_go#concurrency)
- [Go sync.Mutex](https://pkg.go.dev/sync#Mutex)
- [Go sync.RWMutex](https://pkg.go.dev/sync#RWMutex)