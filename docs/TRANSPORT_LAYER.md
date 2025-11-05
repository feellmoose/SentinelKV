# Transport Layer Design

## Overview

GridKV provides a pluggable transport layer supporting multiple network protocols. The transport layer abstracts network communication, allowing the gossip protocol to operate over different transports without modification.

## Transport Interface

### Core Abstractions

```go
type Transport interface {
    Dial(address string) (TransportConn, error)
    Listen(address string) (TransportListener, error)
}

type TransportConn interface {
    WriteDataWithContext(ctx context.Context, data []byte) error
    ReadDataWithContext(ctx context.Context) ([]byte, error)
    Close() error
    LocalAddr() net.Addr
    RemoteAddr() net.Addr
}

type TransportListener interface {
    Start() error
    HandleMessage(handler func([]byte) error) TransportListener
    Stop() error
    Addr() net.Addr
}
```

## Available Transports

### 1. TCP (Default)

**Implementation**: `transports/tcp/transport.go`

**Characteristics**:
- Reliable, ordered delivery
- Connection-oriented
- Flow control
- Zero external dependencies

**Optimizations**:
```go
conn.SetNoDelay(true)          // Disable Nagle's algorithm
conn.SetKeepAlive(true)        // Enable TCP keep-alive
conn.SetKeepAlivePeriod(30s)   // Keep-alive interval
conn.SetReadBuffer(32KB)       // Read buffer size
conn.SetWriteBuffer(32KB)      // Write buffer size
```

**Performance**:
- Latency: ~1-2ms (LAN)
- Throughput: ~500MB/s
- Concurrent connections: >1000

**Message Framing**:
```go
Frame format:
  [4-byte length prefix][message data]
  
Write:
  1. Write 4-byte length (big-endian)
  2. Write message data
  
Read:
  1. Read 4-byte length
  2. Validate length (<10MB max)
  3. Read exact message data
```

**Use Cases**:
- Production deployments (recommended)
- Reliable communication required
- LAN/WAN scenarios

### 2. UDP

**Implementation**: `internal/transport/udp.go`

**Characteristics**:
- Connectionless
- Lower latency than TCP
- No guaranteed delivery
- Suitable for gossip (eventual consistency)

**Optimizations**:
```go
Connection pooling:
  - Reuse UDP sockets
  - Avoid socket creation overhead
  - Per-destination connection tracking
  
Message size:
  - Max: 65,507 bytes (UDP limit)
  - Recommended: <1KB for reliability
```

**Performance**:
- Latency: ~0.5-1ms (LAN)
- Throughput: ~300MB/s
- Packet loss handling: Application-level retry

**Use Cases**:
- Low-latency gossip
- Eventual consistency acceptable
- LAN deployments

### 3. QUIC

**Implementation**: `internal/transport/quic.go`

**Characteristics**:
- Encrypted by default (TLS 1.3)
- Multiplexed streams
- Fast connection establishment
- UDP-based

**Dependencies**: `github.com/quic-go/quic-go`

**Optimizations**:
```go
Stream multiplexing:
  - One connection, multiple streams
  - Reduced handshake overhead
  - Better resource utilization
  
0-RTT connection:
  - Resume previous sessions
  - No handshake on reconnect
  - ~50% latency reduction
```

**Performance**:
- Latency: ~2-3ms (includes crypto)
- Throughput: ~400MB/s
- Connection setup: <1ms (0-RTT)

**Use Cases**:
- WAN deployments
- Security required
- Mobile clients
- High packet loss scenarios

### 4. gnet

**Implementation**: `internal/transport/gnet.go`

**Characteristics**:
- Event-driven architecture
- Zero-copy I/O
- Very high performance
- Linux/macOS only

**Dependencies**: `github.com/panjf2000/gnet/v2`

**Architecture**:
```go
Event-driven model:
  - Reactor pattern
  - Non-blocking I/O
  - Event loop per CPU core
  
Zero-copy:
  - Direct buffer access
  - No intermediate copies
  - Kernel bypass where possible
```

**Performance**:
- Latency: ~0.5-1ms
- Throughput: ~800MB/s
- Connections: >100K

**Use Cases**:
- Maximum throughput required
- High connection count (>10K)
- Linux/macOS deployments

## Connection Management

### Connection Pooling

**Architecture**:
```go
type ConnPool struct {
    transport   Transport
    address     string
    maxIdle     int       // Max idle connections
    maxConns    int       // Max total connections
    idleTimeout time.Duration
    
    mu        sync.Mutex
    idleConns []pooledTransportConn
    total     int
}
```

**Benefits**:
- Reduced connection overhead
- Amortized handshake costs
- Better resource utilization
- Configurable limits

**Configuration**:
```go
Default:
  - MaxConns: 1000
  - MaxIdle: 100
  - IdleTimeout: 90s
  
Tuning:
  - Increase MaxConns for high-concurrency
  - Increase MaxIdle for request bursts
  - Decrease IdleTimeout to save resources
```

### Connection Lifecycle

```go
Connection States:
  1. Created (Dial)
  2. Active (in-use)
  3. Idle (returned to pool)
  4. Closed (timeout or pool full)

Pool Management:
  - Get(): Reuse idle or create new
  - Put(): Return to pool or close
  - Background cleanup: Close expired connections
```

## Transport Selection

### Decision Matrix

| Requirement | Recommended Transport |
|-------------|----------------------|
| Production, reliable | TCP |
| Low latency, eventual consistency | UDP |
| Security required | QUIC |
| Maximum throughput | gnet |
| Development/testing | TCP |

### Performance Comparison

| Transport | Latency | Throughput | Reliability | Encryption |
|-----------|---------|------------|-------------|------------|
| TCP | 1-2ms | 500MB/s | High | No (TLS optional) |
| UDP | 0.5-1ms | 300MB/s | Best-effort | No |
| QUIC | 2-3ms | 400MB/s | High | Yes (built-in) |
| gnet | 0.5-1ms | 800MB/s | High | No |

## Message Protocol

### Framing

All transports use length-prefixed framing:

```
Wire Format:
  [4 bytes: message length (big-endian)][message data]

Benefits:
  - Simple parsing
  - No delimiter ambiguity
  - Efficient buffering
```

### Flow Control

**TCP**: Built-in flow control
**UDP**: Application-level (size limits)
**QUIC**: Built-in flow control per-stream
**gnet**: Event-driven backpressure

## Network Configuration

### Timeouts

```go
NetworkOptions:
  ReadTimeout:  5 * time.Second   // Read operation timeout
  WriteTimeout: 5 * time.Second   // Write operation timeout
  DialTimeout:  3 * time.Second   // Connection establishment
```

**Tuning Guidelines**:
- LAN: 1-5s timeouts
- WAN: 10-30s timeouts
- Satellite: 60s+ timeouts

### Buffer Sizes

```go
Recommended:
  ReadBuffer:  32 KB  // Small messages
  WriteBuffer: 32 KB  // Small messages
  
For large values:
  ReadBuffer:  256 KB
  WriteBuffer: 256 KB
```

## Error Handling

### Retry Logic

```go
Strategy: Exponential backoff with jitter

Attempt 1: Immediate
Attempt 2: 100ms + random(0-50ms)
Attempt 3: 200ms + random(0-100ms)
...
Max attempts: 3 (default)
```

### Connection Failures

```go
Handling:
  1. Detect connection failure
  2. Remove from pool
  3. Retry with new connection
  4. If all retries fail, mark node as suspect (SWIM)
```

## Security

### Transport-Level Security

**TCP**:
- Optional TLS wrapper (future enhancement)
- Application-level encryption (Ed25519 signing)

**QUIC**:
- Built-in TLS 1.3
- Always encrypted
- Certificate validation

**UDP**:
- No built-in security
- Relies on application-level signing

**gnet**:
- No built-in security
- Application-level encryption needed

### Message Authentication

All transports benefit from GridKV's message signing:
```go
Process:
  1. Serialize message
  2. Sign with Ed25519
  3. Attach signature
  4. Send via transport
  
Receiver:
  1. Receive via transport
  2. Verify signature
  3. Process if valid
```

## Monitoring

### Metrics

```go
Per-transport metrics:
  - Active connections
  - Idle connections
  - Bytes sent/received
  - Error rate
  - Latency percentiles
```

### Health Checks

```go
TCP health check:
  conn.SetReadDeadline(1ms)
  conn.Read(1 byte)
  
  Timeout = healthy
  Data/Error = investigate
```

## Transport Registry

### Registration

```go
func init() {
    transport.RegisterTransport("tcp", func() (Transport, error) {
        return NewTCPTransport(), nil
    })
}
```

### Dynamic Selection

```go
transport, err := transport.NewTransport("tcp")
if err != nil {
    // Transport not available (not imported)
}
```

## Future Enhancements

1. **TLS for TCP**: Optional encryption
2. **DTLS for UDP**: Encrypted UDP
3. **HTTP/3**: For web client integration
4. **WebSocket**: Browser client support
5. **Unix Domain Sockets**: Local-only optimization

## Implementation Files

- `internal/transport/transport.go` - Interface definitions
- `internal/transport/registry.go` - Transport registry
- `transports/tcp/` - TCP implementation
- `internal/transport/udp.go` - UDP implementation
- `internal/transport/quic.go` - QUIC implementation
- `internal/transport/gnet.go` - gnet implementation

## References

- TCP: RFC 793, RFC 7323 (TCP Extensions)
- UDP: RFC 768
- QUIC: RFC 9000
- gnet: Event-driven networking in Go

