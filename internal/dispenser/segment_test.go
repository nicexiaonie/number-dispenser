package dispenser

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// 模拟持久化函数
func mockPersist(called *int64) func(int64) error {
	return func(val int64) error {
		atomic.AddInt64(called, 1)
		return nil
	}
}

func TestSegmentDispenser(t *testing.T) {
	var persistCalled int64

	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeSequence,
		Starting: 0,
		Step:     1,
	}

	sd, err := NewSegmentDispenser(cfg, 100, 0.2, mockPersist(&persistCalled))
	if err != nil {
		t.Fatalf("Failed to create segment dispenser: %v", err)
	}

	// 生成150个号码
	numbers := make([]string, 150)
	for i := 0; i < 150; i++ {
		num, err := sd.Next()
		if err != nil {
			t.Fatalf("Failed to generate number: %v", err)
		}
		numbers[i] = num
	}

	// 验证号码连续性
	for i := 0; i < 150; i++ {
		expected := i
		if numbers[i] != formatNum(int64(expected)) {
			t.Errorf("Expected %d, got %s", expected, numbers[i])
		}
	}

	// 验证持久化调用次数
	// 第一个号段: [0, 100), 持久化1次
	// 第二个号段: [100, 200), 持久化1次
	// 预加载第三个号段可能触发
	if persistCalled < 2 {
		t.Errorf("Expected at least 2 persist calls, got %d", persistCalled)
	}

	t.Logf("Generated 150 numbers with only %d disk writes", persistCalled)
	t.Logf("Performance improvement: %.1fx", 150.0/float64(persistCalled))
}

func TestSegmentConcurrency(t *testing.T) {
	var persistCalled int64

	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeSequence,
		Starting: 0,
		Step:     1,
	}

	sd, err := NewSegmentDispenser(cfg, 100, 0.2, mockPersist(&persistCalled))
	if err != nil {
		t.Fatalf("Failed to create segment dispenser: %v", err)
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
				num, err := sd.Next()
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

	// 检查唯一性
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

	t.Logf("Generated %d numbers with %d disk writes", len(seen), persistCalled)
	t.Logf("Disk write reduction: %.1fx", float64(len(seen))/float64(persistCalled))
}

func formatNum(n int64) string {
	return fmt.Sprintf("%d", n)
}

// 性能对比基准测试

func BenchmarkSegmentDispenser(b *testing.B) {
	var persistCalled int64

	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeSequence,
		Step:     1,
	}

	sd, _ := NewSegmentDispenser(cfg, 1000, 0.1, mockPersist(&persistCalled))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = sd.Next()
		}
	})

	b.ReportMetric(float64(b.N)/float64(persistCalled), "numbers/write")
}

func BenchmarkRegularDispenser(b *testing.B) {
	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeSequence,
		Step:     1,
	}

	d, _ := NewDispenser(cfg)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = d.Next()
		}
	})
}

// 模拟立即保存的基准测试
func BenchmarkDispenserWithImmediateSave(b *testing.B) {
	var persistCalled int64

	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeSequence,
		Step:     1,
	}

	d, _ := NewDispenser(cfg)
	persist := mockPersist(&persistCalled)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.Next()
		_ = persist(d.GetCurrent()) // 模拟每次都保存
	}

	b.ReportMetric(float64(b.N)/float64(persistCalled), "numbers/write")
}
