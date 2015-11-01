// Copyright 2015 Daniel Pupius

package rcache_test

import (
	"fmt"
	"github.com/dpup/rcache"
)

type CarKey struct {
	Manufacturer string
	Model        string
}

type ThumbnailKey struct {
	Manufacturer string
	Model        string
	Size         int
}

func (key ThumbnailKey) Dependencies() []interface{} {
	return []interface{}{CarKey{key.Manufacturer, key.Model}}
}

// The following example (pretends to) request images for a car, then has a
// dependent cache for thumbnails. If the original image is invalidated, so are
// the thumbnails.
func ExampleCache() {
	c := rcache.New("mycache")
	c.RegisterFetcher(func(key CarKey) ([]byte, error) {
		return getCarImage(key.Manufacturer, key.Model), nil
	})
	c.RegisterFetcher(func(key ThumbnailKey) ([]byte, error) {
		fullImage, _ := c.Get(CarKey{key.Manufacturer, key.Model})
		return resize(fullImage, key.Size), nil
	})

	// Original image is only fetched once.
	tx, _ := c.Get(CarKey{"BMW", "M5"})
	t200, _ := c.Get(ThumbnailKey{"BMW", "M5", 200})
	t400, _ := c.Get(ThumbnailKey{"BMW", "M5", 400})

	fmt.Printf("%s + %s + %s", tx, t200, t400)

	// Output: image:BMW.M5 + image:BMW.M5@200 + image:BMW.M5@400
}

func getCarImage(manufacturer, model string) []byte {
	// In the real world this would go off and fetch an actual image.
	return []byte(fmt.Sprintf("image:%s.%s", manufacturer, model))
}

func resize(image []byte, size int) []byte {
	// In the real world this would resize the image.
	return []byte(fmt.Sprintf("%s@%d", image, size))
}
