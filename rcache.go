// Copyright 2015 Daniel Pupius

// Package cache provides a generic in-memory, read through, hierarchical cache,
// of []byte.
//
// Cache keys are structs that can provide detailed parameters for the requested
// resource. CacheKeys declare what other keys they depend on, which allows for
// removal of down-stream entries.
//
// The following example requests images for a car, then has a dependent cache
// for thumbnails. If the original image is invalidated, so are the thumbnails.
//
//  type CarKey struct {
//    Manufacturer string
//    Model string
//  }
//
//  func (key CarKey) Dependencies []rcache.CacheKey {
//    return rcache.NoDeps
//  }
//
//  type ThumbnailKey struct {
//    Manufacturer string
//    Model string
//    Size  int
//  }
//
//  func (key ThumbnailKey) Dependencies() []rcache.CacheKey {
//    return []CacheKey{CarKey{key.Manufacturer, key.Model}}
//  }
//
//  c := rcache.New("mycache")
//  c.RegisterFetcher(func(key CarKey) ([]byte, error) {
//    return imageLibrary.GetCarImage(key.Manufacturer, key.Model), nil
//  })
//  c.RegisterFetcher(func(key ThumbnailKey) ([]byte, error) {
//    fullImage := c.Get(CarKey{key.Manufacturer, key.Model})
//    return imageProc.Resize(fullImage, key.Size)
//  })
//
//  // Original image is only fetched once.
//  t200, err := c.Get(ThumbnailKey{"BMW", "M5", 200})
//  t400, err := c.Get(ThumbnailKey{"BMW", "M5", 400})
//
// Due to the use of reflection for keys, cache misses are 2-5x slower than
// using a regular, typed map. But cache hits are fast.
//
//  BenchmarkCacheWithMisses      1000000      2217 ns/op
//  BenchmarkCacheWithHits        10000000     167 ns/op
//  BenchmarkNormalMapWithMisses  2000000      831 ns/op
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

type CacheEntry struct {
	Key      CacheKey
	Bytes    []byte
	Created  time.Time
	Accessed time.Time
	Error    error
	wg       sync.WaitGroup
}

type Cache struct {
	fetchers        map[reflect.Type]reflect.Value
	cache           map[CacheKey]*CacheEntry
	cacheLock       sync.Mutex
	cacheSize       int64
	cacheSizeExpVar *expvar.Int
}

func New(name string) *Cache {
	return &Cache{
		fetchers:        make(map[reflect.Type]reflect.Value),
		cache:           make(map[CacheKey]*CacheEntry),
		cacheSizeExpVar: expvar.NewInt(fmt.Sprintf("cacheSize (%s)", name)),
	}
}

// RegisterFetcher registers a fetcher function which the cache will use to load
// data on a cache miss. The function should have a single argument, a type that
// implements the CacheKey interface. The return value should be ([]byte error).
func (c *Cache) RegisterFetcher(fn interface{}) {
	v := reflect.ValueOf(fn)
	t := v.Type()
	assertValidFetcher(t)

	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()

	// Map the argument type to the fetcher.
	arg := t.In(0)
	c.fetchers[arg] = v
}

// Get returns the data for a key, falling back to a fetcher function if the
// data hasn't yet been loaded. Concurrent callers will multiplex to the same
// fetcher.
func (c *Cache) Get(key CacheKey) ([]byte, error) {
	e := c.GetCacheEntry(key)
	return e.Bytes, e.Error
}

// GetCacheEntry is the same as Get but returns the meta cache entry.
func (c *Cache) GetCacheEntry(key CacheKey) *CacheEntry {
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

// Peek returns true if the key is currently cached. If the key is in the
// process of being fetched, Peek will block and return true on success.
func (c *Cache) Peek(key CacheKey) bool {
	c.cacheLock.Lock()
	if entry, ok := c.cache[key]; ok {
		c.cacheLock.Unlock()
		entry.wg.Wait()
		return entry.Error == nil
	}
	c.cacheLock.Unlock()
	return false
}

// Entries returns an array of entries currently in the cache.
func (c *Cache) Entries() []CacheEntry {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	entries := make([]CacheEntry, 0, len(c.cache))
	for _, v := range c.cache {
		entries = append(entries, *v)
	}
	return entries
}

// Invalidate removes an entry, and any entries that depend on it, from the cache.
func (c *Cache) Invalidate(key CacheKey) bool {
	c.cacheLock.Lock()
	defer c.cacheLock.Unlock()
	return c.invalidate(key)
}

func (c *Cache) Size() int64 {
	return c.cacheSize
}

func (c *Cache) invalidate(key CacheKey) bool {
	if entry, ok := c.cache[key]; ok {
		size := int64(len(entry.Bytes))
		c.cacheSizeExpVar.Add(-size)
		c.cacheSize -= size
		delete(c.cache, key)
		c.invalidateDependents(key)
		return true
	}
	return false
}

func (c *Cache) invalidateDependents(key CacheKey) {
	// TODO: this can be optimized.
	for k, _ := range c.cache {
		for _, dep := range k.Dependencies() {
			if dep == key {
				c.invalidate(k)
			}
		}
	}
}

// fetch uses reflection to look up the right fetcher, then requests the data.
func (c *Cache) fetch(key CacheKey) ([]byte, error) {
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
