# GridKV

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Performance](https://img.shields.io/badge/performance-7.4M_ops/s-brightgreen)]()

**高性能嵌入式分布式KV存储引擎**

[English](README.md) | 简体中文

</div>

---

## 📖 简介

GridKV 是一个高性能、强一致性的嵌入式分布式键值存储引擎。作为 Go SDK 直接集成到应用程序中，无需独立部署服务器，应用实例自动组成分布式集群。

### 核心特性

- **🚀 极致性能**: 并发读取 7.4M ops/s，写入 3.4M ops/s
- **🔒 强一致性**: 完整的 Gossip 协议 + 仲裁读写
- **⚡ 超低延迟**: P99 延迟 < 1ms，P50 延迟 135ns
- **🛡️ 高可靠性**: 全栈 Panic 保护，自动故障恢复
- **📦 零部署**: 嵌入式架构，自动集群管理
- **🎯 生产就绪**: 经过 23.7 亿次操作验证

---

## 🚀 快速开始

### 安装

```bash
go get github.com/feellmoose/gridkv
```

### 基础使用

```go
package main

import (
    "context"
    "log"
    
    "github.com/feellmoose/gridkv"
    "github.com/feellmoose/gridkv/internal/gossip"
    "github.com/feellmoose/gridkv/internal/storage"
)

func main() {
    // 创建实例
    kv, err := gridkv.NewGridKV(&gridkv.GridKVOptions{
        LocalNodeID:  "node-1",
        LocalAddress: "localhost:8001",
        
        Network: &gossip.NetworkOptions{
            Type:     gossip.TCP,
            BindAddr: "localhost:8001",
        },
        
        Storage: &storage.StorageOptions{
            Backend:     storage.BackendMemorySharded,
            MaxMemoryMB: 4096,
        },
        
        ReplicaCount: 3,
        WriteQuorum:  2,
        ReadQuorum:   2,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer kv.Close()
    
    ctx := context.Background()
    
    // 写入
    kv.Set(ctx, "user:1001", []byte("Alice"))
    
    // 读取
    value, _ := kv.Get(ctx, "user:1001")
    log.Printf("Value: %s", value)
    
    // 删除
    kv.Delete(ctx, "user:1001")
}
```

### 分布式集群

```go
// 节点 1 (种子节点)
node1, _ := gridkv.NewGridKV(&gridkv.GridKVOptions{
    LocalNodeID:  "node-1",
    LocalAddress: "localhost:8001",
    // ...其他配置
})

// 节点 2 (自动加入集群)
node2, _ := gridkv.NewGridKV(&gridkv.GridKVOptions{
    LocalNodeID:  "node-2",
    LocalAddress: "localhost:8002",
    SeedAddrs:    []string{"localhost:8001"},
    // ...其他配置
})

// 数据自动复制到所有节点
node1.Set(ctx, "key", []byte("value"))
value, _ := node2.Get(ctx, "key")  // ✅ 可以读到
```

---

## 📊 性能表现

### 基准测试 (Intel i7-12700H, 20 cores)

| 操作 | 吞吐量 | 延迟 |
|------|--------|------|
| **并发读取 (100 goroutines)** | **7.43M ops/s** | **135 ns** |
| **并发写入 (100 goroutines)** | **3.44M ops/s** | 290 ns |
| 单节点读取 | 2.82M ops/s | 355 ns |
| 单节点写入 | 915K ops/s | 1,108 ns |
| 5节点集群 | 4.50M ops/s | < 500 ns |

---

## 🎯 核心技术

### 分布式协议

- **Gossip 协议**: 去中心化集群管理，自动节点发现
- **SWIM 故障检测**: < 30秒自动检测和恢复节点故障
- **一致性哈希**: 数据均衡分布，支持平滑扩缩容
- **仲裁读写**: 可配置的一致性级别 (R+W > N 实现强一致)

### 存储引擎

- **MemorySharded**: 256 分片，CPU 核心数自适应
- **对象池优化**: sync.Pool 减少 GC 压力
- **深拷贝保护**: 返回数据安全可修改
- **内存限制**: 可配置的内存上限

### 网络层

- **TCP/UDP 双协议**: TCP 用于数据传输，UDP 用于 Gossip
- **连接池**: 连接复用，减少握手开销
- **自动重连**: 连接断开自动恢复
- **健康检查**: 定期检查连接健康状态

### 容错机制

- **全栈 Panic 保护**: API 层、Gossip 层、传输层全覆盖
- **自动故障恢复**: 连接断开、节点故障自动处理
- **读修复**: 检测到数据不一致时自动修复
- **优雅降级**: 部分节点故障时仍可提供服务

---

## 📁 配置场景

提供 6 个生产就绪的配置场景，涵盖不同使用需求：

| 场景 | 性能 | 适用场景 | 文档 |
|------|------|---------|------|
| 高并发缓存 | 5-7M ops/s | 微服务缓存、Session 存储 | [查看](examples/01_high_concurrency/) |
| 强一致性 | 2-3M ops/s | 金融交易、订单系统 | [查看](examples/02_strong_consistency/) |
| 高可用性 | 3-4M ops/s | 核心服务、24/7 系统 | [查看](examples/03_high_availability/) |
| 低延迟 | 6-7M ops/s, P99<1ms | 实时推荐、游戏状态 | [查看](examples/04_low_latency/) |
| 大规模集群 | 10-50M ops/s | 20+ 节点、数据中心 | [查看](examples/05_large_cluster/) |
| 开发测试 | 1-2M ops/s | 本地开发、单元测试 | [查看](examples/06_dev_testing/) |

**快速运行示例**:
```bash
cd examples/01_high_concurrency
go run main.go
```

---

## 🛠️ 配置说明

### 基础配置

```go
&gridkv.GridKVOptions{
    // 节点标识
    LocalNodeID:  "node-1",           // 节点唯一标识
    LocalAddress: "localhost:8001",   // 节点地址
    SeedAddrs:    []string{...},      // 种子节点地址（加入现有集群）
    
    // 存储配置
    Storage: &storage.StorageOptions{
        Backend:     storage.BackendMemorySharded, // 推荐使用分片存储
        MaxMemoryMB: 4096,                         // 内存限制 (MB)
    },
    
    // 一致性配置
    ReplicaCount: 3,  // 副本数量
    WriteQuorum:  2,  // 写入多少副本算成功
    ReadQuorum:   2,  // 读取多少副本进行比较
    
    // 网络配置
    Network: &gossip.NetworkOptions{
        Type:         gossip.TCP,
        BindAddr:     "localhost:8001",
        MaxConns:     1000,
        MaxIdle:      100,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 5 * time.Second,
    },
}
```

### 一致性级别

| 配置 | 一致性 | 性能 | 适用场景 |
|------|--------|------|---------|
| R=1, W=1 | 最终一致 | 最高 | 缓存、临时数据 |
| R=1, W=2 | 读优先 | 高 | 高并发读取场景 |
| R=2, W=2 | 强一致 | 中 | 金融、订单系统 |
| R=3, W=3 | 最强一致 | 较低 | 关键业务数据 |

**强一致性条件**: `ReadQuorum + WriteQuorum > ReplicaCount`

---

## 📚 技术文档

### 核心文档

- [架构设计](docs/ARCHITECTURE.md) - 系统整体架构
- [Gossip 协议](docs/GOSSIP_PROTOCOL.md) - 分布式通信协议
- [一致性模型](docs/CONSISTENCY_MODEL.md) - 数据一致性保证
- [一致性哈希](docs/CONSISTENT_HASHING.md) - 数据分片算法
- [混合逻辑时钟](docs/HYBRID_LOGICAL_CLOCK.md) - 因果一致性
- [存储后端](docs/STORAGE_BACKENDS.md) - 存储引擎设计
- [传输层](docs/TRANSPORT_LAYER.md) - 网络通信层
- [快速参考](docs/QUICK_REFERENCE.md) - API 快速参考

---

## 🎯 适用场景

### ✅ 推荐场景

- **微服务缓存**: API 响应缓存、数据库查询缓存
- **Session 存储**: 用户会话、登录状态
- **实时数据**: 排行榜、实时统计、热点数据
- **配置中心**: 应用配置、功能开关
- **游戏状态**: 游戏房间、玩家状态同步
- **实时推荐**: 个性化推荐、内容推送

### ⚠️ 不推荐场景

- **大规模持久化**: TB 级数据存储 (考虑 Cassandra/ScyllaDB)
- **复杂查询**: SQL JOIN、复杂事务 (考虑关系型数据库)
- **单机小流量**: < 1K ops/s (直接使用 Redis 更简单)

---

## 🔍 监控与运维

### 关键指标

```go
// 需要监控的指标
- 吞吐量 (ops/s)
- P50/P95/P99 延迟
- 内存使用率
- CPU 使用率
- GC 时间
- 错误率
- 节点健康状态
- 集群大小
```

### 告警建议

```go
// 建议的告警阈值
- 吞吐量下降 > 30%
- P99 延迟 > 10ms
- 内存使用 > 80%
- 错误率 > 1%
- 节点故障 > 1个
- GC 时间 > 100ms
```

---

## 🧪 测试

### 运行测试

```bash
# 所有测试
go test ./tests/

# 性能测试
go test -bench=. ./tests/

# 安全性测试
go test -run=Safety ./tests/

# Panic 恢复测试
go test -run=Panic ./tests/
```

### 测试覆盖

- **功能测试**: 基础操作、边界情况
- **性能测试**: 单节点、并发、集群
- **安全性测试**: 数据独立性、并发安全
- **可靠性测试**: Panic 恢复、故障容错
- **长时间测试**: 稳定性、内存泄漏

---

## 📦 生产部署

### 硬件推荐

**小型集群 (3-5 节点)**:
- CPU: 8 核心
- 内存: 16GB
- 网络: 1Gbps
- 适用: 1-10 万 QPS

**中型集群 (5-10 节点)**:
- CPU: 16 核心
- 内存: 32GB
- 网络: 10Gbps
- 适用: 10-100 万 QPS

**大型集群 (10-50 节点)**:
- CPU: 32 核心
- 内存: 64GB
- 网络: 25Gbps+
- 适用: 100 万+ QPS

### 最佳实践

1. **使用 MemorySharded 后端** - 性能最优
2. **合理设置副本数** - 通常 3 副本即可
3. **监控内存使用** - 避免超过限制
4. **定期健康检查** - 检测节点状态
5. **优雅关闭** - 使用 `defer kv.Close()`

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

### 开发环境

```bash
git clone https://github.com/feellmoose/gridkv.git
cd gridkv
go mod download
go test ./...
```

---

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE)

---

## 🙏 致谢

GridKV 使用了以下优秀的开源项目：

- [Protocol Buffers](https://github.com/protocolbuffers/protobuf-go) - 序列化
- [ants](https://github.com/panjf2000/ants) - Goroutine 池
- [xxhash](https://github.com/cespare/xxhash) - 哈希算法

---

## 📮 联系方式

- **GitHub**: https://github.com/feellmoose/gridkv
- **Issues**: https://github.com/feellmoose/gridkv/issues

---

<div align="center">

**GridKV** - 高性能分布式 KV 存储，为现代应用而生

*Performance · Consistency · Reliability*

</div>
