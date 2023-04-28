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
	"image/jpeg"
	_ "image/png"

	"golang.org/x/image/bmp"
)

// ImageFormat represents an Image Format used by a Stream Deck Device.
type ImageFormat string

const (
	// BMP is a BMP ImageFormat.
	BMP ImageFormat = "BMP"
	// JPEG is a JPEG ImageFormat.
	JPEG ImageFormat = "JPEG"
)

// getImageForButton encodes an image using a ImageFormat.
func getImageForButton(img image.Image, format ImageFormat) ([]byte, error) {
	var b bytes.Buffer
	var err error
	switch format {
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
