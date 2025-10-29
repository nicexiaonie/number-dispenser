package dispenser

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	mathrand "math/rand"
	"sync"
	"time"
)

var (
	ErrInvalidType     = errors.New("invalid dispenser type")
	ErrInvalidLength   = errors.New("invalid length")
	ErrInvalidStarting = errors.New("invalid starting value")
	ErrInvalidStep     = errors.New("invalid step value")
	ErrInvalidMachine  = errors.New("invalid machine id")
	ErrNumberExhausted = errors.New("number range exhausted")
	ErrInvalidCharset  = errors.New("invalid charset")
	ErrInvalidFormat   = errors.New("invalid format")
)

// Type represents the dispenser type
type Type int

const (
	TypeNumericRandom      Type = 1 // 纯数字随机（去重缓存）
	TypeNumericIncremental Type = 2 // 纯数字自增
	TypeAlphanumericRandom Type = 3 // 字符随机（hex/base62）
	TypeSnowflake          Type = 4 // 雪花ID
	TypeUUID               Type = 5 // 标准UUID
)

// IncrementalMode represents the incremental mode
type IncrementalMode string

const (
	IncrModeFixed    IncrementalMode = "fixed"    // 固定位数
	IncrModeSequence IncrementalMode = "sequence" // 普通序列
)

// Charset represents the character set for Type 3
type Charset string

const (
	CharsetHex    Charset = "hex"    // 十六进制 (0-9, a-f)
	CharsetBase62 Charset = "base62" // Base62 (0-9, a-z, A-Z)
)

// UUIDFormat represents the UUID format for Type 5
type UUIDFormat string

const (
	UUIDFormatStandard UUIDFormat = "standard" // 标准格式：550e8400-e29b-41d4-a716-446655440000
	UUIDFormatCompact  UUIDFormat = "compact"  // 紧凑格式：550e8400e29b41d4a716446655440000
)

// Config represents the configuration of a dispenser
type Config struct {
	Type            Type                `json:"type"`                        // 发号器类型
	Length          int                 `json:"length,omitempty"`            // 长度（Type 1, 2 fixed, 3 使用）
	Starting        int64               `json:"starting,omitempty"`          // 起始值（Type 2 使用）
	Step            int64               `json:"step,omitempty"`              // 步长（Type 2 使用）
	MachineID       int64               `json:"machine_id,omitempty"`        // 机器ID（Type 4 使用）
	DatacenterID    int64               `json:"datacenter_id,omitempty"`     // 数据中心ID（Type 4 使用）
	IncrMode        IncrementalMode     `json:"incr_mode,omitempty"`         // 自增模式（Type 2 使用）
	Charset         Charset             `json:"charset,omitempty"`           // 字符集（Type 3 使用）
	UUIDFormat      UUIDFormat          `json:"uuid_format,omitempty"`       // UUID格式（Type 5 使用）
	AutoDisk        PersistenceStrategy `json:"auto_disk,omitempty"`         // 持久化策略
	UniqueCheck     bool                `json:"unique_check,omitempty"`      // 是否去重（Type 1 使用）
	UniqueCacheSize int                 `json:"unique_cache_size,omitempty"` // 去重缓存大小（Type 1 使用）
}

// Dispenser represents a number dispenser
type Dispenser struct {
	mu      sync.Mutex
	config  Config
	current int64
	rng     *mathrand.Rand

	// 分布式支持：号段分配
	segmentStart int64
	segmentEnd   int64

	// Type 1: 去重支持
	used map[string]bool // 已使用的号码

	// Type 4: Snowflake 支持
	seqCounter     int64 // 序列计数器
	lastTimestamp  int64 // 上次生成的时间戳
	snowflakeEpoch int64 // Snowflake纪元（毫秒）

	// 统计信息
	totalGenerated int64
}

// NewDispenser creates a new dispenser with the given configuration
func NewDispenser(cfg Config) (*Dispenser, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	d := &Dispenser{
		config:         cfg,
		rng:            mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
		snowflakeEpoch: 1288834974657, // Twitter Snowflake epoch: 2010-11-04
	}

	// 根据类型初始化
	switch cfg.Type {
	case TypeNumericRandom:
		// Type 1: 默认启用去重
		if !cfg.UniqueCheck {
			d.config.UniqueCheck = true
		}
		d.used = make(map[string]bool)

	case TypeNumericIncremental:
		// Type 2: 初始化起始值
		d.current = cfg.Starting
		// 设置默认步长
		if d.config.Step == 0 {
			d.config.Step = 1
		}
		// 设置默认模式
		if d.config.IncrMode == "" {
			if cfg.Length > 0 {
				d.config.IncrMode = IncrModeFixed
			} else {
				d.config.IncrMode = IncrModeSequence
			}
		}

	case TypeAlphanumericRandom:
		// Type 3: 设置默认字符集
		if d.config.Charset == "" {
			d.config.Charset = CharsetHex
		}

	case TypeSnowflake:
		// Type 4: 初始化Snowflake
		if d.config.MachineID == 0 {
			d.config.MachineID = 1
		}

	case TypeUUID:
		// Type 5: 设置默认格式
		if d.config.UUIDFormat == "" {
			d.config.UUIDFormat = UUIDFormatStandard
		}
	}

	return d, nil
}

// Next generates the next number
func (d *Dispenser) Next() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	switch d.config.Type {
	case TypeNumericRandom:
		return d.nextNumericRandom()
	case TypeNumericIncremental:
		return d.nextNumericIncremental()
	case TypeAlphanumericRandom:
		return d.nextAlphanumericRandom()
	case TypeSnowflake:
		return d.nextSnowflake()
	case TypeUUID:
		return d.nextUUID()
	default:
		return "", ErrInvalidType
	}
}

// ============================================
// Type 1: 纯数字随机（去重缓存）
// ============================================

func (d *Dispenser) nextNumericRandom() (string, error) {
	if d.used == nil {
		d.used = make(map[string]bool)
	}

	min := pow10(d.config.Length - 1)
	max := pow10(d.config.Length) - 1
	totalSpace := max - min + 1

	// 检查使用率，超过80%时拒绝生成
	usedCount := int64(len(d.used))
	if float64(usedCount)/float64(totalSpace) > 0.8 {
		return "", ErrNumberExhausted
	}

	// 尝试生成不重复的号码（最多100次）
	for retry := 0; retry < 100; retry++ {
		num := min + d.rng.Int63n(max-min+1)
		numStr := fmt.Sprintf("%0*d", d.config.Length, num)

		if !d.used[numStr] {
			d.used[numStr] = true
			d.totalGenerated++
			return numStr, nil
		}
	}

	return "", errors.New("failed to generate unique number after 100 retries")
}

// ============================================
// Type 2: 纯数字自增
// ============================================

func (d *Dispenser) nextNumericIncremental() (string, error) {
	// 根据模式生成
	switch d.config.IncrMode {
	case IncrModeFixed:
		return d.nextIncrFixed()
	case IncrModeSequence:
		return d.nextIncrSequence()
	default:
		return d.nextIncrSequence()
	}
}

// 固定位数自增
func (d *Dispenser) nextIncrFixed() (string, error) {
	maxValue := pow10(d.config.Length) - 1

	if d.current > maxValue {
		return "", ErrNumberExhausted
	}

	num := d.current
	d.current += d.config.Step
	d.totalGenerated++

	return fmt.Sprintf("%0*d", d.config.Length, num), nil
}

// 普通序列自增
func (d *Dispenser) nextIncrSequence() (string, error) {
	num := d.current
	d.current += d.config.Step
	d.totalGenerated++
	return fmt.Sprintf("%d", num), nil
}

// ============================================
// Type 3: 字符随机（hex/base62）
// ============================================

func (d *Dispenser) nextAlphanumericRandom() (string, error) {
	switch d.config.Charset {
	case CharsetHex:
		return d.nextHex()
	case CharsetBase62:
		return d.nextBase62()
	default:
		return d.nextHex()
	}
}

// 生成十六进制字符串
func (d *Dispenser) nextHex() (string, error) {
	bytes := make([]byte, (d.config.Length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	hexStr := hex.EncodeToString(bytes)
	if len(hexStr) > d.config.Length {
		hexStr = hexStr[:d.config.Length]
	}

	d.totalGenerated++
	return hexStr, nil
}

// 生成Base62字符串
func (d *Dispenser) nextBase62() (string, error) {
	const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	result := make([]byte, d.config.Length)

	for i := 0; i < d.config.Length; i++ {
		result[i] = base62Chars[d.rng.Intn(len(base62Chars))]
	}

	d.totalGenerated++
	return string(result), nil
}

// ============================================
// Type 4: Snowflake算法
// ============================================

func (d *Dispenser) nextSnowflake() (string, error) {
	// Snowflake ID 结构 (64位):
	// 1位符号位（0） + 41位时间戳 + 10位机器ID + 12位序列号

	timestamp := time.Now().UnixNano() / 1e6 // 毫秒

	// 如果是同一毫秒，序列号自增
	if timestamp == d.lastTimestamp {
		d.seqCounter = (d.seqCounter + 1) & 0xFFF // 12位，最大4095
		// 如果序列号溢出，等待下一毫秒
		if d.seqCounter == 0 {
			for timestamp <= d.lastTimestamp {
				timestamp = time.Now().UnixNano() / 1e6
			}
		}
	} else {
		d.seqCounter = 0
	}
	d.lastTimestamp = timestamp

	// 时间戳部分（减去纪元）
	timestamp -= d.snowflakeEpoch

	// 组合ID
	// [41位时间戳] [5位数据中心ID] [5位机器ID] [12位序列号]
	datacenterID := d.config.DatacenterID & 0x1F // 5位，最大31
	machineID := d.config.MachineID & 0x1F       // 5位，最大31

	id := (timestamp << 22) |
		(datacenterID << 17) |
		(machineID << 12) |
		d.seqCounter

	d.totalGenerated++
	return fmt.Sprintf("%d", id), nil
}

// ============================================
// Type 5: 标准UUID v4
// ============================================

func (d *Dispenser) nextUUID() (string, error) {
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		return "", err
	}

	// 设置版本号（4）和变体（RFC 4122）
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant RFC 4122

	d.totalGenerated++

	if d.config.UUIDFormat == UUIDFormatCompact {
		// 紧凑格式：550e8400e29b41d4a716446655440000
		return hex.EncodeToString(uuid), nil
	}

	// 标准格式：550e8400-e29b-41d4-a716-446655440000
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

// ============================================
// 辅助函数
// ============================================

// pow10 计算10的n次方
func pow10(n int) int64 {
	if n <= 0 {
		return 1
	}
	result := int64(1)
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// ============================================
// 接口方法实现
// ============================================

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
func (d *Dispenser) AllocateSegment(segmentSize int64) (start, end int64, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 只适用于自增类型
	if d.config.Type != TypeNumericIncremental {
		return 0, 0, errors.New("segment allocation only supported for incremental type")
	}

	start = d.current
	end = d.current + segmentSize*d.config.Step
	d.current = end

	return start, end, nil
}

// ============================================
// 配置验证
// ============================================

func validateConfig(cfg Config) error {
	if cfg.Type < TypeNumericRandom || cfg.Type > TypeUUID {
		return ErrInvalidType
	}

	switch cfg.Type {
	case TypeNumericRandom:
		// Type 1: 纯数字随机
		if cfg.Length <= 0 || cfg.Length > 18 {
			return ErrInvalidLength
		}

	case TypeNumericIncremental:
		// Type 2: 纯数字自增
		if cfg.IncrMode == IncrModeFixed {
			if cfg.Length <= 0 || cfg.Length > 18 {
				return ErrInvalidLength
			}
			// 检查起始值是否超过固定位数
			if cfg.Starting >= pow10(cfg.Length) {
				return ErrInvalidStarting
			}
		}
		if cfg.Starting < 0 {
			return ErrInvalidStarting
		}
		if cfg.Step < 0 {
			return ErrInvalidStep
		}

	case TypeAlphanumericRandom:
		// Type 3: 字符随机
		if cfg.Length <= 0 || cfg.Length > 64 {
			return ErrInvalidLength
		}
		if cfg.Charset != "" && cfg.Charset != CharsetHex && cfg.Charset != CharsetBase62 {
			return ErrInvalidCharset
		}

	case TypeSnowflake:
		// Type 4: Snowflake
		if cfg.MachineID < 0 || cfg.MachineID > 31 {
			return ErrInvalidMachine
		}
		if cfg.DatacenterID < 0 || cfg.DatacenterID > 31 {
			return ErrInvalidMachine
		}

	case TypeUUID:
		// Type 5: UUID
		if cfg.UUIDFormat != "" &&
			cfg.UUIDFormat != UUIDFormatStandard &&
			cfg.UUIDFormat != UUIDFormatCompact {
			return ErrInvalidFormat
		}
	}

	return nil
}
