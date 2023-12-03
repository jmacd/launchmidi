package mini

import (
	"bytes"
	"fmt"

	"github.com/jmacd/nerve/pru/program/player/input"
)

type Event struct {
	Timestamp int32
	Status    byte
	Data1     byte
	Data2     byte
}

func (l *LaunchControl) getControl(evt Event) Control {
	switch evt.Status & MIDIStatusCodeMask {
	case MIDIStatusControlChange:
		return l.getControlChangeIndex(Value(evt.Data1))
	case MIDIStatusNoteOn, MIDIStatusNoteOff:
		return l.getNoteChangeIndex(Value(evt.Data1))
	default:
		return ControlInvalid
	}
}

func (l *LaunchControl) event(evt Event) {
	midiChannel := int(evt.Status & MIDIChannelMask)
	control := l.getControl(evt)
	if control == ControlInvalid {
		fmt.Printf("invalid control 0x%x\n", control)
		return
	}

	l.lock.Lock()
	value := Value(evt.Data2)
	var cbs, acbs []input.Callback
	l.value[midiChannel][control] = value
	cbs = l.calls[midiChannel][control]
	acbs = l.calls[AllChannels][control]
	l.lock.Unlock()

	for _, cb := range cbs {
		cb(midiChannel, control, value)
	}
	for _, cb := range acbs {
		cb(midiChannel, control, value)
	}
}

func (l *LaunchControl) sysexEvent(sb []byte) {
	if len(sb) != 9 {
		return
	}

	if !bytes.Equal(sb[1:7], []byte{0x0, 0x20, 0x29, 0x2, 0x11, 0x77}) {
		return
	}

	// "Template changed" is the only documented SysEx from the device.
	l.lock.Lock()
	l.currentChannel = int(sb[7])
	l.lock.Unlock()
}

func (l *LaunchControl) getControlChangeIndex(data Value) Control {
	switch {
	case 0x30 <= data && data <= 0x38:
		return ControlSlider[0] + Control(data-0x30)
	}

	return -1
}

func (l *LaunchControl) getNoteChangeIndex(data Value) Control {
	switch {
	case 0 <= data && data <= 0x47: // 0ffset 0
		return Control(data)

	case 0x52 <= data && data <= 0x59: // Offset 0x48
		return Control(0x48 + data - 0x52)

	case 0x62 <= data && data <= 0x62: // Offset 64
		return Control(0x50 + data - 0x62)

	}

	return -1
}
