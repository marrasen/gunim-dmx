// Package mix merges every source of light into the one universe that
// goes out.
//
// Each source, such as an effect, a cue list or a fader bank, writes
// into a Layer of its own. The Mixer merges the layers channel by
// channel. Brightness goes to the strongest (HTP, highest takes
// precedence), as on any lighting desk, so two sources raising the same
// light never pull it down. A channel can instead go to whichever layer
// set it last (LTP, latest takes precedence), which suits colour mixes
// and positions. A channel no layer sets rests at its home value.
//
// Layers are written from any goroutine, such as an audio thread, with
// no locks: each channel is one atomic.
package mix

import (
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/marrasen/gunim-dmx/dmx"
)

// A Rule says how a channel set by several layers is merged.
type Rule uint8

// The rules.
const (
	// HTP gives the channel the highest value any layer sets.
	HTP Rule = iota
	// LTP gives it the value of the layer that set it last.
	LTP
)

// clock orders every write, for LTP.
var clock atomic.Uint64

// A Layer is one source's say over the universe.
type Layer struct {
	// out holds each channel the layer sets, its value plus one; 0 for
	// one it leaves alone.
	out [dmx.Channels]atomic.Uint32
	// at is when each channel was last set, on clock.
	at [dmx.Channels]atomic.Uint64
	// off says the layer is silent for now, as a group whose song is
	// not the one playing.
	off atomic.Bool
}

// NewLayer makes a layer that sets nothing yet.
func NewLayer() *Layer { return &Layer{} }

// Set sets channel, 1 to 512, to v.
func (l *Layer) Set(channel int, v byte) {
	if channel < 1 || channel > dmx.Channels {
		return
	}
	i := channel - 1
	if l.out[i].Swap(uint32(v)+1) != uint32(v)+1 {
		l.at[i].Store(clock.Add(1))
	}
}

// Release leaves channel to the other layers again.
func (l *Layer) Release(channel int) {
	if channel >= 1 && channel <= dmx.Channels {
		l.out[channel-1].Store(0)
	}
}

// Clear releases every channel.
func (l *Layer) Clear() {
	for i := range l.out {
		l.out[i].Store(0)
	}
}

// Get is what the layer sets channel to, and false when it leaves it
// alone.
func (l *Layer) Get(channel int) (byte, bool) {
	if channel < 1 || channel > dmx.Channels {
		return 0, false
	}
	v := l.out[channel-1].Load()
	return byte(v - 1), v != 0
}

// Mute silences the layer, or lets it be heard again, keeping what it
// sets.
func (l *Layer) Mute(on bool) { l.off.Store(on) }

// Muted reports whether the layer is silent.
func (l *Layer) Muted() bool { return l.off.Load() }

// A Mixer merges layers into a universe.
type Mixer struct {
	mu     sync.Mutex
	layers []*Layer
	home   dmx.Universe
	rules  [dmx.Channels]Rule
	master float64
}

// NewMixer makes a mixer with no layers, every channel resting at 0 and
// merged HTP, and the grand master at full.
func NewMixer() *Mixer { return &Mixer{master: 1} }

// Add has the mixer hear l.
func (m *Mixer) Add(l *Layer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !slices.Contains(m.layers, l) {
		m.layers = append(m.layers, l)
	}
}

// Remove stops the mixer hearing l.
func (m *Mixer) Remove(l *Layer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.layers = slices.DeleteFunc(m.layers, func(x *Layer) bool { return x == l })
}

// SetHome sets where every channel rests when no layer sets it.
func (m *Mixer) SetHome(u dmx.Universe) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.home = u
}

// SetRule sets how channel, 1 to 512, is merged.
func (m *Mixer) SetRule(channel int, r Rule) {
	if channel < 1 || channel > dmx.Channels {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[channel-1] = r
}

// SetMaster sets the grand master, 0 to 1, which scales the channels
// that dim: those in dims. Others, such as a motor or a strobe, carry
// meanings in their values that scaling would change.
func (m *Mixer) SetMaster(level float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.master = min(1, max(0, level))
}

// Merge is the universe the layers make now. dims says which channels
// the grand master scales; nil scales none.
func (m *Mixer) Merge(dims *[dmx.Channels]bool) dmx.Universe {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.home
	var set [dmx.Channels]bool
	var latest [dmx.Channels]uint64
	for _, l := range m.layers {
		if l.off.Load() {
			continue
		}
		for i := range l.out {
			v := l.out[i].Load()
			if v == 0 {
				continue
			}
			b := byte(v - 1)
			switch m.rules[i] {
			case LTP:
				if at := l.at[i].Load(); !set[i] || at > latest[i] {
					u[i], set[i], latest[i] = b, true, at
				}
			default:
				if !set[i] || b > u[i] {
					u[i], set[i] = b, true
				}
			}
		}
	}
	if dims != nil && m.master < 1 {
		for i, d := range dims {
			if d {
				u[i] = dmx.Clamp8f(float64(u[i]) * m.master)
			}
		}
	}
	return u
}

// Drive sends the merge to e, dmx.Rate times a second, until stop is
// called.
func Drive(m *Mixer, e *dmx.Engine, dims *[dmx.Channels]bool) (stop func()) {
	quit, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(time.Second / dmx.Rate)
		defer t.Stop()
		for {
			select {
			case <-quit:
				return
			case <-t.C:
				e.SetUniverse(m.Merge(dims))
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() { close(quit); <-done })
	}
}
