// internal/pokecache/cache.go
package pokecache

import (
	"sync"
	"time"
)

// cacheEntry holds the data and its creation time.
type cacheEntry struct {
	createdAt time.Time
	val       []byte
}

// Cache represents a thread-safe cache for storing PokeAPI responses.
type Cache struct {
	mu       sync.Mutex
	entries  map[string]cacheEntry
	interval time.Duration
}

// NewCache creates a new Cache instance with a specified reaping interval.
func NewCache(interval time.Duration) *Cache {
	cache := &Cache{
		entries:  make(map[string]cacheEntry),
		interval: interval,
	}

	go cache.reapLoop() // Start the reaping loop in a goroutine.
	return cache
}

// Add adds a new entry to the cache.
func (c *Cache) Add(key string, val []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry{
		createdAt: time.Now(),
		val:       val,
	}
}

// Get retrieves an entry from the cache.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, found := c.entries[key]
	if !found {
		return nil, false
	}

	return entry.val, true
}

// reapLoop removes old entries from the cache based on the defined interval.
func (c *Cache) reapLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		<-ticker.C
		c.mu.Lock()
		for key, entry := range c.entries {
			if time.Since(entry.createdAt) > c.interval {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}
