// Package dmx keeps a universe of DMX channels and sends it to a device
// that drives the lights: a USB box, or a node on the network.
//
// A DMX universe is 512 numbered channels, each a value from 0 to 255.
// A light listens to a run of them from its start address. The Engine
// here holds one universe, holds it all down for a blackout, and sends
// it to whichever Device is attached, no faster than the wire takes it.
// The packages beside this one are the devices: enttec for USB boxes,
// artnet and sacn for the network.
package dmx

import (
	"errors"
	"math"
	"time"
)

// Channels is the size of a universe.
const Channels = 512

// A Universe is the value of every channel, channel 1 first.
type Universe [Channels]byte

// Rate is how many frames a second the engine sends at most. A full
// frame of 512 channels takes about 22.7 ms on the cable, so about 44
// is all the cable carries; 40 leaves room.
const Rate = 40

// A Device takes universes to the lights.
//
// The engine calls Send from one goroutine at a time, at most Rate
// times a second, and again every KeepAlive while nothing changes.
type Device interface {
	// Send puts u on the wire.
	Send(u *Universe) error
	// Info is what is known about the device now.
	Info() Info
	// Close lets the device go. The engine darkens the lights first.
	Close() error
}

// A Watched device tells the engine when what it knows about itself
// changes, or when it goes away underneath it, as a USB box pulled out.
type Watched interface {
	Device
	// Watch hands the device two functions. It calls changed when its
	// Info changes, and lost with the reason when it can no longer
	// send. Either may be called from any goroutine.
	Watch(changed func(), lost func(error))
}

// A Refresher is a device whose receivers forget a source that falls
// silent, so the engine sends again every KeepAlive even when nothing
// changed. A device that repeats the last frame by itself, as an
// ENTTEC DMX USB Pro does, needs none.
type Refresher interface {
	KeepAlive() time.Duration
}

// Info is what is known about a device.
type Info struct {
	// Kind names the sort of device, such as "ENTTEC DMX USB Pro" or
	// "Art-Net".
	Kind string
	// Name is where it is: a serial port, or a network address and
	// universe.
	Name string
	// Firmware and Serial are empty until known.
	Firmware string
	Serial   string
	// Detail is anything else worth showing, such as the output rate.
	Detail string
}

// Label is the device as the interface shows it.
func (i Info) Label() string {
	if i.Name == "" {
		return i.Kind
	}
	return i.Kind + " on " + i.Name
}

// ErrNoDevice says there was nothing to connect to.
var ErrNoDevice = errors.New("dmx: no device found")

// Clamp8 clamps a value to a channel value, 0 to 255.
func Clamp8(v int) byte { return byte(min(255, max(0, v))) }

// Clamp8f rounds and clamps a computed value to a channel value.
func Clamp8f(v float64) byte {
	if math.IsNaN(v) {
		return 0
	}
	return Clamp8(int(math.Round(v)))
}

// Level is a level from 0 to 1 as a channel value, so 1 reaches 255.
func Level(v float64) byte { return Clamp8f(v * 255) }

// MidiToDmx stretches a seven-bit MIDI value over the channel's range,
// so 127 reaches 255.
func MidiToDmx(v int) byte {
	return Clamp8f(float64(min(127, max(0, v))) * 255 / 127)
}
