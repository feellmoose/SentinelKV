#!/bin/bash

# Comprehensive distributed testing script for GridKV
# Tests scalability, stability, and long-running scenarios

set -e

RESULTS_DIR="./test_results"
mkdir -p "$RESULTS_DIR"

echo "================================================================================"
echo "GridKV Distributed Testing Suite"
echo "================================================================================"
echo ""

# Test configuration
RUN_QUICK=${RUN_QUICK:-0}
RUN_FULL=${RUN_FULL:-0}
RUN_STRESS=${RUN_STRESS:-0}

if [ "$RUN_QUICK" -eq 0 ] && [ "$RUN_FULL" -eq 0 ] && [ "$RUN_STRESS" -eq 0 ]; then
    echo "Usage:"
    echo "  RUN_QUICK=1 ./run_distributed_tests.sh    # Quick tests (~5 min)"
    echo "  RUN_FULL=1 ./run_distributed_tests.sh     # Full tests (~30 min)"
    echo "  RUN_STRESS=1 ./run_distributed_tests.sh   # Stress tests (~2 hours)"
    echo ""
    echo "Running QUICK tests by default..."
    RUN_QUICK=1
fi

# Quick tests (5-10 minutes)
if [ "$RUN_QUICK" -eq 1 ]; then
    echo "========== Quick Distributed Tests =========="
    echo ""
    
    echo "1. Cluster scalability (3, 5, 10 nodes)..."
    go test -bench=BenchmarkClusterScaling -benchmem -timeout 15m . \
        | tee "$RESULTS_DIR/scalability_quick.txt"
    
    echo ""
    echo "2. Cluster formation performance..."
    go test -bench=BenchmarkClusterFormation -benchmem -timeout 10m . \
        | tee "$RESULTS_DIR/formation.txt"
    
    echo ""
    echo "Quick tests complete. Results in: $RESULTS_DIR"
fi

# Full tests (30 minutes - 1 hour)
if [ "$RUN_FULL" -eq 1 ]; then
    echo ""
    echo "========== Full Distributed Tests =========="
    echo ""
    
    echo "1. Cluster scalability (all sizes)..."
    go test -bench=BenchmarkClusterScaling -benchmem -timeout 30m . \
        | tee "$RESULTS_DIR/scalability_full.txt"
    
    echo ""
    echo "2. Massive cluster (50 nodes)..."
    go test -bench=BenchmarkMassiveCluster_50Nodes -benchmem -timeout 45m . \
        | tee "$RESULTS_DIR/massive_50.txt"
    
    echo ""
    echo "3. Long-running stability (10 nodes, 30 min)..."
    go test -run TestLongRunning_10Nodes_30Min -timeout 45m -v . \
        | tee "$RESULTS_DIR/longrunning_10nodes.txt"
    
    echo ""
    echo "4. Chaos testing (node failures)..."
    go test -run TestChaos_NodeFailures -timeout 15m -v . \
        | tee "$RESULTS_DIR/chaos_failures.txt"
    
    echo ""
    echo "Full tests complete. Results in: $RESULTS_DIR"
fi

# Stress tests (2+ hours)
if [ "$RUN_STRESS" -eq 1 ]; then
    echo ""
    echo "========== Stress Tests (Extended) =========="
    echo ""
    
    echo "1. Massive cluster (100 nodes)..."
    go test -bench=BenchmarkMassiveCluster_100Nodes -benchmem -timeout 60m . \
        | tee "$RESULTS_DIR/massive_100.txt"
    
    echo ""
    echo "2. Long-running stability (5 nodes, 1 hour)..."
    go test -run TestLongRunning_5Nodes_1Hour -timeout 90m -v . \
        | tee "$RESULTS_DIR/longrunning_1hour.txt"
    
    echo ""
    echo "3. Network partition simulation..."
    go test -run TestChaos_NetworkPartition -timeout 20m -v . \
        | tee "$RESULTS_DIR/chaos_partition.txt"
    
    echo ""
    echo "4. High concurrency stress (500 concurrent users)..."
    go test -run TestChaos_HighConcurrency -timeout 15m -v . \
        | tee "$RESULTS_DIR/chaos_concurrency.txt"
    
    echo ""
    echo "5. Memory leak detection..."
    go test -run TestStability_MemoryLeaks -timeout 20m -v . \
        | tee "$RESULTS_DIR/stability_memory.txt"
    
    echo ""
    echo "Stress tests complete. Results in: $RESULTS_DIR"
fi

echo ""
echo "================================================================================"
echo "Testing Complete"
echo "================================================================================"
echo ""
echo "Results summary:"
ls -lh "$RESULTS_DIR"
echo ""
echo "To analyze results:"
echo "  cat $RESULTS_DIR/*.txt | grep -E 'ops/sec|success_%|PASS|FAIL'"
echo ""

