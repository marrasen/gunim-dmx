package enttec

import (
	"errors"
	"io"
	"strconv"
	"sync"

	"go.bug.st/serial"

	"github.com/marrasen/gunim-dmx/dmx"
)

// A Pro is an ENTTEC DMX USB Pro, or a box that speaks its protocol.
//
// The box repeats the last frame on the cable at its own rate, so the
// engine only sends when something changed.
//
// Only one request other than Send DMX and Get Parameters may be made,
// and only before output starts: by the API document, any other message
// flips the box's port back to input. So the serial number is asked for
// once, as the port opens.
type Pro struct {
	// writing serialises the bytes going out, so two frames never
	// interleave on the port.
	writing sync.Mutex
	port    serial.Port

	mu      sync.Mutex
	info    dmx.Info
	parser  FrameParser
	closed  bool
	changed func()
	lost    func(error)
}

// ProKind is what a Pro calls itself.
const ProKind = "ENTTEC DMX USB Pro"

// OpenPro opens the box on the serial port named port.
func OpenPro(port string) (*Pro, error) {
	sp, err := openSerial(port, &serial.Mode{BaudRate: BaudRate})
	if err != nil {
		return nil, errors.New("Opening " + port + ": " + err.Error())
	}
	p := &Pro{port: sp, info: dmx.Info{Kind: ProKind, Name: port}}
	go p.read()
	// The identity questions come first, before any DMX goes out.
	if err := p.write(EncodeGetParameters(0)); err != nil {
		_ = sp.Close()
		return nil, err
	}
	if err := p.write(EncodeGetSerialNumber()); err != nil {
		_ = sp.Close()
		return nil, err
	}
	return p, nil
}

// FindPro opens the port that looks most like a Pro.
func FindPro() (*Pro, error) {
	ports, err := ListPorts()
	if err != nil {
		return nil, err
	}
	p, ok := PickPort(ports)
	if !ok {
		return nil, errors.New("no ENTTEC box found among the serial ports")
	}
	return OpenPro(p.Name)
}

// Send puts a universe on the cable.
func (p *Pro) Send(u *dmx.Universe) error {
	return p.write(EncodeSendDmx(u[:]))
}

// Info is what is known about the box now. Its firmware and serial
// number arrive a moment after it opens.
func (p *Pro) Info() dmx.Info {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.info
}

// Watch is told when the box's firmware and serial number arrive, and
// when it is pulled out.
func (p *Pro) Watch(changed func(), lost func(error)) {
	p.mu.Lock()
	p.changed, p.lost = changed, lost
	p.mu.Unlock()
	// A reply may have come before anyone was watching.
	changed()
}

// Close lets the port go.
func (p *Pro) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	return p.port.Close()
}

func (p *Pro) write(frame []byte, err error) error {
	if err != nil {
		return err
	}
	p.writing.Lock()
	defer p.writing.Unlock()
	if _, err := p.port.Write(frame); err != nil {
		return errors.New("Writing to the DMX box failed: " + err.Error())
	}
	return nil
}

// read pumps the box's replies until the port goes away.
func (p *Pro) read() {
	buf := make([]byte, 512)
	for {
		n, err := p.port.Read(buf)
		p.mu.Lock()
		closed, lost := p.closed, p.lost
		p.mu.Unlock()
		if closed {
			return
		}
		if err != nil {
			if lost != nil {
				if errors.Is(err, io.EOF) {
					lost(errors.New("the DMX box was unplugged"))
				} else {
					lost(errors.New("the DMX box's connection was lost"))
				}
			}
			return
		}
		if n == 0 {
			continue
		}
		p.mu.Lock()
		frames := p.parser.Push(buf[:n])
		p.mu.Unlock()
		for _, f := range frames {
			p.handle(f)
		}
	}
}

// handle folds a reply into what is known about the box. Labels 5 and
// 9, received DMX, are ignored: the port is driven as output only.
func (p *Pro) handle(f Frame) {
	p.mu.Lock()
	switch f.Label {
	case LabelGetParameters:
		par, err := ParseParameters(f.Data)
		if err != nil {
			p.mu.Unlock()
			return
		}
		p.info.Firmware = strconv.Itoa(par.FirmwareMajor) + "." + strconv.Itoa(par.FirmwareMinor)
		rate := "as fast as it can"
		if par.OutputRate > 0 {
			rate = strconv.Itoa(par.OutputRate) + " frames a second"
		}
		p.info.Detail = "Sends " + rate
	case LabelGetSerialNumber:
		number, err := ParseSerialNumber(f.Data)
		if err != nil {
			p.mu.Unlock()
			return
		}
		p.info.Serial = number
	default:
		p.mu.Unlock()
		return
	}
	changed := p.changed
	p.mu.Unlock()
	if changed != nil {
		changed()
	}
}
