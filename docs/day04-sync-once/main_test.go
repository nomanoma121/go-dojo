package main

import (
	"os"
	"sync"
	"testing"
	"time"
)

func TestConfigManager(t *testing.T) {
	// 環境変数を設定
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("API_KEY", "test-api-key")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("MAX_RETRIES", "3")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("API_KEY")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("MAX_RETRIES")
	}()

	t.Run("Configuration loaded once", func(t *testing.T) {
		cm := NewConfigManager()
		
		// 最初の呼び出し
		config1, err1 := cm.GetConfig()
		if err1 != nil {
			t.Fatalf("First call failed: %v", err1)
		}
		
		// 二回目の呼び出し
		config2, err2 := cm.GetConfig()
		if err2 != nil {
			t.Fatalf("Second call failed: %v", err2)
		}
		
		// 同じインスタンスが返されることを確認
		if config1 != config2 {
			t.Error("Different config instances returned")
		}
		
		// 設定値の確認
		if config1.DatabaseURL != "postgres://localhost/test" {
			t.Errorf("Expected DatabaseURL 'postgres://localhost/test', got '%s'", config1.DatabaseURL)
		}
		if config1.APIKey != "test-api-key" {
			t.Errorf("Expected APIKey 'test-api-key', got '%s'", config1.APIKey)
		}
		if config1.MaxRetries != 3 {
			t.Errorf("Expected MaxRetries 3, got %d", config1.MaxRetries)
		}
	})

	t.Run("Concurrent access", func(t *testing.T) {
		cm := NewConfigManager()
		const numGoroutines = 100
		
		var wg sync.WaitGroup
		configs := make([]*Config, numGoroutines)
		errors := make([]error, numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				config, err := cm.GetConfig()
				configs[index] = config
				errors[index] = err
			}(i)
		}
		
		wg.Wait()
		
		// すべて同じインスタンスが返されることを確認
		firstConfig := configs[0]
		if firstConfig == nil {
			t.Fatal("First config is nil")
		}
		
		for i := 1; i < numGoroutines; i++ {
			if errors[i] != nil {
				t.Errorf("Error in goroutine %d: %v", i, errors[i])
			}
			if configs[i] != firstConfig {
				t.Errorf("Different config instance in goroutine %d", i)
			}
		}
	})

	t.Run("Missing environment variables", func(t *testing.T) {
		// 環境変数をクリア
		os.Unsetenv("DATABASE_URL")
		defer os.Setenv("DATABASE_URL", "postgres://localhost/test")
		
		cm := NewConfigManager()
		_, err := cm.GetConfig()
		if err == nil {
			t.Error("Expected error when DATABASE_URL is missing")
		}
	})
}

func TestDatabasePool(t *testing.T) {
	// グローバル変数をリセット
	dbPoolInstance = nil
	dbPoolOnce = sync.Once{}

	t.Run("Singleton behavior", func(t *testing.T) {
		pool1 := GetDatabasePool()
		pool2 := GetDatabasePool()
		
		if pool1 != pool2 {
			t.Error("Different pool instances returned")
		}
	})

	t.Run("Pool initialization", func(t *testing.T) {
		pool := GetDatabasePool()
		err := pool.InitializePool(5)
		if err != nil {
			t.Fatalf("Pool initialization failed: %v", err)
		}
		
		// 再初期化は無視されるべき
		err = pool.InitializePool(10)
		if err != nil {
			t.Fatalf("Second initialization failed: %v", err)
		}
	})

	t.Run("Get connection", func(t *testing.T) {
		pool := GetDatabasePool()
		pool.InitializePool(3)
		
		conn, err := pool.GetConnection()
		if err != nil {
			t.Fatalf("Failed to get connection: %v", err)
		}
		if conn == "" {
			t.Error("Empty connection returned")
		}
	})

	t.Run("Concurrent pool access", func(t *testing.T) {
		// 新しいテスト用にリセット
		dbPoolInstance = nil
		dbPoolOnce = sync.Once{}
		
		const numGoroutines = 50
		var wg sync.WaitGroup
		pools := make([]*DatabasePool, numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				pools[index] = GetDatabasePool()
			}(i)
		}
		
		wg.Wait()
		
		// すべて同じインスタンスであることを確認
		firstPool := pools[0]
		for i := 1; i < numGoroutines; i++ {
			if pools[i] != firstPool {
				t.Errorf("Different pool instance in goroutine %d", i)
			}
		}
	})
}

func TestExpensiveResource(t *testing.T) {
	t.Run("Data initialized once", func(t *testing.T) {
		resource := NewExpensiveResource()
		
		start := time.Now()
		data1, err1 := resource.GetData()
		firstCallTime := time.Since(start)
		
		if err1 != nil {
			t.Fatalf("First call failed: %v", err1)
		}
		
		start = time.Now()
		data2, err2 := resource.GetData()
		secondCallTime := time.Since(start)
		
		if err2 != nil {
			t.Fatalf("Second call failed: %v", err2)
		}
		
		// 同じデータが返されることを確認
		if len(data1) != len(data2) {
			t.Error("Different data lengths returned")
		}
		
		// 二回目の呼び出しは大幅に速いはず
		if secondCallTime >= firstCallTime/2 {
			t.Errorf("Second call too slow: %v vs %v", secondCallTime, firstCallTime)
		}
	})

	t.Run("Concurrent data access", func(t *testing.T) {
		resource := NewExpensiveResource()
		const numGoroutines = 20
		
		var wg sync.WaitGroup
		results := make([][]byte, numGoroutines)
		errors := make([]error, numGoroutines)
		
		start := time.Now()
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				data, err := resource.GetData()
				results[index] = data
				errors[index] = err
			}(i)
		}
		
		wg.Wait()
		elapsed := time.Since(start)
		
		// 初期化は一度だけなので、早く完了するはず
		if elapsed > 2*time.Second {
			t.Errorf("Concurrent access took too long: %v", elapsed)
		}
		
		// すべて同じデータが返されることを確認
		for i := 0; i < numGoroutines; i++ {
			if errors[i] != nil {
				t.Errorf("Error in goroutine %d: %v", i, errors[i])
			}
			if len(results[i]) != len(results[0]) {
				t.Errorf("Different data length in goroutine %d", i)
			}
		}
	})
}

func TestService(t *testing.T) {
	t.Run("Service initialization", func(t *testing.T) {
		service := NewService()
		
		err := service.Initialize()
		if err != nil {
			t.Fatalf("Service initialization failed: %v", err)
		}
		
		// 二回目の初期化は無視されるべき
		err = service.Initialize()
		if err != nil {
			t.Fatalf("Second initialization failed: %v", err)
		}
	})

	t.Run("Get value after initialization", func(t *testing.T) {
		service := NewService()
		service.Initialize()
		
		_, err := service.GetValue("test-key")
		if err != nil {
			t.Fatalf("Failed to get value: %v", err)
		}
	})

	t.Run("Get value without initialization", func(t *testing.T) {
		service := NewService()
		
		_, err := service.GetValue("test-key")
		if err == nil {
			t.Error("Expected error when service not initialized")
		}
	})
}

// ベンチマークテスト
func BenchmarkConfigManagerGetConfig(b *testing.B) {
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("API_KEY", "test-key")
	os.Setenv("LOG_LEVEL", "info")
	os.Setenv("MAX_RETRIES", "3")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("API_KEY")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("MAX_RETRIES")
	}()

	cm := NewConfigManager()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cm.GetConfig()
			if err != nil {
				b.Fatalf("GetConfig failed: %v", err)
			}
		}
	})
}

func BenchmarkDatabasePoolAccess(b *testing.B) {
	// リセット
	dbPoolInstance = nil
	dbPoolOnce = sync.Once{}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pool := GetDatabasePool()
			pool.InitializePool(10)
			_, err := pool.GetConnection()
			if err != nil {
				b.Fatalf("GetConnection failed: %v", err)
			}
		}
	})
}

func BenchmarkExpensiveResourceAccess(b *testing.B) {
	resource := NewExpensiveResource()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := resource.GetData()
			if err != nil {
				b.Fatalf("GetData failed: %v", err)
			}
		}
	})
}