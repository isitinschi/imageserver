package builder

import (
	"io/ioutil"
	"log"
	"image"
	"os"
	
	"cache"
)

const warehouse = "warehouse/"

func Build(cache *cache.Cache) {
	log.Printf("BUILDER: Started building image cache...")

	log.Printf("BUILDER: Reading images from \"%s\" folder ...", warehouse)
	files, err := ioutil.ReadDir(warehouse)
	if err != nil {
		log.Fatal(err)
	}
	
	log.Printf("BUILDER: Found %v images", len(files))
	log.Printf("BUILDER: Populating image cache...")

	for _, file := range files {
		filename := file.Name();
		cache.Set(filename, decode(filename))
	}
	
	log.Printf("BUILDER: Finished building image cache.")
}

func decode(filename string) *image.Image {
	f, err := os.Open(warehouse + filename)
    if err != nil {
		log.Println("File not found")
    	return nil
    }
    	
	image,_,_ := image.Decode(f)
	
	defer f.Close()
    return &image
}