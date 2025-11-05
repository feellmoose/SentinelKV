package tests

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gridkv "github.com/feellmoose/gridkv"
	"github.com/feellmoose/gridkv/internal/gossip"
	"github.com/feellmoose/gridkv/internal/storage"
)

// ============================================================================
// Distributed Cluster Benchmarks
// ============================================================================

func setupCluster(b *testing.B, numNodes int, basePort int) ([]*gridkv.GridKV, error) {
	nodes := make([]*gridkv.GridKV, numNodes)
	var seedAddrs []string

	// Create all nodes
	for i := 0; i < numNodes; i++ {
		nodeID := fmt.Sprintf("node-%d", i+1)
		address := fmt.Sprintf("localhost:%d", basePort+i)

		if i > 0 {
			seedAddrs = []string{fmt.Sprintf("localhost:%d", basePort)}
		}

		opts := &gridkv.GridKVOptions{
			LocalNodeID:  nodeID,
			LocalAddress: address,
			SeedAddrs:    seedAddrs,
			Network: &gossip.NetworkOptions{
				Type:         gossip.TCP,
				BindAddr:     address,
				MaxConns:     2000,
				MaxIdle:      200,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			},
			Storage: &storage.StorageOptions{
				Backend:     storage.BackendMemorySharded,
				MaxMemoryMB: 4096, // 4GB limit
			},
			ReplicaCount:       3,
			WriteQuorum:        2,
			ReadQuorum:         2,
			VirtualNodes:       150,
			MaxReplicators:     8,
			ReplicationTimeout: 2 * time.Second,
			ReadTimeout:        2 * time.Second,
			FailureTimeout:     5 * time.Second,
			SuspectTimeout:     10 * time.Second,
			GossipInterval:     1 * time.Second,
			TTL:                0,
		}

		node, err := gridkv.NewGridKV(opts)
		if err != nil {
			// Cleanup already created nodes
			for j := 0; j < i; j++ {
				nodes[j].Close()
			}
			return nil, err
		}
		nodes[i] = node
	}

	// Wait for cluster to stabilize
	time.Sleep(3 * time.Second)

	return nodes, nil
}

func teardownCluster(nodes []*gridkv.GridKV) {
	for _, node := range nodes {
		if node != nil {
			node.Close()
		}
	}
}

// BenchmarkCluster3Nodes tests 3-node cluster performance
func BenchmarkCluster3Nodes(b *testing.B) {
	nodes, err := setupCluster(b, 3, 20000)
	if err != nil {
		b.Fatal(err)
	}
	defer teardownCluster(nodes)

	ctx := context.Background()
	value := randomValue(100)

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key-%d", i)
			node := nodes[i%len(nodes)]
			if err := node.Set(ctx, key, value); err != nil {
				b.Fatal(err)
			}
		}

		b.StopTimer()
		ops := float64(b.N) / b.Elapsed().Seconds()
		b.ReportMetric(ops, "ops/sec")
	})

	b.Run("Get", func(b *testing.B) {
		// Pre-populate data
		numKeys := 10000
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("key-%d", i)
			node := nodes[i%len(nodes)]
			if err := node.Set(ctx, key, value); err != nil {
				b.Fatal(err)
			}
		}

		time.Sleep(2 * time.Second) // Wait for replication

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key-%d", i%numKeys)
			node := nodes[i%len(nodes)]
			if _, err := node.Get(ctx, key); err != nil {
				b.Fatal(err)
			}
		}

		b.StopTimer()
		ops := float64(b.N) / b.Elapsed().Seconds()
		b.ReportMetric(ops, "ops/sec")
	})

	b.Run("MixedWorkload", func(b *testing.B) {
		var wg sync.WaitGroup
		numGoroutines := 30

		b.ResetTimer()
		b.ReportAllocs()

		opsPerGoroutine := b.N / numGoroutines

		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(gid int) {
				defer wg.Done()
				node := nodes[gid%len(nodes)]

				for i := 0; i < opsPerGoroutine; i++ {
					key := fmt.Sprintf("key-%d", i%10000)

					// 80% reads, 20% writes
					if i%5 == 0 {
						if err := node.Set(ctx, key, value); err != nil {
							b.Error(err)
							return
						}
					} else {
						if _, err := node.Get(ctx, key); err != nil {
							b.Error(err)
							return
						}
					}
				}
			}(g)
		}

		wg.Wait()
		b.StopTimer()

		ops := float64(b.N) / b.Elapsed().Seconds()
		b.ReportMetric(ops, "ops/sec")
	})
}

// BenchmarkCluster5Nodes tests 5-node cluster performance
func BenchmarkCluster5Nodes(b *testing.B) {
	nodes, err := setupCluster(b, 5, 20100)
	if err != nil {
		b.Fatal(err)
	}
	defer teardownCluster(nodes)

	ctx := context.Background()
	value := randomValue(100)

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key-%d", i)
			node := nodes[i%len(nodes)]
			if err := node.Set(ctx, key, value); err != nil {
				b.Fatal(err)
			}
		}

		b.StopTimer()
		ops := float64(b.N) / b.Elapsed().Seconds()
		b.ReportMetric(ops, "ops/sec")
	})

	b.Run("Get", func(b *testing.B) {
		// Pre-populate data
		numKeys := 10000
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("key-%d", i)
			node := nodes[i%len(nodes)]
			if err := node.Set(ctx, key, value); err != nil {
				b.Fatal(err)
			}
		}

		time.Sleep(2 * time.Second)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key-%d", i%numKeys)
			node := nodes[i%len(nodes)]
			if _, err := node.Get(ctx, key); err != nil {
				b.Fatal(err)
			}
		}

		b.StopTimer()
		ops := float64(b.N) / b.Elapsed().Seconds()
		b.ReportMetric(ops, "ops/sec")
	})
}

// BenchmarkClusterScaling tests cluster performance at different scales
func BenchmarkClusterScaling(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping scaling test in short mode")
	}

	nodeCounts := []int{3, 5, 10}

	for _, nodeCount := range nodeCounts {
		b.Run(fmt.Sprintf("%dNodes", nodeCount), func(b *testing.B) {
			nodes, err := setupCluster(b, nodeCount, 25000+nodeCount*100)
			if err != nil {
				b.Fatal(err)
			}
			defer teardownCluster(nodes)

			ctx := context.Background()
			value := randomValue(100)

			// Wait for cluster convergence
			time.Sleep(time.Duration(nodeCount) * 200 * time.Millisecond)

			b.Run("ParallelWrites", func(b *testing.B) {
				var successCount atomic.Int64
				var errorCount atomic.Int64

				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					nodeIdx := 0
					keyIdx := 0

					for pb.Next() {
						node := nodes[nodeIdx%len(nodes)]
						key := fmt.Sprintf("key-%d-%d", nodeIdx, keyIdx)

						err := node.Set(ctx, key, value)
						if err == nil {
							successCount.Add(1)
						} else {
							errorCount.Add(1)
						}

						nodeIdx++
						keyIdx++
					}
				})

				b.StopTimer()
				total := successCount.Load() + errorCount.Load()
				successRate := float64(successCount.Load()) / float64(total) * 100
				b.ReportMetric(float64(successCount.Load())/b.Elapsed().Seconds(), "ops/sec")
				b.ReportMetric(successRate, "success_%")
				b.Logf("Success rate: %.2f%% (%d/%d)", successRate, successCount.Load(), total)
			})

			b.Run("ParallelReads", func(b *testing.B) {
				// Pre-populate data
				for i := 0; i < 1000; i++ {
					key := fmt.Sprintf("read-key-%d", i)
					nodes[0].Set(ctx, key, value)
				}
				time.Sleep(500 * time.Millisecond)

				var successCount atomic.Int64
				var errorCount atomic.Int64

				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					nodeIdx := 0
					keyIdx := 0

					for pb.Next() {
						node := nodes[nodeIdx%len(nodes)]
						key := fmt.Sprintf("read-key-%d", keyIdx%1000)

						_, err := node.Get(ctx, key)
						if err == nil {
							successCount.Add(1)
						} else {
							errorCount.Add(1)
						}

						nodeIdx++
						keyIdx++
					}
				})

				b.StopTimer()
				total := successCount.Load() + errorCount.Load()
				successRate := float64(successCount.Load()) / float64(total) * 100
				b.ReportMetric(float64(successCount.Load())/b.Elapsed().Seconds(), "ops/sec")
				b.ReportMetric(successRate, "success_%")
			})
		})
	}
}

// BenchmarkClusterThroughput measures aggregate cluster throughput
func BenchmarkClusterThroughput(b *testing.B) {
	nodes, err := setupCluster(b, 3, 20200)
	if err != nil {
		b.Fatal(err)
	}
	defer teardownCluster(nodes)

	ctx := context.Background()
	value := randomValue(100)
	numClients := 100

	// Pre-populate data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key-%d", i)
		node := nodes[i%len(nodes)]
		if err := node.Set(ctx, key, value); err != nil {
			b.Fatal(err)
		}
	}

	time.Sleep(2 * time.Second)

	b.ResetTimer()
	b.ReportAllocs()

	var wg sync.WaitGroup
	opsPerClient := b.N / numClients

	startTime := time.Now()

	for c := 0; c < numClients; c++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			node := nodes[clientID%len(nodes)]

			for i := 0; i < opsPerClient; i++ {
				key := fmt.Sprintf("key-%d", i%numKeys)

				// 70% reads, 30% writes
				if i%10 < 7 {
					if _, err := node.Get(ctx, key); err != nil {
						b.Error(err)
						return
					}
				} else {
					if err := node.Set(ctx, key, value); err != nil {
						b.Error(err)
						return
					}
				}
			}
		}(c)
	}

	wg.Wait()
	elapsed := time.Since(startTime)
	b.StopTimer()

	totalOps := float64(b.N)
	throughput := totalOps / elapsed.Seconds()
	b.ReportMetric(throughput, "total_ops/sec")
	b.ReportMetric(throughput/float64(len(nodes)), "ops/sec/node")
}
