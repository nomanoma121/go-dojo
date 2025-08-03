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

// GetConfig returns the application configuration, initializing it once if needed
func (cm *ConfigManager) GetConfig() (*Config, error) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. sync.Once.Do()を使って初期化関数を一度だけ実行
	// 2. 環境変数や設定ファイルから設定を読み込み
	// 3. エラーが発生した場合はcm.errに保存
	// 4. 初期化後は毎回同じconfig、errorを返す
	cm.once.Do(func() {
		cm.loadConfigFromEnv()
	})

	return cm.config, cm.err
}

// loadConfigFromEnv loads configuration from environment variables
func (cm *ConfigManager) loadConfigFromEnv() {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 環境変数から設定値を読み取り
	// 2. 必須項目の検証
	// 3. 数値の変換（MaxRetries）
	// 4. cm.configに設定、エラー時はcm.errに保存
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

// GetDatabasePool returns the singleton database pool instance
func GetDatabasePool() *DatabasePool {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. sync.Once.Do()でシングルトンインスタンスを一度だけ作成
	// 2. DatabasePoolを初期化
	// 3. 接続プールを設定
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
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 排他ロックを取得
	// 2. 初期化済みかチェック
	// 3. 接続プールを作成（模擬）
	// 4. initializedフラグを設定
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
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 読み取りロックを取得
	// 2. 初期化済みかチェック
	// 3. 利用可能な接続を返す
	dp.mu.RLock()
	defer dp.mu.RUnlock()

	if !dp.initialized {
		return "", errors.New("pool not initialized")
	}

	if len(dp.connections) == 0 {
		return "", errors.New("no connections available")
	}
	return dp.connections[0], nil
}

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

// GetData returns the data, initializing it once if needed
func (er *ExpensiveResource) GetData() ([]byte, error) {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. sync.Once.Do()で重い初期化処理を一度だけ実行
	// 2. 大きなデータを生成（模擬）
	// 3. エラー処理
	er.once.Do(func() {
		er.heavyInitialization()
	})
	return er.data, er.err
}

// heavyInitialization simulates a heavy initialization process
func (er *ExpensiveResource) heavyInitialization() {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 重い処理をシミュレート（time.Sleep）
	// 2. 大きなデータを作成
	// 3. エラーが発生する可能性を考慮
	// Simulate expensive computation
	time.Sleep(1 * time.Second)

	// Create large data structure
	er.data = make([]byte, 1024*1024) // 1MB of data
	for i := range er.data {
		er.data[i] = byte(i % 256)
	}
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

// Initialize performs one-time initialization of the service
func (s *Service) Initialize() error {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. sync.Once.Do()で初期化を一度だけ実行
	// 2. 実際の初期化処理を実行
	// 3. エラーハンドリング
	s.once.Do(func() {
		s.performInitialization()
	})
	return s.initError
}

// performInitialization performs the actual initialization work
func (s *Service) performInitialization() {
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 外部リソースの読み込み（模擬）
	// 2. データの準備
	// 3. 初期化完了フラグの設定
	// 4. エラー処理
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
	// TODO: ここに実装を追加してください
	//
	// 実装の流れ:
	// 1. 初期化されているかチェック
	// 2. 初期化エラーがあれば返す
	// 3. データから値を取得
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

func main() {
	// テスト用のサンプル実行
	cm := NewConfigManager()
	config, err := cm.GetConfig()
	if err != nil {
		println("Config error:", err.Error())
	} else {
		println("Config loaded:", config.DatabaseURL)
	}

	// データベースプールのテスト
	pool := GetDatabasePool()
	err = pool.InitializePool(10)
	if err != nil {
		println("Pool error:", err.Error())
	}
}
