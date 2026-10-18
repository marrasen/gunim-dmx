package enttec

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/marrasen/gunim-dmx/dmx"
)

func frame(t *testing.T, label byte, data []byte) []byte {
	t.Helper()
	f, err := EncodeFrame(label, data)
	if err != nil {
		t.Fatalf("EncodeFrame: %v", err)
	}
	return f
}

func TestEncodeFrame(t *testing.T) {
	if got, want := frame(t, 3, []byte{0, 0}), []byte{0x7e, 3, 2, 0, 0, 0, 0xe7}; !bytes.Equal(got, want) {
		t.Errorf("EncodeFrame = %x, want %x", got, want)
	}
	if got, want := frame(t, 10, nil), []byte{0x7e, 10, 0, 0, 0xe7}; !bytes.Equal(got, want) {
		t.Errorf("empty payload = %x, want %x", got, want)
	}
}

func TestEncodeFrameSplitsLongLengths(t *testing.T) {
	f := frame(t, LabelSendDmx, make([]byte, 513))
	if f[2] != 0x01 || f[3] != 0x02 { // 513 is 0x0201
		t.Errorf("length bytes = %02x %02x, want 01 02", f[2], f[3])
	}
	if len(f) != 513+5 || f[len(f)-1] != EndOfMessage {
		t.Errorf("frame is %d bytes ending %02x", len(f), f[len(f)-1])
	}
}

func TestEncodeFrameRejectsOversizedPayloads(t *testing.T) {
	if _, err := EncodeFrame(LabelSendDmx, make([]byte, 601)); !errors.Is(err, ErrTooLong) {
		t.Errorf("601 bytes gave %v, want ErrTooLong", err)
	}
}

func TestEncodeSendDmx(t *testing.T) {
	f, err := EncodeSendDmx([]byte{255, 128})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x7e, LabelSendDmx, 25, 0, 0 /* start code */, 255, 128, 0 /* padding */}
	if !bytes.Equal(f[:len(want)], want) {
		t.Errorf("head = %x, want %x", f[:len(want)], want)
	}
	if len(f) != 25+5 {
		t.Errorf("frame is %d bytes, want %d", len(f), 25+5)
	}
}

func TestEncodeSendDmxFullUniverse(t *testing.T) {
	universe := bytes.Repeat([]byte{7}, dmx.Channels)
	f, err := EncodeSendDmx(universe)
	if err != nil {
		t.Fatal(err)
	}
	if got := int(f[2]) | int(f[3])<<8; got != 513 {
		t.Errorf("payload length = %d, want 513", got)
	}
	if f[4] != 0 || f[4+512] != 7 {
		t.Errorf("start code %d, last channel %d", f[4], f[4+512])
	}
}

func TestEncodeSendDmxRejectsTooManyChannels(t *testing.T) {
	if _, err := EncodeSendDmx(make([]byte, 513)); !errors.Is(err, ErrTooLong) {
		t.Errorf("513 channels gave %v, want ErrTooLong", err)
	}
}

func TestRequests(t *testing.T) {
	get := func(size int) []byte {
		f, err := EncodeGetParameters(size)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	if got, want := get(0), []byte{0x7e, 3, 2, 0, 0, 0, 0xe7}; !bytes.Equal(got, want) {
		t.Errorf("get parameters = %x, want %x", got, want)
	}
	if got, want := get(0x0102), []byte{0x7e, 3, 2, 0, 2, 1, 0xe7}; !bytes.Equal(got, want) {
		t.Errorf("get parameters with user config = %x, want %x", got, want)
	}
	serial, err := EncodeGetSerialNumber()
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x7e, 10, 0, 0, 0xe7}; !bytes.Equal(serial, want) {
		t.Errorf("get serial = %x, want %x", serial, want)
	}
	for _, c := range []struct {
		on   bool
		want []byte
	}{{true, []byte{0x7e, 8, 1, 0, 1, 0xe7}}, {false, []byte{0x7e, 8, 1, 0, 0, 0xe7}}} {
		got, err := EncodeReceiveDmxOnChange(c.on)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, c.want) {
			t.Errorf("receive on change %v = %x, want %x", c.on, got, c.want)
		}
	}
}

func TestEncodeSetParametersClamps(t *testing.T) {
	f, err := EncodeSetParameters(Parameters{BreakTime: 1, MabTime: 0, OutputRate: 99})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x7e, 4, 5, 0, 0, 0, 9, 1, 40, 0xe7}
	if !bytes.Equal(f, want) {
		t.Errorf("set parameters = %x, want %x", f, want)
	}
}

func TestParseParameters(t *testing.T) {
	p, err := ParseParameters([]byte{44, 1, 9, 1, 40, 0xaa})
	if err != nil {
		t.Fatal(err)
	}
	if p.FirmwareMajor != 1 || p.FirmwareMinor != 44 {
		t.Errorf("firmware = %d.%d, want 1.44", p.FirmwareMajor, p.FirmwareMinor)
	}
	if p.BreakTime != 9 || p.MabTime != 1 || p.OutputRate != 40 {
		t.Errorf("timing = %+v", p)
	}
	if !bytes.Equal(p.UserConfig, []byte{0xaa}) {
		t.Errorf("user config = %x", p.UserConfig)
	}
}

func TestParseSerialNumber(t *testing.T) {
	cases := []struct {
		in   []byte
		want string
	}{
		{[]byte{0x78, 0x56, 0x34, 0x12}, "12345678"},
		{[]byte{0x01, 0x00, 0x00, 0x00}, "1"},
		{[]byte{0xff, 0xff, 0xff, 0xff}, ""}, // unprogrammed
	}
	for _, c := range cases {
		got, err := ParseSerialNumber(c.in)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("ParseSerialNumber(%x) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseReceivedDmx(t *testing.T) {
	ok, err := ParseReceivedDmx([]byte{0, 0, 10, 20})
	if err != nil {
		t.Fatal(err)
	}
	if !ok.Valid || ok.StartCode != 0 || !bytes.Equal(ok.Values, []byte{10, 20}) {
		t.Errorf("good packet = %+v", ok)
	}
	bad, err := ParseReceivedDmx([]byte{0b11, 0})
	if err != nil {
		t.Fatal(err)
	}
	if bad.Valid || !bad.QueueOverflow || !bad.Overrun {
		t.Errorf("flagged packet = %+v", bad)
	}
}

func TestApplyChangeOfState(t *testing.T) {
	received := make([]byte, 513)
	// Start byte 1 means index 8. Bits 0 and 9 set pick 8 and 17.
	packet := []byte{1, 0b00000001, 0b00000010, 0, 0, 0, 100, 200}
	if err := ApplyChangeOfState(received, packet); err != nil {
		t.Fatal(err)
	}
	if received[8] != 100 || received[17] != 200 || received[9] != 0 {
		t.Errorf("received[8,9,17] = %d,%d,%d, want 100,0,200", received[8], received[9], received[17])
	}
}

func TestFrameParserOneChunk(t *testing.T) {
	var p FrameParser
	frames := p.Push(frame(t, 10, []byte{0x78, 0x56, 0x34, 0x12}))
	if len(frames) != 1 || frames[0].Label != 10 {
		t.Fatalf("got %d frames: %+v", len(frames), frames)
	}
	if !bytes.Equal(frames[0].Data, []byte{0x78, 0x56, 0x34, 0x12}) {
		t.Errorf("data = %x", frames[0].Data)
	}
}

func TestFrameParserSplitChunks(t *testing.T) {
	var p FrameParser
	bs := frame(t, 3, []byte{44, 1, 9, 1, 40})
	if got := p.Push(bs[:3]); len(got) != 0 {
		t.Fatalf("a part frame produced %d frames", len(got))
	}
	if got := p.Push(bs[3:7]); len(got) != 0 {
		t.Fatalf("a part frame produced %d frames", len(got))
	}
	frames := p.Push(bs[7:])
	if len(frames) != 1 {
		t.Fatalf("got %d frames", len(frames))
	}
	params, err := ParseParameters(frames[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	if params.OutputRate != 40 {
		t.Errorf("output rate = %d, want 40", params.OutputRate)
	}
}

func TestFrameParserSeveralFramesAndGarbage(t *testing.T) {
	var p FrameParser
	chunk := slices.Concat([]byte{0x00, 0x42},
		frame(t, 10, []byte{1, 2, 3, 4}), frame(t, 3, []byte{44, 1, 9, 1, 40}))
	labels := make([]byte, 0, 2)
	for _, f := range p.Push(chunk) {
		labels = append(labels, f.Label)
	}
	if !bytes.Equal(labels, []byte{10, 3}) {
		t.Errorf("labels = %v, want [10 3]", labels)
	}
}

func TestFrameParserDropsBadEndDelimiter(t *testing.T) {
	var p FrameParser
	bad := frame(t, 10, []byte{1, 2, 3, 4})
	bad[len(bad)-1] = 0x00
	if got := p.Push(bad); len(got) != 0 {
		t.Fatalf("a frame with no end delimiter produced %d frames", len(got))
	}
	serial, err := EncodeGetSerialNumber()
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Push(serial); len(got) != 1 {
		t.Errorf("the parser did not pick up again: %d frames", len(got))
	}
}

func TestFrameParserTakesStartByteInTheEndSlot(t *testing.T) {
	var p FrameParser
	truncated := frame(t, 10, []byte{1, 2, 3, 4})[:8] // no end delimiter
	serial, err := EncodeGetSerialNumber()
	if err != nil {
		t.Fatal(err)
	}
	frames := p.Push(slices.Concat(truncated, serial))
	if len(frames) != 1 || frames[0].Label != 10 || len(frames[0].Data) != 0 {
		t.Errorf("got %+v, want one empty frame labelled 10", frames)
	}
}

func TestFrameParserDropsImpossibleLengths(t *testing.T) {
	var p FrameParser
	if got := p.Push([]byte{0x7e, 6, 0xff, 0xff}); len(got) != 0 {
		t.Fatalf("an impossible length produced %d frames", len(got))
	}
	serial, err := EncodeGetSerialNumber()
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Push(serial); len(got) != 1 {
		t.Errorf("the parser did not pick up again: %d frames", len(got))
	}
}
