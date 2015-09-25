// Copyright 2015 Daniel Pupius

package rcache

var NoDeps = []CacheKey{}

// Cache keys must satisfy the CacheKey interface.
type CacheKey interface {
	Dependencies() []CacheKey
}

// StrKey allows strings to be easily used as cache keys with no dependencies.
type StrKey string

func (str StrKey) Dependencies() []CacheKey {
	return NoDeps
}
