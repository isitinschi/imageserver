package cache

import (
	"image"
)

type Value image.Image

var cache = make(map[string]Value)

func Get(key string) image.Image {
	return cache[key]
}

func Set(key string, image image.Image) {
	cache[key] = image
}