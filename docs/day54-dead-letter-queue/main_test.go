package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDeadLetterQueue_BasicOperations(t *testing.T) {
	strategy := &ExponentialBackoffReprocessing{
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		MaxAttempts: 3,
		Multiplier:  2.0,
	}
	dlq := NewDeadLetterQueue(strategy)
	
	// メッセージをDLQに送信
	dlqMessage := &DLQMessage{
		OriginalMessage: &Message{
			ID:    "test-msg-1",
			Topic: "test-topic",
			Data:  []byte("test data"),
		},
		FailureReason: "test error",
		ErrorClass:    TemporaryError,
		FailureCount:  1,
		FirstFailure:  time.Now(),
		LastFailure:   time.Now(),
	}
	
	ctx := context.Background()
	err := dlq.Send(ctx, dlqMessage)
	if err != nil {
		t.Errorf("Failed to send message to DLQ: %v", err)
	}
	
	// メッセージを取得
	retrieved, found := dlq.GetMessage("test-msg-1")
	if !found {
		t.Error("Message not found in DLQ")
	}
	
	if retrieved.OriginalMessage.ID != "test-msg-1" {
		t.Errorf("Retrieved message ID mismatch: expected test-msg-1, got %s", 
			retrieved.OriginalMessage.ID)
	}
	
	// メッセージを削除
	err = dlq.RemoveMessage("test-msg-1")
	if err != nil {
		t.Errorf("Failed to remove message from DLQ: %v", err)
	}
	
	// 削除されたメッセージは見つからないはず
	_, found = dlq.GetMessage("test-msg-1")
	if found {
		t.Error("Message should be removed from DLQ")
	}
	
	t.Log("DLQ basic operations working correctly")
}

func TestDeadLetterQueue_ErrorClassification(t *testing.T) {
	tests := []struct {
		errorMsg string
		expected ErrorClassification
	}{
		{"context deadline exceeded", TimeoutError},
		{"validation failed: invalid email", ValidationError},
		{"unauthorized access attempt", SecurityError},
		{"connection refused", TemporaryError},
		{"unknown error", PermanentError},
	}
	
	for _, tt := range tests {
		t.Run(tt.errorMsg, func(t *testing.T) {
			err := fmt.Errorf(tt.errorMsg)
			classification := ClassifyError(err)
			
			if classification != tt.expected {
				t.Errorf("Expected classification %v, got %v for error: %s", 
					tt.expected, classification, tt.errorMsg)
			}
		})
	}
	
	t.Log("Error classification working correctly")
}

func TestDeadLetterQueue_Reprocessing(t *testing.T) {
	strategy := &ExponentialBackoffReprocessing{
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		MaxAttempts: 3,
		Multiplier:  2.0,
	}
	dlq := NewDeadLetterQueue(strategy)
	publisher := NewSimplePublisher()
	reprocessor := NewBatchReprocessor(dlq, publisher, 5, strategy)
	
	// 再処理可能なメッセージをDLQに追加
	reprocessableMsg := &DLQMessage{
		OriginalMessage: &Message{
			ID:    "reprocess-msg-1",
			Topic: "test-topic",
			Data:  []byte("reprocess data"),
		},
		FailureReason: "temporary error",
		ErrorClass:    TemporaryError,
		FailureCount:  1,
		FirstFailure:  time.Now().Add(-1 * time.Hour),
		LastFailure:   time.Now().Add(-1 * time.Hour),
	}
	
	// 再処理不可能なメッセージをDLQに追加
	nonReprocessableMsg := &DLQMessage{
		OriginalMessage: &Message{
			ID:    "permanent-msg-1",
			Topic: "test-topic",
			Data:  []byte("permanent data"),
		},
		FailureReason: "permanent error",
		ErrorClass:    PermanentError,
		FailureCount:  1,
		FirstFailure:  time.Now(),
		LastFailure:   time.Now(),
	}
	
	ctx := context.Background()
	dlq.Send(ctx, reprocessableMsg)
	dlq.Send(ctx, nonReprocessableMsg)
	
	// 再処理フィルター
	filter := func(dlqMsg *DLQMessage) bool {
		return strategy.ShouldReprocess(dlqMsg)
	}
	
	// バッチ再処理を実行
	err := reprocessor.ReprocessBatch(ctx, filter)
	if err != nil {
		t.Errorf("Reprocessing failed: %v", err)
	}
	
	// 再処理されたメッセージが発行されているかチェック
	publishedMessages := publisher.GetPublishedMessages()
	if len(publishedMessages) != 1 {
		t.Errorf("Expected 1 republished message, got %d", len(publishedMessages))
	}
	
	if publishedMessages[0].Message.ID != "reprocess-msg-1" {
		t.Errorf("Expected republished message ID reprocess-msg-1, got %s", 
			publishedMessages[0].Message.ID)
	}
	
	// 再処理されたメッセージがDLQから削除されているかチェック
	_, found := dlq.GetMessage("reprocess-msg-1")
	if found {
		t.Error("Reprocessed message should be removed from DLQ")
	}
	
	// 再処理不可能なメッセージがDLQに残っているかチェック
	_, found = dlq.GetMessage("permanent-msg-1")
	if !found {
		t.Error("Non-reprocessable message should remain in DLQ")
	}
	
	t.Log("Message reprocessing working correctly")
}

func TestDeadLetterQueue_Analytics(t *testing.T) {
	strategy := &ExponentialBackoffReprocessing{
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		MaxAttempts: 3,
		Multiplier:  2.0,
	}
	dlq := NewDeadLetterQueue(strategy)
	
	// 異なるタイプのメッセージを追加
	messages := []*DLQMessage{
		{
			OriginalMessage: &Message{ID: "msg-1", Topic: "topic-a"},
			ErrorClass:      TemporaryError,
			FailureCount:    1,
			FirstFailure:    time.Now().Add(-2 * time.Hour),
		},
		{
			OriginalMessage: &Message{ID: "msg-2", Topic: "topic-a"},
			ErrorClass:      ValidationError,
			FailureCount:    2,
			FirstFailure:    time.Now().Add(-1 * time.Hour),
		},
		{
			OriginalMessage: &Message{ID: "msg-3", Topic: "topic-b"},
			ErrorClass:      SecurityError,
			FailureCount:    1,
			FirstFailure:    time.Now().Add(-30 * time.Minute),
		},
		{
			OriginalMessage: &Message{ID: "msg-4", Topic: "topic-b"},
			ErrorClass:      TemporaryError,
			FailureCount:    3,
			FirstFailure:    time.Now().Add(-3 * time.Hour),
		},
	}
	
	ctx := context.Background()
	for _, msg := range messages {
		dlq.Send(ctx, msg)
	}
	
	// 分析データを取得
	analytics := dlq.GetAnalytics()
	
	// 基本統計をチェック
	if analytics.TotalMessages != 4 {
		t.Errorf("Expected 4 total messages, got %d", analytics.TotalMessages)
	}
	
	// エラー分類別統計をチェック
	if analytics.ErrorBreakdown[TemporaryError] != 2 {
		t.Errorf("Expected 2 temporary errors, got %d", 
			analytics.ErrorBreakdown[TemporaryError])
	}
	
	if analytics.ErrorBreakdown[ValidationError] != 1 {
		t.Errorf("Expected 1 validation error, got %d", 
			analytics.ErrorBreakdown[ValidationError])
	}
	
	if analytics.ErrorBreakdown[SecurityError] != 1 {
		t.Errorf("Expected 1 security error, got %d", 
			analytics.ErrorBreakdown[SecurityError])
	}
	
	// トピック別統計をチェック
	if analytics.TopicBreakdown["topic-a"] != 2 {
		t.Errorf("Expected 2 messages for topic-a, got %d", 
			analytics.TopicBreakdown["topic-a"])
	}
	
	if analytics.TopicBreakdown["topic-b"] != 2 {
		t.Errorf("Expected 2 messages for topic-b, got %d", 
			analytics.TopicBreakdown["topic-b"])
	}
	
	// 平均再試行回数をチェック
	expectedAvg := float64(1+2+1+3) / 4.0
	if analytics.AverageRetries != expectedAvg {
		t.Errorf("Expected average retries %.2f, got %.2f", 
			expectedAvg, analytics.AverageRetries)
	}
	
	// 最古メッセージをチェック
	if analytics.OldestMessage == nil {
		t.Error("OldestMessage should not be nil")
	}
	
	t.Log("DLQ analytics working correctly")
}

func TestExponentialBackoffReprocessing(t *testing.T) {
	strategy := &ExponentialBackoffReprocessing{
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		MaxAttempts: 3,
		Multiplier:  2.0,
	}
	
	// 再処理可能なケース
	reprocessableMsg := &DLQMessage{
		ErrorClass:   TemporaryError,
		FailureCount: 2,
		LastFailure:  time.Now().Add(-1 * time.Hour),
	}
	
	if !strategy.ShouldReprocess(reprocessableMsg) {
		t.Error("Should be able to reprocess temporary error within max attempts")
	}
	
	// 再処理不可能なケース（最大試行回数超過）
	maxAttemptsMsg := &DLQMessage{
		ErrorClass:   TemporaryError,
		FailureCount: 3,
		LastFailure:  time.Now().Add(-1 * time.Hour),
	}
	
	if strategy.ShouldReprocess(maxAttemptsMsg) {
		t.Error("Should not reprocess message that exceeded max attempts")
	}
	
	// 再処理不可能なケース（永続的エラー）
	permanentMsg := &DLQMessage{
		ErrorClass:   PermanentError,
		FailureCount: 1,
		LastFailure:  time.Now().Add(-1 * time.Hour),
	}
	
	if strategy.ShouldReprocess(permanentMsg) {
		t.Error("Should not reprocess permanent error")
	}
	
	// 遅延時間の計算をテスト
	expectedDelays := []time.Duration{
		10 * time.Millisecond,  // 2^0 * 10ms
		20 * time.Millisecond,  // 2^1 * 10ms
		40 * time.Millisecond,  // 2^2 * 10ms
		80 * time.Millisecond,  // 2^3 * 10ms
		100 * time.Millisecond, // capped at MaxDelay
	}
	
	for i, expected := range expectedDelays {
		testMsg := &DLQMessage{
			FailureCount: i,
			LastFailure:  time.Now(),
		}
		
		nextTime := strategy.NextAttemptTime(testMsg)
		actualDelay := nextTime.Sub(testMsg.LastFailure)
		
		if actualDelay < expected*9/10 || actualDelay > expected*11/10 {
			t.Errorf("Attempt %d: expected delay ~%v, got %v", 
				i, expected, actualDelay)
		}
	}
	
	t.Log("Exponential backoff strategy working correctly")
}

func TestDLQMonitor_Alerting(t *testing.T) {
	strategy := &ExponentialBackoffReprocessing{
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		MaxAttempts: 3,
		Multiplier:  2.0,
	}
	dlq := NewDeadLetterQueue(strategy)
	alerting := NewSimpleAlertingService()
	
	config := MonitorConfig{
		MaxMessages:     2,
		MaxMessageAge:   1 * time.Hour,
		MaxSecurityErrs: 1,
		CheckInterval:   100 * time.Millisecond,
	}
	
	monitor := NewDLQMonitor(dlq, alerting, config)
	
	// 閾値を超えるメッセージを追加
	messages := []*DLQMessage{
		{
			OriginalMessage: &Message{ID: "msg-1", Topic: "test"},
			ErrorClass:      TemporaryError,
			FailureCount:    1,
			FirstFailure:    time.Now().Add(-2 * time.Hour), // 古いメッセージ
		},
		{
			OriginalMessage: &Message{ID: "msg-2", Topic: "test"},
			ErrorClass:      SecurityError,
			FailureCount:    1,
			FirstFailure:    time.Now(),
		},
		{
			OriginalMessage: &Message{ID: "msg-3", Topic: "test"},
			ErrorClass:      SecurityError,
			FailureCount:    1,
			FirstFailure:    time.Now(),
		},
	}
	
	ctx := context.Background()
	for _, msg := range messages {
		dlq.Send(ctx, msg)
	}
	
	// 短時間監視を実行
	monitorCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	
	go monitor.StartMonitoring(monitorCtx)
	
	// 監視処理を待つ
	time.Sleep(300 * time.Millisecond)
	
	// アラートをチェック
	alerts := alerting.GetAlerts()
	if len(alerts) == 0 {
		t.Error("Expected alerts to be generated")
	}
	
	// アラートタイプをチェック
	var hasHighVolumeAlert, hasOldMessageAlert, hasSecurityAlert bool
	
	for _, alert := range alerts {
		switch {
		case strings.Contains(alert.Type, "HIGH_VOLUME"):
			hasHighVolumeAlert = true
		case strings.Contains(alert.Type, "OLD_MESSAGES"):
			hasOldMessageAlert = true
		case strings.Contains(alert.Type, "SECURITY"):
			hasSecurityAlert = true
		}
	}
	
	if !hasHighVolumeAlert {
		t.Error("Expected high volume alert")
	}
	
	if !hasOldMessageAlert {
		t.Error("Expected old message alert")
	}
	
	if !hasSecurityAlert {
		t.Error("Expected security alert")
	}
	
	t.Log("DLQ monitoring and alerting working correctly")
}

func TestSimplePublisher(t *testing.T) {
	publisher := NewSimplePublisher()
	
	message := &Message{
		ID:    "test-msg",
		Topic: "test-topic",
		Data:  []byte("test data"),
	}
	
	ctx := context.Background()
	err := publisher.Publish(ctx, "test-topic", message)
	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}
	
	published := publisher.GetPublishedMessages()
	if len(published) != 1 {
		t.Errorf("Expected 1 published message, got %d", len(published))
	}
	
	if published[0].Message.ID != "test-msg" {
		t.Errorf("Published message ID mismatch: expected test-msg, got %s", 
			published[0].Message.ID)
	}
	
	if published[0].Topic != "test-topic" {
		t.Errorf("Published topic mismatch: expected test-topic, got %s", 
			published[0].Topic)
	}
	
	t.Log("Simple publisher working correctly")
}

func TestSimpleAlertingService(t *testing.T) {
	alerting := NewSimpleAlertingService()
	
	// 各種アラートを送信
	err := alerting.SendWarningAlert("TEST_WARNING", "This is a warning")
	if err != nil {
		t.Errorf("SendWarningAlert failed: %v", err)
	}
	
	err = alerting.SendCriticalAlert("TEST_CRITICAL", "This is critical")
	if err != nil {
		t.Errorf("SendCriticalAlert failed: %v", err)
	}
	
	err = alerting.SendSecurityAlert("TEST_SECURITY", "This is a security alert")
	if err != nil {
		t.Errorf("SendSecurityAlert failed: %v", err)
	}
	
	// アラートを取得
	alerts := alerting.GetAlerts()
	if len(alerts) != 3 {
		t.Errorf("Expected 3 alerts, got %d", len(alerts))
	}
	
	// アラートレベルをチェック
	expectedLevels := map[string]string{
		"TEST_WARNING":  "warning",
		"TEST_CRITICAL": "critical",
		"TEST_SECURITY": "security",
	}
	
	for _, alert := range alerts {
		expectedLevel, exists := expectedLevels[alert.Type]
		if !exists {
			t.Errorf("Unexpected alert type: %s", alert.Type)
			continue
		}
		
		if alert.Level != expectedLevel {
			t.Errorf("Alert %s: expected level %s, got %s", 
				alert.Type, expectedLevel, alert.Level)
		}
	}
	
	t.Log("Simple alerting service working correctly")
}

// ベンチマークテスト
func BenchmarkDeadLetterQueue_Send(b *testing.B) {
	strategy := &ExponentialBackoffReprocessing{
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		MaxAttempts: 3,
		Multiplier:  2.0,
	}
	dlq := NewDeadLetterQueue(strategy)
	
	ctx := context.Background()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			dlqMessage := &DLQMessage{
				OriginalMessage: &Message{
					ID:    fmt.Sprintf("bench-msg-%d", i),
					Topic: "bench-topic",
					Data:  []byte("bench data"),
				},
				FailureReason: "bench error",
				ErrorClass:    TemporaryError,
				FailureCount:  1,
				FirstFailure:  time.Now(),
				LastFailure:   time.Now(),
			}
			
			dlq.Send(ctx, dlqMessage)
			i++
		}
	})
}

func BenchmarkDeadLetterQueue_GetAnalytics(b *testing.B) {
	strategy := &ExponentialBackoffReprocessing{
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		MaxAttempts: 3,
		Multiplier:  2.0,
	}
	dlq := NewDeadLetterQueue(strategy)
	
	// テストデータを準備
	ctx := context.Background()
	for i := 0; i < 1000; i++ {
		dlqMessage := &DLQMessage{
			OriginalMessage: &Message{
				ID:    fmt.Sprintf("msg-%d", i),
				Topic: fmt.Sprintf("topic-%d", i%10),
			},
			ErrorClass:   ErrorClassification([]ErrorClassification{TemporaryError, PermanentError, ValidationError}[i%3]),
			FailureCount: i%5 + 1,
			FirstFailure: time.Now().Add(-time.Duration(i) * time.Minute),
		}
		dlq.Send(ctx, dlqMessage)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dlq.GetAnalytics()
	}
}