package enttec

// A FrameParser decodes the widget's byte stream one chunk at a time.
//
// Feed it whatever the serial port yields; complete frames come back in
// order. Bytes outside a frame, a frame with the wrong end delimiter,
// and one declaring an impossible length are all dropped, and the
// parser picks up again at the next start byte.
type FrameParser struct {
	state  parseState
	label  byte
	length int
	data   []byte
	filled int
}

type parseState int

const (
	atStart parseState = iota
	atLabel
	atLenLSB
	atLenMSB
	atData
	atEnd
)

// Push feeds a chunk and returns every frame it completed, which is
// often none.
func (p *FrameParser) Push(chunk []byte) []Frame {
	var frames []Frame
	for _, b := range chunk {
		switch p.state {
		case atStart:
			if b == StartOfMessage {
				p.state = atLabel
			}
		case atLabel:
			p.label = b
			p.state = atLenLSB
		case atLenLSB:
			p.length = int(b)
			p.state = atLenMSB
		case atLenMSB:
			p.length |= int(b) << 8
			if p.length > MaxDataLength {
				p.Reset()
				break
			}
			p.data = make([]byte, p.length)
			p.filled = 0
			if p.length == 0 {
				p.state = atEnd
			} else {
				p.state = atData
			}
		case atData:
			p.data[p.filled] = b
			p.filled++
			if p.filled == p.length {
				p.state = atEnd
			}
		case atEnd:
			if b == EndOfMessage {
				frames = append(frames, Frame{Label: p.label, Data: p.data})
			}
			p.Reset()
			// A start byte where the end delimiter should be is most
			// likely the next frame beginning, so keep it.
			if b == StartOfMessage {
				p.state = atLabel
			}
		}
	}
	return frames
}

// Reset forgets a part-read frame, as on a fresh connection.
func (p *FrameParser) Reset() {
	p.state = atStart
	p.label = 0
	p.length = 0
	p.data = nil
	p.filled = 0
}
