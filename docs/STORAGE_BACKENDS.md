# Storage Backend Implementations

## Overview

GridKV provides five storage backends with different performance characteristics and use cases. All backends implement the `storage.Storage` interface.

## Backend Comparison

| Backend | Throughput | Latency | Persistence | Dependencies | Memory Usage |
|---------|-----------|---------|-------------|--------------|--------------|
| File | 775K ops/sec | 1.29 µs | Yes | None | Low |
| Memory | 640K ops/sec | 1.56 µs | No | None | Medium |
| Ristretto | 270K ops/sec | 3.71 µs | No | ristretto/v2 | Medium |
| BadgerMemory | 78K ops/sec | 12.81 µs | No | badger/v3 | High |
| Badger | 55K ops/sec | 18.14 µs | Yes | badger/v3 | High |

## Backend Details

### 1. FileStorage

**Implementation**: `backends/file/storage.go`

**Architecture**:
- In-memory map with periodic persistence
- GOB encoding for serialization
- RWMutex for consistency
- Lock-free ring buffer for sync operations

**Characteristics**:
- Fastest backend (775K ops/sec)
- Zero external dependencies
- Persistent storage
- Suitable for small to medium datasets (<10GB)

**Persistence Mechanism**:
```go
Process:
  1. Write to in-memory map
  2. Add to sync buffer
  3. Background goroutine persists periodically (5s default)
  4. Atomic file write (temp file + rename)
  
On restart:
  - Load entire file into memory
  - Parse GOB-encoded data
  - Rebuild in-memory map
```

**Advantages**:
- Highest performance
- Simple deployment
- No external dependencies
- Good for edge/IoT scenarios

**Limitations**:
- Dataset must fit in memory
- Slower restart for large datasets
- No built-in compression

### 2. Memory

**Implementation**: `backends/memory/storage.go`

**Architecture**:
- Pure in-memory using sync.Map
- Lock-free concurrent access
- Atomic ring buffer for sync
- No persistence

**Characteristics**:
- Very high performance (640K ops/sec)
- Zero external dependencies
- Lowest memory overhead
- Perfect for caching

**Concurrency**:
```go
sync.Map advantages:
  - Lock-free reads in common case
  - Optimized for read-heavy workloads
  - Minimal contention

Performance characteristics:
  - Reads: Nearly lock-free
  - Writes: Amortized O(1)
  - No central lock bottleneck
```

**Use Cases**:
- Development and testing
- Session storage
- Cache layer
- Temporary data

### 3. Ristretto

**Implementation**: `backends/ristretto/storage.go`

**Architecture**:
- TinyLFU admission policy
- SampledLFU eviction policy
- Lock-free concurrent access
- Cost-based memory management

**Characteristics**:
- High throughput (270K ops/sec)
- Intelligent cache eviction
- Adaptive to workload patterns
- Memory-bounded

**Eviction Policy**:
```go
TinyLFU (Tiny Least Frequently Used):
  1. Track access frequency with minimal memory
  2. Admission policy: Only admit if more valuable than victim
  3. Eviction: Remove least valuable item by frequency × cost
  
Benefits:
  - High hit rate on real-world workloads
  - Prevents cache pollution
  - Adapts to changing access patterns
```

**Dependencies**: 
- `github.com/dgraph-io/ristretto/v2`

**Use Cases**:
- Hot key optimization
- Cache with automatic memory management
- Read-heavy workloads
- When cache hit rate is critical

### 4. Badger

**Implementation**: `backends/badger/storage.go`

**Architecture**:
- LSM-tree (Log-Structured Merge-tree)
- Badger database + Ristretto cache (two-tier)
- Write-ahead log for durability
- Background compaction

**Characteristics**:
- Suitable for large datasets (>100GB)
- Persistent storage
- Good sequential write performance
- Built-in compression

**LSM-Tree Design**:
```go
Levels:
  L0: MemTable (in-memory, unsorted)
  L1-L6: SSTables (on-disk, sorted)
  
Write Path:
  1. Write to WAL (durability)
  2. Write to MemTable
  3. Background flush to L0
  4. Background compaction (L0→L1→...→L6)
  
Read Path:
  1. Check Ristretto cache (fast path)
  2. Check MemTable
  3. Check SSTables (L0→L1→...→L6)
  4. Populate cache on read
```

**Dependencies**:
- `github.com/dgraph-io/badger/v3`
- `github.com/dgraph-io/ristretto/v2` (used internally by Badger)

**Use Cases**:
- Large datasets
- Durable storage required
- Write-heavy workloads
- When memory is limited

### 5. BadgerMemory

**Implementation**: `backends/badger/storage.go` (with empty dirPath)

**Characteristics**:
- Badger in in-memory mode
- No persistence
- Useful for testing
- Lower performance than Memory backend

**Use Cases**:
- Testing Badger-specific features
- Temporary large datasets
- Development

## Storage Interface

```go
type Storage interface {
    // Basic CRUD operations
    Set(key string, item *StoredItem) error
    Get(key string) (*StoredItem, error)
    Delete(key string, version int64) error
    Keys() []string
    Clear() error
    Close() error
    
    // Distributed synchronization
    GetSyncBuffer() ([]*CacheSyncOperation, error)
    GetFullSyncSnapshot() ([]*FullStateItem, error)
    ApplyIncrementalSync([]*CacheSyncOperation) error
    ApplyFullSyncSnapshot([]*FullStateItem, time.Time) error
    
    // Monitoring
    Stats() StorageStats
}
```

## Synchronization Mechanisms

### Incremental Sync

**Purpose**: Replicate recent changes to peers

**Implementation**:
```go
Ring Buffer:
  - Lock-free atomic operations
  - Power-of-2 sizing for fast modulo
  - Bounded memory (8K operations default)
  
Process:
  1. Each write adds operation to ring buffer
  2. Periodic sync reads buffer (non-blocking)
  3. Send operations to peers
  4. Peers apply with version checking
```

**Performance**:
- Write overhead: ~2 atomic operations
- Sync overhead: O(buffer_size)
- Network efficiency: Batched operations

### Full Sync

**Purpose**: New node bootstrap or recovery from partition

**Implementation**:
```go
Process:
  1. Request full snapshot from peer
  2. Peer serializes all keys
  3. Receiver clears local state
  4. Apply complete snapshot
  5. Resume normal operation
  
Optimization:
  - Snapshot is point-in-time consistent
  - Versioned to detect concurrent changes
  - Applied atomically (clear + load)
```

## Selection Guide

### Decision Matrix

**Choose FileStorage** if:
- Need persistence
- Want highest performance
- Dataset fits in memory
- Zero external dependencies preferred

**Choose Memory** if:
- Temporary/cache data
- Don't need persistence
- Want simplest deployment
- Development/testing

**Choose Ristretto** if:
- Need intelligent eviction
- Want automatic memory management
- Cache hit rate is critical
- Acceptable to add one dependency

**Choose Badger** if:
- Dataset larger than memory
- Need persistent storage
- Write-heavy workload
- Can accept external dependencies

## Performance Tuning

### Memory Backend

```go
MaxMemoryMB: 1024  // Soft limit (not enforced)

Tuning:
  - Increase for larger working sets
  - Monitor actual memory usage
  - No automatic eviction
```

### FileStorage Backend

```go
PersistInterval: 5 * time.Second

Tuning:
  - Lower interval: More durable, higher I/O
  - Higher interval: Less I/O, data loss window
  - Monitor disk I/O and latency
```

### Badger Backend

```go
Badger Options:
  - MaxTableSize: 64MB (default)
  - NumMemtables: 3 (default)
  - NumLevelZeroTables: 5 (default)
  
Tuning:
  - Increase MemTable size for write bursts
  - Adjust compaction for steady-state
  - Monitor LSM tree depth
```

### Ristretto Backend

```go
Configuration:
  - NumCounters: maxMemoryMB * 100
  - MaxCost: maxMemoryMB * 1024 * 1024
  - BufferItems: 64
  
Tuning:
  - Increase counters for better accuracy
  - Adjust MaxCost for memory limit
  - Monitor hit rate and evictions
```

## Memory Allocations

### Per-Operation Allocations

| Backend | Bytes/op | Allocs/op |
|---------|----------|-----------|
| File | 995 | 21 |
| Memory | 1047 | 23 |
| Ristretto | 1520 | 24 |
| BadgerMemory | 4865 | 81 |
| Badger | 5194 | 91 |

### Allocation Hotspots

1. **Protobuf conversions** (~30% of allocations)
2. **Sync buffer operations** (~20%)
3. **Network serialization** (~15%)
4. **Backend-specific** (~35%)

## Monitoring

### Storage Statistics

```go
type StorageStats struct {
    KeyCount      int64   // Current number of keys
    SyncBufferLen int     // Pending sync operations
    CacheHitRate  float64 // Hit rate (0.0-1.0)
    DBSize        int64   // Disk usage (bytes)
}
```

**Access**: 
```go
stats := kv.Stats()
```

**Recommended Metrics**:
- Key count growth rate
- Sync buffer depth (should be <1000 typically)
- Cache hit rate (target >90% for cache backends)
- Disk size growth (for persistent backends)

## Implementation Details

### Registry Pattern

All backends register via `init()`:
```go
func init() {
    storage.RegisterBackend("backend-name", factoryFunc)
}
```

**Import Control**:
```go
import _ "github.com/feellmoose/gridkv/backends/memory"  // Available
// Badger not imported = not available
```

### Object Pooling

**Optimizations in all backends**:
- sync.Pool for frequently allocated objects
- Reduces GC pressure
- Reuses memory

## Future Enhancements

1. **Compression**: LZ4/Snappy for on-disk storage
2. **Tiered Storage**: Hot data in memory, cold on disk
3. **Distributed Badger**: Coordinated compaction
4. **Custom Eviction**: Pluggable eviction policies
5. **Metrics Integration**: Prometheus/StatsD exporters

## References

- Badger Paper: "Badger: Fast Key-Value DB in Go"
- Ristretto Blog: "Introducing Ristretto: A High Performance Go Cache"
- LSM-tree: O'Neil et al., "The Log-Structured Merge-Tree"

