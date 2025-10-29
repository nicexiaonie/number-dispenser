package dispenser

import (
	"sync/atomic"
	"testing"
	"time"
)

// 测试优化版号段发号器 - 展示浪费率降低
func TestOptimizedSegmentDispenser_MinimalWaste(t *testing.T) {
	var persistCalled int64
	var lastPersisted int64

	cfg := Config{
		Type:     TypeNumericIncremental,
		IncrMode: IncrModeSequence,
		Starting: 0,
		Step:     1,
	}

	persistFunc := func(val int64) error {
		atomic.AddInt64(&persistCalled, 1)
		atomic.StoreInt64(&lastPersisted, val)
		return nil
	}

	// 创建优化版发号器
	// checkpoint间隔2秒
	osd, err := NewOptimizedSegmentDispenser(
		cfg,
		100,           // 号段大小100
		0.2,           // 20%时预加载
		2*time.Second, // 每2秒checkpoint
		persistFunc,
	)
	if err != nil {
		t.Fatalf("Failed to create optimized dispenser: %v", err)
	}

	// 生成50个号码
	for i := 0; i < 50; i++ {
		_, err := osd.Next()
		if err != nil {
			t.Fatalf("Failed to generate number: %v", err)
		}
	}

	// 等待checkpoint
	time.Sleep(3 * time.Second)

	// 模拟优雅关闭
	if err := osd.GracefulShutdown(); err != nil {
		t.Fatalf("Failed to shutdown: %v", err)
	}

	// 获取统计信息
	stats := osd.GetStats()

	t.Logf("Generated: %d numbers", stats.TotalGenerated)
	t.Logf("Wasted: %d numbers", stats.TotalWasted)
	t.Logf("Waste rate: %.2f%%", stats.WasteRate)
	t.Logf("Last persisted position: %d", atomic.LoadInt64(&lastPersisted))

	// 优雅关闭后，浪费应该接近0
	// 因为我们保存了实际使用位置（50）而不是号段END（100）
	if stats.TotalWasted > 5 {
		t.Errorf("Expected minimal waste, got %d", stats.TotalWasted)
	}

	if stats.WasteRate > 10.0 {
		t.Errorf("Expected waste rate < 10%%, got %.2f%%", stats.WasteRate)
	}
}

// 对比测试：基础版 vs 优化版
func TestWasteComparison(t *testing.T) {
	tests := []struct {
		name           string
		useOptimized   bool
		withCheckpoint bool
		withShutdown   bool
		expectedWaste  string
	}{
		{
			name:           "基础版-突然重启",
			useOptimized:   false,
			withCheckpoint: false,
			withShutdown:   false,
			expectedWaste:  "50个 (50%)",
		},
		{
			name:           "优化版-无checkpoint-突然重启",
			useOptimized:   true,
			withCheckpoint: false,
			withShutdown:   false,
			expectedWaste:  "50个 (50%)",
		},
		{
			name:           "优化版-有checkpoint-突然重启",
			useOptimized:   true,
			withCheckpoint: true,
			withShutdown:   false,
			expectedWaste:  "~2个 (2%)",
		},
		{
			name:           "优化版-优雅关闭",
			useOptimized:   true,
			withCheckpoint: false,
			withShutdown:   true,
			expectedWaste:  "0个 (0%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lastPersisted int64

			persistFunc := func(val int64) error {
				atomic.StoreInt64(&lastPersisted, val)
				return nil
			}

			if tt.useOptimized {
				var checkpointInterval time.Duration
				if tt.withCheckpoint {
					checkpointInterval = 500 * time.Millisecond
				}

				osd, _ := NewOptimizedSegmentDispenser(
					Config{Type: TypeNumericIncremental, IncrMode: IncrModeSequence, Step: 1},
					100,
					0.2,
					checkpointInterval,
					persistFunc,
				)

				// 生成50个号码
				for i := 0; i < 50; i++ {
					osd.Next()
				}

				if tt.withCheckpoint {
					time.Sleep(1 * time.Second) // 等待checkpoint
				}

				if tt.withShutdown {
					osd.GracefulShutdown()
				}

				stats := osd.GetStats()
				t.Logf("%s: Wasted=%d, Rate=%.2f%%, LastPersisted=%d",
					tt.name, stats.TotalWasted, stats.WasteRate, atomic.LoadInt64(&lastPersisted))
			} else {
				// 基础版（为了对比）
				sd, _ := NewSegmentDispenser(
					Config{Type: TypeNumericIncremental, IncrMode: IncrModeSequence, Step: 1},
					100,
					0.2,
					persistFunc,
				)

				for i := 0; i < 50; i++ {
					sd.Next()
				}

				// 基础版没有优雅关闭，模拟突然重启
				// lastPersisted = 100（号段END）
				// 实际使用到 50
				// 浪费 = 100 - 50 = 50

				t.Logf("%s: 预期浪费=%s, LastPersisted=%d",
					tt.name, tt.expectedWaste, atomic.LoadInt64(&lastPersisted))
			}
		})
	}
}

// 性能测试：优化版不应该明显降低性能
func BenchmarkOptimizedSegmentDispenser(b *testing.B) {
	var persistCalled int64

	persistFunc := func(val int64) error {
		atomic.AddInt64(&persistCalled, 1)
		return nil
	}

	osd, _ := NewOptimizedSegmentDispenser(
		Config{Type: TypeNumericIncremental, IncrMode: IncrModeSequence, Step: 1},
		1000,
		0.1,
		5*time.Second, // checkpoint间隔较长，不影响性能
		persistFunc,
	)
	defer osd.GracefulShutdown()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = osd.Next()
		}
	})

	b.StopTimer()
	stats := osd.GetStats()
	b.ReportMetric(float64(b.N)/float64(persistCalled), "numbers/write")
	b.ReportMetric(stats.WasteRate, "waste%")
	b.Logf("Generated: %d, Wasted: %d, Rate: %.4f%%", stats.TotalGenerated, stats.TotalWasted, stats.WasteRate)
}

// 对比基准测试：基础版 vs 优化版
func BenchmarkComparison(b *testing.B) {
	persistFunc := func(val int64) error { return nil }

	b.Run("Basic", func(b *testing.B) {
		sd, _ := NewSegmentDispenser(
			Config{Type: TypeNumericIncremental, IncrMode: IncrModeSequence, Step: 1},
			1000,
			0.1,
			persistFunc,
		)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sd.Next()
		}
	})

	b.Run("Optimized", func(b *testing.B) {
		osd, _ := NewOptimizedSegmentDispenser(
			Config{Type: TypeNumericIncremental, IncrMode: IncrModeSequence, Step: 1},
			1000,
			0.1,
			10*time.Second, // checkpoint间隔长，不影响性能
			persistFunc,
		)
		defer osd.GracefulShutdown()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			osd.Next()
		}
	})
}
