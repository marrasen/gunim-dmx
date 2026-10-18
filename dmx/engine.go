package dmx

import (
	"sync"
	"time"
)

// A Status says where the connection to the device stands.
type Status string

// The connection states.
const (
	Disconnected Status = "disconnected"
	Connecting   Status = "connecting"
	Connected    Status = "connected"
	Failed       Status = "error"
)

// A State is the engine's connection as the interface shows it.
type State struct {
	Status Status
	Info   Info
	// Err is a human-readable problem: a failed connection, an unplug.
	Err string
	// Blackout is true while the lights are being held down.
	Blackout bool
}

// An Engine keeps one universe and sends it to a device.
//
// Channel writes are gathered up, and the whole universe goes out on
// the next tick, Rate times a second at most. A device whose receivers
// forget a silent source, as Art-Net and sACN nodes do, is sent the
// universe again every so often even when nothing changed.
//
// The universe outlives the device: unplugging and plugging in again
// puts back the look that was showing.
type Engine struct {
	mu       sync.Mutex
	universe Universe
	// blackout holds the lights down: the universe above keeps tracking
	// what it is told, and zeros go out until it is let go.
	blackout bool
	dev      Device
	state    State
	dirty    bool
	// generation rises on every attach and detach, so a device that
	// has gone cannot report anything about the one that replaced it.
	generation int
	// stop and done end and wait for the send loop of the device
	// attached now.
	stop, done chan struct{}

	onState    func(State)
	onUniverse []func(Universe)
}

// NewEngine makes an engine with no device.
func NewEngine() *Engine {
	return &Engine{state: State{Status: Disconnected}}
}

// OnState sets the one function called whenever the connection
// changes, and calls it at once with the state as it stands.
func (e *Engine) OnState(fn func(State)) {
	e.mu.Lock()
	e.onState = fn
	state := e.state
	e.mu.Unlock()
	fn(state)
}

// OnUniverse adds a function called with what goes out whenever a
// channel value changes, so a fader bank can follow it.
func (e *Engine) OnUniverse(fn func(Universe)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onUniverse = append(e.onUniverse, fn)
}

// State is the connection as it stands.
func (e *Engine) State() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// Universe is what goes out, which is nothing at all while the lights
// are held down: the light that is really on, not what was asked for.
func (e *Engine) Universe() Universe {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.outgoingLocked()
}

// Connect opens a device and attaches it, saying it is connecting while
// open runs. A failure leaves the engine with no device, in the Failed
// state.
func (e *Engine) Connect(open func() (Device, error)) error {
	e.mu.Lock()
	e.setStateLocked(State{Status: Connecting})
	e.mu.Unlock()
	d, err := open()
	if err != nil {
		e.mu.Lock()
		e.setStateLocked(State{Status: Failed, Err: err.Error()})
		e.mu.Unlock()
		return err
	}
	e.Attach(d)
	return nil
}

// Attach makes d the device the universe goes to, letting go of any
// before it. The universe as it stands goes out at once.
func (e *Engine) Attach(d Device) {
	e.Disconnect()
	e.mu.Lock()
	e.dev = d
	e.generation++
	gen := e.generation
	e.dirty = true
	e.stop, e.done = make(chan struct{}), make(chan struct{})
	stop, done := e.stop, e.done
	e.setStateLocked(State{Status: Connected, Info: d.Info()})
	e.mu.Unlock()

	if w, ok := d.(Watched); ok {
		w.Watch(func() { e.changed(gen) }, func(err error) { e.lost(gen, err) })
	}
	var keepAlive time.Duration
	if r, ok := d.(Refresher); ok {
		keepAlive = r.KeepAlive()
	}
	go e.run(d, gen, keepAlive, stop, done)
}

// Disconnect lets the device go, darkening the lights on the way: a
// box that repeats its last frame would keep them lit long after the
// app had gone. The universe is kept, so connecting again restores the
// look.
func (e *Engine) Disconnect() {
	e.shut(State{Status: Disconnected}, -1)
}

// Close shuts the engine down for good.
func (e *Engine) Close() { e.Disconnect() }

// SetChannel sets one channel, 1 to 512, to a value.
func (e *Engine) SetChannel(channel, value int) {
	if channel < 1 || channel > Channels {
		return
	}
	v := Clamp8(value)
	e.mu.Lock()
	if e.universe[channel-1] == v {
		e.mu.Unlock()
		return
	}
	e.universe[channel-1] = v
	e.changedLocked()
}

// SetUniverse sets every channel at once.
func (e *Engine) SetUniverse(u Universe) {
	e.mu.Lock()
	if e.universe == u {
		e.mu.Unlock()
		return
	}
	e.universe = u
	e.changedLocked()
}

// SetBlackout holds every channel at nothing, or lets go again.
//
// It latches rather than writing zeros once. Whatever drives the
// universe carries on while it is on, and the universe underneath keeps
// tracking it, but nothing reaches the lights except zeros. Letting go
// puts them back where they were, in one frame. Writing zeros once is
// no use as a panic button: the next change puts the lights up again.
func (e *Engine) SetBlackout(on bool) {
	e.mu.Lock()
	if e.blackout == on {
		e.mu.Unlock()
		return
	}
	e.blackout = on
	state := e.state
	e.setStateLocked(state)
	e.changedLocked()
}

// Blacked reports whether the lights are being held down.
func (e *Engine) Blacked() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.blackout
}

// changedLocked marks the universe to go out, lets go of e.mu, and
// tells the listeners.
func (e *Engine) changedLocked() {
	e.dirty = true
	snapshot, listeners := e.outgoingLocked(), e.onUniverse
	e.mu.Unlock()
	for _, fn := range listeners {
		fn(snapshot)
	}
}

// outgoingLocked is what actually goes out. The caller holds e.mu.
func (e *Engine) outgoingLocked() Universe {
	if e.blackout {
		return Universe{}
	}
	return e.universe
}

func (e *Engine) setStateLocked(s State) {
	// Whether the lights are held down is the engine's to say, not the
	// caller's: a reconnect builds a state afresh and would drop it.
	s.Blackout = e.blackout
	e.state = s
	if fn := e.onState; fn != nil {
		// The listener runs outside the lock, so it may call back in.
		// It is handed the state as it stands when it runs, so two
		// changes close together cannot reach it in the wrong order.
		go func() { fn(e.State()) }()
	}
}

// run sends the universe to d until stop closes, on every tick that
// has a change, and every keepAlive without one.
func (e *Engine) run(d Device, gen int, keepAlive time.Duration, stop, done chan struct{}) {
	defer close(done)
	t := time.NewTicker(time.Second / Rate)
	defer t.Stop()
	var last time.Time
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			e.mu.Lock()
			send := e.dirty || keepAlive > 0 && now.Sub(last) >= keepAlive
			e.dirty = false
			u := e.outgoingLocked()
			e.mu.Unlock()
			if !send {
				continue
			}
			last = now
			if err := d.Send(&u); err != nil {
				go e.lost(gen, err)
				return
			}
		}
	}
}

// changed takes up what the device of generation gen knows about itself
// now.
func (e *Engine) changed(gen int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if gen != e.generation || e.dev == nil {
		return
	}
	s := e.state
	s.Info = e.dev.Info()
	e.setStateLocked(s)
}

// lost says the device of generation gen went away underneath us: an
// unplug, a driver error, a failed send.
func (e *Engine) lost(gen int, err error) {
	msg := "The connection to the lights was lost."
	if err != nil {
		msg = err.Error()
	}
	e.shut(State{Status: Disconnected, Err: msg}, gen)
}

// shut drops the device and reports the state that replaces it. With
// gen at -1 it drops whatever is attached; otherwise only the device of
// that generation.
func (e *Engine) shut(next State, gen int) {
	e.mu.Lock()
	if gen >= 0 && gen != e.generation {
		e.mu.Unlock()
		return
	}
	d, stop, done := e.dev, e.stop, e.done
	e.dev, e.stop, e.done = nil, nil, nil
	e.generation++
	if d != nil || gen < 0 {
		e.setStateLocked(next)
	}
	e.mu.Unlock()
	if d == nil {
		return
	}
	close(stop)
	<-done
	// The send loop has finished, so nothing else is writing: darken the
	// lights, and say nothing of a failure, as the device is going anyway.
	_ = d.Send(&Universe{})
	_ = d.Close()
}
