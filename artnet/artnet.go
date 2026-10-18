// Package artnet sends DMX over a network as Art-Net 4, and finds the
// Art-Net nodes on it.
//
// An Art-Net node is a box on the network with DMX sockets on it, or a
// light that takes Art-Net itself. Each socket listens to one universe,
// named by a 15-bit port address: a net (7 bits), a sub-net (4 bits) and
// a universe (4 bits).
package artnet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/marrasen/gunim-dmx/dmx"
)

// Port is the UDP port Art-Net uses, for sending and for replies.
const Port = 6454

// The operation codes used here.
const (
	OpPoll      = 0x2000
	OpPollReply = 0x2100
	OpDmx       = 0x5000
)

// protocolVersion is Art-Net 4's.
const protocolVersion = 14

var id = []byte("Art-Net\x00")

// A PortAddress names a universe: net, sub-net and universe in one
// number from 0 to 32767.
type PortAddress uint16

// Address builds a port address from its parts.
func Address(netNo, subNet, universe int) PortAddress {
	return PortAddress((netNo&0x7f)<<8 | (subNet&0x0f)<<4 | universe&0x0f)
}

func (a PortAddress) String() string {
	return strconv.Itoa(int(a>>8)) + ":" + strconv.Itoa(int(a>>4&0x0f)) + ":" + strconv.Itoa(int(a&0x0f))
}

// EncodeDmx builds an ArtDmx packet carrying channels to the universe at
// a, numbered seq. The length is rounded up to even, as the standard
// asks.
func EncodeDmx(a PortAddress, seq byte, channels []byte) []byte {
	n := min(len(channels), dmx.Channels)
	n = max(n+n%2, 2)
	b := make([]byte, 18+n)
	copy(b, id)
	binary.LittleEndian.PutUint16(b[8:], OpDmx)
	binary.BigEndian.PutUint16(b[10:], protocolVersion)
	b[12] = seq
	b[13] = 0 // physical port: not used
	b[14] = byte(a)
	b[15] = byte(a >> 8)
	binary.BigEndian.PutUint16(b[16:], uint16(n))
	copy(b[18:], channels)
	return b
}

// EncodePoll builds an ArtPoll packet, asking every node to reply.
func EncodePoll() []byte {
	b := make([]byte, 14)
	copy(b, id)
	binary.LittleEndian.PutUint16(b[8:], OpPoll)
	binary.BigEndian.PutUint16(b[10:], protocolVersion)
	return b
}

// opCode is a packet's operation code, and false when it is not
// Art-Net at all.
func opCode(b []byte) (uint16, bool) {
	if len(b) < 10 || !bytes.Equal(b[:8], id) {
		return 0, false
	}
	return binary.LittleEndian.Uint16(b[8:]), true
}

// A NodeInfo is one node that answered a poll.
type NodeInfo struct {
	IP        net.IP
	ShortName string
	LongName  string
	// Outputs are the universes its output sockets listen to.
	Outputs []PortAddress
}

// ParsePollReply decodes an ArtPollReply.
func ParsePollReply(b []byte) (NodeInfo, error) {
	if op, ok := opCode(b); !ok || op != OpPollReply || len(b) < 194 {
		return NodeInfo{}, errors.New("artnet: not a poll reply")
	}
	n := NodeInfo{
		IP:        net.IPv4(b[10], b[11], b[12], b[13]),
		ShortName: cString(b[26:44]),
		LongName:  cString(b[44:108]),
	}
	netSw, subSw := int(b[18]), int(b[19])
	ports := min(int(binary.BigEndian.Uint16(b[172:])), 4)
	for i := range ports {
		// Bit 7 of the port type says the socket can output DMX.
		if b[174+i]&0x80 != 0 {
			n.Outputs = append(n.Outputs, Address(netSw, subSw, int(b[190+i])))
		}
	}
	return n, nil
}

func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// Discover polls the network and gathers the nodes that answer within
// wait. Replies come back to the Art-Net port, so it fails when another
// program holds that port.
func Discover(wait time.Duration) ([]NodeInfo, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: Port})
	if err != nil {
		return nil, errors.New("listening for Art-Net replies: " + err.Error())
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.WriteToUDP(EncodePoll(), &net.UDPAddr{IP: net.IPv4bcast, Port: Port}); err != nil {
		return nil, errors.New("polling for Art-Net nodes: " + err.Error())
	}
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	var nodes []NodeInfo
	buf := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}
		if node, err := ParsePollReply(buf[:n]); err == nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// A Node sends a universe to one Art-Net node, or to every node when
// its address is a broadcast one.
type Node struct {
	conn     *net.UDPConn
	universe PortAddress
	name     string
	mu       sync.Mutex
	seq      byte
}

// Kind is what a Node calls itself.
const Kind = "Art-Net"

// Dial makes a Node sending universe to the node at host, an IP
// address such as "192.168.1.40", or "255.255.255.255" to reach every node on
// the network.
func Dial(host string, universe PortAddress) (*Node, error) {
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, errors.New("not a network address: " + host)
	}
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: ip, Port: Port})
	if err != nil {
		return nil, errors.New("reaching " + host + ": " + err.Error())
	}
	return &Node{conn: conn, universe: universe, name: host + ", universe " + universe.String()}, nil
}

// Send puts a universe on the network. Sequence numbers run 1 to 255,
// so a node can put packets that arrive out of order back in order.
func (n *Node) Send(u *dmx.Universe) error {
	n.mu.Lock()
	n.seq++
	if n.seq == 0 {
		n.seq = 1
	}
	seq := n.seq
	n.mu.Unlock()
	_, err := n.conn.Write(EncodeDmx(n.universe, seq, u[:]))
	return err
}

// Info is the node and the universe.
func (n *Node) Info() dmx.Info { return dmx.Info{Kind: Kind, Name: n.name} }

// KeepAlive is how often the universe goes again with nothing changed.
// The standard asks for at least every 4 seconds; some nodes let go of
// a silent source sooner.
func (*Node) KeepAlive() time.Duration { return time.Second }

// Close lets the socket go.
func (n *Node) Close() error { return n.conn.Close() }
