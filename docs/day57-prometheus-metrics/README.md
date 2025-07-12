# Day 57: Prometheusによるカスタムメトリクス

## 学習目標
HTTP リクエスト数などのカスタムメトリクスを実装・公開する。

## 課題説明
Prometheus メトリクスを活用してアプリケーションの監視機能を実装し、運用時の可視性を向上させます。

### 要件
1. **Counter メトリクス**: リクエスト数などの累積値
2. **Gauge メトリクス**: 現在値の監視
3. **Summary メトリクス**: レスポンス時間の分布
4. **カスタムラベル**: 詳細な分類用ラベル

## 実行方法
```bash
go run main.go
curl localhost:8080/metrics  # Prometheus メトリクス
curl localhost:8080/api      # サンプルAPI
```