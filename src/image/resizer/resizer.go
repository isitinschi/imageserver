package resizer

import (
	"image"
	"runtime"
	"sync"
	"math"
	"log"
)

// values <1 will sharpen the image
var blur = 1.0

// Resize scales an image to new width and height using the interpolation function interp.
// A new image with the given dimensions will be returned.
// If one of the parameters width or height is set to 0, its size will be calculated so that
// the aspect ratio is that of the originating image.
// The resizing algorithm uses channels for parallel computation.
func Resize(width, height uint, img *image.Image) *image.Image {
	bounds := (*img).Bounds()
	scaleX, scaleY := calcFactors(width, height, float64(bounds.Dx()), float64(bounds.Dy()))
	if width == 0 {
		width = uint(0.7 + float64(bounds.Dx())/scaleX)
	}
	if height == 0 {
		height = uint(0.7 + float64(bounds.Dy())/scaleY)
	}

	// Trivial case: return input image
	if int(width) == bounds.Dx() && int(height) == bounds.Dy() {
		return img
	}

	taps, kernel := 6, lanczos3
	cpus := runtime.GOMAXPROCS(runtime.NumCPU())
	wg := sync.WaitGroup{}

	// Generic access to image.Image is slow in tight loops.
	// The optimal access has to be determined from the concrete image type.
	switch input := (*img).(type) {
		case *image.YCbCr:
			// 8-bit precision
			// accessing the YCbCr arrays in a tight loop is slow.
			// converting the image to ycc increases performance by 2x.
			temp := newYCC(image.Rect(0, 0, input.Bounds().Dy(), int(width)), input.SubsampleRatio)
			result := newYCC(image.Rect(0, 0, int(width), int(height)), image.YCbCrSubsampleRatio444)

			coeffs, offset, filterLength := createWeights8(temp.Bounds().Dy(), taps, blur, scaleX, kernel)
			in := imageYCbCrToYCC(input)
			wg.Add(cpus)
			for i := 0; i < cpus; i++ {
				slice := makeSlice(temp, i, cpus).(*ycc)
				go func() {
					defer wg.Done()
					resizeYCbCr(in, slice, scaleX, coeffs, offset, filterLength)
				}()
			}
			wg.Wait()

			coeffs, offset, filterLength = createWeights8(result.Bounds().Dy(), taps, blur, scaleY, kernel)
			wg.Add(cpus)
			for i := 0; i < cpus; i++ {
				slice := makeSlice(result, i, cpus).(*ycc)
				go func() {
					defer wg.Done()
					resizeYCbCr(temp, slice, scaleY, coeffs, offset, filterLength)
				}()
			}
			wg.Wait()
			ycbcr := image.Image(result.YCbCr())
			img = &ycbcr
		default:
			log.Printf("Unknown image type %T", input)
			
			// 16-bit precision
			temp := image.NewRGBA64(image.Rect(0, 0, bounds.Dy(), int(width)))
			result := image.NewRGBA64(image.Rect(0, 0, int(width), int(height)))

			// horizontal filter, results in transposed temporary image
			coeffs, offset, filterLength := createWeights16(temp.Bounds().Dy(), taps, blur, scaleX, kernel)
			wg.Add(cpus)
			for i := 0; i < cpus; i++ {
				slice := makeSlice(temp, i, cpus).(*image.RGBA64)
				go func() {
					defer wg.Done()
					resizeGeneric(img, slice, scaleX, coeffs, offset, filterLength)
				}()
			}
			wg.Wait()

			// horizontal filter on transposed image, result is not transposed
			coeffs, offset, filterLength = createWeights16(result.Bounds().Dy(), taps, blur, scaleY, kernel)
			wg.Add(cpus)
			for i := 0; i < cpus; i++ {
				slice := makeSlice(result, i, cpus).(*image.RGBA64)
				go func() {
					defer wg.Done()
					resizeRGBA64(temp, slice, scaleY, coeffs, offset, filterLength)
				}()
			}
			wg.Wait()
			
			rgba64 := image.Image(result)
			img = &rgba64
	}
	
	return img
}

// Calculates scaling factors using old and new image dimensions.
func calcFactors(width, height uint, oldWidth, oldHeight float64) (scaleX, scaleY float64) {
	if width == 0 {
		if height == 0 {
			scaleX = 1.0
			scaleY = 1.0
		} else {
			scaleY = oldHeight / float64(height)
			scaleX = scaleY
		}
	} else {
		scaleX = oldWidth / float64(width)
		if height == 0 {
			scaleY = scaleX
		} else {
			scaleY = oldHeight / float64(height)
		}
	}
	return
}

type imageWithSubImage interface {
	image.Image
	SubImage(image.Rectangle) image.Image
}

func makeSlice(img imageWithSubImage, i, n int) image.Image {
	return img.SubImage(image.Rect(img.Bounds().Min.X, img.Bounds().Min.Y+i*img.Bounds().Dy()/n, img.Bounds().Max.X, img.Bounds().Min.Y+(i+1)*img.Bounds().Dy()/n))
}

func sinc(x float64) float64 {
	x = math.Abs(x) * math.Pi
	if x >= 1.220703e-4 {
		return math.Sin(x) / x
	}
	return 1
}

func lanczos3(in float64) float64 {
	if in > -3 && in < 3 {
		return sinc(in) * sinc(in*0.3333333333333333)
	}
	return 0
}

// range [-256,256]
func createWeights8(dy, filterLength int, blur, scale float64, kernel func(float64) float64) ([]int16, []int, int) {
	filterLength = filterLength * int(math.Max(math.Ceil(blur*scale), 1))
	filterFactor := math.Min(1./(blur*scale), 1)

	coeffs := make([]int16, dy*filterLength)
	start := make([]int, dy)
	for y := 0; y < dy; y++ {
		interpX := scale*(float64(y)+0.5) - 0.5
		start[y] = int(interpX) - filterLength/2 + 1
		interpX -= float64(start[y])
		for i := 0; i < filterLength; i++ {
			in := (interpX - float64(i)) * filterFactor
			coeffs[y*filterLength+i] = int16(kernel(in) * 256)
		}
	}

	return coeffs, start, filterLength
}

// range [-65536,65536]
func createWeights16(dy, filterLength int, blur, scale float64, kernel func(float64) float64) ([]int32, []int, int) {
	filterLength = filterLength * int(math.Max(math.Ceil(blur*scale), 1))
	filterFactor := math.Min(1./(blur*scale), 1)

	coeffs := make([]int32, dy*filterLength)
	start := make([]int, dy)
	for y := 0; y < dy; y++ {
		interpX := scale*(float64(y)+0.5) - 0.5
		start[y] = int(interpX) - filterLength/2 + 1
		interpX -= float64(start[y])
		for i := 0; i < filterLength; i++ {
			in := (interpX - float64(i)) * filterFactor
			coeffs[y*filterLength+i] = int32(kernel(in) * 65536)
		}
	}

	return coeffs, start, filterLength
}

func createWeightsNearest(dy, filterLength int, blur, scale float64) ([]bool, []int, int) {
	filterLength = filterLength * int(math.Max(math.Ceil(blur*scale), 1))
	filterFactor := math.Min(1./(blur*scale), 1)

	coeffs := make([]bool, dy*filterLength)
	start := make([]int, dy)
	for y := 0; y < dy; y++ {
		interpX := scale*(float64(y)+0.5) - 0.5
		start[y] = int(interpX) - filterLength/2 + 1
		interpX -= float64(start[y])
		for i := 0; i < filterLength; i++ {
			in := (interpX - float64(i)) * filterFactor
			if in >= -0.5 && in < 0.5 {
				coeffs[y*filterLength+i] = true
			} else {
				coeffs[y*filterLength+i] = false
			}
		}
	}

	return coeffs, start, filterLength
}

// Keep value in [0,255] range.
func clampUint8(in int32) uint8 {
	// casting a negative int to an uint will result in an overflown
	// large uint. this behavior will be exploited here and in other functions
	// to achieve a higher performance.
	if uint32(in) < 256 {
		return uint8(in)
	}
	if in > 255 {
		return 255
	}
	return 0
}

func resizeYCbCr(in *ycc, out *ycc, scale float64, coeffs []int16, offset []int, filterLength int) {
	newBounds := out.Bounds()
	maxX := in.Bounds().Dx() - 1

	for x := newBounds.Min.X; x < newBounds.Max.X; x++ {
		row := in.Pix[x*in.Stride:]
		for y := newBounds.Min.Y; y < newBounds.Max.Y; y++ {
			var p [3]int32
			var sum int32
			start := offset[y]
			ci := y * filterLength
			for i := 0; i < filterLength; i++ {
				coeff := coeffs[ci+i]
				if coeff != 0 {
					xi := start + i
					switch {
					case uint(xi) < uint(maxX):
						xi *= 3
					case xi >= maxX:
						xi = 3 * maxX
					default:
						xi = 0
					}
					p[0] += int32(coeff) * int32(row[xi+0])
					p[1] += int32(coeff) * int32(row[xi+1])
					p[2] += int32(coeff) * int32(row[xi+2])
					sum += int32(coeff)
				}
			}

			xo := (y-newBounds.Min.Y)*out.Stride + (x-newBounds.Min.X)*3
			out.Pix[xo+0] = clampUint8(p[0] / sum)
			out.Pix[xo+1] = clampUint8(p[1] / sum)
			out.Pix[xo+2] = clampUint8(p[2] / sum)
		}
	}
}

// Keep value in [0,65535] range.
func clampUint16(in int64) uint16 {
	if uint64(in) < 65536 {
		return uint16(in)
	}
	if in > 65535 {
		return 65535
	}
	return 0
}

func resizeGeneric(in *image.Image, out *image.RGBA64, scale float64, coeffs []int32, offset []int, filterLength int) {
	newBounds := out.Bounds()
	inBounds := (*in).Bounds()
	maxX := inBounds.Dx() - 1

	for x := newBounds.Min.X; x < newBounds.Max.X; x++ {
		for y := newBounds.Min.Y; y < newBounds.Max.Y; y++ {
			var rgba [4]int64
			var sum int64
			start := offset[y]
			ci := y * filterLength
			for i := 0; i < filterLength; i++ {
				coeff := coeffs[ci+i]
				if coeff != 0 {
					xi := start + i
					switch {
					case xi < 0:
						xi = 0
					case xi >= maxX:
						xi = maxX
					}

					r, g, b, a := (*in).At(xi+inBounds.Min.X, x+inBounds.Min.Y).RGBA()

					rgba[0] += int64(coeff) * int64(r)
					rgba[1] += int64(coeff) * int64(g)
					rgba[2] += int64(coeff) * int64(b)
					rgba[3] += int64(coeff) * int64(a)
					sum += int64(coeff)
				}
			}

			offset := (y-newBounds.Min.Y)*out.Stride + (x-newBounds.Min.X)*8

			value := clampUint16(rgba[0] / sum)
			out.Pix[offset+0] = uint8(value >> 8)
			out.Pix[offset+1] = uint8(value)
			value = clampUint16(rgba[1] / sum)
			out.Pix[offset+2] = uint8(value >> 8)
			out.Pix[offset+3] = uint8(value)
			value = clampUint16(rgba[2] / sum)
			out.Pix[offset+4] = uint8(value >> 8)
			out.Pix[offset+5] = uint8(value)
			value = clampUint16(rgba[3] / sum)
			out.Pix[offset+6] = uint8(value >> 8)
			out.Pix[offset+7] = uint8(value)
		}
	}
}

func resizeRGBA64(in *image.RGBA64, out *image.RGBA64, scale float64, coeffs []int32, offset []int, filterLength int) {
	newBounds := out.Bounds()
	maxX := in.Bounds().Dx() - 1

	for x := newBounds.Min.X; x < newBounds.Max.X; x++ {
		row := in.Pix[x*in.Stride:]
		for y := newBounds.Min.Y; y < newBounds.Max.Y; y++ {
			var rgba [4]int64
			var sum int64
			start := offset[y]
			ci := y * filterLength
			for i := 0; i < filterLength; i++ {
				coeff := coeffs[ci+i]
				if coeff != 0 {
					xi := start + i
					switch {
					case uint(xi) < uint(maxX):
						xi *= 8
					case xi >= maxX:
						xi = 8 * maxX
					default:
						xi = 0
					}

					rgba[0] += int64(coeff) * (int64(row[xi+0])<<8 | int64(row[xi+1]))
					rgba[1] += int64(coeff) * (int64(row[xi+2])<<8 | int64(row[xi+3]))
					rgba[2] += int64(coeff) * (int64(row[xi+4])<<8 | int64(row[xi+5]))
					rgba[3] += int64(coeff) * (int64(row[xi+6])<<8 | int64(row[xi+7]))
					sum += int64(coeff)
				}
			}

			xo := (y-newBounds.Min.Y)*out.Stride + (x-newBounds.Min.X)*8

			value := clampUint16(rgba[0] / sum)
			out.Pix[xo+0] = uint8(value >> 8)
			out.Pix[xo+1] = uint8(value)
			value = clampUint16(rgba[1] / sum)
			out.Pix[xo+2] = uint8(value >> 8)
			out.Pix[xo+3] = uint8(value)
			value = clampUint16(rgba[2] / sum)
			out.Pix[xo+4] = uint8(value >> 8)
			out.Pix[xo+5] = uint8(value)
			value = clampUint16(rgba[3] / sum)
			out.Pix[xo+6] = uint8(value >> 8)
			out.Pix[xo+7] = uint8(value)
		}
	}
}