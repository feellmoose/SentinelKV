# GridKV

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Performance](https://img.shields.io/badge/performance-7.4M_ops/s-brightgreen)]()

**High-Performance Embedded Distributed Key-Value Storage Engine**

English | [简体中文](README_CN.md)

</div>

---

## 📖 Overview

GridKV is a high-performance, strongly consistent embedded distributed key-value storage engine. Integrated directly into applications as a Go SDK without requiring independent server deployment, application instances automatically form a distributed cluster.

### Key Features

- **🚀 Extreme Performance**: 7.4M ops/s concurrent reads, 3.4M ops/s writes
- **🔒 Strong Consistency**: Complete Gossip protocol + Quorum-based reads/writes
- **⚡ Ultra-Low Latency**: P99 latency < 1ms, P50 latency 135ns
- **🛡️ High Reliability**: Full-stack panic protection, automatic failure recovery
- **📦 Zero Deployment**: Embedded architecture, automatic cluster management
- **🎯 Production Ready**: Validated with 23.7 billion operations

---

## 🚀 Quick Start

### Installation

```bash
go get github.com/feellmoose/gridkv
```

### Basic Usage

```go
package main

import (
    "context"
    "log"
    
    "github.com/feellmoose/gridkv"
    "github.com/feellmoose/gridkv/internal/gossip"
    "github.com/feellmoose/gridkv/internal/storage"
)

func main() {
    // Create instance
    kv, err := gridkv.NewGridKV(&gridkv.GridKVOptions{
        LocalNodeID:  "node-1",
        LocalAddress: "localhost:8001",
        
        Network: &gossip.NetworkOptions{
            Type:     gossip.TCP,
            BindAddr: "localhost:8001",
        },
        
        Storage: &storage.StorageOptions{
            Backend:     storage.BackendMemorySharded,
            MaxMemoryMB: 4096,
        },
        
        ReplicaCount: 3,
        WriteQuorum:  2,
        ReadQuorum:   2,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer kv.Close()
    
    ctx := context.Background()
    
    // Write
    kv.Set(ctx, "user:1001", []byte("Alice"))
    
    // Read
    value, _ := kv.Get(ctx, "user:1001")
    log.Printf("Value: %s", value)
    
    // Delete
    kv.Delete(ctx, "user:1001")
}
```

### Distributed Cluster

```go
// Node 1 (seed node)
node1, _ := gridkv.NewGridKV(&gridkv.GridKVOptions{
    LocalNodeID:  "node-1",
    LocalAddress: "localhost:8001",
    // ...other config
})

// Node 2 (automatically join cluster)
node2, _ := gridkv.NewGridKV(&gridkv.GridKVOptions{
    LocalNodeID:  "node-2",
    LocalAddress: "localhost:8002",
    SeedAddrs:    []string{"localhost:8001"},
    // ...other config
})

// Data automatically replicates across nodes
node1.Set(ctx, "key", []byte("value"))
value, _ := node2.Get(ctx, "key")  // ✅ Can read the value
```

---

## 📊 Performance

### Benchmarks (Intel i7-12700H, 20 cores)

| Operation | Throughput | Latency |
|-----------|-----------|---------|
| **Concurrent Read (100 goroutines)** | **7.43M ops/s** | **135 ns** |
| **Concurrent Write (100 goroutines)** | **3.44M ops/s** | 290 ns |
| Single-node Read | 2.82M ops/s | 355 ns |
| Single-node Write | 915K ops/s | 1,108 ns |
| 5-node Cluster | 4.50M ops/s | < 500 ns |

---

## 🎯 Core Technologies

### Distributed Protocol

- **Gossip Protocol**: Decentralized cluster management, automatic node discovery
- **SWIM Failure Detection**: Automatic detection and recovery in < 30 seconds
- **Consistent Hashing**: Balanced data distribution, smooth scaling
- **Quorum Reads/Writes**: Configurable consistency levels (R+W > N for strong consistency)

### Storage Engine

- **MemorySharded**: 256 shards, CPU core adaptive
- **Object Pool Optimization**: sync.Pool reduces GC pressure
- **Deep Copy Protection**: Returned data is safe to modify
- **Memory Limits**: Configurable memory ceiling

### Network Layer

- **TCP/UDP Dual Protocol**: TCP for data transfer, UDP for Gossip
- **Connection Pooling**: Connection reuse, reduced handshake overhead
- **Auto-Reconnection**: Automatic recovery on connection failure
- **Health Checks**: Periodic connection health monitoring

### Fault Tolerance

- **Full-Stack Panic Protection**: Coverage across API, Gossip, and Transport layers
- **Auto Failure Recovery**: Handles connection failures and node failures
- **Read Repair**: Automatically fixes inconsistent data
- **Graceful Degradation**: Continues service during partial node failures

---

## 📁 Configuration Scenarios

6 production-ready configuration scenarios covering different use cases:

| Scenario | Performance | Use Case | Documentation |
|----------|------------|----------|--------------|
| High Concurrency | 5-7M ops/s | Microservice caching, Session storage | [View](examples/01_high_concurrency/) |
| Strong Consistency | 2-3M ops/s | Financial transactions, Order systems | [View](examples/02_strong_consistency/) |
| High Availability | 3-4M ops/s | Core services, 24/7 systems | [View](examples/03_high_availability/) |
| Low Latency | 6-7M ops/s, P99<1ms | Real-time recommendations, Game state | [View](examples/04_low_latency/) |
| Large Cluster | 10-50M ops/s | 20+ nodes, Data centers | [View](examples/05_large_cluster/) |
| Development/Testing | 1-2M ops/s | Local development, Unit tests | [View](examples/06_dev_testing/) |

**Quick Run**:
```bash
cd examples/01_high_concurrency
go run main.go
```

---

## 🛠️ Configuration

### Basic Configuration

```go
&gridkv.GridKVOptions{
    // Node identity
    LocalNodeID:  "node-1",           // Unique node identifier
    LocalAddress: "localhost:8001",   // Node address
    SeedAddrs:    []string{...},      // Seed node addresses (join existing cluster)
    
    // Storage configuration
    Storage: &storage.StorageOptions{
        Backend:     storage.BackendMemorySharded, // Recommended: sharded storage
        MaxMemoryMB: 4096,                         // Memory limit (MB)
    },
    
    // Consistency configuration
    ReplicaCount: 3,  // Number of replicas
    WriteQuorum:  2,  // How many replicas must be written for success
    ReadQuorum:   2,  // How many replicas to read and compare
    
    // Network configuration
    Network: &gossip.NetworkOptions{
        Type:         gossip.TCP,
        BindAddr:     "localhost:8001",
        MaxConns:     1000,
        MaxIdle:      100,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 5 * time.Second,
    },
}
```

### Consistency Levels

| Configuration | Consistency | Performance | Use Case |
|--------------|------------|------------|----------|
| R=1, W=1 | Eventual | Highest | Caching, temporary data |
| R=1, W=2 | Read-optimized | High | High-concurrency read scenarios |
| R=2, W=2 | Strong | Medium | Financial, order systems |
| R=3, W=3 | Strongest | Lower | Critical business data |

**Strong Consistency Requirement**: `ReadQuorum + WriteQuorum > ReplicaCount`

---

## 📚 Technical Documentation

### Core Documents

- [Architecture](docs/ARCHITECTURE.md) - System architecture overview
- [Gossip Protocol](docs/GOSSIP_PROTOCOL.md) - Distributed communication protocol
- [Consistency Model](docs/CONSISTENCY_MODEL.md) - Data consistency guarantees
- [Consistent Hashing](docs/CONSISTENT_HASHING.md) - Data sharding algorithm
- [Hybrid Logical Clock](docs/HYBRID_LOGICAL_CLOCK.md) - Causal consistency
- [Storage Backends](docs/STORAGE_BACKENDS.md) - Storage engine design
- [Transport Layer](docs/TRANSPORT_LAYER.md) - Network communication layer
- [Quick Reference](docs/QUICK_REFERENCE.md) - API quick reference

---

## 🎯 Use Cases

### ✅ Recommended Scenarios

- **Microservice Caching**: API response caching, database query caching
- **Session Storage**: User sessions, login states
- **Real-time Data**: Leaderboards, real-time statistics, hot data
- **Configuration Center**: Application configs, feature flags
- **Game State**: Game rooms, player state synchronization
- **Real-time Recommendations**: Personalized recommendations, content push

### ⚠️ Not Recommended For

- **Large-scale Persistence**: TB-level data storage (consider Cassandra/ScyllaDB)
- **Complex Queries**: SQL JOINs, complex transactions (consider relational databases)
- **Single-instance Low Traffic**: < 1K ops/s (Redis is simpler)

---

## 🔍 Monitoring & Operations

### Key Metrics

```go
// Metrics to monitor
- Throughput (ops/s)
- P50/P95/P99 Latency
- Memory usage
- CPU usage
- GC time
- Error rate
- Node health status
- Cluster size
```

### Alerting Recommendations

```go
// Suggested alert thresholds
- Throughput drop > 30%
- P99 latency > 10ms
- Memory usage > 80%
- Error rate > 1%
- Node failures > 1
- GC time > 100ms
```

---

## 🧪 Testing

### Run Tests

```bash
# All tests
go test ./tests/

# Performance tests
go test -bench=. ./tests/

# Safety tests
go test -run=Safety ./tests/

# Panic recovery tests
go test -run=Panic ./tests/
```

### Test Coverage

- **Functional Tests**: Basic operations, edge cases
- **Performance Tests**: Single-node, concurrent, cluster
- **Safety Tests**: Data independence, concurrent safety
- **Reliability Tests**: Panic recovery, fault tolerance
- **Long-running Tests**: Stability, memory leaks

---

## 📦 Production Deployment

### Hardware Recommendations

**Small Cluster (3-5 nodes)**:
- CPU: 8 cores
- Memory: 16GB
- Network: 1Gbps
- Suitable for: 10K-100K QPS

**Medium Cluster (5-10 nodes)**:
- CPU: 16 cores
- Memory: 32GB
- Network: 10Gbps
- Suitable for: 100K-1M QPS

**Large Cluster (10-50 nodes)**:
- CPU: 32 cores
- Memory: 64GB
- Network: 25Gbps+
- Suitable for: 1M+ QPS

### Best Practices

1. **Use MemorySharded backend** - Best performance
2. **Set appropriate replica count** - Usually 3 replicas suffice
3. **Monitor memory usage** - Avoid exceeding limits
4. **Regular health checks** - Monitor node status
5. **Graceful shutdown** - Use `defer kv.Close()`

---

## 🤝 Contributing

Issues and Pull Requests are welcome!

### Development Environment

```bash
git clone https://github.com/feellmoose/gridkv.git
cd gridkv
go mod download
go test ./...
```

---

## 📄 License

MIT License - See [LICENSE](LICENSE) for details

---

## 🙏 Acknowledgments

GridKV uses the following excellent open-source projects:

- [Protocol Buffers](https://github.com/protocolbuffers/protobuf-go) - Serialization
- [ants](https://github.com/panjf2000/ants) - Goroutine pool
- [xxhash](https://github.com/cespare/xxhash) - Hashing algorithm

---

## 📮 Contact

- **GitHub**: https://github.com/feellmoose/gridkv
- **Issues**: https://github.com/feellmoose/gridkv/issues

---

<div align="center">

**GridKV** - High-Performance Distributed KV Storage for Modern Applications

*Performance · Consistency · Reliability*

</div>

