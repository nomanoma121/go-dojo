# Day 25: タイミング攻撃耐性のある比較

🎯 **本日の目標**
crypto/subtleパッケージを使用してタイミング攻撃（timing attack）に対して安全な文字列比較を実装し、セキュリティに配慮したデータ比較手法を学ぶ。

## 📖 解説

### タイミング攻撃とは

```go
// 【タイミング攻撃の脅威】サイドチャネル攻撃による秘密情報漏洩
// ❌ 問題例：実行時間差によるパスワード・トークン漏洩の災害シナリオ
func catastrophicTimingVulnerability() {
    // 🚨 災害例：タイミング攻撃でAPIキーが完全漏洩
    correctAPIKey := "sk-1a2b3c4d5e6f7g8h9i0j1k2l3m4n5o6p"
    
    // ❌ 脆弱な比較実装（本番環境で実際に発生）
    loginHandler := func(w http.ResponseWriter, r *http.Request) {
        providedKey := r.Header.Get("Authorization")
        
        start := time.Now()
        
        // ❌ 文字列比較で最初の不一致で即座に終了
        if isValidAPIKey(correctAPIKey, providedKey) {
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(map[string]string{
                "status": "success",
                "message": "API access granted",
            })
        } else {
            w.WriteHeader(http.StatusUnauthorized) 
            json.NewEncoder(w).Encode(map[string]string{
                "status": "error",
                "message": "Invalid API key",
            })
        }
        
        duration := time.Since(start)
        log.Printf("API key validation took: %v", duration)
        // ❌ このログが攻撃者に時間情報を提供
    }
    
    // 【攻撃者のシナリオ】自動化されたタイミング攻撃
    // 1. 攻撃者は数万回のリクエストで文字ごとの時間を測定
    // 2. 正しい文字の場合、わずかに処理時間が長くなる
    // 3. 統計的分析で1文字ずつ正解を特定
    // 4. 36文字のAPIキーを数時間で完全復元
    
    attackSimulation := func() {
        log.Println("🚨 Timing attack simulation starting...")
        
        // 攻撃者による systematic brute force
        candidates := []string{
            "a",  // 20μs - 即座に失敗
            "s",  // 25μs - 1文字目一致、2文字目で失敗
            "sk", // 35μs - 2文字目まで一致
            "sk-1", // 45μs - 4文字目まで一致
            // ... 攻撃者は統計的分析で正確な文字を特定
        }
        
        for _, candidate := range candidates {
            times := make([]time.Duration, 1000)
            
            // 1000回の測定で平均時間を算出
            for i := 0; i < 1000; i++ {
                start := time.Now()
                isValidAPIKey(correctAPIKey, candidate)
                times[i] = time.Since(start)
            }
            
            // 統計分析
            avg := calculateAverage(times)
            log.Printf("Candidate '%s': avg time = %v", candidate, avg)
            
            // ❌ 時間が長いほど正解に近い = 攻撃成功
        }
        
        log.Println("❌ API key fully compromised through timing analysis")
        log.Println("❌ All user data accessible, system completely breached")
    }
    
    http.HandleFunc("/api/secure", loginHandler)
    
    // 攻撃シミュレーション実行
    go attackSimulation()
    
    log.Println("❌ Starting vulnerable server...")
    http.ListenAndServe(":8080", nil)
    // 結果：APIキー完全漏洩、全データ流出、システム侵害完了
}

// ❌ 脆弱なAPI key検証（災害の元凶）
func isValidAPIKey(expected, provided string) bool {
    if len(expected) != len(provided) {
        return false // 長さチェックでも時間差が発生
    }
    
    // ❌ 最初の不一致で即座にfalseを返す（致命的脆弱性）
    for i := 0; i < len(expected); i++ {
        if expected[i] != provided[i] {
            // この時点での早期終了が時間差を生む
            return false
        }
        // CPUが文字比較に要する時間：約1-2マイクロ秒
        // 32文字のキーなら最大64マイクロ秒の差が発生
    }
    
    return true
}

// ✅ 正解：エンタープライズ級タイミング攻撃耐性システム
type SecureComparator struct {
    // 【基本設定】
    constantTimeEnabled bool           // 定数時間比較有効化
    
    // 【高度なセキュリティ】
    jitterGenerator    *JitterGenerator // 意図的なタイミングばらつき
    decoyOperations    *DecoyExecutor   // ダミー処理でカモフラージュ
    
    // 【監視・検知】
    timingAnalyzer     *TimingAnalyzer  // タイミング攻撃検知
    alertManager       *SecurityAlertManager
    
    // 【統計・ログ】
    metrics           *SecurityMetrics  // セキュリティメトリクス
    logger            *log.Logger       // セキュリティログ
    
    // 【レート制限】
    rateLimiter       *AuthRateLimiter  // 認証試行制限
    
    mu                sync.RWMutex      // 設定変更保護
}

// 【重要関数】セキュア比較システム初期化
func NewSecureComparator(config *SecureConfig) *SecureComparator {
    return &SecureComparator{
        constantTimeEnabled: true,
        jitterGenerator:     NewJitterGenerator(config.JitterRange),
        decoyOperations:     NewDecoyExecutor(),
        timingAnalyzer:      NewTimingAnalyzer(),
        alertManager:        NewSecurityAlertManager(),
        metrics:            NewSecurityMetrics(),
        logger:             log.New(os.Stdout, "[SECURE-CMP] ", log.LstdFlags),
        rateLimiter:        NewAuthRateLimiter(config.MaxAttemptsPerIP),
    }
}

// 【核心メソッド】タイミング攻撃耐性のある安全な比較
func (sc *SecureComparator) SecureCompare(expected, provided string, clientIP string) (bool, error) {
    startTime := time.Now()
    requestID := generateSecureRequestID()
    
    // 【STEP 1】レート制限チェック
    if !sc.rateLimiter.AllowAttempt(clientIP) {
        sc.metrics.RecordRateLimitHit(clientIP)
        sc.logger.Printf("❌ Rate limit exceeded for IP: %s", clientIP)
        return false, &SecurityError{
            Type:    "RATE_LIMIT_EXCEEDED",
            Message: "Too many authentication attempts",
            IP:      clientIP,
        }
    }
    
    // 【STEP 2】タイミング攻撃検知の開始
    sc.timingAnalyzer.StartMeasurement(requestID, clientIP)
    
    // 【STEP 3】意図的なジッター追加（攻撃妨害）
    baseJitter := sc.jitterGenerator.GenerateJitter()
    time.Sleep(baseJitter)
    
    // 【STEP 4】定数時間比較の実行
    var result bool
    
    if sc.constantTimeEnabled {
        // crypto/subtleによる定数時間比較
        expectedBytes := []byte(expected)
        providedBytes := []byte(provided)
        
        // 長さを統合して比較（長さ情報も秘匿）
        maxLen := max(len(expectedBytes), len(providedBytes))
        paddedExpected := make([]byte, maxLen)
        paddedProvided := make([]byte, maxLen)
        
        copy(paddedExpected, expectedBytes)
        copy(paddedProvided, providedBytes)
        
        // 【重要】crypto/subtle.ConstantTimeCompareで安全比較
        comparison := subtle.ConstantTimeCompare(paddedExpected, paddedProvided)
        lengthMatch := subtle.ConstantTimeEq(int32(len(expectedBytes)), int32(len(providedBytes)))
        
        // 両方が一致する場合のみ成功
        result = (comparison & lengthMatch) == 1
        
    } else {
        // レガシー比較モード（テスト用）
        result = expected == provided
    }
    
    // 【STEP 5】ダミー操作でカモフラージュ
    decoyDuration := sc.decoyOperations.ExecuteDecoyOperations(len(expected))
    
    // 【STEP 6】さらなるジッター追加
    finalJitter := sc.jitterGenerator.GenerateFinalJitter()
    time.Sleep(finalJitter)
    
    // 【STEP 7】タイミング分析と攻撃検知
    totalDuration := time.Since(startTime)
    suspiciousActivity := sc.timingAnalyzer.AnalyzeTiming(requestID, clientIP, totalDuration, result)
    
    if suspiciousActivity {
        sc.alertManager.TriggerTimingAttackAlert(clientIP, requestID)
        sc.logger.Printf("⚠️  Potential timing attack detected from %s", clientIP)
    }
    
    // 【STEP 8】メトリクス記録
    sc.metrics.RecordComparison(clientIP, result, totalDuration)
    
    // 【STEP 9】セキュリティログ出力
    sc.logger.Printf("🔒 Secure comparison completed: result=%t, duration=%v, client=%s", 
        result, totalDuration, clientIP)
    
    return result, nil
}

// 【高度な機能】ジッター生成（タイミング攻撃妨害）
type JitterGenerator struct {
    baseRange    time.Duration  // ベースジッター範囲
    randomSource *rand.Rand     // 暗号学的乱数
    mu          sync.Mutex     // スレッドセーフティ
}

func NewJitterGenerator(baseRange time.Duration) *JitterGenerator {
    return &JitterGenerator{
        baseRange:    baseRange,
        randomSource: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

func (jg *JitterGenerator) GenerateJitter() time.Duration {
    jg.mu.Lock()
    defer jg.mu.Unlock()
    
    // 0から基準範囲までのランダムな遅延
    randomNanos := jg.randomSource.Int63n(int64(jg.baseRange))
    return time.Duration(randomNanos)
}

func (jg *JitterGenerator) GenerateFinalJitter() time.Duration {
    jg.mu.Lock()
    defer jg.mu.Unlock()
    
    // より大きな範囲でのファイナルジッター
    finalRange := jg.baseRange * 2
    randomNanos := jg.randomSource.Int63n(int64(finalRange))
    return time.Duration(randomNanos)
}

// 【高度な機能】ダミー操作実行（カモフラージュ）
type DecoyExecutor struct {
    operations []func(int) time.Duration  // ダミー処理リスト
}

func NewDecoyExecutor() *DecoyExecutor {
    return &DecoyExecutor{
        operations: []func(int) time.Duration{
            func(length int) time.Duration {
                // SHA256ハッシュ計算（CPUを消費）
                data := make([]byte, length)
                for i := range data {
                    data[i] = byte(i % 256)
                }
                start := time.Now()
                sha256.Sum256(data)
                return time.Since(start)
            },
            func(length int) time.Duration {
                // AES暗号化処理（CPUを消費）
                key := make([]byte, 32)
                plaintext := make([]byte, length)
                
                start := time.Now()
                block, _ := aes.NewCipher(key)
                ciphertext := make([]byte, len(plaintext))
                
                for i := 0; i < len(plaintext); i += aes.BlockSize {
                    end := i + aes.BlockSize
                    if end > len(plaintext) {
                        end = len(plaintext)
                    }
                    if end-i == aes.BlockSize {
                        block.Encrypt(ciphertext[i:end], plaintext[i:end])
                    }
                }
                return time.Since(start)
            },
        },
    }
}

func (de *DecoyExecutor) ExecuteDecoyOperations(inputLength int) time.Duration {
    totalDuration := time.Duration(0)
    
    // ランダムに1-3個のダミー操作を実行
    numOps := rand.Intn(3) + 1
    
    for i := 0; i < numOps; i++ {
        opIndex := rand.Intn(len(de.operations))
        duration := de.operations[opIndex](inputLength)
        totalDuration += duration
    }
    
    return totalDuration
}

// 【実用例】プロダクション環境でのセキュア認証
func ProductionSecureAuthUsage() {
    // 【初期化】エンタープライズセキュリティ設定
    config := &SecureConfig{
        JitterRange:       500 * time.Microsecond,  // 500μsのジッター
        MaxAttemptsPerIP:  10,                      // IP毎の最大試行回数
        TimingThreshold:   100 * time.Microsecond,  // 異常検知閾値
    }
    
    comparator := NewSecureComparator(config)
    
    // 【認証エンドポイント】
    http.HandleFunc("/api/auth", func(w http.ResponseWriter, r *http.Request) {
        var authReq struct {
            APIKey string `json:"api_key"`
        }
        
        if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
            http.Error(w, "Invalid JSON", http.StatusBadRequest)
            return
        }
        
        clientIP := getClientIP(r)
        correctAPIKey := getExpectedAPIKey() // 安全に保存されたキー
        
        // 【セキュア比較実行】
        isValid, err := comparator.SecureCompare(correctAPIKey, authReq.APIKey, clientIP)
        if err != nil {
            http.Error(w, "Authentication error", http.StatusTooManyRequests)
            return
        }
        
        if isValid {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "status":    "success",
                "message":   "Authentication successful",
                "timestamp": time.Now().Unix(),
            })
        } else {
            // 【重要】成功・失敗にかかわらず同じ応答時間を維持
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusUnauthorized)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "status":  "error",
                "message": "Invalid credentials",
            })
        }
    })
    
    log.Printf("🔒 Secure authentication server starting on :8080")
    log.Printf("   Timing attack protection: ENABLED")
    log.Printf("   Jitter range: %v", config.JitterRange)
    log.Printf("   Rate limiting: %d attempts per IP", config.MaxAttemptsPerIP)
    
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### crypto/subtle パッケージ

Goの`crypto/subtle`パッケージは、タイミング攻撃に対して安全な操作を提供します。

#### ConstantTimeCompare

```go
import "crypto/subtle"

func secureCompare(expected, provided string) bool {
    expectedBytes := []byte(expected)
    providedBytes := []byte(provided)
    
    // 長さが異なる場合も一定時間で処理
    return subtle.ConstantTimeCompare(expectedBytes, providedBytes) == 1
}
```

### 実際のセキュリティシナリオ

#### 1. パスワード認証

```go
type UserStore struct {
    users map[string][]byte // username -> hashed password
}

func (us *UserStore) authenticate(username, password string) bool {
    hashedPassword, exists := us.users[username]
    if !exists {
        // ユーザーが存在しない場合もダミー処理で時間を一定に
        dummy := make([]byte, 32)
        bcrypt.CompareHashAndPassword(dummy, []byte(password))
        return false
    }
    
    err := bcrypt.CompareHashAndPassword(hashedPassword, []byte(password))
    return err == nil
}
```

#### 2. トークン検証

```go
func validateToken(expectedToken, providedToken string) bool {
    // 長さチェックもタイミング攻撃に配慮
    if len(expectedToken) != len(providedToken) {
        // 長さが違ってもダミー比較を実行
        dummy := make([]byte, len(providedToken))
        subtle.ConstantTimeCompare([]byte(expectedToken), dummy)
        return false
    }
    
    return subtle.ConstantTimeCompare(
        []byte(expectedToken),
        []byte(providedToken),
    ) == 1
}
```

#### 3. HMAC検証

```go
import (
    "crypto/hmac"
    "crypto/sha256"
)

func verifyHMAC(message, signature []byte, key []byte) bool {
    mac := hmac.New(sha256.New, key)
    mac.Write(message)
    expectedSignature := mac.Sum(nil)
    
    // HMACの比較はタイミング攻撃に配慮
    return subtle.ConstantTimeCompare(signature, expectedSignature) == 1
}
```

### 高度なタイミング攻撃対策

#### バイト単位での一定時間比較

```go
func constantTimeByteCompare(a, b []byte) bool {
    if len(a) != len(b) {
        return false
    }
    
    var result byte
    for i := 0; i < len(a); i++ {
        result |= a[i] ^ b[i]
    }
    
    return subtle.ConstantTimeByteEq(result, 0) == 1
}
```

#### 数値の一定時間比較

```go
func constantTimeIntEquals(a, b int) bool {
    return subtle.ConstantTimeEq(int32(a), int32(b)) == 1
}

func constantTimeSelect(condition bool, ifTrue, ifFalse int) int {
    var conditionInt int
    if condition {
        conditionInt = 1
    } else {
        conditionInt = 0
    }
    
    return subtle.ConstantTimeSelect(conditionInt, ifTrue, ifFalse)
}
```

### レスポンス時間の均一化

#### ランダム遅延による対策

```go
import (
    "crypto/rand"
    "math/big"
    "time"
)

func authenticateWithRandomDelay(username, password string) bool {
    start := time.Now()
    
    result := performAuthentication(username, password)
    
    // 認証処理時間を測定
    elapsed := time.Since(start)
    
    // 最小実行時間を設定（例：100ms）
    minDuration := 100 * time.Millisecond
    if elapsed < minDuration {
        delay := minDuration - elapsed
        
        // さらにランダム要素を追加
        maxRandom := int64(10 * time.Millisecond)
        randomInt, _ := rand.Int(rand.Reader, big.NewInt(maxRandom))
        randomDelay := time.Duration(randomInt.Int64())
        
        time.Sleep(delay + randomDelay)
    }
    
    return result
}
```

#### 固定時間スリープ

```go
func authenticateWithFixedTiming(username, password string) bool {
    result := performAuthentication(username, password)
    
    // 常に一定時間待機
    time.Sleep(200 * time.Millisecond)
    
    return result
}
```

### メモリアクセスパターンの隠蔽

#### 一定時間での配列検索

```go
func constantTimeArraySearch(haystack []string, needle string) int {
    needleBytes := []byte(needle)
    foundIndex := -1
    
    for i, item := range haystack {
        itemBytes := []byte(item)
        
        // 長さチェック
        lengthMatch := subtle.ConstantTimeEq(int32(len(needleBytes)), int32(len(itemBytes)))
        
        // 内容チェック（長さが一致する場合のみ）
        var contentMatch int
        if lengthMatch == 1 {
            contentMatch = subtle.ConstantTimeCompare(needleBytes, itemBytes)
        }
        
        // 見つかった場合のインデックス更新（一定時間）
        foundIndex = subtle.ConstantTimeSelect(
            lengthMatch & contentMatch,
            i,
            foundIndex,
        )
    }
    
    return foundIndex
}
```

### 実際のWebアプリケーションでの実装

#### セキュアなAPIキー検証

```go
type SecureAPIKeyValidator struct {
    validKeys map[string]bool
    mutex     sync.RWMutex
}

func (v *SecureAPIKeyValidator) ValidateKey(providedKey string) bool {
    v.mutex.RLock()
    defer v.mutex.RUnlock()
    
    // すべてのキーと比較（早期終了を避ける）
    var isValid bool
    for validKey := range v.validKeys {
        if subtle.ConstantTimeCompare(
            []byte(validKey),
            []byte(providedKey),
        ) == 1 {
            isValid = true
            // 見つかってもループを継続
        }
    }
    
    return isValid
}
```

#### セッショントークン検証

```go
func validateSessionToken(expectedToken, providedToken string) (bool, error) {
    // Base64デコード
    expected, err := base64.URLEncoding.DecodeString(expectedToken)
    if err != nil {
        return false, err
    }
    
    provided, err := base64.URLEncoding.DecodeString(providedToken)
    if err != nil {
        return false, err
    }
    
    // 長さチェック
    if len(expected) != len(provided) {
        // 異なる長さでもダミー比較を実行
        dummyProvided := make([]byte, len(expected))
        copy(dummyProvided, provided)
        subtle.ConstantTimeCompare(expected, dummyProvided)
        return false, nil
    }
    
    // セキュアな比較
    return subtle.ConstantTimeCompare(expected, provided) == 1, nil
}
```

### パフォーマンス考慮事項

#### ベンチマークテスト

```go
func BenchmarkInsecureCompare(b *testing.B) {
    expected := "secret123456789"
    provided := "secret123456780" // 最後の文字が違う
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        insecureCompare(expected, provided)
    }
}

func BenchmarkSecureCompare(b *testing.B) {
    expected := "secret123456789"
    provided := "secret123456780"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        secureCompare(expected, provided)
    }
}
```

### 実装時の注意点

1. **コンパイラ最適化**: 未使用の比較がコンパイラによって除去される可能性
2. **CPU分岐予測**: 分岐パターンによる実行時間の変動
3. **キャッシュ効果**: メモリアクセスパターンによる時間差
4. **ネットワーク遅延**: ネットワークレベルでの時間測定の困難さ

### テスト戦略

```go
func TestTimingAttackResistance(t *testing.T) {
    secret := "topsecretpassword123"
    
    // 異なる長さの入力での測定
    inputs := []string{
        "a",
        "topsecret",
        "topsecretpassword122", // 最後の文字が違う
        "topsecretpassword123", // 正解
    }
    
    for _, input := range inputs {
        start := time.Now()
        result := secureCompare(secret, input)
        duration := time.Since(start)
        
        t.Logf("Input: %s, Result: %v, Duration: %v", input, result, duration)
    }
}
```

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **セキュアな文字列比較**
   - crypto/subtleを使用した一定時間比較
   - 長さが異なる場合の適切な処理
   - エラーハンドリング

2. **APIキー検証システム**
   - タイミング攻撃に耐性のあるキー検証
   - 複数キーでの検索最適化
   - 無効なキーに対する一定時間応答

3. **パスワード認証**
   - bcryptハッシュとの一定時間比較
   - 存在しないユーザーでの一定時間処理
   - ソルト付きハッシュの生成と検証

4. **トークン検証**
   - Base64エンコードされたトークンの比較
   - HMACを使用した署名検証
   - 期限付きトークンの検証

5. **レスポンス時間の均一化**
   - 最小実行時間の保証
   - ランダム遅延の追加
   - 統計的な時間分析対策

6. **メモリ安全性**
   - 機密データのゼロ化
   - GCからの保護
   - メモリダンプ対策

## ✅ 期待される挙動

### 成功パターン

#### セキュアな文字列比較：
```go
result := SecureStringCompare("secret123", "secret123")
// result: true, 実行時間は入力に関わらず一定
```

#### APIキー検証：
```go
validator := NewAPIKeyValidator([]string{"key1", "key2", "key3"})
isValid := validator.ValidateKey("key2")
// isValid: true, すべてのキーを一定時間で検査
```

#### パスワード認証：
```go
auth := NewPasswordAuth()
auth.Register("user1", "password123")
result := auth.Authenticate("user1", "password123")
// result: true, ハッシュ比較は一定時間
```

### タイミング攻撃テスト

#### 実行時間の測定：
```go
// 異なる入力での実行時間測定
measurements := BenchmarkComparison("secret", []string{
    "a",           // 1文字目で不一致
    "sec",         // 3文字目で不一致  
    "secre",       // 5文字目で不一致
    "secret",      // 完全一致
})

// すべての測定時間が統計的に有意な差がないことを確認
```

#### エラーパターンの一貫性：
```go
// 存在しないユーザーでも一定時間で処理
start := time.Now()
result1 := auth.Authenticate("nonexistent", "password")
duration1 := time.Since(start)

start = time.Now()
result2 := auth.Authenticate("user1", "wrongpassword")
duration2 := time.Since(start)

// duration1 ≈ duration2 (統計的に有意な差なし)
```

## 💡 ヒント

1. **crypto/subtle.ConstantTimeCompare**: バイト配列の一定時間比較
2. **crypto/subtle.ConstantTimeSelect**: 条件分岐の一定時間実行
3. **crypto/subtle.ConstantTimeByteEq**: バイトの一定時間等価判定
4. **golang.org/x/crypto/bcrypt**: セキュアなパスワードハッシュ化
5. **time.Sleep**: 実行時間の均一化
6. **crypto/rand**: セキュアな乱数生成

### セキュアな比較の実装例

```go
func SecureStringCompare(expected, provided string) bool {
    expectedBytes := []byte(expected)
    providedBytes := []byte(provided)
    
    // 長さが異なる場合の処理
    if len(expectedBytes) != len(providedBytes) {
        // ダミー比較で時間を一定に
        dummy := make([]byte, len(expectedBytes))
        if len(providedBytes) < len(dummy) {
            copy(dummy, providedBytes)
        }
        subtle.ConstantTimeCompare(expectedBytes, dummy)
        return false
    }
    
    return subtle.ConstantTimeCompare(expectedBytes, providedBytes) == 1
}
```

### タイミング攻撃対策チェックリスト

- [ ] 文字列比較での早期終了を回避
- [ ] 配列検索での一定時間アクセス
- [ ] エラー処理での時間差を排除
- [ ] ネットワーク応答時間の均一化
- [ ] メモリアクセスパターンの隠蔽
- [ ] 統計的分析への対策

### パフォーマンステスト例

```go
func TestConstantTimeProperty(t *testing.T) {
    secret := "verylongsecretpassword123456789"
    
    // 異なる位置での不一致をテスト
    testCases := []string{
        "a" + strings.Repeat("x", len(secret)-1),           // 最初で不一致
        secret[:len(secret)/2] + strings.Repeat("x", len(secret)/2), // 中間で不一致
        secret[:len(secret)-1] + "x",                       // 最後で不一致
        secret,                                             // 完全一致
    }
    
    var durations []time.Duration
    
    for _, testCase := range testCases {
        start := time.Now()
        SecureStringCompare(secret, testCase)
        duration := time.Since(start)
        durations = append(durations, duration)
    }
    
    // 統計的分析で時間差が有意でないことを確認
    // (実装では簡略化したテストを行う)
}
```

### セキュリティ考慮事項

- **機密データの寿命管理**: 使用後の適切なゼロ化
- **GCからの保護**: `runtime.KeepAlive`の適切な使用
- **コンパイラ最適化**: デッドコード削除の回避
- **分岐予測対策**: 予測可能なパターンの回避

これらの実装により、現実的なタイミング攻撃に対して堅牢なセキュリティシステムの基礎を学ぶことができます。