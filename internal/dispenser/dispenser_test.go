package dispenser

import (
	"sync"
	"testing"
)

func TestRandomFixed(t *testing.T) {
	cfg := Config{
		Type:   TypeRandomFixed,
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

		seen[num] = true
	}

	// Should have high uniqueness (at least 90% for 100 samples)
	if len(seen) < 90 {
		t.Errorf("Expected high uniqueness, got %d unique out of 100", len(seen))
	}
}

func TestIncrFixed(t *testing.T) {
	cfg := Config{
		Type:     TypeIncrFixed,
		Length:   8,
		Starting: 10001000,
		Step:     1,
	}

	d, err := NewDispenser(cfg)
	if err != nil {
		t.Fatalf("Failed to create dispenser: %v", err)
	}

	// Test sequential generation
	for i := 0; i < 10; i++ {
		num, err := d.Next()
		if err != nil {
			t.Fatalf("Failed to generate number: %v", err)
		}

		expected := "10001000"
		if i > 0 {
			expectedInt := 10001000 + i
			expected = formatFixed(int64(expectedInt), 8)
		}

		if i == 0 && num != expected {
			t.Errorf("Expected %s, got %s", expected, num)
		}

		if len(num) != 8 {
			t.Errorf("Expected length 8, got %d: %s", len(num), num)
		}
	}
}

func TestIncrZero(t *testing.T) {
	cfg := Config{
		Type:     TypeIncrZero,
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

func TestConcurrency(t *testing.T) {
	cfg := Config{
		Type:     TypeIncrZero,
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

func TestValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid random fixed",
			cfg: Config{
				Type:   TypeRandomFixed,
				Length: 7,
			},
			wantErr: false,
		},
		{
			name: "invalid length",
			cfg: Config{
				Type:   TypeRandomFixed,
				Length: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			cfg: Config{
				Type:   Type(99),
				Length: 7,
			},
			wantErr: true,
		},
		{
			name: "length too large",
			cfg: Config{
				Type:   TypeRandomFixed,
				Length: 20,
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

func formatFixed(num int64, length int) string {
	s := ""
	for i := 0; i < length; i++ {
		s = string(rune('0'+(num%10))) + s
		num /= 10
	}
	return s
}

func BenchmarkRandomFixed(b *testing.B) {
	cfg := Config{
		Type:   TypeRandomFixed,
		Length: 7,
	}

	d, _ := NewDispenser(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Next()
	}
}

func BenchmarkIncrFixed(b *testing.B) {
	cfg := Config{
		Type:     TypeIncrFixed,
		Length:   8,
		Starting: 10000000,
	}

	d, _ := NewDispenser(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Next()
	}
}

func BenchmarkIncrZero(b *testing.B) {
	cfg := Config{
		Type: TypeIncrZero,
	}

	d, _ := NewDispenser(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Next()
	}
}
