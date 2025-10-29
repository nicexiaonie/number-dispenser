package dispenser

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// OptimizedSegmentDispenser 优化版号段发号器
// 通过定期checkpoint和优雅关闭，将号码浪费降到最低
type OptimizedSegmentDispenser struct {
	mu            sync.Mutex
	config        Config
	currentNumber int64 // 当前要生成的号码
	segmentEnd    int64 // 当前号段的结束位置
	segmentSize   int64 // 号段大小
	threshold     float64

	// 下一个号段
	nextSegmentMu    sync.Mutex
	nextSegmentStart int64
	nextSegmentEnd   int64
	nextSegmentReady bool

	// 持久化相关
	persistFunc      func(nextStart int64) error
	lastPersisted    int64 // 上次持久化的位置
	checkpointTicker *time.Ticker
	stopChan         chan struct{}

	// 统计信息
	totalGenerated int64 // 总共生成的号码数
	totalWasted    int64 // 总共浪费的号码数
}

// NewOptimizedSegmentDispenser 创建优化版号段发号器
// checkpointInterval: checkpoint间隔，如 5*time.Second
func NewOptimizedSegmentDispenser(
	cfg Config,
	segmentSize int64,
	threshold float64,
	checkpointInterval time.Duration,
	persistFunc func(int64) error,
) (*OptimizedSegmentDispenser, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	if segmentSize <= 0 {
		segmentSize = 100
	}

	if threshold <= 0 || threshold >= 1 {
		threshold = 0.2
	}

	osd := &OptimizedSegmentDispenser{
		config:      cfg,
		segmentSize: segmentSize,
		threshold:   threshold,
		persistFunc: persistFunc,
		stopChan:    make(chan struct{}),
	}

	// 设置默认步长
	if osd.config.Step == 0 {
		osd.config.Step = 1
	}

	// 初始化第一个号段
	start := cfg.Starting

	if err := osd.allocateSegment(start); err != nil {
		return nil, err
	}

	// 启动定期checkpoint
	if checkpointInterval > 0 {
		osd.startCheckpoint(checkpointInterval)
	}

	return osd, nil
}

// Next 生成下一个号码
func (osd *OptimizedSegmentDispenser) Next() (string, error) {
	// 只支持自增类型
	if osd.config.Type != TypeNumericIncremental {
		return "", fmt.Errorf("segment allocation only supported for incremental type")
	}

	osd.mu.Lock()
	defer osd.mu.Unlock()

	// 检查是否需要切换号段
	if osd.currentNumber >= osd.segmentEnd {
		osd.nextSegmentMu.Lock()
		if osd.nextSegmentReady {
			// 记录浪费的号码数
			wasted := osd.segmentEnd - osd.lastPersisted
			atomic.AddInt64(&osd.totalWasted, wasted)

			osd.currentNumber = osd.nextSegmentStart
			osd.segmentEnd = osd.nextSegmentEnd
			osd.nextSegmentReady = false
			osd.nextSegmentMu.Unlock()
		} else {
			osd.nextSegmentMu.Unlock()
			if err := osd.allocateSegment(osd.segmentEnd); err != nil {
				return "", err
			}
		}
	}

	// 生成号码
	num := osd.currentNumber
	osd.currentNumber += osd.config.Step
	atomic.AddInt64(&osd.totalGenerated, 1)

	// 检查是否需要预加载
	remaining := float64(osd.segmentEnd-osd.currentNumber) / float64(osd.segmentSize*osd.config.Step)
	if remaining <= osd.threshold && !osd.nextSegmentReady {
		go osd.preloadNextSegment()
	}

	// 格式化输出
	if osd.config.IncrMode == IncrModeFixed {
		return fmt.Sprintf("%0*d", osd.config.Length, num), nil
	}
	return fmt.Sprintf("%d", num), nil
}

// allocateSegment 分配新号段
func (osd *OptimizedSegmentDispenser) allocateSegment(start int64) error {
	end := start + osd.segmentSize*osd.config.Step

	// 检查边界
	if osd.config.IncrMode == IncrModeFixed {
		maxValue := pow10(osd.config.Length) - 1

		if start >= maxValue {
			return ErrNumberExhausted
		}

		if end > maxValue {
			end = maxValue + 1
		}
	}

	// 持久化号段END（用于恢复时的起点）
	if osd.persistFunc != nil {
		if err := osd.persistFunc(end); err != nil {
			return err
		}
	}

	osd.currentNumber = start
	osd.segmentEnd = end
	osd.lastPersisted = end // 记录持久化位置

	return nil
}

// preloadNextSegment 预加载下一个号段
func (osd *OptimizedSegmentDispenser) preloadNextSegment() {
	osd.nextSegmentMu.Lock()
	defer osd.nextSegmentMu.Unlock()

	if osd.nextSegmentReady {
		return
	}

	start := osd.segmentEnd
	end := start + osd.segmentSize*osd.config.Step

	if osd.persistFunc != nil {
		if err := osd.persistFunc(end); err != nil {
			return
		}
	}

	osd.nextSegmentStart = start
	osd.nextSegmentEnd = end
	osd.nextSegmentReady = true
}

// startCheckpoint 启动定期checkpoint
func (osd *OptimizedSegmentDispenser) startCheckpoint(interval time.Duration) {
	osd.checkpointTicker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-osd.checkpointTicker.C:
				osd.checkpoint()
			case <-osd.stopChan:
				return
			}
		}
	}()
}

// checkpoint 保存当前实际使用位置（而不是号段END）
// 这是减少浪费的关键
func (osd *OptimizedSegmentDispenser) checkpoint() error {
	osd.mu.Lock()
	current := osd.currentNumber
	osd.mu.Unlock()

	// 如果当前位置和上次持久化位置不同，则保存
	if current != osd.lastPersisted && osd.persistFunc != nil {
		if err := osd.persistFunc(current); err != nil {
			return err
		}
		osd.lastPersisted = current
	}

	return nil
}

// GracefulShutdown 优雅关闭（保存当前位置，而不是号段END）
// 这样可以最大限度减少浪费
func (osd *OptimizedSegmentDispenser) GracefulShutdown() error {
	// 停止checkpoint
	if osd.checkpointTicker != nil {
		osd.checkpointTicker.Stop()
	}
	close(osd.stopChan)

	// 保存当前实际位置
	osd.mu.Lock()
	current := osd.currentNumber
	lastPersisted := osd.lastPersisted
	osd.mu.Unlock()

	if osd.persistFunc != nil {
		if err := osd.persistFunc(current); err != nil {
			return err
		}
	}

	// 计算最终浪费的号码
	// 浪费 = 原本承诺分配到的位置(lastPersisted) - 实际使用的位置(current)
	// 如果优雅关闭前有checkpoint，浪费就更少
	if lastPersisted > current {
		wasted := lastPersisted - current
		atomic.AddInt64(&osd.totalWasted, wasted)
	}
	// 当前号段剩余的号码在优雅关闭后不算浪费，因为我们保存了current

	return nil
}

// GetStats 获取统计信息
func (osd *OptimizedSegmentDispenser) GetStats() DispenserStats {
	generated := atomic.LoadInt64(&osd.totalGenerated)
	wasted := atomic.LoadInt64(&osd.totalWasted)

	var wasteRate float64
	if generated > 0 {
		wasteRate = float64(wasted) / float64(generated+wasted) * 100
	}

	return DispenserStats{
		TotalGenerated: generated,
		TotalWasted:    wasted,
		WasteRate:      wasteRate,
		Strategy:       osd.config.AutoDisk,
	}
}

// GetConfig 返回配置
func (osd *OptimizedSegmentDispenser) GetConfig() Config {
	osd.mu.Lock()
	defer osd.mu.Unlock()
	return osd.config
}

// GetCurrent 返回当前位置
func (osd *OptimizedSegmentDispenser) GetCurrent() int64 {
	osd.mu.Lock()
	defer osd.mu.Unlock()
	return osd.currentNumber
}

// SetCurrent 设置当前位置（用于恢复）
func (osd *OptimizedSegmentDispenser) SetCurrent(current int64) {
	osd.mu.Lock()
	defer osd.mu.Unlock()
	osd.currentNumber = current
}

// Shutdown 优雅关闭（调用GracefulShutdown）
func (osd *OptimizedSegmentDispenser) Shutdown() error {
	return osd.GracefulShutdown()
}
