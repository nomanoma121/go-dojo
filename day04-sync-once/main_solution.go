package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// Config represents application configuration
type Config struct {
	DatabaseURL string
	APIKey      string
	LogLevel    string
	MaxRetries  int
}

// ConfigManager manages application configuration with lazy initialization
type ConfigManager struct {
	config *Config
	once   sync.Once
	err    error
}

// NewConfigManager creates a new ConfigManager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{}
}

// DatabasePool represents a database connection pool singleton
type DatabasePool struct {
	connections []string
	maxConns    int
	initialized bool
	mu          sync.RWMutex
}

var (
	dbPoolInstance *DatabasePool
	dbPoolOnce     sync.Once
)

// ExpensiveResource represents a resource that is expensive to initialize
type ExpensiveResource struct {
	data []byte
	once sync.Once
	err  error
}

// NewExpensiveResource creates a new ExpensiveResource
func NewExpensiveResource() *ExpensiveResource {
	return &ExpensiveResource{}
}

// Service represents a service that requires one-time initialization
type Service struct {
	initialized bool
	data        map[string]string
	once        sync.Once
	initError   error
}

// NewService creates a new Service
func NewService() *Service {
	return &Service{
		data: make(map[string]string),
	}
}

// GetConfig returns the application configuration, initializing it once if needed
func (cm *ConfigManager) GetConfig() (*Config, error) {
	cm.once.Do(func() {
		cm.loadConfigFromEnv()
	})
	return cm.config, cm.err
}

// loadConfigFromEnv loads configuration from environment variables
func (cm *ConfigManager) loadConfigFromEnv() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		cm.err = errors.New("DATABASE_URL is required")
		return
	}
	
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		cm.err = errors.New("API_KEY is required")
		return
	}
	
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	
	maxRetriesStr := os.Getenv("MAX_RETRIES")
	maxRetries := 3 // default
	if maxRetriesStr != "" {
		if parsed, err := strconv.Atoi(maxRetriesStr); err == nil {
			maxRetries = parsed
		}
	}
	
	cm.config = &Config{
		DatabaseURL: databaseURL,
		APIKey:      apiKey,
		LogLevel:    logLevel,
		MaxRetries:  maxRetries,
	}
}

// GetDatabasePool returns the singleton database pool instance
func GetDatabasePool() *DatabasePool {
	dbPoolOnce.Do(func() {
		dbPoolInstance = &DatabasePool{
			connections: make([]string, 0),
			maxConns:    10,
			initialized: false,
		}
	})
	return dbPoolInstance
}

// InitializePool initializes the database connection pool
func (dp *DatabasePool) InitializePool(maxConns int) error {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	
	if dp.initialized {
		return nil // Already initialized
	}
	
	dp.maxConns = maxConns
	dp.connections = make([]string, maxConns)
	
	// Simulate connection creation
	for i := 0; i < maxConns; i++ {
		dp.connections[i] = fmt.Sprintf("connection-%d", i)
	}
	
	dp.initialized = true
	return nil
}

// GetConnection returns a connection from the pool
func (dp *DatabasePool) GetConnection() (string, error) {
	dp.mu.RLock()
	defer dp.mu.RUnlock()
	
	if !dp.initialized {
		return "", errors.New("pool not initialized")
	}
	
	if len(dp.connections) == 0 {
		return "", errors.New("no connections available")
	}
	
	// Return first available connection (simplified)
	return dp.connections[0], nil
}

// GetData returns the data, initializing it once if needed
func (er *ExpensiveResource) GetData() ([]byte, error) {
	er.once.Do(func() {
		er.heavyInitialization()
	})
	return er.data, er.err
}

// heavyInitialization simulates a heavy initialization process
func (er *ExpensiveResource) heavyInitialization() {
	// Simulate expensive computation
	time.Sleep(1 * time.Second)
	
	// Create large data structure
	er.data = make([]byte, 1024*1024) // 1MB of data
	for i := range er.data {
		er.data[i] = byte(i % 256)
	}
}

// Initialize performs one-time initialization of the service
func (s *Service) Initialize() error {
	s.once.Do(func() {
		s.performInitialization()
	})
	return s.initError
}

// performInitialization performs the actual initialization work
func (s *Service) performInitialization() {
	// Simulate loading external resources
	time.Sleep(500 * time.Millisecond)
	
	// Prepare data
	s.data["key1"] = "value1"
	s.data["key2"] = "value2"
	s.data["key3"] = "value3"
	s.data["test-key"] = "test-value"
	
	s.initialized = true
}

// GetValue returns a value from the service (requires initialization)
func (s *Service) GetValue(key string) (string, error) {
	if !s.initialized {
		return "", errors.New("service not initialized")
	}
	
	if s.initError != nil {
		return "", s.initError
	}
	
	value, exists := s.data[key]
	if !exists {
		return "", errors.New("key not found")
	}
	
	return value, nil
}