// Copyright 2015 Daniel Pupius

package rcache

import (
	"fmt"
	"strings"
	"testing"
)

type CarKey struct {
	Manufacturer string
	Model        string
}

func (key CarKey) Dependencies() []CacheKey {
	return NoDeps
}

func (key CarKey) String() string {
	return key.Manufacturer + " " + key.Model
}

type RepeatedKey struct {
	Manufacturer string
	Model        string
	Times        int
}

func (key RepeatedKey) Dependencies() []CacheKey {
	return []CacheKey{CarKey{key.Manufacturer, key.Model}}
}

func (key RepeatedKey) String() string {
	return fmt.Sprintf("%s %s x %d", key.Manufacturer, key.Model, key.Times)
}

func TestGetInvalidate(t *testing.T) {
	c := New("test1")
	i := 0
	c.RegisterFetcher(func(key CarKey) ([]byte, error) {
		i++
		return []byte(key.String() + " xxxx"), nil
	})

	rv1, _ := c.Get(CarKey{"BMW", "M5"})
	rv2, _ := c.Get(CarKey{"VW", "Bug"})

	if string(rv1) != "BMW M5 xxxx" {
		t.Errorf("rv1 was %s", rv1)
	}
	if string(rv2) != "VW Bug xxxx" {
		t.Errorf("rv2 was %s", rv2)
	}

	c.Get(CarKey{"BMW", "M5"})
	c.Get(CarKey{"VW", "Bug"})

	if i != 2 {
		t.Errorf("Expected fetcher to be called twice, was called %d times", i)
	}

	c.Invalidate(CarKey{"BMW", "M5"})

	c.Get(CarKey{"BMW", "M5"})
	c.Get(CarKey{"VW", "Bug"})

	if i != 3 {
		t.Errorf("Expected fetcher to be called twice, was called %d times", i)
	}
}

func TestDependentGet(t *testing.T) {
	c := New("test2")
	oi := 0
	di := 0
	c.RegisterFetcher(func(key CarKey) ([]byte, error) {
		oi++
		return []byte(key.String() + " "), nil
	})
	c.RegisterFetcher(func(key RepeatedKey) ([]byte, error) {
		di++
		o, _ := c.Get(CarKey{key.Manufacturer, key.Model})
		return []byte(strings.Repeat(string(o), key.Times)), nil
	})
	rv1, _ := c.Get(RepeatedKey{"BMW", "M5", 2})
	rv2, _ := c.Get(RepeatedKey{"BMW", "M5", 4})

	if string(rv1) != "BMW M5 BMW M5 " {
		t.Errorf("rv1 was %s", rv1)
	}

	if string(rv2) != "BMW M5 BMW M5 BMW M5 BMW M5 " {
		t.Errorf("rv2 was %s", rv2)
	}

	if oi != 1 {
		t.Errorf("Expected CarKey fetcher to be called once, was called %d times", oi)
	}
	if di != 2 {
		t.Errorf("Expected derived fetcher to be called twice, was called %d times", di)
	}

	// Invalidating 'CarKey' should also invalidate entry for 'derived'.
	c.Invalidate(CarKey{"BMW", "M5"})
	c.Get(RepeatedKey{"BMW", "M5", 2})

	if oi != 2 {
		t.Errorf("Expected CarKey fetcher to be called twice, was called %d times", oi)
	}
	if di != 3 {
		t.Errorf("Expected derived fetcher to be called thrice, was called %d times", di)
	}
}

func TestEntries(t *testing.T) {
	c := New("test3")
	c.RegisterFetcher(func(key StrKey) ([]byte, error) {
		return []byte(string(key)), nil
	})
	c.Get(StrKey("1"))
	c.Get(StrKey("2"))
	c.Get(StrKey("3"))
	c.Get(StrKey("4"))
	entries := c.Entries()
	if len(entries) != 4 {
		t.Errorf("Expected 4 entries, was  %d", len(entries))
	}
}

func TestBadFetcher_noArgs(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test4")
	c.RegisterFetcher(func() ([]byte, error) { return []byte{}, nil })
}

func TestBadFetcher_2args(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test4")
	c.RegisterFetcher(func(a, b CarKey) ([]byte, error) { return []byte{}, nil })
}

func TestBadFetcher_badReturn1(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test5")
	c.RegisterFetcher(func(a CarKey) (int, error) { return 1, nil })
}

func TestBadFetcher_badReturn2(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test6")
	c.RegisterFetcher(func(a CarKey) ([]byte, int) { return []byte{}, 2 })
}

func TestBadFetcher_badReturn3(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test7")
	c.RegisterFetcher(func(a CarKey) {})
}

func TestBadFetcher_badReturn4(t *testing.T) {
	defer func() {
		if e := recover(); e == nil {
			t.Error("There was no panic")
		}
	}()
	c := New("test8")
	c.RegisterFetcher(func(a CarKey) []byte { return []byte{} })
}
