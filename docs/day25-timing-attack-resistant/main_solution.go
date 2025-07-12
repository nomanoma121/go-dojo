package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// SecureStringCompare タイミング攻撃に耐性のある文字列比較
func SecureStringCompare(expected, provided string) bool {
	expectedBytes := []byte(expected)
	providedBytes := []byte(provided)
	
	// 長さが異なる場合の処理
	if len(expectedBytes) != len(providedBytes) {
		// ダミー比較で時間を一定に保つ
		maxLen := len(expectedBytes)
		if len(providedBytes) > maxLen {
			maxLen = len(providedBytes)
		}
		
		dummyExpected := make([]byte, maxLen)
		dummyProvided := make([]byte, maxLen)
		
		copy(dummyExpected, expectedBytes)
		copy(dummyProvided, providedBytes)
		
		subtle.ConstantTimeCompare(dummyExpected, dummyProvided)
		return false
	}
	
	return subtle.ConstantTimeCompare(expectedBytes, providedBytes) == 1
}

// SecureByteCompare バイト配列の一定時間比較
func SecureByteCompare(expected, provided []byte) bool {
	if len(expected) != len(provided) {
		// 長さが異なってもダミー比較を実行
		maxLen := len(expected)
		if len(provided) > maxLen {
			maxLen = len(provided)
		}
		
		dummyExpected := make([]byte, maxLen)
		dummyProvided := make([]byte, maxLen)
		
		copy(dummyExpected, expected)
		copy(dummyProvided, provided)
		
		subtle.ConstantTimeCompare(dummyExpected, dummyProvided)
		return false
	}
	
	return subtle.ConstantTimeCompare(expected, provided) == 1
}

// APIKeyValidator タイミング攻撃に耐性のあるAPIキー検証
type APIKeyValidator struct {
	validKeys map[string]bool
	mutex     sync.RWMutex
}

// NewAPIKeyValidator 新しいAPIキー検証器を作成
func NewAPIKeyValidator(keys []string) *APIKeyValidator {
	validKeys := make(map[string]bool)
	for _, key := range keys {
		validKeys[key] = true
	}
	
	return &APIKeyValidator{
		validKeys: validKeys,
	}
}

// ValidateKey APIキーを一定時間で検証
func (v *APIKeyValidator) ValidateKey(providedKey string) bool {
	v.mutex.RLock()
	defer v.mutex.RUnlock()
	
	var isValid bool
	providedBytes := []byte(providedKey)
	
	// すべてのキーと比較（早期終了を避ける）
	for validKey := range v.validKeys {
		validBytes := []byte(validKey)
		
		// 長さの一定時間比較
		lengthMatch := subtle.ConstantTimeEq(int32(len(providedBytes)), int32(len(validBytes)))
		
		// 内容の一定時間比較
		var contentMatch int
		if lengthMatch == 1 {
			contentMatch = subtle.ConstantTimeCompare(providedBytes, validBytes)
		} else {
			// 長さが異なってもダミー比較を実行
			maxLen := len(providedBytes)
			if len(validBytes) > maxLen {
				maxLen = len(validBytes)
			}
			
			dummyProvided := make([]byte, maxLen)
			dummyValid := make([]byte, maxLen)
			copy(dummyProvided, providedBytes)
			copy(dummyValid, validBytes)
			
			subtle.ConstantTimeCompare(dummyProvided, dummyValid)
			contentMatch = 0
		}
		
		// 見つかった場合でもループを継続
		match := lengthMatch & contentMatch
		if match == 1 {
			isValid = true
		}
	}
	
	return isValid
}

// AddKey 新しいAPIキーを追加
func (v *APIKeyValidator) AddKey(key string) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.validKeys[key] = true
}

// RemoveKey APIキーを削除
func (v *APIKeyValidator) RemoveKey(key string) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	delete(v.validKeys, key)
}

// PasswordAuth タイミング攻撃に耐性のあるパスワード認証
type PasswordAuth struct {
	users map[string][]byte // username -> hashed password
	mutex sync.RWMutex
}

// NewPasswordAuth 新しいパスワード認証システムを作成
func NewPasswordAuth() *PasswordAuth {
	return &PasswordAuth{
		users: make(map[string][]byte),
	}
}

// Register 新しいユーザーを登録
func (pa *PasswordAuth) Register(username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	
	pa.mutex.Lock()
	defer pa.mutex.Unlock()
	pa.users[username] = hashedPassword
	
	return nil
}

// Authenticate ユーザー認証を一定時間で実行
func (pa *PasswordAuth) Authenticate(username, password string) bool {
	pa.mutex.RLock()
	hashedPassword, exists := pa.users[username]
	pa.mutex.RUnlock()
	
	if !exists {
		// ユーザーが存在しない場合もダミー処理で時間を一定に
		dummyHash := []byte("$2a$10$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return false
	}
	
	err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
	return err == nil
}

// TokenValidator トークン検証システム
type TokenValidator struct {
	secretKey []byte
}

// NewTokenValidator 新しいトークン検証器を作成
func NewTokenValidator(secretKey []byte) *TokenValidator {
	return &TokenValidator{
		secretKey: secretKey,
	}
}

// ValidateToken Base64エンコードされたトークンを検証
func (tv *TokenValidator) ValidateToken(expectedToken, providedToken string) (bool, error) {
	// Base64デコード
	expectedBytes, err := base64.URLEncoding.DecodeString(expectedToken)
	if err != nil {
		return false, err
	}
	
	providedBytes, err := base64.URLEncoding.DecodeString(providedToken)
	if err != nil {
		return false, err
	}
	
	// 長さチェック（異なる長さでもダミー比較）
	if len(expectedBytes) != len(providedBytes) {
		maxLen := len(expectedBytes)
		if len(providedBytes) > maxLen {
			maxLen = len(providedBytes)
		}
		
		dummyExpected := make([]byte, maxLen)
		dummyProvided := make([]byte, maxLen)
		
		copy(dummyExpected, expectedBytes)
		copy(dummyProvided, providedBytes)
		
		subtle.ConstantTimeCompare(dummyExpected, dummyProvided)
		return false, nil
	}
	
	// 一定時間比較
	return subtle.ConstantTimeCompare(expectedBytes, providedBytes) == 1, nil
}

// CreateToken 新しいトークンを作成
func (tv *TokenValidator) CreateToken(data []byte) (string, error) {
	// 簡単な実装：データをそのままBase64エンコード
	// 実際のプロダクションではHMAC等を使用
	return base64.URLEncoding.EncodeToString(data), nil
}

// TimingResistantResponse タイミング攻撃に耐性のあるレスポンス
type TimingResistantResponse struct {
	minDuration time.Duration
}

// NewTimingResistantResponse 新しいタイミング耐性レスポンスを作成
func NewTimingResistantResponse(minDuration time.Duration) *TimingResistantResponse {
	return &TimingResistantResponse{
		minDuration: minDuration,
	}
}

// Execute 指定された処理を一定時間で実行
func (trr *TimingResistantResponse) Execute(fn func() bool) bool {
	start := time.Now()
	
	// 関数を実行
	result := fn()
	
	// 実行時間を測定
	elapsed := time.Since(start)
	
	// 最小実行時間になるまで待機
	if elapsed < trr.minDuration {
		delay := trr.minDuration - elapsed
		
		// ランダム要素を追加（最大10ms）
		maxRandom := int64(10 * time.Millisecond)
		if randomInt, err := rand.Int(rand.Reader, big.NewInt(maxRandom)); err == nil {
			randomDelay := time.Duration(randomInt.Int64())
			delay += randomDelay
		}
		
		time.Sleep(delay)
	}
	
	return result
}

// SecureMemory セキュアなメモリ管理
type SecureMemory struct {
	data []byte
}

// NewSecureMemory セキュアなメモリ領域を作成
func NewSecureMemory(size int) *SecureMemory {
	return &SecureMemory{
		data: make([]byte, size),
	}
}

// Write データを書き込み
func (sm *SecureMemory) Write(data []byte) {
	copy(sm.data, data)
}

// Read データを読み取り
func (sm *SecureMemory) Read() []byte {
	return sm.data
}

// Wipe メモリ内容をゼロ化
func (sm *SecureMemory) Wipe() {
	// メモリをゼロで埋める
	for i := range sm.data {
		sm.data[i] = 0
	}
	
	// GCによる最適化を回避
	runtime.KeepAlive(sm.data)
}

// ConstantTimeArraySearch 配列の一定時間検索
func ConstantTimeArraySearch(haystack []string, needle string) int {
	needleBytes := []byte(needle)
	foundIndex := -1
	
	for i, item := range haystack {
		itemBytes := []byte(item)
		
		// 長さの一定時間比較
		lengthMatch := subtle.ConstantTimeEq(int32(len(needleBytes)), int32(len(itemBytes)))
		
		// 内容の一定時間比較
		var contentMatch int
		if lengthMatch == 1 {
			contentMatch = subtle.ConstantTimeCompare(needleBytes, itemBytes)
		} else {
			// 長さが異なってもダミー比較を実行
			maxLen := len(needleBytes)
			if len(itemBytes) > maxLen {
				maxLen = len(itemBytes)
			}
			
			dummyNeedle := make([]byte, maxLen)
			dummyItem := make([]byte, maxLen)
			copy(dummyNeedle, needleBytes)
			copy(dummyItem, itemBytes)
			
			subtle.ConstantTimeCompare(dummyNeedle, dummyItem)
			contentMatch = 0
		}
		
		// 見つかった場合のインデックス更新（一定時間）
		match := lengthMatch & contentMatch
		foundIndex = subtle.ConstantTimeSelect(match, i, foundIndex)
	}
	
	return foundIndex
}

// BenchmarkComparison 比較処理のベンチマーク
func BenchmarkComparison(secret string, inputs []string) []time.Duration {
	var durations []time.Duration
	
	for _, input := range inputs {
		start := time.Now()
		SecureStringCompare(secret, input)
		duration := time.Since(start)
		durations = append(durations, duration)
	}
	
	return durations
}

// insecureCompare 脆弱な比較（テスト比較用）
func insecureCompare(expected, provided string) bool {
	if len(expected) != len(provided) {
		return false
	}
	
	for i := 0; i < len(expected); i++ {
		if expected[i] != provided[i] {
			return false // 最初の不一致で即座にfalseを返す
		}
	}
	
	return true
}

// サンプルハンドラー
func createSampleHandlers() {
	// APIキー検証のサンプル
	validator := NewAPIKeyValidator([]string{
		"api-key-123456789",
		"super-secret-key-abc",
		"dev-key-xyz789",
	})

	// パスワード認証のサンプル
	auth := NewPasswordAuth()
	auth.Register("admin", "admin123")
	auth.Register("user", "userpass")

	// デモ実行
	fmt.Println("=== Timing Attack Resistant Comparison Demo ===")
	
	// 文字列比較のデモ
	fmt.Println("\n1. Secure String Comparison:")
	secret := "topsecret123"
	testInputs := []string{"a", "topsec", "topsecret122", "topsecret123"}
	
	for _, input := range testInputs {
		start := time.Now()
		result := SecureStringCompare(secret, input)
		duration := time.Since(start)
		fmt.Printf("Input: %-15s Result: %-5v Duration: %v\n", input, result, duration)
	}

	// APIキー検証のデモ
	fmt.Println("\n2. API Key Validation:")
	testKeys := []string{"api-key-123456789", "invalid-key", "dev-key-xyz789"}
	
	for _, key := range testKeys {
		start := time.Now()
		result := validator.ValidateKey(key)
		duration := time.Since(start)
		fmt.Printf("Key: %-20s Result: %-5v Duration: %v\n", key, result, duration)
	}

	// パスワード認証のデモ
	fmt.Println("\n3. Password Authentication:")
	testCreds := [][]string{
		{"admin", "admin123"},
		{"admin", "wrongpass"},
		{"nonexistent", "anypass"},
		{"user", "userpass"},
	}
	
	for _, cred := range testCreds {
		start := time.Now()
		result := auth.Authenticate(cred[0], cred[1])
		duration := time.Since(start)
		fmt.Printf("User: %-12s Pass: %-10s Result: %-5v Duration: %v\n", 
			cred[0], cred[1], result, duration)
	}

	// タイミング耐性レスポンスのデモ
	fmt.Println("\n4. Timing Resistant Response:")
	trr := NewTimingResistantResponse(100 * time.Millisecond)
	
	// 高速な処理
	start := time.Now()
	result1 := trr.Execute(func() bool {
		time.Sleep(10 * time.Millisecond)
		return true
	})
	duration1 := time.Since(start)
	fmt.Printf("Fast function: Result: %-5v Duration: %v\n", result1, duration1)
	
	// 低速な処理
	start = time.Now()
	result2 := trr.Execute(func() bool {
		time.Sleep(150 * time.Millisecond)
		return false
	})
	duration2 := time.Since(start)
	fmt.Printf("Slow function: Result: %-5v Duration: %v\n", result2, duration2)

	// 配列検索のデモ
	fmt.Println("\n5. Constant Time Array Search:")
	haystack := []string{"apple", "banana", "cherry", "date"}
	needles := []string{"banana", "notfound", "apple"}
	
	for _, needle := range needles {
		start := time.Now()
		index := ConstantTimeArraySearch(haystack, needle)
		duration := time.Since(start)
		fmt.Printf("Search %-10s: Index: %-2d Duration: %v\n", needle, index, duration)
	}
}

func main() {
	createSampleHandlers()
}