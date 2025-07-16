# Day 13: Circuit Breakerパターン

## 🎯 本日の目標 (Today's Goal)

外部サービスの障害が自身のシステムに波及するのを防ぐCircuit Breakerパターンを実装し、システムの耐障害性を向上させるテクニックを学習します。

## 📖 解説 (Explanation)

### Circuit Breakerパターンとは

Circuit Breakerパターンは、外部サービスの呼び出しを監視し、失敗が一定の閾値を超えた場合に自動的に回路を「開く」ことで、システム全体の障害を防ぐデザインパターンです。

### なぜ必要なのか？

1. **障害の連鎖を防ぐ**: 外部サービスの障害が自分のシステムに波及することを防ぎます
2. **リソースの保護**: 失敗することが分かっている呼び出しを避け、スレッドプールやメモリを無駄に消費しません
3. **高速な失敗**: 外部サービスがダウンしている間は即座に失敗させ、応答時間を短縮します
4. **自動回復**: サービスが復旧した際の自動検知と回復機能を提供します

### Circuit Breakerの3つの状態

#### 1. Closed状態（通常状態）
- すべてのリクエストを外部サービスに転送
- 成功・失敗をカウント
- 失敗率が閾値を超えるとOpen状態に移行

#### 2. Open状態（回路開放状態）
- すべてのリクエストを即座に失敗させる
- 外部サービスへの呼び出しは行わない
- 一定時間経過後にHalf-Open状態に移行

#### 3. Half-Open状態（回復試行状態）
- 限定数のリクエストのみ外部サービスに転送
- 成功すればClosed状態に戻る
- 失敗すればOpen状態に戻る

### 実装における考慮点

#### スレッドセーフ性
複数のgoroutineから同時にアクセスされるため、内部状態の管理には適切な同期処理が必要です。

#### 統計の管理
失敗率の計算には滑動窓（sliding window）を使用し、直近の一定期間での成功・失敗を管理します。

#### タイムアウト設定
各状態でのタイムアウト設定を適切に管理する必要があります：
- リクエストタイムアウト
- Open状態での待機時間
- Half-Open状態での試行時間

## 📝 課題 (The Problem)

以下の構造体とメソッドを実装して、Circuit Breakerパターンを完成させてください：

```go
// 【状態定義】Circuit Breakerの3つの基本状態
type CircuitBreakerState int

const (
    StateClosed   CircuitBreakerState = iota  // 【通常状態】全リクエストを転送
    StateOpen                                 // 【開放状態】全リクエストを即座に拒否
    StateHalfOpen                            // 【半開状態】限定的にリクエスト転送を試行
)

// 【核心実装】Circuit Breaker パターンの完全実装
type CircuitBreaker struct {
    // 【設定パラメータ】システム要件に応じて調整
    maxFailures      int           // 【閾値】Open状態への移行判定（例：5回連続失敗）
    resetTimeout     time.Duration // 【回復時間】Open→HalfOpen移行までの待機時間（例：30秒）
    halfOpenMaxCalls int           // 【試行回数】HalfOpen状態での最大リクエスト数（例：3回）
    
    // 【実行時状態】排他制御下で管理される動的状態
    state         CircuitBreakerState  // 【現在状態】Closed/Open/HalfOpenの状態管理
    failures      int                  // 【失敗カウント】連続失敗回数の追跡
    lastFailTime  time.Time            // 【最終失敗時刻】Open状態からの回復判定用
    halfOpenCalls int                  // 【試行カウント】HalfOpen状態での実行済み回数
    
    // 【排他制御】複数Goroutineからの同時アクセス対応
    mutex sync.RWMutex  // 読み書き分離ロックで高パフォーマンスを実現
}

// 【設定構造体】Circuit Breaker初期化用パラメータ
type Settings struct {
    MaxFailures      int           // 【重要】適切な値設定が成功の鍵
    ResetTimeout     time.Duration // 外部サービスの回復時間を考慮
    HalfOpenMaxCalls int           // 少数で開始し、段階的に増加
}

// 【実装指針】：
// 1. maxFailures: 外部サービスの信頼性レベルに応じて設定
//    - 高信頼性サービス: 3-5回
//    - 通常サービス: 5-10回
//    - 不安定サービス: 10-20回
//
// 2. resetTimeout: サービス復旧時間とバランス
//    - 高速復旧: 10-30秒
//    - 通常復旧: 1-5分
//    - 重い処理: 5-10分
//
// 3. halfOpenMaxCalls: 段階的回復のための慎重な設定
//    - 保守的: 1-3回
//    - 標準的: 3-5回
//    - アグレッシブ: 5-10回
```

### 実装すべきメソッド

1. **NewCircuitBreaker**: 新しいCircuit Breakerを作成
2. **Call**: 外部サービス呼び出しをラップ
3. **GetState**: 現在の状態を取得
4. **GetCounts**: 統計情報を取得

```go
// 【コンストラクタ】Circuit Breaker インスタンスの初期化
func NewCircuitBreaker(settings Settings) *CircuitBreaker {
    // 【パラメータ検証】不正な設定値からシステムを保護
    if settings.MaxFailures <= 0 {
        settings.MaxFailures = 5  // デフォルト値で安全な動作を保証
    }
    if settings.ResetTimeout <= 0 {
        settings.ResetTimeout = 60 * time.Second  // 適切な回復時間を設定
    }
    if settings.HalfOpenMaxCalls <= 0 {
        settings.HalfOpenMaxCalls = 1  // 保守的な試行回数
    }
    
    return &CircuitBreaker{
        // 【設定値の格納】
        maxFailures:      settings.MaxFailures,
        resetTimeout:     settings.ResetTimeout,
        halfOpenMaxCalls: settings.HalfOpenMaxCalls,
        
        // 【初期状態】Closed状態で開始（正常動作モード）
        state:         StateClosed,
        failures:      0,
        lastFailTime:  time.Time{}, // ゼロ値で初期化
        halfOpenCalls: 0,
        
        // 【排他制御】並行アクセスに対応
        mutex: sync.RWMutex{},
    }
}

// 【核心機能】外部サービス呼び出しの実行と監視
func (cb *CircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
    // 【STEP 1】実行可否の判定（読み取りロック）
    cb.mutex.RLock()
    state := cb.state
    failures := cb.failures
    lastFailTime := cb.lastFailTime
    halfOpenCalls := cb.halfOpenCalls
    cb.mutex.RUnlock()
    
    // 【状態別の処理分岐】
    switch state {
    case StateOpen:
        // 【Open状態】回路が開いている場合の処理
        if time.Since(lastFailTime) >= cb.resetTimeout {
            // 【状態遷移】十分な時間が経過 → HalfOpen状態へ
            cb.mutex.Lock()
            if cb.state == StateOpen { // ダブルチェック
                cb.state = StateHalfOpen
                cb.halfOpenCalls = 0
                log.Printf("Circuit breaker transitioning to HALF_OPEN")
            }
            cb.mutex.Unlock()
        } else {
            // 【即座に失敗】外部サービスを呼び出さずエラーを返す
            return nil, &CircuitBreakerError{
                Message: "circuit breaker is OPEN",
                State:   StateOpen,
            }
        }
        
    case StateHalfOpen:
        // 【HalfOpen状態】試行回数制限のチェック
        if halfOpenCalls >= cb.halfOpenMaxCalls {
            // 【制限超過】これ以上の試行を拒否
            return nil, &CircuitBreakerError{
                Message: "circuit breaker HALF_OPEN max calls exceeded",
                State:   StateHalfOpen,
            }
        }
        
        // 【試行回数を増加】
        cb.mutex.Lock()
        cb.halfOpenCalls++
        cb.mutex.Unlock()
        
    case StateClosed:
        // 【Closed状態】通常動作、そのまま実行
        break
    }
    
    // 【STEP 2】実際の外部サービス呼び出し
    result, err := fn()
    
    // 【STEP 3】結果に応じた状態更新
    cb.mutex.Lock()
    defer cb.mutex.Unlock()
    
    if err != nil {
        // 【失敗時の処理】
        cb.onFailure()
    } else {
        // 【成功時の処理】
        cb.onSuccess()
    }
    
    return result, err
}

// 【内部メソッド】失敗時の状態更新（要ロック）
func (cb *CircuitBreaker) onFailure() {
    cb.failures++
    cb.lastFailTime = time.Now()
    
    switch cb.state {
    case StateClosed:
        // 【Closed → Open】失敗回数が閾値に達した場合
        if cb.failures >= cb.maxFailures {
            cb.state = StateOpen
            log.Printf("Circuit breaker OPENED after %d failures", cb.failures)
        }
        
    case StateHalfOpen:
        // 【HalfOpen → Open】試行失敗により即座にOpen状態に戻る
        cb.state = StateOpen
        cb.halfOpenCalls = 0
        log.Printf("Circuit breaker returning to OPEN from HALF_OPEN")
    }
}

// 【内部メソッド】成功時の状態更新（要ロック）
func (cb *CircuitBreaker) onSuccess() {
    switch cb.state {
    case StateClosed:
        // 【成功時リセット】失敗カウントをリセット
        cb.failures = 0
        
    case StateHalfOpen:
        // 【HalfOpen → Closed】試行成功により正常状態に復帰
        cb.state = StateClosed
        cb.failures = 0
        cb.halfOpenCalls = 0
        log.Printf("Circuit breaker CLOSED - service recovered")
    }
}

// 【状態取得】現在のCircuit Breaker状態を安全に取得
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
    cb.mutex.RLock()
    defer cb.mutex.RUnlock()
    
    // 【自動状態遷移】Open状態で十分な時間が経過した場合
    if cb.state == StateOpen && time.Since(cb.lastFailTime) >= cb.resetTimeout {
        // 【注意】読み取り専用メソッドなので状態は変更しない
        // 実際の状態変更はCall()メソッドで行う
    }
    
    return cb.state
}

// 【統計情報】Circuit Breakerの動作統計を取得
func (cb *CircuitBreaker) GetCounts() CircuitBreakerCounts {
    cb.mutex.RLock()
    defer cb.mutex.RUnlock()
    
    return CircuitBreakerCounts{
        TotalCalls:    cb.getTotalCalls(),     // 総呼び出し回数
        SuccessCalls:  cb.getSuccessCalls(),   // 成功回数
        FailureCalls:  cb.failures,            // 失敗回数
        ConsecutiveFailures: cb.failures,      // 連続失敗回数
        State:         cb.state,               // 現在状態
        LastFailTime:  cb.lastFailTime,        // 最終失敗時刻
    }
}

// 【専用エラー型】Circuit Breaker特有のエラー情報
type CircuitBreakerError struct {
    Message string
    State   CircuitBreakerState
}

func (e *CircuitBreakerError) Error() string {
    return fmt.Sprintf("circuit breaker error: %s (state: %v)", e.Message, e.State)
}

// 【統計構造体】Circuit Breakerの動作状況
type CircuitBreakerCounts struct {
    TotalCalls          int
    SuccessCalls        int
    FailureCalls        int
    ConsecutiveFailures int
    State               CircuitBreakerState
    LastFailTime        time.Time
}
```

## ✅ 期待される挙動 (Expected Behavior)

正しく実装されたCircuit Breakerは以下のように動作します：

### Closed状態での動作
```
Request -> [CB: Closed] -> External Service -> Success/Failure
失敗回数が閾値未満: そのまま処理継続
失敗回数が閾値到達: Open状態に移行
```

### Open状態での動作
```
Request -> [CB: Open] -> Immediate Failure (フォールバック実行)
一定時間経過: Half-Open状態に移行
```

### Half-Open状態での動作
```
Request -> [CB: Half-Open] -> External Service (限定的)
成功: Closed状態に復帰
失敗: Open状態に戻る
```

## 💡 ヒント (Hints)

1. **状態管理**: `sync.RWMutex`を使用してスレッドセーフな状態管理を行いましょう
2. **時間管理**: `time.Since()`を使用してタイムアウトを判定しましょう
3. **統計管理**: 成功・失敗のカウンターを適切に更新しましょう
4. **エラーハンドリング**: Circuit Brekerが開いている場合の専用エラーを定義しましょう
5. **テスタビリティ**: 時間に依存する処理は外部から制御可能にしましょう

## 参考実装例

### 基本的な使用例

```go
// 【基本設定】小規模サービス向けの設定例
cb := NewCircuitBreaker(Settings{
    MaxFailures:      3,                // 3回連続失敗でOpen状態に移行
    ResetTimeout:     5 * time.Second,  // 5秒後にHalfOpen状態で回復試行
    HalfOpenMaxCalls: 2,                // HalfOpen状態で最大2回試行
})

// 【外部サービス呼び出しのラップ】
result, err := cb.Call(func() (interface{}, error) {
    return externalServiceCall()
})
```

### 実用的な実装例

```go
// 【HTTPクライアント用Circuit Breaker】プロダクション対応
type HTTPServiceClient struct {
    client         *http.Client
    circuitBreaker *CircuitBreaker
    baseURL        string
}

func NewHTTPServiceClient(baseURL string) *HTTPServiceClient {
    return &HTTPServiceClient{
        client: &http.Client{
            Timeout: 10 * time.Second,
        },
        circuitBreaker: NewCircuitBreaker(Settings{
            MaxFailures:      5,                // 信頼性の高いサービス想定
            ResetTimeout:     30 * time.Second, // 十分な回復時間
            HalfOpenMaxCalls: 3,                // 段階的回復
        }),
        baseURL: baseURL,
    }
}

func (c *HTTPServiceClient) GetUser(userID int) (*User, error) {
    // 【Circuit Breaker経由での安全な呼び出し】
    result, err := c.circuitBreaker.Call(func() (interface{}, error) {
        return c.fetchUser(userID)
    })
    
    if err != nil {
        // 【エラーハンドリング】Circuit Breaker特有のエラーも考慮
        if cbErr, ok := err.(*CircuitBreakerError); ok {
            log.Printf("Circuit breaker error: %s", cbErr.Message)
            // フォールバック処理やキャッシュからの取得
            return c.getUserFromCache(userID)
        }
        return nil, err
    }
    
    user, ok := result.(*User)
    if !ok {
        return nil, fmt.Errorf("unexpected response type")
    }
    
    return user, nil
}

func (c *HTTPServiceClient) fetchUser(userID int) (*User, error) {
    url := fmt.Sprintf("%s/users/%d", c.baseURL, userID)
    resp, err := c.client.Get(url)
    if err != nil {
        return nil, fmt.Errorf("HTTP request failed: %w", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 500 {
        // 【5xx系エラー】Circuit Breakerによる保護対象
        return nil, fmt.Errorf("server error: %d", resp.StatusCode)
    }
    
    if resp.StatusCode == 404 {
        // 【4xx系エラー】Circuit Breakerを動作させない業務エラー
        return nil, fmt.Errorf("user not found")
    }
    
    var user User
    if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
        return nil, fmt.Errorf("JSON decode failed: %w", err)
    }
    
    return &user, nil
}
```

### データベース接続での使用例

```go
// 【データベース用Circuit Breaker】高可用性対応
type DatabaseService struct {
    db             *sql.DB
    circuitBreaker *CircuitBreaker
    cache          *Cache // フォールバック用キャッシュ
}

func NewDatabaseService(db *sql.DB) *DatabaseService {
    return &DatabaseService{
        db: db,
        circuitBreaker: NewCircuitBreaker(Settings{
            MaxFailures:      10,               // DB接続は比較的安定想定
            ResetTimeout:     60 * time.Second, // DB復旧には時間がかかる場合が多い
            HalfOpenMaxCalls: 1,                // 慎重な回復判定
        }),
        cache: NewCache(),
    }
}

func (ds *DatabaseService) GetOrder(orderID int) (*Order, error) {
    // 【Circuit Breaker経由でのDB操作】
    result, err := ds.circuitBreaker.Call(func() (interface{}, error) {
        return ds.queryOrder(orderID)
    })
    
    if err != nil {
        // 【フォールバック】キャッシュまたは読み取り専用レプリカから取得
        if cbErr, ok := err.(*CircuitBreakerError); ok {
            log.Printf("Database circuit breaker activated: %s", cbErr.Message)
            return ds.getOrderFromCache(orderID)
        }
        return nil, err
    }
    
    order := result.(*Order)
    
    // 【成功時】キャッシュに保存
    ds.cache.Set(fmt.Sprintf("order:%d", orderID), order, 5*time.Minute)
    
    return order, nil
}
```

### 複数Circuit Breakerの組み合わせ

```go
// 【マイクロサービス間通信】複数のCircuit Breaker管理
type ServiceRegistry struct {
    userService    *CircuitBreaker
    orderService   *CircuitBreaker
    paymentService *CircuitBreaker
}

func NewServiceRegistry() *ServiceRegistry {
    return &ServiceRegistry{
        // 【ユーザーサービス】高頻度・低重要度
        userService: NewCircuitBreaker(Settings{
            MaxFailures:      3,
            ResetTimeout:     10 * time.Second,
            HalfOpenMaxCalls: 2,
        }),
        
        // 【注文サービス】中頻度・中重要度
        orderService: NewCircuitBreaker(Settings{
            MaxFailures:      5,
            ResetTimeout:     30 * time.Second,
            HalfOpenMaxCalls: 3,
        }),
        
        // 【決済サービス】低頻度・高重要度
        paymentService: NewCircuitBreaker(Settings{
            MaxFailures:      8,
            ResetTimeout:     60 * time.Second,
            HalfOpenMaxCalls: 1,
        }),
    }
}

func (sr *ServiceRegistry) ProcessOrder(order *Order) error {
    // 【STEP 1】ユーザー情報の取得（失敗してもキャッシュでフォールバック可能）
    _, err := sr.userService.Call(func() (interface{}, error) {
        return getUserInfo(order.UserID)
    })
    if err != nil {
        log.Printf("User service unavailable, using cached data: %v", err)
    }
    
    // 【STEP 2】注文の作成（必須処理）
    _, err = sr.orderService.Call(func() (interface{}, error) {
        return createOrder(order)
    })
    if err != nil {
        return fmt.Errorf("order creation failed: %w", err)
    }
    
    // 【STEP 3】決済処理（最も重要）
    _, err = sr.paymentService.Call(func() (interface{}, error) {
        return processPayment(order.PaymentInfo)
    })
    if err != nil {
        // 【補償処理】注文をキャンセル
        cancelOrder(order.ID)
        return fmt.Errorf("payment failed: %w", err)
    }
    
    return nil
}
```

### 監視とメトリクス

```go
// 【監視機能付きCircuit Breaker】運用監視対応
type MonitoredCircuitBreaker struct {
    *CircuitBreaker
    metrics *MetricsCollector
    name    string
}

func NewMonitoredCircuitBreaker(name string, settings Settings) *MonitoredCircuitBreaker {
    return &MonitoredCircuitBreaker{
        CircuitBreaker: NewCircuitBreaker(settings),
        metrics:        NewMetricsCollector(),
        name:          name,
    }
}

func (mcb *MonitoredCircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
    start := time.Now()
    
    result, err := mcb.CircuitBreaker.Call(fn)
    
    duration := time.Since(start)
    
    // 【メトリクス収集】
    mcb.metrics.RecordCall(mcb.name, duration, err)
    
    // 【状態変化の検出とアラート】
    if mcb.GetState() == StateOpen {
        mcb.metrics.RecordCircuitBreakerOpen(mcb.name)
        
        // 【アラート送信】運用チームへの通知
        sendAlert(fmt.Sprintf("Circuit breaker '%s' is OPEN", mcb.name))
    }
    
    return result, err
}

func (mcb *MonitoredCircuitBreaker) GetHealthStatus() map[string]interface{} {
    counts := mcb.GetCounts()
    
    return map[string]interface{}{
        "name":                mcb.name,
        "state":               counts.State.String(),
        "total_calls":         counts.TotalCalls,
        "success_rate":        float64(counts.SuccessCalls) / float64(counts.TotalCalls),
        "consecutive_failures": counts.ConsecutiveFailures,
        "last_failure":        counts.LastFailTime,
    }
}
```

## スコアカード

- ✅ 基本実装: 3つの状態が正しく動作する
- ✅ スレッドセーフ: 並行アクセスで正しく動作する
- ✅ 統計管理: 失敗率が正しく計算される
- ✅ 自動回復: タイムアウト後の自動回復が動作する
- ✅ フォールバック: Open状態で適切に失敗する

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```

## 参考資料

- [Circuit Breaker Pattern - Martin Fowler](https://martinfowler.com/bliki/CircuitBreaker.html)
- [Hystrix Documentation](https://github.com/Netflix/Hystrix/wiki)
- [Go Concurrency Patterns](https://blog.golang.org/concurrency-patterns)