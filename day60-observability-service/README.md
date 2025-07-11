# Day 60: 総集編 - 可観測性完備APIサービス

## 学習目標
slog, Prometheus, OpenTelemetry を導入したミニAPIサービスを構築する。

## 課題説明
これまで学習した技術を統合し、本格的な可観測性を備えたプロダクションレディなAPIサービスを実装します。

### 要件
1. **構造化ログ**: slog による詳細なログ出力
2. **メトリクス**: Prometheus による性能監視
3. **分散トレーシング**: OpenTelemetry によるリクエスト追跡
4. **ヘルスチェック**: アプリケーションの健全性監視

## 特徴
- 60日間の学習の集大成
- プロダクション運用を想定した実装
- 監視・デバッグ・パフォーマンス分析の完全サポート

## 実行方法
```bash
go run main.go
curl localhost:8080/health
curl localhost:8080/api/users
curl localhost:8080/metrics
```