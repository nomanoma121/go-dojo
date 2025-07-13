# Day 32: 指数バックオフリトライ

🎯 **本日の目標**

一時的なDBエラーや外部サービスの障害に対して、指数バックオフアルゴリズムを使った効率的なリトライ機能を実装できるようになる。

📖 **解説**

## 指数バックオフとは

指数バックオフ（Exponential Backoff）は、失敗したリクエストを再試行する際に、待機時間を指数的に増加させるアルゴリズムです。これにより、一時的な障害時にシステムへの負荷を軽減しながら、効率的にリトライを行うことができます。

### なぜ指数バックオフが必要か

1. **システム負荷軽減**: 固定間隔でのリトライは、障害中のシステムに過度な負荷をかける
2. **カスケード障害防止**: 複数のクライアントが同時にリトライすることで生じる悪循環を防ぐ
3. **効率的な復旧**: 適切な間隔でリトライすることで、システム復旧後に迅速に処理を再開

### 基本的な指数バックオフのアルゴリズム

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
    // 2^attempt * baseDelay
    delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
    return delay
}

// ジッターを追加した指数バックオフ
func exponentialBackoffWithJitter(attempt int, baseDelay time.Duration) time.Duration {
    maxDelay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
    
    // 0から最大遅延時間までのランダムな値を追加
    jitter := time.Duration(rand.Float64() * float64(maxDelay))
    return jitter
}
```

### 実践的な指数バックオフ実装

```go
type RetryConfig struct {
    MaxRetries   int
    BaseDelay    time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
    Jitter       bool
}

type RetryableFunc func() error

func executeWithRetry(config RetryConfig, fn RetryableFunc) error {
    var err error
    
    for attempt := 0; attempt <= config.MaxRetries; attempt++ {
        err = fn()
        if err == nil {
            return nil
        }
        
        // 再試行不可能なエラーの場合は即座に終了
        if !isRetryableError(err) {
            return err
        }
        
        if attempt < config.MaxRetries {
            delay := calculateDelay(config, attempt)
            time.Sleep(delay)
        }
    }
    
    return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, err)
}

func calculateDelay(config RetryConfig, attempt int) time.Duration {
    delay := float64(config.BaseDelay) * math.Pow(config.Multiplier, float64(attempt))
    
    if config.MaxDelay > 0 && time.Duration(delay) > config.MaxDelay {
        delay = float64(config.MaxDelay)
    }
    
    if config.Jitter {
        // ±25%のジッターを追加
        jitterRange := delay * 0.25
        jitter := (rand.Float64() - 0.5) * 2 * jitterRange
        delay += jitter
    }
    
    return time.Duration(delay)
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