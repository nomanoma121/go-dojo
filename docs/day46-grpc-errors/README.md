# Day 46: gRPC Error Handling

## 🎯 本日の目標 (Today's Goal)

gRPCにおける詳細なエラーハンドリングシステムを実装し、`status`パッケージを使用してクライアントに有用なエラー情報を提供できるようになる。プロダクションレベルのgRPCサービスにおけるエラー処理のベストプラクティスを習得する。

## 📖 解説 (Explanation)

### gRPCエラーハンドリングの重要性

gRPCサービスでは、適切なエラーハンドリングがクライアント体験とデバッグ効率に大きく影響します。標準的なHTTPステータスコードとは異なり、gRPCには独自のステータスコードシステムがあります。

### gRPCステータスコード

gRPCでは以下のような標準ステータスコードが定義されています：

```go
// 【gRPCステータスコード完全解説】プロダクション環境での使い分け
// ❌ 問題例：適切でないステータスコード選択
func badErrorHandling() error {
    // 🚨 災害例：全てのエラーをINTERNALで返す
    if userNotFound {
        return status.Error(codes.Internal, "something went wrong")
        // ❌ クライアントがリトライして負荷増大
        // ❌ エラー原因が不明で修正困難
    }
}

// ✅ 正解：適切なステータスコード選択
const (
    // 【レベル1：クライアントエラー（4xx相当）】
    OK                 = 0  // 成功 - リクエスト完了
    CANCELLED          = 1  // 操作がキャンセルされた - クライアント側のキャンセル
    INVALID_ARGUMENT   = 3  // 無効な引数 - バリデーション失敗
    NOT_FOUND          = 5  // リソースが見つからない - 存在しないリソース
    ALREADY_EXISTS     = 6  // リソースが既に存在 - 重複作成試行
    PERMISSION_DENIED  = 7  // 権限不足 - 認可失敗
    FAILED_PRECONDITION = 9 // 前提条件エラー - 状態不整合
    OUT_OF_RANGE       = 11 // 範囲外 - ページング範囲外
    UNIMPLEMENTED      = 12 // 未実装 - 機能未実装
    UNAUTHENTICATED    = 16 // 認証不備 - 認証失敗
    
    // 【レベル2：サーバーエラー（5xx相当）】
    UNKNOWN            = 2  // 不明なエラー - 予期しないエラー
    DEADLINE_EXCEEDED  = 4  // タイムアウト - 処理時間超過
    RESOURCE_EXHAUSTED = 8  // リソース枯渇 - レート制限等
    ABORTED            = 10 // トランザクションの中止 - 競合状態
    INTERNAL           = 13 // 内部エラー - サーバー内部問題
    UNAVAILABLE        = 14 // サービス利用不可 - サービス停止
    DATA_LOSS          = 15 // データ損失 - データ破損
)

// 【使い分け戦略】業務要件に応じたエラー分類
func properErrorHandling() error {
    // 【クライアントエラー】リトライ不要なエラー
    if userID == "" {
        return status.Error(codes.InvalidArgument, "user_id required")
        // ✅ クライアントがリトライせずにバリデーション修正
    }
    
    // 【サーバーエラー】リトライ可能なエラー
    if databaseDown {
        return status.Error(codes.Unavailable, "database temporarily unavailable")
        // ✅ クライアントが適切にリトライ
    }
    
    // 【リソース制限】レート制限による制御
    if rateLimitExceeded {
        return status.Error(codes.ResourceExhausted, "rate limit exceeded")
        // ✅ クライアントが適切にbackoff
    }
    
    return nil
}
```

### 基本的なエラー作成

```go
import "google.golang.org/grpc/status"
import "google.golang.org/grpc/codes"

// 【エラー作成の基本パターン】プロダクション環境での実装
// ❌ 問題例：不適切なエラーメッセージ
func badErrorCreation() error {
    // 🚨 災害例：セキュリティ情報の漏洩
    return status.Error(codes.Internal, "SQL query failed: SELECT * FROM users WHERE password='secret123'")
    // ❌ 内部実装の詳細を外部に露出
    // ❌ セキュリティリスクの発生
}

// ✅ 正解：適切なエラー作成
func properErrorCreation() error {
    // 【レベル1】シンプルなエラー（軽量で高速）
    if userNotFound {
        return status.Error(codes.NotFound, "user not found")
        // ✅ 必要最小限の情報のみ
        // ✅ 内部実装の詳細を隠蔽
    }
    
    // 【レベル2】詳細なエラー（デバッグ情報付き）
    if validationFailed {
        st := status.New(codes.InvalidArgument, "validation failed")
        
        // 【重要】詳細情報の安全な付与
        st, _ = st.WithDetails(&pb.ValidationError{
            Field:   "email",
            Message: "invalid email format",
        })
        
        return st.Err()
        // ✅ 構造化されたエラー情報
        // ✅ クライアントが機械的に処理可能
    }
    
    // 【レベル3】複数エラーの集約
    if multipleErrors {
        st := status.New(codes.InvalidArgument, "multiple validation errors")
        
        // 【効率的な処理】バッチでエラー詳細を付与
        errorDetails := &pb.ErrorDetails{
            ValidationErrors: []*pb.ValidationError{
                {Field: "name", Message: "cannot be empty"},
                {Field: "email", Message: "invalid format"},
                {Field: "age", Message: "must be positive"},
            },
            RequestId: generateRequestID(),
            Timestamp: time.Now().Unix(),
        }
        
        st, _ = st.WithDetails(errorDetails)
        return st.Err()
        // ✅ 一度のリクエストで全エラーを通知
        // ✅ 追跡可能なリクエストID付き
    }
    
    return nil
}

// 【高度なエラー作成】コンテキスト情報の付与
func contextualErrorCreation(ctx context.Context, userID string) error {
    // 【分散システム対応】トレース情報の伝播
    if traceID := extractTraceID(ctx); traceID != "" {
        st := status.New(codes.Internal, "database connection failed")
        
        // 【運用最適化】エラー追跡情報の追加
        st, _ = st.WithDetails(&pb.ErrorContext{
            TraceId:    traceID,
            UserId:     userID,
            ServiceName: "user-service",
            Version:    "v1.2.3",
            Timestamp:  time.Now().Unix(),
        })
        
        return st.Err()
        // ✅ 分散システムでの問題切り分けが容易
        // ✅ 運用チームが迅速に対応可能
    }
    
    return nil
}
```

### エラーの詳細情報

gRPCでは`google.rpc.Status`メッセージを使用してエラーの詳細情報を付加できます：

```protobuf
syntax = "proto3";

package user;

import "google/rpc/status.proto";

message ValidationError {
  string field = 1;
  string message = 2;
}

message ErrorDetails {
  repeated ValidationError validation_errors = 1;
  string request_id = 2;
  int64 timestamp = 3;
}
```

### サーバーサイドエラーハンドリング

```go
// 【gRPCサーバー実装】プロダクション品質のエラーハンドリング
type UserServiceServer struct {
    pb.UnimplementedUserServiceServer
    users    map[string]*pb.User
    mu       sync.RWMutex
    metrics  *ServiceMetrics
    logger   *log.Logger
}

// 【重要メソッド】ユーザー取得処理
func (s *UserServiceServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    start := time.Now()
    requestID := generateRequestID()
    
    // 【メトリクス記録】処理開始
    s.metrics.RecordRequest("GetUser")
    
    // 【STEP 1】入力検証（最優先で実行）
    if req.UserId == "" {
        s.metrics.RecordError("GetUser", codes.InvalidArgument)
        s.logger.Printf("❌ [%s] Invalid request: user_id empty", requestID)
        
        st := status.New(codes.InvalidArgument, "user_id is required")
        st, _ = st.WithDetails(&pb.ValidationError{
            Field:   "user_id",
            Message: "cannot be empty",
        })
        return nil, st.Err()
        // ✅ 早期バリデーションでリソース節約
        // ✅ 明確なエラーメッセージ
    }
    
    // 【STEP 2】コンテキスト検証（タイムアウト・キャンセル対応）
    select {
    case <-ctx.Done():
        s.metrics.RecordError("GetUser", codes.DeadlineExceeded)
        s.logger.Printf("⏰ [%s] Request cancelled: %v", requestID, ctx.Err())
        return nil, status.Error(codes.DeadlineExceeded, "request timed out")
    default:
        // 処理継続
    }
    
    // 【STEP 3】ユーザー検索（排他制御付き）
    s.mu.RLock()
    user, exists := s.users[req.UserId]
    s.mu.RUnlock()
    
    if !exists {
        s.metrics.RecordError("GetUser", codes.NotFound)
        s.logger.Printf("🔍 [%s] User not found: %s", requestID, req.UserId)
        
        // 【セキュリティ考慮】情報漏洩を防ぐメッセージ
        return nil, status.Error(codes.NotFound, "user not found")
        // ✅ 存在しないユーザーIDの詳細を隠蔽
    }
    
    // 【STEP 4】成功処理
    duration := time.Since(start)
    s.metrics.RecordSuccess("GetUser", duration)
    s.logger.Printf("✅ [%s] User retrieved successfully: %s (took %v)", requestID, req.UserId, duration)
    
    return user, nil
}

// 【重要メソッド】ユーザー作成処理
func (s *UserServiceServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
    start := time.Now()
    requestID := generateRequestID()
    
    s.metrics.RecordRequest("CreateUser")
    
    // 【STEP 1】包括的バリデーション
    if err := s.validateUser(req.User); err != nil {
        s.metrics.RecordError("CreateUser", codes.InvalidArgument)
        s.logger.Printf("❌ [%s] Validation failed for user creation", requestID)
        return nil, err
    }
    
    // 【STEP 2】排他制御によるデータ競合防止
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // 【STEP 3】重複チェック（データ整合性保証）
    if _, exists := s.users[req.User.Id]; exists {
        s.metrics.RecordError("CreateUser", codes.AlreadyExists)
        s.logger.Printf("⚠️  [%s] User already exists: %s", requestID, req.User.Id)
        
        return nil, status.Errorf(codes.AlreadyExists, "user %s already exists", req.User.Id)
        // ✅ 明確な重複エラーメッセージ
    }
    
    // 【STEP 4】ユーザー作成（原子的操作）
    newUser := &pb.User{
        Id:        req.User.Id,
        Name:      req.User.Name,
        Email:     req.User.Email,
        Age:       req.User.Age,
        CreatedAt: time.Now().Unix(),
    }
    
    s.users[req.User.Id] = newUser
    
    // 【STEP 5】成功処理
    duration := time.Since(start)
    s.metrics.RecordSuccess("CreateUser", duration)
    s.logger.Printf("✅ [%s] User created successfully: %s (took %v)", requestID, req.User.Id, duration)
    
    return newUser, nil
}

// 【重要メソッド】包括的バリデーション
func (s *UserServiceServer) validateUser(user *pb.User) error {
    var validationErrors []*pb.ValidationError
    
    // 【バリデーション1】ID検証
    if user.Id == "" {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "id",
            Message: "cannot be empty",
        })
    } else if len(user.Id) < 3 || len(user.Id) > 50 {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "id",
            Message: "must be between 3 and 50 characters",
        })
    }
    
    // 【バリデーション2】メール検証
    if user.Email == "" {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "email",
            Message: "cannot be empty",
        })
    } else if !isValidEmail(user.Email) {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "email",
            Message: "invalid email format",
        })
    }
    
    // 【バリデーション3】名前検証
    if user.Name == "" {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "name",
            Message: "cannot be empty",
        })
    } else if len(user.Name) > 100 {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "name",
            Message: "must be less than 100 characters",
        })
    }
    
    // 【バリデーション4】年齢検証
    if user.Age <= 0 {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "age",
            Message: "must be positive",
        })
    } else if user.Age > 120 {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "age",
            Message: "must be realistic",
        })
    }
    
    // 【エラー集約】全バリデーション結果をまとめて返す
    if len(validationErrors) > 0 {
        st := status.New(codes.InvalidArgument, "validation failed")
        st, _ = st.WithDetails(&pb.ErrorDetails{
            ValidationErrors: validationErrors,
            RequestId:        generateRequestID(),
            Timestamp:        time.Now().Unix(),
        })
        return st.Err()
        // ✅ 一度のリクエストで全エラーを通知
        // ✅ 追跡可能なリクエストID付き
    }
    
    return nil
}

// 【高度なメソッド】削除処理（トランザクション的動作）
func (s *UserServiceServer) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*emptypb.Empty, error) {
    requestID := generateRequestID()
    
    if req.UserId == "" {
        return nil, status.Error(codes.InvalidArgument, "user_id is required")
    }
    
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // 【存在確認】削除前の検証
    if _, exists := s.users[req.UserId]; !exists {
        s.logger.Printf("🔍 [%s] Attempted to delete non-existent user: %s", requestID, req.UserId)
        return nil, status.Error(codes.NotFound, "user not found")
    }
    
    // 【削除実行】
    delete(s.users, req.UserId)
    s.logger.Printf("🗑️  [%s] User deleted successfully: %s", requestID, req.UserId)
    
    return &emptypb.Empty{}, nil
}
```

### クライアントサイドエラーハンドリング

```go
import (
    "google.golang.org/grpc/status"
    "google.golang.org/grpc/codes"
)

func (c *UserClient) GetUser(userID string) (*pb.User, error) {
    req := &pb.GetUserRequest{UserId: userID}
    
    user, err := c.client.GetUser(context.Background(), req)
    if err != nil {
        return nil, c.handleError(err)
    }
    
    return user, nil
}

func (c *UserClient) handleError(err error) error {
    st, ok := status.FromError(err)
    if !ok {
        // gRPCエラーではない
        return err
    }

    switch st.Code() {
    case codes.NotFound:
        return ErrUserNotFound
    case codes.InvalidArgument:
        // 詳細なバリデーションエラーを処理
        return c.processValidationErrors(st)
    case codes.Unavailable:
        // リトライ可能なエラー
        return ErrServiceUnavailable
    case codes.DeadlineExceeded:
        return ErrTimeout
    default:
        return fmt.Errorf("gRPC error: %s", st.Message())
    }
}

func (c *UserClient) processValidationErrors(st *status.Status) error {
    for _, detail := range st.Details() {
        if errorDetails, ok := detail.(*pb.ErrorDetails); ok {
            var messages []string
            for _, ve := range errorDetails.ValidationErrors {
                messages = append(messages, fmt.Sprintf("%s: %s", ve.Field, ve.Message))
            }
            return fmt.Errorf("validation errors: %s", strings.Join(messages, ", "))
        }
    }
    return fmt.Errorf("validation failed: %s", st.Message())
}
```

### リトライ機能の実装

```go
import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

type RetryConfig struct {
    MaxAttempts int
    BackoffBase time.Duration
    MaxBackoff  time.Duration
}

func (c *UserClient) GetUserWithRetry(userID string, config RetryConfig) (*pb.User, error) {
    var lastErr error
    
    for attempt := 0; attempt < config.MaxAttempts; attempt++ {
        user, err := c.GetUser(userID)
        if err == nil {
            return user, nil
        }
        
        lastErr = err
        
        // リトライ可能なエラーかチェック
        if !c.isRetryableError(err) {
            return nil, err
        }
        
        // 最後の試行でなければ待機
        if attempt < config.MaxAttempts-1 {
            backoff := c.calculateBackoff(attempt, config)
            time.Sleep(backoff)
        }
    }
    
    return nil, fmt.Errorf("max retry attempts reached: %w", lastErr)
}

func (c *UserClient) isRetryableError(err error) bool {
    st, ok := status.FromError(err)
    if !ok {
        return false
    }
    
    switch st.Code() {
    case codes.Unavailable, codes.DeadlineExceeded, codes.Internal:
        return true
    default:
        return false
    }
}

func (c *UserClient) calculateBackoff(attempt int, config RetryConfig) time.Duration {
    backoff := config.BackoffBase * time.Duration(1<<attempt) // 指数バックオフ
    if backoff > config.MaxBackoff {
        backoff = config.MaxBackoff
    }
    return backoff
}
```

### Circuit Breaker との統合

```go
type CircuitBreakerClient struct {
    client  pb.UserServiceClient
    breaker *CircuitBreaker
}

func (c *CircuitBreakerClient) GetUser(userID string) (*pb.User, error) {
    result, err := c.breaker.Execute(func() (interface{}, error) {
        return c.client.GetUser(context.Background(), &pb.GetUserRequest{
            UserId: userID,
        })
    })
    
    if err != nil {
        return nil, err
    }
    
    return result.(*pb.User), nil
}

func (c *CircuitBreakerClient) isFailureForCircuitBreaker(err error) bool {
    st, ok := status.FromError(err)
    if !ok {
        return true
    }
    
    // 5xx相当のエラーのみCircuit Breakerでカウント
    switch st.Code() {
    case codes.Internal, codes.Unavailable, codes.DataLoss:
        return true
    default:
        return false
    }
}
```

### メトリクス収集

```go
type MetricsCollector struct {
    requestCount  *prometheus.CounterVec
    errorCount    *prometheus.CounterVec
    requestDuration *prometheus.HistogramVec
}

func (m *MetricsCollector) RecordRequest(method string, code codes.Code, duration time.Duration) {
    m.requestCount.WithLabelValues(method).Inc()
    m.requestDuration.WithLabelValues(method).Observe(duration.Seconds())
    
    if code != codes.OK {
        m.errorCount.WithLabelValues(method, code.String()).Inc()
    }
}
```

## 📝 課題 (The Problem)

以下の機能を持つgRPCエラーハンドリングシステムを実装してください：

### 1. UserService の実装

```protobuf
service UserService {
  rpc GetUser(GetUserRequest) returns (User);
  rpc CreateUser(CreateUserRequest) returns (User);
  rpc UpdateUser(UpdateUserRequest) returns (User);
  rpc DeleteUser(DeleteUserRequest) returns (google.protobuf.Empty);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
}
```

### 2. エラーハンドリング機能

- **適切なステータスコード**: 各エラーに適したgRPCステータスコードの使用
- **詳細なエラー情報**: バリデーションエラーの詳細な情報提供
- **エラーの分類**: リトライ可能/不可能エラーの適切な分類
- **リクエストID**: エラー追跡のためのリクエストID付与

### 3. クライアントサイド機能

- **エラー処理**: ステータスコードに応じた適切な処理
- **リトライ機能**: 指数バックオフによる自動リトライ
- **タイムアウト処理**: デッドライン設定と適切な処理
- **メトリクス収集**: エラー率とレスポンス時間の収集

### 4. テストケース

- 各種エラーケースの網羅的テスト
- リトライ機能の動作確認
- エラー詳細情報の検証

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestUserService_GetUser_Success
    main_test.go:125: Successfully retrieved user
--- PASS: TestUserService_GetUser_Success (0.01s)

=== RUN   TestUserService_GetUser_NotFound
    main_test.go:145: Correctly returned NotFound error
    main_test.go:148: Error message: user not found
--- PASS: TestUserService_GetUser_NotFound (0.00s)

=== RUN   TestUserService_CreateUser_ValidationError
    main_test.go:168: Correctly returned InvalidArgument error
    main_test.go:175: Validation errors: email: cannot be empty, age: must be positive
--- PASS: TestUserService_CreateUser_ValidationError (0.00s)

=== RUN   TestUserService_CreateUser_AlreadyExists
    main_test.go:195: Correctly returned AlreadyExists error
--- PASS: TestUserService_CreateUser_AlreadyExists (0.00s)

=== RUN   TestClientRetry_Success
    main_test.go:215: Retry succeeded after 2 attempts
--- PASS: TestClientRetry_Success (0.05s)

=== RUN   TestClientRetry_NonRetryableError
    main_test.go:235: Correctly failed without retry for non-retryable error
--- PASS: TestClientRetry_NonRetryableError (0.00s)

PASS
ok      day46-grpc-errors   0.157s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 必要なパッケージ

```go
import (
    "context"
    "fmt"
    "regexp"
    "strings"
    "sync"
    "time"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/emptypb"
)
```

### プロトコルバッファ定義

```protobuf
syntax = "proto3";

package user;
option go_package = "./pb";

import "google/protobuf/empty.proto";

service UserService {
  rpc GetUser(GetUserRequest) returns (User);
  rpc CreateUser(CreateUserRequest) returns (User);
  rpc UpdateUser(UpdateUserRequest) returns (User);
  rpc DeleteUser(DeleteUserRequest) returns (google.protobuf.Empty);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
}

message User {
  string id = 1;
  string name = 2;
  string email = 3;
  int32 age = 4;
  int64 created_at = 5;
}

message GetUserRequest {
  string user_id = 1;
}

message CreateUserRequest {
  User user = 1;
}

message UpdateUserRequest {
  User user = 1;
}

message DeleteUserRequest {
  string user_id = 1;
}

message ListUsersRequest {
  int32 page_size = 1;
  string page_token = 2;
}

message ListUsersResponse {
  repeated User users = 1;
  string next_page_token = 2;
}

message ValidationError {
  string field = 1;
  string message = 2;
}

message ErrorDetails {
  repeated ValidationError validation_errors = 1;
  string request_id = 2;
  int64 timestamp = 3;
}
```

### バリデーション関数

```go
func validateUser(user *pb.User) error {
    var validationErrors []*pb.ValidationError

    if user.Id == "" {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "id",
            Message: "cannot be empty",
        })
    }

    if user.Name == "" {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "name",
            Message: "cannot be empty",
        })
    }

    if user.Email == "" {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "email",
            Message: "cannot be empty",
        })
    } else if !isValidEmail(user.Email) {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "email",
            Message: "invalid email format",
        })
    }

    if user.Age <= 0 {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "age",
            Message: "must be positive",
        })
    }

    if len(validationErrors) > 0 {
        st := status.New(codes.InvalidArgument, "validation failed")
        st, _ = st.WithDetails(&pb.ErrorDetails{
            ValidationErrors: validationErrors,
            RequestId:        generateRequestID(),
            Timestamp:        time.Now().Unix(),
        })
        return st.Err()
    }

    return nil
}

func isValidEmail(email string) bool {
    pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
    match, _ := regexp.MatchString(pattern, email)
    return match
}
```

### リトライ設定

```go
type RetryConfig struct {
    MaxAttempts int
    BackoffBase time.Duration
    MaxBackoff  time.Duration
    RetryableCodes map[codes.Code]bool
}

func NewDefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxAttempts: 3,
        BackoffBase: 100 * time.Millisecond,
        MaxBackoff:  5 * time.Second,
        RetryableCodes: map[codes.Code]bool{
            codes.Unavailable:      true,
            codes.DeadlineExceeded: true,
            codes.Internal:         true,
        },
    }
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **gRPC Interceptor**: エラーログの自動収集
2. **Distributed Tracing**: エラー情報のトレース伝播
3. **Health Check**: サービスヘルスの監視とエラー対応
4. **Load Balancing**: エラー率に基づく負荷分散調整
5. **Error Budget**: SLI/SLOに基づくエラー予算管理

gRPCエラーハンドリングの実装を通じて、堅牢な分散システム設計の重要な側面を学びましょう！