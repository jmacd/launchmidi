package mini

const (
	ColorBrightRed    Color = 0xc
	ColorBrightOrange Color = 0xd
	ColorBrightYellow Color = 0xf
	ColorBrightGreen  Color = 0x3

	ColorDimRed    Color = 0x4
	ColorDimOrange Color = 0x9
	ColorDimYellow Color = 0x5
	ColorDimGreen  Color = 0x1

	ColorFlash                Color = 0x10
	ColorFlashIfUninitialized Color = 0x20
)

var (
	EightColors = []Color{
		ColorBrightRed,
		ColorBrightOrange,
		ColorBrightYellow,
		ColorBrightGreen,
		ColorDimRed,
		ColorDimOrange,
		ColorDimYellow,
		ColorDimGreen,
	}

	FourDimColors = []Color{
		ColorDimRed,
		ColorDimOrange,
		ColorDimYellow,
		ColorDimGreen,
	}
	FourBrightColors = []Color{
		ColorBrightRed,
		ColorBrightOrange,
		ColorBrightYellow,
		ColorBrightGreen,
	}
)

func Flash(c Color) Color {
	return c | ColorFlash
}

func FlashUnknown(c Color) Color {
	return c | ColorFlashIfUninitialized
}

// func (c Color) isFlashing(v Value) bool {
// 	if c&ColorFlash != 0 {
// 		return true
// 	}
// 	if c&ColorFlashIfUninitialized != 0 {
// 		if v == ValueUninitialized {
// 			return true
// 		}
// 	}
// 	return false
// }

// func (c Color) toByte(flashingOff, flasher bool, v Value) byte {
// 	if flashingOff && flasher {
// 		if c != 0 {
// 			return 0
// 		}
// 		c = ColorDimYellow
// 	}
// 	red := (byte(c) & 0xc) >> 2
// 	green := byte(c) & 0x3
// 	return red + green<<4
// }
