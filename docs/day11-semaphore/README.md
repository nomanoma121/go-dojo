# Day 11: Semaphoreパターン

## 🎯 本日の目標 (Today's Goal)

同時に実行可能な処理数を制限するセマフォパターンを実装し、リソース制御と公平性を確保した並行プログラミングを習得する。データベース接続やファイルハンドル、API呼び出しなどの限られたリソースを効率的に管理する。

## 📖 解説 (Explanation)

### Semaphoreパターンとは

セマフォ（Semaphore）は、同時に実行できる処理の数を制限するための同期プリミティブです。特定のリソースへの同時アクセス数を制御し、システムの安定性とパフォーマンスを両立させます。

### なぜSemaphoreが必要か

#### 1. リソース保護

```go
// 【Semaphoreによるリソース保護】データベース接続プールの完全実装
// ❌ 問題例：セマフォなしでのリソース枯渇災害
func disastrousUnlimitedDBAccess() {
    // 🚨 災害例：無制限なデータベース接続
    for i := 0; i < 10000; i++ {
        go func(requestID int) {
            // ❌ 新しい接続を毎回作成
            conn, err := sql.Open("postgres", "...")
            if err != nil {
                log.Printf("❌ Request %d: Connection failed: %v", requestID, err)
                return
            }
            defer conn.Close()
            
            // ❌ 10,000個の並行接続でデータベースサーバーが崩壊
            rows, err := conn.Query("SELECT * FROM users WHERE id = ?", requestID)
            if err != nil {
                log.Printf("❌ Request %d: Query failed: %v", requestID, err)
                return
            }
            defer rows.Close()
            
            // ❌ "too many connections" エラーでサービス全体が停止
            // ❌ データベースサーバーのメモリ枯渇
            // ❌ 他のアプリケーションも巻き込んで影響拡大
        }(i)
    }
    // 結果：データベースサーバーダウン、全サービス停止、顧客影響大
}

// ✅ 正解：プロダクション品質のセマフォ保護接続プール
type EnterpriseDBConnectionPool struct {
    // 【基本構成】
    connections chan *sql.DB
    maxConns    int
    
    // 【高度な機能】
    activeConns   int64              // 現在のアクティブ接続数
    totalRequests int64              // 総リクエスト数
    waitingQueue  int64              // 待機中のリクエスト数
    
    // 【監視・運用】
    metrics      *PoolMetrics
    healthCheck  *HealthChecker
    logger       *log.Logger
    
    // 【制御機能】
    ctx          context.Context
    cancel       context.CancelFunc
    closeOnce    sync.Once
}

func NewEnterpriseDBConnectionPool(maxConns int, dbURL string) (*EnterpriseDBConnectionPool, error) {
    ctx, cancel := context.WithCancel(context.Background())
    
    pool := &EnterpriseDBConnectionPool{
        connections: make(chan *sql.DB, maxConns),
        maxConns:    maxConns,
        metrics:     NewPoolMetrics(),
        healthCheck: NewHealthChecker(),
        logger:      log.New(os.Stdout, "[DB-POOL] ", log.LstdFlags),
        ctx:         ctx,
        cancel:      cancel,
    }
    
    // 【重要】接続の事前作成とヘルスチェック
    for i := 0; i < maxConns; i++ {
        conn, err := sql.Open("postgres", dbURL)
        if err != nil {
            pool.cancel()
            return nil, fmt.Errorf("failed to create connection %d: %w", i, err)
        }
        
        // 【接続設定の最適化】
        conn.SetMaxOpenConns(1)           // プール管理なので1接続
        conn.SetMaxIdleConns(1)           // アイドル接続も1
        conn.SetConnMaxLifetime(1*time.Hour) // 1時間で接続更新
        
        // 【接続テスト】
        if err := conn.PingContext(ctx); err != nil {
            pool.cancel()
            return nil, fmt.Errorf("connection %d ping failed: %w", i, err)
        }
        
        pool.connections <- conn
        pool.logger.Printf("✅ Connection %d established", i+1)
    }
    
    // 【監視開始】
    go pool.startMetricsCollection()
    go pool.startHealthMonitoring()
    
    pool.logger.Printf("🚀 Enterprise DB Pool initialized with %d connections", maxConns)
    return pool, nil
}

// 【重要メソッド】タイムアウト付き接続取得
func (pool *EnterpriseDBConnectionPool) GetConnectionWithTimeout(timeout time.Duration) (*sql.DB, error) {
    // 【メトリクス記録】
    atomic.AddInt64(&pool.totalRequests, 1)
    atomic.AddInt64(&pool.waitingQueue, 1)
    defer atomic.AddInt64(&pool.waitingQueue, -1)
    
    start := time.Now()
    
    // 【タイムアウト付きセマフォ取得】
    select {
    case conn := <-pool.connections:
        // 【成功メトリクス】
        atomic.AddInt64(&pool.activeConns, 1)
        waitTime := time.Since(start)
        pool.metrics.RecordWaitTime(waitTime)
        
        // 【接続ヘルスチェック】
        if err := conn.PingContext(pool.ctx); err != nil {
            pool.logger.Printf("❌ Unhealthy connection detected, creating new one")
            
            // 不健全な接続を破棄し、新しい接続を作成
            conn.Close()
            newConn, err := pool.createNewConnection()
            if err != nil {
                atomic.AddInt64(&pool.activeConns, -1)
                return nil, fmt.Errorf("failed to create replacement connection: %w", err)
            }
            conn = newConn
        }
        
        pool.logger.Printf("✅ Connection acquired in %v (waiting: %d)", 
            waitTime, atomic.LoadInt64(&pool.waitingQueue))
        return conn, nil
        
    case <-time.After(timeout):
        // 【タイムアウトエラー】
        pool.metrics.RecordTimeout()
        waiting := atomic.LoadInt64(&pool.waitingQueue)
        
        pool.logger.Printf("⏰ Connection timeout after %v (waiting: %d, active: %d)", 
            timeout, waiting, atomic.LoadInt64(&pool.activeConns))
        
        return nil, fmt.Errorf("connection acquisition timeout after %v (waiting: %d)", timeout, waiting)
        
    case <-pool.ctx.Done():
        // 【プール停止】
        return nil, errors.New("connection pool is closed")
    }
}

// 【重要メソッド】接続の安全な返却
func (pool *EnterpriseDBConnectionPool) ReleaseConnection(conn *sql.DB) {
    if conn == nil {
        return
    }
    
    // 【接続の健全性チェック】
    if err := conn.PingContext(pool.ctx); err != nil {
        pool.logger.Printf("⚠️  Releasing unhealthy connection, creating replacement")
        
        // 不健全な接続を破棄
        conn.Close()
        atomic.AddInt64(&pool.activeConns, -1)
        
        // 新しい接続を非同期で作成
        go func() {
            if newConn, err := pool.createNewConnection(); err == nil {
                select {
                case pool.connections <- newConn:
                    pool.logger.Printf("✅ Replacement connection created")
                case <-pool.ctx.Done():
                    newConn.Close()
                }
            } else {
                pool.logger.Printf("❌ Failed to create replacement connection: %v", err)
            }
        }()
        return
    }
    
    // 【正常な接続返却】
    select {
    case pool.connections <- conn:
        atomic.AddInt64(&pool.activeConns, -1)
        pool.logger.Printf("✅ Connection released (active: %d)", 
            atomic.LoadInt64(&pool.activeConns))
    case <-pool.ctx.Done():
        // プール停止中は接続を閉じる
        conn.Close()
        atomic.AddInt64(&pool.activeConns, -1)
    }
}

// 【高度な機能】新しい接続の作成
func (pool *EnterpriseDBConnectionPool) createNewConnection() (*sql.DB, error) {
    conn, err := sql.Open("postgres", pool.getConnectionString())
    if err != nil {
        return nil, err
    }
    
    conn.SetMaxOpenConns(1)
    conn.SetMaxIdleConns(1)
    conn.SetConnMaxLifetime(1*time.Hour)
    
    if err := conn.PingContext(pool.ctx); err != nil {
        conn.Close()
        return nil, err
    }
    
    return conn, nil
}

// 【監視機能】メトリクス収集
func (pool *EnterpriseDBConnectionPool) startMetricsCollection() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            active := atomic.LoadInt64(&pool.activeConns)
            waiting := atomic.LoadInt64(&pool.waitingQueue)
            total := atomic.LoadInt64(&pool.totalRequests)
            
            pool.logger.Printf("📊 Pool Stats - Active: %d/%d, Waiting: %d, Total Requests: %d", 
                active, pool.maxConns, waiting, total)
                
            // 【アラート条件】
            if waiting > 10 {
                pool.logger.Printf("⚠️  HIGH WAIT QUEUE: %d requests waiting", waiting)
            }
            
            if float64(active)/float64(pool.maxConns) > 0.9 {
                pool.logger.Printf("⚠️  HIGH POOL UTILIZATION: %.1f%%", 
                    float64(active)/float64(pool.maxConns)*100)
            }
            
        case <-pool.ctx.Done():
            return
        }
    }
}

// 【実用例】プロダクション環境での使用パターン
func ProductionDatabaseUsage() {
    // プール初期化（本番環境設定）
    pool, err := NewEnterpriseDBConnectionPool(25, "postgres://...")
    if err != nil {
        log.Fatalf("Failed to initialize DB pool: %v", err)
    }
    defer pool.Close()
    
    // 【高負荷シナリオ】1000件の並行リクエスト
    var wg sync.WaitGroup
    successCount := int64(0)
    errorCount := int64(0)
    
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func(requestID int) {
            defer wg.Done()
            
            // 【接続取得】最大30秒待機
            conn, err := pool.GetConnectionWithTimeout(30 * time.Second)
            if err != nil {
                atomic.AddInt64(&errorCount, 1)
                log.Printf("❌ Request %d: %v", requestID, err)
                return
            }
            defer pool.ReleaseConnection(conn)
            
            // 【データベース操作】
            rows, err := conn.Query("SELECT id, name FROM users LIMIT 10")
            if err != nil {
                atomic.AddInt64(&errorCount, 1)
                log.Printf("❌ Request %d query failed: %v", requestID, err)
                return
            }
            defer rows.Close()
            
            // 結果処理
            for rows.Next() {
                var id int
                var name string
                rows.Scan(&id, &name)
            }
            
            atomic.AddInt64(&successCount, 1)
            if requestID%100 == 0 {
                log.Printf("✅ Request %d completed", requestID)
            }
        }(i)
    }
    
    wg.Wait()
    
    success := atomic.LoadInt64(&successCount)
    errors := atomic.LoadInt64(&errorCount)
    
    log.Printf("🎯 Final Results: %d success, %d errors (%.2f%% success rate)", 
        success, errors, float64(success)/1000*100)
}
```

#### 2. レート制限

```go
// API呼び出しレート制限の例
type RateLimiter struct {
    semaphore chan struct{}
    interval  time.Duration
}

func NewRateLimiter(maxConcurrent int, interval time.Duration) *RateLimiter {
    return &RateLimiter{
        semaphore: make(chan struct{}, maxConcurrent),
        interval:  interval,
    }
}

func (rl *RateLimiter) Execute(fn func() error) error {
    // セマフォを取得
    rl.semaphore <- struct{}{}
    defer func() { <-rl.semaphore }()
    
    // レート制限を適用
    time.Sleep(rl.interval)
    return fn()
}
```

### 基本的なSemaphore実装

#### 1. Channel-based Semaphore

```go
type Semaphore struct {
    permits chan struct{}
}

func NewSemaphore(maxPermits int) *Semaphore {
    return &Semaphore{
        permits: make(chan struct{}, maxPermits),
    }
}

func (s *Semaphore) Acquire() {
    s.permits <- struct{}{}
}

func (s *Semaphore) Release() {
    <-s.permits
}

func (s *Semaphore) TryAcquire() bool {
    select {
    case s.permits <- struct{}{}:
        return true
    default:
        return false
    }
}

func (s *Semaphore) TryAcquireWithTimeout(timeout time.Duration) bool {
    select {
    case s.permits <- struct{}{}:
        return true
    case <-time.After(timeout):
        return false
    }
}
```

#### 2. Weighted Semaphore

```go
type WeightedSemaphore struct {
    mu       sync.Mutex
    size     int64
    cur      int64
    waiters  []waiter
}

type waiter struct {
    n     int64
    ready chan<- struct{}
}

func NewWeightedSemaphore(n int64) *WeightedSemaphore {
    return &WeightedSemaphore{size: n}
}

func (s *WeightedSemaphore) Acquire(ctx context.Context, n int64) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if s.size-s.cur >= n && len(s.waiters) == 0 {
        s.cur += n
        return nil
    }
    
    if n > s.size {
        return errors.New("semaphore: requested weight exceeds capacity")
    }
    
    ready := make(chan struct{})
    w := waiter{n: n, ready: ready}
    s.waiters = append(s.waiters, w)
    s.mu.Unlock()
    
    select {
    case <-ctx.Done():
        s.mu.Lock()
        s.removeWaiter(w)
        s.mu.Unlock()
        return ctx.Err()
    case <-ready:
        return nil
    }
}

func (s *WeightedSemaphore) Release(n int64) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    s.cur -= n
    s.notifyWaiters()
}

func (s *WeightedSemaphore) notifyWaiters() {
    for len(s.waiters) > 0 {
        next := s.waiters[0]
        if s.size-s.cur < next.n {
            break
        }
        
        s.cur += next.n
        s.waiters = s.waiters[1:]
        close(next.ready)
    }
}
```

### 高度なSemaphoreパターン

#### 1. 公平性を保証するSemaphore

```go
type FairSemaphore struct {
    mu        sync.Mutex
    permits   int
    available int
    waitQueue chan struct{}
}

func NewFairSemaphore(permits int) *FairSemaphore {
    return &FairSemaphore{
        permits:   permits,
        available: permits,
        waitQueue: make(chan struct{}, 1000), // 待機キューの最大サイズ
    }
}

func (fs *FairSemaphore) Acquire(ctx context.Context) error {
    // 待機キューに参加
    select {
    case fs.waitQueue <- struct{}{}:
    case <-ctx.Done():
        return ctx.Err()
    }
    
    // 順番を待つ
    for {
        fs.mu.Lock()
        if fs.available > 0 {
            fs.available--
            <-fs.waitQueue // キューから削除
            fs.mu.Unlock()
            return nil
        }
        fs.mu.Unlock()
        
        select {
        case <-ctx.Done():
            // キューから削除してからエラーを返す
            select {
            case <-fs.waitQueue:
            default:
            }
            return ctx.Err()
        case <-time.After(10 * time.Millisecond):
            // 短い間隔で再試行
        }
    }
}

func (fs *FairSemaphore) Release() {
    fs.mu.Lock()
    defer fs.mu.Unlock()
    
    if fs.available < fs.permits {
        fs.available++
    }
}
```

#### 2. 優先度付きSemaphore

```go
type PrioritySemaphore struct {
    mu          sync.Mutex
    permits     int
    available   int
    highPriQueue chan struct{}
    lowPriQueue  chan struct{}
}

func NewPrioritySemaphore(permits int) *PrioritySemaphore {
    return &PrioritySemaphore{
        permits:      permits,
        available:    permits,
        highPriQueue: make(chan struct{}, 500),
        lowPriQueue:  make(chan struct{}, 500),
    }
}

func (ps *PrioritySemaphore) AcquireHigh(ctx context.Context) error {
    return ps.acquire(ctx, ps.highPriQueue, true)
}

func (ps *PrioritySemaphore) AcquireLow(ctx context.Context) error {
    return ps.acquire(ctx, ps.lowPriQueue, false)
}

func (ps *PrioritySemaphore) acquire(ctx context.Context, queue chan struct{}, isHigh bool) error {
    select {
    case queue <- struct{}{}:
    case <-ctx.Done():
        return ctx.Err()
    }
    
    for {
        ps.mu.Lock()
        if ps.available > 0 {
            // 高優先度のリクエストが待機中の場合、低優先度は待機
            if !isHigh && len(ps.highPriQueue) > 0 {
                ps.mu.Unlock()
                time.Sleep(time.Millisecond)
                continue
            }
            
            ps.available--
            <-queue
            ps.mu.Unlock()
            return nil
        }
        ps.mu.Unlock()
        
        select {
        case <-ctx.Done():
            select {
            case <-queue:
            default:
            }
            return ctx.Err()
        case <-time.After(10 * time.Millisecond):
        }
    }
}
```

#### 3. 動的Semaphore

```go
type DynamicSemaphore struct {
    mu        sync.RWMutex
    permits   chan struct{}
    maxSize   int
    curSize   int
    waiters   []chan struct{}
}

func NewDynamicSemaphore(initialSize, maxSize int) *DynamicSemaphore {
    ds := &DynamicSemaphore{
        permits: make(chan struct{}, maxSize),
        maxSize: maxSize,
        curSize: initialSize,
    }
    
    // 初期permit数を設定
    for i := 0; i < initialSize; i++ {
        ds.permits <- struct{}{}
    }
    
    return ds
}

func (ds *DynamicSemaphore) Acquire(ctx context.Context) error {
    select {
    case <-ds.permits:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (ds *DynamicSemaphore) Release() {
    select {
    case ds.permits <- struct{}{}:
    default:
        // チャネルが満杯の場合は何もしない
    }
}

func (ds *DynamicSemaphore) Resize(newSize int) error {
    ds.mu.Lock()
    defer ds.mu.Unlock()
    
    if newSize > ds.maxSize {
        return errors.New("new size exceeds maximum capacity")
    }
    
    if newSize > ds.curSize {
        // サイズを増加
        for i := ds.curSize; i < newSize; i++ {
            select {
            case ds.permits <- struct{}{}:
            default:
                return errors.New("failed to increase semaphore size")
            }
        }
    } else if newSize < ds.curSize {
        // サイズを減少
        for i := newSize; i < ds.curSize; i++ {
            select {
            case <-ds.permits:
            case <-time.After(100 * time.Millisecond):
                return errors.New("timeout while reducing semaphore size")
            }
        }
    }
    
    ds.curSize = newSize
    return nil
}
```

### 実用例：ワーカープール with Semaphore

```go
type WorkerPool struct {
    semaphore    *Semaphore
    taskQueue    chan Task
    resultQueue  chan Result
    workerCount  int
    ctx          context.Context
    cancel       context.CancelFunc
    wg           sync.WaitGroup
}

type Task struct {
    ID   string
    Data interface{}
    Fn   func(interface{}) (interface{}, error)
}

type Result struct {
    TaskID string
    Data   interface{}
    Error  error
}

func NewWorkerPool(maxWorkers, queueSize int) *WorkerPool {
    ctx, cancel := context.WithCancel(context.Background())
    
    wp := &WorkerPool{
        semaphore:   NewSemaphore(maxWorkers),
        taskQueue:   make(chan Task, queueSize),
        resultQueue: make(chan Result, queueSize),
        workerCount: maxWorkers,
        ctx:         ctx,
        cancel:      cancel,
    }
    
    wp.start()
    return wp
}

func (wp *WorkerPool) start() {
    for i := 0; i < wp.workerCount; i++ {
        wp.wg.Add(1)
        go wp.worker()
    }
}

func (wp *WorkerPool) worker() {
    defer wp.wg.Done()
    
    for {
        select {
        case task := <-wp.taskQueue:
            wp.semaphore.Acquire()
            result := wp.processTask(task)
            wp.semaphore.Release()
            
            select {
            case wp.resultQueue <- result:
            case <-wp.ctx.Done():
                return
            }
            
        case <-wp.ctx.Done():
            return
        }
    }
}

func (wp *WorkerPool) processTask(task Task) Result {
    data, err := task.Fn(task.Data)
    return Result{
        TaskID: task.ID,
        Data:   data,
        Error:  err,
    }
}

func (wp *WorkerPool) Submit(task Task) error {
    select {
    case wp.taskQueue <- task:
        return nil
    case <-wp.ctx.Done():
        return wp.ctx.Err()
    default:
        return errors.New("task queue is full")
    }
}

func (wp *WorkerPool) GetResult() <-chan Result {
    return wp.resultQueue
}

func (wp *WorkerPool) Shutdown(timeout time.Duration) error {
    wp.cancel()
    
    done := make(chan struct{})
    go func() {
        wp.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-time.After(timeout):
        return errors.New("shutdown timeout")
    }
}
```

### パフォーマンス監視とメトリクス

```go
type SemaphoreMetrics struct {
    mu              sync.RWMutex
    acquireCount    int64
    releaseCount    int64
    timeoutCount    int64
    avgWaitTime     time.Duration
    maxWaitTime     time.Duration
    currentWaiters  int64
    totalWaitTime   time.Duration
}

type MonitoredSemaphore struct {
    semaphore *Semaphore
    metrics   *SemaphoreMetrics
}

func NewMonitoredSemaphore(permits int) *MonitoredSemaphore {
    return &MonitoredSemaphore{
        semaphore: NewSemaphore(permits),
        metrics:   &SemaphoreMetrics{},
    }
}

func (ms *MonitoredSemaphore) Acquire(ctx context.Context) error {
    start := time.Now()
    ms.metrics.mu.Lock()
    ms.metrics.currentWaiters++
    ms.metrics.mu.Unlock()
    
    err := ms.semaphore.AcquireWithContext(ctx)
    
    waitTime := time.Since(start)
    ms.metrics.mu.Lock()
    defer ms.metrics.mu.Unlock()
    
    ms.metrics.currentWaiters--
    
    if err != nil {
        ms.metrics.timeoutCount++
        return err
    }
    
    ms.metrics.acquireCount++
    ms.metrics.totalWaitTime += waitTime
    
    if waitTime > ms.metrics.maxWaitTime {
        ms.metrics.maxWaitTime = waitTime
    }
    
    ms.metrics.avgWaitTime = time.Duration(int64(ms.metrics.totalWaitTime) / ms.metrics.acquireCount)
    return nil
}

func (ms *MonitoredSemaphore) Release() {
    ms.semaphore.Release()
    
    ms.metrics.mu.Lock()
    ms.metrics.releaseCount++
    ms.metrics.mu.Unlock()
}

func (ms *MonitoredSemaphore) GetMetrics() SemaphoreMetrics {
    ms.metrics.mu.RLock()
    defer ms.metrics.mu.RUnlock()
    
    return *ms.metrics
}
```

## 📝 課題 (The Problem)

以下の機能を持つ包括的なSemaphoreシステムを実装してください：

### 1. 基本Semaphore
- 指定された数の許可証を管理
- Acquire/Release操作
- タイムアウト付き取得
- コンテキスト対応

### 2. 公平性保証
- FIFO順での許可証配布
- 優先度付きアクセス
- 飢餓状態の防止

### 3. 動的調整
- 実行時での許可証数変更
- 負荷に応じた自動調整
- 設定の動的更新

### 4. 監視機能
- 待機時間の測定
- 利用率の監視
- パフォーマンスメトリクス

### 5. 統合機能
- ワーカープールとの統合
- リソースプールとの連携
- レート制限との組み合わせ

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestSemaphore_BasicOperations
    main_test.go:45: Basic acquire/release operations working
--- PASS: TestSemaphore_BasicOperations (0.01s)

=== RUN   TestSemaphore_Fairness
    main_test.go:65: FIFO fairness maintained correctly
--- PASS: TestSemaphore_Fairness (0.02s)

=== RUN   TestSemaphore_Timeout
    main_test.go:85: Timeout handling working correctly
--- PASS: TestSemaphore_Timeout (0.03s)

=== RUN   TestSemaphore_Performance
    main_test.go:105: Performance metrics within acceptable range
--- PASS: TestSemaphore_Performance (0.05s)

PASS
ok      day11-semaphore   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### チャネルベースの実装

```go
type Semaphore struct {
    permits chan struct{}
}

func NewSemaphore(maxPermits int) *Semaphore {
    s := &Semaphore{
        permits: make(chan struct{}, maxPermits),
    }
    
    // 初期許可証を配布
    for i := 0; i < maxPermits; i++ {
        s.permits <- struct{}{}
    }
    
    return s
}
```

### タイムアウト処理

```go
func (s *Semaphore) AcquireWithTimeout(timeout time.Duration) bool {
    select {
    case <-s.permits:
        return true
    case <-time.After(timeout):
        return false
    }
}
```

### 公平性の実装

```go
type FairSemaphore struct {
    permits chan struct{}
    queue   chan chan struct{}
}

func (fs *FairSemaphore) Acquire() {
    response := make(chan struct{})
    fs.queue <- response
    <-response
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **適応的Semaphore**: 負荷に応じた許可証数の自動調整
2. **分散Semaphore**: 複数ノード間でのセマフォ共有
3. **階層Semaphore**: ネストした資源管理
4. **セマフォクラスター**: 複数のセマフォの協調制御
5. **セマフォパターン解析**: 使用パターンの学習と最適化

Semaphoreパターンの実装を通じて、効率的なリソース管理と並行制御の手法を習得しましょう！