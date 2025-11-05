# GridKV Architecture

## Overview

GridKV is a high-performance distributed key-value store built in Go, designed for enterprise-grade applications requiring consistency, scalability, and minimal dependencies.

## System Architecture

### Component Hierarchy

```
GridKV
├── API Layer (pkg/)
│   └── GridKV Client Interface
├── Core Layer (internal/)
│   ├── Gossip Protocol
│   │   ├── Failure Detection (SWIM)
│   │   ├── State Synchronization
│   │   └── Membership Management
│   ├── Storage Interface
│   │   └── Registry Pattern
│   └── Transport Interface
│       └── Registry Pattern
├── Storage Backends (backends/)
│   ├── Memory (native Go, default)
│   ├── File (native Go, persistent)
│   ├── Badger (LSM-tree, optional)
│   └── Ristretto (TinyLFU cache, optional)
└── Transport Layer (transports/)
    └── TCP (native Go, default)
```

## Core Components

### 1. Gossip Protocol Manager

**Purpose**: Distributed state management and failure detection

**Key Features**:
- SWIM-based failure detection
- Hybrid Logical Clock (HLC) timestamps
- Anti-entropy synchronization
- Configurable consistency levels

**Implementation**: `internal/gossip/core.go`

### 2. Storage Layer

**Architecture**: Registry-based plugin system

**Interface**: `internal/storage/storage.go`
```go
type Storage interface {
    Set(key string, item *StoredItem) error
    Get(key string) (*StoredItem, error)
    Delete(key string, version int64) error
    Keys() []string
    Clear() error
    Close() error
    // Sync operations for distributed consistency
    GetSyncBuffer() ([]*CacheSyncOperation, error)
    GetFullSyncSnapshot() ([]*FullStateItem, error)
    ApplyIncrementalSync([]*CacheSyncOperation) error
    ApplyFullSyncSnapshot([]*FullStateItem, time.Time) error
    Stats() StorageStats
}
```

**Available Backends**:

| Backend | Performance | Memory Allocation | Dependencies | Use Case |
|---------|-------------|-------------------|--------------|----------|
| File | 775K ops/sec | 995 B/op, 21 allocs/op | None | Production (persistent) |
| Memory | 640K ops/sec | 1047 B/op, 23 allocs/op | None | Production (volatile) |
| Ristretto | 270K ops/sec | 1520 B/op, 24 allocs/op | ristretto/v2 | Cache optimization |
| BadgerMemory | 78K ops/sec | 4865 B/op, 81 allocs/op | badger/v3 | Testing |
| Badger | 55K ops/sec | 5194 B/op, 91 allocs/op | badger/v3 | Large datasets |

### 3. Transport Layer

**Architecture**: Registry-based plugin system

**Interface**: `internal/transport/transport.go`
```go
type Transport interface {
    Dial(address string) (TransportConn, error)
    Listen(address string) (TransportListener, error)
}
```

**Available Transports**:
- TCP (default, native Go)
- UDP (low latency, connectionless) - internal/transport
- QUIC (encrypted, multiplexed) - internal/transport
- gnet (zero-copy, event-driven) - internal/transport

### 4. Consistency Model

**Type**: Tunable consistency with quorum-based replication

**Configuration**:
- `ReplicaCount`: Number of replicas per key
- `WriteQuorum`: Minimum successful writes
- `ReadQuorum`: Minimum successful reads

**Consistency Levels**:
- Strong: `WriteQuorum = ReadQuorum = ReplicaCount`
- Eventual: `WriteQuorum = ReadQuorum = 1`
- Balanced: `WriteQuorum = ReadQuorum = (ReplicaCount/2) + 1`

## Dependency Management

### Import Control Strategy

GridKV uses pure Go import control for dependency management. No build tags are required.

**Philosophy**:
- Import only what you need
- Not imported = not compiled = not in binary
- Standard Go semantics, zero magic

**Example**:
```go
import (
    "github.com/feellmoose/gridkv/pkg"
    
    // Import only required backends
    _ "github.com/feellmoose/gridkv/backends/memory"  // Included
    _ "github.com/feellmoose/gridkv/backends/file"    // Included
    // Badger not imported = not available = zero deps
    
    // Import required transport
    _ "github.com/feellmoose/gridkv/transports/tcp"
)
```

**Default Configuration**:
```go
import _ "github.com/feellmoose/gridkv/defaults"
```
Automatically imports: Memory backend + TCP transport (zero external dependencies)

## Data Flow

### Write Operation

```
Client.Set()
    ↓
API Layer (pkg/gridkv.go)
    ↓
GossipManager.Set() (internal/gossip/core.go)
    ↓
├─ Local Write
│   ↓
│   Storage.Set()
│   ↓
│   Ring Buffer Sync
│
└─ Replicate to Peers
    ↓
    Transport.WriteMessage()
    ↓
    Remote Node.HandleMessage()
    ↓
    Remote Storage.Set()
```

### Read Operation

```
Client.Get()
    ↓
API Layer
    ↓
GossipManager.Get()
    ↓
├─ Local Read (if available)
│   ↓
│   Storage.Get()
│   ↓
│   Return (fast path)
│
└─ Quorum Read (if needed)
    ↓
    Parallel reads from replicas
    ↓
    Read repair (if inconsistent)
    ↓
    Return latest value
```

## Consistency Mechanisms

### 1. Versioning

- Hybrid Logical Clock (HLC) timestamps
- Version comparison for conflict resolution
- Last-Write-Wins (LWW) strategy

### 2. Replication

- Consistent hashing with virtual nodes
- Automatic replica placement
- Tunable replication factor

### 3. Read Repair

- Detect inconsistencies during reads
- Asynchronous repair to lagging replicas
- Eventual consistency guarantee

### 4. Anti-Entropy

- Periodic full state synchronization
- Incremental sync via ring buffer
- Merkle tree comparison (future enhancement)

## Performance Optimizations

### Implemented Optimizations

1. **Lock-Free Ring Buffer**
   - Atomic operations for sync buffer
   - Bounded memory, no allocations
   - 37% throughput improvement

2. **Conditional Message Signing**
   - Skip crypto for local-only operations
   - Context-aware security
   - 164% throughput improvement

3. **Connection Pooling**
   - Reusable transport connections
   - Configurable pool size
   - Reduced connection overhead

4. **Object Pooling**
   - sync.Pool for frequent allocations
   - Reduced GC pressure
   - Memory allocation reduction

5. **TCP Optimizations**
   - SetNoDelay(true) - disable Nagle
   - SetKeepAlive(true) - connection health
   - Optimized buffer sizes

### Performance Characteristics

**Latency**:
- Local operations: 1.3-1.6 µs
- Network round-trip: ~1-2 ms (LAN)
- Quorum write: ~2-5 ms (3 nodes)

**Throughput**:
- Single node: 640-775K ops/sec
- 3-node cluster: 450K ops/sec
- 5-node cluster: 380K ops/sec

**Scalability**:
- Linear read scaling
- Write scaling: 94% efficiency
- Horizontal scaling supported

## Monitoring and Observability

### Available Metrics

```go
type StorageStats struct {
    KeyCount      int64
    SyncBufferLen int
    CacheHitRate  float64
    DBSize        int64
}
```

**Access**: `storage.Stats()`

### Logging

**Levels**: INFO, WARN, ERROR
**Format**: Structured logging (zerolog)
**Configuration**: `internal/utils/logging/`

## Security

### Authentication
- Ed25519 message signing (optional)
- Public key infrastructure
- Peer verification

### Network Security
- QUIC transport (encrypted by default)
- TLS support (future enhancement)
- Message integrity verification

## Configuration

### GridKV Options

```go
type GridKVOptions struct {
    LocalNodeID   string
    LocalAddress  string
    Network       *NetworkOptions
    Storage       *StorageOptions
    ReplicaCount  int
    WriteQuorum   int
    ReadQuorum    int
    GossipInterval time.Duration
    ProbeInterval  time.Duration
}
```

### Storage Options

```go
type StorageOptions struct {
    Backend      StorageBackendType
    DirPath      string
    MaxMemoryMB  int64
}
```

### Network Options

```go
type NetworkOptions struct {
    Type         TransportType
    BindAddr     string
    MaxConns     int
    MaxIdle      int
    ReadTimeout  time.Duration
    WriteTimeout time.Duration
}
```

## Extension Points

### Adding a New Storage Backend

1. Implement `storage.Storage` interface
2. Create package in `backends/`
3. Register in `init()` function:
```go
func init() {
    storage.RegisterBackend("mybackend", func(opts *storage.StorageOptions) (storage.Storage, error) {
        return NewMyBackend(opts)
    })
}
```

### Adding a New Transport

1. Implement `transport.Transport` interface
2. Create package in `transports/`
3. Register in `init()` function:
```go
func init() {
    transport.RegisterTransport("mytransport", func() (transport.Transport, error) {
        return NewMyTransport()
    })
}
```

## Directory Structure

```
gridkv/
├── pkg/                    # Public API
│   └── gridkv.go          # Main client interface
├── internal/              # Private implementation
│   ├── gossip/           # Distributed protocol
│   ├── storage/          # Storage interface
│   ├── transport/        # Network interface
│   └── utils/            # Utilities
├── backends/              # Storage implementations
│   ├── memory/           # In-memory cache
│   ├── file/             # File-based storage
│   ├── badger/           # BadgerDB backend
│   ├── ristretto/        # Ristretto cache
│   └── all/              # Convenience import
├── transports/            # Transport implementations
│   ├── tcp/              # TCP transport
│   └── all/              # Convenience import
├── defaults/              # Default configuration
│   └── defaults.go       # Memory + TCP
├── examples/              # Usage examples
├── tests/                 # Benchmarks and tests
└── docs/                  # Documentation

```

## Design Principles

1. **Simplicity**: Use standard Go patterns, avoid magic
2. **Performance**: Optimize hot paths, minimize allocations
3. **Modularity**: Clear interfaces, pluggable components
4. **Flexibility**: Tunable consistency, multiple backends
5. **Production-Ready**: Battle-tested, well-documented

## References

- SWIM Protocol: [arXiv:cs/0511084](https://arxiv.org/abs/cs/0511084)
- Hybrid Logical Clocks: [CSE 2013](https://cse.buffalo.edu/tech-reports/2014-04.pdf)
- Consistent Hashing: [Dynamo Paper](https://www.allthingsdistributed.com/files/amazon-dynamo-sosp2007.pdf)

## Version History

- v3.7.0: Pure import control, removed build tags
- v3.6.0: Modular architecture, defaults package
- v3.5.0: Backend submodules
- v3.4.0: Performance optimizations
- v3.0.0: Initial production release

