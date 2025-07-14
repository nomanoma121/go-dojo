# Day 49: gRPC Bidirectional Streaming

## 🎯 本日の目標 (Today's Goal)

gRPCの双方向ストリーミングを実装し、サーバーとクライアントが同時にメッセージを送受信する仕組みを習得する。チャットシステム、リアルタイム協調機能、双方向データ同期などの用途で活用する。

## 📖 解説 (Explanation)

### 双方向ストリーミングとは

双方向ストリーミングは、クライアントとサーバーが独立してメッセージを送受信できるgRPCの最も柔軟な通信パターンです。

### 主な用途

1. **チャットシステム**: リアルタイムメッセージング
2. **協調編集**: 複数ユーザーでの同時編集
3. **ゲーム**: リアルタイムマルチプレイヤーゲーム
4. **データ同期**: 双方向のリアルタイムデータ同期

### 実装パターン

```go
// プロトコルバッファ定義
service ChatService {
  rpc Chat(stream ChatMessage) returns (stream ChatMessage);
}

// サーバー実装
func (s *Server) Chat(stream pb.ChatService_ChatServer) error {
    for {
        msg, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        
        // メッセージを処理して応答
        response := s.processMessage(msg)
        if err := stream.Send(response); err != nil {
            return err
        }
    }
}

// クライアント実装
func (c *Client) Chat() error {
    stream, err := c.client.Chat(context.Background())
    if err != nil {
        return err
    }
    
    // 送信用ゴルーチン
    go func() {
        for msg := range c.sendChan {
            stream.Send(msg)
        }
    }()
    
    // 受信用ゴルーチン
    for {
        msg, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return err
        }
        c.handleMessage(msg)
    }
    
    return nil
}
```

## 📝 課題 (The Problem)

双方向ストリーミングを使用して以下の機能を実装してください：

1. **チャットシステム**: リアルタイムメッセージング
2. **協調システム**: 複数クライアント間でのデータ共有
3. **ゲームシステム**: プレイヤー間の状態同期
4. **エラーハンドリング**: ストリーム中の適切なエラー処理

## 💡 ヒント (Hints)

- `stream.Recv()`と`stream.Send()`を同時に使用
- ゴルーチンを使った非同期処理
- 適切なチャネルでの送受信管理
- コンテキストを使った適切なライフサイクル管理