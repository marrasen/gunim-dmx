package enttec

import (
	"errors"
	"runtime"
	"sync"
	"time"

	"go.bug.st/serial"

	"github.com/marrasen/gunim-dmx/dmx"
)

// An OpenDMX is an ENTTEC Open DMX USB: a bare FTDI chip with no
// processor of its own. The computer makes the DMX timing, so it sends
// every frame itself, over and over, on a thread of its own.
//
// Each frame is a break of at least 88 µs, a mark after it, the start
// code and then the 512 channels, at 250,000 baud with two stop bits.
// A break is timed by the system's sleep, which on Windows is coarse,
// so breaks come out long. That is within the standard; a few lights
// flicker on it all the same.
type OpenDMX struct {
	port serial.Port
	name string

	mu    sync.Mutex
	frame [dmx.Channels + 1]byte
	lost  func(error)

	stop chan struct{}
	done chan struct{}
}

// OpenDMXKind is what an OpenDMX calls itself.
const OpenDMXKind = "ENTTEC Open DMX USB"

// openDMXBreak is the break before each frame. The standard asks for
// at least 88 µs, and 92 or more is safer.
const openDMXBreak = 100 * time.Microsecond

// OpenOpenDMX opens the box on the serial port named port, and starts
// sending frames of nothing until the first universe arrives.
func OpenOpenDMX(port string) (*OpenDMX, error) {
	sp, err := openSerial(port, &serial.Mode{
		BaudRate: 250000,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.TwoStopBits,
	})
	if err != nil {
		return nil, errors.New("Opening " + port + ": " + err.Error())
	}
	o := &OpenDMX{port: sp, name: port, stop: make(chan struct{}), done: make(chan struct{})}
	go o.run()
	return o, nil
}

// Send makes u the frame that goes out from now on.
func (o *OpenDMX) Send(u *dmx.Universe) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	copy(o.frame[1:], u[:])
	return nil
}

// Info is the box and its port.
func (o *OpenDMX) Info() dmx.Info { return dmx.Info{Kind: OpenDMXKind, Name: o.name} }

// Watch is told when the box stops taking frames.
func (o *OpenDMX) Watch(_ func(), lost func(error)) {
	o.mu.Lock()
	o.lost = lost
	o.mu.Unlock()
}

// Close stops sending, after one last frame, and lets the port go.
func (o *OpenDMX) Close() error {
	close(o.stop)
	<-o.done
	return o.port.Close()
}

// run sends frames until stopped. It keeps to its own thread, so the
// scheduler moving it between threads does not stretch a frame.
func (o *OpenDMX) run() {
	defer close(o.done)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t := time.NewTicker(time.Second / dmx.Rate)
	defer t.Stop()
	var frame [dmx.Channels + 1]byte
	for {
		o.mu.Lock()
		frame = o.frame
		o.mu.Unlock()
		if err := o.frameOut(frame[:]); err != nil {
			o.mu.Lock()
			lost := o.lost
			o.mu.Unlock()
			if lost != nil {
				lost(errors.New("the Open DMX box stopped taking frames: " + err.Error()))
			}
			return
		}
		select {
		case <-o.stop:
			// The engine's last frame, of nothing, has to reach the
			// cable before the port goes.
			o.mu.Lock()
			frame = o.frame
			o.mu.Unlock()
			_ = o.frameOut(frame[:])
			return
		case <-t.C:
		}
	}
}

func (o *OpenDMX) frameOut(frame []byte) error {
	if err := o.port.Break(openDMXBreak); err != nil {
		return err
	}
	_, err := o.port.Write(frame)
	return err
}
