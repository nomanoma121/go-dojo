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
	// TODO: ここに実装を追加してください
	// 
	// 実装の流れ:
	// 1. context.WithCancel()でキャンセル可能なコンテキストを作成
	// 2. 指定された数のワーカーGoroutineを起動
	// 3. cancelAfter時間後にキャンセルシグナルを送信
	// 4. すべてのワーカーの完了を待機
	ctx, cancel := context.WithCancel(context.Background())

	defer cancel() // 関数終了時に必ずキャンセルを呼び出す

	results := make(chan WorkResult, numWorkers)

	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)

		go func(workerID int) {
			defer wg.Done()

			Worker(ctx, workerID, results)
		}(i)
	}

	// 指定時間後にキャンセルシグナルを送信
	time.AfterFunc(cancelAfter, func() {
		cancel()
	})

	go func() {
		wg.Wait() // 全ワーカーの完了を待機
		close(results) // 結果チャネルを閉じる
	}()

	for result := range results {
		fmt.Printf("Worker %d completed: %t, message: %s\n", result.WorkerID, result.Completed, result.Message)
	}

	return nil
}

// Worker は与えられたcontextをチェックして作業を行う
// キャンセルシグナルを受け取ったら即座に停止する
func Worker(ctx context.Context, id int, results chan<- WorkResult) error {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. ループで継続的に作業を実行
	// 2. select文でctx.Done()とその他の処理を監視
	// 3. キャンセルシグナルを受け取ったら適切に終了
	// 4. 結果をresultsチャネルに送信
	workDuration := 1 * time.Second
	workDone := time.After(workDuration)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			results <- WorkResult{
				WorkerID:  id,
				Completed: false,
				Message:   "Cancelled",
			}
			return nil

		case <-workDone:
			results <- WorkResult{
				WorkerID:  id,
				Completed: true,
				Message:   "Work completed",
			}
			return nil

		case <-ticker.C:

		}
	}
}

func main() {
	// テスト用のサンプル実行
	err := ProcessWithCancellation(1000000, 5*time.Second, 2*time.Second)
	if err != nil {
		panic(err)
	}
}
