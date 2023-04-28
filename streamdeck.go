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
	"image"
	"sync"
)

// StreamDeck represents an Elgato Stream Deck.
type StreamDeck struct {
	// device is a wrapper of the underlying USB HID Device.
	device *Device
	// brightness is the StreamDeck's current brightness.
	brightness int

	// cancel is used to cancel the button press and callback goroutines.
	cancel context.CancelFunc
	// ch is the internal channel used to receive button press events.
	ch chan int

	// pressHandlerMx is a mutex used to protect the pressHandler field.
	pressHandlerMx sync.Mutex
	// pressHandler is the callback that is called whenever a button is pressed.
	pressHandler func(context.Context, int) error
}

// New opens a connection to a Stream Deck and provides a user-friendly wrapper
// that makes interacting with the Stream Deck easier and more convenient.
func New(ctx context.Context) (*StreamDeck, error) {
	device, err := Open(ctx)
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, err
	}
	return NewFromDevice(ctx, device)
}

// NewFromDevice creates a new StreamDeck from an existing Device, most users
// should use the New function instead.
//
// This function can be useful if you have a specific USB device you want to use
// like if you are using systemd and have a symlink to /dev/streamdeck or if you
// want to connect to multiple Stream Decks.
func NewFromDevice(ctx context.Context, device *Device) (*StreamDeck, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &StreamDeck{
		device:     device,
		brightness: BrightnessFull,

		cancel: cancel,
		ch:     make(chan int),
	}

	go s.device.buttonPressListener(ctx, s.ch)
	go s.buttonCallbackListener(ctx)

	return s, nil
}

// Close stops the event listeners and closes the underlying connection to the
// Stream Deck Device.
func (s *StreamDeck) Close(ctx context.Context) error {
	s.cancel()
	return s.device.Close(ctx)
}

// Device returns the underlying Stream Deck device.
func (s *StreamDeck) Device() *Device {
	return s.device
}

// Brightness returns the current brightness of the Stream Deck.
func (s *StreamDeck) Brightness() int {
	return s.brightness
}

// SetBrightness sets the brightness of the Stream Deck.
func (s *StreamDeck) SetBrightness(ctx context.Context, brightness int) error {
	if brightness < BrightnessMin {
		brightness = BrightnessMin
	}
	if brightness > BrightnessFull {
		brightness = BrightnessFull
	}
	if err := s.device.SetBrightness(ctx, brightness); err != nil {
		return err
	}
	s.brightness = brightness
	return nil
}

// SetHandler sets the button press handler used by the end-user to handle press
// events.
func (s *StreamDeck) SetHandler(fn func(context.Context, int) error) {
	s.pressHandlerMx.Lock()
	defer s.pressHandlerMx.Unlock()

	s.pressHandler = fn
}

// ProcessImage processes an image to be used with the Stream Deck.
func (s *StreamDeck) ProcessImage(img image.Image) ([]byte, error) {
	if img == nil {
		return nil, nil
	}

	// Resize and rotate the image
	res := image.NewRGBA(s.device.GIFT().Bounds(img.Bounds()))
	s.device.GIFT().Draw(res, img)

	return getImageForButton(res, s.device.ImageFormat())
}

// buttonCallbackListener listens for events to be sent over the StreamDeck#ch
// channel and calls StreamDeck#pressHandler with the data.
func (s *StreamDeck) buttonCallbackListener(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case index := <-s.ch:
			s.pressHandlerMx.Lock()
			pressHandler := s.pressHandler
			s.pressHandlerMx.Unlock()

			if pressHandler == nil {
				continue
			}
			_ = pressHandler(ctx, index)
		}
	}
}
