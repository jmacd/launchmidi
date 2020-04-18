package xl

import "github.com/rakyll/portmidi"

const (
	MIDIStatusNoteOff       = 0x80
	MIDIStatusNoteOn        = 0x90
	MIDIStatusControlChange = 0xb0
	MIDIStatusCodeMask      = 0xf0
	MIDIChannelMask         = 0x0f
)

// discover finds the first connected LaunchControl device as a MIDI
// device.
func discover() (input portmidi.DeviceID, output portmidi.DeviceID, err error) {
	in := -1
	out := -1
	for i := 0; i < portmidi.CountDevices(); i++ {
		info := portmidi.Info(portmidi.DeviceID(i))
		if info.Name == DeviceName {
			if info.IsInputAvailable {
				in = i
			}
			if info.IsOutputAvailable {
				out = i
			}
		}
	}
	if in == -1 || out == -1 {
		err = ErrNoLaunchControl
	} else {
		input = portmidi.DeviceID(in)
		output = portmidi.DeviceID(out)
	}
	return
}
