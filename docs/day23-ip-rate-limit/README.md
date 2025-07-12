# Day 23: IPベースレートリミットミドルウェア

🎯 **本日の目標**
IPアドレス単位でリクエスト頻度を制限するレートリミットミドルウェアを実装し、DDoS攻撃や過負荷からサーバーを保護する手法を学ぶ。

## 📖 解説

### レートリミットの重要性

レートリミットは、特定の時間内にクライアントが送信できるリクエストの数を制限するセキュリティ機能です。これにより以下の脅威を防ぐことができます：

- **DDoS攻撃**: 大量のリクエストによるサービス妨害
- **ブルートフォース攻撃**: パスワード総当たり攻撃
- **API乱用**: 過度なAPIコールによるリソース枯渇
- **スクレイピング攻撃**: 大量データ取得の悪用

### Sliding Windowアルゴリズム

レートリミットの実装には複数のアルゴリズムがありますが、今回はSliding Window（滑動窓）方式を使用します：

```go
type SlidingWindow struct {
    mu        sync.Mutex
    requests  []time.Time
    window    time.Duration
    limit     int
}

func (sw *SlidingWindow) Allow() bool {
    sw.mu.Lock()
    defer sw.mu.Unlock()
    
    now := time.Now()
    cutoff := now.Add(-sw.window)
    
    // 期限切れのリクエストを削除
    for len(sw.requests) > 0 && sw.requests[0].Before(cutoff) {
        sw.requests = sw.requests[1:]
    }
    
    // 制限チェック
    if len(sw.requests) >= sw.limit {
        return false
    }
    
    // 新しいリクエストを記録
    sw.requests = append(sw.requests, now)
    return true
}
```

### IPアドレスの取得

リバースプロキシ環境では、実際のクライアントIPは`X-Forwarded-For`や`X-Real-IP`ヘッダーに含まれます：

```go
func getRealIP(r *http.Request) string {
    // プロキシ経由の場合、X-Forwarded-Forを優先
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        ips := strings.Split(xff, ",")
        if len(ips) > 0 {
            return strings.TrimSpace(ips[0])
        }
    }
    
    // X-Real-IPを確認
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }
    
    // 直接接続の場合
    ip, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    
    return ip
}
```

### メモリ効率的なクリーンアップ

時間が経過した古いエントリを定期的に削除してメモリ使用量を制御します：

```go
func (rl *RateLimiter) cleanup() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            rl.mu.Lock()
            now := time.Now()
            for ip, window := range rl.clients {
                window.mu.Lock()
                cutoff := now.Add(-window.window)
                
                // 期限切れリクエストをすべて削除
                newRequests := make([]time.Time, 0)
                for _, req := range window.requests {
                    if !req.Before(cutoff) {
                        newRequests = append(newRequests, req)
                    }
                }
                window.requests = newRequests
                
                // 空になったウィンドウを削除
                if len(window.requests) == 0 {
                    delete(rl.clients, ip)
                }
                
                window.mu.Unlock()
            }
            rl.mu.Unlock()
        case <-rl.done:
            return
        }
    }
}
```

### HTTPレスポンスヘッダー

レートリミット情報をクライアントに通知するための標準的なヘッダー：

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 87
X-RateLimit-Reset: 1640995200
Retry-After: 60
```

### ホワイトリスト機能

特定のIPアドレスをレートリミットから除外する機能：

```go
type RateLimiter struct {
    // ... 他のフィールド
    whitelist map[string]bool
}

func (rl *RateLimiter) IsWhitelisted(ip string) bool {
    rl.mu.RLock()
    defer rl.mu.RUnlock()
    return rl.whitelist[ip]
}

func (rl *RateLimiter) AddToWhitelist(ip string) {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    rl.whitelist[ip] = true
}
```

### エンドポイント別の制限設定

異なるエンドポイントに異なる制限を適用：

```go
type EndpointConfig struct {
    RequestsPerMinute int
    Window           time.Duration
}

func (rl *RateLimiter) MiddlewareWithConfig(config EndpointConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip := getRealIP(r)
            
            if rl.IsWhitelisted(ip) {
                next.ServeHTTP(w, r)
                return
            }
            
            if !rl.allowWithConfig(ip, config) {
                rl.sendRateLimitResponse(w)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### セキュリティ考慮事項

1. **IP偽装対策**: プロキシ設定の検証
2. **分散レートリミット**: Redis等を使った複数サーバー間での制限
3. **適応的制限**: 攻撃パターンに応じた動的な制限調整
4. **ログ記録**: 制限に達したリクエストのログ
5. **監視**: レートリミット状況のメトリクス収集

## 📝 課題

`main_test.go`に書かれているテストをすべてパスするように、`main_solution.go`ファイルに以下の機能を実装してください：

1. **RateLimiter構造体**
   - IPごとのSlidingWindowを管理
   - 制限値と時間窓の設定
   - ホワイトリスト機能

2. **Sliding Window実装**
   - 時間窓内のリクエスト数カウント
   - 期限切れエントリの自動削除
   - スレッドセーフな操作

3. **ミドルウェア関数**
   - IPアドレスの適切な取得
   - レート制限の判定
   - 適切なHTTPレスポンスの送信

4. **レスポンスヘッダー**
   - X-RateLimit-Limit: 制限値
   - X-RateLimit-Remaining: 残り回数
   - X-RateLimit-Reset: リセット時刻
   - Retry-After: 再試行可能時間

5. **管理機能**
   - ホワイトリストへの追加/削除
   - 制限設定の動的変更
   - メモリクリーンアップ

6. **エラーハンドリング**
   - 429 Too Many Requests応答
   - 適切なJSON形式のエラーメッセージ

## ✅ 期待される挙動

### 成功パターン

#### 制限内のリクエスト：
```bash
curl -v http://localhost:8080/api
```
```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 9
X-RateLimit-Reset: 1640995260
Content-Type: application/json

{
  "message": "Request successful",
  "timestamp": "2023-12-31T23:59:59Z"
}
```

#### ホワイトリストIP：
```bash
curl -H "X-Real-IP: 127.0.0.1" http://localhost:8080/api
```
```json
{
  "message": "Request successful (whitelisted)",
  "ip": "127.0.0.1"
}
```

### エラーパターン

#### レート制限超過（429 Too Many Requests）：
```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1640995320
Retry-After: 60
Content-Type: application/json

{
  "error": "Rate limit exceeded",
  "message": "Too many requests from IP 192.168.1.100",
  "retry_after": 60,
  "limit": 10,
  "window": "1m"
}
```

#### プロキシ経由のIPアドレス：
```bash
curl -H "X-Forwarded-For: 203.0.113.195, 70.41.3.18, 150.172.238.178" http://localhost:8080/api
```
実際のクライアントIP（203.0.113.195）で制限が適用される

## 💡 ヒント

1. **sync.RWMutex**: 読み取り頻度が高い場合の最適化
2. **time.NewTicker**: 定期的なクリーンアップタスク
3. **net.SplitHostPort**: IPアドレスとポートの分離
4. **strings.Split**: X-Forwarded-Forの複数IP処理
5. **HTTP Status 429**: レート制限専用のステータスコード
6. **time.Unix()**: UNIXタイムスタンプでのリセット時刻表現

### レート制限アルゴリズムの選択

```go
// Sliding Window: 正確だがメモリ使用量が多い
type SlidingWindow struct {
    requests []time.Time
}

// Token Bucket: メモリ効率的で突発的トラフィックに対応
type TokenBucket struct {
    tokens     float64
    lastRefill time.Time
}

// Fixed Window: 実装が簡単だが境界問題あり
type FixedWindow struct {
    count  int
    window time.Time
}
```

### プロダクション考慮事項

```go
// Redis を使った分散レートリミット
func (rl *RateLimiter) checkRedisLimit(ip string) bool {
    key := fmt.Sprintf("rate_limit:%s", ip)
    count, err := rl.redis.Incr(key).Result()
    if err != nil {
        return true // フェイルオープン
    }
    
    if count == 1 {
        rl.redis.Expire(key, rl.window)
    }
    
    return count <= int64(rl.limit)
}
```

### テスト戦略

- 並行リクエストでの競合状態テスト
- 時間境界でのウィンドウ動作テスト
- メモリリーク確認のための長時間テスト
- 異なるIPからの同時アクセステスト

これらの実装により、プロダクション環境で使用できる堅牢なレートリミットミドルウェアの基礎を学ぶことができます。