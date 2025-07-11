# Day 04: sync.Onceによる安全な初期化

## 学習目標
一度しか実行したくない初期化処理をスレッドセーフに実装し、sync.Onceの適切な使用方法を理解する。

## 課題説明

アプリケーションで一度だけ実行すべき初期化処理（設定の読み込み、シングルトンの作成、外部リソースの初期化など）を、複数のGoroutineから安全に呼び出せるように実装してください。

### 要件

1. **設定管理**: アプリケーション設定を一度だけ読み込み、以降はキャッシュから返す
2. **シングルトン**: データベース接続プールなどのシングルトンインスタンスを安全に作成
3. **リソース初期化**: 重い初期化処理を複数回実行されないよう保護
4. **エラーハンドリング**: 初期化に失敗した場合の適切な処理

### 実装すべき構造体と関数

```go
// Config represents application configuration
type Config struct {
    DatabaseURL string
    APIKey      string
    LogLevel    string
}

// ConfigManager manages application configuration with lazy initialization
type ConfigManager struct {
    config *Config
    once   sync.Once
    err    error
}

// DatabasePool represents a database connection pool singleton
type DatabasePool struct {
    connections []string
    initialized bool
}
```

## ヒント

1. `sync.Once.Do()`は引数で渡された関数を一度だけ実行する
2. パニックが発生してもOnceは「実行済み」と判定される
3. 初期化でエラーが発生した場合の対処法を考慮する
4. Onceは値で保持し、ポインタで渡さない
5. 初期化関数内でリソースリークを避ける

## スコアカード

- ✅ 基本実装: sync.Onceが正しく動作し、初期化が一度だけ実行される
- ✅ 並行安全性: 複数のGoroutineから同時アクセスしても安全
- ✅ エラーハンドリング: 初期化エラーが適切に処理される
- ✅ パフォーマンス: 初期化後のアクセスで無駄なロックが発生しない

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```

## 参考資料

- [Go sync.Once](https://pkg.go.dev/sync#Once)
- [Singleton Pattern in Go](https://golang.org/doc/faq#closures_and_goroutines)