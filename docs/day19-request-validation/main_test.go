package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestValidator_BasicValidation(t *testing.T) {
	validator := NewRequestValidator()
	validator.RegisterValidator("email", EmailValidator)
	validator.RegisterValidator("password_strength", PasswordStrengthValidator)
	
	tests := []struct {
		name        string
		user        User
		expectValid bool
		expectCode  string
	}{
		{
			name: "Valid user",
			user: User{
				ID:       "123",
				Email:    "test@example.com",
				Username: "testuser",
				Password: "Test123!",
				Name:     "Test User",
				Age:      25,
			},
			expectValid: true,
		},
		{
			name: "Missing required field",
			user: User{
				Email:    "test@example.com",
				Username: "testuser",
				Password: "Test123!",
				Name:     "Test User",
				Age:      25,
			},
			expectValid: false,
			expectCode:  "REQUIRED",
		},
		{
			name: "Invalid email",
			user: User{
				ID:       "123",
				Email:    "invalid-email",
				Username: "testuser",
				Password: "Test123!",
				Name:     "Test User",
				Age:      25,
			},
			expectValid: false,
			expectCode:  "INVALID_EMAIL",
		},
		{
			name: "Weak password",
			user: User{
				ID:       "123",
				Email:    "test@example.com",
				Username: "testuser",
				Password: "weak",
				Name:     "Test User",
				Age:      25,
			},
			expectValid: false,
			expectCode:  "WEAK_PASSWORD",
		},
		{
			name: "Age below minimum",
			user: User{
				ID:       "123",
				Email:    "test@example.com",
				Username: "testuser",
				Password: "Test123!",
				Name:     "Test User",
				Age:      16,
			},
			expectValid: false,
			expectCode:  "MIN_VALUE",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.user)
			req := httptest.NewRequest("POST", "/users", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			err := validator.ValidateRequest(w, req, &User{}, "en")
			
			if tt.expectValid {
				if err != nil {
					t.Errorf("Expected validation to pass, but got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected validation to fail")
				}
				
				if tt.expectCode != "" {
					var response map[string]interface{}
					json.NewDecoder(w.Body).Decode(&response)
					
					if details, ok := response["details"].([]interface{}); ok && len(details) > 0 {
						if detail, ok := details[0].(map[string]interface{}); ok {
							if code, ok := detail["code"].(string); ok {
								if code != tt.expectCode {
									t.Errorf("Expected error code %s, got %s", tt.expectCode, code)
								}
							}
						}
					}
				}
			}
		})
	}
	
	t.Log("Basic struct validation working correctly")
}

func TestRequestValidator_CustomValidators(t *testing.T) {
	validator := NewRequestValidator()
	validator.RegisterValidator("email", EmailValidator)
	validator.RegisterValidator("password_strength", PasswordStrengthValidator)
	validator.RegisterValidator("url", URLValidator)
	validator.RegisterValidator("phone", PhoneValidator)
	
	tests := []struct {
		name     string
		user     User
		expected bool
	}{
		{
			name: "Valid user with optional fields",
			user: User{
				ID:       "123",
				Email:    "test@example.com",
				Username: "testuser",
				Password: "Test123!",
				Name:     "Test User",
				Age:      25,
				Website:  "https://example.com",
				Phone:    "+1-234-567-8900",
			},
			expected: true,
		},
		{
			name: "Invalid URL",
			user: User{
				ID:       "123",
				Email:    "test@example.com",
				Username: "testuser",
				Password: "Test123!",
				Name:     "Test User",
				Age:      25,
				Website:  "not-a-url",
			},
			expected: false,
		},
		{
			name: "Invalid phone",
			user: User{
				ID:       "123",
				Email:    "test@example.com",
				Username: "testuser",
				Password: "Test123!",
				Name:     "Test User",
				Age:      25,
				Phone:    "invalid-phone",
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.user)
			req := httptest.NewRequest("POST", "/users", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			err := validator.ValidateRequest(w, req, &User{}, "en")
			
			if tt.expected && err != nil {
				t.Errorf("Expected validation to pass, but got error: %v", err)
			} else if !tt.expected && err == nil {
				t.Errorf("Expected validation to fail")
			}
		})
	}
	
	t.Log("Custom validation rules applied correctly")
}

func TestRequestValidator_BusinessRules(t *testing.T) {
	validator := NewRequestValidator()
	validator.RegisterValidator("email", EmailValidator)
	validator.RegisterValidator("password_strength", PasswordStrengthValidator)
	
	// モックリポジトリを作成
	mockRepo := NewMockUserRepository()
	mockRepo.AddExistingEmail("existing@example.com")
	mockRepo.AddExistingUsername("existinguser")
	
	// ビジネスルールを追加
	validator.AddBusinessRule(&UserUniquenessRule{userRepository: mockRepo})
	
	tests := []struct {
		name     string
		user     User
		expected bool
	}{
		{
			name: "New user - should pass",
			user: User{
				ID:       "123",
				Email:    "new@example.com",
				Username: "newuser",
				Password: "Test123!",
				Name:     "New User",
				Age:      25,
			},
			expected: true,
		},
		{
			name: "Existing email - should fail",
			user: User{
				ID:       "123",
				Email:    "existing@example.com",
				Username: "newuser",
				Password: "Test123!",
				Name:     "New User",
				Age:      25,
			},
			expected: false,
		},
		{
			name: "Existing username - should fail",
			user: User{
				ID:       "123",
				Email:    "new@example.com",
				Username: "existinguser",
				Password: "Test123!",
				Name:     "New User",
				Age:      25,
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.user)
			req := httptest.NewRequest("POST", "/users", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			err := validator.ValidateRequest(w, req, &User{}, "en")
			
			if tt.expected && err != nil {
				t.Errorf("Expected validation to pass, but got error: %v", err)
			} else if !tt.expected && err == nil {
				t.Errorf("Expected validation to fail")
			}
		})
	}
	
	t.Log("Business rule validation working")
}

func TestRequestValidator_Localization(t *testing.T) {
	validator := NewRequestValidator()
	validator.RegisterValidator("email", EmailValidator)
	
	invalidUser := User{
		Email:    "invalid-email",
		Username: "testuser",
		Password: "Test123!",
		Name:     "Test User",
		Age:      25,
		// ID is missing (required field)
	}
	
	tests := []struct {
		lang     string
		expected string
	}{
		{"en", "Field is required"},
		{"ja", "必須項目です"},
	}
	
	for _, tt := range tests {
		t.Run("Lang_"+tt.lang, func(t *testing.T) {
			jsonData, _ := json.Marshal(invalidUser)
			req := httptest.NewRequest("POST", "/users", bytes.NewReader(jsonData))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			validator.ValidateRequest(w, req, &User{}, tt.lang)
			
			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)
			
			if details, ok := response["details"].([]interface{}); ok && len(details) > 0 {
				if detail, ok := details[0].(map[string]interface{}); ok {
					if message, ok := detail["message"].(string); ok {
						if !strings.Contains(message, tt.expected) {
							t.Errorf("Expected message to contain '%s', got '%s'", tt.expected, message)
						}
					}
				}
			}
		})
	}
	
	t.Log("Localized error messages returned correctly")
}

func TestRequestValidator_Performance(t *testing.T) {
	validator := NewRequestValidator()
	validator.RegisterValidator("email", EmailValidator)
	validator.RegisterValidator("password_strength", PasswordStrengthValidator)
	
	validUser := User{
		ID:       "123",
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Test123!",
		Name:     "Test User",
		Age:      25,
	}
	
	iterations := 100
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		jsonData, _ := json.Marshal(validUser)
		req := httptest.NewRequest("POST", "/users", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		validator.ValidateRequest(w, req, &User{}, "en")
	}
	
	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)
	
	// 平均処理時間が10ms未満であることを確認
	if avgDuration > 10*time.Millisecond {
		t.Errorf("Validation too slow: average %v per request", avgDuration)
	}
	
	// メトリクスをチェック
	metrics := validator.metrics.GetMetrics()
	if totalValidations, ok := metrics["total_validations"].(int64); ok {
		if totalValidations != int64(iterations) {
			t.Errorf("Expected %d total validations, got %d", iterations, totalValidations)
		}
	}
	
	t.Logf("Performance test passed. Average duration: %v", avgDuration)
}

func TestEmailValidator(t *testing.T) {
	tests := []struct {
		email    string
		expected bool
	}{
		{"test@example.com", true},
		{"user.name@domain.co.uk", true},
		{"invalid-email", false},
		{"@example.com", false},
		{"test@", false},
		{"", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := EmailValidator(tt.email)
			if result != tt.expected {
				t.Errorf("EmailValidator(%s) = %v, expected %v", tt.email, result, tt.expected)
			}
		})
	}
}

func TestPasswordStrengthValidator(t *testing.T) {
	tests := []struct {
		password string
		expected bool
	}{
		{"Test123!", true},
		{"ComplexPass1$", true},
		{"weak", false},
		{"NoNumbers!", false},
		{"nouppercase1!", false},
		{"NOLOWERCASE1!", false},
		{"NoSpecialChars1", false},
		{"Short1!", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			result := PasswordStrengthValidator(tt.password)
			if result != tt.expected {
				t.Errorf("PasswordStrengthValidator(%s) = %v, expected %v", tt.password, result, tt.expected)
			}
		})
	}
}

func TestURLValidator(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://example.com", true},
		{"http://test.com/path", true},
		{"", true}, // empty is valid for optional fields
		{"not-a-url", false},
		{"ftp://invalid", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := URLValidator(tt.url)
			if result != tt.expected {
				t.Errorf("URLValidator(%s) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestPhoneValidator(t *testing.T) {
	tests := []struct {
		phone    string
		expected bool
	}{
		{"+1-234-567-8900", true},
		{"(555) 123-4567", true},
		{"", true}, // empty is valid for optional fields
		{"123", false},
		{"invalid-phone", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			result := PhoneValidator(tt.phone)
			if result != tt.expected {
				t.Errorf("PhoneValidator(%s) = %v, expected %v", tt.phone, result, tt.expected)
			}
		})
	}
}

func TestValidationCache(t *testing.T) {
	cache := NewValidationCache(100 * time.Millisecond)
	
	result := ValidationResult{
		IsValid: true,
		Errors:  nil,
	}
	
	// キャッシュに保存
	cache.Set("test-key", result)
	
	// キャッシュから取得
	cached, found := cache.Get("test-key")
	if !found {
		t.Error("Expected to find cached result")
	}
	
	if cached.IsValid != result.IsValid {
		t.Error("Cached result does not match original")
	}
	
	// TTL後にキャッシュが削除されることを確認
	time.Sleep(150 * time.Millisecond)
	
	_, found = cache.Get("test-key")
	if found {
		t.Error("Expected cached result to be expired")
	}
}

func TestValidationMetrics(t *testing.T) {
	metrics := NewValidationMetrics()
	
	// 成功を記録
	metrics.RecordSuccess(10 * time.Millisecond)
	metrics.RecordSuccess(20 * time.Millisecond)
	
	// エラーを記録
	metrics.RecordError("validation", 5*time.Millisecond)
	
	result := metrics.GetMetrics()
	
	if totalValidations, ok := result["total_validations"].(int64); ok {
		if totalValidations != 3 {
			t.Errorf("Expected 3 total validations, got %d", totalValidations)
		}
	}
	
	if successCount, ok := result["success_count"].(int64); ok {
		if successCount != 2 {
			t.Errorf("Expected 2 success count, got %d", successCount)
		}
	}
	
	if errorCount, ok := result["error_count"].(int64); ok {
		if errorCount != 1 {
			t.Errorf("Expected 1 error count, got %d", errorCount)
		}
	}
	
	if successRate, ok := result["success_rate"].(float64); ok {
		expectedRate := float64(2) / float64(3) * 100
		if successRate != expectedRate {
			t.Errorf("Expected success rate %.2f, got %.2f", expectedRate, successRate)
		}
	}
}

func TestSimpleTranslator(t *testing.T) {
	translator := NewSimpleTranslator("en")
	
	// 翻訳を追加
	translator.AddTranslation("REQUIRED", "en", "Field is required")
	translator.AddTranslation("REQUIRED", "ja", "必須項目です")
	translator.AddTranslation("MIN_VALUE", "en", "Minimum value is {{.min}}")
	
	tests := []struct {
		code     string
		lang     string
		params   map[string]interface{}
		expected string
	}{
		{"REQUIRED", "en", nil, "Field is required"},
		{"REQUIRED", "ja", nil, "必須項目です"},
		{"REQUIRED", "fr", nil, "Field is required"}, // fallback to default
		{"MIN_VALUE", "en", map[string]interface{}{"min": 10}, "Minimum value is 10"},
		{"UNKNOWN", "en", nil, "UNKNOWN"}, // return code if not found
	}
	
	for _, tt := range tests {
		t.Run(tt.code+"_"+tt.lang, func(t *testing.T) {
			result := translator.Translate(tt.code, tt.lang, tt.params)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSQLInjectionRule(t *testing.T) {
	rule := &SQLInjectionRule{}
	
	tests := []struct {
		name     string
		user     User
		expected bool
	}{
		{
			name: "Clean input",
			user: User{
				Name:     "John Doe",
				Username: "johndoe",
			},
			expected: true, // no SQL injection detected
		},
		{
			name: "SQL injection attempt",
			user: User{
				Name:     "John'; DROP TABLE users; --",
				Username: "johndoe",
			},
			expected: false, // SQL injection detected
		},
		{
			name: "Union select attempt",
			user: User{
				Name:     "John UNION SELECT * FROM passwords",
				Username: "johndoe",
			},
			expected: false, // SQL injection detected
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", nil)
			errors := rule.Validate(&tt.user, req)
			
			if tt.expected && len(errors) > 0 {
				t.Errorf("Expected no SQL injection detection, but got errors: %v", errors)
			} else if !tt.expected && len(errors) == 0 {
				t.Errorf("Expected SQL injection detection, but got no errors")
			}
		})
	}
}

func TestRateLimitRule(t *testing.T) {
	checker := NewSimpleRateLimitChecker(2, time.Minute)
	rule := &RateLimitRule{limitChecker: checker}
	
	// 最初の2つのリクエストは成功するはず
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		
		errors := rule.Validate(&User{}, req)
		if len(errors) > 0 {
			t.Errorf("Request %d should not be rate limited", i+1)
		}
	}
	
	// 3つ目のリクエストは制限されるはず
	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	
	errors := rule.Validate(&User{}, req)
	if len(errors) == 0 {
		t.Error("Third request should be rate limited")
	}
	
	// 異なるIPからのリクエストは成功するはず
	req = httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "192.168.1.2:12345"
	
	errors = rule.Validate(&User{}, req)
	if len(errors) > 0 {
		t.Error("Request from different IP should not be rate limited")
	}
}

func TestContainsSQLInjectionPattern(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"normal text", false},
		{"SELECT * FROM users", false}, // normal SQL not considered injection
		{"'; DROP TABLE users; --", true},
		{"UNION SELECT password FROM users", true},
		{"DELETE FROM users WHERE 1=1", true},
		{"INSERT INTO users VALUES ('hacker')", true},
		{"UPDATE users SET password='hacked'", true},
		{"<script>alert('xss')</script>", true},
		{"test' OR '1'='1", true},
		{"normal -- comment", true},
		{"normal /* comment */", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsSQLInjectionPattern(tt.input)
			if result != tt.expected {
				t.Errorf("containsSQLInjectionPattern(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		setupReq   func() *http.Request
		expectedIP string
	}{
		{
			name: "X-Forwarded-For header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
				req.RemoteAddr = "127.0.0.1:12345"
				return req
			},
			expectedIP: "192.168.1.1",
		},
		{
			name: "X-Real-IP header",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Real-IP", "192.168.1.2")
				req.RemoteAddr = "127.0.0.1:12345"
				return req
			},
			expectedIP: "192.168.1.2",
		},
		{
			name: "RemoteAddr fallback",
			setupReq: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.3:12345"
				return req
			},
			expectedIP: "192.168.1.3",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			ip := getClientIP(req)
			if ip != tt.expectedIP {
				t.Errorf("Expected IP %s, got %s", tt.expectedIP, ip)
			}
		})
	}
}

// ベンチマークテスト
func BenchmarkRequestValidator_StructValidation(b *testing.B) {
	validator := NewRequestValidator()
	validator.RegisterValidator("email", EmailValidator)
	validator.RegisterValidator("password_strength", PasswordStrengthValidator)
	
	user := User{
		ID:       "123",
		Email:    "test@example.com",
		Username: "testuser",
		Password: "Test123!",
		Name:     "Test User",
		Age:      25,
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			validator.validateStruct(&user)
		}
	})
}

func BenchmarkEmailValidator(b *testing.B) {
	emails := []string{
		"test@example.com",
		"user.name@domain.co.uk",
		"invalid-email",
		"another@test.org",
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			EmailValidator(emails[i%len(emails)])
			i++
		}
	})
}

func BenchmarkPasswordStrengthValidator(b *testing.B) {
	passwords := []string{
		"Test123!",
		"ComplexPass1$",
		"weak",
		"AnotherStrong2@",
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			PasswordStrengthValidator(passwords[i%len(passwords)])
			i++
		}
	})
}

func BenchmarkSQLInjectionDetection(b *testing.B) {
	inputs := []string{
		"normal text",
		"'; DROP TABLE users; --",
		"UNION SELECT password FROM users",
		"another normal input",
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			containsSQLInjectionPattern(inputs[i%len(inputs)])
			i++
		}
	})
}