#!/bin/bash

# Benchmark script for number dispenser

set -e

HOST=${1:-localhost}
PORT=${2:-6380}

echo "=== Number Dispenser Benchmark ==="
echo "Host: $HOST"
echo "Port: $PORT"
echo ""

# Check if redis-benchmark is available
if ! command -v redis-benchmark &> /dev/null; then
    echo "redis-benchmark not found. Please install redis-tools."
    exit 1
fi

# Wait for server to be ready
echo "Waiting for server to be ready..."
for i in {1..10}; do
    if redis-cli -h "$HOST" -p "$PORT" PING &> /dev/null; then
        echo "Server is ready!"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "Server is not responding. Please start the server first."
        exit 1
    fi
    sleep 1
done

echo ""
echo "Setting up test dispensers..."

# Setup test dispensers
redis-cli -h "$HOST" -p "$PORT" HSET bench_random type 1 length 10 > /dev/null
redis-cli -h "$HOST" -p "$PORT" HSET bench_incr type 3 > /dev/null

echo "Running benchmarks..."
echo ""

# Benchmark GET operations for random type
echo "=== Random Fixed Number Generation ==="
redis-benchmark -h "$HOST" -p "$PORT" -t get -n 100000 -c 50 -q -d 0 GET bench_random

echo ""
echo "=== Incremental Number Generation ==="
redis-benchmark -h "$HOST" -p "$PORT" -t get -n 100000 -c 50 -q -d 0 GET bench_incr

echo ""
echo "=== PING Command ==="
redis-benchmark -h "$HOST" -p "$PORT" -t ping -n 100000 -c 50 -q

echo ""
echo "Benchmark completed!"

