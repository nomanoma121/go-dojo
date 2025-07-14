# Day 32: 指数バックオフリトライ実装

## 🎯 本日の目標

一時的なDBエラーや外部サービスの障害に対して、指数バックオフアルゴリズムを使った効率的なリトライ機能を実装する。実用的なシナリオを通じて、堅牢で拡張性の高い障害回復システムを構築できるようになる。

## 📖 解説

### 指数バックオフとは

指数バックオフ（Exponential Backoff）は、失敗したリクエストを再試行する際に、待機時間を指数的に増加させる**障害回復アルゴリズム**です。これにより、一時的な障害時にシステムへの負荷を軽減しながら、効率的にリトライを行うことができます。

**なぜ固定間隔リトライが問題なのか：**

```go
// ❌ 固定間隔リトライの問題例
func badRetryPattern() {
    for attempt := 0; attempt < 5; attempt++ {
        err := makeRequest()
        if err == nil {
            return // 成功
        }
        
        time.Sleep(1 * time.Second) // 常に1秒待機
        // 問題：多数のクライアントが同時に1秒間隔でリトライ
        // → サーバーに定期的な負荷スパイクが発生
        // → 復旧を妨げる可能性
    }
}
```

**指数バックオフによる改善：**

```go
// ✅ 指数バックオフの改善例
func goodRetryPattern() {
    baseDelay := 100 * time.Millisecond
    
    for attempt := 0; attempt < 5; attempt++ {
        err := makeRequest()
        if err == nil {
            return // 成功
        }
        
        // 指数的に増加: 100ms, 200ms, 400ms, 800ms, 1600ms
        delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
        time.Sleep(delay)
        // 利点：リトライ間隔が徐々に増加
        // → サーバーへの負荷が分散
        // → システム復旧の時間を確保
    }
}
```

### 指数バックオフが必要な理由

#### 1. **システム負荷の分散**

```plaintext
固定間隔リトライ（1秒間隔）:
Client A: [Request] -> Error -> Wait 1s -> [Request] -> Error -> Wait 1s -> [Request]
Client B: [Request] -> Error -> Wait 1s -> [Request] -> Error -> Wait 1s -> [Request]  
Client C: [Request] -> Error -> Wait 1s -> [Request] -> Error -> Wait 1s -> [Request]
結果: サーバーに毎秒3つのリクエストが集中

指数バックオフ:
Client A: [Request] -> Error -> Wait 100ms -> [Request] -> Error -> Wait 200ms -> [Request]
Client B: [Request] -> Error -> Wait 100ms -> [Request] -> Error -> Wait 200ms -> [Request]
Client C: [Request] -> Error -> Wait 100ms -> [Request] -> Error -> Wait 200ms -> [Request]
結果: 負荷が時間的に分散され、サーバーの回復時間を確保
```

#### 2. **カスケード障害の防止**

```go
// カスケード障害のシミュレーション
func demonstrateCascadeFailure() {
    // 100個のクライアントが同時にリトライ
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(clientID int) {
            defer wg.Done()
            
            // 固定間隔だと全クライアントが同期してリトライ
            for attempt := 0; attempt < 3; attempt++ {
                err := makeRequest()
                if err == nil {
                    return
                }
                time.Sleep(1 * time.Second) // 全員が同時に1秒待機
            }
        }(i)
    }
    wg.Wait()
    // 結果: サーバーに1秒毎に100リクエストが一斉に送信される
}
```

#### 3. **効率的な復旧支援**

```go
// サーバー復旧パターンの例
func serverRecoveryPattern() {
    // t=0s: サーバー障害発生
    // t=1s: 100クライアントが固定間隔でリトライ → サーバー過負荷継続
    // t=2s: 100クライアントが再度リトライ → 復旧を妨げる
    
    // 指数バックオフを使用した場合:
    // t=0.1s: 一部クライアントがリトライ → 軽微な負荷
    // t=0.2s: さらに一部がリトライ → 段階的負荷増加
    // t=0.4s: 負荷は分散され、サーバーの復旧時間を確保
}
```

### 指数バックオフアルゴリズムの実装

#### 基本実装パターン

```go
package main

import (
    "fmt"
    "math"
    "math/rand"
    "time"
)

// 基本的な指数バックオフの実装
func basicExponentialBackoff(attempt int, baseDelay time.Duration) time.Duration {
    // 計算式: 2^attempt * baseDelay
    multiplier := math.Pow(2, float64(attempt))
    delay := time.Duration(multiplier) * baseDelay
    return delay
}

// 例: attempt=0: 2^0 * 100ms = 100ms
//     attempt=1: 2^1 * 100ms = 200ms  
//     attempt=2: 2^2 * 100ms = 400ms
//     attempt=3: 2^3 * 100ms = 800ms

// ジッター付き指数バックオフ
func exponentialBackoffWithJitter(attempt int, baseDelay time.Duration) time.Duration {
    maxDelay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
    
    // フルジッター: 0 から maxDelay までのランダム値
    jitter := time.Duration(rand.Float64() * float64(maxDelay))
    return jitter
}

// イコールジッター（推奨）
func exponentialBackoffWithEqualJitter(attempt int, baseDelay time.Duration) time.Duration {
    baseTime := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
    
    // 半分は固定、半分はランダム
    jitter := time.Duration(rand.Float64() * float64(baseTime))
    return baseTime/2 + jitter
}

// 上限付き指数バックオフ
func cappedExponentialBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
    delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
    
    if delay > maxDelay {
        return maxDelay
    }
    return delay
}
```

#### ジッターの重要性

```go
// ジッターなしの問題例
func demonstrateThunderingHerd() {
    fmt.Println("=== ジッターなしの同期問題 ===")
    
    // 複数のクライアントが同時に開始
    start := time.Now()
    for i := 0; i < 5; i++ {
        go func(clientID int) {
            for attempt := 0; attempt < 3; attempt++ {
                delay := basicExponentialBackoff(attempt, 100*time.Millisecond)
                elapsed := time.Since(start)
                fmt.Printf("Client %d: リトライ at %v (delay: %v)\n", 
                    clientID, elapsed.Truncate(time.Millisecond), delay)
                time.Sleep(delay)
            }
        }(i)
    }
    
    // 出力例:
    // Client 0: リトライ at 0s (delay: 100ms)
    // Client 1: リトライ at 0s (delay: 100ms)    <- 全て同じタイミング
    // Client 2: リトライ at 0s (delay: 100ms)    <- Thundering Herd
    // Client 3: リトライ at 0s (delay: 100ms)
    // Client 4: リトライ at 0s (delay: 100ms)
}

// ジッター付きの改善例
func demonstrateJitterBenefit() {
    fmt.Println("=== ジッター付きの負荷分散 ===")
    
    start := time.Now()
    for i := 0; i < 5; i++ {
        go func(clientID int) {
            for attempt := 0; attempt < 3; attempt++ {
                delay := exponentialBackoffWithEqualJitter(attempt, 100*time.Millisecond)
                elapsed := time.Since(start)
                fmt.Printf("Client %d: リトライ at %v (delay: %v)\n", 
                    clientID, elapsed.Truncate(time.Millisecond), delay)
                time.Sleep(delay)
            }
        }(i)
    }
    
    // 出力例:
    // Client 0: リトライ at 0s (delay: 73ms)     <- ランダムに分散
    // Client 1: リトライ at 0s (delay: 134ms)    <- Thundering Herd回避
    // Client 2: リトライ at 0s (delay: 91ms)
    // Client 3: リトライ at 0s (delay: 156ms)
    // Client 4: リトライ at 0s (delay: 108ms)
}
```

### 実践的なRetryManagerシステム

#### 高機能なRetryConfig

```go
// RetryConfig は包括的なリトライ設定を提供
type RetryConfig struct {
    MaxRetries      int                    // 最大リトライ回数
    BaseDelay       time.Duration          // 基本遅延時間
    MaxDelay        time.Duration          // 最大遅延時間（上限）
    Multiplier      float64                // 指数の底（通常2.0）
    Jitter          JitterType             // ジッター種別
    RetryableErrors []ErrorMatcher         // リトライ対象エラー
    Timeout         time.Duration          // 全体のタイムアウト
    OnRetry         func(attempt int, err error) // リトライ時のコールバック
}

// JitterType はジッターの種類を定義
type JitterType int

const (
    NoJitter JitterType = iota
    FullJitter    // 0 から計算値まで完全ランダム
    EqualJitter   // 半分固定、半分ランダム（推奨）
    DecorrelatedJitter // 前回の値を基準にしたランダム
)

// ErrorMatcher はエラー判定のインターフェース
type ErrorMatcher interface {
    Matches(error) bool
}

// RetryableFunc はリトライ対象の関数型
type RetryableFunc func() error

// RetryManager は高度なリトライ機能を提供
type RetryManager struct {
    config   RetryConfig
    stats    RetryStats
    stopChan chan struct{}
    mu       sync.RWMutex
}

// RetryStats はリトライ統計を管理
type RetryStats struct {
    TotalAttempts   int64         // 総試行回数
    SuccessCount    int64         // 成功回数
    FailureCount    int64         // 最終失敗回数
    TotalRetries    int64         // 総リトライ回数
    TotalWaitTime   time.Duration // 総待機時間
    LastSuccess     time.Time     // 最後の成功時刻
    LastFailure     time.Time     // 最後の失敗時刻
    mu              sync.RWMutex
}

// NewRetryManager は新しいRetryManagerを作成
func NewRetryManager(config RetryConfig) *RetryManager {
    if config.Multiplier <= 0 {
        config.Multiplier = 2.0
    }
    if config.BaseDelay <= 0 {
        config.BaseDelay = 100 * time.Millisecond
    }
    
    return &RetryManager{
        config:   config,
        stopChan: make(chan struct{}),
    }
}

// Execute はリトライロジック付きで関数を実行
func (rm *RetryManager) Execute(fn RetryableFunc) error {
    return rm.ExecuteWithContext(context.Background(), fn)
}

// ExecuteWithContext はコンテキスト付きでリトライ実行
func (rm *RetryManager) ExecuteWithContext(ctx context.Context, fn RetryableFunc) error {
    startTime := time.Now()
    var lastErr error
    
    // タイムアウト設定
    if rm.config.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, rm.config.Timeout)
        defer cancel()
    }
    
    for attempt := 0; attempt <= rm.config.MaxRetries; attempt++ {
        select {
        case <-ctx.Done():
            rm.recordFailure(lastErr)
            return ctx.Err()
        case <-rm.stopChan:
            return fmt.Errorf("retry manager stopped")
        default:
        }
        
        rm.recordAttempt()
        
        // 関数実行
        lastErr = fn()
        if lastErr == nil {
            rm.recordSuccess()
            return nil
        }
        
        // リトライ可能エラーか判定
        if !rm.isRetryable(lastErr) {
            rm.recordFailure(lastErr)
            return lastErr
        }
        
        // 最後の試行でない場合は遅延
        if attempt < rm.config.MaxRetries {
            delay := rm.calculateDelay(attempt)
            rm.recordWaitTime(delay)
            
            // コールバック実行
            if rm.config.OnRetry != nil {
                rm.config.OnRetry(attempt+1, lastErr)
            }
            
            select {
            case <-ctx.Done():
                rm.recordFailure(lastErr)
                return ctx.Err()
            case <-time.After(delay):
                rm.recordRetry()
                continue
            }
        }
    }
    
    rm.recordFailure(lastErr)
    return fmt.Errorf("operation failed after %d retries: %w", rm.config.MaxRetries, lastErr)
}

// calculateDelay は試行回数に基づく遅延時間を計算
func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
    // 基本の指数バックオフ計算
    base := float64(rm.config.BaseDelay)
    multiplier := math.Pow(rm.config.Multiplier, float64(attempt))
    delay := time.Duration(base * multiplier)
    
    // 最大遅延時間の制限
    if rm.config.MaxDelay > 0 && delay > rm.config.MaxDelay {
        delay = rm.config.MaxDelay
    }
    
    // ジッター適用
    switch rm.config.Jitter {
    case FullJitter:
        delay = time.Duration(rand.Float64() * float64(delay))
    case EqualJitter:
        jitter := time.Duration(rand.Float64() * float64(delay))
        delay = delay/2 + jitter
    case DecorrelatedJitter:
        // 前回の値の3倍まで、最低でも基本遅延時間
        prevDelay := delay
        maxJitter := 3 * prevDelay
        if maxJitter < rm.config.BaseDelay {
            maxJitter = rm.config.BaseDelay
        }
        delay = time.Duration(rand.Float64() * float64(maxJitter))
    }
    
    return delay
}

// isRetryable はエラーがリトライ可能か判定
func (rm *RetryManager) isRetryable(err error) bool {
    if err == nil {
        return false
    }
    
    // 設定されたErrorMatcherで判定
    for _, matcher := range rm.config.RetryableErrors {
        if matcher.Matches(err) {
            return true
        }
    }
    
    // デフォルトのリトライ可能エラー判定
    return isDefaultRetryableError(err)
}

// Stop はRetryManagerを停止
func (rm *RetryManager) Stop() {
    close(rm.stopChan)
}

// GetStats は統計情報を取得
func (rm *RetryManager) GetStats() RetryStats {
    rm.stats.mu.RLock()
    defer rm.stats.mu.RUnlock()
    return rm.stats
}
```

#### 専用ErrorMatcherの実装

```go
// DatabaseErrorMatcher はデータベースエラー用のマッチャー
type DatabaseErrorMatcher struct{}

func (DatabaseErrorMatcher) Matches(err error) bool {
    if err == nil {
        return false
    }
    
    errStr := strings.ToLower(err.Error())
    retryablePatterns := []string{
        "connection refused",
        "connection reset",
        "timeout",
        "temporary failure",
        "server is not ready",
        "deadlock detected",
        "lock wait timeout",
        "too many connections",
    }
    
    for _, pattern := range retryablePatterns {
        if strings.Contains(errStr, pattern) {
            return true
        }
    }
    
    return false
}

// HTTPErrorMatcher はHTTPエラー用のマッチャー
type HTTPErrorMatcher struct {
    RetryableCodes []int
}

func (h HTTPErrorMatcher) Matches(err error) bool {
    if err == nil {
        return false
    }
    
    // net/httpのエラーから状態コードを抽出
    if urlErr, ok := err.(*url.Error); ok {
        if httpErr, ok := urlErr.Err.(*http.ResponseError); ok {
            for _, code := range h.RetryableCodes {
                if httpErr.StatusCode == code {
                    return true
                }
            }
        }
    }
    
    return false
}

// CircuitBreakerErrorMatcher はサーキットブレーカーエラー用
type CircuitBreakerErrorMatcher struct{}

func (CircuitBreakerErrorMatcher) Matches(err error) bool {
    return strings.Contains(err.Error(), "circuit breaker open")
}

// CompositeErrorMatcher は複数のマッチャーを組み合わせ
type CompositeErrorMatcher struct {
    Matchers []ErrorMatcher
}

func (c CompositeErrorMatcher) Matches(err error) bool {
    for _, matcher := range c.Matchers {
        if matcher.Matches(err) {
            return true
        }
    }
    return false
}
```

#### 統計情報管理の実装

```go
func (rs *RetryStats) recordAttempt() {
    rs.mu.Lock()
    defer rs.mu.Unlock()
    rs.TotalAttempts++
}

func (rs *RetryStats) recordSuccess() {
    rs.mu.Lock()
    defer rs.mu.Unlock()
    rs.SuccessCount++
    rs.LastSuccess = time.Now()
}

func (rs *RetryStats) recordFailure(err error) {
    rs.mu.Lock()
    defer rs.mu.Unlock()
    rs.FailureCount++
    rs.LastFailure = time.Now()
}

func (rs *RetryStats) recordRetry() {
    rs.mu.Lock()
    defer rs.mu.Unlock()
    rs.TotalRetries++
}

func (rs *RetryStats) recordWaitTime(duration time.Duration) {
    rs.mu.Lock()
    defer rs.mu.Unlock()
    rs.TotalWaitTime += duration
}

// SuccessRate は成功率を計算
func (rs *RetryStats) SuccessRate() float64 {
    rs.mu.RLock()
    defer rs.mu.RUnlock()
    
    if rs.TotalAttempts == 0 {
        return 0
    }
    return float64(rs.SuccessCount) / float64(rs.TotalAttempts)
}

// AverageRetries は平均リトライ回数を計算
func (rs *RetryStats) AverageRetries() float64 {
    rs.mu.RLock()
    defer rs.mu.RUnlock()
    
    totalOperations := rs.SuccessCount + rs.FailureCount
    if totalOperations == 0 {
        return 0
    }
    return float64(rs.TotalRetries) / float64(totalOperations)
}

func isDefaultRetryableError(err error) bool {
    if err == nil {
        return false
    }
    
    // net.Errorインターフェースでの判定
    if netErr, ok := err.(net.Error); ok {
        return netErr.Temporary() || netErr.Timeout()
    }
    
    // context.Errorでの判定
    if err == context.DeadlineExceeded {
        return true
    }
    
    return false
}
```

### データベース特有のリトライ戦略

データベース接続では、特定のエラーのみをリトライ対象とします：

```go
func isRetryableDBError(err error) bool {
    if err == nil {
        return false
    }
    
    errStr := err.Error()
    
    // 再試行可能なエラーパターン
    retryablePatterns := []string{
        "connection refused",
        "connection reset",
        "timeout",
        "temporary failure",
        "server is not ready",
        "deadlock detected",
        "lock wait timeout",
    }
    
    for _, pattern := range retryablePatterns {
        if strings.Contains(strings.ToLower(errStr), pattern) {
            return true
        }
    }
    
    return false
}
```

### サーキットブレーカーとの組み合わせ

指数バックオフとサーキットブレーカーを組み合わせることで、より堅牢なシステムを構築できます：

```go
type CircuitBreakerRetry struct {
    circuitBreaker *CircuitBreaker
    retryConfig    RetryConfig
}

func (cbr *CircuitBreakerRetry) Execute(fn RetryableFunc) error {
    return executeWithRetry(cbr.retryConfig, func() error {
        return cbr.circuitBreaker.Execute(fn)
    })
}
```

### コンテキストとの統合

長時間のリトライがアプリケーションをブロックしないよう、コンテキストを使用したキャンセレーション機能を追加：

```go
func executeWithRetryAndContext(ctx context.Context, config RetryConfig, fn RetryableFunc) error {
    for attempt := 0; attempt <= config.MaxRetries; attempt++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }
        
        err := fn()
        if err == nil {
            return nil
        }
        
        if !isRetryableError(err) {
            return err
        }
        
        if attempt < config.MaxRetries {
            delay := calculateDelay(config, attempt)
            
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(delay):
                // 続行
            }
        }
    }
    
    return fmt.Errorf("operation failed after %d retries", config.MaxRetries)
}
```

📝 **課題**

以下の機能を持つ指数バックオフリトライシステムを実装してください：

1. **`RetryConfig`構造体**: リトライ設定を管理
2. **`RetryManager`構造体**: リトライロジックを実装
3. **`Execute`メソッド**: 指定された設定でリトライ実行
4. **`ExecuteWithContext`メソッド**: コンテキスト付きリトライ実行
5. **`DatabaseRetry`**: データベース特有のリトライ戦略

具体的な実装要件：
- 指数バックオフアルゴリズムの実装
- ジッター（ランダムな遅延）の追加機能
- 最大遅延時間の制限
- 再試行可能エラーの判定
- コンテキストによるキャンセレーション対応
- 統計情報の収集（リトライ回数、成功率など）

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestRetryManager_Execute
--- PASS: TestRetryManager_Execute (0.01s)
=== RUN   TestRetryManager_ExecuteWithContext
--- PASS: TestRetryManager_ExecuteWithContext (0.02s)
=== RUN   TestRetryManager_ExponentialBackoff
--- PASS: TestRetryManager_ExponentialBackoff (0.05s)
=== RUN   TestRetryManager_Jitter
--- PASS: TestRetryManager_Jitter (0.03s)
=== RUN   TestDatabaseRetry_RetryableErrors
--- PASS: TestDatabaseRetry_RetryableErrors (0.01s)
=== RUN   TestRetryManager_Statistics
--- PASS: TestRetryManager_Statistics (0.02s)
PASS
ok      day32-exponential-backoff    0.145s
```

リトライのログ出力例：
```
2024/07/13 10:30:00 Attempt 1 failed: connection refused, retrying in 100ms
2024/07/13 10:30:00 Attempt 2 failed: connection refused, retrying in 200ms
2024/07/13 10:30:01 Attempt 3 failed: connection refused, retrying in 400ms
2024/07/13 10:30:01 Operation succeeded on attempt 4
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **math**パッケージ: 指数計算（`math.Pow`）
2. **math/rand**パッケージ: ジッター用の乱数生成
3. **time**パッケージ: 遅延処理（`time.Sleep`, `time.After`）
4. **context**パッケージ: キャンセレーション制御
5. **strings**パッケージ: エラーメッセージの判定
6. **sync**パッケージ: 統計情報の並行安全性

エラー分類の例：
- **再試行可能**: ネットワークエラー、タイムアウト、一時的なサービス不可
- **再試行不可能**: 認証エラー、無効なパラメータ、リソース不足

バックオフ計算式の例：
```
delay = min(baseDelay * (multiplier ^ attempt), maxDelay)
with jitter: delay += random(-jitter%, +jitter%)
```

## 実行方法

```bash
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```