package main

import (
    "net/http"
	"regexp"
	"os"
	"image"
	"strconv"
	"log"
	"bytes"
	"time"
	"runtime"
	"runtime/debug"
	
	"image/resizer"
	"cache"
	"warehouse/reader"
	
	"image/jpeg"
)

var imgCache *cache.Cache

func imageHandler(w http.ResponseWriter, r *http.Request, filename string) {
	defer timeTrack(time.Now(), filename)
	
	image := getImageByName(filename)
	
	width,_ := strconv.Atoi(r.URL.Query().Get("w"))
	height,_ := strconv.Atoi(r.URL.Query().Get("h"))
	
	if width != 0 || height != 0 {
		image = resizer.Resize(uint (width), uint (height), image);
	}	
	
	writeImage(w, image);
	
	debug.FreeOSMemory()
}

func getImageByName(filename string) *image.Image {
	image := imgCache.Get(filename)
	if (image == nil) {
		image = reader.Decode(filename)
		imgCache.Set(filename, image)
	}
	
	return image
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

func initialize() {
	log.Printf("IMAGESERVER INITIALIZATION")
		
	cpus := runtime.NumCPU()
	runtime.GOMAXPROCS(cpus)
	log.Printf("IMAGESERVER: Setting GOMAXPROCS=%v", cpus)
	
	imgCache = cache.New()
	
	log.Printf("IMAGESERVER INITIALIZATION FINISHED")
}

func startServer() {
	argsWithProg := os.Args
	
	port := "8080"
	if len(argsWithProg) > 1 {
		port = argsWithProg[1]
	}
	
	defer listenAndServe(port);
	log.Printf("IMAGESERVER STARTED ON PORT %s", port)
}

func listenAndServe(port string) {
	http.HandleFunc("/image/", makeHandler(imageHandler))
    http.ListenAndServe(":" + port, nil)
}

func timeTrack(start time.Time, filename string) {
    elapsed := time.Since(start)
    log.Printf("INFO: request for %s took %s", filename, elapsed)
}

func main() {
	initialize();
	startServer();
}