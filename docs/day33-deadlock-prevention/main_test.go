package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	_ "github.com/lib/pq"
)

var (
	db   *sql.DB
	pool *dockertest.Pool
)

func TestMain(m *testing.M) {
	var err error
	pool, err = dockertest.NewPool("")
	if err != nil {
		panic(fmt.Sprintf("Could not connect to docker: %s", err))
	}

	// PostgreSQLコンテナを起動
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			"POSTGRES_PASSWORD=test",
			"POSTGRES_DB=testdb",
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		panic(fmt.Sprintf("Could not start resource: %s", err))
	}

	hostAndPort := resource.GetHostPort("5432/tcp")
	databaseUrl := fmt.Sprintf("postgres://postgres:test@%s/testdb?sslmode=disable", hostAndPort)

	// データベース接続を待つ
	pool.MaxWait = 120 * time.Second
	if err = pool.Retry(func() error {
		db, err = sql.Open("postgres", databaseUrl)
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		panic(fmt.Sprintf("Could not connect to docker: %s", err))
	}

	// テーブル初期化
	if err := InitializeDeadlockTestDB(db); err != nil {
		panic(fmt.Sprintf("Could not initialize database: %s", err))
	}

	code := m.Run()

	// クリーンアップ
	if err := pool.Purge(resource); err != nil {
		panic(fmt.Sprintf("Could not purge resource: %s", err))
	}

	if code != 0 {
		panic("Tests failed")
	}
}

func setupTestAccounts(t *testing.T) {
	t.Helper()
	
	// テストデータをクリーンアップ
	_, err := db.Exec("DELETE FROM accounts")
	if err != nil {
		t.Fatal(err)
	}

	// テストアカウントを作成
	accounts := []struct{ id int; balance float64 }{
		{1, 1000.0},
		{2, 1000.0},
		{3, 1000.0},
		{4, 1000.0},
	}

	for _, acc := range accounts {
		_, err := db.Exec("INSERT INTO accounts (id, balance, version) VALUES ($1, $2, 0)", acc.id, acc.balance)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeadlockSimulator_SimulateClassicDeadlock(t *testing.T) {
	setupTestAccounts(t)
	simulator := NewDeadlockSimulator(db)

	// デッドロックを発生させるテスト
	err := simulator.SimulateClassicDeadlock(1, 2, 100.0, 50.0)
	
	// デッドロックが発生するか、または正常に完了する
	// 実際の結果は実行環境とタイミングに依存
	if err != nil {
		// デッドロックエラーかチェック
		if !isDeadlockError(err) {
			t.Errorf("Expected deadlock error or success, got: %v", err)
		} else {
			t.Logf("Deadlock successfully simulated: %v", err)
		}
	} else {
		t.Logf("Operations completed without deadlock (timing-dependent)")
	}
}

func TestDeadlockSimulator_SimulateMultiResourceDeadlock(t *testing.T) {
	setupTestAccounts(t)
	simulator := NewDeadlockSimulator(db)

	accountIDs := []int{1, 2, 3, 4}
	amounts := []float64{100.0, 50.0, 75.0, 25.0}

	err := simulator.SimulateMultiResourceDeadlock(accountIDs, amounts)
	
	// マルチリソースデッドロックのテスト
	if err != nil && !isDeadlockError(err) {
		t.Errorf("Unexpected error type: %v", err)
	}
}

func TestDeadlockPreventer_TransferMoneyOrdered(t *testing.T) {
	setupTestAccounts(t)
	preventer := NewDeadlockPreventer(db)

	// 順序付きロックでデッドロックを予防
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// 大量の並行送金でデッドロック予防をテスト
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			
			fromID := (i % 2) + 1
			toID := ((i + 1) % 2) + 1
			amount := 10.0
			
			err := preventer.TransferMoneyOrdered(fromID, toID, amount)
			errors <- err
		}(i)
	}

	wg.Wait()
	close(errors)

	// すべての操作が成功するはず（デッドロック予防）
	for err := range errors {
		if err != nil {
			t.Errorf("Transfer failed: %v", err)
		}
	}

	// 最終残高確認
	var balance1, balance2 float64
	err := db.QueryRow("SELECT balance FROM accounts WHERE id = 1").Scan(&balance1)
	if err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow("SELECT balance FROM accounts WHERE id = 2").Scan(&balance2)
	if err != nil {
		t.Fatal(err)
	}

	// 総額は変わらないはず
	if balance1+balance2 != 2000.0 {
		t.Errorf("Total balance mismatch: %f + %f = %f", balance1, balance2, balance1+balance2)
	}
}

func TestDeadlockPreventer_TransferMultipleOrdered(t *testing.T) {
	setupTestAccounts(t)
	preventer := NewDeadlockPreventer(db)

	transfers := []Transfer{
		{FromID: 1, ToID: 2, Amount: 100.0},
		{FromID: 2, ToID: 3, Amount: 50.0},
		{FromID: 3, ToID: 4, Amount: 25.0},
		{FromID: 4, ToID: 1, Amount: 75.0},
	}

	err := preventer.TransferMultipleOrdered(transfers)
	if err != nil {
		t.Fatalf("Multiple transfers failed: %v", err)
	}

	// 各アカウントの最終残高確認
	expectedBalances := map[int]float64{
		1: 1000.0 - 100.0 + 75.0,
		2: 1000.0 + 100.0 - 50.0,
		3: 1000.0 + 50.0 - 25.0,
		4: 1000.0 + 25.0 - 75.0,
	}

	for accountID, expected := range expectedBalances {
		var actual float64
		err := db.QueryRow("SELECT balance FROM accounts WHERE id = $1", accountID).Scan(&actual)
		if err != nil {
			t.Fatal(err)
		}
		if actual != expected {
			t.Errorf("Account %d balance: expected %f, got %f", accountID, expected, actual)
		}
	}
}

func TestDeadlockDetector_ExecuteWithRetry(t *testing.T) {
	setupTestAccounts(t)
	monitor := NewDeadlockMonitor()
	detector := NewDeadlockDetector(db, 3, monitor)

	// 人工的なデッドロックエラーでリトライテスト
	attempts := 0
	err := detector.ExecuteWithRetry(func(tx *sql.Tx) error {
		attempts++
		if attempts <= 2 {
			// 最初の2回は人工的にデッドロックエラー
			return fmt.Errorf("deadlock detected")
		}
		// 3回目は成功
		_, err := tx.Exec("UPDATE accounts SET balance = balance + 10 WHERE id = 1")
		return err
	})

	if err != nil {
		t.Fatalf("ExecuteWithRetry failed: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	// 統計確認
	stats := monitor.GetStatistics()
	if stats.DeadlockCount != 2 {
		t.Errorf("Expected 2 deadlocks recorded, got %d", stats.DeadlockCount)
	}
}

func TestDeadlockDetector_ExecuteWithTimeout(t *testing.T) {
	setupTestAccounts(t)
	monitor := NewDeadlockMonitor()
	detector := NewDeadlockDetector(db, 5, monitor)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := detector.ExecuteWithTimeout(ctx, func(tx *sql.Tx) error {
		// 常にデッドロックエラーを返す
		return fmt.Errorf("deadlock detected")
	})

	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	// タイムアウト時間内に終了しているかチェック
	if elapsed > 150*time.Millisecond {
		t.Errorf("Operation took too long: %v", elapsed)
	}
}

func TestResourceLockManager_LockResources(t *testing.T) {
	rlm := NewResourceLockManager()

	resources1 := []string{"resource_1", "resource_2", "resource_3"}
	resources2 := []string{"resource_3", "resource_1", "resource_2"} // 異なる順序

	var wg sync.WaitGroup
	results := make(chan error, 2)

	// 2つのゴルーチンで異なる順序でリソースアクセス
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := rlm.WithOrderedLocks(resources1, func() error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		results <- err
	}()

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		err := rlm.WithOrderedLocks(resources2, func() error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		results <- err
	}()

	wg.Wait()
	close(results)

	// 両方とも成功するはず（デッドロック予防）
	for err := range results {
		if err != nil {
			t.Errorf("Resource locking failed: %v", err)
		}
	}
}

func TestDeadlockMonitor_Statistics(t *testing.T) {
	monitor := NewDeadlockMonitor()

	// 初期状態確認
	stats := monitor.GetStatistics()
	if stats.DeadlockCount != 0 {
		t.Errorf("Initial deadlock count should be 0, got %d", stats.DeadlockCount)
	}

	// デッドロック記録
	monitor.RecordDeadlock()
	monitor.RecordDeadlock()

	// リトライ記録
	monitor.RecordRetry(false) // 失敗
	monitor.RecordRetry(true)  // 成功
	monitor.RecordRetry(true)  // 成功

	stats = monitor.GetStatistics()
	if stats.DeadlockCount != 2 {
		t.Errorf("Expected 2 deadlocks, got %d", stats.DeadlockCount)
	}
	if stats.TotalRetries != 3 {
		t.Errorf("Expected 3 retries, got %d", stats.TotalRetries)
	}
	if stats.SuccessfulRetries != 2 {
		t.Errorf("Expected 2 successful retries, got %d", stats.SuccessfulRetries)
	}

	expectedSuccessRate := 2.0 / 3.0
	if abs(stats.RetrySuccessRate-expectedSuccessRate) > 0.01 {
		t.Errorf("Expected success rate %f, got %f", expectedSuccessRate, stats.RetrySuccessRate)
	}

	// 統計リセット
	monitor.ResetStatistics()
	stats = monitor.GetStatistics()
	if stats.DeadlockCount != 0 || stats.TotalRetries != 0 {
		t.Error("Statistics not properly reset")
	}
}

func TestUtilityFunctions(t *testing.T) {
	// isDeadlockError のテスト
	tests := []struct {
		err      error
		expected bool
	}{
		{fmt.Errorf("deadlock detected"), true},
		{fmt.Errorf("ERROR: deadlock (40P01)"), true},
		{fmt.Errorf("connection timeout"), false},
		{fmt.Errorf("syntax error"), false},
		{nil, false},
	}

	for _, test := range tests {
		result := isDeadlockError(test.err)
		if result != test.expected {
			t.Errorf("isDeadlockError(%v) = %v, expected %v", test.err, result, test.expected)
		}
	}

	// calculateBackoffDelay のテスト
	baseDelay := 100 * time.Millisecond
	delay1 := calculateBackoffDelay(0, baseDelay)
	delay2 := calculateBackoffDelay(1, baseDelay)
	delay3 := calculateBackoffDelay(2, baseDelay)

	if delay1 >= delay2 || delay2 >= delay3 {
		t.Error("Backoff delay should increase exponentially")
	}

	// orderAccountIDs のテスト
	ids := []int{3, 1, 4, 2}
	ordered := orderAccountIDs(ids)
	expected := []int{1, 2, 3, 4}
	
	for i, id := range ordered {
		if id != expected[i] {
			t.Errorf("orderAccountIDs failed: expected %v, got %v", expected, ordered)
			break
		}
	}
}

func TestConcurrentDeadlockPrevention(t *testing.T) {
	setupTestAccounts(t)
	preventer := NewDeadlockPreventer(db)

	numGoroutines := 20
	numTransfers := 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numTransfers)

	// 大量の並行送金でデッドロック予防をテスト
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numTransfers; j++ {
				fromID := (goroutineID*numTransfers+j)%4 + 1
				toID := ((goroutineID*numTransfers+j)+1)%4 + 1
				amount := 1.0
				
				err := preventer.TransferMoneyOrdered(fromID, toID, amount)
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// すべての操作が成功するはず
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Transfer error: %v", err)
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected no errors with deadlock prevention, got %d errors", errorCount)
	}
}

// Benchmark tests

func BenchmarkDeadlockPreventer_TransferMoney(b *testing.B) {
	setupTestAccounts(&testing.T{})
	preventer := NewDeadlockPreventer(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fromID := (i % 2) + 1
		toID := ((i + 1) % 2) + 1
		amount := 1.0
		
		err := preventer.TransferMoneyOrdered(fromID, toID, amount)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResourceLockManager_WithOrderedLocks(b *testing.B) {
	rlm := NewResourceLockManager()
	resources := []string{"resource_1", "resource_2", "resource_3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := rlm.WithOrderedLocks(resources, func() error {
			return nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper functions

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}