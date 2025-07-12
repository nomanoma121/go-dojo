# Day 20: 構造化ロギングミドルウェア

🎯 **本日の目標**
slogを使用した構造化ロギングミドルウェアを実装し、リクエストID、ユーザー情報、応答時間などの詳細なHTTPアクセスログを出力できるようになる。

## 📖 解説

### 構造化ロギングとは

構造化ロギングとは、ログメッセージを事前に定義された構造（通常はJSON形式）で出力するロギング手法です。テキストベースの従来のログと比較して、以下の利点があります：

- **検索・フィルタリングが容易**：JSONフィールドで条件検索可能
- **パースが簡単**：ログ分析ツールで自動解析可能
- **一貫性のある形式**：標準化されたフィールド名とデータ型

### slog パッケージ

Go 1.21で追加された`log/slog`パッケージは、構造化ロギングの標準ライブラリです：

```go
import "log/slog"

// JSON形式でログ出力
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// 構造化ログの出力
logger.Info("User logged in",
    "user_id", "12345",
    "ip_address", "192.168.1.1",
    "timestamp", time.Now())
```

### HTTPミドルウェアでのロギング

Webアプリケーションでは、すべてのHTTPリクエストのログを統一的に記録することが重要です：

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // リクエスト情報をログ
        slog.Info("request started",
            "method", r.Method,
            "url", r.URL.Path,
            "user_agent", r.UserAgent())
            
        next.ServeHTTP(w, r)
        
        // レスポンス情報をログ
        slog.Info("request completed",
            "method", r.Method,
            "url", r.URL.Path,
            "duration", time.Since(start))
    })
}
```

### リクエストIDの生成と追跡

分散システムでは、単一のリクエストを複数のサービス間で追跡できることが重要です：

```go
func generateRequestID() string {
    bytes := make([]byte, 8)
    rand.Read(bytes)
    return hex.EncodeToString(bytes)
}

func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := generateRequestID()
        
        // Contextに保存
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        r = r.WithContext(ctx)
        
        // レスポンスヘッダーに設定
        w.Header().Set("X-Request-ID", requestID)
        
        next.ServeHTTP(w, r)
    })
}
```

### レスポンスライターのラッピング

HTTPレスポンスの詳細（ステータスコード、レスポンスサイズ）をログに記録するには、`http.ResponseWriter`をラップする必要があります：

```go
type responseWriter struct {
    http.ResponseWriter
    statusCode   int
    bytesWritten int64
}

func (rw *responseWriter) WriteHeader(statusCode int) {
    rw.statusCode = statusCode
    rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
    n, err := rw.ResponseWriter.Write(data)
    rw.bytesWritten += int64(n)
    return n, err
}
```

### エラーログとパニックリカバリ

アプリケーションエラーとパニックを適切にログ記録し、アプリケーションの安定性を保つことも重要です：

```go
func ErrorMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                slog.Error("panic recovered",
                    "error", rec,
                    "request_id", r.Context().Value("request_id"))
                http.Error(w, "Internal Server Error", 500)
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

### ログレベルとフィルタリング

本番環境では、ログレベルを適切に設定して、必要な情報のみを出力します：

- **Debug**: 開発時のデバッグ情報
- **Info**: 一般的な情報（リクエストログなど）
- **Warn**: 警告（遅いレスポンスなど）
- **Error**: エラー（4xx/5xxレスポンス、例外など）

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **構造化ロギングの設定**
   - JSON形式でログを出力するslogロガーの設定

2. **リクエストIDミドルウェア**
   - 16文字のランダムなリクエストIDを生成
   - ContextとX-Request-IDヘッダーに設定

3. **ロギングミドルウェア**
   - リクエスト開始と完了をログ記録
   - HTTPメソッド、URL、User-Agent、ステータスコード、レスポンスサイズ、処理時間を含む

4. **レスポンスライターラッピング**
   - ステータスコードとレスポンスサイズをキャプチャ

5. **エラーミドルウェア**
   - パニックをキャッチして500エラーを返す
   - エラー情報をログ記録

6. **ユーザーコンテキストミドルウェア**
   - X-User-IDヘッダーからユーザー情報を取得
   - 未設定の場合は"anonymous"として扱う

## ✅ 期待される挙動

テストが成功すると、以下のような構造化ログが出力されます：

```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "request_start",
  "method": "GET",
  "url": "/api/users",
  "user_agent": "Go-http-client/1.1",
  "request_id": "a1b2c3d4e5f67890",
  "user_id": "user123"
}

{
  "time": "2024-01-15T10:30:00.125Z",
  "level": "INFO", 
  "msg": "request_complete",
  "method": "GET",
  "url": "/api/users",
  "status_code": 200,
  "bytes_written": 1024,
  "duration_ms": 125,
  "request_id": "a1b2c3d4e5f67890",
  "user_id": "user123"
}
```

パニックが発生した場合：

```json
{
  "time": "2024-01-15T10:30:05Z",
  "level": "ERROR",
  "msg": "panic_recovered",
  "error": "simulated panic",
  "request_id": "f6e5d4c3b2a10987"
}
```

## 💡 ヒント

1. **slog.JSONHandler**: JSON形式のログ出力に使用
2. **crypto/rand**: 安全なランダム値生成
3. **encoding/hex**: バイト配列を16進文字列に変換
4. **context.WithValue**: Contextにカスタム値を保存
5. **http.ResponseWriter embedding**: インターフェースを満たしながら機能を拡張
6. **recover()**: パニックをキャッチして回復
7. **time.Since()**: 経過時間の測定

### コンテキストキーの型安全性

Contextキーには専用の型を使用して、キーの衝突を防ぎます：

```go
type contextKey string

const (
    RequestIDKey contextKey = "request_id"
    UserIDKey    contextKey = "user_id"
)

// 使用例
ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
requestID := ctx.Value(RequestIDKey).(string)
```

### ログフィールドの標準化

一貫したログ分析のため、フィールド名は標準化します：

- `request_id`: リクエスト識別子
- `user_id`: ユーザー識別子  
- `method`: HTTPメソッド
- `url`: リクエストURL
- `status_code`: HTTPステータスコード
- `bytes_written`: レスポンスサイズ
- `duration_ms`: 処理時間（ミリ秒）

これらの実装により、プロダクションレベルの構造化ロギングシステムを構築できます。
