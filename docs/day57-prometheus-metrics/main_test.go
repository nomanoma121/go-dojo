package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServiceMetrics_Initialization(t *testing.T) {
	metrics := NewServiceMetrics()
	if metrics == nil {
		t.Skip("ServiceMetrics not implemented yet")
	}

	// メトリクスが正しく初期化されているかテスト
	// TODO: 実装後に各メトリクスのnilチェックを追加
}

func TestPrometheusMiddleware_RequestCounting(t *testing.T) {
	
	// テスト用メトリクスを作成
	metrics := NewServiceMetrics()
	if metrics == nil {
		t.Skip("ServiceMetrics not implemented yet")
	}

	middleware := NewMiddlewareHandler(metrics)
	if middleware == nil {
		t.Skip("MiddlewareHandler not implemented yet")
	}

	// テストハンドラー
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// ミドルウェアを適用
	wrappedHandler := middleware.PrometheusMiddleware(handler)

	// テストリクエストを実行
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rec, req)

	// レスポンスの確認
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// TODO: メトリクスが正しく増加しているかテスト
	// 実装後に以下のようなテストを追加：
	// - httpRequestsTotal が1増加していること
	// - httpRequestDuration が記録されていること
	// - activeConnections が正しく管理されていること
}

func TestPrometheusMiddleware_StatusCodeTracking(t *testing.T) {
	metrics := NewServiceMetrics()
	if metrics == nil {
		t.Skip("ServiceMetrics not implemented yet")
	}

	middleware := NewMiddlewareHandler(metrics)
	if middleware == nil {
		t.Skip("MiddlewareHandler not implemented yet")
	}

	testCases := []struct {
		name       string
		statusCode int
		path       string
		method     string
	}{
		{"Success", http.StatusOK, "/api", "GET"},
		{"Not Found", http.StatusNotFound, "/notfound", "GET"},
		{"Server Error", http.StatusInternalServerError, "/error", "POST"},
		{"Created", http.StatusCreated, "/api", "POST"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte("response"))
			})

			wrappedHandler := middleware.PrometheusMiddleware(handler)

			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, req)

			if rec.Code != tc.statusCode {
				t.Errorf("Expected status %d, got %d", tc.statusCode, rec.Code)
			}

			// TODO: 各ステータスコードが正しくメトリクスに記録されているかテスト
		})
	}
}

func TestBusinessLogicHandler_DatabaseMetrics(t *testing.T) {
	metrics := NewServiceMetrics()
	if metrics == nil {
		t.Skip("ServiceMetrics not implemented yet")
	}

	handler := NewBusinessLogicHandler(metrics)
	if handler == nil {
		t.Skip("BusinessLogicHandler not implemented yet")
	}

	// データベースクエリの模擬実行
	handler.SimulateWork()

	// TODO: データベースクエリメトリクスが更新されているかテスト
	// 実装後に以下のようなテストを追加：
	// - databaseQueriesTotal が増加していること
	// - 成功/失敗のラベルが正しく設定されていること
}

func TestMetricsEndpoint_Output(t *testing.T) {
	metrics := NewServiceMetrics()
	if metrics == nil {
		t.Skip("ServiceMetrics not implemented yet")
	}

	middleware := NewMiddlewareHandler(metrics)
	if middleware == nil {
		t.Skip("MiddlewareHandler not implemented yet")
	}

	// テスト用HTTPサーバーのセットアップ
	mux := http.NewServeMux()
	
	// テストハンドラー
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})
	
	mux.Handle("/test", middleware.PrometheusMiddleware(testHandler))
	// mux.Handle("/metrics", promhttp.Handler()) // TODO: 実装後に有効化

	server := httptest.NewServer(mux)
	defer server.Close()

	// テストリクエストを実行
	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("Failed to make test request: %v", err)
	}
	resp.Body.Close()

	// メトリクスエンドポイントを確認
	resp, err = http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read metrics response: %v", err)
	}

	metricsOutput := string(body)
	
	// メトリクスエンドポイントが期待される形式を返しているかテスト
	if !strings.Contains(metricsOutput, "# HELP") {
		t.Error("Metrics output does not contain HELP comments")
	}

	if !strings.Contains(metricsOutput, "# TYPE") {
		t.Error("Metrics output does not contain TYPE comments")
	}

	// TODO: 実装後に以下のメトリクスが出力されているかテスト：
	// - http_requests_total
	// - http_request_duration_seconds
	// - active_connections
	// - database_queries_total
	// - queue_size
}

func TestResponseWriter_StatusCodeCapture(t *testing.T) {
	rec := httptest.NewRecorder()
	
	// responseWriterの初期化をテスト
	rw := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK, // デフォルト値
	}

	// WriteHeaderのテスト
	rw.WriteHeader(http.StatusCreated)
	
	if rw.statusCode != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, rw.statusCode)
	}

	// 実際のレスポンスライターにも正しく書き込まれているかテスト
	if rec.Code != http.StatusCreated {
		t.Errorf("Expected response code %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestConcurrentRequests_MetricsAccuracy(t *testing.T) {
	metrics := NewServiceMetrics()
	if metrics == nil {
		t.Skip("ServiceMetrics not implemented yet")
	}

	middleware := NewMiddlewareHandler(metrics)
	if middleware == nil {
		t.Skip("MiddlewareHandler not implemented yet")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 短い処理時間を模擬
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	wrappedHandler := middleware.PrometheusMiddleware(handler)

	// 並行リクエストを実行
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, req)
			done <- true
		}()
	}

	// すべてのリクエストの完了を待機
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// TODO: 並行処理時のメトリクス精度をテスト
	// 実装後に以下のようなテストを追加：
	// - httpRequestsTotal が正確に numRequests 回増加していること
	// - レース条件が発生していないこと
	// - アクティブ接続数が正しく管理されていること
}

func TestQueueMetrics_UpdateAccuracy(t *testing.T) {
	metrics := NewServiceMetrics()
	if metrics == nil {
		t.Skip("ServiceMetrics not implemented yet")
	}

	handler := NewBusinessLogicHandler(metrics)
	if handler == nil {
		t.Skip("BusinessLogicHandler not implemented yet")
	}

	// キューメトリクスの更新をテスト
	handler.updateQueueMetrics()

	// TODO: キューサイズメトリクスが正しく更新されているかテスト
	// 実装後に以下のようなテストを追加：
	// - queue_size メトリクスが設定されていること
	// - 複数のキューが正しく識別されていること
}

// ベンチマークテスト
func BenchmarkPrometheusMiddleware(b *testing.B) {
	metrics := NewServiceMetrics()
	if metrics == nil {
		b.Skip("ServiceMetrics not implemented yet")
	}

	middleware := NewMiddlewareHandler(metrics)
	if middleware == nil {
		b.Skip("MiddlewareHandler not implemented yet")
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("benchmark"))
	})

	wrappedHandler := middleware.PrometheusMiddleware(handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/benchmark", nil)
			rec := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(rec, req)
		}
	})
}

func BenchmarkMetricsCollection(b *testing.B) {
	metrics := NewServiceMetrics()
	if metrics == nil {
		b.Skip("ServiceMetrics not implemented yet")
	}

	handler := NewBusinessLogicHandler(metrics)
	if handler == nil {
		b.Skip("BusinessLogicHandler not implemented yet")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			handler.SimulateWork()
		}
	})
}