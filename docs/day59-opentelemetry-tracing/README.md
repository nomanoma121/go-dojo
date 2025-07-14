# Day 59: OpenTelemetry Distributed Tracing

## 🎯 本日の目標 (Today's Goal)

OpenTelemetryによる分散トレーシングを実装し、サービスをまたぐリクエストのトレース情報を設定・出力する仕組みを習得する。マイクロサービス間のリクエストフローの可視化と問題の特定を行う。

## 📖 解説 (Explanation)

### OpenTelemetryとは

OpenTelemetryは、テレメトリデータ（メトリクス、ログ、トレース）を収集、処理、エクスポートするためのオープンソースの観測可能性フレームワークです。

### 分散トレーシングとは

分散トレーシングは、複数のサービスやコンポーネントにまたがるリクエストの流れを追跡する仕組みです。一つのリクエストがどのようにシステム内を通過するかを可視化できます。

### 主要概念

1. **Trace**: 一つのリクエストの全体的な流れ
2. **Span**: トレース内の個別の操作単位
3. **Context**: スパン間の関係性を維持する情報
4. **Attributes**: スパンに付加される詳細情報

### 実装パターン

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
    "go.opentelemetry.io/otel/attribute"
)

// トレーサーの取得
tracer := otel.Tracer("service-name")

// スパンの作成
ctx, span := tracer.Start(ctx, "operation-name")
defer span.End()

// 属性の追加
span.SetAttributes(
    attribute.String("user.id", userID),
    attribute.Int("request.size", size),
)

// ステータスの設定
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```

## 📝 課題 (The Problem)

OpenTelemetry分散トレーシングを使用して以下の機能を実装してください：

1. **基本トレーシング**: HTTP リクエストのトレース
2. **分散トレーシング**: サービス間でのコンテキスト伝播
3. **カスタムスパン**: アプリケーション固有の操作のトレース
4. **エラートレーシング**: エラー情報の記録
5. **メタデータ追加**: リクエスト詳細の記録

## 💡 ヒント (Hints)

- `go.opentelemetry.io/otel`ライブラリの使用
- HTTP ヘッダーでのコンテキスト伝播
- スパンの階層構造の設計
- 適切な属性とイベントの記録