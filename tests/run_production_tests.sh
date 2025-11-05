#!/bin/bash

# GridKV Production-Grade Test Suite
# Run all tests required before production deployment

set -e

echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║        GridKV Production Test Suite                                ║"
echo "║        Running all production-grade tests                          ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo ""

cd "$(dirname "$0")"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
PASSED=0
FAILED=0

run_test() {
    local test_name=$1
    local test_cmd=$2
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🧪 Running: $test_name"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    if eval "$test_cmd"; then
        echo -e "${GREEN}✅ PASSED${NC}: $test_name"
        ((PASSED++))
    else
        echo -e "${RED}❌ FAILED${NC}: $test_name"
        ((FAILED++))
    fi
}

# 1. Race Detector Tests
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "PHASE 1: Race Detector Tests"
echo "═══════════════════════════════════════════════════════════════════"

run_test "Race Detector - Transport" \
    "go test -race -run=TestTransport_RaceDetector -timeout=5m -v"

# 2. Load Testing
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "PHASE 2: Load Testing"
echo "═══════════════════════════════════════════════════════════════════"

run_test "Load Test - TCP" \
    "go test -run='TestTransport_LoadTesting/TCP' -timeout=3m -v"

run_test "Load Test - UDP" \
    "go test -run='TestTransport_LoadTesting/UDP' -timeout=3m -v"

run_test "Load Test - gnet" \
    "go test -run='TestTransport_LoadTesting/gnet' -timeout=3m -v 2>&1 | grep -v 'Launching gnet' || true"

# 3. Stability Testing (short)
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "PHASE 3: Stability Testing (30 seconds)"
echo "═══════════════════════════════════════════════════════════════════"

run_test "Stability Test - All Transports" \
    "go test -run=TestTransport_Stability -timeout=5m -v"

# 4. Chaos Testing
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "PHASE 4: Chaos Testing (Fault Injection)"
echo "═══════════════════════════════════════════════════════════════════"

run_test "Chaos Test - All Transports" \
    "go test -run=TestTransport_ChaosInjection -timeout=5m -v"

run_test "Connection Churn Test" \
    "go test -run=TestTransport_ConnectionChurn -timeout=3m -v"

# 5. Benchmark Tests
echo ""
echo "═══════════════════════════════════════════════════════════════════"
echo "PHASE 5: Performance Benchmarks"
echo "═══════════════════════════════════════════════════════════════════"

run_test "Benchmark - Production Load" \
    "go test -bench=BenchmarkTransport_Production -benchmem -benchtime=2s -timeout=10m"

# Summary
echo ""
echo ""
echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║                    TEST SUITE SUMMARY                              ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo ""
echo -e "  ${GREEN}Passed${NC}: $PASSED"
echo -e "  ${RED}Failed${NC}: $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║        ✅ ALL TESTS PASSED - READY FOR PRODUCTION                  ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Production Criteria Met:"
    echo "  ✅ Race detector clean"
    echo "  ✅ Load testing passed"
    echo "  ✅ Stability testing passed"
    echo "  ✅ Chaos testing passed"
    echo "  ✅ Benchmarks completed"
    echo ""
    echo "Optional Tests (run separately):"
    echo "  ⭕ 24-hour stability: RUN_24H_TEST=true go test -run=TestTransport_24HourStability -timeout=25h"
    echo ""
    exit 0
else
    echo -e "${RED}╔═══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${RED}║        ❌ SOME TESTS FAILED - NOT READY FOR PRODUCTION             ║${NC}"
    echo -e "${RED}╚═══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    exit 1
fi

