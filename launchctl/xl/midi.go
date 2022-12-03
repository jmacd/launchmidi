package xl

import (
	"fmt"

	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv"
)

const (
	MIDIStatusNoteOff       = 0x80
	MIDIStatusNoteOn        = 0x90
	MIDIStatusControlChange = 0xb0
	MIDIStatusCodeMask      = 0xf0
	MIDIChannelMask         = 0x0f
)

// // discover finds the first connected LaunchControl device as a MIDI
// // device.
// func discover() (input portmidi.DeviceID, output portmidi.DeviceID, err error) {
// 	in := -1
// 	out := -1
// 	for i := 0; i < portmidi.CountDevices(); i++ {
// 		info := portmidi.Info(portmidi.DeviceID(i))
// 		fmt.Println(info)
// 		if strings.HasPrefix(info.Name, DeviceName) {
// 			if info.IsInputAvailable {
// 				in = i
// 			}
// 			if info.IsOutputAvailable {
// 				out = i
// 			}
// 		}
// 	}
// 	if in == -1 || out == -1 {
// 		err = ErrNoLaunchControl
// 	} else {
// 		input = portmidi.DeviceID(in)
// 		output = portmidi.DeviceID(out)
// 	}
// 	return
// }

func discover() (drivers.In, drivers.Out, error) {
	defer midi.CloseDriver()

	in, err := midi.FindInPort("XL")
	if err != nil {
		return nil, nil, fmt.Errorf("can't find input: %w", err)
	}

	out, err := midi.FindOutPort("XL")
	if err != nil {
		return nil, nil, fmt.Errorf("can't find output: %w", err)
	}

	return in, out, nil
}

// stop, err := midi.ListenTo(in, func(msg midi.Message, timestampms int32) {
// 	fmt.Println("Hey", msg)
// 	var bt []byte
// 	var ch, key, vel uint8
// 	switch {
// 	case msg.GetSysEx(&bt):
// 		fmt.Printf("got sysex: % X\n", bt)
// 	case msg.GetNoteStart(&ch, &key, &vel):
// 		fmt.Printf("starting note %s on channel %v with velocity %v\n", midi.Note(key), ch, vel)
// 	case msg.GetNoteEnd(&ch, &key):
// 		fmt.Printf("ending note %s on channel %v\n", midi.Note(key), ch)
// 	default:
// 		// ignore
// 	}
// }, midi.UseSysEx())

// //fmt.Println("YO", msg)
// if err != nil {
// 	fmt.Printf("ERROR: %s\n", err)
// 	return
// }

// time.Sleep(time.Second * 500)

// stop()
