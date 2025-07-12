# Day 02: Contextによるタイムアウト/デッドライン

## 学習目標
外部API呼び出しなど、時間のかかる処理にタイムアウトを設定し、適切に処理を制御する方法を理解し、実装する。

## 課題説明

外部APIを模擬する関数に対してタイムアウトとデッドラインを設定し、指定時間内に処理が完了しない場合は適切にエラーハンドリングを行うプログラムを実装してください。

### 要件

1. **タイムアウト処理**: 指定時間内に処理が完了しない場合はcontext.DeadlineExceededエラーを返す
2. **デッドライン処理**: 絶対時刻での期限を設定できる
3. **リトライ機能**: 失敗時に指数バックオフでリトライする
4. **グレースフルフェイルオーバー**: タイムアウト時でもリソースリークを防ぐ

### 実装すべき関数

```go
// APICallWithTimeout は外部APIを呼び出し、タイムアウトを設定する
func APICallWithTimeout(ctx context.Context, url string, timeout time.Duration) (*APIResponse, error)

// APICallWithDeadline は絶対時刻でのデッドラインを設定してAPIを呼び出す
func APICallWithDeadline(ctx context.Context, url string, deadline time.Time) (*APIResponse, error)

// APICallWithRetry はタイムアウト付きでリトライ機能を持つAPI呼び出し
func APICallWithRetry(ctx context.Context, url string, timeout time.Duration, maxRetries int) (*APIResponse, error)
```

## ヒント

1. `context.WithTimeout()`でタイムアウト付きコンテキストを作成
2. `context.WithDeadline()`で絶対時刻のデッドラインを設定
3. `time.Sleep()`で処理時間を模擬
4. 指数バックオフは `time.Sleep(time.Duration(math.Pow(2, attempt)) * baseDelay)` で実装
5. `context.Cause()`でタイムアウトの原因を特定（Go 1.20+）

## スコアカード

- ✅ 基本実装: タイムアウトとデッドラインが正しく動作する
- ✅ エラーハンドリング: 適切なコンテキストエラーを返す
- ✅ リトライ機能: 指数バックオフが正しく実装されている  
- ✅ パフォーマンス: 無駄な待機時間がない

## 実行方法

```bash
go test -v
go test -timeout 30s  # テスト自体のタイムアウト設定
```

## 参考資料

- [Go Context Timeouts](https://pkg.go.dev/context#WithTimeout)
- [Context Deadlines](https://pkg.go.dev/context#WithDeadline)