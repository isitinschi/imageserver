package reader

import (
	"image"
	"os"
	"log"
)

const Warehouse = "warehouse/"

func Decode(filename string) *image.Image {
	f, err := os.Open(Warehouse + filename)
    if err != nil {
		log.Println("File not found")
    	return nil
    }
    	
	image,_,_ := image.Decode(f)
		
	defer f.Close()
    return &image
}