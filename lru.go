// Copyright 2015 Daniel Pupius

package rcache

import (
	"container/list"
	"sync"
)

type LRU struct {
	maxSizeBytes int64
	delegate     *Cache
	mu           sync.Mutex
	elementMap   map[CacheKey]*list.Element
	elementList  *list.List // least recently used at the front.
}

func NewLRU(name string, maxSizeBytes int64) *LRU {
	return &LRU{
		maxSizeBytes: maxSizeBytes,
		delegate:     New(name),
		elementMap:   make(map[CacheKey]*list.Element),
		elementList:  list.New(),
	}
}

func (l *LRU) RegisterFetcher(fn interface{}) {
	l.delegate.RegisterFetcher(fn)
}

func (l *LRU) Entries() []CacheEntry {
	// TODO(dan): Return copy that reflects LRU ordering.
	return l.delegate.Entries()
}

func (l *LRU) Size() int64 {
	return l.delegate.Size()
}

func (l *LRU) Invalidate(key CacheKey) bool {
	if ok := l.delegate.Invalidate(key); ok {
		l.elementList.Remove(l.elementMap[key])
		delete(l.elementMap, key)
		return true
	}
	return false
}

func (l *LRU) Peek(key CacheKey) bool {
	return l.delegate.Peek(key)
}

func (l *LRU) Get(key CacheKey) ([]byte, error) {
	e := l.delegate.GetCacheEntry(key)

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
			l.delegate.Invalidate(key)
		}
		l.mu.Unlock()
	}

	return e.Bytes, e.Error
}
