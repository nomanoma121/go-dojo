# Day 26: ミドルウェアチェインのテスト

🎯 **本日の目標**
複数のHTTPミドルウェアが連鎖して動作するシステムの包括的なテスト戦略を学び、ミドルウェアチェインの統合テストと単体テストを実装できるようになる。

## 📖 解説

### ミドルウェアチェインとは

HTTPミドルウェアチェインは、リクエストが最終的なハンドラーに到達するまでに通過する一連のミドルウェア関数です。各ミドルウェアは、リクエストを処理し、次のミドルウェアまたはハンドラーに制御を渡します：

```go
// ミドルウェアチェインの例
func createChain() http.Handler {
    handler := finalHandler()
    handler = authMiddleware(handler)
    handler = loggingMiddleware(handler)
    handler = corsMiddleware(handler)
    return handler
}
```

### ミドルウェアテストの課題

ミドルウェアチェインのテストでは以下の課題があります：

1. **実行順序の検証**：ミドルウェアが正しい順序で実行されるか
2. **状態の伝播**：Contextや値が適切に次のミドルウェアに渡されるか
3. **エラーハンドリング**：エラーが適切に処理され、チェインが停止されるか
4. **副作用の検証**：ログ出力、メトリクス更新などの副作用が発生するか

### 単体テストパターン

個別のミドルウェアの単体テストでは、モックやスパイを使用して依存関係を分離します：

```go
func TestLoggingMiddleware(t *testing.T) {
    var logBuffer bytes.Buffer
    logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
    
    middleware := LoggingMiddleware(logger)
    
    req := httptest.NewRequest("GET", "/test", nil)
    rr := httptest.NewRecorder()
    
    handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    }))
    
    handler.ServeHTTP(rr, req)
    
    // ログ出力の検証
    assert.Contains(t, logBuffer.String(), "request_start")
    assert.Contains(t, logBuffer.String(), "request_complete")
}
```

### 統合テストパターン

ミドルウェアチェイン全体の統合テストでは、実際の HTTP サーバーを起動してエンドツーエンドのテストを行います：

```go
func TestMiddlewareChain(t *testing.T) {
    server := httptest.NewServer(createChain())
    defer server.Close()
    
    resp, err := http.Get(server.URL + "/api/test")
    require.NoError(t, err)
    defer resp.Body.Close()
    
    // チェイン全体の動作を検証
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
    assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))
}
```

### テスト用ヘルパー関数

テストの可読性と再利用性を向上させるため、専用のヘルパー関数を作成します：

```go
// テスト用のレスポンスライター
type testResponseWriter struct {
    *httptest.ResponseRecorder
    headers    map[string]string
    statusCode int
}

func newTestResponseWriter() *testResponseWriter {
    return &testResponseWriter{
        ResponseRecorder: httptest.NewRecorder(),
        headers:          make(map[string]string),
    }
}

// ミドルウェアテスト用のヘルパー
func testMiddleware(t *testing.T, middleware func(http.Handler) http.Handler, req *http.Request) *httptest.ResponseRecorder {
    rr := httptest.NewRecorder()
    handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("test response"))
    }))
    
    handler.ServeHTTP(rr, req)
    return rr
}
```

### コンテキストの検証

ミドルウェアチェインでContextに設定された値が適切に伝播されることを検証します：

```go
func TestContextPropagation(t *testing.T) {
    var receivedRequestID string
    var receivedUserID string
    
    finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        receivedRequestID = getRequestID(r.Context())
        receivedUserID = getUserID(r.Context())
        w.WriteHeader(http.StatusOK)
    })
    
    chain := requestIDMiddleware(authMiddleware(finalHandler))
    
    req := httptest.NewRequest("GET", "/test", nil)
    req.Header.Set("Authorization", "Bearer valid-token")
    
    rr := httptest.NewRecorder()
    chain.ServeHTTP(rr, req)
    
    assert.NotEmpty(t, receivedRequestID)
    assert.Equal(t, "test-user", receivedUserID)
}
```

### エラーケーステスト

ミドルウェアチェインでエラーが発生した場合の動作を検証します：

```go
func TestErrorHandling(t *testing.T) {
    tests := []struct {
        name           string
        setupRequest   func() *http.Request
        expectedStatus int
        expectedBody   string
    }{
        {
            name: "missing auth header",
            setupRequest: func() *http.Request {
                return httptest.NewRequest("GET", "/protected", nil)
            },
            expectedStatus: http.StatusUnauthorized,
            expectedBody:   "Unauthorized",
        },
        {
            name: "invalid auth token",
            setupRequest: func() *http.Request {
                req := httptest.NewRequest("GET", "/protected", nil)
                req.Header.Set("Authorization", "Bearer invalid")
                return req
            },
            expectedStatus: http.StatusUnauthorized,
            expectedBody:   "Invalid token",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := tt.setupRequest()
            rr := httptest.NewRecorder()
            
            chain := authMiddleware(protectedHandler())
            chain.ServeHTTP(rr, req)
            
            assert.Equal(t, tt.expectedStatus, rr.Code)
            assert.Contains(t, rr.Body.String(), tt.expectedBody)
        })
    }
}
```

### パフォーマンステスト

ミドルウェアチェインの性能を測定し、リグレッションを防ぎます：

```go
func BenchmarkMiddlewareChain(b *testing.B) {
    handler := createFullChain()
    req := httptest.NewRequest("GET", "/api/test", nil)
    req.Header.Set("Authorization", "Bearer valid-token")
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        rr := httptest.NewRecorder()
        handler.ServeHTTP(rr, req)
    }
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **ミドルウェア実装**
   - CORS ミドルウェア（Origin、Methods、Headersの設定）
   - 認証ミドルウェア（Bearer トークンの検証）
   - ロギングミドルウェア（リクエスト/レスポンス情報の記録）
   - レート制限ミドルウェア（IP別のリクエスト制限）

2. **ミドルウェアチェイン**
   - 複数のミドルウェアを正しい順序で組み合わせ
   - Context値の適切な伝播

3. **テストヘルパー**
   - ミドルウェアテスト用のヘルパー関数
   - レスポンス検証用のユーティリティ

4. **エラーハンドリング**
   - 認証失敗の適切な処理
   - レート制限超過の処理
   - パニックリカバリ

## ✅ 期待される挙動

### 成功ケース
```bash
GET /api/data
Authorization: Bearer valid-token
Origin: https://example.com

Response:
Status: 200 OK
Access-Control-Allow-Origin: https://example.com
Access-Control-Allow-Methods: GET, POST, PUT, DELETE
X-Request-ID: abc123def456
Content-Type: application/json

{"data": "success"}
```

### エラーケース
```bash
GET /api/data
# Authorization header missing

Response:
Status: 401 Unauthorized
Content-Type: application/json

{"error": "missing authorization header"}
```

### ログ出力例
```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "request_start",
  "method": "GET",
  "url": "/api/data",
  "request_id": "abc123def456",
  "user_agent": "Go-http-client/1.1"
}

{
  "time": "2024-01-15T10:30:00.050Z",
  "level": "INFO",
  "msg": "request_complete", 
  "method": "GET",
  "url": "/api/data",
  "status_code": 200,
  "duration_ms": 50,
  "request_id": "abc123def456"
}
```

## 💡 ヒント

1. **ミドルウェア順序**: CORS → ログ → 認証 → レート制限 → ハンドラー
2. **httptest パッケージ**: HTTPテスト用のユーティリティ
3. **testify/assert**: テストアサーション用ライブラリ
4. **sync/atomic**: スレッドセーフなカウンター実装
5. **context.WithValue**: Context値の設定と取得
6. **time.Since()**: 処理時間の測定
7. **bytes.Buffer**: ログ出力のキャプチャ

### テスト実行順序の制御

テスト間の依存関係を避けるため、各テストは独立して実行できるように設計します：

```go
func TestIndependentMiddleware(t *testing.T) {
    // テストごとに新しいインスタンスを作成
    rateLimiter := NewRateLimiter(10, time.Minute)
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    
    middleware := RateLimitMiddleware(rateLimiter)
    // テスト実行...
}
```

### ベンチマークの最適化

性能測定では、テスト準備コストを除外します：

```go
func BenchmarkChainWithAuth(b *testing.B) {
    handler := createChain()
    req := httptest.NewRequest("GET", "/api/test", nil)
    req.Header.Set("Authorization", "Bearer "+generateValidToken())
    
    b.ResetTimer() // ここでタイマーをリセット
    b.ReportAllocs() // メモリアロケーションも測定
    
    for i := 0; i < b.N; i++ {
        rr := httptest.NewRecorder()
        handler.ServeHTTP(rr, req)
    }
}
```

これらの実装により、プロダクション環境で使用されるミドルウェアチェインの品質を保証するテストシステムを構築できます。