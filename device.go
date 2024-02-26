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
	"strings"

	"github.com/matthewpi/streamdeck/internal/hid"
)

const (
	// BrightnessMin is the lowest brightness that can be set on a StreamDeck.
	BrightnessMin uint8 = 0
	// BrightnessFull is the highest brightness that can be set on a StreamDeck.
	BrightnessFull uint8 = 100
)

// Device represents a Stream Deck Device.
type Device struct {
	DeviceType

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
		for _, dt := range deviceTypes {
			// Check if the VendorID and ProductID match.
			if d.Info().VendorID != elgatoVendorID || d.Info().ProductID != dt.ProductID {
				continue
			}

			// Get a blank image to use when a button has no image set.
			blankImage, err := dt.ImageFormat.Blank(dt.ImageSize, dt.ImageSize)
			if err != nil {
				return nil, err
			}

			// Open a connection to the HID device.
			if err := d.Open(ctx); err != nil {
				return nil, err
			}

			return &Device{
				DeviceType: dt,

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
	_, err := d.fd.SendFeatureReport(ctx, d.ResetPacketFunc())
	return err
}

// SetBrightness sets the brightness of all buttons on the Device.
func (d *Device) SetBrightness(ctx context.Context, brightness byte) error {
	_, err := d.fd.SendFeatureReport(ctx, d.BrightnessPacketFunc(brightness))
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

	return d.DeviceType.ImageTextureFunc(ctx, d.fd.Write, byte(btnIndex), rawImage)
}

// buttonPressListener listens for button presses over the USB HID bus.
func (d *Device) buttonPressListener(ctx context.Context, ch chan int) error {
	numberOfButtons := d.ButtonCount()
	readOffset := d.ButtonOffset

	// TODO: figure out what the proper size to use here is.
	// Trying to set it to readOffset+numberOfButtons caused the ioctl syscall
	// to get very ANGERY at us.
	// I've tried 288 (36 * 8), 384, and only 512 seems to work.
	states := make([]byte, 512)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Zero the entire states array.
			for i := range states {
				states[i] = 0x0
			}

			n, err := d.fd.Read(ctx, states, 0)
			if err != nil {
				if strings.Contains(err.Error(), "timed out") {
					continue
				}
				return err
			}
			if n == 0 {
				return nil
			}

			for i := 0; i < numberOfButtons; i++ {
				if states[readOffset+i] != 1 {
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
