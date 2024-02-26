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

// deviceTypes is a list of known Elgato Stream Deck devices.
var deviceTypes = []DeviceType{
	// Stream Deck
	{
		Name:         "Stream Deck",
		ProductID:    0x60,
		Rows:         3,
		Cols:         5,
		ImageFormat:  BMP,
		ImageSize:    72,
		ImageFlags:   ImageFlagFlipX | ImageFlagFlipY,
		ButtonOffset: 1,

		BrightnessPacketFunc: brightnessPacketGen1,
		ResetPacketFunc:      resetPacketGen1,
		ImageTextureFunc:     imageTextureGen1,
	},
	// Stream Deck MK.2
	{
		Name:         "Stream Deck MK.2",
		ProductID:    0x6d,
		Rows:         3,
		Cols:         5,
		ImageFormat:  JPEG,
		ImageSize:    72,
		ImageFlags:   ImageFlagFlipX | ImageFlagFlipY,
		ButtonOffset: 4,

		BrightnessPacketFunc: brightnessPacketGen2,
		ResetPacketFunc:      resetPacketGen2,
		ImageTextureFunc:     imageTextureGen2,
	},
	// Stream Deck Mini
	{
		Name:         "Stream Deck Mini",
		ProductID:    0x63,
		Rows:         2,
		Cols:         3,
		ImageFormat:  BMP,
		ImageSize:    80,
		ImageFlags:   ImageFlagFlipY | ImageFlagRotate90,
		ButtonOffset: 1,

		BrightnessPacketFunc: brightnessPacketGen1,
		ResetPacketFunc:      resetPacketGen1,
		ImageTextureFunc:     imageTextureMini,
	},
	// Stream Deck Mini v2
	{
		Name:         "Stream Deck Mini",
		ProductID:    0x90,
		Rows:         2,
		Cols:         3,
		ImageFormat:  BMP,
		ImageSize:    80,
		ImageFlags:   ImageFlagFlipY | ImageFlagRotate90,
		ButtonOffset: 1,

		BrightnessPacketFunc: brightnessPacketGen1,
		ResetPacketFunc:      resetPacketGen1,
		ImageTextureFunc:     imageTextureMini,
	},
	// Stream Deck XL
	{
		Name:         "Stream Deck XL",
		ProductID:    0x6c,
		Rows:         4,
		Cols:         8,
		ImageFormat:  JPEG,
		ImageSize:    96,
		ImageFlags:   ImageFlagFlipX | ImageFlagFlipY,
		ButtonOffset: 4,

		BrightnessPacketFunc: brightnessPacketGen2,
		ResetPacketFunc:      resetPacketGen2,
		ImageTextureFunc:     imageTextureGen2,
	},
	// Stream Deck XL v2 (same as the XL but different product id)
	{
		Name:         "Stream Deck XL",
		ProductID:    0x8f,
		Rows:         4,
		Cols:         8,
		ImageFormat:  JPEG,
		ImageSize:    96,
		ImageFlags:   ImageFlagFlipX | ImageFlagFlipY,
		ButtonOffset: 4,

		BrightnessPacketFunc: brightnessPacketGen2,
		ResetPacketFunc:      resetPacketGen2,
		ImageTextureFunc:     imageTextureGen2,
	},
	// Stream Deck Plus
	// TODO: this Stream Deck needs a more advanced read handler to handle
	// inputs from the touchscreen and dials.
	{
		Name:         "Stream Deck Plus",
		ProductID:    0x84,
		Rows:         4,
		Cols:         2,
		ImageFormat:  JPEG,
		ImageSize:    120,
		ButtonOffset: 4,

		BrightnessPacketFunc: brightnessPacketGen2,
		ResetPacketFunc:      resetPacketGen2,
		ImageTextureFunc:     imageTextureGen2,
	},
}
