package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
)

// TODO: ServiceMetrics構造体を実装してください
type ServiceMetrics struct {
	// TODO: 以下のメトリクスを実装
	// - httpRequestsTotal: HTTPリクエスト総数（method, path, statusラベル付き）
	// - httpRequestDuration: HTTPリクエストの処理時間（method, pathラベル付き）
	// - activeConnections: 現在のアクティブ接続数
	// - databaseQueriesTotal: データベースクエリ総数（operation, statusラベル付き）
	// - queueSize: キューサイズ（queueラベル付き）
}

// TODO: NewServiceMetrics関数を実装してください
// Prometheusメトリクスを初期化し、レジストリに登録する
func NewServiceMetrics() *ServiceMetrics {
	// ここに実装
	return nil
}

// TODO: MiddlewareHandler構造体を実装してください
type MiddlewareHandler struct {
	metrics *ServiceMetrics
}

// TODO: NewMiddlewareHandler関数を実装してください
func NewMiddlewareHandler(metrics *ServiceMetrics) *MiddlewareHandler {
	// ここに実装
	return nil
}

// TODO: PrometheusMiddleware関数を実装してください
// HTTPリクエストのメトリクスを収集するミドルウェア
func (m *MiddlewareHandler) PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: 以下の処理を実装
		// 1. リクエスト開始時刻を記録
		// 2. アクティブ接続数を増加
		// 3. レスポンスライターをラップしてステータスコードを取得
		// 4. 次のハンドラーを実行
		// 5. リクエスト完了後にメトリクスを更新
		//    - httpRequestsTotal の増加
		//    - httpRequestDuration の記録
		//    - アクティブ接続数の減少

		next.ServeHTTP(w, r)
	})
}

// TODO: responseWriter構造体を実装してください
// ステータスコードを記録するためのレスポンスライターラッパー
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// TODO: WriteHeader メソッドを実装してください
func (rw *responseWriter) WriteHeader(statusCode int) {
	// ここに実装
}

// TODO: BusinessLogicHandler構造体を実装してください
type BusinessLogicHandler struct {
	metrics *ServiceMetrics
}

// TODO: NewBusinessLogicHandler関数を実装してください
func NewBusinessLogicHandler(metrics *ServiceMetrics) *BusinessLogicHandler {
	// ここに実装
	return nil
}

// TODO: SimulateWork関数を実装してください
// ビジネスロジックを模擬し、対応するメトリクスを更新
func (h *BusinessLogicHandler) SimulateWork() {
	// TODO: 以下の処理を実装
	// 1. データベースクエリの模擬（ランダムな成功/失敗）
	// 2. キューサイズの更新（ランダムな値）
	// 3. 対応するメトリクスの更新
}

// ハンドラー関数群
func (h *BusinessLogicHandler) homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Home Page - Current time: %s\n", time.Now().Format(time.RFC3339))
}

func (h *BusinessLogicHandler) apiHandler(w http.ResponseWriter, r *http.Request) {
	// 模擬的な処理時間
	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	
	// ビジネスロジックの実行
	h.SimulateWork()
	
	// ランダムなエラー発生（10%の確率）
	if rand.Float32() < 0.1 {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error: Random failure occurred\n")
		return
	}
	
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "API Response: Success at %s\n", time.Now().Format(time.RFC3339))
}

func (h *BusinessLogicHandler) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Status: Healthy\n")
}

// TODO: メトリクス更新用のヘルパー関数を実装してください
func (h *BusinessLogicHandler) simulateDatabaseQuery(operation string) {
	// TODO: データベースクエリの模擬と対応するメトリクスの更新
}

func (h *BusinessLogicHandler) updateQueueMetrics() {
	// TODO: キューサイズメトリクスの更新
}

func main() {
	// TODO: ServiceMetricsを初期化

	// TODO: ミドルウェアハンドラーを初期化

	// TODO: ビジネスロジックハンドラーを初期化

	// TODO: HTTPサーバーのセットアップ
	// 1. PrometheusMiddlewareを適用
	// 2. 各エンドポイントのハンドラーを設定
	//    - /: homeHandler
	//    - /api: apiHandler  
	//    - /health: healthHandler
	//    - /metrics: promhttp.Handler()

	// TODO: 定期的なメトリクス更新を開始
	// バックグラウンドでキューサイズなどを定期更新

	// TODO: HTTPサーバーを起動
	log.Println("Server starting on :8080")
	log.Println("Metrics available at http://localhost:8080/metrics")
	
	// ここでサーバーを起動
}