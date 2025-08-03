package main

import (
	"fmt"
	"sync"
)

// 【正しい実装】sync.Onceによる安全な一度限りの初期化
var (
	config *Config   // 【保護対象】設定データ
	once   sync.Once // 【制御機構】一度限りの実行を保証
)

type Config struct {
	Setting1 string
	Setting2 int
}

func GetConfig() *Config {
	// 【核心機能】once.Do()が一度限りの実行を保証
	once.Do(func() {
		// 【重要】この関数は以下を保証する：
		// 1. プログラム実行中に一度だけ呼ばれる
		// 2. 複数のGoroutineが同時に呼び出しても安全
		// 3. 最初のGoroutineが実行中、他は完了まで待機
		// 4. 実行完了後、他のGoroutineは即座にreturn

		config = &Config{
			Setting1: "value1",
			Setting2: 42,
		}
		fmt.Println("Configuration loaded!")

		// 【内部動作の詳細】：
		// sync.Onceは内部で以下の仕組みを使用：
		// - atomic操作による高速な「実行済みチェック」
		// - Mutexによる排他制御（初回実行時のみ）
		// - Memory Barrierによる可視性保証
	})

	// 【パフォーマンス特性】：
	// - 初回呼び出し: Mutex + 関数実行 + 状態更新
	// - 2回目以降: atomic load のみ（非常に高速）
	return config
}

// 【使用例】複数Goroutineからの安全な呼び出し
func demonstrateSafeUsage() {
	var wg sync.WaitGroup

	// 100個のGoroutineが同時にGetConfig()を呼び出し
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			cfg := GetConfig() // 【安全】どのGoroutineも同じ設定を取得
			fmt.Printf("Goroutine %d got config: %v\n", id, cfg != nil)
		}(i)
	}

	wg.Wait()
	// 結果: "Configuration loaded!" は一度だけ出力される
}

func main() {
	// 【実行例】安全な一度限りの初期化をデモ
	demonstrateSafeUsage()

	// 【注意点】：
	// - GetConfig()はスレッドセーフであり、どのGoroutineからも安全に呼び出せる
	// - 初回呼び出し時のみ設定がロードされ、以降はキャッシュされた値が返される
}
