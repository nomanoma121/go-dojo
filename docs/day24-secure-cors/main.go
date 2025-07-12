//go:build ignore

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
	// TODO: CORS構造体を初期化
	// - 設定のバリデーション
	// - デフォルト値の設定
	return nil
}

// isOriginAllowed オリジンが許可されているかチェック
func (cors *CORS) isOriginAllowed(origin string) bool {
	// TODO: オリジン検証の実装
	// 1. AllowAllOriginsの場合はtrueを返す
	// 2. 許可されたオリジンリストをチェック
	// 3. ワイルドカードサブドメインのサポート
	// 4. 大文字小文字を区別しない比較
	return false
}

// isMethodAllowed メソッドが許可されているかチェック
func (cors *CORS) isMethodAllowed(method string) bool {
	// TODO: メソッド検証の実装
	return false
}

// isHeaderAllowed ヘッダーが許可されているかチェック
func (cors *CORS) isHeaderAllowed(header string) bool {
	// TODO: ヘッダー検証の実装
	// 1. 危険なヘッダーのブロック
	// 2. 許可されたヘッダーリストのチェック
	// 3. 大文字小文字を区別しない比較
	return false
}

// handlePreflight プリフライトリクエストを処理
func (cors *CORS) handlePreflight(w http.ResponseWriter, r *http.Request) {
	// TODO: プリフライト処理の実装
	// 1. Originの検証
	// 2. リクエストメソッドの検証
	// 3. リクエストヘッダーの検証
	// 4. 適切なCORSヘッダーの設定
	// 5. ステータスコードの設定
}

// handleSimpleRequest Simple Requestを処理
func (cors *CORS) handleSimpleRequest(w http.ResponseWriter, r *http.Request) bool {
	// TODO: Simple Request処理の実装
	// 1. Originの検証
	// 2. 適切なCORSヘッダーの設定
	// 3. 成功/失敗の判定を返す
	return false
}

// setCORSHeaders CORSヘッダーを設定
func (cors *CORS) setCORSHeaders(w http.ResponseWriter, origin string) {
	// TODO: CORSヘッダーの設定
	// - Access-Control-Allow-Origin
	// - Access-Control-Allow-Credentials
	// - Access-Control-Expose-Headers
	// - Vary
}

// setPreflightHeaders プリフライト用ヘッダーを設定
func (cors *CORS) setPreflightHeaders(w http.ResponseWriter, origin string) {
	// TODO: プリフライト用ヘッダーの設定
	// - Access-Control-Allow-Methods
	// - Access-Control-Allow-Headers
	// - Access-Control-Max-Age
}

// sendCORSError CORS エラーレスポンスを送信
func (cors *CORS) sendCORSError(w http.ResponseWriter, code int, message string, details map[string]interface{}) {
	// TODO: エラーレスポンスの送信
	// - 適切なステータスコード
	// - JSON形式のエラーメッセージ
	// - 追加の詳細情報
}

// Middleware CORSミドルウェア関数
func (cors *CORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: メインのミドルウェア処理
		// 1. プリフライトリクエストの検出と処理
		// 2. Simple Requestの処理
		// 3. 次のハンドラーへの処理継続
		next.ServeHTTP(w, r)
	})
}

// isSimpleRequest リクエストがSimple Requestかどうか判定
func isSimpleRequest(r *http.Request) bool {
	// TODO: Simple Request判定の実装
	// - メソッドチェック (GET, HEAD, POST)
	// - Content-Typeチェック
	// - カスタムヘッダーの有無
	return false
}

// isDangerousHeader 危険なヘッダーかどうか判定
func isDangerousHeader(header string) bool {
	// TODO: 危険なヘッダーの判定
	// - host, connection, upgrade など
	return false
}

// matchesWildcard ワイルドカードパターンにマッチするかチェック
func matchesWildcard(pattern, origin string) bool {
	// TODO: ワイルドカードマッチングの実装
	// - *.example.com パターンのサポート
	return false
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