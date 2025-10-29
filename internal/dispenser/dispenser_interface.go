package dispenser

// NumberDispenser 统一的发号器接口
// 所有持久化策略都实现这个接口
type NumberDispenser interface {
	// Next 生成下一个号码
	Next() (string, error)

	// GetConfig 获取配置
	GetConfig() Config

	// GetCurrent 获取当前位置（用于持久化）
	GetCurrent() int64

	// SetCurrent 设置当前位置（用于恢复）
	SetCurrent(current int64)

	// Shutdown 关闭发号器（优雅关闭）
	Shutdown() error

	// GetStats 获取统计信息
	GetStats() DispenserStats
}

// DispenserStats 发号器统计信息
type DispenserStats struct {
	TotalGenerated int64               // 总共生成的号码数
	TotalWasted    int64               // 总共浪费的号码数
	WasteRate      float64             // 浪费率
	Strategy       PersistenceStrategy // 持久化策略
}
