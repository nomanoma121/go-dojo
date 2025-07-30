# Day 49: gRPC Bidirectional Streaming

## 🎯 本日の目標 (Today's Goal)

gRPCの双方向ストリーミングを実装し、サーバーとクライアントが同時にメッセージを送受信する仕組みを習得する。チャットシステム、リアルタイム協調機能、双方向データ同期などの用途で活用する。

## 📖 解説 (Explanation)

### 双方向ストリーミングとは

```go
// 【gRPC双方向ストリーミングの重要性】リアルタイムシステムの基盤技術
// ❌ 問題例：双方向ストリーミング実装ミスによる大規模障害とユーザー離れ
func bidirectionalStreamingDisasters() {
    // 🚨 災害例：不適切な双方向ストリーミング実装による壊滅的サービス障害
    
    // ❌ 最悪の実装1：メモリリークを引き起こすチャットサーバー
    func BadChatServer(stream pb.ChatService_ChatServer) error {
        // ❌ 接続管理なし - メモリリークの温床
        connectedUsers := make(map[string]pb.ChatService_ChatServer) // リークする！
        
        for {
            msg, err := stream.Recv()
            if err == io.EOF {
                return nil // ❌ クリーンアップなし！
            }
            if err != nil {
                return err // ❌ エラー時もクリーンアップなし！
            }
            
            // ❌ ユーザー追加時のクリーンアップ処理がない
            connectedUsers[msg.UserId] = stream
            
            // ❌ 全ユーザーにブロードキャスト - 無制限負荷
            for _, userStream := range connectedUsers {
                // ❌ Send エラーチェックなし - デッドロック発生
                userStream.Send(msg) // 失敗時の処理なし
            }
        }
        
        // 【災害的結果】
        // - 1時間後: 10,000接続でメモリ使用量50GB
        // - 2時間後: サーバーOOM Kill
        // - 結果: 全ユーザーの接続切断、サービス停止
    }
    
    // ❌ 最悪の実装2：DoS攻撃を受けやすいオンラインゲーム
    func BadGameServer(stream pb.GameService_GameServer) error {
        // ❌ レート制限なし - DoS攻撃の標的
        for {
            action, err := stream.Recv()
            if err != nil {
                return err
            }
            
            // ❌ 入力検証なし - 不正データで他プレイヤーに影響
            if action.Type == "MOVE" {
                // ❌ 座標検証なし - プレイヤーがマップ外に移動可能
                updatePlayerPosition(action.PlayerId, action.X, action.Y)
            }
            
            // ❌ すべてのアクションを全プレイヤーにブロードキャスト
            // 攻撃者が1秒に10,000アクション送信 → 全プレイヤーの帯域圧迫
            broadcastToAllPlayers(action)
        }
        
        // 【災害的結果】
        // - 攻撃者1人が1秒10,000アクション送信
        // - 1,000人のプレイヤー全員が帯域不足でラグ発生
        // - 15分後: ゲームサーバー完全停止
        // - ユーザー離脱率95%、売上激減
    }
    
    // ❌ 最悪の実装3：データ競合だらけの協調編集システム
    func BadCollaborativeEditor(stream pb.EditorService_EditorServer) error {
        // ❌ 排他制御なし - データ競合の嵐
        for {
            edit, err := stream.Recv()
            if err != nil {
                return err
            }
            
            // ❌ 複数ユーザーが同時編集時の競合解決なし
            document := getDocument(edit.DocumentId)
            
            // ❌ トランザクション制御なし
            document.Content = applyEdit(document.Content, edit)
            
            // ❌ 保存に失敗しても他ユーザーに通知
            saveDocument(document) // エラーチェックなし
            
            // ❌ 全ユーザーに無条件ブロードキャスト
            notifyAllUsers(edit) // 順序保証なし、データ不整合発生
        }
        
        // 【災害的結果】
        // - 10人が同時編集: データが完全に破損
        // - 文書の内容が意味不明な状態に
        // - バックアップからの復旧に8時間
        // - 顧客からの信頼失墜、契約解除
    }
    
    // 【実際の被害例】
    // - ゲーム企業：オンラインゲームのサーバー障害で1日売上ゼロ
    // - SaaS企業：協調編集機能でデータ破損、顧客離れ50%
    // - 金融システム：リアルタイム取引で競合状態、監査法人から指摘
    // - 物流システム：配送追跡でデータ不整合、配送遅延多発
    
    fmt.Println("❌ Bidirectional streaming disasters caused service shutdown and user exodus!")
    // 結果：メモリリーク、DoS攻撃、データ競合でサービス崩壊
}

// ✅ 正解：エンタープライズ級双方向ストリーミングシステム
type EnterpriseBidirectionalStreamingSystem struct {
    // 【接続管理】
    connectionManager    *ConnectionManager       // 接続管理
    sessionManager       *SessionManager         // セッション管理
    streamRegistry       *StreamRegistry         // ストリーム登録
    
    // 【メッセージルーティング】
    messageRouter        *MessageRouter          // メッセージルーティング
    broadcastManager     *BroadcastManager       // ブロードキャスト管理
    topicManager         *TopicManager           // トピック管理
    
    // 【セキュリティ】
    authManager          *AuthManager            // 認証管理
    rateLimiter          *RateLimiter            // レート制限
    inputValidator       *InputValidator         // 入力検証
    
    // 【パフォーマンス】
    loadBalancer         *LoadBalancer           // 負荷分散
    backpressureManager  *BackpressureManager    // バックプレッシャー制御
    bufferManager        *BufferManager          // バッファ管理
    
    // 【監視・診断】
    metricsCollector     *MetricsCollector       // メトリクス収集
    healthChecker        *HealthChecker          // ヘルスチェック
    errorTracker         *ErrorTracker           // エラー追跡
    
    // 【障害対応】
    circuitBreaker       *CircuitBreaker         // サーキットブレーカー
    retryManager         *RetryManager           // リトライ管理
    failoverManager      *FailoverManager        // フェイルオーバー
    
    // 【データ整合性】
    conflictResolver     *ConflictResolver       // 競合解決
    transactionManager   *TransactionManager     // トランザクション管理
    versionManager       *VersionManager         // バージョン管理
    
    config               *StreamingConfig        // 設定管理
    mu                   sync.RWMutex            // 並行アクセス制御
}

// 【重要関数】エンタープライズ双方向ストリーミングシステム初期化
func NewEnterpriseBidirectionalStreamingSystem(config *StreamingConfig) *EnterpriseBidirectionalStreamingSystem {
    return &EnterpriseBidirectionalStreamingSystem{
        config:               config,
        connectionManager:    NewConnectionManager(),
        sessionManager:       NewSessionManager(),
        streamRegistry:       NewStreamRegistry(),
        messageRouter:        NewMessageRouter(),
        broadcastManager:     NewBroadcastManager(),
        topicManager:         NewTopicManager(),
        authManager:          NewAuthManager(),
        rateLimiter:          NewRateLimiter(),
        inputValidator:       NewInputValidator(),
        loadBalancer:         NewLoadBalancer(),
        backpressureManager:  NewBackpressureManager(),
        bufferManager:        NewBufferManager(),
        metricsCollector:     NewMetricsCollector(),
        healthChecker:        NewHealthChecker(),
        errorTracker:         NewErrorTracker(),
        circuitBreaker:       NewCircuitBreaker(),
        retryManager:         NewRetryManager(),
        failoverManager:      NewFailoverManager(),
        conflictResolver:     NewConflictResolver(),
        transactionManager:   NewTransactionManager(),
        versionManager:       NewVersionManager(),
    }
}

// 【実用例】エンタープライズ級チャットサーバー実装
func (ebss *EnterpriseBidirectionalStreamingSystem) HandleEnterpriseChat(
    stream pb.ChatService_ChatServer,
) error {
    
    // 【STEP 1】接続認証と制限チェック
    clientID, err := ebss.authManager.AuthenticateStream(stream)
    if err != nil {
        return status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
    }
    
    // レート制限チェック
    if !ebss.rateLimiter.AllowConnection(clientID) {
        return status.Errorf(codes.ResourceExhausted, "connection rate limit exceeded")
    }
    
    // 【STEP 2】接続管理とセッション作成
    connection := ebss.connectionManager.CreateConnection(clientID, stream)
    session := ebss.sessionManager.CreateSession(connection)
    
    // クリーンアップ用defer
    defer func() {
        ebss.connectionManager.RemoveConnection(clientID)
        ebss.sessionManager.CloseSession(session.ID)
        ebss.metricsCollector.RecordDisconnection(clientID)
    }()
    
    // 【STEP 3】受信処理用ゴルーチン
    receiveChan := make(chan *pb.ChatMessage, 100)
    errorChan := make(chan error, 1)
    
    go func() {
        defer close(receiveChan)
        for {
            msg, err := stream.Recv()
            if err == io.EOF {
                return
            }
            if err != nil {
                errorChan <- fmt.Errorf("receive error: %w", err)
                return
            }
            
            // 入力検証
            if err := ebss.inputValidator.ValidateMessage(msg); err != nil {
                errorChan <- fmt.Errorf("validation failed: %w", err)
                return
            }
            
            // レート制限チェック
            if !ebss.rateLimiter.AllowMessage(clientID) {
                continue // メッセージを破棄
            }
            
            receiveChan <- msg
        }
    }()
    
    // 【STEP 4】送信処理用ゴルーチン
    sendChan := make(chan *pb.ChatMessage, 100)
    
    go func() {
        for msg := range sendChan {
            // バックプレッシャー制御
            if !ebss.backpressureManager.CanSend(clientID) {
                // 送信をスキップまたは遅延
                time.Sleep(10 * time.Millisecond)
                continue
            }
            
            // 送信実行
            if err := stream.Send(msg); err != nil {
                ebss.errorTracker.RecordError(clientID, err)
                return
            }
            
            ebss.metricsCollector.RecordMessageSent(clientID)
        }
    }()
    
    // 【STEP 5】メインメッセージ処理ループ
    for {
        select {
        case msg := <-receiveChan:
            if msg == nil {
                return nil // 正常終了
            }
            
            // メッセージ処理
            err := ebss.processMessage(session, msg, sendChan)
            if err != nil {
                ebss.errorTracker.RecordError(clientID, err)
                continue
            }
            
        case err := <-errorChan:
            return fmt.Errorf("stream error: %w", err)
            
        case <-stream.Context().Done():
            return stream.Context().Err()
        }
    }
}

// 【核心メソッド】メッセージ処理とブロードキャスト
func (ebss *EnterpriseBidirectionalStreamingSystem) processMessage(
    session *Session,
    msg *pb.ChatMessage,
    sendChan chan<- *pb.ChatMessage,
) error {
    
    // 【STEP 1】メッセージタイプ別処理
    switch msg.Type {
    case pb.MessageType_CHAT:
        return ebss.processChatMessage(session, msg, sendChan)
    case pb.MessageType_JOIN_ROOM:
        return ebss.processJoinRoom(session, msg, sendChan)
    case pb.MessageType_LEAVE_ROOM:
        return ebss.processLeaveRoom(session, msg, sendChan)
    case pb.MessageType_TYPING:
        return ebss.processTypingIndicator(session, msg, sendChan)
    default:
        return fmt.Errorf("unknown message type: %v", msg.Type)
    }
}

// 【高度機能】チャットメッセージ処理
func (ebss *EnterpriseBidirectionalStreamingSystem) processChatMessage(
    session *Session,
    msg *pb.ChatMessage,
    sendChan chan<- *pb.ChatMessage,
) error {
    
    // 【STEP 1】権限チェック
    if !ebss.authManager.CanSendToRoom(session.UserID, msg.RoomId) {
        return status.Errorf(codes.PermissionDenied, "no permission to send to room")
    }
    
    // 【STEP 2】メッセージ永続化
    messageID, err := ebss.saveMessage(msg)
    if err != nil {
        return fmt.Errorf("failed to save message: %w", err)
    }
    msg.MessageId = messageID
    msg.Timestamp = time.Now().Unix()
    
    // 【STEP 3】ルーム内の全ユーザーにブロードキャスト
    roomUsers := ebss.sessionManager.GetRoomUsers(msg.RoomId)
    
    // 並行ブロードキャスト
    var wg sync.WaitGroup
    for _, userID := range roomUsers {
        wg.Add(1)
        go func(uid string) {
            defer wg.Done()
            
            userSession := ebss.sessionManager.GetSession(uid)
            if userSession == nil {
                return
            }
            
            // 送信試行（タイムアウト付き）
            select {
            case userSession.SendChan <- msg:
                ebss.metricsCollector.RecordBroadcastSuccess(uid)
            case <-time.After(100 * time.Millisecond):
                ebss.metricsCollector.RecordBroadcastTimeout(uid)
            }
        }(userID)
    }
    
    wg.Wait()
    return nil
}

// 【実用例】協調編集システム
func (ebss *EnterpriseBidirectionalStreamingSystem) HandleCollaborativeEditing(
    stream pb.EditorService_EditorServer,
) error {
    
    clientID, err := ebss.authManager.AuthenticateStream(stream)
    if err != nil {
        return status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
    }
    
    connection := ebss.connectionManager.CreateConnection(clientID, stream)
    defer ebss.connectionManager.RemoveConnection(clientID)
    
    for {
        edit, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        
        // 【重要】競合解決システム
        resolvedEdit, err := ebss.conflictResolver.ResolveConflict(edit)
        if err != nil {
            return fmt.Errorf("conflict resolution failed: %w", err)
        }
        
        // 【重要】トランザクション管理
        tx, err := ebss.transactionManager.BeginTransaction()
        if err != nil {
            return fmt.Errorf("transaction failed: %w", err)
        }
        
        // ドキュメント更新
        document, err := ebss.applyEditWithLocking(resolvedEdit, tx)
        if err != nil {
            tx.Rollback()
            return fmt.Errorf("edit application failed: %w", err)
        }
        
        // コミット
        if err := tx.Commit(); err != nil {
            return fmt.Errorf("commit failed: %w", err)
        }
        
        // 他の編集者に変更通知
        ebss.notifyCollaborators(document.ID, resolvedEdit, clientID)
    }
}
```

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