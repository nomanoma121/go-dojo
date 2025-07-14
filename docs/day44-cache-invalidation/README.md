# Day 44: Cache Invalidation Strategies

## 🎯 本日の目標 (Today's Goal)

様々なキャッシュ無効化戦略を実装し、データの整合性を保ちながら効率的なキャッシュ管理を行える技術を習得する。TTL、タグベース無効化、依存関係管理などの高度なキャッシュ戦略を理解する。

## 📖 解説 (Explanation)

### キャッシュ無効化の重要性

キャッシュは性能向上のために不可欠ですが、古いデータが残り続けるとシステムの整合性が損なわれます。効果的な無効化戦略により、パフォーマンスと整合性のバランスを取ります。

### 主な無効化戦略

#### 1. TTL (Time To Live) ベース
- 時間経過による自動無効化
- 設定が簡単で予測可能
- データの更新頻度に基づく調整が重要

#### 2. イベントドリブン無効化
- データ更新時の即座な無効化
- 高い整合性を保証
- 複雑な依存関係の管理が必要

#### 3. タグベース無効化
- 関連データをグループ化して一括無効化
- 柔軟な無効化ポリシー
- Redis Sets を活用した効率的な実装

#### 4. 依存関係ベース無効化
- データ間の依存関係を定義
- 連鎖的な無効化処理
- グラフ理論を活用した最適化

## 📝 課題 (The Problem)

以下の機能を持つ高度なキャッシュ無効化システムを実装してください：

### 1. CacheInvalidator の実装

```go
type CacheInvalidator struct {
    cache     CacheClient
    tagStore  TagStore
    ruleEngine RuleEngine
    metrics   *InvalidationMetrics
}
```

### 2. 必要なメソッドの実装

- `InvalidateByKey(ctx context.Context, key string) error`: 個別キー無効化
- `InvalidateByTag(ctx context.Context, tag string) error`: タグベース無効化
- `InvalidateByPattern(ctx context.Context, pattern string) error`: パターンマッチ無効化
- `InvalidateRelated(ctx context.Context, key string) error`: 関連データ無効化
- `SetTTL(ctx context.Context, key string, ttl time.Duration) error`: TTL更新
- `AddInvalidationRule(rule InvalidationRule) error`: 無効化ルール追加

### 3. 高度な機能

- 無効化の遅延実行とバッチ処理
- 無効化パフォーマンスの監視
- 循環依存の検出と回避
- 無効化失敗時の再試行機能

## ✅ 期待される挙動 (Expected Behavior)

```bash
$ go test -v
=== RUN   TestCacheInvalidation_TagBased
    main_test.go:85: Tagged cache invalidation successful
    main_test.go:92: All related items invalidated: 15
--- PASS: TestCacheInvalidation_TagBased (0.03s)

=== RUN   TestCacheInvalidation_DependencyChain
    main_test.go:125: Dependency chain invalidation completed
    main_test.go:132: Cascaded invalidation affected 8 keys
--- PASS: TestCacheInvalidation_DependencyChain (0.02s)
```

## 💡 ヒント (Hints)

### 基本構造

```go
type InvalidationRule struct {
    Trigger   string        // トリガーとなるキー
    Targets   []string      // 無効化対象のキー/パターン
    Delay     time.Duration // 遅延時間
    Condition func() bool   // 実行条件
}

type TagStore interface {
    AddTag(ctx context.Context, key, tag string) error
    GetKeysByTag(ctx context.Context, tag string) ([]string, error)
    RemoveTag(ctx context.Context, key, tag string) error
}
```

### Redis Lua スクリプトによる効率化

```lua
-- タグに関連するすべてのキーを一括削除
local tag = ARGV[1]
local keys = redis.call('SMEMBERS', 'tag:' .. tag)
for i=1,#keys do
    redis.call('DEL', keys[i])
end
redis.call('DEL', 'tag:' .. tag)
return #keys
```

## 🚀 発展課題

1. **階層的タグシステム**: ネストしたタグによる細かい制御
2. **無効化スケジューリング**: cron のような定期実行
3. **分散無効化**: マルチインスタンス環境での同期
4. **無効化監査**: 無効化操作の完全なログ記録

キャッシュ無効化戦略の実装を通じて、大規模システムでのデータ整合性管理技術を習得しましょう！