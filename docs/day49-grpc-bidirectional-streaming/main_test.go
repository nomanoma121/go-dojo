package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBidirectionalServer_Chat_BasicMessaging(t *testing.T) {
	server := NewBidirectionalServer()
	
	// 2つのクライアントを作成
	client1 := NewBidirectionalClient("user1", server)
	client2 := NewBidirectionalClient("user2", server)
	
	// チャットを開始（バックグラウンドで実行）
	go client1.StartChat()
	go client2.StartChat()
	
	// 少し待機してクライアントが接続されるまで待つ
	time.Sleep(100 * time.Millisecond)
	
	// user1がメッセージを送信
	client1.SendMessage("Hello, user2!")
	
	// user2がメッセージを受信できるまで待機
	time.Sleep(100 * time.Millisecond)
	
	// user2の受信メッセージを確認
	messages := client2.GetReceivedMessages()
	
	foundMessage := false
	foundJoinMessage := false
	
	for _, msg := range messages {
		if chatMsg, ok := msg.(*ChatMessage); ok {
			if chatMsg.Content == "Hello, user2!" && chatMsg.UserID == "user1" {
				foundMessage = true
			}
			if strings.Contains(chatMsg.Content, "has joined the chat") {
				foundJoinMessage = true
			}
		}
	}
	
	if !foundMessage {
		t.Error("user2 did not receive message from user1")
	}
	
	if !foundJoinMessage {
		t.Error("Join message was not received")
	}
	
	// 接続中ユーザーの確認
	users := server.GetConnectedUsers()
	if len(users) != 2 {
		t.Errorf("Expected 2 connected users, got %d", len(users))
	}
	
	client1.Close()
	client2.Close()
	
	t.Logf("Successfully tested basic chat messaging with %d users", len(users))
}

func TestBidirectionalServer_Chat_MultipleUsers(t *testing.T) {
	server := NewBidirectionalServer()
	
	// 5つのクライアントを作成
	clients := make([]*BidirectionalClient, 5)
	for i := 0; i < 5; i++ {
		userID := fmt.Sprintf("user%d", i+1)
		clients[i] = NewBidirectionalClient(userID, server)
		go clients[i].StartChat()
	}
	
	// 接続が完了するまで待機
	time.Sleep(200 * time.Millisecond)
	
	// user1がメッセージを送信
	clients[0].SendMessage("Hello, everyone!")
	
	// メッセージが配信されるまで待機
	time.Sleep(100 * time.Millisecond)
	
	// 他の全ユーザーがメッセージを受信したことを確認
	for i := 1; i < 5; i++ {
		messages := clients[i].GetReceivedMessages()
		foundMessage := false
		
		for _, msg := range messages {
			if chatMsg, ok := msg.(*ChatMessage); ok {
				if chatMsg.Content == "Hello, everyone!" && chatMsg.UserID == "user1" {
					foundMessage = true
					break
				}
			}
		}
		
		if !foundMessage {
			t.Errorf("user%d did not receive broadcast message", i+1)
		}
	}
	
	// 全クライアントを閉じる
	for _, client := range clients {
		client.Close()
	}
	
	t.Log("Successfully tested multi-user chat broadcasting")
}

func TestBidirectionalServer_GameSync_PlayerStates(t *testing.T) {
	server := NewBidirectionalServer()
	
	// 3つのゲームクライアントを作成
	player1 := NewBidirectionalClient("player1", server)
	player2 := NewBidirectionalClient("player2", server)
	player3 := NewBidirectionalClient("player3", server)
	
	// ゲーム同期を開始
	go player1.StartGameSync()
	go player2.StartGameSync()
	go player3.StartGameSync()
	
	// 接続が完了するまで待機
	time.Sleep(100 * time.Millisecond)
	
	// player1の状態を更新
	player1.SendGameState(100.0, 200.0, "move", 100)
	
	// 状態が同期されるまで待機
	time.Sleep(100 * time.Millisecond)
	
	// player2とplayer3が状態を受信したことを確認
	for _, player := range []*BidirectionalClient{player2, player3} {
		messages := player.GetReceivedMessages()
		foundState := false
		
		for _, msg := range messages {
			if gameState, ok := msg.(*GameState); ok {
				if gameState.PlayerID == "player1" && 
				   gameState.X == 100.0 && 
				   gameState.Y == 200.0 &&
				   gameState.Action == "move" &&
				   gameState.Health == 100 {
					foundState = true
					break
				}
			}
		}
		
		if !foundState {
			t.Errorf("%s did not receive player1's game state", player.userID)
		}
	}
	
	player1.Close()
	player2.Close()
	player3.Close()
	
	t.Log("Successfully tested game state synchronization")
}

func TestBidirectionalServer_CollaborativeEditing_DocumentOperations(t *testing.T) {
	server := NewBidirectionalServer()
	
	// 3つの協調編集クライアントを作成
	editor1 := NewBidirectionalClient("editor1", server)
	editor2 := NewBidirectionalClient("editor2", server)
	editor3 := NewBidirectionalClient("editor3", server)
	
	// 協調編集を開始
	go editor1.StartCollaboration()
	go editor2.StartCollaboration()
	go editor3.StartCollaboration()
	
	// 接続が完了するまで待機
	time.Sleep(100 * time.Millisecond)
	
	documentID := "doc1"
	
	// editor1がテキストを挿入
	editor1.SendDocumentOperation(documentID, "insert", 0, "Hello")
	
	// 操作が同期されるまで待機
	time.Sleep(100 * time.Millisecond)
	
	// ドキュメントの内容を確認
	content := server.GetDocument(documentID)
	if content != "Hello" {
		t.Errorf("Expected document content 'Hello', got '%s'", content)
	}
	
	// editor2がテキストを追加
	editor2.SendDocumentOperation(documentID, "insert", 5, ", World!")
	
	time.Sleep(100 * time.Millisecond)
	
	// 更新されたドキュメントの内容を確認
	content = server.GetDocument(documentID)
	if content != "Hello, World!" {
		t.Errorf("Expected document content 'Hello, World!', got '%s'", content)
	}
	
	// editor3がテキストを削除
	editor3.SendDocumentOperation(documentID, "delete", 5, ", World")
	
	time.Sleep(100 * time.Millisecond)
	
	// 削除後のドキュメントの内容を確認
	content = server.GetDocument(documentID)
	if content != "Hello!" {
		t.Errorf("Expected document content 'Hello!', got '%s'", content)
	}
	
	// 他のエディターが操作を受信したことを確認
	for _, editor := range []*BidirectionalClient{editor2, editor3} {
		messages := editor.GetReceivedMessages()
		receivedOperations := 0
		
		for _, msg := range messages {
			if _, ok := msg.(*CollaborativeDocument); ok {
				receivedOperations++
			}
		}
		
		if receivedOperations == 0 {
			t.Errorf("%s did not receive any document operations", editor.userID)
		}
	}
	
	editor1.Close()
	editor2.Close()
	editor3.Close()
	
	t.Logf("Successfully tested collaborative editing with final document: '%s'", content)
}

func TestBidirectionalServer_ConcurrentOperations(t *testing.T) {
	server := NewBidirectionalServer()
	
	// 複数のクライアントを同時に作成・接続
	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalMessages int
	
	numClients := 10
	clients := make([]*BidirectionalClient, numClients)
	
	// 全クライアントを並行して開始
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			userID := fmt.Sprintf("user%d", id)
			client := NewBidirectionalClient(userID, server)
			clients[id] = client
			
			// チャットを開始
			go client.StartChat()
			
			// 少し待機してから複数のメッセージを送信
			time.Sleep(50 * time.Millisecond)
			
			for j := 0; j < 3; j++ {
				message := fmt.Sprintf("Message %d from %s", j+1, userID)
				client.SendMessage(message)
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}
	
	wg.Wait()
	
	// メッセージが配信されるまで待機
	time.Sleep(500 * time.Millisecond)
	
	// 各クライアントが受信したメッセージ数をカウント
	for i, client := range clients {
		if client != nil {
			messages := client.GetReceivedMessages()
			mu.Lock()
			totalMessages += len(messages)
			mu.Unlock()
			
			t.Logf("Client %d received %d messages", i, len(messages))
		}
	}
	
	// 接続されたユーザー数を確認
	connectedUsers := server.GetConnectedUsers()
	if len(connectedUsers) != numClients {
		t.Errorf("Expected %d connected users, got %d", numClients, len(connectedUsers))
	}
	
	// 全クライアントを閉じる
	for _, client := range clients {
		if client != nil {
			client.Close()
		}
	}
	
	t.Logf("Concurrent test completed: %d total messages received across all clients", totalMessages)
}

func TestBidirectionalServer_ErrorHandling(t *testing.T) {
	server := NewBidirectionalServer()
	
	// クライアントを作成してすぐに閉じる
	client := NewBidirectionalClient("test_user", server)
	
	// チャットを開始
	go client.StartChat()
	
	// 少し待機
	time.Sleep(50 * time.Millisecond)
	
	// メッセージを送信
	client.SendMessage("Test message")
	
	// すぐにクライアントを閉じる
	client.Close()
	
	// サーバーが適切にクリーンアップされることを確認
	time.Sleep(100 * time.Millisecond)
	
	users := server.GetConnectedUsers()
	if len(users) != 0 {
		t.Errorf("Expected 0 connected users after client close, got %d", len(users))
	}
	
	t.Log("Successfully tested error handling and cleanup")
}

func TestDocumentOperations_EdgeCases(t *testing.T) {
	server := NewBidirectionalServer()
	documentID := "test_doc"
	
	// 空のドキュメントから開始
	content := server.GetDocument(documentID)
	if content != "" {
		t.Errorf("Expected empty document, got '%s'", content)
	}
	
	// 範囲外の位置への挿入テスト
	doc := &CollaborativeDocument{
		DocumentID: documentID,
		UserID:     "test_user",
		Operation:  "insert",
		Position:   100, // 範囲外
		Content:    "Test",
		Timestamp:  time.Now().Unix(),
	}
	
	err := server.applyDocumentOperation(doc)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// ドキュメントが変更されていないことを確認
	content = server.GetDocument(documentID)
	if content != "" {
		t.Errorf("Expected document to remain empty, got '%s'", content)
	}
	
	// 正常な挿入
	doc.Position = 0
	err = server.applyDocumentOperation(doc)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	content = server.GetDocument(documentID)
	if content != "Test" {
		t.Errorf("Expected 'Test', got '%s'", content)
	}
	
	// 範囲外の削除テスト
	doc.Operation = "delete"
	doc.Position = 100
	doc.Content = "xyz"
	
	err = server.applyDocumentOperation(doc)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	// ドキュメントが変更されていないことを確認
	content = server.GetDocument(documentID)
	if content != "Test" {
		t.Errorf("Expected 'Test', got '%s'", content)
	}
	
	t.Log("Successfully tested document operation edge cases")
}

// ベンチマークテスト
func BenchmarkBidirectionalServer_MessageBroadcast(b *testing.B) {
	server := NewBidirectionalServer()
	
	// 複数のクライアントを作成
	numClients := 100
	clients := make([]*BidirectionalClient, numClients)
	
	for i := 0; i < numClients; i++ {
		userID := fmt.Sprintf("user%d", i)
		clients[i] = NewBidirectionalClient(userID, server)
		go clients[i].StartChat()
	}
	
	// 接続が完了するまで待機
	time.Sleep(100 * time.Millisecond)
	
	// ベンチマーク開始
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		message := fmt.Sprintf("Benchmark message %d", i)
		clients[0].SendMessage(message)
	}
	
	// クリーンアップ
	for _, client := range clients {
		client.Close()
	}
}

func BenchmarkBidirectionalServer_DocumentOperations(b *testing.B) {
	server := NewBidirectionalServer()
	documentID := "benchmark_doc"
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		doc := &CollaborativeDocument{
			DocumentID: documentID,
			UserID:     "benchmark_user",
			Operation:  "insert",
			Position:   0,
			Content:    fmt.Sprintf("Text%d", i),
			Timestamp:  time.Now().Unix(),
		}
		
		err := server.applyDocumentOperation(doc)
		if err != nil {
			b.Fatal(err)
		}
	}
}