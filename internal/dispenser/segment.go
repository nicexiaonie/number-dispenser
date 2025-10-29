package dispenser

import (
	"fmt"
	"sync"
)

// SegmentDispenser 使用号段预分配机制的发号器
// 解决了立即保存的性能问题，同时保证不重复
type SegmentDispenser struct {
	mu            sync.Mutex
	config        Config
	currentNumber int64   // 当前要生成的号码
	segmentEnd    int64   // 当前号段的结束位置（不包含）
	segmentSize   int64   // 号段大小
	threshold     float64 // 剩余比例阈值，触发预加载

	// 下一个号段（异步预加载）
	nextSegmentMu    sync.Mutex
	nextSegmentStart int64
	nextSegmentEnd   int64
	nextSegmentReady bool

	// 持久化回调
	persistFunc func(nextStart int64) error
}

// NewSegmentDispenser 创建基于号段的发号器
// segmentSize: 每个号段的大小，如 100 表示一次预分配 100 个号码
// threshold: 剩余比例阈值，如 0.2 表示剩余 20% 时开始预加载下一段
func NewSegmentDispenser(cfg Config, segmentSize int64, threshold float64, persistFunc func(int64) error) (*SegmentDispenser, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	if segmentSize <= 0 {
		segmentSize = 100 // 默认号段大小
	}

	if threshold <= 0 || threshold >= 1 {
		threshold = 0.2 // 默认剩余20%时预加载
	}

	sd := &SegmentDispenser{
		config:      cfg,
		segmentSize: segmentSize,
		threshold:   threshold,
		persistFunc: persistFunc,
	}

	// 设置默认步长
	if sd.config.Step == 0 {
		sd.config.Step = 1
	}

	// 初始化第一个号段
	start := cfg.Starting
	if cfg.Type == TypeIncrZero && start == 0 {
		start = 0
	}

	if err := sd.allocateSegment(start); err != nil {
		return nil, err
	}

	return sd, nil
}

// Next 生成下一个号码（高性能版本）
func (sd *SegmentDispenser) Next() (string, error) {
	// 随机类型不需要号段机制，直接生成
	if sd.config.Type == TypeRandomFixed {
		return sd.nextRandom()
	}

	sd.mu.Lock()
	defer sd.mu.Unlock()

	// 检查是否需要切换到下一个号段
	if sd.currentNumber >= sd.segmentEnd {
		// 当前号段用尽，切换到预加载的下一段
		sd.nextSegmentMu.Lock()
		if sd.nextSegmentReady {
			sd.currentNumber = sd.nextSegmentStart
			sd.segmentEnd = sd.nextSegmentEnd
			sd.nextSegmentReady = false
			sd.nextSegmentMu.Unlock()
		} else {
			// 下一段还没准备好（异常情况），同步分配
			sd.nextSegmentMu.Unlock()
			if err := sd.allocateSegment(sd.segmentEnd); err != nil {
				return "", err
			}
		}
	}

	// 在号段内生成号码（无磁盘IO，极快）
	num := sd.currentNumber
	sd.currentNumber += sd.config.Step

	// 检查是否需要预加载下一个号段
	remaining := float64(sd.segmentEnd-sd.currentNumber) / float64(sd.segmentSize*sd.config.Step)
	if remaining <= sd.threshold && !sd.nextSegmentReady {
		// 异步预加载下一个号段
		go sd.preloadNextSegment()
	}

	// 格式化输出
	switch sd.config.Type {
	case TypeIncrFixed:
		return fmt.Sprintf("%0*d", sd.config.Length, num), nil
	case TypeIncrZero:
		return fmt.Sprintf("%d", num), nil
	default:
		return "", ErrInvalidType
	}
}

// allocateSegment 分配一个新号段（会写磁盘）
func (sd *SegmentDispenser) allocateSegment(start int64) error {
	end := start + sd.segmentSize*sd.config.Step

	// 检查固定位数类型的边界
	if sd.config.Type == TypeIncrFixed {
		maxValue := int64(1)
		for i := 0; i < sd.config.Length; i++ {
			maxValue *= 10
		}
		maxValue--

		if start >= maxValue {
			return ErrNumberExhausted
		}

		if end > maxValue {
			end = maxValue + 1
		}
	}

	// 持久化号段结束位置
	// 关键：保存的是号段的END，而不是START
	// 这样即使重启，也会从END开始分配新号段，不会重复
	if sd.persistFunc != nil {
		if err := sd.persistFunc(end); err != nil {
			return err
		}
	}

	sd.currentNumber = start
	sd.segmentEnd = end

	return nil
}

// preloadNextSegment 异步预加载下一个号段
func (sd *SegmentDispenser) preloadNextSegment() {
	sd.nextSegmentMu.Lock()
	defer sd.nextSegmentMu.Unlock()

	if sd.nextSegmentReady {
		return // 已经预加载过了
	}

	// 计算下一个号段
	start := sd.segmentEnd
	end := start + sd.segmentSize*sd.config.Step

	// 持久化
	if sd.persistFunc != nil {
		if err := sd.persistFunc(end); err != nil {
			// 预加载失败，下次会同步分配
			return
		}
	}

	sd.nextSegmentStart = start
	sd.nextSegmentEnd = end
	sd.nextSegmentReady = true
}

// nextRandom 生成随机数（随机类型不需要号段）
func (sd *SegmentDispenser) nextRandom() (string, error) {
	min := int64(1)
	for i := 1; i < sd.config.Length; i++ {
		min *= 10
	}
	// max := min*10 - 1

	// 这里需要一个随机数生成器，简化实现
	// 实际应该像原来的 Dispenser 一样使用 rand.Rand
	// 号段模式不推荐用于随机类型
	return fmt.Sprintf("%0*d", sd.config.Length, min), nil
}

// GetConfig 返回配置
func (sd *SegmentDispenser) GetConfig() Config {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	return sd.config
}

// GetCurrent 返回当前号段结束位置（用于持久化）
func (sd *SegmentDispenser) GetCurrent() int64 {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	return sd.segmentEnd // 返回号段END，不是当前号码
}

// GetSegmentInfo 返回号段信息（用于监控）
func (sd *SegmentDispenser) GetSegmentInfo() (current, end int64, usage float64) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	used := sd.currentNumber - (sd.segmentEnd - sd.segmentSize*sd.config.Step)
	total := sd.segmentSize * sd.config.Step
	usage = float64(used) / float64(total)

	return sd.currentNumber, sd.segmentEnd, usage
}

// SetCurrent 设置当前位置（用于恢复）
func (sd *SegmentDispenser) SetCurrent(current int64) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.currentNumber = current
}

// Shutdown 关闭发号器（基础版无需特殊处理）
func (sd *SegmentDispenser) Shutdown() error {
	return nil
}

// GetStats 获取统计信息
func (sd *SegmentDispenser) GetStats() DispenserStats {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// 预分配基础版可能有浪费
	wasted := sd.segmentEnd - sd.currentNumber
	generated := sd.currentNumber - sd.config.Starting
	totalNumbers := generated + wasted

	var wasteRate float64
	if totalNumbers > 0 {
		wasteRate = float64(wasted) / float64(totalNumbers) * 100
	}

	return DispenserStats{
		TotalGenerated: generated,
		TotalWasted:    wasted,
		WasteRate:      wasteRate,
		Strategy:       sd.config.AutoDisk,
	}
}
