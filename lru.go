// Copyright 2015 Daniel Pupius

package rcache

import (
	"container/list"
	"sync"
)

type lru struct {
	maxSizeBytes int64
	delegate     Cache
	mu           sync.Mutex
	elementMap   map[CacheKey]*list.Element
	elementList  *list.List // least recently used at the front.
}

// NewLRU returns a cache with a max byte size. Least recently used entries will
// be evicted first.
func NewLRU(name string, maxSizeBytes int64) Cache {
	return &lru{
		maxSizeBytes: maxSizeBytes,
		delegate:     New(name),
		elementMap:   make(map[CacheKey]*list.Element),
		elementList:  list.New(),
	}
}

func (l *lru) RegisterFetcher(fn interface{}) {
	l.delegate.RegisterFetcher(fn)
}

func (l *lru) Entries() []CacheEntry {
	// TODO(dan): Return copy that reflects LRU ordering.
	return l.delegate.Entries()
}

func (l *lru) Size() int64 {
	return l.delegate.Size()
}

func (l *lru) Invalidate(key CacheKey, recursive bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	// TODO(dan): recursive invalidation of LRU doesn't work.
	if ok := l.delegate.Invalidate(key, false); ok {
		l.elementList.Remove(l.elementMap[key])
		delete(l.elementMap, key)
		return true
	}
	return false
}

func (l *lru) Peek(key CacheKey) bool {
	return l.delegate.Peek(key)
}

func (l *lru) GetCacheEntry(key CacheKey) *CacheEntry {
	return l.delegate.GetCacheEntry(key)
}

func (l *lru) Get(key CacheKey) ([]byte, error) {
	e := l.GetCacheEntry(key)

	if e.Error == nil {
		l.mu.Lock()
		if element, ok := l.elementMap[e.Key]; ok {
			l.elementList.MoveToBack(element)
		} else {
			l.elementMap[key] = l.elementList.PushBack(key)
		}

		// Remove least recently used elements until cache is under capacity.
		for l.delegate.Size() > l.maxSizeBytes {
			f := l.elementList.Front()
			key := l.elementList.Remove(f).(CacheKey)
			delete(l.elementMap, key)
			l.delegate.Invalidate(key, false)
		}
		l.mu.Unlock()
	}

	return e.Bytes, e.Error
}
