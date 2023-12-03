package controller

type Control int

type Value uint8

type Color byte

type Callback func(midiChan int, control Control, value Value)

type Input interface {
	AddCallback(ch int, con Control, cb Callback)

	SetColor(ch int, con Control, c Color)

	AllChannels() int
}

func (v Value) Float() float64 {
	switch {
	case v == 0:
		return 0
	case v == 64:
		return 0.5
	case v == 127:
		return 1
	case v < 64:
		return float64(v) / 128
	default:
		return float64(v-1) / 126
	}
}
