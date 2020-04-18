package main

import (
	"context"
	"log"
	"time"

	"github.com/jmacd/launchmidi/launchctl/xl"
	launchctl "github.com/jmacd/launchmidi/launchctl/xl"
)

const duration = 10 * time.Second

func main() {
	l, err := launchctl.Open()
	if err != nil {
		log.Fatalf("error while openning connection to launchctl: %v", err)
	}
	defer l.Close()

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	start := time.Now()

	go func() {
		time.Sleep(100 * time.Millisecond)

		for i := xl.Control(0); time.Now().Sub(start) < duration/2; {
			l.SetColor(0, i, 0xf)
			l.SwapBuffers(0)
			l.SetColor(0, i, 0)
			i = (i + xl.NumLEDs + 1) % xl.NumLEDs
		}

		l.SetColor(0, xl.ControlButtonTrackFocus[0], xl.ColorBrightRed)
		l.SetColor(0, xl.ControlButtonTrackFocus[1], xl.Flash(xl.ColorBrightYellow))
		l.SetColor(0, xl.ControlButtonTrackFocus[2], xl.Flash(xl.ColorBrightOrange))
		l.SetColor(0, xl.ControlButtonTrackFocus[3], xl.ColorBrightGreen)

		l.SetColor(0, xl.ControlButtonTrackControl[0], xl.ColorDimRed)
		l.SetColor(0, xl.ControlButtonTrackControl[1], xl.Flash(xl.ColorDimYellow))
		l.SetColor(0, xl.ControlButtonTrackControl[2], xl.Flash(xl.ColorDimOrange))
		l.SetColor(0, xl.ControlButtonTrackControl[3], xl.ColorDimGreen)

		l.SetColor(0, xl.ControlKnobSendA[0], xl.Flash(xl.ColorBrightOrange))
		l.SetColor(0, xl.ControlKnobSendA[1], xl.FlashUnknown(xl.ColorBrightOrange))

		_ = l.SwapBuffers(0)
	}()
	if err := l.Run(ctx); err != nil {
		log.Fatal("error running launchctl: ", err)
	}
}
