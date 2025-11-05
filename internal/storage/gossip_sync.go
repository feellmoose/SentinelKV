package storage

// This file provides the bridge between storage and gossip protocol types
// It's kept separate to avoid circular dependencies while eliminating adapter overhead

// GossipSyncOp interface is satisfied by gossip.CacheSyncOperation (proto type)
type GossipSyncOp interface {
	GetKey() string
	GetClientVersion() int64
	GetType() int32 // OperationType enum
	GetSetData() GossipStoredItem
}

// GossipStoredItem interface is satisfied by gossip.StoredItem (proto type)
type GossipStoredItem interface {
	GetExpireAt() uint64
	GetValue() []byte
}

// GossipFullStateItem interface is satisfied by gossip.FullStateItem (proto type)
type GossipFullStateItem interface {
	GetKey() string
	GetVersion() int64
	GetItemData() GossipStoredItem
}
