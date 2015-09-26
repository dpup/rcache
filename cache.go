// Copyright 2015 Daniel Pupius

package rcache

import (
	"expvar"
	"fmt"
	"reflect"
	"sync"
	"time"
)

var (
	byteArrayType = reflect.ValueOf([]byte{}).Type()
	errorType     = reflect.TypeOf((*error)(nil)).Elem()
)

type cache struct {
	fetchers        map[reflect.Type]reflect.Value
	cache           map[CacheKey]*CacheEntry
	cacheLock       sync.Mutex
	cacheSize       int64
	cacheSizeExpVar *expvar.Int
}

// New returns a new cache with no built-in eviction strategy. The cache's name
// is exposed with stats in expvar.
func New(name string) Cache {
	return &cache{
		fetchers:        make(map[reflect.Type]reflect.Value),
		cache:           make(map[CacheKey]*CacheEntry),
		cacheSizeExpVar: expvar.NewInt(fmt.Sprintf("cacheSize (%s)", name)),
	}
}

func (c *cache) RegisterFetcher(fn interface{}) {
	v := reflect.ValueOf(fn)
	t := v.Type()
	assertValidFetcher(t)

	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	// Map the argument type to the fetcher.
	arg := t.In(0)
	c.fetchers[arg] = v
}

func (c *cache) Get(key CacheKey) ([]byte, error) {
	e := c.GetCacheEntry(key)
	return e.Bytes, e.Error
}

func (c *cache) GetCacheEntry(key CacheKey) *CacheEntry {
	c.cacheLock.Lock()
	if entry, ok := c.cache[key]; ok {
		c.cacheLock.Unlock()
		entry.wg.Wait()
		entry.Accessed = time.Now()
		return entry
	}

	// Create the cache entry for future callers to wait on.
	entry := &CacheEntry{Key: key, Created: time.Now(), Accessed: time.Now()}
	entry.wg.Add(1)
	c.cache[key] = entry
	c.cacheLock.Unlock()

	entry.Bytes, entry.Error = c.fetch(key)
	entry.wg.Done()

	c.cacheLock.Lock()
	// We allow the error to be handled by current waiters, but don't persist it
	// for future callers.
	if entry.Error != nil {
		delete(c.cache, key)
	} else {
		size := int64(len(entry.Bytes))
		c.cacheSizeExpVar.Add(size)
		c.cacheSize += size
	}
	c.cacheLock.Unlock()

	return entry
}

func (c *cache) Peek(key CacheKey) bool {
	c.cacheLock.Lock()
	if entry, ok := c.cache[key]; ok {
		c.cacheLock.Unlock()
		entry.wg.Wait()
		return entry.Error == nil
	}
	c.cacheLock.Unlock()
	return false
}

func (c *cache) Entries() []CacheEntry {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	entries := make([]CacheEntry, 0, len(c.cache))
	for _, v := range c.cache {
		entries = append(entries, *v)
	}
	return entries
}

func (c *cache) Invalidate(key CacheKey, recursive bool) bool {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	return c.invalidate(key, recursive)
}

func (c *cache) Size() int64 {
	return c.cacheSize
}

func (c *cache) invalidate(key CacheKey, recursive bool) bool {
	if entry, ok := c.cache[key]; ok {
		size := int64(len(entry.Bytes))
		c.cacheSizeExpVar.Add(-size)
		c.cacheSize -= size
		delete(c.cache, key)
		if recursive {
			c.invalidateDependents(key)
		}
		return true
	}
	return false
}

func (c *cache) invalidateDependents(key CacheKey) {
	// TODO: this can be optimized.
	for k, _ := range c.cache {
		for _, dep := range k.Dependencies() {
			if dep == key {
				c.invalidate(k, true)
			}
		}
	}
}

// fetch uses reflection to look up the right fetcher, then requests the data.
func (c *cache) fetch(key CacheKey) ([]byte, error) {
	v := reflect.ValueOf(key)
	t := v.Type()
	if fetcher, ok := c.fetchers[t]; ok {
		values := fetcher.Call([]reflect.Value{v})
		// We've already verified types should be correct.
		if values[1].Interface() != nil {
			return []byte{}, values[1].Interface().(error)
		} else {
			return values[0].Bytes(), nil
		}
	} else {
		panic(fmt.Sprintf("cache: No fetcher function for type [%v]", t))
	}
}

func assertValidFetcher(t reflect.Type) {
	if t.Kind() != reflect.Func {
		panic(fmt.Sprintf("cache: Fetcher must be a function, got [%v]", t))
	}
	if t.NumIn() != 1 {
		panic(fmt.Sprintf("cache: Fetcher must be function with one arg, has %d [%v]", t.NumIn(), t))
	}
	if t.NumOut() != 2 || t.Out(0) != byteArrayType || t.Out(1) != errorType {
		panic(fmt.Sprintf("cache: Fetcher must be function that returns ([]byte, error), has %d [%v]", t.NumOut(), t))
	}
}
