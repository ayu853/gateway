package cache

import (
	"container/list"
	"sync"
	"time"
)

// Entry represents a cached item.
type Entry struct {
	Key       string
	Value     []byte
	Headers   map[string]string
	Status    int
	ExpiresAt time.Time
}

// Cache implements an in-memory LRU cache with TTL support.
type Cache struct {
	enabled  bool
	ttl      time.Duration
	maxSize  int
	mu       sync.RWMutex
	items    map[string]*list.Element
	eviction *list.List
	hits     int64
	misses   int64
}

// New creates a new LRU cache.
func New(enabled bool, ttl time.Duration, maxSize int) *Cache {
	c := &Cache{
		enabled:  enabled,
		ttl:      ttl,
		maxSize:  maxSize,
		items:    make(map[string]*list.Element),
		eviction: list.New(),
	}

	// Start cleanup goroutine
	go c.cleanup()

	return c
}

// Get retrieves a cached entry by key.
func (c *Cache) Get(key string) (*Entry, bool) {
	if !c.enabled {
		return nil, false
	}

	c.mu.RLock()
	element, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	entry := element.Value.(*Entry)

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		c.Delete(key)
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Move to front (most recently used)
	c.mu.Lock()
	c.eviction.MoveToFront(element)
	c.hits++
	c.mu.Unlock()

	return entry, true
}

// Set stores an entry in the cache.
func (c *Cache) Set(key string, value []byte, headers map[string]string, status int) {
	if !c.enabled {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry
	if element, exists := c.items[key]; exists {
		c.eviction.MoveToFront(element)
		entry := element.Value.(*Entry)
		entry.Value = value
		entry.Headers = headers
		entry.Status = status
		entry.ExpiresAt = time.Now().Add(c.ttl)
		return
	}

	// Evict if at capacity
	for c.eviction.Len() >= c.maxSize {
		c.evictOldest()
	}

	// Add new entry
	entry := &Entry{
		Key:       key,
		Value:     value,
		Headers:   headers,
		Status:    status,
		ExpiresAt: time.Now().Add(c.ttl),
	}
	element := c.eviction.PushFront(entry)
	c.items[key] = element
}

// Delete removes an entry from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, exists := c.items[key]; exists {
		c.eviction.Remove(element)
		delete(c.items, key)
	}
}

func (c *Cache) evictOldest() {
	element := c.eviction.Back()
	if element != nil {
		c.eviction.Remove(element)
		entry := element.Value.(*Entry)
		delete(c.items, entry.Key)
	}
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, element := range c.items {
			entry := element.Value.(*Entry)
			if now.After(entry.ExpiresAt) {
				c.eviction.Remove(element)
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

// Stats returns cache hit/miss statistics.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return CacheStats{
		Size:    c.eviction.Len(),
		MaxSize: c.maxSize,
		Hits:    c.hits,
		Misses:  c.misses,
		HitRate: hitRate,
	}
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Size    int     `json:"size"`
	MaxSize int     `json:"max_size"`
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	HitRate float64 `json:"hit_rate_percent"`
}
