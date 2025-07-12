//go:build ignore

package main

import (
	"context"
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
	
	return nil
}

func main() {
	// テスト用のサンプル実行
	err := ProcessWithCancellation(3, 5*time.Second, 2*time.Second)
	if err != nil {
		panic(err)
	}
}