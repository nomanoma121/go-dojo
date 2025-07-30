# Day 27: `dockertest`による統合テスト

🎯 **本日の目標**
`dockertest`ライブラリを使用して実際のPostgreSQLコンテナを起動し、データベースと連携するWebAPIの本格的なエンドツーエンドテストを実装できるようになる。

## 📖 解説

### dockertestとは

```go
// 【Dockertest統合テストの重要性】本格的な品質保証とプロダクション環境再現
// ❌ 問題例：モックのみでの偽りの安心感による本番障害
func catastrophicMockOnlyTesting() {
    // 🚨 災害例：モックテストのみで本番環境との乖離による壊滅的障害
    
    // ❌ モックテスト：完璧に成功
    func TestUserService_MockSuccess(t *testing.T) {
        mockDB := &MockDB{}
        
        // ❌ 理想的な条件のみテスト
        mockDB.On("CreateUser", mock.Anything).Return(nil)
        mockDB.On("GetUser", 1).Return(&User{ID: 1, Name: "Test"}, nil)
        
        service := NewUserService(mockDB)
        
        // ✅ テストは完璧に成功
        err := service.CreateUser(&User{Name: "Test"})
        assert.NoError(t, err)
        
        user, err := service.GetUser(1)
        assert.NoError(t, err)
        assert.Equal(t, "Test", user.Name)
        
        // 【落とし穴】モックは完璧だが実際のDBは考慮されていない
    }
    
    // 【本番環境での災害】
    // 実際のPostgreSQLでは：
    // 1. 文字エンコーディング問題でデータ化け
    // 2. インデックス未作成で検索が数分かかる
    // 3. 外部キー制約違反でサービス停止
    // 4. トランザクション分離レベル問題でデータ競合
    // 5. 接続プール枯渇でタイムアウト
    // 6. JSON型カラムでの構文エラー
    
    fmt.Println("❌ Mock tests passed, but production database failed!")
    // 結果：完璧なテストカバレッジでも本番で全APIが503エラー
    
    // 【実際の被害例】
    // - ECサイト：決済APIが本番のみエラー→売上ゼロ
    // - 銀行システム：残高照会が無限ループ→全ATM停止
    // - 医療システム：患者データ取得失敗→診療不可能
    // - 物流システム：在庫更新失敗→配送麻痺
}

// ✅ 正解：エンタープライズ級Dockertest統合テストシステム
type EnterpriseDockerTestSystem struct {
    // 【基本機能】
    pool            *dockertest.Pool            // Dockerプール
    resources       map[string]*dockertest.Resource // 起動中リソース
    
    // 【データベース管理】
    postgresResource *dockertest.Resource        // PostgreSQL
    redisResource   *dockertest.Resource         // Redis
    mongoResource   *dockertest.Resource         // MongoDB
    
    // 【メッセージング】
    rabbitMQResource *dockertest.Resource        // RabbitMQ
    kafkaResource   *dockertest.Resource         // Kafka
    
    // 【監視・メトリクス】
    prometheusResource *dockertest.Resource      // Prometheus
    grafanaResource   *dockertest.Resource       // Grafana
    
    // 【高度な機能】
    networkManager   *NetworkManager             // ネットワーク管理
    volumeManager    *VolumeManager              // ボリューム管理
    configManager    *ConfigManager              // 設定管理
    
    // 【テスト環境制御】
    environmentType  EnvironmentType             // 環境タイプ
    isolationLevel   IsolationLevel              // 分離レベル
    
    // 【パフォーマンス最適化】
    containerPool    *ContainerPool              // コンテナプール
    imagePreloader   *ImagePreloader             // イメージ事前ロード
    
    // 【障害再現】
    chaosEngineering *ChaosEngineering           // カオスエンジニアリング
    networkPartition *NetworkPartition           // ネットワーク分断
    
    // 【セキュリティ】
    secretManager    *SecretManager              // シークレット管理
    tlsManager       *TLSManager                 // TLS証明書管理
    
    mu               sync.RWMutex                // 並行アクセス保護
}

// 【重要関数】エンタープライズDockertest環境初期化
func NewEnterpriseDockerTestSystem(config *DockerTestConfig) *EnterpriseDockerTestSystem {
    pool, err := dockertest.NewPool("")
    if err != nil {
        log.Fatalf("Could not create dockertest pool: %s", err)
    }
    
    // Docker APIバージョンの設定
    pool.MaxWait = config.MaxWaitTime
    
    system := &EnterpriseDockerTestSystem{
        pool:            pool,
        resources:       make(map[string]*dockertest.Resource),
        environmentType: config.EnvironmentType,
        isolationLevel:  config.IsolationLevel,
        networkManager:  NewNetworkManager(pool),
        volumeManager:   NewVolumeManager(pool),
        configManager:   NewConfigManager(),
        containerPool:   NewContainerPool(config.PoolSize),
        imagePreloader:  NewImagePreloader(pool),
        chaosEngineering: NewChaosEngineering(),
        networkPartition: NewNetworkPartition(),
        secretManager:   NewSecretManager(),
        tlsManager:      NewTLSManager(),
    }
    
    // 【重要】事前にイメージをプル
    system.preloadImages(config.RequiredImages)
    
    return system
}

// 【核心メソッド】包括的テスト環境構築
func (dt *EnterpriseDockerTestSystem) SetupComprehensiveTestEnvironment() (*TestEnvironment, error) {
    // 【STEP 1】専用ネットワーク作成
    networkID, err := dt.networkManager.CreateTestNetwork("integration-test-network")
    if err != nil {
        return nil, fmt.Errorf("failed to create test network: %w", err)
    }
    
    // 【STEP 2】PostgreSQL起動（本格設定）
    postgresEnv := []string{
        "POSTGRES_DB=test_enterprise_db",
        "POSTGRES_USER=test_admin",
        "POSTGRES_PASSWORD=" + dt.secretManager.GetSecret("postgres_password"),
        "POSTGRES_INITDB_ARGS=--encoding=UTF-8 --lc-collate=ja_JP.UTF-8",
        // 本番レベルの設定
        "shared_preload_libraries=pg_stat_statements,pg_prewarm",
        "max_connections=200",
        "shared_buffers=256MB",
        "effective_cache_size=1GB",
        "maintenance_work_mem=64MB",
        "checkpoint_completion_target=0.9",
        "wal_buffers=16MB",
        "default_statistics_target=100",
        "random_page_cost=1.1",
        "effective_io_concurrency=200",
    }
    
    postgresResource, err := dt.pool.RunWithOptions(&dockertest.RunOptions{
        Repository: "postgres",
        Tag:        "15-alpine",
        Env:        postgresEnv,
        Networks:   []*dockertest.Network{{Name: networkID}},
        ExposedPorts: []string{"5432"},
        PortBindings: map[docker.Port][]docker.PortBinding{
            "5432/tcp": {{HostIP: "", HostPort: ""}},
        },
    })
    if err != nil {
        return nil, fmt.Errorf("failed to start PostgreSQL: %w", err)
    }
    
    dt.resources["postgres"] = postgresResource
    
    // 【STEP 3】Redis起動（クラスター設定）
    redisResource, err := dt.pool.RunWithOptions(&dockertest.RunOptions{
        Repository: "redis",
        Tag:        "7-alpine",
        Env: []string{
            "REDIS_PASSWORD=" + dt.secretManager.GetSecret("redis_password"),
        },
        Cmd: []string{
            "redis-server",
            "--requirepass", dt.secretManager.GetSecret("redis_password"),
            "--maxmemory", "512mb",
            "--maxmemory-policy", "allkeys-lru",
            "--tcp-keepalive", "60",
            "--timeout", "300",
        },
        Networks: []*dockertest.Network{{Name: networkID}},
        ExposedPorts: []string{"6379"},
    })
    if err != nil {
        return nil, fmt.Errorf("failed to start Redis: %w", err)
    }
    
    dt.resources["redis"] = redisResource
    
    // 【STEP 4】データベース接続とスキーマ初期化
    db, err := dt.establishDatabaseConnection(postgresResource)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }
    
    // 【STEP 5】本格的なスキーマ初期化
    if err := dt.initializeEnterpriseSchema(db); err != nil {
        return nil, fmt.Errorf("failed to initialize schema: %w", err)
    }
    
    // 【STEP 6】Redis接続
    redisClient, err := dt.establishRedisConnection(redisResource)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to Redis: %w", err)
    }
    
    return &TestEnvironment{
        Database:    db,
        Redis:       redisClient,
        NetworkID:   networkID,
        PostgresURL: dt.getDatabaseURL(postgresResource),
        RedisURL:    dt.getRedisURL(redisResource),
        Resources:   dt.resources,
        Cleanup:     dt.cleanup,
    }, nil
}
```

`dockertest`は、Go言語のテストコードから直接Dockerコンテナを起動・管理できるライブラリです。統合テストで実際のデータベースやRedis、Message Queueなどの外部依存を使用する際に非常に有用です：

### 統合テストの重要性

単体テストではモックを使用しますが、統合テストでは実際の依存システムを使用します：

- **単体テスト**: 各関数・メソッドの動作を検証
- **統合テスト**: システム全体の連携を検証
- **E2Eテスト**: ユーザーの操作フローを検証

```go
// 単体テスト（モック使用）
func TestUserService_GetUser(t *testing.T) {
    mockDB := &MockDatabase{}
    mockDB.On("FindUser", 1).Return(&User{ID: 1, Name: "Test"}, nil)
    
    service := NewUserService(mockDB)
    user, err := service.GetUser(1)
    
    assert.NoError(t, err)
    assert.Equal(t, "Test", user.Name)
}

// 統合テスト（実際のDB使用）
func TestUserAPI_Integration(t *testing.T) {
    db := setupTestDatabase(t)
    defer db.Close()
    
    server := setupTestServer(db)
    defer server.Close()
    
    resp, err := http.Post(server.URL+"/users", "application/json", 
        strings.NewReader(`{"name": "Test User"}`))
    assert.NoError(t, err)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
```

### PostgreSQLコンテナの起動

```go
// 【エンタープライズデータベース接続】本番環境同等の設定とパフォーマンス最適化
func (dt *EnterpriseDockerTestSystem) establishDatabaseConnection(resource *dockertest.Resource) (*sql.DB, error) {
    // 【接続文字列生成】本番レベル設定
    connectionString := fmt.Sprintf(
        "postgres://test_admin:%s@localhost:%s/test_enterprise_db?"+
            "sslmode=disable&"+
            "connect_timeout=10&"+
            "statement_timeout=30000&"+
            "idle_in_transaction_session_timeout=60000&"+
            "application_name=enterprise_integration_test",
        dt.secretManager.GetSecret("postgres_password"),
        resource.GetPort("5432/tcp"),
    )
    
    var db *sql.DB
    
    // 【接続リトライ】堅牢な接続確立
    err := dt.pool.Retry(func() error {
        var err error
        db, err = sql.Open("postgres", connectionString)
        if err != nil {
            return fmt.Errorf("failed to open database: %w", err)
        }
        
        // 【接続プール設定】本番環境レベル
        db.SetMaxOpenConns(25)    // 最大接続数
        db.SetMaxIdleConns(10)    // アイドル接続数
        db.SetConnMaxLifetime(5 * time.Minute) // 接続寿命
        db.SetConnMaxIdleTime(1 * time.Minute) // アイドル時間
        
        // 【Ping テスト】実際の接続確認
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        
        if err := db.PingContext(ctx); err != nil {
            return fmt.Errorf("failed to ping database: %w", err)
        }
        
        // 【基本健全性チェック】
        var version string
        err = db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
        if err != nil {
            return fmt.Errorf("failed to query database version: %w", err)
        }
        
        log.Printf("✅ Connected to PostgreSQL: %s", version)
        return nil
    })
    
    if err != nil {
        return nil, fmt.Errorf("could not connect to PostgreSQL after retries: %w", err)
    }
    
    return db, nil
}

// 【重要メソッド】エンタープライズスキーマ初期化
func (dt *EnterpriseDockerTestSystem) initializeEnterpriseSchema(db *sql.DB) error {
    // 【本格的なスキーマ】実際のエンタープライズアプリケーション想定
    schema := `
    -- 【拡張機能有効化】
    CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
    CREATE EXTENSION IF NOT EXISTS "pg_trgm";
    CREATE EXTENSION IF NOT EXISTS "btree_gin";
    
    -- 【ユーザー管理テーブル】
    CREATE TABLE IF NOT EXISTS users (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
        email VARCHAR(255) UNIQUE NOT NULL,
        username VARCHAR(100) UNIQUE NOT NULL,
        password_hash VARCHAR(255) NOT NULL,
        first_name VARCHAR(100) NOT NULL,
        last_name VARCHAR(100) NOT NULL,
        phone VARCHAR(20),
        date_of_birth DATE,
        is_active BOOLEAN DEFAULT true,
        is_verified BOOLEAN DEFAULT false,
        last_login_at TIMESTAMP WITH TIME ZONE,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        deleted_at TIMESTAMP WITH TIME ZONE,
        
        -- 【制約】
        CONSTRAINT email_format CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
        CONSTRAINT phone_format CHECK (phone IS NULL OR phone ~* '^\+?[1-9]\d{1,14}$')
    );
    
    -- 【ユーザープロファイル】
    CREATE TABLE IF NOT EXISTS user_profiles (
        user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
        avatar_url TEXT,
        bio TEXT,
        location VARCHAR(255),
        website_url TEXT,
        timezone VARCHAR(50) DEFAULT 'UTC',
        language VARCHAR(10) DEFAULT 'en',
        notification_preferences JSONB DEFAULT '{}',
        privacy_settings JSONB DEFAULT '{}',
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );
    
    -- 【投稿管理】
    CREATE TABLE IF NOT EXISTS posts (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        title VARCHAR(255) NOT NULL,
        slug VARCHAR(255) UNIQUE NOT NULL,
        content TEXT NOT NULL,
        excerpt TEXT,
        featured_image_url TEXT,
        status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
        tags TEXT[] DEFAULT '{}',
        metadata JSONB DEFAULT '{}',
        view_count INTEGER DEFAULT 0,
        like_count INTEGER DEFAULT 0,
        comment_count INTEGER DEFAULT 0,
        published_at TIMESTAMP WITH TIME ZONE,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        deleted_at TIMESTAMP WITH TIME ZONE
    );
    
    -- 【コメント管理】
    CREATE TABLE IF NOT EXISTS comments (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
        post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
        user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        parent_id UUID REFERENCES comments(id) ON DELETE CASCADE,
        content TEXT NOT NULL,
        status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'spam')),
        like_count INTEGER DEFAULT 0,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        deleted_at TIMESTAMP WITH TIME ZONE
    );
    
    -- 【カテゴリ管理】
    CREATE TABLE IF NOT EXISTS categories (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
        name VARCHAR(100) UNIQUE NOT NULL,
        slug VARCHAR(100) UNIQUE NOT NULL,
        description TEXT,
        parent_id UUID REFERENCES categories(id) ON DELETE SET NULL,
        sort_order INTEGER DEFAULT 0,
        is_active BOOLEAN DEFAULT true,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );
    
    -- 【投稿カテゴリ関連】
    CREATE TABLE IF NOT EXISTS post_categories (
        post_id UUID REFERENCES posts(id) ON DELETE CASCADE,
        category_id UUID REFERENCES categories(id) ON DELETE CASCADE,
        PRIMARY KEY (post_id, category_id)
    );
    
    -- 【ユーザーセッション】
    CREATE TABLE IF NOT EXISTS user_sessions (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        session_token VARCHAR(255) UNIQUE NOT NULL,
        refresh_token VARCHAR(255) UNIQUE,
        ip_address INET,
        user_agent TEXT,
        expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
        is_active BOOLEAN DEFAULT true,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );
    
    -- 【監査ログ】
    CREATE TABLE IF NOT EXISTS audit_logs (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
        user_id UUID REFERENCES users(id),
        action VARCHAR(100) NOT NULL,
        resource_type VARCHAR(100) NOT NULL,
        resource_id UUID,
        old_values JSONB,
        new_values JSONB,
        ip_address INET,
        user_agent TEXT,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );
    `
    
    // 【スキーマ実行】
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if _, err := db.ExecContext(ctx, schema); err != nil {
        return fmt.Errorf("failed to execute schema: %w", err)
    }
    
    // 【インデックス作成】パフォーマンス最適化
    indexes := []string{
        // ユーザー検索最適化
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email_active ON users(email) WHERE is_active = true",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_username_lower ON users(lower(username))",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at ON users(created_at DESC)",
        
        // 投稿検索最適化
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_posts_status_published ON posts(status, published_at DESC) WHERE status = 'published'",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_posts_user_id ON posts(user_id, created_at DESC)",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_posts_slug ON posts(slug) WHERE deleted_at IS NULL",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_posts_tags_gin ON posts USING gin(tags)",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_posts_title_trgm ON posts USING gin(title gin_trgm_ops)",
        
        // コメント最適化
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_comments_post_id ON comments(post_id, created_at DESC)",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_comments_user_id ON comments(user_id, created_at DESC)",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_comments_parent_id ON comments(parent_id) WHERE parent_id IS NOT NULL",
        
        // セッション最適化
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_token ON user_sessions(session_token) WHERE is_active = true",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_user_expires ON user_sessions(user_id, expires_at DESC)",
        
        // 監査ログ最適化
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id, created_at DESC)",
        "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id, created_at DESC)",
    }
    
    for _, indexSQL := range indexes {
        if _, err := db.ExecContext(ctx, indexSQL); err != nil {
            // INDEXエラーは警告として扱う（既存の場合など）
            log.Printf("⚠️ Index creation warning: %v", err)
        }
    }
    
    log.Println("✅ Enterprise database schema initialized successfully")
    return nil
}

// 【実用例】本格的な統合テストの実装
func TestEnterpriseUserManagement_FullIntegration(t *testing.T) {
    // 【テスト環境構築】
    testSystem := NewEnterpriseDockerTestSystem(&DockerTestConfig{
        MaxWaitTime: 60 * time.Second,
        EnvironmentType: Integration,
        IsolationLevel: ProcessLevel,
        PoolSize: 5,
        RequiredImages: []string{"postgres:15-alpine", "redis:7-alpine"},
    })
    
    env, err := testSystem.SetupComprehensiveTestEnvironment()
    require.NoError(t, err)
    defer env.Cleanup()
    
    // 【本格的なテストシナリオ】
    t.Run("UserRegistrationToPostCreationFlow", func(t *testing.T) {
        // ユーザー登録
        user := &User{
            Email:     "integration@test.com",
            Username:  "integrationtest",
            FirstName: "Integration",
            LastName:  "Test",
            Password:  "SecurePass123!",
        }
        
        // データベース直接テスト
        userID, err := createUserInDB(env.Database, user)
        require.NoError(t, err)
        assert.NotEqual(t, uuid.Nil, userID)
        
        // プロファイル作成
        profile := &UserProfile{
            UserID:   userID,
            Bio:      "Integration test user",
            Location: "Test City",
            Timezone: "Asia/Tokyo",
        }
        
        err = createUserProfile(env.Database, profile)
        require.NoError(t, err)
        
        // 投稿作成
        post := &Post{
            UserID:  userID,
            Title:   "Integration Test Post",
            Slug:    "integration-test-post",
            Content: "This is a test post created during integration testing",
            Status:  "published",
            Tags:    []string{"test", "integration", "golang"},
        }
        
        postID, err := createPostInDB(env.Database, post)
        require.NoError(t, err)
        
        // コメント作成
        comment := &Comment{
            PostID:  postID,
            UserID:  userID,
            Content: "This is a test comment",
            Status:  "approved",
        }
        
        commentID, err := createCommentInDB(env.Database, comment)
        require.NoError(t, err)
        
        // 【検証】データ整合性チェック
        retrievedUser, err := getUserFromDB(env.Database, userID)
        require.NoError(t, err)
        assert.Equal(t, user.Email, retrievedUser.Email)
        
        retrievedPost, err := getPostFromDB(env.Database, postID)
        require.NoError(t, err)
        assert.Equal(t, post.Title, retrievedPost.Title)
        assert.Equal(t, 1, retrievedPost.CommentCount)
        
        // 【Redis連携テスト】
        err = cacheUserInRedis(env.Redis, userID, retrievedUser)
        require.NoError(t, err)
        
        cachedUser, err := getUserFromRedis(env.Redis, userID)
        require.NoError(t, err)
        assert.Equal(t, retrievedUser.Email, cachedUser.Email)
    })
    
    // 【パフォーマンステスト】
    t.Run("HighConcurrencyUserCreation", func(t *testing.T) {
        const numUsers = 100
        const concurrency = 10
        
        var wg sync.WaitGroup
        semaphore := make(chan struct{}, concurrency)
        errors := make(chan error, numUsers)
        
        start := time.Now()
        
        for i := 0; i < numUsers; i++ {
            wg.Add(1)
            go func(userIndex int) {
                defer wg.Done()
                semaphore <- struct{}{}
                defer func() { <-semaphore }()
                
                user := &User{
                    Email:     fmt.Sprintf("user%d@test.com", userIndex),
                    Username:  fmt.Sprintf("user%d", userIndex),
                    FirstName: "User",
                    LastName:  fmt.Sprintf("Number%d", userIndex),
                    Password:  "SecurePass123!",
                }
                
                _, err := createUserInDB(env.Database, user)
                if err != nil {
                    errors <- err
                    return
                }
            }(i)
        }
        
        wg.Wait()
        close(errors)
        
        duration := time.Since(start)
        
        // エラーチェック
        for err := range errors {
            t.Error(err)
        }
        
        // パフォーマンス検証
        assert.Less(t, duration, 10*time.Second, "User creation should complete within 10 seconds")
        
        // データベース検証
        var count int
        err := env.Database.QueryRow("SELECT COUNT(*) FROM users WHERE email LIKE 'user%@test.com'").Scan(&count)
        require.NoError(t, err)
        assert.Equal(t, numUsers, count)
        
        log.Printf("✅ Created %d users in %v (%.2f users/sec)", 
            numUsers, duration, float64(numUsers)/duration.Seconds())
    })
}
```

dockertestを使用してPostgreSQLコンテナを起動し、接続を確立します：

### データベーススキーマの初期化

テスト実行前にテーブル作成とテストデータの投入を行います：

```go
func initSchema(db *sql.DB) error {
    schema := `
    CREATE TABLE IF NOT EXISTS users (
        id SERIAL PRIMARY KEY,
        name VARCHAR(100) NOT NULL,
        email VARCHAR(100) UNIQUE NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    
    CREATE TABLE IF NOT EXISTS posts (
        id SERIAL PRIMARY KEY,
        user_id INTEGER REFERENCES users(id),
        title VARCHAR(200) NOT NULL,
        content TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`
    
    _, err := db.Exec(schema)
    return err
}

func seedTestData(db *sql.DB) error {
    users := []struct {
        name, email string
    }{
        {"Alice", "alice@example.com"},
        {"Bob", "bob@example.com"},
    }
    
    for _, user := range users {
        _, err := db.Exec(
            "INSERT INTO users (name, email) VALUES ($1, $2)",
            user.name, user.email)
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### HTTPサーバーのテスト起動

テスト用のHTTPサーバーを起動し、実際のHTTPリクエストでテストします：

```go
func setupTestServer(db *sql.DB) *httptest.Server {
    userRepo := NewUserRepository(db)
    userService := NewUserService(userRepo)
    handler := NewUserHandler(userService)
    
    mux := http.NewServeMux()
    mux.HandleFunc("/users", handler.CreateUser)
    mux.HandleFunc("/users/", handler.GetUser)
    
    return httptest.NewServer(mux)
}

func TestUserAPI_CreateAndGet(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()
    
    require.NoError(t, initSchema(db))
    
    server := setupTestServer(db)
    defer server.Close()
    
    // ユーザー作成
    payload := `{"name": "Test User", "email": "test@example.com"}`
    resp, err := http.Post(server.URL+"/users", "application/json", 
        strings.NewReader(payload))
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
    
    var created map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&created)
    require.NoError(t, err)
    
    userID := int(created["id"].(float64))
    
    // ユーザー取得
    resp, err = http.Get(fmt.Sprintf("%s/users/%d", server.URL, userID))
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    
    var retrieved map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&retrieved)
    require.NoError(t, err)
    
    assert.Equal(t, "Test User", retrieved["name"])
    assert.Equal(t, "test@example.com", retrieved["email"])
}
```

### テストの並列実行

複数のテストケースを並列実行する際は、各テストで独立したデータベースを使用します：

```go
func TestUserAPI_Parallel(t *testing.T) {
    tests := []struct {
        name     string
        testFunc func(t *testing.T, server *httptest.Server)
    }{
        {"CreateUser", testCreateUser},
        {"GetUser", testGetUser},
        {"UpdateUser", testUpdateUser},
        {"DeleteUser", testDeleteUser},
    }
    
    for _, tt := range tests {
        tt := tt // capture loop variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // 並列実行を有効化
            
            db, cleanup := setupTestDB(t)
            defer cleanup()
            
            require.NoError(t, initSchema(db))
            require.NoError(t, seedTestData(db))
            
            server := setupTestServer(db)
            defer server.Close()
            
            tt.testFunc(t, server)
        })
    }
}
```

### トランザクションテスト

データベーストランザクションの動作を検証します：

```go
func TestUserService_Transaction(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()
    
    require.NoError(t, initSchema(db))
    
    service := NewUserService(NewUserRepository(db))
    
    // トランザクション内でエラーが発生した場合のロールバックテスト
    err := service.CreateUserWithPosts(&User{
        Name:  "Test User",
        Email: "invalid-email", // バリデーションエラーを意図的に発生
    }, []Post{
        {Title: "Post 1", Content: "Content 1"},
    })
    
    assert.Error(t, err)
    
    // ユーザーが作成されていないことを確認
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM users WHERE name = 'Test User'").Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 0, count)
}
```

### クリーンアップの重要性

テスト実行後は必ずリソースをクリーンアップします：

```go
func TestMain(m *testing.M) {
    // テスト実行前の準備
    pool, err := dockertest.NewPool("")
    if err != nil {
        log.Fatalf("Could not create pool: %s", err)
    }
    
    // テスト実行
    code := m.Run()
    
    // 全体的なクリーンアップ
    // 残ったコンテナがあれば削除
    
    os.Exit(code)
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **データベース接続**
   - PostgreSQLコンテナとの接続確立
   - 接続プールの適切な管理

2. **ユーザー管理API**
   - ユーザーの作成、取得、更新、削除
   - バリデーション処理とエラーハンドリング

3. **投稿管理API**
   - 投稿の作成、取得、一覧表示
   - ユーザーとの関連性管理

4. **トランザクション処理**
   - 複数のデータ操作を一括実行
   - エラー時の適切なロールバック

5. **テストヘルパー**
   - dockertestによるDB起動
   - スキーマ初期化とテストデータ投入
   - HTTPサーバーのテスト起動

## ✅ 期待される挙動

### ユーザー作成API
```bash
POST /api/users
Content-Type: application/json

{
  "name": "John Doe",
  "email": "john@example.com"
}

Response:
HTTP/1.1 201 Created
Content-Type: application/json

{
  "id": 1,
  "name": "John Doe", 
  "email": "john@example.com",
  "created_at": "2024-01-15T10:30:00Z"
}
```

### ユーザー取得API
```bash
GET /api/users/1

Response:
HTTP/1.1 200 OK
Content-Type: application/json

{
  "id": 1,
  "name": "John Doe",
  "email": "john@example.com",
  "created_at": "2024-01-15T10:30:00Z",
  "posts": [
    {
      "id": 1,
      "title": "First Post",
      "content": "Hello World",
      "created_at": "2024-01-15T11:00:00Z"
    }
  ]
}
```

### エラーレスポンス
```bash
POST /api/users
Content-Type: application/json

{
  "name": "",
  "email": "invalid-email"
}

Response:
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "validation failed",
  "details": {
    "name": "name is required",
    "email": "invalid email format"
  }
}
```

## 💡 ヒント

1. **dockertest.NewPool()**: Dockerプールの作成
2. **pool.Run()**: コンテナの起動
3. **pool.Retry()**: 接続試行のリトライ
4. **pool.Purge()**: コンテナの削除
5. **sql.Open()**: データベース接続
6. **httptest.NewServer()**: テスト用HTTPサーバー
7. **t.Parallel()**: テストの並列実行
8. **t.Cleanup()**: テスト終了時の自動クリーンアップ

### dockertestの最適化

大量のテストを実行する際のパフォーマンス向上：

```go
var (
    testDB   *sql.DB
    testPool *dockertest.Pool
    testResource *dockertest.Resource
)

func TestMain(m *testing.M) {
    var err error
    testPool, err = dockertest.NewPool("")
    if err != nil {
        log.Fatalf("Could not create pool: %s", err)
    }
    
    // 全テストで共有するDBコンテナを起動
    testResource, err = testPool.Run("postgres", "13", []string{
        "POSTGRES_PASSWORD=testpass",
        "POSTGRES_DB=testdb", 
        "POSTGRES_USER=testuser",
    })
    if err != nil {
        log.Fatalf("Could not start resource: %s", err)
    }
    
    // 接続確立
    if err = testPool.Retry(func() error {
        var err error
        testDB, err = sql.Open("postgres", fmt.Sprintf(
            "postgres://testuser:testpass@localhost:%s/testdb?sslmode=disable",
            testResource.GetPort("5432/tcp")))
        if err != nil {
            return err
        }
        return testDB.Ping()
    }); err != nil {
        log.Fatalf("Could not connect to database: %s", err)
    }
    
    code := m.Run()
    
    // クリーンアップ
    testDB.Close()
    testPool.Purge(testResource)
    
    os.Exit(code)
}
```

### テストデータの分離

並列テストでのデータ競合を防ぐため、テストごとに独立したデータセットを使用：

```go
func createTestUser(t *testing.T, db *sql.DB) *User {
    user := &User{
        Name:  fmt.Sprintf("test-user-%d", time.Now().UnixNano()),
        Email: fmt.Sprintf("test-%d@example.com", time.Now().UnixNano()),
    }
    
    err := db.QueryRow(
        "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, created_at",
        user.Name, user.Email).Scan(&user.ID, &user.CreatedAt)
    require.NoError(t, err)
    
    return user
}
```

これらの実装により、本格的な統合テスト環境を構築し、データベースと連携するWebAPIの品質を保証できます。