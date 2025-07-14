package main

import (
	"database/sql"
	"fmt"
	"log"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	_ "github.com/lib/pq"
)

var testDB *sqlx.DB
var testDSN string

func TestMain(m *testing.M) {
	// Setup dockertest
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// Start PostgreSQL container
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "13",
		Env: []string{
			"POSTGRES_PASSWORD=secret",
			"POSTGRES_USER=test",
			"POSTGRES_DB=testdb",
			"listen_addresses = '*'",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	// Set expiration for the container
	if err := resource.Expire(120); err != nil {
		log.Fatalf("Could not set expiration: %s", err)
	}

	testDSN = fmt.Sprintf("postgres://test:secret@localhost:%s/testdb?sslmode=disable", resource.GetPort("5432/tcp"))

	// Connect to database
	if err := pool.Retry(func() error {
		var err error
		testDB, err = sqlx.Open("postgres", testDSN)
		if err != nil {
			return err
		}
		return testDB.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	// Setup test schema
	if err := setupDatabase(testDB); err != nil {
		log.Fatalf("Could not setup test database: %s", err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	testDB.Close()
	
	// Exit with the test result code
	if code != 0 {
		log.Fatalf("Tests failed with code: %d", code)
	}
}

func TestJSONB_ScanAndValue(t *testing.T) {
	// Test Scan
	jsonData := []byte(`{"name": "test", "value": 123}`)
	var j JSONB
	
	err := j.Scan(jsonData)
	if err != nil {
		t.Errorf("Failed to scan JSONB: %v", err)
	}

	if j["name"] != "test" {
		t.Errorf("Expected name=test, got %v", j["name"])
	}

	if j["value"] != float64(123) { // JSON numbers are float64
		t.Errorf("Expected value=123, got %v", j["value"])
	}

	// Test Value
	value, err := j.Value()
	if err != nil {
		t.Errorf("Failed to get JSONB value: %v", err)
	}

	if value == nil {
		t.Error("Expected non-nil value")
	}

	// Test nil scan
	var j2 JSONB
	err = j2.Scan(nil)
	if err != nil {
		t.Errorf("Failed to scan nil JSONB: %v", err)
	}
	if j2 != nil {
		t.Error("Expected nil JSONB after scanning nil")
	}
}

func TestUserRepository_CRUD(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	if err := helper.TruncateAll(); err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	userRepo := NewUserRepository(testDB)

	// Test Create
	age := 25
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   &age,
		City:  "Tokyo",
	}

	err := userRepo.Create(user)
	if err != nil {
		t.Errorf("Failed to create user: %v", err)
	}

	if user.ID == 0 {
		t.Error("Expected user ID to be set after creation")
	}

	if user.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	// Test GetByID
	retrievedUser, err := userRepo.GetByID(user.ID)
	if err != nil {
		t.Errorf("Failed to get user by ID: %v", err)
	}

	if retrievedUser.Name != user.Name {
		t.Errorf("Expected name %s, got %s", user.Name, retrievedUser.Name)
	}

	if retrievedUser.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, retrievedUser.Email)
	}

	// Test GetByEmail
	userByEmail, err := userRepo.GetByEmail(user.Email)
	if err != nil {
		t.Errorf("Failed to get user by email: %v", err)
	}

	if userByEmail.ID != user.ID {
		t.Errorf("Expected ID %d, got %d", user.ID, userByEmail.ID)
	}

	// Test Update
	user.Name = "Jane Doe"
	user.City = "Osaka"
	err = userRepo.Update(user)
	if err != nil {
		t.Errorf("Failed to update user: %v", err)
	}

	updatedUser, err := userRepo.GetByID(user.ID)
	if err != nil {
		t.Errorf("Failed to get updated user: %v", err)
	}

	if updatedUser.Name != "Jane Doe" {
		t.Errorf("Expected updated name Jane Doe, got %s", updatedUser.Name)
	}

	// Test Delete
	err = userRepo.Delete(user.ID)
	if err != nil {
		t.Errorf("Failed to delete user: %v", err)
	}

	_, err = userRepo.GetByID(user.ID)
	if err != sql.ErrNoRows {
		t.Error("Expected user to be deleted")
	}
}

func TestUserRepository_BatchInsert(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	if err := helper.TruncateAll(); err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	userRepo := NewUserRepository(testDB)

	users := []User{
		{Name: "User 1", Email: "user1@example.com", City: "Tokyo"},
		{Name: "User 2", Email: "user2@example.com", City: "Osaka"},
		{Name: "User 3", Email: "user3@example.com", City: "Kyoto"},
	}

	err := userRepo.BatchInsert(users)
	if err != nil {
		t.Errorf("Failed to batch insert users: %v", err)
	}

	// Verify users were inserted
	allUsers, err := userRepo.GetAll(10, 0)
	if err != nil {
		t.Errorf("Failed to get all users: %v", err)
	}

	if len(allUsers) != 3 {
		t.Errorf("Expected 3 users, got %d", len(allUsers))
	}
}

func TestUserRepository_GetByIDs(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	if err := helper.TruncateAll(); err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	// Seed test data
	users, err := helper.SeedUsers(5)
	if err != nil {
		t.Fatalf("Failed to seed users: %v", err)
	}

	userRepo := NewUserRepository(testDB)

	// Test getting multiple users
	ids := []int{users[0].ID, users[2].ID, users[4].ID}
	retrievedUsers, err := userRepo.GetByIDs(ids)
	if err != nil {
		t.Errorf("Failed to get users by IDs: %v", err)
	}

	if len(retrievedUsers) != 3 {
		t.Errorf("Expected 3 users, got %d", len(retrievedUsers))
	}

	// Test empty slice
	emptyUsers, err := userRepo.GetByIDs([]int{})
	if err != nil {
		t.Errorf("Failed to get users with empty IDs: %v", err)
	}

	if len(emptyUsers) != 0 {
		t.Errorf("Expected 0 users for empty IDs, got %d", len(emptyUsers))
	}
}

func TestOrderRepository_Advanced(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	if err := helper.TruncateAll(); err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	// Seed users
	users, err := helper.SeedUsers(3)
	if err != nil {
		t.Fatalf("Failed to seed users: %v", err)
	}

	orderRepo := NewOrderRepository(testDB)

	// Test Create with JSONB
	items := JSONB{
		"product_id": "prod_123",
		"quantity":   2,
		"price":      99.99,
	}

	order := &Order{
		UserID: users[0].ID,
		Amount: 199.98,
		Status: "pending",
		Items:  items,
	}

	err = orderRepo.Create(order)
	if err != nil {
		t.Errorf("Failed to create order: %v", err)
	}

	if order.ID == 0 {
		t.Error("Expected order ID to be set")
	}

	// Test GetByID
	retrievedOrder, err := orderRepo.GetByID(order.ID)
	if err != nil {
		t.Errorf("Failed to get order by ID: %v", err)
	}

	if retrievedOrder.Items["product_id"] != "prod_123" {
		t.Error("JSONB items not properly retrieved")
	}

	// Test GetByUserID
	userOrders, err := orderRepo.GetByUserID(users[0].ID)
	if err != nil {
		t.Errorf("Failed to get orders by user ID: %v", err)
	}

	if len(userOrders) != 1 {
		t.Errorf("Expected 1 order, got %d", len(userOrders))
	}

	// Test UpdateStatus
	err = orderRepo.UpdateStatus(order.ID, "completed")
	if err != nil {
		t.Errorf("Failed to update order status: %v", err)
	}

	// Test GetByStatus
	completedOrders, err := orderRepo.GetByStatus("completed")
	if err != nil {
		t.Errorf("Failed to get orders by status: %v", err)
	}

	if len(completedOrders) != 1 {
		t.Errorf("Expected 1 completed order, got %d", len(completedOrders))
	}

	// Test GetOrderSummary
	summary, err := orderRepo.GetOrderSummary(order.ID)
	if err != nil {
		t.Errorf("Failed to get order summary: %v", err)
	}

	if summary.UserName != users[0].Name {
		t.Errorf("Expected user name %s, got %s", users[0].Name, summary.UserName)
	}

	if summary.Status != "completed" {
		t.Errorf("Expected status completed, got %s", summary.Status)
	}
}

func TestTransactionService_Transfer(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	if err := helper.TruncateAll(); err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	// Seed users and accounts
	users, err := helper.SeedUsers(2)
	if err != nil {
		t.Fatalf("Failed to seed users: %v", err)
	}

	userIDs := []int{users[0].ID, users[1].ID}
	_, err = helper.SeedAccounts(userIDs, 1000.0)
	if err != nil {
		t.Fatalf("Failed to seed accounts: %v", err)
	}

	txService := NewTransactionService(testDB)
	accountRepo := NewAccountRepository(testDB)

	// Test successful transfer
	err = txService.Transfer(users[0].ID, users[1].ID, 200.0)
	if err != nil {
		t.Errorf("Failed to transfer money: %v", err)
	}

	// Check balances
	fromAccount, err := accountRepo.GetByUserID(users[0].ID)
	if err != nil {
		t.Errorf("Failed to get sender account: %v", err)
	}

	if fromAccount.Balance != 800.0 {
		t.Errorf("Expected sender balance 800.0, got %f", fromAccount.Balance)
	}

	toAccount, err := accountRepo.GetByUserID(users[1].ID)
	if err != nil {
		t.Errorf("Failed to get receiver account: %v", err)
	}

	if toAccount.Balance != 1200.0 {
		t.Errorf("Expected receiver balance 1200.0, got %f", toAccount.Balance)
	}

	// Test insufficient balance
	err = txService.Transfer(users[0].ID, users[1].ID, 1000.0)
	if err == nil {
		t.Error("Expected error for insufficient balance")
	}

	// Test negative amount
	err = txService.Transfer(users[0].ID, users[1].ID, -100.0)
	if err == nil {
		t.Error("Expected error for negative amount")
	}
}

func TestTransactionService_CreateOrderWithAccount(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	if err := helper.TruncateAll(); err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	// Seed users and accounts
	users, err := helper.SeedUsers(1)
	if err != nil {
		t.Fatalf("Failed to seed users: %v", err)
	}

	userIDs := []int{users[0].ID}
	_, err = helper.SeedAccounts(userIDs, 500.0)
	if err != nil {
		t.Fatalf("Failed to seed accounts: %v", err)
	}

	txService := NewTransactionService(testDB)
	accountRepo := NewAccountRepository(testDB)

	// Test successful order creation
	items := JSONB{
		"product_id": "prod_456",
		"quantity":   1,
		"price":      150.0,
	}

	order, err := txService.CreateOrderWithAccount(users[0].ID, 150.0, items)
	if err != nil {
		t.Errorf("Failed to create order with account: %v", err)
	}

	if order.ID == 0 {
		t.Error("Expected order ID to be set")
	}

	if order.Status != "pending" {
		t.Errorf("Expected order status pending, got %s", order.Status)
	}

	// Check account balance was updated
	account, err := accountRepo.GetByUserID(users[0].ID)
	if err != nil {
		t.Errorf("Failed to get account: %v", err)
	}

	if account.Balance != 350.0 {
		t.Errorf("Expected account balance 350.0, got %f", account.Balance)
	}

	// Test insufficient balance
	_, err = txService.CreateOrderWithAccount(users[0].ID, 400.0, items)
	if err == nil {
		t.Error("Expected error for insufficient balance")
	}
}

func TestQueryBuilder_Dynamic(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	if err := helper.TruncateAll(); err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	// Seed test data
	users, err := helper.SeedUsers(5)
	if err != nil {
		t.Fatalf("Failed to seed users: %v", err)
	}

	qb := NewQueryBuilder(testDB)

	// Test basic query building
	query, args := qb.Select("*").
		From("users").
		Where("city = $1", "Tokyo").
		OrderBy("created_at DESC").
		Limit(3).
		Build()

	expectedQuery := "SELECT * FROM users WHERE city = $1 ORDER BY created_at DESC LIMIT 3"
	if query != expectedQuery {
		t.Errorf("Expected query %s, got %s", expectedQuery, query)
	}

	if len(args) != 1 || args[0] != "Tokyo" {
		t.Errorf("Expected args [Tokyo], got %v", args)
	}

	// Test query execution
	var results []User
	err = qb.Execute(&results)
	if err != nil {
		t.Errorf("Failed to execute query: %v", err)
	}

	// Check that all results are from Tokyo
	for _, user := range results {
		if user.City != "Tokyo" {
			t.Errorf("Expected city Tokyo, got %s", user.City)
		}
	}

	// Test complex query with JOIN
	qb2 := NewQueryBuilder(testDB)
	query2, args2 := qb2.Select("u.name, COUNT(o.id) as order_count").
		From("users u").
		Join("LEFT JOIN orders o ON u.id = o.user_id").
		Where("u.city = $1", "Tokyo").
		Build()

	expectedQuery2 := "SELECT u.name, COUNT(o.id) as order_count FROM users u LEFT JOIN orders o ON u.id = o.user_id WHERE u.city = $1"
	if query2 != expectedQuery2 {
		t.Errorf("Expected query %s, got %s", expectedQuery2, query2)
	}

	if len(args2) != 1 || args2[0] != "Tokyo" {
		t.Errorf("Expected args [Tokyo], got %v", args2)
	}
}

func TestMigrationRunner_Schema(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	// Create a separate database connection for migration testing
	migrationDB, err := sqlx.Open("postgres", testDSN)
	if err != nil {
		t.Fatalf("Failed to open migration database: %v", err)
	}
	defer migrationDB.Close()

	// Drop all tables for clean testing
	_, err = migrationDB.Exec(`
		DROP TABLE IF EXISTS transfers CASCADE;
		DROP TABLE IF EXISTS orders CASCADE;
		DROP TABLE IF EXISTS accounts CASCADE;
		DROP TABLE IF EXISTS users CASCADE;
		DROP TABLE IF EXISTS schema_migrations CASCADE;
	`)
	if err != nil {
		t.Fatalf("Failed to drop tables: %v", err)
	}

	mr := NewMigrationRunner(migrationDB)

	// Add default migrations
	migrations := getDefaultMigrations()
	for _, migration := range migrations {
		mr.AddMigration(migration)
	}

	// Test initial version
	version, err := mr.GetCurrentVersion()
	if err != nil {
		t.Errorf("Failed to get current version: %v", err)
	}

	if version != 0 {
		t.Errorf("Expected initial version 0, got %d", version)
	}

	// Test running migrations
	err = mr.RunMigrations()
	if err != nil {
		t.Errorf("Failed to run migrations: %v", err)
	}

	// Check final version
	finalVersion, err := mr.GetCurrentVersion()
	if err != nil {
		t.Errorf("Failed to get final version: %v", err)
	}

	expectedVersion := 5 // Number of default migrations
	if finalVersion != expectedVersion {
		t.Errorf("Expected final version %d, got %d", expectedVersion, finalVersion)
	}

	// Verify tables were created
	var tableCount int
	err = migrationDB.Get(&tableCount, 
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name IN ('users', 'accounts', 'orders', 'transfers')")
	if err != nil {
		t.Errorf("Failed to count tables: %v", err)
	}

	if tableCount != 4 {
		t.Errorf("Expected 4 tables, got %d", tableCount)
	}

	// Test rollback
	err = mr.RollbackMigration()
	if err != nil {
		t.Errorf("Failed to rollback migration: %v", err)
	}

	rollbackVersion, err := mr.GetCurrentVersion()
	if err != nil {
		t.Errorf("Failed to get version after rollback: %v", err)
	}

	if rollbackVersion != expectedVersion-1 {
		t.Errorf("Expected version %d after rollback, got %d", expectedVersion-1, rollbackVersion)
	}
}

func TestTestHelper_Integration(t *testing.T) {
	if testDB == nil {
		t.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)

	// Test TruncateAll
	err := helper.TruncateAll()
	if err != nil {
		t.Errorf("Failed to truncate all tables: %v", err)
	}

	// Test SeedUsers
	users, err := helper.SeedUsers(3)
	if err != nil {
		t.Errorf("Failed to seed users: %v", err)
	}

	if len(users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(users))
	}

	// Verify users have IDs
	for i, user := range users {
		if user.ID == 0 {
			t.Errorf("User %d has no ID", i)
		}
		if user.Name == "" {
			t.Errorf("User %d has no name", i)
		}
		if user.Email == "" {
			t.Errorf("User %d has no email", i)
		}
	}

	// Test SeedAccounts
	userIDs := make([]int, len(users))
	for i, user := range users {
		userIDs[i] = user.ID
	}

	accounts, err := helper.SeedAccounts(userIDs, 1000.0)
	if err != nil {
		t.Errorf("Failed to seed accounts: %v", err)
	}

	if len(accounts) != 3 {
		t.Errorf("Expected 3 accounts, got %d", len(accounts))
	}

	// Test SeedOrders
	orders, err := helper.SeedOrders(userIDs, 2)
	if err != nil {
		t.Errorf("Failed to seed orders: %v", err)
	}

	expectedOrderCount := len(userIDs) * 2
	if len(orders) != expectedOrderCount {
		t.Errorf("Expected %d orders, got %d", expectedOrderCount, len(orders))
	}

	// Verify orders have valid data
	for i, order := range orders {
		if order.ID == 0 {
			t.Errorf("Order %d has no ID", i)
		}
		if order.UserID == 0 {
			t.Errorf("Order %d has no user ID", i)
		}
		if order.Amount <= 0 {
			t.Errorf("Order %d has invalid amount: %f", i, order.Amount)
		}
		if order.Items == nil {
			t.Errorf("Order %d has no items", i)
		}
	}
}

func BenchmarkUserRepository_GetByID(b *testing.B) {
	if testDB == nil {
		b.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	helper.TruncateAll()

	users, err := helper.SeedUsers(100)
	if err != nil {
		b.Fatal(err)
	}

	userRepo := NewUserRepository(testDB)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			userID := users[b.N%len(users)].ID
			_, err := userRepo.GetByID(userID)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

func BenchmarkUserRepository_BatchInsert(b *testing.B) {
	if testDB == nil {
		b.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	userRepo := NewUserRepository(testDB)

	// Prepare test data
	users := make([]User, 100)
	for i := 0; i < 100; i++ {
		users[i] = User{
			Name:  fmt.Sprintf("User %d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			City:  "Tokyo",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		helper.TruncateAll()
		err := userRepo.BatchInsert(users)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkQueryBuilder_Complex(b *testing.B) {
	if testDB == nil {
		b.Skip("Database not available")
	}

	helper := NewTestHelper(testDB)
	helper.TruncateAll()
	helper.SeedUsers(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb := NewQueryBuilder(testDB)
		var results []User
		err := qb.Select("*").
			From("users").
			Where("city = $1", "Tokyo").
			OrderBy("created_at DESC").
			Limit(10).
			Execute(&results)
		if err != nil {
			b.Error(err)
		}
	}
}