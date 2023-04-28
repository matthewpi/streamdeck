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
	"image"

	"github.com/disintegration/gift"
)

type XL struct{}

var _ DeviceProvider = (*XL)(nil)

func (*XL) Name() string {
	return "Stream Deck XL"
}

func (*XL) VendorID() uint16 {
	return 0x0fd9
}

func (*XL) ProductID() uint16 {
	return 0x6c
}

func (*XL) Rows() int {
	return 4
}

func (*XL) Cols() int {
	return 8
}

func (device *XL) ButtonCount() int {
	return device.Rows() * device.Cols()
}

func (*XL) ReadOffset() int {
	return 4
}

func (*XL) ImageFormat() ImageFormat {
	return JPEG
}

func (device *XL) ImagePayloadSize() int {
	return 1024
}

func (*XL) ImageSize() image.Point {
	return image.Point{X: 96, Y: 96}
}

func (device *XL) BrightnessPacket() []byte {
	b := make([]byte, 2)
	b[0] = 0x03
	b[1] = 0x08
	return b
}

func (device *XL) ResetPacket() []byte {
	b := make([]byte, 32)
	b[0] = 0x03
	b[1] = 0x02
	return b
}

func (device *XL) GetImageHeader(bytesRemaining, btnIndex, pageNumber int) []byte {
	thisLength := bytesRemaining
	if device.ImagePayloadSize() < bytesRemaining {
		thisLength = device.ImagePayloadSize()
	}

	header := []byte{'\x02', '\x07', byte(btnIndex)}
	if thisLength == bytesRemaining {
		header = append(header, '\x01')
	} else {
		header = append(header, '\x00')
	}

	header = append(header, byte(thisLength&0xff))
	header = append(header, byte(thisLength>>8))

	header = append(header, byte(pageNumber&0xff))
	header = append(header, byte(pageNumber>>8))

	return header
}

func (device *XL) GIFT() *gift.GIFT {
	return gift.New(
		gift.Resize(
			device.ImageSize().X,
			device.ImageSize().Y,
			gift.LanczosResampling,
		),
		gift.Rotate180(),
	)
}
