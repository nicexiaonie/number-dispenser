# 快速开始指南

本指南将帮助你快速搭建和使用 Number Dispenser 服务。

## 安装

### 方式 1: 从源码构建

```bash
# 克隆仓库
git clone https://github.com/nicexiaonie/number-dispenser.git
cd number-dispenser

# 安装依赖
make install

# 构建
make build

# 运行
make run
```

### 方式 2: 使用 Docker

```bash
# 构建镜像
docker build -t number-dispenser .

# 运行容器
docker run -d -p 6380:6380 -v $(pwd)/data:/app/data number-dispenser
```

### 方式 3: 使用 Docker Compose

```bash
# 单节点部署
docker-compose up -d

# 多节点集群部署
docker-compose --profile cluster up -d
```

## 基本使用

### 1. 连接到服务器

使用任何 Redis 客户端连接：

```bash
redis-cli -p 6380
```

### 2. 创建发号器

#### 场景 1: 用户ID（7位随机数字）

```bash
127.0.0.1:6380> HSET user_id type 1 length 7
(integer) 1
```

#### 场景 2: 订单号（8位固定长度，从10000000开始）

```bash
127.0.0.1:6380> HSET order_id type 2 length 8 starting 10000000
(integer) 2
```

#### 场景 3: 自增序列（从0开始，步长为1）

```bash
127.0.0.1:6380> HSET sequence_id type 3
(integer) 1
```

### 3. 生成号码

```bash
127.0.0.1:6380> GET user_id
"3845627"

127.0.0.1:6380> GET order_id
"10000000"

127.0.0.1:6380> GET order_id
"10000001"

127.0.0.1:6380> GET sequence_id
"0"

127.0.0.1:6380> GET sequence_id
"1"
```

### 4. 查看发号器信息

```bash
127.0.0.1:6380> INFO order_id
"name:order_id
type:2
length:8
starting:10000000
step:1
current:10000002"
```

### 5. 删除发号器

```bash
127.0.0.1:6380> DEL user_id
(integer) 1
```

## 程序化使用

### Go 语言

```go
package main

import (
    "context"
    "fmt"
    "github.com/go-redis/redis/v8"
)

func main() {
    ctx := context.Background()
    
    // 连接服务器
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6380",
    })
    
    // 创建发号器
    client.Do(ctx, "HSET", "user_id", "type", "1", "length", "7").Result()
    
    // 生成号码
    for i := 0; i < 10; i++ {
        id, err := client.Get(ctx, "user_id").Result()
        if err != nil {
            panic(err)
        }
        fmt.Printf("User ID %d: %s\n", i+1, id)
    }
}
```

### Python 语言

```python
import redis

# 连接服务器
r = redis.Redis(host='localhost', port=6380, decode_responses=True)

# 创建发号器
r.execute_command('HSET', 'order_id', 'type', '2', 'length', '8', 'starting', '10000000')

# 生成号码
for i in range(10):
    order_id = r.get('order_id')
    print(f"Order {i+1}: {order_id}")
```

### Node.js

```javascript
const redis = require('redis');

(async () => {
    // 连接服务器
    const client = redis.createClient({
        socket: {
            host: 'localhost',
            port: 6380
        }
    });
    
    await client.connect();
    
    // 创建发号器
    await client.sendCommand(['HSET', 'sequence_id', 'type', '3']);
    
    // 生成号码
    for (let i = 0; i < 10; i++) {
        const id = await client.get('sequence_id');
        console.log(`Sequence ${i+1}: ${id}`);
    }
    
    await client.quit();
})();
```

### Java

```java
import redis.clients.jedis.Jedis;

public class NumberDispenserExample {
    public static void main(String[] args) {
        // 连接服务器
        Jedis jedis = new Jedis("localhost", 6380);
        
        // 创建发号器
        jedis.sendCommand(Protocol.Command.HSET, 
            "user_id", "type", "1", "length", "7");
        
        // 生成号码
        for (int i = 0; i < 10; i++) {
            String id = jedis.get("user_id");
            System.out.println("User ID " + (i+1) + ": " + id);
        }
        
        jedis.close();
    }
}
```

## 测试服务器

运行提供的测试脚本：

```bash
# 启动服务器（在一个终端）
make run

# 运行测试（在另一个终端）
./scripts/test_server.sh
```

## 性能测试

运行基准测试：

```bash
# 启动服务器
make run

# 运行基准测试
./scripts/benchmark.sh
```

## 常见问题

### Q: 如何确保分布式环境下号码不重复？

A: 对于自增类型的发号器，系统使用号段分配机制。每个节点预先分配一个号段（默认1000个号码），在号段内独立生成，不会冲突。

### Q: 随机类型的发号器会重复吗？

A: 有可能。随机类型适合对唯一性要求不严格的场景。如果需要严格唯一性，请使用自增类型。

### Q: 服务重启后数据会丢失吗？

A: 不会。系统每5秒自动保存数据，服务关闭时也会完整保存。重启后会自动从持久化文件恢复。

### Q: 如何监控发号器状态？

A: 使用 `INFO <name>` 命令查看发号器的当前状态，包括已生成到哪个号码。

### Q: 可以修改已创建的发号器配置吗？

A: 可以，再次使用 `HSET` 命令即可。但注意，修改配置会创建一个全新的发号器，之前的状态会丢失。

## 下一步

- 查看 [完整文档](../README.md)
- 了解 [API 参考](API.md)
- 查看 [部署指南](DEPLOYMENT.md)

