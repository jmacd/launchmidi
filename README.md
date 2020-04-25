# launchmidi [![GoDoc](https://godoc.org/github.com/jmacd/launchmidi?status.svg)](https://godoc.org/github.com/jmacd/launchmidi)
This package allows lets you program with your Novation LaunchControl XL in Go.

This library offers full control over this device, including:
- Read current values of 24 knobs, 8 sliders, and 24 buttons
- Callbacks for reacting to change on 56 controls
- Template switching support
- Color doubled-buffering
- Flashing LEDs
- `FlashUnknown()` supports flashing uninitialized knobs and sliders.

```sh
go get github.com/jmacd/launchmidi
```

[Portmidi](github.com/rakyll/launchmidi) is required to use this package.

```sh
$ apt-get install libportmidi-dev
# or
$ brew install portmidi
```

## Example: Flash all the buttons

This program flashes the 3 rows of knohs and 2 rows of buttons times 8 columns of LEDs at startup.  Half of the controls are set to flashing uninitialized, the other half flash indefinitely.

This exhibits how "flash" is treated for sliders, which do not have LEDs.  Sliders flash on the adjacent Track Focus button below.  Any flashing slider causes the four right-side (Device, Mute, Solo, Record Arm) buttons to flash.

```go
package main

import (
	"context"
	"log"

	"github.com/jmacd/launchmidi/launchctl/xl"
)

func main() {
	l, err := xl.Open()
	if err != nil {
		log.Fatalf("error while openning connection to launchctl: %v", err)
	}
	defer l.Close()

	go l.Run(context.Background())

	const midiChan = 0
	for i := 0; i < 8; i++ {
		c := xl.EightColors[i]
		cf := c
		cfu := c
		if i%2 == 1 {
			cf = xl.Flash(c)
		} else {
			cfu = xl.FlashUnknown(c)
		}

		l.SetColor(midiChan, xl.ControlKnobSendA[i], cfu)
		l.SetColor(midiChan, xl.ControlKnobSendB[i], cfu)
		l.SetColor(midiChan, xl.ControlKnobPanDevice[i], cfu)
		l.SetColor(midiChan, xl.ControlSlider[i], cfu)
		l.SetColor(midiChan, xl.ControlButtonTrackFocus[i], cf)
		l.SetColor(midiChan, xl.ControlButtonTrackControl[i], cf)
	}
	l.SwapBuffers(midiChan)
	select {}
}
```
