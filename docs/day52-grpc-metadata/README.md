# Day 52: gRPC Metadata

## 🎯 本日の目標 (Today's Goal)

gRPCメタデータを使用してリクエストIDやトレース情報などの付加情報をサービス間で伝播させる仕組みを実装できるようになる。メタデータの送受信、伝播パターン、セキュリティ考慮事項を習得する。

## 📖 解説 (Explanation)

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