package gossip

import (
	"sort"
	"strconv"
	"sync"

	"github.com/cespare/xxhash/v2" // Using high-performance xxHash
)

// Hash defines the function signature for generating a hash key from a byte slice.
type Hash func(data []byte) uint32

// ConsistentHash represents the core structure of the consistent hash ring.
type ConsistentHash struct {
	mu sync.RWMutex // RWMutex to protect the state of the ring and nodes.

	hashFunc Hash // The hash function used to compute key and node hashes.
	replicas int  // The number of virtual nodes (replicas) for each real node.

	// keys is a sorted list of virtual node hash values for binary search on the ring.
	keys []uint32

	// ring maps virtual node hash values to their corresponding real node name.
	ring map[uint32]string

	// nodes stores the names of all real nodes added to the ring, used for quick checks and removal.
	nodes map[string]bool
}

// NewConsistentHash creates a new ConsistentHash ring instance.
// replicas: specifies the number of virtual nodes created for each real node. More replicas improve load balancing but increase memory consumption.
// fn: Optional hash function; if nil, it defaults to the high-performance xxHash (XXH32).
func NewConsistentHash(replicas int, fn Hash) *ConsistentHash {
	if fn == nil {
		// Default to the high-performance xxHash (XXH32) for fast and balanced distribution.
		fn = func(data []byte) uint32 {
			// Using XXH32, which is faster than traditional CRC32/FNV for this purpose.
			return uint32(xxhash.Sum64(data))
		}
	}

	return &ConsistentHash{
		hashFunc: fn,
		replicas: replicas,
		ring:     make(map[uint32]string),
		nodes:    make(map[string]bool),
	}
}

// AddNode adds a real node and all its virtual replicas to the hash ring.
// name: The real node's name (e.g., "server-1").
// OPTIMIZED: Uses binary search insert to maintain sorted order, avoiding O(n log n) full sort.
func (c *ConsistentHash) Add(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if the node already exists to prevent duplicate addition.
	if c.nodes[name] {
		return
	}
	c.nodes[name] = true

	// OPTIMIZATION: Pre-allocate capacity for all new virtual nodes
	newKeys := make([]uint32, 0, len(c.keys)+c.replicas)

	// Collect all hashes for this node first
	hashes := make([]uint32, c.replicas)
	for i := 0; i < c.replicas; i++ {
		// Generate a unique key for each virtual node: ReplicaIndex + NodeName.
		key := strconv.Itoa(i) + name
		hash := c.hashFunc([]byte(key))
		hashes[i] = hash

		// Store the virtual node hash and its corresponding real node name.
		c.ring[hash] = name
	}

	// Sort the new hashes to minimize insertions
	sort.Slice(hashes, func(i, j int) bool {
		return hashes[i] < hashes[j]
	})

	// OPTIMIZATION: Merge sorted arrays in O(n) instead of O(n log n) sort
	i, j := 0, 0
	for i < len(c.keys) && j < len(hashes) {
		if c.keys[i] < hashes[j] {
			newKeys = append(newKeys, c.keys[i])
			i++
		} else {
			newKeys = append(newKeys, hashes[j])
			j++
		}
	}

	// Append remaining elements
	if i < len(c.keys) {
		newKeys = append(newKeys, c.keys[i:]...)
	}
	if j < len(hashes) {
		newKeys = append(newKeys, hashes[j:]...)
	}

	c.keys = newKeys
}

// RemoveNode deletes a real node and all its virtual replicas from the hash ring.
// OPTIMIZED: Efficiently removes hashes without full rebuild.
func (c *ConsistentHash) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if the node does not exist.
	if !c.nodes[name] {
		return
	}
	delete(c.nodes, name)

	// OPTIMIZATION: Build set of hashes to remove
	toRemove := make(map[uint32]bool, c.replicas)
	for i := 0; i < c.replicas; i++ {
		key := strconv.Itoa(i) + name
		hash := c.hashFunc([]byte(key))

		// Delete the virtual node from the ring map.
		delete(c.ring, hash)
		toRemove[hash] = true
	}

	// OPTIMIZATION: Filter keys in-place to avoid allocation
	// This is O(n) instead of O(n log n) rebuild+sort
	newLen := 0
	for i := 0; i < len(c.keys); i++ {
		if !toRemove[c.keys[i]] {
			c.keys[newLen] = c.keys[i]
			newLen++
		}
	}
	c.keys = c.keys[:newLen]
	// Keys remain sorted since we only removed elements
}

// GetNode finds the first virtual node encountered clockwise on the hash ring for a given key (e.g., data item, user ID)
// and returns its corresponding real node name. This is the primary node.
// key: The key to be looked up.
// Returns: The real node name responsible for storing or processing the key; returns an empty string if the ring is empty.
func (c *ConsistentHash) Get(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.keys) == 0 {
		return "" // Ring is empty.
	}

	// Calculate the hash value of the key.
	hash := c.hashFunc([]byte(key))

	// Use Go's standard library sort.Search for binary lookup.
	// Find the index of the first virtual node hash value that is greater than or equal to 'hash'.
	i := sort.Search(len(c.keys), func(j int) bool {
		return c.keys[j] >= hash
	})

	// Wrap-around logic: If i equals the length of the keys list, the key's hash is greater than all virtual node hashes,
	// so it should wrap back to the start of the ring, keys[0].
	index := i % len(c.keys)

	// Return the real node name corresponding to that virtual node.
	return c.ring[c.keys[index]]
}

// GetReplicas finds and returns a list of N unique real node names responsible for the given key (including the primary node).
// This list is found by traversing the ring clockwise starting from the key's position.
// replicationFactor: The desired number of unique nodes to return (N).
func (c *ConsistentHash) GetN(key string, replicationFactor int) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.keys) == 0 || replicationFactor == 0 {
		return nil
	}

	if replicationFactor > len(c.nodes) {
		replicationFactor = len(c.nodes)
	}

	hash := c.hashFunc([]byte(key))

	i := sort.Search(len(c.keys), func(j int) bool {
		return c.keys[j] >= hash
	})

	var nodes []string
	seen := make(map[string]bool)

	startIndex := i

	for count := 0; len(nodes) < replicationFactor && count < len(c.keys); count++ {
		currentIdx := (startIndex + count) % len(c.keys)

		virtualNodeHash := c.keys[currentIdx]
		nodeID := c.ring[virtualNodeHash]

		if !seen[nodeID] {
			nodes = append(nodes, nodeID)
			seen[nodeID] = true
		}
	}

	return nodes
}

// GetNodes returns a list of the names of all real nodes in the ring.
func (c *ConsistentHash) Members() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.nodes))
	for name := range c.nodes {
		names = append(names, name)
	}
	return names
}
