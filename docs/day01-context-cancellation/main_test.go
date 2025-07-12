package main

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestProcessWithCancellation(t *testing.T) {
	tests := []struct {
		name         string
		numWorkers   int
		workDuration time.Duration
		cancelAfter  time.Duration
		expectError  bool
	}{
		{
			name:         "正常なキャンセル処理",
			numWorkers:   3,
			workDuration: 5 * time.Second,
			cancelAfter:  1 * time.Second,
			expectError:  false,
		},
		{
			name:         "多数のワーカー",
			numWorkers:   10,
			workDuration: 3 * time.Second,
			cancelAfter:  500 * time.Millisecond,
			expectError:  false,
		},
		{
			name:         "即座にキャンセル",
			numWorkers:   5,
			workDuration: 2 * time.Second,
			cancelAfter:  10 * time.Millisecond,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Goroutineリークを検出するため、開始時のGoroutine数を記録
			initialGoroutines := runtime.NumGoroutine()

			start := time.Now()
			err := ProcessWithCancellation(tt.numWorkers, tt.workDuration, tt.cancelAfter)

			if (err != nil) != tt.expectError {
				t.Errorf("ProcessWithCancellation() error = %v, expectError %v", err, tt.expectError)
			}

			elapsed := time.Since(start)
			
			// キャンセル処理が適切な時間で完了することを確認
			// キャンセル時間 + 少しのマージンで完了すべき
			maxExpectedTime := tt.cancelAfter + 500*time.Millisecond
			if elapsed > maxExpectedTime {
				t.Errorf("ProcessWithCancellation() took %v, expected around %v", elapsed, tt.cancelAfter)
			}

			// 少し待ってからGoroutineリークをチェック
			time.Sleep(100 * time.Millisecond)
			runtime.GC()
			time.Sleep(100 * time.Millisecond)

			finalGoroutines := runtime.NumGoroutine()
			if finalGoroutines > initialGoroutines+1 { // テスト自体のGoroutineを考慮
				t.Errorf("Goroutine leak detected: initial=%d, final=%d", initialGoroutines, finalGoroutines)
			}
		})
	}
}

func TestWorker(t *testing.T) {
	tests := []struct {
		name           string
		cancelAfter    time.Duration
		expectComplete bool
	}{
		{
			name:           "キャンセルなしで完了",
			cancelAfter:    2 * time.Second,
			expectComplete: true,
		},
		{
			name:           "早期キャンセル",
			cancelAfter:    100 * time.Millisecond,
			expectComplete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			results := make(chan WorkResult, 1)

			// ワーカーを別のGoroutineで実行
			go func() {
				Worker(ctx, 1, results)
			}()

			// 指定時間後にキャンセル
			time.AfterFunc(tt.cancelAfter, cancel)

			// 結果を待機（タイムアウト付き）
			select {
			case result := <-results:
				if result.Completed != tt.expectComplete {
					t.Errorf("Worker completed = %v, expected %v", result.Completed, tt.expectComplete)
				}
				if result.WorkerID != 1 {
					t.Errorf("Worker ID = %v, expected 1", result.WorkerID)
				}
			case <-time.After(3 * time.Second):
				if tt.expectComplete {
					t.Error("Worker did not complete within timeout")
				}
			}
		})
	}
}

// ベンチマークテスト: 大量のワーカーでのパフォーマンス測定
func BenchmarkProcessWithCancellation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		err := ProcessWithCancellation(100, 1*time.Second, 100*time.Millisecond)
		if err != nil {
			b.Fatalf("ProcessWithCancellation failed: %v", err)
		}
	}
}

// レースコンディションのテスト
func TestProcessWithCancellationRace(t *testing.T) {
	// このテストは -race フラグと一緒に実行される
	for i := 0; i < 10; i++ {
		err := ProcessWithCancellation(20, 500*time.Millisecond, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Race test failed: %v", err)
		}
	}
}