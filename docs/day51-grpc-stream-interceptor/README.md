# Day 51: gRPC Stream Interceptor

## 🎯 本日の目標 (Today's Goal)

gRPCのStream Interceptor（ストリームインターセプタ）を実装し、ストリーミングRPCに対して共通の処理（認証、ログ、メトリクス、レート制限、回復処理）を適用できるようになる。複数のインターセプタを組み合わせたミドルウェアチェインの構築方法を習得する。

## 📖 解説 (Explanation)

```go
// 【gRPCストリームインターセプタの重要性】エンタープライズストリーミングシステムの中核技術
// ❌ 問題例：不適切なストリームインターセプタ実装による壊滅的セキュリティ侵害とシステム崩壊
func streamInterceptorDisasters() {
    // 🚨 災害例：不正実装によるメモリリーク、認証バイパス、DoS攻撃増幅
    
    // ❌ 最悪の実装1：メモリリークを引き起こすメトリクスインターセプタ
    func BadStreamMetricsInterceptor() StreamServerInterceptor {
        // ❌ グローバル変数でストリーム情報を永続保存 - メモリリーク
        var allStreamMetrics []StreamMetric // 削除されない！
        
        return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
            start := time.Now()
            
            // ❌ ストリーム全体の内容をメモリに保存
            wrappedStream := &BadWrappedServerStream{
                ServerStream: ss,
                sentMessages: make([]interface{}, 0), // 無限に蓄積
                recvMessages: make([]interface{}, 0), // 無限に蓄積
            }
            
            err := handler(srv, wrappedStream)
            
            // ❌ 全ストリームデータを永続保存 - メモリ爆発
            metric := StreamMetric{
                Method:       info.FullMethod,
                Duration:     time.Since(start),
                SentMessages: wrappedStream.sentMessages, // 全データ保存！
                RecvMessages: wrappedStream.recvMessages, // 全データ保存！
                Timestamp:    time.Now(),
            }
            allStreamMetrics = append(allStreamMetrics, metric) // 無限増加
            
            return err
        }
        
        // 【災害的結果】
        // - 1日で100万ストリーム → メモリ使用量1TB
        // - 2日後: サーバーOOM Kill、全サービス停止
        // - 復旧に48時間、売上損失100億円
    }
    
    // ❌ 最悪の実装2：認証バイパス可能なセキュリティインターセプタ
    func BadStreamAuthInterceptor() StreamServerInterceptor {
        return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
            ctx := ss.Context()
            
            // ❌ 認証チェックをストリーム開始時のみ実行
            md, _ := metadata.FromIncomingContext(ctx)
            token := getMetadataValue(md, "authorization")
            
            // ❌ トークン検証なし - 偽造トークンでも通過
            if token == "" {
                // ❌ 認証なしでも実行許可 - セキュリティホール
                log.Println("Warning: No auth token, but allowing access")
            }
            
            // ❌ トークン期限切れを検証しない
            // 長時間ストリーミング中にトークンが無効になっても気づかない
            
            // ❌ ストリーミング中の権限変更を検知しない
            // ユーザーが途中で権限を剥奪されても継続実行
            
            return handler(srv, ss)
        }
        
        // 【災害的結果】
        // - 期限切れトークンで24時間継続アクセス
        // - 元従業員が退職後も機密データにアクセス
        // - データ漏洩で制裁金50億円、信頼失墜
    }
    
    // ❌ 最悪の実装3：DoS攻撃を増幅するレート制限インターセプタ
    func BadStreamRateLimitInterceptor() StreamServerInterceptor {
        // ❌ 排他制御なしでマップアクセス - レースコンディション
        activeStreams := make(map[string]int)
        
        return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
            // ❌ クライアント識別が脆弱 - IPスプーフィング可能
            clientIP := getClientIP(ss.Context()) // X-Forwarded-For偽装可能
            
            // ❌ 競合状態でカウンタが不正確
            activeStreams[clientIP]++ // データ競合発生
            
            // ❌ レート制限チェックが後 - リソース消費済み
            if activeStreams[clientIP] > 100 {
                return fmt.Errorf("too many streams")
            }
            
            // ❌ クリーンアップなし - カウンタが減らない
            err := handler(srv, ss)
            // activeStreams[clientIP]-- が実行されない！
            
            return err
        }
        
        // 【災害的結果】
        // - 攻撃者がIP偽装でレート制限回避
        // - カウンタリークで実際より多い接続数を記録
        // - 正常ユーザーが接続拒否、サービス利用不能
    }
    
    // ❌ 最悪の実装4：機密情報を漏洩するログインターセプタ
    func BadStreamLoggingInterceptor() StreamServerInterceptor {
        return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
            // ❌ ストリーム全体の内容をログ出力 - 機密情報大量流出
            wrappedStream := &LoggingWrappedStream{
                ServerStream: ss,
                logger:       log.New(os.Stdout, "", log.LstdFlags),
            }
            
            return handler(srv, wrappedStream)
        }
    }
    
    type LoggingWrappedStream struct {
        ServerStream
        logger *log.Logger
    }
    
    func (ls *LoggingWrappedStream) SendMsg(m interface{}) error {
        // ❌ 送信メッセージ全体をログ出力 - 個人情報流出
        ls.logger.Printf("SEND: %+v", m) // パスワード、クレジットカード番号も出力
        return ls.ServerStream.SendMsg(m)
    }
    
    func (ls *LoggingWrappedStream) RecvMsg(m interface{}) error {
        err := ls.ServerStream.RecvMsg(m)
        if err == nil {
            // ❌ 受信メッセージ全体をログ出力 - 機密データ流出
            ls.logger.Printf("RECV: %+v", m) // 医療記録、財務情報も出力
        }
        return err
    }
        
        // 【災害的結果】
        // - 患者の医療記録、金融取引データがログに記録
        // - ログ監視システム経由で機密情報が開発チーム全員に配信
        // - GDPR違反、医療法違反で経営陣逮捕、企業解散
    
    // ❌ 最悪の実装5：リカバリー処理でさらに深刻な障害を引き起こす
    func BadStreamRecoveryInterceptor() StreamServerInterceptor {
        return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
            defer func() {
                if r := recover(); r != nil {
                    // ❌ パニック情報を機密データと一緒にログ出力
                    log.Printf("PANIC in stream %s: %+v", info.FullMethod, r)
                    
                    // ❌ パニック時にリソースクリーンアップなし
                    // データベース接続、ファイルハンドルがリーク
                    
                    // ❌ クライアントに内部エラー情報を送信 - 情報漏洩
                    ss.SetTrailer(metadata.Pairs("error", fmt.Sprintf("%+v", r)))
                    
                    // ❌ パニック発生を隠蔽 - 障害の根本原因特定不能
                    return // エラーを返さずに隠蔽
                }
            }()
            
            return handler(srv, ss)
        }
        
        // 【災害的結果】
        // - パニック時にデータベース接続1000個リーク
        // - 内部システム構造が攻撃者に漏洩
        // - 障害の根本原因が特定できず、再発防止不能
    }
    
    // 【実際の被害例】
    // - 金融システム：ストリームメトリクス蓄積でメモリ枯渇、取引システム停止
    // - 医療システム：患者データログ出力で個人情報流出、集団訴訟
    // - 政府システム：認証バイパスで機密文書アクセス、国家機密漏洩
    // - ECサイト：レート制限バグで攻撃者が無制限アクセス、サーバー崩壊
    
    fmt.Println("❌ Stream interceptor disasters caused national security breach!")
    // 結果：メモリリーク、認証バイパス、情報漏洩、国家レベルの問題
}

// ✅ 正解：エンタープライズ級ストリームインターセプタシステム
type EnterpriseStreamInterceptorSystem struct {
    // 【セキュリティ】
    authManager          *AuthManager            // 認証・認可管理
    tokenValidator       *TokenValidator         // トークン検証
    permissionChecker    *PermissionChecker      // 権限チェック
    encryptionManager    *EncryptionManager      // データ暗号化
    
    // 【監査・コンプライアンス】
    auditLogger          *AuditLogger            // セキュリティ監査
    privacyProtector     *PrivacyProtector       // プライバシー保護
    complianceChecker    *ComplianceChecker      // コンプライアンスチェック
    
    // 【リソース管理】
    rateLimiter          *DistributedRateLimiter // 分散レート制限
    resourceMonitor      *ResourceMonitor        // リソース監視
    memoryManager        *MemoryManager          // メモリ管理
    connectionPool       *ConnectionPool         // 接続プール管理
    
    // 【パフォーマンス】
    metricsCollector     *StreamMetricsCollector // ストリームメトリクス
    performanceAnalyzer  *PerformanceAnalyzer    // パフォーマンス分析
    loadBalancer         *LoadBalancer           // 負荷分散
    
    // 【障害対応】
    circuitBreaker       *CircuitBreaker         // サーキットブレーカー
    healthChecker        *HealthChecker          // ヘルスチェック
    recoveryManager      *RecoveryManager        // 復旧管理
    
    // 【ストリーム管理】
    streamRegistry       *StreamRegistry         // ストリーム登録管理
    sessionManager       *SessionManager         // セッション管理
    lifecycleManager     *LifecycleManager       // ライフサイクル管理
    
    config               *InterceptorConfig      // 設定管理
    mu                   sync.RWMutex            // 並行アクセス制御
}

// 【重要関数】エンタープライズストリームインターセプタシステム初期化
func NewEnterpriseStreamInterceptorSystem(config *InterceptorConfig) *EnterpriseStreamInterceptorSystem {
    return &EnterpriseStreamInterceptorSystem{
        config:               config,
        authManager:          NewAuthManager(),
        tokenValidator:       NewTokenValidator(),
        permissionChecker:    NewPermissionChecker(),
        encryptionManager:    NewEncryptionManager(),
        auditLogger:          NewAuditLogger(),
        privacyProtector:     NewPrivacyProtector(),
        complianceChecker:    NewComplianceChecker(),
        rateLimiter:          NewDistributedRateLimiter(),
        resourceMonitor:      NewResourceMonitor(),
        memoryManager:        NewMemoryManager(),
        connectionPool:       NewConnectionPool(),
        metricsCollector:     NewStreamMetricsCollector(),
        performanceAnalyzer:  NewPerformanceAnalyzer(),
        loadBalancer:         NewLoadBalancer(),
        circuitBreaker:       NewCircuitBreaker(),
        healthChecker:        NewHealthChecker(),
        recoveryManager:      NewRecoveryManager(),
        streamRegistry:       NewStreamRegistry(),
        sessionManager:       NewSessionManager(),
        lifecycleManager:     NewLifecycleManager(),
    }
}
```

### Stream Interceptor とは

Stream Interceptorは、gRPCのストリーミングRPC（Server-side streaming、Client-side streaming、Bidirectional streaming）に対して、横断的な関心事を実装するためのミドルウェアパターンです。

### Unary Interceptor との違い

**Unary Interceptor:**
- 単一のリクエスト/レスポンス
- シンプルな前処理/後処理

**Stream Interceptor:**
- 継続的なメッセージ交換
- ストリームのライフサイクル管理
- リアルタイムメトリクス収集

### Stream Interceptor の実装

#### 基本的なインターフェース

```go
type StreamServerInterceptor func(
    srv interface{}, 
    ss ServerStream, 
    info *StreamServerInfo, 
    handler StreamHandler
) error

type StreamServerInfo struct {
    FullMethod     string
    IsClientStream bool
    IsServerStream bool
}

type StreamHandler func(srv interface{}, stream ServerStream) error
```

#### ServerStream インターフェース

```go
type ServerStream interface {
    SetHeader(map[string]string) error
    SendHeader(map[string]string) error
    SetTrailer(map[string]string)
    Context() context.Context
    SendMsg(m interface{}) error
    RecvMsg(m interface{}) error
}
```

### 主要なインターセプタの実装

#### 1. ログインターセプタ

```go
func StreamLoggingInterceptor() StreamServerInterceptor {
    return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
        start := time.Now()
        
        log.Printf("[STREAM START] Method: %s, Type: client=%t server=%t", 
            info.FullMethod, info.IsClientStream, info.IsServerStream)
        
        wrappedStream := NewWrappedServerStream(ss)
        err := handler(srv, wrappedStream)
        
        sent, recv, duration := wrappedStream.GetStats()
        status := "SUCCESS"
        if err != nil {
            status = "ERROR"
        }
        
        log.Printf("[STREAM END] Method: %s, Duration: %v, Sent: %d, Recv: %d, Status: %s", 
            info.FullMethod, duration, sent, recv, status)
        
        return err
    }
}
```

#### 2. 認証インターセプタ

```go
func StreamAuthInterceptor() StreamServerInterceptor {
    return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
        ctx := ss.Context()
        token := extractTokenFromContext(ctx)
        
        if token == "" {
            return fmt.Errorf("stream authentication required")
        }
        
        _, err := validateStreamToken(token)
        if err != nil {
            return fmt.Errorf("stream authentication failed: %w", err)
        }
        
        return handler(srv, ss)
    }
}
```

#### 3. メトリクスインターセプタ

```go
func StreamMetricsInterceptor(metrics *StreamMetrics) StreamServerInterceptor {
    return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
        metrics.StartStream(info.FullMethod)
        
        wrappedStream := NewWrappedServerStream(ss)
        err := handler(srv, wrappedStream)
        
        sent, recv, duration := wrappedStream.GetStats()
        metrics.EndStream(info.FullMethod, sent, recv, duration)
        
        return err
    }
}
```

#### 4. レート制限インターセプタ

```go
func StreamRateLimitInterceptor(limiter *StreamRateLimiter) StreamServerInterceptor {
    return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
        if !limiter.CanStartStream(info.FullMethod) {
            return fmt.Errorf("stream rate limit exceeded for method: %s", info.FullMethod)
        }
        
        limiter.StartStream(info.FullMethod)
        defer limiter.EndStream(info.FullMethod)
        
        return handler(srv, ss)
    }
}
```

### WrappedServerStream によるメトリクス収集

```go
type WrappedServerStream struct {
    ServerStream
    sentCount     int64
    recvCount     int64
    startTime     time.Time
    lastActivity  time.Time
    mu            sync.RWMutex
}

func (w *WrappedServerStream) SendMsg(m interface{}) error {
    w.mu.Lock()
    w.sentCount++
    w.lastActivity = time.Now()
    w.mu.Unlock()
    
    return w.ServerStream.SendMsg(m)
}

func (w *WrappedServerStream) RecvMsg(m interface{}) error {
    w.mu.Lock()
    w.recvCount++
    w.lastActivity = time.Now()
    w.mu.Unlock()
    
    return w.ServerStream.RecvMsg(m)
}
```

### インターセプタチェイニング

```go
func ChainStreamServer(interceptors ...StreamServerInterceptor) StreamServerInterceptor {
    switch len(interceptors) {
    case 0:
        return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
            return handler(srv, ss)
        }
    case 1:
        return interceptors[0]
    default:
        return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
            chainerHandler := func(currentSrv interface{}, currentStream ServerStream) error {
                return ChainStreamServer(interceptors[1:]...)(currentSrv, currentStream, info, handler)
            }
            return interceptors[0](srv, ss, info, chainerHandler)
        }
    }
}
```

### 使用例

```go
// 複数のインターセプタを組み合わせ
metrics := NewStreamMetrics()
limiter := NewStreamRateLimiter()

chainedInterceptor := ChainStreamServer(
    StreamRecoveryInterceptor(),
    StreamLoggingInterceptor(),
    StreamAuthInterceptor(),
    StreamMetricsInterceptor(metrics),
    StreamRateLimitInterceptor(limiter),
)

server := NewInterceptorStreamServer(service, chainedInterceptor)
```

## 📝 課題 (The Problem)

以下の機能を持つStream Interceptorシステムを実装してください：

### 1. StreamServerInterceptor の実装

```go
type StreamServerInterceptor func(
    srv interface{}, 
    ss ServerStream, 
    info *StreamServerInfo, 
    handler StreamHandler
) error
```

### 2. 必要なインターセプタの実装

- `StreamLoggingInterceptor`: ストリーミングログ
- `StreamAuthInterceptor`: ストリーミング認証  
- `StreamMetricsInterceptor`: ストリーミングメトリクス
- `StreamRateLimitInterceptor`: ストリーミングレート制限
- `StreamRecoveryInterceptor`: ストリーミング回復処理

### 3. WrappedServerStream の実装

メッセージ送受信数とストリーム持続時間の追跡

### 4. インターセプタチェイニング

複数のインターセプタを組み合わせるチェイン機能

### 5. メトリクス収集

ストリーミング統計情報の詳細な収集と分析

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestStreamLoggingInterceptor
    main_test.go:45: [STREAM START] Method: /StreamingService/ServerSideStream
    main_test.go:48: [STREAM END] Method: /StreamingService/ServerSideStream, Duration: 501ms, Sent: 5, Recv: 0
--- PASS: TestStreamLoggingInterceptor (0.50s)

=== RUN   TestStreamAuthInterceptor
    main_test.go:75: Stream authentication successful
--- PASS: TestStreamAuthInterceptor (0.01s)

=== RUN   TestStreamMetricsInterceptor
    main_test.go:105: Stream metrics collected: sent=5, recv=0, duration=501ms
--- PASS: TestStreamMetricsInterceptor (0.50s)

=== RUN   TestChainedStreamInterceptors
    main_test.go:135: All interceptors executed in correct order
--- PASS: TestChainedStreamInterceptors (0.50s)

PASS
ok      day51-grpc-stream-interceptor   2.025s
```

## 💡 ヒント (Hints)

### WrappedServerStream の実装

```go
type WrappedServerStream struct {
    ServerStream
    sentCount     int64
    recvCount     int64
    startTime     time.Time
    mu            sync.RWMutex
}

func (w *WrappedServerStream) SendMsg(m interface{}) error {
    atomic.AddInt64(&w.sentCount, 1)
    return w.ServerStream.SendMsg(m)
}
```

### メトリクス収集の実装

```go
type StreamMetrics struct {
    ActiveStreams    map[string]int64
    CompletedStreams map[string]int64
    MessagesSent     map[string]int64
    MessagesReceived map[string]int64
    mu               sync.RWMutex
}
```

### レート制限の実装

```go
type StreamRateLimiter struct {
    activeStreams map[string]int
    maxStreams    map[string]int
    mu            sync.RWMutex
}

func (srl *StreamRateLimiter) CanStartStream(method string) bool {
    srl.mu.RLock()
    defer srl.mu.RUnlock()
    
    limit := srl.maxStreams[method]
    current := srl.activeStreams[method]
    return current < limit
}
```

## 🚀 発展課題 (Advanced Features)

基本実装完了後、以下の追加機能にもチャレンジしてください：

1. **アダプティブレート制限**: 負荷に応じた動的制限調整
2. **ストリーム品質監視**: メッセージ遅延やスループットの監視
3. **自動復旧機能**: 異常ストリームの自動終了と復旧
4. **分散メトリクス**: 複数サーバー間でのメトリクス集約
5. **ストリーム記録**: デバッグ用のストリーム内容記録

Stream Interceptorの実装を通じて、gRPCストリーミングにおける高度なミドルウェアパターンを習得しましょう！