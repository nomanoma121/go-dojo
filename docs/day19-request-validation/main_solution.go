package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ValidationError はバリデーションエラーの詳細情報
type ValidationError struct {
	Field    string                 `json:"field"`
	Message  string                 `json:"message"`
	Code     string                 `json:"code"`
	Value    interface{}            `json:"value,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ValidationResult はバリデーション結果
type ValidationResult struct {
	IsValid  bool              `json:"is_valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []ValidationError `json:"warnings,omitempty"`
}

// RequestValidator はリクエストバリデーター
type RequestValidator struct {
	customValidators map[string]ValidatorFunc
	businessRules    []BusinessRule
	securityRules    []SecurityRule
	translator       Translator
	cache           *ValidationCache
	metrics         *ValidationMetrics
}

// ValidatorFunc はカスタムバリデーター関数
type ValidatorFunc func(interface{}) bool

// BusinessRule はビジネスルールインターフェース
type BusinessRule interface {
	Validate(interface{}) []ValidationError
}

// SecurityRule はセキュリティルールインターフェース
type SecurityRule interface {
	Validate(interface{}, *http.Request) []ValidationError
}

// Translator は翻訳インターフェース
type Translator interface {
	Translate(code, lang string, params map[string]interface{}) string
}

// NewRequestValidator はRequestValidatorを初期化
func NewRequestValidator() *RequestValidator {
	translator := NewSimpleTranslator("en")
	setupDefaultTranslations(translator)
	
	return &RequestValidator{
		customValidators: make(map[string]ValidatorFunc),
		businessRules:    make([]BusinessRule, 0),
		securityRules:    make([]SecurityRule, 0),
		translator:       translator,
		cache:           NewValidationCache(5 * time.Minute),
		metrics:         NewValidationMetrics(),
	}
}

// RegisterValidator はカスタムバリデーターを登録
func (rv *RequestValidator) RegisterValidator(name string, fn ValidatorFunc) {
	rv.customValidators[name] = fn
}

// AddBusinessRule はビジネスルールを追加
func (rv *RequestValidator) AddBusinessRule(rule BusinessRule) {
	rv.businessRules = append(rv.businessRules, rule)
}

// AddSecurityRule はセキュリティルールを追加
func (rv *RequestValidator) AddSecurityRule(rule SecurityRule) {
	rv.securityRules = append(rv.securityRules, rule)
}

// ValidateRequest はリクエストをバリデーション
func (rv *RequestValidator) ValidateRequest(w http.ResponseWriter, r *http.Request, target interface{}, lang string) error {
	start := time.Now()
	defer func() {
		rv.metrics.RecordSuccess(time.Since(start))
	}()
	
	// 1. Content-Type検証
	contentType := r.Header.Get("Content-Type")
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		if !strings.HasPrefix(contentType, "application/json") {
			rv.metrics.RecordError("content_type", time.Since(start))
			return rv.writeErrorResponse(w, []ValidationError{{
				Field:   "Content-Type",
				Message: "Invalid content type",
				Code:    "INVALID_CONTENT_TYPE",
				Value:   contentType,
			}}, lang)
		}
	}
	
	// 2. JSONデコード
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		
		if err := decoder.Decode(target); err != nil {
			rv.metrics.RecordError("json_decode", time.Since(start))
			return rv.writeErrorResponse(w, []ValidationError{{
				Field:   "body",
				Message: "Invalid JSON format",
				Code:    "INVALID_JSON",
				Value:   err.Error(),
			}}, lang)
		}
	}
	
	// 3. 構造バリデーション
	var allErrors []ValidationError
	structErrors := rv.validateStruct(target)
	allErrors = append(allErrors, structErrors...)
	
	// 4. カスタムバリデーション
	customErrors := rv.validateCustom(target)
	allErrors = append(allErrors, customErrors...)
	
	// 5. ビジネスルールバリデーション
	for _, rule := range rv.businessRules {
		businessErrors := rule.Validate(target)
		allErrors = append(allErrors, businessErrors...)
	}
	
	// 6. セキュリティバリデーション
	for _, rule := range rv.securityRules {
		securityErrors := rule.Validate(target, r)
		allErrors = append(allErrors, securityErrors...)
	}
	
	// エラーがある場合はレスポンスを書き込み
	if len(allErrors) > 0 {
		rv.metrics.RecordError("validation", time.Since(start))
		return rv.writeErrorResponse(w, allErrors, lang)
	}
	
	return nil
}

// validateStruct は構造体バリデーション
func (rv *RequestValidator) validateStruct(data interface{}) []ValidationError {
	var errors []ValidationError
	
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	if val.Kind() != reflect.Struct {
		return errors
	}
	
	typ := val.Type()
	
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)
		
		// validateタグを取得
		validateTag := field.Tag.Get("validate")
		if validateTag == "" {
			continue
		}
		
		jsonTag := field.Tag.Get("json")
		fieldName := field.Name
		if jsonTag != "" && jsonTag != "-" {
			fieldName = strings.Split(jsonTag, ",")[0]
		}
		
		// バリデーションルールを解析
		rules := parseValidationTags(validateTag)
		
		for _, rule := range rules {
			if err := rv.validateField(fieldName, fieldValue.Interface(), rule); err != nil {
				errors = append(errors, *err)
			}
		}
	}
	
	return errors
}

// validateField は単一フィールドのバリデーション
func (rv *RequestValidator) validateField(fieldName string, value interface{}, rule string) *ValidationError {
	parts := strings.Split(rule, "=")
	ruleName := parts[0]
	var ruleValue string
	if len(parts) > 1 {
		ruleValue = parts[1]
	}
	
	switch ruleName {
	case "required":
		if isEmpty(value) {
			return &ValidationError{
				Field:   fieldName,
				Message: "Field is required",
				Code:    "REQUIRED",
				Value:   value,
			}
		}
	case "min":
		if minVal, err := strconv.Atoi(ruleValue); err == nil {
			if !validateMin(value, minVal) {
				return &ValidationError{
					Field:   fieldName,
					Message: fmt.Sprintf("Minimum value is %d", minVal),
					Code:    "MIN_VALUE",
					Value:   value,
					Metadata: map[string]interface{}{"min": minVal},
				}
			}
		}
	case "max":
		if maxVal, err := strconv.Atoi(ruleValue); err == nil {
			if !validateMax(value, maxVal) {
				return &ValidationError{
					Field:   fieldName,
					Message: fmt.Sprintf("Maximum value is %d", maxVal),
					Code:    "MAX_VALUE",
					Value:   value,
					Metadata: map[string]interface{}{"max": maxVal},
				}
			}
		}
	case "email":
		if !rv.customValidators["email"](value) {
			return &ValidationError{
				Field:   fieldName,
				Message: "Invalid email format",
				Code:    "INVALID_EMAIL",
				Value:   value,
			}
		}
	case "password_strength":
		if !rv.customValidators["password_strength"](value) {
			return &ValidationError{
				Field:   fieldName,
				Message: "Password does not meet strength requirements",
				Code:    "WEAK_PASSWORD",
				Value:   "***",
			}
		}
	case "url":
		if !rv.customValidators["url"](value) {
			return &ValidationError{
				Field:   fieldName,
				Message: "Invalid URL format",
				Code:    "INVALID_URL",
				Value:   value,
			}
		}
	case "phone":
		if !rv.customValidators["phone"](value) {
			return &ValidationError{
				Field:   fieldName,
				Message: "Invalid phone number format",
				Code:    "INVALID_PHONE",
				Value:   value,
			}
		}
	}
	
	return nil
}

// validateCustom はカスタムバリデーション
func (rv *RequestValidator) validateCustom(data interface{}) []ValidationError {
	var errors []ValidationError
	
	// 追加のカスタムバリデーションロジックをここに実装
	// 例：複合フィールドの検証、ビジネスロジック固有の検証など
	
	return errors
}

// writeErrorResponse はエラーレスポンスを作成
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

// バリデーションキャッシュ

// ValidationCache はバリデーション結果のキャッシュ
type ValidationCache struct {
	cache sync.Map
	ttl   time.Duration
}

// CachedValidationResult はキャッシュされたバリデーション結果
type CachedValidationResult struct {
	Result    ValidationResult
	ExpiresAt time.Time
}

// NewValidationCache はキャッシュを初期化
func NewValidationCache(ttl time.Duration) *ValidationCache {
	vc := &ValidationCache{
		ttl: ttl,
	}
	
	// 定期的にキャッシュをクリーンアップ
	go vc.cleanup()
	
	return vc
}

// Get はキャッシュから取得
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

// Set はキャッシュに保存
func (vc *ValidationCache) Set(key string, result ValidationResult) {
	cached := &CachedValidationResult{
		Result:    result,
		ExpiresAt: time.Now().Add(vc.ttl),
	}
	vc.cache.Store(key, cached)
}

// cleanup は期限切れのキャッシュエントリを削除
func (vc *ValidationCache) cleanup() {
	ticker := time.NewTicker(vc.ttl / 2)
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		vc.cache.Range(func(key, value interface{}) bool {
			if cached, ok := value.(*CachedValidationResult); ok {
				if now.After(cached.ExpiresAt) {
					vc.cache.Delete(key)
				}
			}
			return true
		})
	}
}

// バリデーションメトリクス

// ValidationMetrics はバリデーションメトリクス
type ValidationMetrics struct {
	totalValidations int64
	successCount     int64
	errorCount       int64
	avgDuration      time.Duration
	mu              sync.RWMutex
	totalDuration   time.Duration
}

// NewValidationMetrics はメトリクスを初期化
func NewValidationMetrics() *ValidationMetrics {
	return &ValidationMetrics{}
}

// RecordSuccess はバリデーション成功を記録
func (vm *ValidationMetrics) RecordSuccess(duration time.Duration) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	vm.totalValidations++
	vm.successCount++
	vm.totalDuration += duration
	vm.avgDuration = time.Duration(int64(vm.totalDuration) / vm.totalValidations)
}

// RecordError はバリデーションエラーを記録
func (vm *ValidationMetrics) RecordError(errorType string, duration time.Duration) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	
	vm.totalValidations++
	vm.errorCount++
	vm.totalDuration += duration
	vm.avgDuration = time.Duration(int64(vm.totalDuration) / vm.totalValidations)
}

// GetMetrics はメトリクスを取得
func (vm *ValidationMetrics) GetMetrics() map[string]interface{} {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	
	successRate := float64(0)
	if vm.totalValidations > 0 {
		successRate = float64(vm.successCount) / float64(vm.totalValidations) * 100
	}
	
	return map[string]interface{}{
		"total_validations": vm.totalValidations,
		"success_count":     vm.successCount,
		"error_count":       vm.errorCount,
		"success_rate":      successRate,
		"avg_duration_ms":   float64(vm.avgDuration.Nanoseconds()) / 1e6,
	}
}

// 翻訳機能

// SimpleTranslator は簡単な翻訳器
type SimpleTranslator struct {
	translations map[string]map[string]string
	defaultLang  string
	mu          sync.RWMutex
}

// NewSimpleTranslator は翻訳器を初期化
func NewSimpleTranslator(defaultLang string) *SimpleTranslator {
	return &SimpleTranslator{
		translations: make(map[string]map[string]string),
		defaultLang:  defaultLang,
	}
}

// Translate はメッセージを翻訳
func (st *SimpleTranslator) Translate(code, lang string, params map[string]interface{}) string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	// 指定言語の翻訳を検索
	if langMap, exists := st.translations[code]; exists {
		if message, exists := langMap[lang]; exists {
			return st.interpolate(message, params)
		}
		// デフォルト言語を試行
		if message, exists := langMap[st.defaultLang]; exists {
			return st.interpolate(message, params)
		}
	}
	
	// 翻訳が見つからない場合はコードをそのまま返す
	return code
}

// AddTranslation は翻訳を追加
func (st *SimpleTranslator) AddTranslation(code, lang, message string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	if st.translations[code] == nil {
		st.translations[code] = make(map[string]string)
	}
	st.translations[code][lang] = message
}

// interpolate はパラメータを置換
func (st *SimpleTranslator) interpolate(message string, params map[string]interface{}) string {
	if params == nil {
		return message
	}
	
	result := message
	for key, value := range params {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	
	return result
}

// カスタムバリデーター関数

// EmailValidator はメールアドレスバリデーター
func EmailValidator(value interface{}) bool {
	if str, ok := value.(string); ok {
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
		return emailRegex.MatchString(str)
	}
	return false
}

// PasswordStrengthValidator はパスワード強度バリデーター
func PasswordStrengthValidator(value interface{}) bool {
	if str, ok := value.(string); ok {
		// 最低8文字
		if len(str) < 8 {
			return false
		}
		
		// 大文字、小文字、数字、記号を含む
		hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(str)
		hasLower := regexp.MustCompile(`[a-z]`).MatchString(str)
		hasDigit := regexp.MustCompile(`\d`).MatchString(str)
		hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(str)
		
		return hasUpper && hasLower && hasDigit && hasSpecial
	}
	return false
}

// URLValidator はURLバリデーター
func URLValidator(value interface{}) bool {
	if str, ok := value.(string); ok {
		if str == "" {
			return true // omitemptyの場合は空文字を許可
		}
		_, err := url.ParseRequestURI(str)
		return err == nil
	}
	return false
}

// PhoneValidator は電話番号バリデーター
func PhoneValidator(value interface{}) bool {
	if str, ok := value.(string); ok {
		if str == "" {
			return true // omitemptyの場合は空文字を許可
		}
		// 国際電話番号形式をチェック
		phoneRegex := regexp.MustCompile(`^\+?[\d\s\-\(\)]{10,15}$`)
		return phoneRegex.MatchString(str)
	}
	return false
}

// ビジネスルール

// UserUniquenessRule はユーザー一意性ルール
type UserUniquenessRule struct {
	userRepository UserRepository
}

// UserRepository はユーザーリポジトリインターフェース
type UserRepository interface {
	ExistsByEmail(email string) (bool, error)
	ExistsByUsername(username string) (bool, error)
}

// Validate はユーザー一意性ルールを実行
func (uur *UserUniquenessRule) Validate(data interface{}) []ValidationError {
	var errors []ValidationError
	
	if user, ok := data.(*User); ok {
		// メールアドレスの重複チェック
		if exists, err := uur.userRepository.ExistsByEmail(user.Email); err == nil && exists {
			errors = append(errors, ValidationError{
				Field:   "email",
				Message: "Email address already exists",
				Code:    "EMAIL_EXISTS",
				Value:   user.Email,
			})
		}
		
		// ユーザー名の重複チェック
		if exists, err := uur.userRepository.ExistsByUsername(user.Username); err == nil && exists {
			errors = append(errors, ValidationError{
				Field:   "username",
				Message: "Username already exists",
				Code:    "USERNAME_EXISTS",
				Value:   user.Username,
			})
		}
	}
	
	return errors
}

// ProductAvailabilityRule は商品在庫ルール
type ProductAvailabilityRule struct {
	productRepository ProductRepository
}

// ProductRepository は商品リポジトリインターフェース
type ProductRepository interface {
	GetStock(productID string) (int, error)
	IsActive(productID string) (bool, error)
}

// Validate は商品在庫ルールを実行
func (par *ProductAvailabilityRule) Validate(data interface{}) []ValidationError {
	var errors []ValidationError
	
	if order, ok := data.(*Order); ok {
		for i, item := range order.Items {
			// 商品がアクティブかチェック
			if active, err := par.productRepository.IsActive(item.ProductID); err == nil && !active {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("items[%d].product_id", i),
					Message: "Product is not available",
					Code:    "PRODUCT_INACTIVE",
					Value:   item.ProductID,
				})
				continue
			}
			
			// 在庫チェック
			if stock, err := par.productRepository.GetStock(item.ProductID); err == nil {
				if stock < item.Quantity {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("items[%d].quantity", i),
						Message: "Insufficient stock",
						Code:    "INSUFFICIENT_STOCK",
						Value:   item.Quantity,
						Metadata: map[string]interface{}{
							"available": stock,
							"requested": item.Quantity,
						},
					})
				}
			}
		}
	}
	
	return errors
}

// セキュリティルール

// RateLimitRule はレート制限ルール
type RateLimitRule struct {
	limitChecker RateLimitChecker
}

// RateLimitChecker はレート制限チェッカーインターフェース
type RateLimitChecker interface {
	IsAllowed(clientIP string) bool
}

// Validate はレート制限ルールを実行
func (rlr *RateLimitRule) Validate(data interface{}, r *http.Request) []ValidationError {
	clientIP := getClientIP(r)
	
	if !rlr.limitChecker.IsAllowed(clientIP) {
		return []ValidationError{{
			Field:   "rate_limit",
			Message: "Rate limit exceeded",
			Code:    "RATE_LIMIT_EXCEEDED",
			Metadata: map[string]interface{}{
				"client_ip": clientIP,
			},
		}}
	}
	
	return nil
}

// SQLInjectionRule はSQLインジェクション検出ルール
type SQLInjectionRule struct{}

// Validate はSQLインジェクション検出ルールを実行
func (sir *SQLInjectionRule) Validate(data interface{}, r *http.Request) []ValidationError {
	var errors []ValidationError
	
	// 構造体の文字列フィールドをチェック
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	if val.Kind() == reflect.Struct {
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)
			fieldValue := val.Field(i)
			
			if fieldValue.Kind() == reflect.String {
				if containsSQLInjectionPattern(fieldValue.String()) {
					jsonTag := field.Tag.Get("json")
					fieldName := field.Name
					if jsonTag != "" && jsonTag != "-" {
						fieldName = strings.Split(jsonTag, ",")[0]
					}
					
					errors = append(errors, ValidationError{
						Field:   fieldName,
						Message: "Potential SQL injection detected",
						Code:    "SQL_INJECTION",
						Value:   "***",
					})
				}
			}
		}
	}
	
	return errors
}

// データ構造

// User はユーザー情報
type User struct {
	ID       string `json:"id" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=20"`
	Password string `json:"password" validate:"required,password_strength"`
	Name     string `json:"name" validate:"required,min=2,max=50"`
	Age      int    `json:"age" validate:"required,min=18,max=120"`
	Website  string `json:"website" validate:"omitempty,url"`
	Phone    string `json:"phone" validate:"omitempty,phone"`
}

// Order は注文情報
type Order struct {
	ID     string      `json:"id"`
	UserID string      `json:"user_id" validate:"required"`
	Items  []OrderItem `json:"items" validate:"required,min=1"`
	Total  float64     `json:"total" validate:"required,min=0"`
}

// OrderItem は注文アイテム
type OrderItem struct {
	ProductID string  `json:"product_id" validate:"required"`
	Quantity  int     `json:"quantity" validate:"required,min=1"`
	UnitPrice float64 `json:"unit_price" validate:"required,min=0"`
}

// ユーティリティ関数

// getStructFields はリフレクションでフィールドを取得
func getStructFields(data interface{}) []reflect.StructField {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	if val.Kind() != reflect.Struct {
		return nil
	}
	
	typ := val.Type()
	fields := make([]reflect.StructField, typ.NumField())
	
	for i := 0; i < typ.NumField(); i++ {
		fields[i] = typ.Field(i)
	}
	
	return fields
}

// parseValidationTags はバリデーションタグを解析
func parseValidationTags(tag string) []string {
	if tag == "" {
		return nil
	}
	return strings.Split(tag, ",")
}

// getClientIP はクライアントIPを取得
func getClientIP(r *http.Request) string {
	// X-Forwarded-Forヘッダーをチェック
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// X-Real-IPヘッダーをチェック
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// RemoteAddrから取得
	return strings.Split(r.RemoteAddr, ":")[0]
}

// containsSQLInjectionPattern はSQLインジェクションパターンを検出
func containsSQLInjectionPattern(input string) bool {
	patterns := []string{
		`(?i)(union\s+select)`,
		`(?i)(drop\s+table)`,
		`(?i)(delete\s+from)`,
		`(?i)(insert\s+into)`,
		`(?i)(update\s+.+set)`,
		`(?i)(exec\s*\()`,
		`(?i)(script\s*>)`,
		`(?i)('|\").*(\bor\b|\band\b).*('|\")`,
		`(?i)(--|\#|\/\*)`,
	}
	
	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, input); matched {
			return true
		}
	}
	return false
}

// isEmpty は値が空かチェック
func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.String:
		return val.String() == ""
	case reflect.Slice, reflect.Array, reflect.Map:
		return val.Len() == 0
	case reflect.Ptr:
		return val.IsNil()
	default:
		return false
	}
}

// validateMin は最小値バリデーション
func validateMin(value interface{}, min int) bool {
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.String:
		return len(val.String()) >= min
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int() >= int64(min)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return val.Uint() >= uint64(min)
	case reflect.Float32, reflect.Float64:
		return val.Float() >= float64(min)
	case reflect.Slice, reflect.Array:
		return val.Len() >= min
	default:
		return true
	}
}

// validateMax は最大値バリデーション
func validateMax(value interface{}, max int) bool {
	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.String:
		return len(val.String()) <= max
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int() <= int64(max)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return val.Uint() <= uint64(max)
	case reflect.Float32, reflect.Float64:
		return val.Float() <= float64(max)
	case reflect.Slice, reflect.Array:
		return val.Len() <= max
	default:
		return true
	}
}

// setupDefaultTranslations はデフォルトの翻訳を設定
func setupDefaultTranslations(translator *SimpleTranslator) {
	// 英語
	translator.AddTranslation("REQUIRED", "en", "Field is required")
	translator.AddTranslation("INVALID_EMAIL", "en", "Invalid email format")
	translator.AddTranslation("WEAK_PASSWORD", "en", "Password does not meet strength requirements")
	translator.AddTranslation("INVALID_URL", "en", "Invalid URL format")
	translator.AddTranslation("INVALID_PHONE", "en", "Invalid phone number format")
	translator.AddTranslation("MIN_VALUE", "en", "Minimum value is {{.min}}")
	translator.AddTranslation("MAX_VALUE", "en", "Maximum value is {{.max}}")
	translator.AddTranslation("EMAIL_EXISTS", "en", "Email address already exists")
	translator.AddTranslation("USERNAME_EXISTS", "en", "Username already exists")
	translator.AddTranslation("INSUFFICIENT_STOCK", "en", "Insufficient stock (available: {{.available}}, requested: {{.requested}})")
	translator.AddTranslation("RATE_LIMIT_EXCEEDED", "en", "Rate limit exceeded")
	translator.AddTranslation("SQL_INJECTION", "en", "Potential security threat detected")
	
	// 日本語
	translator.AddTranslation("REQUIRED", "ja", "必須項目です")
	translator.AddTranslation("INVALID_EMAIL", "ja", "メールアドレスの形式が正しくありません")
	translator.AddTranslation("WEAK_PASSWORD", "ja", "パスワードが強度要件を満たしていません")
	translator.AddTranslation("INVALID_URL", "ja", "URLの形式が正しくありません")
	translator.AddTranslation("INVALID_PHONE", "ja", "電話番号の形式が正しくありません")
	translator.AddTranslation("MIN_VALUE", "ja", "最小値は{{.min}}です")
	translator.AddTranslation("MAX_VALUE", "ja", "最大値は{{.max}}です")
	translator.AddTranslation("EMAIL_EXISTS", "ja", "このメールアドレスは既に使用されています")
	translator.AddTranslation("USERNAME_EXISTS", "ja", "このユーザー名は既に使用されています")
	translator.AddTranslation("INSUFFICIENT_STOCK", "ja", "在庫不足です（利用可能: {{.available}}, 要求: {{.requested}}）")
	translator.AddTranslation("RATE_LIMIT_EXCEEDED", "ja", "アクセス制限に達しました")
	translator.AddTranslation("SQL_INJECTION", "ja", "セキュリティ上の脅威が検出されました")
}

// generateCacheKey はキャッシュキーを生成
func generateCacheKey(data interface{}) string {
	jsonData, _ := json.Marshal(data)
	return fmt.Sprintf("%x", md5.Sum(jsonData))
}

// SimpleRateLimitChecker は簡単なレート制限チェッカー
type SimpleRateLimitChecker struct {
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	mu       sync.RWMutex
}

// NewSimpleRateLimitChecker は新しいレート制限チェッカーを作成
func NewSimpleRateLimitChecker(limit int, window time.Duration) *SimpleRateLimitChecker {
	checker := &SimpleRateLimitChecker{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	
	// 定期的にクリーンアップ
	go checker.cleanup()
	
	return checker
}

// IsAllowed はリクエストが許可されているかチェック
func (srlc *SimpleRateLimitChecker) IsAllowed(clientIP string) bool {
	srlc.mu.Lock()
	defer srlc.mu.Unlock()
	
	now := time.Now()
	windowStart := now.Add(-srlc.window)
	
	// 古いリクエストを削除
	if requests, exists := srlc.requests[clientIP]; exists {
		validRequests := make([]time.Time, 0)
		for _, reqTime := range requests {
			if reqTime.After(windowStart) {
				validRequests = append(validRequests, reqTime)
			}
		}
		srlc.requests[clientIP] = validRequests
	} else {
		srlc.requests[clientIP] = make([]time.Time, 0)
	}
	
	// 制限チェック
	if len(srlc.requests[clientIP]) >= srlc.limit {
		return false
	}
	
	// リクエストを記録
	srlc.requests[clientIP] = append(srlc.requests[clientIP], now)
	return true
}

// cleanup は古いエントリを削除
func (srlc *SimpleRateLimitChecker) cleanup() {
	ticker := time.NewTicker(srlc.window)
	defer ticker.Stop()
	
	for range ticker.C {
		srlc.mu.Lock()
		now := time.Now()
		windowStart := now.Add(-srlc.window)
		
		for ip, requests := range srlc.requests {
			validRequests := make([]time.Time, 0)
			for _, reqTime := range requests {
				if reqTime.After(windowStart) {
					validRequests = append(validRequests, reqTime)
				}
			}
			
			if len(validRequests) == 0 {
				delete(srlc.requests, ip)
			} else {
				srlc.requests[ip] = validRequests
			}
		}
		srlc.mu.Unlock()
	}
}

// MockUserRepository はテスト用のユーザーリポジトリ
type MockUserRepository struct {
	existingEmails    map[string]bool
	existingUsernames map[string]bool
}

// NewMockUserRepository は新しいモックリポジトリを作成
func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		existingEmails:    make(map[string]bool),
		existingUsernames: make(map[string]bool),
	}
}

// ExistsByEmail はメールアドレスの存在チェック
func (mur *MockUserRepository) ExistsByEmail(email string) (bool, error) {
	return mur.existingEmails[email], nil
}

// ExistsByUsername はユーザー名の存在チェック
func (mur *MockUserRepository) ExistsByUsername(username string) (bool, error) {
	return mur.existingUsernames[username], nil
}

// AddExistingEmail は既存メールアドレスを追加
func (mur *MockUserRepository) AddExistingEmail(email string) {
	mur.existingEmails[email] = true
}

// AddExistingUsername は既存ユーザー名を追加
func (mur *MockUserRepository) AddExistingUsername(username string) {
	mur.existingUsernames[username] = true
}

// メイン関数（テスト用）
func main() {
	validator := NewRequestValidator()
	
	// カスタムバリデーターを登録
	validator.RegisterValidator("email", EmailValidator)
	validator.RegisterValidator("password_strength", PasswordStrengthValidator)
	validator.RegisterValidator("url", URLValidator)
	validator.RegisterValidator("phone", PhoneValidator)
	
	// セキュリティルールを追加
	rateLimitChecker := NewSimpleRateLimitChecker(100, time.Minute)
	validator.AddSecurityRule(&RateLimitRule{limitChecker: rateLimitChecker})
	validator.AddSecurityRule(&SQLInjectionRule{})
	
	// ユーザー作成エンドポイント
	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		var user User
		lang := r.Header.Get("Accept-Language")
		if lang == "" {
			lang = "en"
		}
		
		if err := validator.ValidateRequest(w, r, &user, lang); err != nil {
			return // エラーレスポンスは ValidateRequest 内で処理
		}
		
		// バリデーション成功時の処理
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "User created successfully",
			"id":      user.ID,
		})
	})
	
	// メトリクスエンドポイント
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := validator.metrics.GetMetrics()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metrics)
	})
	
	fmt.Println("Starting server on :8080")
	fmt.Println("Try:")
	fmt.Println(`curl -X POST http://localhost:8080/users -d '{"id":"123","email":"test@example.com","username":"testuser","password":"Test123!","name":"Test User","age":25}' -H "Content-Type: application/json"`)
	fmt.Println(`curl http://localhost:8080/metrics`)
	
	http.ListenAndServe(":8080", nil)
}