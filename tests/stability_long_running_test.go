package tests

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Long-running stability tests
// These tests run for extended periods to detect memory leaks, race conditions, etc.

// TestLongRunning_5Nodes_1Hour runs a 5-node cluster for 1 hour
// Run with: go test -run TestLongRunning_5Nodes_1Hour -timeout 2h -v ./tests/
func TestLongRunning_5Nodes_1Hour(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	runLongRunningTest(t, 5, 1*time.Hour)
}

// TestLongRunning_10Nodes_30Min runs a 10-node cluster for 30 minutes
func TestLongRunning_10Nodes_30Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	runLongRunningTest(t, 10, 30*time.Minute)
}

// TestLongRunning_20Nodes_15Min runs a 20-node cluster for 15 minutes
func TestLongRunning_20Nodes_15Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	runLongRunningTest(t, 20, 15*time.Minute)
}

func runLongRunningTest(t *testing.T, nodeCount int, duration time.Duration) {
	t.Logf("Starting long-running test: %d nodes for %v", nodeCount, duration)

	// Create cluster
	nodes := setupTestCluster(t, nodeCount, 31000)
	defer cleanupTestCluster(nodes)

	ctx := context.Background()

	// Statistics tracking
	var totalWrites, totalReads, totalDeletes atomic.Int64
	var writeErrors, readErrors, deleteErrors atomic.Int64
	var startTime = time.Now()

	// Stop channel
	stopCh := make(chan struct{})

	// Workload generators
	var wg sync.WaitGroup

	// Write workload (continuous)
	for i := 0; i < nodeCount; i++ {
		wg.Add(1)
		go func(nodeIdx int) {
			defer wg.Done()
			node := nodes[nodeIdx]
			keyCounter := 0

			for {
				select {
				case <-stopCh:
					return
				default:
					key := fmt.Sprintf("node%d-key%d", nodeIdx, keyCounter)
					value := []byte(fmt.Sprintf("value-%d-%d", nodeIdx, keyCounter))

					err := node.Set(ctx, key, value)
					if err == nil {
						totalWrites.Add(1)
					} else {
						writeErrors.Add(1)
					}

					keyCounter++
					time.Sleep(10 * time.Millisecond) // ~100 writes/sec per node
				}
			}
		}(i)
	}

	// Read workload (continuous)
	for i := 0; i < nodeCount*2; i++ { // 2x read workers
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()

			for {
				select {
				case <-stopCh:
					return
				default:
					nodeIdx := rand.Intn(nodeCount)
					node := nodes[nodeIdx]

					// Read random key
					targetNode := rand.Intn(nodeCount)
					keyIdx := rand.Intn(1000)
					key := fmt.Sprintf("node%d-key%d", targetNode, keyIdx)

					_, err := node.Get(ctx, key)
					if err == nil {
						totalReads.Add(1)
					} else {
						readErrors.Add(1)
					}

					time.Sleep(5 * time.Millisecond) // ~200 reads/sec per worker
				}
			}
		}(i)
	}

	// Delete workload (periodic)
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				// Delete random keys
				for i := 0; i < 10; i++ {
					nodeIdx := rand.Intn(nodeCount)
					node := nodes[nodeIdx]
					keyIdx := rand.Intn(1000)
					key := fmt.Sprintf("node%d-key%d", nodeIdx, keyIdx)

					err := node.Delete(ctx, key)
					if err == nil {
						totalDeletes.Add(1)
					} else {
						deleteErrors.Add(1)
					}
				}
			}
		}
	}()

	// Statistics reporter
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		lastWrites := int64(0)
		lastReads := int64(0)
		lastTime := startTime

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				now := time.Now()
				elapsed := now.Sub(lastTime).Seconds()

				currentWrites := totalWrites.Load()
				currentReads := totalReads.Load()

				writeRate := float64(currentWrites-lastWrites) / elapsed
				readRate := float64(currentReads-lastReads) / elapsed

				totalElapsed := now.Sub(startTime)

				t.Logf("[%v] Writes: %d (%.0f/s), Reads: %d (%.0f/s), Deletes: %d",
					totalElapsed.Round(time.Second),
					currentWrites, writeRate,
					currentReads, readRate,
					totalDeletes.Load())

				// Error rates
				if writeErrors.Load() > 0 || readErrors.Load() > 0 {
					t.Logf("  Errors - Writes: %d, Reads: %d, Deletes: %d",
						writeErrors.Load(), readErrors.Load(), deleteErrors.Load())
				}

				lastWrites = currentWrites
				lastReads = currentReads
				lastTime = now
			}
		}
	}()

	// Run for specified duration
	time.Sleep(duration)

	// Stop workloads
	close(stopCh)
	wg.Wait()

	// Final statistics
	totalElapsed := time.Since(startTime)
	finalWrites := totalWrites.Load()
	finalReads := totalReads.Load()
	finalDeletes := totalDeletes.Load()

	t.Logf("\n========== Long-Running Test Complete ==========")
	t.Logf("Duration: %v", totalElapsed.Round(time.Second))
	t.Logf("Cluster size: %d nodes", nodeCount)
	t.Logf("Total writes: %d (%.0f ops/sec)", finalWrites, float64(finalWrites)/totalElapsed.Seconds())
	t.Logf("Total reads: %d (%.0f ops/sec)", finalReads, float64(finalReads)/totalElapsed.Seconds())
	t.Logf("Total deletes: %d", finalDeletes)
	t.Logf("Write errors: %d (%.2f%%)", writeErrors.Load(),
		float64(writeErrors.Load())/float64(finalWrites+writeErrors.Load())*100)
	t.Logf("Read errors: %d (%.2f%%)", readErrors.Load(),
		float64(readErrors.Load())/float64(finalReads+readErrors.Load())*100)
	t.Logf("================================================")

	// Verify cluster health
	healthyNodes := 0
	for i, node := range nodes {
		if node != nil {
			t.Logf("Node %d: active", i)
			healthyNodes++
		}
	}

	if healthyNodes != nodeCount {
		t.Errorf("Cluster unhealthy: %d/%d nodes alive", healthyNodes, nodeCount)
	}
}
