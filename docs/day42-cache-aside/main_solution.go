package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

// ErrCacheMiss は、キャッシュにキーが存在しない場合のエラー
var ErrCacheMiss = errors.New("cache miss")

// ErrUserNotFound は、ユーザーが存在しない場合のエラー
var ErrUserNotFound = errors.New("user not found")

// TTL 定数
const (
	UserCacheTTL     = 1 * time.Hour    // ユーザー個別キャッシュ
	UserListCacheTTL = 30 * time.Minute // ユーザーリストキャッシュ
)

// User は、ユーザー情報を表す構造体
type User struct {
	ID        int       `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Email     string    `json:"email" db:"email"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ServiceMetrics は、サービスの統計情報を保持する構造体
type ServiceMetrics struct {
	CacheHits   int64         // キャッシュヒット数
	CacheMisses int64         // キャッシュミス数
	DBQueries   int64         // データベースクエリ数
	SharedLoads int64         // Single Flight で統合されたリクエスト数
	AvgLoadTime time.Duration // 平均読み込み時間
}

// CacheClient は、キャッシュクライアントのインターフェース
type CacheClient interface {
	GetJSON(ctx context.Context, key string, dest interface{}) error
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// UserRepository は、ユーザーデータベース操作のインターフェース
type UserRepository interface {
	GetUser(ctx context.Context, userID int) (*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, userID int) error
	ListUsers(ctx context.Context) ([]*User, error)
}

// UserService は、Cache-Aside パターンを実装するサービス
type UserService struct {
	db      UserRepository
	cache   CacheClient
	sf      *singleflight.Group
	metrics *ServiceMetrics
}

// NewUserService は、新しい UserService を作成します
func NewUserService(db UserRepository, cache CacheClient) *UserService {
	return &UserService{
		db:      db,
		cache:   cache,
		sf:      &singleflight.Group{},
		metrics: &ServiceMetrics{},
	}
}

// GetUser は、Cache-Aside パターンでユーザーを取得します
func (s *UserService) GetUser(ctx context.Context, userID int) (*User, error) {
	cacheKey := userCacheKey(userID)
	
	// 1. キャッシュから取得を試行
	var user User
	err := s.cache.GetJSON(ctx, cacheKey, &user)
	if err == nil {
		// キャッシュヒット
		atomic.AddInt64(&s.metrics.CacheHits, 1)
		return &user, nil
	}
	
	// キャッシュミス - Single Flight でDB アクセスを統合
	v, err, shared := s.sf.Do(cacheKey, func() (interface{}, error) {
		return s.loadUserFromDB(ctx, userID)
	})
	
	if err != nil {
		return nil, err
	}
	
	userPtr := v.(*User)
	
	// Single Flight で統合された場合のメトリクス更新
	if shared {
		atomic.AddInt64(&s.metrics.SharedLoads, 1)
		atomic.AddInt64(&s.metrics.CacheHits, 1) // 他のリクエストのために読み込まれたデータ
	}
	
	return userPtr, nil
}

// CreateUser は、ユーザーを作成します
func (s *UserService) CreateUser(ctx context.Context, user *User) error {
	// データベースに保存
	err := s.db.CreateUser(ctx, user)
	if err != nil {
		return err
	}
	
	// 作成されたユーザーをキャッシュに保存
	cacheKey := userCacheKey(user.ID)
	s.cache.SetJSON(ctx, cacheKey, user, UserCacheTTL)
	
	// ユーザーリストキャッシュを無効化
	s.cache.Delete(ctx, allUsersCacheKey())
	
	return nil
}

// UpdateUser は、ユーザー情報を更新します
func (s *UserService) UpdateUser(ctx context.Context, user *User) error {
	// データベースを更新
	err := s.db.UpdateUser(ctx, user)
	if err != nil {
		return err
	}
	
	// キャッシュを無効化
	cacheKey := userCacheKey(user.ID)
	s.cache.Delete(ctx, cacheKey)
	
	// 関連キャッシュも無効化
	s.cache.Delete(ctx, allUsersCacheKey())
	
	return nil
}

// DeleteUser は、ユーザーを削除します
func (s *UserService) DeleteUser(ctx context.Context, userID int) error {
	// データベースから削除
	err := s.db.DeleteUser(ctx, userID)
	if err != nil {
		return err
	}
	
	// キャッシュからも削除
	cacheKey := userCacheKey(userID)
	s.cache.Delete(ctx, cacheKey)
	
	// ユーザーリストキャッシュも無効化
	s.cache.Delete(ctx, allUsersCacheKey())
	
	return nil
}

// ListUsers は、すべてのユーザーを取得します
func (s *UserService) ListUsers(ctx context.Context) ([]*User, error) {
	cacheKey := allUsersCacheKey()
	
	// キャッシュから取得を試行
	var users []*User
	err := s.cache.GetJSON(ctx, cacheKey, &users)
	if err == nil {
		// キャッシュヒット
		atomic.AddInt64(&s.metrics.CacheHits, 1)
		return users, nil
	}
	
	// キャッシュミス - データベースから取得
	atomic.AddInt64(&s.metrics.CacheMisses, 1)
	atomic.AddInt64(&s.metrics.DBQueries, 1)
	
	start := time.Now()
	users, err = s.db.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	loadTime := time.Since(start)
	
	// 平均読み込み時間を更新（簡単な移動平均）
	currentAvg := atomic.LoadInt64((*int64)(&s.metrics.AvgLoadTime))
	newAvg := (time.Duration(currentAvg) + loadTime) / 2
	atomic.StoreInt64((*int64)(&s.metrics.AvgLoadTime), int64(newAvg))
	
	// キャッシュに保存
	s.cache.SetJSON(ctx, cacheKey, users, UserListCacheTTL)
	
	return users, nil
}

// GetMetrics は、現在のサービスメトリクスを返します
func (s *UserService) GetMetrics() ServiceMetrics {
	return ServiceMetrics{
		CacheHits:   atomic.LoadInt64(&s.metrics.CacheHits),
		CacheMisses: atomic.LoadInt64(&s.metrics.CacheMisses),
		DBQueries:   atomic.LoadInt64(&s.metrics.DBQueries),
		SharedLoads: atomic.LoadInt64(&s.metrics.SharedLoads),
		AvgLoadTime: time.Duration(atomic.LoadInt64((*int64)(&s.metrics.AvgLoadTime))),
	}
}

// GetHitRate は、キャッシュヒット率を計算します
func (s *UserService) GetHitRate() float64 {
	hits := atomic.LoadInt64(&s.metrics.CacheHits)
	misses := atomic.LoadInt64(&s.metrics.CacheMisses)
	total := hits + misses
	
	if total == 0 {
		return 0.0
	}
	
	return float64(hits) / float64(total) * 100.0
}

// loadUserFromDB は、データベースからユーザーを読み込みます（内部メソッド）
func (s *UserService) loadUserFromDB(ctx context.Context, userID int) (*User, error) {
	atomic.AddInt64(&s.metrics.CacheMisses, 1)
	atomic.AddInt64(&s.metrics.DBQueries, 1)
	
	start := time.Now()
	user, err := s.db.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	loadTime := time.Since(start)
	
	// 平均読み込み時間を更新
	currentAvg := atomic.LoadInt64((*int64)(&s.metrics.AvgLoadTime))
	newAvg := (time.Duration(currentAvg) + loadTime) / 2
	atomic.StoreInt64((*int64)(&s.metrics.AvgLoadTime), int64(newAvg))
	
	// キャッシュに保存
	cacheKey := userCacheKey(userID)
	s.cache.SetJSON(ctx, cacheKey, user, UserCacheTTL)
	
	return user, nil
}

// GetUserWithFallback は、キャッシュエラー時のフォールバック付きでユーザーを取得します
func (s *UserService) GetUserWithFallback(ctx context.Context, userID int) (*User, error) {
	cacheKey := userCacheKey(userID)
	
	// キャッシュから取得を試行
	var user User
	err := s.cache.GetJSON(ctx, cacheKey, &user)
	if err == nil {
		atomic.AddInt64(&s.metrics.CacheHits, 1)
		return &user, nil
	}
	
	// キャッシュエラーの場合、直接データベースから取得
	if err != ErrCacheMiss {
		// キャッシュサービスの問題 - フォールバック
		atomic.AddInt64(&s.metrics.DBQueries, 1)
		return s.db.GetUser(ctx, userID)
	}
	
	// 通常のキャッシュミス処理
	return s.GetUser(ctx, userID)
}

// WarmCache は、指定されたユーザーIDリストでキャッシュを事前ロードします
func (s *UserService) WarmCache(ctx context.Context, userIDs []int) error {
	for _, userID := range userIDs {
		cacheKey := userCacheKey(userID)
		
		// すでにキャッシュされているかチェック
		exists, err := s.cache.Exists(ctx, cacheKey)
		if err != nil || exists {
			continue
		}
		
		// データベースから取得してキャッシュに保存
		user, err := s.db.GetUser(ctx, userID)
		if err != nil {
			continue // エラーがあっても他のユーザーの処理を続ける
		}
		
		s.cache.SetJSON(ctx, cacheKey, user, UserCacheTTL)
	}
	
	return nil
}

// RefreshUser は、ユーザーキャッシュを強制的に更新します
func (s *UserService) RefreshUser(ctx context.Context, userID int) (*User, error) {
	// キャッシュを削除
	cacheKey := userCacheKey(userID)
	s.cache.Delete(ctx, cacheKey)
	
	// データベースから再読み込み
	user, err := s.db.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	
	// 新しいデータをキャッシュに保存
	s.cache.SetJSON(ctx, cacheKey, user, UserCacheTTL)
	
	return user, nil
}

// userCacheKey は、ユーザーキャッシュのキーを生成します
func userCacheKey(userID int) string {
	return fmt.Sprintf("user:%d", userID)
}

// allUsersCacheKey は、全ユーザーリストのキャッシュキーを生成します
func allUsersCacheKey() string {
	return "users:all"
}

// GetUsersByIDs は、複数のユーザーIDから効率的にユーザーを取得します
func (s *UserService) GetUsersByIDs(ctx context.Context, userIDs []int) ([]*User, error) {
	users := make([]*User, 0, len(userIDs))
	missedIDs := make([]int, 0)
	
	// まずキャッシュから取得
	for _, userID := range userIDs {
		cacheKey := userCacheKey(userID)
		var user User
		err := s.cache.GetJSON(ctx, cacheKey, &user)
		if err == nil {
			atomic.AddInt64(&s.metrics.CacheHits, 1)
			users = append(users, &user)
		} else {
			atomic.AddInt64(&s.metrics.CacheMisses, 1)
			missedIDs = append(missedIDs, userID)
		}
	}
	
	// キャッシュミスしたユーザーをデータベースから取得
	if len(missedIDs) > 0 {
		atomic.AddInt64(&s.metrics.DBQueries, 1)
		
		for _, userID := range missedIDs {
			user, err := s.db.GetUser(ctx, userID)
			if err != nil {
				continue // 存在しないユーザーはスキップ
			}
			
			users = append(users, user)
			
			// キャッシュに保存
			cacheKey := userCacheKey(userID)
			s.cache.SetJSON(ctx, cacheKey, user, UserCacheTTL)
		}
	}
	
	return users, nil
}