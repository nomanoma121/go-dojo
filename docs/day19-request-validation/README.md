# Day 19: Request Validation

## 🎯 本日の目標 (Today's Goal)

包括的で高性能なリクエストバリデーションシステムを実装し、セキュアで堅牢なWeb APIを構築する。カスタムバリデーションルール、多言語対応、パフォーマンス最適化を含む実用的なバリデーションフレームワークを習得する。

## 📖 解説 (Explanation)

### Request Validationの重要性

```go
// 【Request Validationの重要性】SQLインジェクションと業務ロジック破壊からの保護
// ❌ 問題例：バリデーション不備による壊滅的セキュリティ侵害
func catastrophicNoValidation() {
    // 🚨 災害例：バリデーションなしでのSQLインジェクションとデータ全損失
    
    http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        var userReq struct {
            Name     string `json:"name"`
            Email    string `json:"email"`
            Age      int    `json:"age"`
            Role     string `json:"role"`
            Password string `json:"password"`
        }
        
        // ❌ バリデーションなしでJSONを直接デコード
        json.NewDecoder(r.Body).Decode(&userReq)
        
        log.Printf("Creating user: %+v", userReq)
        
        // ❌ SQL直接実行（SQLインジェクション脆弱性）
        query := fmt.Sprintf("INSERT INTO users (name, email, age, role, password) VALUES ('%s', '%s', %d, '%s', '%s')",
            userReq.Name, userReq.Email, userReq.Age, userReq.Role, userReq.Password)
        
        // 攻撃例：
        // name: "admin'; DROP TABLE users; --"
        // 実行されるSQL: INSERT INTO users (name, email, age, role, password) VALUES ('admin'; DROP TABLE users; --', ...)
        // ❌ 結果：usersテーブル完全削除、全ユーザーデータ消失
        
        db.Exec(query)
        
        // ❌ 権限昇格攻撃
        // role: "super_admin" → 管理者権限取得
        // age: -999999999 → integer overflow
        // email: "<script>alert('XSS')</script>" → XSS攻撃
        
        // ❌ 業務ロジック破壊
        // name: 1MB のデータ → メモリ枯渇
        // password: "" → 空パスワードでアカウント作成
        
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "success",
            "message": "User created",
        })
    })
    
    log.Println("❌ Starting server without validation...")
    http.ListenAndServe(":8080", nil)
    // 結果：データベース破壊、権限昇格、XSS攻撃、業務停止
}

// ✅ 正解：エンタープライズ級Request Validationシステム
type EnterpriseValidator struct {
    // 【基本機能】
    rules           map[string][]ValidationRule  // フィールド別ルール
    customRules     map[string]CustomValidator   // カスタムバリデーター
    
    // 【高度な機能】
    contextRules    *ContextualRules            // 文脈依存ルール
    schemaRegistry  *SchemaRegistry             // スキーマ管理
    sanitizer       *InputSanitizer             // 入力サニタイゼーション
    
    // 【セキュリティ】
    sqlInjectionDetector *SQLInjectionDetector   // SQLインジェクション検知
    xssDetector         *XSSDetector            // XSS攻撃検知
    rateLimiter         *ValidationRateLimiter   // バリデーション回数制限
    
    // 【監視・ログ】
    metrics         *ValidationMetrics          // バリデーションメトリクス
    logger          *log.Logger                 // 構造化ログ
    alertManager    *ValidationAlertManager     // アラート管理
    
    // 【パフォーマンス】
    cache           *ValidationCache            // バリデーション結果キャッシュ
    threadPool      *ValidationThreadPool      // 並列バリデーション
    
    // 【国際化】
    i18n            *I18nManager                // 多言語エラーメッセージ
    
    mu              sync.RWMutex                // スレッドセーフティ
}

// 【重要関数】エンタープライズバリデーター初期化
func NewEnterpriseValidator(config *ValidatorConfig) *EnterpriseValidator {
    validator := &EnterpriseValidator{
        rules:       make(map[string][]ValidationRule),
        customRules: make(map[string]CustomValidator),
        
        contextRules:         NewContextualRules(),
        schemaRegistry:       NewSchemaRegistry(),
        sanitizer:           NewInputSanitizer(),
        sqlInjectionDetector: NewSQLInjectionDetector(),
        xssDetector:         NewXSSDetector(),
        rateLimiter:         NewValidationRateLimiter(config.MaxValidationsPerIP),
        metrics:             NewValidationMetrics(),
        logger:              log.New(os.Stdout, "[VALIDATOR] ", log.LstdFlags),
        alertManager:        NewValidationAlertManager(),
        cache:               NewValidationCache(config.CacheSize),
        threadPool:          NewValidationThreadPool(config.ThreadPoolSize),
        i18n:                NewI18nManager(config.DefaultLanguage),
    }
    
    // 【基本ルール登録】
    validator.registerDefaultRules()
    
    // 【セキュリティルール登録】
    validator.registerSecurityRules()
    
    // 【業界標準ルール登録】
    validator.registerIndustryStandardRules()
    
    validator.logger.Printf("🚀 Enterprise validator initialized")
    validator.logger.Printf("   Validation rules: %d registered", len(validator.rules))
    validator.logger.Printf("   Security detectors: SQL injection, XSS enabled")
    validator.logger.Printf("   Thread pool size: %d", config.ThreadPoolSize)
    
    return validator
}

// 【核心メソッド】包括的リクエストバリデーション
func (v *EnterpriseValidator) ValidateRequest(r *http.Request, target interface{}, clientIP string) (*ValidationResult, error) {
    startTime := time.Now()
    requestID := generateValidationRequestID()
    
    // 【STEP 1】レート制限チェック
    if !v.rateLimiter.AllowValidation(clientIP) {
        v.metrics.RecordRateLimitHit(clientIP)
        return nil, &ValidationError{
            Type:    "RATE_LIMIT_EXCEEDED",
            Message: "Too many validation requests",
            Field:   "request",
        }
    }
    
    // 【STEP 2】Content-Type事前検証
    if err := v.validateContentType(r); err != nil {
        v.metrics.RecordValidationError("content_type", clientIP)
        return nil, err
    }
    
    // 【STEP 3】リクエストボディサイズ検証
    if err := v.validateBodySize(r); err != nil {
        v.metrics.RecordValidationError("body_size", clientIP)
        return nil, err
    }
    
    // 【STEP 4】JSON構造の事前検証
    bodyBytes, err := ioutil.ReadAll(r.Body)
    if err != nil {
        return nil, &ValidationError{
            Type:    "BODY_READ_ERROR",
            Message: "Failed to read request body",
            Field:   "body",
        }
    }
    
    // ボディを復元（後続処理のため）
    r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
    
    // 【STEP 5】SQLインジェクション検知
    if suspiciousSQL := v.sqlInjectionDetector.DetectSQLInjection(string(bodyBytes)); len(suspiciousSQL) > 0 {
        v.alertManager.TriggerSQLInjectionAlert(clientIP, suspiciousSQL)
        v.metrics.RecordSecurityThreat("sql_injection", clientIP)
        
        v.logger.Printf("❌ SQL injection attempt detected from %s: %v", clientIP, suspiciousSQL)
        return nil, &SecurityValidationError{
            Type:           "SQL_INJECTION_DETECTED",
            Message:        "Potential SQL injection attack detected",
            ThreatLevel:    "HIGH",
            DetectedTokens: suspiciousSQL,
            ClientIP:       clientIP,
        }
    }
    
    // 【STEP 6】XSS攻撃検知
    if xssPayloads := v.xssDetector.DetectXSS(string(bodyBytes)); len(xssPayloads) > 0 {
        v.alertManager.TriggerXSSAlert(clientIP, xssPayloads)
        v.metrics.RecordSecurityThreat("xss", clientIP)
        
        v.logger.Printf("❌ XSS attack detected from %s: %v", clientIP, xssPayloads)
        return nil, &SecurityValidationError{
            Type:           "XSS_DETECTED",
            Message:        "Potential XSS attack detected",
            ThreatLevel:    "HIGH",
            DetectedTokens: xssPayloads,
            ClientIP:       clientIP,
        }
    }
    
    // 【STEP 7】JSONデコードと基本検証
    if err := json.Unmarshal(bodyBytes, target); err != nil {
        v.metrics.RecordValidationError("json_decode", clientIP)
        return nil, &ValidationError{
            Type:    "INVALID_JSON",
            Message: "Invalid JSON format: " + err.Error(),
            Field:   "body",
        }
    }
    
    // 【STEP 8】入力サニタイゼーション
    sanitizedTarget := v.sanitizer.SanitizeInput(target)
    
    // 【STEP 9】キャッシュチェック
    cacheKey := v.generateCacheKey(sanitizedTarget, clientIP)
    if cachedResult, found := v.cache.Get(cacheKey); found {
        v.metrics.RecordCacheHit()
        return cachedResult.(*ValidationResult), nil
    }
    
    // 【STEP 10】並列バリデーション実行
    validationResult := v.threadPool.ExecuteValidation(func() *ValidationResult {
        return v.performDetailedValidation(sanitizedTarget, clientIP, requestID)
    })
    
    // 【STEP 11】結果キャッシュ
    if validationResult.IsValid {
        v.cache.Set(cacheKey, validationResult)
    }
    
    // 【STEP 12】メトリクス記録
    duration := time.Since(startTime)
    v.metrics.RecordValidationDuration(duration)
    v.metrics.RecordValidationResult(validationResult.IsValid, clientIP)
    
    v.logger.Printf("✅ Validation completed: request=%s, valid=%t, duration=%v", 
        requestID, validationResult.IsValid, duration)
    
    return validationResult, nil
}

// 【詳細バリデーション実行】
func (v *EnterpriseValidator) performDetailedValidation(target interface{}, clientIP, requestID string) *ValidationResult {
    result := &ValidationResult{
        RequestID:    requestID,
        ClientIP:     clientIP,
        IsValid:      true,
        Errors:       make([]ValidationError, 0),
        Warnings:     make([]ValidationWarning, 0),
        Suggestions:  make([]string, 0),
        ProcessedAt:  time.Now(),
    }
    
    targetValue := reflect.ValueOf(target)
    if targetValue.Kind() == reflect.Ptr {
        targetValue = targetValue.Elem()
    }
    targetType := targetValue.Type()
    
    // 【フィールド単位でのバリデーション】
    for i := 0; i < targetValue.NumField(); i++ {
        field := targetValue.Field(i)
        fieldType := targetType.Field(i)
        fieldName := fieldType.Name
        
        // JSONタグからフィールド名を取得
        if jsonTag := fieldType.Tag.Get("json"); jsonTag != "" {
            if commaIndex := strings.Index(jsonTag, ","); commaIndex != -1 {
                fieldName = jsonTag[:commaIndex]
            } else {
                fieldName = jsonTag
            }
        }
        
        // 【基本ルール適用】
        if rules, exists := v.rules[fieldName]; exists {
            for _, rule := range rules {
                if err := rule.Validate(field.Interface(), fieldName); err != nil {
                    result.Errors = append(result.Errors, *err)
                    result.IsValid = false
                }
            }
        }
        
        // 【カスタムルール適用】
        if customRule, exists := v.customRules[fieldName]; exists {
            if err := customRule.ValidateCustom(field.Interface(), fieldName, target); err != nil {
                result.Errors = append(result.Errors, *err)
                result.IsValid = false
            }
        }
        
        // 【文脈依存ルール適用】
        if contextErr := v.contextRules.ValidateInContext(fieldName, field.Interface(), target); contextErr != nil {
            result.Errors = append(result.Errors, *contextErr)
            result.IsValid = false
        }
        
        // 【型固有バリデーション】
        v.performTypeSpecificValidation(field, fieldName, result)
    }
    
    // 【クロスフィールドバリデーション】
    v.performCrossFieldValidation(target, result)
    
    // 【業務ルールバリデーション】
    v.performBusinessRuleValidation(target, result)
    
    return result
}

// 【実用例】高セキュリティユーザー登録API
func SecureUserRegistrationAPI() {
    config := &ValidatorConfig{
        MaxValidationsPerIP: 100,
        CacheSize:          1000,
        ThreadPoolSize:     10,
        DefaultLanguage:    "ja",
    }
    
    validator := NewEnterpriseValidator(config)
    
    // ユーザー登録構造体
    type UserRegistration struct {
        Name            string `json:"name" validate:"required,min=2,max=50,no_sql_injection"`
        Email           string `json:"email" validate:"required,email,unique_email"`
        Password        string `json:"password" validate:"required,strong_password,min=12"`
        ConfirmPassword string `json:"confirm_password" validate:"required,matches_password"`
        Age             int    `json:"age" validate:"required,min=13,max=120"`
        Role            string `json:"role" validate:"required,allowed_roles"`
        PhoneNumber     string `json:"phone_number" validate:"phone_format"`
        Terms           bool   `json:"terms" validate:"required,must_be_true"`
    }
    
    http.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        var userReq UserRegistration
        clientIP := getClientIP(r)
        
        // 【包括的バリデーション実行】
        validationResult, err := validator.ValidateRequest(r, &userReq, clientIP)
        if err != nil {
            // セキュリティエラーの場合
            if secErr, ok := err.(*SecurityValidationError); ok {
                http.Error(w, "Security violation detected", http.StatusForbidden)
                log.Printf("🚨 Security threat: %+v", secErr)
                return
            }
            
            // 一般的なバリデーションエラー
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        
        if !validationResult.IsValid {
            // バリデーションエラーの詳細レスポンス
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusBadRequest)
            json.NewEncoder(w).Encode(map[string]interface{}{
                "status":      "validation_failed",
                "errors":      validationResult.Errors,
                "warnings":    validationResult.Warnings,
                "suggestions": validationResult.Suggestions,
            })
            return
        }
        
        // 【データベース安全保存】
        // プリペアドステートメントでSQLインジェクション完全防止
        stmt, err := db.Prepare("INSERT INTO users (name, email, password_hash, age, role, phone_number) VALUES (?, ?, ?, ?, ?, ?)")
        if err != nil {
            http.Error(w, "Database error", http.StatusInternalServerError)
            return
        }
        defer stmt.Close()
        
        hashedPassword := hashPassword(userReq.Password)
        
        _, err = stmt.Exec(userReq.Name, userReq.Email, hashedPassword, userReq.Age, userReq.Role, userReq.PhoneNumber)
        if err != nil {
            http.Error(w, "Failed to create user", http.StatusInternalServerError)
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status":      "success",
            "message":     "User created successfully",
            "user_id":     generateUserID(),
            "validation_score": validationResult.GetQualityScore(),
        })
    })
    
    log.Printf("🔒 Secure user registration API starting on :8080")
    log.Printf("   Security features: SQL injection protection, XSS detection, input sanitization")
    log.Printf("   Validation features: Multi-language, caching, parallel processing")
    
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### バリデーションの階層

#### 1. 構文バリデーション

```go
// JSON形式の検証
func ValidateJSONStructure(r *http.Request) error {
    var temp interface{}
    decoder := json.NewDecoder(r.Body)
    decoder.DisallowUnknownFields() // 不明なフィールドを拒否
    
    if err := decoder.Decode(&temp); err != nil {
        return &ValidationError{
            Field:   "body",
            Message: "Invalid JSON format",
            Code:    "INVALID_JSON",
        }
    }
    
    return nil
}

// Content-Typeの検証
func ValidateContentType(expectedType string) func(*http.Request) error {
    return func(r *http.Request) error {
        contentType := r.Header.Get("Content-Type")
        if !strings.HasPrefix(contentType, expectedType) {
            return &ValidationError{
                Field:   "Content-Type",
                Message: fmt.Sprintf("Expected %s, got %s", expectedType, contentType),
                Code:    "INVALID_CONTENT_TYPE",
            }
        }
        return nil
    }
}
```

#### 2. セマンティックバリデーション

```go
type User struct {
    ID       string    `json:"id" validate:"required,uuid4"`
    Email    string    `json:"email" validate:"required,email"`
    Age      int       `json:"age" validate:"required,min=18,max=120"`
    Name     string    `json:"name" validate:"required,min=2,max=50"`
    Password string    `json:"password" validate:"required,password_strength"`
    Bio      string    `json:"bio" validate:"max=500"`
    Website  string    `json:"website" validate:"omitempty,url"`
    Country  string    `json:"country" validate:"required,country_code"`
    CreatedAt time.Time `json:"created_at" validate:"required"`
}

// カスタムバリデーションルール
func passwordStrength(fl validator.FieldLevel) bool {
    password := fl.Field().String()
    
    // 最低8文字
    if len(password) < 8 {
        return false
    }
    
    hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
    hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
    hasDigit := regexp.MustCompile(`\d`).MatchString(password)
    hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)
    
    return hasUpper && hasLower && hasDigit && hasSpecial
}
```

#### 3. ビジネスルールバリデーション

```go
type OrderValidationContext struct {
    UserID    string
    ProductDB ProductRepository
    UserDB    UserRepository
    PriceAPI  PricingService
}

func (ctx *OrderValidationContext) ValidateOrder(order *Order) []ValidationError {
    var errors []ValidationError
    
    // ユーザー存在確認
    if user, err := ctx.UserDB.GetByID(order.UserID); err != nil || user == nil {
        errors = append(errors, ValidationError{
            Field:   "user_id",
            Message: "User does not exist",
            Code:    "USER_NOT_FOUND",
        })
    }
    
    // 商品とその在庫確認
    for i, item := range order.Items {
        product, err := ctx.ProductDB.GetByID(item.ProductID)
        if err != nil || product == nil {
            errors = append(errors, ValidationError{
                Field:   fmt.Sprintf("items[%d].product_id", i),
                Message: "Product not found",
                Code:    "PRODUCT_NOT_FOUND",
            })
            continue
        }
        
        if product.Stock < item.Quantity {
            errors = append(errors, ValidationError{
                Field:   fmt.Sprintf("items[%d].quantity", i),
                Message: "Insufficient stock",
                Code:    "INSUFFICIENT_STOCK",
                Metadata: map[string]interface{}{
                    "available": product.Stock,
                    "requested": item.Quantity,
                },
            })
        }
        
        // 価格検証
        currentPrice, err := ctx.PriceAPI.GetCurrentPrice(item.ProductID)
        if err == nil && math.Abs(currentPrice-item.UnitPrice) > 0.01 {
            errors = append(errors, ValidationError{
                Field:   fmt.Sprintf("items[%d].unit_price", i),
                Message: "Price has changed",
                Code:    "PRICE_CHANGED",
                Metadata: map[string]interface{}{
                    "current_price": currentPrice,
                    "provided_price": item.UnitPrice,
                },
            })
        }
    }
    
    return errors
}
```

### 高度なバリデーションパターン

#### 1. 階層バリデーション

```go
type ValidationResult struct {
    IsValid bool                    `json:"is_valid"`
    Errors  []ValidationError       `json:"errors,omitempty"`
    Warnings []ValidationWarning    `json:"warnings,omitempty"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ValidationPipeline struct {
    stages []ValidationStage
}

type ValidationStage struct {
    Name      string
    Validator func(interface{}) ValidationResult
    StopOnError bool
}

func (vp *ValidationPipeline) Validate(data interface{}) ValidationResult {
    var allErrors []ValidationError
    var allWarnings []ValidationWarning
    metadata := make(map[string]interface{})
    
    for _, stage := range vp.stages {
        result := stage.Validator(data)
        
        allErrors = append(allErrors, result.Errors...)
        allWarnings = append(allWarnings, result.Warnings...)
        
        for k, v := range result.Metadata {
            metadata[k] = v
        }
        
        if !result.IsValid && stage.StopOnError {
            break
        }
    }
    
    return ValidationResult{
        IsValid:  len(allErrors) == 0,
        Errors:   allErrors,
        Warnings: allWarnings,
        Metadata: metadata,
    }
}
```

#### 2. 条件付きバリデーション

```go
type ConditionalValidator struct {
    Condition func(interface{}) bool
    Validator func(interface{}) ValidationResult
}

func (cv *ConditionalValidator) Validate(data interface{}) ValidationResult {
    if !cv.Condition(data) {
        return ValidationResult{IsValid: true}
    }
    
    return cv.Validator(data)
}

// 使用例
func NewUserValidator() *ValidationPipeline {
    pipeline := &ValidationPipeline{}
    
    // 管理者の場合のみ追加検証
    adminValidator := &ConditionalValidator{
        Condition: func(data interface{}) bool {
            if user, ok := data.(*User); ok {
                return user.Role == "admin"
            }
            return false
        },
        Validator: func(data interface{}) ValidationResult {
            // 管理者固有のバリデーション
            return validateAdminFields(data.(*User))
        },
    }
    
    pipeline.AddStage("admin_validation", adminValidator.Validate, false)
    return pipeline
}
```

#### 3. 非同期バリデーション

```go
type AsyncValidator struct {
    validators []func(interface{}) <-chan ValidationResult
    timeout    time.Duration
}

func (av *AsyncValidator) Validate(data interface{}) ValidationResult {
    ctx, cancel := context.WithTimeout(context.Background(), av.timeout)
    defer cancel()
    
    results := make([]<-chan ValidationResult, len(av.validators))
    
    // 全バリデーターを並行実行
    for i, validator := range av.validators {
        results[i] = validator(data)
    }
    
    var allErrors []ValidationError
    var allWarnings []ValidationWarning
    
    // 結果を収集
    for _, resultChan := range results {
        select {
        case result := <-resultChan:
            allErrors = append(allErrors, result.Errors...)
            allWarnings = append(allWarnings, result.Warnings...)
        case <-ctx.Done():
            allErrors = append(allErrors, ValidationError{
                Field:   "timeout",
                Message: "Validation timeout",
                Code:    "VALIDATION_TIMEOUT",
            })
        }
    }
    
    return ValidationResult{
        IsValid:  len(allErrors) == 0,
        Errors:   allErrors,
        Warnings: allWarnings,
    }
}
```

#### 4. キャッシュ付きバリデーション

```go
type CachedValidator struct {
    cache      sync.Map
    validator  func(interface{}) ValidationResult
    keyFunc    func(interface{}) string
    ttl        time.Duration
}

type CachedResult struct {
    Result    ValidationResult
    ExpiresAt time.Time
}

func (cv *CachedValidator) Validate(data interface{}) ValidationResult {
    key := cv.keyFunc(data)
    
    if cached, ok := cv.cache.Load(key); ok {
        if cachedResult, ok := cached.(*CachedResult); ok {
            if time.Now().Before(cachedResult.ExpiresAt) {
                return cachedResult.Result
            }
            cv.cache.Delete(key)
        }
    }
    
    result := cv.validator(data)
    
    cv.cache.Store(key, &CachedResult{
        Result:    result,
        ExpiresAt: time.Now().Add(cv.ttl),
    })
    
    return result
}
```

### 多言語対応バリデーション

```go
type LocalizedValidator struct {
    validator   func(interface{}) ValidationResult
    translator  Translator
    defaultLang string
}

type Translator interface {
    Translate(key, lang string, params map[string]interface{}) string
}

func (lv *LocalizedValidator) ValidateWithLocale(data interface{}, lang string) ValidationResult {
    result := lv.validator(data)
    
    if lang == "" {
        lang = lv.defaultLang
    }
    
    // エラーメッセージを翻訳
    for i := range result.Errors {
        result.Errors[i].Message = lv.translator.Translate(
            result.Errors[i].Code,
            lang,
            result.Errors[i].Metadata,
        )
    }
    
    return result
}

// 翻訳例
var translations = map[string]map[string]string{
    "REQUIRED_FIELD": {
        "en": "Field {{.field}} is required",
        "ja": "{{.field}}は必須項目です",
        "es": "El campo {{.field}} es requerido",
    },
    "INVALID_EMAIL": {
        "en": "Invalid email format",
        "ja": "メールアドレスの形式が正しくありません",
        "es": "Formato de correo electrónico inválido",
    },
}
```

### パフォーマンス最適化

#### 1. 早期リターン

```go
func OptimizedValidator(data interface{}) ValidationResult {
    // 高速で重要なチェックを最初に実行
    if err := quickSecurityCheck(data); err != nil {
        return ValidationResult{
            IsValid: false,
            Errors:  []ValidationError{*err},
        }
    }
    
    // より複雑なバリデーションは後で実行
    return detailedValidation(data)
}
```

#### 2. バッチバリデーション

```go
type BatchValidator struct {
    batchSize int
    validator func([]interface{}) []ValidationResult
}

func (bv *BatchValidator) ValidateBatch(items []interface{}) []ValidationResult {
    var results []ValidationResult
    
    for i := 0; i < len(items); i += bv.batchSize {
        end := i + bv.batchSize
        if end > len(items) {
            end = len(items)
        }
        
        batch := items[i:end]
        batchResults := bv.validator(batch)
        results = append(results, batchResults...)
    }
    
    return results
}
```

### 実用的な統合例

```go
type RequestValidator struct {
    structValidator   *validator.Validate
    businessValidator BusinessValidator
    securityValidator SecurityValidator
    translator       Translator
    cache           *CachedValidator
    metrics         *ValidationMetrics
}

func NewRequestValidator() *RequestValidator {
    v := validator.New()
    
    // カスタムバリデーターを登録
    v.RegisterValidation("password_strength", passwordStrength)
    v.RegisterValidation("country_code", countryCodeValidator)
    
    return &RequestValidator{
        structValidator: v,
        translator:     NewTranslator(),
        metrics:       NewValidationMetrics(),
    }
}

func (rv *RequestValidator) ValidateRequest(w http.ResponseWriter, r *http.Request, target interface{}, lang string) error {
    start := time.Now()
    defer func() {
        rv.metrics.RecordValidationDuration(time.Since(start))
    }()
    
    // 1. Content-Type検証
    if err := ValidateContentType("application/json")(r); err != nil {
        rv.metrics.RecordValidationError("content_type")
        return rv.writeErrorResponse(w, []ValidationError{*err.(*ValidationError)}, lang)
    }
    
    // 2. JSON構造検証
    if err := json.NewDecoder(r.Body).Decode(target); err != nil {
        rv.metrics.RecordValidationError("json_decode")
        return rv.writeErrorResponse(w, []ValidationError{{
            Field:   "body",
            Message: "Invalid JSON",
            Code:    "INVALID_JSON",
        }}, lang)
    }
    
    // 3. 構造バリデーション
    if err := rv.structValidator.Struct(target); err != nil {
        rv.metrics.RecordValidationError("struct_validation")
        validationErrors := rv.convertValidatorErrors(err)
        return rv.writeErrorResponse(w, validationErrors, lang)
    }
    
    // 4. ビジネスルールバリデーション
    if businessErrors := rv.businessValidator.Validate(target); len(businessErrors) > 0 {
        rv.metrics.RecordValidationError("business_rules")
        return rv.writeErrorResponse(w, businessErrors, lang)
    }
    
    // 5. セキュリティバリデーション
    if securityErrors := rv.securityValidator.Validate(target, r); len(securityErrors) > 0 {
        rv.metrics.RecordValidationError("security")
        return rv.writeErrorResponse(w, securityErrors, lang)
    }
    
    rv.metrics.RecordValidationSuccess()
    return nil
}

func (rv *RequestValidator) writeErrorResponse(w http.ResponseWriter, errors []ValidationError, lang string) error {
    // エラーメッセージを翻訳
    for i := range errors {
        errors[i].Message = rv.translator.Translate(errors[i].Code, lang, errors[i].Metadata)
    }
    
    response := map[string]interface{}{
        "error":   "validation_failed",
        "details": errors,
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusBadRequest)
    return json.NewEncoder(w).Encode(response)
}
```

## 📝 課題 (The Problem)

以下の機能を持つ包括的なRequest Validationシステムを実装してください：

### 1. RequestValidator の実装

```go
type RequestValidator struct {
    structValidator   *validator.Validate
    customValidators  map[string]validator.Func
    businessRules     []BusinessRule
    securityRules     []SecurityRule
    translator        Translator
    cache            ValidationCache
}
```

### 2. 必要な機能

- **階層バリデーション**: 構文 → セマンティック → ビジネスルール → セキュリティ
- **カスタムバリデーター**: ドメイン固有のバリデーションルール
- **多言語対応**: エラーメッセージの国際化
- **パフォーマンス最適化**: キャッシング、早期リターン、並行処理
- **詳細なエラー情報**: フィールド固有の情報とメタデータ

### 3. バリデーションルール

- Email, Phone, URL, UUID
- Password Strength
- Country Code, Currency Code
- Date Range, Age Verification
- File Size, Image Dimensions
- Credit Card, Banking Details

### 4. ビジネスルール

- User Uniqueness Check
- Product Availability
- Pricing Validation
- Permission Verification
- Quota Limits

### 5. セキュリティチェック

- SQL Injection Prevention
- XSS Protection
- Rate Limiting
- IP Filtering
- Content Size Limits

## ✅ 期待される挙動 (Expected Behavior)

実装が正しく完了すると、以下のようなテスト結果が得られます：

```bash
$ go test -v
=== RUN   TestRequestValidator_BasicValidation
    main_test.go:45: Basic struct validation working correctly
--- PASS: TestRequestValidator_BasicValidation (0.01s)

=== RUN   TestRequestValidator_CustomValidators
    main_test.go:65: Custom validation rules applied correctly
--- PASS: TestRequestValidator_CustomValidators (0.01s)

=== RUN   TestRequestValidator_BusinessRules
    main_test.go:85: Business rule validation working
--- PASS: TestRequestValidator_BusinessRules (0.03s)

=== RUN   TestRequestValidator_Localization
    main_test.go:105: Localized error messages returned correctly
--- PASS: TestRequestValidator_Localization (0.01s)

=== RUN   TestRequestValidator_Performance
    main_test.go:125: Validation completed within performance threshold
--- PASS: TestRequestValidator_Performance (0.02s)

PASS
ok      day19-request-validation   0.156s
```

## 💡 ヒント (Hints)

実装に詰まった場合は、以下のヒントを参考にしてください：

### 基本的なバリデーター設定

```go
func setupValidator() *validator.Validate {
    v := validator.New()
    
    // カスタムタグ名を設定
    v.RegisterTagNameFunc(func(fld reflect.StructField) string {
        name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
        if name == "-" {
            return ""
        }
        return name
    })
    
    // カスタムバリデーターを登録
    v.RegisterValidation("password_strength", passwordStrength)
    
    return v
}
```

### エラー変換

```go
func convertValidatorErrors(err error) []ValidationError {
    var errors []ValidationError
    
    if validationErrors, ok := err.(validator.ValidationErrors); ok {
        for _, fieldError := range validationErrors {
            errors = append(errors, ValidationError{
                Field:   fieldError.Field(),
                Message: getErrorMessage(fieldError),
                Code:    fieldError.Tag(),
                Value:   fieldError.Value(),
            })
        }
    }
    
    return errors
}
```

### キャッシュ実装

```go
type ValidationCache struct {
    cache sync.Map
    ttl   time.Duration
}

func (vc *ValidationCache) Get(key string) (ValidationResult, bool) {
    if value, ok := vc.cache.Load(key); ok {
        if cached, ok := value.(*CachedValidationResult); ok {
            if time.Now().Before(cached.ExpiresAt) {
                return cached.Result, true
            }
            vc.cache.Delete(key)
        }
    }
    return ValidationResult{}, false
}
```

## 🚀 発展課題 (Advanced Challenges)

基本実装が完了したら、以下の発展的な機能にもチャレンジしてみてください：

1. **GraphQL統合**: GraphQLスキーマバリデーション
2. **OpenAPI仕様**: OpenAPI仕様からの自動バリデーション生成
3. **機械学習**: 異常検知による新しいバリデーションルール発見
4. **分散バリデーション**: マイクロサービス間でのバリデーション結果共有
5. **リアルタイム更新**: 設定変更のホットリロード

Request Validationの実装を通じて、セキュアで保守可能なWeb APIの構築手法を習得しましょう！