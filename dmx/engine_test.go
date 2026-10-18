package dmx

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeDevice records what it was sent, and fails every send once told
// to.
type fakeDevice struct {
	mu     sync.Mutex
	sent   []Universe
	fail   bool
	closed bool
	keep   time.Duration
}

func (d *fakeDevice) Send(u *Universe) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fail {
		return errors.New("the device has gone")
	}
	d.sent = append(d.sent, *u)
	return nil
}

func (d *fakeDevice) Info() Info { return Info{Kind: "Fake", Name: "here"} }

func (d *fakeDevice) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	return nil
}

func (d *fakeDevice) last() (u Universe, n int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.sent) == 0 {
		return Universe{}, 0
	}
	return d.sent[len(d.sent)-1], len(d.sent)
}

// refreshing is a fake device whose receivers forget a silent source.
type refreshing struct{ fakeDevice }

func (d *refreshing) KeepAlive() time.Duration { return d.keep }

// waitFor polls until ok holds, or fails the test.
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

// TestBlackoutHoldsTheLightsDown: it latches, the universe underneath
// keeps tracking what it is told, and letting go puts the lights back.
func TestBlackoutHoldsTheLightsDown(t *testing.T) {
	e := NewEngine()
	var mu sync.Mutex
	var last Universe
	e.OnUniverse(func(u Universe) { mu.Lock(); last = u; mu.Unlock() })
	get := func() Universe { mu.Lock(); defer mu.Unlock(); return last }

	e.SetChannel(1, 255)
	if get()[0] != 255 {
		t.Fatalf("channel 1 went out at %d, want 255", get()[0])
	}
	e.SetBlackout(true)
	if !e.Blacked() || !e.State().Blackout {
		t.Error("the lights are not held down")
	}
	if get() != (Universe{}) {
		t.Error("something still went out with the lights held down")
	}
	e.SetChannel(2, 200)
	if get() != (Universe{}) {
		t.Error("a channel written while held down went out")
	}
	e.SetBlackout(false)
	if u := get(); u[0] != 255 || u[1] != 200 {
		t.Errorf("letting go gave %d and %d, want 255 and 200", u[0], u[1])
	}
}

// TestTheUniverseGoesOutAndComesBackAfterAReconnect: what was set goes
// to the device, closing darkens it, and a new device gets the look.
func TestTheUniverseGoesOutAndComesBackAfterAReconnect(t *testing.T) {
	e := NewEngine()
	e.SetChannel(3, 128)
	d := &fakeDevice{}
	e.Attach(d)
	waitFor(t, "the first frame", func() bool { u, n := d.last(); return n > 0 && u[2] == 128 })
	if s := e.State(); s.Status != Connected || s.Info.Kind != "Fake" {
		t.Errorf("the state is %+v", s)
	}

	e.Disconnect()
	if u, _ := d.last(); u != (Universe{}) || !d.closed {
		t.Error("letting the device go did not darken and close it")
	}

	d2 := &fakeDevice{}
	e.Attach(d2)
	waitFor(t, "the look again", func() bool { u, n := d2.last(); return n > 0 && u[2] == 128 })
	e.Close()
}

// TestNothingChangedSendsNothingUnlessTheDeviceForgets: a device that
// repeats its last frame is not sent it again, and one whose receivers
// forget a silent source is.
func TestNothingChangedSendsNothingUnlessTheDeviceForgets(t *testing.T) {
	e := NewEngine()
	d := &fakeDevice{}
	e.Attach(d)
	waitFor(t, "the first frame", func() bool { _, n := d.last(); return n > 0 })
	time.Sleep(150 * time.Millisecond)
	if _, n := d.last(); n != 1 {
		t.Errorf("%d frames went out with nothing changing, want 1", n)
	}
	e.Close()

	r := &refreshing{fakeDevice{keep: 50 * time.Millisecond}}
	e.Attach(r)
	waitFor(t, "frames kept alive", func() bool { _, n := r.last(); return n >= 3 })
	e.Close()
}

// TestAFailedSendLosesTheDevice: the state says so, the blackout
// survives it, and the engine is not locked up.
func TestAFailedSendLosesTheDevice(t *testing.T) {
	e := NewEngine()
	e.SetBlackout(true)
	d := &fakeDevice{fail: true}
	e.Attach(d)
	waitFor(t, "the loss", func() bool { return e.State().Status == Disconnected })
	if s := e.State(); s.Err == "" || !s.Blackout {
		t.Errorf("after the loss the state is %+v", s)
	}
	e.SetChannel(1, 1)
	e.Close()
}

// TestAFailedOpenSaysWhy: the state carries the reason.
func TestAFailedOpenSaysWhy(t *testing.T) {
	e := NewEngine()
	err := e.Connect(func() (Device, error) { return nil, ErrNoDevice })
	if !errors.Is(err, ErrNoDevice) {
		t.Fatalf("Connect said %v", err)
	}
	if s := e.State(); s.Status != Failed || s.Err == "" {
		t.Errorf("the state is %+v", s)
	}
}
