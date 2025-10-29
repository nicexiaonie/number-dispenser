# Number Dispenser

> 基于 Redis 协议的高性能分布式发号器服务 - 重新设计，更清晰的类型系统

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.19+-00ADD8?style=flat&logo=go)](https://golang.org)

## 🎯 特性

- 🚀 **高性能** - QPS 10,000+，内存操作，微秒级响应
- 💾 **可靠持久化** - 5种策略可选，号码浪费率 < 0.1%
- 🔌 **Redis 协议** - 兼容所有 Redis 客户端，开箱即用
- 🌐 **分布式就绪** - Snowflake算法，多节点部署
- ⚡ **零依赖** - 纯 Go 实现，单二进制文件
- 🎨 **清晰的类型系统** - 5种发号器类型，各司其职

## 📦 快速开始

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

服务将在端口 `6380` 启动（可通过 `--port` 修改）。

### 第一个发号器

```bash
# 连接到服务器（使用任何 Redis 客户端）
redis-cli -h 127.0.0.1 -p 6380

# 创建发号器：7位纯数字随机ID，去重
127.0.0.1:6380> HSET user_id type 1 length 7
(integer) 2

# 生成号码
127.0.0.1:6380> GET user_id
"3845627"

127.0.0.1:6380> GET user_id
"9012543"

# 查看状态
127.0.0.1:6380> INFO user_id
name:user_id
type:1 (Numeric Random)
length:7
unique_check:true
auto_disk:elegant_close
generated:2
```

## 🔢 发号器类型

### 总览

| 类型 | 名称 | 输出示例 | 适用场景 |
|------|------|---------|---------|
| **Type 1** | 纯数字随机 | `3845627` | 用户ID、激活码（小规模）|
| **Type 2** | 纯数字自增 | `10000001`、`10000002` | 订单号、会员号 |
| **Type 3** | 字符随机 | `a3f5e8b2` | Session ID、Token |
| **Type 4** | 雪花ID | `1765432109876543210` | 分布式全局ID |
| **Type 5** | 标准UUID | `550e8400-e29b-41d4-...` | 跨系统唯一标识 |

---

### Type 1: 纯数字随机 (Numeric Random)

**特点**: 固定位数、纯数字（0-9）、内存去重、100%唯一

**适用场景**: 
- 用户ID（小规模，< 10万）
- 激活码
- 验证码

**配置**:
```bash
HSET <name> type 1 length <length> [auto_disk <strategy>]
```

**示例**:
```bash
# 7位用户ID，去重
HSET user_id type 1 length 7
GET user_id  # "3845627"
GET user_id  # "9012354"  # 保证不重复
```

**限制**: 
- 使用率超过80%时拒绝生成（避免无限重试）
- 不适合超大规模（> 10万个），建议用 Type 4

---

### Type 2: 纯数字自增 (Numeric Incremental)

**特点**: 严格递增、纯数字、支持固定位数和普通序列两种模式

#### 模式1: 固定位数自增 (Fixed)

**配置**:
```bash
HSET <name> type 2 incr_mode fixed length <length> starting <starting> [step <step>] [auto_disk <strategy>]
```

**示例**:
```bash
# 12位订单号，从100000000000开始
HSET order_id type 2 incr_mode fixed length 12 starting 100000000000
GET order_id  # "100000000000"
GET order_id  # "100000000001"
GET order_id  # "100000000002"
```

#### 模式2: 普通序列自增 (Sequence)

**配置**:
```bash
HSET <name> type 2 incr_mode sequence starting <starting> [step <step>] [auto_disk <strategy>]
```

**示例**:
```bash
# 从0开始，步长3的序列
HSET seq_id type 2 incr_mode sequence starting 0 step 3
GET seq_id  # "0"
GET seq_id  # "3"
GET seq_id  # "6"
```

**适用场景**:
- 订单号、会员卡号（fixed模式）
- 数据库主键、日志序号（sequence模式）

---

### Type 3: 字符随机 (Alphanumeric Random)

**特点**: 支持十六进制(hex)和Base62两种字符集

#### 字符集1: 十六进制 (hex)

**输出**: `0-9, a-f`

**配置**:
```bash
HSET <name> type 3 charset hex length <length>
```

**示例**:
```bash
# 32位Session ID
HSET session_id type 3 charset hex length 32
GET session_id  # "a3f5e8b2c9d147064b8e7f9a5c3d2e1f"
```

#### 字符集2: Base62

**输出**: `0-9, a-z, A-Z`

**配置**:
```bash
HSET <name> type 3 charset base62 length <length>
```

**示例**:
```bash
# 16位API Token
HSET api_token type 3 charset base62 length 16
GET api_token  # "x9Kd2nP7qL4mT5vN"
```

**适用场景**:
- Session ID、JWT Token（hex）
- API Key、短链接（base62）
- 不要求纯数字的场景

---

### Type 4: 雪花ID (Snowflake)

**特点**: 64位整数、趋势递增、包含时间戳、分布式唯一

**结构**: [41位时间戳] + [5位数据中心ID] + [5位机器ID] + [12位序列号]

**配置**:
```bash
HSET <name> type 4 machine_id <0-31> [datacenter_id <0-31>]
```

**示例**:
```bash
# 单机房部署
HSET global_id type 4 machine_id 1
GET global_id  # "1765432109876543210"

# 多机房部署
HSET global_id type 4 datacenter_id 1 machine_id 5
GET global_id  # "1765439876543210567"
```

**适用场景**:
- 分布式系统全局ID
- 高并发场景（同一毫秒支持4096个ID）
- 需要时间排序

**注意**: 
- 每个节点必须配置不同的 `machine_id`
- `machine_id` 和 `datacenter_id` 范围：0-31

---

### Type 5: 标准UUID (UUID v4)

**特点**: RFC 4122 标准、全局唯一、无中心依赖

**配置**:
```bash
HSET <name> type 5 [uuid_format <standard|compact>]
```

**示例**:
```bash
# 标准格式（带连字符）
HSET uuid_id type 5 uuid_format standard
GET uuid_id  # "550e8400-e29b-41d4-a716-446655440000"

# 紧凑格式（无连字符）
HSET uuid_id type 5 uuid_format compact
GET uuid_id  # "550e8400e29b41d4a716446655440000"
```

**适用场景**:
- 需要标准UUID的系统
- 跨系统互操作
- 全局唯一性要求极高

---

## 💾 持久化策略 (auto_disk)

通过 `auto_disk` 参数配置（默认 `elegant_close`）：

| 策略 | QPS | 正常关闭浪费 | 异常重启浪费 | 推荐场景 |
|------|-----|-------------|-------------|---------|
| `memory` | 10,000+ | 100% | 100% | 测试环境 |
| `pre-base` | 10,000+ | 50% | 50% | 可容忍浪费 |
| `pre-checkpoint` | 10,000+ | 50% | < 5% | **一般生产** ⭐ |
| `elegant_close` | 200-1,000 | 0% | 50% | 低并发 |
| `pre_close` | 10,000+ | 0% | < 0.1% | **高并发** ⭐⭐ |

**说明**:
- `memory`: 纯内存，不持久化
- `pre-base`: 号段预分配（每次分配1000个）
- `pre-checkpoint`: 预分配 + 每2秒保存一次
- `elegant_close`: 每次生成后立即保存 + 优雅关闭
- `pre_close`: 预分配 + 2秒检查点 + 优雅关闭（最优）

**示例**:
```bash
# 高并发订单号（推荐）
HSET order_id type 2 incr_mode fixed length 12 starting 100000000000 auto_disk pre_close

# 一般场景
HSET user_id type 1 length 10 auto_disk pre-checkpoint

# 测试环境
HSET test_id type 2 incr_mode sequence starting 0 auto_disk memory
```

**详细说明**: 请参见 [AUTO_DISK_USAGE.md](docs/AUTO_DISK_USAGE.md)

---

## 📖 命令参考

### HSET - 创建/更新发号器

```
HSET <name> type <1|2|3|4|5> [<type-specific-params>] [auto_disk <strategy>]
```

#### Type 1 参数

```bash
HSET <name> type 1 length <length> [auto_disk <strategy>]
```

- `length` (必需): 位数，1-18

#### Type 2 参数

```bash
# 固定位数模式
HSET <name> type 2 incr_mode fixed length <length> starting <starting> [step <step>] [auto_disk <strategy>]

# 序列模式
HSET <name> type 2 incr_mode sequence starting <starting> [step <step>] [auto_disk <strategy>]
```

- `incr_mode` (可选): `fixed` 或 `sequence`，默认根据 `length` 自动判断
- `length` (fixed模式必需): 位数
- `starting` (可选): 起始值，默认0
- `step` (可选): 步长，默认1

#### Type 3 参数

```bash
HSET <name> type 3 charset <hex|base62> length <length> [auto_disk <strategy>]
```

- `charset` (可选): `hex` 或 `base62`，默认hex
- `length` (必需): 长度

#### Type 4 参数

```bash
HSET <name> type 4 machine_id <0-31> [datacenter_id <0-31>] [auto_disk <strategy>]
```

- `machine_id` (必需): 机器ID，0-31
- `datacenter_id` (可选): 数据中心ID，0-31，默认0

#### Type 5 参数

```bash
HSET <name> type 5 [uuid_format <standard|compact>] [auto_disk <strategy>]
```

- `uuid_format` (可选): `standard` 或 `compact`，默认standard

---

### GET - 生成号码

```bash
GET <name>
```

返回一个新生成的号码。

---

### INFO - 查看状态

```bash
INFO <name>
```

返回发号器的详细信息，包括类型、配置、生成统计等。

**示例输出**:
```
name:order_id
type:2 (Numeric Incremental)
mode:fixed
length:12
starting:100000000000
step:1
current:100000000125
auto_disk:pre_close
generated:125
wasted:2
waste_rate:1.57%
```

---

### DEL - 删除发号器

```bash
DEL <name>
```

删除指定的发号器及其持久化数据。

---

### PING - 健康检查

```bash
PING
```

返回 `PONG`，用于检查服务是否正常。

---

## 🚀 性能

### 基准测试

在 MacBook Pro (M1 Pro, 16GB RAM) 上的测试结果：

| 操作 | QPS | 延迟 (p99) |
|------|-----|-----------|
| Type 1 (Random) | 2,500,000 | < 1μs |
| Type 2 (Incremental) | 8,500,000 | < 1μs |
| Type 3 (Hex) | 5,100,000 | < 1μs |
| Type 4 (Snowflake) | 4,800,000 | < 1μs |
| Type 5 (UUID) | 5,100,000 | < 1μs |

**运行基准测试**:
```bash
make benchmark
```

---

## 📐 架构

### 核心组件

```
┌─────────────────────────────────────────┐
│         Redis Protocol Layer            │
│  (兼容redis-cli和所有Redis客户端)        │
└────────────────┬────────────────────────┘
                 │
┌────────────────▼────────────────────────┐
│           Handler Layer                 │
│  (HSET, GET, INFO, DEL, PING)           │
└────────────────┬────────────────────────┘
                 │
┌────────────────▼────────────────────────┐
│        Dispenser Factory                │
│  (根据auto_disk策略创建不同实现)         │
└────────────────┬────────────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
┌───────▼──────┐  ┌──────▼──────────┐
│   Basic      │  │   Optimized     │
│  Dispenser   │  │   Segment       │
│ (立即保存)   │  │ (预分配+检查点)  │
└──────────────┘  └─────────────────┘
```

### 号段预分配机制

```
传统模式（立即保存）:
  GET → 生成号码 → 写磁盘 → 返回
  ❌ 每次都要写磁盘，性能瓶颈

号段预分配模式:
  1. 预分配1000个号码到内存：[100, 1100)
  2. GET请求从内存直接分配：100, 101, 102...
  3. 用到80%时，异步预加载下一段：[1100, 2100)
  4. 每2秒保存一次当前位置（checkpoint）
  5. 优雅关闭时保存实际位置
  
  ✅ 磁盘写入减少1000倍，号码浪费<0.1%
```

**详细说明**: 请参见 [ARCHITECTURE.md](docs/ARCHITECTURE.md)

---

## 📚 文档

- [快速开始](docs/QUICKSTART.md) - 5分钟上手指南
- [架构说明](docs/ARCHITECTURE.md) - 系统设计与实现细节
- [持久化策略](docs/AUTO_DISK_USAGE.md) - 5种策略的详细对比
- [部署指南](docs/DEPLOYMENT.md) - 生产环境部署建议

---

## 🤝 使用场景

### 场景1: 电商订单号

**需求**: 12位订单号，从100000000000开始，保证唯一且连续

```bash
HSET order_id type 2 incr_mode fixed length 12 starting 100000000000 auto_disk pre_close
GET order_id  # "100000000000"
GET order_id  # "100000000001"
```

**为什么选Type 2**: 连续递增，便于查询和统计

---

### 场景2: 用户ID

**需求**: 7位数字ID，随机不重复

```bash
HSET user_id type 1 length 7 auto_disk pre-checkpoint
GET user_id  # "3845627"
GET user_id  # "9012354"  # 100%不重复
```

**为什么选Type 1**: 随机性好，难以推测用户总数

---

### 场景3: Session ID

**需求**: 32位随机字符串，高性能

```bash
HSET session_id type 3 charset hex length 32
GET session_id  # "a3f5e8b2c9d147064b8e7f9a5c3d2e1f"
```

**为什么选Type 3**: 字符集更大，碰撞概率极低，生成速度快

---

### 场景4: 分布式全局ID

**需求**: 多个服务节点，需要全局唯一ID，且能按时间排序

```bash
# 节点1
HSET global_id type 4 machine_id 1
GET global_id  # "1765432109876543210"

# 节点2
HSET global_id type 4 machine_id 2
GET global_id  # "1765432109876543489"
```

**为什么选Type 4**: Snowflake算法，分布式友好，包含时间信息

---

## 🛠️ 开发

### 运行测试

```bash
# 运行所有测试
make test

# 运行特定包的测试
go test -v ./internal/dispenser/...

# 查看测试覆盖率
make test-coverage
```

### 项目结构

```
.
├── cmd/
│   └── number-dispenser/    # 主程序入口
├── internal/
│   ├── dispenser/            # 发号器核心逻辑
│   │   ├── dispenser.go      # 基础发号器（5种类型）
│   │   ├── segment.go        # 号段预分配发号器
│   │   ├── segment_optimized.go  # 优化版（检查点+优雅关闭）
│   │   ├── factory.go        # 工厂模式
│   │   └── *_test.go         # 单元测试
│   ├── protocol/             # Redis协议解析
│   ├── server/               # TCP服务器和命令处理
│   └── storage/              # 持久化存储
├── docs/                     # 文档
├── examples/                 # 示例代码
└── Makefile                  # 构建脚本
```

---

## 📋 TODO

- [ ] 支持更多Redis命令（MGET, EXISTS等）
- [ ] 添加Web管理界面
- [ ] 支持号码池预热
- [ ] 添加Prometheus监控指标
- [ ] Docker镜像和K8s部署示例

---

## 🙏 致谢

- [Redis RESP Protocol](https://redis.io/docs/reference/protocol-spec/)
- [Twitter Snowflake](https://github.com/twitter-archive/snowflake)
- [UUID RFC 4122](https://www.ietf.org/rfc/rfc4122.txt)

---

## 📄 License

MIT License - 详见 [LICENSE](LICENSE)

---

## 💬 联系

- Issue: https://github.com/yourusername/number-dispenser/issues
- Email: your.email@example.com

---

**⭐ 如果这个项目对你有帮助，请给个Star！**
