# Day 12: Future / Promiseパターン

## 🎯 本日の目標 (Today's Goal)

JavaScriptのPromiseやJavaのFutureのような非同期処理パターンをGoで実装し、処理の開始と結果の取得を分離した柔軟な並行プログラミングを習得する。

## 📖 解説 (Explanation)

### Future / Promiseパターンとは

Future / Promiseパターンは非同期処理の抽象化手法の一つです。このパターンでは：

- **Promise**: 非同期処理の結果を「約束」するオブジェクト（書き込み側）
- **Future**: 非同期処理の結果を「将来的に」受け取るオブジェクト（読み取り側）

このパターンの利点：
1. **責任の分離**: 結果を設定する側と取得する側を分離
2. **型安全性**: ジェネリクスにより型安全な非同期処理
3. **合成可能性**: 複数のFutureを組み合わせや連鎖が可能
4. **一度性保証**: 結果は一度だけ設定され、複数回の設定を防ぐ

### 他言語での類似機能

```javascript
// JavaScript Promise
const promise = new Promise((resolve, reject) => {
    setTimeout(() => resolve("Hello"), 1000);
});

promise.then(result => console.log(result));
```

```java
// Java CompletableFuture
CompletableFuture<String> future = CompletableFuture.supplyAsync(() -> {
    return "Hello";
});

String result = future.get();
```

### Goでの実装アプローチ

Goでは以下の要素を組み合わせて実装します：

1. **ジェネリクス**: 型安全性を保証
2. **チャネル**: 結果の非同期通信
3. **sync.Once**: 一度だけの実行保証
4. **context**: タイムアウトとキャンセレーション

### 基本的なデータ構造

```go
// 結果を表現する構造体（成功またはエラー）
type Result[T any] struct {
    Value T     // 成功時の値
    Error error // エラー時のエラー
}

// 非同期処理の結果を表現
type Future[T any] struct {
    result chan Result[T] // 結果を受け渡すチャネル
    done   chan struct{}  // 完了を通知するチャネル
}

// 結果を設定する側のインターフェース
type Promise[T any] struct {
    future *Future[T] // 関連するFuture
    once   sync.Once  // 一度だけの実行を保証
}
```

### 実装時の重要なポイント

#### 1. 一度性の保証

```go
func (p *Promise[T]) Resolve(value T) {
    p.once.Do(func() {
        p.future.result <- Result[T]{Value: value}
        close(p.future.done)
    })
}
```

`sync.Once`により、ResolveやRejectが複数回呼ばれても、最初の呼び出しのみが実行されます。

#### 2. ノンブロッキングな状態確認

```go
func (f *Future[T]) IsDone() bool {
    select {
    case <-f.done:
        return true
    default:
        return false
    }
}
```

#### 3. タイムアウト付き取得

```go
func (f *Future[T]) GetWithTimeout(timeout time.Duration) (T, error) {
    select {
    case result := <-f.result:
        return result.Value, result.Error
    case <-time.After(timeout):
        var zero T
        return zero, errors.New("timeout")
    }
}
```

#### 4. コンテキストによるキャンセレーション

```go
func (f *Future[T]) GetWithContext(ctx context.Context) (T, error) {
    select {
    case result := <-f.result:
        return result.Value, result.Error
    case <-ctx.Done():
        var zero T
        return zero, ctx.Err()
    }
}
```

### 応用パターン

#### チェイニング（連鎖）
```go
future := RunAsync(func() (int, error) { return 10, nil })
    .Then(func(x int) (any, error) { return x * 2, nil })
    .Then(func(x any) (any, error) { return fmt.Sprintf("Result: %v", x), nil })
```

#### 並行実行の合成
```go
// すべてのFutureが完了するまで待機
allFuture := AllOf(future1, future2, future3)

// いずれかのFutureが完了するまで待機
anyFuture := AnyOf(future1, future2, future3)
```

### 実際の使用例

```go
// APIの非同期呼び出し
func fetchUserAsync(userID int) *Future[User] {
    return RunAsync(func() (User, error) {
        resp, err := http.Get(fmt.Sprintf("/api/users/%d", userID))
        if err != nil {
            return User{}, err
        }
        defer resp.Body.Close()
        
        var user User
        err = json.NewDecoder(resp.Body).Decode(&user)
        return user, err
    })
}

// 複数のAPIを並行して呼び出し
userFuture := fetchUserAsync(123)
profileFuture := fetchProfileAsync(123)
settingsFuture := fetchSettingsAsync(123)

// すべての結果を待って合成
results := AllOf(userFuture, profileFuture, settingsFuture)
allData, err := results.GetWithTimeout(5 * time.Second)
```

## 📝 課題 (The Problem)

`main_test.go`に書かれているテストをすべてパスするよう、`main.go`のTODO部分を実装してください。

以下の機能を実装する必要があります：

1. **基本的なPromise/Future操作**
   - `NewPromise()`: 新しいPromiseとFutureのペアを作成
   - `Resolve()` / `Reject()`: 結果の設定
   - `Get()`: 結果の取得

2. **タイムアウトとキャンセレーション**
   - `GetWithTimeout()`: タイムアウト付き取得
   - `GetWithContext()`: コンテキストによるキャンセレーション対応

3. **状態確認**
   - `IsDone()`: 完了状態の確認

4. **関数型操作**
   - `Then()`: 結果に関数を適用して新しいFutureを作成
   - `Map()`: 変換関数を適用

5. **ユーティリティ関数**
   - `Completed()`: 即座に完了するFuture
   - `Failed()`: 即座に失敗するFuture
   - `RunAsync()`: 関数を非同期実行
   - `Delay()`: 遅延実行
   - `AllOf()`: 複数Futureの並行実行（すべて完了待ち）
   - `AnyOf()`: 複数Futureの並行実行（いずれか完了待ち）

## ✅ 期待される挙動 (Expected Behavior)

### 基本的な使用例
```
=== Future/Promise Pattern Demo ===
Result: Hello, Future!

=== Async Execution ===
Async result: 42

=== Future Chaining ===
Chained result: Result: 20

=== Multiple Futures ===
All results: [API response for ID 1 API response for ID 2 API response for ID 3]
```

### テスト実行
```bash
$ go test -v
=== RUN   TestBasicPromiseFuture
=== RUN   TestBasicPromiseFuture/Promise_resolve
=== RUN   TestBasicPromiseFuture/Promise_reject
=== RUN   TestBasicPromiseFuture/Promise_resolve_only_once
--- PASS: TestBasicPromiseFuture (0.15s)
    --- PASS: TestBasicPromiseFuture/Promise_resolve (0.05s)
    --- PASS: TestBasicPromiseFuture/Promise_reject (0.05s)
    --- PASS: TestBasicPromiseFuture/Promise_resolve_only_once (0.00s)
...
PASS
ok      go-dojo/day12-future-promise    0.891s
```

### レース条件テスト
```bash
$ go test -race
PASS
ok      go-dojo/day12-future-promise    1.234s
```

### ベンチマーク
```bash
$ go test -bench=.
BenchmarkPromiseResolve-8      1000000   1203 ns/op
BenchmarkFutureChaining-8       500000   2456 ns/op
BenchmarkAllOf-8                200000   7890 ns/op
PASS
```

## 💡 ヒント (Hints)

### 詰まった時の解決の糸口

1. **チャネルの初期化**: 
   - `result`チャネルはバッファサイズ1で作成（`make(chan Result[T], 1)`）
   - `done`チャネルはバッファなしで作成（`make(chan struct{})`）

2. **sync.Onceの使用**:
   ```go
   p.once.Do(func() {
       // 一度だけ実行される処理
   })
   ```

3. **ゼロ値の返却**:
   ```go
   var zero T
   return zero, err
   ```

4. **非ブロッキングselect**:
   ```go
   select {
   case <-ch:
       return true
   default:
       return false
   }
   ```

5. **Goroutineでの非同期実行**:
   ```go
   go func() {
       // 非同期処理
   }()
   ```

### 参考になるGoの標準パッケージ

- `context`: キャンセレーションとタイムアウト
- `sync`: Onceによる一度だけの実行保証
- `time`: タイムアウト処理
- `runtime`: Goroutineの制御

### 型システムのポイント

```go
// ジェネリクス制約の使用
func NewPromise[T any]() *Promise[T]

// any型（interface{}の新しい書き方）の使用
func (f *Future[T]) Then(fn func(T) (any, error)) *Future[any]
```

## スコアカード

- ✅ **基本実装**: Promise/Futureパターンが動作する
- ✅ **一度性保証**: Resolveは一度だけ実行される
- ✅ **エラーハンドリング**: エラーが適切に伝播される
- ✅ **タイムアウト**: 結果取得でタイムアウトが機能する
- ✅ **コンテキスト対応**: キャンセレーションが正しく動作する
- ✅ **状態確認**: IsDoneが正しく動作する
- ✅ **チェイニング**: ThenとMapで処理を連鎖できる
- ✅ **ユーティリティ**: 各種便利関数が動作する
- ✅ **並行性**: AllOfとAnyOfが正しく動作する
- ✅ **レースコンディション**: 複数goroutineからの同時アクセスが安全

## 実行方法

```bash
go test -v                    # 詳細なテスト実行
go test -race                 # レースコンディション検出
go test -bench=.              # ベンチマーク実行
go test -cover                # カバレッジ測定
```

## 参考資料

- [Go Generics Tutorial](https://go.dev/doc/tutorial/generics)
- [sync.Once Documentation](https://pkg.go.dev/sync#Once)
- [Context Package](https://pkg.go.dev/context)
- [Channel Direction](https://go.dev/tour/concurrency/3)
- [Select Statement](https://go.dev/tour/concurrency/5)
- [Effective Go - Concurrency](https://go.dev/doc/effective_go#concurrency)