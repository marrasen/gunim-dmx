package enttec

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"go.bug.st/serial"

	"github.com/marrasen/gunim-dmx/dmx"
)

// fakePort is a box with no hardware: it answers the identity questions
// with replies queued up front, records what is written, and fails
// every write once told to.
type fakePort struct {
	serial.Port
	mu      sync.Mutex
	written bytes.Buffer
	replies chan []byte
	fail    bool
	closed  chan struct{}
	once    sync.Once
}

func newFakePort(replies ...[]byte) *fakePort {
	p := &fakePort{replies: make(chan []byte, len(replies)), closed: make(chan struct{})}
	for _, r := range replies {
		p.replies <- r
	}
	return p
}

func (p *fakePort) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fail {
		return 0, errors.New("the device has gone")
	}
	p.written.Write(b)
	return len(b), nil
}

func (p *fakePort) Read(b []byte) (int, error) {
	select {
	case r := <-p.replies:
		return copy(b, r), nil
	case <-p.closed:
		return 0, errors.New("closed")
	}
}

func (p *fakePort) Close() error {
	p.once.Do(func() { close(p.closed) })
	return nil
}

func (p *fakePort) Break(time.Duration) error { return nil }

func usePort(t *testing.T, p serial.Port) {
	t.Helper()
	was := openSerial
	openSerial = func(string, *serial.Mode) (serial.Port, error) { return p, nil }
	t.Cleanup(func() { openSerial = was })
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting: " + what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestAProSaysWhatItIsAndCarriesTheUniverse: the replies to the
// identity questions reach the engine's state, and a channel set goes
// out in a Send DMX frame.
func TestAProSaysWhatItIsAndCarriesTheUniverse(t *testing.T) {
	params, _ := EncodeFrame(LabelGetParameters, []byte{44, 1, 9, 1, 40})
	serialNo, _ := EncodeFrame(LabelGetSerialNumber, []byte{0x78, 0x56, 0x34, 0x12})
	port := newFakePort(params, serialNo)
	usePort(t, port)

	e := dmx.NewEngine()
	if err := e.Connect(func() (dmx.Device, error) { return OpenPro("/dev/ttyUSB0") }); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the serial number", func() bool { return e.State().Info.Serial == "12345678" })
	if s := e.State(); s.Info.Firmware != "1.44" || s.Info.Kind != ProKind {
		t.Errorf("the state is %+v", s.Info)
	}

	e.SetChannel(1, 200)
	u := dmx.Universe{0: 200}
	want, _ := EncodeSendDmx(u[:])
	waitFor(t, "the frame", func() bool {
		port.mu.Lock()
		defer port.mu.Unlock()
		return bytes.Contains(port.written.Bytes(), want)
	})
	e.Close()
}

// TestAProPulledOutDoesNotLockTheEngine: a failed write loses the box,
// and losing it darkens the lights on the way out, which writes too.
func TestAProPulledOutDoesNotLockTheEngine(t *testing.T) {
	port := newFakePort()
	usePort(t, port)
	e := dmx.NewEngine()
	if err := e.Connect(func() (dmx.Device, error) { return OpenPro("/dev/ttyUSB0") }); err != nil {
		t.Fatal(err)
	}
	port.mu.Lock()
	port.fail = true
	port.mu.Unlock()
	done := make(chan struct{})
	go func() {
		e.SetChannel(1, 255)
		waitFor(t, "the loss", func() bool { return e.State().Status == dmx.Disconnected })
		e.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a failed write locked the engine")
	}
}

// TestAnOpenDMXSendsFramesOfItsOwn: with no change at all, frames keep
// going out, each the start code and 512 channels.
func TestAnOpenDMXSendsFramesOfItsOwn(t *testing.T) {
	port := newFakePort()
	usePort(t, port)
	o, err := OpenOpenDMX("/dev/ttyUSB0")
	if err != nil {
		t.Fatal(err)
	}
	u := dmx.Universe{0: 9}
	_ = o.Send(&u)
	waitFor(t, "a few frames", func() bool {
		port.mu.Lock()
		defer port.mu.Unlock()
		return port.written.Len() >= 3*(dmx.Channels+1)
	})
	if err := o.Close(); err != nil {
		t.Fatal(err)
	}
	port.mu.Lock()
	b := port.written.Bytes()
	port.mu.Unlock()
	if b[len(b)-dmx.Channels-1] != StartCode || b[len(b)-dmx.Channels] != 9 {
		t.Error("the last frame is not the start code and the universe")
	}
}
