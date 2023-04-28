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
	"sync/atomic"
)

// StreamDeck represents an Elgato Stream Deck.
type StreamDeck struct {
	// device is a wrapper of the underlying USB HID Device.
	device *Device
	// brightness is the Stream Deck's target brightness.  brightness is not
	// always guaranteed to be the Stream Deck's current brightness, such as if
	// the Stream Deck is set to sleep after `x` amount of inactivity in order
	// to preserve the LCD's lifespan.
	brightness atomic.Uint32
	// isSleeping determines whether the Stream Deck is sleeping or not.  Sleep
	// mode turns off the display on the Stream Deck by setting the brightness
	// to BrightnessMin, and intercepts all button presses as a way to disable
	// sleep mode, any button presses while sleep mode is enabled will turn back
	// on the display and WILL NOT trigger their associated pressHandler unless
	// the button is pressed again while the Stream Deck is not sleeping.
	isSleeping atomic.Bool

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

// NewFromDevice creates a new Stream Deck from an existing Device, most users
// should use the New function instead.
//
// This function can be useful if you have a specific USB device you want to use
// like if you are using systemd and have a symlink to /dev/streamdeck or if you
// want to connect to multiple Stream Decks.
func NewFromDevice(ctx context.Context, device *Device) (*StreamDeck, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &StreamDeck{
		device: device,

		cancel: cancel,
		ch:     make(chan int),
	}
	s.brightness.Store(BrightnessFull)

	go s.device.buttonPressListener(ctx, s.ch)
	go s.buttonCallbackListener(ctx)

	return s, nil
}

// Close stops the event listeners and closes the underlying connection to the
// Stream Deck device.
func (s *StreamDeck) Close(ctx context.Context) error {
	s.cancel()
	return s.device.Close(ctx)
}

// Device returns the underlying Stream Deck device.
func (s *StreamDeck) Device() *Device {
	return s.device
}

// Brightness returns the target brightness of the Stream Deck.  This will not
// return 0 if the Stream Deck is sleeping.  To check if the Stream Deck is
// sleeping use StreamDeck#IsSleeping().
func (s *StreamDeck) Brightness() uint32 {
	return s.brightness.Load()
}

// SetBrightness sets the brightness of the Stream Deck.
func (s *StreamDeck) SetBrightness(ctx context.Context, brightness uint32) error {
	if brightness < BrightnessMin {
		brightness = BrightnessMin
	}
	if brightness > BrightnessFull {
		brightness = BrightnessFull
	}
	// Only update the Stream Deck's actual brightness if it isn't sleeping.
	if !s.IsSleeping() {
		if err := s.setBrightness(ctx, brightness); err != nil {
			return err
		}
	}
	// Always persist the new target brightness.
	s.brightness.Store(brightness)
	return nil
}

// setBrightness sets the brightness of the Stream Deck.
func (s *StreamDeck) setBrightness(ctx context.Context, brightness uint32) error {
	if err := s.device.SetBrightness(ctx, brightness); err != nil {
		return err
	}
	return nil
}

// IsSleeping returns true if the Stream Deck is currently sleeping.
func (s *StreamDeck) IsSleeping() bool {
	return s.isSleeping.Load()
}

// SetSleeping sets whether the Stream Deck is sleeping or not.
func (s *StreamDeck) SetSleeping(ctx context.Context, sleeping bool) error {
	newBrightness := s.Brightness()
	if sleeping {
		newBrightness = BrightnessMin
	}
	if err := s.setBrightness(ctx, newBrightness); err != nil {
		return err
	}

	// Update the isSleeping state only after successfully changing the
	// Stream Deck's brightness.
	s.isSleeping.Store(sleeping)

	return nil
}

// ToggleSleep toggles the sleep state for the Stream Deck.
func (s *StreamDeck) ToggleSleep(ctx context.Context) (bool, error) {
	if err := s.SetSleeping(ctx, !s.IsSleeping()); err != nil {
		return false, err
	}
	return s.IsSleeping(), nil
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

			// Disable sleep whenever a button is pressed, another button press
			// is required to trigger the underlying press handler.
			if s.IsSleeping() {
				// TODO: we should probably do something about this error.
				_ = s.SetSleeping(ctx, false)
				continue
			}

			if pressHandler == nil {
				continue
			}
			// TODO: we should probably do something about this error.
			_ = pressHandler(ctx, index)
		}
	}
}
