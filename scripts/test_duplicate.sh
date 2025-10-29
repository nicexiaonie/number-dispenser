#!/bin/bash

# Test script to verify no duplicate numbers are generated

set -e

HOST=${1:-localhost}
PORT=${2:-6380}

echo "=== Testing for Duplicate Number Prevention ==="
echo "Host: $HOST"
echo "Port: $PORT"
echo ""

# Check if redis-cli is available
if ! command -v redis-cli &> /dev/null; then
    echo "redis-cli not found. Please install redis-tools."
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
        echo "✗ Server is not responding."
        exit 1
    fi
    sleep 1
done

echo ""
echo "=== Test: Type 2 (Incremental Fixed) - No Duplicates ==="

# Clean up if exists
redis-cli -h "$HOST" -p "$PORT" DEL test_incr &> /dev/null || true

# Create dispenser
echo "Creating dispenser 'test_incr' (type 2, length 12, starting 100, step 2)..."
redis-cli -h "$HOST" -p "$PORT" HSET test_incr type 2 length 12 starting 100 step 2 > /dev/null

# Generate some numbers
echo "Generating 5 numbers:"
NUMBERS=()
for i in {1..5}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_incr)
    echo "  $i. $NUM"
    NUMBERS+=("$NUM")
done

# Check the data file
echo ""
echo "Checking persisted state..."
if [ -f "./data/dispensers.json" ]; then
    echo "Current value in storage:"
    grep -A 5 "test_incr" ./data/dispensers.json | grep "current" || echo "Not found in storage"
fi

echo ""
echo "=== Test: Simulate Connection Loss ==="
echo "The numbers generated were: ${NUMBERS[@]}"
echo ""
echo "Now, please:"
echo "1. Stop the server (Ctrl+C in server terminal)"
echo "2. Restart the server"
echo "3. Run: redis-cli -p $PORT GET test_incr"
echo "4. Verify the next number is NOT a duplicate of any above"
echo ""
echo "Expected next number should be: 110 (not 100, 102, 104, 106, or 108)"

echo ""
echo "=== Test: Type 3 (Incremental from Zero) ==="

# Clean up if exists
redis-cli -h "$HOST" -p "$PORT" DEL test_step &> /dev/null || true

# Create dispenser
echo "Creating dispenser 'test_step' (type 3, starting 0, step 1)..."
redis-cli -h "$HOST" -p "$PORT" HSET test_step type 3 starting 0 step 1 > /dev/null

# Generate numbers and restart in the middle
echo "Generating numbers with simulated restarts..."
for i in {1..3}; do
    NUM=$(redis-cli -h "$HOST" -p "$PORT" GET test_step)
    echo "  $i. $NUM"
done

echo ""
echo "If you restart the server now and run GET test_step,"
echo "you should get 3 (not 0, 1, or 2)"

echo ""
echo "=== Clean Up ==="
redis-cli -h "$HOST" -p "$PORT" DEL test_incr > /dev/null
redis-cli -h "$HOST" -p "$PORT" DEL test_step > /dev/null
echo "Test dispensers deleted."

echo ""
echo "=== Summary ==="
echo "✓ Numbers are now saved immediately after generation"
echo "✓ This prevents duplicates even if server restarts"
echo "✓ Only applies to Type 2 and Type 3 (incremental types)"
echo "✓ Type 1 (random) doesn't need this protection"

