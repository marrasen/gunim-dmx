package enttec

import (
	"fmt"
	"strconv"
	"strings"

	"go.bug.st/serial"
	"go.bug.st/serial/enumerator"
)

// A PortInfo names one serial port that might be a box.
type PortInfo struct {
	// Name is the system's name for the port: /dev/ttyUSB0, COM3.
	Name string
	// VendorID and ProductID are the USB ids, and zero for a port that
	// is not on USB.
	VendorID, ProductID int
	// Product is the USB product string, where the system reports one.
	Product string
	// Serial is the USB serial number, where the system reports one.
	Serial string
}

// IsFTDI reports whether the port looks like an ENTTEC box, which is an
// FTDI device.
func (p PortInfo) IsFTDI() bool { return p.VendorID == FTDIVendorID }

// Label is the port as the interface shows it.
func (p PortInfo) Label() string {
	if p.Product != "" {
		return p.Name + " — " + p.Product
	}
	return p.Name
}

// ListPorts is every serial port the system can see.
//
// On Windows an Open DMX USB shows up only once its FTDI driver has the
// virtual COM port turned on ("Load VCP" in the driver's properties).
func ListPorts() ([]PortInfo, error) {
	details, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("enttec: listing serial ports: %w", err)
	}
	out := make([]PortInfo, 0, len(details))
	for _, d := range details {
		p := PortInfo{Name: d.Name, Product: d.Product, Serial: d.SerialNumber}
		if d.IsUSB {
			p.VendorID = parseHex(d.VID)
			p.ProductID = parseHex(d.PID)
		}
		out = append(out, p)
	}
	return out, nil
}

// PickPort chooses a box's port from those available: an FTDI one if
// it is there, else one whose name or product mentions ENTTEC or DMX.
// It reports false rather than guessing at an unrelated port, since a
// Bluetooth serial port enumerates here too and fails to open.
func PickPort(ports []PortInfo) (PortInfo, bool) {
	for _, p := range ports {
		if p.IsFTDI() {
			return p, true
		}
	}
	for _, p := range ports {
		text := strings.ToLower(p.Name + " " + p.Product)
		if strings.Contains(text, "enttec") || strings.Contains(text, "dmx") {
			return p, true
		}
	}
	return PortInfo{}, false
}

func parseHex(s string) int {
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0
	}
	return int(n)
}

// openSerial opens a port. Tests put a port with no hardware behind it
// in its place.
var openSerial = serial.Open
