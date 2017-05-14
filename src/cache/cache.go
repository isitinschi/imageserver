package cache

import (
	"cache/lru"
	"image"
)

const numberOfImagesToCache = 50

type Cache struct {
	lru *lru.LRUCache
}

type Value struct {
	image *image.Image
}

func (v *Value) Size() int {
	return 1
}

func New() *Cache {
	return &Cache{
		lru: lru.NewLRUCache(numberOfImagesToCache),
	}
}

func (cache *Cache) Get(key string) *image.Image {
	value, ok := cache.lru.Get(key)
	if !ok {
		return nil
	}

	return value.(*Value).image
}

func (cache *Cache) Set(key string, image *image.Image) {
	value := &Value{image}
	cache.lru.Set(key, value)
}
