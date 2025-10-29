package server

import (
	"testing"

	"github.com/nicexiaonie/number-dispenser/internal/dispenser"
	"github.com/nicexiaonie/number-dispenser/internal/protocol"
	"github.com/nicexiaonie/number-dispenser/internal/storage"
)

// 测试HSET命令对已存在的发号器的处理
func TestHandleHSet_ExistingDispenser(t *testing.T) {
	// 创建服务器
	stor, err := storage.NewFileStorage("test_data", false)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer stor.Delete("test_id") // 清理

	srv := &Server{
		storage:    stor,
		dispensers: make(map[string]dispenser.NumberDispenser),
		factory:    dispenser.NewDispenserFactory(stor.Save),
	}

	// 第一次创建发号器
	result := srv.handleHSet([]string{
		"test_id", "type", "2", "incr_mode", "sequence", "starting", "100", "step", "1", "auto_disk", "memory",
	})

	if result.Type == protocol.Error {
		t.Fatalf("Failed to create dispenser: %s", result.Str)
	}

	// 生成几个号码
	for i := 0; i < 5; i++ {
		srv.handleGet([]string{"test_id"})
	}

	// 获取当前值（应该是105）
	d := srv.dispensers["test_id"]
	currentBefore := d.GetCurrent()
	if currentBefore != 105 {
		t.Errorf("Expected current=105, got %d", currentBefore)
	}

	// 测试1: 尝试修改type（应该失败）
	t.Run("CannotChangeType", func(t *testing.T) {
		result := srv.handleHSet([]string{
			"test_id", "type", "1", "length", "7",
		})

		if result.Type != protocol.Error {
			t.Error("Expected error when changing type, but succeeded")
		}
		if result.Str == "" || len(result.Str) < 10 {
			t.Errorf("Expected detailed error message, got: %s", result.Str)
		}
		t.Logf("Error message (expected): %s", result.Str)

		// 确认current值没有被重置
		currentAfter := srv.dispensers["test_id"].GetCurrent()
		if currentAfter != currentBefore {
			t.Errorf("Current value was reset! Before=%d, After=%d", currentBefore, currentAfter)
		}
	})

	// 测试2: 尝试修改核心参数（应该失败）
	t.Run("CannotChangeCoreParams", func(t *testing.T) {
		result := srv.handleHSet([]string{
			"test_id", "type", "2", "starting", "200", "step", "2",
		})

		if result.Type != protocol.Error {
			t.Error("Expected error when changing core params, but succeeded")
		}
		t.Logf("Error message (expected): %s", result.Str)

		// 确认current值没有被重置
		currentAfter := srv.dispensers["test_id"].GetCurrent()
		if currentAfter != currentBefore {
			t.Errorf("Current value was reset! Before=%d, After=%d", currentBefore, currentAfter)
		}
	})

	// 测试3: 修改auto_disk（应该成功，并保留current值）
	t.Run("CanChangeAutoDisk", func(t *testing.T) {
		result := srv.handleHSet([]string{
			"test_id", "type", "2", "incr_mode", "sequence", "auto_disk", "elegant_close",
		})

		if result.Type == protocol.Error {
			t.Errorf("Failed to change auto_disk: %s", result.Str)
		}

		// 确认current值被保留
		currentAfter := srv.dispensers["test_id"].GetCurrent()
		if currentAfter != currentBefore {
			t.Errorf("Current value was not preserved! Before=%d, After=%d", currentBefore, currentAfter)
		}

		// 确认auto_disk已更改
		cfg := srv.dispensers["test_id"].GetConfig()
		if cfg.AutoDisk != dispenser.StrategyElegantClose {
			t.Errorf("Expected auto_disk=elegant_close, got %s", cfg.AutoDisk)
		}

		t.Logf("Successfully changed auto_disk from memory to elegant_close, current preserved: %d", currentAfter)
	})

	// 测试4: 相同配置再次HSET（应该成功，无副作用）
	t.Run("IdempotentHSet", func(t *testing.T) {
		result := srv.handleHSet([]string{
			"test_id", "type", "2", "incr_mode", "sequence", "auto_disk", "elegant_close",
		})

		if result.Type == protocol.Error {
			t.Errorf("Failed on idempotent HSET: %s", result.Str)
		}

		// 确认current值没有变化
		currentAfter := srv.dispensers["test_id"].GetCurrent()
		if currentAfter != currentBefore {
			t.Errorf("Current value changed on idempotent HSET! Before=%d, After=%d", currentBefore, currentAfter)
		}
	})

	// 测试5: 验证生成的号码继续从正确的位置开始
	t.Run("ContinuesFromCorrectPosition", func(t *testing.T) {
		result := srv.handleGet([]string{"test_id"})
		if result.Type == protocol.Error {
			t.Fatalf("Failed to get number: %s", result.Str)
		}

		// 应该是 "105"（之前生成到104）
		if result.Bulk != "105" {
			t.Errorf("Expected next number to be 105, got %s", result.Bulk)
		}
		t.Logf("Next number after auto_disk change: %s (correct!)", result.Bulk)
	})
}

// 测试对随机类型发号器的处理
func TestHandleHSet_RandomTypeDispenser(t *testing.T) {
	stor, err := storage.NewFileStorage("test_data", false)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer stor.Delete("random_id")

	srv := &Server{
		storage:    stor,
		dispensers: make(map[string]dispenser.NumberDispenser),
		factory:    dispenser.NewDispenserFactory(stor.Save),
	}

	// 创建Type 1发号器
	result := srv.handleHSet([]string{
		"random_id", "type", "1", "length", "7",
	})

	if result.Type == protocol.Error {
		t.Fatalf("Failed to create random dispenser: %s", result.Str)
	}

	// 生成一些号码
	for i := 0; i < 10; i++ {
		srv.handleGet([]string{"random_id"})
	}

	// 尝试修改length（应该失败）
	t.Run("CannotChangeLength", func(t *testing.T) {
		result := srv.handleHSet([]string{
			"random_id", "type", "1", "length", "8",
		})

		if result.Type != protocol.Error {
			t.Error("Expected error when changing length")
		}
		t.Logf("Error message: %s", result.Str)
	})

	// 修改auto_disk（应该成功）
	t.Run("CanChangeAutoDisk", func(t *testing.T) {
		result := srv.handleHSet([]string{
			"random_id", "type", "1", "length", "7", "auto_disk", "pre-checkpoint",
		})

		if result.Type == protocol.Error {
			t.Errorf("Failed to change auto_disk: %s", result.Str)
		}

		cfg := srv.dispensers["random_id"].GetConfig()
		if cfg.AutoDisk != dispenser.StrategyPreCheckpoint {
			t.Errorf("Expected auto_disk=pre-checkpoint, got %s", cfg.AutoDisk)
		}
	})
}

// 测试Type 3字符随机发号器
func TestHandleHSet_AlphanumericDispenser(t *testing.T) {
	stor, err := storage.NewFileStorage("test_data", false)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer stor.Delete("session_id")

	srv := &Server{
		storage:    stor,
		dispensers: make(map[string]dispenser.NumberDispenser),
		factory:    dispenser.NewDispenserFactory(stor.Save),
	}

	// 创建Type 3发号器
	result := srv.handleHSet([]string{
		"session_id", "type", "3", "charset", "hex", "length", "32",
	})

	if result.Type == protocol.Error {
		t.Fatalf("Failed to create alphanumeric dispenser: %s", result.Str)
	}

	// 尝试修改charset（应该失败）
	t.Run("CannotChangeCharset", func(t *testing.T) {
		result := srv.handleHSet([]string{
			"session_id", "type", "3", "charset", "base62", "length", "32",
		})

		if result.Type != protocol.Error {
			t.Error("Expected error when changing charset")
		}
		t.Logf("Error message: %s", result.Str)
	})
}
