package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestConditionalMiddleware_BasicRules(t *testing.T) {
	cm := NewConditionalMiddleware()
	
	// テスト用のミドルウェア
	testMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "applied")
			next.ServeHTTP(w, r)
		})
	}
	
	// パスベースのルールを追加
	cm.AddRule(MiddlewareRule{
		Name:       "api_test",
		Condition:  PathMatches("^/api/"),
		Middleware: testMiddleware,
		Priority:   100,
		Enabled:    true,
	})
	
	// テストハンドラー
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := cm.Apply(handler)
	
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"API path should apply middleware", "/api/users", "applied"},
		{"Non-API path should not apply middleware", "/users", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			
			wrappedHandler.ServeHTTP(w, req)
			
			result := w.Header().Get("X-Test")
			if result != tt.expected {
				t.Errorf("Expected X-Test header to be '%s', got '%s'", tt.expected, result)
			}
		})
	}
	
	t.Log("Path-based rule applied correctly")
}

func TestConditionalMiddleware_Priority(t *testing.T) {
	cm := NewConditionalMiddleware()
	
	var executionOrder []string
	mu := sync.Mutex{}
	
	// 優先度の異なるミドルウェアを作成
	createMiddleware := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				executionOrder = append(executionOrder, name)
				mu.Unlock()
				next.ServeHTTP(w, r)
			})
		}
	}
	
	// ルールを追加（意図的に順序を混在）
	cm.AddRule(MiddlewareRule{
		Name:       "low_priority",
		Condition:  func(r *http.Request) bool { return true },
		Middleware: createMiddleware("low"),
		Priority:   10,
		Enabled:    true,
	})
	
	cm.AddRule(MiddlewareRule{
		Name:       "high_priority",
		Condition:  func(r *http.Request) bool { return true },
		Middleware: createMiddleware("high"),
		Priority:   100,
		Enabled:    true,
	})
	
	cm.AddRule(MiddlewareRule{
		Name:       "medium_priority",
		Condition:  func(r *http.Request) bool { return true },
		Middleware: createMiddleware("medium"),
		Priority:   50,
		Enabled:    true,
	})
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := cm.Apply(handler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	
	wrappedHandler.ServeHTTP(w, req)
	
	// 実行順序を検証（高い優先度から順に実行されるべき）
	expected := []string{"high", "medium", "low"}
	
	mu.Lock()
	defer mu.Unlock()
	
	if len(executionOrder) != len(expected) {
		t.Fatalf("Expected %d middlewares to execute, got %d", len(expected), len(executionOrder))
	}
	
	for i, expectedName := range expected {
		if executionOrder[i] != expectedName {
			t.Errorf("Expected middleware '%s' at position %d, got '%s'", expectedName, i, executionOrder[i])
		}
	}
	
	t.Log("Middleware executed in correct priority order")
}

func TestConditionalMiddleware_DynamicUpdate(t *testing.T) {
	cm := NewConditionalMiddleware()
	
	testMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Dynamic", "active")
			next.ServeHTTP(w, r)
		})
	}
	
	// 初期ルールを追加
	cm.AddRule(MiddlewareRule{
		Name:       "dynamic_rule",
		Condition:  func(r *http.Request) bool { return true },
		Middleware: testMiddleware,
		Priority:   100,
		Enabled:    false, // 最初は無効
	})
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := cm.Apply(handler)
	
	// 最初のテスト（無効化されているのでミドルウェアは適用されない）
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)
	
	if w.Header().Get("X-Dynamic") != "" {
		t.Error("Middleware should not be applied when disabled")
	}
	
	// ルールを更新して有効化
	cm.UpdateRule("dynamic_rule", MiddlewareRule{
		Name:       "dynamic_rule",
		Condition:  func(r *http.Request) bool { return true },
		Middleware: testMiddleware,
		Priority:   100,
		Enabled:    true, // 有効化
	})
	
	// 二番目のテスト（有効化されているのでミドルウェアが適用される）
	req = httptest.NewRequest("GET", "/test", nil)
	w = httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)
	
	if w.Header().Get("X-Dynamic") != "active" {
		t.Error("Middleware should be applied when enabled")
	}
	
	t.Log("Dynamic rule update working correctly")
}

func TestABTestMiddleware(t *testing.T) {
	variantMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Variant", "B")
			next.ServeHTTP(w, r)
		})
	}
	
	controlMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Variant", "A")
			next.ServeHTTP(w, r)
		})
	}
	
	// 50%の分割でA/Bテストを作成
	abTest := NewABTestMiddleware("test_experiment", 0.5, variantMiddleware, controlMiddleware)
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := abTest.Apply(handler)
	
	// 複数回テストして両方のバリアントが呼ばれることを確認
	userIDs := []string{"user1", "user2", "user3", "user4", "user5", "user6", "user7", "user8"}
	
	variantCount := 0
	controlCount := 0
	
	for _, userID := range userIDs {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-User-ID", userID)
		w := httptest.NewRecorder()
		
		wrappedHandler.ServeHTTP(w, req)
		
		variant := w.Header().Get("X-AB-Test-Variant")
		if variant == "A" {
			controlCount++
		} else if variant == "B" {
			variantCount++
		}
		
		// テスト名が設定されていることを確認
		if w.Header().Get("X-AB-Test-Name") != "test_experiment" {
			t.Error("AB test name header not set correctly")
		}
	}
	
	// 両方のバリアントが実行されたことを確認
	if variantCount == 0 || controlCount == 0 {
		t.Errorf("Both variants should be executed. Variant: %d, Control: %d", variantCount, controlCount)
	}
	
	t.Logf("A/B test variant distribution - Variant B: %d, Control A: %d", variantCount, controlCount)
}

func TestFeatureFlagMiddleware(t *testing.T) {
	flagClient := NewSimpleFeatureFlagClient()
	
	enabledMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Feature", "enabled")
			next.ServeHTTP(w, r)
		})
	}
	
	fallbackMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Feature", "disabled")
			next.ServeHTTP(w, r)
		})
	}
	
	ffm := NewFeatureFlaggedMiddleware("test_feature", flagClient, enabledMiddleware, fallbackMiddleware)
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := ffm.Apply(handler)
	
	// 機能フラグが無効の場合
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user123")
	w := httptest.NewRecorder()
	
	wrappedHandler.ServeHTTP(w, req)
	
	if w.Header().Get("X-Feature") != "disabled" {
		t.Error("Feature should be disabled when flag is not set")
	}
	
	if w.Header().Get("X-Feature-Flag") != "disabled" {
		t.Error("Feature flag header should indicate disabled")
	}
	
	// 機能フラグを有効化
	flagClient.SetFlag("test_feature", true)
	
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user123")
	w = httptest.NewRecorder()
	
	wrappedHandler.ServeHTTP(w, req)
	
	if w.Header().Get("X-Feature") != "enabled" {
		t.Error("Feature should be enabled when flag is set")
	}
	
	if w.Header().Get("X-Feature-Flag") != "enabled" {
		t.Error("Feature flag header should indicate enabled")
	}
	
	t.Log("Feature flag toggle working correctly")
}

func TestAdvancedMiddlewareRouter(t *testing.T) {
	router := NewAdvancedMiddlewareRouter()
	flagClient := NewSimpleFeatureFlagClient()
	
	// 機能フラグを設定
	flagClient.SetFlag("new_ui", true)
	
	// 機能フラグ付きミドルウェアを追加
	router.AddFeatureFlag("new_ui", flagClient,
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-UI-Version", "2.0")
				next.ServeHTTP(w, r)
			})
		},
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-UI-Version", "1.0")
				next.ServeHTTP(w, r)
			})
		},
	)
	
	// A/Bテストを追加
	router.AddABTest("button_color", 0.3,
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Button-Color", "red")
				next.ServeHTTP(w, r)
			})
		},
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Button-Color", "blue")
				next.ServeHTTP(w, r)
			})
		},
	)
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := router.Handler(handler)
	
	// 通常のリクエストテスト
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-User-ID", "user123")
	w := httptest.NewRecorder()
	
	wrappedHandler.ServeHTTP(w, req)
	
	// 機能フラグが適用されていることを確認
	if w.Header().Get("X-UI-Version") != "2.0" {
		t.Error("Feature flag should enable new UI version")
	}
	
	// A/Bテストが適用されていることを確認
	buttonColor := w.Header().Get("X-Button-Color")
	if buttonColor != "red" && buttonColor != "blue" {
		t.Error("A/B test should set button color")
	}
	
	// デフォルトのミドルウェアが適用されていることを確認
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS middleware should be applied")
	}
	
	t.Log("Advanced middleware router working correctly")
}

func TestConditionalMiddleware_MethodCondition(t *testing.T) {
	cm := NewConditionalMiddleware()
	
	testMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Method-Test", "applied")
			next.ServeHTTP(w, r)
		})
	}
	
	// POST/PUTメソッドでのみ適用されるルール
	cm.AddRule(MiddlewareRule{
		Name:       "method_test",
		Condition:  MethodIs("POST", "PUT"),
		Middleware: testMiddleware,
		Priority:   100,
		Enabled:    true,
	})
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := cm.Apply(handler)
	
	tests := []struct {
		method   string
		expected string
	}{
		{"GET", ""},
		{"POST", "applied"},
		{"PUT", "applied"},
		{"DELETE", ""},
	}
	
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			w := httptest.NewRecorder()
			
			wrappedHandler.ServeHTTP(w, req)
			
			result := w.Header().Get("X-Method-Test")
			if result != tt.expected {
				t.Errorf("Method %s: expected '%s', got '%s'", tt.method, tt.expected, result)
			}
		})
	}
}

func TestConditionalMiddleware_HeaderCondition(t *testing.T) {
	cm := NewConditionalMiddleware()
	
	testMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Header-Test", "applied")
			next.ServeHTTP(w, r)
		})
	}
	
	// 特定のヘッダー値でのみ適用されるルール
	cm.AddRule(MiddlewareRule{
		Name:       "header_test",
		Condition:  HasHeader("X-API-Version", "v2"),
		Middleware: testMiddleware,
		Priority:   100,
		Enabled:    true,
	})
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := cm.Apply(handler)
	
	// ヘッダーがない場合
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)
	
	if w.Header().Get("X-Header-Test") != "" {
		t.Error("Middleware should not apply without correct header")
	}
	
	// 正しいヘッダーがある場合
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Version", "v2")
	w = httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)
	
	if w.Header().Get("X-Header-Test") != "applied" {
		t.Error("Middleware should apply with correct header")
	}
	
	// 異なるヘッダー値の場合
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Version", "v1")
	w = httptest.NewRecorder()
	wrappedHandler.ServeHTTP(w, req)
	
	if w.Header().Get("X-Header-Test") != "" {
		t.Error("Middleware should not apply with incorrect header value")
	}
}

func TestConditionalMiddleware_Performance(t *testing.T) {
	cm := NewConditionalMiddleware()
	
	// 複数のルールを追加
	for i := 0; i < 100; i++ {
		cm.AddRule(MiddlewareRule{
			Name: fmt.Sprintf("rule_%d", i),
			Condition: func(r *http.Request) bool {
				return r.URL.Path == fmt.Sprintf("/path_%d", i%10)
			},
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r)
				})
			},
			Priority: i,
			Enabled:  true,
		})
	}
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := cm.Apply(handler)
	
	// パフォーマンステスト
	start := time.Now()
	iterations := 1000
	
	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)
	}
	
	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)
	
	// 平均処理時間が1ms未満であることを確認
	if avgDuration > time.Millisecond {
		t.Errorf("Average request duration too slow: %v", avgDuration)
	}
	
	t.Logf("Performance test passed. Average duration: %v", avgDuration)
}

// ベンチマークテスト
func BenchmarkConditionalMiddleware(b *testing.B) {
	cm := NewConditionalMiddleware()
	
	// 10個のルールを追加
	for i := 0; i < 10; i++ {
		cm.AddRule(MiddlewareRule{
			Name: fmt.Sprintf("rule_%d", i),
			Condition: func(r *http.Request) bool {
				return r.URL.Path == "/test"
			},
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r)
				})
			},
			Priority: i,
			Enabled:  true,
		})
	}
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	
	wrappedHandler := cm.Apply(handler)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)
		}
	})
}