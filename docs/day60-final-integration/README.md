# Day 60: 総集編 - Production-Ready Microservice

## 🎯 本日の目標 (Today's Goal)

Go道場60日間で学習した全ての技術（slog、Prometheus、OpenTelemetry、gRPC、分散システムパターン、並行処理、キャッシュ戦略、メッセージング）を統合し、本格的なプロダクションレベルのE-commerceマイクロサービスプラットフォームを構築できるようになる。現実的なビジネス要件に対応した拡張性、信頼性、保守性を備えたシステムアーキテクチャを習得する。

## 📖 解説 (Explanation)

### 総集編の背景と意義

60日間のGoプログラミング学習の集大成として、単なる技術的な統合ではなく、実際のビジネス要件に応える実用的なシステムを構築します。これまで学んだ技術要素を有機的に結合し、エンタープライズレベルの品質を持つマイクロサービスプラットフォームを完成させます。

```go
// 【Go道場60日間の集大成】エンタープライズE-commerceプラットフォーム
// ❌ 問題例：技術的負債が蓄積した旧システムの課題
func catastrophicLegacyEcommerceSystem() {
    // 🚨 災害例：技術的負債まみれのレガシーシステム
    
    // 【問題の旧システム構成】
    // - モノリシック構造：全機能が1つのプロセス
    // - データベース直結：ビジネスロジックとDB処理が混在
    // - 監視なし：問題の早期発見不可能
    // - 並行処理なし：スケーラビリティの欠如
    // - エラーハンドリング貧弱：障害の連鎖反応
    
    http.HandleFunc("/checkout", func(w http.ResponseWriter, r *http.Request) {
        // 【致命的問題の連鎖】
        
        // 1. 【在庫確認】データベース直接操作（N+1問題）
        for _, item := range parseCartItems(r) {
            // ❌ 各商品ごとにDB接続→パフォーマンス劣化
            db, _ := sql.Open("mysql", "user:pass@tcp(localhost:3306)/store")
            row := db.QueryRow("SELECT stock FROM products WHERE id = ?", item.ProductID)
            
            var stock int
            row.Scan(&stock)
            
            if stock < item.Quantity {
                // ❌ エラー処理が不十分
                http.Error(w, "Out of stock", http.StatusBadRequest)
                return
            }
            db.Close() // ❌ 毎回接続・切断でリソース浪費
        }
        
        // 2. 【決済処理】外部API呼び出し（タイムアウト・リトライなし）
        paymentResponse, err := http.Post("https://payment-api.com/charge", 
            "application/json", bytes.NewReader(buildPaymentRequest(r)))
        if err != nil {
            // ❌ 決済失敗時の対応不備
            log.Printf("Payment failed: %v", err)
            http.Error(w, "Payment error", http.StatusInternalServerError)
            return
        }
        // ❌ レスポンスの適切な処理なし（ステータスコード確認不足）
        
        // 3. 【在庫減算】レースコンディション発生
        // ❌ 並行処理での在庫管理が不適切
        updateStock(parseCartItems(r)) // 複数ユーザーが同時実行で在庫がマイナスに
        
        // 4. 【注文確定】トランザクション管理なし
        createOrder(r) // ❌ 失敗時のロールバック不可能
        
        // 【実際の災害シナリオ】：
        // ブラックフライデー：同時接続数10,000
        // → データベース接続プール枯渇
        // → 在庫の二重減算で商品がマイナス在庫
        // → 決済は成功したが注文作成失敗で売上計上されず
        // → 顧客クレーム殺到、売上機会損失数億円
        // → システム復旧に3日間、信頼失墜
        
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Order completed")) // ❌ 実際には多数の問題が発生済み
    })
    
    log.Println("Starting legacy e-commerce system (DISASTER WAITING TO HAPPEN)")
    http.ListenAndServe(":8080", nil)
}

// ✅ 正解：Go道場60日間技術総結集エンタープライズE-commerceプラットフォーム
type EnterpriseEcommercePlatform struct {
    // 【Day 1-15: 基盤並行処理技術】
    contextManager      *AdvancedContextManager    // Context制御・キャンセル伝播
    mutexController     *OptimizedMutexController  // RWMutex最適化
    onceManager         *SafeOnceManager          // 一度限り初期化
    poolManager         *SyncPoolManager          // オブジェクト再利用
    workerPools         *WorkerPoolOrchestrator   // ワーカープール管理
    
    // 【Day 16-30: HTTP・ミドルウェア技術】
    httpServer          *ProductionHTTPServer     // 本格HTTPサーバー
    timeoutManager      *TimeoutManager           // タイムアウト制御
    gracefulShutdown    *GracefulShutdownManager  // グレースフルシャットダウン
    middlewareChain     *MiddlewareOrchestrator   // ミドルウェア統合
    authSystem          *JWTAuthenticationSystem  // JWT認証システム
    rateLimiter         *AdaptiveRateLimiter      // 適応的レート制限
    
    // 【Day 31-45: データベース・キャッシュ技術】
    dbManager           *AdvancedDatabaseManager  // 高度DB管理
    transactionManager  *DistributedTxManager     // 分散トランザクション
    connectionPool      *OptimizedConnectionPool  // 接続プール最適化
    cacheSystem         *MultiTierCacheSystem     // 多層キャッシュシステム
    dataLoader          *BatchDataLoader          // N+1問題解決
    
    // 【Day 46-52: gRPC・分散通信技術】
    grpcServer          *ProductionGRPCServer     // プロダクショngRPCサーバー
    grpcClient          *LoadBalancedGRPCClient   // 負荷分散gRPCクライアント
    streamManager       *BidirectionalStreamManager // 双方向ストリーミング
    interceptorChain    *gRPCInterceptorChain     // gRPCインターセプター
    
    // 【Day 53-59: メッセージング・監視技術】
    messageSystem       *EnterpriseMessageSystem  // エンタープライズメッセージング
    dlqManager          *DeadLetterQueueManager   // デッドレターキュー
    orderingManager     *MessageOrderingManager   // メッセージ順序保証
    idempotencyManager  *IdempotencyManager       // 冪等性制御
    metricsSystem       *PrometheusMetricsSystem  // Prometheus監視
    
    // 【統合システム管理】
    serviceRegistry     *ServiceRegistry          // サービス発見・登録
    configManager       *DynamicConfigManager     // 動的設定管理
    healthChecker       *ComprehensiveHealthCheck // 包括的ヘルスチェック
    circuitBreaker      *AdaptiveCircuitBreaker   // 適応的サーキットブレーカー
    traceManager        *DistributedTracing       // 分散トレーシング
    logManager          *StructuredLogging        // 構造化ログ
}

// 【統合E-commerceビジネスロジック】全技術要素の有機的結合
func (platform *EnterpriseEcommercePlatform) ProcessOrderWithFullIntegration(ctx context.Context, req *OrderRequest) (*OrderResponse, error) {
    // 【STEP 1: Context・タイムアウト制御】（Day 1-2技術）
    orderCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    traceID := platform.traceManager.StartTrace(orderCtx, "order_processing")
    defer platform.traceManager.FinishTrace(traceID)
    
    // 【STEP 2: 認証・レート制限】（Day 21, 23技術）
    userContext, err := platform.authSystem.ValidateAndExtractUser(orderCtx, req.AuthToken)
    if err != nil {
        platform.metricsSystem.RecordAuthFailure("order_processing", err)
        return nil, status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
    }
    
    if !platform.rateLimiter.Allow(userContext.UserID, "order") {
        platform.metricsSystem.RecordRateLimitExceeded("order_processing", userContext.UserID)
        return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
    }
    
    // 【STEP 3: 在庫確認（DataLoader技術）】（Day 36技術）
    productIDs := extractProductIDs(req.Items)
    products, err := platform.dataLoader.LoadProducts(orderCtx, productIDs)
    if err != nil {
        platform.metricsSystem.RecordDataLoadFailure("products", len(productIDs), err)
        return nil, status.Errorf(codes.Internal, "product loading failed: %v", err)
    }
    
    // 【在庫検証（並行処理）】（Day 6技術）
    stockValidation := platform.workerPools.SubmitStockValidation(orderCtx, req.Items, products)
    
    // 【STEP 4: 並行処理による効率化】（Day 3-6技術）
    var wg sync.WaitGroup
    errChan := make(chan error, 3)
    
    // 【並行タスク1: 配送料計算】
    wg.Add(1)
    go func() {
        defer wg.Done()
        if _, err := platform.calculateShipping(orderCtx, req); err != nil {
            errChan <- fmt.Errorf("shipping calculation failed: %w", err)
        }
    }()
    
    // 【並行タスク2: 税金計算】
    wg.Add(1) 
    go func() {
        defer wg.Done()
        if _, err := platform.calculateTax(orderCtx, req); err != nil {
            errChan <- fmt.Errorf("tax calculation failed: %w", err)
        }
    }()
    
    // 【並行タスク3: 割引適用】
    wg.Add(1)
    go func() {
        defer wg.Done()
        if _, err := platform.applyDiscounts(orderCtx, req, userContext); err != nil {
            errChan <- fmt.Errorf("discount application failed: %w", err)
        }
    }()
    
    // 【並行処理完了待機】
    go func() {
        wg.Wait()
        close(errChan)
    }()
    
    // 【エラーチェック】
    for err := range errChan {
        if err != nil {
            platform.metricsSystem.RecordOrderProcessingError("parallel_calculation", err)
            return nil, status.Errorf(codes.Internal, "order calculation failed: %v", err)
        }
    }
    
    // 【STEP 5: 分散トランザクション開始】（Day 31技術）
    txManager := platform.transactionManager.Begin(orderCtx)
    defer func() {
        if r := recover(); r != nil {
            txManager.Rollback()
            panic(r)
        }
    }()
    
    // 【STEP 6: 決済処理（gRPC + サーキットブレーカー）】（Day 13, 46技術）
    paymentResp, err := platform.processPaymentWithCircuitBreaker(orderCtx, req, txManager)
    if err != nil {
        txManager.Rollback()
        platform.metricsSystem.RecordPaymentFailure(req.PaymentMethod, err)
        return nil, status.Errorf(codes.Internal, "payment processing failed: %v", err)
    }
    
    // 【STEP 7: 在庫更新（キャッシュ統合）】（Day 42-45技術）
    if err := platform.updateInventoryWithCache(orderCtx, req.Items, txManager); err != nil {
        txManager.Rollback()
        platform.metricsSystem.RecordInventoryUpdateFailure(len(req.Items), err)
        return nil, status.Errorf(codes.Internal, "inventory update failed: %v", err)
    }
    
    // 【STEP 8: 注文作成（Repository + UnitOfWork）】（Day 34技術）
    order, err := platform.createOrderWithUnitOfWork(orderCtx, req, paymentResp, txManager)
    if err != nil {
        txManager.Rollback()
        platform.metricsSystem.RecordOrderCreationFailure(userContext.UserID, err)
        return nil, status.Errorf(codes.Internal, "order creation failed: %v", err)
    }
    
    // 【STEP 9: トランザクションコミット】
    if err := txManager.Commit(); err != nil {
        platform.metricsSystem.RecordTransactionCommitFailure("order_processing", err)
        return nil, status.Errorf(codes.Internal, "transaction commit failed: %v", err)
    }
    
    // 【STEP 10: 非同期処理（メッセージング）】（Day 53-56技術）
    if err := platform.publishOrderEvents(orderCtx, order); err != nil {
        // 【注意】メッセージング失敗は注文処理失敗ではない
        platform.metricsSystem.RecordMessagePublishFailure("order_events", err)
        log.Printf("Warning: Order event publishing failed: %v", err)
    }
    
    // 【STEP 11: キャッシュ更新】（Day 43技術 - Write-Through）
    platform.updateCacheWithWriteThrough(orderCtx, order)
    
    // 【STEP 12: メトリクス記録】（Day 57-58技術）
    platform.recordOrderProcessingMetrics(order, time.Since(time.Now()))
    
    // 【STEP 13: 正常応答】
    response := &OrderResponse{
        OrderID:       order.ID,
        Status:        order.Status,
        TotalAmount:   order.TotalAmount,
        PaymentID:     paymentResp.PaymentID,
        EstimatedDelivery: calculateDeliveryDate(order.ShippingMethod),
    }
    
    platform.metricsSystem.RecordOrderSuccess(userContext.UserID, order.TotalAmount)
    platform.logManager.LogOrderCompletion(orderCtx, order, userContext)
    
    return response, nil
}

// 【決済処理（gRPC + サーキットブレーカー統合）】
func (platform *EnterpriseEcommercePlatform) processPaymentWithCircuitBreaker(ctx context.Context, req *OrderRequest, txManager *DistributedTxManager) (*PaymentResponse, error) {
    // 【サーキットブレーカーによる外部API保護】
    return platform.circuitBreaker.Execute(ctx, "payment_service", func() (*PaymentResponse, error) {
        // 【gRPCクライアント使用】負荷分散・リトライ機能付き
        client, err := platform.grpcClient.GetPaymentServiceClient(ctx)
        if err != nil {
            return nil, fmt.Errorf("payment service client unavailable: %w", err)
        }
        
        paymentReq := &payment.ProcessPaymentRequest{
            Amount:        req.TotalAmount,
            Currency:      req.Currency,
            PaymentMethod: req.PaymentMethod,
            CustomerID:    req.CustomerID,
            TransactionID: txManager.GetTransactionID(),
        }
        
        // 【gRPCタイムアウト付き呼び出し】
        paymentCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
        defer cancel()
        
        resp, err := client.ProcessPayment(paymentCtx, paymentReq)
        if err != nil {
            // 【gRPCエラー分類】
            if status.Code(err) == codes.DeadlineExceeded {
                platform.metricsSystem.RecordPaymentTimeout()
                return nil, &RetryableError{Err: err}
            }
            return nil, fmt.Errorf("payment processing failed: %w", err)
        }
        
        return &PaymentResponse{
            PaymentID:     resp.PaymentId,
            Status:        resp.Status,
            TransactionID: resp.TransactionId,
        }, nil
    })
}

// 【包括的メトリクス記録】（Day 57-58統合技術）
func (platform *EnterpriseEcommercePlatform) recordOrderProcessingMetrics(order *Order, duration time.Duration) {
    labels := prometheus.Labels{
        "user_segment":    order.UserSegment,
        "order_source":    order.Source,
        "payment_method":  order.PaymentMethod,
        "shipping_method": order.ShippingMethod,
        "country":         order.ShippingAddress.Country,
    }
    
    // 【基本メトリクス】
    platform.metricsSystem.OrdersTotal.With(labels).Inc()
    platform.metricsSystem.OrderValue.With(labels).Add(order.TotalAmount)
    platform.metricsSystem.OrderProcessingTime.With(labels).Observe(duration.Seconds())
    
    // 【ビジネスメトリクス】
    platform.metricsSystem.RevenueTotal.Add(order.TotalAmount)
    platform.metricsSystem.ItemsSold.Add(float64(order.ItemCount))
    
    // 【パフォーマンスメトリクス】
    if duration > 5*time.Second {
        platform.metricsSystem.SlowOrdersTotal.With(labels).Inc()
    }
    
    // 【Histogram記録】P50, P95, P99分析用
    platform.metricsSystem.OrderProcessingHistogram.With(labels).Observe(duration.Seconds())
}
```

#### 技術スタックの統合

**基盤技術（Days 1-15）**
- Context Based Cancellation: リクエストライフサイクル管理
- Advanced Goroutine Patterns: 並行処理最適化
- Worker Pool Pattern: 負荷分散処理
- Pipeline Pattern: データストリーム処理
- Thread-Safe Cache: 高速データアクセス

**Web API技術（Days 16-30）**
- HTTP Server with Timeouts: 高可用性Web API
- Middleware Chain: 横断的関心事の分離
- Authentication Middleware: セキュアなアクセス制御
- API Rate Limiting: サービス保護
- Request Validation: データ整合性確保

**データベース技術（Days 31-45）**
- Connection Pooling: 効率的なリソース管理
- Transaction Management: データ一貫性保証
- Repository Pattern: データアクセス抽象化
- Distributed Cache: スケーラブルなキャッシュ戦略
- Thundering Herd Prevention: 高負荷対策

**分散システム技術（Days 46-60）**
- gRPC Communication: 高性能サービス間通信
- Message Queue Patterns: 非同期処理
- Circuit Breaker Pattern: 障害回避機能
- Prometheus Metrics: 運用監視
- OpenTelemetry Tracing: 分散トレーシング

### アーキテクチャ設計

#### 1. 全体システム構成

```
                    ┌─────────────────────────┐
                    │      Load Balancer      │
                    │      (nginx/HAProxy)    │
                    └─────────────────────────┘
                                │
                    ┌─────────────────────────┐
                    │      API Gateway        │
                    │   (Authentication,      │
                    │    Rate Limiting,       │
                    │     Routing)            │
                    └─────────────────────────┘
                                │
            ┌───────────────────┼───────────────────┐
            │                   │                   │
    ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
    │ User Service  │  │Product Service│  │ Order Service │
    │   (gRPC)      │  │    (gRPC)     │  │    (gRPC)     │
    └───────────────┘  └───────────────┘  └───────────────┘
            │                   │                   │
            └───────────────────┼───────────────────┘
                                │
                    ┌─────────────────────────┐
                    │     Message Queue       │
                    │    (Event Streaming)    │
                    └─────────────────────────┘
                                │
                    ┌─────────────────────────┐
                    │   Notification Service  │
                    │      (Async Tasks)      │
                    └─────────────────────────┘

        ┌─────────────────┐         ┌─────────────────┐
        │   PostgreSQL    │         │      Redis      │
        │   (Primary DB)  │         │   (Cache/Lock)  │
        └─────────────────┘         └─────────────────┘

        ┌─────────────────┐         ┌─────────────────┐
        │   Prometheus    │         │     Jaeger      │
        │   (Metrics)     │         │   (Tracing)     │
        └─────────────────┘         └─────────────────┘
```

#### 2. マイクロサービス構成

**User Service**
```go
// 【プロダクション品質のUserService】60日間の学習集大成
type UserService struct {
    // 【依存関係注入】Clean Architecture準拠
    repo         UserRepository
    cache        CacheService
    logger       *slog.Logger
    tracer       trace.Tracer
    metrics      *UserMetrics
    eventBus     EventBus
    
    // 【品質保証機能】信頼性とスケーラビリティ
    circuitBreaker *CircuitBreaker
    rateLimiter    *RateLimiter
    validator      *Validator
    
    // 【高度な機能】プロダクション運用対応
    connectionPool *sql.DB
    healthChecker  *HealthChecker
    configManager  *ConfigManager
    securityManager *SecurityManager
}

// 【重要メソッド】ユーザー作成処理の完全実装
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
    // 【STEP 1】分散トレーシング開始
    ctx, span := s.tracer.Start(ctx, "UserService.CreateUser",
        trace.WithSpanKind(trace.SpanKindServer),
        trace.WithAttributes(
            attribute.String("operation", "create_user"),
            attribute.String("user.email", req.Email),
        ),
    )
    defer span.End()
    
    start := time.Now()
    requestID := GetRequestID(ctx)
    
    // 【STEP 2】リクエストメトリクス記録
    s.metrics.RequestsTotal.WithLabelValues("create_user").Inc()
    
    // 【STEP 3】レート制限チェック
    if !s.rateLimiter.Allow() {
        s.metrics.RateLimitExceeded.Inc()
        span.SetStatus(codes.Error, "rate limit exceeded")
        
        s.logger.WarnContext(ctx, "Rate limit exceeded for user creation",
            slog.String("request_id", requestID),
            slog.String("user_email", req.Email),
        )
        
        return nil, NewRateLimitError("User creation rate limit exceeded")
    }
    
    // 【STEP 4】包括的バリデーション
    if err := s.validator.ValidateCreateUserRequest(req); err != nil {
        s.metrics.ValidationErrors.Inc()
        span.RecordError(err)
        span.SetStatus(codes.Error, "validation failed")
        
        s.logger.WarnContext(ctx, "User creation validation failed",
            slog.String("request_id", requestID),
            slog.String("error", err.Error()),
        )
        
        return nil, err
    }
    
    // 【STEP 5】重複チェック（キャッシュ活用）
    exists, err := s.checkUserExists(ctx, req.Email)
    if err != nil {
        s.metrics.DatabaseErrors.Inc()
        span.RecordError(err)
        return nil, fmt.Errorf("failed to check user existence: %w", err)
    }
    
    if exists {
        s.metrics.DuplicateUsers.Inc()
        span.SetStatus(codes.Error, "user already exists")
        
        s.logger.WarnContext(ctx, "Attempted to create duplicate user",
            slog.String("request_id", requestID),
            slog.String("user_email", req.Email),
        )
        
        return nil, NewDuplicateUserError("User with this email already exists")
    }
    
    // 【STEP 6】パスワードハッシュ化（セキュリティ）
    hashedPassword, err := s.securityManager.HashPassword(req.Password)
    if err != nil {
        s.metrics.SecurityErrors.Inc()
        span.RecordError(err)
        return nil, fmt.Errorf("failed to hash password: %w", err)
    }
    
    // 【STEP 7】ユーザーエンティティ作成
    user := &User{
        ID:          generateUserID(),
        Name:        req.Name,
        Email:       req.Email,
        Password:    hashedPassword,
        Status:      UserStatusActive,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }
    
    // 【STEP 8】データベーストランザクション
    err = s.executeInTransaction(ctx, func(tx *sql.Tx) error {
        // データベースに保存
        if err := s.repo.CreateWithTx(ctx, tx, user); err != nil {
            return fmt.Errorf("failed to create user in database: %w", err)
        }
        
        // 初期プロファイル作成
        profile := &UserProfile{
            UserID:    user.ID,
            CreatedAt: time.Now(),
        }
        
        if err := s.repo.CreateProfileWithTx(ctx, tx, profile); err != nil {
            return fmt.Errorf("failed to create user profile: %w", err)
        }
        
        return nil
    })
    
    if err != nil {
        s.metrics.DatabaseErrors.Inc()
        span.RecordError(err)
        span.SetStatus(codes.Error, "database transaction failed")
        
        s.logger.ErrorContext(ctx, "User creation transaction failed",
            slog.String("request_id", requestID),
            slog.String("user_email", req.Email),
            slog.String("error", err.Error()),
        )
        
        return nil, err
    }
    
    // 【STEP 9】キャッシュ更新
    if err := s.cache.Set(ctx, s.buildUserCacheKey(user.ID), user, time.Hour); err != nil {
        s.metrics.CacheErrors.Inc()
        // キャッシュエラーはサービス継続性のために警告のみ
        s.logger.WarnContext(ctx, "Failed to cache user",
            slog.String("request_id", requestID),
            slog.String("user_id", user.ID),
            slog.String("error", err.Error()),
        )
    }
    
    // 【STEP 10】イベント発行（非同期処理）
    event := &UserCreatedEvent{
        UserID:    user.ID,
        Email:     user.Email,
        Name:      user.Name,
        CreatedAt: user.CreatedAt,
        Metadata: EventMetadata{
            RequestID: requestID,
            TraceID:   span.SpanContext().TraceID().String(),
            SpanID:    span.SpanContext().SpanID().String(),
        },
    }
    
    if err := s.eventBus.PublishAsync(ctx, event); err != nil {
        s.metrics.EventPublishingErrors.Inc()
        // イベント発行エラーもサービス継続性のために警告のみ
        s.logger.WarnContext(ctx, "Failed to publish user created event",
            slog.String("request_id", requestID),
            slog.String("user_id", user.ID),
            slog.String("error", err.Error()),
        )
    }
    
    // 【STEP 11】成功メトリクス記録
    duration := time.Since(start)
    s.metrics.SuccessfulOperations.WithLabelValues("create_user").Inc()
    s.metrics.OperationDuration.WithLabelValues("create_user").Observe(duration.Seconds())
    
    span.SetStatus(codes.Ok, "user created successfully")
    span.SetAttributes(
        attribute.String("user.id", user.ID),
        attribute.Duration("operation.duration", duration),
    )
    
    s.logger.InfoContext(ctx, "User created successfully",
        slog.String("request_id", requestID),
        slog.String("user_id", user.ID),
        slog.String("user_email", user.Email),
        slog.Duration("duration", duration),
    )
    
    return user, nil
}

// 【重要メソッド】ユーザー存在チェック（キャッシュ活用）
func (s *UserService) checkUserExists(ctx context.Context, email string) (bool, error) {
    // キャッシュから確認
    cacheKey := s.buildUserEmailCacheKey(email)
    var exists bool
    
    if err := s.cache.Get(ctx, cacheKey, &exists); err == nil {
        s.metrics.CacheHits.Inc()
        return exists, nil
    }
    
    // キャッシュミス：データベースから確認
    s.metrics.CacheMisses.Inc()
    
    user, err := s.repo.GetByEmail(ctx, email)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            // 存在しない場合もキャッシュに保存
            s.cache.Set(ctx, cacheKey, false, 10*time.Minute)
            return false, nil
        }
        return false, err
    }
    
    // 存在する場合もキャッシュに保存
    s.cache.Set(ctx, cacheKey, true, 10*time.Minute)
    return user != nil, nil
}

// 【重要メソッド】トランザクション実行（データ一貫性保証）
func (s *UserService) executeInTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
    tx, err := s.connectionPool.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()
    
    if err := fn(tx); err != nil {
        return err
    }
    
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    
    return nil
}

// 【重要メソッド】キャッシュキー生成（統一規則）
func (s *UserService) buildUserCacheKey(userID string) string {
    return fmt.Sprintf("user:id:%s", userID)
}

func (s *UserService) buildUserEmailCacheKey(email string) string {
    return fmt.Sprintf("user:email:%s", email)
}
```

type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id string) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter *UserFilter) ([]*User, error)
}

type User struct {
    ID          string    `json:"id" db:"id"`
    Email       string    `json:"email" db:"email"`
    Name        string    `json:"name" db:"name"`
    Password    string    `json:"-" db:"password_hash"`
    Status      string    `json:"status" db:"status"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
    LastLoginAt *time.Time `json:"last_login_at,omitempty" db:"last_login_at"`
}
```

**Order Service**
```go
type OrderService struct {
    repo            OrderRepository
    userService     UserServiceClient
    productService  ProductServiceClient
    paymentService  PaymentServiceClient
    cache           CacheService
    eventBus        EventBus
    logger          *slog.Logger
    tracer          trace.Tracer
    metrics         *OrderMetrics
    
    // ビジネスロジック
    pricingEngine   PricingEngine
    inventoryManager InventoryManager
    fulfillmentService FulfillmentService
    
    // 信頼性機能
    circuitBreaker  *CircuitBreaker
    retryPolicy     *RetryPolicy
    timeoutConfig   TimeoutConfig
}

type Order struct {
    ID              string      `json:"id" db:"id"`
    UserID          string      `json:"user_id" db:"user_id"`
    Status          OrderStatus `json:"status" db:"status"`
    Items           []OrderItem `json:"items"`
    TotalAmount     decimal.Decimal `json:"total_amount" db:"total_amount"`
    Currency        string      `json:"currency" db:"currency"`
    PaymentStatus   string      `json:"payment_status" db:"payment_status"`
    ShippingAddress Address     `json:"shipping_address"`
    CreatedAt       time.Time   `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time   `json:"updated_at" db:"updated_at"`
}

type OrderItem struct {
    ProductID string          `json:"product_id"`
    Quantity  int             `json:"quantity"`
    UnitPrice decimal.Decimal `json:"unit_price"`
    Total     decimal.Decimal `json:"total"`
}
```

#### 3. 可観測性アーキテクチャ

**統合ログシステム**
```go
type ObservabilityStack struct {
    logger *slog.Logger
    tracer trace.Tracer
    meter  metric.Meter
    
    // ログ集約
    logProcessor LogProcessor
    logForwarder LogForwarder
    
    // メトリクス収集
    metricsCollector MetricsCollector
    metricsExporter  MetricsExporter
    
    // トレース管理
    traceCollector TraceCollector
    traceExporter  TraceExporter
    
    // アラート
    alertManager AlertManager
    ruleEngine   RuleEngine
}

func NewObservabilityStack(config *Config) *ObservabilityStack {
    // 構造化ログ設定
    logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
        AddSource: true,
    })
    logger := slog.New(logHandler)
    
    // OpenTelemetry設定
    tp := initTraceProvider(config.ServiceName)
    tracer := tp.Tracer(config.ServiceName)
    
    // Prometheus設定
    registry := prometheus.NewRegistry()
    meter := initMeterProvider(registry)
    
    return &ObservabilityStack{
        logger: logger,
        tracer: tracer,
        meter:  meter,
        logProcessor: NewLogProcessor(config.LogConfig),
        metricsCollector: NewMetricsCollector(registry),
        traceCollector: NewTraceCollector(tp),
        alertManager: NewAlertManager(config.AlertConfig),
    }
}
```

**カスタムメトリクス収集**
```go
type BusinessMetrics struct {
    // ユーザーメトリクス
    UserRegistrations   *prometheus.CounterVec
    ActiveUsers         *prometheus.GaugeVec
    UserSessions        *prometheus.HistogramVec
    
    // 注文メトリクス
    OrdersTotal         *prometheus.CounterVec
    OrderValue          *prometheus.HistogramVec
    OrderProcessingTime *prometheus.HistogramVec
    OrderErrors         *prometheus.CounterVec
    
    // システムメトリクス
    DatabaseQueries     *prometheus.HistogramVec
    CacheHitRate        *prometheus.GaugeVec
    ExternalAPILatency  *prometheus.HistogramVec
    CircuitBreakerState *prometheus.GaugeVec
    
    // SLI メトリクス
    ServiceAvailability *prometheus.GaugeVec
    ErrorBudget         *prometheus.GaugeVec
    SLOCompliance       *prometheus.GaugeVec
}

func (m *BusinessMetrics) RecordOrderCreated(ctx context.Context, order *Order) {
    span := trace.SpanFromContext(ctx)
    
    // メトリクス記録
    m.OrdersTotal.WithLabelValues(
        order.Status.String(),
        order.Currency,
    ).Inc()
    
    m.OrderValue.WithLabelValues(
        order.Currency,
    ).Observe(float64(order.TotalAmount.InexactFloat64()))
    
    // トレースに情報追加
    span.SetAttributes(
        attribute.String("order.id", order.ID),
        attribute.String("order.status", order.Status.String()),
        attribute.Float64("order.amount", order.TotalAmount.InexactFloat64()),
    )
    
    // 構造化ログ
    slog.InfoContext(ctx, "Order created",
        slog.String("order_id", order.ID),
        slog.String("user_id", order.UserID),
        slog.Float64("amount", order.TotalAmount.InexactFloat64()),
        slog.String("currency", order.Currency),
    )
}
```

#### 4. 分散システムパターンの統合

**サーキットブレーカー統合**
```go
type ServiceClient struct {
    name           string
    baseURL        string
    client         *http.Client
    circuitBreaker *CircuitBreaker
    tracer         trace.Tracer
    metrics        ClientMetrics
    retryPolicy    RetryPolicy
}

func (c *ServiceClient) CallWithProtection(ctx context.Context, req *APIRequest) (*APIResponse, error) {
    operation := fmt.Sprintf("%s_%s", c.name, req.Method)
    
    ctx, span := c.tracer.Start(ctx, operation,
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            attribute.String("service.name", c.name),
            attribute.String("http.method", req.Method),
            attribute.String("http.url", req.URL),
        ),
    )
    defer span.End()
    
    // サーキットブレーカーを通してリクエスト実行
    response, err := c.circuitBreaker.Execute(func() (interface{}, error) {
        return c.executeWithRetry(ctx, req)
    })
    
    if err != nil {
        c.metrics.ErrorCount.WithLabelValues(
            c.name, req.Method, "circuit_breaker",
        ).Inc()
        
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        
        slog.ErrorContext(ctx, "Service call failed",
            slog.String("service", c.name),
            slog.String("error", err.Error()),
        )
        
        return nil, err
    }
    
    return response.(*APIResponse), nil
}

func (c *ServiceClient) executeWithRetry(ctx context.Context, req *APIRequest) (*APIResponse, error) {
    var lastErr error
    
    for attempt := 0; attempt < c.retryPolicy.MaxAttempts; attempt++ {
        start := time.Now()
        
        resp, err := c.executeRequest(ctx, req)
        duration := time.Since(start)
        
        // メトリクス記録
        c.metrics.RequestDuration.WithLabelValues(
            c.name, req.Method,
        ).Observe(duration.Seconds())
        
        if err == nil {
            c.metrics.RequestCount.WithLabelValues(
                c.name, req.Method, "success",
            ).Inc()
            return resp, nil
        }
        
        lastErr = err
        
        // リトライ可能なエラーかチェック
        if !c.isRetryableError(err) {
            break
        }
        
        // 最後の試行でなければ待機
        if attempt < c.retryPolicy.MaxAttempts-1 {
            backoff := c.calculateBackoff(attempt)
            
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(backoff):
                continue
            }
        }
    }
    
    c.metrics.RequestCount.WithLabelValues(
        c.name, req.Method, "error",
    ).Inc()
    
    return nil, fmt.Errorf("max retry attempts reached: %w", lastErr)
}
```

**イベント駆動アーキテクチャ**
```go
type EventBus struct {
    broker    MessageBroker
    handlers  map[EventType][]EventHandler
    publisher EventPublisher
    tracer    trace.Tracer
    logger    *slog.Logger
    
    // イベント処理の信頼性
    retryPolicy      RetryPolicy
    deadLetterQueue  DeadLetterQueue
    duplicateFilter  DuplicateFilter
}

type Event struct {
    ID          string                 `json:"id"`
    Type        EventType             `json:"type"`
    AggregateID string                `json:"aggregate_id"`
    Data        map[string]interface{} `json:"data"`
    Metadata    EventMetadata         `json:"metadata"`
    Timestamp   time.Time             `json:"timestamp"`
}

type EventMetadata struct {
    CorrelationID string            `json:"correlation_id"`
    CausationID   string            `json:"causation_id"`
    UserID        string            `json:"user_id,omitempty"`
    TraceID       string            `json:"trace_id"`
    SpanID        string            `json:"span_id"`
    Source        string            `json:"source"`
    Version       string            `json:"version"`
    Headers       map[string]string `json:"headers"`
}

func (eb *EventBus) Publish(ctx context.Context, event *Event) error {
    ctx, span := eb.tracer.Start(ctx, "EventBus.Publish",
        trace.WithAttributes(
            attribute.String("event.type", string(event.Type)),
            attribute.String("event.aggregate_id", event.AggregateID),
        ),
    )
    defer span.End()
    
    // トレース情報をイベントメタデータに追加
    spanContext := span.SpanContext()
    event.Metadata.TraceID = spanContext.TraceID().String()
    event.Metadata.SpanID = spanContext.SpanID().String()
    
    // イベントの検証
    if err := eb.validateEvent(event); err != nil {
        span.RecordError(err)
        return err
    }
    
    // 重複チェック
    if eb.duplicateFilter.IsDuplicate(event.ID) {
        slog.WarnContext(ctx, "Duplicate event detected",
            slog.String("event_id", event.ID),
            slog.String("event_type", string(event.Type)),
        )
        return nil
    }
    
    // イベント公開
    err := eb.publisher.Publish(ctx, event)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        
        slog.ErrorContext(ctx, "Failed to publish event",
            slog.String("event_id", event.ID),
            slog.String("event_type", string(event.Type)),
            slog.String("error", err.Error()),
        )
        
        return err
    }
    
    slog.InfoContext(ctx, "Event published",
        slog.String("event_id", event.ID),
        slog.String("event_type", string(event.Type)),
        slog.String("aggregate_id", event.AggregateID),
    )
    
    return nil
}

func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
    if eb.handlers[eventType] == nil {
        eb.handlers[eventType] = make([]EventHandler, 0)
    }
    eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

func (eb *EventBus) HandleEvent(ctx context.Context, event *Event) error {
    ctx, span := eb.tracer.Start(ctx, "EventBus.HandleEvent",
        trace.WithAttributes(
            attribute.String("event.type", string(event.Type)),
            attribute.String("event.id", event.ID),
        ),
    )
    defer span.End()
    
    handlers, exists := eb.handlers[event.Type]
    if !exists {
        slog.WarnContext(ctx, "No handlers for event type",
            slog.String("event_type", string(event.Type)),
        )
        return nil
    }
    
    // 複数のハンドラーを並列実行
    var wg sync.WaitGroup
    errChan := make(chan error, len(handlers))
    
    for _, handler := range handlers {
        wg.Add(1)
        go func(h EventHandler) {
            defer wg.Done()
            
            if err := eb.executeHandlerWithRetry(ctx, h, event); err != nil {
                errChan <- err
            }
        }(handler)
    }
    
    wg.Wait()
    close(errChan)
    
    // エラー収集
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("handler errors: %v", errors)
    }
    
    return nil
}
```

### プロダクション対応機能

#### 1. 設定管理

```go
type Config struct {
    Server    ServerConfig    `mapstructure:"server"`
    Database  DatabaseConfig  `mapstructure:"database"`
    Redis     RedisConfig     `mapstructure:"redis"`
    Services  ServicesConfig  `mapstructure:"services"`
    Security  SecurityConfig  `mapstructure:"security"`
    Observability ObservabilityConfig `mapstructure:"observability"`
    Features  FeatureFlags    `mapstructure:"features"`
}

type ServerConfig struct {
    Port         int           `mapstructure:"port" default:"8080"`
    ReadTimeout  time.Duration `mapstructure:"read_timeout" default:"30s"`
    WriteTimeout time.Duration `mapstructure:"write_timeout" default:"30s"`
    IdleTimeout  time.Duration `mapstructure:"idle_timeout" default:"120s"`
    GracefulTimeout time.Duration `mapstructure:"graceful_timeout" default:"30s"`
}

type DatabaseConfig struct {
    Host         string `mapstructure:"host" default:"localhost"`
    Port         int    `mapstructure:"port" default:"5432"`
    User         string `mapstructure:"user"`
    Password     string `mapstructure:"password"`
    Database     string `mapstructure:"database"`
    SSLMode      string `mapstructure:"ssl_mode" default:"prefer"`
    MaxOpenConns int    `mapstructure:"max_open_conns" default:"25"`
    MaxIdleConns int    `mapstructure:"max_idle_conns" default:"5"`
    ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime" default:"1h"`
}

func LoadConfig() (*Config, error) {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    viper.AddConfigPath("./configs")
    viper.AddConfigPath("/etc/myapp")
    
    // 環境変数の自動マッピング
    viper.AutomaticEnv()
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
    
    // デフォルト値設定
    setDefaults()
    
    if err := viper.ReadInConfig(); err != nil {
        return nil, fmt.Errorf("failed to read config: %w", err)
    }
    
    var config Config
    if err := viper.Unmarshal(&config); err != nil {
        return nil, fmt.Errorf("failed to unmarshal config: %w", err)
    }
    
    // 設定値検証
    if err := validateConfig(&config); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }
    
    return &config, nil
}
```

#### 2. ヘルスチェック

```go
type HealthChecker struct {
    checks map[string]HealthCheck
    logger *slog.Logger
    tracer trace.Tracer
}

type HealthCheck interface {
    Name() string
    Check(ctx context.Context) HealthStatus
}

type HealthStatus struct {
    Status    string                 `json:"status"`
    Message   string                 `json:"message,omitempty"`
    Details   map[string]interface{} `json:"details,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
    Duration  time.Duration          `json:"duration"`
}

type DatabaseHealthCheck struct {
    db     *sql.DB
    tracer trace.Tracer
}

func (hc *DatabaseHealthCheck) Name() string {
    return "database"
}

func (hc *DatabaseHealthCheck) Check(ctx context.Context) HealthStatus {
    ctx, span := hc.tracer.Start(ctx, "HealthCheck.Database")
    defer span.End()
    
    start := time.Now()
    
    // 簡単なクエリで接続確認
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    var result int
    err := hc.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
    
    duration := time.Since(start)
    
    if err != nil {
        span.RecordError(err)
        return HealthStatus{
            Status:    "unhealthy",
            Message:   err.Error(),
            Timestamp: time.Now(),
            Duration:  duration,
        }
    }
    
    return HealthStatus{
        Status:    "healthy",
        Timestamp: time.Now(),
        Duration:  duration,
        Details: map[string]interface{}{
            "query_result": result,
        },
    }
}

func (hc *HealthChecker) CheckAll(ctx context.Context) map[string]HealthStatus {
    results := make(map[string]HealthStatus)
    
    var wg sync.WaitGroup
    var mu sync.Mutex
    
    for name, check := range hc.checks {
        wg.Add(1)
        go func(name string, check HealthCheck) {
            defer wg.Done()
            
            status := check.Check(ctx)
            
            mu.Lock()
            results[name] = status
            mu.Unlock()
        }(name, check)
    }
    
    wg.Wait()
    return results
}
```

#### 3. グレースフルシャットダウン

```go
type Application struct {
    config    *Config
    server    *http.Server
    db        *sql.DB
    redis     *redis.Client
    eventBus  *EventBus
    logger    *slog.Logger
    tracer    trace.Tracer
    
    // リソース管理
    connectionPool *sql.DB
    workerPool     *WorkerPool
    scheduler      *Scheduler
    
    // シグナル処理
    shutdownChan chan os.Signal
    doneChan     chan struct{}
}

func (app *Application) Run() error {
    // シグナル処理設定
    app.shutdownChan = make(chan os.Signal, 1)
    app.doneChan = make(chan struct{})
    signal.Notify(app.shutdownChan, syscall.SIGINT, syscall.SIGTERM)
    
    // サーバー起動
    go func() {
        app.logger.Info("Starting server",
            slog.Int("port", app.config.Server.Port),
        )
        
        if err := app.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            app.logger.Error("Server failed to start",
                slog.String("error", err.Error()),
            )
        }
    }()
    
    app.logger.Info("Application started successfully")
    
    // シャットダウンシグナル待機
    <-app.shutdownChan
    app.logger.Info("Shutdown signal received")
    
    return app.gracefulShutdown()
}

func (app *Application) gracefulShutdown() error {
    app.logger.Info("Starting graceful shutdown")
    
    ctx, cancel := context.WithTimeout(context.Background(), app.config.Server.GracefulTimeout)
    defer cancel()
    
    var wg sync.WaitGroup
    errors := make(chan error, 5)
    
    // HTTP サーバーのシャットダウン
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := app.server.Shutdown(ctx); err != nil {
            errors <- fmt.Errorf("http server shutdown failed: %w", err)
        } else {
            app.logger.Info("HTTP server shut down successfully")
        }
    }()
    
    // ワーカープールのシャットダウン
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := app.workerPool.Shutdown(ctx); err != nil {
            errors <- fmt.Errorf("worker pool shutdown failed: %w", err)
        } else {
            app.logger.Info("Worker pool shut down successfully")
        }
    }()
    
    // イベントバスのシャットダウン
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := app.eventBus.Shutdown(ctx); err != nil {
            errors <- fmt.Errorf("event bus shutdown failed: %w", err)
        } else {
            app.logger.Info("Event bus shut down successfully")
        }
    }()
    
    // データベース接続のクローズ
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := app.db.Close(); err != nil {
            errors <- fmt.Errorf("database close failed: %w", err)
        } else {
            app.logger.Info("Database connections closed successfully")
        }
    }()
    
    // Redis接続のクローズ
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := app.redis.Close(); err != nil {
            errors <- fmt.Errorf("redis close failed: %w", err)
        } else {
            app.logger.Info("Redis connections closed successfully")
        }
    }()
    
    wg.Wait()
    close(errors)
    
    // エラー収集
    var shutdownErrors []error
    for err := range errors {
        shutdownErrors = append(shutdownErrors, err)
    }
    
    if len(shutdownErrors) > 0 {
        for _, err := range shutdownErrors {
            app.logger.Error("Shutdown error", slog.String("error", err.Error()))
        }
        return fmt.Errorf("shutdown completed with errors: %v", shutdownErrors)
    }
    
    app.logger.Info("Graceful shutdown completed successfully")
    close(app.doneChan)
    return nil
}
```

## 📝 課題 (The Problem)

以下の機能を統合したプロダクションレベルのE-commerceマイクロサービスプラットフォームを実装してください：

### 1. コアビジネスサービス

```go
type EcommerceService struct {
    UserService         *UserService
    ProductService      *ProductService
    OrderService        *OrderService
    PaymentService      *PaymentService
    NotificationService *NotificationService
}
```

### 2. 必要な機能

- **ユーザー管理**: 認証、プロファイル管理、権限制御
- **商品管理**: カタログ、在庫管理、価格設定
- **注文処理**: カート、注文作成、決済連携
- **決済処理**: 複数決済手段、トランザクション管理
- **通知システム**: メール、SMS、プッシュ通知

### 3. 横断的機能

- **API Gateway**: リクエストルーティング、認証、レート制限
- **サービスメッシュ**: サービス間通信、負荷分散、回復力
- **設定管理**: 環境別設定、フィーチャーフラグ
- **セキュリティ**: JWT認証、RBAC、データ暗号化

### 4. 運用機能

- **ヘルスチェック**: サービス状態監視、依存関係チェック
- **グレースフルシャットダウン**: 安全なサービス停止
- **負荷分散**: トラフィック制御、キャパシティ管理
- **災害復旧**: バックアップ、フェイルオーバー

### 5. 可観測性スタック

- **構造化ログ**: JSON形式、コンテキスト情報
- **メトリクス収集**: ビジネス指標、SLI/SLO
- **分散トレーシング**: リクエストフロー、パフォーマンス分析
- **アラート**: 閾値監視、異常検知

## ✅ 期待される成果 (Expected Outcomes)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestEcommerce_UserRegistration
    main_test.go:125: User registration completed successfully
    main_test.go:128: Metrics recorded correctly
    main_test.go:131: Event published to notification service
--- PASS: TestEcommerce_UserRegistration (0.15s)

=== RUN   TestEcommerce_OrderProcessing
    main_test.go:155: Order created successfully
    main_test.go:158: Payment processed
    main_test.go:161: Inventory updated
    main_test.go:164: Notification sent
--- PASS: TestEcommerce_OrderProcessing (0.25s)

=== RUN   TestEcommerce_CircuitBreaker
    main_test.go:185: Circuit breaker activated on service failure
    main_test.go:188: Fallback response returned
--- PASS: TestEcommerce_CircuitBreaker (0.12s)

=== RUN   TestEcommerce_DistributedTracing
    main_test.go:215: Trace spans created across all services
    main_test.go:218: Span relationships maintained correctly
--- PASS: TestEcommerce_DistributedTracing (0.08s)

=== RUN   TestEcommerce_GracefulShutdown
    main_test.go:245: Graceful shutdown completed without data loss
--- PASS: TestEcommerce_GracefulShutdown (2.03s)

PASS
ok      day60-final-integration   2.856s
```

### API動作確認

```bash
# サービス起動
go run main.go

# ユーザー登録
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Doe",
    "email": "john.doe@example.com",
    "password": "securepassword"
  }'

# 商品作成
curl -X POST http://localhost:8080/api/v1/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <jwt_token>" \
  -d '{
    "name": "Premium Laptop",
    "description": "High-performance laptop for professionals",
    "price": "1299.99",
    "currency": "USD",
    "inventory": 100
  }'

# 注文作成
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <jwt_token>" \
  -d '{
    "user_id": "user-123",
    "items": [
      {
        "product_id": "product-456",
        "quantity": 1
      }
    ],
    "shipping_address": {
      "street": "123 Main St",
      "city": "San Francisco",
      "state": "CA",
      "zip": "94105",
      "country": "US"
    }
  }'

# 決済処理
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <jwt_token>" \
  -d '{
    "order_id": "order-789",
    "payment_method": "credit_card",
    "payment_details": {
      "card_number": "4111111111111111",
      "expiry_month": "12",
      "expiry_year": "2025",
      "cvv": "123"
    }
  }'

# ヘルスチェック
curl http://localhost:8080/health

# メトリクス確認
curl http://localhost:8080/metrics

# OpenAPIドキュメント
curl http://localhost:8080/api/docs
```

### メトリクス出力例

```
# HELP ecommerce_users_total Total number of registered users
# TYPE ecommerce_users_total counter
ecommerce_users_total{status="active"} 1250
ecommerce_users_total{status="inactive"} 85

# HELP ecommerce_orders_total Total number of orders
# TYPE ecommerce_orders_total counter
ecommerce_orders_total{status="completed",currency="USD"} 450
ecommerce_orders_total{status="pending",currency="USD"} 23
ecommerce_orders_total{status="cancelled",currency="USD"} 15

# HELP ecommerce_order_value_dollars Order value in dollars
# TYPE ecommerce_order_value_dollars histogram
ecommerce_order_value_dollars_bucket{currency="USD",le="50"} 125
ecommerce_order_value_dollars_bucket{currency="USD",le="100"} 245
ecommerce_order_value_dollars_bucket{currency="USD",le="500"} 380
ecommerce_order_value_dollars_bucket{currency="USD",le="1000"} 440
ecommerce_order_value_dollars_bucket{currency="USD",le="+Inf"} 450

# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="POST",endpoint="/api/v1/orders",status="200"} 450
http_requests_total{method="GET",endpoint="/api/v1/products",status="200"} 2340

# HELP service_availability Service availability percentage
# TYPE service_availability gauge
service_availability{service="user-service"} 99.95
service_availability{service="order-service"} 99.92
service_availability{service="payment-service"} 99.99
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### プロジェクト構成

```
project/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── application.go
│   │   └── config.go
│   ├── domain/
│   │   ├── user/
│   │   ├── product/
│   │   ├── order/
│   │   └── payment/
│   ├── infrastructure/
│   │   ├── database/
│   │   ├── cache/
│   │   ├── messaging/
│   │   └── external/
│   ├── interfaces/
│   │   ├── http/
│   │   ├── grpc/
│   │   └── events/
│   └── pkg/
│       ├── observability/
│       ├── middleware/
│       ├── patterns/
│       └── utils/
├── configs/
│   ├── config.yaml
│   ├── config.prod.yaml
│   └── config.test.yaml
├── deployments/
│   ├── docker-compose.yml
│   └── kubernetes/
├── docs/
└── scripts/
```

### 依存関係注入

```go
type Dependencies struct {
    Config      *Config
    Logger      *slog.Logger
    Tracer      trace.Tracer
    DB          *sql.DB
    Redis       *redis.Client
    EventBus    *EventBus
    
    // サービス
    UserService    *UserService
    ProductService *ProductService
    OrderService   *OrderService
    PaymentService *PaymentService
    
    // 外部クライアント
    EmailClient    EmailClient
    SMSClient      SMSClient
    PaymentGateway PaymentGateway
}

func NewDependencies() (*Dependencies, error) {
    config, err := LoadConfig()
    if err != nil {
        return nil, err
    }
    
    // 可観測性スタック
    observability := NewObservabilityStack(config)
    
    // データベース
    db, err := NewDatabaseConnection(config.Database)
    if err != nil {
        return nil, err
    }
    
    // Redis
    redisClient := NewRedisClient(config.Redis)
    
    // イベントバス
    eventBus := NewEventBus(config.EventBus, observability)
    
    return &Dependencies{
        Config:   config,
        Logger:   observability.Logger,
        Tracer:   observability.Tracer,
        DB:       db,
        Redis:    redisClient,
        EventBus: eventBus,
    }, nil
}
```

### エラーハンドリング

```go
type APIError struct {
    Code      string                 `json:"code"`
    Message   string                 `json:"message"`
    Details   map[string]interface{} `json:"details,omitempty"`
    RequestID string                 `json:"request_id"`
    Timestamp time.Time              `json:"timestamp"`
}

func (e *APIError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewValidationError(field, message string) *APIError {
    return &APIError{
        Code:    "VALIDATION_ERROR",
        Message: fmt.Sprintf("Validation failed for field '%s': %s", field, message),
        Details: map[string]interface{}{
            "field": field,
            "validation_message": message,
        },
        Timestamp: time.Now(),
    }
}

func HandleError(ctx context.Context, w http.ResponseWriter, err error) {
    requestID := GetRequestID(ctx)
    
    var apiErr *APIError
    if !errors.As(err, &apiErr) {
        apiErr = &APIError{
            Code:      "INTERNAL_ERROR",
            Message:   "An internal error occurred",
            RequestID: requestID,
            Timestamp: time.Now(),
        }
    } else {
        apiErr.RequestID = requestID
    }
    
    statusCode := getStatusCodeForError(apiErr.Code)
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(apiErr)
}
```

### テスト構成

```go
type TestSuite struct {
    deps     *Dependencies
    server   *httptest.Server
    cleanup  func()
}

func NewTestSuite(t *testing.T) *TestSuite {
    // テスト用設定
    config := &Config{
        Database: DatabaseConfig{
            Host:     "localhost",
            Port:     5432,
            Database: "ecommerce_test",
        },
        Redis: RedisConfig{
            Addr: "localhost:6379",
            DB:   1, // テスト用DB
        },
    }
    
    // テスト用依存関係
    deps, err := NewTestDependencies(config)
    require.NoError(t, err)
    
    // テストサーバー
    handler := NewRouter(deps)
    server := httptest.NewServer(handler)
    
    return &TestSuite{
        deps:   deps,
        server: server,
        cleanup: func() {
            server.Close()
            deps.DB.Close()
            deps.Redis.Close()
        },
    }
}

func (ts *TestSuite) Cleanup() {
    ts.cleanup()
}

func TestUserRegistration(t *testing.T) {
    suite := NewTestSuite(t)
    defer suite.Cleanup()
    
    user := &User{
        Name:     "Test User",
        Email:    "test@example.com",
        Password: "password123",
    }
    
    // ユーザー登録
    resp, err := suite.createUser(user)
    assert.NoError(t, err)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
    
    // データベース確認
    savedUser, err := suite.deps.UserService.GetByEmail(context.Background(), user.Email)
    assert.NoError(t, err)
    assert.Equal(t, user.Name, savedUser.Name)
}
```

## 🚀 発展課題 (Advanced Features)

基本実装が完了したら、以下の追加機能にもチャレンジしてください：

1. **Kubernetes対応**: Deployment、Service、ConfigMapの作成
2. **Istio Service Mesh**: トラフィック管理、セキュリティポリシー
3. **ArgoCD**: GitOps による継続的デプロイメント
4. **Prometheus Operator**: 高度な監視とアラート
5. **Chaos Engineering**: 障害テストとシステム強化

Go道場60日間の集大成として、エンタープライズレベルのマイクロサービスプラットフォームを構築しましょう！