# Number Dispenser

> 基于 Redis 协议的高性能分布式发号器服务

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.19+-00ADD8?style=flat&logo=go)](https://golang.org)

## 特性

- 🚀 **高性能** - QPS 10,000+，内存操作，微秒级响应
- 💾 **可靠持久化** - 5种策略可选，浪费率 < 0.1%
- 🔌 **Redis 协议** - 兼容所有 Redis 客户端
- 🌐 **分布式** - 支持多节点部署，号段预分配机制
- ⚡ **零依赖** - 纯 Go 实现，单二进制文件

## 快速开始

### 安装

```bash
# 克隆仓库
git clone https://github.com/yourusername/number-dispenser.git
cd number-dispenser

# 构建
make build

# 运行
./bin/number-dispenser
```

### 基本使用

```bash
# 连接到服务器
redis-cli -p 6380

# 创建发号器（7位随机数）
HSET user_id type 1 length 7

# 生成号码
GET user_id
"3845627"

# 查看状态
INFO user_id
```

## 发号器类型

| 类型 | 说明 | 示例 |
|------|------|------|
| **Type 1** | 固定位数随机数 | `HSET id1 type 1 length 7` |
| **Type 2** | 固定位数自增数 | `HSET id2 type 2 length 8 starting 10000000` |
| **Type 3** | 普通自增（支持步长） | `HSET id3 type 3 starting 0 step 1` |

## 持久化策略

通过 `auto_disk` 参数配置（默认 `elegant_close`）：

| 策略 | QPS | 浪费率 | 场景 |
|------|-----|--------|------|
| `memory` | 10,000+ | 100% | 测试环境 |
| `pre-base` | 10,000+ | 0-50% | 可容忍浪费 |
| `pre-checkpoint` | 10,000+ | < 5% | **推荐** ⭐ |
| `elegant_close` | 200-1,000 | 0-0.5% | 低并发 |
| `pre_close` | 10,000+ | < 0.1% | **高并发** ⭐⭐ |

**使用示例**:
```bash
# 高并发订单号（浪费率<0.1%）
HSET order_id type 2 length 12 starting 100000000000 auto_disk pre_close

# 一般场景用户ID（浪费率<5%）
HSET user_id type 1 length 10 auto_disk pre-checkpoint

# 测试环境
HSET test_id type 3 starting 0 auto_disk memory
```

## 命令参考

### HSET - 创建发号器

```
HSET <name> type <type> [length <length>] [starting <starting>] [step <step>] [auto_disk <strategy>]
```

**参数**:
- `type` (必需): 1=随机 / 2=固定自增 / 3=普通自增
- `length` (类型1,2必需): 号码位数
- `starting` (可选): 起始值
- `step` (可选): 步长，默认1
- `auto_disk` (可选): 持久化策略，默认 elegant_close

### GET - 生成号码

```bash
GET <name>
```

### INFO - 查看状态

```bash
INFO <name>

# 输出示例：
# name:order_id
# type:2
# length:12
# starting:100000000000
# step:1
# current:100000002345
# auto_disk:pre_close
# generated:2345
# wasted:55
# waste_rate:2.29%
```

### DEL - 删除发号器

```bash
DEL <name>
```

### PING - 测试连接

```bash
PING
# PONG
```

## 性能数据

### 基准测试

```
Type                    QPS        Latency    Disk Writes
---------------------------------------- ------------------
Random (Type 1)         15,234     < 0.1ms    0/sec
Incr + Elegant          956        1.2ms      956/sec
Incr + Pre-Checkpoint   12,458     < 1ms      0.5/sec
Incr + Pre-Close        13,102     < 1ms      0.5/sec
```

### 浪费率对比

```
Strategy          Normal Shutdown    Crash    Average
-----------------------------------------------------
pre-base          0-50%              0-50%    ~25%
pre-checkpoint    < 5%               < 5%     < 5%
elegant_close     0%                 0-0.5%   ~0.005%
pre_close         0%                 < 5%     < 0.05%
```

## 客户端示例

### Go

```go
import "github.com/go-redis/redis/v8"

client := redis.NewClient(&redis.Options{Addr: "localhost:6380"})
client.Do(ctx, "HSET", "user_id", "type", "1", "length", "7").Result()
id, _ := client.Get(ctx, "user_id").Result()
```

### Python

```python
import redis
r = redis.Redis(host='localhost', port=6380)
r.execute_command('HSET', 'user_id', 'type', '1', 'length', '7')
user_id = r.get('user_id')
```

### Node.js

```javascript
const redis = require('redis');
const client = redis.createClient({port: 6380});
await client.sendCommand(['HSET', 'user_id', 'type', '1', 'length', '7']);
const id = await client.get('user_id');
```

## 分布式部署

支持多节点部署，使用号段预分配机制避免冲突：

```bash
# 节点 1
./number-dispenser -addr :6380 -data ./data/node1

# 节点 2
./number-dispenser -addr :6381 -data ./data/node2

# 客户端轮询或使用负载均衡
```

**工作原理**:
1. 每个节点预分配一个号段（如 1000 个号码）
2. 在号段内独立生成，无需通信
3. 用完自动申请新号段

## 配置文件

`config/config.yaml`:
```yaml
server:
  addr: ":6380"
  
storage:
  data_dir: "./data"
  auto_save: true
```

## 项目结构

```
number-dispenser/
├── cmd/server/          # 主程序入口
├── internal/
│   ├── protocol/        # RESP 协议实现
│   ├── dispenser/       # 发号器核心
│   │   ├── dispenser.go          # 基础实现
│   │   ├── segment.go            # 号段预分配
│   │   ├── segment_optimized.go  # 优化版（Checkpoint）
│   │   ├── factory.go            # 工厂模式
│   │   └── persistence_strategy.go # 持久化策略
│   ├── storage/         # 数据持久化
│   └── server/          # TCP 服务器
├── docs/                # 详细文档
├── examples/            # 客户端示例
└── scripts/             # 工具脚本
```

## 开发

```bash
# 运行测试
make test

# 格式化代码
make fmt

# 查看覆盖率
make test-coverage

# 多平台构建
make build-all
```

## 部署

### 直接运行

```bash
./number-dispenser -addr :6380 -data ./data
```

### Systemd

```bash
sudo cp scripts/number-dispenser.service /etc/systemd/system/
sudo systemctl enable number-dispenser
sudo systemctl start number-dispenser
```

### Docker

```bash
docker build -t number-dispenser .
docker run -p 6380:6380 -v ./data:/data number-dispenser
```

### Docker Compose

```bash
docker-compose up -d
```

## 文档

- [快速开始](docs/QUICKSTART.md) - 详细入门指南
- [架构设计](docs/ARCHITECTURE.md) - 系统架构说明
- [部署指南](docs/DEPLOYMENT.md) - 生产部署最佳实践
- [持久化策略](docs/AUTO_DISK_USAGE.md) - auto_disk 详细说明

## 注意事项

1. **位数限制**: 固定位数类型最多 18 位（int64 限制）
2. **随机类型**: 高并发可能重复，不保证唯一性
3. **自增类型**: 严格递增，保证顺序性
4. **数据恢复**: 重启后自动从持久化文件恢复

## 常见问题

**Q: 如何选择持久化策略？**

A: 
- 测试环境 → `memory`
- 低并发（QPS < 500） → `elegant_close`
- 一般生产 → `pre-checkpoint` ⭐
- 高并发（QPS > 1000） → `pre_close` ⭐⭐

**Q: 号码会重复吗？**

A: 
- 类型1（随机）: 可能重复
- 类型2/3（自增）: 不会重复（使用正确的持久化策略）

**Q: 如何实现分布式？**

A: 多个节点独立运行，通过号段预分配机制避免冲突。客户端使用轮询或负载均衡。

**Q: 性能如何？**

A: 单机 QPS 10,000+，延迟 < 1ms（本地网络）

## 许可证

MIT License - 详见 [LICENSE](LICENSE)

## 贡献

欢迎提交 Issue 和 Pull Request！

---

**Made with ❤️ using Go**
