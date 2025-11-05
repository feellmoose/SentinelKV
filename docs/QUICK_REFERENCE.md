# GridKV Quick Reference

## Installation

```bash
go get github.com/feellmoose/gridkv
```

## Basic Operations

### Initialization

```go
import (
    "github.com/feellmoose/gridkv/pkg"
    "github.com/feellmoose/gridkv/internal/gossip"
    _ "github.com/feellmoose/gridkv/defaults"
)

kv, err := gridkv.NewGridKV(&gridkv.GridKVOptions{
    LocalNodeID:  "node-1",
    LocalAddress: "localhost:8001",
    Network: &gossip.NetworkOptions{
        Type:     gossip.TCP,
        BindAddr: "localhost:8001",
    },
    Storage: &gossip.StorageOptions{
        Backend: gossip.Memory,
    },
    ReplicaCount: 3,
})
```

### Set

```go
ctx := context.Background()
err := kv.Set(ctx, "key", []byte("value"))
```

### Get

```go
value, err := kv.Get(ctx, "key")
```

### Delete

```go
err := kv.Delete(ctx, "key")
```

### List Keys

```go
keys := kv.Keys()
```

### Close

```go
err := kv.Close()
```

## Storage Backends

### Memory (Default)

```go
import _ "github.com/feellmoose/gridkv/backends/memory"

Storage: &gossip.StorageOptions{
    Backend:     gossip.Memory,
    MaxMemoryMB: 1024,
}
```

**Performance**: 640K ops/sec
**Dependencies**: None
**Persistence**: No

### FileStorage (Recommended)

```go
import _ "github.com/feellmoose/gridkv/backends/file"

Storage: &gossip.StorageOptions{
    Backend: gossip.FileStorage,
    DirPath: "/var/lib/gridkv",
}
```

**Performance**: 775K ops/sec
**Dependencies**: None
**Persistence**: Yes

### Badger

```go
import _ "github.com/feellmoose/gridkv/backends/badger"

Storage: &gossip.StorageOptions{
    Backend: gossip.Badger,
    DirPath: "/var/lib/gridkv",
}
```

**Performance**: 55K ops/sec
**Dependencies**: badger/v3, ristretto/v2
**Persistence**: Yes
**Use Case**: Large datasets

### Ristretto

```go
import _ "github.com/feellmoose/gridkv/backends/ristretto"

Storage: &gossip.StorageOptions{
    Backend:     gossip.Ristretto,
    MaxMemoryMB: 2048,
}
```

**Performance**: 270K ops/sec
**Dependencies**: ristretto/v2
**Persistence**: No
**Use Case**: Cache optimization

## Configuration

### Consistency Levels

```go
// Strong Consistency
ReplicaCount: 3
WriteQuorum:  3
ReadQuorum:   3

// Balanced
ReplicaCount: 3
WriteQuorum:  2
ReadQuorum:   2

// Eventual
ReplicaCount: 3
WriteQuorum:  1
ReadQuorum:   1
```

### Network Configuration

```go
Network: &gossip.NetworkOptions{
    Type:         gossip.TCP,
    BindAddr:     "localhost:8001",
    MaxConns:     1000,
    MaxIdle:      100,
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 5 * time.Second,
}
```

### Gossip Configuration

```go
GossipInterval: 1 * time.Second  // State propagation interval
ProbeInterval:  2 * time.Second  // Failure detection interval
```

## Distributed Setup

### Three-Node Cluster

```go
// Node 1 (seed node)
kv1, _ := gridkv.NewGridKV(&gridkv.GridKVOptions{
    LocalNodeID:  "node-1",
    LocalAddress: "192.168.1.10:8001",
    SeedNodes:    []string{},
    ReplicaCount: 3,
    WriteQuorum:  2,
    ReadQuorum:   2,
})

// Node 2
kv2, _ := gridkv.NewGridKV(&gridkv.GridKVOptions{
    LocalNodeID:  "node-2",
    LocalAddress: "192.168.1.11:8001",
    SeedNodes:    []string{"192.168.1.10:8001"},
    ReplicaCount: 3,
    WriteQuorum:  2,
    ReadQuorum:   2,
})

// Node 3
kv3, _ := gridkv.NewGridKV(&gridkv.GridKVOptions{
    LocalNodeID:  "node-3",
    LocalAddress: "192.168.1.12:8001",
    SeedNodes:    []string{"192.168.1.10:8001"},
    ReplicaCount: 3,
    WriteQuorum:  2,
    ReadQuorum:   2,
})
```

## Error Handling

```go
import "errors"

value, err := kv.Get(ctx, "key")
if errors.Is(err, storage.ErrItemNotFound) {
    // Key does not exist
}
if errors.Is(err, storage.ErrItemExpired) {
    // Key has expired
}
if errors.Is(err, context.DeadlineExceeded) {
    // Operation timeout
}
```

## Monitoring

### Storage Statistics

```go
stats := kv.Stats()
fmt.Printf("Keys: %d\n", stats.KeyCount)
fmt.Printf("Cache Hit Rate: %.2f\n", stats.CacheHitRate)
fmt.Printf("DB Size: %d bytes\n", stats.DBSize)
```

## Performance Tips

1. **Choose the Right Backend**
   - Use FileStorage for best performance with persistence
   - Use Memory for volatile data
   - Use Badger only for very large datasets

2. **Tune Consistency**
   - Strong consistency: Higher latency, guaranteed consistency
   - Eventual consistency: Lower latency, may read stale data
   - Balanced: Good compromise for most use cases

3. **Connection Pooling**
   - Set MaxConns appropriately for your workload
   - Monitor connection pool utilization

4. **Batch Operations**
   - Group related operations when possible
   - Use async patterns for better throughput

## Common Patterns

### Cache-Aside

```go
value, err := kv.Get(ctx, cacheKey)
if errors.Is(err, storage.ErrItemNotFound) {
    // Cache miss - fetch from primary store
    value = fetchFromDatabase(key)
    kv.Set(ctx, cacheKey, value)
}
return value
```

### Write-Through

```go
// Update primary store
updateDatabase(key, value)

// Update cache
kv.Set(ctx, key, value)
```

### Distributed Lock (Simple)

```go
lockKey := "lock:" + resourceID
acquired := false

// Try to acquire lock
err := kv.Set(ctx, lockKey, []byte(nodeID))
if err == nil {
    acquired = true
    defer kv.Delete(ctx, lockKey)
}

if acquired {
    // Perform critical section
}
```

## Troubleshooting

### High Latency

1. Check network configuration
2. Verify quorum settings
3. Monitor storage backend performance
4. Check CPU and memory usage

### Memory Issues

1. Reduce MaxMemoryMB for Memory backend
2. Switch to FileStorage or Badger
3. Implement eviction policies
4. Monitor allocation patterns

### Connection Errors

1. Verify network connectivity
2. Check firewall rules
3. Increase connection pool size
4. Monitor connection statistics

## See Also

- [Architecture Documentation](ARCHITECTURE.md)
- [Build Tags Guide](BUILD_TAGS.md)
- [Examples](../examples/)
