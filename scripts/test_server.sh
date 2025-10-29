#!/bin/bash

# Test script for number dispenser server

set -e

HOST=${1:-localhost}
PORT=${2:-6380}

echo "=== Testing Number Dispenser Server ==="
echo "Host: $HOST"
echo "Port: $PORT"
echo ""

# Check if redis-cli is available
if ! command -v redis-cli &> /dev/null; then
    echo "redis-cli not found. Please install redis-tools."
    echo "  Ubuntu/Debian: sudo apt-get install redis-tools"
    echo "  macOS: brew install redis"
    exit 1
fi

# Wait for server to be ready
echo "Checking server connection..."
for i in {1..10}; do
    if redis-cli -h "$HOST" -p "$PORT" PING &> /dev/null; then
        echo "✓ Server is ready!"
        break
    fi
    if [ $i -eq 10 ]; then
        echo "✗ Server is not responding. Please start the server first:"
        echo "  make run"
        echo "  or"
        echo "  ./bin/number-dispenser"
        exit 1
    fi
    sleep 1
done

echo ""
echo "=== Test 1: Random Fixed-Length Numbers ==="
echo "Creating dispenser 'test_random' (type 1, length 7)..."
redis-cli -h "$HOST" -p "$PORT" HSET test_random type 1 length 7

echo "Generating 5 random numbers:"
for i in {1..5}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_random)
    echo "  $i. $NUM"
done

echo ""
echo "=== Test 2: Fixed-Length Incremental Numbers ==="
echo "Creating dispenser 'test_incr_fixed' (type 2, length 8, starting 10001000)..."
redis-cli -h "$HOST" -p "$PORT" HSET test_incr_fixed type 2 length 8 starting 10001000

echo "Generating 5 incremental numbers:"
for i in {1..5}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_incr_fixed)
    echo "  $i. $NUM"
done

echo ""
echo "=== Test 3: Incremental with Step ==="
echo "Creating dispenser 'test_step' (type 3, starting 5, step 3)..."
redis-cli -h "$HOST" -p "$PORT" HSET test_step type 3 starting 5 step 3

echo "Generating 5 numbers with step 3:"
for i in {1..5}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_step)
    echo "  $i. $NUM"
done

echo ""
echo "=== Test 4: Dispenser Info ==="
echo "Getting info for 'test_incr_fixed':"
redis-cli -h "$HOST" -p "$PORT" INFO test_incr_fixed

echo ""
echo "=== Test 5: Delete Dispenser ==="
echo "Deleting 'test_random'..."
RESULT=$(redis-cli -h "$HOST" -p "$PORT" DEL test_random)
echo "Deleted: $RESULT dispenser(s)"

echo ""
echo "=== All Tests Passed! ==="
echo ""
echo "Try these commands yourself:"
echo "  redis-cli -p $PORT"
echo ""
echo "Example commands:"
echo "  HSET myapp_userid type 1 length 10"
echo "  GET myapp_userid"
echo "  INFO myapp_userid"
echo "  DEL myapp_userid"

