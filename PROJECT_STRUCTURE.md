# GridKV Project Structure

**Version**: 1.0  
**Last Updated**: 2025-11-05

---

## 📁 Directory Structure

```
GridKV/
├── README.md                   # Main documentation (Chinese)
├── README_EN.md                # Main documentation (English)
├── LICENSE                     # MIT License
├── go.mod                      # Go module definition
├── go.sum                      # Go dependencies checksums
├── gridkv.go                   # Main API entry point
│
├── docs/                       # Technical documentation
│   ├── README.md              # Documentation index
│   ├── ARCHITECTURE.md        # System architecture
│   ├── GOSSIP_PROTOCOL.md     # Distributed protocol
│   ├── CONSISTENT_HASHING.md  # Data sharding
│   ├── CONSISTENCY_MODEL.md   # Consistency guarantees
│   ├── HYBRID_LOGICAL_CLOCK.md# Distributed timestamps
│   ├── STORAGE_BACKENDS.md    # Storage engines
│   ├── TRANSPORT_LAYER.md     # Network layer
│   └── QUICK_REFERENCE.md     # API reference
│
├── examples/                   # Configuration scenarios
│   ├── README.md              # Scenarios overview
│   ├── run_all.sh             # Run all examples
│   ├── 01_high_concurrency/   # High concurrency config
│   ├── 02_strong_consistency/ # Strong consistency config
│   ├── 03_high_availability/  # High availability config
│   ├── 04_low_latency/        # Low latency config
│   ├── 05_large_cluster/      # Large cluster config
│   └── 06_dev_testing/        # Development config
│
├── internal/                   # Internal packages
│   ├── gossip/                # Gossip protocol implementation
│   │   ├── api_simple.go      # Simple API layer
│   │   ├── consistent_hash.go # Consistent hashing
│   │   ├── failure_detection.go# SWIM failure detection
│   │   ├── gossip_manager.go  # Core gossip manager
│   │   ├── gridkv_gossip.proto# Protobuf definitions
│   │   ├── gridkv_gossip.pb.go# Generated protobuf code
│   │   ├── kv_store.go        # KV store interface
│   │   ├── message_handling.go# Message processing
│   │   ├── network.go         # Network abstraction
│   │   ├── replication.go     # Data replication
│   │   ├── storage_bridge.go  # Storage adapter
│   │   ├── sync.go            # State synchronization
│   │   └── type_converters.go # Type conversion utilities
│   │
│   ├── storage/               # Storage backends
│   │   ├── storage.go         # Storage interface
│   │   ├── registry.go        # Backend registry
│   │   ├── init.go            # Initialization
│   │   ├── memory.go          # Memory backend
│   │   ├── memory_sharded.go  # Sharded memory backend
│   │   ├── object_pool.go     # Object pooling
│   │   └── gossip_sync.go     # Gossip sync utilities
│   │
│   ├── transport/             # Network transport
│   │   ├── transport.go       # Transport interface
│   │   ├── registry.go        # Transport registry
│   │   ├── tcp.go             # TCP implementation
│   │   ├── udp.go             # UDP implementation
│   │   ├── gnet.go            # gnet implementation
│   │   └── gnet_metrics.go    # gnet metrics
│   │
│   └── utils/                 # Utility packages
│       ├── crypto/            # Cryptography utilities
│       │   └── ed25519.go     # Ed25519 signing
│       ├── hlc/               # Hybrid Logical Clock
│       │   └── hlc.go         # HLC implementation
│       ├── logging/           # Logging utilities
│       │   └── logging.go     # Logger implementation
│       ├── opid/              # Operation IDs
│       │   └── opid.go        # OPID generation
│       └── pool/              # Generic pooling
│           └── pool.go        # Pool implementation
│
├── tests/                     # Test suites
│   ├── init_test.go           # Test initialization
│   ├── test_helpers.go        # Test utilities
│   ├── benchmark_test.go      # Performance benchmarks
│   ├── benchmark_distributed_test.go # Cluster benchmarks
│   ├── safety_test.go         # Data safety tests
│   ├── panic_recovery_test.go # Panic recovery tests
│   ├── stability_chaos_test.go# Chaos engineering tests
│   ├── stability_long_running_test.go # Long-running tests
│   ├── simple_cluster_performance_test.go # Cluster perf
│   ├── progressive_massive_test.go # Massive scale tests
│   ├── transport_chaos_test.go # Network chaos tests
│   ├── transport_production_test.go # Production tests
│   ├── transport_24h_stability_test.go # 24h stability
│   ├── run_benchmarks.sh      # Benchmark runner
│   ├── run_distributed_tests.sh # Distributed test runner
│   └── run_production_tests.sh # Production test runner
│
└── cmd/                       # Command-line tools
    ├── bench-all/             # All benchmarks
    ├── bench-storage/         # Storage benchmarks
    ├── bench-transport/       # Transport benchmarks
    └── transport_monitor/     # Transport monitoring tool
```

---

## 📦 Core Packages

### gridkv (root)
**Purpose**: Main API entry point

**Key Types**:
- `GridKV`: Main distributed KV store
- `GridKVOptions`: Configuration options

**Key Functions**:
- `NewGridKV()`: Create new instance
- `Set()`: Write key-value pair
- `Get()`: Read value by key
- `Delete()`: Remove key
- `Close()`: Shutdown instance

---

### internal/gossip
**Purpose**: Distributed protocol implementation

**Components**:
- `GossipManager`: Core protocol coordinator
- `ConsistentHash`: Data distribution
- `FailureDetector`: SWIM-based failure detection
- `Network`: Network abstraction
- `Replicator`: Data replication

**Key Features**:
- Membership management
- Failure detection and recovery
- State synchronization
- Quorum-based operations

---

### internal/storage
**Purpose**: Storage backend abstraction

**Backends**:
- `Memory`: Single-lock memory backend
- `MemorySharded`: Multi-shard memory backend (recommended)

**Features**:
- Pluggable backend system
- Object pooling optimization
- Memory limits
- TTL support

---

### internal/transport
**Purpose**: Network communication layer

**Implementations**:
- `TCP`: Reliable data transfer
- `UDP`: Low-latency gossip messages
- `gnet`: High-performance alternative

**Features**:
- Connection pooling
- Auto-reconnection
- Health checks
- Metrics collection

---

### internal/utils
**Purpose**: Common utilities

**Modules**:
- `crypto`: Ed25519 signatures
- `hlc`: Hybrid Logical Clock
- `logging`: Structured logging
- `opid`: Operation IDs
- `pool`: Generic object pooling

---

## 🧪 Testing Structure

### Test Types

**Unit Tests**:
- Individual component testing
- Mock dependencies
- Fast execution

**Integration Tests**:
- Multi-component testing
- Real dependencies
- Cluster formation

**Performance Tests**:
- Throughput benchmarks
- Latency measurements
- Scalability tests

**Reliability Tests**:
- Panic recovery
- Fault tolerance
- Long-running stability
- Chaos engineering

---

## 📚 Documentation Structure

### README Files
- **README.md**: Main documentation (Chinese)
- **README_EN.md**: Main documentation (English)
- **docs/README.md**: Technical docs index
- **examples/README.md**: Configuration scenarios

### Technical Docs
- **Architecture**: System design
- **Protocols**: Distributed algorithms
- **Implementation**: Code details
- **API Reference**: Usage guide

---

## 🔍 Code Organization Principles

### 1. Clear Separation
- API layer (`gridkv.go`)
- Protocol layer (`internal/gossip/`)
- Storage layer (`internal/storage/`)
- Network layer (`internal/transport/`)
- Utilities (`internal/utils/`)

### 2. Interface-Driven
- All major components define interfaces
- Easy to mock for testing
- Pluggable implementations

### 3. Dependency Direction
```
gridkv → gossip → storage
              ↓
           transport
              ↓
            utils
```

### 4. Package Principles
- `internal/`: Not importable by external packages
- Clear API boundaries
- Minimal inter-package dependencies

---

## 🚀 Getting Started

### For Users
1. Read `README.md` or `README_EN.md`
2. Check `examples/` for configuration
3. Refer to `docs/QUICK_REFERENCE.md`

### For Developers
1. Read `docs/ARCHITECTURE.md`
2. Explore `internal/` packages
3. Run tests in `tests/`
4. Study examples in `examples/`

---

## 📝 Code Style

### Comments
- All comments in English
- Clear and concise
- Include examples where helpful

### Naming
- Descriptive names
- Follow Go conventions
- Package-level documentation

### Structure
- One package per directory
- Keep files focused and small
- Group related functionality

---

## 🔄 Build & Development

### Build Tags
None currently used, but structure supports conditional compilation for optional backends.

### Dependencies
- Minimal external dependencies
- Core: Protocol Buffers, ants, xxhash
- All dependencies in `go.mod`

### Tools
- Standard Go toolchain
- Protocol Buffer compiler for `.proto` files
- No special build requirements

---

## 📮 Contact

- **GitHub**: https://github.com/feellmoose/gridkv
- **Issues**: https://github.com/feellmoose/gridkv/issues
- **Docs**: https://github.com/feellmoose/gridkv/tree/main/docs

---

**GridKV** - High-Performance Distributed KV Storage

