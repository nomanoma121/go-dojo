package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
)

// PoolConfig holds database connection pool configuration
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	Environment     string // development, staging, production
}

// DefaultConfigs returns default configurations for different environments
func DefaultConfigs() map[string]PoolConfig {
	return map[string]PoolConfig{
		"development": {
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 1 * time.Minute,
			ConnMaxIdleTime: 30 * time.Second,
			Environment:     "development",
		},
		"staging": {
			MaxOpenConns:    15,
			MaxIdleConns:    5,
			ConnMaxLifetime: 3 * time.Minute,
			ConnMaxIdleTime: 1 * time.Minute,
			Environment:     "staging",
		},
		"production": {
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 2 * time.Minute,
			Environment:     "production",
		},
	}
}

// Apply applies the configuration to a database connection
func (pc *PoolConfig) Apply(db *sql.DB) {
	db.SetMaxOpenConns(pc.MaxOpenConns)
	db.SetMaxIdleConns(pc.MaxIdleConns)
	db.SetConnMaxLifetime(pc.ConnMaxLifetime)
	db.SetConnMaxIdleTime(pc.ConnMaxIdleTime)
}

// Validate validates the pool configuration
func (pc *PoolConfig) Validate() error {
	if pc.MaxOpenConns < 0 {
		return errors.New("MaxOpenConns must be non-negative")
	}
	if pc.MaxIdleConns < 0 {
		return errors.New("MaxIdleConns must be non-negative")
	}
	if pc.MaxIdleConns > pc.MaxOpenConns && pc.MaxOpenConns > 0 {
		return errors.New("MaxIdleConns cannot be greater than MaxOpenConns")
	}
	if pc.ConnMaxLifetime < 0 {
		return errors.New("ConnMaxLifetime must be non-negative")
	}
	if pc.ConnMaxIdleTime < 0 {
		return errors.New("ConnMaxIdleTime must be non-negative")
	}
	if pc.Environment == "" {
		return errors.New("Environment must be specified")
	}
	return nil
}

// ConnectionManager manages database connections and pool configuration
type ConnectionManager struct {
	db        *sql.DB
	config    PoolConfig
	dsn       string
	mu        sync.RWMutex
	monitor   *PoolMonitor
	healthChk *HealthChecker
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(dsn string, config PoolConfig) (*ConnectionManager, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	config.Apply(db)

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	cm := &ConnectionManager{
		db:     db,
		config: config,
		dsn:    dsn,
	}

	// Initialize monitor and health checker
	cm.monitor = NewPoolMonitor(db, config.Environment, 5*time.Second)
	cm.healthChk = NewHealthChecker(db, 30*time.Second, 10*time.Second)

	return cm, nil
}

// GetDB returns the database connection
func (cm *ConnectionManager) GetDB() *sql.DB {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.db
}

// UpdateConfig updates the pool configuration dynamically
func (cm *ConnectionManager) UpdateConfig(newConfig PoolConfig) error {
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	newConfig.Apply(cm.db)
	cm.config = newConfig

	return nil
}

// GetStats returns current connection pool statistics
func (cm *ConnectionManager) GetStats() sql.DBStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.db.Stats()
}

// Close closes all connections and cleanup
func (cm *ConnectionManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.monitor != nil {
		cm.monitor.Stop()
	}
	if cm.healthChk != nil {
		cm.healthChk.Stop()
	}

	return cm.db.Close()
}

// HealthChecker performs database health checks
type HealthChecker struct {
	db        *sql.DB
	interval  time.Duration
	timeout   time.Duration
	stopCh    chan struct{}
	mu        sync.RWMutex
	lastCheck time.Time
	isHealthy bool
	errorMsg  string
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *sql.DB, interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		db:        db,
		interval:  interval,
		timeout:   timeout,
		stopCh:    make(chan struct{}),
		isHealthy: true,
	}
}

// Start starts the health checking routine
func (hc *HealthChecker) Start() {
	go func() {
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hc.performCheck()
			case <-hc.stopCh:
				return
			}
		}
	}()
}

// Stop stops the health checking routine
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// IsHealthy returns the current health status
func (hc *HealthChecker) IsHealthy() (bool, string, time.Time) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.isHealthy, hc.errorMsg, hc.lastCheck
}

// CheckNow performs an immediate health check
func (hc *HealthChecker) CheckNow() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hc.timeout)
	defer cancel()

	err := hc.db.PingContext(ctx)
	
	hc.mu.Lock()
	hc.lastCheck = time.Now()
	if err != nil {
		hc.isHealthy = false
		hc.errorMsg = err.Error()
	} else {
		hc.isHealthy = true
		hc.errorMsg = ""
	}
	hc.mu.Unlock()

	return hc.isHealthy, err
}

func (hc *HealthChecker) performCheck() {
	hc.CheckNow()
}

// PoolMonitor monitors connection pool statistics
type PoolMonitor struct {
	db       *sql.DB
	interval time.Duration
	stats    []PoolStats
	mu       sync.RWMutex
	stopCh   chan struct{}
	name     string
}

// PoolStats holds connection pool statistics at a point in time
type PoolStats struct {
	Timestamp       time.Time
	OpenConnections int
	InUse           int
	Idle            int
	WaitCount       int64
	WaitDuration    time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
}

// NewPoolMonitor creates a new pool monitor
func NewPoolMonitor(db *sql.DB, name string, interval time.Duration) *PoolMonitor {
	return &PoolMonitor{
		db:       db,
		interval: interval,
		stats:    make([]PoolStats, 0),
		stopCh:   make(chan struct{}),
		name:     name,
	}
}

// Start starts monitoring the connection pool
func (pm *PoolMonitor) Start() {
	go func() {
		ticker := time.NewTicker(pm.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pm.collectStats()
			case <-pm.stopCh:
				return
			}
		}
	}()
}

// Stop stops monitoring
func (pm *PoolMonitor) Stop() {
	close(pm.stopCh)
}

// GetStats returns collected statistics
func (pm *PoolMonitor) GetStats() []PoolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	statsCopy := make([]PoolStats, len(pm.stats))
	copy(statsCopy, pm.stats)
	return statsCopy
}

// GetLatestStats returns the most recent statistics
func (pm *PoolMonitor) GetLatestStats() (PoolStats, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	if len(pm.stats) == 0 {
		return PoolStats{}, false
	}
	
	return pm.stats[len(pm.stats)-1], true
}

// ClearStats clears collected statistics
func (pm *PoolMonitor) ClearStats() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.stats = pm.stats[:0]
}

func (pm *PoolMonitor) collectStats() {
	dbStats := pm.db.Stats()
	
	stats := PoolStats{
		Timestamp:       time.Now(),
		OpenConnections: dbStats.OpenConnections,
		InUse:           dbStats.InUse,
		Idle:            dbStats.Idle,
		WaitCount:       dbStats.WaitCount,
		WaitDuration:    dbStats.WaitDuration,
		MaxOpenConns:    dbStats.MaxOpenConnections,
		MaxIdleConns:    int(dbStats.MaxIdleClosed),
	}
	
	pm.mu.Lock()
	pm.stats = append(pm.stats, stats)
	
	// Keep only the last 100 entries to prevent memory growth
	if len(pm.stats) > 100 {
		pm.stats = pm.stats[1:]
	}
	pm.mu.Unlock()
}

// LoadTester performs load testing on the connection pool
type LoadTester struct {
	db          *sql.DB
	concurrency int
	duration    time.Duration
	queryFunc   func(*sql.DB) error
	results     LoadTestResults
	mu          sync.Mutex
}

// LoadTestResults holds the results of a load test
type LoadTestResults struct {
	TotalRequests     int64
	SuccessfulReqs    int64
	FailedReqs        int64
	AvgResponseTime   time.Duration
	MaxResponseTime   time.Duration
	MinResponseTime   time.Duration
	RequestsPerSecond float64
	Errors            []string
}

// NewLoadTester creates a new load tester
func NewLoadTester(db *sql.DB, concurrency int, duration time.Duration) *LoadTester {
	return &LoadTester{
		db:          db,
		concurrency: concurrency,
		duration:    duration,
		queryFunc:   defaultQueryFunction,
		results:     LoadTestResults{MinResponseTime: time.Hour}, // Initialize with large value
	}
}

// SetQueryFunc sets the query function for load testing
func (lt *LoadTester) SetQueryFunc(queryFunc func(*sql.DB) error) {
	lt.queryFunc = queryFunc
}

// Run executes the load test
func (lt *LoadTester) Run() LoadTestResults {
	var wg sync.WaitGroup
	startTime := time.Now()
	stopCh := make(chan struct{})
	
	var totalRequests int64
	var successfulReqs int64
	var failedReqs int64
	var totalResponseTime int64
	var maxResponseTime int64
	var minResponseTime int64 = int64(time.Hour)
	
	errors := make([]string, 0)
	var errorsMu sync.Mutex

	// Stop all workers after duration
	go func() {
		time.Sleep(lt.duration)
		close(stopCh)
	}()

	// Start workers
	for i := 0; i < lt.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					requestStart := time.Now()
					err := lt.queryFunc(lt.db)
					requestDuration := time.Since(requestStart)
					
					atomic.AddInt64(&totalRequests, 1)
					atomic.AddInt64(&totalResponseTime, int64(requestDuration))
					
					// Update min response time
					for {
						current := atomic.LoadInt64(&minResponseTime)
						if int64(requestDuration) >= current {
							break
						}
						if atomic.CompareAndSwapInt64(&minResponseTime, current, int64(requestDuration)) {
							break
						}
					}
					
					// Update max response time
					for {
						current := atomic.LoadInt64(&maxResponseTime)
						if int64(requestDuration) <= current {
							break
						}
						if atomic.CompareAndSwapInt64(&maxResponseTime, current, int64(requestDuration)) {
							break
						}
					}
					
					if err != nil {
						atomic.AddInt64(&failedReqs, 1)
						errorsMu.Lock()
						if len(errors) < 10 { // Limit error collection
							errors = append(errors, err.Error())
						}
						errorsMu.Unlock()
					} else {
						atomic.AddInt64(&successfulReqs, 1)
					}
				}
			}
		}()
	}

	wg.Wait()
	
	actualDuration := time.Since(startTime)
	
	lt.mu.Lock()
	lt.results = LoadTestResults{
		TotalRequests:     totalRequests,
		SuccessfulReqs:    successfulReqs,
		FailedReqs:        failedReqs,
		RequestsPerSecond: float64(totalRequests) / actualDuration.Seconds(),
		Errors:            errors,
	}
	
	if totalRequests > 0 {
		lt.results.AvgResponseTime = time.Duration(totalResponseTime / totalRequests)
	}
	
	lt.results.MaxResponseTime = time.Duration(maxResponseTime)
	if minResponseTime < int64(time.Hour) {
		lt.results.MinResponseTime = time.Duration(minResponseTime)
	}
	lt.mu.Unlock()
	
	return lt.results
}

// GetResults returns the load test results
func (lt *LoadTester) GetResults() LoadTestResults {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	return lt.results
}

// PoolOptimizer suggests optimal pool configurations based on workload
type PoolOptimizer struct {
	stats           []PoolStats
	recommendations map[string]interface{}
	mu              sync.RWMutex
}

// NewPoolOptimizer creates a new pool optimizer
func NewPoolOptimizer() *PoolOptimizer {
	return &PoolOptimizer{
		stats:           make([]PoolStats, 0),
		recommendations: make(map[string]interface{}),
	}
}

// AnalyzeStats analyzes pool statistics and generates recommendations
func (po *PoolOptimizer) AnalyzeStats(stats []PoolStats) map[string]interface{} {
	if len(stats) == 0 {
		return map[string]interface{}{"error": "no statistics provided"}
	}

	po.mu.Lock()
	defer po.mu.Unlock()
	
	po.stats = stats
	recommendations := make(map[string]interface{})
	
	// Calculate averages
	var avgInUse, avgIdle, avgWaitCount float64
	var totalWaitDuration time.Duration
	var maxInUse, maxIdle int
	
	for _, stat := range stats {
		avgInUse += float64(stat.InUse)
		avgIdle += float64(stat.Idle)
		avgWaitCount += float64(stat.WaitCount)
		totalWaitDuration += stat.WaitDuration
		
		if stat.InUse > maxInUse {
			maxInUse = stat.InUse
		}
		if stat.Idle > maxIdle {
			maxIdle = stat.Idle
		}
	}
	
	avgInUse /= float64(len(stats))
	avgIdle /= float64(len(stats))
	avgWaitCount /= float64(len(stats))
	avgWaitDuration := totalWaitDuration / time.Duration(len(stats))
	
	recommendations["average_in_use"] = avgInUse
	recommendations["average_idle"] = avgIdle
	recommendations["average_wait_count"] = avgWaitCount
	recommendations["average_wait_duration"] = avgWaitDuration
	recommendations["max_in_use"] = maxInUse
	recommendations["max_idle"] = maxIdle
	
	// Generate recommendations
	if avgWaitCount > 1 || avgWaitDuration > time.Millisecond {
		recommendations["increase_max_open_conns"] = true
		recommendations["suggested_max_open_conns"] = int(float64(maxInUse) * 1.5)
	}
	
	if avgIdle > avgInUse*2 {
		recommendations["decrease_max_idle_conns"] = true
		recommendations["suggested_max_idle_conns"] = int(avgInUse * 1.2)
	}
	
	po.recommendations = recommendations
	return recommendations
}

// SuggestConfig suggests an optimal configuration based on analysis
func (po *PoolOptimizer) SuggestConfig(environment string) PoolConfig {
	po.mu.RLock()
	defer po.mu.RUnlock()
	
	defaults := DefaultConfigs()
	config := defaults[environment]
	
	if maxOpenConns, exists := po.recommendations["suggested_max_open_conns"]; exists {
		if value, ok := maxOpenConns.(int); ok && value > config.MaxOpenConns {
			config.MaxOpenConns = value
		}
	}
	
	if maxIdleConns, exists := po.recommendations["suggested_max_idle_conns"]; exists {
		if value, ok := maxIdleConns.(int); ok && value < config.MaxIdleConns {
			config.MaxIdleConns = value
		}
	}
	
	return config
}

// GetRecommendations returns current recommendations
func (po *PoolOptimizer) GetRecommendations() map[string]interface{} {
	po.mu.RLock()
	defer po.mu.RUnlock()
	
	result := make(map[string]interface{})
	for k, v := range po.recommendations {
		result[k] = v
	}
	return result
}

// setupTestDatabase creates a test database connection
func setupTestDatabase(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	
	return db, nil
}

// defaultQueryFunction is the default query function for load testing
func defaultQueryFunction(db *sql.DB) error {
	var result int
	return db.QueryRow("SELECT 1").Scan(&result)
}

// createTestSchema creates test tables for connection pool testing
func createTestSchema(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS test_connections (
			id SERIAL PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			data TEXT
		);
		
		CREATE INDEX IF NOT EXISTS idx_test_connections_created_at ON test_connections(created_at);
	`
	
	_, err := db.Exec(schema)
	return err
}