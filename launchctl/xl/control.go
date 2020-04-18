package xl

const (
	ControlButtonDevice Control = 40
	ControlButtonMute   Control = 41
	ControlButtonSolo   Control = 42
	ControlButtonRecord Control = 43
	ControlButtonUp     Control = 44
	ControlButtonDown   Control = 45
	ControlButtonLeft   Control = 46
	ControlButtonRight  Control = 47

	ControlInvalid Control = NumControls
)

var (
	ControlKnobSendA          = controlRange(0, 8)
	ControlKnobSendB          = controlRange(8, 16)
	ControlKnobPanDevice      = controlRange(16, 24)
	ControlButtonTrackFocus   = controlRange(24, 32)
	ControlButtonTrackControl = controlRange(32, 40)
)

func controlRange(from, to Control) (r []Control) {
	for c := from; c < to; c++ {
		r = append(r, c)
	}
	return
}
