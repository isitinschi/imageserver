package main

import (
	"fmt"
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

var queryCount int = 0
var failedQueryCount int = 0
var startTime time.Time = time.Now()
var lastModified string = time.Now().Format(http.TimeFormat)
var expires string = time.Now().AddDate(0, 0, 30).Format(http.TimeFormat)

func statusHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Status:\n")
	fmt.Fprintf(w, "Query count: %v\n", queryCount)
	fmt.Fprintf(w, "Failed query count: %v\n", failedQueryCount)
	fmt.Fprintf(w, "Start time: %v\n", startTime)
	fmt.Fprintf(w, "Running time: %v\n", time.Since(startTime))
	
	
	debug.FreeOSMemory()
}

var imgCache *cache.Cache

func imageHandler(w http.ResponseWriter, r *http.Request, filename string) {
	defer timeTrack(time.Now(), filename)
	
	image := getImageByName(filename)
	if (image == nil) {
		failedQueryCount += 1
		http.NotFound(w, r)			
        return
	}
	
	width,_ := strconv.Atoi(r.URL.Query().Get("w"))
	height,_ := strconv.Atoi(r.URL.Query().Get("h"))
	
	if width != 0 || height != 0 {
		image = resizer.Resize(uint (width), uint (height), image);
	}	
	
	writeImage(w, image);
	
	debug.FreeOSMemory()
	queryCount += 1
}

func getImageByName(filename string) *image.Image {
	image := imgCache.Get(filename)
	if (image == nil) {
		image = reader.Decode(filename)
		if (image == nil) {			
			return nil
		}
		imgCache.Set(filename, image)
	}
	
	return image
}

// writeImage encodes an image 'img' in jpeg format and writes it into ResponseWriter.
func writeImage(w http.ResponseWriter, img *image.Image) {
    buffer := new(bytes.Buffer)
    if err := jpeg.Encode(buffer, *img, nil); err != nil {
        log.Println("unable to encode image.")
		failedQueryCount += 1
    }

    w.Header().Set("Content-Type", "image/jpeg")
    w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))
	w.Header().Set("Cache-Control", "max-age:604800, public")
	w.Header().Set("Last-Modified", lastModified)
	w.Header().Set("Expires", expires)
    if _, err := w.Write(buffer.Bytes()); err != nil {
        log.Println("unable to write image.")
		failedQueryCount += 1
    }
}

var validPath = regexp.MustCompile("^/(.*)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
        if m == nil {
            failedQueryCount += 1
			http.NotFound(w, r)			
            return
        }
        fn(w, r, m[1])
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
	http.HandleFunc("/", makeHandler(imageHandler))
	http.HandleFunc("/status/", statusHandler)
	http.Handle("/favicon.ico", http.FileServer(http.Dir("./warehouse")))
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