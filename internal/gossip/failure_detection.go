package gossip

import (
	"errors"
	"time"

	"github.com/feellmoose/gridkv/internal/utils/logging"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// updateNode updates or adds a node to the cluster membership.
// This method implements vector clock-based conflict resolution.
//
// Parameters:
//   - nodeID: Unique identifier for the node
//   - address: Network address of the node
//   - newState: New state for the node (ALIVE, SUSPECT, DEAD)
//   - version: Version number for conflict resolution
func (gm *GossipManager) updateNode(nodeID, address string, newState NodeState, version int64) {
	var add, remove bool

	gm.mu.Lock()
	now := timestamppb.Now()
	existing, found := gm.liveNodes[nodeID]

	// Ignore stale updates (version-based conflict resolution)
	if found && version < existing.Version {
		gm.mu.Unlock()
		logging.Debug("Ignore stale gossip", "node", nodeID)
		return
	}

	// Handle new node
	if !found {
		gm.liveNodes[nodeID] = &NodeInfo{
			NodeId:       nodeID,
			Address:      address,
			LastActiveTs: now,
			State:        NodeState_NODE_STATE_ALIVE,
			Version:      version,
		}
		if nodeID != gm.localNodeID {
			add = true
		}
		gm.mu.Unlock()

		if add {
			gm.hashRing.Add(nodeID)
			logging.Debug("MEMBER JOIN added to ring", "node", nodeID)
		}
		return
	}

	// Handle existing node state transition
	if version >= existing.Version {
		if newState == NodeState_NODE_STATE_DEAD && existing.State != NodeState_NODE_STATE_DEAD {
			remove = true
		}
		if newState == NodeState_NODE_STATE_ALIVE && existing.State != NodeState_NODE_STATE_ALIVE && nodeID != gm.localNodeID {
			add = true
		}
		existing.State = newState
		existing.LastActiveTs = now
		existing.Version = version
	}
	gm.mu.Unlock()

	// Apply ring changes outside lock
	if remove {
		gm.hashRing.Remove(nodeID)
		logging.Error(errors.New("MEMBER DEAD"), "removed from ring", "node", nodeID)
	}
	if add {
		gm.hashRing.Add(nodeID)
		logging.Debug("re-added to ring", "node", nodeID)
	}
}

// markNodeAliveFromProbe marks a suspect node as alive after successful probe.
//
// Parameters:
//   - nodeID: ID of the node to mark alive
func (gm *GossipManager) markNodeAliveFromProbe(nodeID string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if n, ok := gm.liveNodes[nodeID]; ok && n.State == NodeState_NODE_STATE_SUSPECT {
		n.State = NodeState_NODE_STATE_ALIVE
		n.LastActiveTs = timestamppb.Now()
		n.Version = gm.incrementLocalVersion()
		logging.Debug("MEMBER RECOVER via probe", "node", nodeID)
	}
}

// runFailureDetection implements SWIM-style failure detection.
// This runs periodically to detect and mark failed nodes.
//
// State transitions:
//   - ALIVE -> SUSPECT: After FailureTimeout with no heartbeat
//   - SUSPECT -> DEAD: After FailureTimeout + SuspectTimeout
//   - DEAD -> Removed: After cleanup period
func (gm *GossipManager) runFailureDetection() {
	now := time.Now()
	var toRemove, toMarkDead []string

	gm.mu.Lock()
	for id, node := range gm.liveNodes {
		// Skip local node
		if id == gm.localNodeID {
			continue
		}

		elapsed := now.Sub(node.LastActiveTs.AsTime())

		switch node.State {
		case NodeState_NODE_STATE_ALIVE:
			// Mark as suspect after failure timeout
			if elapsed > gm.failureTimeout {
				node.State = NodeState_NODE_STATE_SUSPECT
				node.Version = gm.incrementLocalVersion()
				go gm.initiateProbe(id)
				logging.Warn("MEMBER SUSPECT", "node", id, "elapsed", elapsed)
			}

		case NodeState_NODE_STATE_SUSPECT:
			// Mark as dead after suspect timeout
			if elapsed > gm.failureTimeout+gm.suspectTimeout {
				node.State = NodeState_NODE_STATE_DEAD
				node.Version = gm.incrementLocalVersion()
				toMarkDead = append(toMarkDead, id)
				logging.Error(errors.New("MEMBER DEAD"), "member dead", "node", id)
			}

		case NodeState_NODE_STATE_DEAD:
			// Remove dead node after cleanup period
			if elapsed > gm.failureTimeout+gm.suspectTimeout {
				toRemove = append(toRemove, id)
			}
		}
	}
	gm.mu.Unlock()

	// Remove dead nodes from hash ring
	for _, id := range toMarkDead {
		gm.hashRing.Remove(id)
	}

	// Clean up dead nodes from memory
	if len(toRemove) > 0 {
		gm.mu.Lock()
		for _, id := range toRemove {
			delete(gm.liveNodes, id)
		}
		gm.mu.Unlock()
	}
}

// initiateProbe performs an indirect probe through a random peer.
// This is part of the SWIM protocol's indirect probing mechanism.
//
// Parameters:
//   - suspectID: ID of the suspect node to probe
func (gm *GossipManager) initiateProbe(suspectID string) {
	// Select a random peer to perform indirect probe
	pinger := gm.getRandomPeerID(suspectID)
	if pinger == "" {
		logging.Warn("PROBE: no pinger", "suspect", suspectID)
		return
	}

	peer, ok := gm.getNode(pinger)
	if !ok {
		return
	}

	// Send probe request
	msg := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_PROBE_REQUEST,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_ProbeRequestPayload{
			ProbeRequestPayload: &ProbePayload{
				TargetNodeId: suspectID,
				RequesterId:  gm.localNodeID,
			},
		},
	}
	gm.signMessageCanonical(msg)
	gm.network.SendWithTimeout(peer.Address, msg, 500*time.Millisecond)
}
