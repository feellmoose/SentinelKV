# GridKV Configuration Scenario Examples

This directory contains various high-performance configuration scenario examples for GridKV, each optimized for different use cases.

---

## 📁 Scenario List

### 1. [High Concurrency Cache](01_high_concurrency/) ⭐⭐⭐⭐⭐
**Use Case**: High concurrent read/write, microservice cache, session storage

**Performance Expectations**:
- Concurrent Reads: 5-7M ops/s
- Concurrent Writes: 3-4M ops/s
- Latency: < 500ns

**Configuration Highlights**:
- MemorySharded backend (256 shards)
- 3 replicas + ReadQuorum=1 (read performance priority)
- Large memory configuration (16GB)
- High-concurrency replication pool

---

### 2. [Strong Consistency](02_strong_consistency/) ⭐⭐⭐⭐⭐
**Use Case**: Financial transactions, order systems, inventory management

**Performance Expectations**:
- Reads: 2-3M ops/s
- Writes: 1-2M ops/s
- Consistency: Strong consistency

**Configuration Highlights**:
- 3 replicas + ReadQuorum=2 + WriteQuorum=2
- Short timeout settings (fast fail)
- Automatic read repair
- Strict quorum reads/writes

---

### 3. [High Availability](03_high_availability/) ⭐⭐⭐⭐
**Use Case**: 24/7 services, critical business systems

**Performance Expectations**:
- Reads: 3-4M ops/s
- Writes: 2-3M ops/s
- Availability: 99.99%+

**Configuration Highlights**:
- 5 replicas (tolerates 2 node failures)
- ReadQuorum=2, WriteQuorum=3
- Long timeout configuration (tolerates network jitter)
- SWIM failure detection optimization

---

### 4. [Low Latency](04_low_latency/) ⭐⭐⭐⭐⭐
**Use Case**: Real-time recommendations, game state, real-time bidding

**Performance Expectations**:
- P99 Latency: < 1ms
- P50 Latency: < 200ns
- Reads: 6-7M ops/s

**Configuration Highlights**:
- Single replica or 2 replicas
- ReadQuorum=1 (lowest latency)
- Memory pre-allocation
- Disable unnecessary checks

---

### 5. [Large Cluster](05_large_cluster/) ⭐⭐⭐⭐
**Use Case**: 20+ node clusters, data center-level deployment

**Performance Expectations**:
- Cluster throughput: 10-50M ops/s
- Single node: 2-3M ops/s
- Scalability: Near-linear

**Configuration Highlights**:
- Virtual node optimization (150-200)
- Longer Gossip intervals
- Batch replication optimization
- Network parameter tuning

---

### 6. [Development Testing](06_dev_testing/) ⭐⭐⭐
**Use Case**: Local development, unit testing, feature validation

**Performance Expectations**:
- Fast startup (< 100ms)
- Low resource usage
- Simple configuration

**Configuration Highlights**:
- Single replica
- Small memory limit (512MB)
- Short timeout
- Simplified network configuration

---

## 🚀 Quick Start

### Run a Single Scenario

```bash
# Enter the scenario directory
cd 01_high_concurrency

# Run the example
go run main.go
```

### Run All Scenarios

```bash
# In the examples directory
./run_all.sh
```

---

## 📊 Performance Comparison

| Scenario | Reads (ops/s) | Writes (ops/s) | Latency (P99) | Consistency | Rating |
|----------|---------------|----------------|---------------|-------------|--------|
| **High Concurrency Cache** | 5-7M | 3-4M | < 500ns | Eventual | ⭐⭐⭐⭐⭐ |
| **Strong Consistency** | 2-3M | 1-2M | < 2ms | Strong | ⭐⭐⭐⭐⭐ |
| **High Availability** | 3-4M | 2-3M | < 1ms | Strong | ⭐⭐⭐⭐ |
| **Low Latency** | 6-7M | 4-5M | < 200ns | Eventual | ⭐⭐⭐⭐⭐ |
| **Large Cluster** | 10-50M | 5-20M | < 1ms | Strong | ⭐⭐⭐⭐ |
| **Dev Testing** | 1-2M | 500K-1M | < 5ms | Eventual | ⭐⭐⭐ |

---

## 🎯 How to Choose a Scenario

### Decision Flow Chart

```
Do you need strong consistency?
├─ Yes → Financial/Order system?
│      ├─ Yes → [Strong Consistency Scenario]
│      └─ No → Need high availability?
│             ├─ Yes → [High Availability Scenario]
│             └─ No → [Strong Consistency Scenario]
│
└─ No → Latency requirements?
       ├─ < 1ms → [Low Latency Scenario]
       ├─ < 10ms → Concurrency level?
       │          ├─ High (>100K QPS) → [High Concurrency Cache Scenario]
       │          └─ Medium/Low → [Dev Testing Scenario]
       │
       └─ Not strict → Cluster size?
                  ├─ >20 nodes → [Large Cluster Scenario]
                  └─ <20 nodes → [High Concurrency Cache Scenario]
```

---

## 💡 Configuration Tuning Guide

### Key Parameter Explanations

#### 1. Replica Configuration
```go
ReplicaCount: 3,    // Number of replicas
WriteQuorum:  2,    // How many replicas must ack writes
ReadQuorum:   2,    // How many replicas to read for comparison
```

**Rules**:
- `WriteQuorum + ReadQuorum > ReplicaCount` → Strong consistency
- `ReadQuorum = 1` → Highest read performance
- `WriteQuorum = ReplicaCount` → Highest data safety

#### 2. Storage Backend
```go
storage.BackendMemory        // Single lock, simple
storage.BackendMemorySharded // Multiple locks, high performance (recommended)
```

**Performance Comparison**:
- Memory: ~800K-1M ops/s
- MemorySharded: ~1M-1.5M ops/s (+15-20%)

#### 3. Network Configuration
```go
MaxConns:     2000,  // Maximum connections
MaxIdle:      200,   // Idle connections
ReadTimeout:  5s,    // Read timeout
WriteTimeout: 5s,    // Write timeout
```

**Tuning Recommendations**:
- High concurrency: MaxConns=2000+, MaxIdle=200+
- Low latency: Short timeout (1-2s)
- High availability: Long timeout (5-10s)

#### 4. Virtual Nodes
```go
VirtualNodes: 150,  // Consistent hashing virtual nodes
```

**Rules**:
- Small cluster (<10 nodes): 100-150
- Large cluster (>20 nodes): 150-200
- Very large cluster (>50 nodes): 200-300

#### 5. Replication Configuration
```go
MaxReplicators:     16,  // Replication goroutine pool size
ReplicationTimeout: 2s,  // Replication timeout
```

**Tuning Recommendations**:
- High concurrent writes: MaxReplicators = CPU cores × 2
- Low latency: Short timeout (1-2s)

---

## 🔍 Performance Tuning Tips

### 1. Read Performance Optimization

**Method 1**: Lower ReadQuorum
```go
ReadQuorum: 1,  // Read from only 1 replica (fastest)
```
Effect: 2-3x read performance improvement

**Method 2**: Use MemorySharded
```go
Backend: storage.BackendMemorySharded,
```
Effect: 15-20% read performance improvement

### 2. Write Performance Optimization

**Method 1**: Lower replica count
```go
ReplicaCount: 1,  // Single replica (fastest)
WriteQuorum:  1,
```
Effect: 3-5x write performance improvement

**Method 2**: Increase replication pool
```go
MaxReplicators: runtime.NumCPU() * 2,
```
Effect: 50-100% concurrent write improvement

### 3. Latency Optimization

**Method 1**: Short timeout
```go
ReadTimeout:  1 * time.Second,
WriteTimeout: 1 * time.Second,
```
Effect: 50% P99 latency reduction

**Method 2**: Single replica + ReadQuorum=1
```go
ReplicaCount: 1,
ReadQuorum:   1,
```
Effect: Latency reduced to < 500ns

---

## ⚠️ Considerations

### Performance vs Consistency Trade-offs

| Configuration | Performance | Consistency | Availability |
|--------------|-------------|-------------|--------------|
| Single replica + R=1 | ⭐⭐⭐⭐⭐ | ❌ No guarantee | ❌ Low |
| 3 replicas + R=1,W=1 | ⭐⭐⭐⭐ | ❌ Eventual | ⭐⭐⭐ |
| 3 replicas + R=2,W=2 | ⭐⭐⭐ | ✅ Strong | ⭐⭐⭐⭐ |
| 5 replicas + R=3,W=3 | ⭐⭐ | ✅ Strong | ⭐⭐⭐⭐⭐ |

### Common Pitfalls

1. **Too many replicas**
   - Problem: ReplicaCount=10
   - Impact: 10x write performance degradation
   - Recommendation: Usually 3-5 replicas are sufficient

2. **Too short timeout**
   - Problem: Timeout=100ms
   - Impact: Many failures due to network jitter
   - Recommendation: At least 1 second or more

3. **Mismatched Quorum**
   - Problem: R=1, W=1 (3 replicas)
   - Impact: Cannot guarantee consistency
   - Recommendation: Ensure R+W > N

---

## 📚 Related Documentation

- [Performance Test Report](../FINAL_PERFORMANCE_REPORT.md)
- [Configuration Parameters](../docs/CONFIGURATION.md)
- [Best Practices](../docs/BEST_PRACTICES.md)
- [Troubleshooting](../docs/TROUBLESHOOTING.md)

---

**GridKV** - High-performance distributed KV storage, built for modern applications

GitHub: https://github.com/feellmoose/gridkv
