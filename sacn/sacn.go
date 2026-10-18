// Package sacn sends DMX over a network as sACN, Streaming ACN, the
// standard ANSI E1.31.
//
// Each universe, numbered 1 to 63999, has its own multicast group, so
// every receiver on the network that wants a universe hears it without
// being named. A receiver forgets a source that has been silent for 2.5
// seconds, so a source keeps sending even when nothing changes.
package sacn

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/marrasen/gunim-dmx/dmx"
)

// Port is the UDP port sACN uses.
const Port = 5568

// The universes a source can send to.
const (
	MinUniverse = 1
	MaxUniverse = 63999
)

// DefaultPriority is the priority a source sends at unless told
// otherwise. A receiver hearing two sources follows the higher.
const DefaultPriority = 100

// packetSize is a data packet carrying all 512 channels.
const packetSize = 126 + dmx.Channels

// A CID names a source, the same for as long as it runs.
type CID [16]byte

// NewCID makes a fresh random one.
func NewCID() CID {
	var c CID
	_, _ = rand.Read(c[:])
	c[6] = c[6]&0x0f | 0x40 // a version 4 UUID
	c[8] = c[8]&0x3f | 0x80
	return c
}

// MulticastAddr is the group a universe is sent to.
func MulticastAddr(universe int) *net.UDPAddr {
	return &net.UDPAddr{IP: net.IPv4(239, 255, byte(universe>>8), byte(universe)), Port: Port}
}

// A Packet is the parts of a data packet that change from source to
// source.
type Packet struct {
	CID      CID
	Source   string
	Priority int
	Sequence byte
	Universe int
}

// Encode builds a data packet carrying all 512 channels.
func (p Packet) Encode(channels *dmx.Universe) []byte {
	b := make([]byte, packetSize)
	// The root layer.
	binary.BigEndian.PutUint16(b[0:], 0x0010)
	copy(b[4:], "ASC-E1.17")
	binary.BigEndian.PutUint16(b[16:], 0x7000|uint16(packetSize-16))
	binary.BigEndian.PutUint32(b[18:], 0x00000004)
	copy(b[22:38], p.CID[:])
	// The framing layer.
	binary.BigEndian.PutUint16(b[38:], 0x7000|uint16(packetSize-38))
	binary.BigEndian.PutUint32(b[40:], 0x00000002)
	name := p.Source
	if len(name) > 63 {
		name = name[:63]
	}
	copy(b[44:108], name)
	b[108] = byte(min(200, max(0, p.Priority)))
	b[111] = p.Sequence
	binary.BigEndian.PutUint16(b[113:], uint16(p.Universe))
	// The DMP layer, which carries the start code and the channels.
	binary.BigEndian.PutUint16(b[115:], 0x7000|uint16(packetSize-115))
	b[117] = 0x02
	b[118] = 0xa1
	binary.BigEndian.PutUint16(b[121:], 0x0001)
	binary.BigEndian.PutUint16(b[123:], dmx.Channels+1)
	b[125] = 0 // start code
	copy(b[126:], channels[:])
	return b
}

// A Source sends one universe.
type Source struct {
	conn *net.UDPConn
	name string
	mu   sync.Mutex
	p    Packet
}

// Kind is what a Source calls itself.
const Kind = "sACN"

// An Option changes how a Source sends.
type Option func(*Source)

// WithPriority sets the priority, 0 to 200.
func WithPriority(p int) Option { return func(s *Source) { s.p.Priority = p } }

// WithCID names the source with a CID kept from before, so receivers
// know it as the same source after a restart.
func WithCID(c CID) Option { return func(s *Source) { s.p.CID = c } }

// WithHost sends to one receiver at host rather than to the universe's
// multicast group.
func WithHost(host string) Option { return func(s *Source) { s.name = host } }

// Dial makes a Source sending universe, under the name source, which
// receivers show.
func Dial(universe int, source string, opts ...Option) (*Source, error) {
	if universe < MinUniverse || universe > MaxUniverse {
		return nil, errors.New("an sACN universe is from 1 to 63999, not " + strconv.Itoa(universe))
	}
	s := &Source{p: Packet{CID: NewCID(), Source: source, Priority: DefaultPriority, Universe: universe}}
	for _, o := range opts {
		o(s)
	}
	addr := MulticastAddr(universe)
	if s.name != "" {
		ip := net.ParseIP(s.name)
		if ip == nil {
			return nil, errors.New("not a network address: " + s.name)
		}
		addr = &net.UDPAddr{IP: ip, Port: Port}
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, errors.New("reaching sACN universe " + strconv.Itoa(universe) + ": " + err.Error())
	}
	s.conn = conn
	where := "universe " + strconv.Itoa(universe)
	if s.name != "" {
		where = s.name + ", " + where
	}
	s.name = where
	return s, nil
}

// Send puts a universe on the network.
func (s *Source) Send(u *dmx.Universe) error {
	s.mu.Lock()
	s.p.Sequence++
	b := s.p.Encode(u)
	s.mu.Unlock()
	_, err := s.conn.Write(b)
	return err
}

// Info is the source and its universe.
func (s *Source) Info() dmx.Info { return dmx.Info{Kind: Kind, Name: s.name} }

// KeepAlive is how often the universe goes again with nothing changed,
// well inside the 2.5 seconds a receiver waits before letting go.
func (*Source) KeepAlive() time.Duration { return time.Second }

// Close lets the socket go.
func (s *Source) Close() error { return s.conn.Close() }
