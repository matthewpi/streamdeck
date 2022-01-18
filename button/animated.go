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

package button

import (
	"context"
	"image/gif"
	"time"

	"github.com/matthewpi/streamdeck"
)

// Animated represents a Button that is animated.
type Animated interface {
	// Animate is called when the Button should start animating,
	// it is expected that this function exists only after an error
	// or the passed context is cancelled.
	//
	// The closure passed should be called with an image processed by
	// StreamDeck#ProcessImage which should be done ahead of time before Animate
	// is called.
	Animate(context.Context, func(context.Context, []byte) error) error
}

// GIF represents an animated Button displaying a GIF
type GIF struct {
	gif    *gif.GIF
	frames [][]byte
	delay  []time.Duration
}

var (
	_ Animated = (*GIF)(nil)
	_ Button   = (*GIF)(nil)
)

// NewGIF returns a new animated Button that displays a GIF.
func NewGIF(sd *streamdeck.StreamDeck, gif *gif.GIF) *GIF {
	if len(gif.Image) != len(gif.Delay) {
		panic("button: amount of frames does not match amount of delay")
		return nil
	}

	g := &GIF{
		gif:    gif,
		frames: make([][]byte, len(gif.Image)),
		delay:  make([]time.Duration, len(gif.Delay)),
	}
	for i, img := range gif.Image {
		rawImage, err := sd.ProcessImage(img)
		if err != nil {
			return nil
		}
		g.frames[i] = rawImage
	}
	for i, v := range gif.Delay {
		g.delay[i] = time.Duration(v) * time.Millisecond * 10
	}

	return g
}

// Animate satisfies the Animated interface.
func (g *GIF) Animate(ctx context.Context, fn func(context.Context, []byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			for i, f := range g.frames {
				if err := fn(ctx, f); err != nil {
					return err
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				// TODO: https://tylerstiene.ca/blog/careful-gos-standard-ticker-is-not-realtime/
				case <-time.NewTimer(g.delay[i]).C:
				}
			}
		}
	}
}

// Image satisfies the Button interface.
func (*GIF) Image() []byte {
	return nil
}
