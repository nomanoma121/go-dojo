# Day 10: Rate Limiter (Ticker版)

## 🎯 本日の目標 (Today's Goal)

このチャレンジを通して、以下のスキルを身につけることができます：

- **time.Tickerを使った一定間隔処理システムを実装できるようになる**
- **Token Bucketアルゴリズムによるレート制限を理解し実装できるようになる**
- **バースト処理とスループット制御のバランスを取れるようになる**
- **並行処理環境で安全なレートリミッターを構築できるようになる**

## 📖 解説 (Explanation)

### なぜRate Limiterが必要なのか？

システム開発では、リソースの過負荷を防ぐためにアクセス制限が必要な場面が多くあります：

```go
// 問題のある例：制限なしのAPI呼び出し
func callExternalAPI() {
    for i := 0; i < 10000; i++ {
        go func(id int) {
            // 10,000個のGoroutineが同時にAPI呼び出し
            resp, err := http.Get("https://api.example.com/data")
            // APIサーバーが過負荷で停止する可能性
        }(i)
    }
}
```

この方法の問題点：
1. **外部サービスの過負荷**: APIサーバーがダウンする可能性
2. **Rate Limit違反**: APIプロバイダーからのアクセス禁止
3. **システムリソースの浪費**: 無駄なネットワーク帯域とCPU使用
4. **レスポンス品質の劣化**: 全てのリクエストが遅くなる

### Rate Limiterの基本概念

Rate Limiterは、**単位時間あたりの処理数を制限**して、システムを保護する仕組みです：

```go
import (
    "sync"
    "time"
    "context"
)

// 基本的なRate Limiterの構造
type RateLimiter struct {
    rate       time.Duration // トークン補充間隔
    capacity   int           // バケットの容量
    tokens     int           // 現在のトークン数
    ticker     *time.Ticker  // 定期実行用
    mu         sync.Mutex    // 排他制御
    tokenChan  chan struct{} // トークン配布用チャネル
    done       chan struct{} // 停止用チャネル
}

func NewRateLimiter(requestsPerSecond int, burstCapacity int) *RateLimiter {
    interval := time.Second / time.Duration(requestsPerSecond)
    
    rl := &RateLimiter{
        rate:      interval,
        capacity:  burstCapacity,
        tokens:    burstCapacity, // 初期状態では満タン
        tokenChan: make(chan struct{}, burstCapacity),
        done:      make(chan struct{}),
    }
    
    // 初期トークンを配布
    for i := 0; i < burstCapacity; i++ {
        rl.tokenChan <- struct{}{}
    }
    
    return rl
}
```

**Rate Limiterの利点：**
- **システム保護**: 過負荷からの保護
- **品質保証**: 安定したレスポンス時間
- **リソース管理**: 効率的なリソース利用
- **外部制約への準拠**: API制限の遵守

### Token Bucketアルゴリズムの実装

最も一般的なRate Limiterアルゴリズム：

```go
func (rl *RateLimiter) Start() {
    rl.ticker = time.NewTicker(rl.rate)
    
    go func() {
        defer rl.ticker.Stop()
        
        for {
            select {
            case <-rl.ticker.C:
                rl.mu.Lock()
                // トークンを1つ追加（容量まで）
                if rl.tokens < rl.capacity {
                    rl.tokens++
                    // ノンブロッキングでチャネルに送信
                    select {
                    case rl.tokenChan <- struct{}{}:
                    default:
                        // チャネルが満杯の場合はスキップ
                    }
                }
                rl.mu.Unlock()
                
            case <-rl.done:
                return
            }
        }
    }()
}

func (rl *RateLimiter) Stop() {
    close(rl.done)
}

// ブロッキング取得
func (rl *RateLimiter) Allow() {
    <-rl.tokenChan
}

// ノンブロッキング取得
func (rl *RateLimiter) TryAllow() bool {
    select {
    case <-rl.tokenChan:
        return true
    default:
        return false
    }
}

// タイムアウト付き取得
func (rl *RateLimiter) AllowWithTimeout(timeout time.Duration) bool {
    select {
    case <-rl.tokenChan:
        return true
    case <-time.After(timeout):
        return false
    }
}
```

### より高度なRate Limiter実装

実用的な機能を追加したバージョン：

```go
type AdvancedRateLimiter struct {
    rate        time.Duration
    capacity    int
    tokens      int
    lastRefill  time.Time
    mu          sync.RWMutex
    stats       *LimiterStats
}

type LimiterStats struct {
    TotalRequests   int64
    AllowedRequests int64
    RejectedRequests int64
    AverageWaitTime time.Duration
    mu              sync.RWMutex
}

func NewAdvancedRateLimiter(requestsPerSecond int, burstCapacity int) *AdvancedRateLimiter {
    return &AdvancedRateLimiter{
        rate:       time.Second / time.Duration(requestsPerSecond),
        capacity:   burstCapacity,
        tokens:     burstCapacity,
        lastRefill: time.Now(),
        stats:      &LimiterStats{},
    }
}

func (arl *AdvancedRateLimiter) Allow() bool {
    start := time.Now()
    
    arl.mu.Lock()
    defer arl.mu.Unlock()
    
    // 経過時間に基づいてトークンを補充
    now := time.Now()
    elapsed := now.Sub(arl.lastRefill)
    tokensToAdd := int(elapsed / arl.rate)
    
    if tokensToAdd > 0 {
        arl.tokens += tokensToAdd
        if arl.tokens > arl.capacity {
            arl.tokens = arl.capacity
        }
        arl.lastRefill = now
    }
    
    // 統計情報を更新
    arl.stats.mu.Lock()
    arl.stats.TotalRequests++
    
    if arl.tokens > 0 {
        arl.tokens--
        arl.stats.AllowedRequests++
        
        // 待機時間を計算（この実装では瞬時）
        waitTime := time.Since(start)
        count := arl.stats.AllowedRequests
        arl.stats.AverageWaitTime = time.Duration(
            (int64(arl.stats.AverageWaitTime)*(count-1) + int64(waitTime)) / count,
        )
        
        arl.stats.mu.Unlock()
        return true
    } else {
        arl.stats.RejectedRequests++
        arl.stats.mu.Unlock()
        return false
    }
}

func (arl *AdvancedRateLimiter) GetStats() LimiterStats {
    arl.stats.mu.RLock()
    defer arl.stats.mu.RUnlock()
    
    return *arl.stats
}

func (arl *AdvancedRateLimiter) GetTokenCount() int {
    arl.mu.RLock()
    defer arl.mu.RUnlock()
    
    return arl.tokens
}
```

### 複数レート制限の階層化

異なる時間窓でのレート制限を組み合わせ：

```go
type HierarchicalRateLimiter struct {
    limiters map[time.Duration]*AdvancedRateLimiter
    mu       sync.RWMutex
}

func NewHierarchicalRateLimiter() *HierarchicalRateLimiter {
    return &HierarchicalRateLimiter{
        limiters: make(map[time.Duration]*AdvancedRateLimiter),
    }
}

func (hrl *HierarchicalRateLimiter) AddLimit(window time.Duration, requests int) {
    hrl.mu.Lock()
    defer hrl.mu.Unlock()
    
    // requests per window を requests per second に変換
    requestsPerSecond := int(float64(requests) / window.Seconds())
    if requestsPerSecond == 0 {
        requestsPerSecond = 1
    }
    
    hrl.limiters[window] = NewAdvancedRateLimiter(requestsPerSecond, requests)
}

func (hrl *HierarchicalRateLimiter) Allow() bool {
    hrl.mu.RLock()
    defer hrl.mu.RUnlock()
    
    // 全ての制限をチェック
    for _, limiter := range hrl.limiters {
        if !limiter.Allow() {
            return false
        }
    }
    
    return true
}

// 使用例
func setupHierarchicalLimiter() *HierarchicalRateLimiter {
    limiter := NewHierarchicalRateLimiter()
    
    // 1秒間に10リクエスト
    limiter.AddLimit(time.Second, 10)
    
    // 1分間に300リクエスト
    limiter.AddLimit(time.Minute, 300)
    
    // 1時間に10,000リクエスト
    limiter.AddLimit(time.Hour, 10000)
    
    return limiter
}
```

### 分散Rate Limiter（Redis使用）

複数インスタンス間でレート制限を共有：

```go
import (
    "github.com/go-redis/redis/v8"
    "strconv"
)

type DistributedRateLimiter struct {
    redis    *redis.Client
    key      string
    rate     int           // requests per second
    capacity int           // burst capacity
    window   time.Duration // sliding window
}

func NewDistributedRateLimiter(redisClient *redis.Client, key string, rate, capacity int) *DistributedRateLimiter {
    return &DistributedRateLimiter{
        redis:    redisClient,
        key:      key,
        rate:     rate,
        capacity: capacity,
        window:   time.Second,
    }
}

func (drl *DistributedRateLimiter) Allow(ctx context.Context) (bool, error) {
    // Lua スクリプトでatomicに実行
    script := `
    local key = KEYS[1]
    local capacity = tonumber(ARGV[1])
    local tokens = tonumber(ARGV[2])
    local interval = tonumber(ARGV[3])
    local now = tonumber(ARGV[4])
    
    local bucket = redis.call('hmget', key, 'tokens', 'last_refill')
    local current_tokens = tonumber(bucket[1]) or capacity
    local last_refill = tonumber(bucket[2]) or now
    
    -- トークンを補充
    local elapsed = now - last_refill
    local tokens_to_add = math.floor(elapsed / interval * tokens)
    current_tokens = math.min(capacity, current_tokens + tokens_to_add)
    
    if current_tokens >= 1 then
        current_tokens = current_tokens - 1
        redis.call('hmset', key, 'tokens', current_tokens, 'last_refill', now)
        redis.call('expire', key, 3600) -- 1時間でexpire
        return {1, current_tokens}
    else
        redis.call('hmset', key, 'tokens', current_tokens, 'last_refill', now)
        redis.call('expire', key, 3600)
        return {0, current_tokens}
    end
    `
    
    now := time.Now().UnixNano()
    interval := drl.window.Nanoseconds() / int64(drl.rate)
    
    result, err := drl.redis.Eval(ctx, script, []string{drl.key}, 
        drl.capacity, drl.rate, interval, now).Result()
    
    if err != nil {
        return false, err
    }
    
    resultSlice := result.([]interface{})
    allowed := resultSlice[0].(int64) == 1
    
    return allowed, nil
}
```

### リアルタイム適応型Rate Limiter

システム負荷に応じて動的に制限を調整：

```go
type AdaptiveRateLimiter struct {
    baseLimiter    *AdvancedRateLimiter
    currentRate    int
    minRate        int
    maxRate        int
    monitor        *SystemMonitor
    adjustInterval time.Duration
    mu             sync.RWMutex
}

type SystemMonitor struct {
    CPUThreshold    float64
    MemoryThreshold float64
    ErrorRateThreshold float64
}

func NewAdaptiveRateLimiter(baseRate, minRate, maxRate int, monitor *SystemMonitor) *AdaptiveRateLimiter {
    return &AdaptiveRateLimiter{
        baseLimiter:    NewAdvancedRateLimiter(baseRate, baseRate*2),
        currentRate:    baseRate,
        minRate:        minRate,
        maxRate:        maxRate,
        monitor:        monitor,
        adjustInterval: 30 * time.Second,
    }
}

func (arl *AdaptiveRateLimiter) Start(ctx context.Context) {
    ticker := time.NewTicker(arl.adjustInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            arl.adjustRate()
        case <-ctx.Done():
            return
        }
    }
}

func (arl *AdaptiveRateLimiter) adjustRate() {
    arl.mu.Lock()
    defer arl.mu.Unlock()
    
    // システムメトリクスを取得
    cpuUsage := getCurrentCPUUsage()
    memoryUsage := getCurrentMemoryUsage()
    errorRate := arl.calculateErrorRate()
    
    newRate := arl.currentRate
    
    // 負荷が高い場合はレートを下げる
    if cpuUsage > arl.monitor.CPUThreshold || 
       memoryUsage > arl.monitor.MemoryThreshold ||
       errorRate > arl.monitor.ErrorRateThreshold {
        newRate = int(float64(arl.currentRate) * 0.8)
        if newRate < arl.minRate {
            newRate = arl.minRate
        }
    } else {
        // 負荷が低い場合はレートを上げる
        newRate = int(float64(arl.currentRate) * 1.1)
        if newRate > arl.maxRate {
            newRate = arl.maxRate
        }
    }
    
    if newRate != arl.currentRate {
        arl.currentRate = newRate
        // 新しいレートでlimiterを再作成
        arl.baseLimiter = NewAdvancedRateLimiter(newRate, newRate*2)
    }
}

func (arl *AdaptiveRateLimiter) Allow() bool {
    arl.mu.RLock()
    defer arl.mu.RUnlock()
    
    return arl.baseLimiter.Allow()
}

func getCurrentCPUUsage() float64 {
    // CPU使用率を取得（実装は環境依存）
    return 0.5 // プレースホルダー
}

func getCurrentMemoryUsage() float64 {
    // メモリ使用率を取得（実装は環境依存）
    return 0.3 // プレースホルダー
}

func (arl *AdaptiveRateLimiter) calculateErrorRate() float64 {
    stats := arl.baseLimiter.GetStats()
    if stats.TotalRequests == 0 {
        return 0
    }
    return float64(stats.RejectedRequests) / float64(stats.TotalRequests)
}
```

### 実用的な使用例

HTTP APIサーバーでの使用例：

```go
type APIServer struct {
    limiter *AdvancedRateLimiter
    server  *http.Server
}

func NewAPIServer(port string, requestsPerSecond int) *APIServer {
    limiter := NewAdvancedRateLimiter(requestsPerSecond, requestsPerSecond*2)
    
    mux := http.NewServeMux()
    
    server := &APIServer{
        limiter: limiter,
        server: &http.Server{
            Addr:    ":" + port,
            Handler: mux,
        },
    }
    
    mux.Handle("/api/", server.rateLimitMiddleware(http.HandlerFunc(server.handleAPI)))
    mux.Handle("/health", http.HandlerFunc(server.handleHealth))
    
    return server
}

func (s *APIServer) rateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !s.limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}

func (s *APIServer) handleAPI(w http.ResponseWriter, r *http.Request) {
    // API処理
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status": "success"}`))
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
    stats := s.limiter.GetStats()
    
    response := map[string]interface{}{
        "status": "healthy",
        "rate_limiter": map[string]interface{}{
            "total_requests":   stats.TotalRequests,
            "allowed_requests": stats.AllowedRequests,
            "rejected_requests": stats.RejectedRequests,
            "current_tokens":   s.limiter.GetTokenCount(),
        },
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

## 📝 課題 (The Problem)

`main_test.go`のテストケースをすべてパスするように、以下の関数を実装してください：

1. **`NewRateLimiter(requestsPerSecond, burstCapacity int) *RateLimiter`**: Rate Limiterを初期化する
2. **`(rl *RateLimiter) Start()`**: トークンの定期補充を開始する
3. **`(rl *RateLimiter) Stop()`**: Rate Limiterを停止する
4. **`(rl *RateLimiter) Allow() bool`**: リクエストの許可判定を行う
5. **`(rl *RateLimiter) TryAllow() bool`**: ノンブロッキングで許可判定を行う
6. **`(rl *RateLimiter) AllowWithTimeout(timeout time.Duration) bool`**: タイムアウト付きで許可判定を行う
7. **`NewAdvancedRateLimiter(requestsPerSecond, burstCapacity int) *AdvancedRateLimiter`**: 高機能Rate Limiterを作成する

**重要な実装要件：**
- time.Tickerを使って一定間隔でトークンを補充すること
- Token Bucketアルゴリズムを正しく実装すること
- バースト処理を適切に制限すること
- 複数のGoroutineから安全にアクセスできること
- 統計情報を正確に収集すること
- 大量の並行リクエスト（1,000件以上）を効率的に処理できること

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のような結果が得られます：

### テスト実行例
```bash
$ go test -v
=== RUN   TestRateLimiter
=== RUN   TestRateLimiter/Basic_functionality
=== RUN   TestRateLimiter/Burst_handling
=== RUN   TestRateLimiter/Rate_limiting
=== RUN   TestRateLimiter/Concurrent_access
--- PASS: TestRateLimiter (0.25s)
=== RUN   TestAdvancedRateLimiter
=== RUN   TestAdvancedRateLimiter/Statistics_collection
=== RUN   TestAdvancedRateLimiter/Token_refill
--- PASS: TestAdvancedRateLimiter (0.15s)
PASS
```

### ベンチマーク実行例
```bash
$ go test -bench=.
BenchmarkRateLimiterAllow-8         	 1000000	      1200 ns/op
BenchmarkRateLimiterTryAllow-8      	 5000000	       240 ns/op
BenchmarkConcurrentAccess-8         	  500000	      2400 ns/op
```
ノンブロッキング版が5倍高速で、並行アクセスでも安定したパフォーマンスが確認できます。

### プログラム実行例
```bash
$ go run main.go
=== Rate Limiter Demo ===

Configuration:
- Rate: 10 requests/second
- Burst capacity: 20 requests
- Test duration: 30 seconds

Testing basic rate limiting...

Time: 0.0s | Tokens: 20 | Request: ALLOWED (burst)
Time: 0.1s | Tokens: 19 | Request: ALLOWED (burst)
Time: 0.2s | Tokens: 18 | Request: ALLOWED (burst)
...
Time: 2.0s | Tokens: 0 | Request: REJECTED (rate limited)
Time: 2.1s | Tokens: 1 | Request: ALLOWED (refilled)
Time: 2.2s | Tokens: 0 | Request: REJECTED (rate limited)

Concurrent access test with 100 goroutines...

Statistics after 30 seconds:
- Total requests: 1247
- Allowed requests: 312 (25.0%)
- Rejected requests: 935 (75.0%)
- Average wait time: 0.15ms
- Effective rate: 10.4 requests/second

Rate limiting is working correctly!
```

## 💡 ヒント (Hints)

詰まってしまった場合は、以下のヒントを参考にしてください：

### 基本的なToken Bucket実装
```go
type RateLimiter struct {
    rate      time.Duration
    capacity  int
    tokens    int
    ticker    *time.Ticker
    tokenChan chan struct{}
    mu        sync.Mutex
}

func (rl *RateLimiter) Start() {
    rl.ticker = time.NewTicker(rl.rate)
    go func() {
        for range rl.ticker.C {
            rl.mu.Lock()
            if rl.tokens < rl.capacity {
                rl.tokens++
                select {
                case rl.tokenChan <- struct{}{}:
                default:
                }
            }
            rl.mu.Unlock()
        }
    }()
}
```

### 許可判定の実装
```go
func (rl *RateLimiter) Allow() bool {
    select {
    case <-rl.tokenChan:
        return true
    }
}

func (rl *RateLimiter) TryAllow() bool {
    select {
    case <-rl.tokenChan:
        return true
    default:
        return false
    }
}
```

### 統計情報の更新
```go
func (arl *AdvancedRateLimiter) updateStats(allowed bool, waitTime time.Duration) {
    arl.stats.mu.Lock()
    defer arl.stats.mu.Unlock()
    
    arl.stats.TotalRequests++
    if allowed {
        arl.stats.AllowedRequests++
        // 平均待機時間を更新
        count := arl.stats.AllowedRequests
        arl.stats.AverageWaitTime = time.Duration(
            (int64(arl.stats.AverageWaitTime)*(count-1) + int64(waitTime)) / count,
        )
    } else {
        arl.stats.RejectedRequests++
    }
}
```

### 使用する主要なパッケージ
- `time` - Ticker、Duration、タイムアウト処理
- `sync` - Mutex、RWMutex、並行制御
- `context` - キャンセレーション処理
- `sync/atomic` - アトミック操作（高性能版）

### デバッグのコツ
1. `go test -race`でレースコンディションを検出
2. トークン補充のタイミングをログで確認
3. 統計情報の計算が正確か検証
4. 並行アクセス時の挙動をテスト

### よくある間違い
- Tickerの停止忘れ → リソースリーク
- チャネルのデッドロック → ノンブロッキング送信を使用
- 統計の競合状態 → 適切な排他制御
- トークンの過剰補充 → 容量制限を実装

## 実行方法

```bash
# テスト実行
go test -v

# レースコンディション検出
go test -race

# ベンチマーク測定
go test -bench=.

# ロングランニングテスト
go test -timeout=60s

# プログラム実行
go run main.go
```

## 参考資料

- [Go time package](https://pkg.go.dev/time)
- [Token Bucket Algorithm](https://en.wikipedia.org/wiki/Token_bucket)
- [Rate Limiting Patterns](https://blog.golang.org/context)
- [Go Concurrency Patterns](https://golang.org/doc/codewalk/sharemem/)