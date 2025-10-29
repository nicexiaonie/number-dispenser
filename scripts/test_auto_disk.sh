#!/bin/bash

# Test script for auto_disk persistence strategies

set -e

HOST=${1:-localhost}
PORT=${2:-6380}

echo "=== Testing auto_disk Persistence Strategies ==="
echo "Host: $HOST"
echo "Port: $PORT"
echo ""

# Check if redis-cli is available
if ! command -v redis-cli &> /dev/null; then
    echo "redis-cli not found. Please install redis-tools."
    exit 1
fi

# Wait for server
echo "Waiting for server..."
for i in {1..10}; do
    if redis-cli -h "$HOST" -p "$PORT" PING &> /dev/null; then
        echo "✓ Server is ready!"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "✗ Server not responding"
        exit 1
    fi
    sleep 1
done

echo ""
echo "=== Strategy 1: memory (内存模式) ==="
echo "创建: HSET test_memory type 1 length 6 auto_disk memory"
redis-cli -h "$HOST" -p "$PORT" HSET test_memory type 1 length 6 auto_disk memory > /dev/null

echo "生成5个号码:"
for i in {1..5}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_memory)
    echo "  $i. $NUM"
done

echo "查看信息:"
redis-cli -h "$HOST" -p "$PORT" INFO test_memory | grep -E "(auto_disk|generated|wasted|waste_rate)"

echo ""
echo "=== Strategy 2: pre-base (预分配基础版) ==="
echo "创建: HSET test_prebase type 2 length 8 starting 10000000 auto_disk pre-base"
redis-cli -h "$HOST" -p "$PORT" HSET test_prebase type 2 length 8 starting 10000000 auto_disk pre-base > /dev/null

echo "生成5个号码:"
for i in {1..5}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_prebase)
    echo "  $i. $NUM"
done

echo "查看信息:"
redis-cli -h "$HOST" -p "$PORT" INFO test_prebase | grep -E "(auto_disk|generated|wasted|waste_rate)"

echo ""
echo "=== Strategy 3: pre-checkpoint (预分配+检查点) ⭐ ==="
echo "创建: HSET test_checkpoint type 2 length 10 starting 1000000000 auto_disk pre-checkpoint"
redis-cli -h "$HOST" -p "$PORT" HSET test_checkpoint type 2 length 10 starting 1000000000 auto_disk pre-checkpoint > /dev/null

echo "生成5个号码:"
for i in {1..5}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_checkpoint)
    echo "  $i. $NUM"
done

echo "等待checkpoint（2秒）..."
sleep 2

echo "查看信息:"
redis-cli -h "$HOST" -p "$PORT" INFO test_checkpoint | grep -E "(auto_disk|generated|wasted|waste_rate)"

echo ""
echo "=== Strategy 4: elegant_close (优雅关闭) ==="
echo "创建: HSET test_elegant type 3 starting 100 step 1 auto_disk elegant_close"
redis-cli -h "$HOST" -p "$PORT" HSET test_elegant type 3 starting 100 step 1 auto_disk elegant_close > /dev/null

echo "生成5个号码:"
for i in {1..5}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_elegant)
    echo "  $i. $NUM"
done

echo "查看信息:"
redis-cli -h "$HOST" -p "$PORT" INFO test_elegant | grep -E "(auto_disk|generated|wasted|waste_rate)"

echo ""
echo "=== Strategy 5: pre_close (完整方案) ⭐⭐⭐ ==="
echo "创建: HSET test_preclose type 2 length 12 starting 100000000000 auto_disk pre_close"
redis-cli -h "$HOST" -p "$PORT" HSET test_preclose type 2 length 12 starting 100000000000 auto_disk pre_close > /dev/null

echo "生成5个号码:"
for i in {1..5}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_preclose)
    echo "  $i. $NUM"
done

echo "等待checkpoint（2秒）..."
sleep 2

echo "查看信息:"
redis-cli -h "$HOST" -p "$PORT" INFO test_preclose | grep -E "(auto_disk|generated|wasted|waste_rate)"

echo ""
echo "=== 策略对比总结 ==="
echo ""
echo "| 策略 | QPS | 延迟 | 浪费率 | 适用场景 |"
echo "|------|-----|------|--------|---------|"
echo "| memory | 10,000+ | <0.1ms | 100% | 测试环境 |"
echo "| pre-base | 10,000+ | <1ms | 0-50% | 可容忍浪费 |"
echo "| pre-checkpoint ⭐ | 10,000+ | <1ms | <5% | 推荐大多数场景 |"
echo "| elegant_close | 200-1,000 | 1-20ms | 0-0.5% | 低并发 |"
echo "| pre_close ⭐⭐⭐ | 10,000+ | <1ms | <0.1% | 高并发推荐 |"

echo ""
echo "=== 清理测试数据 ==="
redis-cli -h "$HOST" -p "$PORT" DEL test_memory > /dev/null
redis-cli -h "$HOST" -p "$PORT" DEL test_prebase > /dev/null
redis-cli -h "$HOST" -p "$PORT" DEL test_checkpoint > /dev/null
redis-cli -h "$HOST" -p "$PORT" DEL test_elegant > /dev/null
redis-cli -h "$HOST" -p "$PORT" DEL test_preclose > /dev/null
echo "✓ 清理完成"

echo ""
echo "=== 测试完成！==="
echo ""
echo "详细文档: docs/AUTO_DISK_USAGE.md"

