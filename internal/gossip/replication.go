package gossip

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/feellmoose/gridkv/internal/storage"
	"github.com/feellmoose/gridkv/internal/utils/logging"
)

// Set performs a distributed write operation with quorum-based replication.
//
// The write flow:
//  1. Hash the key to find N replica nodes using consistent hashing
//  2. The first replica (coordinator) receives the write
//  3. If this node is not the coordinator, forward to coordinator
//  4. Coordinator writes locally, then replicates to N-1 replicas
//  5. Wait for W acknowledgments (including local write)
//  6. Return success if quorum is reached, error otherwise
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - key: The key to write
//   - item: The value and metadata to store
//
// Returns:
//   - error: Quorum error if W replicas don't acknowledge
func (gm *GossipManager) Set(ctx context.Context, key string, item *storage.StoredItem) error {
	if item == nil {
		return errors.New("nil item")
	}

	replicas := gm.hashRing.GetN(key, gm.replicaCount)
	if len(replicas) == 0 {
		return fmt.Errorf("no replicas for %s", key)
	}

	coordinator := replicas[0]
	if coordinator != gm.localNodeID {
		// Forward to coordinator
		return gm.forwardWrite(key, item, coordinator)
	}

	// Local write (we are the coordinator)
	if err := gm.store.Set(key, item); err != nil {
		return fmt.Errorf("local write failed: %w", err)
	}

	// Replicate to other nodes
	return gm.replicateToNodes(ctx, key, item, replicas[1:])
}

// Delete performs a distributed delete operation with quorum-based replication.
//
// Similar to Set, but removes the key-value pair instead.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - key: The key to delete
//   - version: Version number for optimistic concurrency control
//
// Returns:
//   - error: Quorum error if W replicas don't acknowledge
func (gm *GossipManager) Delete(ctx context.Context, key string, version int64) error {
	replicas := gm.hashRing.GetN(key, gm.replicaCount)
	if len(replicas) == 0 {
		return fmt.Errorf("no replicas for %s", key)
	}

	coordinator := replicas[0]
	if coordinator != gm.localNodeID {
		// Forward to coordinator
		return gm.forwardDelete(key, version, coordinator)
	}

	// Local delete (we are the coordinator)
	if err := gm.store.Delete(key, version); err != nil {
		return fmt.Errorf("local delete failed: %w", err)
	}

	// Replicate deletion to other nodes
	return gm.replicateDeleteToNodes(ctx, key, version, replicas[1:])
}

// Get performs a distributed read operation with quorum and read repair.
//
// The read flow:
//  1. Hash the key to find N replica nodes
//  2. If this node is not a replica, forward to coordinator
//  3. Otherwise, read from R replicas (including local)
//  4. Return the value with the highest version
//  5. Perform read repair in background for stale replicas
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - key: The key to read
//
// Returns:
//   - *storage.StoredItem: The stored item (highest version)
//   - error: Not found error or quorum error
func (gm *GossipManager) Get(ctx context.Context, key string) (*storage.StoredItem, error) {
	replicas := gm.hashRing.GetN(key, gm.replicaCount)
	if len(replicas) == 0 {
		return nil, fmt.Errorf("no replicas for %s", key)
	}

	// Check if local node is a replica
	isLocalReplica := false
	for _, r := range replicas {
		if r == gm.localNodeID {
			isLocalReplica = true
			break
		}
	}

	// If not a replica, forward to coordinator
	if !isLocalReplica {
		return gm.forwardReadToCoordinator(ctx, key, replicas[0])
	}

	// Perform read quorum
	return gm.readWithQuorum(ctx, key, replicas)
}

// forwardWrite forwards a write to the coordinator node.
func (gm *GossipManager) forwardWrite(key string, item *storage.StoredItem, coordinatorID string) error {
	n, ok := gm.getNode(coordinatorID)
	if !ok {
		return fmt.Errorf("coordinator %s unknown", coordinatorID)
	}

	protoOp := &CacheSyncOperation{
		Key:           key,
		ClientVersion: item.Version,
		Type:          OperationType_OP_SET,
		DataPayload: &CacheSyncOperation_SetData{
			SetData: storageItemToProto(item),
		},
	}

	msg := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_CACHE_SYNC,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_CacheSyncPayload{
			CacheSyncPayload: &SyncMessage{
				SyncType: &SyncMessage_IncrementalSync{
					IncrementalSync: &IncrementalSyncPayload{
						Operations: []*CacheSyncOperation{protoOp},
					},
				},
			},
		},
	}
	gm.signMessageCanonical(msg)
	return gm.network.SendWithTimeout(n.Address, msg, gm.replicationTimeout)
}

// forwardDelete forwards a delete to the coordinator node.
func (gm *GossipManager) forwardDelete(key string, version int64, coordinatorID string) error {
	n, ok := gm.getNode(coordinatorID)
	if !ok {
		return fmt.Errorf("coordinator %s unknown", coordinatorID)
	}

	protoOp := &CacheSyncOperation{
		Key:           key,
		ClientVersion: version,
		Type:          OperationType_OP_DELETE,
	}

	msg := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_CACHE_SYNC,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_CacheSyncPayload{
			CacheSyncPayload: &SyncMessage{
				SyncType: &SyncMessage_IncrementalSync{
					IncrementalSync: &IncrementalSyncPayload{
						Operations: []*CacheSyncOperation{protoOp},
					},
				},
			},
		},
	}
	gm.signMessageCanonical(msg)
	return gm.network.SendWithTimeout(n.Address, msg, gm.replicationTimeout)
}

// replicateToNodes replicates a write to multiple nodes with quorum.
func (gm *GossipManager) replicateToNodes(ctx context.Context, key string, item *storage.StoredItem, replicaIDs []string) error {
	if len(replicaIDs) == 0 {
		return nil // No replicas to write to
	}

	opID := gm.generateOpID()
	protoOp := &CacheSyncOperation{
		Key:           key,
		ClientVersion: item.Version,
		Type:          OperationType_OP_SET,
		DataPayload: &CacheSyncOperation_SetData{
			SetData: storageItemToProto(item),
		},
	}

	msg := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_CACHE_SYNC,
		Sender: gm.localNodeID,
		OpId:   opID,
		Hlc:    gm.hlc.Now(),
		Payload: &GossipMessage_CacheSyncPayload{
			CacheSyncPayload: &SyncMessage{
				SyncType: &SyncMessage_IncrementalSync{
					IncrementalSync: &IncrementalSyncPayload{
						Operations: []*CacheSyncOperation{protoOp},
					},
				},
			},
		},
	}
	gm.signMessageCanonical(msg)

	// Collect alive replica targets
	gm.mu.RLock()
	targets := make([]struct{ addr, id string }, 0, len(replicaIDs))
	for _, replicaID := range replicaIDs {
		if n, ok := gm.liveNodes[replicaID]; ok && n.State == NodeState_NODE_STATE_ALIVE {
			targets = append(targets, struct{ addr, id string }{addr: n.Address, id: replicaID})
		}
	}
	gm.mu.RUnlock()

	required := gm.writeQuorum - 1 // excluding primary (already written)
	if required <= 0 {
		return nil
	}

	// Use goroutine pool for bounded concurrency (OPTIMIZATION)
	ackCh := make(chan bool, len(targets))
	for _, t := range targets {
		addr := t.addr
		err := gm.replicationPool.Submit(func() {
			ok, err := gm.network.SendAndWaitAck(addr, msg, gm.replicationTimeout)
			if err != nil {
				logging.Error(err, "replicate send/ack failed", "target", addr)
				ackCh <- false
				return
			}
			ackCh <- ok
		})
		if err != nil {
			logging.Error(err, "failed to submit replication task", "target", addr)
			ackCh <- false
		}
	}

	// Wait for required acks or timeout
	ctx2, cancel := context.WithTimeout(ctx, gm.replicationTimeout)
	defer cancel()

	acks := 0
	for {
		select {
		case ok := <-ackCh:
			if ok {
				acks++
			}
			if acks >= required {
				return nil
			}
		case <-ctx2.Done():
			return fmt.Errorf("quorum not reached: got %d required %d", acks, required)
		}
	}
}

// replicateDeleteToNodes replicates a delete to multiple nodes with quorum.
func (gm *GossipManager) replicateDeleteToNodes(ctx context.Context, key string, version int64, replicaIDs []string) error {
	if len(replicaIDs) == 0 {
		return nil
	}

	opID := gm.generateOpID()
	protoOp := &CacheSyncOperation{
		Key:           key,
		ClientVersion: version,
		Type:          OperationType_OP_DELETE,
	}

	msg := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_CACHE_SYNC,
		Sender: gm.localNodeID,
		OpId:   opID,
		Hlc:    gm.hlc.Now(),
		Payload: &GossipMessage_CacheSyncPayload{
			CacheSyncPayload: &SyncMessage{
				SyncType: &SyncMessage_IncrementalSync{
					IncrementalSync: &IncrementalSyncPayload{
						Operations: []*CacheSyncOperation{protoOp},
					},
				},
			},
		},
	}
	gm.signMessageCanonical(msg)

	gm.mu.RLock()
	targets := make([]struct{ addr, id string }, 0, len(replicaIDs))
	for _, replicaID := range replicaIDs {
		if n, ok := gm.liveNodes[replicaID]; ok && n.State == NodeState_NODE_STATE_ALIVE {
			targets = append(targets, struct{ addr, id string }{addr: n.Address, id: replicaID})
		}
	}
	gm.mu.RUnlock()

	required := gm.writeQuorum - 1
	if required <= 0 {
		return nil
	}

	ackCh := make(chan bool, len(targets))
	for _, t := range targets {
		addr := t.addr
		err := gm.replicationPool.Submit(func() {
			ok, err := gm.network.SendAndWaitAck(addr, msg, gm.replicationTimeout)
			if err != nil {
				logging.Error(err, "replicate delete send/ack failed", "target", addr)
				ackCh <- false
				return
			}
			ackCh <- ok
		})
		if err != nil {
			logging.Error(err, "failed to submit delete replication task", "target", addr)
			ackCh <- false
		}
	}

	ctx2, cancel := context.WithTimeout(ctx, gm.replicationTimeout)
	defer cancel()

	acks := 0
	for {
		select {
		case ok := <-ackCh:
			if ok {
				acks++
			}
			if acks >= required {
				return nil
			}
		case <-ctx2.Done():
			return fmt.Errorf("delete quorum not reached: got %d required %d", acks, required)
		}
	}
}

// forwardReadToCoordinator forwards a read request to the coordinator node.
func (gm *GossipManager) forwardReadToCoordinator(ctx context.Context, key string, coordinatorID string) (*storage.StoredItem, error) {
	if coordinatorID == gm.localNodeID {
		return nil, errors.New("coordinator is local but not in replica set")
	}

	peer, ok := gm.getNode(coordinatorID)
	if !ok {
		return nil, fmt.Errorf("coordinator %s not found", coordinatorID)
	}

	requestID := gm.generateOpID()
	respCh := make(chan *ReadResponsePayload, 1)
	gm.pendingReads.Store(requestID, respCh)
	defer func() {
		gm.pendingReads.Delete(requestID)
		close(respCh)
	}()

	msg := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_READ_REQUEST,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_ReadRequestPayload{
			ReadRequestPayload: &ReadRequestPayload{
				Key:         key,
				RequesterId: gm.localNodeID,
				RequestId:   requestID,
			},
		},
	}
	gm.signMessageCanonical(msg)

	if err := gm.network.SendWithTimeout(peer.Address, msg, gm.readTimeout); err != nil {
		return nil, fmt.Errorf("forward read failed: %w", err)
	}

	ctx2, cancel := context.WithTimeout(ctx, gm.readTimeout)
	defer cancel()

	select {
	case resp := <-respCh:
		if !resp.Found {
			return nil, storage.ErrItemNotFound
		}
		return protoItemToStorage(resp.ItemData, resp.Version), nil
	case <-ctx2.Done():
		return nil, fmt.Errorf("read forward timeout for key %s", key)
	}
}

// readWithQuorum performs a read from R replicas and returns the latest version.
func (gm *GossipManager) readWithQuorum(ctx context.Context, key string, replicas []string) (*storage.StoredItem, error) {
	// Read from local store first
	localItem, localErr := gm.store.Get(key)

	// If read quorum is 1, return local result immediately
	if gm.readQuorum == 1 {
		if localErr != nil {
			return nil, localErr
		}
		return localItem, nil
	}

	// Collect responses from R replicas
	results := make(chan readResult, len(replicas))

	// Add local result
	if localErr == nil {
		results <- readResult{
			item:    localItem,
			version: localItem.Version,
			nodeID:  gm.localNodeID,
			found:   true,
		}
	} else if localErr == storage.ErrItemNotFound || localErr == storage.ErrItemExpired {
		results <- readResult{
			nodeID: gm.localNodeID,
			found:  false,
		}
	}

	// Request from other replicas
	requestID := gm.generateOpID()
	respCh := make(chan *ReadResponsePayload, len(replicas))
	gm.pendingReads.Store(requestID, respCh)
	defer func() {
		gm.pendingReads.Delete(requestID)
		close(respCh)
	}()

	var wg sync.WaitGroup
	for _, replicaID := range replicas {
		if replicaID == gm.localNodeID {
			continue
		}

		peer, ok := gm.getNode(replicaID)
		if !ok || peer.State != NodeState_NODE_STATE_ALIVE {
			continue
		}

		wg.Add(1)
		go func(nodeID, addr string) {
			defer wg.Done()

			msg := &GossipMessage{
				Type:   GossipMessageType_MESSAGE_TYPE_READ_REQUEST,
				Sender: gm.localNodeID,
				Payload: &GossipMessage_ReadRequestPayload{
					ReadRequestPayload: &ReadRequestPayload{
						Key:         key,
						RequesterId: gm.localNodeID,
						RequestId:   requestID,
					},
				},
			}
			gm.signMessageCanonical(msg)

			if err := gm.network.SendWithTimeout(addr, msg, gm.readTimeout); err != nil {
				logging.Error(err, "read request failed", "target", nodeID, "key", key)
			}
		}(replicaID, peer.Address)
	}

	// Wait for responses in background
	go wg.Wait()

	// Collect R responses
	ctx2, cancel := context.WithTimeout(ctx, gm.readTimeout)
	defer cancel()

	collected := 1 // Already have local result
	for collected < gm.readQuorum {
		select {
		case resp := <-respCh:
			if resp.Found {
				results <- readResult{
					item:    protoItemToStorage(resp.ItemData, resp.Version),
					version: resp.Version,
					nodeID:  resp.ResponderId,
					found:   true,
				}
			} else {
				results <- readResult{
					nodeID: resp.ResponderId,
					found:  false,
				}
			}
			collected++
		case <-ctx2.Done():
			return nil, fmt.Errorf("read quorum not reached for %s: got %d required %d", key, collected, gm.readQuorum)
		}
	}

	// Find the item with the highest version (latest)
	close(results)
	var latestItem *storage.StoredItem
	var latestVersion int64 = -1
	foundCount := 0
	allResults := make([]readResult, 0)

	for result := range results {
		allResults = append(allResults, result)
		if result.found {
			foundCount++
			if result.version > latestVersion {
				latestVersion = result.version
				latestItem = result.item
			}
		}
	}

	if foundCount == 0 {
		return nil, storage.ErrItemNotFound
	}

	// Read repair: update stale replicas in background
	go gm.performReadRepair(key, latestItem, allResults)

	return latestItem, nil
}

// handleReadRequest processes an incoming read request and sends back the value.
func (gm *GossipManager) handleReadRequest(req *ReadRequestPayload, senderID string) {
	if req == nil || req.Key == "" {
		logging.Warn("Invalid read request")
		return
	}

	item, err := gm.store.Get(req.Key)

	resp := &ReadResponsePayload{
		Key:         req.Key,
		RequestId:   req.RequestId,
		Found:       err == nil,
		ResponderId: gm.localNodeID,
	}

	if err == nil {
		resp.ItemData = storageItemToProto(item)
		resp.Version = item.Version
	}

	peer, ok := gm.getNode(senderID)
	if !ok {
		logging.Warn("Sender node not found for read response", "sender", senderID)
		return
	}

	msg := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_READ_RESPONSE,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_ReadResponsePayload{
			ReadResponsePayload: resp,
		},
	}
	gm.signMessageCanonical(msg)

	if err := gm.network.SendWithTimeout(peer.Address, msg, gm.readTimeout); err != nil {
		logging.Error(err, "Failed to send read response", "key", req.Key, "target", senderID)
	}
}

// handleReadResponse processes an incoming read response.
func (gm *GossipManager) handleReadResponse(resp *ReadResponsePayload) {
	if resp == nil || resp.RequestId == "" {
		logging.Warn("Invalid read response")
		return
	}

	if ch, ok := gm.pendingReads.Load(resp.RequestId); ok {
		select {
		case ch.(chan *ReadResponsePayload) <- resp:
			logging.Debug("Read response delivered", "requestId", resp.RequestId, "found", resp.Found)
		default:
			logging.Warn("Failed to deliver read response (channel full or closed)", "requestId", resp.RequestId)
		}
	} else {
		logging.Debug("Received read response for unknown or expired requestId", "requestId", resp.RequestId)
	}
}

// performReadRepair updates stale replicas with the latest version in the background.
func (gm *GossipManager) performReadRepair(key string, latestItem *storage.StoredItem, allResults []readResult) {
	if latestItem == nil {
		return
	}

	for _, result := range allResults {
		if result.nodeID == gm.localNodeID {
			// Update local if stale
			if !result.found || result.version < latestItem.Version {
				if err := gm.store.Set(key, latestItem); err != nil {
					logging.Error(err, "Read repair: local update failed", "key", key)
				} else {
					logging.Debug("Read repair: updated local", "key", key, "version", latestItem.Version)
				}
			}
			continue
		}

		// Update remote replicas if stale
		if !result.found || result.version < latestItem.Version {
			go func(nodeID string) {
				peer, ok := gm.getNode(nodeID)
				if !ok {
					return
				}

				protoOp := &CacheSyncOperation{
					Key:           key,
					ClientVersion: latestItem.Version,
					Type:          OperationType_OP_SET,
					DataPayload: &CacheSyncOperation_SetData{
						SetData: storageItemToProto(latestItem),
					},
				}

				msg := &GossipMessage{
					Type:   GossipMessageType_MESSAGE_TYPE_CACHE_SYNC,
					Sender: gm.localNodeID,
					Payload: &GossipMessage_CacheSyncPayload{
						CacheSyncPayload: &SyncMessage{
							SyncType: &SyncMessage_IncrementalSync{
								IncrementalSync: &IncrementalSyncPayload{
									Operations: []*CacheSyncOperation{protoOp},
								},
							},
						},
					},
				}
				gm.signMessageCanonical(msg)

				if err := gm.network.SendWithTimeout(peer.Address, msg, gm.replicationTimeout); err != nil {
					logging.Error(err, "Read repair: failed to update replica", "target", nodeID, "key", key)
				} else {
					logging.Debug("Read repair: updated replica", "target", nodeID, "key", key, "version", latestItem.Version)
				}
			}(result.nodeID)
		}
	}
}

// readResult represents the result of a read operation from a replica.
type readResult struct {
	item    *storage.StoredItem
	version int64
	nodeID  string
	found   bool
}
