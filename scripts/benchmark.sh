#!/bin/bash
# ============================================================================
# HP Gateway — Benchmark Script
# ============================================================================
# Usage: ./scripts/benchmark.sh [gateway_url]
# Requires: hey (go install github.com/rakyll/hey@latest)
# ============================================================================

set -e

GATEWAY_URL="${1:-http://localhost:8080}"
RESULTS_DIR="benchmarks"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo ""
echo "╔══════════════════════════════════════════════════════╗"
echo "║          HP Gateway — Benchmark Suite                ║"
echo "╠══════════════════════════════════════════════════════╣"
echo "║  Target: $GATEWAY_URL"
echo "║  Time:   $(date)"
echo "╚══════════════════════════════════════════════════════╝"
echo ""

# Check if hey is installed
if ! command -v hey &> /dev/null; then
    echo "❌ 'hey' is not installed."
    echo "   Install it with: go install github.com/rakyll/hey@latest"
    exit 1
fi

# Check if gateway is running
if ! curl -s "$GATEWAY_URL/api/health" > /dev/null 2>&1; then
    echo "❌ Gateway is not running at $GATEWAY_URL"
    echo "   Start it with: make run"
    exit 1
fi

mkdir -p "$RESULTS_DIR"

# Benchmark 1: Basic throughput
echo -e "${BLUE}━━━ Test 1: Basic Throughput (1000 requests, 10 concurrent) ━━━${NC}"
hey -n 1000 -c 10 "$GATEWAY_URL/" 2>&1 | tee "$RESULTS_DIR/throughput_$TIMESTAMP.txt"
echo ""

# Benchmark 2: High concurrency
echo -e "${BLUE}━━━ Test 2: High Concurrency (5000 requests, 100 concurrent) ━━━${NC}"
hey -n 5000 -c 100 "$GATEWAY_URL/" 2>&1 | tee "$RESULTS_DIR/concurrency_$TIMESTAMP.txt"
echo ""

# Benchmark 3: Sustained load (30 seconds)
echo -e "${BLUE}━━━ Test 3: Sustained Load (30 seconds, 50 concurrent) ━━━${NC}"
hey -z 30s -c 50 "$GATEWAY_URL/" 2>&1 | tee "$RESULTS_DIR/sustained_$TIMESTAMP.txt"
echo ""

# Benchmark 4: Rate limit test
echo -e "${BLUE}━━━ Test 4: Rate Limit Test (200 requests, 1 concurrent, rapid) ━━━${NC}"
hey -n 200 -c 1 "$GATEWAY_URL/" 2>&1 | tee "$RESULTS_DIR/ratelimit_$TIMESTAMP.txt"
echo ""

echo -e "${GREEN}✅ Benchmarks complete! Results saved to: $RESULTS_DIR/${NC}"
echo ""

# Print analytics
echo -e "${YELLOW}━━━ Gateway Analytics ━━━${NC}"
curl -s "$GATEWAY_URL/api/analytics" | python3 -m json.tool 2>/dev/null || curl -s "$GATEWAY_URL/api/analytics"
echo ""
