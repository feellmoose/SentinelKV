package gossip

import (
	"time"

	"github.com/feellmoose/gridkv/internal/storage"
)

// storageItemToProto converts a storage.StoredItem to proto StoredItem.
//
// NOTE: This function creates a new proto object each time. While object pooling
// could reduce allocations, protobuf messages have complex lifecycles that make
// it difficult to safely return objects to a pool without risking memory leaks.
// The Go GC is efficient enough to handle these allocations.
//
// Parameters:
//   - item: The storage item to convert
//
// Returns:
//   - *StoredItem: Proto representation
func storageItemToProto(item *storage.StoredItem) *StoredItem {
	if item == nil {
		return nil
	}

	return &StoredItem{
		ExpireAt: uint64(item.ExpireAt.Unix()),
		Value:    item.Value,
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
