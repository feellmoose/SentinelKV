#!/bin/bash

# GridKV Examples Runner
# Run all configuration scenario examples

set -e

echo "═══════════════════════════════════════════════════════"
echo "  GridKV Configuration Scenarios - Run All Examples"
echo "═══════════════════════════════════════════════════════"
echo ""

EXAMPLES_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Scenario list
scenarios=(
    "01_high_concurrency:High Concurrency Cache"
    "02_strong_consistency:Strong Consistency"
    "03_high_availability:High Availability"
    "04_low_latency:Low Latency"
    "05_large_cluster:Large Cluster"
    "06_dev_testing:Development Testing"
)

# Run counters
total=${#scenarios[@]}
success=0
failed=0

echo "Total $total scenario examples"
echo ""

# Run each scenario
for scenario_info in "${scenarios[@]}"; do
    IFS=':' read -r dir name <<< "$scenario_info"
    
    echo "─────────────────────────────────────────────────────"
    echo "Running scenario: $name ($dir)"
    echo "─────────────────────────────────────────────────────"
    echo ""
    
    if [ -d "$EXAMPLES_DIR/$dir" ]; then
        cd "$EXAMPLES_DIR/$dir"
        
        if go run main.go; then
            echo ""
            echo "✅ $name scenario completed successfully"
            ((success++))
        else
            echo ""
            echo "❌ $name scenario failed"
            ((failed++))
        fi
    else
        echo "❌ Directory not found: $dir"
        ((failed++))
    fi
    
    echo ""
    echo "Press Enter to continue to next scenario..."
    read -r
    echo ""
done

# Summary
echo "═══════════════════════════════════════════════════════"
echo "  Execution Summary"
echo "═══════════════════════════════════════════════════════"
echo ""
echo "Total scenarios: $total"
echo "Successful: $success"
echo "Failed: $failed"
echo ""

if [ $failed -eq 0 ]; then
    echo "✅ All scenarios completed successfully!"
    exit 0
else
    echo "⚠️  $failed scenario(s) failed"
    exit 1
fi
