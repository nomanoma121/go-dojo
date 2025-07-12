# Day 13: Circuit Breakerパターン

## 🎯 本日の目標 (Today's Goal)

外部サービスの障害が自身のシステムに波及するのを防ぐCircuit Breakerパターンを実装し、システムの耐障害性を向上させるテクニックを学習します。

## 📖 解説 (Explanation)

### Circuit Breakerパターンとは

Circuit Breakerパターンは、外部サービスの呼び出しを監視し、失敗が一定の閾値を超えた場合に自動的に回路を「開く」ことで、システム全体の障害を防ぐデザインパターンです。

### なぜ必要なのか？

1. **障害の連鎖を防ぐ**: 外部サービスの障害が自分のシステムに波及することを防ぎます
2. **リソースの保護**: 失敗することが分かっている呼び出しを避け、スレッドプールやメモリを無駄に消費しません
3. **高速な失敗**: 外部サービスがダウンしている間は即座に失敗させ、応答時間を短縮します
4. **自動回復**: サービスが復旧した際の自動検知と回復機能を提供します

### Circuit Breakerの3つの状態

#### 1. Closed状態（通常状態）
- すべてのリクエストを外部サービスに転送
- 成功・失敗をカウント
- 失敗率が閾値を超えるとOpen状態に移行

#### 2. Open状態（回路開放状態）
- すべてのリクエストを即座に失敗させる
- 外部サービスへの呼び出しは行わない
- 一定時間経過後にHalf-Open状態に移行

#### 3. Half-Open状態（回復試行状態）
- 限定数のリクエストのみ外部サービスに転送
- 成功すればClosed状態に戻る
- 失敗すればOpen状態に戻る

### 実装における考慮点

#### スレッドセーフ性
複数のgoroutineから同時にアクセスされるため、内部状態の管理には適切な同期処理が必要です。

#### 統計の管理
失敗率の計算には滑動窓（sliding window）を使用し、直近の一定期間での成功・失敗を管理します。

#### タイムアウト設定
各状態でのタイムアウト設定を適切に管理する必要があります：
- リクエストタイムアウト
- Open状態での待機時間
- Half-Open状態での試行時間

## 📝 課題 (The Problem)

以下の構造体とメソッドを実装して、Circuit Breakerパターンを完成させてください：

```go
// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState int

const (
    StateClosed CircuitBreakerState = iota
    StateOpen
    StateHalfOpen
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
    maxFailures    int           // 失敗回数の閾値
    resetTimeout   time.Duration // Open状態からHalf-Openに移行する時間
    halfOpenMaxCalls int         // Half-Open状態での最大試行回数
    
    state         CircuitBreakerState
    failures      int
    lastFailTime  time.Time
    halfOpenCalls int
    
    mutex sync.RWMutex
}

// Settings for circuit breaker configuration
type Settings struct {
    MaxFailures      int
    ResetTimeout     time.Duration
    HalfOpenMaxCalls int
}
```

### 実装すべきメソッド

1. **NewCircuitBreaker**: 新しいCircuit Breakerを作成
2. **Call**: 外部サービス呼び出しをラップ
3. **GetState**: 現在の状態を取得
4. **GetCounts**: 統計情報を取得

## ✅ 期待される挙動 (Expected Behavior)

正しく実装されたCircuit Breakerは以下のように動作します：

### Closed状態での動作
```
Request -> [CB: Closed] -> External Service -> Success/Failure
失敗回数が閾値未満: そのまま処理継続
失敗回数が閾値到達: Open状態に移行
```

### Open状態での動作
```
Request -> [CB: Open] -> Immediate Failure (フォールバック実行)
一定時間経過: Half-Open状態に移行
```

### Half-Open状態での動作
```
Request -> [CB: Half-Open] -> External Service (限定的)
成功: Closed状態に復帰
失敗: Open状態に戻る
```

## 💡 ヒント (Hints)

1. **状態管理**: `sync.RWMutex`を使用してスレッドセーフな状態管理を行いましょう
2. **時間管理**: `time.Since()`を使用してタイムアウトを判定しましょう
3. **統計管理**: 成功・失敗のカウンターを適切に更新しましょう
4. **エラーハンドリング**: Circuit Brekerが開いている場合の専用エラーを定義しましょう
5. **テスタビリティ**: 時間に依存する処理は外部から制御可能にしましょう

## 参考実装例

```go
// 基本的な使用例
cb := NewCircuitBreaker(Settings{
    MaxFailures:      3,
    ResetTimeout:     5 * time.Second,
    HalfOpenMaxCalls: 2,
})

// 外部サービス呼び出しのラップ
result, err := cb.Call(func() (interface{}, error) {
    return externalServiceCall()
})
```

## スコアカード

- ✅ 基本実装: 3つの状態が正しく動作する
- ✅ スレッドセーフ: 並行アクセスで正しく動作する
- ✅ 統計管理: 失敗率が正しく計算される
- ✅ 自動回復: タイムアウト後の自動回復が動作する
- ✅ フォールバック: Open状態で適切に失敗する

## 実行方法

```bash
go test -v
go test -race
go test -bench=.
```

## 参考資料

- [Circuit Breaker Pattern - Martin Fowler](https://martinfowler.com/bliki/CircuitBreaker.html)
- [Hystrix Documentation](https://github.com/Netflix/Hystrix/wiki)
- [Go Concurrency Patterns](https://blog.golang.org/concurrency-patterns)