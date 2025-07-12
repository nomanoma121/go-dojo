# Day 24: セキュアなCORS設定

🎯 **本日の目標**
Cross-Origin Resource Sharing (CORS) の安全な設定を実装し、ブラウザベースのWebアプリケーションでのAPIアクセス制御を学ぶ。

## 📖 解説

### CORS の基礎知識

CORS（Cross-Origin Resource Sharing）は、Webブラウザが実装するセキュリティ機能で、異なるオリジン（ドメイン、プロトコル、ポート）間でのリソース共有を制御します。

#### Same-Origin Policy

ブラウザはデフォルトで「同一オリジンポリシー」を適用し、異なるオリジンからのリクエストをブロックします：

```javascript
// https://example.com から実行される JavaScript
fetch('https://api.other-domain.com/data') // ブロックされる
```

#### CORS ヘッダーによる許可

サーバーは適切なCORSヘッダーを送信することで、特定のクロスオリジンリクエストを許可できます：

```go
func corsHandler(w http.ResponseWriter, r *http.Request) {
    // 特定のオリジンを許可
    w.Header().Set("Access-Control-Allow-Origin", "https://trusted-domain.com")
    
    // 許可するHTTPメソッド
    w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
    
    // 許可するヘッダー
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
    
    // 認証情報の送信を許可
    w.Header().Set("Access-Control-Allow-Credentials", "true")
    
    // プリフライトキャッシュ時間
    w.Header().Set("Access-Control-Max-Age", "3600")
}
```

### CORS のリクエストタイプ

#### Simple Requests

以下の条件を満たすリクエストは「Simple Request」として直接送信されます：

- メソッド: GET, HEAD, POST
- ヘッダー: 標準的なヘッダーのみ
- Content-Type: application/x-www-form-urlencoded, multipart/form-data, text/plain

```go
// Simple Request の例
fetch('https://api.example.com/data', {
    method: 'GET',
    headers: {
        'Content-Type': 'text/plain'
    }
})
```

#### Preflight Requests

Simple Request の条件を満たさない場合、ブラウザは事前にOPTIONSリクエストを送信します：

```http
OPTIONS /api/data HTTP/1.1
Host: api.example.com
Origin: https://webapp.example.com
Access-Control-Request-Method: PUT
Access-Control-Request-Headers: Content-Type, X-Custom-Header
```

サーバーの応答例：

```http
HTTP/1.1 200 OK
Access-Control-Allow-Origin: https://webapp.example.com
Access-Control-Allow-Methods: GET, POST, PUT, DELETE
Access-Control-Allow-Headers: Content-Type, X-Custom-Header
Access-Control-Max-Age: 86400
```

### セキュリティ考慮事項

#### 1. Origin検証の重要性

```go
type CORSConfig struct {
    AllowedOrigins     []string
    AllowAllOrigins    bool // 危険：本番環境では使用禁止
    AllowedMethods     []string
    AllowedHeaders     []string
    ExposedHeaders     []string
    AllowCredentials   bool
    MaxAge             int
}

func (c *CORSConfig) isOriginAllowed(origin string) bool {
    if c.AllowAllOrigins {
        return true // 危険
    }
    
    for _, allowed := range c.AllowedOrigins {
        if origin == allowed {
            return true
        }
        
        // ワイルドカードサブドメインのサポート
        if strings.HasPrefix(allowed, "*.") {
            domain := allowed[2:]
            if strings.HasSuffix(origin, "."+domain) {
                return true
            }
        }
    }
    
    return false
}
```

#### 2. 認証情報付きリクエストの制限

```go
func (cors *CORS) handleCredentials(w http.ResponseWriter, origin string) {
    if cors.config.AllowCredentials {
        // 認証情報を許可する場合、Origin を明示的に指定
        w.Header().Set("Access-Control-Allow-Origin", origin)
        w.Header().Set("Access-Control-Allow-Credentials", "true")
        
        // Vary ヘッダーでキャッシュ制御
        w.Header().Set("Vary", "Origin")
    }
}
```

#### 3. ヘッダーの適切な制限

```go
var dangerousHeaders = map[string]bool{
    "host":               true,
    "connection":         true,
    "upgrade":            true,
    "proxy-authorization": true,
}

func (cors *CORS) isHeaderAllowed(header string) bool {
    header = strings.ToLower(header)
    
    // 危険なヘッダーを拒否
    if dangerousHeaders[header] {
        return false
    }
    
    // 許可リストをチェック
    for _, allowed := range cors.config.AllowedHeaders {
        if strings.ToLower(allowed) == header {
            return true
        }
    }
    
    return false
}
```

### プリフライト最適化

#### キャッシュ戦略

```go
func (cors *CORS) setPreflightCache(w http.ResponseWriter) {
    // 適切なキャッシュ時間を設定（1時間〜24時間）
    maxAge := strconv.Itoa(cors.config.MaxAge)
    w.Header().Set("Access-Control-Max-Age", maxAge)
    
    // プロキシでのキャッシュも制御
    w.Header().Set("Cache-Control", "public, max-age="+maxAge)
}
```

#### 動的Origin許可

```go
func (cors *CORS) checkDynamicOrigin(origin string) bool {
    // 開発環境での動的許可
    if cors.isDevelopment() && strings.HasPrefix(origin, "http://localhost:") {
        return true
    }
    
    // データベースからの動的許可リスト
    return cors.isOriginInDatabase(origin)
}
```

### 高度なCORS設定

#### 条件付きCORS

```go
func (cors *CORS) ConditionalMiddleware(condition func(*http.Request) bool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if condition(r) {
                cors.Middleware(next).ServeHTTP(w, r)
            } else {
                next.ServeHTTP(w, r)
            }
        })
    }
}

// 使用例：API エンドポイントのみに CORS を適用
apiOnlyCORS := cors.ConditionalMiddleware(func(r *http.Request) bool {
    return strings.HasPrefix(r.URL.Path, "/api/")
})
```

#### ルート別CORS設定

```go
type RouteCORSConfig struct {
    Path       string
    CORSConfig CORSConfig
}

func (cors *CORS) RouteSpecificMiddleware(routes []RouteCORSConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            for _, route := range routes {
                if strings.HasPrefix(r.URL.Path, route.Path) {
                    corsHandler := NewCORS(route.CORSConfig)
                    corsHandler.Middleware(next).ServeHTTP(w, r)
                    return
                }
            }
            
            // デフォルトCORS設定
            cors.Middleware(next).ServeHTTP(w, r)
        })
    }
}
```

### セキュリティベストプラクティス

1. **最小権限の原則**: 必要最小限のオリジン、メソッド、ヘッダーのみ許可
2. **ワイルドカードの制限**: `*` の使用は認証情報なしの場合のみ
3. **HTTPS強制**: 本番環境では HTTPS オリジンのみ許可
4. **定期的な監査**: 許可されたオリジンの定期見直し
5. **ログ記録**: CORS エラーのログ収集と分析

### テスト戦略

```go
func TestCORS(t *testing.T) {
    tests := []struct {
        name           string
        origin         string
        method         string
        headers        map[string]string
        expectedStatus int
        expectedCORS   map[string]string
    }{
        {
            name:   "Allowed origin",
            origin: "https://trusted.example.com",
            method: "GET",
            expectedStatus: 200,
            expectedCORS: map[string]string{
                "Access-Control-Allow-Origin": "https://trusted.example.com",
            },
        },
        {
            name:   "Blocked origin",
            origin: "https://malicious.com",
            method: "GET",
            expectedStatus: 403,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // テスト実装
        })
    }
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **CORS構造体**
   - 設定可能なオリジン、メソッド、ヘッダーリスト
   - 認証情報許可フラグ
   - プリフライトキャッシュ時間

2. **Origin検証機能**
   - 許可されたオリジンとのマッチング
   - ワイルドカードサブドメインサポート
   - 大文字小文字を区別しない比較

3. **プリフライト処理**
   - OPTIONSリクエストの適切な処理
   - リクエストメソッドとヘッダーの検証
   - 適切なCORSヘッダーの設定

4. **Simple Request処理**
   - GETやPOSTリクエストの処理
   - Origin検証とヘッダー設定
   - エラー時の適切な応答

5. **セキュリティ機能**
   - 認証情報付きリクエストの制限
   - 危険なヘッダーのブロック
   - 不正なオリジンの拒否

6. **設定管理**
   - 柔軟な CORS 設定
   - 開発・本番環境の切り替え
   - ルート別設定サポート

## ✅ 期待される挙動

### 成功パターン

#### 許可されたオリジンからのSimple Request：
```bash
curl -H "Origin: https://trusted.example.com" http://localhost:8080/api/data
```
```http
HTTP/1.1 200 OK
Access-Control-Allow-Origin: https://trusted.example.com
Access-Control-Allow-Credentials: true
Vary: Origin
Content-Type: application/json

{
  "data": "success",
  "timestamp": "2023-12-31T23:59:59Z"
}
```

#### プリフライトリクエスト：
```bash
curl -X OPTIONS \
     -H "Origin: https://trusted.example.com" \
     -H "Access-Control-Request-Method: PUT" \
     -H "Access-Control-Request-Headers: Content-Type, X-Custom-Header" \
     http://localhost:8080/api/data
```
```http
HTTP/1.1 200 OK
Access-Control-Allow-Origin: https://trusted.example.com
Access-Control-Allow-Methods: GET, POST, PUT, DELETE
Access-Control-Allow-Headers: Content-Type, X-Custom-Header, Authorization
Access-Control-Max-Age: 86400
Access-Control-Allow-Credentials: true
Vary: Origin
```

#### ワイルドカードサブドメイン：
```bash
curl -H "Origin: https://app.trusted.example.com" http://localhost:8080/api/data
```
許可設定 `*.trusted.example.com` で許可される

### エラーパターン

#### 許可されていないオリジン（403 Forbidden）：
```bash
curl -H "Origin: https://malicious.com" http://localhost:8080/api/data
```
```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": "Origin not allowed",
  "origin": "https://malicious.com"
}
```

#### 許可されていないメソッド（405 Method Not Allowed）：
```bash
curl -X OPTIONS \
     -H "Origin: https://trusted.example.com" \
     -H "Access-Control-Request-Method: PATCH" \
     http://localhost:8080/api/data
```
```http
HTTP/1.1 405 Method Not Allowed
Content-Type: application/json

{
  "error": "Method not allowed",
  "method": "PATCH",
  "allowed_methods": ["GET", "POST", "PUT", "DELETE"]
}
```

## 💡 ヒント

1. **strings.HasPrefix/HasSuffix**: ワイルドカードマッチング
2. **http.MethodOptions**: プリフライトリクエストの検出
3. **r.Header.Get("Origin")**: オリジンヘッダーの取得
4. **strings.ToLower()**: 大文字小文字を区別しない比較
5. **http.StatusForbidden**: オリジン拒否時のステータス
6. **Vary: Origin**: キャッシュ制御ヘッダー

### CORS設定例

```go
config := CORSConfig{
    AllowedOrigins: []string{
        "https://example.com",
        "*.trusted.example.com",
        "http://localhost:3000", // 開発環境
    },
    AllowedMethods: []string{
        "GET", "POST", "PUT", "DELETE", "OPTIONS",
    },
    AllowedHeaders: []string{
        "Content-Type", "Authorization", "X-Requested-With",
    },
    ExposedHeaders: []string{
        "X-Total-Count", "X-Page-Number",
    },
    AllowCredentials: true,
    MaxAge:          86400, // 24時間
}
```

### セキュリティチェックリスト

- [ ] Origin の厳密な検証
- [ ] 認証情報付きリクエストでのワイルドカード禁止
- [ ] 危険なヘッダーのブロック
- [ ] プリフライトキャッシュの適切な設定
- [ ] HTTPS 環境での設定確認

### ブラウザテスト例

```html
<!DOCTYPE html>
<html>
<head>
    <title>CORS Test</title>
</head>
<body>
    <script>
        // Simple Request
        fetch('http://localhost:8080/api/data', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            }
        })
        .then(response => response.json())
        .then(data => console.log('Simple request:', data));

        // Preflight Request
        fetch('http://localhost:8080/api/data', {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'X-Custom-Header': 'value'
            },
            body: JSON.stringify({test: 'data'})
        })
        .then(response => response.json())
        .then(data => console.log('Preflight request:', data));
    </script>
</body>
</html>
```

これらの実装により、セキュアで柔軟なCORS制御システムの基礎を学ぶことができます。