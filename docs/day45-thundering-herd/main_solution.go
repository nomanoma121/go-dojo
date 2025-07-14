// Day 45: Thundering Herd Problem Prevention
// キャッシュミス時の大量同時アクセスを防ぐ対策システムの実装

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

// エラー定義
var (
	ErrLockNotAcquired = errors.New("lock not acquired")
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrNotFound        = errors.New("data not found")
)

// Data はキャッシュされるデータ構造です
type Data struct {
	ID        string    `json:"id"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

// Circuit Breaker の状態
type CircuitState int

const (
	Closed CircuitState = iota
	Open
	HalfOpen
)

// CircuitBreaker はデータベース保護のための回路遮断器です
type CircuitBreaker struct {
	state       CircuitState
	failures    int64
	threshold   int64
	timeout     time.Duration
	lastFailure time.Time
	mutex       sync.RWMutex
}

// DistributedLock は分散ロックを表します
type DistributedLock struct {
	client LockClient
	key    string
	value  string
	ttl    time.Duration
}

// ProtectionMetrics は対策の効果を測定するメトリクスです
type ProtectionMetrics struct {
	TotalRequests       int64
	CacheHits           int64
	CacheMisses         int64
	SingleFlightHits    int64
	LockAcquisitions    int64
	CircuitBreakerTrips int64
	StaleReturns        int64
	BackgroundRefresh   int64
}

// ThunderingHerdProtector はThundering Herd対策の統合システムです
type ThunderingHerdProtector struct {
	cache          CacheClient
	db             DataRepository
	sf             *singleflight.Group
	lockManager    LockManager
	circuitBreaker *CircuitBreaker
	metrics        *ProtectionMetrics
	jitterPercent  float64
}

// インターフェース定義
type CacheClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	GetWithTTL(ctx context.Context, key string) (string, time.Duration, error)
	Pipeline() Pipeline
}

type Pipeline interface {
	TTL(ctx context.Context, key string) TTLCmd
	Get(ctx context.Context, key string) GetCmd
	Exec(ctx context.Context) ([]Result, error)
}

type TTLCmd interface {
	Val() time.Duration
}

type GetCmd interface {
	Val() string
}

type Result interface{}

type DataRepository interface {
	GetByID(ctx context.Context, id string) (*Data, error)
	Create(ctx context.Context, data *Data) error
}

type LockManager interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (Lock, error)
}

type LockClient interface {
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) error
}

type Lock interface {
	Release(ctx context.Context) error
}

// NewThunderingHerdProtector プロテクターを初期化します
func NewThunderingHerdProtector(
	cache CacheClient,
	db DataRepository,
	lockManager LockManager,
	circuitBreakerThreshold int64,
	circuitBreakerTimeout time.Duration,
	jitterPercent float64,
) *ThunderingHerdProtector {
	return &ThunderingHerdProtector{
		cache:          cache,
		db:             db,
		sf:             &singleflight.Group{},
		lockManager:    lockManager,
		circuitBreaker: NewCircuitBreaker(circuitBreakerThreshold, circuitBreakerTimeout),
		metrics:        &ProtectionMetrics{},
		jitterPercent:  jitterPercent,
	}
}

// Get Single Flight、分散ロック、Circuit Breakerを組み合わせたデータ取得
func (p *ThunderingHerdProtector) Get(ctx context.Context, key string) (*Data, error) {
	p.recordMetric(&p.metrics.TotalRequests)

	// 1. 通常のキャッシュアクセス
	if data, err := p.getFromCache(ctx, key); err == nil {
		p.recordMetric(&p.metrics.CacheHits)
		return data, nil
	}
	p.recordMetric(&p.metrics.CacheMisses)

	// 2. Single Flight で重複リクエストを統合
	v, err, shared := p.sf.Do(key, func() (interface{}, error) {
		return p.getWithProtection(ctx, key)
	})

	if shared {
		p.recordMetric(&p.metrics.SingleFlightHits)
	}

	if err != nil {
		return nil, err
	}

	return v.(*Data), nil
}

// Set TTLジッターを追加してキャッシュに設定
func (p *ThunderingHerdProtector) Set(ctx context.Context, key string, value *Data, ttl time.Duration) error {
	// TTLにジッターを追加
	actualTTL := addJitter(ttl, p.jitterPercent)

	// データをJSONにシリアライズ
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return p.cache.Set(ctx, key, string(jsonData), actualTTL)
}

// GetStaleWhileRevalidate 期限切れデータを返しながらバックグラウンド更新
func (p *ThunderingHerdProtector) GetStaleWhileRevalidate(ctx context.Context, key string) (*Data, error) {
	p.recordMetric(&p.metrics.TotalRequests)

	data, isStale, err := p.getWithStaleness(ctx, key)
	if err == nil {
		if isStale {
			p.recordMetric(&p.metrics.StaleReturns)
			// バックグラウンドで更新を開始
			go p.refreshInBackground(key)
		} else {
			p.recordMetric(&p.metrics.CacheHits)
		}
		return data, nil
	}

	p.recordMetric(&p.metrics.CacheMisses)
	// キャッシュミスの場合は通常通り取得
	return p.Get(ctx, key)
}

// GetMetrics 現在のメトリクスを返す
func (p *ThunderingHerdProtector) GetMetrics() ProtectionMetrics {
	return ProtectionMetrics{
		TotalRequests:       atomic.LoadInt64(&p.metrics.TotalRequests),
		CacheHits:           atomic.LoadInt64(&p.metrics.CacheHits),
		CacheMisses:         atomic.LoadInt64(&p.metrics.CacheMisses),
		SingleFlightHits:    atomic.LoadInt64(&p.metrics.SingleFlightHits),
		LockAcquisitions:    atomic.LoadInt64(&p.metrics.LockAcquisitions),
		CircuitBreakerTrips: atomic.LoadInt64(&p.metrics.CircuitBreakerTrips),
		StaleReturns:        atomic.LoadInt64(&p.metrics.StaleReturns),
		BackgroundRefresh:   atomic.LoadInt64(&p.metrics.BackgroundRefresh),
	}
}

// getFromCache キャッシュからデータを取得
func (p *ThunderingHerdProtector) getFromCache(ctx context.Context, key string) (*Data, error) {
	value, err := p.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var data Data
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return &data, nil
}

// getWithStaleness データと期限切れ情報を同時に取得
func (p *ThunderingHerdProtector) getWithStaleness(ctx context.Context, key string) (*Data, bool, error) {
	value, ttl, err := p.cache.GetWithTTL(ctx, key)
	if err != nil {
		return nil, false, err
	}

	// TTL が 0 以下の場合は期限切れ
	isStale := ttl <= 0

	var data Data
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return &data, isStale, nil
}

// loadFromDB データベースからデータを取得してキャッシュに保存
func (p *ThunderingHerdProtector) loadFromDB(ctx context.Context, key string) (*Data, error) {
	data, err := p.db.GetByID(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load from DB: %w", err)
	}

	// キャッシュに保存（デフォルトTTL: 5分）
	if err := p.Set(ctx, key, data, 5*time.Minute); err != nil {
		// キャッシュ保存エラーはログ出力のみで、データは返す
		fmt.Printf("Warning: failed to cache data: %v\n", err)
	}

	return data, nil
}

// getWithProtection 分散ロックとCircuit Breakerを使用した保護付きデータ取得
func (p *ThunderingHerdProtector) getWithProtection(ctx context.Context, key string) (*Data, error) {
	lockKey := "lock:" + key

	// 分散ロック取得試行
	lock, err := p.lockManager.TryLock(ctx, lockKey, 5*time.Second)
	if err != nil {
		// ロック取得失敗 - 代替戦略実行
		return p.fallbackStrategy(ctx, key)
	}
	defer lock.Release(ctx)

	p.recordMetric(&p.metrics.LockAcquisitions)

	// ロック取得後、再度キャッシュ確認
	if data, err := p.getFromCache(ctx, key); err == nil {
		return data, nil
	}

	// Circuit Breaker でDB保護
	result, err := p.circuitBreaker.Call(func() (interface{}, error) {
		return p.loadFromDB(ctx, key)
	})

	if err == ErrCircuitOpen {
		p.recordMetric(&p.metrics.CircuitBreakerTrips)
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return result.(*Data), nil
}

// fallbackStrategy ロック取得失敗時の代替戦略
func (p *ThunderingHerdProtector) fallbackStrategy(ctx context.Context, key string) (*Data, error) {
	// 短時間待機後、キャッシュ再確認
	select {
	case <-time.After(10 * time.Millisecond):
		if data, err := p.getFromCache(ctx, key); err == nil {
			return data, nil
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 古いデータがあれば返す
	if data, _, err := p.getWithStaleness(ctx, key); err == nil {
		p.recordMetric(&p.metrics.StaleReturns)
		return data, nil
	}

	// 最後の手段：Circuit Breaker 経由でDB直接アクセス
	result, err := p.circuitBreaker.Call(func() (interface{}, error) {
		return p.loadFromDB(ctx, key)
	})

	if err == ErrCircuitOpen {
		p.recordMetric(&p.metrics.CircuitBreakerTrips)
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return result.(*Data), nil
}

// refreshInBackground バックグラウンドでデータを更新
func (p *ThunderingHerdProtector) refreshInBackground(key string) {
	p.recordMetric(&p.metrics.BackgroundRefresh)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 分散ロックを使用してリフレッシュの重複を防ぐ
	lockKey := "refresh:" + key
	lock, err := p.lockManager.TryLock(ctx, lockKey, 30*time.Second)
	if err != nil {
		// 他のプロセスがリフレッシュ中
		return
	}
	defer lock.Release(ctx)

	// データベースから最新データを取得
	data, err := p.db.GetByID(ctx, key)
	if err != nil {
		fmt.Printf("Background refresh failed for key %s: %v\n", key, err)
		return
	}

	// キャッシュを更新
	if err := p.Set(ctx, key, data, 5*time.Minute); err != nil {
		fmt.Printf("Failed to update cache during background refresh: %v\n", err)
	}
}

// recordMetric アトミックにメトリクスを記録
func (p *ThunderingHerdProtector) recordMetric(metric *int64) {
	atomic.AddInt64(metric, 1)
}

// addJitter TTLにランダムなジッターを追加
func addJitter(baseTTL time.Duration, jitterPercent float64) time.Duration {
	if jitterPercent <= 0 {
		return baseTTL
	}

	// ±jitterPercent のランダムな値を生成
	maxJitter := int64(float64(baseTTL) * jitterPercent)
	if maxJitter == 0 {
		return baseTTL
	}

	jitter, err := rand.Int(rand.Reader, big.NewInt(maxJitter*2))
	if err != nil {
		return baseTTL
	}

	actualJitter := jitter.Int64() - maxJitter
	return baseTTL + time.Duration(actualJitter)
}

// Circuit Breaker 実装

// NewCircuitBreaker 新しいCircuit Breakerを作成
func NewCircuitBreaker(threshold int64, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:     Closed,
		threshold: threshold,
		timeout:   timeout,
	}
}

// Call Circuit Breakerを通してリクエストを実行
func (cb *CircuitBreaker) Call(fn func() (interface{}, error)) (interface{}, error) {
	if !cb.canExecute() {
		return nil, ErrCircuitOpen
	}

	result, err := fn()
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return result, err
}

// recordFailure 失敗を記録
func (cb *CircuitBreaker) recordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = Open
	}
}

// recordSuccess 成功を記録
func (cb *CircuitBreaker) recordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.failures = 0
	cb.state = Closed
}

// canExecute 実行可能かどうかを判定
func (cb *CircuitBreaker) canExecute() bool {
	cb.mutex.RLock()
	state := cb.state
	lastFailure := cb.lastFailure
	cb.mutex.RUnlock()

	switch state {
	case Closed:
		return true
	case Open:
		if time.Since(lastFailure) > cb.timeout {
			cb.mutex.Lock()
			defer cb.mutex.Unlock()
			
			// ダブルチェック
			if cb.state == Open && time.Since(cb.lastFailure) > cb.timeout {
				cb.state = HalfOpen
				return true
			}
		}
		return false
	case HalfOpen:
		return true
	}
	return false
}

// Distributed Lock 実装

// NewDistributedLock 新しい分散ロックを作成
func NewDistributedLock(client LockClient, key string, ttl time.Duration) *DistributedLock {
	return &DistributedLock{
		client: client,
		key:    key,
		value:  generateLockValue(),
		ttl:    ttl,
	}
}

// Acquire Redis SETNXを使用してロックを取得
func (l *DistributedLock) Acquire(ctx context.Context) error {
	acquired, err := l.client.SetNX(ctx, l.key, l.value, l.ttl)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		return ErrLockNotAcquired
	}

	return nil
}

// Release Luaスクリプトを使用して安全にロックを解放
func (l *DistributedLock) Release(ctx context.Context) error {
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	return l.client.Eval(ctx, script, []string{l.key}, l.value)
}

// generateLockValue ユニークなロック値を生成
func generateLockValue() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func main() {
	fmt.Println("Day 45: Thundering Herd Problem Prevention")
	fmt.Println("Run 'go test -v' to see the protection system in action")
}