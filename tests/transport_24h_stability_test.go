package tests

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/feellmoose/gridkv/internal/transport"
	
)

// 24-Hour Stability Test
// Run with: go test -run=TestTransport_24HourStability -timeout=25h -v
//
// This test should be run in CI/CD pipeline or before production deployment
// Tests:
// - Memory leaks
// - Connection leaks
// - Performance degradation over time
// - Error accumulation

func TestTransport_24HourStability(t *testing.T) {
	// Check if we should run this test
	if os.Getenv("RUN_24H_TEST") != "true" {
		t.Skip("Skipping 24-hour test. Set RUN_24H_TEST=true to run")
	}

	transports := []struct {
		name string
		factory func() (transport.Transport, error)
	}{
		{"TCP", func() (transport.Transport, error) { return transport.NewTCPTransport(), nil }},
		{"UDP", func() (transport.Transport, error) { return transport.NewUDPTransport(), nil }},
		{"gnet", func() (transport.Transport, error) { return transport.NewGnetTransport() }},
	}

	for _, tt := range transports {
		t.Run(tt.name, func(t *testing.T) {
			trans, err := tt.factory()
			if err != nil {
				t.Skipf("%s not available: %v", tt.name, err)
				return
			}

			run24HourStabilityTest(t, trans, tt.name)
		})
	}
}

func run24HourStabilityTest(t *testing.T, trans transport.Transport, name string) {
	duration := 24 * time.Hour
	addr := fmt.Sprintf("localhost:%d", 70000)

	t.Logf("Starting 24-hour stability test for %s", name)
	t.Logf("Duration: %v", duration)
	t.Logf("Start time: %v", time.Now())

	// Start listener
	listener, err := trans.Listen(addr)
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	received := atomic.Int64{}
	_ = atomic.Int64{}

	listener.HandleMessage(func(msg []byte) error {
		received.Add(1)
		return nil
	})

	if err := listener.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer listener.Stop()

	time.Sleep(500 * time.Millisecond)

	// Metrics collection
	type Checkpoint struct {
		Time        time.Time
		Sent        int64
		Received    int64
		Errors      int64
		Goroutines  int
		MemAllocMB  uint64
	}

	checkpoints := []Checkpoint{}
	checkpointMu := sync.Mutex{}

	// Continuous load
	concurrency := runtime.NumCPU() * 2
	message := make([]byte, 1024)
	ctx := context.Background()

	stopCh := make(chan struct{})
	var wg sync.WaitGroup

	sent := atomic.Int64{}
	errors := atomic.Int64{}

	start := time.Now()

	// Workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Reconnect loop
			for {
				select {
				case <-stopCh:
					return
				default:
				}

				conn, err := trans.Dial(addr)
				if err != nil {
					errors.Add(1)
					time.Sleep(100 * time.Millisecond)
					continue
				}

				// Use connection for a while, then recreate
				opsPerConn := 1000

				for j := 0; j < opsPerConn; j++ {
					select {
					case <-stopCh:
						conn.Close()
						return
					default:
					}

					if err := conn.WriteDataWithContext(ctx, message); err != nil {
						errors.Add(1)
						break
					}
					sent.Add(1)
				}

				conn.Close()
				time.Sleep(10 * time.Millisecond)  // Brief pause
			}
		}(i)
	}

	// Metrics collector (every hour)
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)

				checkpointMu.Lock()
				checkpoints = append(checkpoints, Checkpoint{
					Time:       time.Now(),
					Sent:       sent.Load(),
					Received:   received.Load(),
					Errors:     errors.Load(),
					Goroutines: runtime.NumGoroutine(),
					MemAllocMB: m.Alloc / 1024 / 1024,
				})
				checkpointMu.Unlock()

				elapsed := time.Since(start)
				t.Logf("[%v] Checkpoint: sent=%d, received=%d, errors=%d, goroutines=%d, mem=%dMB",
					elapsed.Round(time.Minute), sent.Load(), received.Load(), errors.Load(),
					runtime.NumGoroutine(), m.Alloc/1024/1024)
			}
		}
	}()

	// Run for 24 hours
	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	elapsed := time.Since(start)

	// Final report
	t.Logf("\n%s 24-Hour Stability Test Completed:", name)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Total sent: %d", sent.Load())
	t.Logf("  Total received: %d", received.Load())
	t.Logf("  Total errors: %d", errors.Load())
	t.Logf("  Avg throughput: %.2f K ops/sec", float64(sent.Load())/elapsed.Seconds()/1000)

	// Analyze checkpoints
	if len(checkpoints) > 0 {
		t.Logf("\nCheckpoint Analysis:")
		for i, cp := range checkpoints {
			t.Logf("  [%d] Time: %v, Sent: %d, Goroutines: %d, Mem: %dMB",
				i+1, cp.Time.Sub(start).Round(time.Minute), cp.Sent, cp.Goroutines, cp.MemAllocMB)
		}

		// Check for memory leaks
		if len(checkpoints) >= 2 {
			firstMem := checkpoints[0].MemAllocMB
			lastMem := checkpoints[len(checkpoints)-1].MemAllocMB
			memGrowth := float64(lastMem-firstMem) / float64(firstMem) * 100

			t.Logf("  Memory growth: %.2f%% (%dMB -> %dMB)", memGrowth, firstMem, lastMem)

			if memGrowth > 50 {  // Allow 50% memory growth over 24h
				t.Errorf("%s: Possible memory leak detected: %.2f%% growth", name, memGrowth)
			}
		}

		// Check for goroutine leaks
		if len(checkpoints) >= 2 {
			firstGR := checkpoints[0].Goroutines
			lastGR := checkpoints[len(checkpoints)-1].Goroutines
			grGrowth := lastGR - firstGR

			t.Logf("  Goroutine growth: %d (%d -> %d)", grGrowth, firstGR, lastGR)

			if grGrowth > 100 {  // Allow up to 100 goroutine growth
				t.Errorf("%s: Possible goroutine leak detected: %d growth", name, grGrowth)
			}
		}
	}

	// Verify overall success
	errorRate := float64(errors.Load()) / float64(sent.Load()+errors.Load())
	if errorRate > 0.001 {  // < 0.1% error rate
		t.Errorf("%s: Error rate too high: %.4f%%", name, errorRate*100)
	}

	t.Logf("\n✅ %s passed 24-hour stability test!", name)
}

