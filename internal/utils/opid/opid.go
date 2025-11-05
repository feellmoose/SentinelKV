package opid

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Generator generates unique operation IDs with minimal allocations.
// Format: <nodeID>:<timestamp>:<counter>
//
// This implementation is optimized for high-throughput scenarios:
//   - Uses buffer pooling to avoid allocations
//   - Avoids fmt.Sprintf which is expensive
//   - Thread-safe with atomic counter
type Generator struct {
	nodeID  string
	counter uint64
	pool    sync.Pool
}

// NewGenerator creates a new operation ID generator for the specified node.
//
// Parameters:
//   - nodeID: Unique identifier for this node
//
// Returns:
//   - *Generator: A new operation ID generator
func NewGenerator(nodeID string) *Generator {
	return &Generator{
		nodeID: nodeID,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 64) // Pre-allocated buffer
			},
		},
	}
}

// Generate creates a unique operation ID.
// The ID format is: nodeID:timestamp:counter
//
// This method is thread-safe and optimized for performance.
//
// Returns:
//   - string: A unique operation ID
func (g *Generator) Generate() string {
	buf := g.pool.Get().([]byte)[:0] // Reset buffer

	// Append node ID
	buf = append(buf, g.nodeID...)
	buf = append(buf, ':')

	// Append timestamp
	buf = strconv.AppendInt(buf, time.Now().UnixNano(), 10)
	buf = append(buf, ':')

	// Append counter
	counter := atomic.AddUint64(&g.counter, 1)
	buf = strconv.AppendUint(buf, counter, 10)

	// Convert to string (this allocates, but it's one allocation vs many in Sprintf)
	result := string(buf)

	// Return buffer to pool
	g.pool.Put(buf)

	return result
}
