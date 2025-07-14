# Day 46: gRPC Error Handling

## 🎯 本日の目標 (Today's Goal)

gRPCにおける詳細なエラーハンドリングシステムを実装し、`status`パッケージを使用してクライアントに有用なエラー情報を提供できるようになる。プロダクションレベルのgRPCサービスにおけるエラー処理のベストプラクティスを習得する。

## 📖 解説 (Explanation)

### gRPCエラーハンドリングの重要性

gRPCサービスでは、適切なエラーハンドリングがクライアント体験とデバッグ効率に大きく影響します。標準的なHTTPステータスコードとは異なり、gRPCには独自のステータスコードシステムがあります。

### gRPCステータスコード

gRPCでは以下のような標準ステータスコードが定義されています：

```go
// よく使用されるgRPCステータスコード
OK                 = 0  // 成功
CANCELLED          = 1  // 操作がキャンセルされた
UNKNOWN            = 2  // 不明なエラー
INVALID_ARGUMENT   = 3  // 無効な引数
DEADLINE_EXCEEDED  = 4  // タイムアウト
NOT_FOUND          = 5  // リソースが見つからない
ALREADY_EXISTS     = 6  // リソースが既に存在
PERMISSION_DENIED  = 7  // 権限不足
RESOURCE_EXHAUSTED = 8  // リソース枯渇
FAILED_PRECONDITION = 9 // 前提条件エラー
ABORTED            = 10 // トランザクションの中止
OUT_OF_RANGE       = 11 // 範囲外
UNIMPLEMENTED      = 12 // 未実装
INTERNAL           = 13 // 内部エラー
UNAVAILABLE        = 14 // サービス利用不可
DATA_LOSS          = 15 // データ損失
UNAUTHENTICATED    = 16 // 認証不備
```

### 基本的なエラー作成

```go
import "google.golang.org/grpc/status"
import "google.golang.org/grpc/codes"

// シンプルなエラー
err := status.Error(codes.NotFound, "user not found")

// より詳細なエラー
st := status.New(codes.InvalidArgument, "validation failed")
st, _ = st.WithDetails(&pb.ValidationError{
    Field: "email",
    Message: "invalid email format",
})
return nil, st.Err()
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
type UserServiceServer struct {
    pb.UnimplementedUserServiceServer
    users map[string]*pb.User
    mu    sync.RWMutex
}

func (s *UserServiceServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    // 入力検証
    if req.UserId == "" {
        st := status.New(codes.InvalidArgument, "user_id is required")
        st, _ = st.WithDetails(&pb.ValidationError{
            Field:   "user_id",
            Message: "cannot be empty",
        })
        return nil, st.Err()
    }

    // ユーザー検索
    s.mu.RLock()
    user, exists := s.users[req.UserId]
    s.mu.RUnlock()

    if !exists {
        return nil, status.Error(codes.NotFound, "user not found")
    }

    return user, nil
}

func (s *UserServiceServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
    // バリデーション
    if err := s.validateUser(req.User); err != nil {
        return nil, err
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    // 重複チェック
    if _, exists := s.users[req.User.Id]; exists {
        return nil, status.Errorf(codes.AlreadyExists, "user %s already exists", req.User.Id)
    }

    // ユーザー作成
    s.users[req.User.Id] = req.User
    return req.User, nil
}

func (s *UserServiceServer) validateUser(user *pb.User) error {
    var validationErrors []*pb.ValidationError

    if user.Id == "" {
        validationErrors = append(validationErrors, &pb.ValidationError{
            Field:   "id",
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