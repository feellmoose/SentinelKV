#!/bin/bash

# GridKV Comprehensive Benchmark Suite
# This script runs all benchmarks and generates a detailed performance report

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BENCHMARK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$BENCHMARK_DIR")"
RESULTS_DIR="$BENCHMARK_DIR/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="$RESULTS_DIR/benchmark_report_$TIMESTAMP.txt"

# Create results directory
mkdir -p "$RESULTS_DIR"

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║         GridKV Comprehensive Benchmark Suite                  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Initialize report file
cat > "$REPORT_FILE" << EOF
================================================================================
GridKV Comprehensive Benchmark Report
================================================================================
Generated: $(date)
Platform: $(uname -s) $(uname -r)
Go Version: $(go version)
CPU: $(lscpu | grep "Model name" | sed 's/Model name://g' | xargs)
Memory: $(free -h | grep Mem | awk '{print $2}')
================================================================================

EOF

# Function to run benchmarks and append to report
run_benchmark_suite() {
    local suite_name=$1
    local benchmark_pattern=$2
    
    echo -e "${YELLOW}Running $suite_name...${NC}"
    echo ""
    echo "================================================================================" >> "$REPORT_FILE"
    echo "$suite_name" >> "$REPORT_FILE"
    echo "================================================================================" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    cd "$PROJECT_ROOT"
    go test -bench="$benchmark_pattern" -benchmem -benchtime=5s -timeout=30m \
        ./tests/... 2>&1 | tee -a "$REPORT_FILE"
    
    echo "" >> "$REPORT_FILE"
    echo -e "${GREEN}✓ $suite_name completed${NC}"
    echo ""
}

# Function to check system prerequisites
check_prerequisites() {
    echo -e "${YELLOW}Checking prerequisites...${NC}"
    
    # Check Go
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed${NC}"
        exit 1
    fi
    
    # Check if project builds
    cd "$PROJECT_ROOT"
    if ! go build ./...; then
        echo -e "${RED}Error: Project does not build${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Prerequisites satisfied${NC}"
    echo ""
}

# Function to check optional benchmark tools
check_optional_tools() {
    echo -e "${YELLOW}Checking optional benchmark tools...${NC}"
    
    local tools_available=0
    
    # Check Redis
    if command -v redis-benchmark &> /dev/null; then
        echo -e "${GREEN}✓ Redis benchmark tool available${NC}"
        tools_available=$((tools_available + 1))
    else
        echo -e "${YELLOW}⚠ Redis benchmark tool not found (optional)${NC}"
    fi
    
    # Check etcd
    if command -v benchmark &> /dev/null; then
        echo -e "${GREEN}✓ etcd benchmark tool available${NC}"
        tools_available=$((tools_available + 1))
    else
        echo -e "${YELLOW}⚠ etcd benchmark tool not found (optional)${NC}"
    fi
    
    # Check Memcached
    if command -v memtier_benchmark &> /dev/null; then
        echo -e "${GREEN}✓ Memcached benchmark tool available${NC}"
        tools_available=$((tools_available + 1))
    else
        echo -e "${YELLOW}⚠ Memcached benchmark tool not found (optional)${NC}"
    fi
    
    echo ""
    return $tools_available
}

# Main benchmark execution
main() {
    echo -e "${BLUE}Starting benchmark suite at $(date)${NC}"
    echo ""
    
    # Check prerequisites
    check_prerequisites
    
    # Check optional tools
    check_optional_tools
    
    # 1. Core Operation Benchmarks
    run_benchmark_suite \
        "Core Operations (Set/Get/Delete)" \
        "Benchmark(Set|Get|Delete)_"
    
    # 2. Concurrent Operation Benchmarks
    run_benchmark_suite \
        "Concurrent Operations" \
        "BenchmarkConcurrent"
    
    # 3. Mixed Workload Benchmarks
    run_benchmark_suite \
        "Mixed Workloads" \
        "BenchmarkMixedWorkload"
    
    # 4. Latency Benchmarks
    run_benchmark_suite \
        "Latency Measurements" \
        "BenchmarkLatency"
    
    # 5. Distributed Cluster Benchmarks
    run_benchmark_suite \
        "Distributed Cluster Operations" \
        "BenchmarkCluster"
    
    # Generate summary
    echo "" >> "$REPORT_FILE"
    echo "================================================================================" >> "$REPORT_FILE"
    echo "Benchmark Summary" >> "$REPORT_FILE"
    echo "================================================================================" >> "$REPORT_FILE"
    echo "Report generated at: $(date)" >> "$REPORT_FILE"
    echo "Total execution time: $SECONDS seconds" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # Print completion message
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║         Benchmark Suite Completed Successfully                ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${GREEN}Report saved to: $REPORT_FILE${NC}"
    echo ""
    
    # Show quick summary
    echo -e "${YELLOW}Quick Summary:${NC}"
    grep -E "Benchmark.*-[0-9]+" "$REPORT_FILE" | tail -20
}

# Run main function
main

exit 0

