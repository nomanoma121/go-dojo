# Day 03: sync.Mutex vs RWMutex

## 学習目標
読み取りと書き込みの競合状態を制御し、MutexとRWMutexのパフォーマンス特性の違いを理解し、適切な使い分けを学ぶ。

## 課題説明

共有データに対する読み取り専用アクセスと書き込みアクセスが混在する環境で、MutexとRWMutexの性能差を測定し、適切な同期プリミティブを選択する方法を実装してください。

### 要件

1. **データ構造**: 同じデータ構造でMutexとRWMutexの両方を実装
2. **読み取り操作**: 複数のGoroutineからの並行読み取りを効率的に処理
3. **書き込み操作**: 排他的な書き込みアクセスを保証
4. **パフォーマンス測定**: 読み書き比率による性能差を測定

### 実装すべき構造体と関数

```go
// MutexCache は sync.Mutex を使ったキャッシュ実装
type MutexCache struct {
    data  map[string]string
    mutex sync.Mutex
}

// RWMutexCache は sync.RWMutex を使ったキャッシュ実装  
type RWMutexCache struct {
    data    map[string]string
    rwmutex sync.RWMutex
}

// Cache インターフェース
type Cache interface {
    Get(key string) (string, bool)
    Set(key, value string)
    Delete(key string)
    Len() int
}
```

## ヒント

1. 読み取り専用の操作では`RLock()`と`RUnlock()`を使用
2. 書き込み操作では`Lock()`と`Unlock()`を使用  
3. `defer`文を使ってアンロックを確実に実行
4. ベンチマークで読み書き比率を変えてパフォーマンスを比較
5. レースコンディションを避けるためマップの初期化を忘れずに

## スコアカード

- ✅ 基本実装: MutexとRWMutexの両方が正しく動作する
- ✅ 同期安全性: レースコンディションが発生しない
- ✅ パフォーマンス: 読み取り優位な場面でRWMutexが優秀
- ✅ エラーハンドリング: 適切なエラーハンドリングとデバッグ情報

## 実行方法

```bash
go test -v
go test -race          # レースコンディション検出
go test -bench=.       # パフォーマンス比較
go test -bench=. -benchmem  # メモリ使用量も測定
```

## 参考資料

- [Go sync.Mutex](https://pkg.go.dev/sync#Mutex)
- [Go sync.RWMutex](https://pkg.go.dev/sync#RWMutex)