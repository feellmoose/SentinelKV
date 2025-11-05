# GridKV Technical Documentation

This directory contains detailed technical documentation for GridKV's architecture, protocols, and implementation.

---

## 📚 Documentation Index

### Core Architecture

| Document | Description |
|----------|-------------|
| [Architecture](ARCHITECTURE.md) | System architecture overview, component design, and data flow |
| [Gossip Protocol](GOSSIP_PROTOCOL.md) | Distributed communication protocol, SWIM failure detection |
| [Consistent Hashing](CONSISTENT_HASHING.md) | Data sharding algorithm, virtual nodes, rebalancing |
| [Consistency Model](CONSISTENCY_MODEL.md) | Quorum-based consistency, read repair, version control |

### Advanced Topics

| Document | Description |
|----------|-------------|
| [Hybrid Logical Clock](HYBRID_LOGICAL_CLOCK.md) | Distributed timestamp mechanism for causal consistency |
| [Storage Backends](STORAGE_BACKENDS.md) | Storage engine design, Memory vs MemorySharded |
| [Transport Layer](TRANSPORT_LAYER.md) | Network communication, TCP/UDP protocols, connection pooling |

### Quick Reference

| Document | Description |
|----------|-------------|
| [Quick Reference](QUICK_REFERENCE.md) | API reference, configuration options, usage examples |

---

## 🎯 Reading Guide

### For New Users

Start with these documents to understand GridKV:

1. **[Architecture](ARCHITECTURE.md)** - Understand overall system design
2. **[Quick Reference](QUICK_REFERENCE.md)** - Learn API usage
3. **[Consistency Model](CONSISTENCY_MODEL.md)** - Understand consistency guarantees

### For Advanced Users

Deep dive into implementation details:

1. **[Gossip Protocol](GOSSIP_PROTOCOL.md)** - Distributed protocol implementation
2. **[Consistent Hashing](CONSISTENT_HASHING.md)** - Data distribution algorithm
3. **[Hybrid Logical Clock](HYBRID_LOGICAL_CLOCK.md)** - Timestamp mechanism

### For Developers

Contributing to GridKV:

1. **[Architecture](ARCHITECTURE.md)** - Component boundaries
2. **[Storage Backends](STORAGE_BACKEND.md)** - Implementing new backends
3. **[Transport Layer](TRANSPORT_LAYER.md)** - Network layer details

---

## 📖 Document Descriptions

### Architecture.md
- **Purpose**: Comprehensive system architecture overview
- **Contents**: Component design, data flow, interaction patterns
- **Audience**: All users and developers

### Gossip Protocol.md
- **Purpose**: Distributed communication protocol specification
- **Contents**: SWIM protocol, failure detection, node lifecycle
- **Audience**: Advanced users, developers

### Consistent Hashing.md
- **Purpose**: Data distribution and sharding algorithm
- **Contents**: Hash ring, virtual nodes, rebalancing strategies
- **Audience**: Advanced users, developers

### Consistency Model.md
- **Purpose**: Data consistency guarantees and mechanisms
- **Contents**: Quorum reads/writes, read repair, versioning
- **Audience**: All users designing critical systems

### Hybrid Logical Clock.md
- **Purpose**: Distributed timestamp mechanism
- **Contents**: HLC implementation, causal consistency, clock synchronization
- **Audience**: Advanced users, researchers

### Storage Backends.md
- **Purpose**: Storage engine design and implementation
- **Contents**: Memory backend, MemorySharded, performance comparison
- **Audience**: Developers, performance engineers

### Transport Layer.md
- **Purpose**: Network communication layer
- **Contents**: TCP/UDP protocols, connection pooling, error handling
- **Audience**: Developers, network engineers

### Quick Reference.md
- **Purpose**: Fast API and configuration reference
- **Contents**: API examples, configuration options, common patterns
- **Audience**: All users

---

## 🔍 Finding Information

### By Topic

**Performance Tuning**:
- [Storage Backends](STORAGE_BACKENDS.md) - Backend selection
- [Consistent Hashing](CONSISTENT_HASHING.md) - Sharding configuration
- [Architecture](ARCHITECTURE.md) - System optimization

**Consistency and Reliability**:
- [Consistency Model](CONSISTENCY_MODEL.md) - Consistency levels
- [Gossip Protocol](GOSSIP_PROTOCOL.md) - Failure detection
- [Hybrid Logical Clock](HYBRID_LOGICAL_CLOCK.md) - Ordering guarantees

**Development**:
- [Architecture](ARCHITECTURE.md) - Component structure
- [Storage Backends](STORAGE_BACKENDS.md) - Implementing backends
- [Transport Layer](TRANSPORT_LAYER.md) - Network protocols

---

## 🤝 Contributing to Documentation

Documentation improvements are welcome! When contributing:

1. **Keep it clear**: Use simple language
2. **Add examples**: Include code samples
3. **Update index**: Reflect changes in this README
4. **Check links**: Ensure all links work
5. **Use diagrams**: ASCII art for visual clarity

---

## 📮 Feedback

Found an error or have suggestions? Please [open an issue](https://github.com/feellmoose/gridkv/issues).

---

**Last Updated**: 2025-11-05

