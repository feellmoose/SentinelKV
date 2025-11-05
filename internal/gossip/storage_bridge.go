package gossip

import (
	"time"

	"github.com/feellmoose/gridkv/internal/storage"
)

// OPTIMIZATION: Ultra-lightweight storage bridge (compiler can inline these)
// This provides zero-overhead type conversion between storage and proto types

// StorageBridge wraps any storage.Storage implementation and provides proto type conversion
// The Go compiler inlines these methods, so there's virtually NO overhead
type StorageBridge struct {
	store storage.Storage // Generic storage interface
}

// NewStorageBridge creates an optimized bridge for any storage type
func NewStorageBridge(store storage.Storage) *StorageBridge {
	return &StorageBridge{store: store}
}

// Delegate basic operations to underlying storage
func (b *StorageBridge) Set(key string, item *storage.StoredItem) error {
	return b.store.Set(key, item)
}

func (b *StorageBridge) Get(key string) (*storage.StoredItem, error) {
	return b.store.Get(key)
}

func (b *StorageBridge) Delete(key string, version int64) error {
	return b.store.Delete(key, version)
}

func (b *StorageBridge) Keys() []string {
	return b.store.Keys()
}

func (b *StorageBridge) Clear() error {
	return b.store.Clear()
}

func (b *StorageBridge) Close() error {
	return b.store.Close()
}

// GetSyncBuffer returns proto CacheSyncOperation types
func (b *StorageBridge) GetSyncBuffer() ([]*CacheSyncOperation, error) {
	ops, err := b.store.GetSyncBuffer()
	if err != nil || len(ops) == 0 {
		return nil, err
	}

	// Convert storage ops to proto ops (OPTIMIZATION: compiler can inline)
	protoOps := make([]*CacheSyncOperation, len(ops))
	for i, op := range ops {
		protoOps[i] = storageSyncOpToProto(op)
	}
	return protoOps, nil
}

// GetFullSyncSnapshot returns proto FullStateItem types
func (b *StorageBridge) GetFullSyncSnapshot() ([]*FullStateItem, error) {
	items, err := b.store.GetFullSyncSnapshot()
	if err != nil || len(items) == 0 {
		return nil, err
	}

	// Convert storage items to proto items (OPTIMIZATION: compiler can inline)
	protoItems := make([]*FullStateItem, len(items))
	for i, item := range items {
		protoItems[i] = storageFullItemToProto(item)
	}
	return protoItems, nil
}

// ApplyIncrementalSync accepts proto CacheSyncOperation types
func (b *StorageBridge) ApplyIncrementalSync(operations []*CacheSyncOperation) error {
	// Convert proto ops to storage ops (OPTIMIZATION: direct conversion, inline-able)
	for _, op := range operations {
		switch op.Type {
		case OperationType_OP_SET:
			item := protoItemToStorage(op.GetSetData(), op.ClientVersion)
			if err := b.store.Set(op.Key, item); err != nil {
				return err
			}
		case OperationType_OP_DELETE:
			if err := b.store.Delete(op.Key, op.ClientVersion); err != nil {
				if err != storage.ErrItemNotFound {
					return err
				}
			}
		}
	}
	return nil
}

// ApplyFullSyncSnapshot accepts proto FullStateItem types
func (b *StorageBridge) ApplyFullSyncSnapshot(snapshot []*FullStateItem, snapshotTS time.Time) error {
	// Convert proto items to storage items
	storageItems := make([]*storage.FullStateItem, len(snapshot))
	for i, protoItem := range snapshot {
		storageItems[i] = protoFullItemToStorage(protoItem)
	}
	return b.store.ApplyFullSyncSnapshot(storageItems, snapshotTS)
}

// Stats returns storage statistics (pass-through)
func (b *StorageBridge) Stats() storage.StorageStats {
	return b.store.Stats()
}
