# Scenario 6: Development Testing Configuration

## 📖 Scenario Description

This scenario provides the simplest configuration, suitable for local development, unit testing, and feature validation.

### Use Cases

✅ **Local Development**: Quick startup testing  
✅ **Unit Testing**: CI/CD integration  
✅ **Feature Validation**: New feature testing  
✅ **Rapid Prototyping**: POC validation  

---

## 🎯 Performance Expectations

| Metric | Performance |
|--------|-------------|
| **Startup Time** | < 100ms |
| **Reads** | 1-2M ops/s |
| **Writes** | 500K-1M ops/s |
| **Memory Usage** | < 512MB |

---

## ⚙️ Core Configuration

### Minimal Configuration

```go
ReplicaCount: 1,  // Single replica
WriteQuorum:  1,
ReadQuorum:   1,

MaxMemoryMB: 512,  // 512MB limit
```

**Design Philosophy**:
- Minimal resource usage
- Fastest startup speed
- Simplest configuration

### Simplified Network Configuration

```go
MaxConns:     100,  // Small connection pool
MaxIdle:      10,
ReadTimeout:  1 * time.Second,
```

**Why simplify?**
- Development environment doesn't need large connection pool
- Short timeout provides quick feedback on issues
- Reduce resource usage

---

## 🚀 Run Example

```bash
cd examples/06_dev_testing
go run main.go
```

### Expected Output

```
═══════════════════════════════════════════════════════
  GridKV Scenario 6: Development Testing Configuration
═══════════════════════════════════════════════════════

📦 Creating dev node...
✅ Node created successfully (time: 45ms)

═══════════════════════════════════════════════════════
  Basic Functionality Testing
═══════════════════════════════════════════════════════

1️⃣  Test Set/Get:
   ✅ Set successful
   ✅ Get successful: Alice

2️⃣  Test Delete:
   ✅ Create temporary data
   ✅ Verify data exists
   ✅ Delete successful
   ✅ Verify data deleted

3️⃣  Test Batch operations:
   ✅ Batch write 1000 keys
   ✅ Write speed: 850432 ops/s
   ✅ Batch read 1000 keys
   ✅ Read speed: 1245678 ops/s

4️⃣  Test Various data types:
   ✅ String
   ✅ Number
   ✅ JSON
   ✅ Binary
   ✅ Empty
   ✅ Large data (10KB)

5️⃣  Test Concurrent safety:
   ✅ Concurrent test completed
   ✅ 10 goroutines × 100 ops = 1000 operations
   ✅ Throughput: 654321 ops/s
```

---

## 💡 Usage Scenarios

### 1. Unit Testing

```go
func TestMyFeature(t *testing.T) {
    // Create test GridKV
    kv, err := gridkv.NewGridKV(&gridkv.GridKVOptions{
        LocalNodeID:  "test-node",
        LocalAddress: "localhost:15001",
        Storage: &storage.StorageOptions{
            Backend:     storage.BackendMemory,
            MaxMemoryMB: 128,  // 128MB sufficient
        },
        ReplicaCount: 1,
        WriteQuorum:  1,
        ReadQuorum:   1,
    })
    defer kv.Close()
    
    // Test code...
}
```

### 2. Local Development

```go
// main.go
func main() {
    kv, _ := gridkv.NewGridKV(&gridkv.GridKVOptions{
        LocalNodeID:  "dev",
        LocalAddress: "localhost:8001",
        Storage: &storage.StorageOptions{
            Backend:     storage.BackendMemory,
            MaxMemoryMB: 512,
        },
        ReplicaCount: 1,
    })
    defer kv.Close()
    
    // Development debugging...
}
```

### 3. CI/CD Integration

```yaml
# .github/workflows/test.yml
- name: Run Tests
  run: |
    go test ./tests/
    # Use single replica config for fast testing
```

---

## 🔍 Development Tips

### Quick Test Data

```go
// Quickly populate test data
func populateTestData(kv *gridkv.GridKV, n int) {
    for i := 0; i < n; i++ {
        key := fmt.Sprintf("test-%d", i)
        value := []byte(fmt.Sprintf("value-%d", i))
        kv.Set(context.Background(), key, value)
    }
}
```

### Log Debugging

```go
// Enable verbose logging
import "github.com/feellmoose/gridkv/internal/utils/logging"

logging.SetLevel(logging.LevelDebug)
```

### Memory Monitoring

```go
import "runtime"

func printMemStats() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    fmt.Printf("Alloc = %v MB\n", m.Alloc / 1024 / 1024)
}
```

---

## ⚠️ Considerations

### Not Suitable for Production

```
❌ Single replica no redundancy
❌ Data not persisted
❌ No fault tolerance
❌ Performance not optimized

Only for:
✅ Development
✅ Testing
✅ Validation
```

### Resource Limits

```
Default 512MB memory:
- Can store ~1 million 100-byte KVs
- Can store ~100 thousand 1KB KVs
- Can store ~5 thousand 10KB KVs

Exceeding limit will trigger errors
```

### Data Persistence

```
Memory backend data in RAM:
- Lost on restart ❌
- Not persisted ❌

Need persistence:
- Use production config
- Or implement persistence backend
```

---

## 🚀 From Development to Production

### Migration Checklist

```
Development → Production:

Configuration changes:
□ ReplicaCount: 1 → 3
□ MaxMemoryMB: 512 → 8192+
□ Backend: Memory → MemorySharded
□ ReadTimeout: 1s → 5s
□ WriteQuorum: 1 → 2
□ ReadQuorum: 1 → 2

Environment changes:
□ Single node → Multi-node cluster
□ Local network → Production network
□ Dev machine → Production servers

Monitoring changes:
□ No monitoring → Complete monitoring
□ No alerts → Alert configuration
□ No backup → Regular backups
```

---

## 📚 Related Resources

- [Quick Start](../../START_HERE.md)
- [Testing Guide](../../docs/TESTING_GUIDE.md)
- [Production Deployment](../01_high_concurrency/README.md)

---

**Use Case**: Development, testing, validation  
**Difficulty Level**: ⭐ (Very simple)  
**Recommendation**: ⭐⭐⭐⭐⭐ (Development essential)
