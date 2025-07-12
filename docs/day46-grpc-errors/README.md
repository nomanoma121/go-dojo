# Day 46: gRPCのエラーハンドリング

## 学習目標
status パッケージを使い、gRPC で詳細なエラー情報を返すエラーハンドリングを実装する。

## 課題説明
gRPC における適切なエラーハンドリングを実装し、クライアントに有用なエラー情報を提供します。

### 要件
1. **gRPC Status Codes**: 適切なステータスコードの使用
2. **Error Details**: エラーの詳細情報付与
3. **クライアント処理**: エラーコードに応じた処理分岐
4. **Retry Logic**: 一時的エラーの再試行機能

## 実行方法
```bash
go generate  # protobuf コード生成
go test -v
go run server/main.go & go run client/main.go
```