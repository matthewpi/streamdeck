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

package view

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/matthewpi/streamdeck"
	"github.com/matthewpi/streamdeck/button"
)

// Buttons is a View that allows setting individual buttons.
type Buttons struct {
	sd *streamdeck.StreamDeck

	buttonsMx sync.Mutex
	buttons   []button.Button
}

var _ streamdeck.View = (*Buttons)(nil)

// NewButtons returns a Buttons View capable of displaying multiple static
// or animated buttons.
func NewButtons(sd *streamdeck.StreamDeck) (*Buttons, error) {
	if sd == nil {
		return nil, errors.New("view: streamdeck cannot be nil")
	}
	return &Buttons{
		sd:      sd,
		buttons: make([]button.Button, sd.Device().ButtonCount()),
	}, nil
}

// Apply satisfies the View interface.
func (b *Buttons) Apply(ctx context.Context) error {
	b.buttonsMx.Lock()
	defer b.buttonsMx.Unlock()

	for i, btn := range b.buttons {
		if btn, ok := btn.(button.Animated); ok {
			i := i
			btn := btn
			go b.animate(ctx, i, btn)
			continue
		}

		if err := b.updateButton(ctx, i, btn); err != nil {
			return err
		}
	}
	return nil
}

func (b *Buttons) animate(ctx context.Context, i int, btn button.Animated) {
	fn := func(ctx context.Context, v []byte) error {
		return b.update(ctx, i, v)
	}

	if err := btn.Animate(ctx, fn); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("failed to animate button: %v\n", err)
	}
}

// Set sets a Button on the view, it will not render the image on a
// Stream Deck, a separate call to View#Apply or Buttons#Update is required to
// actually apply the change(s).
//
// This method is safe to call concurrently.
func (b *Buttons) Set(index int, btn button.Button) *Buttons {
	b.buttonsMx.Lock()
	b.buttons[index] = btn
	b.buttonsMx.Unlock()
	return b
}

// Update updates the image displayed on a StreamDeck using the Button set on
// this view.
func (b *Buttons) Update(ctx context.Context, index int) error {
	if index >= len(b.buttons) {
		return errors.New("view: button out of range")
	}

	b.buttonsMx.Lock()
	btn := b.buttons[index]
	b.buttonsMx.Unlock()
	return b.updateButton(ctx, index, btn)
}

func (b *Buttons) updateButton(ctx context.Context, index int, btn button.Button) error {
	var v []byte
	if btn != nil {
		v = btn.Image()
	}
	return b.sd.Device().SetButton(ctx, index, v)
}

func (b *Buttons) update(ctx context.Context, index int, v []byte) error {
	return b.sd.Device().SetButton(ctx, index, v)
}
