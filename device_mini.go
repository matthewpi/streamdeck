//
// Copyright (c) 2022 Matthew Penner
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

type Mini struct{}

var _ DeviceProvider = (*Mini)(nil)

func (*Mini) Name() string {
	return "Stream Deck Mini"
}

func (*Mini) VendorID() uint16 {
	return 0x0fd9
}

func (*Mini) ProductID() uint16 {
	return 0x63
}

func (*Mini) Rows() int {
	return 2
}

func (*Mini) Cols() int {
	return 3
}

func (device *Mini) ButtonCount() int {
	return device.Rows() * device.Cols()
}

func (*Mini) ReadOffset() int {
	return 1
}

func (*Mini) ImageFormat() ImageFormat {
	return BMP
}

func (device *Mini) ImagePayloadSize() int {
	return 1024
}

func (*Mini) ImageSize() image.Point {
	return image.Point{X: 80, Y: 80}
}

func (device *Mini) BrightnessPacket() []byte {
	b := make([]byte, 5)
	b[0] = 0x05
	b[1] = 0x55
	b[2] = 0xaa
	b[3] = 0xd1
	b[4] = 0x01
	return b
}

func (device *Mini) ResetPacket() []byte {
	b := make([]byte, 16)
	b[0] = 0x0b
	b[1] = 0x63
	return b
}

func (device *Mini) GetImageHeader(bytesRemaining, btnIndex, pageNumber int) []byte {
	thisLength := bytesRemaining
	if device.ImagePayloadSize() < bytesRemaining {
		thisLength = device.ImagePayloadSize()
	}

	var element byte
	if thisLength == bytesRemaining {
		element = '\x01'
	} else {
		element = '\x00'
	}

	header := []byte{
		'\x02',
		'\x01',
		byte(pageNumber),
		0,
		element,
		byte(btnIndex + 1),
		'\x00',
		'\x00',
		'\x00',
		'\x00',
		'\x00',
		'\x00',
		'\x00',
		'\x00',
		'\x00',
		'\x00',
	}

	return header
}

func (device *Mini) GIFT() *gift.GIFT {
	return gift.New(
		gift.Resize(
			device.ImageSize().X,
			device.ImageSize().Y,
			gift.LanczosResampling,
		),
		gift.Rotate90(),
		gift.FlipVertical(),
	)
}
