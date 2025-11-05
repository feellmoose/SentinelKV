package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// TransportConn defines the basic operations for pluggable network connections.
type TransportConn interface {
	WriteDataWithContext(ctx context.Context, data []byte) error
	ReadDataWithContext(ctx context.Context) ([]byte, error)
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

// HealthCheckable is an optional interface for connections that support health checking
type HealthCheckable interface {
	HealthCheck() error
}

// Transport defines the functionality required for a pluggable network layer.
type Transport interface {
	Dial(address string) (TransportConn, error)
	Listen(address string) (TransportListener, error)
}

// TransportListener defines the basic operations for a listener.
type TransportListener interface {
	Start() error
	HandleMessage(handler func(message []byte) error) TransportListener
	Stop() error
	Addr() net.Addr
}

// pooledConn wraps a TransportConn with last used timestamp.
type pooledTransportConn struct {
	conn     TransportConn
	lastUsed time.Time
}

// ConnPool manages a pool of TransportConn connections.
type ConnPool struct {
	transport   Transport
	address     string
	maxIdle     int
	maxConns    int
	idleTimeout time.Duration

	mu        sync.Mutex
	cond      *sync.Cond // Used to wake up Goroutines waiting for a connection in Get()
	idleConns []pooledTransportConn
	total     int // Total number of connections, both idle and in-use
	closed    bool
}

// NewConnPool creates a new connection pool.
func NewConnPool(transport Transport, address string, maxIdle, maxConns int, idleTimeout time.Duration) *ConnPool {
	pool := &ConnPool{
		transport:   transport,
		address:     address,
		maxIdle:     maxIdle,
		maxConns:    maxConns,
		idleTimeout: idleTimeout,
		idleConns:   make([]pooledTransportConn, 0, maxIdle),
	}
	// Initialize the condition variable for waiting on available connections.
	pool.cond = sync.NewCond(&pool.mu)

	// Start the background cleanup Goroutine.
	go pool.cleanupLoop()
	return pool
}

// cleanupLoop periodically cleans up expired idle connections.
func (p *ConnPool) cleanupLoop() {
	ticker := time.NewTicker(p.idleTimeout / 2)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return
		}
		now := time.Now()
		// Use slice trick for efficient in-place cleanup
		active := p.idleConns[:0]
		for _, pc := range p.idleConns {
			if now.Sub(pc.lastUsed) < p.idleTimeout {
				active = append(active, pc)
			} else {
				// Close the expired connection and decrement the total count.
				_ = pc.conn.Close()
				p.total--
			}
		}
		p.idleConns = active
		p.mu.Unlock()
	}
}

// Get returns an active TransportConn. If the pool is exhausted, it waits until one is available.
// This method is Context-aware, supporting cancellation/timeout while waiting.
func (p *ConnPool) Get(ctx context.Context) (TransportConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		// 1. Check if the connection pool is closed
		if p.closed {
			return nil, errors.New("connection pool closed")
		}

		now := time.Now()
		// 2. Try to get a connection from the idle list
		for len(p.idleConns) > 0 {
			// Pop connection from the end
			pc := p.idleConns[len(p.idleConns)-1]
			p.idleConns = p.idleConns[:len(p.idleConns)-1]

			// Check for expiration
			if now.Sub(pc.lastUsed) > p.idleTimeout {
				_ = pc.conn.Close()
				p.total--
				continue // Try the next one
			}

			// OPTIMIZATION: Health check if supported
			if healthChecker, ok := pc.conn.(HealthCheckable); ok {
				if err := healthChecker.HealthCheck(); err != nil {
					// Connection is broken, close and try next
					_ = pc.conn.Close()
					p.total--
					continue
				}
			}

			return pc.conn, nil // Found a healthy connection
		}

		// 3. Try to create a new connection (if max connections not reached)
		if p.total < p.maxConns {
			p.total++ // Pre-increment total to reserve a spot
			p.mu.Unlock()

			// Critical optimization: Dialing is done outside the lock to prevent blocking
			conn, err := p.transport.Dial(p.address)

			p.mu.Lock() // Re-acquire the lock
			if err != nil {
				// Dial failed, revert the total count
				p.total--
				return nil, err
			}
			return conn, nil
		}

		// 4. Pool exhausted (total == maxConns and no idle connections). Wait.
		// Check context for expiration before waiting
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// CRITICAL FIX: Use a goroutine to wake up on context cancellation
		// without this, the goroutine would wait forever even if context is cancelled
		waitDone := make(chan struct{})
		go func() {
			<-ctx.Done()
			select {
			case <-waitDone:
				// Wait finished normally, do nothing
			default:
				p.mu.Lock()
				p.cond.Broadcast() // Wake up all waiters
				p.mu.Unlock()
			}
		}()

		// Block the current Goroutine using Cond.Wait() until a Signal or Broadcast is received
		p.cond.Wait()

		// Signal the context watcher to exit
		select {
		case <-waitDone:
			// Already closed
		default:
			close(waitDone)
		}

		// Check context again after waking up
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// The loop restarts after being signaled
	}
}

// Put returns a connection back to the pool.
func (p *ConnPool) Put(conn TransportConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If the pool is closed or the idle list is full, close the connection and decrement the total count.
	if p.closed || len(p.idleConns) >= p.maxIdle {
		_ = conn.Close()
		p.total--
		p.cond.Signal() // Notify a waiter that total count has decreased (allowing new Dial attempts)
		return
	}

	// Add connection to the pool and update its last used time.
	p.idleConns = append(p.idleConns, pooledTransportConn{
		conn:     conn,
		lastUsed: time.Now(),
	})

	// Critical optimization: Notify a Goroutine waiting in Get() that a new connection is available.
	p.cond.Signal()
}

// Invalidate closes a connection and decrements the total count.
// This should be called instead of Put() when a retrieved connection is found to be broken or unusable.
func (p *ConnPool) Invalidate(conn TransportConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	_ = conn.Close()
	// Decrement total count to free up space for a new connection.
	if p.total > 0 {
		p.total--
		// Critical optimization: Notify a waiter that the total count has decreased, allowing a new connection to be created.
		p.cond.Signal()
	}
}

// Close closes all connections in the pool and stops the cleanup routine.
func (p *ConnPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}
	p.closed = true // Signal cleanupLoop to exit

	// Wake up all waiting Goroutines so they can see p.closed = true and return an error
	p.cond.Broadcast()

	// Close all current idle connections.
	for _, pc := range p.idleConns {
		_ = pc.conn.Close()
	}
	p.idleConns = nil
	p.total = 0
	return nil
}
