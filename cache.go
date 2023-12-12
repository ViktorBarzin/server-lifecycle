package main

import (
	"container/list"
	"fmt"
	"sync"
)

// LRUCache represents an LRU cache
type LRUCache struct {
	capacity int
	cache    map[string]*list.Element
	lruList  *list.List
	mutex    sync.Mutex
}

// CacheEntry represents an entry in the cache
type CacheEntry struct {
	key   string
	value interface{}
}

// NewLRUCache creates a new LRUCache with the specified capacity
func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		lruList:  list.New(),
	}
}

// Get retrieves the value associated with the key from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, exists := c.cache[key]; exists {
		c.lruList.MoveToFront(element)
		return element.Value.(*CacheEntry).value, true
	}

	return nil, false
}

// Put adds or updates an entry in the cache
func (c *LRUCache) Put(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if the key already exists
	if element, exists := c.cache[key]; exists {
		c.lruList.MoveToFront(element)
		element.Value.(*CacheEntry).value = value
	} else {
		// If the cache is at capacity, evict the least recently used item
		if len(c.cache) == c.capacity {
			back := c.lruList.Back()
			if back != nil {
				delete(c.cache, back.Value.(*CacheEntry).key)
				c.lruList.Remove(back)
			}
		}

		// Add the new entry to the cache
		entry := &CacheEntry{key: key, value: value}
		element := c.lruList.PushFront(entry)
		c.cache[key] = element
	}
}

// PrintCache prints the contents of the cache
func (c *LRUCache) PrintCache() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	fmt.Print("LRU Cache: ")
	for element := c.lruList.Front(); element != nil; element = element.Next() {
		entry := element.Value.(*CacheEntry)
		fmt.Printf("(%s:%v) ", entry.key, entry.value)
	}
	fmt.Println()
}
