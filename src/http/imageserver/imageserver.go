package main

import (
    "net/http"
	"regexp"
	"os"
	"bufio"
	"image"
	"strconv"
	"log"
	"bytes"
	
	"image/resizer"
	"cache"
	
	"image/jpeg"
)

const warehouse = "warehouse/"
var imgCache *cache.Cache

func imageHandler(w http.ResponseWriter, r *http.Request, filename string) {
	image := getImageByName(filename);
	
	width,_ := strconv.Atoi(r.URL.Query().Get("w"))
	height,_ := strconv.Atoi(r.URL.Query().Get("h"))
	
	if width != 0 || height != 0 {
		image = resizer.Resize(uint (width), uint (height), image);
	}	
	
	writeImage(w, image);
}

func getImageByName(filename string) *image.Image {
	var image = imgCache.Get(filename)
	if image == nil {
		image,_,_ := decode(filename)
		imgCache.Set(filename, &image)
	}
	
	return image
}

func decode(filename string) (image.Image, string, error) {
	f, err := os.Open(warehouse + filename)
    if err != nil {
		log.Println("File not found")
    	return nil, "", err
    }
    defer f.Close()
    return image.Decode(bufio.NewReader(f))
}

// writeImage encodes an image 'img' in jpeg format and writes it into ResponseWriter.
func writeImage(w http.ResponseWriter, img *image.Image) {
    buffer := new(bytes.Buffer)
    if err := jpeg.Encode(buffer, *img, nil); err != nil {
        log.Println("unable to encode image.")
    }

    w.Header().Set("Content-Type", "image/jpeg")
    w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))
    if _, err := w.Write(buffer.Bytes()); err != nil {
        log.Println("unable to write image.")
    }
}

var validPath = regexp.MustCompile("^/(image)/(.*)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
        if m == nil {
            http.NotFound(w, r)
            return
        }
        fn(w, r, m[2])
    }
}

func main() {
	imgCache = cache.New()

	argsWithProg := os.Args
	
	if len(argsWithProg) > 1 {
		listenAndServe(argsWithProg[1])
	}
	
	listenAndServe("8080");
}

func listenAndServe(port string) {
	http.HandleFunc("/image/", makeHandler(imageHandler))
    http.ListenAndServe(":" + port, nil)
}