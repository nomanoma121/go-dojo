package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

var (
	testDB   *sql.DB
	pool     *dockertest.Pool
	resource *dockertest.Resource
)

func TestMain(m *testing.M) {
	// Setup test database
	setupTestDB()
	defer teardownTestDB()
	
	// Run tests
	m.Run()
}

func setupTestDB() {
	var err error
	pool, err = dockertest.NewPool("")
	if err != nil {
		panic(fmt.Sprintf("Could not connect to docker: %s", err))
	}

	// Start PostgreSQL container
	resource, err = pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15",
		Env: []string{
			"POSTGRES_PASSWORD=test",
			"POSTGRES_DB=testdb",
			"POSTGRES_USER=postgres",
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

	// Wait for database to be ready
	pool.MaxWait = 120 * time.Second
	if err = pool.Retry(func() error {
		var err error
		testDB, err = sql.Open("postgres", databaseUrl)
		if err != nil {
			return err
		}
		return testDB.Ping()
	}); err != nil {
		panic(fmt.Sprintf("Could not connect to database: %s", err))
	}

	// Setup schema
	if err := setupDatabase(testDB); err != nil {
		panic(fmt.Sprintf("Could not setup database: %s", err))
	}
}

func teardownTestDB() {
	if testDB != nil {
		testDB.Close()
	}
	if pool != nil && resource != nil {
		pool.Purge(resource)
	}
}

func TestTransactionManager_ExecuteInTransaction(t *testing.T) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	t.Run("Successful transaction", func(t *testing.T) {
		err := tm.ExecuteInTransaction(ctx, func(tx *sql.Tx) error {
			_, err := tx.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", "Alice", "alice@test.com")
			return err
		})
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Verify data was inserted
		var count int
		err = testDB.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", "alice@test.com").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 user, got %d", count)
		}
	})

	t.Run("Failed transaction rollback", func(t *testing.T) {
		err := tm.ExecuteInTransaction(ctx, func(tx *sql.Tx) error {
			_, err := tx.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", "Bob", "bob@test.com")
			if err != nil {
				return err
			}
			// Force an error to trigger rollback
			return fmt.Errorf("simulated error")
		})
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		// Verify data was not inserted due to rollback
		var count int
		err = testDB.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", "bob@test.com").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 users (rollback), got %d", count)
		}
	})
}

func TestTransactionManager_WithSavepoint(t *testing.T) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	err := tm.ExecuteInTransaction(ctx, func(tx *sql.Tx) error {
		// Insert first user
		_, err := tx.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", "Charlie", "charlie@test.com")
		if err != nil {
			return err
		}

		// Use savepoint for risky operation
		err = tm.WithSavepoint(tx, "sp1", func(tx *sql.Tx) error {
			_, err := tx.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", "David", "invalid-email")
			return err // This might fail due to constraints
		})
		// Even if savepoint fails, the transaction should continue

		// Insert another user
		_, err = tx.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", "Eve", "eve@test.com")
		return err
	})

	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify Charlie and Eve were inserted
	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM users WHERE email IN ($1, $2)", "charlie@test.com", "eve@test.com").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 users, got %d", count)
	}
}

func TestTransactionManager_CreateUser(t *testing.T) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	user, err := tm.CreateUser(ctx, "Frank", "frank@test.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.ID == 0 {
		t.Error("User ID should be set")
	}
	if user.Name != "Frank" {
		t.Errorf("Expected name 'Frank', got '%s'", user.Name)
	}
	if user.Email != "frank@test.com" {
		t.Errorf("Expected email 'frank@test.com', got '%s'", user.Email)
	}
	if user.Version != 0 {
		t.Errorf("Expected version 0, got %d", user.Version)
	}

	// Verify account was also created
	account, err := tm.GetAccount(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get account: %v", err)
	}
	if account.UserID != user.ID {
		t.Errorf("Expected user ID %d, got %d", user.ID, account.UserID)
	}
	if account.Balance != 0 {
		t.Errorf("Expected balance 0, got %f", account.Balance)
	}
}

func TestTransactionManager_UpdateUserWithOptimisticLock(t *testing.T) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	// Create a user
	user, err := tm.CreateUser(ctx, "Grace", "grace@test.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	t.Run("Successful update", func(t *testing.T) {
		err := tm.UpdateUserWithOptimisticLock(ctx, user, "Grace Updated")
		if err != nil {
			t.Fatalf("Failed to update user: %v", err)
		}

		// Verify version was incremented
		if user.Version != 1 {
			t.Errorf("Expected version 1, got %d", user.Version)
		}

		// Verify name was updated
		updatedUser, err := tm.GetUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if updatedUser.Name != "Grace Updated" {
			t.Errorf("Expected name 'Grace Updated', got '%s'", updatedUser.Name)
		}
	})

	t.Run("Optimistic lock conflict", func(t *testing.T) {
		// Create another user instance with old version
		oldUser := &User{
			ID:      user.ID,
			Name:    user.Name,
			Email:   user.Email,
			Version: 0, // Old version
		}

		err := tm.UpdateUserWithOptimisticLock(ctx, oldUser, "Should Fail")
		if err == nil {
			t.Fatal("Expected optimistic lock error, got nil")
		}
	})
}

func TestTransactionManager_TransferMoney(t *testing.T) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	// Create two users with accounts
	user1, err := tm.CreateUser(ctx, "Sender", "sender@test.com")
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2, err := tm.CreateUser(ctx, "Receiver", "receiver@test.com")
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Set initial balance for user1
	_, err = testDB.Exec("UPDATE accounts SET balance = 1000 WHERE user_id = $1", user1.ID)
	if err != nil {
		t.Fatalf("Failed to set initial balance: %v", err)
	}

	t.Run("Successful transfer", func(t *testing.T) {
		err := tm.TransferMoney(ctx, user1.ID, user2.ID, 100)
		if err != nil {
			t.Fatalf("Failed to transfer money: %v", err)
		}

		// Verify balances
		account1, err := tm.GetAccount(ctx, user1.ID)
		if err != nil {
			t.Fatalf("Failed to get account1: %v", err)
		}
		if account1.Balance != 900 {
			t.Errorf("Expected balance 900, got %f", account1.Balance)
		}

		account2, err := tm.GetAccount(ctx, user2.ID)
		if err != nil {
			t.Fatalf("Failed to get account2: %v", err)
		}
		if account2.Balance != 100 {
			t.Errorf("Expected balance 100, got %f", account2.Balance)
		}
	})

	t.Run("Insufficient funds", func(t *testing.T) {
		err := tm.TransferMoney(ctx, user1.ID, user2.ID, 2000) // More than available
		if err == nil {
			t.Fatal("Expected insufficient funds error, got nil")
		}
	})
}

func TestTransactionManager_BulkOperation(t *testing.T) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	// Prepare test data
	users := make([]User, 10)
	for i := 0; i < 10; i++ {
		users[i] = User{
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@test.com", i),
		}
	}

	err := tm.BulkOperation(ctx, users, 3) // Batch size 3
	if err != nil {
		t.Fatalf("Bulk operation failed: %v", err)
	}

	// Verify all users were created
	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM users WHERE email LIKE 'user%@test.com'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	if count != 10 {
		t.Errorf("Expected 10 users, got %d", count)
	}
}

func TestTransactionManager_ExecuteWithDeadlockRetry(t *testing.T) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	t.Run("Successful operation", func(t *testing.T) {
		var executed bool
		err := tm.ExecuteWithDeadlockRetry(ctx, 3, func(tx *sql.Tx) error {
			executed = true
			_, err := tx.Exec("INSERT INTO users (name, email) VALUES ($1, $2)", "Retry User", "retry@test.com")
			return err
		})

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !executed {
			t.Error("Operation should have been executed")
		}
	})

	t.Run("Retry on simulated deadlock", func(t *testing.T) {
		attemptCount := 0
		err := tm.ExecuteWithDeadlockRetry(ctx, 3, func(tx *sql.Tx) error {
			attemptCount++
			if attemptCount < 3 {
				// Simulate deadlock error
				return fmt.Errorf("deadlock detected")
			}
			// Success on third attempt
			return nil
		})

		if err != nil {
			t.Fatalf("Expected no error after retries, got %v", err)
		}
		if attemptCount != 3 {
			t.Errorf("Expected 3 attempts, got %d", attemptCount)
		}
	})

	t.Run("Max retries exceeded", func(t *testing.T) {
		err := tm.ExecuteWithDeadlockRetry(ctx, 2, func(tx *sql.Tx) error {
			return fmt.Errorf("deadlock detected")
		})

		if err == nil {
			t.Fatal("Expected error after max retries, got nil")
		}
	})
}

func TestConcurrentTransactions(t *testing.T) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	// Create a user for concurrent testing
	user, err := tm.CreateUser(ctx, "Concurrent User", "concurrent@test.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	const numGoroutines = 10
	const updatesPerGoroutine = 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*updatesPerGoroutine)

	// Run concurrent updates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < updatesPerGoroutine; j++ {
				// Get current user data
				currentUser, err := tm.GetUser(ctx, user.ID)
				if err != nil {
					errors <- err
					return
				}

				newName := fmt.Sprintf("User-G%d-U%d", goroutineID, j)
				err = tm.UpdateUserWithOptimisticLock(ctx, currentUser, newName)
				if err != nil {
					// Optimistic lock conflicts are expected in concurrent scenarios
					if !isOptimisticLockError(err) {
						errors <- err
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for unexpected errors
	for err := range errors {
		t.Errorf("Unexpected error in concurrent test: %v", err)
	}

	// Verify final state
	finalUser, err := tm.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get final user state: %v", err)
	}

	// Version should be > 0 due to successful updates
	if finalUser.Version == 0 {
		t.Error("Expected version > 0 after concurrent updates")
	}
}

func isOptimisticLockError(err error) bool {
	return err != nil && (
		fmt.Sprintf("%v", err) == "optimistic lock error: data was modified by another transaction" ||
		fmt.Sprintf("%v", err) == "no rows affected")
}

func TestIsDeadlockError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "PostgreSQL deadlock error code",
			err:      fmt.Errorf("pq: deadlock detected (SQLSTATE 40P01)"),
			expected: true,
		},
		{
			name:     "Generic deadlock message",
			err:      fmt.Errorf("deadlock detected"),
			expected: true,
		},
		{
			name:     "Other error",
			err:      fmt.Errorf("connection timeout"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDeadlockError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for error: %v", tt.expected, result, tt.err)
			}
		})
	}
}

// Benchmark tests
func BenchmarkTransactionManager_CreateUser(b *testing.B) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tm.CreateUser(ctx, fmt.Sprintf("BenchUser%d", i), fmt.Sprintf("bench%d@test.com", i))
		if err != nil {
			b.Fatalf("Failed to create user: %v", err)
		}
	}
}

func BenchmarkTransactionManager_UpdateUser(b *testing.B) {
	tm := NewTransactionManager(testDB)
	ctx := context.Background()

	// Create a user for benchmarking
	user, err := tm.CreateUser(ctx, "Bench User", "benchupdate@test.com")
	if err != nil {
		b.Fatalf("Failed to create user: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Get current user (to get latest version)
		currentUser, err := tm.GetUser(ctx, user.ID)
		if err != nil {
			b.Fatalf("Failed to get user: %v", err)
		}

		err = tm.UpdateUserWithOptimisticLock(ctx, currentUser, fmt.Sprintf("Updated%d", i))
		if err != nil {
			b.Fatalf("Failed to update user: %v", err)
		}
	}
}