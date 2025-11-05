package gossip

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/feellmoose/gridkv/internal/utils/logging"
)

// Object pools for gossip synchronization (OPTIMIZATION)
var (
	// NodeInfo slice pool for cluster sync
	nodeInfoSlicePool = sync.Pool{
		New: func() interface{} {
			slice := make([]*NodeInfo, 0, 32)
			return &slice
		},
	}
)

// gossipPeriodically broadcasts cluster membership to a random peer.
// This is the core of the gossip protocol for membership dissemination.
func (gm *GossipManager) gossipPeriodically() {
	// Fast path: skip if no peers (OPTIMIZATION)
	gm.mu.RLock()
	peerCount := len(gm.liveNodes) - 1 // Exclude self
	gm.mu.RUnlock()

	if peerCount == 0 {
		return // No peers to gossip with
	}

	// Use pooled slice for members (OPTIMIZATION)
	gm.mu.RLock()
	membersPtr := nodeInfoSlicePool.Get().(*[]*NodeInfo)
	members := (*membersPtr)[:0] // Reset to zero length

	for _, n := range gm.liveNodes {
		// Reuse NodeInfo objects instead of creating new ones (OPTIMIZATION)
		members = append(members, &NodeInfo{
			NodeId:       n.NodeId,
			Address:      n.Address,
			LastActiveTs: n.LastActiveTs,
			State:        n.State,
			Version:      n.Version,
		})
	}
	target := gm.getRandomPeerID("")
	gm.mu.RUnlock()

	if target == "" {
		nodeInfoSlicePool.Put(membersPtr)
		return
	}

	// Create cluster sync message
	sync := &GossipMessage{
		Type:   CLUSTER_SYNC,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_ClusterSyncPayload{
			ClusterSyncPayload: &ClusterSyncPayload{Nodes: members},
		},
	}
	gm.signMessageCanonical(sync)

	if peer, ok := gm.getNode(target); ok {
		gm.network.SendWithTimeout(peer.Address, sync, 500*time.Millisecond)
	}

	// Return pooled slice
	nodeInfoSlicePool.Put(membersPtr)

	// Batch cache gossip with cluster gossip to same target (OPTIMIZATION)
	gm.gossipCachePeriodically(target)
}

// gossipCachePeriodically broadcasts incremental cache updates to a target node.
//
// Parameters:
//   - targetNodeID: The node to send cache updates to
func (gm *GossipManager) gossipCachePeriodically(targetNodeID string) {
	if gm.store == nil {
		return
	}

	items, err := gm.store.GetSyncBuffer()
	if err != nil {
		logging.Error(err, "get sync buffer failed")
		return
	}

	// Fast path: skip if nothing to sync (OPTIMIZATION)
	if len(items) == 0 {
		return
	}

	// Adaptive batching - size adjusts based on message rate (OPTIMIZATION)
	maxOpsPerBatch := gm.getAdaptiveBatchSize()

	for i := 0; i < len(items); i += maxOpsPerBatch {
		end := i + maxOpsPerBatch
		if end > len(items) {
			end = len(items)
		}
		batch := items[i:end]

		// Create message for this batch
		msg := &GossipMessage{
			Type:   CACHE_SYNC,
			Sender: gm.localNodeID,
			Payload: &GossipMessage_CacheSyncPayload{
				CacheSyncPayload: &SyncMessage{
					SyncType: &SyncMessage_IncrementalSync{
						IncrementalSync: &IncrementalSyncPayload{
							Operations: batch,
						},
					},
				},
			},
		}
		gm.signMessageCanonical(msg)

		if peer, ok := gm.getNode(targetNodeID); ok {
			gm.network.SendWithTimeout(peer.Address, msg, 500*time.Millisecond)
			// Track message rate for adaptive batching (OPTIMIZATION)
			gm.msgRateCounter.Add(1)
		}
	}
}

// getAdaptiveBatchSize calculates optimal batch size based on message rate.
// OPTIMIZATION: Inspired by TCP congestion control.
//
// Returns:
//   - int: Recommended batch size
func (gm *GossipManager) getAdaptiveBatchSize() int {
	now := time.Now().Unix()
	lastCheck := gm.lastRateCheck.Load()

	// Update rate every second
	if now > lastCheck {
		msgCount := gm.msgRateCounter.Swap(0) // Reset counter
		gm.lastRateCheck.Store(now)

		// Adaptive algorithm
		var newBatchSize int32
		if msgCount > 10000 {
			newBatchSize = 200 // High rate: larger batches
		} else if msgCount > 1000 {
			newBatchSize = 100 // Medium rate: default batches
		} else {
			newBatchSize = 50 // Low rate: smaller batches (less delay)
		}

		gm.lastBatchSize.Store(newBatchSize)
		logging.Debug("Adaptive batch size updated", "rate", msgCount, "batchSize", newBatchSize)
	}

	return int(gm.lastBatchSize.Load())
}

// RequestFullSync initiates a full state synchronization from a peer node.
// This is typically used during node recovery or initial cluster join.
//
// Parameters:
//   - targetNodeID: The node to request full sync from (empty string = random peer)
//
// Returns:
//   - error: Any error encountered
func (gm *GossipManager) RequestFullSync(targetNodeID string) error {
	if targetNodeID == "" {
		targetNodeID = gm.getRandomPeerID("")
		if targetNodeID == "" {
			return errors.New("no peer for full sync")
		}
	}

	peer, ok := gm.getNode(targetNodeID)
	if !ok {
		return fmt.Errorf("peer %s not found", targetNodeID)
	}

	msg := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_FULL_SYNC_REQUEST,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_FullSyncRequestPayload{
			FullSyncRequestPayload: &FullSyncRequestPayload{
				RequesterId: gm.localNodeID,
			},
		},
	}
	gm.signMessageCanonical(msg)
	logging.Info("SYNC request", "target", targetNodeID)
	return gm.network.SendWithTimeout(peer.Address, msg, gm.replicationTimeout)
}

// handleFullSyncRequest processes a full sync request and sends back complete state.
//
// Parameters:
//   - requesterID: The node requesting the full sync
func (gm *GossipManager) handleFullSyncRequest(requesterID string) {
	if gm.store == nil {
		logging.Warn("SYNC store nil")
		return
	}

	items, err := gm.store.GetFullSyncSnapshot()
	if err != nil {
		logging.Error(err, "get full snapshot failed")
		return
	}

	payload := &FullSyncResponsePayload{
		FullSync: &FullSyncPayload{
			Items:             items,
			SnapshotTimestamp: uint64(gm.localVersion),
		},
	}

	peer, ok := gm.getNode(requesterID)
	if !ok {
		logging.Warn("requester not found", "req", requesterID)
		return
	}

	resp := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_FULL_SYNC_RESPONSE,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_FullSyncResponsePayload{
			FullSyncResponsePayload: payload,
		},
	}
	gm.signMessageCanonical(resp)
	gm.network.SendWithTimeout(peer.Address, resp, gm.replicationTimeout)
}

// handleFullSyncResponse applies a full snapshot from a peer node.
//
// Parameters:
//   - payload: The full sync payload containing all state
func (gm *GossipManager) handleFullSyncResponse(payload *FullSyncPayload) {
	if gm.store == nil {
		logging.Warn("SYNC apply store nil")
		return
	}

	if err := gm.store.ApplyFullSyncSnapshot(payload.GetItems(), time.Unix(int64(payload.GetSnapshotTimestamp()), 0)); err != nil {
		logging.Error(err, "apply full sync failed")
	}
	logging.Info("SYNC applied", "items", len(payload.Items))
}
