# Day 43: Write-Through Pattern

## 🎯 本日の目標 (Today's Goal)

Write-Through キャッシングパターンを実装し、データベースへの書き込みと同時にキャッシュも更新する同期的なキャッシュシステムを構築できるようになる。データの整合性を保ちながら、読み取りパフォーマンスを最適化する手法を理解する。

## 📖 解説 (Explanation)

### Write-Through パターンとは

Write-Through は、データベースへの書き込み処理と同時にキャッシュも更新するキャッシングパターンです。書き込み操作が完了するまで、データベースとキャッシュの両方への更新が同期的に実行されます。

### Write-Through の動作フロー

#### 読み取り処理（Read）

```
1. キャッシュからデータを読み取り試行
2. キャッシュヒット → データを返す
3. キャッシュミス → データベースからデータを取得
4. 取得したデータをキャッシュに保存
5. データを返す
```

#### 書き込み処理（Write）

```
1. データベースにデータを書き込み
2. データベース書き込み成功時
3. キャッシュにも同じデータを書き込み
4. 両方の操作が成功時のみ、書き込み完了を返す
```

### Write-Through の特徴

**利点：**
- データベースとキャッシュの整合性が保たれる
- 書き込み後のキャッシュミスが発生しない
- データの新鮮性が保証される
- 読み取りパフォーマンスが安定している

**欠点：**
- 書き込み時のレイテンシが大きい（2つの操作が必要）
- キャッシュ障害時に書き込みが失敗する可能性
- 使用されないデータもキャッシュされる

### 実装例

```go
// 【Write-Through基本実装】データベースとキャッシュの同期更新
func (s *ProductService) UpdateProduct(ctx context.Context, product *Product) error {
    start := time.Now()
    defer func() {
        s.recordWriteLatency(time.Since(start))
    }()
    
    // 【STEP 1】データベースへの書き込み（メインデータストア）
    err := s.db.UpdateProduct(ctx, product)
    if err != nil {
        s.recordDatabaseWriteError()
        return fmt.Errorf("database update failed: %w", err)
    }
    
    // 【成功】データベース書き込み完了
    s.recordDatabaseWrite()
    log.Printf("💾 Database updated: Product %d", product.ID)
    
    // 【STEP 2】キャッシュへの同期書き込み（Write-Through特性）
    cacheKey := productCacheKey(product.ID)
    err = s.cache.SetJSON(ctx, cacheKey, product, ProductCacheTTL)
    if err != nil {
        // 【重要な設計判断】キャッシュ書き込み失敗時の戦略
        s.recordCacheWriteError()
        
        if s.config.StrictConsistency {
            // 【厳密な整合性モード】データベース更新をロールバック
            log.Printf("❌ ROLLBACK: Cache write failed for product %d, reverting DB", product.ID)
            
            // 元の状態に戻すためのロールバック処理
            if rollbackErr := s.rollbackDatabaseUpdate(ctx, product.ID); rollbackErr != nil {
                log.Printf("💥 CRITICAL: Rollback failed for product %d: %v", product.ID, rollbackErr)
                s.recordCriticalError()
            }
            
            return fmt.Errorf("write-through failed, transaction rolled back: %w", err)
        } else {
            // 【柔軟な整合性モード】警告ログを出力してサービス継続
            log.Printf("⚠️  Cache write failed for product %d: %v (continuing with eventual consistency)", product.ID, err)
            
            // 【非同期修復】後でキャッシュを修復する仕組み
            s.scheduleAsyncCacheRepair(product.ID, product)
        }
    } else {
        // 【成功】キャッシュ書き込み完了
        s.recordCacheWrite()
        log.Printf("⚡ Write-Through completed: Product %d (DB + Cache)", product.ID)
    }
    
    return nil
}

// 【非同期修復】キャッシュ書き込み失敗時の後処理
func (s *ProductService) scheduleAsyncCacheRepair(productID int, product *Product) {
    go func() {
        // 【指数バックオフ】で再試行
        backoff := 100 * time.Millisecond
        maxRetries := 3
        
        for attempt := 0; attempt < maxRetries; attempt++ {
            time.Sleep(backoff)
            
            repairCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            cacheKey := productCacheKey(productID)
            
            if err := s.cache.SetJSON(repairCtx, cacheKey, product, ProductCacheTTL); err == nil {
                log.Printf("🔧 Cache repair successful for product %d (attempt %d)", productID, attempt+1)
                cancel()
                return
            }
            
            cancel()
            backoff *= 2 // 指数バックオフ
            log.Printf("🔧 Cache repair attempt %d failed for product %d", attempt+1, productID)
        }
        
        log.Printf("💥 Cache repair failed permanently for product %d", productID)
        s.recordPermanentCacheFailure()
    }()
}

// 【ロールバック処理】データベース更新の取り消し
func (s *ProductService) rollbackDatabaseUpdate(ctx context.Context, productID int) error {
    // 【注意】実際の実装では元の値を事前に保存しておく必要がある
    // ここでは簡略化して、削除またはマーク処理で対応
    
    log.Printf("⏪ Attempting rollback for product %d", productID)
    
    // オプション1: 更新前の値に戻す（要：更新前データの保存）
    // return s.db.UpdateProduct(ctx, originalProduct)
    
    // オプション2: 無効フラグを設定
    return s.db.MarkProductAsInconsistent(ctx, productID)
}

// 【メトリクス記録】運用監視用
func (s *ProductService) recordDatabaseWrite() {
    atomic.AddInt64(&s.metrics.DatabaseWrites, 1)
}

func (s *ProductService) recordCacheWrite() {
    atomic.AddInt64(&s.metrics.CacheWrites, 1)
}

func (s *ProductService) recordDatabaseWriteError() {
    atomic.AddInt64(&s.metrics.DatabaseWriteErrors, 1)
}

func (s *ProductService) recordCacheWriteError() {
    atomic.AddInt64(&s.metrics.CacheWriteErrors, 1)
}

func (s *ProductService) recordCriticalError() {
    atomic.AddInt64(&s.metrics.CriticalErrors, 1)
    // アラート送信なども可能
}

func (s *ProductService) recordPermanentCacheFailure() {
    atomic.AddInt64(&s.metrics.PermanentCacheFailures, 1)
}

func (s *ProductService) recordWriteLatency(duration time.Duration) {
    s.metrics.mu.Lock()
    defer s.metrics.mu.Unlock()
    
    // 指数移動平均で書き込み時間を追跡
    if s.metrics.AvgWriteTime == 0 {
        s.metrics.AvgWriteTime = duration
    } else {
        s.metrics.AvgWriteTime = time.Duration(
            float64(s.metrics.AvgWriteTime)*0.9 + float64(duration)*0.1,
        )
    }
}
```

### Cache-Aside との比較

| 特徴 | Cache-Aside | Write-Through |
|------|-------------|---------------|
| 読み取り戦略 | Lazy Loading | Lazy Loading |
| 書き込み戦略 | Cache Invalidation | Cache Update |
| データ整合性 | 短期間の不整合あり | 常に整合 |
| 書き込み性能 | 高速 | 低速 |
| 読み取り性能 | キャッシュミス時に遅延 | 安定 |
| 実装複雑度 | 簡単 | やや複雑 |

### トランザクション管理

Write-Through では、データベースとキャッシュの更新をトランザクション的に扱う必要があります：

```go
func (s *ProductService) CreateProduct(ctx context.Context, product *Product) error {
    // データベーストランザクション開始
    tx, err := s.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // データベースに保存
    err = tx.CreateProduct(ctx, product)
    if err != nil {
        return err
    }
    
    // キャッシュに保存
    cacheKey := productCacheKey(product.ID)
    err = s.cache.SetJSON(ctx, cacheKey, product, ProductCacheTTL)
    if err != nil {
        // キャッシュ失敗時はロールバック
        return fmt.Errorf("cache write failed: %w", err)
    }
    
    // 両方成功時にコミット
    return tx.Commit()
}
```

### バルク操作の最適化

複数のデータを効率的に処理するためのバルク操作：

```go
func (s *ProductService) UpdateProducts(ctx context.Context, products []*Product) error {
    // データベースのバルク更新
    err := s.db.UpdateProducts(ctx, products)
    if err != nil {
        return err
    }
    
    // キャッシュのバルク更新
    cacheUpdates := make(map[string]interface{})
    for _, product := range products {
        key := productCacheKey(product.ID)
        cacheUpdates[key] = product
    }
    
    return s.cache.SetMulti(ctx, cacheUpdates, ProductCacheTTL)
}
```

### エラーハンドリング戦略

Write-Through でのエラー処理には複数のアプローチがあります：

#### 1. Strict Consistency（厳密な整合性）

```go
func (s *ProductService) UpdateProduct(ctx context.Context, product *Product) error {
    err := s.db.UpdateProduct(ctx, product)
    if err != nil {
        return err
    }
    
    err = s.cache.SetJSON(ctx, productCacheKey(product.ID), product, ProductCacheTTL)
    if err != nil {
        // キャッシュ失敗時はデータベース更新をロールバック
        s.db.UpdateProduct(ctx, originalProduct) // 元に戻す
        return fmt.Errorf("cache update failed: %w", err)
    }
    
    return nil
}
```

#### 2. Eventually Consistent（結果整合性）

```go
func (s *ProductService) UpdateProduct(ctx context.Context, product *Product) error {
    err := s.db.UpdateProduct(ctx, product)
    if err != nil {
        return err
    }
    
    // キャッシュ更新を非同期で実行
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        err := s.cache.SetJSON(ctx, productCacheKey(product.ID), product, ProductCacheTTL)
        if err != nil {
            log.Printf("Async cache update failed: %v", err)
            // 失敗したキーをキューに入れて後で再試行
            s.retryQueue.Add(productCacheKey(product.ID), product)
        }
    }()
    
    return nil
}
```

### パフォーマンス最適化

#### キャッシュ書き込みの並列化

```go
func (s *ProductService) UpdateProduct(ctx context.Context, product *Product) error {
    // データベースとキャッシュ更新を並列実行
    var dbErr, cacheErr error
    var wg sync.WaitGroup
    
    wg.Add(2)
    
    // データベース更新
    go func() {
        defer wg.Done()
        dbErr = s.db.UpdateProduct(ctx, product)
    }()
    
    // キャッシュ更新
    go func() {
        defer wg.Done()
        cacheKey := productCacheKey(product.ID)
        cacheErr = s.cache.SetJSON(ctx, cacheKey, product, ProductCacheTTL)
    }()
    
    wg.Wait()
    
    if dbErr != nil {
        return dbErr
    }
    
    if cacheErr != nil {
        // キャッシュエラーをログに記録
        log.Printf("Cache update failed: %v", cacheErr)
    }
    
    return nil
}
```

## 📝 課題 (The Problem)

以下の機能を持つ Write-Through パターンを実装してください：

### 1. ProductService の実装

```go
type Product struct {
    ID          int       `json:"id" db:"id"`
    Name        string    `json:"name" db:"name"`
    Price       float64   `json:"price" db:"price"`
    Category    string    `json:"category" db:"category"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type ProductService struct {
    db    ProductRepository
    cache CacheClient
    config ServiceConfig
    metrics *WriteMetrics
}
```

### 2. 必要なメソッドの実装

- `NewProductService(db ProductRepository, cache CacheClient) *ProductService`: サービスの初期化
- `GetProduct(ctx context.Context, productID int) (*Product, error)`: 商品取得
- `CreateProduct(ctx context.Context, product *Product) error`: 商品作成（Write-Through）
- `UpdateProduct(ctx context.Context, product *Product) error`: 商品更新（Write-Through）
- `DeleteProduct(ctx context.Context, productID int) error`: 商品削除
- `BulkUpdateProducts(ctx context.Context, products []*Product) error`: バルク更新
- `GetMetrics() WriteMetrics`: 書き込みメトリクス取得

### 3. エラーハンドリング戦略

- データベース書き込み失敗時の適切な処理
- キャッシュ書き込み失敗時の選択可能な戦略
- 部分的な失敗時のロールバック機能

### 4. パフォーマンス最適化

- バルク操作による効率的な更新
- 並列処理によるレイテンシ改善
- 失敗時の再試行機能

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestProductService_WriteThrough
    main_test.go:85: Product created with write-through
    main_test.go:92: Product immediately available in cache
    main_test.go:99: Database and cache are consistent
--- PASS: TestProductService_WriteThrough (0.02s)

=== RUN   TestProductService_BulkUpdate
    main_test.go:125: 100 products updated in bulk
    main_test.go:132: All products immediately available in cache
    main_test.go:139: Bulk operation completed in 45ms
--- PASS: TestProductService_BulkUpdate (0.05s)

=== RUN   TestProductService_ErrorHandling
    main_test.go:165: Database failure properly handled
    main_test.go:172: Cache failure with graceful degradation
    main_test.go:179: Consistency maintained during errors
--- PASS: TestProductService_ErrorHandling (0.03s)

PASS
ok      day43-write-through     0.187s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### パッケージのインポート

```go
import (
    "context"
    "fmt"
    "sync"
    "time"
)
```

### 設定構造体

```go
type ServiceConfig struct {
    StrictConsistency    bool          // 厳密な整合性を要求するか
    AsyncCacheUpdate     bool          // 非同期キャッシュ更新
    CacheWriteTimeout    time.Duration // キャッシュ書き込みタイムアウト
    MaxRetries          int           // 最大再試行回数
}
```

### メトリクス構造体

```go
type WriteMetrics struct {
    DatabaseWrites    int64
    CacheWrites      int64
    WriteFailures    int64
    AvgWriteTime     time.Duration
    ConsistencyErrors int64
}
```

### トランザクション処理

```go
func (s *ProductService) transactionalUpdate(ctx context.Context, fn func() error) error {
    if s.config.StrictConsistency {
        // 厳密な整合性モード
        return fn()
    } else {
        // 結果整合性モード
        go fn() // 非同期実行
        return nil
    }
}
```

### バルク操作の実装

```go
func (s *ProductService) BulkUpdateProducts(ctx context.Context, products []*Product) error {
    // データベースバルク更新
    err := s.db.UpdateProducts(ctx, products)
    if err != nil {
        return err
    }
    
    // キャッシュバルク更新
    cacheData := make(map[string]interface{})
    for _, product := range products {
        cacheData[productCacheKey(product.ID)] = product
    }
    
    return s.cache.SetMulti(ctx, cacheData, ProductCacheTTL)
}
```

### 再試行メカニズム

```go
func (s *ProductService) updateWithRetry(ctx context.Context, key string, value interface{}) error {
    for i := 0; i < s.config.MaxRetries; i++ {
        err := s.cache.SetJSON(ctx, key, value, ProductCacheTTL)
        if err == nil {
            return nil
        }
        
        if i < s.config.MaxRetries-1 {
            backoff := time.Duration(i+1) * 100 * time.Millisecond
            time.Sleep(backoff)
        }
    }
    
    return fmt.Errorf("max retries exceeded")
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **Write-Behind Queue**: 非同期書き込みキューによる性能向上
2. **Conflict Resolution**: 並行更新時の競合解決機能
3. **Distributed Locking**: 分散環境での排他制御
4. **Cache Warming**: 書き込み時の関連データ事前読み込み
5. **Circuit Breaker**: キャッシュ障害時の自動フォールバック
6. **Audit Logging**: データ変更の追跡とログ記録

Write-Through パターンの実装を通じて、データの整合性を保ちながら高性能なキャッシュシステムを構築する技術を習得しましょう！