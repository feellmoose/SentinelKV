package tests

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gridkv "github.com/feellmoose/gridkv"
	"github.com/feellmoose/gridkv/internal/gossip"
	"github.com/feellmoose/gridkv/internal/storage"
)

// TestResult stores the results of a single test run
type TestResult struct {
	TestID        int
	NumNodes      int
	Duration      time.Duration
	NumClients    int
	TotalOps      int64
	SuccessOps    int64
	ErrorOps      int64
	Writes        int64
	Reads         int64
	Deletes       int64
	AvgThroughput float64
	ErrorRate     float64
	PanicCount    int64
	StartTime     time.Time
	EndTime       time.Time
}

// Progressive test configurations (OPTIMIZED based on Test 1-2 results)
var progressiveTests = []struct {
	name       string
	numNodes   int
	duration   time.Duration
	numClients int
	basePort   int
}{
	{"Test01_5nodes_15min", 5, 15 * time.Minute, 100, 58000},   // Optimized: 100 clients
	{"Test02_7nodes_15min", 7, 15 * time.Minute, 140, 59000},   // Optimized: 140 clients
	{"Test03_10nodes_15min", 10, 15 * time.Minute, 200, 60000}, // Optimized: 200 clients
	{"Test04_10nodes_20min", 10, 20 * time.Minute, 200, 61000},
	{"Test05_15nodes_20min", 15, 20 * time.Minute, 300, 62000}, // Optimized: 300 clients
	{"Test06_15nodes_30min", 15, 30 * time.Minute, 300, 63000},
	{"Test07_20nodes_30min", 20, 30 * time.Minute, 400, 64000}, // Optimized: 400 clients
	{"Test08_20nodes_45min", 20, 45 * time.Minute, 400, 65000},
	{"Test09_25nodes_60min", 25, 60 * time.Minute, 500, 66000}, // Optimized: 500 clients
	{"Test10_30nodes_12h", 30, 12 * time.Hour, 600, 67000},     // Optimized: 600 clients
}

// RunProgressiveTest executes a single progressive test
func RunProgressiveTest(t *testing.T, testID int, config struct {
	name       string
	numNodes   int
	duration   time.Duration
	numClients int
	basePort   int
}) *TestResult {

	result := &TestResult{
		TestID:     testID,
		NumNodes:   config.numNodes,
		Duration:   config.duration,
		NumClients: config.numClients,
		StartTime:  time.Now(),
	}

	t.Logf("")
	t.Logf("╔══════════════════════════════════════════════════════════════╗")
	t.Logf("║  Test #%02d: %s", testID, config.name)
	t.Logf("╚══════════════════════════════════════════════════════════════╝")
	t.Logf("Nodes:      %d", config.numNodes)
	t.Logf("Duration:   %v", config.duration)
	t.Logf("Clients:    %d", config.numClients)
	t.Logf("Base Port:  %d", config.basePort)
	t.Log("")

	// Statistics
	var (
		totalWrites  atomic.Int64
		totalReads   atomic.Int64
		totalDeletes atomic.Int64
		writeErrors  atomic.Int64
		readErrors   atomic.Int64
		deleteErrors atomic.Int64
		panicCount   atomic.Int64
	)

	// Create cluster
	t.Logf("[%v] Creating %d-node cluster...", time.Since(result.StartTime).Round(time.Second), config.numNodes)
	nodes := createOptimizedCluster(t, config.numNodes, config.basePort)
	if nodes == nil {
		t.Error("Failed to create cluster")
		return result
	}

	defer func() {
		t.Log("Cleaning up cluster...")
		for i, node := range nodes {
			if node != nil {
				if err := node.Close(); err != nil {
					t.Logf("Warning: error closing node %d: %v", i, err)
				}
			}
		}
	}()

	t.Logf("[%v] ✅ Cluster ready", time.Since(result.StartTime).Round(time.Second))

	// Pre-populate
	ctx := context.Background()
	numKeys := config.numNodes * 1000 // 1000 keys per node

	t.Logf("[%v] Pre-populating %d keys...", time.Since(result.StartTime).Round(time.Second), numKeys)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := []byte(fmt.Sprintf("initial-value-%d", i))
		node := nodes[i%len(nodes)]
		_ = node.Set(ctx, key, value)
	}
	t.Logf("[%v] ✅ Data ready", time.Since(result.StartTime).Round(time.Second))
	t.Log("")

	// Start workload
	deadline := time.Now().Add(config.duration)
	workloadStart := time.Now()

	t.Logf("[%v] 🚀 Starting workload for %v...", time.Since(result.StartTime).Round(time.Second), config.duration)
	t.Log("")

	var clientWg sync.WaitGroup
	for c := 0; c < config.numClients; c++ {
		clientWg.Add(1)
		go func(clientID int) {
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
					t.Errorf("Client %d panicked: %v", clientID, r)
				}
				clientWg.Done()
			}()

			node := nodes[clientID%len(nodes)]

			for time.Now().Before(deadline) {
				keyID := rand.Intn(numKeys)
				key := fmt.Sprintf("key-%d", keyID)
				value := []byte(fmt.Sprintf("value-%d-%d", clientID, time.Now().UnixNano()))

				op := rand.Intn(10)

				switch {
				case op < 6: // 60% reads
					_, err := node.Get(ctx, key)
					if err == nil || err == storage.ErrItemNotFound {
						totalReads.Add(1)
					} else {
						readErrors.Add(1)
					}

				case op < 9: // 30% writes
					err := node.Set(ctx, key, value)
					if err == nil {
						totalWrites.Add(1)
					} else {
						writeErrors.Add(1)
					}

				default: // 10% deletes
					err := node.Delete(ctx, key)
					if err == nil || err == storage.ErrItemNotFound {
						totalDeletes.Add(1)
					} else {
						deleteErrors.Add(1)
					}
				}
			}
		}(c)
	}

	// Monitoring loop
	reportTicker := time.NewTicker(1 * time.Minute)
	defer reportTicker.Stop()

	reportNum := 0
	for {
		select {
		case <-reportTicker.C:
			reportNum++
			elapsed := time.Since(workloadStart)

			writes := totalWrites.Load()
			reads := totalReads.Load()
			deletes := totalDeletes.Load()
			total := writes + reads + deletes

			opsPerSec := float64(total) / elapsed.Seconds()

			t.Logf("[%v] Report #%d: %d ops (%.0f ops/s), Errors: W=%d R=%d D=%d, Panics: %d",
				elapsed.Round(time.Second),
				reportNum,
				total,
				opsPerSec,
				writeErrors.Load(),
				readErrors.Load(),
				deleteErrors.Load(),
				panicCount.Load())

		case <-time.After(config.duration + 5*time.Second):
			goto workloadDone
		}
	}

workloadDone:
	clientWg.Wait()
	result.EndTime = time.Now()

	// Collect final stats
	result.Writes = totalWrites.Load()
	result.Reads = totalReads.Load()
	result.Deletes = totalDeletes.Load()
	result.SuccessOps = result.Writes + result.Reads + result.Deletes

	errW := writeErrors.Load()
	errR := readErrors.Load()
	errD := deleteErrors.Load()
	result.ErrorOps = errW + errR + errD
	result.TotalOps = result.SuccessOps + result.ErrorOps

	actualDuration := result.EndTime.Sub(workloadStart)
	result.AvgThroughput = float64(result.SuccessOps) / actualDuration.Seconds()
	result.ErrorRate = float64(result.ErrorOps) / float64(result.TotalOps) * 100
	result.PanicCount = panicCount.Load()

	// Print final results
	t.Log("")
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("  Test #%02d FINAL RESULTS", testID)
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Duration:        %v", actualDuration.Round(time.Second))
	t.Logf("Total Ops:       %d", result.SuccessOps)
	t.Logf("Avg Throughput:  %.0f ops/sec", result.AvgThroughput)
	t.Logf("Error Rate:      %.3f%%", result.ErrorRate)
	t.Logf("Panics:          %d", result.PanicCount)
	t.Log("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Log("")

	return result
}

// createOptimizedCluster creates a cluster with optimized settings
func createOptimizedCluster(t *testing.T, numNodes, basePort int) []*gridkv.GridKV {
	nodes := make([]*gridkv.GridKV, numNodes)

	// Create seed node
	seed, err := gridkv.NewGridKV(&gridkv.GridKVOptions{
		LocalNodeID:  "node-0",
		LocalAddress: fmt.Sprintf("localhost:%d", basePort),
		Network: &gossip.NetworkOptions{
			Type:         gossip.TCP,
			BindAddr:     fmt.Sprintf("localhost:%d", basePort),
			MaxConns:     2000,
			MaxIdle:      200,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		Storage: &storage.StorageOptions{
			Backend:     storage.BackendMemorySharded,
			MaxMemoryMB: 8192, // 8GB per node
		},
		ReplicaCount:       1, // Single replica for max performance
		WriteQuorum:        1,
		ReadQuorum:         1,
		VirtualNodes:       150,
		MaxReplicators:     16,
		ReplicationTimeout: 3 * time.Second,
		ReadTimeout:        3 * time.Second,
	})
	if err != nil {
		t.Errorf("Failed to create seed: %v", err)
		return nil
	}
	nodes[0] = seed

	// Create other nodes in parallel
	var wg sync.WaitGroup
	seedAddr := []string{fmt.Sprintf("localhost:%d", basePort)}

	for i := 1; i < numNodes; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			node, err := gridkv.NewGridKV(&gridkv.GridKVOptions{
				LocalNodeID:  fmt.Sprintf("node-%d", idx),
				LocalAddress: fmt.Sprintf("localhost:%d", basePort+idx),
				SeedAddrs:    seedAddr,
				Network: &gossip.NetworkOptions{
					Type:         gossip.TCP,
					BindAddr:     fmt.Sprintf("localhost:%d", basePort+idx),
					MaxConns:     2000,
					MaxIdle:      200,
					ReadTimeout:  5 * time.Second,
					WriteTimeout: 5 * time.Second,
				},
				Storage: &storage.StorageOptions{
					Backend:     storage.BackendMemorySharded,
					MaxMemoryMB: 8192,
				},
				ReplicaCount:       1,
				WriteQuorum:        1,
				ReadQuorum:         1,
				VirtualNodes:       150,
				MaxReplicators:     16,
				ReplicationTimeout: 3 * time.Second,
				ReadTimeout:        3 * time.Second,
			})

			if err != nil {
				t.Errorf("Failed to create node %d: %v", idx, err)
				return
			}

			nodes[idx] = node
		}(i)
	}

	wg.Wait()

	// Wait for convergence
	time.Sleep(time.Duration(numNodes) * 200 * time.Millisecond)

	return nodes
}

// TestProgressive01_5Nodes_15Min - Test 1: 5 nodes, 15 minutes
func TestProgressive01_5Nodes_15Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-run test in short mode")
	}

	result := RunProgressiveTest(t, 1, progressiveTests[0])
	saveTestResult(t, result)

	// Validate results
	if result.ErrorRate > 5.0 {
		t.Errorf("High error rate: %.2f%%", result.ErrorRate)
	}
	if result.PanicCount > 0 {
		t.Errorf("Found %d panics", result.PanicCount)
	}
	if result.AvgThroughput < 100000 {
		t.Logf("⚠️  Lower than expected throughput: %.0f ops/s", result.AvgThroughput)
	}
}

// TestProgressive02_7Nodes_15Min - Test 2: 7 nodes, 15 minutes
func TestProgressive02_7Nodes_15Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-run test in short mode")
	}

	result := RunProgressiveTest(t, 2, progressiveTests[1])
	saveTestResult(t, result)
}

// TestProgressive03_10Nodes_15Min - Test 3: 10 nodes, 15 minutes
func TestProgressive03_10Nodes_15Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-run test in short mode")
	}

	result := RunProgressiveTest(t, 3, progressiveTests[2])
	saveTestResult(t, result)
}

// TestProgressive04_10Nodes_20Min - Test 4: 10 nodes, 20 minutes
func TestProgressive04_10Nodes_20Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-run test in short mode")
	}

	result := RunProgressiveTest(t, 4, progressiveTests[3])
	saveTestResult(t, result)
}

// TestProgressive05_15Nodes_20Min - Test 5: 15 nodes, 20 minutes
func TestProgressive05_15Nodes_20Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-run test in short mode")
	}

	result := RunProgressiveTest(t, 5, progressiveTests[4])
	saveTestResult(t, result)
}

// TestProgressive06_15Nodes_30Min - Test 6: 15 nodes, 30 minutes
func TestProgressive06_15Nodes_30Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-run test in short mode")
	}

	result := RunProgressiveTest(t, 6, progressiveTests[5])
	saveTestResult(t, result)
}

// TestProgressive07_20Nodes_30Min - Test 7: 20 nodes, 30 minutes
func TestProgressive07_20Nodes_30Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-run test in short mode")
	}

	result := RunProgressiveTest(t, 7, progressiveTests[6])
	saveTestResult(t, result)
}

// TestProgressive08_20Nodes_45Min - Test 8: 20 nodes, 45 minutes
func TestProgressive08_20Nodes_45Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-run test in short mode")
	}

	result := RunProgressiveTest(t, 8, progressiveTests[7])
	saveTestResult(t, result)
}

// TestProgressive09_25Nodes_60Min - Test 9: 25 nodes, 60 minutes
func TestProgressive09_25Nodes_60Min(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-run test in short mode")
	}

	result := RunProgressiveTest(t, 9, progressiveTests[8])
	saveTestResult(t, result)
}

// TestProgressive10_30Nodes_12H - Test 10: 30 nodes, 12 hours (FINAL TEST)
func TestProgressive10_30Nodes_12H(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 12-hour test in short mode")
	}

	t.Log("")
	t.Log("╔══════════════════════════════════════════════════════════════╗")
	t.Log("║                                                              ║")
	t.Log("║        🔥 FINAL TEST: 30 Nodes, 12 Hours 🔥                  ║")
	t.Log("║                                                              ║")
	t.Log("╚══════════════════════════════════════════════════════════════╝")
	t.Log("")

	result := RunProgressiveTest(t, 10, progressiveTests[9])
	saveTestResult(t, result)

	// Special validation for 12-hour test
	if result.ErrorRate > 1.0 {
		t.Errorf("12-hour test has high error rate: %.3f%%", result.ErrorRate)
	} else {
		t.Logf("✅ Excellent error rate for 12-hour run: %.3f%%", result.ErrorRate)
	}

	if result.PanicCount > 0 {
		t.Errorf("❌ Found %d panics in 12-hour test", result.PanicCount)
	} else {
		t.Log("✅ No panics in 12-hour continuous operation!")
	}

	if result.SuccessOps > 1000000000 { // 1 billion ops
		t.Logf("🏆 MILESTONE: Over 1 billion operations! (%d)", result.SuccessOps)
	}
}

// saveTestResult saves test results to file
func saveTestResult(t *testing.T, result *TestResult) {
	filename := fmt.Sprintf("/home/feellmoose/Documents/workspace/GridKV/test_results/test_%02d_%s.txt",
		result.TestID, time.Now().Format("20060102_150405"))

	// Create directory if not exists
	os.MkdirAll("/home/feellmoose/Documents/workspace/GridKV/test_results", 0755)

	f, err := os.Create(filename)
	if err != nil {
		t.Logf("Warning: failed to save results: %v", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "GridKV Progressive Test Result\n")
	fmt.Fprintf(f, "================================\n\n")
	fmt.Fprintf(f, "Test ID:         %d\n", result.TestID)
	fmt.Fprintf(f, "Nodes:           %d\n", result.NumNodes)
	fmt.Fprintf(f, "Duration:        %v\n", result.Duration)
	fmt.Fprintf(f, "Clients:         %d\n", result.NumClients)
	fmt.Fprintf(f, "Start Time:      %v\n", result.StartTime.Format(time.RFC3339))
	fmt.Fprintf(f, "End Time:        %v\n", result.EndTime.Format(time.RFC3339))
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "Operations:\n")
	fmt.Fprintf(f, "  Total:         %d\n", result.TotalOps)
	fmt.Fprintf(f, "  Success:       %d\n", result.SuccessOps)
	fmt.Fprintf(f, "  Errors:        %d\n", result.ErrorOps)
	fmt.Fprintf(f, "  Writes:        %d\n", result.Writes)
	fmt.Fprintf(f, "  Reads:         %d\n", result.Reads)
	fmt.Fprintf(f, "  Deletes:       %d\n", result.Deletes)
	fmt.Fprintf(f, "\n")
	fmt.Fprintf(f, "Performance:\n")
	fmt.Fprintf(f, "  Throughput:    %.0f ops/sec\n", result.AvgThroughput)
	fmt.Fprintf(f, "  Error Rate:    %.3f%%\n", result.ErrorRate)
	fmt.Fprintf(f, "  Panics:        %d\n", result.PanicCount)
	fmt.Fprintf(f, "\n")

	t.Logf("📝 Results saved to: %s", filename)
}
