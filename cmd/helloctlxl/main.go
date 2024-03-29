package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jmacd/launchmidi/launchctl/xl"
)

const duration = 10 * time.Second

func main() {
	l, err := xl.Open()
	if err != nil {
		log.Fatalf("error while openning connection to launchctl: %v", err)
	}
	defer l.Close()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	start := time.Now()

	go func() {
		time.Sleep(100 * time.Millisecond)

		for i := xl.Control(0); time.Now().Sub(start) < duration/4; {
			l.SetColor(0, i, 0xf)
			l.SwapBuffers(0)
			l.SetColor(0, i, 0)
			i = (i + xl.NumLEDs + 1) % xl.NumLEDs
		}

		// TODO: Note that the behavior written above continues at this point,
		// does this indicate buffered data?

		for b := 0; b < 8; b += 4 {
			l.SetColor(0, xl.ControlButtonTrackFocus[0+b], xl.ColorBrightRed)
			l.SetColor(0, xl.ControlButtonTrackFocus[1+b], xl.Flash(xl.ColorBrightYellow))
			l.SetColor(0, xl.ControlButtonTrackFocus[2+b], xl.Flash(xl.ColorBrightOrange))
			l.SetColor(0, xl.ControlButtonTrackFocus[3+b], xl.ColorBrightGreen)

			l.SetColor(0, xl.ControlButtonTrackControl[0+b], xl.ColorDimRed)
			l.SetColor(0, xl.ControlButtonTrackControl[1+b], xl.Flash(xl.ColorDimYellow))
			l.SetColor(0, xl.ControlButtonTrackControl[2+b], xl.Flash(xl.ColorDimOrange))
			l.SetColor(0, xl.ControlButtonTrackControl[3+b], xl.ColorDimGreen)
		}

		for i := xl.ControlButtonDevice; i <= xl.ControlButtonRight; i++ {
			l.SetColor(0, i, 0xf)
		}

		for i := xl.Control(0); i < 12; i++ {
			l.SetColor(0, i*2, xl.Flash(xl.EightColors[(i*2)%8]))
			l.SetColor(0, i*2+1, xl.FlashUnknown(xl.EightColors[(i*2+1)%8]))
		}

		_ = l.SwapBuffers(0)
	}()

	cb := func(ch int, control xl.Control, value xl.Value) {
		fmt.Println("Set channel", ch, "control", control, "value", value.Float())
	}
	for i := xl.Control(0); i < xl.NumControls; i++ {
		l.AddCallback(xl.AllChannels, i, cb)
	}

	if err := l.Run(ctx); err != nil {
		log.Fatal("error running launchctl: ", err)
	}
}
