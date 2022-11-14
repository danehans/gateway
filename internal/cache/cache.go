package cache

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

const (
	NoExpiration      time.Duration = -1
	DefaultExpiration time.Duration = 0
)

// Cache defines a cache for storing items.
type Cache struct {
	defaultExpiration time.Duration
	items             map[string]Item
	mu                sync.RWMutex
	onEvicted         func(string, interface{})
	manager           *manager
}

// Item defines an item stored in the cache.
type Item struct {
	Object     interface{}
	Expiration int64
}

// Expired returns true if the item has expired.
func (i Item) Expired() bool {
	if i.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > i.Expiration
}

// New returns a new cache with the provided expiration and cleanup interval.
// If the expiration duration is less than NoExpiration, the items in the cache
// never expire. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
func New(expiration, cleanupInterval time.Duration) *Cache {
	items := make(map[string]Item)

	if expiration == 0 {
		expiration = NoExpiration
	}
	c := &Cache{
		defaultExpiration: expiration,
		items:             items,
	}

	if cleanupInterval > 0 {
		runManager(c, cleanupInterval)
		runtime.SetFinalizer(c, stopManager)
	}

	return c
}

// Store stores an item in the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *Cache) Store(key string, val interface{}, duration time.Duration) {
	var exp int64

	if duration == DefaultExpiration {
		duration = c.defaultExpiration
	}
	if duration > 0 {
		exp = time.Now().Add(duration).UnixNano()
	}

	c.mu.Lock()
	c.items[key] = Item{
		Object:     val,
		Expiration: exp,
	}
	c.mu.Unlock()
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()

	item, found := c.items[key]
	if !found {
		c.mu.RUnlock()
		return nil, false
	}

	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.mu.RUnlock()
			return nil, false
		}
	}
	c.mu.RUnlock()

	return item.Object, true
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *Cache) Add(key string, val interface{}, duration time.Duration) error {
	c.mu.Lock()
	_, found := c.Get(key)
	if found {
		c.mu.Unlock()
		return fmt.Errorf("item %s already exists", key)
	}
	c.Store(key, val, duration)
	c.mu.Unlock()
	return nil
}

type keyAndValue struct {
	key   string
	value interface{}
}

// DeleteExpired deletes all expired items from the cache.
func (c *Cache) DeleteExpired() {
	var evictedItems []keyAndValue

	c.mu.Lock()
	for k, item := range c.items {
		if item.Expired() {
			val, evicted := c.delete(k)
			if evicted {
				evictedItems = append(evictedItems, keyAndValue{k, val})
			}
		}
	}
	c.mu.Unlock()

	for _, v := range evictedItems {
		c.onEvicted(v.key, v.value)
	}
}

func (c *Cache) delete(key string) (interface{}, bool) {
	if c.onEvicted != nil {
		if v, found := c.items[key]; found {
			delete(c.items, key)
			return v.Object, true
		}
	}
	delete(c.items, key)

	return nil, false
}

// OnEvicted sets a function that is called with the key and value when an
// item is evicted from the cache.
func (c *Cache) OnEvicted(f func(string, interface{})) {
	c.mu.Lock()
	c.onEvicted = f
	c.mu.Unlock()
}

// Manager manages the cache.
type manager struct {
	interval time.Duration
	stop     chan bool
}

func runManager(c *Cache, ci time.Duration) {
	m := &manager{
		interval: ci,
		stop:     make(chan bool),
	}
	c.manager = m
	go m.Run(c)
}

func stopManager(c *Cache) {
	c.manager.stop <- true
}

func (m *manager) Run(c *Cache) {
	ticker := time.NewTicker(m.interval)
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-m.stop:
			ticker.Stop()
			return
		}
	}
}
