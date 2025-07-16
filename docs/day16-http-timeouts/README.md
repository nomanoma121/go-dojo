# Day 16: HTTP Serverタイムアウト設定

## 🎯 本日の目標 (Today's Goal)

HTTPサーバーのRead/Write/Idleタイムアウトを適切に設定し、プロダクション環境で安定動作するサーバーの実装方法を理解する。

## 📖 解説 (Explanation)

### HTTPサーバーのタイムアウトとは

HTTPサーバーにおけるタイムアウト設定は、サーバーの安定性とセキュリティにとって重要な要素です。適切なタイムアウト設定により、以下の問題を防ぐことができます：

- 遅いクライアントによるリソース枯渇
- 悪意のあるクライアントからのスローロリス攻撃
- メモリリークや接続プールの枯渇

### タイムアウトの種類

#### 1. ReadTimeout
クライアントからのリクエスト全体を読み取るまでの最大時間です。

```go
// 【ReadTimeout】クライアントからのリクエスト読み取りタイムアウト
server := &http.Server{
    ReadTimeout: 10 * time.Second,  // 【重要】リクエスト全体の読み取り制限時間
    
    // 【内部動作】：
    // 1. TCP接続確立後から開始
    // 2. HTTPヘッダー + リクエストボディの読み取り完了まで
    // 3. 制限時間を超過した場合、接続を強制クローズ
    // 4. エラーログ：「http: request body read timeout」
    //
    // 【保護対象】：
    // - 遅いネットワーク環境のクライアント
    // - 意図的に遅いリクエストを送信する攻撃者
    // - 大きなファイルアップロードの無制限実行
    //
    // 【設定指針】：
    // - API: 5-30秒
    // - ファイルアップロード: 5-10分
    // - ストリーミング: 無効化または長時間設定
}
```

**用途：**
- リクエストボディの読み取り時間を制限
- スローなクライアントから保護

#### 2. WriteTimeout
レスポンスの書き込み開始からレスポンス完了までの最大時間です。

```go
// 【WriteTimeout】サーバーからのレスポンス送信タイムアウト
server := &http.Server{
    WriteTimeout: 10 * time.Second,  // 【重要】レスポンス送信の制限時間
    
    // 【内部動作】：
    // 1. ハンドラー関数の実行開始から測定開始
    // 2. レスポンスヘッダー + レスポンスボディの送信完了まで
    // 3. 制限時間内にクライアントが受信しなければ接続クローズ
    // 4. ハンドラー内の処理時間も含む（重要な注意点）
    //
    // 【保護対象】：
    // - 遅い受信速度のクライアント
    // - 意図的に受信を遅らせる攻撃者
    // - 大きなレスポンスのダウンロード攻撃
    //
    // 【注意点】：
    // - ハンドラー処理時間 + ネットワーク送信時間の合計
    // - 重い処理があるAPIは別途context.WithTimeoutを検討
    // - ストリーミングレスポンスでは特に重要
    //
    // 【設定指針】：
    // - 軽量API: 10-30秒
    // - 重い処理API: 1-5分
    // - ファイルダウンロード: 10-30分
}
```

**用途：**
- レスポンス送信時間を制限
- ネットワークが遅いクライアントから保護

#### 3. IdleTimeout
Keep-Alive接続で次のリクエストを待つ最大時間です。

```go
server := &http.Server{
    IdleTimeout: 60 * time.Second,
}
```

**用途：**
- アイドル接続の自動クローズ
- コネクションプールの効率的な管理

#### 4. ReadHeaderTimeout
リクエストヘッダーの読み取り最大時間です。

```go
server := &http.Server{
    ReadHeaderTimeout: 5 * time.Second,
}
```

**用途：**
- スローロリス攻撃の防止
- ヘッダー読み取りの高速化

### 実装例

基本的なタイムアウト設定：

```go
type ServerConfig struct {
    ReadTimeout       time.Duration
    WriteTimeout      time.Duration
    IdleTimeout       time.Duration
    ReadHeaderTimeout time.Duration
    Port              string
}

func NewServerConfig() *ServerConfig {
    return &ServerConfig{
        ReadTimeout:       10 * time.Second,
        WriteTimeout:      10 * time.Second,
        IdleTimeout:       60 * time.Second,
        ReadHeaderTimeout: 5 * time.Second,
        Port:              ":8080",
    }
}

func NewTimeoutServer(config *ServerConfig) *TimeoutServer {
    server := &http.Server{
        Addr:              config.Port,
        ReadTimeout:       config.ReadTimeout,
        WriteTimeout:      config.WriteTimeout,
        IdleTimeout:       config.IdleTimeout,
        ReadHeaderTimeout: config.ReadHeaderTimeout,
    }
    
    return &TimeoutServer{
        server: server,
        config: config,
    }
}
```

### タイムアウトの適切な設定値

| タイムアウト | 推奨値 | 説明 |
|-------------|--------|------|
| ReadTimeout | 10-30秒 | アップロードサイズに応じて調整 |
| WriteTimeout | 10-30秒 | レスポンスサイズに応じて調整 |
| IdleTimeout | 60-300秒 | Keep-Aliveの効果と切断頻度のバランス |
| ReadHeaderTimeout | 5-10秒 | ヘッダーは通常小さいため短めに設定 |

### コンテキストとの連携

タイムアウトはリクエストコンテキストとも連携します：

```go
func slowHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    select {
    case <-time.After(5 * time.Second):
        // 正常処理
        w.Write([]byte("処理完了"))
    case <-ctx.Done():
        // タイムアウトまたはキャンセル
        http.Error(w, "リクエストがタイムアウトしました", http.StatusRequestTimeout)
    }
}
```

## 📝 課題 (The Problem)

`main_test.go`に書かれているテストをパスするように、以下の機能を実装してください：

1. **ServerConfig構造体**: 各種タイムアウト設定を保持
2. **NewServerConfig関数**: 適切なデフォルト値でConfigを作成
3. **TimeoutServer構造体**: タイムアウト設定されたHTTPサーバー
4. **NewTimeoutServer関数**: 設定を適用したサーバーを作成
5. **ハンドラー実装**: ヘルスチェック、データAPI、スロー処理ハンドラー

### 実装すべき関数

```go
// ServerConfig holds server configuration
type ServerConfig struct {
    ReadTimeout       time.Duration
    WriteTimeout      time.Duration
    IdleTimeout       time.Duration
    ReadHeaderTimeout time.Duration
    Port              string
}

// NewServerConfig creates default server configuration
func NewServerConfig() *ServerConfig

// TimeoutServer represents an HTTP server with proper timeout configuration
type TimeoutServer struct {
    server *http.Server
    config *ServerConfig
}

// NewTimeoutServer creates a new server with timeout configuration
func NewTimeoutServer(config *ServerConfig) *TimeoutServer

// Start starts the server
func (ts *TimeoutServer) Start() error

// Shutdown gracefully shuts down the server
func (ts *TimeoutServer) Shutdown(ctx context.Context) error
```

## ✅ 期待される挙動 (Expected Behavior)

実装が完了すると、以下のような動作が期待されます：

### 1. 正常な起動
```bash
$ go run main_solution.go
Server starting on :8080
ReadTimeout: 10s, WriteTimeout: 10s
IdleTimeout: 60s, ReadHeaderTimeout: 5s
```

### 2. ヘルスチェックの応答
```bash
$ curl http://localhost:8080/health
{
  "status": "healthy",
  "timestamp": 1609459200,
  "timeouts": {
    "read": "10s",
    "write": "10s",
    "idle": "60s",
    "read_header": "5s"
  }
}
```

### 3. スローエンドポイントでのタイムアウト
```bash
$ curl http://localhost:8080/slow?delay=2s
# WriteTimeoutが1秒の場合、タイムアウトエラーが発生
```

### 4. テスト結果
```bash
$ go test -v
=== RUN   TestServerConfig
--- PASS: TestServerConfig (0.00s)
=== RUN   TestTimeoutServer
--- PASS: TestTimeoutServer (0.00s)
=== RUN   TestServerTimeouts
--- PASS: TestServerTimeouts (5.00s)
PASS
```

## 💡 ヒント (Hints)

詰まった場合は、以下のヒントを参考にしてください：

### 1. http.Serverの設定
```go
server := &http.Server{
    Addr:              ":8080",
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      10 * time.Second,
    IdleTimeout:       60 * time.Second,
    ReadHeaderTimeout: 5 * time.Second,
    Handler:           mux,
}
```

### 2. 役立つパッケージ
- `net/http`: HTTPサーバーとタイムアウト設定
- `context`: リクエストコンテキストとタイムアウト
- `time`: タイムアウト値の設定
- `encoding/json`: JSONレスポンスの生成

### 3. テストでのタイムアウト確認
```go
// クライアント側でもタイムアウトを設定
client := &http.Client{
    Timeout: 5 * time.Second,
}

// サーバーのタイムアウトをテスト
resp, err := client.Get("http://localhost:8080/slow")
```

### 4. グレースフルシャットダウン
```go
func (ts *TimeoutServer) Shutdown(ctx context.Context) error {
    return ts.server.Shutdown(ctx)
}
```

### 5. ハンドラーでのコンテキスト使用
```go
func (ts *TimeoutServer) slowHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    // ctx.Done()でタイムアウトを検知
}
```

これらのヒントを参考に、段階的に実装を進めてください。まずは基本的なサーバー設定から始めて、徐々にタイムアウト機能を追加していくのがおすすめです。