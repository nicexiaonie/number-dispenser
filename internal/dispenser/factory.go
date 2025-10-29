package dispenser

import (
	"fmt"
	"time"
)

// DispenserFactory 发号器工厂
type DispenserFactory struct {
	persistFunc func(string, Config, int64) error
}

// NewDispenserFactory 创建发号器工厂
func NewDispenserFactory(persistFunc func(string, Config, int64) error) *DispenserFactory {
	return &DispenserFactory{
		persistFunc: persistFunc,
	}
}

// CreateDispenser 根据配置创建发号器
func (f *DispenserFactory) CreateDispenser(name string, cfg Config) (NumberDispenser, error) {
	// 如果没有指定策略，默认使用 elegant_close
	if cfg.AutoDisk == "" {
		cfg.AutoDisk = StrategyElegantClose
	}

	// 验证策略是否有效
	if !ValidPersistenceStrategies[cfg.AutoDisk] {
		return nil, fmt.Errorf("invalid persistence strategy: %s", cfg.AutoDisk)
	}

	switch cfg.AutoDisk {
	case StrategyMemory:
		return f.createMemoryDispenser(cfg)

	case StrategyPreBase:
		return f.createPreBaseDispenser(name, cfg)

	case StrategyPreCheckpoint:
		return f.createPreCheckpointDispenser(name, cfg)

	case StrategyElegantClose:
		return f.createElegantCloseDispenser(cfg)

	case StrategyPreClose:
		return f.createPreCloseDispenser(name, cfg)

	default:
		return nil, fmt.Errorf("unknown persistence strategy: %s", cfg.AutoDisk)
	}
}

// createMemoryDispenser 创建内存模式发号器（不持久化）
func (f *DispenserFactory) createMemoryDispenser(cfg Config) (NumberDispenser, error) {
	return NewDispenser(cfg)
}

// createPreBaseDispenser 创建预分配基础版发号器
func (f *DispenserFactory) createPreBaseDispenser(name string, cfg Config) (NumberDispenser, error) {
	segmentSize := int64(1000) // 默认号段大小

	persistFunc := func(val int64) error {
		if f.persistFunc != nil {
			return f.persistFunc(name, cfg, val)
		}
		return nil
	}

	return NewSegmentDispenser(cfg, segmentSize, 0.1, persistFunc)
}

// createPreCheckpointDispenser 创建预分配+检查点发号器
func (f *DispenserFactory) createPreCheckpointDispenser(name string, cfg Config) (NumberDispenser, error) {
	segmentSize := int64(1000)
	checkpointInterval := 2 * time.Second // 2秒检查点

	persistFunc := func(val int64) error {
		if f.persistFunc != nil {
			return f.persistFunc(name, cfg, val)
		}
		return nil
	}

	return NewOptimizedSegmentDispenser(cfg, segmentSize, 0.1, checkpointInterval, persistFunc)
}

// createElegantCloseDispenser 创建优雅关闭模式发号器（立即保存）
func (f *DispenserFactory) createElegantCloseDispenser(cfg Config) (NumberDispenser, error) {
	// 这个就是基础的 Dispenser，配合外部的立即保存逻辑
	return NewDispenser(cfg)
}

// createPreCloseDispenser 创建预分配+检查点+优雅关闭发号器（最优）
func (f *DispenserFactory) createPreCloseDispenser(name string, cfg Config) (NumberDispenser, error) {
	segmentSize := int64(1000)
	checkpointInterval := 2 * time.Second

	persistFunc := func(val int64) error {
		if f.persistFunc != nil {
			return f.persistFunc(name, cfg, val)
		}
		return nil
	}

	return NewOptimizedSegmentDispenser(cfg, segmentSize, 0.1, checkpointInterval, persistFunc)
}
