package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// エラー定義
var (
	ErrCacheMiss       = errors.New("cache miss")
	ErrProductNotFound = errors.New("product not found")
	ErrCacheTimeout    = errors.New("cache operation timeout")
)

// TTL 定数
const (
	ProductCacheTTL  = 2 * time.Hour
	CategoryCacheTTL = 1 * time.Hour
)

// Product は、商品情報を表す構造体
type Product struct {
	ID        int       `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Price     float64   `json:"price" db:"price"`
	Category  string    `json:"category" db:"category"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ServiceConfig は、サービスの設定を保持する構造体
type ServiceConfig struct {
	StrictConsistency bool          // 厳密な整合性を要求するか
	AsyncCacheUpdate  bool          // 非同期キャッシュ更新
	CacheWriteTimeout time.Duration // キャッシュ書き込みタイムアウト
	MaxRetries        int           // 最大再試行回数
}

// WriteMetrics は、書き込み操作の統計情報を保持する構造体
type WriteMetrics struct {
	DatabaseWrites    int64         // データベース書き込み数
	CacheWrites       int64         // キャッシュ書き込み数
	WriteFailures     int64         // 書き込み失敗数
	AvgWriteTime      time.Duration // 平均書き込み時間
	ConsistencyErrors int64         // 整合性エラー数
}

// CacheClient は、キャッシュクライアントのインターフェース
type CacheClient interface {
	GetJSON(ctx context.Context, key string, dest interface{}) error
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	SetMulti(ctx context.Context, pairs map[string]interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// ProductRepository は、商品データベース操作のインターフェース
type ProductRepository interface {
	GetProduct(ctx context.Context, productID int) (*Product, error)
	CreateProduct(ctx context.Context, product *Product) error
	UpdateProduct(ctx context.Context, product *Product) error
	UpdateProducts(ctx context.Context, products []*Product) error
	DeleteProduct(ctx context.Context, productID int) error
	ListProducts(ctx context.Context) ([]*Product, error)
}

// ProductService は、Write-Through パターンを実装するサービス
type ProductService struct {
	db      ProductRepository
	cache   CacheClient
	config  ServiceConfig
	metrics *WriteMetrics
}

// NewProductService は、新しい ProductService を作成します
func NewProductService(db ProductRepository, cache CacheClient) *ProductService {
	return &ProductService{
		db:     db,
		cache:  cache,
		config: defaultConfig(),
		metrics: &WriteMetrics{},
	}
}

// NewProductServiceWithConfig は、設定付きで新しい ProductService を作成します
func NewProductServiceWithConfig(db ProductRepository, cache CacheClient, config ServiceConfig) *ProductService {
	return &ProductService{
		db:      db,
		cache:   cache,
		config:  config,
		metrics: &WriteMetrics{},
	}
}

// GetProduct は、商品を取得します（Cache-Aside パターン）
func (s *ProductService) GetProduct(ctx context.Context, productID int) (*Product, error) {
	cacheKey := productCacheKey(productID)
	
	// キャッシュから取得を試行
	var product Product
	err := s.cache.GetJSON(ctx, cacheKey, &product)
	if err == nil {
		return &product, nil
	}
	
	// キャッシュミス - データベースから取得
	productPtr, err := s.db.GetProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	
	// キャッシュに保存
	s.cache.SetJSON(ctx, cacheKey, productPtr, ProductCacheTTL)
	
	return productPtr, nil
}

// CreateProduct は、商品を作成します（Write-Through パターン）
func (s *ProductService) CreateProduct(ctx context.Context, product *Product) error {
	product.UpdatedAt = time.Now()
	
	if s.config.StrictConsistency {
		return s.createWithStrictConsistency(ctx, product)
	} else {
		return s.createWithEventualConsistency(ctx, product)
	}
}

// UpdateProduct は、商品を更新します（Write-Through パターン）
func (s *ProductService) UpdateProduct(ctx context.Context, product *Product) error {
	product.UpdatedAt = time.Now()
	
	if s.config.StrictConsistency {
		return s.updateWithStrictConsistency(ctx, product)
	} else {
		return s.updateWithEventualConsistency(ctx, product)
	}
}

// DeleteProduct は、商品を削除します
func (s *ProductService) DeleteProduct(ctx context.Context, productID int) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.updateAvgWriteTime(duration)
	}()
	
	// データベースから削除
	err := s.db.DeleteProduct(ctx, productID)
	if err != nil {
		atomic.AddInt64(&s.metrics.WriteFailures, 1)
		return err
	}
	
	atomic.AddInt64(&s.metrics.DatabaseWrites, 1)
	
	// キャッシュからも削除
	cacheKey := productCacheKey(productID)
	err = s.cache.Delete(ctx, cacheKey)
	if err != nil {
		log.Printf("Cache deletion failed for product %d: %v", productID, err)
		// 削除の場合、キャッシュエラーは致命的ではない
	} else {
		atomic.AddInt64(&s.metrics.CacheWrites, 1)
	}
	
	// 関連キャッシュも削除
	s.cache.Delete(ctx, allProductsCacheKey())
	
	return nil
}

// BulkUpdateProducts は、複数の商品を一括更新します
func (s *ProductService) BulkUpdateProducts(ctx context.Context, products []*Product) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.updateAvgWriteTime(duration)
	}()
	
	// 更新時刻を設定
	now := time.Now()
	for _, product := range products {
		product.UpdatedAt = now
	}
	
	// データベースバルク更新
	err := s.db.UpdateProducts(ctx, products)
	if err != nil {
		atomic.AddInt64(&s.metrics.WriteFailures, 1)
		return err
	}
	
	atomic.AddInt64(&s.metrics.DatabaseWrites, int64(len(products)))
	
	// キャッシュバルク更新
	cacheData := make(map[string]interface{})
	for _, product := range products {
		cacheData[productCacheKey(product.ID)] = product
	}
	
	err = s.cache.SetMulti(ctx, cacheData, ProductCacheTTL)
	if err != nil {
		if s.config.StrictConsistency {
			atomic.AddInt64(&s.metrics.ConsistencyErrors, 1)
			return fmt.Errorf("bulk cache update failed: %w", err)
		}
		log.Printf("Bulk cache update failed: %v", err)
	} else {
		atomic.AddInt64(&s.metrics.CacheWrites, int64(len(products)))
	}
	
	// 関連キャッシュを無効化
	s.cache.Delete(ctx, allProductsCacheKey())
	
	return nil
}

// ListProducts は、すべての商品を取得します
func (s *ProductService) ListProducts(ctx context.Context) ([]*Product, error) {
	cacheKey := allProductsCacheKey()
	
	// キャッシュから取得を試行
	var products []*Product
	err := s.cache.GetJSON(ctx, cacheKey, &products)
	if err == nil {
		return products, nil
	}
	
	// キャッシュミス - データベースから取得
	products, err = s.db.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	
	// キャッシュに保存
	s.cache.SetJSON(ctx, cacheKey, products, ProductCacheTTL)
	
	return products, nil
}

// GetMetrics は、現在の書き込みメトリクスを返します
func (s *ProductService) GetMetrics() WriteMetrics {
	return WriteMetrics{
		DatabaseWrites:    atomic.LoadInt64(&s.metrics.DatabaseWrites),
		CacheWrites:       atomic.LoadInt64(&s.metrics.CacheWrites),
		WriteFailures:     atomic.LoadInt64(&s.metrics.WriteFailures),
		AvgWriteTime:      time.Duration(atomic.LoadInt64((*int64)(&s.metrics.AvgWriteTime))),
		ConsistencyErrors: atomic.LoadInt64(&s.metrics.ConsistencyErrors),
	}
}

// createWithStrictConsistency は、厳密な整合性モードでの作成を実行します
func (s *ProductService) createWithStrictConsistency(ctx context.Context, product *Product) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.updateAvgWriteTime(duration)
	}()
	
	// データベースに保存
	err := s.db.CreateProduct(ctx, product)
	if err != nil {
		atomic.AddInt64(&s.metrics.WriteFailures, 1)
		return err
	}
	
	atomic.AddInt64(&s.metrics.DatabaseWrites, 1)
	
	// キャッシュに保存（タイムアウト付き）
	cacheCtx, cancel := context.WithTimeout(ctx, s.config.CacheWriteTimeout)
	defer cancel()
	
	cacheKey := productCacheKey(product.ID)
	err = s.updateCacheWithRetry(cacheCtx, cacheKey, product)
	if err != nil {
		atomic.AddInt64(&s.metrics.ConsistencyErrors, 1)
		// 厳密な整合性モードでは、キャッシュ失敗時にエラーを返す
		return fmt.Errorf("cache write failed in strict mode: %w", err)
	}
	
	atomic.AddInt64(&s.metrics.CacheWrites, 1)
	
	// 関連キャッシュを無効化
	s.cache.Delete(ctx, allProductsCacheKey())
	
	return nil
}

// createWithEventualConsistency は、結果整合性モードでの作成を実行します
func (s *ProductService) createWithEventualConsistency(ctx context.Context, product *Product) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.updateAvgWriteTime(duration)
	}()
	
	// データベースに保存
	err := s.db.CreateProduct(ctx, product)
	if err != nil {
		atomic.AddInt64(&s.metrics.WriteFailures, 1)
		return err
	}
	
	atomic.AddInt64(&s.metrics.DatabaseWrites, 1)
	
	// キャッシュ更新を非同期で実行
	if s.config.AsyncCacheUpdate {
		go s.asyncCacheUpdate(product)
	} else {
		// 同期だが、エラーを無視
		cacheKey := productCacheKey(product.ID)
		err = s.cache.SetJSON(ctx, cacheKey, product, ProductCacheTTL)
		if err != nil {
			log.Printf("Cache write failed (eventual consistency): %v", err)
		} else {
			atomic.AddInt64(&s.metrics.CacheWrites, 1)
		}
	}
	
	// 関連キャッシュを無効化
	s.cache.Delete(ctx, allProductsCacheKey())
	
	return nil
}

// updateWithStrictConsistency は、厳密な整合性モードでの更新を実行します
func (s *ProductService) updateWithStrictConsistency(ctx context.Context, product *Product) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.updateAvgWriteTime(duration)
	}()
	
	// 並列でデータベースとキャッシュを更新
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
		cacheCtx, cancel := context.WithTimeout(ctx, s.config.CacheWriteTimeout)
		defer cancel()
		
		cacheKey := productCacheKey(product.ID)
		cacheErr = s.updateCacheWithRetry(cacheCtx, cacheKey, product)
	}()
	
	wg.Wait()
	
	// エラー処理
	if dbErr != nil {
		atomic.AddInt64(&s.metrics.WriteFailures, 1)
		return dbErr
	}
	
	atomic.AddInt64(&s.metrics.DatabaseWrites, 1)
	
	if cacheErr != nil {
		atomic.AddInt64(&s.metrics.ConsistencyErrors, 1)
		return fmt.Errorf("cache update failed in strict mode: %w", cacheErr)
	}
	
	atomic.AddInt64(&s.metrics.CacheWrites, 1)
	
	// 関連キャッシュを無効化
	s.cache.Delete(ctx, allProductsCacheKey())
	
	return nil
}

// updateWithEventualConsistency は、結果整合性モードでの更新を実行します
func (s *ProductService) updateWithEventualConsistency(ctx context.Context, product *Product) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		s.updateAvgWriteTime(duration)
	}()
	
	// データベース更新
	err := s.db.UpdateProduct(ctx, product)
	if err != nil {
		atomic.AddInt64(&s.metrics.WriteFailures, 1)
		return err
	}
	
	atomic.AddInt64(&s.metrics.DatabaseWrites, 1)
	
	// キャッシュ更新を非同期で実行
	if s.config.AsyncCacheUpdate {
		go s.asyncCacheUpdate(product)
	} else {
		// 同期だが、エラーを無視
		cacheKey := productCacheKey(product.ID)
		err = s.cache.SetJSON(ctx, cacheKey, product, ProductCacheTTL)
		if err != nil {
			log.Printf("Cache write failed (eventual consistency): %v", err)
		} else {
			atomic.AddInt64(&s.metrics.CacheWrites, 1)
		}
	}
	
	// 関連キャッシュを無効化
	s.cache.Delete(ctx, allProductsCacheKey())
	
	return nil
}

// updateCacheWithRetry は、再試行機能付きでキャッシュを更新します
func (s *ProductService) updateCacheWithRetry(ctx context.Context, key string, value interface{}) error {
	var lastErr error
	
	for i := 0; i < s.config.MaxRetries; i++ {
		err := s.cache.SetJSON(ctx, key, value, ProductCacheTTL)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// 最後の試行でなければ、バックオフ待機
		if i < s.config.MaxRetries-1 {
			backoff := time.Duration(i+1) * 100 * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	
	return fmt.Errorf("max retries (%d) exceeded: %w", s.config.MaxRetries, lastErr)
}

// asyncCacheUpdate は、非同期でキャッシュを更新します
func (s *ProductService) asyncCacheUpdate(product *Product) {
	ctx, cancel := context.WithTimeout(context.Background(), s.config.CacheWriteTimeout)
	defer cancel()
	
	cacheKey := productCacheKey(product.ID)
	err := s.updateCacheWithRetry(ctx, cacheKey, product)
	if err != nil {
		log.Printf("Async cache update failed for product %d: %v", product.ID, err)
		return
	}
	
	atomic.AddInt64(&s.metrics.CacheWrites, 1)
}

// updateAvgWriteTime は、平均書き込み時間を更新します
func (s *ProductService) updateAvgWriteTime(duration time.Duration) {
	// 簡単な移動平均を計算
	current := atomic.LoadInt64((*int64)(&s.metrics.AvgWriteTime))
	newAvg := (time.Duration(current) + duration) / 2
	atomic.StoreInt64((*int64)(&s.metrics.AvgWriteTime), int64(newAvg))
}

// GetWriteThroughEfficiency は、Write-Through の効率性を計算します
func (s *ProductService) GetWriteThroughEfficiency() float64 {
	metrics := s.GetMetrics()
	totalWrites := metrics.DatabaseWrites
	if totalWrites == 0 {
		return 0.0
	}
	
	successfulWrites := totalWrites - metrics.WriteFailures
	return float64(successfulWrites) / float64(totalWrites) * 100.0
}

// GetCacheConsistencyRate は、キャッシュ整合性率を計算します
func (s *ProductService) GetCacheConsistencyRate() float64 {
	metrics := s.GetMetrics()
	totalCacheOps := metrics.CacheWrites
	if totalCacheOps == 0 {
		return 0.0
	}
	
	successfulOps := totalCacheOps - metrics.ConsistencyErrors
	return float64(successfulOps) / float64(totalCacheOps) * 100.0
}

// RefreshProduct は、商品キャッシュを強制的に更新します
func (s *ProductService) RefreshProduct(ctx context.Context, productID int) (*Product, error) {
	// キャッシュを削除
	cacheKey := productCacheKey(productID)
	s.cache.Delete(ctx, cacheKey)
	
	// データベースから再読み込み
	product, err := s.db.GetProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	
	// 新しいデータをキャッシュに保存
	s.cache.SetJSON(ctx, cacheKey, product, ProductCacheTTL)
	
	return product, nil
}

// defaultConfig は、デフォルトの設定を返します
func defaultConfig() ServiceConfig {
	return ServiceConfig{
		StrictConsistency: true,
		AsyncCacheUpdate:  false,
		CacheWriteTimeout: 5 * time.Second,
		MaxRetries:        3,
	}
}

// productCacheKey は、商品キャッシュのキーを生成します
func productCacheKey(productID int) string {
	return fmt.Sprintf("product:%d", productID)
}

// allProductsCacheKey は、全商品リストのキャッシュキーを生成します
func allProductsCacheKey() string {
	return "products:all"
}

// categoryCacheKey は、カテゴリ別商品リストのキャッシュキーを生成します
func categoryCacheKey(category string) string {
	return fmt.Sprintf("products:category:%s", category)
}

// GetProductsByCategory は、カテゴリ別の商品を取得します
func (s *ProductService) GetProductsByCategory(ctx context.Context, category string) ([]*Product, error) {
	cacheKey := categoryCacheKey(category)
	
	// キャッシュから取得を試行
	var products []*Product
	err := s.cache.GetJSON(ctx, cacheKey, &products)
	if err == nil {
		return products, nil
	}
	
	// キャッシュミス - 全商品から該当カテゴリをフィルタ
	allProducts, err := s.ListProducts(ctx)
	if err != nil {
		return nil, err
	}
	
	products = make([]*Product, 0)
	for _, product := range allProducts {
		if product.Category == category {
			products = append(products, product)
		}
	}
	
	// カテゴリ別リストをキャッシュに保存
	s.cache.SetJSON(ctx, cacheKey, products, CategoryCacheTTL)
	
	return products, nil
}