# 架构设计文档

## 系统概述

Number Dispenser 是一个基于 Redis 协议的高性能分布式发号器服务，采用 Go 语言实现，支持多种号码生成策略。

## 设计目标

1. **高性能**: 支持高并发号码生成，单机 QPS > 10,000
2. **高可用**: 支持多节点部署，服务不中断
3. **数据持久化**: 自动保存状态，重启后数据不丢失
4. **易用性**: 兼容 Redis 协议，可使用任何 Redis 客户端
5. **可扩展**: 模块化设计，易于扩展新功能

## 系统架构

```
┌─────────────────────────────────────────────────────────┐
│                     Client Layer                         │
│  (Any Redis Client: Go, Python, Java, Node.js, etc.)   │
└────────────────────┬────────────────────────────────────┘
                     │
                     │ RESP Protocol
                     │
┌────────────────────▼────────────────────────────────────┐
│                  Server Layer                            │
│  ┌────────────┐  ┌──────────────┐  ┌─────────────────┐ │
│  │  Protocol  │  │   Command    │  │   Connection    │ │
│  │   Parser   │─▶│   Processor  │  │    Manager      │ │
│  └────────────┘  └──────────────┘  └─────────────────┘ │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│                 Business Layer                           │
│  ┌──────────────────────────────────────────────────┐   │
│  │           Dispenser Manager                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌──────────┐ │   │
│  │  │  Type 1     │  │  Type 2     │  │  Type 3  │ │   │
│  │  │  Random     │  │  Incr Fixed │  │  Incr    │ │   │
│  │  └─────────────┘  └─────────────┘  └──────────┘ │   │
│  └──────────────────────────────────────────────────┘   │
│                                                           │
│  ┌──────────────────────────────────────────────────┐   │
│  │          Cluster Coordinator                      │   │
│  │  (Segment Allocation for Distributed Deployment) │   │
│  └──────────────────────────────────────────────────┘   │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│                 Storage Layer                            │
│  ┌──────────────────┐     ┌─────────────────────────┐   │
│  │  File Storage    │────▶│  dispensers.json        │   │
│  │  (Auto-save)     │     │  (State Persistence)    │   │
│  └──────────────────┘     └─────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## 核心模块

### 1. Protocol 层 (`internal/protocol`)

**职责**: 实现 RESP (Redis Serialization Protocol) 协议的解析和序列化

**核心组件**:
- `Reader`: 从连接读取并解析 RESP 协议消息
- `Writer`: 将响应序列化为 RESP 格式并写入连接
- `Value`: RESP 数据类型的统一表示

**支持的 RESP 类型**:
- Simple String (+)
- Error (-)
- Integer (:)
- Bulk String ($)
- Array (*)

**设计特点**:
- 流式解析，内存占用小
- 支持任何标准 Redis 客户端连接

### 2. Server 层 (`internal/server`)

**职责**: 处理 TCP 连接、命令分发和发号器生命周期管理

**核心组件**:
- `Server`: TCP 服务器，管理连接和发号器实例
- `Handlers`: 命令处理器，实现各种 Redis 命令

**支持的命令**:
- `HSET`: 创建/配置发号器
- `GET`: 生成号码
- `INFO`: 查询发号器信息
- `DEL`: 删除发号器
- `PING`: 健康检查

**并发处理**:
- 每个客户端连接一个 goroutine
- 使用 sync.RWMutex 保护发号器映射
- 优雅关闭，确保数据不丢失

### 3. Dispenser 层 (`internal/dispenser`)

**职责**: 实现核心的号码生成逻辑

#### 类型 1: 固定位数随机数字

```go
// 配置
Config{
    Type:   TypeRandomFixed,
    Length: 7,  // 生成7位数字
}

// 实现原理
min = 10^(length-1)  // 例如: 1000000
max = 10^length - 1  // 例如: 9999999
number = random(min, max)
```

**特点**:
- 生成速度快
- 无状态，适合分布式
- 可能重复（概率极低）

#### 类型 2: 固定位数自增数字

```go
// 配置
Config{
    Type:     TypeIncrFixed,
    Length:   8,
    Starting: 10000000,
    Step:     1,
}

// 实现原理
current = starting
number = format("%08d", current)
current += step
```

**特点**:
- 严格递增
- 需要状态维护
- 支持自定义起始值和步长

#### 类型 3: 普通自增

```go
// 配置
Config{
    Type:     TypeIncrZero,
    Starting: 0,  // 可选
    Step:     1,  // 可选
}

// 实现原理
current = starting (默认0)
number = current
current += step
```

**特点**:
- 简单高效
- 无位数限制
- 适合内部序列号

**并发安全**:
```go
type Dispenser struct {
    mu      sync.Mutex  // 保护 current
    current int64
    // ...
}

func (d *Dispenser) Next() (string, error) {
    d.mu.Lock()
    defer d.mu.Unlock()
    // 生成号码
}
```

### 4. Storage 层 (`internal/storage`)

**职责**: 持久化发号器状态，支持数据恢复

**存储格式** (JSON):
```json
{
  "order_id": {
    "config": {
      "type": 2,
      "length": 8,
      "starting": 10000000,
      "step": 1
    },
    "current": 10002345,
    "updated": "2025-10-29T10:30:00Z"
  }
}
```

**持久化策略**:
1. **自动保存**: 每 5 秒检查并保存变更
2. **优雅关闭**: 服务关闭时完整保存
3. **原子写入**: 先写临时文件，再原子重命名

**实现**:
```go
type FileStorage struct {
    mu       sync.RWMutex
    data     map[string]DispenserData
    dirty    bool  // 标记是否有变更
    autoSave bool  // 是否启用自动保存
}

// 自动保存循环
func (fs *FileStorage) autoSaveLoop() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        if fs.dirty {
            fs.saveToDisk()
        }
    }
}
```

### 5. Cluster 层 (`internal/cluster`)

**职责**: 支持分布式部署的号段分配

**问题**:
在分布式环境中，多个节点如何避免生成重复号码？

**解决方案**: 号段分配机制

```
中心协调器
    │
    ├─▶ 节点1: 分配号段 [1, 1000]
    ├─▶ 节点2: 分配号段 [1001, 2000]
    └─▶ 节点3: 分配号段 [2001, 3000]

节点1 在号段内独立生成: 1, 2, 3, ..., 1000
节点2 在号段内独立生成: 1001, 1002, ..., 2000
```

**实现**:
```go
type SegmentAllocator struct {
    currentSegment *Segment
    segmentSize    int64  // 默认 1000
}

type Segment struct {
    Start   int64
    End     int64
    Current int64
}

// 分配新号段
func (d *Dispenser) AllocateSegment(size int64) (start, end int64, err error) {
    start = d.current
    end = d.current + size * d.config.Step
    d.current = end
    return
}
```

**优势**:
- 节点间无需实时通信
- 号段内生成极快
- 自动续期，无缝切换

## 数据流

### 创建发号器流程

```
Client                Server              Dispenser           Storage
  │                     │                     │                  │
  │─HSET fahaoqi type 1 length 7────────────▶│                  │
  │                     │                     │                  │
  │                     │──NewDispenser()────▶│                  │
  │                     │                     │                  │
  │                     │                     │◀─Validate()      │
  │                     │                     │                  │
  │                     │──Save()────────────────────────────────▶│
  │                     │                     │                  │
  │◀──OK──────────────│                     │                  │
```

### 生成号码流程

```
Client                Server              Dispenser           Storage
  │                     │                     │                  │
  │─GET fahaoqi───────▶│                     │                  │
  │                     │                     │                  │
  │                     │──Next()────────────▶│                  │
  │                     │                     │                  │
  │                     │                     │──Lock()          │
  │                     │                     │──Generate()      │
  │                     │                     │──current++       │
  │                     │                     │──Unlock()        │
  │                     │                     │                  │
  │                     │◀──"1234567"────────│                  │
  │                     │                     │                  │
  │◀──"1234567"───────│                     │                  │
  │                     │                     │                  │
  │                  (5秒后)                 │                  │
  │                     │──AutoSave()────────────────────────────▶│
```

## 性能优化

### 1. 内存优化

- 使用 `sync.Pool` 复用 buffer（可选扩展）
- 流式协议解析，避免大对象分配
- 发号器实例缓存在内存中

### 2. 并发优化

- 读写锁 (`sync.RWMutex`) 用于发号器映射
- 每个发号器独立的互斥锁
- 减少锁粒度，提高并发度

### 3. I/O 优化

- 使用 `bufio` 缓冲 I/O
- 异步持久化，不阻塞号码生成
- 批量写入（dirty flag）

### 4. 算法优化

- 随机数使用独立 `rand.Rand` 实例（避免全局锁）
- 自增类型使用简单的原子操作
- 格式化使用 `fmt.Sprintf` 的高效实现

## 可靠性设计

### 1. 数据持久化

```
内存状态 ─┬─▶ 每5秒自动保存 ─▶ 磁盘
          │
          └─▶ 关闭时完整保存 ─▶ 磁盘
                                 │
                                 ▼
                            原子写入
                        (tmp → rename)
```

### 2. 优雅关闭

```go
func (s *Server) handleShutdown() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    <-sigChan
    s.Stop()  // 关闭监听器，等待连接完成，保存数据
}
```

### 3. 故障恢复

启动时自动从 `dispensers.json` 恢复所有发号器状态。

## 扩展性

### 支持的扩展点

1. **新的发号器类型**: 实现 `Dispenser` 接口
2. **新的存储后端**: 实现 `Storage` 接口（如 Redis、MySQL）
3. **新的命令**: 在 `handlers.go` 中添加
4. **协调机制**: 替换 `Coordinator` 实现（如使用 etcd）

### 未来功能

- [ ] 认证和权限控制
- [ ] 监控指标导出（Prometheus）
- [ ] 分布式协调（etcd/consul 集成）
- [ ] Web 管理界面
- [ ] 号码回收机制
- [ ] 更多号码格式（带前缀、UUID 等）

## 测试策略

### 单元测试

- 每个模块独立测试
- 覆盖正常和异常场景
- 并发安全测试

### 集成测试

- 完整的命令流程测试
- 持久化和恢复测试
- 多客户端并发测试

### 性能测试

- 使用 `redis-benchmark` 进行压测
- 基准测试覆盖关键路径
- 监控内存和 CPU 使用

## 安全考虑

1. **输入验证**: 严格验证所有配置参数
2. **资源限制**: 限制连接数、号码长度
3. **错误处理**: 避免泄露敏感信息
4. **网络隔离**: 建议部署在内网
5. **数据加密**: 支持 TLS（通过 stunnel）

## 总结

Number Dispenser 采用分层架构，各层职责清晰：

- **Protocol 层**: 标准协议兼容
- **Server 层**: 连接和命令管理
- **Business 层**: 核心业务逻辑
- **Storage 层**: 数据持久化
- **Cluster 层**: 分布式支持

通过合理的设计和优化，实现了高性能、高可用、易扩展的分布式发号器服务。

