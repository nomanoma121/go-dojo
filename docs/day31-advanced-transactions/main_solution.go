package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// User represents a user entity with optimistic locking
type User struct {
	ID      int
	Name    string
	Email   string
	Version int
}

// Account represents an account entity with optimistic locking
type Account struct {
	ID      int
	UserID  int
	Balance float64
	Version int
}

// TransactionManager manages database transactions
type TransactionManager struct {
	db *sql.DB
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(db *sql.DB) *TransactionManager {
	return &TransactionManager{
		db: db,
	}
}

// ExecuteInTransaction executes a function within a transaction
func (tm *TransactionManager) ExecuteInTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("transaction error: %v, rollback error: %v", err, rbErr)
			}
		}
	}()

	err = fn(tx)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// WithSavepoint executes operations within a savepoint
func (tm *TransactionManager) WithSavepoint(tx *sql.Tx, savepointName string, fn func(*sql.Tx) error) error {
	// セーブポイントを作成
	_, err := tx.Exec(fmt.Sprintf("SAVEPOINT %s", savepointName))
	if err != nil {
		return fmt.Errorf("failed to create savepoint %s: %w", savepointName, err)
	}

	err = fn(tx)
	if err != nil {
		// エラーが発生した場合はセーブポイントまでロールバック
		_, rbErr := tx.Exec(fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", savepointName))
		if rbErr != nil {
			return fmt.Errorf("savepoint operation error: %v, rollback to savepoint error: %v", err, rbErr)
		}
		return err
	}

	// 成功時はセーブポイントを解放
	_, err = tx.Exec(fmt.Sprintf("RELEASE SAVEPOINT %s", savepointName))
	if err != nil {
		return fmt.Errorf("failed to release savepoint %s: %w", savepointName, err)
	}

	return nil
}

// TransferMoney transfers money between user accounts
func (tm *TransactionManager) TransferMoney(ctx context.Context, fromUserID, toUserID int, amount float64) error {
	return tm.ExecuteInTransaction(ctx, func(tx *sql.Tx) error {
		// 送金元アカウント取得（排他ロック）
		var fromBalance float64
		err := tx.QueryRowContext(ctx, 
			"SELECT balance FROM accounts WHERE user_id = $1 FOR UPDATE", 
			fromUserID,
		).Scan(&fromBalance)
		if err != nil {
			return fmt.Errorf("failed to get from account: %w", err)
		}

		// 送金先アカウント取得（排他ロック）
		var toBalance float64
		err = tx.QueryRowContext(ctx,
			"SELECT balance FROM accounts WHERE user_id = $1 FOR UPDATE",
			toUserID,
		).Scan(&toBalance)
		if err != nil {
			return fmt.Errorf("failed to get to account: %w", err)
		}

		// 残高チェック
		if fromBalance < amount {
			return fmt.Errorf("insufficient funds")
		}

		// 送金元の残高を減額
		_, err = tx.ExecContext(ctx,
			"UPDATE accounts SET balance = balance - $1 WHERE user_id = $2",
			amount, fromUserID,
		)
		if err != nil {
			return fmt.Errorf("failed to update from account: %w", err)
		}

		// 送金先の残高を増額
		_, err = tx.ExecContext(ctx,
			"UPDATE accounts SET balance = balance + $1 WHERE user_id = $2",
			amount, toUserID,
		)
		if err != nil {
			return fmt.Errorf("failed to update to account: %w", err)
		}

		return nil
	})
}

// CreateUser creates a new user
func (tm *TransactionManager) CreateUser(ctx context.Context, name, email string) (*User, error) {
	var user *User
	err := tm.ExecuteInTransaction(ctx, func(tx *sql.Tx) error {
		// ユーザーを作成
		var userID int
		err := tx.QueryRowContext(ctx,
			"INSERT INTO users (name, email, version) VALUES ($1, $2, 0) RETURNING id",
			name, email,
		).Scan(&userID)
		if err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		// 初期アカウントを作成
		_, err = tx.ExecContext(ctx,
			"INSERT INTO accounts (user_id, balance, version) VALUES ($1, 0, 0)",
			userID,
		)
		if err != nil {
			return fmt.Errorf("failed to create initial account: %w", err)
		}

		user = &User{
			ID:      userID,
			Name:    name,
			Email:   email,
			Version: 0,
		}

		return nil
	})
	return user, err
}

// BulkOperation performs bulk operations with batch commits
func (tm *TransactionManager) BulkOperation(ctx context.Context, users []User, batchSize int) error {
	totalUsers := len(users)
	batches := int(math.Ceil(float64(totalUsers) / float64(batchSize)))

	for i := 0; i < batches; i++ {
		start := i * batchSize
		end := start + batchSize
		if end > totalUsers {
			end = totalUsers
		}

		batch := users[start:end]
		err := tm.ExecuteInTransaction(ctx, func(tx *sql.Tx) error {
			for _, user := range batch {
				_, err := tx.ExecContext(ctx,
					"INSERT INTO users (name, email, version) VALUES ($1, $2, 0)",
					user.Name, user.Email,
				)
				if err != nil {
					return fmt.Errorf("failed to insert user %s: %w", user.Name, err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to process batch %d: %w", i+1, err)
		}
	}

	return nil
}

// UpdateUserWithOptimisticLock updates user with optimistic locking
func (tm *TransactionManager) UpdateUserWithOptimisticLock(ctx context.Context, user *User, newName string) error {
	return tm.ExecuteInTransaction(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			"UPDATE users SET name = $1, version = version + 1 WHERE id = $2 AND version = $3",
			newName, user.ID, user.Version,
		)
		if err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to check affected rows: %w", err)
		}

		if affected == 0 {
			return fmt.Errorf("optimistic lock error: data was modified by another transaction")
		}

		user.Name = newName
		user.Version++
		return nil
	})
}

// ExecuteWithDeadlockRetry executes operation with deadlock retry
func (tm *TransactionManager) ExecuteWithDeadlockRetry(ctx context.Context, maxRetries int, fn func(*sql.Tx) error) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := tm.ExecuteInTransaction(ctx, fn)
		if err == nil {
			return nil
		}

		// デッドロックエラーかチェック
		if isDeadlockError(err) && attempt < maxRetries-1 {
			// 指数バックオフで待機
			backoffDuration := time.Duration(attempt+1) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDuration):
				continue
			}
		}
		return err
	}
	return fmt.Errorf("transaction failed after %d retries", maxRetries)
}

// isDeadlockError checks if error is a deadlock error
func isDeadlockError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// PostgreSQLのデッドロックエラーコード: 40P01
	return strings.Contains(errStr, "40p01") || 
		   strings.Contains(errStr, "deadlock detected") ||
		   strings.Contains(errStr, "deadlock")
}

// GetUser retrieves a user by ID
func (tm *TransactionManager) GetUser(ctx context.Context, userID int) (*User, error) {
	var user User
	err := tm.db.QueryRowContext(ctx,
		"SELECT id, name, email, version FROM users WHERE id = $1",
		userID,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetAccount retrieves an account by user ID
func (tm *TransactionManager) GetAccount(ctx context.Context, userID int) (*Account, error) {
	var account Account
	err := tm.db.QueryRowContext(ctx,
		"SELECT id, user_id, balance, version FROM accounts WHERE user_id = $1",
		userID,
	).Scan(&account.ID, &account.UserID, &account.Balance, &account.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}
	return &account, nil
}

// setupDatabase initializes the database schema
func setupDatabase(db *sql.DB) error {
	schema := `
	DROP TABLE IF EXISTS accounts;
	DROP TABLE IF EXISTS users;
	
	CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		version INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE accounts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
		balance DECIMAL(10, 2) DEFAULT 0 CHECK (balance >= 0),
		version INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX idx_accounts_user_id ON accounts(user_id);
	`

	_, err := db.Exec(schema)
	return err
}

func main() {
	// データベースに接続
	db, err := sql.Open("postgres", "postgres://postgres:test@localhost:5432/testdb?sslmode=disable")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// スキーマを初期化
	if err := setupDatabase(db); err != nil {
		panic(err)
	}

	// TransactionManagerを作成
	tm := NewTransactionManager(db)

	ctx := context.Background()

	fmt.Println("=== Advanced Transaction Management Demo ===")

	// ユーザーを作成
	user1, err := tm.CreateUser(ctx, "Alice", "alice@example.com")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created user 1: %+v\n", user1)

	user2, err := tm.CreateUser(ctx, "Bob", "bob@example.com")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created user 2: %+v\n", user2)

	// 初期残高を設定
	_, err = db.Exec("UPDATE accounts SET balance = 1000 WHERE user_id = $1", user1.ID)
	if err != nil {
		panic(err)
	}

	// 送金テスト
	fmt.Println("\nTransferring $100 from Alice to Bob...")
	err = tm.TransferMoney(ctx, user1.ID, user2.ID, 100)
	if err != nil {
		panic(err)
	}

	// 残高確認
	account1, err := tm.GetAccount(ctx, user1.ID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Alice's account: balance=%.2f\n", account1.Balance)

	account2, err := tm.GetAccount(ctx, user2.ID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Bob's account: balance=%.2f\n", account2.Balance)

	// 楽観的ロックのテスト
	fmt.Println("\nTesting optimistic locking...")
	err = tm.UpdateUserWithOptimisticLock(ctx, user1, "Alice Updated")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Updated user: %+v\n", user1)

	fmt.Println("\nTransaction management demo completed!")
}