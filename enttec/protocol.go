// Package enttec drives ENTTEC's USB DMX boxes: the DMX USB Pro, with
// the boxes that copy its protocol (the Eurolite USB-DMX512 PRO and the
// DMXking ultraDMX), and the plain Open DMX USB.
//
// The Pro has its own processor that makes the DMX timing; the computer
// sends it framed messages over a virtual COM port. The Open DMX is a
// bare FTDI chip, so the computer makes the timing itself.
package enttec

import (
	"errors"
	"fmt"
	"strings"

	"github.com/marrasen/gunim-dmx/dmx"
)

// The framing bytes. Every message on the wire is
//
//	0x7E · label · length LSB · length MSB · data[length] · 0xE7
//
// per the DMX USB Pro API document, version 1.44. The widget is an FTDI
// virtual COM port.
const (
	StartOfMessage = 0x7e
	EndOfMessage   = 0xe7
)

// MaxDataLength is the largest payload the framing allows.
const MaxDataLength = 600

// What goes before the channels, and the shortest payload the widget
// takes.
const (
	StartCode    = 0x00
	MinSendBytes = 25 // the widget rejects a shorter Send DMX payload
)

// The message labels of the API document.
const (
	LabelReprogramFirmware  = 1
	LabelProgramFlashPage   = 2
	LabelGetParameters      = 3
	LabelSetParameters      = 4
	LabelReceivedDmx        = 5
	LabelSendDmx            = 6
	LabelSendRdm            = 7
	LabelReceiveDmxOnChange = 8
	LabelDmxChangeOfState   = 9
	LabelGetSerialNumber    = 10
	LabelSendRdmDiscovery   = 11
)

// BaudRate is the rate the widget's virtual COM port is opened at. It
// is a dummy — it does not set the USB speed — but 57600 is what
// everyone uses.
const BaudRate = 57600

// FTDIVendorID is FTDI's USB vendor id. The Pro is an FT245 device.
const FTDIVendorID = 0x0403

// A Frame is one message from the widget: its label and its payload.
type Frame struct {
	Label byte
	Data  []byte
}

// ErrTooLong says a payload is longer than the framing allows.
var ErrTooLong = errors.New("enttec: payload too long")

// EncodeFrame wraps data in the framing for label.
func EncodeFrame(label byte, data []byte) ([]byte, error) {
	if len(data) > MaxDataLength {
		return nil, fmt.Errorf("%w: %d > %d", ErrTooLong, len(data), MaxDataLength)
	}
	out := make([]byte, len(data)+5)
	out[0] = StartOfMessage
	out[1] = label
	out[2] = byte(len(data))
	out[3] = byte(len(data) >> 8)
	copy(out[4:], data)
	out[len(out)-1] = EndOfMessage
	return out, nil
}

// EncodeSendDmx builds an Output Only Send DMX Packet, label 6.
//
// channels holds up to 512 values for channels 1 upwards; the start
// code goes in front. The widget keeps re-sending this packet at its
// own output rate, so the host only sends again when a value changes.
// A short payload is padded to the widget's 25-byte minimum.
func EncodeSendDmx(channels []byte) ([]byte, error) {
	if len(channels) > dmx.Channels {
		return nil, fmt.Errorf("%w: %d channels > %d", ErrTooLong, len(channels), dmx.Channels)
	}
	data := make([]byte, max(len(channels)+1, MinSendBytes))
	data[0] = StartCode
	copy(data[1:], channels)
	return EncodeFrame(LabelSendDmx, data)
}

// EncodeGetParameters builds a Get Widget Parameters request, label 3,
// asking for userConfigSize further bytes.
func EncodeGetParameters(userConfigSize int) ([]byte, error) {
	return EncodeFrame(LabelGetParameters, []byte{byte(userConfigSize), byte(userConfigSize >> 8)})
}

// EncodeGetSerialNumber builds a Get Widget Serial Number request,
// label 10, which carries no data.
func EncodeGetSerialNumber() ([]byte, error) {
	return EncodeFrame(LabelGetSerialNumber, nil)
}

// EncodeReceiveDmxOnChange builds a Receive DMX On Change request,
// label 8: send always, or only when something changed.
func EncodeReceiveDmxOnChange(onChangeOnly bool) ([]byte, error) {
	b := byte(0)
	if onChangeOnly {
		b = 1
	}
	return EncodeFrame(LabelReceiveDmxOnChange, []byte{b})
}

// Parameters are the widget's timing and output settings, as labels 3
// and 4 exchange them.
type Parameters struct {
	// FirmwareMajor and FirmwareMinor spell the version, so 1 and 44
	// are firmware 1.44. The major byte is also the variety: 1 for DMX,
	// 2 for RDM, 3 for a sniffer.
	FirmwareMajor, FirmwareMinor int
	// BreakTime is the output break in units of 10.67 µs, 9 to 127.
	BreakTime int
	// MabTime is the mark after break, in the same units, 1 to 127.
	MabTime int
	// OutputRate is packets per second, 1 to 40, or 0 for as fast as
	// the widget can.
	OutputRate int
	// UserConfig is whatever user bytes followed the fixed fields.
	UserConfig []byte
}

// EncodeSetParameters builds a Set Widget Parameters request, label 4.
// The settings survive a power cut. Values outside the document's
// ranges are clamped.
func EncodeSetParameters(p Parameters) ([]byte, error) {
	data := make([]byte, 5+len(p.UserConfig))
	data[0] = byte(len(p.UserConfig))
	data[1] = byte(len(p.UserConfig) >> 8)
	data[2] = byte(clamp(p.BreakTime, 9, 127))
	data[3] = byte(clamp(p.MabTime, 1, 127))
	data[4] = byte(clamp(p.OutputRate, 0, 40))
	copy(data[5:], p.UserConfig)
	return EncodeFrame(LabelSetParameters, data)
}

// ParseParameters decodes a Get Widget Parameters reply, label 3.
func ParseParameters(data []byte) (Parameters, error) {
	if len(data) < 5 {
		return Parameters{}, errors.New("enttec: widget parameters reply too short")
	}
	return Parameters{
		FirmwareMinor: int(data[0]),
		FirmwareMajor: int(data[1]),
		BreakTime:     int(data[2]),
		MabTime:       int(data[3]),
		OutputRate:    int(data[4]),
		UserConfig:    append([]byte(nil), data[5:]...),
	}, nil
}

// ParseSerialNumber decodes a Get Widget Serial Number reply, label 10:
// four BCD bytes, least significant first, spelling the digits printed
// on the case. An unprogrammed widget reports 0xFFFFFFFF, which comes
// back as the empty string.
func ParseSerialNumber(data []byte) (string, error) {
	if len(data) < 4 {
		return "", errors.New("enttec: serial number reply too short")
	}
	if data[0] == 0xff && data[1] == 0xff && data[2] == 0xff && data[3] == 0xff {
		return "", nil
	}
	var b strings.Builder
	for i := 3; i >= 0; i-- {
		fmt.Fprintf(&b, "%x%x", data[i]>>4, data[i]&0x0f)
	}
	// Leading zeros are not printed on the case either, but keep the
	// last digit so an all-zero serial does not vanish.
	s := strings.TrimLeft(b.String(), "0")
	if s == "" {
		return "0", nil
	}
	return s, nil
}

// ReceivedDmx is a decoded Received DMX Packet, label 5.
type ReceivedDmx struct {
	// Valid is true when neither the overflow nor the overrun bit is set.
	Valid         bool
	QueueOverflow bool
	Overrun       bool
	StartCode     byte
	// Values holds channels 1 upwards; the start code is not in it.
	Values []byte
}

// ParseReceivedDmx decodes a Received DMX Packet payload, label 5.
func ParseReceivedDmx(data []byte) (ReceivedDmx, error) {
	if len(data) < 2 {
		return ReceivedDmx{}, errors.New("enttec: received packet too short")
	}
	st := data[0]
	return ReceivedDmx{
		Valid:         st == 0,
		QueueOverflow: st&0x01 != 0,
		Overrun:       st&0x02 != 0,
		StartCode:     data[1],
		Values:        append([]byte(nil), data[2:]...),
	}, nil
}

// ApplyChangeOfState folds a Received DMX Change Of State packet, label
// 9, into a 513-byte receive buffer holding the start code and then
// channels 1 to 512, by the algorithm of the API document.
func ApplyChangeOfState(received, data []byte) error {
	if len(data) < 6 {
		return errors.New("enttec: change-of-state packet too short")
	}
	start := int(data[0])
	changed := 6
	for bit := range 40 {
		if data[1+bit>>3]>>(bit&7)&1 == 0 {
			continue
		}
		if index := start*8 + bit; index < len(received) && changed < len(data) {
			received[index] = data[changed]
		}
		changed++
	}
	return nil
}

func clamp(v, lo, hi int) int { return min(hi, max(lo, v)) }
