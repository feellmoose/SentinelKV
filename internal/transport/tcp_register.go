// Package transport provides TCP transport implementation.
// Simply import this package to enable TCP support.
//
// The transport will be automatically registered and available for use.
// This transport has ZERO external dependencies (native Go).
// It is ALWAYS available and is the DEFAULT transport.
package transport

func init() {
	// Auto-register TCP transport when this package is imported
	// TCP is the default transport - always available
	RegisterTransport("tcp", func() (Transport, error) {
		return NewTCPTransport(), nil
	})
}

// TCP transport is now available!
// Use with:
//   Network: &gossip.NetworkOptions{
//       Type: gossip.TCP,
//   }
