package main

import (
	"crypto/rand"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestSecureStringCompare(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		provided string
		want     bool
	}{
		{
			name:     "Exact match",
			expected: "secret123",
			provided: "secret123",
			want:     true,
		},
		{
			name:     "Different strings",
			expected: "secret123",
			provided: "secret456",
			want:     false,
		},
		{
			name:     "Different lengths",
			expected: "secret",
			provided: "secretlong",
			want:     false,
		},
		{
			name:     "Empty strings",
			expected: "",
			provided: "",
			want:     true,
		},
		{
			name:     "One empty",
			expected: "secret",
			provided: "",
			want:     false,
		},
		{
			name:     "First character differs",
			expected: "secret123",
			provided: "aecret123",
			want:     false,
		},
		{
			name:     "Last character differs",
			expected: "secret123",
			provided: "secret124",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SecureStringCompare(tt.expected, tt.provided)
			if got != tt.want {
				t.Errorf("SecureStringCompare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecureByteCompare(t *testing.T) {
	tests := []struct {
		name     string
		expected []byte
		provided []byte
		want     bool
	}{
		{
			name:     "Identical bytes",
			expected: []byte{1, 2, 3, 4, 5},
			provided: []byte{1, 2, 3, 4, 5},
			want:     true,
		},
		{
			name:     "Different bytes",
			expected: []byte{1, 2, 3, 4, 5},
			provided: []byte{1, 2, 3, 4, 6},
			want:     false,
		},
		{
			name:     "Different lengths",
			expected: []byte{1, 2, 3},
			provided: []byte{1, 2, 3, 4},
			want:     false,
		},
		{
			name:     "Empty arrays",
			expected: []byte{},
			provided: []byte{},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SecureByteCompare(tt.expected, tt.provided)
			if got != tt.want {
				t.Errorf("SecureByteCompare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIKeyValidator(t *testing.T) {
	keys := []string{"key1", "key2", "key3", "very-long-api-key-123456789"}
	validator := NewAPIKeyValidator(keys)

	if validator == nil {
		t.Fatal("NewAPIKeyValidator returned nil")
	}

	// Valid keys
	for _, key := range keys {
		t.Run("Valid_"+key, func(t *testing.T) {
			if !validator.ValidateKey(key) {
				t.Errorf("Valid key %s should be accepted", key)
			}
		})
	}

	// Invalid keys
	invalidKeys := []string{"invalid", "key4", "wrong-key", ""}
	for _, key := range invalidKeys {
		t.Run("Invalid_"+key, func(t *testing.T) {
			if validator.ValidateKey(key) {
				t.Errorf("Invalid key %s should be rejected", key)
			}
		})
	}

	// Test add/remove functionality
	t.Run("AddRemoveKey", func(t *testing.T) {
		newKey := "new-test-key"
		
		// Should not exist initially
		if validator.ValidateKey(newKey) {
			t.Error("New key should not exist initially")
		}

		// Add the key
		validator.AddKey(newKey)
		if !validator.ValidateKey(newKey) {
			t.Error("Added key should be valid")
		}

		// Remove the key
		validator.RemoveKey(newKey)
		if validator.ValidateKey(newKey) {
			t.Error("Removed key should not be valid")
		}
	})
}

func TestPasswordAuth(t *testing.T) {
	auth := NewPasswordAuth()
	if auth == nil {
		t.Fatal("NewPasswordAuth returned nil")
	}

	// Register users
	testUsers := map[string]string{
		"user1": "password123",
		"admin": "supersecret",
		"test":  "testpass",
	}

	for username, password := range testUsers {
		err := auth.Register(username, password)
		if err != nil {
			t.Errorf("Failed to register user %s: %v", username, err)
		}
	}

	// Test valid authentication
	for username, password := range testUsers {
		t.Run("Valid_"+username, func(t *testing.T) {
			if !auth.Authenticate(username, password) {
				t.Errorf("Valid credentials for %s should authenticate", username)
			}
		})
	}

	// Test invalid authentication
	invalidTests := []struct {
		username string
		password string
		reason   string
	}{
		{"user1", "wrongpass", "wrong password"},
		{"nonexistent", "anypass", "nonexistent user"},
		{"admin", "admin", "wrong password"},
		{"", "password123", "empty username"},
		{"user1", "", "empty password"},
	}

	for _, tt := range invalidTests {
		t.Run("Invalid_"+tt.reason, func(t *testing.T) {
			if auth.Authenticate(tt.username, tt.password) {
				t.Errorf("Invalid credentials (%s) should not authenticate", tt.reason)
			}
		})
	}
}

func TestTokenValidator(t *testing.T) {
	secretKey := []byte("test-secret-key-32-bytes-long!!")
	validator := NewTokenValidator(secretKey)

	if validator == nil {
		t.Fatal("NewTokenValidator returned nil")
	}

	// Create test tokens
	testData := []byte("test-data-for-token")
	token, err := validator.CreateToken(testData)
	if err != nil {
		t.Fatalf("Failed to create token: %v", err)
	}

	// Test valid token
	t.Run("ValidToken", func(t *testing.T) {
		valid, err := validator.ValidateToken(token, token)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !valid {
			t.Error("Valid token should validate successfully")
		}
	})

	// Test invalid tokens
	invalidTokens := []string{
		"invalid-token",
		"",
		"dGVzdA==", // "test" in base64
		token + "x", // Modified token
	}

	for i, invalidToken := range invalidTokens {
		t.Run(fmt.Sprintf("InvalidToken_%d", i), func(t *testing.T) {
			valid, err := validator.ValidateToken(token, invalidToken)
			if err != nil && invalidToken != "" && invalidToken != "invalid-token" {
				// Some invalid tokens might cause decode errors, which is acceptable
				return
			}
			if valid {
				t.Errorf("Invalid token should not validate: %s", invalidToken)
			}
		})
	}
}

func TestTimingResistantResponse(t *testing.T) {
	minDuration := 100 * time.Millisecond
	trr := NewTimingResistantResponse(minDuration)

	if trr == nil {
		t.Fatal("NewTimingResistantResponse returned nil")
	}

	// Test fast function
	t.Run("FastFunction", func(t *testing.T) {
		start := time.Now()
		result := trr.Execute(func() bool {
			time.Sleep(10 * time.Millisecond) // Fast function
			return true
		})
		duration := time.Since(start)

		if !result {
			t.Error("Function result should be preserved")
		}

		if duration < minDuration {
			t.Errorf("Duration %v should be at least %v", duration, minDuration)
		}
	})

	// Test slow function
	t.Run("SlowFunction", func(t *testing.T) {
		slowDuration := 200 * time.Millisecond
		start := time.Now()
		result := trr.Execute(func() bool {
			time.Sleep(slowDuration)
			return false
		})
		duration := time.Since(start)

		if result {
			t.Error("Function result should be preserved")
		}

		// Should not add unnecessary delay for already slow functions
		expectedMax := slowDuration + 50*time.Millisecond // Allow some overhead
		if duration > expectedMax {
			t.Errorf("Duration %v should not exceed %v for slow functions", duration, expectedMax)
		}
	})
}

func TestSecureMemory(t *testing.T) {
	size := 32
	sm := NewSecureMemory(size)

	if sm == nil {
		t.Fatal("NewSecureMemory returned nil")
	}

	// Test write and read
	testData := []byte("secure test data")
	sm.Write(testData)

	readData := sm.Read()
	if string(readData[:len(testData)]) != string(testData) {
		t.Error("Written and read data should match")
	}

	// Test wipe
	sm.Wipe()
	readDataAfterWipe := sm.Read()
	
	// Check if data is zeroed
	allZero := true
	for _, b := range readDataAfterWipe {
		if b != 0 {
			allZero = false
			break
		}
	}

	if !allZero {
		t.Error("Memory should be zeroed after wipe")
	}
}

func TestConstantTimeArraySearch(t *testing.T) {
	haystack := []string{"apple", "banana", "cherry", "date", "elderberry"}

	tests := []struct {
		needle   string
		expected int
	}{
		{"apple", 0},
		{"banana", 1},
		{"cherry", 2},
		{"date", 3},
		{"elderberry", 4},
		{"notfound", -1},
		{"", -1},
	}

	for _, tt := range tests {
		t.Run("Search_"+tt.needle, func(t *testing.T) {
			result := ConstantTimeArraySearch(haystack, tt.needle)
			if result != tt.expected {
				t.Errorf("Expected index %d, got %d for needle %s", tt.expected, result, tt.needle)
			}
		})
	}
}

func TestInsecureVsSecureComparison(t *testing.T) {
	// This test demonstrates the difference between secure and insecure comparison
	// In practice, timing differences might be too small to measure reliably in unit tests
	
	secret := "verylongsecretpassword123456789"
	testInputs := []string{
		"a" + strings.Repeat("x", len(secret)-1),           // First character differs
		secret[:len(secret)/2] + strings.Repeat("x", len(secret)/2), // Middle differs
		secret[:len(secret)-1] + "x",                       // Last character differs
		secret,                                             // Exact match
	}

	t.Run("SecureComparison", func(t *testing.T) {
		var durations []time.Duration
		
		for _, input := range testInputs {
			start := time.Now()
			SecureStringCompare(secret, input)
			duration := time.Since(start)
			durations = append(durations, duration)
		}

		// Log the durations for manual inspection
		for i, duration := range durations {
			t.Logf("Secure comparison %d: %v", i, duration)
		}
	})

	t.Run("InsecureComparison", func(t *testing.T) {
		var durations []time.Duration
		
		for _, input := range testInputs {
			start := time.Now()
			insecureCompare(secret, input)
			duration := time.Since(start)
			durations = append(durations, duration)
		}

		// Log the durations for manual inspection
		for i, duration := range durations {
			t.Logf("Insecure comparison %d: %v", i, duration)
		}
	})
}

func TestTimingResistanceStatistical(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping statistical timing test in short mode")
	}

	secret := "secretpassword123"
	iterations := 100

	// Test inputs with different failure points
	inputs := []string{
		"a" + strings.Repeat("x", len(secret)-1),     // Fails at position 0
		secret[:5] + strings.Repeat("x", len(secret)-5), // Fails at position 5
		secret[:10] + strings.Repeat("x", len(secret)-10), // Fails at position 10
		secret, // Success
	}

	results := make([][]time.Duration, len(inputs))

	for inputIdx, input := range inputs {
		results[inputIdx] = make([]time.Duration, iterations)
		
		for i := 0; i < iterations; i++ {
			start := time.Now()
			SecureStringCompare(secret, input)
			results[inputIdx][i] = time.Since(start)
		}
	}

	// Calculate statistics
	for i, durations := range results {
		mean := calculateMean(durations)
		stddev := calculateStdDev(durations, mean)
		t.Logf("Input %d: mean=%v, stddev=%v", i, mean, stddev)
	}

	// In a real implementation, you would perform statistical tests here
	// to verify that the timing differences are not statistically significant
}

func TestBenchmarkComparison(t *testing.T) {
	secret := "testsecret123"
	inputs := []string{"t", "test", "testsecr", "testsecret123"}

	durations := BenchmarkComparison(secret, inputs)
	
	if len(durations) != len(inputs) {
		t.Errorf("Expected %d durations, got %d", len(inputs), len(durations))
	}

	for i, duration := range durations {
		if duration <= 0 {
			t.Errorf("Duration %d should be positive, got %v", i, duration)
		}
		t.Logf("Input %q: %v", inputs[i], duration)
	}
}

// Helper functions for statistical analysis
func calculateMean(durations []time.Duration) time.Duration {
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func calculateStdDev(durations []time.Duration, mean time.Duration) time.Duration {
	var variance float64
	for _, d := range durations {
		diff := float64(d - mean)
		variance += diff * diff
	}
	variance /= float64(len(durations))
	return time.Duration(math.Sqrt(variance))
}

func BenchmarkSecureStringCompare(b *testing.B) {
	expected := "verylongsecretpassword123456789"
	provided := "verylongsecretpassword123456780" // Last character different

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SecureStringCompare(expected, provided)
	}
}

func BenchmarkInsecureCompare(b *testing.B) {
	expected := "verylongsecretpassword123456789"
	provided := "verylongsecretpassword123456780" // Last character different

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		insecureCompare(expected, provided)
	}
}

func BenchmarkAPIKeyValidation(b *testing.B) {
	// Create a large set of keys
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keyBytes := make([]byte, 32)
		rand.Read(keyBytes)
		keys[i] = fmt.Sprintf("key-%x", keyBytes)
	}

	validator := NewAPIKeyValidator(keys)
	testKey := keys[50] // Key in the middle

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateKey(testKey)
	}
}

func BenchmarkPasswordAuth(b *testing.B) {
	auth := NewPasswordAuth()
	auth.Register("testuser", "testpassword123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth.Authenticate("testuser", "testpassword123")
	}
}