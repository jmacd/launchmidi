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
	"fmt"
	"sync"
	"time"

	"github.com/rakyll/portmidi"
)

const (
	DeviceName = "Launch Control XL"

	MIDIStatusNoteOff       = 0x80
	MIDIStatusNoteOn        = 0x90
	MIDIStatusControlChange = 0xb0
	MIDIStatusCodeMask      = 0xf0
	MIDIChannelMask         = 0x0f

	MaxEventsPerPoll = 1024
	ReadBufferDepth  = 16
	PollingPeriod    = 10 * time.Millisecond
	NumChannels      = 16
	NumControls      = 6*8 + 4 + 4
	NumLEDs          = 48

	FlashPeriod = 433 * time.Millisecond

	ValueUninitialized Value = 128

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

	ErrNoLaunchControl = fmt.Errorf("launchctl: no launch control xl is connected")
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

	lock           sync.Mutex
	flashes        int64
	currentChannel int
	errorChan      chan error

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
	if inStream, err = portmidi.NewInputStream(input, MaxEventsPerPoll); err != nil {
		return nil, err
	}
	if outStream, err = portmidi.NewOutputStream(output, MaxEventsPerPoll, 0); err != nil {
		return nil, err
	}
	lc := &LaunchControl{
		inputStream:  inStream,
		outputStream: outStream,
		errorChan:    make(chan error, 1),
	}
	for ch := 0; ch < NumChannels; ch++ {
		for cc := 0; cc < NumControls; cc++ {
			lc.value[ch][cc] = ValueUninitialized
		}
	}
	return lc, nil
}

// Run begins listening for updates, blocking the caller until the
// context is canceled.
func (l *LaunchControl) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	defer cancel()
	ch := make(chan []portmidi.Event, ReadBufferDepth)
	wg := sync.WaitGroup{}
	wg.Add(3)

	for i := 0; i < NumChannels; i++ {
		if err := l.Reset(i); err != nil {
			return err
		}
		// The first swap enables double buffering.
		if err := l.SwapBuffers(i); err != nil {
			return err
		}
	}

	if err := l.SetTemplate(0); err != nil {
		return err
	}

	go l.tester() // @@@ remove me

	go func() {
		defer wg.Done()
		for {
			// Return when canceled
			select {
			case <-ctx.Done():
				return
			default:
			}
			// Note: Is there a portmidi or libusb function that blocks
			// while polling?
			time.Sleep(PollingPeriod)

			evts, err := l.inputStream.Read(MaxEventsPerPoll)
			if err != nil {
				_ = l.handleError(err)
				return
			}
			if len(evts) != 0 {
				ch <- evts
			}
		}
	}()
	go func() {
		defer wg.Done()
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
		defer wg.Done()

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
		return ctx.Err()
	case err := <-l.errorChan:
		return err
	}
}

func (l *LaunchControl) SetColor(midiChan int, ctrl Control, color Color) {
	l.lock.Lock()
	l.color[midiChan][ctrl] = color
	l.lock.Unlock()
}

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

func (l *LaunchControl) flash() {
	l.lock.Lock()
	l.flashes++
	ch := l.currentChannel
	l.lock.Unlock()

	_ = l.SwapBuffers(ch)
}

func (l *LaunchControl) setPixels(midiChan int, colors []Color) error {
	flashing := l.flashes%2 == 1

	data := make([]byte, 0, 60)
	data = append(data, 0xf0, 0x00, 0x20, 0x29, 0x02, 0x11, 0x78, byte(midiChan))
	for i := 0; i < 48; i++ {
		data = append(data, byte(i), colors[i].toByte(flashing, l.value[midiChan][i]))
	}
	data = append(data, 0xf7)
	if err := l.outputStream.WriteSysExBytes(portmidi.Time(), data); err != nil {
		return l.handleError(fmt.Errorf("midi: write sysex: %w", err))
	}
	return nil
}

func (l *LaunchControl) Reset(midiChan int) error {
	if err := l.outputStream.WriteShort(0xb0+int64(midiChan), 0x00, 0x00); err != nil {
		return l.handleError(fmt.Errorf("midi: reset: %w", err))
	}
	return nil
}

func (l *LaunchControl) SwapBuffers(midiChan int) error {
	l.lock.Lock()
	swapNum := l.swaps[midiChan]
	l.swaps[midiChan]++
	err := l.setPixels(midiChan, l.color[midiChan][:])
	l.lock.Unlock()

	if err != nil {
		return err
	}

	var data Value
	if swapNum%2 == 0 {
		data = 0x21
	} else {
		data = 0x24
	}

	if err := l.outputStream.WriteShort(0xb0+int64(midiChan), 0, int64(data)); err != nil {
		return l.handleError(fmt.Errorf("midi: swap buffers: %w", err))
	}
	return nil
}

func (l *LaunchControl) SetTemplate(midiChan int) error {
	data := []byte{0xf0, 0x00, 0x20, 0x29, 0x02, 0x11, 0x77, byte(midiChan), 0xf7}
	if err := l.outputStream.WriteSysExBytes(portmidi.Time(), data); err != nil {
		return l.handleError(fmt.Errorf("midi: set template: %w", err))
	}
	return nil
}

func (l *LaunchControl) Close() error {
	err1 := l.inputStream.Close()
	err2 := l.outputStream.Close()

	if err1 != nil {
		return l.handleError(fmt.Errorf("midi: close streams: %w", err1))
	}
	if err2 != nil {
		return l.handleError(fmt.Errorf("midi: close streams: %w", err2))
	}
	return nil
}

func (l *LaunchControl) handleError(err error) error {
	if err == nil {
		return err
	}
	select {
	case l.errorChan <- err:
	default:
	}
	return err
}

// discovers the currently connected LaunchControl device
// as a MIDI device.
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

func controlRange(from, to Control) (r []Control) {
	for c := from; c < to; c++ {
		r = append(r, c)
	}
	return
}

func (l *LaunchControl) tester() {
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
	// 	l.color[0][ControlButtonTrackFocus[0]+Control(j)] = Color(j)
	// }
	// l.SwapBuffers(0)

	l.SetColor(0, ControlButtonTrackFocus[0], ColorBrightRed)
	l.SetColor(0, ControlButtonTrackFocus[1], Flash(ColorBrightYellow))
	l.SetColor(0, ControlButtonTrackFocus[2], Flash(ColorBrightOrange))
	l.SetColor(0, ControlButtonTrackFocus[3], ColorBrightGreen)

	l.SetColor(0, ControlButtonTrackControl[0], ColorDimRed)
	l.SetColor(0, ControlButtonTrackControl[1], Flash(ColorDimYellow))
	l.SetColor(0, ControlButtonTrackControl[2], Flash(ColorDimOrange))
	l.SetColor(0, ControlButtonTrackControl[3], ColorDimGreen)

	l.SetColor(0, ControlKnobSendA[0], Flash(ColorBrightOrange))
	l.SetColor(0, ControlKnobSendA[1], FlashUnknown(ColorBrightOrange))

	_ = l.SwapBuffers(0)
}

func FlashUnknown(c Color) Color {
	return c | ColorFlashIfUninitialized
}

func Flash(c Color) Color {
	return c | ColorFlash
}

func (v Value) toFloat() float64 {
	// The XL knobs have an indent at 64, so make it 0.5
	if v == 64 {
		return 0.5
	} else if v < 64 {
		return float64(v) / 128.0
	}
	return 0.5 + float64(v-64)/63.0
}

func (c Color) toByte(flashOff bool, v Value) byte {
	if flashOff && c&ColorFlash != 0 {
		return 0
	}
	if flashOff && c&ColorFlashIfUninitialized != 0 {
		if v == ValueUninitialized {
			return 0
		}
	}
	red := (byte(c) & 0xc) >> 2
	green := byte(c) & 0x3
	return red + green<<4
}
