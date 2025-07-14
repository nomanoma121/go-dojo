# Day 51: gRPC Stream Interceptor

## 🎯 本日の目標 (Today's Goal)

gRPCのStream Interceptor（ストリームインターセプタ）を実装し、ストリーミングRPCに対して共通の処理（認証、ログ、メトリクス、レート制限、回復処理）を適用できるようになる。複数のインターセプタを組み合わせたミドルウェアチェインの構築方法を習得する。

## 📖 解説 (Explanation)

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