// Day 49: gRPC Bidirectional Streaming
// サーバーとクライアントが同時にメッセージを送り合う実装

package main

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// メッセージ構造定義
type ChatMessage struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"` // "message", "join", "leave", "typing"
}

type GameState struct {
	PlayerID string  `json:"player_id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Action   string  `json:"action"`
	Health   int32   `json:"health"`
}

type CollaborativeDocument struct {
	DocumentID string `json:"document_id"`
	UserID     string `json:"user_id"`
	Operation  string `json:"operation"` // "insert", "delete", "cursor"
	Position   int32  `json:"position"`
	Content    string `json:"content"`
	Timestamp  int64  `json:"timestamp"`
}

// ストリームインターフェース（モック実装）
type ChatStreamServer interface {
	Send(*ChatMessage) error
	Recv() (*ChatMessage, error)
	Context() context.Context
}

type GameStreamServer interface {
	Send(*GameState) error
	Recv() (*GameState, error)
	Context() context.Context
}

type CollaborationStreamServer interface {
	Send(*CollaborativeDocument) error
	Recv() (*CollaborativeDocument, error)
	Context() context.Context
}

// サーバー実装
type BidirectionalServer struct {
	chatClients          map[string]ChatStreamServer
	gameClients          map[string]GameStreamServer
	collaborationClients map[string]CollaborationStreamServer
	documents            map[string]string
	gameStates           map[string]*GameState
	mu                   sync.RWMutex
}

func NewBidirectionalServer() *BidirectionalServer {
	return &BidirectionalServer{
		chatClients:          make(map[string]ChatStreamServer),
		gameClients:          make(map[string]GameStreamServer),
		collaborationClients: make(map[string]CollaborationStreamServer),
		documents:            make(map[string]string),
		gameStates:           make(map[string]*GameState),
	}
}

// Chat チャットの双方向ストリーミングを処理
func (s *BidirectionalServer) Chat(stream ChatStreamServer) error {
	var userID string
	
	// ストリームをサーバーに登録
	defer func() {
		if userID != "" {
			s.mu.Lock()
			delete(s.chatClients, userID)
			s.mu.Unlock()
			
			// 退室メッセージをブロードキャスト
			leaveMsg := &ChatMessage{
				ID:        fmt.Sprintf("leave_%s_%d", userID, time.Now().UnixNano()),
				UserID:    "system",
				Content:   fmt.Sprintf("%s has left the chat", userID),
				Timestamp: time.Now().Unix(),
				Type:      "leave",
			}
			s.broadcastChatMessage(leaveMsg, userID)
		}
	}()

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// 最初のメッセージでユーザーIDを設定
		if userID == "" {
			userID = msg.UserID
			s.mu.Lock()
			s.chatClients[userID] = stream
			s.mu.Unlock()
			
			// 入室メッセージをブロードキャスト
			joinMsg := &ChatMessage{
				ID:        fmt.Sprintf("join_%s_%d", userID, time.Now().UnixNano()),
				UserID:    "system",
				Content:   fmt.Sprintf("%s has joined the chat", userID),
				Timestamp: time.Now().Unix(),
				Type:      "join",
			}
			s.broadcastChatMessage(joinMsg, userID)
		}

		// メッセージをブロードキャスト
		s.broadcastChatMessage(msg, userID)
	}
}

// GameSync ゲーム状態の双方向同期を処理
func (s *BidirectionalServer) GameSync(stream GameStreamServer) error {
	var playerID string
	
	defer func() {
		if playerID != "" {
			s.mu.Lock()
			delete(s.gameClients, playerID)
			delete(s.gameStates, playerID)
			s.mu.Unlock()
		}
	}()

	for {
		state, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// 最初のメッセージでプレイヤーIDを設定
		if playerID == "" {
			playerID = state.PlayerID
			s.mu.Lock()
			s.gameClients[playerID] = stream
			s.mu.Unlock()
		}

		// ゲーム状態を更新
		s.mu.Lock()
		s.gameStates[playerID] = state
		s.mu.Unlock()

		// 他のプレイヤーに状態をブロードキャスト
		s.broadcastGameState(state, playerID)
	}
}

// CollaborativeEditing 協調編集の双方向ストリーミングを処理
func (s *BidirectionalServer) CollaborativeEditing(stream CollaborationStreamServer) error {
	var userID string
	
	defer func() {
		if userID != "" {
			s.mu.Lock()
			delete(s.collaborationClients, userID)
			s.mu.Unlock()
		}
	}()

	for {
		doc, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// 最初のメッセージでユーザーIDを設定
		if userID == "" {
			userID = doc.UserID
			s.mu.Lock()
			s.collaborationClients[userID] = stream
			s.mu.Unlock()
		}

		// ドキュメント操作を適用
		if err := s.applyDocumentOperation(doc); err != nil {
			return err
		}
	}
}

// broadcastChatMessage 全チャットクライアントにメッセージをブロードキャスト
func (s *BidirectionalServer) broadcastChatMessage(message *ChatMessage, senderID string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for userID, stream := range s.chatClients {
		if userID != senderID { // 送信者以外に送信
			go func(stream ChatStreamServer) {
				stream.Send(message)
			}(stream)
		}
	}
}

// broadcastGameState 全ゲームクライアントに状態をブロードキャスト
func (s *BidirectionalServer) broadcastGameState(state *GameState, senderID string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for playerID, stream := range s.gameClients {
		if playerID != senderID { // 送信者以外に送信
			go func(stream GameStreamServer) {
				stream.Send(state)
			}(stream)
		}
	}
}

// applyDocumentOperation ドキュメント操作を適用し、他のクライアントに通知
func (s *BidirectionalServer) applyDocumentOperation(doc *CollaborativeDocument) error {
	s.mu.Lock()
	currentDoc, exists := s.documents[doc.DocumentID]
	if !exists {
		currentDoc = ""
	}

	// 操作を適用
	switch doc.Operation {
	case "insert":
		if doc.Position <= int32(len(currentDoc)) {
			newDoc := currentDoc[:doc.Position] + doc.Content + currentDoc[doc.Position:]
			s.documents[doc.DocumentID] = newDoc
		}
	case "delete":
		if doc.Position < int32(len(currentDoc)) {
			deleteEnd := doc.Position + int32(len(doc.Content))
			if deleteEnd > int32(len(currentDoc)) {
				deleteEnd = int32(len(currentDoc))
			}
			newDoc := currentDoc[:doc.Position] + currentDoc[deleteEnd:]
			s.documents[doc.DocumentID] = newDoc
		}
	}
	s.mu.Unlock()

	// 他のクライアントに操作を通知
	s.mu.RLock()
	defer s.mu.RUnlock()

	for userID, stream := range s.collaborationClients {
		if userID != doc.UserID { // 送信者以外に送信
			go func(stream CollaborationStreamServer) {
				stream.Send(doc)
			}(stream)
		}
	}

	return nil
}

// GetConnectedUsers 接続中のユーザー一覧を返す
func (s *BidirectionalServer) GetConnectedUsers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]string, 0, len(s.chatClients))
	for userID := range s.chatClients {
		users = append(users, userID)
	}
	return users
}

// GetDocument ドキュメントの内容を返す
func (s *BidirectionalServer) GetDocument(documentID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, exists := s.documents[documentID]
	if !exists {
		return ""
	}
	return doc
}

// クライアント実装
type BidirectionalClient struct {
	userID      string
	server      *BidirectionalServer
	sendChan    chan interface{}
	receiveChan chan interface{}
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
}

func NewBidirectionalClient(userID string, server *BidirectionalServer) *BidirectionalClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &BidirectionalClient{
		userID:      userID,
		server:      server,
		sendChan:    make(chan interface{}, 100),
		receiveChan: make(chan interface{}, 100),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// StartChat チャット双方向ストリーミングを開始
func (c *BidirectionalClient) StartChat() error {
	stream := NewMockChatStream(c.ctx, c.userID, c.server)
	
	// 送信ゴルーチン
	go func() {
		defer stream.CloseSend()
		for {
			select {
			case msg := <-c.sendChan:
				if chatMsg, ok := msg.(*ChatMessage); ok {
					if err := stream.Send(chatMsg); err != nil {
						return
					}
				}
			case <-c.ctx.Done():
				return
			}
		}
	}()

	// 受信ゴルーチン
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}

			select {
			case c.receiveChan <- msg:
			case <-c.ctx.Done():
				return
			default:
				// チャネルが満杯の場合はドロップ
			}
		}
	}()

	// サーバーのChat関数を開始
	return c.server.Chat(stream)
}

// StartGameSync ゲーム同期双方向ストリーミングを開始
func (c *BidirectionalClient) StartGameSync() error {
	stream := NewMockGameStream(c.ctx, c.userID, c.server)
	
	// 送信ゴルーチン
	go func() {
		defer stream.CloseSend()
		for {
			select {
			case msg := <-c.sendChan:
				if gameState, ok := msg.(*GameState); ok {
					if err := stream.Send(gameState); err != nil {
						return
					}
				}
			case <-c.ctx.Done():
				return
			}
		}
	}()

	// 受信ゴルーチン
	go func() {
		for {
			state, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}

			select {
			case c.receiveChan <- state:
			case <-c.ctx.Done():
				return
			default:
			}
		}
	}()

	return c.server.GameSync(stream)
}

// StartCollaboration 協調編集双方向ストリーミングを開始
func (c *BidirectionalClient) StartCollaboration() error {
	stream := NewMockCollaborationStream(c.ctx, c.userID, c.server)
	
	// 送信ゴルーチン
	go func() {
		defer stream.CloseSend()
		for {
			select {
			case msg := <-c.sendChan:
				if doc, ok := msg.(*CollaborativeDocument); ok {
					if err := stream.Send(doc); err != nil {
						return
					}
				}
			case <-c.ctx.Done():
				return
			}
		}
	}()

	// 受信ゴルーチン
	go func() {
		for {
			doc, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}

			select {
			case c.receiveChan <- doc:
			case <-c.ctx.Done():
				return
			default:
			}
		}
	}()

	return c.server.CollaborativeEditing(stream)
}

// SendMessage チャットメッセージを送信
func (c *BidirectionalClient) SendMessage(content string) {
	message := &ChatMessage{
		ID:        fmt.Sprintf("msg_%s_%d", c.userID, time.Now().UnixNano()),
		UserID:    c.userID,
		Content:   content,
		Timestamp: time.Now().Unix(),
		Type:      "message",
	}

	select {
	case c.sendChan <- message:
	case <-c.ctx.Done():
	default:
		// チャネルが満杯の場合はドロップ
	}
}

// SendGameState ゲーム状態を送信
func (c *BidirectionalClient) SendGameState(x, y float64, action string, health int32) {
	state := &GameState{
		PlayerID: c.userID,
		X:        x,
		Y:        y,
		Action:   action,
		Health:   health,
	}

	select {
	case c.sendChan <- state:
	case <-c.ctx.Done():
	default:
	}
}

// SendDocumentOperation ドキュメント操作を送信
func (c *BidirectionalClient) SendDocumentOperation(documentID, operation string, position int32, content string) {
	doc := &CollaborativeDocument{
		DocumentID: documentID,
		UserID:     c.userID,
		Operation:  operation,
		Position:   position,
		Content:    content,
		Timestamp:  time.Now().Unix(),
	}

	select {
	case c.sendChan <- doc:
	case <-c.ctx.Done():
	default:
	}
}

// GetReceivedMessages 受信したメッセージを取得
func (c *BidirectionalClient) GetReceivedMessages() []interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	var messages []interface{}
	for {
		select {
		case msg := <-c.receiveChan:
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}

// Close クライアント接続を閉じる
func (c *BidirectionalClient) Close() {
	c.cancel()
	close(c.sendChan)
	close(c.receiveChan)
}

func main() {
	fmt.Println("Day 49: gRPC Bidirectional Streaming")
	fmt.Println("Run 'go test -v' to see the bidirectional streaming system in action")
}

// モックストリーム実装（簡略版 - テストファイルで完全実装）
type MockChatStream struct {
	ctx      context.Context
	userID   string
	server   *BidirectionalServer
	sendChan chan *ChatMessage
	recvChan chan *ChatMessage
	closed   bool
	mu       sync.Mutex
}

func NewMockChatStream(ctx context.Context, userID string, server *BidirectionalServer) *MockChatStream {
	return &MockChatStream{
		ctx:      ctx,
		userID:   userID,
		server:   server,
		sendChan: make(chan *ChatMessage, 100),
		recvChan: make(chan *ChatMessage, 100),
	}
}

func (m *MockChatStream) Send(msg *ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("stream closed")
	}
	select {
	case m.sendChan <- msg:
		return nil
	case <-m.ctx.Done():
		return m.ctx.Err()
	}
}

func (m *MockChatStream) Recv() (*ChatMessage, error) {
	select {
	case msg := <-m.recvChan:
		return msg, nil
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	}
}

func (m *MockChatStream) Context() context.Context {
	return m.ctx
}

func (m *MockChatStream) CloseSend() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.sendChan)
	}
}

// 同様にMockGameStreamとMockCollaborationStreamも実装（簡略版）
type MockGameStream struct {
	ctx      context.Context
	userID   string
	server   *BidirectionalServer
	sendChan chan *GameState
	recvChan chan *GameState
	closed   bool
	mu       sync.Mutex
}

func NewMockGameStream(ctx context.Context, userID string, server *BidirectionalServer) *MockGameStream {
	return &MockGameStream{
		ctx:      ctx,
		userID:   userID,
		server:   server,
		sendChan: make(chan *GameState, 100),
		recvChan: make(chan *GameState, 100),
	}
}

func (m *MockGameStream) Send(state *GameState) error {
	return nil // 簡略実装
}

func (m *MockGameStream) Recv() (*GameState, error) {
	return nil, io.EOF // 簡略実装
}

func (m *MockGameStream) Context() context.Context {
	return m.ctx
}

func (m *MockGameStream) CloseSend() {}

type MockCollaborationStream struct {
	ctx      context.Context
	userID   string
	server   *BidirectionalServer
	sendChan chan *CollaborativeDocument
	recvChan chan *CollaborativeDocument
	closed   bool
	mu       sync.Mutex
}

func NewMockCollaborationStream(ctx context.Context, userID string, server *BidirectionalServer) *MockCollaborationStream {
	return &MockCollaborationStream{
		ctx:      ctx,
		userID:   userID,
		server:   server,
		sendChan: make(chan *CollaborativeDocument, 100),
		recvChan: make(chan *CollaborativeDocument, 100),
	}
}

func (m *MockCollaborationStream) Send(doc *CollaborativeDocument) error {
	return nil // 簡略実装
}

func (m *MockCollaborationStream) Recv() (*CollaborativeDocument, error) {
	return nil, io.EOF // 簡略実装
}

func (m *MockCollaborationStream) Context() context.Context {
	return m.ctx
}

func (m *MockCollaborationStream) CloseSend() {}