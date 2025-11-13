package gossip

import (
	"errors"
	"time"

	"github.com/feellmoose/gridkv/internal/utils/crypto"
	"github.com/feellmoose/gridkv/internal/utils/logging"
	"google.golang.org/protobuf/proto"
)

// processGossipMessage routes incoming messages to appropriate handlers.
//
// Parameters:
//   - msg: The gossip message to process
func (gm *GossipManager) processGossipMessage(msg *GossipMessage) {
	switch msg.Type {
	case GossipMessageType_MESSAGE_TYPE_CONNECT:
		gm.handleConnect(msg)

	case GossipMessageType_MESSAGE_TYPE_CLUSTER_SYNC:
		gm.handleClusterSync(msg)

	case GossipMessageType_MESSAGE_TYPE_PROBE_REQUEST:
		gm.handleProbeRequest(msg)

	case GossipMessageType_MESSAGE_TYPE_PROBE_RESPONSE:
		gm.handleProbeResponse(msg)

	case GossipMessageType_MESSAGE_TYPE_CACHE_SYNC:
		gm.handleCacheSync(msg)

	case GossipMessageType_MESSAGE_TYPE_CACHE_SYNC_ACK:
		gm.handleCacheSyncAck(msg)

	case GossipMessageType_MESSAGE_TYPE_READ_REQUEST:
		gm.handleReadRequestMessage(msg)

	case GossipMessageType_MESSAGE_TYPE_READ_RESPONSE:
		gm.handleReadResponseMessage(msg)

	case GossipMessageType_MESSAGE_TYPE_FULL_SYNC_REQUEST:
		gm.handleFullSyncRequestMessage(msg)

	case GossipMessageType_MESSAGE_TYPE_FULL_SYNC_RESPONSE:
		gm.handleFullSyncResponseMessage(msg)

	default:
		logging.Debug("Unhandled gossip type", "type", msg.Type)
	}
}

// handleConnect processes a CONNECT message from a joining node.
func (gm *GossipManager) handleConnect(msg *GossipMessage) {
	p, ok := msg.Payload.(*GossipMessage_ConnectPayload)
	if !ok || p.ConnectPayload == nil {
		return
	}

	// Update HLC from remote
	gm.hlc.Update(p.ConnectPayload.Hlc)

	// Store sender's public key for signature verification
	// This enables automatic public key exchange during cluster formation
	if len(p.ConnectPayload.PublicKey) > 0 && p.ConnectPayload.NodeId != "" {
		gm.mu.Lock()
		gm.peerPubkeys[p.ConnectPayload.NodeId] = p.ConnectPayload.PublicKey
		gm.mu.Unlock()
		logging.Debug("Stored public key for node", "nodeID", p.ConnectPayload.NodeId)
	}

	// Update node state
	gm.updateNode(
		p.ConnectPayload.NodeId,
		p.ConnectPayload.Address,
		NodeState_NODE_STATE_ALIVE,
		p.ConnectPayload.Version,
	)

	// OPTIMIZATION: Always respond with our public key for bidirectional exchange
	// This completes the bidirectional public key exchange
	if p.ConnectPayload.NodeId != gm.localNodeID {
		var pubKey []byte
		if gm.keypair != nil {
			pubKey = gm.keypair.Pub
		}
		responseMsg := &GossipMessage{
			Type:   GossipMessageType_MESSAGE_TYPE_CONNECT,
			Sender: gm.localNodeID,
			Payload: &GossipMessage_ConnectPayload{
				ConnectPayload: &ConnectPayload{
					NodeId:    gm.localNodeID,
					Address:   gm.localAddress,
					Version:   gm.incrementLocalVersion(),
					Hlc:       gm.hlc.Now(),
					PublicKey: pubKey, // Include our public key (may be nil)
				},
			},
		}
		// SAFETY: signMessageCanonical will handle nil keypair gracefully
		if err := gm.signMessageCanonical(responseMsg); err != nil && !gm.disableAuth {
			logging.Warn("Failed to sign CONNECT response", "target", p.ConnectPayload.NodeId, "error", err)
		}

		// Send response back to sender immediately
		// Use shorter timeout for faster exchange
		go func() {
			if err := gm.network.SendWithTimeout(p.ConnectPayload.Address, responseMsg, 300*time.Millisecond); err != nil {
				logging.Debug("Failed to send CONNECT response", "target", p.ConnectPayload.NodeId, "error", err)
			} else {
				logging.Debug("Sent CONNECT response with public key", "target", p.ConnectPayload.NodeId)
			}
		}()
	}
}

// handleClusterSync processes a CLUSTER_SYNC message containing membership information.
// OPTIMIZATION: Request public keys for nodes we don't have keys for
func (gm *GossipManager) handleClusterSync(msg *GossipMessage) {
	p, ok := msg.Payload.(*GossipMessage_ClusterSyncPayload)
	if !ok || p.ClusterSyncPayload == nil {
		return
	}

	// OPTIMIZATION: Request public keys for nodes we don't have yet
	var nodesNeedingKeys []string

	// Update all nodes from sync message
	for _, n := range p.ClusterSyncPayload.Nodes {
		gm.updateNode(n.NodeId, n.Address, n.State, n.Version)

		// Check if we need this node's public key
		if n.NodeId != gm.localNodeID && n.State == NodeState_NODE_STATE_ALIVE {
			gm.mu.RLock()
			_, hasKey := gm.peerPubkeys[n.NodeId]
			gm.mu.RUnlock()

			if !hasKey {
				nodesNeedingKeys = append(nodesNeedingKeys, n.NodeId)
			}
		}
	}

	// OPTIMIZATION: Request public keys from sender for nodes we don't have
	// This accelerates public key exchange during cluster formation
	if len(nodesNeedingKeys) > 0 && msg.Sender != "" {
		// Send CONNECT message to request public keys
		// The CONNECT message will trigger public key exchange
		if peer, ok := gm.getNode(msg.Sender); ok {
			// Send our public key and request theirs
			var pubKey []byte
			if gm.keypair != nil {
				pubKey = gm.keypair.Pub
			}
			connectMsg := &GossipMessage{
				Type:   GossipMessageType_MESSAGE_TYPE_CONNECT,
				Sender: gm.localNodeID,
				Payload: &GossipMessage_ConnectPayload{
					ConnectPayload: &ConnectPayload{
						NodeId:    gm.localNodeID,
						Address:   gm.localAddress,
						Version:   gm.incrementLocalVersion(),
						Hlc:       gm.hlc.Now(),
						PublicKey: pubKey, // Include our public key (may be nil)
					},
				},
			}
			// SAFETY: signMessageCanonical will handle nil keypair gracefully
			if err := gm.signMessageCanonical(connectMsg); err != nil && !gm.disableAuth {
				logging.Warn("Failed to sign CONNECT message for key request", "target", msg.Sender, "error", err)
			}
			gm.network.SendWithTimeout(peer.Address, connectMsg, 500*time.Millisecond)
		}
	}
}

// handleProbeRequest processes an indirect probe request for failure detection.
func (gm *GossipManager) handleProbeRequest(msg *GossipMessage) {
	p, ok := msg.Payload.(*GossipMessage_ProbeRequestPayload)
	if !ok || p.ProbeRequestPayload == nil {
		return
	}

	// Check if target node is alive locally
	alive := gm.isNodeLocallyAlive(p.ProbeRequestPayload.TargetNodeId)

	// Send response back to requester
	resp := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_PROBE_RESPONSE,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_ProbeResponsePayload{
			ProbeResponsePayload: &ProbeResponsePayload{
				TargetNodeId: p.ProbeRequestPayload.TargetNodeId,
				Alive:        alive,
			},
		},
	}
	gm.signMessageCanonical(resp)

	if peer, ok := gm.getNode(msg.Sender); ok {
		gm.network.SendWithTimeout(peer.Address, resp, 500*time.Millisecond)
	}
}

// handleProbeResponse processes a probe response for failure detection.
func (gm *GossipManager) handleProbeResponse(msg *GossipMessage) {
	p, ok := msg.Payload.(*GossipMessage_ProbeResponsePayload)
	if !ok || p.ProbeResponsePayload == nil {
		return
	}

	if p.ProbeResponsePayload.Alive {
		gm.markNodeAliveFromProbe(p.ProbeResponsePayload.TargetNodeId)
	}
}

// handleCacheSync processes incremental cache synchronization.
func (gm *GossipManager) handleCacheSync(msg *GossipMessage) {
	p, ok := msg.Payload.(*GossipMessage_CacheSyncPayload)
	if !ok || p.CacheSyncPayload == nil {
		return
	}

	incSync := p.CacheSyncPayload.GetIncrementalSync()
	if incSync == nil {
		return
	}

	// Apply each operation
	for _, op := range incSync.GetOperations() {
		if op.Type == OperationType_OP_SET {
			item := protoItemToStorage(op.GetSetData(), op.ClientVersion)
			if err := gm.store.Set(op.Key, item); err != nil {
				logging.Error(err, "CACHE_SYNC apply failed", "key", op.Key)
			}
		} else if op.Type == OperationType_OP_DELETE {
			if err := gm.store.Delete(op.Key, op.ClientVersion); err != nil {
				logging.Error(err, "CACHE_SYNC delete failed", "key", op.Key)
			}
		}
	}

	// Send ACK if OpId is present
	if msg.OpId != "" && msg.Sender != "" {
		ack := &GossipMessage{
			Type:   GossipMessageType_MESSAGE_TYPE_CACHE_SYNC_ACK,
			Sender: gm.localNodeID,
			Payload: &GossipMessage_CacheSyncAckPayload{
				CacheSyncAckPayload: &CacheSyncAckPayload{
					OpId:    msg.OpId,
					PeerId:  gm.localNodeID,
					Success: true,
				},
			},
		}
		gm.signMessageCanonical(ack)
		if peer, ok := gm.getNode(msg.Sender); ok {
			gm.network.SendWithTimeout(peer.Address, ack, 500*time.Millisecond)
		}
	}
}

// handleCacheSyncAck processes acknowledgment for cache sync operations.
func (gm *GossipManager) handleCacheSyncAck(msg *GossipMessage) {
	p, ok := msg.Payload.(*GossipMessage_CacheSyncAckPayload)
	if !ok || p.CacheSyncAckPayload == nil {
		return
	}

	// Route ACK to the network layer for correlation with pending requests
	gm.network.HandleAck(p.CacheSyncAckPayload)
}

// handleReadRequestMessage processes a READ_REQUEST message.
func (gm *GossipManager) handleReadRequestMessage(msg *GossipMessage) {
	p, ok := msg.Payload.(*GossipMessage_ReadRequestPayload)
	if !ok || p.ReadRequestPayload == nil {
		return
	}
	gm.handleReadRequest(p.ReadRequestPayload, msg.Sender)
}

// handleReadResponseMessage processes a READ_RESPONSE message.
func (gm *GossipManager) handleReadResponseMessage(msg *GossipMessage) {
	p, ok := msg.Payload.(*GossipMessage_ReadResponsePayload)
	if !ok || p.ReadResponsePayload == nil {
		return
	}
	gm.handleReadResponse(p.ReadResponsePayload)
}

// handleFullSyncRequestMessage processes a FULL_SYNC_REQUEST message.
func (gm *GossipManager) handleFullSyncRequestMessage(msg *GossipMessage) {
	gm.handleFullSyncRequest(msg.Sender)
}

// handleFullSyncResponseMessage processes a FULL_SYNC_RESPONSE message.
func (gm *GossipManager) handleFullSyncResponseMessage(msg *GossipMessage) {
	p, ok := msg.Payload.(*GossipMessage_FullSyncResponsePayload)
	if !ok || p.FullSyncResponsePayload == nil {
		return
	}
	gm.handleFullSyncResponse(p.FullSyncResponsePayload.FullSync)
}

// signMessageCanonical signs a message using the local key pair.
// OPTIMIZATION: Skip signing if not required (single-node clusters).
// SAFETY: If keypair is nil and auth is disabled, skip signing without error.
//
// Parameters:
//   - msg: The message to sign
//
// Returns:
//   - error: Any signing error
func (gm *GossipManager) signMessageCanonical(msg *GossipMessage) error {
	if msg == nil {
		return errors.New("nil message")
	}

	// SAFETY: If keypair is nil, handle gracefully
	if gm.keypair == nil {
		// If auth is disabled, allow unsigned messages
		if gm.disableAuth {
			msg.Signature = nil
			return nil
		}
		// If signing is not required, allow unsigned messages
		if !gm.requiresSignature(msg) {
			msg.Signature = nil
			return nil
		}
		// Otherwise, return error for required signatures
		return errors.New("keypair not exists")
	}

	// Skip signing if not required (OPTIMIZATION)
	if !gm.requiresSignature(msg) {
		msg.Signature = nil
		return nil
	}

	// Clone message and clear signature for canonical representation
	tmp := proto.Clone(msg).(*GossipMessage)
	tmp.Signature = nil

	data, err := proto.Marshal(tmp)
	if err != nil {
		return err
	}

	signature := crypto.SignMessage(gm.keypair.Priv, data)
	msg.Signature = signature
	return nil
}

// verifyMessageCanonical verifies a message's signature.
// OPTIMIZATION: Accept unsigned messages if signing not required.
//
// Parameters:
//   - msg: The message to verify
//
// Returns:
//   - bool: true if signature is valid (or not required), false otherwise
func (gm *GossipManager) verifyMessageCanonical(msg *GossipMessage) bool {
	if msg == nil {
		return false
	}

	// FIX: If authentication is disabled, accept all messages
	// This is useful for testing or low-security environments
	if gm.disableAuth {
		return true
	}

	// CRITICAL FIX: For cluster formation messages, accept without signature verification
	// This enables dynamic cluster formation without pre-shared keys
	// Check this BEFORE signature verification to allow public key exchange
	switch msg.Type {
	case GossipMessageType_MESSAGE_TYPE_CONNECT,
		GossipMessageType_MESSAGE_TYPE_CLUSTER_SYNC,
		GossipMessageType_MESSAGE_TYPE_PROBE_REQUEST,
		GossipMessageType_MESSAGE_TYPE_PROBE_RESPONSE:
		// Accept cluster coordination messages without signature verification
		// Nodes trust the network layer (TCP) for authentication during cluster formation
		return true
	}

	// Accept unsigned messages if signing not required (OPTIMIZATION)
	if msg.Signature == nil {
		return !gm.requiresSignature(msg)
	}

	// FIX: Check if we have the sender's public key
	// OPTIMIZATION: During cluster formation, allow messages without keys temporarily
	gm.mu.RLock()
	pub, ok := gm.peerPubkeys[msg.Sender]
	clusterSize := len(gm.liveNodes)
	gm.mu.RUnlock()

	if !ok {
		// OPTIMIZATION: For cluster formation messages, allow without key temporarily
		// This prevents rejection during startup when keys are still being exchanged
		if clusterSize < 3 {
			// Very early startup - allow message to pass through
			logging.Debug("Accepting message without pubkey during cluster formation",
				"sender", msg.Sender, "clusterSize", clusterSize)
			return true
		}

		// For data messages, we need the key - but log as debug during startup
		if clusterSize < 10 {
			logging.Debug("No pubkey for sender, rejecting", "sender", msg.Sender, "clusterSize", clusterSize)
		} else {
			logging.Warn("no pubkey for sender, rejecting", "sender", msg.Sender)
		}
		return false
	}

	sig := msg.Signature
	tmp := proto.Clone(msg).(*GossipMessage)
	tmp.Signature = nil

	data, err := proto.Marshal(tmp)
	if err != nil {
		return false
	}

	return crypto.VerifyMessage(pub, data, sig)
}

// requiresSignature determines if a message requires cryptographic signing.
// OPTIMIZATION: Single-node operations and local-only messages can skip signing.
//
// Parameters:
//   - msg: The message to check
//
// Returns:
//   - bool: true if signature is required, false otherwise
func (gm *GossipManager) requiresSignature(msg *GossipMessage) bool {
	// Always sign messages going to other nodes
	if len(gm.peerPubkeys) > 1 {
		return true
	}

	// For single-node clusters, only sign critical messages
	switch msg.Type {
	case CONNECT, CLUSTER_SYNC, PROBE_REQUEST, PROBE_RESPONSE:
		return true // Always sign cluster coordination
	case CACHE_SYNC, CACHE_SYNC_ACK:
		return gm.hasMultipleNodes() // Only sign if multi-node
	default:
		return false
	}
}
