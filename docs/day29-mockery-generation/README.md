# Day 29: `mockery`によるモック生成

🎯 **本日の目標**
`mockery`ツールを使用してインターフェースからモックを自動生成し、依存関係を分離した効率的な単体テストの作成手法を習得できるようになる。

## 📖 解説

### モックとは

```go
// 【Mockeryモック生成の重要性】依存関係分離と効率的なテスト自動化
// ❌ 問題例：手動モック作成での保守地獄と外部依存によるテスト不安定性
func catastrophicManualMockCreation() {
    // 🚨 災害例：手動モック作成による開発効率激減とテスト品質問題
    
    // ❌ 手動モック1：膨大なボイラープレートコード
    type ManualUserRepositoryMock struct {
        createUserCalls []CreateUserCall
        getUserCalls    []GetUserCall
        // ... 50個のメソッドの記録用構造体
        
        createUserReturns map[int]CreateUserReturn
        getUserReturns    map[int]GetUserReturn
        // ... 50個のメソッドの戻り値マップ
    }
    
    func (m *ManualUserRepositoryMock) CreateUser(user *User) error {
        call := CreateUserCall{User: user, CallTime: time.Now()}
        m.createUserCalls = append(m.createUserCalls, call)
        
        // ❌ 複雑な条件分岐を手動実装
        for _, returnValue := range m.createUserReturns {
            if returnValue.Condition(user) {
                return returnValue.Error
            }
        }
        return nil
    }
    
    // ❌ 50個のメソッド×同様のボイラープレート = 2500行の無駄なコード
    
    // ❌ インターフェース変更時の悪夢
    // 元のインターフェースにメソッド追加
    type UserRepository interface {
        CreateUser(user *User) error
        GetUser(id int) (*User, error)
        // ... 既存の48個のメソッド
        
        // 新規追加：バッチ処理メソッド
        BatchCreateUsers(users []*User) error           // 追加1
        BatchUpdateUsers(users []*User) error           // 追加2
        GetUsersByFilter(filter UserFilter) ([]*User, error) // 追加3
        // ... さらに10個追加
    }
    
    // 【保守の悪夢】
    // 1. 手動モック13箇所すべてに13個のメソッド追加必要
    // 2. 各メソッドに50行のボイラープレートコード必要
    // 3. 13箇所 × 13メソッド × 50行 = 8450行の手動作業
    // 4. 実装忘れやタイプミスでコンパイルエラー多発
    // 5. テストケース追加のたびに全モック修正
    
    fmt.Println("❌ Manual mock creation caused 8450 lines of maintenance nightmare!")
    
    // ❌ 外部依存でのテスト不安定性
    func TestUserService_WithRealDependencies(t *testing.T) {
        // 実際のデータベース接続（テスト環境）
        db, err := sql.Open("postgres", "postgres://test:test@testdb:5432/testdb")
        if err != nil {
            t.Fatal("Database connection failed") // テスト環境問題でテスト失敗
        }
        
        // 実際のメールサービス（外部API）
        emailService := smtp.NewSMTPService("smtp.gmail.com:587", "test", "password")
        
        // 実際の決済API（外部サービス）
        paymentService := stripe.NewPaymentService(os.Getenv("STRIPE_TEST_KEY"))
        
        service := NewUserService(db, emailService, paymentService)
        
        // ❌ テスト実行時の様々な障害
        user := &User{Name: "Test", Email: "test@example.com"}
        err = service.CreateUser(user)
        
        // 失敗する可能性：
        // 1. テストDB接続失敗→テスト停止
        // 2. SMTPサーバーダウン→メール送信失敗
        // 3. Stripe API制限→決済処理失敗
        // 4. ネットワーク遅延→テストタイムアウト
        // 5. 外部サービスメンテナンス→全テスト失敗
        
        if err != nil {
            t.Fatal("Test failed due to external dependency") // 外部要因で失敗
        }
    }
    
    // 【実際の被害例】
    // - 金融システム：外部決済API障害で全テスト失敗→リリース遅延
    // - ECサイト：メールサーバー問題でCI/CD停止→開発チーム待機
    // - 医療システム：DB接続問題でテスト不可→品質検証不能
    // - 物流システム：外部API変更でモック更新漏れ→本番障害
    
    // 結果：テスト実行に30分、メンテナンスに週40時間、信頼性ゼロ
}

// ✅ 正解：エンタープライズ級Mockery自動生成システム
type EnterpriseMockerySystem struct {
    // 【基本モック管理】
    mockRegistry     *MockRegistry                    // モック登録管理
    generationEngine *GenerationEngine               // 自動生成エンジン
    templateManager  *TemplateManager                // テンプレート管理
    
    // 【高度な機能】
    interfaceAnalyzer *InterfaceAnalyzer             // インターフェース解析
    dependencyMapper  *DependencyMapper              // 依存関係マッピング
    mockValidator     *MockValidator                 // モック検証
    
    // 【コード生成最適化】
    codeFormatter     *CodeFormatter                 // コード整形
    importManager     *ImportManager                 // インポート管理
    docGenerator      *DocGenerator                  // ドキュメント生成
    
    // 【テスト統合】
    testSuiteManager  *TestSuiteManager              // テストスイート管理
    assertionBuilder  *AssertionBuilder              // アサーション構築
    scenarioGenerator *ScenarioGenerator             // シナリオ生成
    
    // 【CI/CD統合】
    versionManager    *VersionManager                // バージョン管理
    hookManager       *HookManager                   // フック管理
    
    // 【パフォーマンス最適化】
    cacheManager      *CacheManager                  // キャッシュ管理
    parallelGenerator *ParallelGenerator             // 並列生成
    
    // 【品質保証】
    qualityChecker    *QualityChecker                // 品質チェック
    coverageAnalyzer  *CoverageAnalyzer              // カバレッジ解析
    
    config            *MockeryConfig                 // 設定管理
    mu                sync.RWMutex                   // 並行アクセス制御
}

// 【重要関数】包括的モック生成システム初期化
func NewEnterpriseMockerySystem(config *MockeryConfig) *EnterpriseMockerySystem {
    system := &EnterpriseMockerySystem{
        config:           config,
        mockRegistry:     NewMockRegistry(),
        generationEngine: NewGenerationEngine(config),
        templateManager:  NewTemplateManager(),
        interfaceAnalyzer: NewInterfaceAnalyzer(),
        dependencyMapper: NewDependencyMapper(),
        mockValidator:    NewMockValidator(),
        codeFormatter:    NewCodeFormatter(),
        importManager:    NewImportManager(),
        docGenerator:     NewDocGenerator(),
        testSuiteManager: NewTestSuiteManager(),
        assertionBuilder: NewAssertionBuilder(),
        scenarioGenerator: NewScenarioGenerator(),
        versionManager:   NewVersionManager(),
        hookManager:      NewHookManager(),
        cacheManager:     NewCacheManager(),
        parallelGenerator: NewParallelGenerator(),
        qualityChecker:   NewQualityChecker(),
        coverageAnalyzer: NewCoverageAnalyzer(),
    }
    
    // 【自動設定】
    system.setupAutoGeneration()
    system.registerHooks()
    
    return system
}

// 【核心メソッド】インテリジェントモック生成
func (ems *EnterpriseMockerySystem) GenerateIntelligentMocks(
    packagePath string,
) (*GenerationResult, error) {
    
    // 【STEP 1】インターフェース検出と解析
    interfaces, err := ems.interfaceAnalyzer.AnalyzePackage(packagePath)
    if err != nil {
        return nil, fmt.Errorf("interface analysis failed: %w", err)
    }
    
    // 【STEP 2】依存関係マッピング
    dependencies := ems.dependencyMapper.MapDependencies(interfaces)
    
    // 【STEP 3】生成計画作成
    plan := ems.generationEngine.CreateGenerationPlan(interfaces, dependencies)
    
    // 【STEP 4】並列モック生成
    results := make([]*MockGenerationResult, len(interfaces))
    
    err = ems.parallelGenerator.ExecuteParallel(plan, func(i int, iface *Interface) error {
        mockCode, err := ems.generateAdvancedMock(iface)
        if err != nil {
            return fmt.Errorf("mock generation failed for %s: %w", iface.Name, err)
        }
        
        results[i] = &MockGenerationResult{
            Interface: iface,
            MockCode:  mockCode,
            TestCode:  ems.generateTestHelpers(iface),
            Docs:      ems.docGenerator.GenerateDocumentation(iface),
        }
        
        return nil
    })
    
    if err != nil {
        return nil, fmt.Errorf("parallel generation failed: %w", err)
    }
    
    // 【STEP 5】コード品質検証
    for _, result := range results {
        if err := ems.qualityChecker.ValidateGeneratedCode(result); err != nil {
            return nil, fmt.Errorf("quality check failed: %w", err)
        }
    }
    
    // 【STEP 6】ファイル出力
    outputResult, err := ems.writeGeneratedFiles(results)
    if err != nil {
        return nil, fmt.Errorf("file output failed: %w", err)
    }
    
    return &GenerationResult{
        GeneratedFiles: outputResult.Files,
        Statistics:     ems.generateStatistics(results),
        QualityReport:  ems.qualityChecker.GenerateReport(results),
    }, nil
}

// 【高度メソッド】インテリジェントモック生成
func (ems *EnterpriseMockerySystem) generateAdvancedMock(iface *Interface) (string, error) {
    template := `// Code generated by Enterprise Mockery System. DO NOT EDIT.

package mocks

import (
    "sync"
    "time"
    "context"
    "github.com/stretchr/testify/mock"
    {{range .Imports}}
    "{{.}}"
    {{end}}
)

// {{.InterfaceName}}Mock は {{.InterfaceName}} インターフェースの高機能モック実装です
type {{.InterfaceName}}Mock struct {
    mock.Mock
    
    // 【拡張機能】
    callHistory    []CallRecord
    mutex         sync.RWMutex
    callCount     map[string]int
    latencySimulator *LatencySimulator
    errorInjector *ErrorInjector
    
    // 【監視機能】
    metricsCollector *MetricsCollector
    traceRecorder   *TraceRecorder
}

// NewMock{{.InterfaceName}} は新しいモックインスタンスを作成します
func NewMock{{.InterfaceName}}() *{{.InterfaceName}}Mock {
    return &{{.InterfaceName}}Mock{
        callHistory:      make([]CallRecord, 0),
        callCount:        make(map[string]int),
        latencySimulator: NewLatencySimulator(),
        errorInjector:    NewErrorInjector(),
        metricsCollector: NewMetricsCollector(),
        traceRecorder:    NewTraceRecorder(),
    }
}

{{range .Methods}}
// {{.Name}} は {{$.InterfaceName}}.{{.Name}} のモック実装です
func (m *{{$.InterfaceName}}Mock) {{.Name}}({{.Parameters}}) {{.Returns}} {
    // 【呼び出し記録】
    m.mutex.Lock()
    m.callCount["{{.Name}}"]++
    callRecord := CallRecord{
        Method:    "{{.Name}}",
        Args:      []interface{}{ {{.ArgsList}} },
        Timestamp: time.Now(),
    }
    m.callHistory = append(m.callHistory, callRecord)
    m.mutex.Unlock()
    
    // 【レイテンシシミュレーション】
    if latency := m.latencySimulator.GetLatency("{{.Name}}"); latency > 0 {
        time.Sleep(latency)
    }
    
    // 【エラーインジェクション】
    if err := m.errorInjector.ShouldInjectError("{{.Name}}", {{.ArgsList}}); err != nil {
        {{if .HasError}}
        return {{.ZeroValues}}, err
        {{else}}
        panic(fmt.Sprintf("Injected error in {{.Name}}: %v", err))
        {{end}}
    }
    
    // 【メトリクス収集】
    startTime := time.Now()
    defer func() {
        m.metricsCollector.RecordCall("{{.Name}}", time.Since(startTime))
    }()
    
    // 【トレース記録】
    span := m.traceRecorder.StartSpan("{{$.InterfaceName}}.{{.Name}}")
    defer span.End()
    
    // 【基本モック機能】
    ret := m.Called({{.ArgsList}})
    
    {{if .Returns}}
    return {{range $i, $ret := .ReturnsList}}
        {{if eq $ret "error"}}
        ret.Error({{$i}})
        {{else}}
        ret.Get({{$i}}).({{$ret}})
        {{end}}
        {{if not (isLast $i $.ReturnsList)}}, {{end}}
    {{end}}
    {{end}}
}

// {{.Name}}WithContext は {{.Name}} のコンテキスト対応版です
{{if .HasContext}}
func (m *{{$.InterfaceName}}Mock) {{.Name}}WithContext(ctx context.Context, {{.ParametersWithoutContext}}) {{.Returns}} {
    select {
    case <-ctx.Done():
        {{if .HasError}}
        return {{.ZeroValues}}, ctx.Err()
        {{else}}
        panic(fmt.Sprintf("Context cancelled in {{.Name}}: %v", ctx.Err()))
        {{end}}
    default:
        return m.{{.Name}}({{.ArgsListWithContext}})
    }
}
{{end}}
{{end}}

// 【拡張ヘルパーメソッド】

// GetCallHistory は呼び出し履歴を返します
func (m *{{.InterfaceName}}Mock) GetCallHistory() []CallRecord {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    
    history := make([]CallRecord, len(m.callHistory))
    copy(history, m.callHistory)
    return history
}

// GetCallCount は指定メソッドの呼び出し回数を返します
func (m *{{.InterfaceName}}Mock) GetCallCount(methodName string) int {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    return m.callCount[methodName]
}

// SimulateLatency は指定メソッドのレイテンシをシミュレートします
func (m *{{.InterfaceName}}Mock) SimulateLatency(methodName string, latency time.Duration) {
    m.latencySimulator.SetLatency(methodName, latency)
}

// InjectError は指定条件でエラーを注入します
func (m *{{.InterfaceName}}Mock) InjectError(methodName string, condition func(...interface{}) bool, err error) {
    m.errorInjector.AddErrorCondition(methodName, condition, err)
}

// GetMetrics はパフォーマンスメトリクスを返します
func (m *{{.InterfaceName}}Mock) GetMetrics() *PerformanceMetrics {
    return m.metricsCollector.GetMetrics()
}

// Reset はモック状態をリセットします
func (m *{{.InterfaceName}}Mock) Reset() {
    m.Mock.ExpectedCalls = nil
    m.Mock.Calls = nil
    
    m.mutex.Lock()
    m.callHistory = make([]CallRecord, 0)
    m.callCount = make(map[string]int)
    m.mutex.Unlock()
    
    m.latencySimulator.Reset()
    m.errorInjector.Reset()
    m.metricsCollector.Reset()
    m.traceRecorder.Reset()
}
`
    
    // テンプレート実行
    generatedCode, err := ems.templateManager.ExecuteTemplate(template, iface)
    if err != nil {
        return "", fmt.Errorf("template execution failed: %w", err)
    }
    
    // コード整形
    formattedCode, err := ems.codeFormatter.Format(generatedCode)
    if err != nil {
        return "", fmt.Errorf("code formatting failed: %w", err)
    }
    
    return formattedCode, nil
}
```

モック（Mock）は、テスト対象のコードが依存する外部システムやコンポーネントの振る舞いを模倣するテスト用のオブジェクトです。モックを使用することで、以下の利点があります：

- **依存関係の分離**: 外部システムに依存せずにテスト実行
- **テストの高速化**: 実際のDBやAPIアクセスを回避
- **予測可能な結果**: 期待する結果を事前に設定
- **エラーケースのテスト**: 意図的にエラーを発生させてテスト

### mockeryとは

`mockery`は、Goのインターフェースから自動的にモックコードを生成するツールです。手動でモックを作成する手間を省き、インターフェースの変更に自動で追従します：

```bash
# mockeryのインストール
go install github.com/vektra/mockery/v2@latest

# モック生成
mockery --name=UserRepository --output=./mocks
```

### 基本的なインターフェースとモック

まず、モック化したいインターフェースを定義します：

```go
//go:generate mockery --name=UserRepository
type UserRepository interface {
    CreateUser(user *User) error
    GetUser(id int) (*User, error)
    UpdateUser(user *User) error
    DeleteUser(id int) error
    ListUsers() ([]*User, error)
}

//go:generate mockery --name=EmailService
type EmailService interface {
    SendEmail(to, subject, body string) error
    SendWelcomeEmail(user *User) error
}

//go:generate mockery --name=PaymentProcessor
type PaymentProcessor interface {
    ProcessPayment(amount float64, cardToken string) (*PaymentResult, error)
    RefundPayment(transactionID string) error
}
```

### モック生成の実行

`go:generate`コメントを使用して、コード生成を自動化：

```bash
# プロジェクトルートで実行
go generate ./...

# または手動実行
mockery --all --output=./mocks --case=underscore
```

生成されたモックの例：

```go
// mocks/UserRepository.go
package mocks

import (
    "github.com/stretchr/testify/mock"
)

type UserRepository struct {
    mock.Mock
}

func (m *UserRepository) CreateUser(user *User) error {
    ret := m.Called(user)
    
    var r0 error
    if rf, ok := ret.Get(0).(func(*User) error); ok {
        r0 = rf(user)
    } else {
        r0 = ret.Error(0)
    }
    
    return r0
}

func (m *UserRepository) GetUser(id int) (*User, error) {
    ret := m.Called(id)
    
    var r0 *User
    if rf, ok := ret.Get(0).(func(int) *User); ok {
        r0 = rf(id)
    } else {
        if ret.Get(0) != nil {
            r0 = ret.Get(0).(*User)
        }
    }
    
    var r1 error
    if rf, ok := ret.Get(1).(func(int) error); ok {
        r1 = rf(id)
    } else {
        r1 = ret.Error(1)
    }
    
    return r0, r1
}
```

### モックを使用したテスト

生成されたモックを使用してサービス層のテストを作成：

```go
func TestUserService_CreateUser(t *testing.T) {
    // モックの準備
    mockRepo := new(mocks.UserRepository)
    mockEmail := new(mocks.EmailService)
    
    service := NewUserService(mockRepo, mockEmail)
    
    // テストデータ
    user := &User{
        Name:  "John Doe",
        Email: "john@example.com",
    }
    
    // モックの期待値設定
    mockRepo.On("CreateUser", user).Return(nil)
    mockEmail.On("SendWelcomeEmail", user).Return(nil)
    
    // テスト実行
    err := service.CreateUser(user)
    
    // アサーション
    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
    mockEmail.AssertExpectations(t)
}

func TestUserService_CreateUser_EmailFailure(t *testing.T) {
    mockRepo := new(mocks.UserRepository)
    mockEmail := new(mocks.EmailService)
    
    service := NewUserService(mockRepo, mockEmail)
    
    user := &User{Name: "John", Email: "john@example.com"}
    
    // リポジトリは成功、メールサービスは失敗
    mockRepo.On("CreateUser", user).Return(nil)
    mockEmail.On("SendWelcomeEmail", user).Return(errors.New("SMTP error"))
    
    err := service.CreateUser(user)
    
    // エラーが返されることを確認
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "SMTP error")
    
    mockRepo.AssertExpectations(t)
    mockEmail.AssertExpectations(t)
}
```

### 高度なモック設定

複雑な戻り値や条件に応じた振る舞いを設定：

```go
func TestUserService_ComplexScenario(t *testing.T) {
    mockRepo := new(mocks.UserRepository)
    mockPayment := new(mocks.PaymentProcessor)
    
    service := NewUserService(mockRepo, mockPayment)
    
    // 条件に応じた戻り値
    mockRepo.On("GetUser", 1).Return(&User{ID: 1, Name: "John"}, nil)
    mockRepo.On("GetUser", 999).Return(nil, errors.New("user not found"))
    
    // 引数マッチャーを使用
    mockPayment.On("ProcessPayment", mock.AnythingOfType("float64"), mock.AnythingOfType("string")).
        Return(&PaymentResult{TransactionID: "tx123"}, nil)
    
    // 条件付きの戻り値
    mockPayment.On("ProcessPayment", mock.MatchedBy(func(amount float64) bool {
        return amount > 10000
    }), mock.Anything).Return(nil, errors.New("amount too large"))
    
    // テスト実行...
}
```

### モックの検証パターン

様々な検証方法：

```go
func TestUserService_Verification(t *testing.T) {
    mockRepo := new(mocks.UserRepository)
    service := NewUserService(mockRepo)
    
    user := &User{Name: "John"}
    
    // 1. 基本的な呼び出し検証
    mockRepo.On("CreateUser", user).Return(nil).Once()
    
    service.CreateUser(user)
    
    mockRepo.AssertExpectations(t)
    
    // 2. 特定のメソッドが呼ばれたことを検証
    mockRepo.AssertCalled(t, "CreateUser", user)
    
    // 3. 呼び出し回数の検証
    mockRepo.AssertNumberOfCalls(t, "CreateUser", 1)
    
    // 4. メソッドが呼ばれなかったことを検証
    mockRepo.AssertNotCalled(t, "DeleteUser", mock.Anything)
}
```

### テーブル駆動テストとモック

複数のシナリオを効率的にテスト：

```go
func TestUserService_TableDriven(t *testing.T) {
    tests := []struct {
        name          string
        setupMock     func(*mocks.UserRepository)
        input         *User
        expectedError bool
        errorContains string
    }{
        {
            name: "successful creation",
            setupMock: func(m *mocks.UserRepository) {
                m.On("CreateUser", mock.AnythingOfType("*User")).Return(nil)
            },
            input:         &User{Name: "John", Email: "john@example.com"},
            expectedError: false,
        },
        {
            name: "database error",
            setupMock: func(m *mocks.UserRepository) {
                m.On("CreateUser", mock.AnythingOfType("*User")).
                    Return(errors.New("database connection failed"))
            },
            input:         &User{Name: "John", Email: "john@example.com"},
            expectedError: true,
            errorContains: "database connection",
        },
        {
            name: "duplicate email",
            setupMock: func(m *mocks.UserRepository) {
                m.On("CreateUser", mock.AnythingOfType("*User")).
                    Return(errors.New("email already exists"))
            },
            input:         &User{Name: "John", Email: "existing@example.com"},
            expectedError: true,
            errorContains: "email already exists",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockRepo := new(mocks.UserRepository)
            tt.setupMock(mockRepo)
            
            service := NewUserService(mockRepo)
            err := service.CreateUser(tt.input)
            
            if tt.expectedError {
                assert.Error(t, err)
                if tt.errorContains != "" {
                    assert.Contains(t, err.Error(), tt.errorContains)
                }
            } else {
                assert.NoError(t, err)
            }
            
            mockRepo.AssertExpectations(t)
        })
    }
}
```

### 非同期処理のモック

コールバックやチャネルを使用した非同期処理のテスト：

```go
//go:generate mockery --name=AsyncProcessor
type AsyncProcessor interface {
    ProcessAsync(data string, callback func(result string, err error))
    ProcessWithChannel(data string) <-chan ProcessResult
}

func TestAsyncService(t *testing.T) {
    mockProcessor := new(mocks.AsyncProcessor)
    service := NewAsyncService(mockProcessor)
    
    // コールバック関数のモック
    mockProcessor.On("ProcessAsync", "test data", mock.AnythingOfType("func(string, error)")).
        Run(func(args mock.Arguments) {
            callback := args.Get(1).(func(string, error))
            callback("processed: test data", nil)
        }).Return()
    
    var result string
    var err error
    
    service.ProcessData("test data", func(r string, e error) {
        result = r
        err = e
    })
    
    assert.NoError(t, err)
    assert.Equal(t, "processed: test data", result)
    mockProcessor.AssertExpectations(t)
}
```

### HTTP クライアントのモック

外部APIへの依存を分離：

```go
//go:generate mockery --name=HTTPClient
type HTTPClient interface {
    Get(url string) (*http.Response, error)
    Post(url string, body io.Reader) (*http.Response, error)
}

func TestAPIService_GetUserData(t *testing.T) {
    mockClient := new(mocks.HTTPClient)
    service := NewAPIService(mockClient)
    
    // レスポンスのモック作成
    responseBody := `{"id": 1, "name": "John"}`
    resp := &http.Response{
        StatusCode: 200,
        Body:       io.NopCloser(strings.NewReader(responseBody)),
        Header:     make(http.Header),
    }
    resp.Header.Set("Content-Type", "application/json")
    
    mockClient.On("Get", "https://api.example.com/users/1").Return(resp, nil)
    
    user, err := service.GetUserData(1)
    
    assert.NoError(t, err)
    assert.Equal(t, 1, user.ID)
    assert.Equal(t, "John", user.Name)
    mockClient.AssertExpectations(t)
}
```

### カスタムマッチャー

複雑な引数の検証：

```go
func TestUserService_CustomMatcher(t *testing.T) {
    mockRepo := new(mocks.UserRepository)
    service := NewUserService(mockRepo)
    
    // カスタムマッチャー: 有効なメールアドレスを持つユーザー
    validUserMatcher := mock.MatchedBy(func(user *User) bool {
        return user != nil && 
               user.Name != "" && 
               strings.Contains(user.Email, "@")
    })
    
    mockRepo.On("CreateUser", validUserMatcher).Return(nil)
    
    user := &User{Name: "John", Email: "john@example.com"}
    err := service.CreateUser(user)
    
    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
}
```

### モックのリセットと再利用

テスト間でのモック状態のリセット：

```go
func TestUserService_MultipleTests(t *testing.T) {
    mockRepo := new(mocks.UserRepository)
    service := NewUserService(mockRepo)
    
    t.Run("test 1", func(t *testing.T) {
        mockRepo.On("GetUser", 1).Return(&User{ID: 1}, nil).Once()
        
        user, err := service.GetUser(1)
        assert.NoError(t, err)
        assert.Equal(t, 1, user.ID)
        
        mockRepo.AssertExpectations(t)
    })
    
    // モックの状態をリセット
    mockRepo.ExpectedCalls = nil
    mockRepo.Calls = nil
    
    t.Run("test 2", func(t *testing.T) {
        mockRepo.On("GetUser", 2).Return(&User{ID: 2}, nil).Once()
        
        user, err := service.GetUser(2)
        assert.NoError(t, err)
        assert.Equal(t, 2, user.ID)
        
        mockRepo.AssertExpectations(t)
    })
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **サービス層の実装**
   - ユーザー管理サービス（作成、取得、更新、削除）
   - 通知サービス（メール、SMS送信）
   - 決済処理サービス（支払い、返金）

2. **インターフェースの定義**
   - リポジトリインターフェース
   - 外部サービスインターフェース
   - HTTPクライアントインターフェース

3. **ビジネスロジック**
   - バリデーション処理
   - トランザクション管理
   - エラーハンドリング

4. **モック対応設計**
   - 依存性注入の実装
   - インターフェース分離
   - テスタブルな設計

## ✅ 期待される挙動

### モック生成
```bash
# インターフェースからモック生成
go generate ./...

# 生成されたモックファイルの確認
ls mocks/
# UserRepository.go
# EmailService.go
# PaymentProcessor.go
```

### テスト実行結果
```bash
go test -v

=== RUN   TestUserService_CreateUser
--- PASS: TestUserService_CreateUser (0.00s)
=== RUN   TestUserService_CreateUser_ValidationError
--- PASS: TestUserService_CreateUser_ValidationError (0.00s)
=== RUN   TestUserService_CreateUser_RepositoryError
--- PASS: TestUserService_CreateUser_RepositoryError (0.00s)
=== RUN   TestUserService_CreateUser_EmailError
--- PASS: TestUserService_CreateUser_EmailError (0.00s)
```

### モックの検証
```go
// 期待される呼び出しが正しく行われたかチェック
mockRepo.AssertExpectations(t)
mockEmail.AssertExpectations(t)

// 特定のメソッドが呼ばれたかチェック
mockRepo.AssertCalled(t, "CreateUser", expectedUser)

// 呼び出し回数のチェック
mockRepo.AssertNumberOfCalls(t, "CreateUser", 1)
```

## 💡 ヒント

1. **mockery**: モック自動生成ツール
2. **go:generate**: コード生成の自動化
3. **mock.Mock**: testifyのモック基底構造体
4. **mock.On()**: メソッド呼び出しの期待値設定
5. **mock.Return()**: 戻り値の設定
6. **mock.AnythingOfType()**: 型マッチング
7. **mock.MatchedBy()**: カスタムマッチャー
8. **AssertExpectations()**: モック検証

### mockeryの設定ファイル

`.mockery.yaml`でプロジェクト全体の設定を管理：

```yaml
with-expecter: true
output: "mocks"
case: "underscore"
interfaces:
  UserRepository:
    config:
      output: "internal/mocks"
  EmailService:
    config:
      output: "pkg/email/mocks"
```

### テストヘルパー関数

モック設定の再利用性を向上：

```go
func setupUserServiceTest() (*UserService, *mocks.UserRepository, *mocks.EmailService) {
    mockRepo := new(mocks.UserRepository)
    mockEmail := new(mocks.EmailService)
    service := NewUserService(mockRepo, mockEmail)
    return service, mockRepo, mockEmail
}

func createTestUser() *User {
    return &User{
        Name:  "Test User",
        Email: "test@example.com",
        Age:   25,
    }
}
```

### CI/CDでのモック管理

```bash
# CI/CDパイプラインでモック生成を確認
go generate ./...
git diff --exit-code mocks/
```

これらの実装により、保守性が高く効率的なテストシステムを構築し、外部依存を適切に分離したテストを実現できます。