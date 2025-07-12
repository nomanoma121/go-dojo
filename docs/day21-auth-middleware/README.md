# Day 21: 認証ミドルウェア

🎯 **本日の目標**
HTTPヘッダーからトークンを読み取り、リクエストを認証するミドルウェアを実装し、JWTとAPIキーによる2つの認証方式とロールベースのアクセス制御を学ぶ。

## 📖 解説

### Web認証の基礎

Web APIにおける認証は、ユーザーが誰であるかを確認する重要なセキュリティ機能です。認証なしでは、すべてのAPIエンドポイントが公開されてしまいます。

#### 認証 vs 認可
- **認証（Authentication）**: ユーザーが本人かどうか確認する
- **認可（Authorization）**: 認証されたユーザーが特定のリソースにアクセスする権限があるか確認する

### JWTによる認証

JWT（JSON Web Token）は、情報を安全に送信するためのコンパクトなトークン形式です。

#### JWT の構造
```
header.payload.signature
```

- **Header**: アルゴリズムとトークンタイプを定義
- **Payload**: ユーザー情報やクレームを含む
- **Signature**: 改ざん検知のための署名

```go
// JWTの例
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.
eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.
SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c
```

### HTTPヘッダーでの認証情報送信

#### Bearer Token方式
```http
Authorization: Bearer <token>
```

#### API Key方式
```http
X-API-Key: <api-key>
```

### Go での JWT 実装例

簡単なJWT検証の実装：

```go
func validateJWT(tokenString string, secret string) (*User, error) {
    // トークンの形式チェック
    parts := strings.Split(tokenString, ".")
    if len(parts) != 3 {
        return nil, errors.New("invalid token format")
    }
    
    // 署名検証（実際のプロダクションでは適切なライブラリを使用）
    header := parts[0]
    payload := parts[1]
    signature := parts[2]
    
    // ペイロードデコード
    payloadBytes, err := base64.URLEncoding.DecodeString(payload)
    if err != nil {
        return nil, err
    }
    
    var claims map[string]interface{}
    if err := json.Unmarshal(payloadBytes, &claims); err != nil {
        return nil, err
    }
    
    // ユーザー情報の抽出
    user := &User{
        ID:    claims["sub"].(string),
        Email: claims["email"].(string),
    }
    
    return user, nil
}
```

### ミドルウェアでの認証実装

```go
func (am *AuthMiddleware) JWTAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Authorization ヘッダー取得
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "Authorization header required", http.StatusUnauthorized)
            return
        }
        
        // Bearer プレフィックス確認
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        if tokenString == authHeader {
            http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
            return
        }
        
        // JWT検証
        user, err := am.validateJWT(tokenString)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        // Context にユーザー情報を追加
        ctx := context.WithValue(r.Context(), "user", user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### ロールベースアクセス制御（RBAC）

ユーザーには複数の役割（ロール）を割り当て、エンドポイントごとに必要な役割を定義します：

```go
type User struct {
    ID    string   `json:"id"`
    Email string   `json:"email"`
    Roles []string `json:"roles"`
}

func (am *AuthMiddleware) RequireRoles(requiredRoles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user, ok := getUserFromContext(r.Context())
            if !ok {
                http.Error(w, "User not authenticated", http.StatusUnauthorized)
                return
            }
            
            // ユーザーの役割をチェック
            hasRole := false
            for _, userRole := range user.Roles {
                for _, reqRole := range requiredRoles {
                    if userRole == reqRole {
                        hasRole = true
                        break
                    }
                }
                if hasRole {
                    break
                }
            }
            
            if !hasRole {
                http.Error(w, "Insufficient permissions", http.StatusForbidden)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### セキュリティ考慮事項

1. **秘密鍵の管理**: JWT署名用の秘密鍵は環境変数で管理
2. **トークンの有効期限**: 短期間の有効期限を設定
3. **HTTPS必須**: 本番環境では必ずHTTPS通信
4. **レート制限**: 同一IPからの大量リクエストを制限
5. **ログ記録**: 認証失敗をログに記録（セキュリティ監査用）

### エラーハンドリングのベストプラクティス

```go
func (am *AuthMiddleware) sendErrorResponse(w http.ResponseWriter, code int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    
    response := map[string]interface{}{
        "error": message,
        "timestamp": time.Now().Unix(),
    }
    
    json.NewEncoder(w).Encode(response)
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **AuthMiddleware構造体**
   - JWT秘密鍵とAPIキーのマップを保持
   - 初期化時にサンプルのAPIキーとユーザーを設定

2. **JWT認証ミドルウェア**
   - Authorizationヘッダーの`Bearer <token>`形式を解析
   - 簡略化されたJWT検証（実際のプロダクションでは`jwt-go`等のライブラリを使用）
   - ユーザー情報をContextに格納

3. **APIキー認証ミドルウェア**
   - X-API-Keyヘッダーからキーを取得
   - 事前登録されたキーかチェック
   - 対応するユーザー情報をContextに格納

4. **オプショナル認証**
   - 認証情報があれば検証、なければスキップ
   - 失敗してもエラーにしない

5. **ロールベース認可**
   - 必要な役割を指定できるミドルウェア関数
   - ユーザーの役割をチェックして403または次のハンドラーへ

6. **ヘルパー関数**
   - Contextからユーザー情報を取得
   - エラーレスポンスの統一的な送信

## ✅ 期待される挙動

### 成功パターン

#### 有効なJWTトークンでのアクセス：
```bash
curl -H "Authorization: Bearer valid-jwt-token" http://localhost:8080/protected
```
```json
{
  "message": "protected endpoint",
  "user": {
    "id": "user123",
    "email": "user@example.com",
    "roles": ["user"]
  }
}
```

#### 有効なAPIキーでのアクセス：
```bash
curl -H "X-API-Key: api-key-123" http://localhost:8080/protected
```
```json
{
  "message": "protected endpoint",
  "user": {
    "id": "api-user",
    "email": "api@example.com",
    "roles": ["admin"]
  }
}
```

### エラーパターン

#### 認証情報なし（401 Unauthorized）：
```json
{
  "error": "Authorization header required"
}
```

#### 無効なトークン（401 Unauthorized）：
```json
{
  "error": "Invalid token"
}
```

#### 権限不足（403 Forbidden）：
```json
{
  "error": "Insufficient permissions"
}
```

## 💡 ヒント

1. **strings.TrimPrefix**: "Bearer "プレフィックスの除去
2. **context.WithValue**: Contextにユーザー情報を格納
3. **type assertion**: interface{}からの型変換
4. **HTTP Status Codes**: 
   - 401: 認証失敗
   - 403: 認可失敗（権限不足）
5. **JSON encoding**: エラーレスポンスのJSON形式での送信
6. **slice contains**: スライス内の要素検索

### サンプルのJWTトークン形式（テスト用）

```go
// テスト用の簡略化されたJWT
// Header: {"alg":"HS256","typ":"JWT"}
// Payload: {"sub":"user123","email":"user@example.com","roles":["user"]}
// 実際のプロダクションでは適切なライブラリを使用すること
```

### ミドルウェアチェーンの例

```go
// 複数のミドルウェアを組み合わせ
protected := auth.JWTAuth(auth.RequireRoles("admin")(handler))
```

### セキュリティのヒント

- 本番環境では適切なJWTライブラリ（`github.com/golang-jwt/jwt`）を使用
- 秘密鍵は環境変数から読み込み
- トークンの有効期限をチェック
- レート制限やブルートフォース攻撃対策を検討

これらの実装により、本格的なWebアプリケーションで使用できる認証・認可システムの基礎を学ぶことができます。