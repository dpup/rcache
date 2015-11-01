// Copyright 2015 Daniel Pupius

package rcache

import (
	"strconv"
	"testing"
)

var runs = 0

func BenchmarkCacheWithMisses(b *testing.B) {
	runs++
	c := New("bench" + strconv.Itoa(runs))
	c.RegisterFetcher(func(key string) ([]byte, error) {
		return []byte(key), nil
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(strconv.Itoa(i))
	}
}

func BenchmarkCacheWithHits(b *testing.B) {
	runs++
	c := New("bench" + strconv.Itoa(runs))
	c.RegisterFetcher(func(key string) ([]byte, error) {
		return []byte(key), nil
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get("1")
	}
}

func BenchmarkNormalMapWithMisses(b *testing.B) {
	m := make(map[string][]byte)
	for i := 0; i < b.N; i++ {
		name := strconv.Itoa(i)
		m[name] = []byte(name)
	}
}
