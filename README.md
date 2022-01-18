# Stream Deck

Library for directly interacting and controlling an Elgato Stream Deck on Linux.

This library is designed to take exclusive control over a Stream Deck using USB HID, if you are
an end-user looking for software just to control your Stream Deck, this is not what you are looking
for. If you are looking to build your own software whether it be a CLI or GUI app to control your
Stream Deck, you have come to the right place.

This library was inspired by many of the other go streamdeck libraries, I created this one because
all the other ones I could find either didn't work, didn't support the features I wanted, required
CGO, or were poorly designed (imo) making them hard to use.

The internal `hid` package was heavily-based on <https://github.com/zserge/hid> with some
improvements from <https://github.com/rafaelmartins/usbfs>.

## Features

- Native Linux support (No CGO)
  - Caveat: This library does not support Windows or MacOS, and will not for the conceivable future.
- Supports GIFs
  - The most use~~less~~ful feature
- Easy to use
- Performant

### Missing

- Tests
- More in-depth examples and documentation
- Probably some other things I had no idea existed

## Example

```go
package main

import (
	"context"
	"embed"
	"fmt"
	"image"
	"image/gif"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/matthewpi/streamdeck"
	"github.com/matthewpi/streamdeck/button"
	"github.com/matthewpi/streamdeck/view"
)

//go:embed .embed/*.png .embed/*.gif
var embedFs embed.FS

func main() {
	if err := start(context.Background()); err != nil {
		panic(err)
		return
	}
}

func start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sd, err := streamdeck.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to find or connect to a streamdeck: %w", err)
	}
	if sd == nil {
		return fmt.Errorf("no streamdeck devices found: %w", err)
	}
	defer func(ctx context.Context, sd *streamdeck.StreamDeck) {
		if err := sd.Close(ctx); err != nil {
			log.Printf("an error occurred while closing the streamdeck: %v\n")
		}
	}(ctx, sd)

	if err := sd.SetBrightness(ctx, 25); err != nil {
		return fmt.Errorf("failed to set streamdeck brightness: %w", err)
	}

	buttons, err := view.NewButtons(sd)
	if err != nil {
		return fmt.Errorf("failed to create button view: %w", err)
	}

	sd.SetHandler(func(ctx context.Context, index int) error {
		switch index {
		case 0:
			fmt.Println("you pressed a button!")
		case 1:
			fmt.Println("you pressed another button!")
		}
		return nil
	})

	buttons.Set(1, button.NewImage(mustGetImage(sd, "spotify_play.png")))
	buttons.Set(2, button.NewGIF(sd, mustGetGIF("peepoDance.gif")))

	ctx3, cancel3 := context.WithCancel(ctx)
	defer cancel3()
	if err := buttons.Apply(ctx3); err != nil {
		return fmt.Errorf("failed to update streamdeck buttons: %w", err)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, unix.SIGTERM)
	<-ch
	log.Println("shutting down")
	return nil
}

func getImage(sd *streamdeck.StreamDeck, filename string) ([]byte, error) {
	f, err := embedFs.Open(filepath.Join(".embed", filename))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return sd.ProcessImage(img)
}

func mustGetImage(sd *streamdeck.StreamDeck, filename string) []byte {
	img, err := getImage(sd, filename)
	if err != nil {
		panic(err)
		return nil
	}
	return img
}

func getGIF(filename string) (*gif.GIF, error) {
	f, err := embedFs.Open(filepath.Join(".embed", filename))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func mustGetGIF(filename string) *gif.GIF {
	img, err := getGIF(filename)
	if err != nil {
		panic(err)
		return nil
	}
	return img
}
```

## Design

### Device

`Device` is the lowest-level API that is exposed. A Device allows sending raw data as well as
providing some convenience functions, it's main purpose is to provide a base that the `StreamDeck`
structure interacts with in order to expose a more user-friendly API.

### StreamDeck

`StreamDeck` provides the user-friendly API that most integrations of this library will use, it
allows the use of [Views](#view) in order to provide an easy way of setting buttons and handling
press events.

### View

A `View` is used by a [`Streamdeck`]() to set the images for all buttons, a View may optionally
override the Stream Deck's default button press handler in order to provide a different API for
handling button presses.

###### Example (refer to [`view/buttons.go`](view/buttons.go))

### Button

`Button` is used for buttons with static content, like a solid color or image button. If you need
to change the content of a static button, you can either just create a new button and update the
`View`, or implement a custom button that pulls its content from an external source and is capable
of updating the View itself.

###### Example (refer to [`button/button.go`](button/button.go))

### Button (Animated)

`Animated` is an addon to the `Button` interface that allows a button to determine when it wants to
update itself.  A common example of this would be displaying a GIF which from my knowledge the
Stream Deck does not natively support, meaning we need to wait the time required for each frame and
keep updating the image displayed on the Stream Deck. This interface could also be useful for
displaying dynamic content like Spotify Album art for example.

###### Example (refer to [`button/animated.go`](button/animated.go))
