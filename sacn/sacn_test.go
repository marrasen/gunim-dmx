package sacn

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/marrasen/gunim-dmx/dmx"
)

func TestADataPacketIsLaidOutAsTheStandardSays(t *testing.T) {
	cid := NewCID()
	u := dmx.Universe{0: 1, 511: 2}
	b := Packet{CID: cid, Source: "crossroads", Priority: 100, Sequence: 9, Universe: 258}.Encode(&u)
	if len(b) != 638 {
		t.Fatalf("the packet is %d bytes, want 638", len(b))
	}
	if string(b[4:13]) != "ASC-E1.17" || !bytes.Equal(b[22:38], cid[:]) || string(b[44:54]) != "crossroads" {
		t.Error("the root or framing layer is wrong")
	}
	for _, c := range []struct {
		at   int
		want uint16
	}{{16, 0x7000 | 622}, {38, 0x7000 | 600}, {115, 0x7000 | 523}, {113, 258}, {123, 513}} {
		if got := binary.BigEndian.Uint16(b[c.at:]); got != c.want {
			t.Errorf("at %d: %#x, want %#x", c.at, got, c.want)
		}
	}
	if b[108] != 100 || b[111] != 9 || b[117] != 2 || b[118] != 0xa1 || b[125] != 0 || b[126] != 1 || b[637] != 2 {
		t.Error("priority, sequence, DMP header or channels are wrong")
	}
	if a := MulticastAddr(258); !a.IP.Equal(net.IPv4(239, 255, 1, 2)) || a.Port != 5568 {
		t.Errorf("universe 258 goes to %v", a)
	}
}

func TestAUniverseOutOfRangeIsRefused(t *testing.T) {
	if _, err := Dial(0, "x"); err == nil {
		t.Error("universe 0 was taken")
	}
}

// TestASourceSendsToOneReceiver: a packet reaches a listener on this
// machine.
func TestASourceSendsToOneReceiver(t *testing.T) {
	l, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: Port})
	if err != nil {
		t.Skip("the sACN port is taken: ", err)
	}
	defer func() { _ = l.Close() }()
	s, err := Dial(1, "test", WithHost("127.0.0.1"), WithPriority(150))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	u := dmx.Universe{4: 77}
	if err := s.Send(&u); err != nil {
		t.Fatal(err)
	}
	_ = l.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, _, err := l.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != 638 || buf[108] != 150 || buf[130] != 77 {
		t.Errorf("received %d bytes, priority %d, channel 5 at %d", n, buf[108], buf[130])
	}
}
