//
// Copyright (c) 2023 Matthew Penner
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package streamdeck

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"

	"github.com/disintegration/gift"
	"golang.org/x/image/bmp"
)

// ImageFlags are used to apply translations to an image before displaying it
// on a Stream Deck.
type ImageFlags uint8

const (
	// ImageFlagFlipX flips an image horizontally.
	ImageFlagFlipX = 1 << iota
	// ImageFlagFlipY flips an image vertically.
	ImageFlagFlipY
	// ImageFlagRotate90 rotates an image 90 degrees counter-clockwise.
	ImageFlagRotate90
	// ImageFlagRotate180 rotates an image 180 degrees counter-clockwise.
	ImageFlagRotate180
)

// imageFlagMap maps ImageFlag options into their associated gift filters used
// to process images for a Stream Deck device.
var imageFlagMap = map[ImageFlags]gift.Filter{
	ImageFlagFlipX:     gift.FlipHorizontal(),
	ImageFlagFlipY:     gift.FlipVertical(),
	ImageFlagRotate90:  gift.Rotate90(),
	ImageFlagRotate180: gift.Rotate180(),
}

// Has returns true if a specific image flag is set.
func (f ImageFlags) Has(v ImageFlags) bool {
	return f&v != 0
}

// GIFT returns the GIFT instance created by the flags.
func (f ImageFlags) GIFT(size int) *gift.GIFT {
	filters := []gift.Filter{
		gift.Resize(
			size,
			size,
			gift.LanczosResampling,
		),
	}
	for k, v := range imageFlagMap {
		if !f.Has(k) {
			continue
		}
		filters = append(filters, v)
	}
	return gift.New(filters...)
}

// ImageFormat represents an Image Format used by a Stream Deck Device.
type ImageFormat string

const (
	// BMP is a BMP ImageFormat.
	BMP ImageFormat = "BMP"
	// JPEG is a JPEG ImageFormat.
	JPEG ImageFormat = "JPEG"
)

// Encode encodes an image using a ImageFormat.
func (f ImageFormat) Encode(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	var err error
	switch f {
	case BMP:
		err = bmp.Encode(&b, img)
	case JPEG:
		err = jpeg.Encode(&b, img, &jpeg.Options{Quality: 100})
	}
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Blank creates and encodes a blank image used to represent an empty button
// on a Stream Deck.
func (f ImageFormat) Blank(x, y int) ([]byte, error) {
	// Get a blank image to use when a button has no image set.
	img := image.NewRGBA(image.Rect(0, 0, x, y))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.Black), image.Point{X: 0, Y: 0}, draw.Src)
	return f.Encode(img)
}
