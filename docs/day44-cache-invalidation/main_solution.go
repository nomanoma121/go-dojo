package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// InvalidationRule は、キャッシュ無効化のルールを定義する構造体
type InvalidationRule struct {
	Trigger   string        // トリガーとなるキー
	Targets   []string      // 無効化対象のキー/パターン
	Delay     time.Duration // 遅延時間
	Condition func() bool   // 実行条件
}

// InvalidationMetrics は、無効化操作の統計情報を保持する構造体
type InvalidationMetrics struct {
	TotalInvalidations   int64         // 総無効化数
	TagInvalidations     int64         // タグベース無効化数
	PatternInvalidations int64         // パターンマッチ無効化数
	FailedInvalidations  int64         // 失敗した無効化数
	AvgInvalidationTime  time.Duration // 平均無効化時間
}

// CacheClient は、キャッシュクライアントのインターフェース
type CacheClient interface {
	Delete(ctx context.Context, key string) error
	DeleteMulti(ctx context.Context, keys []string) error
	Exists(ctx context.Context, key string) (bool, error)
	SetTTL(ctx context.Context, key string, ttl time.Duration) error
	Scan(ctx context.Context, pattern string) ([]string, error)
}

// TagStore は、タグとキーの関連付けを管理するインターフェース
type TagStore interface {
	AddTag(ctx context.Context, key, tag string) error
	GetKeysByTag(ctx context.Context, tag string) ([]string, error)
	RemoveTag(ctx context.Context, key, tag string) error
	RemoveAllTags(ctx context.Context, key string) error
}

// RuleEngine は、無効化ルールを管理するインターフェース
type RuleEngine interface {
	AddRule(rule InvalidationRule) error
	ExecuteRules(ctx context.Context, triggerKey string) error
	RemoveRule(trigger string) error
}

// CacheInvalidator は、様々な無効化戦略を実装するメイン構造体
type CacheInvalidator struct {
	cache      CacheClient
	tagStore   TagStore
	ruleEngine RuleEngine
	metrics    *InvalidationMetrics
}

// MemoryTagStore は、メモリベースのタグストア実装
type MemoryTagStore struct {
	tagToKeys map[string]map[string]bool
	keyToTags map[string]map[string]bool
	mutex     sync.RWMutex
}

// NewMemoryTagStore は、新しいメモリタグストアを作成します
func NewMemoryTagStore() *MemoryTagStore {
	return &MemoryTagStore{
		tagToKeys: make(map[string]map[string]bool),
		keyToTags: make(map[string]map[string]bool),
	}
}

func (m *MemoryTagStore) AddTag(ctx context.Context, key, tag string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.tagToKeys[tag] == nil {
		m.tagToKeys[tag] = make(map[string]bool)
	}
	m.tagToKeys[tag][key] = true
	
	if m.keyToTags[key] == nil {
		m.keyToTags[key] = make(map[string]bool)
	}
	m.keyToTags[key][tag] = true
	
	return nil
}

func (m *MemoryTagStore) GetKeysByTag(ctx context.Context, tag string) ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	keys := make([]string, 0)
	if keyMap, exists := m.tagToKeys[tag]; exists {
		for key := range keyMap {
			keys = append(keys, key)
		}
	}
	
	return keys, nil
}

func (m *MemoryTagStore) RemoveTag(ctx context.Context, key, tag string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if keyMap, exists := m.tagToKeys[tag]; exists {
		delete(keyMap, key)
		if len(keyMap) == 0 {
			delete(m.tagToKeys, tag)
		}
	}
	
	if tagMap, exists := m.keyToTags[key]; exists {
		delete(tagMap, tag)
		if len(tagMap) == 0 {
			delete(m.keyToTags, key)
		}
	}
	
	return nil
}

func (m *MemoryTagStore) RemoveAllTags(ctx context.Context, key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if tagMap, exists := m.keyToTags[key]; exists {
		for tag := range tagMap {
			if keyMap, exists := m.tagToKeys[tag]; exists {
				delete(keyMap, key)
				if len(keyMap) == 0 {
					delete(m.tagToKeys, tag)
				}
			}
		}
		delete(m.keyToTags, key)
	}
	
	return nil
}

// MemoryRuleEngine は、メモリベースのルールエンジン実装
type MemoryRuleEngine struct {
	rules      map[string][]InvalidationRule
	invalidator *CacheInvalidator
	mutex      sync.RWMutex
}

// NewMemoryRuleEngine は、新しいメモリルールエンジンを作成します
func NewMemoryRuleEngine() *MemoryRuleEngine {
	return &MemoryRuleEngine{
		rules: make(map[string][]InvalidationRule),
	}
}

func (m *MemoryRuleEngine) SetInvalidator(invalidator *CacheInvalidator) {
	m.invalidator = invalidator
}

func (m *MemoryRuleEngine) AddRule(rule InvalidationRule) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.rules[rule.Trigger] = append(m.rules[rule.Trigger], rule)
	return nil
}

func (m *MemoryRuleEngine) ExecuteRules(ctx context.Context, triggerKey string) error {
	m.mutex.RLock()
	rules, exists := m.rules[triggerKey]
	m.mutex.RUnlock()
	
	if !exists {
		return nil
	}
	
	for _, rule := range rules {
		if rule.Condition != nil && !rule.Condition() {
			continue
		}
		
		if rule.Delay > 0 {
			// 非同期で遅延実行
			go func(r InvalidationRule) {
				time.Sleep(r.Delay)
				m.executeRule(ctx, r)
			}(rule)
		} else {
			m.executeRule(ctx, rule)
		}
	}
	
	return nil
}

func (m *MemoryRuleEngine) executeRule(ctx context.Context, rule InvalidationRule) {
	if m.invalidator == nil {
		return
	}
	
	for _, target := range rule.Targets {
		// パターンかどうかをチェック
		if containsWildcard(target) {
			m.invalidator.InvalidateByPattern(ctx, target)
		} else {
			m.invalidator.InvalidateByKey(ctx, target)
		}
	}
}

func (m *MemoryRuleEngine) RemoveRule(trigger string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	delete(m.rules, trigger)
	return nil
}

// NewCacheInvalidator は、新しい CacheInvalidator を作成します
func NewCacheInvalidator(cache CacheClient, tagStore TagStore, ruleEngine RuleEngine) *CacheInvalidator {
	invalidator := &CacheInvalidator{
		cache:      cache,
		tagStore:   tagStore,
		ruleEngine: ruleEngine,
		metrics:    &InvalidationMetrics{},
	}
	
	// ルールエンジンに自身への参照を設定（循環参照対応）
	if memEngine, ok := ruleEngine.(*MemoryRuleEngine); ok {
		memEngine.SetInvalidator(invalidator)
	}
	
	return invalidator
}

// InvalidateByKey は、指定されたキーを無効化します
func (c *CacheInvalidator) InvalidateByKey(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		c.updateAvgInvalidationTime(duration)
	}()
	
	// キーを削除
	err := c.cache.Delete(ctx, key)
	if err != nil {
		atomic.AddInt64(&c.metrics.FailedInvalidations, 1)
		return err
	}
	
	atomic.AddInt64(&c.metrics.TotalInvalidations, 1)
	
	// 関連するタグも削除
	c.tagStore.RemoveAllTags(ctx, key)
	
	// 無効化ルールを実行
	c.ruleEngine.ExecuteRules(ctx, key)
	
	return nil
}

// InvalidateByTag は、指定されたタグに関連するすべてのキーを無効化します
func (c *CacheInvalidator) InvalidateByTag(ctx context.Context, tag string) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		c.updateAvgInvalidationTime(duration)
	}()
	
	// タグに関連するキーを取得
	keys, err := c.tagStore.GetKeysByTag(ctx, tag)
	if err != nil {
		atomic.AddInt64(&c.metrics.FailedInvalidations, 1)
		return err
	}
	
	if len(keys) == 0 {
		return nil
	}
	
	// 一括削除
	err = c.cache.DeleteMulti(ctx, keys)
	if err != nil {
		atomic.AddInt64(&c.metrics.FailedInvalidations, 1)
		return err
	}
	
	atomic.AddInt64(&c.metrics.TotalInvalidations, int64(len(keys)))
	atomic.AddInt64(&c.metrics.TagInvalidations, 1)
	
	// タグ情報をクリア
	for _, key := range keys {
		c.tagStore.RemoveAllTags(ctx, key)
	}
	
	return nil
}

// InvalidateByPattern は、パターンにマッチするキーを無効化します
func (c *CacheInvalidator) InvalidateByPattern(ctx context.Context, pattern string) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		c.updateAvgInvalidationTime(duration)
	}()
	
	// パターンにマッチするキーを検索
	keys, err := c.cache.Scan(ctx, pattern)
	if err != nil {
		atomic.AddInt64(&c.metrics.FailedInvalidations, 1)
		return err
	}
	
	if len(keys) == 0 {
		return nil
	}
	
	// 一括削除
	err = c.cache.DeleteMulti(ctx, keys)
	if err != nil {
		atomic.AddInt64(&c.metrics.FailedInvalidations, 1)
		return err
	}
	
	atomic.AddInt64(&c.metrics.TotalInvalidations, int64(len(keys)))
	atomic.AddInt64(&c.metrics.PatternInvalidations, 1)
	
	// タグ情報をクリア
	for _, key := range keys {
		c.tagStore.RemoveAllTags(ctx, key)
	}
	
	return nil
}

// InvalidateRelated は、指定されたキーに関連するデータを無効化します
func (c *CacheInvalidator) InvalidateRelated(ctx context.Context, key string) error {
	// まず指定されたキーを無効化
	err := c.InvalidateByKey(ctx, key)
	if err != nil {
		return err
	}
	
	// 関連パターンの無効化（例：user:123 → user:123:*）
	relatedPattern := key + ":*"
	return c.InvalidateByPattern(ctx, relatedPattern)
}

// SetTTL は、指定されたキーのTTLを更新します
func (c *CacheInvalidator) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	return c.cache.SetTTL(ctx, key, ttl)
}

// AddInvalidationRule は、新しい無効化ルールを追加します
func (c *CacheInvalidator) AddInvalidationRule(rule InvalidationRule) error {
	return c.ruleEngine.AddRule(rule)
}

// GetMetrics は、現在の無効化メトリクスを返します
func (c *CacheInvalidator) GetMetrics() InvalidationMetrics {
	return InvalidationMetrics{
		TotalInvalidations:   atomic.LoadInt64(&c.metrics.TotalInvalidations),
		TagInvalidations:     atomic.LoadInt64(&c.metrics.TagInvalidations),
		PatternInvalidations: atomic.LoadInt64(&c.metrics.PatternInvalidations),
		FailedInvalidations:  atomic.LoadInt64(&c.metrics.FailedInvalidations),
		AvgInvalidationTime:  time.Duration(atomic.LoadInt64((*int64)(&c.metrics.AvgInvalidationTime))),
	}
}

// BatchInvalidate は、複数のキーを効率的に無効化します
func (c *CacheInvalidator) BatchInvalidate(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		c.updateAvgInvalidationTime(duration)
	}()
	
	err := c.cache.DeleteMulti(ctx, keys)
	if err != nil {
		atomic.AddInt64(&c.metrics.FailedInvalidations, 1)
		return err
	}
	
	atomic.AddInt64(&c.metrics.TotalInvalidations, int64(len(keys)))
	
	// タグ情報をクリア
	for _, key := range keys {
		c.tagStore.RemoveAllTags(ctx, key)
	}
	
	return nil
}

// InvalidateHierarchy は、階層的なキーを無効化します
func (c *CacheInvalidator) InvalidateHierarchy(ctx context.Context, baseKey string) error {
	// 自分自身を無効化
	err := c.InvalidateByKey(ctx, baseKey)
	if err != nil {
		return err
	}
	
	// 子要素を無効化
	childPattern := baseKey + ":*"
	err = c.InvalidateByPattern(ctx, childPattern)
	if err != nil {
		return err
	}
	
	// 親要素のリストキャッシュなどを無効化
	parentPattern := extractParentPattern(baseKey)
	if parentPattern != "" {
		return c.InvalidateByPattern(ctx, parentPattern)
	}
	
	return nil
}

// ScheduleInvalidation は、指定された時間後に無効化を実行します
func (c *CacheInvalidator) ScheduleInvalidation(ctx context.Context, key string, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		c.InvalidateByKey(ctx, key)
	}()
}

// GetInvalidationEfficiency は、無効化の効率性を計算します
func (c *CacheInvalidator) GetInvalidationEfficiency() float64 {
	metrics := c.GetMetrics()
	total := metrics.TotalInvalidations
	if total == 0 {
		return 0.0
	}
	
	successful := total - metrics.FailedInvalidations
	return float64(successful) / float64(total) * 100.0
}

// updateAvgInvalidationTime は、平均無効化時間を更新します
func (c *CacheInvalidator) updateAvgInvalidationTime(duration time.Duration) {
	current := atomic.LoadInt64((*int64)(&c.metrics.AvgInvalidationTime))
	newAvg := (time.Duration(current) + duration) / 2
	atomic.StoreInt64((*int64)(&c.metrics.AvgInvalidationTime), int64(newAvg))
}

// containsWildcard は、文字列にワイルドカードが含まれているかチェックします
func containsWildcard(pattern string) bool {
	return fmt.Sprintf("%s", pattern) != pattern || 
		   containsChar(pattern, '*') || 
		   containsChar(pattern, '?')
}

// containsChar は、文字列に指定された文字が含まれているかチェックします
func containsChar(s string, c rune) bool {
	for _, r := range s {
		if r == c {
			return true
		}
	}
	return false
}

// extractParentPattern は、キーから親パターンを抽出します
func extractParentPattern(key string) string {
	// 簡単な実装：最後の : より前の部分
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == ':' {
			return key[:i] + ":*"
		}
	}
	return ""
}