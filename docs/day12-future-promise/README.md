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
// 【Future/Promiseパターンの完全実装】プロダクション品質の非同期処理
// ❌ 問題例：コールバック地獄による可読性とメンテナンス性の崩壊
func callbackHellDisaster() {
    // 🚨 災害例：ネストしたコールバックによる複雑性の爆発
    fetchUser(123, func(user User, err error) {
        if err != nil {
            log.Printf("❌ Failed to fetch user: %v", err)
            return
        }
        
        fetchUserPosts(user.ID, func(posts []Post, err error) {
            if err != nil {
                log.Printf("❌ Failed to fetch posts: %v", err)
                return
            }
            
            for _, post := range posts {
                fetchComments(post.ID, func(comments []Comment, err error) {
                    if err != nil {
                        log.Printf("❌ Failed to fetch comments: %v", err)
                        return
                    }
                    
                    for _, comment := range comments {
                        fetchUserProfile(comment.UserID, func(profile UserProfile, err error) {
                            if err != nil {
                                log.Printf("❌ Failed to fetch profile: %v", err)
                                return
                            }
                            
                            // ❌ 16層のネスト！！コードの可読性が皆無
                            // ❌ エラーハンドリングが重複・複雑化
                            // ❌ 並行処理が困難（全て順次実行）
                            // ❌ テストが不可能（モックが困難）
                            processProfile(profile)
                        })
                    }
                })
            }
        })
    })
    // 結果：開発速度低下、バグ多発、メンテナンス不可能
}

// ✅ 正解：Future/Promiseによる宣言的非同期処理
// 結果を表現する構造体（成功またはエラー）
type Result[T any] struct {
    Value     T              // 成功時の値
    Error     error          // エラー時のエラー
    Timestamp time.Time      // 結果取得時刻
    Duration  time.Duration  // 処理にかかった時間
}

// 【プロダクション品質Future】高度な機能付き非同期処理
type Future[T any] struct {
    // 【基本機能】
    result     chan Result[T]  // 結果を受け渡すチャネル
    done       chan struct{}   // 完了を通知するチャネル
    
    // 【高度な機能】
    startTime  time.Time       // 処理開始時刻
    callbacks  []func(Result[T]) // 完了時のコールバック関数
    chainNext  *Future[any]    // チェーン処理用の次のFuture
    
    // 【制御機能】
    ctx        context.Context // コンテキスト（タイムアウト・キャンセル）
    cancel     context.CancelFunc
    
    // 【統計・監視】
    futureID   string          // デバッグ用の一意ID
    createdAt  time.Time       // Future作成時刻
    
    // 【スレッドセーフティ】
    mu         sync.RWMutex    // 読み書きミューテックス
    resolved   bool            // 解決済みフラグ
}

// 【プロダクション品質Promise】一度性保証とエラーハンドリング
type Promise[T any] struct {
    future     *Future[T]      // 関連するFuture
    once       sync.Once       // 一度だけの実行を保証
    logger     *log.Logger     // ログ出力用
    
    // 【高度な機能】
    timeout    time.Duration   // デフォルトタイムアウト
    retryCount int             // リトライ回数
    metadata   map[string]interface{} // メタデータ
}

// 【重要関数】Future/Promiseペアの作成
func NewFuturePromise[T any](ctx context.Context, timeout time.Duration) (*Future[T], *Promise[T]) {
    // タイムアウト付きコンテキスト
    if timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, timeout)
        // cancelは返されるFutureで管理
        _ = cancel
    }
    
    futureID := generateFutureID()
    now := time.Now()
    
    future := &Future[T]{
        result:    make(chan Result[T], 1), // バッファサイズ1で非ブロッキング
        done:      make(chan struct{}),
        startTime: now,
        callbacks: make([]func(Result[T]), 0),
        ctx:       ctx,
        futureID:  futureID,
        createdAt: now,
    }
    
    promise := &Promise[T]{
        future:   future,
        logger:   log.New(os.Stdout, fmt.Sprintf("[Future-%s] ", futureID[:8]), log.LstdFlags),
        timeout:  timeout,
        metadata: make(map[string]interface{}),
    }
    
    // 【重要】タイムアウト監視の開始
    go promise.startTimeoutMonitoring()
    
    promise.logger.Printf("🚀 Future/Promise pair created (timeout: %v)", timeout)
    return future, promise
}

// 【重要メソッド】成功結果の設定
func (p *Promise[T]) Resolve(value T) bool {
    resolved := false
    
    p.once.Do(func() {
        duration := time.Since(p.future.startTime)
        
        result := Result[T]{
            Value:     value,
            Timestamp: time.Now(),
            Duration:  duration,
        }
        
        // 【非ブロッキング送信】
        select {
        case p.future.result <- result:
            resolved = true
            
            // 【状態更新】
            p.future.mu.Lock()
            p.future.resolved = true
            p.future.mu.Unlock()
            
            close(p.future.done)
            
            p.logger.Printf("✅ Promise resolved successfully (took %v)", duration)
            
            // 【コールバック実行】
            go p.executeCallbacks(result)
            
        case <-p.future.ctx.Done():
            // コンテキストがキャンセルされた場合
            p.logger.Printf("❌ Promise resolution cancelled")
        }
    })
    
    return resolved
}

// 【重要メソッド】エラー結果の設定
func (p *Promise[T]) Reject(err error) bool {
    rejected := false
    
    p.once.Do(func() {
        duration := time.Since(p.future.startTime)
        
        result := Result[T]{
            Error:     err,
            Timestamp: time.Now(),
            Duration:  duration,
        }
        
        select {
        case p.future.result <- result:
            rejected = true
            
            p.future.mu.Lock()
            p.future.resolved = true
            p.future.mu.Unlock()
            
            close(p.future.done)
            
            p.logger.Printf("❌ Promise rejected with error: %v (took %v)", err, duration)
            
            // エラー時もコールバック実行
            go p.executeCallbacks(result)
            
        case <-p.future.ctx.Done():
            p.logger.Printf("❌ Promise rejection cancelled")
        }
    })
    
    return rejected
}

// 【重要メソッド】結果の取得（ブロッキング）
func (f *Future[T]) Get() (T, error) {
    select {
    case result := <-f.result:
        if result.Error != nil {
            var zero T
            return zero, result.Error
        }
        return result.Value, nil
        
    case <-f.ctx.Done():
        var zero T
        return zero, f.ctx.Err()
    }
}

// 【重要メソッド】タイムアウト付き結果取得
func (f *Future[T]) GetWithTimeout(timeout time.Duration) (T, error) {
    select {
    case result := <-f.result:
        if result.Error != nil {
            var zero T
            return zero, result.Error
        }
        return result.Value, nil
        
    case <-time.After(timeout):
        var zero T
        return zero, fmt.Errorf("future timeout after %v", timeout)
        
    case <-f.ctx.Done():
        var zero T
        return zero, f.ctx.Err()
    }
}

// 【高度な機能】複数Futureの統合処理
func All[T any](ctx context.Context, futures ...*Future[T]) *Future[[]T] {
    resultFuture, resultPromise := NewFuturePromise[[]T](ctx, 0)
    
    if len(futures) == 0 {
        resultPromise.Resolve([]T{})
        return resultFuture
    }
    
    go func() {
        results := make([]T, len(futures))
        var wg sync.WaitGroup
        var mu sync.Mutex
        var firstError error
        
        for i, future := range futures {
            wg.Add(1)
            go func(index int, f *Future[T]) {
                defer wg.Done()
                
                value, err := f.Get()
                
                mu.Lock()
                defer mu.Unlock()
                
                if err != nil && firstError == nil {
                    firstError = err
                } else if err == nil {
                    results[index] = value
                }
            }(i, future)
        }
        
        wg.Wait()
        
        if firstError != nil {
            resultPromise.Reject(firstError)
        } else {
            resultPromise.Resolve(results)
        }
    }()
    
    return resultFuture
}

// 【実用例】Future/Promiseによる宣言的非同期処理
func elegantAsyncProcessing() {
    ctx := context.Background()
    
    // 【STEP 1】ユーザー情報取得
    userFuture, userPromise := NewFuturePromise[User](ctx, 10*time.Second)
    go func() {
        user, err := fetchUserFromDB(123)
        if err != nil {
            userPromise.Reject(err)
        } else {
            userPromise.Resolve(user)
        }
    }()
    
    // 【STEP 2】ユーザー投稿取得（並行実行）
    postsFuture, postsPromise := NewFuturePromise[[]Post](ctx, 15*time.Second)
    go func() {
        posts, err := fetchUserPosts(123)
        if err != nil {
            postsPromise.Reject(err)
        } else {
            postsPromise.Resolve(posts)
        }
    }()
    
    // 【STEP 3】プロフィール画像取得（並行実行）
    imageFuture, imagePromise := NewFuturePromise[[]byte](ctx, 20*time.Second)
    go func() {
        image, err := fetchProfileImage(123)
        if err != nil {
            imagePromise.Reject(err)
        } else {
            imagePromise.Resolve(image)
        }
    }()
    
    // 【STEP 4】全ての結果を統合
    user, err := userFuture.Get()
    if err != nil {
        log.Printf("❌ Failed to get user: %v", err)
        return
    }
    
    posts, err := postsFuture.Get()
    if err != nil {
        log.Printf("❌ Failed to get posts: %v", err)
        return
    }
    
    image, err := imageFuture.Get()
    if err != nil {
        log.Printf("⚠️  Failed to get image, using default: %v", err)
        image = getDefaultProfileImage()
    }
    
    // 【成功】全てのデータが取得完了
    profile := UserProfile{
        User:  user,
        Posts: posts,
        Image: image,
    }
    
    log.Printf("✅ Profile completed: %d posts, %d bytes image", len(posts), len(image))
    processProfile(profile)
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