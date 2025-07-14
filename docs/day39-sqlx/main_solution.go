package main

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// User represents a user entity
type User struct {
	ID        int       `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Age       *int      `db:"age"`
	City      string    `db:"city"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// Order represents an order entity
type Order struct {
	ID        int       `db:"id"`
	UserID    int       `db:"user_id"`
	Amount    float64   `db:"amount"`
	Status    string    `db:"status"`
	Items     JSONB     `db:"items"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// Account represents a user account with balance
type Account struct {
	ID        int       `db:"id"`
	UserID    int       `db:"user_id"`
	Balance   float64   `db:"balance"`
	Currency  string    `db:"currency"`
	CreatedAt time.Time `db:"created_at"`
}

// Transfer represents a money transfer
type Transfer struct {
	ID         int       `db:"id"`
	FromUserID int       `db:"from_user_id"`
	ToUserID   int       `db:"to_user_id"`
	Amount     float64   `db:"amount"`
	CreatedAt  time.Time `db:"created_at"`
}

// JSONB represents a PostgreSQL JSONB field
type JSONB map[string]interface{}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("cannot scan into JSONB: value is not []byte")
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	result := make(map[string]interface{})
	err := json.Unmarshal(bytes, &result)
	if err != nil {
		return err
	}

	*j = result
	return nil
}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// UserRepository handles user database operations
type UserRepository struct {
	db *sqlx.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetByID retrieves a user by ID
func (ur *UserRepository) GetByID(id int) (*User, error) {
	var user User
	err := ur.db.Get(&user, "SELECT * FROM users WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail retrieves a user by email
func (ur *UserRepository) GetByEmail(email string) (*User, error) {
	var user User
	err := ur.db.Get(&user, "SELECT * FROM users WHERE email = $1", email)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetAll retrieves all users with pagination
func (ur *UserRepository) GetAll(limit, offset int) ([]User, error) {
	var users []User
	err := ur.db.Select(&users, 
		"SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2", 
		limit, offset)
	return users, err
}

// GetByIDs retrieves multiple users by their IDs
func (ur *UserRepository) GetByIDs(ids []int) ([]User, error) {
	if len(ids) == 0 {
		return []User{}, nil
	}

	query, args, err := sqlx.In("SELECT * FROM users WHERE id IN (?)", ids)
	if err != nil {
		return nil, err
	}

	query = ur.db.Rebind(query)
	var users []User
	err = ur.db.Select(&users, query, args...)
	return users, err
}

// Create creates a new user
func (ur *UserRepository) Create(user *User) error {
	query := `
		INSERT INTO users (name, email, age, city) 
		VALUES (:name, :email, :age, :city) 
		RETURNING id, created_at, updated_at`

	stmt, err := ur.db.PrepareNamed(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	return stmt.Get(user, user)
}

// Update updates an existing user
func (ur *UserRepository) Update(user *User) error {
	query := `
		UPDATE users 
		SET name = :name, email = :email, age = :age, city = :city, updated_at = NOW()
		WHERE id = :id
		RETURNING updated_at`

	stmt, err := ur.db.PrepareNamed(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	return stmt.Get(user, user)
}

// Delete deletes a user
func (ur *UserRepository) Delete(id int) error {
	_, err := ur.db.Exec("DELETE FROM users WHERE id = $1", id)
	return err
}

// BatchInsert inserts multiple users efficiently
func (ur *UserRepository) BatchInsert(users []User) error {
	if len(users) == 0 {
		return nil
	}

	tx, err := ur.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareNamed(`
		INSERT INTO users (name, email, age, city) 
		VALUES (:name, :email, :age, :city)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, user := range users {
		if _, err := stmt.Exec(user); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// OrderRepository handles order database operations
type OrderRepository struct {
	db *sqlx.DB
}

// NewOrderRepository creates a new order repository
func NewOrderRepository(db *sqlx.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// GetByID retrieves an order by ID
func (or *OrderRepository) GetByID(id int) (*Order, error) {
	var order Order
	err := or.db.Get(&order, "SELECT * FROM orders WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// GetByUserID retrieves orders for a specific user
func (or *OrderRepository) GetByUserID(userID int) ([]Order, error) {
	var orders []Order
	err := or.db.Select(&orders, 
		"SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC", 
		userID)
	return orders, err
}

// GetByStatus retrieves orders by status
func (or *OrderRepository) GetByStatus(status string) ([]Order, error) {
	var orders []Order
	err := or.db.Select(&orders, 
		"SELECT * FROM orders WHERE status = $1 ORDER BY created_at DESC", 
		status)
	return orders, err
}

// Create creates a new order
func (or *OrderRepository) Create(order *Order) error {
	query := `
		INSERT INTO orders (user_id, amount, status, items) 
		VALUES (:user_id, :amount, :status, :items) 
		RETURNING id, created_at, updated_at`

	stmt, err := or.db.PrepareNamed(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	return stmt.Get(order, order)
}

// UpdateStatus updates order status
func (or *OrderRepository) UpdateStatus(id int, status string) error {
	_, err := or.db.Exec(
		"UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2", 
		status, id)
	return err
}

// GetOrderSummary retrieves order summary with user information
func (or *OrderRepository) GetOrderSummary(orderID int) (*OrderSummary, error) {
	var summary OrderSummary
	query := `
		SELECT 
			o.id as order_id,
			u.name as user_name,
			u.email as user_email,
			o.amount,
			o.status,
			COALESCE(jsonb_array_length(o.items), 0) as item_count,
			o.created_at
		FROM orders o
		JOIN users u ON o.user_id = u.id
		WHERE o.id = $1`

	err := or.db.Get(&summary, query, orderID)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// OrderSummary represents an order with user information
type OrderSummary struct {
	OrderID   int       `db:"order_id"`
	UserName  string    `db:"user_name"`
	UserEmail string    `db:"user_email"`
	Amount    float64   `db:"amount"`
	Status    string    `db:"status"`
	ItemCount int       `db:"item_count"`
	CreatedAt time.Time `db:"created_at"`
}

// TransactionService handles complex database transactions
type TransactionService struct {
	db          *sqlx.DB
	userRepo    *UserRepository
	orderRepo   *OrderRepository
	accountRepo *AccountRepository
}

// NewTransactionService creates a new transaction service
func NewTransactionService(db *sqlx.DB) *TransactionService {
	return &TransactionService{
		db:          db,
		userRepo:    NewUserRepository(db),
		orderRepo:   NewOrderRepository(db),
		accountRepo: NewAccountRepository(db),
	}
}

// Transfer performs money transfer between accounts
func (ts *TransactionService) Transfer(fromUserID, toUserID int, amount float64) error {
	if amount <= 0 {
		return errors.New("transfer amount must be positive")
	}

	tx, err := ts.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get sender's account for update
	var fromAccount Account
	err = tx.Get(&fromAccount, 
		"SELECT * FROM accounts WHERE user_id = $1 FOR UPDATE", 
		fromUserID)
	if err != nil {
		return fmt.Errorf("failed to get sender account: %w", err)
	}

	if fromAccount.Balance < amount {
		return errors.New("insufficient balance")
	}

	// Get receiver's account for update
	var toAccount Account
	err = tx.Get(&toAccount, 
		"SELECT * FROM accounts WHERE user_id = $1 FOR UPDATE", 
		toUserID)
	if err != nil {
		return fmt.Errorf("failed to get receiver account: %w", err)
	}

	// Update sender's balance
	_, err = tx.Exec(
		"UPDATE accounts SET balance = balance - $1 WHERE user_id = $2",
		amount, fromUserID)
	if err != nil {
		return fmt.Errorf("failed to update sender balance: %w", err)
	}

	// Update receiver's balance
	_, err = tx.Exec(
		"UPDATE accounts SET balance = balance + $1 WHERE user_id = $2",
		amount, toUserID)
	if err != nil {
		return fmt.Errorf("failed to update receiver balance: %w", err)
	}

	// Record transfer
	_, err = tx.NamedExec(`
		INSERT INTO transfers (from_user_id, to_user_id, amount)
		VALUES (:from_user_id, :to_user_id, :amount)`,
		map[string]interface{}{
			"from_user_id": fromUserID,
			"to_user_id":   toUserID,
			"amount":       amount,
		})
	if err != nil {
		return fmt.Errorf("failed to record transfer: %w", err)
	}

	return tx.Commit()
}

// CreateOrderWithAccount creates an order and updates account balance
func (ts *TransactionService) CreateOrderWithAccount(userID int, orderAmount float64, items JSONB) (*Order, error) {
	if orderAmount <= 0 {
		return nil, errors.New("order amount must be positive")
	}

	tx, err := ts.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check account balance
	var account Account
	err = tx.Get(&account, 
		"SELECT * FROM accounts WHERE user_id = $1 FOR UPDATE", 
		userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	if account.Balance < orderAmount {
		return nil, errors.New("insufficient balance")
	}

	// Create order
	order := &Order{
		UserID: userID,
		Amount: orderAmount,
		Status: "pending",
		Items:  items,
	}

	err = tx.Get(order, `
		INSERT INTO orders (user_id, amount, status, items) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id, created_at, updated_at`,
		order.UserID, order.Amount, order.Status, order.Items)
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// Update account balance
	_, err = tx.Exec(
		"UPDATE accounts SET balance = balance - $1 WHERE user_id = $2",
		orderAmount, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to update account balance: %w", err)
	}

	return order, tx.Commit()
}

// AccountRepository handles account database operations
type AccountRepository struct {
	db *sqlx.DB
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *sqlx.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// GetByUserID retrieves account by user ID
func (ar *AccountRepository) GetByUserID(userID int) (*Account, error) {
	var account Account
	err := ar.db.Get(&account, "SELECT * FROM accounts WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// UpdateBalance updates account balance
func (ar *AccountRepository) UpdateBalance(userID int, amount float64) error {
	_, err := ar.db.Exec(
		"UPDATE accounts SET balance = balance + $1 WHERE user_id = $2",
		amount, userID)
	return err
}

// QueryBuilder helps build dynamic SQL queries
type QueryBuilder struct {
	db        *sqlx.DB
	query     strings.Builder
	args      []interface{}
	namedArgs map[string]interface{}
	argCount  int
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(db *sqlx.DB) *QueryBuilder {
	return &QueryBuilder{
		db:        db,
		args:      make([]interface{}, 0),
		namedArgs: make(map[string]interface{}),
	}
}

// Select starts a SELECT query
func (qb *QueryBuilder) Select(fields string) *QueryBuilder {
	qb.query.WriteString("SELECT ")
	qb.query.WriteString(fields)
	return qb
}

// From adds FROM clause
func (qb *QueryBuilder) From(table string) *QueryBuilder {
	qb.query.WriteString(" FROM ")
	qb.query.WriteString(table)
	return qb
}

// Where adds WHERE clause
func (qb *QueryBuilder) Where(condition string, args ...interface{}) *QueryBuilder {
	if strings.Contains(qb.query.String(), "WHERE") {
		qb.query.WriteString(" AND ")
	} else {
		qb.query.WriteString(" WHERE ")
	}
	qb.query.WriteString(condition)
	qb.args = append(qb.args, args...)
	return qb
}

// Join adds JOIN clause
func (qb *QueryBuilder) Join(joinClause string) *QueryBuilder {
	qb.query.WriteString(" ")
	qb.query.WriteString(joinClause)
	return qb
}

// OrderBy adds ORDER BY clause
func (qb *QueryBuilder) OrderBy(orderClause string) *QueryBuilder {
	qb.query.WriteString(" ORDER BY ")
	qb.query.WriteString(orderClause)
	return qb
}

// Limit adds LIMIT clause
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.query.WriteString(fmt.Sprintf(" LIMIT %d", limit))
	return qb
}

// Build builds the final query
func (qb *QueryBuilder) Build() (string, []interface{}) {
	return qb.query.String(), qb.args
}

// Execute executes the query and returns results
func (qb *QueryBuilder) Execute(dest interface{}) error {
	query, args := qb.Build()
	return qb.db.Select(dest, query, args...)
}

// MigrationRunner handles database schema migrations
type MigrationRunner struct {
	db         *sqlx.DB
	migrations []Migration
}

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	UpSQL       string
	DownSQL     string
}

// NewMigrationRunner creates a new migration runner
func NewMigrationRunner(db *sqlx.DB) *MigrationRunner {
	mr := &MigrationRunner{
		db:         db,
		migrations: make([]Migration, 0),
	}

	// Create migrations table if it doesn't exist
	mr.createMigrationsTable()
	return mr
}

func (mr *MigrationRunner) createMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	_, err := mr.db.Exec(query)
	return err
}

// AddMigration adds a migration to the runner
func (mr *MigrationRunner) AddMigration(migration Migration) {
	mr.migrations = append(mr.migrations, migration)
	// Sort migrations by version
	sort.Slice(mr.migrations, func(i, j int) bool {
		return mr.migrations[i].Version < mr.migrations[j].Version
	})
}

// RunMigrations executes all pending migrations
func (mr *MigrationRunner) RunMigrations() error {
	currentVersion, err := mr.GetCurrentVersion()
	if err != nil {
		return err
	}

	for _, migration := range mr.migrations {
		if migration.Version <= currentVersion {
			continue // Skip already executed migrations
		}

		tx, err := mr.db.Beginx()
		if err != nil {
			return err
		}

		// Execute migration
		_, err = tx.Exec(migration.UpSQL)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", migration.Version, err)
		}

		// Record migration
		_, err = tx.Exec(
			"INSERT INTO schema_migrations (version, description) VALUES ($1, $2)",
			migration.Version, migration.Description)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
		}
	}

	return nil
}

// RollbackMigration rolls back the last migration
func (mr *MigrationRunner) RollbackMigration() error {
	currentVersion, err := mr.GetCurrentVersion()
	if err != nil {
		return err
	}

	if currentVersion == 0 {
		return errors.New("no migrations to rollback")
	}

	// Find the migration to rollback
	var migrationToRollback Migration
	found := false
	for _, migration := range mr.migrations {
		if migration.Version == currentVersion {
			migrationToRollback = migration
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("migration %d not found", currentVersion)
	}

	tx, err := mr.db.Beginx()
	if err != nil {
		return err
	}

	// Execute rollback
	_, err = tx.Exec(migrationToRollback.DownSQL)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("rollback failed: %w", err)
	}

	// Remove migration record
	_, err = tx.Exec("DELETE FROM schema_migrations WHERE version = $1", currentVersion)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	return tx.Commit()
}

// GetCurrentVersion returns the current schema version
func (mr *MigrationRunner) GetCurrentVersion() (int, error) {
	var version int
	err := mr.db.Get(&version, 
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err != nil {
		return 0, err
	}
	return version, nil
}

// TestHelper provides utility functions for testing
type TestHelper struct {
	db *sqlx.DB
}

// NewTestHelper creates a new test helper
func NewTestHelper(db *sqlx.DB) *TestHelper {
	return &TestHelper{db: db}
}

// TruncateAll truncates all tables for testing
func (th *TestHelper) TruncateAll() error {
	tables := []string{"transfers", "orders", "accounts", "users"}

	tx, err := th.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, table := range tables {
		_, err := tx.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// SeedUsers inserts test users
func (th *TestHelper) SeedUsers(count int) ([]User, error) {
	users := make([]User, 0, count)
	cities := []string{"Tokyo", "Osaka", "Yokohama", "Nagoya", "Sapporo"}

	tx, err := th.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for i := 1; i <= count; i++ {
		user := User{
			Name:  fmt.Sprintf("User %d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			City:  cities[i%len(cities)],
		}

		if i%3 != 0 { // Some users have age, some don't
			age := 20 + (i % 50)
			user.Age = &age
		}

		err := tx.Get(&user, `
			INSERT INTO users (name, email, age, city) 
			VALUES ($1, $2, $3, $4) 
			RETURNING id, created_at, updated_at`,
			user.Name, user.Email, user.Age, user.City)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	return users, tx.Commit()
}

// SeedOrders inserts test orders
func (th *TestHelper) SeedOrders(userIDs []int, ordersPerUser int) ([]Order, error) {
	if len(userIDs) == 0 {
		return []Order{}, nil
	}

	orders := make([]Order, 0, len(userIDs)*ordersPerUser)
	statuses := []string{"pending", "completed", "cancelled", "processing"}

	tx, err := th.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, userID := range userIDs {
		for i := 0; i < ordersPerUser; i++ {
			items := JSONB{
				"product_id": fmt.Sprintf("prod_%d", i+1),
				"quantity":   i + 1,
				"price":      float64(100 + i*50),
			}

			order := Order{
				UserID: userID,
				Amount: float64(100 + i*50),
				Status: statuses[i%len(statuses)],
				Items:  items,
			}

			err := tx.Get(&order, `
				INSERT INTO orders (user_id, amount, status, items) 
				VALUES ($1, $2, $3, $4) 
				RETURNING id, created_at, updated_at`,
				order.UserID, order.Amount, order.Status, order.Items)
			if err != nil {
				return nil, err
			}

			orders = append(orders, order)
		}
	}

	return orders, tx.Commit()
}

// SeedAccounts inserts test accounts
func (th *TestHelper) SeedAccounts(userIDs []int, initialBalance float64) ([]Account, error) {
	if len(userIDs) == 0 {
		return []Account{}, nil
	}

	accounts := make([]Account, 0, len(userIDs))

	tx, err := th.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, userID := range userIDs {
		account := Account{
			UserID:   userID,
			Balance:  initialBalance,
			Currency: "USD",
		}

		err := tx.Get(&account, `
			INSERT INTO accounts (user_id, balance, currency) 
			VALUES ($1, $2, $3) 
			RETURNING id, created_at`,
			account.UserID, account.Balance, account.Currency)
		if err != nil {
			return nil, err
		}

		accounts = append(accounts, account)
	}

	return accounts, tx.Commit()
}

// setupDatabase creates the database schema
func setupDatabase(db *sqlx.DB) error {
	schema := `
		DROP TABLE IF EXISTS transfers CASCADE;
		DROP TABLE IF EXISTS orders CASCADE;
		DROP TABLE IF EXISTS accounts CASCADE;
		DROP TABLE IF EXISTS users CASCADE;
		DROP TABLE IF EXISTS schema_migrations CASCADE;

		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INTEGER,
			city VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE accounts (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			balance DECIMAL(15,2) DEFAULT 0.00,
			currency VARCHAR(3) DEFAULT 'USD',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			amount DECIMAL(10,2) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			items JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE transfers (
			id SERIAL PRIMARY KEY,
			from_user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			to_user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			amount DECIMAL(10,2) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX idx_users_email ON users(email);
		CREATE INDEX idx_accounts_user_id ON accounts(user_id);
		CREATE INDEX idx_orders_user_id ON orders(user_id);
		CREATE INDEX idx_orders_status ON orders(status);
		CREATE INDEX idx_transfers_from_user ON transfers(from_user_id);
		CREATE INDEX idx_transfers_to_user ON transfers(to_user_id);
	`

	_, err := db.Exec(schema)
	return err
}

// getDefaultMigrations returns the default set of migrations
func getDefaultMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Create users table",
			UpSQL: `
				CREATE TABLE users (
					id SERIAL PRIMARY KEY,
					name VARCHAR(255) NOT NULL,
					email VARCHAR(255) UNIQUE NOT NULL,
					age INTEGER,
					city VARCHAR(100),
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);
			`,
			DownSQL: "DROP TABLE users;",
		},
		{
			Version:     2,
			Description: "Create accounts table",
			UpSQL: `
				CREATE TABLE accounts (
					id SERIAL PRIMARY KEY,
					user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
					balance DECIMAL(15,2) DEFAULT 0.00,
					currency VARCHAR(3) DEFAULT 'USD',
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);
			`,
			DownSQL: "DROP TABLE accounts;",
		},
		{
			Version:     3,
			Description: "Create orders table",
			UpSQL: `
				CREATE TABLE orders (
					id SERIAL PRIMARY KEY,
					user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
					amount DECIMAL(10,2) NOT NULL,
					status VARCHAR(50) DEFAULT 'pending',
					items JSONB,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);
			`,
			DownSQL: "DROP TABLE orders;",
		},
		{
			Version:     4,
			Description: "Create transfers table",
			UpSQL: `
				CREATE TABLE transfers (
					id SERIAL PRIMARY KEY,
					from_user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
					to_user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
					amount DECIMAL(10,2) NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				);
			`,
			DownSQL: "DROP TABLE transfers;",
		},
		{
			Version:     5,
			Description: "Add indexes",
			UpSQL: `
				CREATE INDEX idx_users_email ON users(email);
				CREATE INDEX idx_accounts_user_id ON accounts(user_id);
				CREATE INDEX idx_orders_user_id ON orders(user_id);
				CREATE INDEX idx_orders_status ON orders(status);
				CREATE INDEX idx_transfers_from_user ON transfers(from_user_id);
				CREATE INDEX idx_transfers_to_user ON transfers(to_user_id);
			`,
			DownSQL: `
				DROP INDEX idx_users_email;
				DROP INDEX idx_accounts_user_id;
				DROP INDEX idx_orders_user_id;
				DROP INDEX idx_orders_status;
				DROP INDEX idx_transfers_from_user;
				DROP INDEX idx_transfers_to_user;
			`,
		},
	}
}