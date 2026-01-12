package watcher

import (
	"sync"
	"time"
)

// FileCache provides a simple, reactive cache that automatically invalidates
// when the watcher detects changes. It's designed for caching file contents,
// directory listings, or any path-based data.
type FileCache struct {
	mu      sync.RWMutex
	data    map[string]cacheEntry
	watcher *Watcher
	ttl     time.Duration
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

// NewFileCache creates a cache that subscribes to the given watcher for auto-invalidation.
func NewFileCache(w *Watcher, ttl time.Duration) *FileCache {
	fc := &FileCache{
		data:    make(map[string]cacheEntry),
		watcher: w,
		ttl:     ttl,
	}

	// Subscribe to watcher events for auto-invalidation
	w.SubscribeFunc(func(evt Event) {
		fc.Invalidate(evt.Path)
	})

	return fc
}

// Get retrieves a value from the cache.
func (fc *FileCache) Get(path string) (interface{}, bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	entry, ok := fc.data[path]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false // Expired
	}

	return entry.value, true
}

// Set stores a value in the cache.
func (fc *FileCache) Set(path string, value interface{}) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.data[path] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(fc.ttl),
	}
}

// Invalidate removes an entry from the cache.
func (fc *FileCache) Invalidate(path string) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	delete(fc.data, path)
}

// InvalidateAll clears the entire cache.
func (fc *FileCache) InvalidateAll() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.data = make(map[string]cacheEntry)
}

// Size returns the number of cached entries.
func (fc *FileCache) Size() int {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return len(fc.data)
}
