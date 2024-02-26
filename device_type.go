//
// Copyright (c) 2024 Matthew Penner
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
	"image"

	"github.com/disintegration/gift"
)

const (
	// elgatoVendorID is Elgato's Vendor ID for their USB devices.
	elgatoVendorID = 0x0fd9
)

// DeviceType represents a type of Elgato Stream Deck.
type DeviceType struct {
	// Name of the Device Type.
	Name string

	// ProductID of the Device. This will be used to identify the DeviceType
	// alongside Elgato's Vendor ID.
	ProductID uint16

	// Rows of buttons on the Device.
	Rows int

	// Cols of buttons on the Device.
	Cols int

	// ImageFormat used to encode images displayed on the Device.
	ImageFormat ImageFormat

	// ImageSize to use to transform images for the Device.
	ImageSize int

	// ImageFlags to apply to images before displaying them on the Device.
	ImageFlags ImageFlags

	// ButtonOffset is the offset to used to detect what physical button on the
	// device was pressed. This offset value varies by generation, but is
	// usually either `1` or `4`.
	ButtonOffset int

	// BrightnessPacketFunc returns a packet to change the brightness on the
	// Device.
	BrightnessPacketFunc

	// ResetPacketFunc returns a packet to reset the display on the Device.
	ResetPacketFunc

	// ImageTextureFunc sets an image on the Device.
	ImageTextureFunc
}

// ButtonCount returns the total number of buttons on the Device.
func (t DeviceType) ButtonCount() int {
	return t.Rows * t.Cols
}

// GIFT returns the GIFT instance used to transform images for the Device.
func (t DeviceType) GIFT() *gift.GIFT {
	return t.ImageFlags.GIFT(t.ImageSize)
}

// EncodeImage encodes an image to be used with the Stream Deck.
func (t DeviceType) EncodeImage(img image.Image) ([]byte, error) {
	if img == nil {
		return nil, nil
	}

	g := t.GIFT()

	// Resize and rotate the image
	res := image.NewRGBA(g.Bounds(img.Bounds()))
	g.Draw(res, img)
	return t.ImageFormat.Encode(res)
}

// BrightnessPacketFunc is a function that returns a packet used to change the
// brightness of a Device.
type BrightnessPacketFunc func(brightness byte) []byte

func brightnessPacketGen1(brightness byte) []byte {
	b := make([]byte, 17)
	b[0] = 0x05
	b[1] = 0x55
	b[2] = 0xaa
	b[3] = 0xd1
	b[4] = 0x01
	b[5] = brightness
	return b
}

func brightnessPacketGen2(brightness byte) []byte {
	b := make([]byte, 32)
	b[0] = 0x03
	b[1] = 0x08
	b[2] = brightness
	return b
}

// ResetPacketFunc is a function that returns a packet used to reset the Device.
type ResetPacketFunc func() []byte

func resetPacketGen1() []byte {
	b := make([]byte, 16)
	b[0] = 0x0b
	b[1] = 0x63
	return b
}

func resetPacketGen2() []byte {
	b := make([]byte, 32)
	b[0] = 0x03
	b[1] = 0x02
	return b
}

// ImageTextureFunc is a function that displays an image for the specified
// button on a Device.
type ImageTextureFunc func(
	ctx context.Context,
	w func(context.Context, []byte) (int, error),
	button byte,
	buffer []byte,
) error

// imageTextureOldShared is for gen1 and minis which use the same logic with a
// different packageSize.
func imageTextureOldShared(
	ctx context.Context,
	w func(context.Context, []byte) (int, error),
	button byte,
	buffer []byte,
	packageSize int,
) error {
	// headerSize is the size of the header at the beginning of the payload.
	const headerSize = 16
	// payloadSize is the size available for data in the payload after the header.
	payloadSize := packageSize - headerSize

	// Allocate enough memory for the full payload (header + image)
	payload := make([]byte, packageSize)

	// Set the required data for the payload header
	payload[0] = 0x02
	payload[1] = 0x01
	// payload[2] = page
	payload[3] = 0x00
	// payload[4] = 0x01 if last chunk, 0x00 otherwise.
	payload[5] = button + 1
	// Rest of the header is zeroed.
	for i := 6; i < headerSize; i++ {
		payload[i] = 0x00
	}

	// Start at "page" 0 and with the full size of the buffer.
	page := 0
	bytesRemaining := len(buffer)

	// Keep iterating until all the data has been sent.
	for bytesRemaining > 0 {
		payload[2] = byte(page)

		// Get the size of the chunk we will be sending, the maximum size of a
		// chunk is `payloadSize`.
		chunkSize := min(bytesRemaining, payloadSize)
		if chunkSize == bytesRemaining {
			payload[4] = 0x01
		} else {
			payload[4] = 0x00
		}

		// Calculate the amount of data we have already sent to the Stream Deck.
		bytesSent := page * payloadSize

		// Copy the image into the payload after the header.
		copy(payload[headerSize:], buffer[bytesSent:(bytesSent+chunkSize)])

		// Zero the rest of the payload if the chunk doesn't fill all the
		// available space.
		paddingSize := payloadSize - chunkSize
		if paddingSize > 0 {
			for i := packageSize - paddingSize; i < packageSize; i++ {
				payload[i] = 0
			}
		}

		// Write the payload
		if _, err := w(ctx, payload); err != nil {
			return err
		}

		// Update the tracking variables
		bytesRemaining = bytesRemaining - chunkSize
		page++
	}

	return nil
}

func imageTextureMini(
	ctx context.Context,
	w func(context.Context, []byte) (int, error),
	button byte,
	buffer []byte,
) error {
	return imageTextureOldShared(ctx, w, button, buffer, 1024)
}

func imageTextureGen1(
	ctx context.Context,
	w func(context.Context, []byte) (int, error),
	button byte,
	buffer []byte,
) error {
	return imageTextureOldShared(ctx, w, button, buffer, 8191)
}

func imageTextureGen2(
	ctx context.Context,
	w func(context.Context, []byte) (int, error),
	button byte,
	buffer []byte,
) error {
	const (
		// packageSize is the full size of the payload sent to the Stream Deck.
		packageSize = 1024
		// headerSize is the size of the header at the beginning of the payload.
		headerSize = 8
		// payloadSize is the size available for data in the payload after the header.
		payloadSize = packageSize - headerSize
	)

	// Allocate enough memory for the full payload (header + image)
	payload := make([]byte, packageSize)

	// Set the required data for the payload header
	payload[0] = 0x02
	payload[1] = 0x07
	payload[2] = button

	// Start at "page" 0 and with the full size of the buffer.
	page := 0
	bytesRemaining := len(buffer)

	// Keep iterating until all the data has been sent.
	for bytesRemaining > 0 {
		// Get the size of the chunk we will be sending, the maximum size of a
		// chunk is `payloadSize`.
		chunkSize := min(bytesRemaining, payloadSize)
		if chunkSize == bytesRemaining {
			payload[3] = 0x01
		} else {
			payload[3] = 0x00
		}
		payload[4] = byte(chunkSize & 0xff)
		payload[5] = byte(chunkSize >> 8)
		payload[6] = byte(page & 0xff)
		payload[7] = byte(page >> 8)

		// Calculate the amount of data we have already sent to the Stream Deck.
		bytesSent := page * payloadSize

		// Copy the image into the payload after the header.
		copy(payload[headerSize:], buffer[bytesSent:(bytesSent+chunkSize)])

		// Zero the rest of the payload if the chunk doesn't fill all the
		// available space.
		paddingSize := payloadSize - chunkSize
		if paddingSize > 0 {
			for i := packageSize - paddingSize; i < packageSize; i++ {
				payload[i] = 0
			}
		}

		// Write the payload
		if _, err := w(ctx, payload); err != nil {
			return err
		}

		// Update the tracking variables
		bytesRemaining = bytesRemaining - chunkSize
		page++
	}

	return nil
}
