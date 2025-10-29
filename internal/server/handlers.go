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
// 支持的字段: type, length, starting, step, auto_disk
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

		case "auto_disk":
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

	// 使用工厂创建发号器
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
		if cfg.Type == dispenser.TypeIncrFixed || cfg.Type == dispenser.TypeIncrZero {
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

	info := fmt.Sprintf("name:%s\ntype:%d\nlength:%d\nstarting:%d\nstep:%d\ncurrent:%d\nauto_disk:%s\ngenerated:%d\nwasted:%d\nwaste_rate:%.2f%%",
		name, cfg.Type, cfg.Length, cfg.Starting, cfg.Step, current,
		cfg.AutoDisk, stats.TotalGenerated, stats.TotalWasted, stats.WasteRate)

	return protocol.Value{Type: protocol.BulkString, Bulk: info}
}
