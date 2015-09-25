// Copyright 2015 Daniel Pupius

package rcache

import (
	"strings"
	"testing"
)

type FixedSizeEntry struct {
	size int
}

func (key FixedSizeEntry) Dependencies() []CacheKey {
	return NoDeps
}

func TestLRUBehavior(t *testing.T) {
	lru := NewLRU("lru1", 9)
	lru.RegisterFetcher(func(key FixedSizeEntry) ([]byte, error) {
		return []byte(strings.Repeat(".", key.size)), nil
	})

	a := FixedSizeEntry{1}
	b := FixedSizeEntry{2}
	c := FixedSizeEntry{3}
	d := FixedSizeEntry{4}
	e := FixedSizeEntry{5}

	lru.Get(a)
	lru.Get(b)
	lru.Get(c)

	if !lru.Peek(a) {
		t.Error("Expected entry (a) to be cached")
	}
	if !lru.Peek(b) {
		t.Error("Expected entry (b) to be cached")
	}
	if !lru.Peek(c) {
		t.Error("Expected entry (c) to be cached")
	}

	lru.Get(a) // Moves to least recently used.
	lru.Get(d) // Pushes (b) out, as size would be 10.

	if !lru.Peek(a) {
		t.Error("Expected entry (a) to be cached")
	}
	if lru.Peek(b) {
		t.Error("Entry (b) should have been evicted")
	}
	if !lru.Peek(c) {
		t.Error("Expected entry (c) to be cached")
	}
	if !lru.Peek(d) {
		t.Error("Expected entry (d) to be cached")
	}

	lru.Get(e) // Only enough space for (d) and (e) now

	if lru.Peek(a) {
		t.Error("Entry (a) should have been evicted")
	}
	if lru.Peek(b) {
		t.Error("Entry (b) should have been evicted")
	}
	if lru.Peek(c) {
		t.Error("Entry (c) should have been evicted")
	}
	if !lru.Peek(d) {
		t.Error("Expected entry (d) to be cached")
	}
	if !lru.Peek(e) {
		t.Error("Expected entry (d) to be cached")
	}

}
