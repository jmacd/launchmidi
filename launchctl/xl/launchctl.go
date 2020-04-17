// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package xl provides interfaces to talk to Novation Launch Control XL via MIDI in and out.
package xl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rakyll/portmidi"
)

const (
	MIDI_Status_Note_Off       = 0x80
	MIDI_Status_Note_On        = 0x90
	MIDI_Status_Control_Change = 0xb0
	MIDI_Status_Code_Mask      = 0xf0
	MIDI_Channel_Mask          = 0x0f

	MaxEventsPerPoll = 1024
	ReadBufferDepth  = 16
	PollingPeriod    = 10 * time.Millisecond
	NumChannels      = 16
	NumControls      = 6*8 + 4 + 4
	NumLEDs          = 48

	FlashPeriod = 433 * time.Millisecond

	Value_Uninitialized Value = 128

	Color_BrightRed    Color = 0xc
	Color_BrightOrange Color = 0xd
	Color_BrightYellow Color = 0xf
	Color_BrightGreen  Color = 0x3

	Color_DimRed    Color = 0x4
	Color_DimOrange Color = 0x9
	Color_DimYellow Color = 0x5
	Color_DimGreen  Color = 0x1

	Color_Flash                Color = 0x10
	Color_FlashIfUninitialized Color = 0x20

	Control_Button_Device Control = 40
	Control_Button_Mute   Control = 41
	Control_Button_Solo   Control = 42
	Control_Button_Record Control = 43
	Control_Button_Up     Control = 44
	Control_Button_Down   Control = 45
	Control_Button_Left   Control = 46
	Control_Button_Right  Control = 47

	Control_Invalid Control = NumControls
)

var (
	Control_Knob_SendA          = controlRange(0, 8)
	Control_Knob_SendB          = controlRange(8, 16)
	Control_Knob_PanDevice      = controlRange(16, 24)
	Control_Button_TrackFocus   = controlRange(24, 32)
	Control_Button_TrackControl = controlRange(32, 40)
)

type (
	Value uint8

	// Control indexes are assigned in the range [0, NumControls).
	// They are ordered such that the control number equals the
	// LED number for the first 48 controls.
	Control int

	Color byte
)

// LaunchControl represents a device with an input and output MIDI stream.
type LaunchControl struct {
	inputStream  *portmidi.Stream
	outputStream *portmidi.Stream

	lock    sync.Mutex
	flashes int64

	currentChannel int
	// by MIDI channel:

	swaps [NumChannels]int64              // number of swaps
	color [NumChannels][NumControls]Color // colors [0-15] + 2 bits
	value [NumChannels][NumControls]Value // control values [0-127] or uninitialized
}

// Open opens a connection to the XL and initializes an input and
// output stream to the currently connected device. If there are no
// devices connected, it returns an error.
func Open() (*LaunchControl, error) {
	input, output, err := discover()
	if err != nil {
		return nil, err
	}

	var inStream, outStream *portmidi.Stream
	if inStream, err = portmidi.NewInputStream(input, 1024); err != nil {
		return nil, err
	}
	if outStream, err = portmidi.NewOutputStream(output, 1024, 0); err != nil {
		return nil, err
	}
	lc := &LaunchControl{inputStream: inStream, outputStream: outStream}
	for ch := 0; ch < NumChannels; ch++ {
		for cc := 0; cc < NumControls; cc++ {
			lc.value[ch][cc] = Value_Uninitialized
		}
	}
	return lc, nil
}

func FlashUnknown(c Color) Color {
	return c | Color_FlashIfUninitialized
}

func Flash(c Color) Color {
	return c | Color_Flash
}

func (v Value) toFloat() float64 {
	if v == 64 {
		return 0.5
	} else if v < 64 {
		return float64(v) / 128.0
	}
	return float64(v) / 127.0
}

func (c Color) toByte(flashOff bool, v Value) byte {
	if flashOff && c&Color_Flash != 0 {
		return 0
	}
	if flashOff && c&Color_FlashIfUninitialized != 0 {
		if v == Value_Uninitialized {
			return 0
		}
	}
	red := (byte(c) & 0xc) >> 2
	green := byte(c) & 0x3
	return red + green<<4
}

func (l *LaunchControl) SetColor(midiChan int, ctrl Control, color Color) {
	l.lock.Lock()
	l.color[midiChan][ctrl] = color
	l.lock.Unlock()
}

// Run begins listening for updates, blocking the caller until the
// context is canceled.
func (l *LaunchControl) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)

	defer cancel()
	ch := make(chan []portmidi.Event, ReadBufferDepth)

	for i := 0; i < NumChannels; i++ {
		if err := l.Reset(i); err != nil {
			log.Fatal("Error in reset", err)
		}
		// The first swap enables double buffering.
		l.SwapBuffers(i)
	}

	go func() {

		// i := 0
		// for {
		// 	i = (i + NumLEDs + 1) % NumLEDs
		// 	l.color[0][i] = 0xf
		// 	l.SwapBuffers(0)
		// 	l.color[0][i] = 0
		// }

		// for {
		// 	for j := 0; j < NumLEDs; j++ {
		// 		l.color[0][j] = Color(l.value[0][0] / 8)
		// 	}
		// 	l.SwapBuffers(0)
		// 	time.Sleep(50 * time.Millisecond)
		// }

		// for j := 0; j < 16; j++ {
		// 	l.color[0][Control_Button_TrackFocus[0]+Control(j)] = Color(j)
		// }
		// l.SwapBuffers(0)

		l.SetColor(0, Control_Button_TrackFocus[0], Color_BrightRed)
		l.SetColor(0, Control_Button_TrackFocus[1], Flash(Color_BrightYellow))
		l.SetColor(0, Control_Button_TrackFocus[2], Flash(Color_BrightOrange))
		l.SetColor(0, Control_Button_TrackFocus[3], Color_BrightGreen)

		l.SetColor(0, Control_Button_TrackControl[0], Color_DimRed)
		l.SetColor(0, Control_Button_TrackControl[1], Flash(Color_DimYellow))
		l.SetColor(0, Control_Button_TrackControl[2], Flash(Color_DimOrange))
		l.SetColor(0, Control_Button_TrackControl[3], Color_DimGreen)

		l.SetColor(0, Control_Knob_SendA[0], Flash(Color_BrightOrange))
		l.SetColor(0, Control_Knob_SendA[1], FlashUnknown(Color_BrightOrange))

		l.SwapBuffers(0)

	}()
	go func() {
		for {
			// return when canceled
			select {
			case <-ctx.Done():
				return
			default:
			}
			// TODO: Is there a portmidi or libusb function that lets us poll?
			time.Sleep(PollingPeriod)

			evts, err := l.inputStream.Read(MaxEventsPerPoll)
			if err != nil {
				fmt.Println("MIDI error", err)
				cancel()
				return
			}
			if len(evts) != 0 {
				ch <- evts
			}
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evts := <-ch:
				for _, evt := range evts {
					l.event(evt)
				}
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(FlashPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.flash()
			}
		}
	}()

	select {
	case <-ctx.Done():
	}
}

func (l *LaunchControl) getControl(evt portmidi.Event) Control {
	switch evt.Status & MIDI_Status_Code_Mask {
	case MIDI_Status_Control_Change:
		return l.getControlChangeIndex(Value(evt.Data1))
	case MIDI_Status_Note_On, MIDI_Status_Note_Off:
		return l.getNoteChangeIndex(Value(evt.Data1))
	default:
		return Control_Invalid
	}
}

func (l *LaunchControl) event(evt portmidi.Event) {
	if len(evt.SysEx) != 0 {
		l.sysexEvent(evt)
		return
	}

	midiChannel := Value(evt.Status & MIDI_Channel_Mask)
	control := l.getControl(evt)
	if control == Control_Invalid {
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

	if bytes.Compare(sb[1:7], []byte{0x0, 0x20, 0x29, 0x2, 0x11, 0x77}) != 0 {
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

func (l *LaunchControl) setPixels(midiChan int, colors []Color) error {
	flashing := l.flashes%2 == 1

	data := make([]byte, 0, 60)
	data = append(data, 0xf0, 0x00, 0x20, 0x29, 0x02, 0x11, 0x78, byte(midiChan))
	for i := 0; i < 48; i++ {
		data = append(data, byte(i), colors[i].toByte(flashing, l.value[midiChan][i]))
	}
	data = append(data, 0xf7)
	return l.outputStream.WriteSysExBytes(portmidi.Time(), data)
}

func (l *LaunchControl) Reset(midiChan int) error {
	return l.outputStream.WriteShort(0xb0+int64(midiChan), 0x00, 0x00)
}

func (l *LaunchControl) flash() {
	l.lock.Lock()
	l.flashes++
	l.lock.Unlock()

	// TODO have a "current" channel, stop passing "midiChan" (?).
	l.SwapBuffers(0)
}

func (l *LaunchControl) SwapBuffers(midiChan int) error {
	l.lock.Lock()
	swapNum := l.swaps[midiChan]
	l.swaps[midiChan]++
	l.setPixels(midiChan, l.color[midiChan][:])
	l.lock.Unlock()

	var data Value
	if swapNum%2 == 0 {
		data = 0x21
	} else {
		data = 0x24
	}

	return l.outputStream.WriteShort(0xb0+int64(midiChan), 0, int64(data))
}

func (l *LaunchControl) Close() error {
	l.inputStream.Close()
	l.outputStream.Close()
	return nil
}

// discovers the currently connected LaunchControl device
// as a MIDI device.
func discover() (input portmidi.DeviceID, output portmidi.DeviceID, err error) {
	in := -1
	out := -1
	for i := 0; i < portmidi.CountDevices(); i++ {
		info := portmidi.Info(portmidi.DeviceID(i))
		if info.Name == "Launch Control XL" {
			if info.IsInputAvailable {
				in = i
			}
			if info.IsOutputAvailable {
				out = i
			}
		}
	}
	if in == -1 || out == -1 {
		err = errors.New("launchctl: no launch control xl is connected")
	} else {
		input = portmidi.DeviceID(in)
		output = portmidi.DeviceID(out)
	}
	return
}

func controlRange(from, to Control) (r []Control) {
	for c := from; c < to; c++ {
		r = append(r, c)
	}
	return
}
