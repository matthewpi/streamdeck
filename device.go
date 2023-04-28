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
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"time"

	"github.com/disintegration/gift"

	"github.com/matthewpi/streamdeck/internal/hid"
)

const (
	// BrightnessMin is the lowest brightness that can be set on a StreamDeck.
	BrightnessMin uint32 = 0
	// BrightnessFull is the highest brightness that can be set on a StreamDeck.
	BrightnessFull uint32 = 100
)

// deviceProviders holds a slice of all known Stream Deck device types.
var deviceProviders = []DeviceProvider{
	&Mini{},
	&Original{},
	&OriginalMk2{},
	&XL{},
}

// DeviceProvider represents a device provider which provides data about
// different Stream Deck models/versions.
type DeviceProvider interface {
	Name() string
	VendorID() uint16
	ProductID() uint16
	Rows() int
	Cols() int
	ButtonCount() int
	ReadOffset() int
	ImageFormat() ImageFormat
	ImagePayloadSize() int
	ImageSize() image.Point
	BrightnessPacket() []byte
	ResetPacket() []byte
	GetImageHeader(int, int, int) []byte
	GIFT() *gift.GIFT
}

// Device represents a Stream Deck Device.
type Device struct {
	DeviceProvider

	fd         *hid.USB
	blankImage []byte
}

// Open attempts to open a connection to a Stream Deck Device.
func Open(ctx context.Context) (*Device, error) {
	return OpenPath(ctx, hid.USBDevBus)
}

// OpenPath attempts to open a connection to a Stream Deck Device at the given
// path.
func OpenPath(ctx context.Context, path string) (*Device, error) {
	d, err := open(ctx, path)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, nil
	}
	if err := d.Reset(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

// open attempts to open a connection to a Stream Deck Device.
func open(ctx context.Context, path string) (*Device, error) {
	// Get a list of all USB HID devices.
	devices, err := hid.Devices(path)
	if err != nil {
		return nil, err
	}

	// Iterate over all the devices we found.
	for _, d := range devices {
		// Iterate over all the device types we have and see if we can find a
		// match with a supported device.
		for _, p := range deviceProviders {
			// Check if the VendorID and ProductID match.
			if d.Info().VendorID != p.VendorID() || d.Info().ProductID != p.ProductID() {
				continue
			}

			// Get a blank image to use when a button has no image set.
			img := image.NewRGBA(image.Rect(0, 0, p.ImageSize().X, p.ImageSize().Y))
			draw.Draw(img, img.Bounds(), image.NewUniform(color.Black), image.Point{X: 0, Y: 0}, draw.Src)
			blankImage, err := getImageForButton(img, p.ImageFormat())
			if err != nil {
				return nil, err
			}

			// Open a connection to the HID device.
			if err := d.Open(ctx); err != nil {
				return nil, err
			}

			return &Device{
				DeviceProvider: p,

				fd:         d,
				blankImage: blankImage,
			}, nil
		}
	}

	return nil, nil
}

// Close resets the Device and closes the USB HID connection to the Stream Deck.
func (d *Device) Close(ctx context.Context) error {
	if err := d.Reset(ctx); err != nil {
		return err
	}
	if err := d.SetBrightness(ctx, BrightnessFull); err != nil {
		return err
	}
	return d.fd.Close(ctx)
}

// Clear clears all buttons on the Device.
func (d *Device) Clear(ctx context.Context) error {
	for i := 0; i < d.ButtonCount(); i++ {
		if err := d.SetButton(ctx, i, nil); err != nil {
			return err
		}
	}
	return nil
}

// Reset resets the Device, restoring its initial state displaying the Elgato
// logo.
func (d *Device) Reset(ctx context.Context) error {
	pkt := d.ResetPacket()
	_, err := d.fd.SendFeatureReport(ctx, pkt)
	return err
}

// SetBrightness sets the brightness of all buttons on the Device.
func (d *Device) SetBrightness(ctx context.Context, brightness uint32) error {
	pkt := append(d.BrightnessPacket(), byte(brightness))
	_, err := d.fd.SendFeatureReport(ctx, pkt)
	return err
}

// SetButton sets the image displayed by a specific button on the Device.
func (d *Device) SetButton(ctx context.Context, btnIndex int, rawImage []byte) error {
	if rawImage == nil {
		rawImage = d.blankImage
	}

	if min(max(btnIndex, 0), d.ButtonCount()) != btnIndex {
		return fmt.Errorf("streamdeck: invalid key index: %d", btnIndex)
	}

	var pageNumber int
	bytesRemaining := len(rawImage)
	for bytesRemaining > 0 {
		header := d.GetImageHeader(bytesRemaining, btnIndex, pageNumber)
		imageReportLength := d.ImagePayloadSize()
		imageReportPayloadLength := imageReportLength - len(header)

		var thisLength int
		if imageReportPayloadLength < bytesRemaining {
			thisLength = imageReportPayloadLength
		} else {
			thisLength = bytesRemaining
		}

		bytesSent := pageNumber * imageReportPayloadLength

		payload := append(header, rawImage[bytesSent:(bytesSent+thisLength)]...)
		padding := make([]byte, imageReportLength-len(payload))

		thingToSend := append(payload, padding...)
		if _, err := d.fd.Write(ctx, thingToSend); err != nil {
			return err
		}

		bytesRemaining = bytesRemaining - thisLength
		pageNumber++
	}
	return nil
}

// buttonPressListener listens for button presses over the USB HID bus.
func (d *Device) buttonPressListener(ctx context.Context, ch chan int) error {
	numberOfButtons := d.ButtonCount()
	readOffset := d.ReadOffset()

	var data []byte
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			data = make([]byte, 512)
			if _, err := d.fd.Read(ctx, data, 3*time.Second); err != nil {
				if strings.Contains(err.Error(), "timed out") {
					continue
				}
				return err
			}

			for i := 0; i < numberOfButtons; i++ {
				if data[readOffset+i] != 1 {
					continue
				}
				ch <- i
			}
		}
	}
}

// min is the same as math#Min except that it uses int as the type.
func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

// max is the same as math#Max except that it uses int as the type.
func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}
