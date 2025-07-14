# Day 50: gRPC Unary Interceptor

## 🎯 本日の目標 (Today's Goal)

gRPCのUnaryインターセプタを実装し、全てのUnary RPCで共通の処理（ログ、認証、メトリクス収集）を挟み込む仕組みを習得する。プロダクションレベルのgRPCサービスにおける横断的関心事の実装方法を学ぶ。

## 📖 解説 (Explanation)

### Unaryインターセプタとは

Unaryインターセプタは、gRPCのUnary RPC（1リクエスト-1レスポンス）の前後で共通処理を実行するためのミドルウェア機能です。

### 主な用途

1. **ログ出力**: リクエスト/レスポンスのログ
2. **認証・認可**: トークン検証やアクセス制御
3. **メトリクス収集**: レスポンス時間やエラー率の測定
4. **エラーハンドリング**: 統一されたエラー処理
5. **レート制限**: リクエスト頻度の制御

### 実装パターン

```go
// サーバーサイドインターセプタ
func LoggingInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        start := time.Now()
        
        // リクエストログ
        log.Printf("Request: %s", info.FullMethod)
        
        // 実際のハンドラを実行
        resp, err := handler(ctx, req)
        
        // レスポンスログ
        duration := time.Since(start)
        log.Printf("Response: %s (duration: %v, error: %v)", info.FullMethod, duration, err)
        
        return resp, err
    }
}

// クライアントサイドインターセプタ
func AuthInterceptor(token string) grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        // 認証ヘッダーを追加
        ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
        
        // 実際のRPCを実行
        return invoker(ctx, method, req, reply, cc, opts...)
    }
}

// インターセプタの登録
server := grpc.NewServer(
    grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
        LoggingInterceptor(),
        AuthInterceptor(),
        MetricsInterceptor(),
    )),
)
```

## 📝 課題 (The Problem)

Unaryインターセプタを使用して以下の機能を実装してください：

1. **ログインターセプタ**: リクエスト/レスポンスの詳細ログ
2. **認証インターセプタ**: JWTトークンによる認証
3. **メトリクスインターセプタ**: レスポンス時間とエラー率の収集
4. **レート制限インターセプタ**: IPベースのレート制限
5. **インターセプタチェーン**: 複数のインターセプタの組み合わせ

## 💡 ヒント (Hints)

- `grpc.UnaryServerInterceptor`と`grpc.UnaryClientInterceptor`の使用
- `context.Context`を使ったメタデータの伝播
- `grpc.UnaryHandler`による実際のRPC実行
- エラーハンドリングとメトリクス収集の組み合わせ