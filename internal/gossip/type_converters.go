package gossip

import (
	"sync"
	"time"

	"github.com/feellmoose/gridkv/internal/storage"
)

// Object pools for proto type conversion (OPTIMIZATION: reduces allocations)
var (
	// Proto message pools
	gossipMessagePool = sync.Pool{
		New: func() interface{} {
			return &GossipMessage{}
		},
	}

	protoItemPool = sync.Pool{
		New: func() interface{} {
			return &StoredItem{}
		},
	}
)

// getMessage retrieves a GossipMessage from the pool.
func getMessage() *GossipMessage {
	msg := gossipMessagePool.Get().(*GossipMessage)
	msg.Reset() // Clear previous data
	return msg
}

// putMessage returns a GossipMessage to the pool.
func putMessage(msg *GossipMessage) {
	if msg != nil {
		gossipMessagePool.Put(msg)
	}
}

// storageItemToProto converts a storage.StoredItem to proto StoredItem.
// OPTIMIZED: Reuses pooled objects to reduce allocations.
//
// Parameters:
//   - item: The storage item to convert
//
// Returns:
//   - *StoredItem: Proto representation (from pool)
func storageItemToProto(item *storage.StoredItem) *StoredItem {
	if item == nil {
		return nil
	}

	protoItem := protoItemPool.Get().(*StoredItem)
	protoItem.ExpireAt = uint64(item.ExpireAt.Unix())
	protoItem.Value = item.Value

	return protoItem
}

// returnProtoItemToPool returns a proto item to pool.
// Call this after the item is no longer needed to avoid memory leaks.
//
// Parameters:
//   - item: The proto item to return to pool
func returnProtoItemToPool(item *StoredItem) {
	if item != nil {
		item.Value = nil // Clear to avoid holding references
		protoItemPool.Put(item)
	}
}

// protoItemToStorage converts a proto StoredItem to storage.StoredItem.
//
// Parameters:
//   - item: The proto item to convert
//   - version: Version number to assign
//
// Returns:
//   - *storage.StoredItem: Storage representation
func protoItemToStorage(item *StoredItem, version int64) *storage.StoredItem {
	if item == nil {
		return nil
	}
	return &storage.StoredItem{
		ExpireAt: time.Unix(int64(item.ExpireAt), 0),
		Version:  version,
		Value:    item.Value,
	}
}

// storageSyncOpToProto converts storage.CacheSyncOperation to proto CacheSyncOperation.
//
// Parameters:
//   - op: The storage sync operation to convert
//
// Returns:
//   - *CacheSyncOperation: Proto representation
func storageSyncOpToProto(op *storage.CacheSyncOperation) *CacheSyncOperation {
	if op == nil {
		return nil
	}

	protoOp := &CacheSyncOperation{
		Key:           op.Key,
		ClientVersion: op.Version,
	}

	switch op.Type {
	case "SET":
		protoOp.Type = OperationType_OP_SET
		if op.Data != nil {
			protoOp.DataPayload = &CacheSyncOperation_SetData{
				SetData: storageItemToProto(op.Data),
			}
		}
	case "DELETE":
		protoOp.Type = OperationType_OP_DELETE
	default:
		protoOp.Type = OperationType_OP_UNSPECIFIED
	}

	return protoOp
}

// storageFullItemToProto converts storage.FullStateItem to proto FullStateItem.
//
// Parameters:
//   - item: The full state item to convert
//
// Returns:
//   - *FullStateItem: Proto representation
func storageFullItemToProto(item *storage.FullStateItem) *FullStateItem {
	if item == nil {
		return nil
	}
	return &FullStateItem{
		Key:      item.Key,
		Version:  item.Version,
		ItemData: storageItemToProto(item.Item),
	}
}

// protoFullItemToStorage converts proto FullStateItem to storage.FullStateItem.
//
// Parameters:
//   - item: The proto full state item to convert
//
// Returns:
//   - *storage.FullStateItem: Storage representation
func protoFullItemToStorage(item *FullStateItem) *storage.FullStateItem {
	if item == nil {
		return nil
	}
	return &storage.FullStateItem{
		Key:     item.Key,
		Version: item.Version,
		Item:    protoItemToStorage(item.ItemData, item.Version),
	}
}
