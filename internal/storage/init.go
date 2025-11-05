package storage

func init() {
	// Auto-register Memory backend
	RegisterBackend(BackendMemory, func(opts *StorageOptions) (Storage, error) {
		maxMem := opts.MaxMemoryMB
		if maxMem == 0 {
			maxMem = 1024 // Default: 1GB
		}
		return NewMemoryStorage(maxMem)
	})

	// Auto-register MemorySharded backend (256 shards for high concurrency)
	RegisterBackend(BackendMemorySharded, func(opts *StorageOptions) (Storage, error) {
		maxMem := opts.MaxMemoryMB
		if maxMem == 0 {
			maxMem = 1024 // Default: 1GB
		}
		return NewShardedMemoryStorage(maxMem)
	})
}
