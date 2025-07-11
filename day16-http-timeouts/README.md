# Day 16: http.Serverのタイムアウト設定

## 学習目標
Read/Write/Idleの各タイムアウトを設定し、サーバーの安定性を高める。

## 課題説明
プロダクション環境で安定して動作するHTTPサーバーを構築するため、適切なタイムアウト設定を実装してください。

### 要件
1. **ReadTimeout**: リクエスト読み取りタイムアウト
2. **WriteTimeout**: レスポンス書き込みタイムアウト  
3. **IdleTimeout**: Keep-Alive接続のアイドルタイムアウト
4. **HeaderTimeout**: リクエストヘッダー読み取りタイムアウト

## 実行方法
```bash
go test -v
go run main.go
curl -X POST localhost:8080/api/data
```