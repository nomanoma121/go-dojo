package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CORSConfig CORS設定を定義
type CORSConfig struct {
	AllowedOrigins   []string
	AllowAllOrigins  bool
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// CORS Cross-Origin Resource Sharing ミドルウェア
type CORS struct {
	config CORSConfig
}

// NewCORS 新しいCORSミドルウェアを作成
func NewCORS(config CORSConfig) *CORS {
	// デフォルト値の設定
	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	
	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization"}
	}
	
	if config.MaxAge == 0 {
		config.MaxAge = 86400 // 24時間
	}
	
	// AllowAllOriginsとAllowCredentialsの組み合わせチェック
	if config.AllowAllOrigins && config.AllowCredentials {
		// セキュリティ上の理由で、すべてのオリジンを許可する場合は認証情報を無効化
		config.AllowCredentials = false
	}
	
	return &CORS{config: config}
}

// isOriginAllowed オリジンが許可されているかチェック
func (cors *CORS) isOriginAllowed(origin string) bool {
	if origin == "" {
		return true // Originヘッダーがない場合は許可
	}
	
	if cors.config.AllowAllOrigins {
		return true
	}
	
	// 大文字小文字を区別しない比較のために小文字に変換
	origin = strings.ToLower(origin)
	
	for _, allowed := range cors.config.AllowedOrigins {
		allowed = strings.ToLower(allowed)
		
		// 完全一致
		if origin == allowed {
			return true
		}
		
		// ワイルドカードサブドメインチェック
		if strings.HasPrefix(allowed, "*.") && matchesWildcard(allowed, origin) {
			return true
		}
	}
	
	return false
}

// isMethodAllowed メソッドが許可されているかチェック
func (cors *CORS) isMethodAllowed(method string) bool {
	method = strings.ToUpper(method)
	
	for _, allowed := range cors.config.AllowedMethods {
		if strings.ToUpper(allowed) == method {
			return true
		}
	}
	
	return false
}

// isHeaderAllowed ヘッダーが許可されているかチェック
func (cors *CORS) isHeaderAllowed(header string) bool {
	header = strings.ToLower(header)
	
	// 危険なヘッダーをブロック
	if isDangerousHeader(header) {
		return false
	}
	
	for _, allowed := range cors.config.AllowedHeaders {
		if strings.ToLower(allowed) == header {
			return true
		}
	}
	
	return false
}

// handlePreflight プリフライトリクエストを処理
func (cors *CORS) handlePreflight(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	
	// Originの検証
	if !cors.isOriginAllowed(origin) {
		cors.sendCORSError(w, http.StatusForbidden, "Origin not allowed", map[string]interface{}{
			"origin": origin,
		})
		return
	}
	
	// リクエストメソッドの検証
	requestMethod := r.Header.Get("Access-Control-Request-Method")
	if requestMethod == "" || !cors.isMethodAllowed(requestMethod) {
		cors.sendCORSError(w, http.StatusMethodNotAllowed, "Method not allowed", map[string]interface{}{
			"method":          requestMethod,
			"allowed_methods": cors.config.AllowedMethods,
		})
		return
	}
	
	// リクエストヘッダーの検証
	requestHeaders := r.Header.Get("Access-Control-Request-Headers")
	if requestHeaders != "" {
		headers := strings.Split(requestHeaders, ",")
		for _, header := range headers {
			header = strings.TrimSpace(header)
			if !cors.isHeaderAllowed(header) {
				cors.sendCORSError(w, http.StatusForbidden, "Header not allowed", map[string]interface{}{
					"header":          header,
					"allowed_headers": cors.config.AllowedHeaders,
				})
				return
			}
		}
	}
	
	// CORSヘッダーの設定
	cors.setCORSHeaders(w, origin)
	cors.setPreflightHeaders(w, origin)
	
	w.WriteHeader(http.StatusOK)
}

// handleSimpleRequest Simple Requestを処理
func (cors *CORS) handleSimpleRequest(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	
	// Originヘッダーがない場合は常に許可
	if origin == "" {
		return true
	}
	
	// Originの検証
	if !cors.isOriginAllowed(origin) {
		cors.sendCORSError(w, http.StatusForbidden, "Origin not allowed", map[string]interface{}{
			"origin": origin,
		})
		return false
	}
	
	// CORSヘッダーの設定
	cors.setCORSHeaders(w, origin)
	
	return true
}

// setCORSHeaders CORSヘッダーを設定
func (cors *CORS) setCORSHeaders(w http.ResponseWriter, origin string) {
	if cors.config.AllowAllOrigins {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		// Varyヘッダーでキャッシュ制御
		w.Header().Set("Vary", "Origin")
	}
	
	if cors.config.AllowCredentials && !cors.config.AllowAllOrigins {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	
	if len(cors.config.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(cors.config.ExposedHeaders, ", "))
	}
}

// setPreflightHeaders プリフライト用ヘッダーを設定
func (cors *CORS) setPreflightHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(cors.config.AllowedMethods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(cors.config.AllowedHeaders, ", "))
	w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cors.config.MaxAge))
}

// sendCORSError CORS エラーレスポンスを送信
func (cors *CORS) sendCORSError(w http.ResponseWriter, code int, message string, details map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	
	response := map[string]interface{}{
		"error":   message,
		"details": details,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Middleware CORSミドルウェア関数
func (cors *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// プリフライトリクエストの処理
		if r.Method == http.MethodOptions {
			origin := r.Header.Get("Origin")
			requestMethod := r.Header.Get("Access-Control-Request-Method")
			
			// プリフライトリクエストかどうかの判定
			if origin != "" && requestMethod != "" {
				cors.handlePreflight(w, r)
				return
			}
		}
		
		// Simple Requestの処理
		if !cors.handleSimpleRequest(w, r) {
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// isSimpleRequest リクエストがSimple Requestかどうか判定
func isSimpleRequest(r *http.Request) bool {
	// Simple Requestのメソッドチェック
	method := r.Method
	if method != "GET" && method != "HEAD" && method != "POST" {
		return false
	}
	
	// Content-Typeのチェック（POSTの場合）
	if method == "POST" {
		contentType := r.Header.Get("Content-Type")
		if contentType != "" {
			// Simple RequestのContent-Type
			simpleContentTypes := []string{
				"application/x-www-form-urlencoded",
				"multipart/form-data",
				"text/plain",
			}
			
			// メディアタイプの抽出（パラメータを除外）
			mediaType := strings.Split(contentType, ";")[0]
			mediaType = strings.TrimSpace(mediaType)
			
			isSimpleContentType := false
			for _, simpleType := range simpleContentTypes {
				if strings.ToLower(mediaType) == simpleType {
					isSimpleContentType = true
					break
				}
			}
			
			if !isSimpleContentType {
				return false
			}
		}
	}
	
	// カスタムヘッダーのチェック
	for headerName := range r.Header {
		headerName = strings.ToLower(headerName)
		
		// Simple Requestで許可されているヘッダー
		simpleHeaders := map[string]bool{
			"accept":          true,
			"accept-language": true,
			"content-language": true,
			"content-type":    true,
		}
		
		if !simpleHeaders[headerName] {
			return false
		}
	}
	
	return true
}

// isDangerousHeader 危険なヘッダーかどうか判定
func isDangerousHeader(header string) bool {
	header = strings.ToLower(header)
	
	dangerousHeaders := map[string]bool{
		"host":               true,
		"connection":         true,
		"upgrade":            true,
		"proxy-authorization": true,
		"sec-websocket-key":   true,
		"sec-websocket-version": true,
		"sec-websocket-protocol": true,
		"sec-websocket-extensions": true,
	}
	
	return dangerousHeaders[header]
}

// matchesWildcard ワイルドカードパターンにマッチするかチェック
func matchesWildcard(pattern, origin string) bool {
	if !strings.HasPrefix(pattern, "*.") {
		return false
	}
	
	domain := pattern[2:] // "*.example.com" → "example.com"
	
	// "https://app.example.com" → "app.example.com"
	// プロトコルを除去してドメイン部分を抽出
	originWithoutProtocol := origin
	if strings.Contains(origin, "://") {
		parts := strings.SplitN(origin, "://", 2)
		if len(parts) == 2 {
			originWithoutProtocol = parts[1]
		}
	}
	
	// ポート番号があれば除去
	if strings.Contains(originWithoutProtocol, ":") {
		parts := strings.Split(originWithoutProtocol, ":")
		originWithoutProtocol = parts[0]
	}
	
	// サブドメインマッチング
	return strings.HasSuffix(originWithoutProtocol, "."+domain)
}

// DefaultCORSConfig デフォルトのCORS設定を返す
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{},
		AllowAllOrigins: false,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "Authorization"},
		ExposedHeaders: []string{},
		AllowCredentials: false,
		MaxAge: 86400, // 24時間
	}
}

// テスト用のサンプルハンドラー
func sampleAPIHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"message":   "API request successful",
			"method":    r.Method,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", "100") // テスト用の追加ヘッダー
		json.NewEncoder(w).Encode(response)
	})
}

func main() {
	// CORS設定
	config := CORSConfig{
		AllowedOrigins: []string{
			"https://example.com",
			"*.trusted.example.com",
			"http://localhost:3000",
			"http://localhost:8080",
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Content-Type", "Authorization", "X-Requested-With", "X-Custom-Header",
		},
		ExposedHeaders: []string{
			"X-Total-Count", "X-Page-Number",
		},
		AllowCredentials: true,
		MaxAge:          86400,
	}

	corsMiddleware := NewCORS(config)
	
	// ハンドラーにCORSミドルウェアを適用
	handler := corsMiddleware.Middleware(sampleAPIHandler())

	http.Handle("/api/data", handler)
	
	// 静的ファイル（テスト用HTML）
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head>
    <title>CORS Test</title>
</head>
<body>
    <h1>CORS Test Page</h1>
    <button onclick="testSimpleRequest()">Test Simple Request</button>
    <button onclick="testPreflightRequest()">Test Preflight Request</button>
    <div id="results"></div>
    
    <script>
        async function testSimpleRequest() {
            try {
                const response = await fetch('/api/data');
                const data = await response.json();
                document.getElementById('results').innerHTML += '<p>Simple Request: ' + JSON.stringify(data) + '</p>';
            } catch (error) {
                document.getElementById('results').innerHTML += '<p>Simple Request Error: ' + error.message + '</p>';
            }
        }
        
        async function testPreflightRequest() {
            try {
                const response = await fetch('/api/data', {
                    method: 'PUT',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Custom-Header': 'test-value'
                    },
                    body: JSON.stringify({test: 'data'})
                });
                const data = await response.json();
                document.getElementById('results').innerHTML += '<p>Preflight Request: ' + JSON.stringify(data) + '</p>';
            } catch (error) {
                document.getElementById('results').innerHTML += '<p>Preflight Request Error: ' + error.message + '</p>';
            }
        }
    </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	// サーバー起動
	println("Server starting on :8080")
	println("Test page: http://localhost:8080/")
	println("API endpoint: http://localhost:8080/api/data")
	
	http.ListenAndServe(":8080", nil)
}