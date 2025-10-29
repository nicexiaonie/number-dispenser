package dispenser

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

var (
	ErrInvalidType     = errors.New("invalid dispenser type")
	ErrInvalidLength   = errors.New("invalid length")
	ErrInvalidStarting = errors.New("invalid starting value")
	ErrInvalidStep     = errors.New("invalid step value")
	ErrNumberExhausted = errors.New("number range exhausted")
)

// Type represents the dispenser type
type Type int

const (
	TypeRandomFixed Type = 1 // 固定位数，随机
	TypeIncrFixed   Type = 2 // 固定位数，自增
	TypeIncrZero    Type = 3 // 从0开始自增
)

// Config represents the configuration of a dispenser
type Config struct {
	Type     Type                `json:"type"`
	Length   int                 `json:"length,omitempty"`
	Starting int64               `json:"starting,omitempty"`
	Step     int64               `json:"step,omitempty"`
	AutoDisk PersistenceStrategy `json:"auto_disk,omitempty"` // 持久化策略
}

// Dispenser represents a number dispenser
type Dispenser struct {
	mu      sync.Mutex
	config  Config
	current int64
	rng     *rand.Rand

	// 分布式支持：号段分配
	segmentStart int64
	segmentEnd   int64

	// 统计信息
	totalGenerated int64
}

// NewDispenser creates a new dispenser with the given configuration
func NewDispenser(cfg Config) (*Dispenser, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	d := &Dispenser{
		config: cfg,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// 初始化当前值
	switch cfg.Type {
	case TypeRandomFixed:
		// 随机类型不需要初始化current
	case TypeIncrFixed:
		d.current = cfg.Starting
	case TypeIncrZero:
		if cfg.Starting > 0 {
			d.current = cfg.Starting
		} else {
			d.current = 0
		}
	}

	// 设置默认步长
	if d.config.Step == 0 {
		d.config.Step = 1
	}

	return d, nil
}

// Next generates the next number
func (d *Dispenser) Next() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	switch d.config.Type {
	case TypeRandomFixed:
		return d.nextRandomFixed()
	case TypeIncrFixed:
		return d.nextIncrFixed()
	case TypeIncrZero:
		return d.nextIncrZero()
	default:
		return "", ErrInvalidType
	}
}

// nextRandomFixed generates a random fixed-length number
func (d *Dispenser) nextRandomFixed() (string, error) {
	min := int64(1)
	for i := 1; i < d.config.Length; i++ {
		min *= 10
	}
	max := min*10 - 1

	num := min + d.rng.Int63n(max-min+1)
	return fmt.Sprintf("%0*d", d.config.Length, num), nil
}

// nextIncrFixed generates an incremental fixed-length number
func (d *Dispenser) nextIncrFixed() (string, error) {
	// 检查是否超出固定位数的最大值
	maxValue := int64(1)
	for i := 0; i < d.config.Length; i++ {
		maxValue *= 10
	}
	maxValue--

	if d.current > maxValue {
		return "", ErrNumberExhausted
	}

	num := d.current
	d.current += d.config.Step
	d.totalGenerated++

	return fmt.Sprintf("%0*d", d.config.Length, num), nil
}

// nextIncrZero generates an incremental number starting from zero
func (d *Dispenser) nextIncrZero() (string, error) {
	num := d.current
	d.current += d.config.Step
	d.totalGenerated++
	return fmt.Sprintf("%d", num), nil
}

// GetConfig returns the dispenser configuration
func (d *Dispenser) GetConfig() Config {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.config
}

// GetCurrent returns the current value (for persistence)
func (d *Dispenser) GetCurrent() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.current
}

// SetCurrent sets the current value (for recovery)
func (d *Dispenser) SetCurrent(current int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.current = current
}

// Shutdown 关闭发号器（基础版无需特殊处理）
func (d *Dispenser) Shutdown() error {
	return nil
}

// GetStats 获取统计信息
func (d *Dispenser) GetStats() DispenserStats {
	d.mu.Lock()
	defer d.mu.Unlock()

	return DispenserStats{
		TotalGenerated: d.totalGenerated,
		TotalWasted:    0,
		WasteRate:      0,
		Strategy:       d.config.AutoDisk,
	}
}

// AllocateSegment allocates a number segment for distributed deployment
// This allows multiple instances to generate numbers without conflicts
func (d *Dispenser) AllocateSegment(segmentSize int64) (start, end int64, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Only applicable for incremental types
	if d.config.Type == TypeRandomFixed {
		return 0, 0, errors.New("segment allocation not supported for random type")
	}

	start = d.current
	end = d.current + segmentSize*d.config.Step
	d.current = end

	return start, end, nil
}

// validateConfig validates the dispenser configuration
func validateConfig(cfg Config) error {
	if cfg.Type < TypeRandomFixed || cfg.Type > TypeIncrZero {
		return ErrInvalidType
	}

	switch cfg.Type {
	case TypeRandomFixed:
		if cfg.Length <= 0 || cfg.Length > 18 {
			return ErrInvalidLength
		}
	case TypeIncrFixed:
		if cfg.Length <= 0 || cfg.Length > 18 {
			return ErrInvalidLength
		}
		if cfg.Starting < 0 {
			return ErrInvalidStarting
		}
		// 检查起始值是否超过固定位数
		maxValue := int64(1)
		for i := 0; i < cfg.Length; i++ {
			maxValue *= 10
		}
		if cfg.Starting >= maxValue {
			return ErrInvalidStarting
		}
	case TypeIncrZero:
		if cfg.Starting < 0 {
			return ErrInvalidStarting
		}
	}

	if cfg.Step < 0 {
		return ErrInvalidStep
	}

	return nil
}
