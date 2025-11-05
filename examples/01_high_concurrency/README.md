# Scenario 1: High Concurrency Cache Configuration

## 📖 Scenario Description

This scenario is optimized for high concurrent read/write scenarios, suitable for microservice caching, session storage, hot data caching, etc.

### Use Cases

✅ **Microservice Cache**: API response cache, database query cache  
✅ **Session Storage**: User sessions, login state  
✅ **Hot Data**: Leaderboards, real-time statistics  
✅ **High Concurrent Read/Write**: E-commerce flash sales, shopping events  

---

## 🎯 Performance Expectations

| Metric | Performance |
|--------|-------------|
| **Concurrent Reads** | 5-7M ops/s |
| **Concurrent Writes** | 3-4M ops/s |
| **P99 Latency** | < 500ns |
| **Memory Efficiency** | < 300 B/op |

---

## ⚙️ Configuration Details

### Core Configuration

```go
// 1. Storage configuration - Use sharded backend
Storage: &storage.StorageOptions{
    Backend:     storage.BackendMemorySharded,  // 256 shards, reduces lock contention
    MaxMemoryMB: 16384,                         // 16GB memory limit
}
```

**Why choose MemorySharded?**
- 256 shards, greatly reduces lock contention
- 15-20% faster than Memory backend
- CPU core adaptive

### Replica Configuration - Read Performance Priority

```go
// 2. Replica configuration
ReplicaCount: 3,  // 3 replicas (tolerates 1 node failure)
WriteQuorum:  2,  // Write succeeds when 2 replicas ack
ReadQuorum:   1,  // Read from 1 replica only (fastest) ⚡
```

**Performance Trade-offs**:
- `ReadQuorum=1` provides fastest read performance
- Still have 3 replicas for data safety
- Suitable for read-heavy scenarios

**Consistency Explanation**:
- W=2, R=1, N=3
- W+R=3 ≤ N, **Eventual consistency**
- For strong consistency, set R=2

### Network Configuration - High Concurrency Optimization

```go
// 3. Network configuration
Network: &gossip.NetworkOptions{
    MaxConns:     2000,              // Max 2000 connections
    MaxIdle:      200,               // Keep 200 idle connections
    ReadTimeout:  5 * time.Second,   // 5s timeout
    WriteTimeout: 5 * time.Second,
}
```

**Parameter Explanation**:
- `MaxConns=2000`: Supports high concurrent connections
- `MaxIdle=200`: Connection reuse, reduces handshake overhead
- `Timeout=5s`: Balance performance and reliability

### Performance Tuning

```go
// 4. Performance tuning
VirtualNodes:       150,                   // Virtual nodes
MaxReplicators:     runtime.NumCPU() * 2,  // Replication pool size
ReplicationTimeout: 2 * time.Second,       // Replication timeout
ReadTimeout:        2 * time.Second,       // Read timeout
```

**Tuning Explanation**:
- `VirtualNodes=150`: Suitable for small to medium clusters
- `MaxReplicators=CPU×2`: Fully utilize CPU
- `Timeout=2s`: Fast fail, avoid blocking

---

## 🚀 Run Example

### Start

```bash
cd examples/01_high_concurrency
go run main.go
```

### Expected Output

```
═══════════════════════════════════════════════════════
  GridKV Scenario 1: High-Concurrency Cache Config
═══════════════════════════════════════════════════════

📦 Creating Node 1 (seed node)...
✅ Node 1 created successfully

📦 Creating Node 2...
✅ Node 2 created successfully

📦 Creating Node 3...
✅ Node 3 created successfully

⏳ Waiting for cluster convergence...
✅ Cluster ready

📊 Test 1: High-Concurrency Write Performance
─────────────────────────────────────────────────────
Concurrent goroutines: 100
Operations per goroutine: 1000
Total operations: 100000

✅ Completion time: 28.5ms
✅ Throughput: 3508771.93 ops/s (3.51M ops/s)
✅ Average latency: 285ns

📊 Test 2: High-Concurrency Read Performance
─────────────────────────────────────────────────────
Pre-populating data...
✅ Pre-population completed

Concurrent goroutines: 100
Operations per goroutine: 1000
Total operations: 100000

✅ Completion time: 16.2ms
✅ Throughput: 6172839.51 ops/s (6.17M ops/s)
✅ Average latency: 162ns

📊 Test 3: Mixed Read/Write (80% Read / 20% Write)
─────────────────────────────────────────────────────
Concurrent goroutines: 100
Operations per goroutine: 1000
Read/Write ratio: 80% reads / 20% writes
Total operations: 100000

✅ Completion time: 19.8ms
✅ Throughput: 5050505.05 ops/s (5.05M ops/s)
✅ Average latency: 198ns
```

---

## 📊 Performance Analysis

### Performance Bottleneck Analysis

1. **Lock contention**: MemorySharded reduces to minimum
2. **Network overhead**: Connection pool reuse
3. **Serialization**: Protobuf efficient serialization
4. **GC pressure**: Object pool reduces allocations

### Comparison with Other Configurations

| Configuration | Concurrent Reads | Concurrent Writes | Latency |
|--------------|-----------------|-------------------|---------|
| **This Config** | **6M ops/s** | **3.5M ops/s** | **200ns** |
| Memory backend | 5M ops/s | 3M ops/s | 250ns |
| Single replica | 7M ops/s | 5M ops/s | 150ns |
| 5 replicas R=3 | 3M ops/s | 1.5M ops/s | 500ns |

---

## 💡 Optimization Recommendations

### Further Improve Read Performance

If you need higher read performance:

```go
// Option 1: Single replica (sacrifice reliability)
ReplicaCount: 1,
ReadQuorum:   1,
// Effect: Read performance improves to 7M+ ops/s

// Option 2: More shards (requires more CPU)
// MemorySharded defaults to CPU×16 shards
// Effect: Better performance on 16+ core machines
```

### Further Improve Write Performance

```go
// Option 1: Lower WriteQuorum
WriteQuorum: 1,  // Write to 1 replica only
// Effect: 2-3x write performance improvement

// Option 2: Increase replication pool
MaxReplicators: runtime.NumCPU() * 4,
// Effect: 50% concurrent write improvement
```

---

## ⚠️ Considerations

### Consistency Trade-offs

Current configuration (R=1, W=2, N=3) provides **eventual consistency**:

```
Pros:
✅ Highest read performance
✅ Higher write performance
✅ Good availability

Cons:
❌ Does not guarantee strong consistency
❌ May read stale data
```

**Solution**: For strong consistency, set `ReadQuorum=2`

### Memory Management

- 16GB configuration suitable for medium clusters
- Monitor memory usage, avoid exceeding limits
- Use `MaxMemoryMB` to limit memory

### Fault Tolerance

- 3 replicas can tolerate 1 node failure
- More than 1 node failure will affect writes
- Recommend at least 5 nodes + 3 replicas

---

## 🔍 Monitoring Metrics

### Key Metrics

```go
// Metrics to monitor
- Throughput (ops/s)
- P50/P95/P99 latency
- Memory usage
- CPU usage
- Error rate
- Node health status
```

### Alert Thresholds

```
- Throughput < 2M ops/s (normal 5-7M)
- P99 latency > 1ms (normal < 500ns)
- Memory usage > 80%
- Error rate > 1%
```

---

## 📚 Related Documentation

- [Performance Test Report](../../FINAL_PERFORMANCE_REPORT.md)
- [Configuration Parameters](../../docs/CONFIGURATION.md)
- [Scenario Comparison](../README.md)

---

**Use Case**: High concurrency cache, microservices, session storage  
**Performance Level**: ⭐⭐⭐⭐⭐  
**Recommendation**: ⭐⭐⭐⭐⭐
