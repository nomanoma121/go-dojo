package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// Account represents a bank account
type Account struct {
	ID      int
	Balance float64
	Version int
}

// DeadlockSimulator simulates deadlock conditions for testing
type DeadlockSimulator struct {
	db *sql.DB
}

// NewDeadlockSimulator creates a new deadlock simulator
func NewDeadlockSimulator(db *sql.DB) *DeadlockSimulator {
	return &DeadlockSimulator{
		db: db,
	}
}

// SimulateClassicDeadlock creates a classic deadlock scenario with two transactions
func (ds *DeadlockSimulator) SimulateClassicDeadlock(account1ID, account2ID int, amount1, amount2 float64) error {
	var wg sync.WaitGroup
	errors := make(chan error, 2)

	// トランザクション1: アカウント1→2の順でロック
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := ds.transferWithDelay(account1ID, account2ID, amount1, 100*time.Millisecond)
		errors <- err
	}()

	// トランザクション2: アカウント2→1の順でロック（逆順）
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond) // わずかな遅延でタイミングを調整
		err := ds.transferWithDelay(account2ID, account1ID, amount2, 100*time.Millisecond)
		errors <- err
	}()

	wg.Wait()
	close(errors)

	// いずれかでデッドロックが発生した場合はエラーを返す
	for err := range errors {
		if err != nil && isDeadlockError(err) {
			return fmt.Errorf("deadlock detected: %w", err)
		} else if err != nil {
			return err
		}
	}

	return nil
}

// transferWithDelay performs transfer with artificial delay to induce deadlock
func (ds *DeadlockSimulator) transferWithDelay(fromID, toID int, amount float64, delay time.Duration) error {
	tx, err := ds.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 最初のアカウントをロック
	_, err = tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)
	if err != nil {
		return err
	}

	// 意図的な遅延でデッドロックを誘発
	time.Sleep(delay)

	// 二番目のアカウントをロック（ここでデッドロックが発生する可能性）
	_, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// SimulateMultiResourceDeadlock creates deadlock with multiple resources
func (ds *DeadlockSimulator) SimulateMultiResourceDeadlock(accountIDs []int, amounts []float64) error {
	if len(accountIDs) != len(amounts) {
		return fmt.Errorf("accountIDs and amounts length mismatch")
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(accountIDs))

	// 各ゴルーチンが異なる順序でアカウントにアクセス
	for i := 0; i < len(accountIDs); i++ {
		wg.Add(1)
		go func(startIdx int) {
			defer wg.Done()

			tx, err := ds.db.Begin()
			if err != nil {
				errors <- err
				return
			}
			defer tx.Rollback()

			// 各ゴルーチンが異なる開始点から循環的にアカウントにアクセス
			for j := 0; j < len(accountIDs); j++ {
				accountIdx := (startIdx + j) % len(accountIDs)
				accountID := accountIDs[accountIdx]
				amount := amounts[accountIdx]

				_, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, accountID)
				if err != nil {
					errors <- err
					return
				}

				// デッドロック誘発のための遅延
				time.Sleep(50 * time.Millisecond)
			}

			err = tx.Commit()
			errors <- err
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

// DeadlockPreventer prevents deadlocks using ordered locking
type DeadlockPreventer struct {
	db *sql.DB
}

// NewDeadlockPreventer creates a new deadlock preventer
func NewDeadlockPreventer(db *sql.DB) *DeadlockPreventer {
	return &DeadlockPreventer{
		db: db,
	}
}

// TransferMoneyOrdered transfers money using ordered resource locking
func (dp *DeadlockPreventer) TransferMoneyOrdered(fromID, toID int, amount float64) error {
	// 常に小さいIDから大きいIDの順でロック
	firstID, secondID := fromID, toID
	firstAmount, secondAmount := -amount, amount

	if fromID > toID {
		firstID, secondID = toID, fromID
		firstAmount, secondAmount = amount, -amount
	}

	tx, err := dp.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 順序付きでアカウントを更新
	_, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", firstAmount, firstID)
	if err != nil {
		return err
	}

	_, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", secondAmount, secondID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// TransferMultipleOrdered transfers money between multiple accounts with ordered locking
func (dp *DeadlockPreventer) TransferMultipleOrdered(transfers []Transfer) error {
	if len(transfers) == 0 {
		return nil
	}

	// 関係するすべてのアカウントIDを収集
	accountIDSet := make(map[int]bool)
	for _, transfer := range transfers {
		accountIDSet[transfer.FromID] = true
		accountIDSet[transfer.ToID] = true
	}

	// アカウントIDを順序付け
	var accountIDs []int
	for id := range accountIDSet {
		accountIDs = append(accountIDs, id)
	}
	sort.Ints(accountIDs)

	tx, err := dp.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 順序付きでアカウントをロック（FOR UPDATE）
	accountBalances := make(map[int]float64)
	for _, accountID := range accountIDs {
		var balance float64
		err := tx.QueryRow("SELECT balance FROM accounts WHERE id = $1 FOR UPDATE", accountID).Scan(&balance)
		if err != nil {
			return err
		}
		accountBalances[accountID] = balance
	}

	// 残高変更を計算
	balanceChanges := make(map[int]float64)
	for _, transfer := range transfers {
		balanceChanges[transfer.FromID] -= transfer.Amount
		balanceChanges[transfer.ToID] += transfer.Amount
	}

	// 残高チェック
	for accountID, change := range balanceChanges {
		newBalance := accountBalances[accountID] + change
		if newBalance < 0 {
			return fmt.Errorf("insufficient balance for account %d", accountID)
		}
	}

	// 実際の残高更新
	for accountID, change := range balanceChanges {
		if change != 0 {
			_, err := tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", change, accountID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// Transfer represents a money transfer operation
type Transfer struct {
	FromID int
	ToID   int
	Amount float64
}

// DeadlockDetector detects and retries on deadlock errors
type DeadlockDetector struct {
	db         *sql.DB
	maxRetries int
	monitor    *DeadlockMonitor
}

// NewDeadlockDetector creates a new deadlock detector
func NewDeadlockDetector(db *sql.DB, maxRetries int, monitor *DeadlockMonitor) *DeadlockDetector {
	return &DeadlockDetector{
		db:         db,
		maxRetries: maxRetries,
		monitor:    monitor,
	}
}

// ExecuteWithRetry executes operation with deadlock detection and retry
func (dd *DeadlockDetector) ExecuteWithRetry(operation func(*sql.Tx) error) error {
	return dd.ExecuteWithTimeout(context.Background(), operation)
}

// ExecuteWithTimeout executes operation with both retry and timeout
func (dd *DeadlockDetector) ExecuteWithTimeout(ctx context.Context, operation func(*sql.Tx) error) error {
	for attempt := 0; attempt <= dd.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		tx, err := dd.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		err = operation(tx)
		if err != nil {
			tx.Rollback()

			if isDeadlockError(err) {
				dd.monitor.RecordDeadlock()

				if attempt < dd.maxRetries {
					dd.monitor.RecordRetry(false)
					delay := calculateBackoffDelay(attempt, 50*time.Millisecond)

					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(delay):
						continue
					}
				}
			}
			return err
		}

		err = tx.Commit()
		if err != nil {
			if isDeadlockError(err) {
				dd.monitor.RecordDeadlock()

				if attempt < dd.maxRetries {
					dd.monitor.RecordRetry(false)
					delay := calculateBackoffDelay(attempt, 50*time.Millisecond)

					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(delay):
						continue
					}
				}
			}
			return err
		}

		if attempt > 0 {
			dd.monitor.RecordRetry(true)
		}
		return nil
	}

	return fmt.Errorf("operation failed after %d retries", dd.maxRetries)
}

// ResourceLockManager manages ordered resource locking
type ResourceLockManager struct {
	mu              sync.Mutex
	lockedResources map[string]bool
}

// NewResourceLockManager creates a new resource lock manager
func NewResourceLockManager() *ResourceLockManager {
	return &ResourceLockManager{
		lockedResources: make(map[string]bool),
	}
}

// LockResources locks multiple resources in a consistent order
func (rlm *ResourceLockManager) LockResources(resourceIDs []string) error {
	orderedIDs := orderResourceIDs(resourceIDs)

	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	// 既にロックされているリソースがないかチェック
	for _, id := range orderedIDs {
		if rlm.lockedResources[id] {
			return fmt.Errorf("resource %s is already locked", id)
		}
	}

	// すべてのリソースをロック
	for _, id := range orderedIDs {
		rlm.lockedResources[id] = true
	}

	return nil
}

// UnlockResources unlocks multiple resources
func (rlm *ResourceLockManager) UnlockResources(resourceIDs []string) {
	rlm.mu.Lock()
	defer rlm.mu.Unlock()

	for _, id := range resourceIDs {
		delete(rlm.lockedResources, id)
	}
}

// WithOrderedLocks executes function with ordered resource locks
func (rlm *ResourceLockManager) WithOrderedLocks(resourceIDs []string, fn func() error) error {
	err := rlm.LockResources(resourceIDs)
	if err != nil {
		return err
	}
	defer rlm.UnlockResources(resourceIDs)

	return fn()
}

// DeadlockMonitor monitors deadlock occurrences and statistics
type DeadlockMonitor struct {
	deadlockCount     int64
	lastDeadlock      time.Time
	totalRetries      int64
	successfulRetries int64
	mutex             sync.RWMutex
}

// NewDeadlockMonitor creates a new deadlock monitor
func NewDeadlockMonitor() *DeadlockMonitor {
	return &DeadlockMonitor{}
}

// RecordDeadlock records a deadlock occurrence
func (dm *DeadlockMonitor) RecordDeadlock() {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.deadlockCount++
	dm.lastDeadlock = time.Now()
}

// RecordRetry records a retry attempt
func (dm *DeadlockMonitor) RecordRetry(success bool) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.totalRetries++
	if success {
		dm.successfulRetries++
	}
}

// GetStatistics returns current deadlock statistics
func (dm *DeadlockMonitor) GetStatistics() DeadlockStats {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	var successRate float64
	if dm.totalRetries > 0 {
		successRate = float64(dm.successfulRetries) / float64(dm.totalRetries)
	}

	return DeadlockStats{
		DeadlockCount:     dm.deadlockCount,
		LastDeadlock:      dm.lastDeadlock,
		TotalRetries:      dm.totalRetries,
		SuccessfulRetries: dm.successfulRetries,
		RetrySuccessRate:  successRate,
	}
}

// ResetStatistics resets all statistics
func (dm *DeadlockMonitor) ResetStatistics() {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.deadlockCount = 0
	dm.lastDeadlock = time.Time{}
	dm.totalRetries = 0
	dm.successfulRetries = 0
}

// DeadlockStats holds deadlock statistics
type DeadlockStats struct {
	DeadlockCount     int64
	LastDeadlock      time.Time
	TotalRetries      int64
	SuccessfulRetries int64
	RetrySuccessRate  float64
}

// Utility functions

// isDeadlockError checks if error is a deadlock error
func isDeadlockError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "deadlock") ||
		strings.Contains(errStr, "40p01") // PostgreSQL deadlock error code
}

// calculateBackoffDelay calculates exponential backoff delay
func calculateBackoffDelay(attempt int, baseDelay time.Duration) time.Duration {
	// 指数バックオフ: baseDelay * 2^attempt
	delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay

	// 最大遅延時間を制限（5秒）
	maxDelay := 5 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	// ジッターを追加（±25%）
	jitter := time.Duration(rand.Float64()*0.5-0.25) * delay
	return delay + jitter
}

// orderResourceIDs sorts resource IDs consistently
func orderResourceIDs(ids []string) []string {
	result := make([]string, len(ids))
	copy(result, ids)
	sort.Strings(result)
	return result
}

// orderAccountIDs sorts account IDs consistently
func orderAccountIDs(ids []int) []int {
	result := make([]int, len(ids))
	copy(result, ids)
	sort.Ints(result)
	return result
}

// Database helper functions

// transferMoney performs a basic money transfer (potentially deadlock-prone)
func transferMoney(db *sql.DB, fromID, toID int, amount float64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	return transferMoneyInTx(tx, fromID, toID, amount)
}

// transferMoneyInTx performs money transfer within existing transaction
func transferMoneyInTx(tx *sql.Tx, fromID, toID int, amount float64) error {
	// 送金元から減額
	_, err := tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", amount, fromID)
	if err != nil {
		return err
	}

	// 送金先に加算
	_, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", amount, toID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// getAccountForUpdate gets account with FOR UPDATE lock
func getAccountForUpdate(tx *sql.Tx, accountID int) (*Account, error) {
	var account Account
	err := tx.QueryRow("SELECT id, balance, version FROM accounts WHERE id = $1 FOR UPDATE", accountID).
		Scan(&account.ID, &account.Balance, &account.Version)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// updateAccountBalance updates account balance
func updateAccountBalance(tx *sql.Tx, accountID int, newBalance float64) error {
	_, err := tx.Exec("UPDATE accounts SET balance = $1 WHERE id = $2", newBalance, accountID)
	return err
}

// Database initialization functions

// InitializeDeadlockTestDB creates tables for deadlock testing
func InitializeDeadlockTestDB(db *sql.DB) error {
	schema := `
	DROP TABLE IF EXISTS accounts;
	
	CREATE TABLE accounts (
		id SERIAL PRIMARY KEY,
		balance DECIMAL(10, 2) DEFAULT 0 CHECK (balance >= 0),
		version INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX idx_accounts_id ON accounts(id);
	`

	_, err := db.Exec(schema)
	return err
}

// CleanupDeadlockTestDB drops test tables
func CleanupDeadlockTestDB(db *sql.DB) error {
	_, err := db.Exec("DROP TABLE IF EXISTS accounts")
	return err
}

// Advanced deadlock scenarios

// DeadlockScenarioConfig configures deadlock simulation
type DeadlockScenarioConfig struct {
	NumAccounts     int
	NumTransactions int
	TransferAmount  float64
	DelayBetweenOps time.Duration
}

// RunDeadlockStressTest runs stress test to induce deadlocks
func RunDeadlockStressTest(db *sql.DB, config DeadlockScenarioConfig) (*DeadlockStats, error) {
	monitor := NewDeadlockMonitor()
	detector := NewDeadlockDetector(db, 3, monitor)

	var wg sync.WaitGroup
	errorChan := make(chan error, config.NumTransactions)

	// 大量の並行トランザクションを実行
	for i := 0; i < config.NumTransactions; i++ {
		wg.Add(1)
		go func(transactionID int) {
			defer wg.Done()

			fromID := (transactionID % config.NumAccounts) + 1
			toID := ((transactionID + 1) % config.NumAccounts) + 1

			err := detector.ExecuteWithRetry(func(tx *sql.Tx) error {
				return transferMoneyInTx(tx, fromID, toID, config.TransferAmount)
			})

			errorChan <- err
		}(i)

		// トランザクション間の遅延
		if config.DelayBetweenOps > 0 {
			time.Sleep(config.DelayBetweenOps)
		}
	}

	wg.Wait()
	close(errorChan)

	// エラーカウント
	errorCount := 0
	for err := range errorChan {
		if err != nil {
			errorCount++
		}
	}

	stats := monitor.GetStatistics()
	if errorCount > 0 {
		return &stats, fmt.Errorf("stress test completed with %d errors", errorCount)
	}

	return &stats, nil
}

func main() {
	// デモンストレーション用のコード

	// データベースに接続
	db, err := sql.Open("postgres", "postgres://postgres:test@localhost:5432/testdb?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// テストデータベースの初期化
	if err := InitializeDeadlockTestDB(db); err != nil {
		panic(err)
	}

	// テストアカウントの作成
	accounts := []struct{ id int; balance float64 }{
		{1, 1000.0},
		{2, 1000.0},
	}

	for _, acc := range accounts {
		_, err := db.Exec("INSERT INTO accounts (id, balance, version) VALUES ($1, $2, 0)", acc.id, acc.balance)
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("=== Deadlock Prevention Demo ===")

	// デッドロック予防デモ
	preventer := NewDeadlockPreventer(db)
	
	fmt.Println("Performing ordered transfers...")
	err = preventer.TransferMoneyOrdered(1, 2, 100.0)
	if err != nil {
		fmt.Printf("Transfer failed: %v\n", err)
	} else {
		fmt.Println("Transfer completed successfully")
	}

	// デッドロック検出とリトライデモ
	monitor := NewDeadlockMonitor()
	detector := NewDeadlockDetector(db, 3, monitor)

	fmt.Println("\nPerforming transfer with deadlock detection...")
	err = detector.ExecuteWithRetry(func(tx *sql.Tx) error {
		return transferMoneyInTx(tx, 2, 1, 50.0)
	})

	if err != nil {
		fmt.Printf("Transfer with retry failed: %v\n", err)
	} else {
		fmt.Println("Transfer with retry completed successfully")
	}

	// 統計表示
	stats := monitor.GetStatistics()
	fmt.Printf("\nDeadlock Statistics:\n")
	fmt.Printf("- Deadlocks detected: %d\n", stats.DeadlockCount)
	fmt.Printf("- Total retries: %d\n", stats.TotalRetries)
	fmt.Printf("- Successful retries: %d\n", stats.SuccessfulRetries)
	fmt.Printf("- Retry success rate: %.2f%%\n", stats.RetrySuccessRate*100)

	fmt.Println("Deadlock prevention demo completed!")
}