package cache

import (
	"bytes"
	"container/list"
	"net/http"
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

// Middleware returns an HTTP middleware that caches GET responses.
func (c *Cache) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only cache GET requests
		if !c.enabled || r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		cacheKey := r.Method + ":" + r.URL.Path

		// Check cache
		if entry, ok := c.Get(cacheKey); ok {
			// Cache hit — serve from cache
			for k, v := range entry.Headers {
				w.Header().Set(k, v)
			}
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(entry.Status)
			w.Write(entry.Value)
			return
		}

		// Cache miss — capture the response
		w.Header().Set("X-Cache", "MISS")
		recorder := &cacheResponseWriter{
			ResponseWriter: w,
			statusCode:     200,
			body:           &bytes.Buffer{},
		}

		next.ServeHTTP(recorder, r)

		// Only cache successful responses
		if recorder.statusCode >= 200 && recorder.statusCode < 300 {
			headers := make(map[string]string)
			for k, v := range recorder.Header() {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}
			c.Set(cacheKey, recorder.body.Bytes(), headers, recorder.statusCode)
		}
	})
}

// cacheResponseWriter captures the response for caching.
type cacheResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (w *cacheResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
