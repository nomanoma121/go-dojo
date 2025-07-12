# Day 28: テーブル駆動テストの応用

🎯 **本日の目標**
テーブル駆動テストの高度なパターンと技法を学び、複雑なテストケースの管理、カスタムアサーション、並列実行、ベンチマークテストなどを効率的に実装できるようになる。

## 📖 解説

### テーブル駆動テストとは

テーブル駆動テストは、同じロジックで複数の入力・出力パターンをテストする手法です。テストケースをテーブル（スライス）として定義し、ループで実行することで、テストコードの重複を削減し、可読性を向上させます：

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name string
        a, b int
        want int
    }{
        {"positive numbers", 2, 3, 5},
        {"negative numbers", -1, -2, -3},
        {"zero", 0, 5, 5},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := Add(tt.a, tt.b); got != tt.want {
                t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
            }
        })
    }
}
```

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