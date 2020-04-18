package xl

import (
	"bytes"

	"github.com/rakyll/portmidi"
)

func (l *LaunchControl) getControl(evt portmidi.Event) Control {
	switch evt.Status & MIDIStatusCodeMask {
	case MIDIStatusControlChange:
		return l.getControlChangeIndex(Value(evt.Data1))
	case MIDIStatusNoteOn, MIDIStatusNoteOff:
		return l.getNoteChangeIndex(Value(evt.Data1))
	default:
		return ControlInvalid
	}
}

func (l *LaunchControl) event(evt portmidi.Event) {
	if len(evt.SysEx) != 0 {
		l.sysexEvent(evt)
		return
	}

	midiChannel := Value(evt.Status & MIDIChannelMask)
	control := l.getControl(evt)
	if control == ControlInvalid {
		return
	}

	l.lock.Lock()
	defer l.lock.Unlock()

	l.value[midiChannel][control] = Value(evt.Data2)
}

func (l *LaunchControl) sysexEvent(evt portmidi.Event) {
	sb := evt.SysEx
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
	case 13 <= data && data <= 20: // 0ffset 0
		return Control(data - 13 + 0)

	case 29 <= data && data <= 36: // Offset 8
		return Control(data - 29 + 8)

	case 49 <= data && data <= 56: // Offset 16
		return Control(data - 49 + 16)

	case 104 <= data && data <= 107: // Offset 44
		return Control(data - 104 + 44)

	case 77 <= data && data <= 84: // Offset 48 -- Sliders are missing LEDs :sadface:
		return Control(data - 77 + 48)
	}

	return -1
}

func (l *LaunchControl) getNoteChangeIndex(data Value) Control {
	switch {
	case 41 <= data && data <= 44: // 0ffset 24
		return Control(data - 41 + 24)

	case 57 <= data && data <= 60: // Offset 28
		return Control(data - 57 + 28)

	case 73 <= data && data <= 76: // Offset 32
		return Control(data - 73 + 32)

	case 89 <= data && data <= 92: // Offset 36
		return Control(data - 89 + 36)

	case 105 <= data && data <= 108: // Offset 40
		return Control(data - 105 + 40)
	}

	return -1
}
