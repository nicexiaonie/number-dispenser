package dispenser

import (
	"strings"
	"sync"
	"testing"
)

// ============================================
// Type 1: 纯数字随机测试
// ============================================

func TestType1_NumericRandom(t *testing.T) {
	cfg := Config{
		Type:   TypeNumericRandom,
		Length: 7,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	// Generate multiple numbers and check uniqueness
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		num, err := d.Next()
		if err != nil {
			t.Fatalf("Failed to generate number: %v", err)
		}

		if len(num) != 7 {
			t.Errorf("Expected length 7, got %d: %s", len(num), num)
		}

		// 检查是否为纯数字
		for _, c := range num {
			if c < '0' || c > '9' {
				t.Errorf("Expected numeric only, got: %s", num)
			}
		}

		seen[num] = true
	}

	// Should be 100% unique with dedup cache
	if len(seen) != 100 {
		t.Errorf("Expected 100 unique numbers, got %d", len(seen))
	}
}

// ============================================
// Type 2: 纯数字自增测试
// ============================================

func TestType2_NumericIncrementalFixed(t *testing.T) {
	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeFixed,
		Length:   8,
		Starting: 10001000,
		Step:     1,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	// Test sequential generation
	expected := []string{"10001000", "10001001", "10001002", "10001003", "10001004"}
	for i, exp := range expected {
		num, err := d.Next()
		if err != nil {
			t.Fatalf("Failed to generate number: %v", err)
		}

		if num != exp {
			t.Errorf("Iteration %d: expected %s, got %s", i, exp, num)
		}

		if len(num) != 8 {
			t.Errorf("Expected length 8, got %d: %s", len(num), num)
		}
	}
}

func TestType2_NumericIncrementalSequence(t *testing.T) {
	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeSequence,
		Starting: 5,
		Step:     3,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	expected := []string{"5", "8", "11", "14", "17"}
	for i, exp := range expected {
		num, err := d.Next()
		if err != nil {
			t.Fatalf("Failed to generate number: %v", err)
		}

		if num != exp {
			t.Errorf("Iteration %d: expected %s, got %s", i, exp, num)
		}
	}
}

// ============================================
// Type 3: 字符随机测试
// ============================================

func TestType3_AlphanumericRandomHex(t *testing.T) {
	cfg := Config{
		Type:    TypeAlphanumericRandom,
		Charset: CharsetHex,
		Length:  16,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	for i := 0; i < 10; i++ {
		num, err := d.Next()
		if err != nil {
			t.Fatalf("Failed to generate number: %v", err)
		}

		if len(num) != 16 {
			t.Errorf("Expected length 16, got %d: %s", len(num), num)
		}

		// 检查是否为十六进制（0-9, a-f）
		for _, c := range num {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("Expected hex chars, got: %s", num)
			}
		}
	}
}

func TestType3_AlphanumericRandomBase62(t *testing.T) {
	cfg := Config{
		Type:    TypeAlphanumericRandom,
		Charset: CharsetBase62,
		Length:  12,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	for i := 0; i < 10; i++ {
		num, err := d.Next()
		if err != nil {
			t.Fatalf("Failed to generate number: %v", err)
		}

		if len(num) != 12 {
			t.Errorf("Expected length 12, got %d: %s", len(num), num)
		}

		// 检查是否为base62（0-9, a-z, A-Z）
		for _, c := range num {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				t.Errorf("Expected base62 chars, got: %s", num)
			}
		}
	}
}

// ============================================
// Type 4: Snowflake测试
// ============================================

func TestType4_Snowflake(t *testing.T) {
	cfg := Config{
		Type:         TypeSnowflake,
		MachineID:    1,
		DatacenterID: 0,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		num, err := d.Next()
		if err != nil {
			t.Fatalf("Failed to generate number: %v", err)
		}

		// Snowflake ID应该是纯数字
		for _, c := range num {
			if c < '0' || c > '9' {
				t.Errorf("Expected numeric only, got: %s", num)
			}
		}

		// 检查唯一性
		if seen[num] {
			t.Errorf("Duplicate snowflake ID: %s", num)
		}
		seen[num] = true
	}
}

// ============================================
// Type 5: UUID测试
// ============================================

func TestType5_UUIDStandard(t *testing.T) {
	cfg := Config{
		Type:       TypeUUID,
		UUIDFormat: UUIDFormatStandard,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	for i := 0; i < 10; i++ {
		num, err := d.Next()
		if err != nil {
			t.Fatalf("Failed to generate UUID: %v", err)
		}

		// 标准UUID格式：xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
		parts := strings.Split(num, "-")
		if len(parts) != 5 {
			t.Errorf("Expected 5 parts, got %d: %s", len(parts), num)
		}

		if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
			t.Errorf("Invalid UUID format: %s", num)
		}
	}
}

func TestType5_UUIDCompact(t *testing.T) {
	cfg := Config{
		Type:       TypeUUID,
		UUIDFormat: UUIDFormatCompact,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	for i := 0; i < 10; i++ {
		num, err := d.Next()
		if err != nil {
			t.Fatalf("Failed to generate UUID: %v", err)
		}

		// 紧凑UUID格式：32个十六进制字符
		if len(num) != 32 {
			t.Errorf("Expected length 32, got %d: %s", len(num), num)
		}

		// 检查是否为十六进制
		for _, c := range num {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("Expected hex chars, got: %s", num)
			}
		}
	}
}

// ============================================
// 并发测试
// ============================================

func TestConcurrency(t *testing.T) {
	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeSequence,
		Starting: 0,
		Step:     1,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	const goroutines = 10
	const numbersPerGoroutine = 100

	var wg sync.WaitGroup
	results := make(chan string, goroutines*numbersPerGoroutine)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numbersPerGoroutine; j++ {
				num, err := d.Next()
				if err != nil {
					t.Errorf("Failed to generate number: %v", err)
					return
				}
				results <- num
			}
		}()
	}

	wg.Wait()
	close(results)

	// Check uniqueness
	seen := make(map[string]bool)
	for num := range results {
		if seen[num] {
			t.Errorf("Duplicate number generated: %s", num)
		}
		seen[num] = true
	}

	if len(seen) != goroutines*numbersPerGoroutine {
		t.Errorf("Expected %d unique numbers, got %d", goroutines*numbersPerGoroutine, len(seen))
	}
}

// ============================================
// 配置验证测试
// ============================================

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid type 1",
			cfg: Config{
				Type:   TypeNumericRandom,
				Length: 7,
			},
			wantErr: false,
		},
		{
			name: "type 1 invalid length",
			cfg: Config{
				Type:   TypeNumericRandom,
				Length: 0,
			},
			wantErr: true,
		},
		{
			name: "type 1 length too large",
			cfg: Config{
				Type:   TypeNumericRandom,
				Length: 20,
			},
			wantErr: true,
		},
		{
			name: "valid type 2 fixed",
			cfg: Config{
				Type:     TypeNumericIncremental,
				IncrMode: IncrModeFixed,
				Length:   8,
				Starting: 10000000,
			},
			wantErr: false,
		},
		{
			name: "type 2 starting exceeds length",
			cfg: Config{
				Type:     TypeNumericIncremental,
				IncrMode: IncrModeFixed,
				Length:   8,
				Starting: 100000000,
			},
			wantErr: true,
		},
		{
			name: "valid type 3 hex",
			cfg: Config{
				Type:    TypeAlphanumericRandom,
				Charset: CharsetHex,
				Length:  16,
			},
			wantErr: false,
		},
		{
			name: "type 3 invalid charset",
			cfg: Config{
				Type:    TypeAlphanumericRandom,
				Charset: "invalid",
				Length:  16,
			},
			wantErr: true,
		},
		{
			name: "valid type 4 snowflake",
			cfg: Config{
				Type:      TypeSnowflake,
				MachineID: 1,
			},
			wantErr: false,
		},
		{
			name: "type 4 invalid machine_id",
			cfg: Config{
				Type:      TypeSnowflake,
				MachineID: 32,
			},
			wantErr: true,
		},
		{
			name: "valid type 5 uuid",
			cfg: Config{
				Type:       TypeUUID,
				UUIDFormat: UUIDFormatStandard,
			},
			wantErr: false,
		},
		{
			name: "invalid type",
			cfg: Config{
				Type:   Type(99),
				Length: 7,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDispenser(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDispenser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ============================================
// 基准测试
// ============================================

func BenchmarkType1_NumericRandom(b *testing.B) {
	cfg := Config{
		Type:   TypeNumericRandom,
		Length: 7,
	}

	d, _ := NewDispenser(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Next()
	}
}

func BenchmarkType2_NumericIncremental(b *testing.B) {
	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeFixed,
		Length:   8,
		Starting: 10000000,
	}

	d, _ := NewDispenser(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Next()
	}
}

func BenchmarkType3_AlphanumericHex(b *testing.B) {
	cfg := Config{
		Type:    TypeAlphanumericRandom,
		Charset: CharsetHex,
		Length:  16,
	}

	d, _ := NewDispenser(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Next()
	}
}

func BenchmarkType4_Snowflake(b *testing.B) {
	cfg := Config{
		Type:      TypeSnowflake,
		MachineID: 1,
	}

	d, _ := NewDispenser(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Next()
	}
}

func BenchmarkType5_UUID(b *testing.B) {
	cfg := Config{
		Type:       TypeUUID,
		UUIDFormat: UUIDFormatCompact,
	}

	d, _ := NewDispenser(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Next()
	}
}
