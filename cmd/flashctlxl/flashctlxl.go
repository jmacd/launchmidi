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
