package cache_test

import (
	"log"
	"io/ioutil"

	"cache"
	"warehouse/reader"
	"testing"
)

func TestCachePopulating(t *testing.T) {
	cache := cache.New()

	log.Printf("BUILDER: Started building image cache...")

	log.Printf("BUILDER: Reading images from \"%s\" folder ...", reader.Warehouse)
	files, err := ioutil.ReadDir("./../../../" + reader.Warehouse)
	if err != nil {
		log.Fatal(err)
	}
	
	log.Printf("BUILDER: Found %v images", len(files))
	log.Printf("BUILDER: Populating image cache...")

	for _, file := range files {
		filename := file.Name()
		populate(cache, filename)
	}
	
	log.Printf("BUILDER: Finished building image cache.")
}

func populate(cache *cache.Cache, filename string) {
	cache.Set(filename, reader.Decode(filename))
}

