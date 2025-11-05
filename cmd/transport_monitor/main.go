package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/feellmoose/gridkv/internal/transport"
)

// Transport Monitor Dashboard
// Real-time monitoring of transport performance with zero overhead metrics

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║        GridKV Transport Monitor Dashboard                         ║")
	fmt.Println("║        Real-time Performance Monitoring                            ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Create gnet transport with metrics
	gnetTrans, err := transport.NewGnetTransport()
	if err != nil {
		fmt.Printf("Failed to create gnet transport: %v\n", err)
		return
	}

	fmt.Println("Starting gnet listener on localhost:8000...")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Start listener
	listener, err := gnetTrans.Listen("localhost:8000")
	if err != nil {
		fmt.Printf("Failed to start listener: %v\n", err)
		return
	}

	listener.HandleMessage(func(msg []byte) error {
		// Echo handler
		return nil
	})

	// Start in background
	go func() {
		if err := listener.Start(); err != nil {
			fmt.Printf("Listener error: %v\n", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	// Monitor loop
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	startTime := time.Now()
	lastMetrics := transport.MetricsSnapshot{}

	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    REAL-TIME METRICS                               ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	for {
		select {
		case <-sigCh:
			fmt.Println("\n\nShutting down...")
			listener.Stop()
			return

		case <-ticker.C:
			clearScreen()
			printDashboard(gnetTrans, startTime, lastMetrics)
			
			// Update last metrics for rate calculation
			lastMetrics = gnetTrans.GetMetrics()
		}
	}
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func printDashboard(trans *transport.GnetTransport, startTime time.Time, lastMetrics transport.MetricsSnapshot) {
	metrics := trans.GetMetrics()
	uptime := metrics.Uptime

	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              gnet Transport - Real-time Dashboard                  ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Printf("  Uptime: %v\n", uptime.Round(time.Second))
	fmt.Printf("  Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println()

	fmt.Println("📊 CONNECTION METRICS")
	fmt.Println("────────────────────────────────────────────────────────────────────")
	fmt.Printf("  Active Connections:   %6d\n", metrics.ActiveConnections)
	fmt.Printf("  Total Connections:    %6d\n", metrics.TotalConnections)
	fmt.Printf("  Failed Connections:   %6d\n", metrics.FailedConnections)
	
	if metrics.TotalConnections > 0 {
		successRate := float64(metrics.TotalConnections-metrics.FailedConnections) / float64(metrics.TotalConnections) * 100
		fmt.Printf("  Success Rate:         %6.2f%%\n", successRate)
	}
	fmt.Println()

	fmt.Println("📨 MESSAGE METRICS")
	fmt.Println("────────────────────────────────────────────────────────────────────")
	fmt.Printf("  Messages Sent:        %10d\n", metrics.MessagesSent)
	fmt.Printf("  Messages Received:    %10d\n", metrics.MessagesReceived)
	fmt.Printf("  Bytes Sent:           %10d (%.2f MB)\n", metrics.BytesSent, float64(metrics.BytesSent)/1024/1024)
	fmt.Printf("  Bytes Received:       %10d (%.2f MB)\n", metrics.BytesReceived, float64(metrics.BytesReceived)/1024/1024)
	fmt.Println()

	fmt.Println("⚡ PERFORMANCE METRICS")
	fmt.Println("────────────────────────────────────────────────────────────────────")
	fmt.Printf("  Messages/sec:         %10.2f K\n", metrics.MessagesPerSec/1000)
	fmt.Printf("  Bandwidth:            %10.2f MB/s\n", metrics.MBPerSec)
	fmt.Printf("  Last Latency:         %10d µs\n", metrics.LastLatencyUs)
	fmt.Printf("  Avg Latency:          %10d µs\n", metrics.AvgLatencyUs)
	fmt.Println()

	fmt.Println("❌ ERROR METRICS")
	fmt.Println("────────────────────────────────────────────────────────────────────")
	fmt.Printf("  Write Errors:         %6d\n", metrics.WriteErrors)
	fmt.Printf("  Read Errors:          %6d\n", metrics.ReadErrors)
	fmt.Printf("  Handler Errors:       %6d\n", metrics.HandlerErrors)
	
	totalErrors := metrics.WriteErrors + metrics.ReadErrors + metrics.HandlerErrors
	totalOps := metrics.MessagesSent + metrics.MessagesReceived
	if totalOps > 0 {
		errorRate := float64(totalErrors) / float64(totalOps) * 100
		fmt.Printf("  Error Rate:           %6.4f%%\n", errorRate)
	}
	fmt.Println()

	fmt.Println("📈 RATE METRICS (since last update)")
	fmt.Println("────────────────────────────────────────────────────────────────────")
	if lastMetrics.MessagesSent > 0 {
		deltaSent := metrics.MessagesSent - lastMetrics.MessagesSent
		deltaBytes := metrics.BytesSent - lastMetrics.BytesSent
		deltaTime := 2.0  // 2 second update interval
		
		fmt.Printf("  Messages/sec:         %10.2f K\n", float64(deltaSent)/deltaTime/1000)
		fmt.Printf("  Bandwidth:            %10.2f MB/s\n", float64(deltaBytes)/deltaTime/1024/1024)
	} else {
		fmt.Printf("  Waiting for traffic...\n")
	}
	fmt.Println()

	fmt.Println("Press Ctrl+C to stop monitoring")
}

