# Day 21: 認証ミドルウェア

## 学習目標
HTTPヘッダーからトークンを読み取り、リクエストを認証するミドルウェアを実装する。

## 課題説明
JWT トークンやAPI キーによる認証機能を持つミドルウェアを実装し、保護されたエンドポイントへのアクセス制御を行います。

### 要件
1. **Bearer Token認証**: Authorization ヘッダーからJWTトークンを抽出
2. **API Key認証**: X-API-Key ヘッダーからAPIキーを検証
3. **User Context**: 認証されたユーザー情報をcontextに格納
4. **エラーハンドリング**: 適切なHTTPステータスコードとエラーレスポンス

## 実行方法
```bash
go test -v
go run main.go
curl -H "Authorization: Bearer valid-token" localhost:8080/protected
```