package cache

import (
	"image"
	"sync"
)

type Cache struct {
	table map[string]*image.Image
	lock *sync.RWMutex
}

func New() *Cache {
    return &Cache{table : make(map[string]*image.Image), lock: new(sync.RWMutex)}
}

func (cache *Cache) Get(key string) *image.Image {
	return cache.table[key]
}

func (cache *Cache) Set(key string, image *image.Image) {
	cache.lock.Lock()
    defer cache.lock.Unlock()
	cache.table[key] = image
}