# Day 47: gRPC Server-side Streaming

## 🎯 本日の目標 (Today's Goal)

gRPCのサーバーサイドストリーミングを実装し、サーバーからクライアントへ連続的にデータを送信する仕組みを習得する。リアルタイム通知、ログストリーミング、大量データの分割送信などの用途で活用する。

## 📖 解説 (Explanation)

### サーバーサイドストリーミングとは

サーバーサイドストリーミングは、クライアントが一つのリクエストを送信し、サーバーが複数のレスポンスを順次返すgRPCの通信パターンです。

### 主な用途

1. **リアルタイム通知**: チャット、アラート、イベント通知
2. **大量データの分割送信**: ファイルダウンロード、データベースの大量レコード
3. **継続的なデータ配信**: ログストリーミング、メトリクス監視
4. **プログレッシブ配信**: 検索結果の段階的表示

### 実装パターン

```go
// プロトコルバッファ定義
service DataService {
  rpc StreamData(StreamRequest) returns (stream DataResponse);
}

// サーバー実装
func (s *Server) StreamData(req *pb.StreamRequest, stream pb.DataService_StreamDataServer) error {
    for i := 0; i < 10; i++ {
        response := &pb.DataResponse{
            Data: fmt.Sprintf("Data %d", i),
            Timestamp: time.Now().Unix(),
        }
        
        if err := stream.Send(response); err != nil {
            return err
        }
        
        time.Sleep(time.Second)
    }
    return nil
}
```

## 📝 課題 (The Problem)

サーバーサイドストリーミングを使用して以下の機能を実装してください：

1. **ログストリーミング**: リアルタイムでログを配信
2. **ファイル転送**: 大きなファイルを分割して送信
3. **イベント通知**: システムイベントの継続配信
4. **エラーハンドリング**: ストリーム中のエラー処理

## 💡 ヒント (Hints)

- `stream.Send()`でデータを送信
- `context`を使用したキャンセル処理
- 適切なバッファリングとフロー制御
- クライアント切断の検出と処理