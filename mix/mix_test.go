package mix

import (
	"testing"

	"github.com/marrasen/gunim-dmx/dmx"
)

// TestTheStrongestWinsAndUnsetChannelsRest: two layers on one channel
// give the higher; a channel nobody sets rests at home.
func TestTheStrongestWinsAndUnsetChannelsRest(t *testing.T) {
	m := NewMixer()
	m.SetHome(dmx.Universe{0: 10, 1: 20})
	a, b := NewLayer(), NewLayer()
	m.Add(a)
	m.Add(b)
	a.Set(1, 100)
	b.Set(1, 50)
	b.Set(3, 0)
	u := m.Merge(nil)
	if u[0] != 100 || u[1] != 20 || u[2] != 0 {
		t.Errorf("merged %d, %d, %d; want 100, 20, 0", u[0], u[1], u[2])
	}
	// A layer setting less than home still sets it.
	a.Release(1)
	b.Set(1, 5)
	if u := m.Merge(nil); u[0] != 5 {
		t.Errorf("a channel set to 5 went out at %d", u[0])
	}
}

// TestTheLatestWinsWhereAsked: an LTP channel follows whoever moved it
// last, even down.
func TestTheLatestWinsWhereAsked(t *testing.T) {
	m := NewMixer()
	m.SetRule(4, LTP)
	a, b := NewLayer(), NewLayer()
	m.Add(a)
	m.Add(b)
	a.Set(4, 200)
	b.Set(4, 30)
	if u := m.Merge(nil); u[3] != 30 {
		t.Errorf("the latest set 30, and %d went out", u[3])
	}
	a.Set(4, 180)
	if u := m.Merge(nil); u[3] != 180 {
		t.Errorf("the latest set 180, and %d went out", u[3])
	}
	// Setting the same value again does not count as moving it.
	b.Set(4, 30)
	if u := m.Merge(nil); u[3] != 180 {
		t.Errorf("an unchanged value took the channel: %d", u[3])
	}
}

// TestAMutedLayerIsNotHeardAndTheMasterDims: muting keeps what the
// layer sets; the grand master scales only the dimming channels.
func TestAMutedLayerIsNotHeardAndTheMasterDims(t *testing.T) {
	m := NewMixer()
	l := NewLayer()
	m.Add(l)
	l.Set(1, 200)
	l.Set(2, 200)
	l.Mute(true)
	if u := m.Merge(nil); u[0] != 0 {
		t.Errorf("a muted layer was heard: %d", u[0])
	}
	l.Mute(false)
	m.SetMaster(0.5)
	dims := [dmx.Channels]bool{0: true}
	if u := m.Merge(&dims); u[0] != 100 || u[1] != 200 {
		t.Errorf("at half master: %d and %d, want 100 and 200", u[0], u[1])
	}
	m.Remove(l)
	if u := m.Merge(nil); u[0] != 0 {
		t.Error("a removed layer was heard")
	}
	if v, ok := l.Get(1); !ok || v != 200 {
		t.Errorf("the layer reads back %d, %v", v, ok)
	}
}
