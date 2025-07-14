package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// User represents a user entity
type User struct {
	ID        int       `db:"id"`
	Name      string    `db:"name"`
	Email     string    `db:"email"`
	Age       int       `db:"age"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// UserFilter represents search criteria for users
type UserFilter struct {
	Name   string
	Email  string
	MinAge int
	Limit  int
	Offset int
}

// DBCluster manages primary and replica databases
type DBCluster struct {
	primary  *sqlx.DB
	replicas []*sqlx.DB
	mu       sync.RWMutex
	current  int64 // Atomic counter for round-robin
}

// NewDBCluster creates a new database cluster
func NewDBCluster(primaryDSN string, replicaDSNs []string) (*DBCluster, error) {
	primary, err := sqlx.Open("postgres", primaryDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to primary: %w", err)
	}

	if err := primary.Ping(); err != nil {
		primary.Close()
		return nil, fmt.Errorf("failed to ping primary: %w", err)
	}

	replicas := make([]*sqlx.DB, 0, len(replicaDSNs))
	for i, dsn := range replicaDSNs {
		replica, err := sqlx.Open("postgres", dsn)
		if err != nil {
			// Close previously opened connections
			for _, r := range replicas {
				r.Close()
			}
			primary.Close()
			return nil, fmt.Errorf("failed to connect to replica %d: %w", i, err)
		}

		if err := replica.Ping(); err != nil {
			replica.Close()
			// Close previously opened connections
			for _, r := range replicas {
				r.Close()
			}
			primary.Close()
			return nil, fmt.Errorf("failed to ping replica %d: %w", i, err)
		}

		replicas = append(replicas, replica)
	}

	return &DBCluster{
		primary:  primary,
		replicas: replicas,
	}, nil
}

// GetPrimary returns the primary database for write operations
func (cluster *DBCluster) GetPrimary() *sqlx.DB {
	return cluster.primary
}

// GetReplica returns a replica database for read operations
func (cluster *DBCluster) GetReplica() *sqlx.DB {
	if len(cluster.replicas) == 0 {
		return cluster.primary // Fallback to primary if no replicas
	}

	// Atomic round-robin selection
	index := atomic.AddInt64(&cluster.current, 1) % int64(len(cluster.replicas))
	return cluster.replicas[index]
}

// GetHealthyReplicas returns only healthy replicas
func (cluster *DBCluster) GetHealthyReplicas() []*sqlx.DB {
	cluster.mu.RLock()
	defer cluster.mu.RUnlock()

	healthy := make([]*sqlx.DB, 0, len(cluster.replicas))
	for _, replica := range cluster.replicas {
		if replica.Ping() == nil {
			healthy = append(healthy, replica)
		}
	}
	return healthy
}

// Close closes all database connections
func (cluster *DBCluster) Close() error {
	var errs []error

	if err := cluster.primary.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close primary: %w", err))
	}

	for i, replica := range cluster.replicas {
		if err := replica.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close replica %d: %w", i, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing connections: %v", errs)
	}
	return nil
}

// RoutingStrategy defines how to select replicas
type RoutingStrategy interface {
	SelectReplica(replicas []*sqlx.DB, metrics *RoutingMetrics) *sqlx.DB
}

// RoundRobinStrategy implements round-robin replica selection
type RoundRobinStrategy struct {
	current int64
}

// NewRoundRobinStrategy creates a new round-robin strategy
func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{}
}

// SelectReplica selects a replica using round-robin
func (rr *RoundRobinStrategy) SelectReplica(replicas []*sqlx.DB, metrics *RoutingMetrics) *sqlx.DB {
	if len(replicas) == 0 {
		return nil
	}

	index := atomic.AddInt64(&rr.current, 1) % int64(len(replicas))
	return replicas[index]
}

// WeightedStrategy implements weighted replica selection
type WeightedStrategy struct {
	weights []int
	total   int
	mu      sync.RWMutex
}

// NewWeightedStrategy creates a new weighted strategy
func NewWeightedStrategy(weights []int) *WeightedStrategy {
	total := 0
	for _, w := range weights {
		total += w
	}
	return &WeightedStrategy{
		weights: append([]int(nil), weights...), // Copy slice
		total:   total,
	}
}

// SelectReplica selects a replica using weights
func (ws *WeightedStrategy) SelectReplica(replicas []*sqlx.DB, metrics *RoutingMetrics) *sqlx.DB {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	if len(replicas) == 0 || len(ws.weights) != len(replicas) || ws.total == 0 {
		// Fallback to round-robin
		if len(replicas) > 0 {
			return replicas[rand.Intn(len(replicas))]
		}
		return nil
	}

	// Weighted random selection
	r := rand.Intn(ws.total)
	cumulative := 0
	for i, weight := range ws.weights {
		cumulative += weight
		if r < cumulative {
			return replicas[i]
		}
	}

	// Fallback (should not reach here)
	return replicas[0]
}

// RoutingMetrics holds routing performance metrics
type RoutingMetrics struct {
	readCount    int64
	writeCount   int64
	errorCount   int64
	responseTime map[*sqlx.DB]int64 // nanoseconds as int64 for atomic operations
	mu           sync.RWMutex
}

// NewRoutingMetrics creates new routing metrics
func NewRoutingMetrics() *RoutingMetrics {
	return &RoutingMetrics{
		responseTime: make(map[*sqlx.DB]int64),
	}
}

// RecordRead records a read operation
func (rm *RoutingMetrics) RecordRead(db *sqlx.DB, duration time.Duration) {
	atomic.AddInt64(&rm.readCount, 1)

	rm.mu.Lock()
	rm.responseTime[db] = int64(duration)
	rm.mu.Unlock()
}

// RecordWrite records a write operation
func (rm *RoutingMetrics) RecordWrite(duration time.Duration) {
	atomic.AddInt64(&rm.writeCount, 1)
}

// RecordError records an error
func (rm *RoutingMetrics) RecordError() {
	atomic.AddInt64(&rm.errorCount, 1)
}

// GetStats returns current statistics
func (rm *RoutingMetrics) GetStats() (int64, int64, int64) {
	return atomic.LoadInt64(&rm.readCount), 
		   atomic.LoadInt64(&rm.writeCount), 
		   atomic.LoadInt64(&rm.errorCount)
}

// RoutingManager handles read-write routing
type RoutingManager struct {
	cluster     *DBCluster
	strategy    RoutingStrategy
	health      *HealthMonitor
	lagDetector *LagDetector
	metrics     *RoutingMetrics
}

// NewRoutingManager creates a new routing manager
func NewRoutingManager(cluster *DBCluster, strategy RoutingStrategy) *RoutingManager {
	rm := &RoutingManager{
		cluster:     cluster,
		strategy:    strategy,
		metrics:     NewRoutingMetrics(),
		lagDetector: NewLagDetector(cluster, 100*time.Millisecond),
	}
	rm.health = NewHealthMonitor(cluster, 30*time.Second)
	return rm
}

// RouteRead routes read operations to appropriate replica
func (rm *RoutingManager) RouteRead(ctx context.Context) *sqlx.DB {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		rm.metrics.RecordRead(nil, duration) // Simplified for this example
	}()

	// Get healthy replicas with low lag
	var replicas []*sqlx.DB
	if rm.health != nil {
		replicas = rm.health.GetHealthyReplicas()
	} else {
		replicas = rm.cluster.GetHealthyReplicas()
	}

	if rm.lagDetector != nil {
		lowLagReplicas, err := rm.lagDetector.GetLowLagReplicas(ctx)
		if err == nil && len(lowLagReplicas) > 0 {
			replicas = lowLagReplicas
		}
	}

	if len(replicas) == 0 {
		return rm.cluster.GetPrimary() // Fallback to primary
	}

	if rm.strategy != nil {
		return rm.strategy.SelectReplica(replicas, rm.metrics)
	}

	return replicas[0]
}

// RouteWrite routes write operations to primary
func (rm *RoutingManager) RouteWrite(ctx context.Context) *sqlx.DB {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		rm.metrics.RecordWrite(duration)
	}()

	return rm.cluster.GetPrimary()
}

// HealthMonitor monitors database health
type HealthMonitor struct {
	cluster       *DBCluster
	healthMap     map[*sqlx.DB]bool
	mu            sync.RWMutex
	checkInterval time.Duration
	stopCh        chan struct{}
	running       bool
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(cluster *DBCluster, checkInterval time.Duration) *HealthMonitor {
	hm := &HealthMonitor{
		cluster:       cluster,
		healthMap:     make(map[*sqlx.DB]bool),
		checkInterval: checkInterval,
		stopCh:        make(chan struct{}),
	}

	// Initialize health status
	hm.healthMap[cluster.primary] = true
	for _, replica := range cluster.replicas {
		hm.healthMap[replica] = true
	}

	return hm
}

// Start starts health monitoring
func (hm *HealthMonitor) Start() {
	hm.mu.Lock()
	if hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = true
	hm.mu.Unlock()

	go func() {
		ticker := time.NewTicker(hm.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hm.checkHealth()
			case <-hm.stopCh:
				return
			}
		}
	}()
}

// Stop stops health monitoring
func (hm *HealthMonitor) Stop() {
	hm.mu.Lock()
	if !hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = false
	hm.mu.Unlock()

	close(hm.stopCh)
}

// IsHealthy checks if a database is healthy
func (hm *HealthMonitor) IsHealthy(db *sqlx.DB) bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.healthMap[db]
}

// GetHealthyReplicas returns healthy replicas
func (hm *HealthMonitor) GetHealthyReplicas() []*sqlx.DB {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	healthy := make([]*sqlx.DB, 0)
	for _, replica := range hm.cluster.replicas {
		if hm.healthMap[replica] {
			healthy = append(healthy, replica)
		}
	}
	return healthy
}

func (hm *HealthMonitor) checkHealth() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Check primary
	hm.healthMap[hm.cluster.primary] = hm.pingDB(hm.cluster.primary)

	// Check replicas
	for _, replica := range hm.cluster.replicas {
		hm.healthMap[replica] = hm.pingDB(replica)
	}
}

func (hm *HealthMonitor) pingDB(db *sqlx.DB) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx) == nil
}

// LagDetector monitors replication lag
type LagDetector struct {
	cluster *DBCluster
	maxLag  time.Duration
	lagMap  map[*sqlx.DB]time.Duration
	mu      sync.RWMutex
}

// NewLagDetector creates a new lag detector
func NewLagDetector(cluster *DBCluster, maxLag time.Duration) *LagDetector {
	return &LagDetector{
		cluster: cluster,
		maxLag:  maxLag,
		lagMap:  make(map[*sqlx.DB]time.Duration),
	}
}

// CheckReplicationLag checks replication lag for all replicas
func (ld *LagDetector) CheckReplicationLag(ctx context.Context) (map[*sqlx.DB]time.Duration, error) {
	lagMap := make(map[*sqlx.DB]time.Duration)

	// Get current time from primary
	var primaryTime time.Time
	err := ld.cluster.GetPrimary().GetContext(ctx, &primaryTime, "SELECT NOW()")
	if err != nil {
		return nil, fmt.Errorf("failed to get primary time: %w", err)
	}

	// Check lag for each replica
	for _, replica := range ld.cluster.replicas {
		var replicaTime time.Time
		err := replica.GetContext(ctx, &replicaTime, "SELECT NOW()")
		if err != nil {
			lagMap[replica] = time.Hour // Mark as severely lagged
			continue
		}

		lag := primaryTime.Sub(replicaTime)
		if lag < 0 {
			lag = 0 // Replica might be ahead due to clock differences
		}

		lagMap[replica] = lag
	}

	ld.mu.Lock()
	ld.lagMap = lagMap
	ld.mu.Unlock()

	return lagMap, nil
}

// GetLowLagReplicas returns replicas with acceptable lag
func (ld *LagDetector) GetLowLagReplicas(ctx context.Context) ([]*sqlx.DB, error) {
	lagMap, err := ld.CheckReplicationLag(ctx)
	if err != nil {
		return nil, err
	}

	lowLagReplicas := make([]*sqlx.DB, 0)
	for replica, lag := range lagMap {
		if lag <= ld.maxLag {
			lowLagReplicas = append(lowLagReplicas, replica)
		}
	}

	return lowLagReplicas, nil
}

// LoadBalancer distributes load across replicas
type LoadBalancer struct {
	strategy RoutingStrategy
	metrics  *RoutingMetrics
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(strategy RoutingStrategy) *LoadBalancer {
	return &LoadBalancer{
		strategy: strategy,
		metrics:  NewRoutingMetrics(),
	}
}

// SelectReplica selects the best replica for a read operation
func (lb *LoadBalancer) SelectReplica(replicas []*sqlx.DB) *sqlx.DB {
	if lb.strategy != nil {
		return lb.strategy.SelectReplica(replicas, lb.metrics)
	}

	if len(replicas) == 0 {
		return nil
	}

	return replicas[0]
}

// FailoverManager handles automatic failover
type FailoverManager struct {
	cluster            *DBCluster
	health             *HealthMonitor
	failoverInProgress bool
	mu                 sync.Mutex
}

// NewFailoverManager creates a new failover manager
func NewFailoverManager(cluster *DBCluster, health *HealthMonitor) *FailoverManager {
	return &FailoverManager{
		cluster: cluster,
		health:  health,
	}
}

// HandlePrimaryFailure handles primary database failure
func (fm *FailoverManager) HandlePrimaryFailure(ctx context.Context) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.failoverInProgress {
		return errors.New("failover already in progress")
	}

	fm.failoverInProgress = true
	defer func() {
		fm.failoverInProgress = false
	}()

	// Find a healthy replica to promote
	healthyReplicas := fm.health.GetHealthyReplicas()
	if len(healthyReplicas) == 0 {
		return errors.New("no healthy replicas available for failover")
	}

	// Promote the first healthy replica
	newPrimary := healthyReplicas[0]
	return fm.PromoteReplica(newPrimary)
}

// PromoteReplica promotes a replica to primary
func (fm *FailoverManager) PromoteReplica(replica *sqlx.DB) error {
	// In a real implementation, this would involve:
	// 1. Stopping writes to old primary
	// 2. Ensuring replica is caught up
	// 3. Promoting replica to primary
	// 4. Redirecting writes to new primary
	// 5. Updating cluster configuration

	// For this example, we'll just swap the primary reference
	fm.cluster.mu.Lock()
	defer fm.cluster.mu.Unlock()

	// Find the replica in the list and remove it
	for i, r := range fm.cluster.replicas {
		if r == replica {
			// Remove from replicas list
			fm.cluster.replicas = append(fm.cluster.replicas[:i], fm.cluster.replicas[i+1:]...)
			break
		}
	}

	// Add old primary to replicas (if it recovers)
	fm.cluster.replicas = append(fm.cluster.replicas, fm.cluster.primary)

	// Set new primary
	fm.cluster.primary = replica

	return nil
}

// UserService demonstrates read-write splitting
type UserService struct {
	router *RoutingManager
}

// NewUserService creates a new user service
func NewUserService(router *RoutingManager) *UserService {
	return &UserService{router: router}
}

// CreateUser creates a new user (write operation)
func (us *UserService) CreateUser(ctx context.Context, user *User) error {
	db := us.router.RouteWrite(ctx)

	query := `
		INSERT INTO users (name, email, age) 
		VALUES ($1, $2, $3) 
		RETURNING id, created_at, updated_at`

	return db.GetContext(ctx, user, query, user.Name, user.Email, user.Age)
}

// GetUser retrieves a user by ID (read operation)
func (us *UserService) GetUser(ctx context.Context, id int) (*User, error) {
	db := us.router.RouteRead(ctx)

	var user User
	err := db.GetContext(ctx, &user, "SELECT * FROM users WHERE id = $1", id)
	if err != nil {
		us.router.metrics.RecordError()
		return nil, err
	}

	return &user, nil
}

// UpdateUser updates an existing user (write operation)
func (us *UserService) UpdateUser(ctx context.Context, user *User) error {
	db := us.router.RouteWrite(ctx)

	query := `
		UPDATE users 
		SET name = $1, email = $2, age = $3, updated_at = NOW() 
		WHERE id = $4
		RETURNING updated_at`

	return db.GetContext(ctx, &user.UpdatedAt, query, user.Name, user.Email, user.Age, user.ID)
}

// SearchUsers searches for users (read operation)
func (us *UserService) SearchUsers(ctx context.Context, filter UserFilter) ([]User, error) {
	db := us.router.RouteRead(ctx)

	query := `
		SELECT * FROM users 
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%')
		  AND ($2 = '' OR email ILIKE '%' || $2 || '%')
		  AND ($3 = 0 OR age >= $3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5`

	var users []User
	err := db.SelectContext(ctx, &users, query,
		filter.Name, filter.Email, filter.MinAge, filter.Limit, filter.Offset)
	if err != nil {
		us.router.metrics.RecordError()
		return nil, err
	}

	return users, nil
}

// GetUserStats retrieves user statistics (read operation)
func (us *UserService) GetUserStats(ctx context.Context) (*UserStats, error) {
	db := us.router.RouteRead(ctx)

	var stats UserStats
	query := `
		SELECT 
			COUNT(*) as total_users,
			COUNT(CASE WHEN created_at > NOW() - INTERVAL '30 days' THEN 1 END) as active_users,
			AVG(age) as average_age
		FROM users`

	err := db.GetContext(ctx, &stats, query)
	if err != nil {
		us.router.metrics.RecordError()
		return nil, err
	}

	return &stats, nil
}

// UserStats holds user statistics
type UserStats struct {
	TotalUsers  int     `db:"total_users"`
	ActiveUsers int     `db:"active_users"`
	AverageAge  float64 `db:"average_age"`
}

// TransactionManager handles transactions with read-replica awareness
type TransactionManager struct {
	cluster *DBCluster
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(cluster *DBCluster) *TransactionManager {
	return &TransactionManager{cluster: cluster}
}

// WithReadOnlyTransaction executes read-only operations in a transaction
func (tm *TransactionManager) WithReadOnlyTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	db := tm.cluster.GetReplica()

	tx, err := db.BeginTxx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

// WithWriteTransaction executes write operations in a transaction
func (tm *TransactionManager) WithWriteTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	db := tm.cluster.GetPrimary()

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

// setupDatabase creates the database schema
func setupDatabase(db *sqlx.DB) error {
	schema := `
		DROP TABLE IF EXISTS users CASCADE;

		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			age INTEGER NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX idx_users_email ON users(email);
		CREATE INDEX idx_users_created_at ON users(created_at);
	`

	_, err := db.Exec(schema)
	return err
}

// seedTestData inserts test data
func seedTestData(db *sqlx.DB, userCount int) error {
	if userCount <= 0 {
		return nil
	}

	// Clear existing data
	_, err := db.Exec("DELETE FROM users")
	if err != nil {
		return err
	}

	// Reset sequence
	_, err = db.Exec("ALTER SEQUENCE users_id_seq RESTART WITH 1")
	if err != nil {
		return err
	}

	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := 1; i <= userCount; i++ {
		_, err := tx.Exec(
			"INSERT INTO users (name, email, age) VALUES ($1, $2, $3)",
			fmt.Sprintf("User %d", i),
			fmt.Sprintf("user%d@example.com", i),
			20+(i%50),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}