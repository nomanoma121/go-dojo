# Day 22: パニックリカバリミドルウェア

🎯 **本日の目標**
ハンドラ内で発生したパニックを捕捉し、アプリケーションクラッシュを防ぐリカバリミドルウェアを実装し、安定性の高いWebアプリケーションの構築方法を学ぶ。

## 📖 解説

### パニックとは

Goにおけるパニックは、プログラムが回復不可能なエラー状態に陥った際に発生する実行時エラーです。パニックが発生すると、通常はプログラム全体が停止してしまいます。

```go
// パニックを発生させる例
func riskyFunction() {
    panic("Something went wrong!")
}

// 配列の範囲外アクセスもパニックの原因
func outOfBounds() {
    slice := []int{1, 2, 3}
    _ = slice[10] // panic: runtime error: index out of range
}
```

### recover()によるパニック捕捉

Goの`recover()`関数を使用することで、パニックを捕捉し、プログラムの実行を継続できます：

```go
func safeFunction() {
    defer func() {
        if r := recover(); r != nil {
            fmt.Printf("Recovered from panic: %v\n", r)
        }
    }()
    
    panic("This will be caught!")
    fmt.Println("This won't be printed")
}
```

### HTTPミドルウェアでのパニックリカバリ

Webアプリケーションでは、個々のHTTPリクエストでパニックが発生しても、サーバー全体が停止しないようにすることが重要です：

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                // パニックをログに記録
                log.Printf("Panic recovered: %v", err)
                
                // クライアントに500エラーを返す
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

### スタックトレースの取得

デバッグのために、パニック発生時のスタックトレースを記録することが重要です：

```go
import (
    "runtime/debug"
)

func RecoveryWithStackTrace(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                // スタックトレースを取得
                stack := debug.Stack()
                
                log.Printf("Panic recovered: %v\nStack trace:\n%s", err, stack)
                
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

### 構造化ログでのパニック記録

`slog`を使用して、パニック情報を構造化形式で記録：

```go
func (rm *RecoveryMiddleware) Recover(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                requestID := getRequestID(r.Context())
                
                rm.logger.Error("panic recovered",
                    "error", err,
                    "request_id", requestID,
                    "method", r.Method,
                    "url", r.URL.String(),
                    "user_agent", r.UserAgent(),
                    "stack_trace", string(debug.Stack()),
                )
                
                // JSONエラーレスポンス
                rm.sendErrorResponse(w, http.StatusInternalServerError, "Internal Server Error")
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

### エラーレスポンスの統一

パニック発生時のエラーレスポンスを統一的に処理：

```go
type ErrorResponse struct {
    Error     string `json:"error"`
    Message   string `json:"message"`
    Timestamp int64  `json:"timestamp"`
    RequestID string `json:"request_id,omitempty"`
}

func (rm *RecoveryMiddleware) sendErrorResponse(w http.ResponseWriter, code int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    
    response := ErrorResponse{
        Error:     http.StatusText(code),
        Message:   message,
        Timestamp: time.Now().Unix(),
    }
    
    json.NewEncoder(w).Encode(response)
}
```

### パニック発生パターンの分類

一般的なパニック発生パターンと対策：

#### 1. Null Pointer Dereference
```go
var user *User
name := user.Name // panic: runtime error: invalid memory address
```

#### 2. Type Assertion Failed
```go
var val interface{} = "string"
num := val.(int) // panic: interface conversion
```

#### 3. Channel Operations
```go
ch := make(chan int)
close(ch)
ch <- 1 // panic: send on closed channel
```

#### 4. Slice/Map Access
```go
slice := []int{1, 2, 3}
val := slice[10] // panic: index out of range
```

### 本番環境での考慮事項

1. **セキュリティ**: スタックトレースはログにのみ記録し、クライアントには送信しない
2. **監視**: パニック発生率の監視とアラート設定
3. **ログレベル**: パニックは常にERRORレベルでログ記録
4. **メトリクス**: パニック発生回数のメトリクス収集

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、以下の機能を実装してください：

1. **RecoveryMiddleware構造体**
   - 構造化ログ用のloggerを保持
   - 設定可能なオプション

2. **パニックリカバリ機能**
   - defer/recover パターンでパニックを捕捉
   - 500 Internal Server Errorを返却
   - アプリケーションの継続実行を保証

3. **詳細ログ記録**
   - パニック内容の記録
   - リクエスト情報（URL、メソッド、User-Agent）
   - スタックトレースの取得と記録
   - リクエストIDがある場合は含める

4. **エラーレスポンス**
   - JSON形式での統一されたエラーレスポンス
   - タイムスタンプとリクエストIDを含む
   - セキュアな情報のみクライアントに送信

5. **異なるパニックタイプの処理**
   - 文字列パニック
   - error型パニック
   - その他の型のパニック

6. **設定可能な動作**
   - デバッグモードでのスタックトレース表示制御
   - カスタムエラーメッセージ
   - ログレベルの調整

## ✅ 期待される挙動

### パニック発生時のログ出力：
```json
{
  "time": "2024-01-15T10:30:05Z",
  "level": "ERROR",
  "msg": "panic recovered",
  "error": "division by zero",
  "request_id": "req_123456",
  "method": "GET",
  "url": "/api/calculate",
  "user_agent": "curl/7.68.0",
  "stack_trace": "goroutine 1 [running]:\n..."
}
```

### クライアントへのエラーレスポンス：
```json
{
  "error": "Internal Server Error",
  "message": "An internal error occurred",
  "timestamp": 1705317005,
  "request_id": "req_123456"
}
```

### 正常継続の確認：
パニック発生後も他のリクエストが正常に処理されることを確認できます。

## 💡 ヒント

1. **defer文**: 必ずdeferで実行されるrecover処理
2. **runtime/debug.Stack()**: スタックトレースの取得
3. **type assertion**: パニック値の型に応じた処理
4. **slog.Error()**: 構造化エラーログの出力
5. **http.StatusInternalServerError**: 500エラーの定数
6. **json.NewEncoder()**: JSONレスポンスの生成

### パニック処理のベストプラクティス

```go
defer func() {
    if r := recover(); r != nil {
        // 1. パニック値の型を確認
        var err string
        switch v := r.(type) {
        case error:
            err = v.Error()
        case string:
            err = v
        default:
            err = fmt.Sprintf("%v", v)
        }
        
        // 2. 詳細ログ記録
        logger.Error("panic recovered", 
            "error", err,
            "stack", string(debug.Stack()))
        
        // 3. セキュアなレスポンス
        sendErrorResponse(w, 500, "Internal Server Error")
    }
}()
```

### テスト時の注意点

- パニックリカバリのテストでは、実際にパニックを発生させる
- レスポンスコードとレスポンス内容の両方を検証
- ログ出力の内容も検証対象に含める
- 複数のパニックパターンをテスト

これらの実装により、障害に強い本番レベルのWebアプリケーションを構築できます。