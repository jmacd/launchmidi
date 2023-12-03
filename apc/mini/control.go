package mini

const (
	// ControlButtonDevice Control = 40
	// ControlButtonMute   Control = 41
	// ControlButtonSolo   Control = 42
	// ControlButtonRecord Control = 43
	// ControlButtonUp     Control = 44
	// ControlButtonDown   Control = 45
	// ControlButtonLeft   Control = 46
	// ControlButtonRight  Control = 47

	ControlInvalid Control = NumControls
)

var (
	// ControlKnobSendA          = controlRange(0, 8)
	// ControlKnobSendB          = controlRange(8, 16)
	// ControlKnobPanDevice      = controlRange(16, 24)
	ControlButtonTrackFocus   = controlRange(0x0, 0x8)
	ControlButtonTrackControl = controlRange(0x8, 0x10)
	// ... 0x3f (i.e., 63) are 8x8 grid of buttons
	// then two more [0x40-0x4f] + 1 (i.1., 64+17 total) buttons

	ControlSlider = controlRange(0x51, 0x5a)
)

func controlRange(from, to Control) (r []Control) {
	for c := from; c < to; c++ {
		r = append(r, c)
	}
	return
}

// func (c Control) IsButton() bool {
// 	return c >= 24 && c < 48
// }

func (l *LaunchControl) Get(control Control) float64 {
	l.lock.Lock()
	defer l.lock.Unlock()
	v := l.value[l.currentChannel][control]
	if v == ValueUninitialized {
		return 0.5
	}
	return v.Float()
}
