# Day 17: Graceful Shutdown（優雅な停止）

## 🎯 本日の目標 (Today's Goal)

os.Signalとcontextを使い、進行中のリクエストを待機してから安全にサーバーを停止するGraceful Shutdownを実装できるようになる。

## 📖 解説 (Explanation)

### Graceful Shutdownとは

Graceful Shutdown（優雅な停止）は、サーバーを停止する際に、進行中のリクエストの処理を完了させてから安全に停止させる仕組みです。これにより、ユーザーに迷惑をかけることなく、データの整合性を保ったままサーバーメンテナンスが可能になります。

### なぜGraceful Shutdownが必要なのか

**問題のあるシャットダウン:**
```bash
# 強制終了 - 進行中のリクエストが途中で切断される
$ kill -9 <server-pid>
```

この場合以下の問題が発生します：
- ユーザーのリクエストが途中で切断される
- データベースの更新処理が中断される
- ファイルアップロードが失敗する
- レスポンスが返らずにタイムアウトエラーになる

**Graceful Shutdownの利点:**
- 進行中のリクエストを完了まで待機
- 新しいリクエストの受付を停止
- リソースの適切なクリーンアップ
- ユーザーエクスペリエンスの向上

### シグナルハンドリング

Unixシステムでは、プロセスにシグナルを送ることで制御を行います：

```go
import (
    "os"
    "os/signal"
    "syscall"
)

func setupSignalHandling() chan os.Signal {
    sigChan := make(chan os.Signal, 1)
    
    // SIGINT (Ctrl+C) と SIGTERM を監視
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    return sigChan
}
```

**主要なシグナル:**
- `SIGINT` (Interrupt): Ctrl+Cで送信される
- `SIGTERM` (Terminate): killコマンドのデフォルト
- `SIGKILL`: 強制終了（ハンドリング不可）

### HTTP ServerのGraceful Shutdown

Go 1.8以降、`http.Server`にはGraceful Shutdownをサポートする`Shutdown`メソッドが提供されています：

```go
func (srv *Server) Shutdown(ctx context.Context) error
```

このメソッドは以下の動作を行います：
1. 新しい接続の受付を停止
2. アイドル接続を即座にクローズ
3. アクティブな接続が完了するまで待機
4. コンテキストがタイムアウトすると強制終了

### 基本的な実装パターン

```go
func main() {
    server := &http.Server{
        Addr:    ":8080",
        Handler: setupRoutes(),
    }
    
    // シグナルハンドリングの設定
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    // サーバーを別のgoroutineで起動
    go func() {
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server error: %v", err)
        }
    }()
    
    log.Println("Server starting on :8080")
    
    // シグナルを待機
    <-sigChan
    log.Println("Shutting down server...")
    
    // タイムアウト付きでGraceful Shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server shutdown failed: %v", err)
    }
    
    log.Println("Server stopped gracefully")
}
```

### アクティブリクエストの追跡

より高度な制御のために、アクティブなリクエスト数を追跡することができます：

```go
type GracefulServer struct {
    server         *http.Server
    activeRequests int64
    shutdown       chan struct{}
}

func (gs *GracefulServer) requestTrackingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // リクエスト数を増加
        atomic.AddInt64(&gs.activeRequests, 1)
        defer atomic.AddInt64(&gs.activeRequests, -1)
        
        // シャットダウン中は新しいリクエストを拒否
        select {
        case <-gs.shutdown:
            http.Error(w, "Server is shutting down", http.StatusServiceUnavailable)
            return
        default:
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### タイムアウト付きの実装

```go
func (gs *GracefulServer) Shutdown(timeout time.Duration) error {
    log.Println("Starting graceful shutdown...")
    
    // 新しいリクエストの受付を停止
    close(gs.shutdown)
    
    // サーバーの停止開始
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    shutdownErr := gs.server.Shutdown(ctx)
    
    // アクティブリクエストの完了を待機
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        active := atomic.LoadInt64(&gs.activeRequests)
        if active == 0 {
            break
        }
        
        select {
        case <-ctx.Done():
            log.Printf("Shutdown timeout, forcing close with %d active requests", active)
            return ctx.Err()
        case <-ticker.C:
            log.Printf("Waiting for %d active requests to complete...", active)
        }
    }
    
    log.Println("All requests completed")
    return shutdownErr
}
```

### 長時間実行タスクの処理

長時間実行されるリクエストに対しては、コンテキストを利用して適切にキャンセル処理を実装します：

```go
func longRunningHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // 長時間の処理をシミュレート
    select {
    case <-time.After(10 * time.Second):
        w.Write([]byte("処理完了"))
    case <-ctx.Done():
        log.Println("Request cancelled due to shutdown")
        http.Error(w, "Request cancelled", http.StatusRequestTimeout)
    }
}
```

### 実践的な実装例

```go
type GracefulServer struct {
    server          *http.Server
    config          *ServerConfig
    shutdownSignal  chan os.Signal
    activeRequests  int64
    shutdownOnce    sync.Once
    isShuttingDown  bool
    shutdownMutex   sync.RWMutex
}

func (gs *GracefulServer) Start() error {
    // シグナルハンドリング設定
    signal.Notify(gs.shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
    
    // サーバー起動
    serverErr := make(chan error, 1)
    go func() {
        if err := gs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            serverErr <- err
        }
        close(serverErr)
    }()
    
    // シグナル待機
    select {
    case err := <-serverErr:
        return err
    case sig := <-gs.shutdownSignal:
        log.Printf("Received signal: %v", sig)
        return gs.gracefulShutdown()
    }
}

func (gs *GracefulServer) gracefulShutdown() error {
    var shutdownErr error
    
    gs.shutdownOnce.Do(func() {
        log.Println("Starting graceful shutdown...")
        
        // シャットダウン状態に設定
        gs.shutdownMutex.Lock()
        gs.isShuttingDown = true
        gs.shutdownMutex.Unlock()
        
        // タイムアウト付きコンテキスト
        ctx, cancel := context.WithTimeout(context.Background(), gs.config.ShutdownTimeout)
        defer cancel()
        
        // サーバー停止
        shutdownErr = gs.server.Shutdown(ctx)
        
        // アクティブリクエストの完了待機
        gs.waitForActiveRequests(ctx)
    })
    
    return shutdownErr
}
```

### デプロイメント環境での考慮事項

#### Docker環境
```dockerfile
# SIGTERMを適切に処理するため
STOPSIGNAL SIGTERM

# タイムアウト設定（デフォルトは10秒）
# docker stop --time=30 container_name
```

#### Kubernetes環境
```yaml
apiVersion: v1
kind: Pod
spec:
  terminationGracePeriodSeconds: 30  # Pod終了の猶予時間
  containers:
  - name: app
    # アプリケーションは30秒以内にSIGTERMに応答する必要がある
```

#### ロードバランサーとの連携
1. ヘルスチェックエンドポイントでシャットダウン状態を通知
2. ロードバランサーがトラフィックを他のインスタンスに流す
3. 猶予時間後にGraceful Shutdownを実行

## 📝 課題 (The Problem)

`main_test.go`に書かれているテストをパスするように、以下の機能を実装してください：

1. **ServerConfig構造体**: サーバー設定とシャットダウンタイムアウト
2. **GracefulServer構造体**: Graceful Shutdown機能付きサーバー
3. **シグナルハンドリング**: SIGINT/SIGTERMの適切な処理
4. **リクエスト追跡**: アクティブリクエスト数の監視
5. **ハンドラー実装**: ヘルスチェック、ステータス、長時間実行ハンドラー

### 実装すべき関数

```go
// ServerConfig holds server configuration
type ServerConfig struct {
    Port            string
    ShutdownTimeout time.Duration
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
}

// GracefulServer represents a server with graceful shutdown capability
type GracefulServer struct {
    server          *http.Server
    config          *ServerConfig
    shutdownSignal  chan os.Signal
    activeRequests  int64
    requestsMu      sync.RWMutex
    shutdownOnce    sync.Once
    isShuttingDown  bool
}

// NewServerConfig creates default server configuration
func NewServerConfig() *ServerConfig

// NewGracefulServer creates a new server with graceful shutdown capability
func NewGracefulServer(config *ServerConfig) *GracefulServer

// Start starts the server and sets up signal handling
func (gs *GracefulServer) Start() error

// Shutdown gracefully shuts down the server
func (gs *GracefulServer) Shutdown(ctx context.Context) error

// Request tracking methods
func (gs *GracefulServer) incrementActiveRequests()
func (gs *GracefulServer) decrementActiveRequests()
func (gs *GracefulServer) getActiveRequests() int64
```

## ✅ 期待される挙動 (Expected Behavior)

実装が完了すると、以下のような動作が期待されます：

### 1. 正常な起動とシャットダウン
```bash
$ go run main_solution.go
Server starting on :8080
Send SIGINT (Ctrl+C) or SIGTERM to gracefully shutdown

# Ctrl+C を押す
^C
Received signal: interrupt
Starting graceful shutdown...
All requests completed, server stopped gracefully
```

### 2. アクティブリクエスト待機
```bash
# 長時間リクエストを送信中にCtrl+C
$ curl "http://localhost:8080/long-running?delay=5s" &
$ # Ctrl+C を押す

# サーバーログ
Received signal: interrupt
Starting graceful shutdown...
Waiting for 1 active requests to complete...
Long-running request completed successfully
All requests completed, server stopped gracefully
```

### 3. ステータスエンドポイント
```bash
$ curl http://localhost:8080/status
{
  "active_requests": 0,
  "is_shutting_down": false,
  "server_config": {
    "port": ":8080",
    "shutdown_timeout": "30s"
  }
}
```

### 4. テスト結果
```bash
$ go test -v
=== RUN   TestServerConfig
--- PASS: TestServerConfig (0.00s)
=== RUN   TestGracefulShutdown
--- PASS: TestGracefulShutdown (6.00s)
=== RUN   TestRequestTracking
--- PASS: TestRequestTracking (1.30s)
PASS
```

## 💡 ヒント (Hints)

詰まった場合は、以下のヒントを参考にしてください：

### 1. シグナルハンドリングの基本
```go
import (
    "os"
    "os/signal"
    "syscall"
)

func setupSignalHandling() chan os.Signal {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    return sigChan
}
```

### 2. 役立つパッケージ
- `os/signal`: シグナルハンドリング
- `context`: タイムアウト制御
- `sync/atomic`: アトミック操作でリクエスト数管理
- `sync`: Mutex、Once、WaitGroup
- `net/http`: HTTP サーバーとShutdownメソッド

### 3. アトミック操作でのリクエスト追跡
```go
import "sync/atomic"

type GracefulServer struct {
    activeRequests int64
    // その他のフィールド
}

func (gs *GracefulServer) incrementActiveRequests() {
    atomic.AddInt64(&gs.activeRequests, 1)
}

func (gs *GracefulServer) decrementActiveRequests() {
    atomic.AddInt64(&gs.activeRequests, -1)
}

func (gs *GracefulServer) getActiveRequests() int64 {
    return atomic.LoadInt64(&gs.activeRequests)
}
```

### 4. リクエスト追跡ミドルウェア
```go
func (gs *GracefulServer) requestTrackingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // シャットダウン中チェック
        gs.requestsMu.RLock()
        if gs.isShuttingDown {
            gs.requestsMu.RUnlock()
            http.Error(w, "Server is shutting down", http.StatusServiceUnavailable)
            return
        }
        gs.requestsMu.RUnlock()
        
        // リクエスト追跡
        gs.incrementActiveRequests()
        defer gs.decrementActiveRequests()
        
        next.ServeHTTP(w, r)
    })
}
```

### 5. コンテキストを利用した長時間処理
```go
func (gs *GracefulServer) longRunningHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    delay := 2 * time.Second // デフォルト
    
    // クエリパラメータから遅延時間を取得
    if delayStr := r.URL.Query().Get("delay"); delayStr != "" {
        if parsedDelay, err := time.ParseDuration(delayStr); err == nil {
            delay = parsedDelay
        }
    }
    
    select {
    case <-time.After(delay):
        // 正常完了
        json.NewEncoder(w).Encode(map[string]interface{}{
            "message": "Long-running operation completed",
            "delay":   delay.String(),
        })
    case <-ctx.Done():
        // キャンセルまたはタイムアウト
        http.Error(w, "Request cancelled", http.StatusRequestTimeout)
    }
}
```

これらのヒントを参考に、段階的に実装を進めてください。まずは基本的なサーバー構造から始めて、徐々にシグナルハンドリングとGraceful Shutdown機能を追加していくのがおすすめです。