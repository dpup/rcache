// Copyright 2015 Daniel Pupius

// Package rcache provides a generic in-memory, read through, hierarchical
// cache, of []byte.
//
// Cache keys are structs that can provide detailed parameters which registered
// fetcher functions can use to create the requested resource. CacheKeys declare
// what keys they depend on, which allows for removal of down-stream entries.
//
// Due to the use of reflection for keys, cache misses are 2-5x slower than
// using a regular, typed map. But cache hits are fast.
//
//  BenchmarkCacheWithMisses      1000000      2217 ns/op
//  BenchmarkCacheWithHits        10000000     167 ns/op
//  BenchmarkNormalMapWithMisses  2000000      831 ns/op
package rcache

import (
	"sync"
	"time"
)

// The Cache interface has multiple implementations. Both provide generic
// fetcher functions for complex cache keys.
type Cache interface {
	// RegisterFetcher registers a fetcher function which the cache will use to load
	// data on a cache miss. The function should have a single argument, the type
	// of the argument should be unique to the fetcher.
	// The return value should be ([]byte error).
	RegisterFetcher(fn interface{})

	// Get returns the data for a key, falling back to a fetcher function if the
	// data hasn't yet been loaded. Concurrent callers will multiplex to the same
	// fetcher.
	Get(key interface{}) ([]byte, error)

	// GetCacheEntry is the same as Get but returns the meta cache entry.
	GetCacheEntry(key interface{}) *CacheEntry

	// Peek returns true if the key is currently cached. If the key is in the
	// process of being fetched, Peek will block and return true on success.
	Peek(key interface{}) bool

	// Invalidate removes an entry, and if `recursive` is true any entries that
	// depend on it, from the cache.
	Invalidate(key interface{}, recursive bool) bool

	// Entries returns an array of entries currently in the cache.
	Entries() []CacheEntry

	// Size returns the number of bytes stored in the cache.
	Size() int64
}

// CacheEntry stores details about an entry in the cache, including the content,
// when it was created, and when it was last accessed.
type CacheEntry struct {
	Key      interface{}
	Bytes    []byte
	Created  time.Time
	Accessed time.Time
	Error    error
	wg       sync.WaitGroup
}

// CacheKey is an interface that compound keys can implement in order to declare
// dependent keys that should be invalidated.
type CacheKey interface {
	// Dependencies returns an array of cache keys that the key is dependent on.
	Dependencies() []interface{}
}
