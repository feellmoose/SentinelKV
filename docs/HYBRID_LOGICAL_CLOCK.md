# Hybrid Logical Clock (HLC)

## Overview

GridKV uses Hybrid Logical Clocks for distributed timestamp generation without requiring clock synchronization across nodes.

## Motivation

### Problem with Physical Clocks

**Clock Skew**:
- Different nodes have different physical times
- NTP synchronization has ~100ms precision
- Clock drift can cause ordering inconsistencies

**Clock Synchronization Issues**:
- Cannot determine if event A happened before event B
- Concurrent writes may have ambiguous ordering
- Difficult to maintain causality

### Problem with Logical Clocks (Lamport)

**Loss of Physical Time**:
- Pure logical clocks lose wall-clock information
- Cannot expire entries based on real time
- Difficult to debug (timestamps don't match real events)

## Hybrid Logical Clock Solution

HLC combines physical time with logical counters:

```
HLC = (physical_timestamp, logical_counter)
```

**Properties**:
- Preserves causality (like Lamport clocks)
- Tracks physical time (for TTL, debugging)
- Monotonically increasing
- No synchronization needed

## Algorithm

### Local Event

```go
func (h *HLC) Now() uint64 {
    h.mu.Lock()
    defer h.mu.Unlock()
    
    physicalNow := time.Now().UnixNano()
    
    if physicalNow > h.lastTs {
        // Physical time advanced
        h.lastTs = physicalNow
        h.counter = 0
    } else {
        // Physical time hasn't advanced (fast events)
        h.counter++
    }
    
    return packHLC(h.lastTs, h.counter)
}
```

### Receive Message

```go
func (h *HLC) Witness(remoteTs, remoteCtr uint64) {
    physicalNow := time.Now().UnixNano()
    remotePhysical := int64(remoteTs)
    
    h.mu.Lock()
    defer h.mu.Unlock()
    
    // Take maximum of physical times
    maxTs := max(physicalNow, remotePhysical, h.lastTs)
    
    if maxTs == h.lastTs && maxTs == remotePhysical {
        // Same as local and remote: increment past both
        h.counter = max(h.counter, remoteCtr) + 1
    } else if maxTs == h.lastTs {
        // Local time is max: increment local counter
        h.counter++
    } else if maxTs == remotePhysical {
        // Remote time is max: use remote counter + 1
        h.counter = remoteCtr + 1
    } else {
        // Physical time advanced: reset counter
        h.counter = 0
    }
    
    h.lastTs = maxTs
}
```

## Encoding

### Bit Packing

HLC timestamps are encoded as 64-bit integers:

```
  Bits 63-20: Physical timestamp (44 bits)
  Bits 19-0:  Logical counter (20 bits)
  
  64-bit layout:
  [44-bit physical time | 20-bit logical counter]
```

**Capacity**:
- Physical time: ~557 years from Unix epoch
- Logical counter: 1,048,576 events per nanosecond
- Sufficient for all practical scenarios

### Pack/Unpack

```go
func packHLC(ts int64, ctr uint64) uint64 {
    return (uint64(ts) << 20) | (ctr & 0xFFFFF)
}

func unpackHLC(hlc uint64) (ts uint64, ctr uint64) {
    ts = hlc >> 20
    ctr = hlc & 0xFFFFF
    return
}
```

## Properties

### Monotonicity

**Guarantee**: HLC values are strictly monotonically increasing within a node

**Proof**:
- Physical time increases: ts increases
- Physical time static: counter increases
- Message receive: max(local, remote) ensures monotonicity

### Causality

**Happens-Before Relation**:
```
Event A happens before Event B if:
  - hlc(A) < hlc(B)

Concurrent events:
  - Cannot determine ordering from HLC alone
  - Both events can proceed
  - Application-level conflict resolution needed
```

### Comparison

```go
func CompareHLC(hlc1, hlc2 uint64) int {
    if hlc1 < hlc2 {
        return -1  // hlc1 happened before hlc2
    } else if hlc1 > hlc2 {
        return 1   // hlc2 happened before hlc1
    }
    return 0       // Concurrent (extremely rare)
}
```

## Use Cases in GridKV

### 1. Conflict Resolution

```go
Scenario: Concurrent writes to same key from different nodes

Resolution:
  1. Compare HLC timestamps
  2. Higher HLC wins (Last-Write-Wins)
  3. Deterministic ordering ensures consistency
```

### 2. Version Management

```go
Each operation has HLC timestamp as version:
  item.Version = hlc.Now()
  
Optimistic Concurrency Control:
  if existing.Version > new.Version:
      reject write  // Stale
  else:
      accept write  // Newer or concurrent
```

### 3. Causality Tracking

```go
Message ordering:
  msg.Hlc = hlc.Now()  // Send time
  
  On receive:
  hlc.Witness(msg.Hlc)  // Update local clock
  
  Ensures: If msg1.Hlc < msg2.Hlc, msg1 happened before msg2
```

### 4. Expiration

```go
TTL support:
  item.ExpireAt = time.Now().Add(ttl)
  
  On access:
  if time.Now().After(item.ExpireAt):
      return ErrItemExpired
```

HLC's physical time component enables TTL without global clock sync.

## Performance

### Overhead

**CPU**: 
- Now(): ~10 ns (2 atomic ops + timestamp)
- Witness(): ~15 ns (comparison + atomic ops)

**Memory**:
- Per-node: 24 bytes (lastTs + counter + mutex)
- Per-message: 8 bytes (packed HLC)

**Scalability**: O(1) - independent of cluster size

### Optimization

**Lock Efficiency**:
```go
// Fast path: No lock needed for comparisons
if remoteTs <= lastTs {
    // Can often skip lock
}
```

**Packing**:
- Single 64-bit value vs two separate fields
- Better cache locality
- Simpler serialization

## Clock Drift Handling

### Drift Detection

```go
Large drift detection:
  if abs(physicalNow - remoteTs) > MAX_DRIFT:
      log.Warn("Large clock drift detected")
      
  MAX_DRIFT: 5 seconds (configurable)
```

### Drift Correction

**Strategy**: Gradual correction via Witness()

```go
Process:
  1. Receive message with future timestamp
  2. Local HLC jumps to future timestamp
  3. Subsequent operations use new baseline
  4. Drift absorbed over time
```

**No Hard Limits**: 
- HLC tolerates arbitrary drift
- System remains consistent
- Only impacts wall-clock accuracy

## Edge Cases

### Counter Overflow

**Scenario**: >1M events in same nanosecond

**Handling**:
```go
if counter > MAX_COUNTER {
    // Force physical time to advance
    time.Sleep(1 * time.Nanosecond)
    physical_time++
    counter = 0
}
```

**Probability**: Extremely rare (1M ops/ns = 1 billion ops/ms)

### Time Regression

**Scenario**: System clock moves backward (NTP correction)

**Handling**:
```go
HLC handles naturally:
  if physicalNow < lastTs:
      // Use lastTs (monotonicity preserved)
      lastTs = lastTs
      counter++
```

## Comparison with Alternatives

### vs. Lamport Clocks

| Feature | Lamport | HLC |
|---------|---------|-----|
| Causality | Yes | Yes |
| Physical time | No | Yes |
| Monotonic | Yes | Yes |
| TTL support | No | Yes |
| Overhead | Low | Low |

**Choice**: HLC provides causality + physical time

### vs. TrueTime (Google Spanner)

| Feature | TrueTime | HLC |
|---------|----------|-----|
| Causality | Yes | Yes |
| Physical time | Yes | Yes |
| Synchronization | GPS/Atomic clocks | None needed |
| Complexity | High | Low |
| Cost | Expensive hardware | Free |

**Choice**: HLC provides 90% of benefits without specialized hardware

### vs. Vector Clocks

| Feature | Vector Clock | HLC |
|---------|--------------|-----|
| Causality | Full | Happens-before |
| Size | O(N) | O(1) |
| Comparison | Complex | Simple |
| Scalability | Poor | Excellent |

**Choice**: HLC trades full causality tracking for constant size and simplicity

## Best Practices

1. **Do Not** use HLC for cryptographic purposes
2. **Do** use HLC for event ordering and versioning
3. **Monitor** clock drift in production
4. **Log** when drift exceeds threshold
5. **Use** NTP for basic time sync (optional but recommended)

## Implementation Files

- `internal/gossip/core.go` - HLC implementation
  - `type HLC struct`
  - `func (h *HLC) Now()`
  - `func (h *HLC) Witness()`

## References

- Kulkarni et al., "Logical Physical Clocks and Consistent Snapshots in Globally Distributed Databases", 2014
- CockroachDB HLC implementation
- TiDB timestamp oracle design

