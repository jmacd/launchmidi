package mini

import (
	"fmt"

	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv"
)

const (
	MIDIStatusNoteOff = 0x80

	MIDIStatusNoteOn        = 0x90
	MIDIStatusControlChange = 0xb0
	MIDIStatusCodeMask      = 0xf0
	MIDIChannelMask         = 0x0f
)

func discover() (drivers.In, drivers.Out, error) {
	defer midi.CloseDriver()

	in, err := midi.FindInPort("MINI")
	if err != nil {
		return nil, nil, fmt.Errorf("can't find input: %w", err)
	}

	out, err := midi.FindOutPort("MINI")
	if err != nil {
		return nil, nil, fmt.Errorf("can't find output: %w", err)
	}

	return in, out, nil
}
