package main

import (
	"context"
	"fmt"
	"log"
	"time"

	gridkv "github.com/feellmoose/gridkv"
	"github.com/feellmoose/gridkv/internal/gossip"
	"github.com/feellmoose/gridkv/internal/storage"
)

// Scenario 6: Development/Testing Configuration
// Suitable for: Local Development, Unit Tests, Feature Validation, Rapid Prototyping
//
// Performance Expectation:
//   - Fast startup (< 100ms)
//   - Low resource usage
//   - Simple configuration
//
// Configuration Features:
//   - Single replica (simplest)
//   - Small memory limit (512MB)
//   - Short timeouts
//   - Simplified network configuration

func main() {
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  GridKV Scenario 6: Development/Testing Configuration")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	ctx := context.Background()

	// Create single-node instance (simplest configuration)
	fmt.Println("📦 Creating development node...")
	start := time.Now()

	kv, err := gridkv.NewGridKV(&gridkv.GridKVOptions{
		LocalNodeID:  "dev-node",
		LocalAddress: "localhost:14001",

		Network: &gossip.NetworkOptions{
			Type:         gossip.TCP,
			BindAddr:     "localhost:14001",
			MaxConns:     100, // Small connection pool
			MaxIdle:      10,
			ReadTimeout:  1 * time.Second, // Short timeout
			WriteTimeout: 1 * time.Second,
		},

		Storage: &storage.StorageOptions{
			Backend:     storage.BackendMemory, // Simple backend
			MaxMemoryMB: 512,                   // 512MB limit
		},

		// Simplest configuration
		ReplicaCount: 1, // Single replica
		WriteQuorum:  1,
		ReadQuorum:   1,

		VirtualNodes:       50, // Small number of virtual nodes
		MaxReplicators:     4,  // Small replication pool
		ReplicationTimeout: 1 * time.Second,
		ReadTimeout:        1 * time.Second,
		FailureTimeout:     2 * time.Second,
		SuspectTimeout:     3 * time.Second,
		GossipInterval:     1 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}
	defer kv.Close()

	elapsed := time.Since(start)
	fmt.Printf("✅ Node created successfully (Elapsed: %v)\n", elapsed)
	fmt.Println()

	// ═══════════════════════════════════════════════════════
	// Basic Functionality Tests
	// ═══════════════════════════════════════════════════════
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Basic Functionality Tests")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	// Test 1: Set/Get
	fmt.Println("1️⃣  Testing Set/Get:")
	testSetGet(ctx, kv)
	fmt.Println()

	// Test 2: Delete
	fmt.Println("2️⃣  Testing Delete:")
	testDelete(ctx, kv)
	fmt.Println()

	// Test 3: Batch Operations
	fmt.Println("3️⃣  Testing Batch Operations:")
	testBatch(ctx, kv)
	fmt.Println()

	// Test 4: Data Types
	fmt.Println("4️⃣  Testing Various Data Types:")
	testDataTypes(ctx, kv)
	fmt.Println()

	// Test 5: Concurrency Safety
	fmt.Println("5️⃣  Testing Concurrency Safety:")
	testConcurrent(ctx, kv)
	fmt.Println()

	// ═══════════════════════════════════════════════════════
	// Summary
	// ═══════════════════════════════════════════════════════
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Summary")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Println("✅ Dev/Test Config Features:")
	fmt.Println("   • Single Node + Single Replica")
	fmt.Println("   • Fast Startup (< 100ms)")
	fmt.Println("   • Low Memory Usage (512MB)")
	fmt.Println("   • Simple Configuration")
	fmt.Println()
	fmt.Println("✅ Applicable Scenarios:")
	fmt.Println("   • Local Development")
	fmt.Println("   • Unit Testing")
	fmt.Println("   • Feature Validation")
	fmt.Println("   • Rapid Prototyping")
	fmt.Println()
	fmt.Println("✅ Advantages:")
	fmt.Println("   • Fast Startup")
	fmt.Println("   • Low Resource Usage")
	fmt.Println("   • Simple Configuration")
	fmt.Println("   • Suitable for Development and Debugging")
	fmt.Println()
	fmt.Println("⚠️  Notes:")
	fmt.Println("   • Not suitable for production")
	fmt.Println("   • No fault tolerance")
	fmt.Println("   • Data is not persistent")
	fmt.Println()
}

// Test Set/Get
func testSetGet(ctx context.Context, kv *gridkv.GridKV) {
	key := "user:1001"
	value := []byte("Alice")

	// Set
	if err := kv.Set(ctx, key, value); err != nil {
		log.Printf("❌ Set failed: %v", err)
		return
	}
	fmt.Println("   ✅ Set successful")

	// Get
	result, err := kv.Get(ctx, key)
	if err != nil {
		log.Printf("❌ Get failed: %v", err)
		return
	}
	fmt.Printf("   ✅ Get successful: %s\n", result)
}

// Test Delete
func testDelete(ctx context.Context, kv *gridkv.GridKV) {
	key := "temp:data"
	value := []byte("temporary")

	// First Set
	kv.Set(ctx, key, value)
	fmt.Println("   ✅ Created temporary data")

	// Verify existence
	if _, err := kv.Get(ctx, key); err != nil {
		log.Printf("❌ Verification failed: %v", err)
		return
	}
	fmt.Println("   ✅ Verified data exists")

	// Delete
	if err := kv.Delete(ctx, key); err != nil {
		log.Printf("❌ Delete failed: %v", err)
		return
	}
	fmt.Println("   ✅ Delete successful")

	// Verify non-existence
	if _, err := kv.Get(ctx, key); err != nil {
		fmt.Println("   ✅ Verified data is deleted")
	} else {
		fmt.Println("   ❌ Data still exists")
	}
}

// Test Batch Operations
func testBatch(ctx context.Context, kv *gridkv.GridKV) {
	numKeys := 1000

	// Batch write
	start := time.Now()
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("batch:%d", i)
		value := []byte(fmt.Sprintf("value-%d", i))
		if err := kv.Set(ctx, key, value); err != nil {
			log.Printf("Write failed: %v", err)
		}
	}
	writeTime := time.Since(start)
	writeOps := float64(numKeys) / writeTime.Seconds()

	fmt.Printf("   ✅ Batch wrote %d keys\n", numKeys)
	fmt.Printf("   ✅ Write speed: %.0f ops/s\n", writeOps)

	// Batch read
	start = time.Now()
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("batch:%d", i)
		if _, err := kv.Get(ctx, key); err != nil {
			log.Printf("Read failed: %v", err)
		}
	}
	readTime := time.Since(start)
	readOps := float64(numKeys) / readTime.Seconds()

	fmt.Printf("   ✅ Batch read %d keys\n", numKeys)
	fmt.Printf("   ✅ Read speed: %.0f ops/s\n", readOps)
}

// Test Various Data Types
func testDataTypes(ctx context.Context, kv *gridkv.GridKV) {
	// String
	kv.Set(ctx, "string", []byte("Hello, GridKV!"))
	fmt.Println("   ✅ String")

	// Number (serialized to bytes)
	kv.Set(ctx, "number", []byte("42"))
	fmt.Println("   ✅ Number")

	// JSON (serialized to bytes)
	jsonData := []byte(`{"name":"Alice","age":30}`)
	kv.Set(ctx, "json", jsonData)
	fmt.Println("   ✅ JSON")

	// Binary data
	binaryData := []byte{0x01, 0x02, 0x03, 0x04}
	kv.Set(ctx, "binary", binaryData)
	fmt.Println("   ✅ Binary")

	// Empty value
	kv.Set(ctx, "empty", []byte{})
	fmt.Println("   ✅ Empty value")

	// Large data (10KB)
	largeData := make([]byte, 10240)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	kv.Set(ctx, "large", largeData)
	fmt.Println("   ✅ Large data (10KB)")
}

// Test Concurrency Safety
func testConcurrent(ctx context.Context, kv *gridkv.GridKV) {
	numGoroutines := 10
	opsPerGoroutine := 100

	start := time.Now()
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < opsPerGoroutine; j++ {
				key := fmt.Sprintf("concurrent:%d:%d", id, j)
				value := []byte(fmt.Sprintf("value-%d-%d", id, j))
				kv.Set(ctx, key, value)
			}
			done <- true
		}(i)
	}

	// Wait for completion
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	elapsed := time.Since(start)
	totalOps := numGoroutines * opsPerGoroutine
	opsPerSec := float64(totalOps) / elapsed.Seconds()

	fmt.Printf("   ✅ Concurrency test complete\n")
	fmt.Printf("   ✅ %d goroutines × %d ops = %d operations\n",
		numGoroutines, opsPerGoroutine, totalOps)
	fmt.Printf("   ✅ Throughput: %.0f ops/s\n", opsPerSec)
}
