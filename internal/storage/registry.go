package storage

import (
	"fmt"
	"sync"
)

// BackendFactory is a function that creates a storage backend from options
type BackendFactory func(opts *StorageOptions) (Storage, error)

// Global backend registry
var (
	backendRegistry   = make(map[StorageBackendType]BackendFactory)
	backendRegistryMu sync.RWMutex
)

// RegisterBackend registers a storage backend factory.
// This allows conditional compilation - backends with external dependencies
// can be excluded via build tags.
func RegisterBackend(backend StorageBackendType, factory BackendFactory) {
	backendRegistryMu.Lock()
	defer backendRegistryMu.Unlock()
	backendRegistry[backend] = factory
}

// NewStorage creates a storage backend using the registered factory.
// This function is build-tag aware and will fail gracefully if a backend
// isn't compiled in.
func NewStorage(opts *StorageOptions) (Storage, error) {
	if opts == nil {
		return nil, fmt.Errorf("storage options cannot be nil")
	}

	backendRegistryMu.RLock()
	factory, exists := backendRegistry[opts.Backend]
	backendRegistryMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("storage backend '%s' not available (may need to enable with build tags)", opts.Backend)
	}

	return factory(opts)
}

// AvailableBackends returns a list of currently available storage backends.
// This list depends on build tags used during compilation.
func AvailableBackends() []StorageBackendType {
	backendRegistryMu.RLock()
	defer backendRegistryMu.RUnlock()

	backends := make([]StorageBackendType, 0, len(backendRegistry))
	for backend := range backendRegistry {
		backends = append(backends, backend)
	}
	return backends
}

// IsBackendAvailable checks if a specific backend is available.
func IsBackendAvailable(backend StorageBackendType) bool {
	backendRegistryMu.RLock()
	defer backendRegistryMu.RUnlock()
	_, exists := backendRegistry[backend]
	return exists
}

// GetBackendInfo returns information about available backends
func GetBackendInfo() string {
	backends := AvailableBackends()
	if len(backends) == 0 {
		return "No storage backends available (check build tags)"
	}

	info := "Available storage backends:\n"
	for _, backend := range backends {
		info += fmt.Sprintf("  - %s\n", backend)
	}
	return info
}
