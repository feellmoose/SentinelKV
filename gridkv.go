package gridkv

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/feellmoose/gridkv/internal/gossip"
	"github.com/feellmoose/gridkv/internal/storage"
	"github.com/feellmoose/gridkv/internal/utils/crypto"
	"github.com/feellmoose/gridkv/internal/utils/logging"
)

// GridKV is the main entry point for the distributed key-value store.
// It provides a high-level API for storing and retrieving data across a cluster
// with configurable replication, quorum, and consistency guarantees.
type GridKV struct {
	gm       *gossip.GossipManager
	store    gossip.KVStore
	network  gossip.Network
	hashRing *gossip.ConsistentHash
	ttl      time.Duration
}

// NewGridKV creates and initializes a GridKV instance with the specified configuration.
// This function sets up the entire distributed system including:
//   - Network layer for inter-node communication
//   - Consistent hash ring for data distribution
//   - Local storage backend (Badger)
//   - Gossip protocol for failure detection and state sync
//   - Cryptographic key pair for message signing
func NewGridKV(opts *GridKVOptions) (*GridKV, error) {
	if opts == nil {
		return nil, errors.New("GridKVOptions cannot be nil")
	}

	// Validate required options
	if opts.LocalNodeID == "" {
		return nil, errors.New("LocalNodeID is required")
	}
	if opts.LocalAddress == "" {
		return nil, errors.New("LocalAddress is required")
	}
	if opts.Network == nil {
		return nil, errors.New("Network options are required")
	}
	if opts.Storage == nil {
		return nil, errors.New("Storage options are required")
	}

	// Initialize logging (optional)
	if opts.Log != nil {
		logging.Log = logging.NewLogger(opts.Log)
	}

	// Create network layer
	network, err := gossip.NewNetwork(opts.Network)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	// Create storage backend using the registry pattern (build-tag aware)
	// This allows conditional compilation - backends with external dependencies
	// can be excluded via build tags to reduce binary size and dependencies
	rawStore, err := storage.NewStorage(opts.Storage)
	if err != nil {
		network.Stop()
		return nil, fmt.Errorf("failed to create storage backend: %w", err)
	}

	// Wrap with gossip bridge for proto type conversion
	store := gossip.NewStorageBridge(rawStore)

	// Create consistent hash ring
	hashRing := gossip.NewConsistentHash(opts.VirtualNodes, nil)

	// Generate or load cryptographic key pair for message signing
	var keypair *crypto.KeyPair
	if opts.KeyPair != nil {
		keypair = opts.KeyPair
	} else {
		var err error
		keypair, err = crypto.GenerateKeyPair()
		if err != nil {
			network.Stop()
			store.Close()
			return nil, fmt.Errorf("failed to generate keypair: %w", err)
		}
	}

	// Initialize peer public keys map (for signature verification)
	peerPubkeys := make(map[string]crypto.PublicKey)
	if opts.PeerPublicKeys != nil {
		peerPubkeys = opts.PeerPublicKeys
	}
	// Add own public key
	peerPubkeys[opts.LocalNodeID] = keypair.Pub

	// Create gossip options
	gossipOpts := &gossip.GossipOptions{
		LocalNodeID:        opts.LocalNodeID,
		LocalAddress:       opts.LocalAddress,
		SeedAddrs:          opts.SeedAddrs,
		FailureTimeout:     opts.FailureTimeout,
		SuspectTimeout:     opts.SuspectTimeout,
		GossipInterval:     opts.GossipInterval,
		ReplicaCount:       opts.ReplicaCount,
		WriteQuorum:        opts.WriteQuorum,
		ReadQuorum:         opts.ReadQuorum,
		MaxReplicators:     opts.MaxReplicators,
		ReplicationTimeout: opts.ReplicationTimeout,
		ReadTimeout:        opts.ReadTimeout,
	}

	// Create gossip manager
	manager, err := gossip.NewGossipManager(gossipOpts, hashRing, network, store, keypair, peerPubkeys)
	if err != nil {
		network.Stop()
		store.Close()
		return nil, fmt.Errorf("failed to create gossip manager: %w", err)
	}

	gridKV := &GridKV{
		gm:       manager,
		store:    store,
		network:  network,
		hashRing: hashRing,
		ttl:      opts.TTL,
	}

	// Start gossip manager
	manager.Start()

	// Setup network message handler
	if err := network.Listen(func(msg *gossip.GossipMessage) error {
		manager.SimulateReceive(msg)
		return nil
	}); err != nil {
		gridKV.Close()
		return nil, fmt.Errorf("failed to start network listener: %w", err)
	}

	logging.Info("GridKV initialized successfully", "nodeID", opts.LocalNodeID, "address", opts.LocalAddress)
	return gridKV, nil
}

// Set writes a key-value pair with distributed replication.
// The data is replicated to N nodes based on consistent hashing and written to W nodes
// to satisfy the write quorum before returning success.
//
// This method is panic-safe and will recover from any internal panics.
func (g *GridKV) Set(ctx context.Context, key string, value []byte) (err error) {
	// SAFETY: Recover from panics to prevent application crash
	var item *storage.StoredItem
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in Set operation: %v", r)
			logging.Error(err, "Set panic recovered", "key", key)
			// Ensure object pool item is returned even on panic
			if item != nil {
				storage.PutStoredItem(item)
			}
		}
	}()

	if g.gm == nil {
		return errors.New("GridKV not initialized")
	}
	if key == "" {
		return errors.New("key cannot be empty")
	}

	// Create StoredItem with TTL
	// OPTIMIZATION: Use object pool to reduce allocations
	item = storage.GetStoredItem()
	item.Value = value
	item.Version = time.Now().UnixNano() // Use timestamp as version

	if g.ttl > 0 {
		item.ExpireAt = time.Now().Add(g.ttl)
	} else {
		item.ExpireAt = time.Time{} // No expiration (zero value)
	}

	// Delegate to gossip manager for distributed write
	// Note: The gossip manager will copy the data, so we can reuse the item
	err = g.gm.Set(ctx, key, item)

	// Return item to pool after use
	storage.PutStoredItem(item)

	return err
}

// Get retrieves the value corresponding to the key from the distributed store.
// This operation reads from R replicas to satisfy read quorum and automatically
// performs read repair if stale data is detected.
//
// The returned []byte is a deep copy and safe for modification by the caller.
// This method is panic-safe and will recover from any internal panics.
func (g *GridKV) Get(ctx context.Context, key string) (value []byte, err error) {
	// SAFETY: Recover from panics to prevent application crash
	var item *storage.StoredItem
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in Get operation: %v", r)
			logging.Error(err, "Get panic recovered", "key", key)
			value = nil
			// Ensure object pool item is returned even on panic
			if item != nil {
				storage.PutStoredItem(item)
			}
		}
	}()

	if g.gm == nil {
		return nil, errors.New("GridKV not initialized")
	}
	if key == "" {
		return nil, errors.New("key cannot be empty")
	}

	// Delegate to gossip manager for distributed read
	item, err = g.gm.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	// Check expiration
	if !item.ExpireAt.IsZero() && time.Now().After(item.ExpireAt) {
		// IMPORTANT: Return StoredItem to pool before returning error
		storage.PutStoredItem(item)
		return nil, storage.ErrItemExpired
	}

	// SAFETY: Make a deep copy of Value to ensure user can safely modify it
	// This also allows us to return the StoredItem object to the pool
	value = append([]byte(nil), item.Value...)

	// OPTIMIZATION: Return StoredItem to object pool
	storage.PutStoredItem(item)

	return value, nil
}

// Delete removes the key-value pair with distributed replication.
// The deletion is replicated to W nodes to satisfy write quorum.
//
// This method is panic-safe and will recover from any internal panics.
func (g *GridKV) Delete(ctx context.Context, key string) (err error) {
	// SAFETY: Recover from panics to prevent application crash
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in Delete operation: %v", r)
			logging.Error(err, "Delete panic recovered", "key", key)
		}
	}()

	if g.gm == nil {
		return errors.New("GridKV not initialized")
	}
	if key == "" {
		return errors.New("key cannot be empty")
	}

	// Get current version for optimistic locking
	item, err := g.store.Get(key)
	if err != nil {
		if err == storage.ErrItemNotFound {
			return nil // Already deleted
		}
		return err
	}

	// Delegate to gossip manager for distributed delete
	return g.gm.Delete(ctx, key, item.Version)
}

// Close gracefully shuts down the GridKV instance and releases all resources.
// This includes stopping the gossip protocol, closing network connections,
// and flushing the storage backend.
func (g *GridKV) Close() error {
	var errs []error

	if g.gm != nil {
		g.gm.Stop()
	}

	if g.network != nil {
		if err := g.network.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("network stop failed: %w", err))
		}
	}

	if g.store != nil {
		if err := g.store.Close(); err != nil {
			errs = append(errs, fmt.Errorf("store close failed: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	logging.Info("GridKV closed successfully")
	return nil
}

// GridKVOptions is the configuration for a GridKV instance.
// It combines all the necessary settings for distributed operation including
// network, storage, replication, and consistency parameters.
type GridKVOptions struct {
	// Node Identity
	LocalNodeID  string   // Unique identifier for this node
	LocalAddress string   // Address for this node (host:port)
	SeedAddrs    []string // Bootstrap nodes for joining the cluster

	// Network Configuration
	Network *gossip.NetworkOptions // Network layer settings

	// Storage Configuration
	Storage *gossip.StorageOptions // Storage backend settings

	// Logging Configuration
	Log *logging.LogOptions // Logging settings

	// Replication & Consistency
	ReplicaCount int // N: Number of replicas per key (default: 3)
	WriteQuorum  int // W: Write quorum (default: (N/2)+1)
	ReadQuorum   int // R: Read quorum (default: (N/2)+1)

	// Performance Tuning
	VirtualNodes       int           // Virtual nodes per physical node in hash ring (default: 150)
	MaxReplicators     int           // Max concurrent replication goroutines (default: 4)
	ReplicationTimeout time.Duration // Timeout for write replication (default: 2s)
	ReadTimeout        time.Duration // Timeout for read operations (default: 2s)

	// Failure Detection
	FailureTimeout time.Duration // Timeout before marking node as suspect (default: 5s)
	SuspectTimeout time.Duration // Timeout before marking suspect node as dead (default: 10s)
	GossipInterval time.Duration // Interval for gossip protocol (default: 1s)

	// Data Lifecycle
	TTL time.Duration // Default TTL for stored items (0 = no expiration)

	// Security
	KeyPair        *crypto.KeyPair             // Cryptographic key pair for signing
	PeerPublicKeys map[string]crypto.PublicKey // Public keys of peer nodes
}
