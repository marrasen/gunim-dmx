package artnet

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/marrasen/gunim-dmx/dmx"
)

func TestAPortAddressPacksItsParts(t *testing.T) {
	a := Address(1, 2, 3)
	if a != 0x0123 || a.String() != "1:2:3" {
		t.Errorf("Address(1, 2, 3) = %#x, %s", uint16(a), a)
	}
}

func TestADmxPacketIsLaidOutAsTheStandardSays(t *testing.T) {
	b := EncodeDmx(Address(1, 2, 3), 7, []byte{10, 20, 30})
	if string(b[:8]) != "Art-Net\x00" {
		t.Fatalf("the packet starts %q", b[:8])
	}
	if op, _ := opCode(b); op != OpDmx {
		t.Errorf("op code %#x", op)
	}
	if b[11] != 14 || b[12] != 7 || b[14] != 0x23 || b[15] != 0x01 {
		t.Errorf("version, sequence or address wrong: % x", b[10:16])
	}
	// Three channels round up to an even four.
	if n := binary.BigEndian.Uint16(b[16:]); n != 4 || len(b) != 22 || b[20] != 30 || b[21] != 0 {
		t.Errorf("length %d, packet % x", n, b)
	}
}

func TestAPollReplyNamesItsOutputs(t *testing.T) {
	b := make([]byte, 239)
	copy(b, id)
	binary.LittleEndian.PutUint16(b[8:], OpPollReply)
	copy(b[10:], []byte{192, 168, 1, 40})
	b[18], b[19] = 0, 1
	copy(b[26:], "Node")
	copy(b[44:], "A node with two outputs")
	binary.BigEndian.PutUint16(b[172:], 2)
	b[174], b[175] = 0x80, 0x80
	b[190], b[191] = 0, 1
	n, err := ParsePollReply(b)
	if err != nil {
		t.Fatal(err)
	}
	if !n.IP.Equal(net.IPv4(192, 168, 1, 40)) || n.ShortName != "Node" || len(n.Outputs) != 2 ||
		n.Outputs[1] != Address(0, 1, 1) {
		t.Errorf("the reply read as %+v", n)
	}
}

// TestANodeSendsTheUniverse: a packet reaches a listener on this
// machine, and the sequence moves on.
func TestANodeSendsTheUniverse(t *testing.T) {
	l, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: Port})
	if err != nil {
		t.Skip("the Art-Net port is taken: ", err)
	}
	defer func() { _ = l.Close() }()
	n, err := Dial("127.0.0.1", Address(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = n.Close() }()
	u := dmx.Universe{0: 255, 511: 1}
	if err := n.Send(&u); err != nil {
		t.Fatal(err)
	}
	_ = l.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	got, _, err := l.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != 18+512 || buf[12] != 1 || buf[14] != 1 || buf[18] != 255 || buf[18+511] != 1 {
		t.Errorf("received %d bytes: % x", got, buf[:20])
	}
}
