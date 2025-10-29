package dispenser

// PersistenceStrategy 持久化策略类型
type PersistenceStrategy string

const (
	// StrategyMemory 内存模式 - 不持久化，重启后从头开始
	StrategyMemory PersistenceStrategy = "memory"

	// StrategyPreBase 预分配基础版 - 号段预分配，但可能浪费50%
	StrategyPreBase PersistenceStrategy = "pre-base"

	// StrategyPreCheckpoint 预分配+检查点 - 2秒检查点，浪费<5%
	StrategyPreCheckpoint PersistenceStrategy = "pre-checkpoint"

	// StrategyElegantClose 优雅关闭 - 立即保存+优雅关闭，正常关闭0浪费
	StrategyElegantClose PersistenceStrategy = "elegant_close"

	// StrategyPreClose 预分配+检查点+优雅关闭 - 最优方案，浪费<0.1%
	StrategyPreClose PersistenceStrategy = "pre_close"
)

// ValidPersistenceStrategies 所有有效的持久化策略
var ValidPersistenceStrategies = map[PersistenceStrategy]bool{
	StrategyMemory:        true,
	StrategyPreBase:       true,
	StrategyPreCheckpoint: true,
	StrategyElegantClose:  true,
	StrategyPreClose:      true,
}
