package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	_ "github.com/lib/pq"
)

var testPrimaryDB *sqlx.DB
var testReplicaDB *sqlx.DB
var testPrimaryDSN string
var testReplicaDSN string

func TestMain(m *testing.M) {
	// Setup dockertest
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// Start Primary PostgreSQL container
	primaryResource, err := pool.RunWithOptions(&dockertest.RunOptions{
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
		log.Fatalf("Could not start primary resource: %s", err)
	}

	// Start Replica PostgreSQL container (simulated)
	replicaResource, err := pool.RunWithOptions(&dockertest.RunOptions{
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
		log.Fatalf("Could not start replica resource: %s", err)
	}

	// Set expiration for containers
	if err := primaryResource.Expire(120); err != nil {
		log.Fatalf("Could not set expiration for primary: %s", err)
	}
	if err := replicaResource.Expire(120); err != nil {
		log.Fatalf("Could not set expiration for replica: %s", err)
	}

	testPrimaryDSN = fmt.Sprintf("postgres://test:secret@localhost:%s/testdb?sslmode=disable", primaryResource.GetPort("5432/tcp"))
	testReplicaDSN = fmt.Sprintf("postgres://test:secret@localhost:%s/testdb?sslmode=disable", replicaResource.GetPort("5432/tcp"))

	// Connect to primary database
	if err := pool.Retry(func() error {
		var err error
		testPrimaryDB, err = sqlx.Open("postgres", testPrimaryDSN)
		if err != nil {
			return err
		}
		return testPrimaryDB.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to primary database: %s", err)
	}

	// Connect to replica database
	if err := pool.Retry(func() error {
		var err error
		testReplicaDB, err = sqlx.Open("postgres", testReplicaDSN)
		if err != nil {
			return err
		}
		return testReplicaDB.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to replica database: %s", err)
	}

	// Setup schema for both databases
	if err := setupDatabase(testPrimaryDB); err != nil {
		log.Fatalf("Could not setup primary database: %s", err)
	}
	if err := setupDatabase(testReplicaDB); err != nil {
		log.Fatalf("Could not setup replica database: %s", err)
	}

	// Seed test data
	if err := seedTestData(testPrimaryDB, 100); err != nil {
		log.Fatalf("Could not seed primary test data: %s", err)
	}
	if err := seedTestData(testReplicaDB, 100); err != nil {
		log.Fatalf("Could not seed replica test data: %s", err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	if err := pool.Purge(primaryResource); err != nil {
		log.Fatalf("Could not purge primary resource: %s", err)
	}
	if err := pool.Purge(replicaResource); err != nil {
		log.Fatalf("Could not purge replica resource: %s", err)
	}

	testPrimaryDB.Close()
	testReplicaDB.Close()
	
	// Exit with the test result code
	if code != 0 {
		log.Fatalf("Tests failed with code: %d", code)
	}
}

func TestDBCluster_BasicOperations(t *testing.T) {
	if testPrimaryDB == nil || testReplicaDB == nil {
		t.Skip("Databases not available")
	}

	cluster, err := NewDBCluster(testPrimaryDSN, []string{testReplicaDSN})
	if err != nil {
		t.Fatalf("Failed to create DB cluster: %v", err)
	}
	defer cluster.Close()

	// Test GetPrimary
	primary := cluster.GetPrimary()
	if primary == nil {
		t.Error("Expected primary database, got nil")
	}

	// Test ping primary
	if err := primary.Ping(); err != nil {
		t.Errorf("Failed to ping primary: %v", err)
	}

	// Test GetReplica
	replica := cluster.GetReplica()
	if replica == nil {
		t.Error("Expected replica database, got nil")
	}

	// Test ping replica
	if err := replica.Ping(); err != nil {
		t.Errorf("Failed to ping replica: %v", err)
	}

	// Test GetHealthyReplicas
	healthyReplicas := cluster.GetHealthyReplicas()
	if len(healthyReplicas) == 0 {
		t.Error("Expected at least one healthy replica")
	}
}

func TestRoundRobinStrategy_SelectReplica(t *testing.T) {
	strategy := NewRoundRobinStrategy()
	metrics := NewRoutingMetrics()

	// Create mock replicas (we'll use the same DB for simplicity)
	replicas := []*sqlx.DB{testReplicaDB, testReplicaDB, testReplicaDB}

	// Test round-robin selection
	selections := make(map[*sqlx.DB]int)
	for i := 0; i < 9; i++ {
		selected := strategy.SelectReplica(replicas, metrics)
		if selected == nil {
			t.Error("Expected replica, got nil")
		}
		selections[selected]++
	}

	// Since we're using the same DB instance, all selections should go to the same instance
	if len(selections) != 1 {
		t.Errorf("Expected 1 unique selection, got %d", len(selections))
	}

	// Test with empty replicas
	emptySelected := strategy.SelectReplica([]*sqlx.DB{}, metrics)
	if emptySelected != nil {
		t.Error("Expected nil for empty replicas")
	}
}

func TestWeightedStrategy_SelectReplica(t *testing.T) {
	weights := []int{3, 2, 1}
	strategy := NewWeightedStrategy(weights)
	metrics := NewRoutingMetrics()

	// Create mock replicas
	replicas := []*sqlx.DB{testReplicaDB, testReplicaDB, testReplicaDB}

	// Test weighted selection
	for i := 0; i < 10; i++ {
		selected := strategy.SelectReplica(replicas, metrics)
		if selected == nil {
			t.Error("Expected replica, got nil")
		}
	}

	// Test with mismatched weights and replicas
	wrongWeightStrategy := NewWeightedStrategy([]int{1, 2})
	selected := wrongWeightStrategy.SelectReplica(replicas, metrics)
	if selected == nil {
		t.Error("Expected fallback selection, got nil")
	}

	// Test with empty replicas
	emptySelected := strategy.SelectReplica([]*sqlx.DB{}, metrics)
	if emptySelected != nil {
		t.Error("Expected nil for empty replicas")
	}
}

func TestRoutingMetrics_Operations(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Test initial state
	reads, writes, errors := metrics.GetStats()
	if reads != 0 || writes != 0 || errors != 0 {
		t.Error("Expected initial stats to be zero")
	}

	// Test recording operations
	metrics.RecordRead(testReplicaDB, 10*time.Millisecond)
	metrics.RecordWrite(5*time.Millisecond)
	metrics.RecordError()

	reads, writes, errors = metrics.GetStats()
	if reads != 1 {
		t.Errorf("Expected 1 read, got %d", reads)
	}
	if writes != 1 {
		t.Errorf("Expected 1 write, got %d", writes)
	}
	if errors != 1 {
		t.Errorf("Expected 1 error, got %d", errors)
	}

	// Test concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			metrics.RecordRead(testReplicaDB, time.Millisecond)
		}()
		go func() {
			defer wg.Done()
			metrics.RecordWrite(time.Millisecond)
		}()
		go func() {
			defer wg.Done()
			metrics.RecordError()
		}()
	}
	wg.Wait()

	reads, writes, errors = metrics.GetStats()
	if reads != 101 {
		t.Errorf("Expected 101 reads, got %d", reads)
	}
	if writes != 101 {
		t.Errorf("Expected 101 writes, got %d", writes)
	}
	if errors != 101 {
		t.Errorf("Expected 101 errors, got %d", errors)
	}
}

func TestRoutingManager_ReadWriteSplit(t *testing.T) {
	if testPrimaryDB == nil || testReplicaDB == nil {
		t.Skip("Databases not available")
	}

	cluster, err := NewDBCluster(testPrimaryDSN, []string{testReplicaDSN})
	if err != nil {
		t.Fatalf("Failed to create DB cluster: %v", err)
	}
	defer cluster.Close()

	strategy := NewRoundRobinStrategy()
	router := NewRoutingManager(cluster, strategy)

	ctx := context.Background()

	// Test read routing
	readDB := router.RouteRead(ctx)
	if readDB == nil {
		t.Error("Expected read database, got nil")
	}

	// Test write routing
	writeDB := router.RouteWrite(ctx)
	if writeDB == nil {
		t.Error("Expected write database, got nil")
	}

	// Write should always go to primary
	if writeDB != cluster.GetPrimary() {
		t.Error("Write should be routed to primary")
	}
}

func TestHealthMonitor_FailureDetection(t *testing.T) {
	if testPrimaryDB == nil || testReplicaDB == nil {
		t.Skip("Databases not available")
	}

	cluster, err := NewDBCluster(testPrimaryDSN, []string{testReplicaDSN})
	if err != nil {
		t.Fatalf("Failed to create DB cluster: %v", err)
	}
	defer cluster.Close()

	monitor := NewHealthMonitor(cluster, 100*time.Millisecond)

	// Test initial health status
	if !monitor.IsHealthy(cluster.GetPrimary()) {
		t.Error("Primary should be healthy initially")
	}

	// Test getting healthy replicas
	healthyReplicas := monitor.GetHealthyReplicas()
	if len(healthyReplicas) == 0 {
		t.Error("Expected at least one healthy replica")
	}

	// Start monitoring
	monitor.Start()
	defer monitor.Stop()

	// Wait for a health check cycle
	time.Sleep(200 * time.Millisecond)

	// Verify that health monitoring is working
	if !monitor.IsHealthy(cluster.GetPrimary()) {
		t.Error("Primary should still be healthy after monitoring")
	}
}

func TestLagDetector_ReplicationMonitoring(t *testing.T) {
	if testPrimaryDB == nil || testReplicaDB == nil {
		t.Skip("Databases not available")
	}

	cluster, err := NewDBCluster(testPrimaryDSN, []string{testReplicaDSN})
	if err != nil {
		t.Fatalf("Failed to create DB cluster: %v", err)
	}
	defer cluster.Close()

	lagDetector := NewLagDetector(cluster, 100*time.Millisecond)
	ctx := context.Background()

	// Test checking replication lag
	lagMap, err := lagDetector.CheckReplicationLag(ctx)
	if err != nil {
		t.Errorf("Failed to check replication lag: %v", err)
	}

	if len(lagMap) == 0 {
		t.Error("Expected lag information for replicas")
	}

	// Test getting low-lag replicas
	lowLagReplicas, err := lagDetector.GetLowLagReplicas(ctx)
	if err != nil {
		t.Errorf("Failed to get low-lag replicas: %v", err)
	}

	// Since we're using separate databases (not actual replicas), 
	// they should all be considered low-lag
	if len(lowLagReplicas) == 0 {
		t.Error("Expected at least one low-lag replica")
	}
}

func TestLoadBalancer_ReplicaSelection(t *testing.T) {
	strategy := NewRoundRobinStrategy()
	loadBalancer := NewLoadBalancer(strategy)

	// Create mock replicas
	replicas := []*sqlx.DB{testReplicaDB, testReplicaDB}

	// Test replica selection
	selected := loadBalancer.SelectReplica(replicas)
	if selected == nil {
		t.Error("Expected selected replica, got nil")
	}

	// Test with empty replicas
	emptySelected := loadBalancer.SelectReplica([]*sqlx.DB{})
	if emptySelected != nil {
		t.Error("Expected nil for empty replicas")
	}
}

func TestFailoverManager_Integration(t *testing.T) {
	if testPrimaryDB == nil || testReplicaDB == nil {
		t.Skip("Databases not available")
	}

	cluster, err := NewDBCluster(testPrimaryDSN, []string{testReplicaDSN})
	if err != nil {
		t.Fatalf("Failed to create DB cluster: %v", err)
	}
	defer cluster.Close()

	healthMonitor := NewHealthMonitor(cluster, 100*time.Millisecond)
	failoverManager := NewFailoverManager(cluster, healthMonitor)

	// Test promoting a replica
	replicas := cluster.GetHealthyReplicas()
	if len(replicas) == 0 {
		t.Skip("No healthy replicas for failover test")
	}

	originalPrimary := cluster.GetPrimary()
	err = failoverManager.PromoteReplica(replicas[0])
	if err != nil {
		t.Errorf("Failed to promote replica: %v", err)
	}

	// Verify primary has changed
	newPrimary := cluster.GetPrimary()
	if newPrimary == originalPrimary {
		t.Error("Primary should have changed after promotion")
	}
}

func TestUserService_CRUD(t *testing.T) {
	if testPrimaryDB == nil || testReplicaDB == nil {
		t.Skip("Databases not available")
	}

	cluster, err := NewDBCluster(testPrimaryDSN, []string{testReplicaDSN})
	if err != nil {
		t.Fatalf("Failed to create DB cluster: %v", err)
	}
	defer cluster.Close()

	strategy := NewRoundRobinStrategy()
	router := NewRoutingManager(cluster, strategy)
	userService := NewUserService(router)

	ctx := context.Background()

	// Clear existing data
	cluster.GetPrimary().Exec("DELETE FROM users")
	testReplicaDB.Exec("DELETE FROM users")

	// Test CreateUser (write operation)
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	err = userService.CreateUser(ctx, user)
	if err != nil {
		t.Errorf("Failed to create user: %v", err)
	}

	if user.ID == 0 {
		t.Error("Expected user ID to be set after creation")
	}

	// Sync data to replica for testing (in real scenario, this would be automatic)
	testReplicaDB.Exec("INSERT INTO users (id, name, email, age, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)",
		user.ID, user.Name, user.Email, user.Age, user.CreatedAt, user.UpdatedAt)

	// Test GetUser (read operation)
	retrievedUser, err := userService.GetUser(ctx, user.ID)
	if err != nil {
		t.Errorf("Failed to get user: %v", err)
	}

	if retrievedUser.Name != user.Name {
		t.Errorf("Expected name %s, got %s", user.Name, retrievedUser.Name)
	}

	// Test UpdateUser (write operation)
	user.Name = "Jane Doe"
	err = userService.UpdateUser(ctx, user)
	if err != nil {
		t.Errorf("Failed to update user: %v", err)
	}

	// Test SearchUsers (read operation)
	filter := UserFilter{
		Name:   "Jane",
		Limit:  10,
		Offset: 0,
	}

	users, err := userService.SearchUsers(ctx, filter)
	if err != nil {
		t.Errorf("Failed to search users: %v", err)
	}

	if len(users) == 0 {
		t.Error("Expected to find users in search")
	}

	// Test GetUserStats (read operation)
	stats, err := userService.GetUserStats(ctx)
	if err != nil {
		t.Errorf("Failed to get user stats: %v", err)
	}

	if stats.TotalUsers < 0 {
		t.Error("Expected non-negative total users")
	}
}

func TestTransactionManager_ReadWriteTransactions(t *testing.T) {
	if testPrimaryDB == nil || testReplicaDB == nil {
		t.Skip("Databases not available")
	}

	cluster, err := NewDBCluster(testPrimaryDSN, []string{testReplicaDSN})
	if err != nil {
		t.Fatalf("Failed to create DB cluster: %v", err)
	}
	defer cluster.Close()

	txManager := NewTransactionManager(cluster)
	ctx := context.Background()

	// Test write transaction
	err = txManager.WithWriteTransaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.Exec("INSERT INTO users (name, email, age) VALUES ($1, $2, $3)",
			"Transaction User", "tx@example.com", 25)
		return err
	})
	if err != nil {
		t.Errorf("Failed to execute write transaction: %v", err)
	}

	// Test read-only transaction
	err = txManager.WithReadOnlyTransaction(ctx, func(tx *sqlx.Tx) error {
		var count int
		return tx.Get(&count, "SELECT COUNT(*) FROM users")
	})
	if err != nil {
		t.Errorf("Failed to execute read-only transaction: %v", err)
	}

	// Test transaction rollback
	err = txManager.WithWriteTransaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.Exec("INSERT INTO users (name, email, age) VALUES ($1, $2, $3)",
			"Rollback User", "rollback@example.com", 30)
		if err != nil {
			return err
		}
		return fmt.Errorf("intentional error for rollback")
	})
	if err == nil {
		t.Error("Expected transaction to fail and rollback")
	}

	// Verify rollback worked
	var count int
	err = cluster.GetPrimary().Get(&count, "SELECT COUNT(*) FROM users WHERE email = 'rollback@example.com'")
	if err != nil {
		t.Errorf("Failed to check rollback: %v", err)
	}
	if count != 0 {
		t.Error("Expected rollback user to not exist")
	}
}

func BenchmarkRoutingManager_ReadOperations(b *testing.B) {
	if testPrimaryDB == nil || testReplicaDB == nil {
		b.Skip("Databases not available")
	}

	cluster, err := NewDBCluster(testPrimaryDSN, []string{testReplicaDSN})
	if err != nil {
		b.Fatal(err)
	}
	defer cluster.Close()

	strategy := NewRoundRobinStrategy()
	router := NewRoutingManager(cluster, strategy)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			db := router.RouteRead(ctx)
			if db == nil {
				b.Error("Expected database, got nil")
			}
		}
	})
}

func BenchmarkUserService_GetUser(b *testing.B) {
	if testPrimaryDB == nil || testReplicaDB == nil {
		b.Skip("Databases not available")
	}

	cluster, err := NewDBCluster(testPrimaryDSN, []string{testReplicaDSN})
	if err != nil {
		b.Fatal(err)
	}
	defer cluster.Close()

	strategy := NewRoundRobinStrategy()
	router := NewRoutingManager(cluster, strategy)
	userService := NewUserService(router)
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			userID := (b.N % 100) + 1 // Assuming we have users with IDs 1-100
			_, err := userService.GetUser(ctx, userID)
			if err != nil && err != sql.ErrNoRows {
				b.Error(err)
			}
		}
	})
}