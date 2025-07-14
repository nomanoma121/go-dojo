# Day 48: gRPC Client-side Streaming

## 🎯 本日の目標 (Today's Goal)

gRPCのクライアントサイドストリーミングを実装し、クライアントからサーバーへ複数のリクエストを継続的に送信する仕組みを習得する。バッチ処理、ファイルアップロード、リアルタイムデータ送信などの用途で活用する。

## 📖 解説 (Explanation)

### クライアントサイドストリーミングとは

クライアントサイドストリーミングは、クライアントが複数のリクエストを順次送信し、サーバーが単一のレスポンスを返すgRPCの通信パターンです。

### 主な用途

1. **ファイルアップロード**: 大きなファイルを分割してアップロード
2. **バッチデータ送信**: 大量のデータを効率的に送信
3. **ログ収集**: クライアントからのログを継続的に収集
4. **メトリクス送信**: 複数のメトリクスを一括送信

### 実装パターン

```go
// プロトコルバッファ定義
service DataCollector {
  rpc CollectData(stream DataPoint) returns (CollectionResult);
}

// クライアント実装
func (c *Client) SendData(dataPoints []*pb.DataPoint) (*pb.CollectionResult, error) {
    stream, err := c.client.CollectData(context.Background())
    if err != nil {
        return nil, err
    }
    
    for _, point := range dataPoints {
        if err := stream.Send(point); err != nil {
            return nil, err
        }
    }
    
    return stream.CloseAndRecv()
}

// サーバー実装
func (s *Server) CollectData(stream pb.DataCollector_CollectDataServer) error {
    var count int32
    
    for {
        point, err := stream.Recv()
        if err == io.EOF {
            return stream.SendAndClose(&pb.CollectionResult{
                TotalPoints: count,
                Status:      "SUCCESS",
            })
        }
        if err != nil {
            return err
        }
        
        // データポイントを処理
        s.processDataPoint(point)
        count++
    }
}
```

## 📝 課題 (The Problem)

クライアントサイドストリーミングを使用して以下の機能を実装してください：

1. **データ収集システム**: 複数のデータポイントを効率的に収集
2. **ファイルアップロード**: 大きなファイルを分割してアップロード
3. **ログ集約**: 複数のログエントリを一括送信
4. **エラーハンドリング**: ストリーム中のエラー処理

## 💡 ヒント (Hints)

- `stream.Send()`でデータを送信
- `stream.CloseAndRecv()`で送信完了とレスポンス受信
- 適切なバッファリングとフロー制御
- サーバーサイドでの`io.EOF`処理