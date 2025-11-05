# Gossip Protocol and Failure Detection

## Overview

GridKV implements a SWIM-based (Scalable Weakly-consistent Infection-style process group Membership) gossip protocol for distributed state management and failure detection.

## Protocol Components

### 1. SWIM Failure Detection

#### Algorithm Phases

**Alive State**
- Node is operating normally
- Responds to probes within timeout
- Participates in gossip

**Suspect State**
- Node failed to respond to direct probe
- Indirect probes initiated via other nodes
- Grace period before marking as dead

**Dead State**
- Node confirmed failed after suspect timeout
- Removed from hash ring
- State propagated to all members

#### Implementation

```go
State Transition:
  ALIVE → SUSPECT → DEAD
  
Timeouts:
  - failureTimeout: Time before marking node as suspect (default: 10s)
  - suspectTimeout: Grace period before marking as dead (default: 5s)
```

**Probe Mechanism**:
1. Direct probe to target node
2. If timeout, send indirect probes via 3 random nodes
3. If indirect probes fail, mark as suspect
4. After suspect timeout, mark as dead

#### Code Location

`internal/gossip/core.go`:
- `runFailureDetection()` - Main detection loop
- `initiateProbe()` - Direct probe mechanism
- `handleProbeRequest()` - Probe response handling
- `markNodeAliveFromProbe()` - State recovery

### 2. State Propagation

#### Gossip Dissemination

**Piggybacking Strategy**:
- Membership state piggybacked on regular messages
- Reduced network overhead
- Eventual consistency guarantee

**Gossip Interval**:
- Default: 1 second
- Configurable per deployment
- Trade-off: convergence speed vs network load

#### Anti-Entropy Synchronization

**Incremental Sync**:
```go
Process:
  1. Collect recent operations in ring buffer
  2. Send delta to peers periodically
  3. Apply operations with version checking
  4. Skip stale updates (OCC)
```

**Full Sync**:
```go
Trigger Conditions:
  - New node joining cluster
  - Node recovering from partition
  - Explicit sync request
  
Process:
  1. Request full snapshot from peer
  2. Clear local state
  3. Apply complete snapshot
  4. Resume normal operation
```

### 3. Hybrid Logical Clock (HLC)

#### Purpose

Provides causally-consistent timestamps without clock synchronization.

#### Algorithm

```
HLC Timestamp = (physical_time, logical_counter)

On local event:
  ts = max(physical_time, last_ts)
  if ts == last_ts:
    counter++
  else:
    counter = 0

On message receive:
  ts = max(physical_time, message.ts, last_ts)
  if ts == last_ts == message.ts:
    counter = max(counter, message.counter) + 1
  else if ts == last_ts:
    counter++
  else if ts == message.ts:
    counter = message.counter + 1
  else:
    counter = 0
```

#### Implementation

Location: `internal/gossip/core.go`
```go
type HLC struct {
    nodeID  string
    lastTs  int64  // Physical timestamp (Unix nanos)
    counter uint64 // Logical counter
}
```

### 4. Message Types

#### Core Messages

**CONNECT**
- Purpose: Node join notification
- Payload: NodeID, Address, Version, HLC
- Response: Cluster membership state

**CLUSTER_SYNC**
- Purpose: Membership state propagation
- Payload: All known nodes and their states
- Frequency: Every gossip interval

**PROBE_REQUEST / PROBE_RESPONSE**
- Purpose: Failure detection
- Payload: Requester ID, Target ID
- Timeout: Configurable (default 2s)

**CACHE_SYNC**
- Purpose: Data replication
- Payload: Operations (SET/DELETE)
- Acknowledgment: CACHE_SYNC_ACK

**READ_REQUEST / READ_RESPONSE**
- Purpose: Quorum reads
- Payload: Key, Version requirements
- Timeout: Configurable (default 5s)

### 5. Security

#### Message Signing

**Algorithm**: Ed25519 (Edwards-curve Digital Signature)

**Optimization**: Conditional signing
- Single-node clusters: Skip non-critical messages
- Multi-node clusters: Sign all inter-node messages
- Critical messages: Always signed

**Implementation**:
```go
func (gm *GossipManager) requiresSignature(msg *GossipMessage) bool {
    if len(peerPubkeys) > 1:
        return true  // Multi-node: always sign
    
    switch msg.Type:
        case CONNECT, CLUSTER_SYNC, PROBE:
            return true  // Critical: always sign
        default:
            return false // Local: skip signing
}
```

**Performance Impact**: 
- Conditional signing: +164% throughput improvement
- Reduced CPU usage by ~50% for single-node deployments

## Performance Characteristics

### Failure Detection

- Detection time: failureTimeout + suspectTimeout (default: 15s)
- False positive rate: Very low (indirect probes reduce false positives)
- Network overhead: O(log N) per node

### State Propagation

- Convergence time: O(log N) gossip rounds
- Message complexity: O(N) total messages
- Bandwidth: Configurable via gossip interval

### Scalability

- Cluster size: Tested up to 100 nodes
- Membership changes: O(log N) propagation
- State overhead: O(N) per node

## Configuration

```go
GossipOptions:
  - GossipInterval: 1s (state propagation frequency)
  - FailureTimeout: 10s (probe timeout)
  - SuspectTimeout: 5s (grace period)
  - ProbeInterval: 2s (health check frequency)
```

## References

- SWIM Paper: Das et al., "SWIM: Scalable Weakly-consistent Infection-style Process Group Membership Protocol"
- HLC Paper: Kulkarni et al., "Logical Physical Clocks and Consistent Snapshots in Globally Distributed Databases"

## Implementation Files

- `internal/gossip/core.go` - Main gossip manager
- `internal/gossip/network.go` - Network layer integration
- `internal/gossip/gridkv_gossip.proto` - Protocol buffer definitions

