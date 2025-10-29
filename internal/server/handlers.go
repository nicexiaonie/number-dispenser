package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nicexiaonie/number-dispenser/internal/dispenser"
	"github.com/nicexiaonie/number-dispenser/internal/protocol"
)

// handleHSet handles the HSET command for configuring a dispenser
// Format: HSET key field1 value1 [field2 value2 ...]
//
// 新的类型系统：
// Type 1: 纯数字随机 - length, unique_check, auto_disk
// Type 2: 纯数字自增 - length (可选), starting, step, incr_mode, auto_disk
// Type 3: 字符随机 - length, charset, auto_disk
// Type 4: 雪花ID - machine_id, datacenter_id, auto_disk
// Type 5: UUID - uuid_format, auto_disk
func (s *Server) handleHSet(args []string) protocol.Value {
	if len(args) < 3 || len(args)%2 == 0 {
		return protocol.Value{Type: protocol.Error, Str: "ERR wrong number of arguments for 'hset' command"}
	}

	name := args[0]
	fields := args[1:]

	// Parse configuration from fields
	cfg := dispenser.Config{}
	hasType := false

	for i := 0; i < len(fields); i += 2 {
		field := strings.ToLower(fields[i])
		value := fields[i+1]

		switch field {
		case "type":
			typeVal, err := strconv.Atoi(value)
			if err != nil {
				return protocol.Value{Type: protocol.Error, Str: "ERR invalid type value"}
			}
			cfg.Type = dispenser.Type(typeVal)
			hasType = true

		case "length":
			length, err := strconv.Atoi(value)
			if err != nil {
				return protocol.Value{Type: protocol.Error, Str: "ERR invalid length value"}
			}
			cfg.Length = length

		case "starting":
			starting, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return protocol.Value{Type: protocol.Error, Str: "ERR invalid starting value"}
			}
			cfg.Starting = starting

		case "step":
			step, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return protocol.Value{Type: protocol.Error, Str: "ERR invalid step value"}
			}
			cfg.Step = step

		case "machine_id", "machine-id":
			machineID, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return protocol.Value{Type: protocol.Error, Str: "ERR invalid machine_id value"}
			}
			cfg.MachineID = machineID

		case "datacenter_id", "datacenter-id":
			datacenterID, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return protocol.Value{Type: protocol.Error, Str: "ERR invalid datacenter_id value"}
			}
			cfg.DatacenterID = datacenterID

		case "incr_mode", "incr-mode":
			cfg.IncrMode = dispenser.IncrementalMode(strings.ToLower(value))
			if cfg.IncrMode != dispenser.IncrModeFixed && cfg.IncrMode != dispenser.IncrModeSequence {
				return protocol.Value{Type: protocol.Error,
					Str: "ERR invalid incr_mode value, valid values: fixed, sequence"}
			}

		case "charset":
			cfg.Charset = dispenser.Charset(strings.ToLower(value))
			if cfg.Charset != dispenser.CharsetHex && cfg.Charset != dispenser.CharsetBase62 {
				return protocol.Value{Type: protocol.Error,
					Str: "ERR invalid charset value, valid values: hex, base62"}
			}

		case "uuid_format", "uuid-format":
			cfg.UUIDFormat = dispenser.UUIDFormat(strings.ToLower(value))
			if cfg.UUIDFormat != dispenser.UUIDFormatStandard && cfg.UUIDFormat != dispenser.UUIDFormatCompact {
				return protocol.Value{Type: protocol.Error,
					Str: "ERR invalid uuid_format value, valid values: standard, compact"}
			}

		case "unique_check", "unique-check":
			unique, err := strconv.ParseBool(value)
			if err != nil {
				return protocol.Value{Type: protocol.Error, Str: "ERR invalid unique_check value"}
			}
			cfg.UniqueCheck = unique

		case "unique_cache_size", "unique-cache-size":
			size, err := strconv.Atoi(value)
			if err != nil {
				return protocol.Value{Type: protocol.Error, Str: "ERR invalid unique_cache_size value"}
			}
			cfg.UniqueCacheSize = size

		case "auto_disk", "auto-disk":
			cfg.AutoDisk = dispenser.PersistenceStrategy(strings.ToLower(value))
			// 验证策略是否有效
			if !dispenser.ValidPersistenceStrategies[cfg.AutoDisk] {
				return protocol.Value{Type: protocol.Error,
					Str: fmt.Sprintf("ERR invalid auto_disk value '%s', valid values: memory, pre-base, pre-checkpoint, elegant_close, pre_close", value)}
			}

		default:
			return protocol.Value{Type: protocol.Error, Str: fmt.Sprintf("ERR unknown field '%s'", field)}
		}
	}

	if !hasType {
		return protocol.Value{Type: protocol.Error, Str: "ERR type field is required"}
	}

	// 如果没有指定auto_disk，使用默认值 elegant_close
	if cfg.AutoDisk == "" {
		cfg.AutoDisk = dispenser.StrategyElegantClose
	}

	// 检查发号器是否已存在
	s.mu.Lock()
	existingDispenser, exists := s.dispensers[name]
	s.mu.Unlock()

	if exists {
		// 发号器已存在，只允许修改 auto_disk 策略
		existingCfg := existingDispenser.GetConfig()

		// 检查核心配置是否改变
		if cfg.Type != existingCfg.Type {
			return protocol.Value{Type: protocol.Error,
				Str: fmt.Sprintf("ERR cannot change dispenser type (existing: %d, new: %d). Use DEL first if you want to recreate", existingCfg.Type, cfg.Type)}
		}

		// 检查其他关键参数
		configChanged := false
		var changedFields []string

		if cfg.Length != 0 && cfg.Length != existingCfg.Length {
			changedFields = append(changedFields, "length")
			configChanged = true
		}
		if cfg.Starting != 0 && cfg.Starting != existingCfg.Starting {
			changedFields = append(changedFields, "starting")
			configChanged = true
		}
		if cfg.Step != 0 && cfg.Step != existingCfg.Step {
			changedFields = append(changedFields, "step")
			configChanged = true
		}
		if cfg.IncrMode != "" && cfg.IncrMode != existingCfg.IncrMode {
			changedFields = append(changedFields, "incr_mode")
			configChanged = true
		}
		if cfg.Charset != "" && cfg.Charset != existingCfg.Charset {
			changedFields = append(changedFields, "charset")
			configChanged = true
		}
		if cfg.UUIDFormat != "" && cfg.UUIDFormat != existingCfg.UUIDFormat {
			changedFields = append(changedFields, "uuid_format")
			configChanged = true
		}
		if cfg.MachineID != 0 && cfg.MachineID != existingCfg.MachineID {
			changedFields = append(changedFields, "machine_id")
			configChanged = true
		}
		if cfg.DatacenterID != 0 && cfg.DatacenterID != existingCfg.DatacenterID {
			changedFields = append(changedFields, "datacenter_id")
			configChanged = true
		}

		if configChanged {
			return protocol.Value{Type: protocol.Error,
				Str: fmt.Sprintf("ERR cannot change core parameters (%s) for existing dispenser. Only 'auto_disk' can be modified. Use DEL first if you want to recreate",
					strings.Join(changedFields, ", "))}
		}

		// 只允许修改 auto_disk
		if cfg.AutoDisk != existingCfg.AutoDisk {
			// 需要使用新的策略重新创建发号器
			// 但保留 current 值和统计信息
			currentValue := existingDispenser.GetCurrent()

			// 使用现有配置，只更新auto_disk
			newCfg := existingCfg
			newCfg.AutoDisk = cfg.AutoDisk

			// 创建新的发号器实例
			d, err := s.factory.CreateDispenser(name, newCfg)
			if err != nil {
				return protocol.Value{Type: protocol.Error, Str: fmt.Sprintf("ERR %v", err)}
			}

			// 恢复 current 值（只对自增类型有效）
			if newCfg.Type == dispenser.TypeNumericIncremental {
				d.SetCurrent(currentValue)
			}

			// 替换发号器
			s.mu.Lock()
			// 关闭旧的发号器
			if err := existingDispenser.Shutdown(); err != nil {
				s.mu.Unlock()
				return protocol.Value{Type: protocol.Error, Str: fmt.Sprintf("ERR failed to shutdown old dispenser: %v", err)}
			}
			s.dispensers[name] = d
			s.mu.Unlock()

			// 保存
			if err := s.storage.Save(name, newCfg, d.GetCurrent()); err != nil {
				return protocol.Value{Type: protocol.Error, Str: fmt.Sprintf("ERR failed to save: %v", err)}
			}

			return protocol.Value{Type: protocol.Integer, Num: int64(len(fields) / 2)}
		}

		// 配置没有变化，返回成功
		return protocol.Value{Type: protocol.Integer, Num: int64(len(fields) / 2)}
	}

	// 发号器不存在，创建新的
	d, err := s.factory.CreateDispenser(name, cfg)
	if err != nil {
		return protocol.Value{Type: protocol.Error, Str: fmt.Sprintf("ERR %v", err)}
	}

	// Save to storage
	s.mu.Lock()
	s.dispensers[name] = d
	s.mu.Unlock()

	if err := s.storage.Save(name, cfg, d.GetCurrent()); err != nil {
		return protocol.Value{Type: protocol.Error, Str: fmt.Sprintf("ERR failed to save: %v", err)}
	}

	return protocol.Value{Type: protocol.Integer, Num: int64(len(fields) / 2)}
}

// handleGet handles the GET command to generate a new number
// Format: GET key
func (s *Server) handleGet(args []string) protocol.Value {
	if len(args) != 1 {
		return protocol.Value{Type: protocol.Error, Str: "ERR wrong number of arguments for 'get' command"}
	}

	name := args[0]

	s.mu.RLock()
	d, exists := s.dispensers[name]
	s.mu.RUnlock()

	if !exists {
		return protocol.Value{Type: protocol.Error, Str: "ERR dispenser not found"}
	}

	number, err := d.Next()
	if err != nil {
		return protocol.Value{Type: protocol.Error, Str: fmt.Sprintf("ERR %v", err)}
	}

	// 根据持久化策略决定是否立即保存
	cfg := d.GetConfig()

	// 只有 elegant_close 策略需要立即保存
	if cfg.AutoDisk == dispenser.StrategyElegantClose {
		// 只对自增类型立即保存
		if cfg.Type == dispenser.TypeNumericIncremental {
			if err := s.storage.Save(name, cfg, d.GetCurrent()); err != nil {
				// 记录错误但继续返回
			}
		}
	}
	// 其他策略（pre-base, pre-checkpoint, pre_close）有自己的持久化机制
	// memory 策略不需要持久化

	return protocol.Value{Type: protocol.BulkString, Bulk: number}
}

// handleDel handles the DEL command to delete a dispenser
// Format: DEL key
func (s *Server) handleDel(args []string) protocol.Value {
	if len(args) != 1 {
		return protocol.Value{Type: protocol.Error, Str: "ERR wrong number of arguments for 'del' command"}
	}

	name := args[0]

	s.mu.Lock()
	_, exists := s.dispensers[name]
	if exists {
		delete(s.dispensers, name)
	}
	s.mu.Unlock()

	if !exists {
		return protocol.Value{Type: protocol.Integer, Num: 0}
	}

	if err := s.storage.Delete(name); err != nil {
		return protocol.Value{Type: protocol.Error, Str: fmt.Sprintf("ERR failed to delete: %v", err)}
	}

	return protocol.Value{Type: protocol.Integer, Num: 1}
}

// handleInfo handles the INFO command to get dispenser information
// Format: INFO key
func (s *Server) handleInfo(args []string) protocol.Value {
	if len(args) != 1 {
		return protocol.Value{Type: protocol.Error, Str: "ERR wrong number of arguments for 'info' command"}
	}

	name := args[0]

	s.mu.RLock()
	d, exists := s.dispensers[name]
	s.mu.RUnlock()

	if !exists {
		return protocol.Value{Type: protocol.Error, Str: "ERR dispenser not found"}
	}

	cfg := d.GetConfig()
	current := d.GetCurrent()

	// 获取统计信息
	stats := d.GetStats()

	// 根据类型显示不同的信息
	var info string
	switch cfg.Type {
	case dispenser.TypeNumericRandom:
		// Type 1: 纯数字随机
		info = fmt.Sprintf("name:%s\ntype:1 (Numeric Random)\nlength:%d\nunique_check:%v\nauto_disk:%s\ngenerated:%d",
			name, cfg.Length, cfg.UniqueCheck, cfg.AutoDisk, stats.TotalGenerated)

	case dispenser.TypeNumericIncremental:
		// Type 2: 纯数字自增
		if cfg.IncrMode == dispenser.IncrModeFixed {
			info = fmt.Sprintf("name:%s\ntype:2 (Numeric Incremental)\nmode:fixed\nlength:%d\nstarting:%d\nstep:%d\ncurrent:%d\nauto_disk:%s\ngenerated:%d\nwasted:%d\nwaste_rate:%.2f%%",
				name, cfg.Length, cfg.Starting, cfg.Step, current, cfg.AutoDisk, stats.TotalGenerated, stats.TotalWasted, stats.WasteRate)
		} else {
			info = fmt.Sprintf("name:%s\ntype:2 (Numeric Incremental)\nmode:sequence\nstarting:%d\nstep:%d\ncurrent:%d\nauto_disk:%s\ngenerated:%d\nwasted:%d\nwaste_rate:%.2f%%",
				name, cfg.Starting, cfg.Step, current, cfg.AutoDisk, stats.TotalGenerated, stats.TotalWasted, stats.WasteRate)
		}

	case dispenser.TypeAlphanumericRandom:
		// Type 3: 字符随机
		info = fmt.Sprintf("name:%s\ntype:3 (Alphanumeric Random)\nlength:%d\ncharset:%s\nauto_disk:%s\ngenerated:%d",
			name, cfg.Length, cfg.Charset, cfg.AutoDisk, stats.TotalGenerated)

	case dispenser.TypeSnowflake:
		// Type 4: 雪花ID
		info = fmt.Sprintf("name:%s\ntype:4 (Snowflake)\nmachine_id:%d\ndatacenter_id:%d\nauto_disk:%s\ngenerated:%d",
			name, cfg.MachineID, cfg.DatacenterID, cfg.AutoDisk, stats.TotalGenerated)

	case dispenser.TypeUUID:
		// Type 5: UUID
		info = fmt.Sprintf("name:%s\ntype:5 (UUID)\nformat:%s\nauto_disk:%s\ngenerated:%d",
			name, cfg.UUIDFormat, cfg.AutoDisk, stats.TotalGenerated)

	default:
		info = fmt.Sprintf("name:%s\ntype:%d (Unknown)", name, cfg.Type)
	}

	return protocol.Value{Type: protocol.BulkString, Bulk: info}
}
