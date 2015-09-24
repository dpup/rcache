// Copyright 2015 Daniel Pupius

package rcache

var NoDeps = []CacheKey{}

// FetchFn is a function type used for populating the cache for a given key.
type FetchFn func(CacheKey) ([]byte, error)

// Cache keys must satisfy the CacheKey interface.
type CacheKey interface {
	Dependencies() []CacheKey
}

type StrKey string

func (str StrKey) Dependencies() []CacheKey {
	return NoDeps
}
