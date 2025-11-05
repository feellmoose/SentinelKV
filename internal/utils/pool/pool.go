package pool

import "sync"

// ByteBufferPool provides pooled byte buffers to reduce memory allocations.
// Inspired by HashiCorp memberlist's buffer pooling strategy.
type ByteBufferPool struct {
	pool sync.Pool
}

// NewByteBufferPool creates a new buffer pool with the specified initial capacity.
//
// Parameters:
//   - initialCap: Initial capacity for new buffers (in bytes)
//
// Returns:
//   - *ByteBufferPool: A new buffer pool
func NewByteBufferPool(initialCap int) *ByteBufferPool {
	return &ByteBufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 0, initialCap)
				return &buf
			},
		},
	}
}

// Get retrieves a buffer from the pool.
// The buffer is reset to zero length but retains its capacity.
//
// Returns:
//   - *[]byte: A pooled buffer
func (p *ByteBufferPool) Get() *[]byte {
	buf := p.pool.Get().(*[]byte)
	*buf = (*buf)[:0] // Reset length but keep capacity
	return buf
}

// Put returns a buffer to the pool.
// The buffer's length is reset to zero before pooling.
//
// Parameters:
//   - buf: The buffer to return to the pool
func (p *ByteBufferPool) Put(buf *[]byte) {
	if buf != nil {
		*buf = (*buf)[:0] // Reset length but keep capacity
		p.pool.Put(buf)
	}
}

// Global buffer pools for common sizes
var (
	// SmallBufferPool for small messages (4KB)
	SmallBufferPool = NewByteBufferPool(4096)

	// MediumBufferPool for medium messages (8KB)
	MediumBufferPool = NewByteBufferPool(8192)

	// LargeBufferPool for large messages (64KB)
	LargeBufferPool = NewByteBufferPool(65536)
)
