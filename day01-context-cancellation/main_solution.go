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
func ProcessWithCancellation(numWorkers int, workDuration time.Duration, cancelAfter time.Duration) error {
	// 1. context.WithCancel()でキャンセル可能なコンテキストを作成
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// 2. 結果を受信するチャネル
	results := make(chan WorkResult, numWorkers)
	var wg sync.WaitGroup
	
	// 3. 指定された数のワーカーGoroutineを起動
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			Worker(ctx, workerID, results)
		}(i)
	}
	
	// 4. cancelAfter時間後にキャンセルシグナルを送信
	time.AfterFunc(cancelAfter, func() {
		cancel()
	})
	
	// 5. すべてのワーカーの完了を待機
	go func() {
		wg.Wait()
		close(results)
	}()
	
	// 6. 結果を収集
	for result := range results {
		fmt.Printf("Worker %d: %s\n", result.WorkerID, result.Message)
	}
	
	return nil
}

// Worker は与えられたcontextをチェックして作業を行う
// キャンセルシグナルを受け取ったら即座に停止する
func Worker(ctx context.Context, id int, results chan<- WorkResult) error {
	// 作業完了までの時間を設定（約1秒で完了）
	workDuration := 1 * time.Second
	workDone := time.After(workDuration)
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			// キャンセルシグナルを受信
			results <- WorkResult{
				WorkerID:  id,
				Completed: false,
				Message:   "Worker cancelled",
			}
			return ctx.Err()
			
		case <-workDone:
			// 作業完了
			results <- WorkResult{
				WorkerID:  id,
				Completed: true,
				Message:   "Work completed successfully",
			}
			return nil
			
		case <-ticker.C:
			// 通常の作業処理中（進捗報告）
			// このケースでは結果を送信しない（テストでは単一の結果を期待している）
		}
	}
}