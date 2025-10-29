# 部署指南

本文档介绍如何在生产环境中部署 Number Dispenser 服务。

## 生产环境推荐配置

### 系统要求

- **CPU**: 2 核或更多
- **内存**: 512MB 最小，1GB 推荐
- **磁盘**: 1GB 用于数据存储
- **操作系统**: Linux (Ubuntu 20.04+, CentOS 7+), macOS, Windows

### 性能指标

- **延迟**: < 1ms (本地网络)
- **吞吐量**: 10,000+ QPS (单核)
- **并发连接**: 1,000+ 同时连接

## 部署方式

### 方式 1: 直接运行二进制

#### 1. 编译

```bash
# 下载源码
git clone https://github.com/nicexiaonie/number-dispenser.git
cd number-dispenser

# 编译
make build

# 或编译多平台版本
make build-all
```

#### 2. 创建服务用户

```bash
sudo useradd -r -s /bin/false number-dispenser
```

#### 3. 创建目录

```bash
sudo mkdir -p /opt/number-dispenser/bin
sudo mkdir -p /var/lib/number-dispenser/data
sudo mkdir -p /var/log/number-dispenser

sudo cp bin/number-dispenser /opt/number-dispenser/bin/
sudo chown -R number-dispenser:number-dispenser /var/lib/number-dispenser
sudo chown -R number-dispenser:number-dispenser /var/log/number-dispenser
```

#### 4. 创建 systemd 服务

创建文件 `/etc/systemd/system/number-dispenser.service`:

```ini
[Unit]
Description=Number Dispenser Service
After=network.target

[Service]
Type=simple
User=number-dispenser
Group=number-dispenser
WorkingDirectory=/opt/number-dispenser
ExecStart=/opt/number-dispenser/bin/number-dispenser -addr :6380 -data /var/lib/number-dispenser/data
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# 资源限制
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

#### 5. 启动服务

```bash
sudo systemctl daemon-reload
sudo systemctl enable number-dispenser
sudo systemctl start number-dispenser
sudo systemctl status number-dispenser
```

#### 6. 查看日志

```bash
sudo journalctl -u number-dispenser -f
```

### 方式 2: Docker 部署

#### 1. 构建镜像

```bash
docker build -t number-dispenser:latest .
```

#### 2. 运行容器

```bash
docker run -d \
  --name number-dispenser \
  -p 6380:6380 \
  -v /var/lib/number-dispenser:/app/data \
  --restart unless-stopped \
  number-dispenser:latest
```

#### 3. 查看日志

```bash
docker logs -f number-dispenser
```

### 方式 3: Docker Compose

#### 1. 单节点部署

```yaml
# docker-compose.yaml
version: '3.8'

services:
  dispenser:
    image: number-dispenser:latest
    ports:
      - "6380:6380"
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

```bash
docker-compose up -d
```

#### 2. 多节点集群部署

```yaml
# docker-compose.yaml
version: '3.8'

services:
  dispenser-node1:
    image: number-dispenser:latest
    ports:
      - "6380:6380"
    volumes:
      - ./data/node1:/app/data
    restart: unless-stopped

  dispenser-node2:
    image: number-dispenser:latest
    ports:
      - "6381:6380"
    volumes:
      - ./data/node2:/app/data
    restart: unless-stopped

  dispenser-node3:
    image: number-dispenser:latest
    ports:
      - "6382:6380"
    volumes:
      - ./data/node3:/app/data
    restart: unless-stopped

  # 负载均衡器
  nginx:
    image: nginx:alpine
    ports:
      - "6379:6379"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - dispenser-node1
      - dispenser-node2
      - dispenser-node3
```

Nginx 配置 (`nginx.conf`):

```nginx
stream {
    upstream number_dispenser {
        least_conn;
        server dispenser-node1:6380;
        server dispenser-node2:6380;
        server dispenser-node3:6380;
    }

    server {
        listen 6379;
        proxy_pass number_dispenser;
        proxy_connect_timeout 1s;
    }
}

events {
    worker_connections 1024;
}
```

### 方式 4: Kubernetes 部署

#### 1. 创建 Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: number-dispenser
  labels:
    app: number-dispenser
spec:
  replicas: 3
  selector:
    matchLabels:
      app: number-dispenser
  template:
    metadata:
      labels:
        app: number-dispenser
    spec:
      containers:
      - name: number-dispenser
        image: number-dispenser:latest
        ports:
        - containerPort: 6380
        volumeMounts:
        - name: data
          mountPath: /app/data
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          tcpSocket:
            port: 6380
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          tcpSocket:
            port: 6380
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: number-dispenser-pvc
```

#### 2. 创建 Service

```yaml
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: number-dispenser
spec:
  type: LoadBalancer
  selector:
    app: number-dispenser
  ports:
  - protocol: TCP
    port: 6380
    targetPort: 6380
```

#### 3. 创建 PersistentVolumeClaim

```yaml
# k8s/pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: number-dispenser-pvc
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

#### 4. 部署

```bash
kubectl apply -f k8s/pvc.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

## 高可用架构

### 主从模式

```
                    ┌─────────────┐
                    │ Load Balancer │
                    └───────┬───────┘
                            │
            ┌───────────────┼───────────────┐
            │               │               │
    ┌───────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
    │   Node 1     │ │   Node 2    │ │   Node 3    │
    │   :6380      │ │   :6380     │ │   :6380     │
    └──────────────┘ └─────────────┘ └─────────────┘
```

每个节点独立运行，通过号段分配机制保证不冲突。

### 数据备份

#### 定期备份脚本

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/backup/number-dispenser"
DATA_DIR="/var/lib/number-dispenser/data"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR
tar -czf $BACKUP_DIR/backup_$DATE.tar.gz -C $DATA_DIR .

# 保留最近30天的备份
find $BACKUP_DIR -name "backup_*.tar.gz" -mtime +30 -delete
```

添加到 crontab:

```bash
# 每天凌晨2点备份
0 2 * * * /opt/number-dispenser/scripts/backup.sh
```

## 监控

### 健康检查

```bash
# 使用 PING 命令检查
redis-cli -h localhost -p 6380 PING

# 检查脚本
#!/bin/bash
if redis-cli -h localhost -p 6380 PING | grep -q PONG; then
    echo "OK"
    exit 0
else
    echo "FAIL"
    exit 1
fi
```

### 指标收集

推荐使用 Prometheus + Grafana 进行监控。

#### Prometheus 配置示例

```yaml
scrape_configs:
  - job_name: 'number-dispenser'
    static_configs:
      - targets: ['localhost:6380']
```

### 日志管理

#### 集中式日志

使用 ELK Stack 或类似工具收集日志：

```bash
# 使用 journalctl 导出日志到文件
journalctl -u number-dispenser -f --output=json > /var/log/dispenser.json
```

## 安全加固

### 1. 防火墙配置

```bash
# 只允许特定IP访问
sudo ufw allow from 192.168.1.0/24 to any port 6380
sudo ufw enable
```

### 2. 使用 stunnel 添加 SSL/TLS

#### 安装 stunnel

```bash
sudo apt-get install stunnel4
```

#### 配置 stunnel

创建 `/etc/stunnel/number-dispenser.conf`:

```ini
[number-dispenser]
accept = 6379
connect = 127.0.0.1:6380
cert = /etc/stunnel/cert.pem
key = /etc/stunnel/key.pem
```

客户端连接到 6379 端口即可使用加密连接。

### 3. 网络隔离

将 Number Dispenser 部署在内网，通过 API Gateway 对外提供服务。

## 性能优化

### 1. 系统调优

```bash
# 增加文件描述符限制
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# TCP 优化
echo "net.ipv4.tcp_max_syn_backlog = 4096" >> /etc/sysctl.conf
echo "net.core.somaxconn = 4096" >> /etc/sysctl.conf
sysctl -p
```

### 2. 应用优化

- 使用连接池而非频繁创建连接
- 批量生成号码以减少网络往返
- 对于随机类型，考虑客户端缓存

## 故障排查

### 常见问题

#### 1. 连接被拒绝

```bash
# 检查服务状态
systemctl status number-dispenser

# 检查端口监听
netstat -tlnp | grep 6380

# 检查防火墙
sudo ufw status
```

#### 2. 性能下降

```bash
# 检查系统资源
top
iostat -x 1

# 检查连接数
netstat -an | grep 6380 | wc -l
```

#### 3. 数据丢失

```bash
# 检查数据文件
ls -lh /var/lib/number-dispenser/data/

# 从备份恢复
tar -xzf backup_YYYYMMDD.tar.gz -C /var/lib/number-dispenser/data/
systemctl restart number-dispenser
```

## 升级指南

### 零停机升级（使用多节点）

1. 更新节点1，停止服务
2. 流量自动切换到节点2和节点3
3. 升级节点1并重启
4. 重复步骤1-3升级其他节点

### 数据迁移

数据文件格式向后兼容，直接替换二进制文件即可。

## 总结

选择合适的部署方式：

- **单机应用**: 直接运行二进制文件
- **容器环境**: 使用 Docker
- **微服务架构**: 使用 Kubernetes
- **高可用需求**: 多节点 + 负载均衡

定期备份数据，监控服务状态，确保系统稳定运行。

