# Day 02: Contextによるタイムアウト/デッドライン

## 🎯 本日の目標 (Today's Goal)

Goのcontext.Contextを使用したタイムアウトとデッドライン制御を完全に理解し、実装する。外部API呼び出し、データベース接続、ネットワーク通信などの時間制約がある処理において、適切なタイムアウト制御により信頼性の高いアプリケーションを構築する。指数バックオフによるリトライ機能、グレースフルなタイムアウト処理、リソースリーク防止を含む包括的な時間制御システムを習得する。

## 📖 解説 (Explanation)

### なぜタイムアウト制御が必要なのか

現代のアプリケーションでは、ネットワーク経由での外部サービス呼び出しが一般的です。しかし、ネットワークの不安定さや外部サービスの問題により、処理が予期せず長時間ブロックされる可能性があります。

#### 1. タイムアウトなしの問題

```go
// 【危険な例】：タイムアウトなしのHTTP呼び出し - 本番環境では絶対に避けるべき
func dangerousAPICall(url string) (*http.Response, error) {
    // 【致命的問題1】タイムアウトが設定されていない
    // http.Client{}のデフォルト設定：
    // - Timeout: 0 (無制限)
    // - 接続タイムアウト: 無制限
    // - レスポンス読み取りタイムアウト: 無制限
    client := &http.Client{}
    
    // 【致命的問題2】外部サービスが応答しない場合、無限に待機
    // 以下のシナリオで永続的にブロック：
    // - ネットワーク分断
    // - サーバーのハング
    // - DNS解決の失敗
    // - TCP接続の確立タイムアウト
    resp, err := client.Get(url)
    if err != nil {
        return nil, err
    }
    
    return resp, nil
}

// 【問題のシナリオ】実際のプロダクション環境で発生しうる災害的状況
func problematicExample() {
    // 【災害シナリオ】複数のAPIを並行呼び出し
    for i := 0; i < 100; i++ {
        go func(id int) {
            // 【重大リスク】各Goroutineが無期限に待機する可能性
            // 外部APIが応答しない場合：
            // 1. 100個のGoroutineが永続的にブロック
            // 2. メモリ使用量が蓄積（各Goroutine = 約2-8KB）
            // 3. ファイルディスクリプタ枯渇
            // 4. アプリケーション全体の応答停止
            resp, err := dangerousAPICall("https://slow-api.example.com")
            if err != nil {
                log.Printf("Request %d failed: %v", id, err)
                return
            }
            defer resp.Body.Close()
            
            // 【追加問題】レスポンス処理中にもブロック可能性
            // resp.Body.Read()も無制限に待機する可能性がある
        }(i) // 【ループ変数キャプチャ】正しい実装
    }
    
    // 【結果】100個のGoroutineが同時にハングし、
    // アプリケーション全体が実質的に停止状態になる
    // サーバー再起動が唯一の復旧手段となる
}
```

この例の問題点：
- **リソース枯渇**: 大量のGoroutineが同時にブロック
- **レスポンス劣化**: ユーザー体験の大幅な悪化
- **デッドロック**: 依存関係のある処理での停止
- **監視困難**: 問題の検知と対処が困難

#### 2. 時間制約のある処理の例

```go
// リアルな使用例
type UserService struct {
    dbClient    *sql.DB
    apiClient   *http.Client
    cacheClient *redis.Client
}

func (us *UserService) GetUserProfile(userID string) (*UserProfile, error) {
    // 1. データベースからユーザー基本情報を取得（最大2秒）
    // 2. 外部APIから追加情報を取得（最大3秒）
    // 3. キャッシュに結果を保存（最大1秒）
    // 
    // 総処理時間は6秒以下でなければならない
}
```

### Context.WithTimeoutとContext.WithDeadline

#### 1. WithTimeoutの基本使用法

```go
import (
    "context"
    "fmt"
    "net/http"
    "time"
)

// 【正しい実装】タイムアウト付きHTTPクライアント
// プロダクション環境で使用すべき安全な設計パターン
type TimeoutHTTPClient struct {
    client  *http.Client      // 【基盤】実際のHTTP通信を行うクライアント
    timeout time.Duration     // 【制約】リクエスト全体のタイムアウト時間
}

// 【コンストラクタ】安全なHTTPクライアントを作成
func NewTimeoutHTTPClient(timeout time.Duration) *TimeoutHTTPClient {
    return &TimeoutHTTPClient{
        // 【重要】ここでもclient自体にタイムアウトを設定可能
        // しかし、Context方式の方が柔軟性が高いため基本設定のまま使用
        client:  &http.Client{},
        timeout: timeout,
    }
}

// 【安全なGET実装】タイムアウト制御付きHTTPリクエスト
func (thc *TimeoutHTTPClient) Get(url string) (*http.Response, error) {
    // 【Step 1】指定時間でタイムアウトするContextを作成
    // context.WithTimeout()の動作：
    // - 指定時間後にctx.Done()チャネルがクローズされる
    // - ctx.Err()がcontext.DeadlineExceededを返すようになる
    // - 自動的にHTTPリクエストがキャンセルされる
    ctx, cancel := context.WithTimeout(context.Background(), thc.timeout)
    
    // 【重要】defer cancel()でリソースリークを防ぐ
    // この処理により以下が保証される：
    // 1. 関数終了時に必ずcancel()が呼ばれる
    // 2. Contextに関連する内部リソースが解放される
    // 3. タイムアウト前に処理が完了してもリソースが残らない
    defer cancel()
    
    // 【Step 2】Contextを使ってリクエストを作成
    // http.NewRequestWithContext()により：
    // - HTTPリクエスト自体にContextを関連付け
    // - ネットワーク層でのタイムアウト制御が有効化
    // - TCP接続、DNS解決、レスポンス読み取りすべてに適用
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    // 【Step 3】タイムアウト付きでリクエストを実行
    // client.Do(req)は以下の段階でタイムアウトを監視：
    // 1. DNS解決
    // 2. TCP接続確立
    // 3. HTTPリクエスト送信
    // 4. HTTPレスポンス受信
    // 5. レスポンスボディの読み取り開始
    resp, err := thc.client.Do(req)
    if err != nil {
        // 【エラー分類】タイムアウトエラーを特別に処理
        // Context.Err()でタイムアウトの種類を判定：
        // - context.DeadlineExceeded: WithTimeout/WithDeadlineでのタイムアウト
        // - context.Canceled: cancel()関数による明示的キャンセル
        if ctx.Err() == context.DeadlineExceeded {
            return nil, fmt.Errorf("request timed out after %v: %w", thc.timeout, err)
        }
        return nil, fmt.Errorf("request failed: %w", err)
    }
    
    // 【Step 4】成功時のレスポンス返却
    // 注意：レスポンスボディの読み取りも呼び出し側でタイムアウト考慮が必要
    // defer resp.Body.Close()は呼び出し側の責任
    return resp, nil
}

// 【発展機能】複数段階のタイムアウト制御
func (thc *TimeoutHTTPClient) GetWithStages(url string, connectTimeout, responseTimeout time.Duration) (*http.Response, error) {
    // 【全体タイムアウト】リクエスト全体の制限時間
    totalTimeout := connectTimeout + responseTimeout
    ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
    defer cancel()
    
    // 【接続段階】DNS解決+TCP接続の制限時間
    connectCtx, connectCancel := context.WithTimeout(ctx, connectTimeout)
    defer connectCancel()
    
    req, err := http.NewRequestWithContext(connectCtx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    // 【レスポンス段階】HTTPレスポンス受信の制限時間
    // 接続が完了した後、レスポンス受信に別のタイムアウトを適用
    resp, err := thc.client.Do(req)
    if err != nil {
        if connectCtx.Err() == context.DeadlineExceeded {
            return nil, fmt.Errorf("connection timed out after %v: %w", connectTimeout, err)
        }
        return nil, fmt.Errorf("request failed: %w", err)
    }
    
    return resp, nil
}
```

#### 2. WithDeadlineの使用法

```go
// 絶対時刻でのデッドライン設定
func processWithDeadline(deadline time.Time, work func(context.Context) error) error {
    ctx, cancel := context.WithDeadline(context.Background(), deadline)
    defer cancel()
    
    // 作業を実行
    err := work(ctx)
    
    if ctx.Err() == context.DeadlineExceeded {
        return fmt.Errorf("deadline exceeded at %v", deadline)
    }
    
    return err
}

// 使用例：営業時間内での処理制限
func processBusinessHours(work func(context.Context) error) error {
    now := time.Now()
    
    // 営業時間終了（17:00）をデッドラインに設定
    endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, now.Location())
    
    if now.After(endOfDay) {
        return fmt.Errorf("processing not allowed after business hours")
    }
    
    return processWithDeadline(endOfDay, work)
}
```

### 実践的なタイムアウトパターン

#### 1. 段階的タイムアウト

```go
type ServiceClient struct {
    httpClient    *http.Client
    dbClient      *sql.DB
    shortTimeout  time.Duration  // 高速操作用（1-2秒）
    mediumTimeout time.Duration  // 中程度操作用（5-10秒）
    longTimeout   time.Duration  // 重い操作用（30-60秒）
}

func NewServiceClient() *ServiceClient {
    return &ServiceClient{
        httpClient:    &http.Client{},
        shortTimeout:  2 * time.Second,
        mediumTimeout: 10 * time.Second,
        longTimeout:   60 * time.Second,
    }
}

func (sc *ServiceClient) QuickHealthCheck() error {
    ctx, cancel := context.WithTimeout(context.Background(), sc.shortTimeout)
    defer cancel()
    
    return sc.healthCheck(ctx)
}

func (sc *ServiceClient) ProcessUserData(userID string) (*UserData, error) {
    ctx, cancel := context.WithTimeout(context.Background(), sc.mediumTimeout)
    defer cancel()
    
    return sc.fetchAndProcessUserData(ctx, userID)
}

func (sc *ServiceClient) GenerateReport() (*Report, error) {
    ctx, cancel := context.WithTimeout(context.Background(), sc.longTimeout)
    defer cancel()
    
    return sc.generateComplexReport(ctx)
}
```

#### 2. Contextチェーンとタイムアウト継承

```go
// 親操作から子操作への時間制約継承
func processUserRequest(userID string, overallTimeout time.Duration) error {
    // 全体のタイムアウトを設定
    ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
    defer cancel()
    
    // Step 1: ユーザー認証（全体時間の20%を割り当て）
    authCtx, authCancel := context.WithTimeout(ctx, overallTimeout/5)
    defer authCancel()
    
    if err := authenticateUser(authCtx, userID); err != nil {
        return fmt.Errorf("authentication failed: %w", err)
    }
    
    // Step 2: データ取得（全体時間の50%を割り当て）
    dataCtx, dataCancel := context.WithTimeout(ctx, overallTimeout/2)
    defer dataCancel()
    
    data, err := fetchUserData(dataCtx, userID)
    if err != nil {
        return fmt.Errorf("data fetch failed: %w", err)
    }
    
    // Step 3: データ処理（残り時間を使用）
    return processData(ctx, data)
}
```

### 指数バックオフによるリトライ

#### 1. 基本的なリトライ実装

```go
type RetryConfig struct {
    MaxAttempts   int
    BaseDelay     time.Duration
    MaxDelay      time.Duration
    Multiplier    float64
    Jitter        bool
}

type RetryableError struct {
    Err       error
    Retryable bool
}

func (re RetryableError) Error() string {
    return re.Err.Error()
}

func (re RetryableError) Unwrap() error {
    return re.Err
}

func NewDefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   100 * time.Millisecond,
        MaxDelay:    30 * time.Second,
        Multiplier:  2.0,
        Jitter:      true,
    }
}

func RetryWithBackoff(ctx context.Context, config RetryConfig, operation func() error) error {
    var lastErr error
    
    for attempt := 0; attempt < config.MaxAttempts; attempt++ {
        // 最初の試行はすぐに実行
        if attempt > 0 {
            delay := calculateDelay(config, attempt-1)
            
            select {
            case <-time.After(delay):
                // 遅延完了
            case <-ctx.Done():
                return fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
            }
        }
        
        // 操作を実行
        err := operation()
        if err == nil {
            return nil // 成功
        }
        
        lastErr = err
        
        // リトライ可能かチェック
        var retryableErr RetryableError
        if errors.As(err, &retryableErr) && !retryableErr.Retryable {
            return fmt.Errorf("non-retryable error: %w", err)
        }
        
        // Contextがキャンセルされた場合は即座に終了
        if ctx.Err() != nil {
            return fmt.Errorf("context cancelled: %w", ctx.Err())
        }
        
        log.Printf("Attempt %d failed: %v, retrying...", attempt+1, err)
    }
    
    return fmt.Errorf("all %d attempts failed, last error: %w", config.MaxAttempts, lastErr)
}

func calculateDelay(config RetryConfig, attempt int) time.Duration {
    delay := time.Duration(float64(config.BaseDelay) * math.Pow(config.Multiplier, float64(attempt)))
    
    // 最大遅延時間でキャップ
    if delay > config.MaxDelay {
        delay = config.MaxDelay
    }
    
    // ジッターを追加（サンダリングハード問題を回避）
    if config.Jitter {
        jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
        delay += jitter
    }
    
    return delay
}
```

#### 2. タイムアウト付きリトライの実用例

```go
type APIClient struct {
    client      *http.Client
    baseURL     string
    retryConfig RetryConfig
}

func NewAPIClient(baseURL string) *APIClient {
    return &APIClient{
        client: &http.Client{
            Timeout: 5 * time.Second, // 個別リクエストのタイムアウト
        },
        baseURL:     baseURL,
        retryConfig: NewDefaultRetryConfig(),
    }
}

func (ac *APIClient) GetUserData(ctx context.Context, userID string) (*UserData, error) {
    var userData *UserData
    
    operation := func() error {
        // 個別のタイムアウトを設定
        requestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
        defer cancel()
        
        url := fmt.Sprintf("%s/users/%s", ac.baseURL, userID)
        req, err := http.NewRequestWithContext(requestCtx, "GET", url, nil)
        if err != nil {
            return RetryableError{Err: err, Retryable: false}
        }
        
        resp, err := ac.client.Do(req)
        if err != nil {
            // ネットワークエラーはリトライ可能
            if isNetworkError(err) || requestCtx.Err() == context.DeadlineExceeded {
                return RetryableError{Err: err, Retryable: true}
            }
            return RetryableError{Err: err, Retryable: false}
        }
        defer resp.Body.Close()
        
        // HTTPステータスコードに基づくリトライ判定
        if resp.StatusCode >= 500 {
            // サーバーエラーはリトライ可能
            return RetryableError{
                Err:       fmt.Errorf("server error: status %d", resp.StatusCode),
                Retryable: true,
            }
        } else if resp.StatusCode >= 400 {
            // クライアントエラーはリトライ不可
            return RetryableError{
                Err:       fmt.Errorf("client error: status %d", resp.StatusCode),
                Retryable: false,
            }
        }
        
        // レスポンスのパース
        err = json.NewDecoder(resp.Body).Decode(&userData)
        if err != nil {
            return RetryableError{Err: err, Retryable: false}
        }
        
        return nil
    }
    
    err := RetryWithBackoff(ctx, ac.retryConfig, operation)
    if err != nil {
        return nil, err
    }
    
    return userData, nil
}

func isNetworkError(err error) bool {
    var netErr net.Error
    return errors.As(err, &netErr)
}
```

### タイムアウト監視とメトリクス

#### 1. タイムアウト統計の収集

```go
type TimeoutMetrics struct {
    totalRequests    int64
    timeoutCount     int64
    successCount     int64
    averageDuration  time.Duration
    maxDuration      time.Duration
    mu               sync.RWMutex
}

func NewTimeoutMetrics() *TimeoutMetrics {
    return &TimeoutMetrics{}
}

func (tm *TimeoutMetrics) RecordRequest(duration time.Duration, timedOut bool) {
    tm.mu.Lock()
    defer tm.mu.Unlock()
    
    tm.totalRequests++
    
    if timedOut {
        tm.timeoutCount++
    } else {
        tm.successCount++
    }
    
    // 平均時間の更新
    if tm.totalRequests == 1 {
        tm.averageDuration = duration
    } else {
        // 移動平均の計算
        tm.averageDuration = time.Duration(
            (int64(tm.averageDuration)*tm.totalRequests + int64(duration)) / (tm.totalRequests + 1),
        )
    }
    
    // 最大時間の更新
    if duration > tm.maxDuration {
        tm.maxDuration = duration
    }
}

func (tm *TimeoutMetrics) GetStats() (total, timeouts, successes int64, avgDuration, maxDuration time.Duration) {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    
    return tm.totalRequests, tm.timeoutCount, tm.successCount, tm.averageDuration, tm.maxDuration
}

func (tm *TimeoutMetrics) TimeoutRate() float64 {
    tm.mu.RLock()
    defer tm.mu.RUnlock()
    
    if tm.totalRequests == 0 {
        return 0
    }
    
    return float64(tm.timeoutCount) / float64(tm.totalRequests)
}
```

#### 2. 監視付きタイムアウト実行

```go
type MonitoredExecutor struct {
    metrics         *TimeoutMetrics
    alertThreshold  float64 // タイムアウト率の警告閾値
    alertCallback   func(rate float64)
}

func NewMonitoredExecutor(alertThreshold float64, alertCallback func(float64)) *MonitoredExecutor {
    return &MonitoredExecutor{
        metrics:        NewTimeoutMetrics(),
        alertThreshold: alertThreshold,
        alertCallback:  alertCallback,
    }
}

func (me *MonitoredExecutor) Execute(ctx context.Context, timeout time.Duration, operation func(context.Context) error) error {
    start := time.Now()
    
    // タイムアウト付きContextを作成
    timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    
    // 操作を実行
    err := operation(timeoutCtx)
    duration := time.Since(start)
    
    // タイムアウトかどうかを判定
    timedOut := timeoutCtx.Err() == context.DeadlineExceeded
    
    // メトリクスを記録
    me.metrics.RecordRequest(duration, timedOut)
    
    // アラート閾値チェック
    if me.alertCallback != nil {
        timeoutRate := me.metrics.TimeoutRate()
        if timeoutRate > me.alertThreshold {
            me.alertCallback(timeoutRate)
        }
    }
    
    return err
}
```

### デッドラインとリソース管理

#### 1. データベース接続のタイムアウト管理

```go
type DatabaseManager struct {
    db             *sql.DB
    queryTimeout   time.Duration
    connectTimeout time.Duration
}

func NewDatabaseManager(dsn string) (*DatabaseManager, error) {
    // 接続タイムアウトを設定
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }
    
    // 接続の確認
    if err := db.PingContext(ctx); err != nil {
        return nil, fmt.Errorf("database connection failed: %w", err)
    }
    
    return &DatabaseManager{
        db:             db,
        queryTimeout:   5 * time.Second,
        connectTimeout: 10 * time.Second,
    }, nil
}

func (dm *DatabaseManager) QueryWithTimeout(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
    // クエリタイムアウトを設定
    queryCtx, cancel := context.WithTimeout(ctx, dm.queryTimeout)
    defer cancel()
    
    rows, err := dm.db.QueryContext(queryCtx, query, args...)
    if err != nil {
        if queryCtx.Err() == context.DeadlineExceeded {
            return nil, fmt.Errorf("query timed out after %v: %w", dm.queryTimeout, err)
        }
        return nil, err
    }
    
    return rows, nil
}

func (dm *DatabaseManager) TransactionWithTimeout(ctx context.Context, timeout time.Duration, operations func(*sql.Tx) error) error {
    // トランザクションタイムアウトを設定
    txCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    
    tx, err := dm.db.BeginTx(txCtx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p) // re-throw panic after rollback
        } else if txCtx.Err() == context.DeadlineExceeded {
            tx.Rollback()
        }
    }()
    
    // 操作を実行
    if err := operations(tx); err != nil {
        tx.Rollback()
        return err
    }
    
    // タイムアウトチェック
    if txCtx.Err() == context.DeadlineExceeded {
        tx.Rollback()
        return fmt.Errorf("transaction timed out after %v", timeout)
    }
    
    return tx.Commit()
}
```

## 📝 課題 (The Problem)

以下の機能を持つ包括的なタイムアウト制御システムを実装してください：

### 1. 基本タイムアウト機能
- WithTimeoutを使用したタイムアウト制御
- WithDeadlineを使用した絶対時刻制御
- 適切なエラーハンドリング
- リソースリークの防止

### 2. リトライ機能
- 指数バックオフによるリトライ
- リトライ可能エラーの判定
- Contextキャンセルの考慮
- ジッター追加によるサンダリングハード問題回避

### 3. 監視機能
- タイムアウト統計の収集
- パフォーマンスメトリクス
- アラート機能
- ログ記録

### 実装すべき関数

```go
// APICallWithTimeout は外部APIを呼び出し、タイムアウトを設定する
func APICallWithTimeout(ctx context.Context, url string, timeout time.Duration) (*APIResponse, error)

// APICallWithDeadline は絶対時刻でのデッドラインを設定してAPIを呼び出す
func APICallWithDeadline(ctx context.Context, url string, deadline time.Time) (*APIResponse, error)

// APICallWithRetry はタイムアウト付きでリトライ機能を持つAPI呼び出し
func APICallWithRetry(ctx context.Context, url string, timeout time.Duration, maxRetries int) (*APIResponse, error)

// MonitoredOperation は監視付きでタイムアウト制御された操作を実行
func MonitoredOperation(ctx context.Context, timeout time.Duration, operation func(context.Context) error) error
```

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestTimeoutBasic
    main_test.go:45: Basic timeout working correctly
--- PASS: TestTimeoutBasic (0.01s)

=== RUN   TestDeadlineHandling
    main_test.go:65: Deadline handling working correctly
--- PASS: TestDeadlineHandling (0.02s)

=== RUN   TestRetryWithBackoff
    main_test.go:85: Retry with backoff functioning
--- PASS: TestRetryWithBackoff (0.03s)

=== RUN   TestTimeoutMetrics
    main_test.go:105: Timeout metrics collection working
--- PASS: TestTimeoutMetrics (0.04s)

PASS
ok      day02-context-timeout   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 基本的なタイムアウト実装

```go
func executeWithTimeout(ctx context.Context, timeout time.Duration, operation func() error) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    
    done := make(chan error, 1)
    go func() {
        done <- operation()
    }()
    
    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### エラーの種別判定

```go
func isRetryableError(err error) bool {
    if err == nil {
        return false
    }
    
    // タイムアウトエラーはリトライ可能
    if errors.Is(err, context.DeadlineExceeded) {
        return true
    }
    
    // ネットワークエラーはリトライ可能
    var netErr net.Error
    if errors.As(err, &netErr) && netErr.Timeout() {
        return true
    }
    
    return false
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **適応的タイムアウト**: 過去の実行時間に基づく動的調整
2. **回路遮断器統合**: タイムアウト率に基づくサーキットブレーカー
3. **分散タイムアウト**: 複数サービス間での統一タイムアウト制御
4. **機械学習予測**: 処理時間の学習による最適タイムアウト設定
5. **リアルタイム監視**: ダッシュボードによるタイムアウト状況可視化

Contextによるタイムアウト/デッドライン制御の実装を通じて、信頼性の高いGoアプリケーション構築の重要な技術を習得しましょう！