package storage

import (
	"fmt"
	"time"
)

type StoredItem struct {
	ExpireAt time.Time // When the item should be considered expired. Used for TTL and tombstone grace period.
	Version  int64
	Value    []byte
}

// Storage defines the interface for a key-value store with versioning, TTL support, and synchronization.
type Storage interface {
	// Basic storage operations
	Set(key string, item *StoredItem) error
	Get(key string) (*StoredItem, error)
	Delete(key string, version int64) error
	Keys() []string
	Clear() error
	Close() error

	// Gossip synchronization methods (required for distributed operation)
	GetSyncBuffer() ([]*CacheSyncOperation, error)
	GetFullSyncSnapshot() ([]*FullStateItem, error)
	ApplyIncrementalSync(operations []*CacheSyncOperation) error
	ApplyFullSyncSnapshot(snapshot []*FullStateItem, snapshotTS time.Time) error

	// Monitoring and statistics
	Stats() StorageStats
}

// CacheSyncOperation models a single delta change for incremental sync.
type CacheSyncOperation struct {
	Key     string
	Version int64
	Type    string      // "SET" or "DELETE"
	Data    *StoredItem // only present for SET
}

// FullStateItem models a complete key-value entry for full sync.
type FullStateItem struct {
	Key     string
	Version int64
	Item    *StoredItem
}

// ErrItemNotFound is returned when the requested item is not found in the store.
var ErrItemNotFound = fmt.Errorf("item not found")

// ErrVersionMismatch is returned when the provided version does not match the stored version.
var ErrVersionMismatch = fmt.Errorf("version mismatch")

// ErrItemExpired is returned when the requested item has expired based on its TTL.
var ErrItemExpired = fmt.Errorf("item has expired")

// StorageBackendType identifies the storage backend type
type StorageBackendType string

const (
	BackendMemory        StorageBackendType = "Memory"        // High-performance lock-free cache (sync.Map, 600K-700K ops/sec)
	BackendMemorySharded StorageBackendType = "MemorySharded" // Ultra-high-performance sharded cache (256 shards, 1M-1.5M+ ops/sec)
)

// StorageOptions configures the storage backend
type StorageOptions struct {
	Backend StorageBackendType

	// Memory cache options
	MaxMemoryMB int64 // Max memory for in-memory backends (default: 1024MB)
}

// StorageStats provides runtime statistics for monitoring
type StorageStats struct {
	KeyCount      int64
	SyncBufferLen int
	CacheHitRate  float64
	DBSize        int64
}

// AtomicSyncOp wraps CacheSyncOperation for atomic storage in ring buffers (exported for backends)
type AtomicSyncOp struct {
	Op *CacheSyncOperation
}

// NextPowerOf2 rounds up to the next power of 2 (for ring buffer sizing, exported for backends)
func NextPowerOf2(n uint64) uint64 {
	if n == 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	return n + 1
}
