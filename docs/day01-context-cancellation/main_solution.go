package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WorkResult represents the result of a worker's task
type WorkResult struct {
	WorkerID  int
	Completed bool
	Message   string
}

// ProcessWithCancellation は複数のワーカーGoroutineを起動し、
// 指定時間後にキャンセルシグナルを送信して全ワーカーを停止させる
//
// 【Context Cancellationパターンの実装】
// このパターンは以下の課題を解決します：
// - 長時間実行されるタスクの安全な停止
// - リソースリークの防止
// - レスポンシブなシャットダウン
func ProcessWithCancellation(numWorkers int, workDuration time.Duration, cancelAfter time.Duration) error {
	// 1. 【Context作成】キャンセル可能なコンテキストを作成
	// context.Background()から派生させることで、親コンテキストなしの
	// ルートコンテキストを作成。cancel()関数で明示的にキャンセル可能
	ctx, cancel := context.WithCancel(context.Background())
	
	// 【重要】defer cancel()でリソースリークを防止
	// 関数終了時に必ずcancel()が呼ばれ、コンテキストに関連する
	// 内部リソースが確実に解放される
	defer cancel()
	
	// 2. 【バッファ付きチャネル】ワーカーからの結果受信
	// バッファサイズをnumWorkersにすることで、全ワーカーが
	// ブロックされることなく結果を送信可能
	results := make(chan WorkResult, numWorkers)
	
	// 【WaitGroup】全ワーカーの完了待機用
	// Goroutineの完了を効率的に待機するGoの標準パターン
	var wg sync.WaitGroup
	
	// 3. 【並行ワーカー起動】指定された数のワーカーGoroutineを起動
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)  // WaitGroupのカウンターを増加
		
		// 【Goroutine起動】各ワーカーを独立したGoroutineで実行
		go func(workerID int) {
			defer wg.Done()  // Goroutine終了時にカウンターを減算
			
			// 同じcontextを全ワーカーに渡すことで、
			// 統一されたキャンセルシグナル配信が可能
			Worker(ctx, workerID, results)
		}(i)  // 【重要】ループ変数のキャプチャ問題を回避
	}
	
	// 4. 【タイマー駆動キャンセル】指定時間後の自動キャンセル
	// time.AfterFunc()により、メインgoroutineをブロックせずに
	// 指定時間後にcancel()を実行
	time.AfterFunc(cancelAfter, func() {
		fmt.Printf("Cancellation signal sent after %v\n", cancelAfter)
		cancel()  // 全ワーカーにキャンセルシグナルを送信
	})
	
	// 5. 【非同期完了処理】全ワーカーの完了を非同期で監視
	go func() {
		wg.Wait()      // 全ワーカーの完了を待機
		close(results) // チャネルクローズでrange終了を通知
	}()
	
	// 6. 【結果収集】チャネルから結果を順次受信
	// range文によりチャネルがクローズされるまで受信を継続
	// この設計により、完了順序に関係なく全結果を収集可能
	for result := range results {
		fmt.Printf("Worker %d: %s\n", result.WorkerID, result.Message)
	}
	
	return nil
}

// Worker は与えられたcontextをチェックして作業を行う
// キャンセルシグナルを受け取ったら即座に停止する
//
// 【キャンセル対応ワーカーの実装パターン】
// この関数は以下の重要な概念を実装しています：
// - select文による非ブロッキング複数チャネル監視
// - Context.Done()チャネルを用いたキャンセル検出
// - レスポンシブなタスク実行（定期的なキャンセルチェック）
func Worker(ctx context.Context, id int, results chan<- WorkResult) error {
	// 【作業時間シミュレーション】実際の処理時間を模擬
	// 1秒の作業時間により、キャンセル動作のテストが可能
	workDuration := 1 * time.Second
	workDone := time.After(workDuration)
	
	// 【定期チェック用タイマー】100msごとのheartbeat
	// 実際のアプリケーションでは、長時間の処理を小さな単位に分割し、
	// 定期的にキャンセルシグナルをチェックすることが重要
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()  // 【リソース解放】タイマーリソースの確実な解放
	
	// 【メインループ】select文による複数イベントの監視
	// Goの並行性の核心：複数のチャネルから最初に到着したメッセージを処理
	for {
		select {
		// 【最優先】キャンセルシグナルの監視
		// Context.Done()はキャンセル時にクローズされ、受信可能になる
		case <-ctx.Done():
			// キャンセルされた場合は即座に処理を停止
			results <- WorkResult{
				WorkerID:  id,
				Completed: false,
				Message:   "Worker cancelled",
			}
			// 【エラー返却】ctx.Err()でキャンセル理由を返す
			// context.Canceled または context.DeadlineExceeded
			return ctx.Err()
			
		// 【正常完了】作業時間終了
		case <-workDone:
			// 予定通り作業が完了した場合
			results <- WorkResult{
				WorkerID:  id,
				Completed: true,
				Message:   "Work completed successfully",
			}
			return nil
			
		// 【定期処理】作業継続中のheartbeat
		case <-ticker.C:
			// 【設計ポイント】進捗報告やヘルスチェック
			// 実際のアプリケーションでは、ここで以下を実行：
			// - 進捗状況の報告
			// - 部分的な作業の実行
			// - 外部リソースのヘルスチェック
			// 
			// 今回はテストの簡潔性のため結果送信は省略
			// （テストが単一結果を期待しているため）
		}
	}
}