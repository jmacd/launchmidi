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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rakyll/portmidi"
)

type (
	// LaunchControl represents a device with an input and output MIDI stream.
	LaunchControl struct {
		inputStream  *portmidi.Stream
		outputStream *portmidi.Stream

		lock           sync.Mutex
		flashes        int64
		currentChannel int
		errorChan      chan error

		swaps [NumChannels]int64              // number of swaps
		color [NumChannels][NumControls]Color // colors [0-15] + 2 bits
		value [NumChannels][NumControls]Value // control values [0-127] or uninitialized

		// calls have an additional entry representing AllChannels.
		calls [NumChannels + 1][NumControls][]Callback
	}

	// Value is the value of any of the Control variables, in the range 0-127.
	// The special value `ValueUninitialized` (128) is used to represent the
	// initially unknown state of the Control.  Additional color bits
	// `ColorFlash` (and `ColorFlashIfUninitialized`) support flashing (when
	// uninitialized).
	Value uint8

	// Color is 4 bits of color information.
	Color byte

	// Control indexes are assigned in the range [0, NumControls).
	// They are ordered such that the control number equals the
	// LED number for the first 48 controls.
	Control int

	// Callback is called when Control values change.  Register with SetCallback.
	Callback func(midiChan int, control Control, value Value)
)

const (
	DeviceName = "Launch Control XL"

	NumChannels = 16 // a.k.a "templates", 8 user and 8 factory settings
	NumControls = 56 // (3+1+2)*8 rows of knobs, sliders, buttons + 2*4 side buttons
	NumLEDs     = 48 // all except the sliders

	ValueUninitialized Value = 128

	AllChannels = NumChannels // Use with AddCallback.

	FlashPeriod      = 433 * time.Millisecond
	MaxEventsPerPoll = 1024
	PollingPeriod    = 10 * time.Millisecond
	ReadBufferDepth  = 16
)

var (
	ErrNoLaunchControl = fmt.Errorf("launchctl: no launch control xl is connected")
)

// toFloat ensures that the xl's knob indents at Value 64 return 0.5.
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
		for cc := Control(0); cc < NumControls; cc++ {
			v0 := ValueUninitialized
			if cc.IsButton() {
				v0 = 0
			}
			lc.value[ch][cc] = v0
		}
	}
	return lc, nil
}

func (l *LaunchControl) AddCallback(midiChan int, control Control, callback Callback) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.calls[midiChan][control] = append(l.calls[midiChan][control], callback)
}

// Run begins listening for updates, blocking the caller until the
// context is canceled.
func (l *LaunchControl) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	defer cancel()
	ch := make(chan []portmidi.Event, ReadBufferDepth)
	wg := sync.WaitGroup{}
	wg.Add(3) // event reader, event processor, flasher routines

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
				l.lock.Lock()
				l.flashes++
				ch := l.currentChannel
				l.lock.Unlock()

				_ = l.SwapBuffers(ch)
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

func (l *LaunchControl) isFlashing(midiChan int, ctrl Control) bool {
	return l.color[midiChan][ctrl].isFlashing(l.value[midiChan][ctrl])
}

func (l *LaunchControl) computeFlash(midiChan int, ctrl Control) bool {
	if l.isFlashing(midiChan, ctrl) {
		return true
	}

	switch {
	case ctrl >= ControlButtonTrackFocus[0] && ctrl <= ControlButtonTrackFocus[7]:
		// Track focus buttons are adjacent to one slider each.
		return l.isFlashing(midiChan, ControlSlider[ctrl-ControlButtonTrackFocus[0]])
	case ctrl >= ControlButtonDevice && ctrl <= ControlButtonRecord:
		// Device-Record buttons are adjacent to all sliders.
		for i := 0; i < 8; i++ {
			if l.isFlashing(midiChan, ControlSlider[i]) {
				return true
			}
		}
	}
	return false
}

func (l *LaunchControl) setPixels(midiChan int, colors []Color) error {
	flashingOff := l.flashes%2 == 1

	data := make([]byte, 0, 60)
	data = append(data, 0xf0, 0x00, 0x20, 0x29, 0x02, 0x11, 0x78, byte(midiChan))
	for i := 0; i < 48; i++ {
		flasher := l.computeFlash(midiChan, Control(i))
		show := colors[i].toByte(
			flashingOff,
			flasher,
			l.value[midiChan][i],
		)

		data = append(data, byte(i), show)
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
	l.currentChannel = midiChan
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
