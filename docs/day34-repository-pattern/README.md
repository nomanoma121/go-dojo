# Day 34: Repositoryパターン

🎯 **本日の目標**

DB操作のロジックをカプセル化し、ビジネスロジックから分離するRepositoryパターンを実装できるようになる。データアクセス層の抽象化によりテスタビリティと保守性を向上させる。

📖 **解説**

### Repositoryパターンの重要性

```go
// 【Repositoryパターンの重要性】データアクセス層の適切な抽象化と保守性向上
// ❌ 問題例：データアクセスロジックが散在し保守不可能なシステム
func catastrophicDirectDatabaseAccess() {
    // 🚨 災害例：直接SQL操作でスパゲッティコード化
    
    http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
        // ❌ ハンドラに直接SQL書き込み（最悪のアンチパターン）
        db, err := sql.Open("postgres", "postgres://user:pass@localhost/db")
        if err != nil {
            log.Fatal("Database connection failed:", err)
        }
        defer db.Close()
        
        // ❌ SQL文字列が各所に散在→保守地獄
        query := "SELECT id, name, email FROM users WHERE deleted_at IS NULL"
        rows, err := db.Query(query)
        if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        defer rows.Close()
        
        var users []User
        for rows.Next() {
            var u User
            rows.Scan(&u.ID, &u.Name, &u.Email)
            users = append(users, u)
        }
        
        json.NewEncoder(w).Encode(users)
    })
    
    http.HandleFunc("/users/create", func(w http.ResponseWriter, r *http.Request) {
        var user User
        json.NewDecoder(r.Body).Decode(&user)
        
        // ❌ 同じようなSQL文が別の場所にも（重複）
        db, err := sql.Open("postgres", "postgres://user:pass@localhost/db")
        if err != nil {
            http.Error(w, "DB connection failed", http.StatusInternalServerError)
            return
        }
        defer db.Close()
        
        // ❌ SQLインジェクション脆弱性（プレースホルダーなし）
        insertQuery := fmt.Sprintf("INSERT INTO users (name, email) VALUES ('%s', '%s')", 
            user.Name, user.Email)
        
        _, err = db.Exec(insertQuery)
        if err != nil {
            log.Printf("Insert failed: %v", err)
            http.Error(w, "Failed to create user", http.StatusInternalServerError)
            return
        }
        
        w.WriteHeader(http.StatusCreated)
    })
    
    // 【災害シナリオ】
    // 1. 100箇所にSQL文が散在→仕様変更で100箇所修正
    // 2. データベーススキーマ変更→全コード調査と修正
    // 3. SQLインジェクション脆弱性→データ全削除・漏洩
    // 4. トランザクション管理不備→データ整合性破綻
    // 5. テスト不可能→品質保証不可、バグ多発
    // 6. 接続プール管理なし→パフォーマンス最悪
    
    log.Println("❌ Starting server with direct database access...")
    http.ListenAndServe(":8080", nil)
    // 結果：保守不可能、セキュリティホール、パフォーマンス最悪、開発効率激減
}

// ✅ 正解：エンタープライズ級Repositoryパターンシステム
type EnterpriseRepositorySystem struct {
    // 【基本Repository層】
    userRepo        UserRepository              // ユーザーデータ
    productRepo     ProductRepository           // 商品データ  
    orderRepo       OrderRepository             // 注文データ
    auditRepo       AuditRepository             // 監査データ
    
    // 【高度なデータアクセス】
    cacheRepo       CacheRepository             // キャッシュ統合
    searchRepo      SearchRepository            // 全文検索
    analyticsRepo   AnalyticsRepository         // 分析データ
    timeSeriesRepo  TimeSeriesRepository        // 時系列データ
    
    // 【トランザクション管理】
    unitOfWork      UnitOfWork                  // トランザクション境界
    txManager       TransactionManager          // 分散トランザクション
    sagaManager     SagaManager                 // Sagaパターン実装
    
    // 【パフォーマンス最適化】
    connectionPool  *ConnectionPool             // 接続プール
    queryOptimizer  *QueryOptimizer             // クエリ最適化
    batchProcessor  *BatchProcessor             // バッチ処理
    readReplicaManager *ReadReplicaManager      // リードレプリカ管理
    
    // 【監視・ログ】
    queryLogger     *QueryLogger                // クエリログ
    performanceMonitor *PerformanceMonitor      // パフォーマンス監視
    alertManager    *AlertManager               // アラート管理
    
    // 【セキュリティ】
    accessController *AccessController          // アクセス制御
    dataEncryption  *DataEncryption             // データ暗号化
    auditLogger     *AuditLogger                // 監査ログ
    
    // 【高可用性】
    failoverManager *FailoverManager            // フェイルオーバー
    backupManager   *BackupManager              // バックアップ管理
    
    mu              sync.RWMutex                // 設定変更保護
}

// 【重要関数】エンタープライズRepository初期化
func NewEnterpriseRepositorySystem(config *RepositoryConfig) *EnterpriseRepositorySystem {
    // 【接続プール初期化】
    connectionPool := NewConnectionPool(&ConnectionPoolConfig{
        MaxOpenConns:    config.MaxOpenConns,
        MaxIdleConns:    config.MaxIdleConns,
        ConnMaxLifetime: config.ConnMaxLifetime,
        ConnMaxIdleTime: config.ConnMaxIdleTime,
    })
    
    system := &EnterpriseRepositorySystem{
        userRepo:        NewPostgreSQLUserRepository(connectionPool, config.UserTableConfig),
        productRepo:     NewPostgreSQLProductRepository(connectionPool, config.ProductTableConfig),
        orderRepo:       NewPostgreSQLOrderRepository(connectionPool, config.OrderTableConfig),
        auditRepo:       NewPostgreSQLAuditRepository(connectionPool, config.AuditTableConfig),
        cacheRepo:       NewRedisRepository(config.RedisConfig),
        searchRepo:      NewElasticsearchRepository(config.ElasticsearchConfig),
        analyticsRepo:   NewClickHouseRepository(config.ClickHouseConfig),
        timeSeriesRepo:  NewInfluxDBRepository(config.InfluxDBConfig),
        unitOfWork:      NewUnitOfWork(connectionPool),
        txManager:       NewTransactionManager(config.TxConfig),
        sagaManager:     NewSagaManager(config.SagaConfig),
        connectionPool:  connectionPool,
        queryOptimizer:  NewQueryOptimizer(config.OptimizationConfig),
        batchProcessor:  NewBatchProcessor(config.BatchConfig),
        readReplicaManager: NewReadReplicaManager(config.ReplicaConfig),
        queryLogger:     NewQueryLogger(config.LogConfig),
        performanceMonitor: NewPerformanceMonitor(config.MonitorConfig),
        alertManager:    NewAlertManager(config.AlertConfig),
        accessController: NewAccessController(config.SecurityConfig),
        dataEncryption:  NewDataEncryption(config.EncryptionConfig),
        auditLogger:     NewAuditLogger(config.AuditConfig),
        failoverManager: NewFailoverManager(config.FailoverConfig),
        backupManager:   NewBackupManager(config.BackupConfig),
    }
    
    // 【重要】バックグラウンド処理開始
    go system.startConnectionPoolMonitoring()
    go system.startQueryPerformanceAnalysis()
    go system.startHealthChecking()
    go system.startBackupScheduling()
    
    log.Printf("🗄️  Enterprise repository system initialized")
    log.Printf("   Connection pool: max_open=%d, max_idle=%d", 
        config.MaxOpenConns, config.MaxIdleConns)
    log.Printf("   Read replicas: %d configured", len(config.ReplicaConfig.Replicas))
    log.Printf("   Cache layer: %s", config.RedisConfig.ClusterNodes)
    log.Printf("   Search engine: %s", config.ElasticsearchConfig.Addresses)
    
    return system
}

// 【核心インターフェース】汎用Repository基底
type BaseRepository[T Entity] interface {
    // 【基本CRUD操作】
    Create(ctx context.Context, entity T) error
    GetByID(ctx context.Context, id EntityID) (T, error)
    Update(ctx context.Context, entity T) error
    Delete(ctx context.Context, id EntityID) error
    
    // 【検索・フィルタリング】
    FindBySpec(ctx context.Context, spec Specification[T]) ([]T, error)
    List(ctx context.Context, opts ListOptions) ([]T, error)
    Count(ctx context.Context, spec Specification[T]) (int64, error)
    
    // 【バッチ操作】
    CreateBatch(ctx context.Context, entities []T) error
    UpdateBatch(ctx context.Context, entities []T) error
    DeleteBatch(ctx context.Context, ids []EntityID) error
    
    // 【トランザクション対応】
    WithTx(tx Transaction) BaseRepository[T]
    
    // 【パフォーマンス最適化】
    Preload(ctx context.Context, relations ...string) BaseRepository[T]
    WithCache(ctx context.Context, ttl time.Duration) BaseRepository[T]
    WithReadReplica(ctx context.Context) BaseRepository[T]
}

// 【高度な実装】PostgreSQLユーザーRepository
type PostgreSQLUserRepository struct {
    // 【データアクセス】
    pool            *ConnectionPool             // 接続プール
    tx              Transaction                 // トランザクション
    cache           CacheRepository             // キャッシュ層
    readReplica     *ReadReplicaManager         // リードレプリカ
    
    // 【設定・最適化】
    tableConfig     *TableConfig                // テーブル設定
    queryBuilder    *QueryBuilder               // クエリビルダー
    queryOptimizer  *QueryOptimizer             // クエリ最適化
    
    // 【監視・ログ】
    queryLogger     *QueryLogger                // クエリログ
    metricsCollector *MetricsCollector          // メトリクス収集
    
    // 【セキュリティ】
    accessController *AccessController          // アクセス制御
    dataEncryption  *DataEncryption             // データ暗号化
    auditLogger     *AuditLogger                // 監査ログ
    
    // 【設定】
    useCache        bool                        // キャッシュ使用フラグ
    useReadReplica  bool                        // リードレプリカ使用フラグ
    preloadRelations []string                   // プリロード関係
    
    mu              sync.RWMutex                // 設定変更保護
}

// 【重要関数】PostgreSQLユーザーRepository初期化
func NewPostgreSQLUserRepository(
    pool *ConnectionPool, 
    config *TableConfig,
) UserRepository {
    return &PostgreSQLUserRepository{
        pool:            pool,
        tableConfig:     config,
        queryBuilder:    NewQueryBuilder(config.TableName, config.Schema),
        queryOptimizer:  pool.GetQueryOptimizer(),
        queryLogger:     pool.GetQueryLogger(),
        metricsCollector: pool.GetMetricsCollector(),
        accessController: pool.GetAccessController(),
        dataEncryption:  pool.GetDataEncryption(),
        auditLogger:     pool.GetAuditLogger(),
        cache:           pool.GetCacheRepository(),
        readReplica:     pool.GetReadReplicaManager(),
    }
}

// 【核心メソッド】高度なCreate実装
func (r *PostgreSQLUserRepository) Create(ctx context.Context, user *User) error {
    startTime := time.Now()
    operationID := generateOperationID()
    
    // 【STEP 1】アクセス制御チェック
    if !r.accessController.CanCreate(ctx, "users", user) {
        r.auditLogger.LogUnauthorizedAccess(ctx, operationID, "CREATE", "users", user.ID)
        return ErrUnauthorized
    }
    
    // 【STEP 2】データバリデーション
    if err := r.validateUser(user); err != nil {
        r.metricsCollector.RecordValidationError("users", "create")
        return fmt.Errorf("validation failed: %w", err)
    }
    
    // 【STEP 3】機密データ暗号化
    encryptedUser, err := r.dataEncryption.EncryptUserData(user)
    if err != nil {
        r.metricsCollector.RecordEncryptionError("users", "create")
        return fmt.Errorf("encryption failed: %w", err)
    }
    
    // 【STEP 4】重複チェック（キャッシュ経由）
    existing, err := r.findByEmailFromCache(ctx, user.Email)
    if err != nil && !errors.Is(err, ErrNotFound) {
        return fmt.Errorf("duplicate check failed: %w", err)
    }
    if existing != nil {
        r.metricsCollector.RecordDuplicateError("users", "email")
        return ErrDuplicateEmail
    }
    
    // 【STEP 5】SQLクエリ生成・最適化
    query, args := r.queryBuilder.Insert().
        Values(map[string]interface{}{
            "id":           encryptedUser.ID,
            "username":     encryptedUser.Username,
            "email":        encryptedUser.Email,
            "password_hash": encryptedUser.PasswordHash,
            "profile_data": encryptedUser.ProfileData,
            "created_at":   time.Now(),
            "updated_at":   time.Now(),
        }).
        Returning("id", "created_at").
        Build()
    
    // クエリ最適化
    optimizedQuery := r.queryOptimizer.OptimizeInsert(query, args)
    
    // 【STEP 6】データベース実行
    var db QueryExecutor
    if r.tx != nil {
        db = r.tx
    } else {
        conn, err := r.pool.AcquireWriteConnection(ctx)
        if err != nil {
            r.metricsCollector.RecordConnectionError("write")
            return fmt.Errorf("connection acquisition failed: %w", err)
        }
        defer r.pool.ReleaseConnection(conn)
        db = conn
    }
    
    // クエリ実行
    var createdAt time.Time
    err = db.QueryRowContext(ctx, optimizedQuery.SQL, optimizedQuery.Args...).
        Scan(&user.ID, &createdAt)
    
    if err != nil {
        r.queryLogger.LogFailedQuery(ctx, operationID, optimizedQuery.SQL, err)
        r.metricsCollector.RecordQueryError("users", "create")
        
        // PostgreSQL固有エラーの変換
        if isPGDuplicateKeyError(err) {
            return ErrDuplicateKey
        }
        return fmt.Errorf("insert execution failed: %w", err)
    }
    
    // 【STEP 7】作成完了処理
    user.CreatedAt = createdAt
    user.UpdatedAt = createdAt
    
    // 【STEP 8】キャッシュ更新
    if r.useCache {
        cacheKey := fmt.Sprintf("user:id:%s", user.ID)
        r.cache.Set(ctx, cacheKey, user, 10*time.Minute)
        
        emailCacheKey := fmt.Sprintf("user:email:%s", user.Email)
        r.cache.Set(ctx, emailCacheKey, user, 10*time.Minute)
    }
    
    // 【STEP 9】監査ログ記録
    r.auditLogger.LogDataAccess(ctx, &AuditEntry{
        OperationID:   operationID,
        EntityType:    "users",
        EntityID:      user.ID,
        Operation:     "CREATE",
        ActorID:       getActorIDFromContext(ctx),
        Data:          user,
        Timestamp:     time.Now(),
        IPAddress:     getIPFromContext(ctx),
        UserAgent:     getUserAgentFromContext(ctx),
    })
    
    // 【STEP 10】メトリクス記録
    duration := time.Since(startTime)
    r.metricsCollector.RecordQueryDuration("users", "create", duration)
    r.metricsCollector.RecordSuccessfulOperation("users", "create")
    
    // 【STEP 11】クエリログ記録
    r.queryLogger.LogSuccessfulQuery(ctx, &QueryLog{
        OperationID: operationID,
        Query:       optimizedQuery.SQL,
        Args:        optimizedQuery.Args,
        Duration:    duration,
        RowsAffected: 1,
        EntityType:  "users",
        Operation:   "CREATE",
    })
    
    return nil
}

// 【核心メソッド】高度なFindBySpec実装
func (r *PostgreSQLUserRepository) FindBySpec(
    ctx context.Context, 
    spec UserSpecification,
) ([]*User, error) {
    startTime := time.Now()
    operationID := generateOperationID()
    
    // 【STEP 1】アクセス制御チェック
    if !r.accessController.CanRead(ctx, "users", spec) {
        r.auditLogger.LogUnauthorizedAccess(ctx, operationID, "READ", "users", "")
        return nil, ErrUnauthorized
    }
    
    // 【STEP 2】キャッシュチェック
    if r.useCache {
        cacheKey := spec.CacheKey()
        if cached, err := r.cache.Get(ctx, cacheKey); err == nil {
            var users []*User
            if err := json.Unmarshal(cached, &users); err == nil {
                r.metricsCollector.RecordCacheHit("users", "find_by_spec")
                return users, nil
            }
        }
        r.metricsCollector.RecordCacheMiss("users", "find_by_spec")
    }
    
    // 【STEP 3】クエリ生成・最適化
    queryBuilder := r.queryBuilder.Select().
        Columns("id", "username", "email", "profile_data", "created_at", "updated_at")
    
    // Specification適用
    whereClause, args := spec.ToSQL()
    queryBuilder = queryBuilder.Where(whereClause, args...)
    
    // プリロード関係の処理
    for _, relation := range r.preloadRelations {
        queryBuilder = queryBuilder.Join(relation)
    }
    
    query, queryArgs := queryBuilder.Build()
    optimizedQuery := r.queryOptimizer.OptimizeSelect(query, queryArgs)
    
    // 【STEP 4】データベース接続取得
    var db QueryExecutor
    if r.useReadReplica {
        conn, err := r.readReplica.AcquireReadConnection(ctx)
        if err != nil {
            // フォールバック：マスター接続
            conn, err = r.pool.AcquireReadConnection(ctx)
            if err != nil {
                r.metricsCollector.RecordConnectionError("read")
                return nil, fmt.Errorf("connection acquisition failed: %w", err)
            }
        }
        defer r.pool.ReleaseConnection(conn)
        db = conn
    } else {
        conn, err := r.pool.AcquireReadConnection(ctx)
        if err != nil {
            r.metricsCollector.RecordConnectionError("read")
            return nil, fmt.Errorf("connection acquisition failed: %w", err)
        }
        defer r.pool.ReleaseConnection(conn)
        db = conn
    }
    
    // 【STEP 5】クエリ実行
    rows, err := db.QueryContext(ctx, optimizedQuery.SQL, optimizedQuery.Args...)
    if err != nil {
        r.queryLogger.LogFailedQuery(ctx, operationID, optimizedQuery.SQL, err)
        r.metricsCollector.RecordQueryError("users", "find_by_spec")
        return nil, fmt.Errorf("query execution failed: %w", err)
    }
    defer rows.Close()
    
    // 【STEP 6】結果セット処理
    var users []*User
    rowCount := 0
    
    for rows.Next() {
        user := &User{}
        var encryptedProfileData []byte
        
        err := rows.Scan(
            &user.ID,
            &user.Username,
            &user.Email,
            &encryptedProfileData,
            &user.CreatedAt,
            &user.UpdatedAt,
        )
        if err != nil {
            r.metricsCollector.RecordScanError("users")
            return nil, fmt.Errorf("row scan failed: %w", err)
        }
        
        // データ復号化
        if err := r.dataEncryption.DecryptUserProfile(user, encryptedProfileData); err != nil {
            r.metricsCollector.RecordDecryptionError("users", "find_by_spec")
            log.Printf("Failed to decrypt user profile: %v", err)
            // エラーログを記録するが処理続行
        }
        
        users = append(users, user)
        rowCount++
    }
    
    if err := rows.Err(); err != nil {
        r.metricsCollector.RecordQueryError("users", "find_by_spec")
        return nil, fmt.Errorf("rows iteration failed: %w", err)
    }
    
    // 【STEP 7】キャッシュ更新
    if r.useCache && len(users) > 0 {
        cacheKey := spec.CacheKey()
        if cacheData, err := json.Marshal(users); err == nil {
            r.cache.Set(ctx, cacheKey, cacheData, 5*time.Minute)
        }
    }
    
    // 【STEP 8】監査ログ記録
    r.auditLogger.LogDataAccess(ctx, &AuditEntry{
        OperationID:   operationID,
        EntityType:    "users",
        Operation:     "FIND_BY_SPEC",
        ActorID:       getActorIDFromContext(ctx),
        QuerySpec:     spec.String(),
        ResultCount:   rowCount,
        Timestamp:     time.Now(),
        IPAddress:     getIPFromContext(ctx),
        UserAgent:     getUserAgentFromContext(ctx),
    })
    
    // 【STEP 9】メトリクス記録
    duration := time.Since(startTime)
    r.metricsCollector.RecordQueryDuration("users", "find_by_spec", duration)
    r.metricsCollector.RecordRowsReturned("users", "find_by_spec", rowCount)
    r.metricsCollector.RecordSuccessfulOperation("users", "find_by_spec")
    
    // 【STEP 10】クエリログ記録
    r.queryLogger.LogSuccessfulQuery(ctx, &QueryLog{
        OperationID:  operationID,
        Query:        optimizedQuery.SQL,
        Args:         optimizedQuery.Args,
        Duration:     duration,
        RowsReturned: rowCount,
        EntityType:   "users",
        Operation:    "FIND_BY_SPEC",
    })
    
    return users, nil
}

// 【実用例】プロダクション環境でのRepository使用
func ProductionRepositoryUsage() {
    // 【設定】エンタープライズRepository設定
    config := &RepositoryConfig{
        MaxOpenConns:    50,
        MaxIdleConns:    10,
        ConnMaxLifetime: 1 * time.Hour,
        ConnMaxIdleTime: 30 * time.Minute,
        UserTableConfig: &TableConfig{
            TableName: "users",
            Schema:    "public",
            PrimaryKey: "id",
            Indexes:   []string{"email", "username", "created_at"},
        },
        RedisConfig: &RedisConfig{
            ClusterNodes: []string{"redis-1:6379", "redis-2:6379", "redis-3:6379"},
            Password:     getEnv("REDIS_PASSWORD"),
        },
        ElasticsearchConfig: &ElasticsearchConfig{
            Addresses: []string{"es-1:9200", "es-2:9200", "es-3:9200"},
            Username:  getEnv("ES_USERNAME"),
            Password:  getEnv("ES_PASSWORD"),
        },
        ReplicaConfig: &ReplicaConfig{
            Replicas: []ReplicaInfo{
                {Host: "replica-1", Weight: 50},
                {Host: "replica-2", Weight: 30},
                {Host: "replica-3", Weight: 20},
            },
            LoadBalanceStrategy: "WEIGHTED_ROUND_ROBIN",
        },
        EncryptionConfig: &EncryptionConfig{
            KeyID:       getEnv("ENCRYPTION_KEY_ID"),
            Algorithm:   "AES-256-GCM",
            RotationInterval: 30 * 24 * time.Hour, // 30日
        },
    }
    
    repoSystem := NewEnterpriseRepositorySystem(config)
    
    // 【ビジネスロジック層】
    userService := &UserService{
        userRepo:    repoSystem.userRepo,
        auditRepo:   repoSystem.auditRepo,
        unitOfWork:  repoSystem.unitOfWork,
    }
    
    // 【HTTP ハンドラー】
    http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        
        switch r.Method {
        case http.MethodGet:
            // 【検索】Specificationパターン使用
            spec := &UserActiveSpecification{
                CreatedAfter: time.Now().AddDate(0, -1, 0), // 1ヶ月以内
            }
            
            users, err := repoSystem.userRepo.WithCache(ctx, 5*time.Minute).
                WithReadReplica(ctx).
                FindBySpec(ctx, spec)
            
            if err != nil {
                http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
                return
            }
            
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(users)
            
        case http.MethodPost:
            // 【作成】トランザクション使用
            var user User
            if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
                http.Error(w, "Invalid JSON", http.StatusBadRequest)
                return
            }
            
            err := userService.CreateUserWithProfile(ctx, &user)
            if err != nil {
                http.Error(w, "Failed to create user", http.StatusInternalServerError)
                return
            }
            
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusCreated)
            json.NewEncoder(w).Encode(user)
        }
    })
    
    // 【管理エンドポイント】
    http.HandleFunc("/admin/repository/stats", func(w http.ResponseWriter, r *http.Request) {
        stats := map[string]interface{}{
            "connection_pool": repoSystem.connectionPool.GetStats(),
            "cache_stats":     repoSystem.cacheRepo.GetStats(),
            "query_stats":     repoSystem.queryLogger.GetStats(),
            "performance":     repoSystem.performanceMonitor.GetStats(),
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(stats)
    })
    
    // 【サーバー起動】
    server := &http.Server{
        Addr:    ":8080",
        Handler: nil,
        ReadTimeout:  30 * time.Second,
        WriteTimeout: 30 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    log.Printf("🚀 Enterprise repository server starting on :8080")
    log.Printf("   Database connections: max_open=%d, max_idle=%d", 
        config.MaxOpenConns, config.MaxIdleConns)
    log.Printf("   Cache layer: Redis cluster with %d nodes", 
        len(config.RedisConfig.ClusterNodes))
    log.Printf("   Search engine: Elasticsearch with %d nodes", 
        len(config.ElasticsearchConfig.Addresses))
    log.Printf("   Read replicas: %d configured", len(config.ReplicaConfig.Replicas))
    log.Printf("   Data encryption: %s", config.EncryptionConfig.Algorithm)
    
    log.Fatal(server.ListenAndServe())
}
```

## Repositoryパターンとは

Repositoryパターンは、データアクセス層を抽象化するデザインパターンです。ビジネスロジックからデータベースの詳細を隠蔽し、データアクセスロジックを一箇所に集約することで、保守性とテスタビリティを向上させます。

### Repositoryパターンの利点

1. **関心の分離**: ビジネスロジックとデータアクセスロジックを分離
2. **テスタビリティ**: インターフェースによりモック可能
3. **保守性**: データアクセス層の変更がビジネスロジックに影響しない
4. **再利用性**: 共通のデータアクセスロジックを再利用可能

### 基本的なRepositoryパターンの実装

```go
package main

import (
    "context"
    "database/sql"
)

// User represents a user entity
type User struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
    Email    string `json:"email"`
    Created  time.Time `json:"created"`
}

// UserRepository defines the interface for user data access
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id int) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id int) error
    List(ctx context.Context, limit, offset int) ([]*User, error)
}

// PostgreSQLUserRepository implements UserRepository for PostgreSQL
type PostgreSQLUserRepository struct {
    db *sql.DB
}

// NewPostgreSQLUserRepository creates a new PostgreSQL user repository
func NewPostgreSQLUserRepository(db *sql.DB) UserRepository {
    return &PostgreSQLUserRepository{db: db}
}

// Create creates a new user
func (r *PostgreSQLUserRepository) Create(ctx context.Context, user *User) error {
    query := `
        INSERT INTO users (username, email, created) 
        VALUES ($1, $2, $3) 
        RETURNING id`
    
    err := r.db.QueryRowContext(ctx, query, user.Username, user.Email, time.Now()).
        Scan(&user.ID)
    return err
}

// GetByID retrieves a user by ID
func (r *PostgreSQLUserRepository) GetByID(ctx context.Context, id int) (*User, error) {
    query := `
        SELECT id, username, email, created 
        FROM users 
        WHERE id = $1`
    
    user := &User{}
    err := r.db.QueryRowContext(ctx, query, id).
        Scan(&user.ID, &user.Username, &user.Email, &user.Created)
    
    if err == sql.ErrNoRows {
        return nil, nil
    }
    return user, err
}
```

### トランザクション対応のRepository

```go
// TxRepository represents a repository that can work with transactions
type TxRepository interface {
    UserRepository
    WithTx(tx *sql.Tx) UserRepository
}

// PostgreSQLUserTxRepository extends PostgreSQL repository with transaction support
type PostgreSQLUserTxRepository struct {
    db *sql.DB
    tx *sql.Tx
}

// NewPostgreSQLUserTxRepository creates a transaction-aware repository
func NewPostgreSQLUserTxRepository(db *sql.DB) TxRepository {
    return &PostgreSQLUserTxRepository{db: db}
}

// WithTx returns a repository that uses the provided transaction
func (r *PostgreSQLUserTxRepository) WithTx(tx *sql.Tx) UserRepository {
    return &PostgreSQLUserTxRepository{db: r.db, tx: tx}
}

// getDB returns the appropriate database connection or transaction
func (r *PostgreSQLUserTxRepository) getDB() interface {
    QueryRowContext(context.Context, string, ...interface{}) *sql.Row
    ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
    QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
} {
    if r.tx != nil {
        return r.tx
    }
    return r.db
}

// Create with transaction support
func (r *PostgreSQLUserTxRepository) Create(ctx context.Context, user *User) error {
    query := `
        INSERT INTO users (username, email, created) 
        VALUES ($1, $2, $3) 
        RETURNING id`
    
    db := r.getDB()
    err := db.QueryRowContext(ctx, query, user.Username, user.Email, time.Now()).
        Scan(&user.ID)
    return err
}
```

### サービス層との統合

```go
// UserService provides business logic for user operations
type UserService struct {
    userRepo UserRepository
    db       *sql.DB
}

// NewUserService creates a new user service
func NewUserService(userRepo UserRepository, db *sql.DB) *UserService {
    return &UserService{
        userRepo: userRepo,
        db:       db,
    }
}

// CreateUserWithProfile creates a user and their profile in a single transaction
func (s *UserService) CreateUserWithProfile(ctx context.Context, user *User, profile *Profile) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // トランザクション対応リポジトリを使用
    txUserRepo := s.userRepo.(TxRepository).WithTx(tx)
    
    // ユーザー作成
    err = txUserRepo.Create(ctx, user)
    if err != nil {
        return err
    }

    // プロフィール作成（ここではダミー実装）
    profile.UserID = user.ID
    // profileRepo.WithTx(tx).Create(ctx, profile)

    return tx.Commit()
}
```

### Unit of Work パターン

```go
// UnitOfWork manages multiple repositories in a single transaction
type UnitOfWork struct {
    db       *sql.DB
    tx       *sql.Tx
    userRepo TxRepository
    postRepo TxRepository
}

// NewUnitOfWork creates a new unit of work
func NewUnitOfWork(db *sql.DB) *UnitOfWork {
    return &UnitOfWork{
        db:       db,
        userRepo: NewPostgreSQLUserTxRepository(db),
        postRepo: NewPostgreSQLPostTxRepository(db),
    }
}

// Begin starts a new transaction
func (uow *UnitOfWork) Begin(ctx context.Context) error {
    tx, err := uow.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    uow.tx = tx
    return nil
}

// Users returns the user repository within the transaction
func (uow *UnitOfWork) Users() UserRepository {
    if uow.tx != nil {
        return uow.userRepo.WithTx(uow.tx)
    }
    return uow.userRepo
}

// Posts returns the post repository within the transaction
func (uow *UnitOfWork) Posts() PostRepository {
    if uow.tx != nil {
        return uow.postRepo.WithTx(uow.tx)
    }
    return uow.postRepo
}

// Commit commits the transaction
func (uow *UnitOfWork) Commit() error {
    if uow.tx == nil {
        return fmt.Errorf("no active transaction")
    }
    err := uow.tx.Commit()
    uow.tx = nil
    return err
}

// Rollback rolls back the transaction
func (uow *UnitOfWork) Rollback() error {
    if uow.tx == nil {
        return nil
    }
    err := uow.tx.Rollback()
    uow.tx = nil
    return err
}
```

### テスト用モックRepository

```go
// MockUserRepository implements UserRepository for testing
type MockUserRepository struct {
    users map[int]*User
    nextID int
    mu     sync.RWMutex
}

// NewMockUserRepository creates a new mock repository
func NewMockUserRepository() UserRepository {
    return &MockUserRepository{
        users:  make(map[int]*User),
        nextID: 1,
    }
}

// Create creates a user in memory
func (m *MockUserRepository) Create(ctx context.Context, user *User) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    user.ID = m.nextID
    m.nextID++
    user.Created = time.Now()
    m.users[user.ID] = user
    return nil
}

// GetByID retrieves a user by ID from memory
func (m *MockUserRepository) GetByID(ctx context.Context, id int) (*User, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    user, exists := m.users[id]
    if !exists {
        return nil, nil
    }
    
    // Return a copy to avoid data races
    userCopy := *user
    return &userCopy, nil
}
```

### Specification パターンの組み合わせ

```go
// UserSpecification defines criteria for querying users
type UserSpecification interface {
    ToSQL() (string, []interface{})
}

// UserByEmailSpec specification for finding users by email
type UserByEmailSpec struct {
    Email string
}

func (s UserByEmailSpec) ToSQL() (string, []interface{}) {
    return "email = $1", []interface{}{s.Email}
}

// UserCreatedAfterSpec specification for finding users created after a date
type UserCreatedAfterSpec struct {
    After time.Time
}

func (s UserCreatedAfterSpec) ToSQL() (string, []interface{}) {
    return "created > $1", []interface{}{s.After}
}

// AndSpec combines specifications with AND
type AndSpec struct {
    Left, Right UserSpecification
}

func (s AndSpec) ToSQL() (string, []interface{}) {
    leftSQL, leftArgs := s.Left.ToSQL()
    rightSQL, rightArgs := s.Right.ToSQL()
    
    sql := fmt.Sprintf("(%s) AND (%s)", leftSQL, rightSQL)
    args := append(leftArgs, rightArgs...)
    return sql, args
}

// Enhanced repository with specification support
func (r *PostgreSQLUserRepository) FindBySpec(ctx context.Context, spec UserSpecification) ([]*User, error) {
    whereClause, args := spec.ToSQL()
    query := fmt.Sprintf("SELECT id, username, email, created FROM users WHERE %s", whereClause)
    
    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var users []*User
    for rows.Next() {
        user := &User{}
        err := rows.Scan(&user.ID, &user.Username, &user.Email, &user.Created)
        if err != nil {
            return nil, err
        }
        users = append(users, user)
    }
    
    return users, rows.Err()
}
```

📝 **課題**

以下の機能を持つRepositoryパターンシステムを実装してください：

1. **`UserRepository`インターフェース**: ユーザーデータアクセスの抽象化
2. **`PostgreSQLUserRepository`**: PostgreSQL実装
3. **`MockUserRepository`**: テスト用インメモリ実装
4. **`UserService`**: ビジネスロジック層
5. **`UnitOfWork`**: トランザクション管理

具体的な実装要件：
- CRUD操作の完全な実装
- トランザクション対応
- エラーハンドリング
- コンテキスト対応
- ページング機能
- 検索機能（Specification パターン）

✅ **期待される挙動**

実装が完了すると、以下のような動作が期待されます：

```bash
$ go test -v
=== RUN   TestUserRepository_Create
--- PASS: TestUserRepository_Create (0.01s)
=== RUN   TestUserRepository_GetByID
--- PASS: TestUserRepository_GetByID (0.01s)
=== RUN   TestUserRepository_Update
--- PASS: TestUserRepository_Update (0.01s)
=== RUN   TestUserRepository_Delete
--- PASS: TestUserRepository_Delete (0.01s)
=== RUN   TestUserRepository_List
--- PASS: TestUserRepository_List (0.01s)
=== RUN   TestUserRepository_FindBySpec
--- PASS: TestUserRepository_FindBySpec (0.02s)
=== RUN   TestUnitOfWork_Transaction
--- PASS: TestUnitOfWork_Transaction (0.01s)
=== RUN   TestUserService_CreateUserWithProfile
--- PASS: TestUserService_CreateUserWithProfile (0.02s)
PASS
ok      day34-repository-pattern    0.095s
```

💡 **ヒント**

実装に詰まった場合は、以下を参考にしてください：

1. **database/sql**: Goの標準SQL ドライバ
2. **context**: リクエストスコープの管理
3. **sync**: 並行安全性（モック実装で必要）
4. **time**: タイムスタンプ処理
5. **fmt**: SQLクエリの動的生成

Repository パターンのベストプラクティス：
- **インターフェース優先**: 具象型ではなくインターフェースに依存
- **単一責任**: 各Repositoryは一つのエンティティに責任を持つ
- **トランザクション透過性**: Repositoryはトランザクションの境界を知らない
- **エラーハンドリング**: データベース固有のエラーをアプリケーションエラーに変換

データベーススキーマ例：
```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    title VARCHAR(255) NOT NULL,
    content TEXT,
    created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## 実行方法

```bash
# PostgreSQLコンテナを起動
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=test -e POSTGRES_DB=testdb postgres:15

# テスト実行
go test -v
go test -race  # レースコンディションの検出
go test -bench=.  # ベンチマークテスト
```