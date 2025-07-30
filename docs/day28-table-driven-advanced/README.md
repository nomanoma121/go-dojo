# Day 28: テーブル駆動テストの応用

🎯 **本日の目標**
テーブル駆動テストの高度なパターンと技法を学び、複雑なテストケースの管理、カスタムアサーション、並列実行、ベンチマークテストなどを効率的に実装できるようになる。

## 📖 解説

### テーブル駆動テストとは

```go
// 【テーブル駆動テストの重要性】体系的なテストケース管理と品質保証の自動化
// ❌ 問題例：個別テストケースでの保守困難とテスト漏れによる品質問題
func catastrophicIndividualTestCases() {
    // 🚨 災害例：個別テストケースの乱立による保守地獄
    
    // ❌ テストケース1：コードが重複
    func TestAddPositiveNumbers(t *testing.T) {
        result := Add(2, 3)
        if result != 5 {
            t.Errorf("Expected 5, got %d", result)
        }
    }
    
    // ❌ テストケース2：同じような構造の重複
    func TestAddNegativeNumbers(t *testing.T) {
        result := Add(-1, -2)
        if result != -3 {
            t.Errorf("Expected -3, got %d", result)
        }
    }
    
    // ❌ テストケース3：微妙に異なるアサーション方法
    func TestAddWithZero(t *testing.T) {
        got := Add(0, 5)
        want := 5
        assert.Equal(t, want, got)
    }
    
    // ❌ 追加テストケース：統一性なし
    func TestAddLargeNumbers(t *testing.T) {
        if Add(1000000, 2000000) != 3000000 {
            t.Error("Large number addition failed")
        }
    }
    
    // 【保守の悪夢】
    // 1. 100個のテストケース→100個の関数
    // 2. アサーション方法がバラバラ
    // 3. エラーメッセージが統一されていない
    // 4. 新しいテストケース追加に巨大なコピペ
    // 5. バグ修正時に100箇所修正必要
    // 6. エッジケースの漏れを発見困難
    
    fmt.Println("❌ Individual test functions created maintenance nightmare!")
    // 結果：テストケース追加が困難、品質担保不十分、開発効率激減
    
    // 【実際の被害例】
    // - 金融システム：エッジケースの漏れで計算ミス→数億円損失
    // - Eコマース：価格計算バグ→顧客からクレーム殺到
    // - 医療システム：薬剤計算エラー→患者安全問題
}

// ✅ 正解：エンタープライズ級テーブル駆動テストシステム
type EnterpriseTableDrivenTestSystem struct {
    // 【基本テストケース管理】
    testSuites     map[string]*TestSuite          // テストスイート管理
    caseGenerator  *TestCaseGenerator             // テストケース自動生成
    dataProvider   *TestDataProvider              // テストデータプロバイダー
    
    // 【高度な検証システム】
    assertionEngine *AssertionEngine              // カスタムアサーションエンジン
    matcherLibrary  *MatcherLibrary               // マッチャーライブラリ
    validator       *TestValidator                // テスト検証エンジン
    
    // 【パフォーマンステスト】
    benchmarkSuite  *BenchmarkSuite               // ベンチマークスイート
    profileManager  *ProfileManager               // プロファイル管理
    
    // 【並列実行制御】
    parallelManager *ParallelTestManager          // 並列テスト管理
    resourcePool    *ResourcePool                 // リソースプール
    
    // 【テストデータ管理】
    fixtureManager  *FixtureManager               // フィクスチャ管理
    builderFactory  *BuilderFactory               // テストデータビルダー
    seedManager     *SeedManager                  // シードデータ管理
    
    // 【結果レポート】
    reportGenerator *ReportGenerator              // レポート生成
    metricsCollector *MetricsCollector            // メトリクス収集
    coverageAnalyzer *CoverageAnalyzer            // カバレッジ解析
    
    // 【品質管理】
    qualityGate     *QualityGate                  // 品質ゲート
    regressionSuite *RegressionTestSuite          // リグレッションテスト
    
    mu              sync.RWMutex                  // 並行アクセス制御
}

// 【重要関数】包括的テーブル駆動テストの実装
func (ttd *EnterpriseTableDrivenTestSystem) ExecuteComprehensiveTestSuite(
    suiteName string, 
    testCases []TestCase,
) *TestResult {
    
    // 【STEP 1】テストケース前処理
    processedCases := ttd.caseGenerator.ProcessTestCases(testCases)
    
    // 【STEP 2】データ検証
    if err := ttd.validator.ValidateTestCases(processedCases); err != nil {
        return &TestResult{Error: fmt.Errorf("test case validation failed: %w", err)}
    }
    
    // 【STEP 3】並列実行制御
    executionPlan := ttd.parallelManager.CreateExecutionPlan(processedCases)
    
    // 【STEP 4】テスト実行
    results := make([]*TestCaseResult, len(processedCases))
    
    for i, testCase := range processedCases {
        // 個別テストケース実行
        result := ttd.executeIndividualTestCase(testCase)
        results[i] = result
        
        // リアルタイム品質チェック
        if !ttd.qualityGate.PassesQualityCheck(result) {
            return &TestResult{
                Error: fmt.Errorf("quality gate failed for test case: %s", testCase.Name),
                Results: results[:i+1],
            }
        }
    }
    
    // 【STEP 5】結果集約とレポート生成
    finalResult := ttd.reportGenerator.GenerateComprehensiveReport(suiteName, results)
    
    return finalResult
}

// 【核心メソッド】高度なテストケース構造の実装
type AdvancedTestCase struct {
    // 【基本情報】
    Name        string                          // テスト名
    Description string                          // 詳細説明
    Tags        []string                        // タグ（smoke, regression, etc.）
    Priority    TestPriority                    // 優先度
    
    // 【セットアップ・クリーンアップ】
    Setup       func(t *testing.T) *TestContext // セットアップ関数
    Cleanup     func(*TestContext)              // クリーンアップ関数
    Timeout     time.Duration                   // タイムアウト
    
    // 【入力データ】
    Input       interface{}                     // 入力データ
    InputBuilder func() interface{}             // 動的入力生成
    
    // 【期待値】
    Expected    interface{}                     // 期待値
    ExpectedGen func(*TestContext) interface{} // 動的期待値生成
    
    // 【エラー検証】
    ExpectError     bool                        // エラー期待フラグ
    ErrorType       error                       // 期待するエラー型
    ErrorContains   []string                    // エラーメッセージ含有文字列
    ErrorMatcher    func(error) bool            // カスタムエラーマッチャー
    
    // 【カスタム検証】
    CustomAssertion func(t *testing.T, got, want interface{}) // カスタムアサーション
    PostValidation  func(t *testing.T, result interface{})    // 事後検証
    
    // 【パフォーマンステスト】
    BenchmarkMode   bool                        // ベンチマークモード
    MemoryTest      bool                        // メモリ使用量テスト
    ConcurrencyTest bool                        // 並行性テスト
    
    // 【条件分岐】
    SkipCondition func() bool                   // スキップ条件
    RunCondition  func() bool                   // 実行条件
    
    // 【依存関係】
    Dependencies []string                       // 依存テストケース
    Fixtures     []string                       // 必要フィクスチャ
}

// 【実用例】エンタープライズレベルの計算機テストスイート
func TestAdvancedCalculator_ComprehensiveTestSuite(t *testing.T) {
    testSystem := NewEnterpriseTableDrivenTestSystem()
    
    // 【基本演算テストスイート】
    arithmeticTests := []AdvancedTestCase{
        {
            Name:        "基本加算_正数同士",
            Description: "正の整数同士の基本的な加算操作を検証",
            Tags:        []string{"arithmetic", "basic", "smoke"},
            Priority:    HighPriority,
            Input: ArithmeticInput{
                Operand1: 15,
                Operand2: 25,
                Operation: "add",
            },
            Expected: ArithmeticResult{
                Value: 40,
                Error: nil,
            },
            CustomAssertion: func(t *testing.T, got, want interface{}) {
                result := got.(ArithmeticResult)
                expected := want.(ArithmeticResult)
                
                assert.Equal(t, expected.Value, result.Value)
                assert.NoError(t, result.Error)
                
                // 【追加検証】計算プロセス検証
                assert.True(t, result.ProcessingTime < 1*time.Millisecond,
                    "Basic addition should complete in under 1ms")
                assert.Equal(t, "addition", result.OperationType)
            },
        },
        {
            Name:        "境界値_最大整数加算",
            Description: "Go言語の最大整数値での加算時のオーバーフロー検証",
            Tags:        []string{"arithmetic", "boundary", "edge-case"},
            Priority:    HighPriority,
            Input: ArithmeticInput{
                Operand1: math.MaxInt64,
                Operand2: 1,
                Operation: "add",
            },
            ExpectError: true,
            ErrorType: &OverflowError{},
            ErrorContains: []string{"integer overflow", "maximum value exceeded"},
            CustomAssertion: func(t *testing.T, got, want interface{}) {
                result := got.(ArithmeticResult)
                
                // オーバーフローエラーの詳細検証
                var overflowErr *OverflowError
                assert.True(t, errors.As(result.Error, &overflowErr),
                    "Error should be OverflowError type")
                assert.Equal(t, "int64", overflowErr.DataType)
                assert.Equal(t, math.MaxInt64, overflowErr.AttemptedValue)
            },
        },
        {
            Name:        "ゼロ除算_エラーハンドリング",
            Description: "ゼロ除算時の適切なエラーハンドリングを検証",
            Tags:        []string{"arithmetic", "error-handling", "division"},
            Priority:    CriticalPriority,
            Input: ArithmeticInput{
                Operand1: 100,
                Operand2: 0,
                Operation: "divide",
            },
            ExpectError: true,
            ErrorType: &DivisionByZeroError{},
            PostValidation: func(t *testing.T, result interface{}) {
                arithmeticResult := result.(ArithmeticResult)
                
                // エラーログが適切に記録されているか検証
                logs := testSystem.GetErrorLogs()
                assert.Contains(t, logs, "division by zero attempted")
                
                // セキュリティ：機密情報が漏洩していないか
                assert.NotContains(t, arithmeticResult.Error.Error(), "internal")
                assert.NotContains(t, arithmeticResult.Error.Error(), "memory address")
            },
        },
        {
            Name:        "高精度小数点計算",
            Description: "金融計算レベルの高精度小数点演算を検証",
            Tags:        []string{"arithmetic", "precision", "financial"},
            Priority:    HighPriority,
            Setup: func(t *testing.T) *TestContext {
                return &TestContext{
                    PrecisionMode: HighPrecision,
                    RoundingMode: BankersRounding,
                    DecimalPlaces: 8,
                }
            },
            Input: ArithmeticInput{
                Operand1: 0.1,
                Operand2: 0.2,
                Operation: "add",
                Precision: 8,
            },
            Expected: ArithmeticResult{
                Value: 0.30000000, // 正確に0.3
                Error: nil,
            },
            CustomAssertion: func(t *testing.T, got, want interface{}) {
                result := got.(ArithmeticResult)
                expected := want.(ArithmeticResult)
                
                // 高精度比較
                delta := math.Abs(expected.Value.(float64) - result.Value.(float64))
                assert.Less(t, delta, 1e-8, 
                    "High precision calculation should be accurate to 8 decimal places")
                
                // 丸め方式の検証
                assert.Equal(t, "bankers_rounding", result.RoundingMethod)
            },
        },
        {
            Name:        "並行計算_スレッドセーフティ",
            Description: "同時並行計算での計算結果の整合性を検証",
            Tags:        []string{"arithmetic", "concurrency", "thread-safety"},
            Priority:    HighPriority,
            ConcurrencyTest: true,
            Input: ArithmeticInput{
                Operand1: 1000,
                Operand2: 2000,
                Operation: "multiply",
                ConcurrentRequests: 100,
            },
            Expected: ArithmeticResult{
                Value: 2000000,
                Error: nil,
            },
            CustomAssertion: func(t *testing.T, got, want interface{}) {
                result := got.(ArithmeticResult)
                expected := want.(ArithmeticResult)
                
                assert.Equal(t, expected.Value, result.Value)
                
                // 並行処理の整合性検証
                assert.Equal(t, 100, result.ConcurrentExecutions)
                assert.True(t, result.AllResultsConsistent,
                    "All concurrent calculations should produce identical results")
                assert.Less(t, result.MaxResponseTime, 100*time.Millisecond,
                    "Concurrent calculations should complete within 100ms")
            },
        },
    }
    
    // 【パフォーマンステストスイート】
    performanceTests := []AdvancedTestCase{
        {
            Name:        "大規模データセット処理",
            Description: "100万件のデータ処理性能を検証",
            Tags:        []string{"performance", "large-dataset", "benchmark"},
            Priority:    MediumPriority,
            BenchmarkMode: true,
            MemoryTest: true,
            Setup: func(t *testing.T) *TestContext {
                return &TestContext{
                    DatasetSize: 1000000,
                    MemoryLimit: 512 * 1024 * 1024, // 512MB
                }
            },
            InputBuilder: func() interface{} {
                return generateLargeDataset(1000000)
            },
            CustomAssertion: func(t *testing.T, got, want interface{}) {
                result := got.(ProcessingResult)
                
                // 処理時間の検証
                assert.Less(t, result.ProcessingTime, 5*time.Second,
                    "Large dataset processing should complete within 5 seconds")
                
                // メモリ使用量の検証
                assert.Less(t, result.PeakMemoryUsage, 512*1024*1024,
                    "Memory usage should not exceed 512MB")
                
                // スループットの検証
                throughput := float64(1000000) / result.ProcessingTime.Seconds()
                assert.Greater(t, throughput, 200000.0,
                    "Processing throughput should exceed 200k records/second")
            },
        },
    }
    
    // 【統合テスト実行】
    t.Run("ArithmeticOperations", func(t *testing.T) {
        result := testSystem.ExecuteComprehensiveTestSuite("arithmetic", arithmeticTests)
        assert.NoError(t, result.Error)
        assert.Equal(t, len(arithmeticTests), result.PassedTests)
    })
    
    t.Run("PerformanceValidation", func(t *testing.T) {
        result := testSystem.ExecuteComprehensiveTestSuite("performance", performanceTests)
        assert.NoError(t, result.Error)
        
        // パフォーマンス基準の検証
        assert.Less(t, result.AverageExecutionTime, 100*time.Millisecond)
        assert.Less(t, result.P95ExecutionTime, 200*time.Millisecond)
        assert.Greater(t, result.ThroughputRPS, 1000.0)
    })
}
```

テーブル駆動テストは、同じロジックで複数の入力・出力パターンをテストする手法です。テストケースをテーブル（スライス）として定義し、ループで実行することで、テストコードの重複を削減し、可読性を向上させます：

### 高度なテストケース構造

複雑なテストケースでは、セットアップ関数、クリーンアップ関数、カスタムアサーションを含む構造を使用します：

```go
type testCase struct {
    name        string
    setup       func(t *testing.T) *TestData
    input       InputData
    want        OutputData
    wantErr     bool
    wantErrType error
    cleanup     func(*TestData)
    assertion   func(t *testing.T, got, want OutputData)
}

func TestComplexOperation(t *testing.T) {
    tests := []testCase{
        {
            name: "successful operation",
            setup: func(t *testing.T) *TestData {
                return &TestData{DB: setupTestDB(t)}
            },
            input: InputData{UserID: 1, Action: "create"},
            want:  OutputData{Status: "success", ID: 1},
            assertion: func(t *testing.T, got, want OutputData) {
                assert.Equal(t, want.Status, got.Status)
                assert.NotZero(t, got.ID)
            },
            cleanup: func(td *TestData) {
                td.DB.Close()
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // セットアップ
            testData := tt.setup(t)
            defer tt.cleanup(testData)
            
            // 実行
            got, err := ComplexOperation(tt.input)
            
            // アサーション
            if tt.wantErr {
                assert.Error(t, err)
                if tt.wantErrType != nil {
                    assert.IsType(t, tt.wantErrType, err)
                }
                return
            }
            
            require.NoError(t, err)
            tt.assertion(t, got, tt.want)
        })
    }
}
```

### サブテストと並列実行

サブテストを使用してテストケースを論理的にグループ化し、並列実行で効率を向上させます：

```go
func TestUserOperations(t *testing.T) {
    t.Run("CreateUser", func(t *testing.T) {
        tests := []struct {
            name string
            user User
            want bool
        }{
            {"valid user", User{Name: "John", Email: "john@example.com"}, true},
            {"empty name", User{Name: "", Email: "john@example.com"}, false},
            {"invalid email", User{Name: "John", Email: "invalid"}, false},
        }
        
        for _, tt := range tests {
            tt := tt // ループ変数をキャプチャ
            t.Run(tt.name, func(t *testing.T) {
                t.Parallel() // 並列実行を有効化
                
                got := CreateUser(tt.user)
                assert.Equal(t, tt.want, got != nil)
            })
        }
    })
    
    t.Run("UpdateUser", func(t *testing.T) {
        // 別のテストグループ
    })
}
```

### データ駆動型ベンチマーク

ベンチマークテストもテーブル駆動にして、異なる入力サイズでの性能を比較します：

```go
func BenchmarkSortAlgorithms(b *testing.B) {
    sizes := []int{10, 100, 1000, 10000}
    algorithms := map[string]func([]int){
        "BubbleSort": BubbleSort,
        "QuickSort":  QuickSort,
        "MergeSort":  MergeSort,
    }
    
    for name, sortFunc := range algorithms {
        for _, size := range sizes {
            b.Run(fmt.Sprintf("%s-%d", name, size), func(b *testing.B) {
                data := generateRandomData(size)
                b.ResetTimer()
                
                for i := 0; i < b.N; i++ {
                    testData := make([]int, len(data))
                    copy(testData, data)
                    sortFunc(testData)
                }
            })
        }
    }
}
```

### カスタムマッチャーとアサーション

複雑なデータ構造の比較には、カスタムアサーション関数を作成します：

```go
type UserMatcher struct {
    ID       *int
    Name     *string
    Email    *string
    Contains []string
}

func (m UserMatcher) Matches(user User) bool {
    if m.ID != nil && user.ID != *m.ID {
        return false
    }
    if m.Name != nil && user.Name != *m.Name {
        return false
    }
    if m.Email != nil && user.Email != *m.Email {
        return false
    }
    
    for _, keyword := range m.Contains {
        if !strings.Contains(user.Description, keyword) {
            return false
        }
    }
    
    return true
}

func TestUserSearch(t *testing.T) {
    tests := []struct {
        name    string
        query   SearchQuery
        matcher UserMatcher
    }{
        {
            name:  "search by name",
            query: SearchQuery{Name: "John"},
            matcher: UserMatcher{
                Name: stringPtr("John"),
            },
        },
        {
            name:  "search with keywords",
            query: SearchQuery{Keywords: []string{"developer", "golang"}},
            matcher: UserMatcher{
                Contains: []string{"developer", "golang"},
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            results := SearchUsers(tt.query)
            
            assert.NotEmpty(t, results)
            for _, user := range results {
                assert.True(t, tt.matcher.Matches(user),
                    "User %+v does not match criteria", user)
            }
        })
    }
}
```

### エラーケーステスト

エラーパターンを体系的にテストします：

```go
func TestUserValidation(t *testing.T) {
    tests := []struct {
        name        string
        user        User
        wantErr     bool
        errContains string
        errType     error
    }{
        {
            name:        "valid user",
            user:        User{Name: "John", Email: "john@example.com", Age: 25},
            wantErr:     false,
        },
        {
            name:        "empty name",
            user:        User{Name: "", Email: "john@example.com", Age: 25},
            wantErr:     true,
            errContains: "name is required",
            errType:     &ValidationError{},
        },
        {
            name:        "invalid email",
            user:        User{Name: "John", Email: "invalid-email", Age: 25},
            wantErr:     true,
            errContains: "invalid email format",
            errType:     &ValidationError{},
        },
        {
            name:        "negative age",
            user:        User{Name: "John", Email: "john@example.com", Age: -1},
            wantErr:     true,
            errContains: "age must be positive",
            errType:     &ValidationError{},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateUser(tt.user)
            
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errContains)
                
                if tt.errType != nil {
                    assert.IsType(t, tt.errType, err)
                }
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### HTTP APIテスト

HTTPエンドポイントのテーブル駆動テストでは、リクエスト・レスポンスの詳細をテストケースに含めます：

```go
func TestUserAPI(t *testing.T) {
    server := httptest.NewServer(setupRoutes())
    defer server.Close()
    
    tests := []struct {
        name           string
        method         string
        path           string
        body           interface{}
        headers        map[string]string
        wantStatus     int
        wantBody       interface{}
        wantHeaders    map[string]string
        bodyAssertion  func(t *testing.T, body []byte)
    }{
        {
            name:       "create user success",
            method:     "POST",
            path:       "/users",
            body:       User{Name: "John", Email: "john@example.com"},
            headers:    map[string]string{"Content-Type": "application/json"},
            wantStatus: http.StatusCreated,
            bodyAssertion: func(t *testing.T, body []byte) {
                var user User
                err := json.Unmarshal(body, &user)
                require.NoError(t, err)
                assert.NotZero(t, user.ID)
                assert.Equal(t, "John", user.Name)
            },
        },
        {
            name:       "create user validation error",
            method:     "POST",
            path:       "/users",
            body:       User{Name: "", Email: "invalid"},
            headers:    map[string]string{"Content-Type": "application/json"},
            wantStatus: http.StatusBadRequest,
            bodyAssertion: func(t *testing.T, body []byte) {
                var errResp ErrorResponse
                err := json.Unmarshal(body, &errResp)
                require.NoError(t, err)
                assert.Contains(t, errResp.Message, "validation")
            },
        },
        {
            name:       "get user not found",
            method:     "GET",
            path:       "/users/999",
            wantStatus: http.StatusNotFound,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var body io.Reader
            if tt.body != nil {
                jsonBody, _ := json.Marshal(tt.body)
                body = bytes.NewReader(jsonBody)
            }
            
            req, err := http.NewRequest(tt.method, server.URL+tt.path, body)
            require.NoError(t, err)
            
            for key, value := range tt.headers {
                req.Header.Set(key, value)
            }
            
            resp, err := http.DefaultClient.Do(req)
            require.NoError(t, err)
            defer resp.Body.Close()
            
            assert.Equal(t, tt.wantStatus, resp.StatusCode)
            
            for key, expected := range tt.wantHeaders {
                assert.Equal(t, expected, resp.Header.Get(key))
            }
            
            if tt.bodyAssertion != nil {
                bodyBytes, err := io.ReadAll(resp.Body)
                require.NoError(t, err)
                tt.bodyAssertion(t, bodyBytes)
            }
        })
    }
}
```

### テストデータファクトリ

テストデータの生成を効率化するファクトリパターン：

```go
type UserBuilder struct {
    user User
}

func NewUserBuilder() *UserBuilder {
    return &UserBuilder{
        user: User{
            Name:  "Default User",
            Email: "default@example.com",
            Age:   25,
        },
    }
}

func (b *UserBuilder) WithName(name string) *UserBuilder {
    b.user.Name = name
    return b
}

func (b *UserBuilder) WithEmail(email string) *UserBuilder {
    b.user.Email = email
    return b
}

func (b *UserBuilder) WithAge(age int) *UserBuilder {
    b.user.Age = age
    return b
}

func (b *UserBuilder) Build() User {
    return b.user
}

func TestUserOperationsWithBuilder(t *testing.T) {
    tests := []struct {
        name    string
        user    User
        wantErr bool
    }{
        {
            name: "valid adult user",
            user: NewUserBuilder().WithAge(30).Build(),
        },
        {
            name: "valid senior user",
            user: NewUserBuilder().WithAge(65).Build(),
        },
        {
            name:    "invalid young user",
            user:    NewUserBuilder().WithAge(10).Build(),
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ProcessUser(tt.user)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **ユーザー管理システム**
   - ユーザーの作成、検索、更新、削除
   - バリデーション機能（名前、メール、年齢）
   - 権限管理（admin/user）

2. **データ処理エンジン**
   - 複数のソートアルゴリズム
   - データ変換とフィルタリング
   - 統計計算機能

3. **HTTP API**
   - RESTful エンドポイント
   - リクエスト/レスポンスバリデーション
   - エラーハンドリング

4. **高度なテストパターン**
   - カスタムマッチャーとアサーション
   - ベンチマークテスト
   - 並列実行対応テスト

## ✅ 期待される挙動

### ユーザー作成成功
```go
user := User{
    Name:  "John Doe",
    Email: "john@example.com", 
    Age:   30,
    Role:  "user",
}

result, err := CreateUser(user)
// result.ID != 0
// err == nil
```

### バリデーションエラー
```go
user := User{
    Name:  "",
    Email: "invalid-email",
    Age:   -1,
}

result, err := CreateUser(user)
// result == nil
// err contains "name is required", "invalid email", "age must be positive"
```

### データソート性能
```bash
BenchmarkSortAlgorithms/BubbleSort-100
BenchmarkSortAlgorithms/QuickSort-100  
BenchmarkSortAlgorithms/MergeSort-100
```

## 💡 ヒント

1. **t.Run()**: サブテストの作成
2. **t.Parallel()**: テストの並列実行
3. **testing.B**: ベンチマークテスト
4. **httptest.NewServer()**: HTTPテスト用サーバー
5. **reflect.DeepEqual()**: 深い比較
6. **testify/assert**: 豊富なアサーション
7. **testify/require**: 必須条件チェック
8. **json.Marshal/Unmarshal**: JSON処理

### テストの分離とクリーンアップ

```go
func TestWithCleanup(t *testing.T) {
    // リソースの準備
    resource := setupResource()
    
    // 自動クリーンアップの登録
    t.Cleanup(func() {
        resource.Close()
    })
    
    // テスト実行...
}
```

### ベンチマークの最適化

```go
func BenchmarkFunction(b *testing.B) {
    // セットアップ（測定対象外）
    data := prepareTestData()
    
    b.ResetTimer()    // タイマーリセット
    b.ReportAllocs()  // アロケーション報告
    
    for i := 0; i < b.N; i++ {
        // 測定対象の処理
        Function(data)
    }
}
```

### 並列テストの注意点

```go
for _, tt := range tests {
    tt := tt // ループ変数をキャプチャ（重要！）
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // テスト実行...
    })
}
```

これらの実装により、保守性が高く効率的なテストスイートを構築し、コードの品質を継続的に保証できます。