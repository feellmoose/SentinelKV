package gossip

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/feellmoose/gridkv/internal/utils/crypto"
	"github.com/feellmoose/gridkv/internal/utils/hlc"
	"github.com/feellmoose/gridkv/internal/utils/logging"
	"github.com/feellmoose/gridkv/internal/utils/opid"
	"github.com/panjf2000/ants/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Constants for node states and message types for cleaner code
const (
	Alive   NodeState = NodeState_NODE_STATE_ALIVE   // Node is healthy and responsive
	Suspect NodeState = NodeState_NODE_STATE_SUSPECT // Node failed heartbeat and is suspected dead
	Dead    NodeState = NodeState_NODE_STATE_DEAD    // Node confirmed dead and removed from the ring
)

const (
	CACHE_SYNC         GossipMessageType = GossipMessageType_MESSAGE_TYPE_CACHE_SYNC
	CLUSTER_SYNC       GossipMessageType = GossipMessageType_MESSAGE_TYPE_CLUSTER_SYNC
	CONNECT            GossipMessageType = GossipMessageType_MESSAGE_TYPE_CONNECT
	PROBE_REQUEST      GossipMessageType = GossipMessageType_MESSAGE_TYPE_PROBE_REQUEST
	PROBE_RESPONSE     GossipMessageType = GossipMessageType_MESSAGE_TYPE_PROBE_RESPONSE
	CACHE_SYNC_ACK     GossipMessageType = GossipMessageType_MESSAGE_TYPE_CACHE_SYNC_ACK
	FULL_SYNC_REQUEST  GossipMessageType = GossipMessageType_MESSAGE_TYPE_FULL_SYNC_REQUEST
	FULL_SYNC_RESPONSE GossipMessageType = GossipMessageType_MESSAGE_TYPE_FULL_SYNC_RESPONSE
	READ_REQUEST       GossipMessageType = GossipMessageType_MESSAGE_TYPE_READ_REQUEST
	READ_RESPONSE      GossipMessageType = GossipMessageType_MESSAGE_TYPE_READ_RESPONSE
)

// GossipOptions contains configuration for the GossipManager.
type GossipOptions struct {
	LocalNodeID        string        // Unique identifier for this node
	LocalAddress       string        // Network address for this node (host:port)
	SeedAddrs          []string      // Bootstrap nodes for cluster formation
	FailureTimeout     time.Duration // Timeout before marking node as suspect
	SuspectTimeout     time.Duration // Timeout before marking suspect node as dead
	GossipInterval     time.Duration // Interval for periodic gossip broadcasts
	ReplicaCount       int           // N: Number of replicas for each key
	WriteQuorum        int           // W: Write quorum (must ack W replicas)
	ReadQuorum         int           // R: Read quorum (must read from R replicas)
	MaxReplicators     int           // Max concurrent replication goroutines
	ReplicationTimeout time.Duration // Timeout for replication operations
	ReadTimeout        time.Duration // Timeout for read operations
}

// GossipManager is the core component that manages cluster membership, failure detection,
// and data replication using a gossip protocol.
//
// Key responsibilities:
//   - Maintain cluster membership using SWIM-based failure detection
//   - Coordinate distributed reads/writes with quorum
//   - Replicate data across nodes using consistent hashing
//   - Synchronize state via incremental and full sync
//   - Sign and verify messages for security
type GossipManager struct {
	mu           sync.RWMutex
	localNodeID  string
	localAddress string
	liveNodes    map[string]*NodeInfo // Active cluster members

	hashRing *ConsistentHash // Consistent hash ring for data distribution
	store    KVStore         // Local storage backend
	network  Network         // Network layer for communication

	inputCh chan *GossipMessage // Incoming message queue
	stopCh  chan struct{}       // Shutdown signal
	wg      sync.WaitGroup      // Wait group for graceful shutdown

	// Configuration
	failureTimeout time.Duration
	suspectTimeout time.Duration
	gossipInterval time.Duration

	replicaCount       int
	writeQuorum        int
	readQuorum         int
	maxReplicators     int
	replicationTimeout time.Duration
	readTimeout        time.Duration

	// Versioning and timing
	localVersion int64           // Atomic counter for local version
	hlc          *hlc.HLC        // Hybrid logical clock
	opidGen      *opid.Generator // Operation ID generator

	// Cryptography
	keypair     *crypto.KeyPair             // Local key pair for signing
	peerPubkeys map[string]crypto.PublicKey // Peer public keys for verification

	// Read operation tracking
	pendingReads sync.Map // map[requestId]chan *ReadResponsePayload

	// Adaptive batching (OPTIMIZATION)
	msgRateCounter atomic.Int64 // Messages sent per second
	lastBatchSize  atomic.Int32 // Last calculated batch size
	lastRateCheck  atomic.Int64 // Last time we checked message rate

	// Goroutine pool for replication workers (OPTIMIZATION)
	replicationPool *ants.Pool
}

// NewGossipManager creates a new GossipManager instance with the specified configuration.
//
// This function initializes all subsystems including:
//   - Hybrid logical clock for distributed timestamps
//   - Operation ID generator for request tracking
//   - Goroutine pool for bounded concurrency
//   - Cryptographic signing for message authentication
//
// Parameters:
//   - opts: Configuration options (required)
//   - hashRing: Consistent hash ring for key distribution (required)
//   - network: Network layer for communication (required)
//   - store: Local storage backend (required)
//   - keypair: Cryptographic key pair for signing (optional)
//   - peerPubkeys: Map of peer public keys for verification (optional)
//
// Returns:
//   - *GossipManager: The initialized gossip manager
//   - error: Any initialization error
func NewGossipManager(opts *GossipOptions, hashRing *ConsistentHash, network Network, store KVStore, keypair *crypto.KeyPair, peerPubkeys map[string]crypto.PublicKey) (*GossipManager, error) {
	// Validate required parameters
	if opts == nil {
		return nil, errors.New("gossip options nil")
	}
	if opts.LocalNodeID == "" || opts.LocalAddress == "" {
		return nil, errors.New("LocalNodeID and LocalAddress required")
	}
	if hashRing == nil || network == nil || store == nil {
		return nil, errors.New("hash ring, network, and store cannot be nil")
	}

	// Set defaults for optional parameters
	if opts.FailureTimeout == 0 {
		opts.FailureTimeout = 5 * time.Second
	}
	if opts.SuspectTimeout == 0 {
		opts.SuspectTimeout = 10 * time.Second
	}
	if opts.GossipInterval == 0 {
		opts.GossipInterval = 1 * time.Second
	}
	if opts.ReplicaCount <= 0 {
		opts.ReplicaCount = 3
	}
	if opts.WriteQuorum <= 0 {
		opts.WriteQuorum = (opts.ReplicaCount / 2) + 1
	}
	if opts.ReadQuorum <= 0 {
		opts.ReadQuorum = (opts.ReplicaCount / 2) + 1
	}
	if opts.MaxReplicators <= 0 {
		opts.MaxReplicators = 4
	}
	if opts.ReplicationTimeout == 0 {
		opts.ReplicationTimeout = 2 * time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 2 * time.Second
	}

	rand.Seed(time.Now().UnixNano())

	gm := &GossipManager{
		localNodeID:        opts.LocalNodeID,
		localAddress:       opts.LocalAddress,
		liveNodes:          make(map[string]*NodeInfo),
		hashRing:           hashRing,
		store:              store,
		network:            network,
		inputCh:            make(chan *GossipMessage, 256),
		stopCh:             make(chan struct{}),
		failureTimeout:     opts.FailureTimeout,
		suspectTimeout:     opts.SuspectTimeout,
		gossipInterval:     opts.GossipInterval,
		replicaCount:       opts.ReplicaCount,
		writeQuorum:        opts.WriteQuorum,
		readQuorum:         opts.ReadQuorum,
		maxReplicators:     opts.MaxReplicators,
		replicationTimeout: opts.ReplicationTimeout,
		readTimeout:        opts.ReadTimeout,
		localVersion:       1,
		hlc:                hlc.NewHLC(opts.LocalNodeID),
		opidGen:            opid.NewGenerator(opts.LocalNodeID),
		keypair:            keypair,
		peerPubkeys:        peerPubkeys,
	}

	// Add local node to liveNodes and hash ring
	gm.liveNodes[gm.localNodeID] = &NodeInfo{
		NodeId:       gm.localNodeID,
		Address:      gm.localAddress,
		LastActiveTs: timestamppb.Now(),
		State:        NodeState_NODE_STATE_ALIVE,
		Version:      gm.localVersion,
	}

	// CRITICAL: Add local node to hash ring during initialization
	hashRing.Add(gm.localNodeID)

	// Initialize adaptive batching
	gm.lastBatchSize.Store(100) // Start with default batch size
	gm.lastRateCheck.Store(time.Now().Unix())

	// Initialize goroutine pool for replication (OPTIMIZATION)
	poolSize := opts.MaxReplicators * 4 // 4x buffer for burst traffic
	if poolSize < 16 {
		poolSize = 16 // Minimum pool size
	}
	pool, err := ants.NewPool(poolSize,
		ants.WithPreAlloc(true),     // Pre-allocate workers
		ants.WithNonblocking(false), // Block when pool is full
		ants.WithPanicHandler(func(err interface{}) {
			logging.Error(fmt.Errorf("panic in replication worker: %v", err), "replication pool panic")
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create replication pool: %w", err)
	}
	gm.replicationPool = pool

	return gm, nil
}

// Start initiates the GossipManager's background processes.
// This includes:
//   - Message processing loop
//   - Periodic gossip broadcasts
//   - Failure detection
//
// This method is non-blocking and returns immediately.
func (gm *GossipManager) Start() {
	gm.wg.Add(1)
	go gm.processLoop()
	logging.Debug("Gossip Manager started", "node", gm.localNodeID)

	// Send initial CONNECT message to announce presence
	join := &GossipMessage{
		Type:   GossipMessageType_MESSAGE_TYPE_CONNECT,
		Sender: gm.localNodeID,
		Payload: &GossipMessage_ConnectPayload{
			ConnectPayload: &ConnectPayload{
				NodeId:  gm.localNodeID,
				Address: gm.localAddress,
				Version: gm.incrementLocalVersion(),
				Hlc:     gm.hlc.Now(),
			},
		},
	}
	gm.signMessageCanonical(join)
	gm.SimulateReceive(join)
}

// Stop gracefully shuts down the GossipManager.
// This blocks until all background goroutines have exited.
func (gm *GossipManager) Stop() {
	close(gm.stopCh)
	gm.wg.Wait()

	// Release goroutine pool
	if gm.replicationPool != nil {
		gm.replicationPool.Release()
	}

	logging.Debug("Gossip Manager stopped", "node", gm.localNodeID)
}

// SimulateReceive enqueues a message for processing.
// This is the main entry point for incoming messages.
//
// Parameters:
//   - msg: The message to process
func (gm *GossipManager) SimulateReceive(msg *GossipMessage) {
	select {
	case gm.inputCh <- msg:
	default:
		logging.Warn("Dropped gossip message: input full", "type", msg.Type, "sender", msg.Sender)
	}
}

// processLoop is the main event loop that processes messages and runs periodic tasks.
func (gm *GossipManager) processLoop() {
	defer gm.wg.Done()

	// SAFETY: Recover from panics and restart the loop
	defer func() {
		if r := recover(); r != nil {
			logging.Error(fmt.Errorf("panic in processLoop: %v", r),
				"Gossip processLoop panic recovered - restarting")

			// Wait a bit before restarting to avoid rapid panic loops
			time.Sleep(1 * time.Second)

			// Restart the process loop
			gm.wg.Add(1)
			go gm.processLoop()
		}
	}()

	failureTick := time.NewTicker(gm.failureTimeout / 4)
	defer failureTick.Stop()

	gossipTick := time.NewTicker(gm.gossipInterval)
	defer gossipTick.Stop()

	for {
		select {
		case msg := <-gm.inputCh:
			// SAFETY: Recover from individual message processing panics
			func() {
				defer func() {
					if r := recover(); r != nil {
						logging.Error(fmt.Errorf("panic processing message: %v", r),
							"Message processing panic recovered",
							"sender", msg.GetSender(),
							"type", msg.GetType())
					}
				}()

				// Verify message signature
				if ok := gm.verifyMessageCanonical(msg); !ok {
					logging.Warn("Dropping msg: invalid signature", "sender", msg.Sender)
					return
				}
				gm.processGossipMessage(msg)
			}()

		case <-failureTick.C:
			// SAFETY: Protect failure detection
			func() {
				defer func() {
					if r := recover(); r != nil {
						logging.Error(fmt.Errorf("panic in failure detection: %v", r),
							"Failure detection panic recovered")
					}
				}()
				gm.runFailureDetection()
			}()

		case <-gossipTick.C:
			// SAFETY: Protect gossip broadcasting
			func() {
				defer func() {
					if r := recover(); r != nil {
						logging.Error(fmt.Errorf("panic in gossip periodic: %v", r),
							"Gossip periodic panic recovered")
					}
				}()
				gm.gossipPeriodically()
			}()

		case <-gm.stopCh:
			return
		}
	}
}

// Helper methods

// getNode retrieves node info by ID (thread-safe).
func (gm *GossipManager) getNode(id string) (*NodeInfo, bool) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	n, ok := gm.liveNodes[id]
	return n, ok
}

// isNodeLocallyAlive checks if a node is alive locally.
func (gm *GossipManager) isNodeLocallyAlive(nodeID string) bool {
	n, ok := gm.getNode(nodeID)
	return ok && n.State == NodeState_NODE_STATE_ALIVE
}

// incrementLocalVersion atomically increments and returns the local version counter.
func (gm *GossipManager) incrementLocalVersion() int64 {
	return atomic.AddInt64(&gm.localVersion, 1)
}

// generateOpID creates a unique operation ID for request tracking.
func (gm *GossipManager) generateOpID() string {
	return gm.opidGen.Generate()
}

// hasMultipleNodes checks if cluster has multiple nodes.
func (gm *GossipManager) hasMultipleNodes() bool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return len(gm.liveNodes) > 1
}

// getRandomPeerID returns a random peer ID, excluding the specified ID.
// OPTIMIZATION: Fast path for small clusters.
func (gm *GossipManager) getRandomPeerID(exclude string) string {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	// Fast path for small clusters
	if len(gm.liveNodes) <= 2 {
		for id := range gm.liveNodes {
			if id != gm.localNodeID && id != exclude {
				return id
			}
		}
		return ""
	}

	// Pre-allocate with exact capacity
	ids := make([]string, 0, len(gm.liveNodes))
	for id := range gm.liveNodes {
		if id == gm.localNodeID || id == exclude {
			continue
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return ""
	}

	return ids[rand.Intn(len(ids))]
}
