package redis

import (
	"container/list"
	"errors"
	"sync"
)

type cacheEntry struct {
	key   string
	value string
}

// InMemoryFallbackCache is a small LRU cache used when Redis is unavailable.
type InMemoryFallbackCache struct {
	mu      sync.Mutex
	maxSize int
	items   map[string]*list.Element
	order   *list.List
}

// NewInMemoryFallbackCache creates an in-process LRU fallback cache.
func NewInMemoryFallbackCache(maxSize int) (*InMemoryFallbackCache, error) {
	if maxSize <= 0 {
		return nil, errors.New("fallback cache max size must be greater than 0")
	}
	return &InMemoryFallbackCache{
		maxSize: maxSize,
		items:   make(map[string]*list.Element, maxSize),
		order:   list.New(),
	}, nil
}

// Get reads a value and marks it as recently used.
func (c *InMemoryFallbackCache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[key]
	if !ok {
		return "", false
	}
	c.order.MoveToFront(element)
	entry := element.Value.(cacheEntry)
	return entry.value, true
}

// Set writes a value and evicts the least recently used item when full.
func (c *InMemoryFallbackCache) Set(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.items[key]; ok {
		element.Value = cacheEntry{key: key, value: value}
		c.order.MoveToFront(element)
		return
	}

	element := c.order.PushFront(cacheEntry{key: key, value: value})
	c.items[key] = element
	if len(c.items) > c.maxSize {
		c.evictOldest()
	}
}

// Delete removes a value if it exists.
func (c *InMemoryFallbackCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.items[key]
	if !ok {
		return
	}
	c.order.Remove(element)
	delete(c.items, key)
}

// Exists checks whether a key exists.
func (c *InMemoryFallbackCache) Exists(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, ok := c.items[key]
	return ok
}

func (c *InMemoryFallbackCache) evictOldest() {
	element := c.order.Back()
	if element == nil {
		return
	}
	entry := element.Value.(cacheEntry)
	c.order.Remove(element)
	delete(c.items, entry.key)
}
