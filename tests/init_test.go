package tests

// This file ensures all transports are available for benchmarks
// Using pure import control - the Go way!

import (
	// Import TCP transport (default)
	_ "github.com/feellmoose/gridkv/internal/transport"
)

// Memory backends are built-in and automatically available.
// Transport is now available for benchmarks!
