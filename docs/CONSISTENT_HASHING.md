# Consistent Hashing

## Overview

GridKV uses consistent hashing to distribute keys across cluster nodes with minimal redistribution when nodes join or leave.

## Algorithm

### Core Concept

Traditional hash-based distribution (key % N) requires redistributing most keys when N changes. Consistent hashing maps both keys and nodes to points on a hash ring, requiring redistribution of only K/N keys on average when a node is added or removed.

### Virtual Nodes

Each physical node is mapped to multiple positions on the hash ring (virtual nodes) to improve load distribution.

**Benefits**:
- Better load balancing
- Reduced variance in key distribution
- Smoother redistribution on membership changes

**Trade-off**: Memory overhead proportional to (nodes × replicas)

### Hash Function

GridKV uses xxHash (XXH64) for:
- High performance (faster than CRC32, FNV)
- Good distribution properties
- Low collision rate

## Implementation

### Data Structures

```go
type ConsistentHash struct {
    hashFunc Hash                // Hash function (xxHash)
    replicas int                 // Virtual nodes per physical node
    keys     []uint32            // Sorted hash values (for binary search)
    ring     map[uint32]string   // Hash → NodeID mapping
    nodes    map[string]bool     // Set of physical nodes
}
```

### Key Operations

#### Add Node

```go
Process:
  1. Generate replicas virtual nodes (e.g., 150 vnodes)
  2. For each vnode:
     - Compute hash(replica_index + node_name)
     - Add to ring map and keys array
  3. Sort keys array for binary search
  
Time Complexity: O(V log V) where V = total virtual nodes
```

#### Remove Node

```go
Process:
  1. Delete all virtual nodes for physical node
  2. Remove from ring map
  3. Rebuild and sort keys array
  
Time Complexity: O(V log V)
```

#### Get Node

```go
Process:
  1. Compute hash(key)
  2. Binary search for first vnode >= hash
  3. If not found, wrap to keys[0]
  4. Return physical node from ring map
  
Time Complexity: O(log V)
```

#### Get N Replicas

```go
Process:
  1. Compute hash(key)
  2. Binary search for starting position
  3. Walk clockwise collecting unique physical nodes
  4. Return first N unique nodes
  
Time Complexity: O(log V + N)
```

### Code Location

`internal/gossip/consistent_hash.go`:
- `NewConsistentHash()` - Constructor
- `Add()` - Add physical node
- `Remove()` - Remove physical node
- `Get()` - Find primary node for key
- `GetN()` - Find N replica nodes for key

## Configuration

### Virtual Nodes

```go
Default: 150 virtual nodes per physical node

Trade-offs:
  - More vnodes: Better distribution, higher memory
  - Fewer vnodes: Lower memory, potential hot spots
  
Recommended:
  - Small clusters (<10 nodes): 150-200 vnodes
  - Large clusters (>10 nodes): 100-150 vnodes
```

## Load Distribution

### Variance Analysis

With V virtual nodes per physical node:
- Expected variance: O(log N / V)
- Load imbalance: Typically <5% with V=150
- Worst case: ~10% for small clusters

### Key Redistribution

On node add/remove:
- Keys redistributed: 1/N (theoretical)
- Actual redistribution: ~(1/N) × (keys) with variance
- Affected replicas: Typically 1-2 per key

## Performance

### Operation Complexity

| Operation | Time Complexity | Space Complexity |
|-----------|----------------|------------------|
| Add Node | O(V log V) | O(V) |
| Remove Node | O(V log V) | O(V) |
| Get Primary | O(log V) | O(1) |
| Get N Replicas | O(log V + N) | O(N) |

### Actual Performance

- Add/Remove: ~1-2ms for 150 vnodes
- Lookup: ~100-200ns (binary search on sorted array)
- Memory: ~20KB per node with 150 vnodes

## Integration

### Replication

```go
Example: Write with replication_factor=3

1. Get replicas for key "user:123"
   replicas = GetN("user:123", 3)
   // Returns: ["node-1", "node-2", "node-3"]

2. Coordinator (first replica) writes locally

3. Replicate to remaining nodes
   for node in replicas[1:]:
       send_write(node, data)
```

### Failure Handling

```go
Node Failure:
  1. SWIM detects failure
  2. Node marked as DEAD
  3. Remove from hash ring
  4. Keys redistributed to next nodes in ring
  5. Read repair ensures consistency
```

## Comparison with Alternatives

### vs. Jump Hash

**Jump Hash Advantages**:
- Constant time O(1) for node lookup
- Minimal memory footprint

**Consistent Hash Advantages** (chosen):
- Supports weighted nodes (via vnode count)
- Predictable key distribution
- Better for heterogeneous clusters
- Easier to reason about data placement

### vs. Rendezvous Hashing

**Rendezvous Advantages**:
- No virtual nodes needed
- Simpler implementation

**Consistent Hash Advantages** (chosen):
- Faster lookups O(log V) vs O(N)
- Better for large clusters
- Industry standard, well-understood

## Best Practices

1. **Virtual Node Count**: Use 100-200 vnodes per node
2. **Hash Function**: xxHash provides best performance
3. **Replication Factor**: Minimum 3 for production
4. **Load Monitoring**: Track key distribution variance
5. **Rebalancing**: Consider periodic rebalancing for long-running clusters

## References

- Karger et al., "Consistent Hashing and Random Trees: Distributed Caching Protocols for Relieving Hot Spots on the World Wide Web"
- Amazon Dynamo Paper: "Dynamo: Amazon's Highly Available Key-value Store"

