# Day 52: gRPC Metadata

## 🎯 本日の目標 (Today's Goal)

gRPCメタデータを使用してリクエストIDやトレース情報などの付加情報をサービス間で伝播させる仕組みを実装できるようになる。メタデータの送受信、伝播パターン、セキュリティ考慮事項を習得する。

## 📖 解説 (Explanation)

```go
// 【gRPCメタデータの重要性】分散システム通信の中核技術
// ❌ 問題例：不適切なメタデータ実装による壊滅的セキュリティ侵害と情報漏洩
func metadataDisasters() {
    // 🚨 災害例：不正実装による認証バイパス、情報漏洩、システム乗っ取り
    
    // ❌ 最悪の実装1：認証トークンをそのままメタデータに格納し漏洩
    func BadAuthTokenHandler(ctx context.Context, req *pb.UserRequest) (*pb.UserResponse, error) {
        // ❌ 平文で認証情報をメタデータに追加 - 通信傍受で漏洩
        md := metadata.Pairs(
            "username", "admin",
            "password", "super_secret_password", // ❌ 平文パスワード！
            "credit_card", "4111-1111-1111-1111", // ❌ クレジットカード番号！
            "social_security", "123-45-6789",      // ❌ 社会保障番号！
        )
        
        // ❌ 機密情報を含むメタデータをログ出力
        log.Printf("Sending metadata: %+v", md) // 全て標準出力に記録！
        
        // ❌ 下流サービスに機密情報をそのまま伝播
        ctx = metadata.NewOutgoingContext(ctx, md)
        
        // ❌ メタデータをファイルに保存 - 永続的情報漏洩
        saveMetadataToFile(md, "/tmp/metadata.log") // 誰でもアクセス可能
        
        return downstream.CallService(ctx, req)
        
        // 【災害的結果】
        // - ネットワーク通信を傍受されて認証情報全て漏洩
        // - ログファイルから顧客のクレジットカード情報が流出
        // - 攻撃者が管理者権限で全システムにアクセス
        // - 個人情報保護法違反で制裁金100億円
    }
    
    // ❌ 最悪の実装2：メタデータ検証なしで任意コード実行
    func BadMetadataProcessor(ctx context.Context, req *pb.ProcessRequest) (*pb.ProcessResponse, error) {
        md, _ := metadata.FromIncomingContext(ctx)
        
        // ❌ メタデータの値をそのままシステムコマンドに使用
        command := getMetadataValue(md, "system_command")
        if command != "" {
            // ❌ 入力検証なし - 任意のシステムコマンド実行可能
            // 攻撃者: "rm -rf / && cat /etc/passwd"
            exec.Command("sh", "-c", command).Run() // システム完全破壊！
        }
        
        // ❌ メタデータ値をSQLクエリに直接挿入
        userID := getMetadataValue(md, "user_id")
        // 攻撃者: "1'; DROP TABLE users; --"
        query := fmt.Sprintf("SELECT * FROM data WHERE user_id = '%s'", userID)
        database.Exec(query) // データベース全削除！
        
        // ❌ メタデータ値をファイルパスに使用
        filePath := getMetadataValue(md, "file_path")
        // 攻撃者: "../../../../../etc/passwd"
        content, _ := ioutil.ReadFile(filePath) // 任意ファイル読み取り！
        
        return &pb.ProcessResponse{
            Result: string(content), // 機密ファイルの内容を返す
        }, nil
        
        // 【災害的結果】
        // - 攻撃者がメタデータ経由でサーバーを完全制御
        // - 全データベーステーブル削除、10年分のデータ消失
        // - システムファイル漏洩で更なる攻撃の足がかり提供
        // - 会社の全インフラが攻撃者の支配下に
    }
    
    // ❌ 最悪の実装3：メタデータ改ざんによる権限昇格攻撃
    func BadPermissionCheck(ctx context.Context, req *pb.AdminRequest) (*pb.AdminResponse, error) {
        md, _ := metadata.FromIncomingContext(ctx)
        
        // ❌ メタデータの権限情報を信頼 - クライアントが自由に変更可能
        role := getMetadataValue(md, "user_role")      // 攻撃者: "admin"
        permissions := getMetadataValue(md, "permissions") // 攻撃者: "all"
        
        // ❌ デジタル署名や暗号化なし - 改ざん検知不能
        if role == "admin" {
            // ❌ サーバーサイドでの権限検証なし
            // 一般ユーザーが "admin" をメタデータに設定するだけで管理者権限取得
            
            // ❌ 危険な管理者操作を無条件実行
            if req.Operation == "DELETE_ALL_USERS" {
                deleteAllUsers() // 全ユーザーデータ削除
            }
            if req.Operation == "TRANSFER_FUNDS" {
                transferAllFundsToAccount(req.TargetAccount) // 全資金移転
            }
        }
        
        // ❌ 操作ログにメタデータ情報をそのまま記録
        auditLog.Printf("Admin operation by user: %s", getMetadataValue(md, "username"))
        // 攻撃者が偽装した管理者名でログ記録、証拠隠滅
        
        return &pb.AdminResponse{Status: "SUCCESS"}, nil
        
        // 【災害的結果】
        // - 一般ユーザーが管理者権限を自由に取得
        // - 全顧客データ削除、全資金の不正移転
        // - 偽装されたログで攻撃の痕跡を隠蔽
        // - 金融業界から永久追放、刑事責任追及
    }
    
    // ❌ 最悪の実装4：メタデータ蓄積によるメモリリークとDoS攻撃
    func BadMetadataCollector(ctx context.Context, req *pb.CollectRequest) (*pb.CollectResponse, error) {
        md, _ := metadata.FromIncomingContext(ctx)
        
        // ❌ 全てのメタデータを永続保存 - メモリ無限増加
        allMetadata = append(allMetadata, md) // グローバル変数で蓄積
        
        // ❌ メタデータサイズ制限なし - 巨大データでメモリ爆発
        for key, values := range md {
            for _, value := range values {
                // 攻撃者が1GBのメタデータ値を送信可能
                storedMetadata[key] = value // 無制限に保存
            }
        }
        
        // ❌ メタデータ処理で重い計算 - CPU DoS攻撃
        for key, values := range md {
            // 攻撃者が大量のキーを送信 → CPU使用率100%
            for i := 0; i < 1000000; i++ {
                hash := sha256.Sum256([]byte(key + values[0])) // 無駄な計算
                _ = hash
            }
        }
        
        return &pb.CollectResponse{}, nil
        
        // 【災害的結果】
        // - 1日で10TBのメタデータ蓄積、メモリ枯渇
        // - CPU使用率100%で全APIが応答不能
        // - サーバー群が順次ダウン、全社サービス停止
        // - 復旧に1週間、機会損失500億円
    }
    
    // ❌ 最悪の実装5：トレーシング情報漏洩による内部構造暴露
    func BadTracePropagation(ctx context.Context, req *pb.TraceRequest) (*pb.TraceResponse, error) {
        md, _ := metadata.FromIncomingContext(ctx)
        
        // ❌ 内部システム構造をトレース情報に含める
        traceInfo := map[string]string{
            "database_host":     "prod-db-master.internal.company.com", // 内部ホスト名
            "api_key":          "sk-1234567890abcdef",                   // 内部APIキー
            "service_topology": "auth->user->payment->fraud_detection", // システム構成
            "sql_query":        "SELECT * FROM secret_customer_data",   // 実行SQL
        }
        
        // ❌ 機密情報をメタデータとしてクライアントに送信
        for key, value := range traceInfo {
            md = metadata.AppendToOutgoingContext(ctx, key, value)
        }
        
        // ❌ エラー時に内部エラー情報をメタデータに含める
        if err := someInternalOperation(); err != nil {
            // 攻撃者に内部システムの詳細情報を提供
            grpc.SetTrailer(ctx, metadata.Pairs(
                "internal_error", err.Error(),              // 内部エラー詳細
                "stack_trace", fmt.Sprintf("%+v", err),     // スタックトレース
                "database_version", "PostgreSQL 13.7",      // ソフトウェアバージョン
            ))
        }
        
        // 【災害的結果】
        // - 攻撃者が内部システム構成を完全把握
        // - データベース直接攻撃で機密情報全て漏洩
        // - APIキー悪用で他システムへの侵害拡大
        // - システム設計情報流出で競合他社に技術盗用
    }
    
    // 【実際の被害例】
    // - 金融システム：メタデータ経由で取引情報改ざん、数千億円の損失
    // - 医療システム：患者情報がメタデータ経由で流出、集団訴訟
    // - 政府システム：権限昇格攻撃で機密文書アクセス、国家機密漏洩
    // - ECサイト：メタデータDoS攻撃でブラックフライデー全停止、売上ゼロ
    
    fmt.Println("❌ Metadata disasters caused complete system compromise!")
    // 結果：認証バイパス、システム乗っ取り、情報漏洩、国家レベルの問題
}

// ✅ 正解：エンタープライズ級メタデータ管理システム
type EnterpriseMetadataSystem struct {
    // 【セキュリティ】
    encryptionManager    *EncryptionManager      // メタデータ暗号化
    signatureValidator   *SignatureValidator     // デジタル署名検証
    authManager          *AuthManager            // 認証・認可
    permissionChecker    *PermissionChecker      // 権限チェック
    
    // 【入力検証】
    inputValidator       *InputValidator         // 入力検証
    sanitizer            *DataSanitizer          // データサニタイズ
    sizeValidator        *SizeValidator          // サイズ制限
    formatValidator      *FormatValidator        // フォーマット検証
    
    // 【プライバシー保護】
    privacyProtector     *PrivacyProtector       // プライバシー保護
    dataClassifier       *DataClassifier         // データ分類
    anonymizer           *Anonymizer             // 匿名化
    
    // 【監査・コンプライアンス】
    auditLogger          *AuditLogger            // セキュリティ監査
    complianceChecker    *ComplianceChecker      // コンプライアンスチェック
    gdprManager          *GDPRManager            // GDPR対応
    
    // 【リソース管理】
    rateLimiter          *RateLimiter            // レート制限
    resourceMonitor      *ResourceMonitor        // リソース監視
    memoryManager        *MemoryManager          // メモリ管理
    quotaManager         *QuotaManager           // 容量制限
    
    // 【伝播制御】
    propagationManager   *PropagationManager     // 伝播管理
    filterManager        *FilterManager          // フィルタ管理
    transformManager     *TransformManager       // 変換管理
    
    // 【トレーシング】
    traceManager         *SecureTraceManager     // セキュアトレーシング
    correlationManager   *CorrelationManager     // 相関ID管理
    
    // 【パフォーマンス】
    compressionManager   *CompressionManager     // データ圧縮
    cacheManager         *CacheManager           // キャッシュ管理
    
    // 【監視・診断】
    metricsCollector     *MetricsCollector       // メトリクス収集
    healthChecker        *HealthChecker          // ヘルスチェック
    
    config               *MetadataConfig         // 設定管理
    mu                   sync.RWMutex            // 並行アクセス制御
}

// 【重要関数】エンタープライズメタデータシステム初期化
func NewEnterpriseMetadataSystem(config *MetadataConfig) *EnterpriseMetadataSystem {
    return &EnterpriseMetadataSystem{
        config:               config,
        encryptionManager:    NewEncryptionManager(),
        signatureValidator:   NewSignatureValidator(),
        authManager:          NewAuthManager(),
        permissionChecker:    NewPermissionChecker(),
        inputValidator:       NewInputValidator(),
        sanitizer:            NewDataSanitizer(),
        sizeValidator:        NewSizeValidator(),
        formatValidator:      NewFormatValidator(),
        privacyProtector:     NewPrivacyProtector(),
        dataClassifier:       NewDataClassifier(),
        anonymizer:           NewAnonymizer(),
        auditLogger:          NewAuditLogger(),
        complianceChecker:    NewComplianceChecker(),
        gdprManager:          NewGDPRManager(),
        rateLimiter:          NewRateLimiter(),
        resourceMonitor:      NewResourceMonitor(),
        memoryManager:        NewMemoryManager(),
        quotaManager:         NewQuotaManager(),
        propagationManager:   NewPropagationManager(),
        filterManager:        NewFilterManager(),
        transformManager:     NewTransformManager(),
        traceManager:         NewSecureTraceManager(),
        correlationManager:   NewCorrelationManager(),
        compressionManager:   NewCompressionManager(),
        cacheManager:         NewCacheManager(),
        metricsCollector:     NewMetricsCollector(),
        healthChecker:        NewHealthChecker(),
    }
}
```

### gRPCメタデータとは

gRPCメタデータは、RPCコールに付随するキー・バリューペアの情報です。HTTPヘッダーに相当するもので、認証情報、リクエストID、分散トレーシング情報などを伝達するために使用されます。

### メタデータの種類

#### 1. リクエストメタデータ (Incoming Metadata)
クライアントからサーバーへ送信されるメタデータ

#### 2. レスポンスメタデータ (Outgoing Metadata)
サーバーからクライアントへ送信されるメタデータ

#### 3. トレーラー (Trailer)
ストリーム終了時に送信される最終メタデータ

### メタデータの基本操作

#### メタデータの作成と送信

```go
// クライアント側でメタデータを設定
md := metadata.Pairs(
    "request-id", "req-123",
    "user-id", "user-456",
    "authorization", "Bearer token123",
)

ctx := metadata.NewOutgoingContext(context.Background(), md)
response, err := client.GetUser(ctx, request)
```

#### メタデータの受信と読み取り

```go
// サーバー側でメタデータを取得
func (s *UserService) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return nil, status.Error(codes.Internal, "failed to get metadata")
    }
    
    requestID := getMetadataValue(md, "request-id")
    userID := getMetadataValue(md, "user-id")
    
    // ビジネスロジック処理
    return response, nil
}
```

### メタデータ伝播パターン

#### 1. Request ID 伝播

```go
type RequestIDPropagator struct{}

func (p *RequestIDPropagator) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        md, _ := metadata.FromIncomingContext(ctx)
        
        requestID := getMetadataValue(md, "request-id")
        if requestID == "" {
            requestID = generateRequestID()
        }
        
        // コンテキストにリクエストIDを設定
        ctx = context.WithValue(ctx, "request-id", requestID)
        
        // 下流への伝播用メタデータを設定
        ctx = metadata.AppendToOutgoingContext(ctx, "request-id", requestID)
        
        return handler(ctx, req)
    }
}
```

#### 2. 分散トレーシング伝播

```go
type TracePropagator struct{}

func (p *TracePropagator) PropagateTrace(ctx context.Context) context.Context {
    md, _ := metadata.FromIncomingContext(ctx)
    
    traceID := getMetadataValue(md, "trace-id")
    spanID := getMetadataValue(md, "span-id")
    
    if traceID != "" && spanID != "" {
        // 新しいスパンIDを生成
        newSpanID := generateSpanID()
        
        // 下流サービスへの伝播
        ctx = metadata.AppendToOutgoingContext(ctx,
            "trace-id", traceID,
            "parent-span-id", spanID,
            "span-id", newSpanID,
        )
    }
    
    return ctx
}
```

### セキュリティ考慮事項

#### 1. メタデータフィルタリング

```go
type MetadataFilter struct {
    allowedKeys map[string]bool
    sensitiveKeys map[string]bool
}

func (f *MetadataFilter) FilterIncoming(md metadata.MD) metadata.MD {
    filtered := metadata.New(nil)
    
    for key, values := range md {
        // 許可されたキーのみを通す
        if f.allowedKeys[key] {
            filtered[key] = values
        }
        
        // 機密情報をログから除外
        if f.sensitiveKeys[key] {
            log.Printf("Filtered sensitive metadata key: %s", key)
        }
    }
    
    return filtered
}
```

#### 2. 認証メタデータの検証

```go
type AuthMetadataValidator struct {
    tokenValidator TokenValidator
}

func (v *AuthMetadataValidator) ValidateAuth(ctx context.Context) (string, error) {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return "", status.Error(codes.Unauthenticated, "no metadata")
    }
    
    authHeader := getMetadataValue(md, "authorization")
    if authHeader == "" {
        return "", status.Error(codes.Unauthenticated, "no authorization header")
    }
    
    // Bearer トークンの検証
    token := strings.TrimPrefix(authHeader, "Bearer ")
    userID, err := v.tokenValidator.Validate(token)
    if err != nil {
        return "", status.Error(codes.Unauthenticated, "invalid token")
    }
    
    return userID, nil
}
```

### ストリーミングでのメタデータ処理

#### サーバーサイドストリーミング

```go
func (s *StreamService) ServerStream(req *StreamRequest, stream StreamService_ServerStreamServer) error {
    // 初期メタデータを設定
    md := metadata.Pairs(
        "stream-id", generateStreamID(),
        "compression", "gzip",
    )
    stream.SetHeader(md)
    
    // ストリーミング処理
    for i := 0; i < 10; i++ {
        response := &StreamResponse{
            Data: fmt.Sprintf("message-%d", i),
            Timestamp: time.Now().Unix(),
        }
        
        if err := stream.Send(response); err != nil {
            return err
        }
    }
    
    // 最終メタデータ（トレーラー）を設定
    trailer := metadata.Pairs(
        "final-count", "10",
        "stream-status", "completed",
    )
    stream.SetTrailer(trailer)
    
    return nil
}
```

### クライアント側のメタデータ処理

```go
type MetadataAwareClient struct {
    client UserServiceClient
    defaultMetadata metadata.MD
}

func (c *MetadataAwareClient) GetUserWithMetadata(ctx context.Context, userID string) (*User, metadata.MD, error) {
    // デフォルトメタデータを追加
    ctx = metadata.NewOutgoingContext(ctx, c.defaultMetadata)
    
    // 追加のメタデータを設定
    ctx = metadata.AppendToOutgoingContext(ctx,
        "request-id", generateRequestID(),
        "client-version", "1.0.0",
    )
    
    var header, trailer metadata.MD
    
    request := &GetUserRequest{UserId: userID}
    response, err := c.client.GetUser(ctx, request, 
        grpc.Header(&header),
        grpc.Trailer(&trailer),
    )
    
    if err != nil {
        return nil, nil, err
    }
    
    // レスポンスメタデータを処理
    serverID := getMetadataValue(header, "server-id")
    log.Printf("Response from server: %s", serverID)
    
    return response, header, nil
}
```

### 高度なメタデータパターン

#### 1. メタデータチェイニング

```go
type MetadataChain struct {
    processors []MetadataProcessor
}

type MetadataProcessor interface {
    Process(ctx context.Context, md metadata.MD) (context.Context, metadata.MD, error)
}

func (c *MetadataChain) Process(ctx context.Context, md metadata.MD) (context.Context, metadata.MD, error) {
    currentCtx := ctx
    currentMD := md
    
    for _, processor := range c.processors {
        var err error
        currentCtx, currentMD, err = processor.Process(currentCtx, currentMD)
        if err != nil {
            return ctx, md, err
        }
    }
    
    return currentCtx, currentMD, nil
}
```

#### 2. 条件付きメタデータ注入

```go
type ConditionalMetadataInjector struct {
    conditions map[string]func(context.Context) bool
    metadata   map[string]metadata.MD
}

func (i *ConditionalMetadataInjector) Inject(ctx context.Context) context.Context {
    for condName, condFunc := range i.conditions {
        if condFunc(ctx) {
            if md, exists := i.metadata[condName]; exists {
                ctx = metadata.NewOutgoingContext(ctx, md)
            }
        }
    }
    
    return ctx
}
```

## 📝 課題 (The Problem)

以下の機能を持つgRPCメタデータシステムを実装してください：

### 1. MetadataManager の実装

```go
type MetadataManager struct {
    propagators []MetadataPropagator
    filters     []MetadataFilter
    validators  []MetadataValidator
}
```

### 2. 必要なコンポーネントの実装

- `RequestIDPropagator`: リクエストID伝播
- `TracePropagator`: 分散トレーシング情報伝播
- `AuthMetadataValidator`: 認証メタデータ検証
- `MetadataFilter`: メタデータフィルタリング
- `MetadataChain`: メタデータ処理チェイン

### 3. インターセプタ統合

Unary/Streamインターセプタでのメタデータ処理

### 4. クライアント支援機能

メタデータ注入とレスポンス処理の自動化

### 5. セキュリティ機能

機密情報の保護と適切な伝播制御

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestRequestIDPropagation
    main_test.go:45: Request ID propagated: req-123
--- PASS: TestRequestIDPropagation (0.01s)

=== RUN   TestTraceMetadataPropagation
    main_test.go:75: Trace context propagated successfully
--- PASS: TestTraceMetadataPropagation (0.01s)

=== RUN   TestAuthMetadataValidation
    main_test.go:105: Authentication metadata validated
--- PASS: TestAuthMetadataValidation (0.01s)

=== RUN   TestMetadataFiltering
    main_test.go:135: Sensitive metadata filtered correctly
--- PASS: TestMetadataFiltering (0.01s)

PASS
ok      day52-grpc-metadata   0.085s
```

## 💡 ヒント (Hints)

### メタデータ操作

```go
func getMetadataValue(md metadata.MD, key string) string {
    values := md.Get(key)
    if len(values) > 0 {
        return values[0]
    }
    return ""
}

func setMetadataValue(md metadata.MD, key, value string) {
    md.Set(key, value)
}
```

### コンテキスト操作

```go
func propagateMetadata(ctx context.Context, key, value string) context.Context {
    return metadata.AppendToOutgoingContext(ctx, key, value)
}

func extractFromContext(ctx context.Context, key string) string {
    if value, ok := ctx.Value(key).(string); ok {
        return value
    }
    return ""
}
```

### ID生成

```go
func generateRequestID() string {
    return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}

func generateTraceID() string {
    return fmt.Sprintf("trace-%s", uuid.New().String())
}
```

## 🚀 発展課題 (Advanced Features)

基本実装完了後、以下の追加機能にもチャレンジしてください：

1. **メタデータ圧縮**: 大きなメタデータの圧縮機能
2. **メタデータ暗号化**: 機密メタデータの暗号化
3. **動的メタデータ**: 実行時条件によるメタデータ生成
4. **メタデータキャッシュ**: 頻繁に使用されるメタデータのキャッシュ
5. **メタデータ監視**: メタデータの利用状況とパフォーマンス監視

gRPCメタデータの実装を通じて、マイクロサービス間での効果的な情報伝播パターンを習得しましょう！