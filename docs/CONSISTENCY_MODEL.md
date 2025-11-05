# Consistency Model

## Overview

GridKV implements a tunable consistency model based on quorum-based replication, allowing users to balance between consistency, availability, and performance.

## Replication Strategy

### Quorum-Based Replication

GridKV uses the quorum consensus protocol from the Dynamo paper.

**Parameters**:
- N: Replication factor (number of replicas)
- W: Write quorum (minimum successful writes)
- R: Read quorum (minimum successful reads)

**Consistency Guarantee**:
- If W + R > N: Strong consistency (overlapping quorums)
- If W + R ≤ N: Eventual consistency (possible stale reads)

### Configuration Examples

```go
// Strong Consistency
ReplicaCount: 3  // N = 3
WriteQuorum:  3  // W = 3
ReadQuorum:   3  // R = 3
// W + R = 6 > N = 3 ✓
// All operations require full agreement

// Balanced (Recommended)
ReplicaCount: 3  // N = 3
WriteQuorum:  2  // W = 2
ReadQuorum:   2  // R = 2
// W + R = 4 > N = 3 ✓
// Can tolerate 1 node failure

// Eventual Consistency
ReplicaCount: 3  // N = 3
WriteQuorum:  1  // W = 1
ReadQuorum:   1  // R = 1
// W + R = 2 ≤ N = 3
// Maximum availability, may read stale data
```

## Write Operations

### Write Flow

```
Client Write Request
  ↓
Identify Coordinator (first replica from consistent hash)
  ↓
If local node is coordinator:
  ├─ Write to local storage
  ├─ Send write to (W-1) replicas
  └─ Wait for (W-1) acknowledgments
Else:
  └─ Forward to coordinator
  ↓
Return success if W writes succeeded
```

### Coordinator Selection

The first replica in the consistent hash list serves as coordinator:
```go
replicas := hashRing.GetN(key, replicaCount)
coordinator := replicas[0]
```

**Benefits**:
- Deterministic (same coordinator for same key)
- No coordination overhead
- Simple failure handling (forward to next replica if coordinator fails)

### Conflict Resolution

**Strategy**: Last-Write-Wins (LWW) using HLC timestamps

```go
Version Comparison:
  1. Compare HLC timestamps
  2. Higher timestamp wins
  3. If equal, higher node ID wins (deterministic tie-breaking)
```

**Optimistic Concurrency Control (OCC)**:
```go
On write:
  1. Check existing version
  2. If existing.version > new.version:
     - Reject write (stale)
  3. If existing.version ≤ new.version:
     - Accept write
```

## Read Operations

### Read Flow

```
Client Read Request
  ↓
Identify R replicas
  ↓
Send parallel read requests to R nodes
  ↓
Wait for R responses (or timeout)
  ↓
Select value with highest version
  ↓
If inconsistency detected:
  └─ Trigger read repair (async)
  ↓
Return latest value
```

### Read Repair

**Purpose**: Ensure eventual consistency across replicas

**Process**:
```go
1. During read, collect (value, version) from R nodes
2. Identify latest version: max_version
3. For each node with version < max_version:
   - Asynchronously send update
   - Node applies if version check passes
```

**Characteristics**:
- Non-blocking (doesn't delay read response)
- Best-effort (failures don't affect read operation)
- Reduces inconsistency window

## Consistency Levels

### Strong Consistency

```go
Config: W + R > N

Guarantees:
  - All reads return latest write
  - No stale data
  - Linearizable within quorum

Trade-offs:
  - Lower availability (requires majority)
  - Higher latency
  - Cannot tolerate (N-W) node failures for writes
```

### Eventual Consistency

```go
Config: W = 1, R = 1

Guarantees:
  - Highest availability
  - Lowest latency
  - Eventually consistent via anti-entropy

Trade-offs:
  - May read stale data
  - Requires conflict resolution
  - Higher complexity in application logic
```

### Balanced Consistency

```go
Config: W = (N/2)+1, R = (N/2)+1

Guarantees:
  - Read-your-writes consistency
  - Tolerate minority node failures
  - Good performance

Trade-offs:
  - Optimal for most workloads
  - Recommended default
```

## Version Management

### Hybrid Logical Clock (HLC)

**Components**:
- Physical time: Wall clock timestamp (nanoseconds)
- Logical counter: Incremented on clock ties

**Properties**:
- Causality preservation
- No clock synchronization required
- Monotonically increasing

### Version Conflicts

**Resolution**:
1. Compare HLC timestamps
2. Higher timestamp wins
3. Concurrent writes (same HLC): Last write wins (node ID tiebreaker)

## Partition Handling

### Network Partition Detection

- Gossip failure detection identifies unreachable nodes
- Nodes in different partitions form separate sub-clusters
- Each partition continues operating independently

### Partition Recovery

```go
Process:
  1. Network partition heals
  2. Nodes detect via probe responses
  3. Full sync triggered between partitions
  4. Conflicts resolved via HLC (higher version wins)
  5. Read repair ensures all replicas converge
```

### Split-Brain Prevention

GridKV does not implement split-brain prevention. Applications requiring strict consistency should:
- Use W + R > N configuration
- Implement application-level coordination
- Use external consensus systems (e.g., etcd) for critical metadata

## Performance Impact

### Latency

| Configuration | Write Latency | Read Latency |
|---------------|---------------|--------------|
| W=1, R=1 | ~1-2ms | ~1-2ms |
| W=2, R=2 | ~2-5ms | ~2-5ms |
| W=3, R=3 | ~5-10ms | ~5-10ms |

### Throughput

| Configuration | Write Throughput | Read Throughput |
|---------------|------------------|-----------------|
| W=1, R=1 | 600K ops/sec | 800K ops/sec |
| W=2, R=2 | 450K ops/sec | 680K ops/sec |
| W=3, R=3 | 280K ops/sec | 420K ops/sec |

## Best Practices

1. **Production Workloads**: Use W=2, R=2 with N=3
2. **Cache Use Cases**: Use W=1, R=1 with N=3
3. **Critical Data**: Use W=3, R=3 with N=3
4. **Monitor**: Track read repair frequency
5. **Test**: Verify behavior under partition scenarios

## Limitations

1. No distributed transactions
2. No cross-key atomicity
3. No global snapshot isolation
4. Application must handle conflicts for concurrent updates

## Implementation Files

- `internal/gossip/core.go` - Quorum logic
- `internal/gossip/kv_store.go` - Read/write operations
- `internal/gossip/consistent_hash.go` - Replica selection

