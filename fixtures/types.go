// Package fixtures holds the fixture database, the rig the user has
// patched, and the maths that turns a fixture channel into a universe
// slot.
//
// A DMX universe is 512 numbered slots. A fixture listens to a run of
// them starting at its DMX start address, so the slot is the start
// address plus the channel offset, less one. A Definition describes
// that run for one product; a Patch is one unit the user owns, with a
// name and an address.
package fixtures

// A Range is one documented value band of a channel, such as "128-255
// motor speed".
type Range struct {
	// From and To are inclusive, 0 to 255.
	From, To int
	// Label is the manual's wording for what the band does.
	Label string
}

// A Kind says broadly what a channel does. It groups channels in the
// interface, and marks the ones that can seize control from the desk.
type Kind string

// The kinds of channel.
const (
	KindIntensity Kind = "intensity"
	KindColor     Kind = "color"
	KindStrobe    Kind = "strobe"
	KindMotor     Kind = "motor"
	KindProgram   Kind = "program"
	KindOther     Kind = "other"
)

// A Role is the function a channel performs, named the same way across
// every product.
//
// Kind says a channel is colour; Role says it is specifically red. That
// difference is what lets a route target a Group by function — "the
// front wash, red" — and have each member work out its own offset, so
// the same show runs on a rig it was never programmed against.
//
// A fixture declares only the roles it really has. A route asking for
// one a fixture lacks simply skips it.
type Role string

// The roles a channel can have.
const (
	RoleDimmer       Role = "dimmer"
	RoleRed          Role = "red"
	RoleGreen        Role = "green"
	RoleBlue         Role = "blue"
	RoleWhite        Role = "white"
	RoleAmber        Role = "amber"
	RoleUV           Role = "uv"
	RoleLime         Role = "lime"
	RoleCyan         Role = "cyan"
	RoleMagenta      Role = "magenta"
	RoleYellow       Role = "yellow"
	RoleColorWheel   Role = "color-wheel"
	RoleStrobe       Role = "strobe"
	RolePan          Role = "pan"
	RolePanFine      Role = "pan-fine"
	RoleTilt         Role = "tilt"
	RoleTiltFine     Role = "tilt-fine"
	RoleZoom         Role = "zoom"
	RoleFocus        Role = "focus"
	RoleIris         Role = "iris"
	RoleGobo         Role = "gobo"
	RoleGoboRotation Role = "gobo-rotation"
	RolePrism        Role = "prism"
	RoleSpeed        Role = "speed"
	RoleProgram      Role = "program"
	RoleControl      Role = "control"
)

// RoleLabels name each role for the group target picker.
var RoleLabels = map[Role]string{
	RoleDimmer:       "Dimmer",
	RoleRed:          "Red",
	RoleGreen:        "Green",
	RoleBlue:         "Blue",
	RoleWhite:        "White",
	RoleAmber:        "Amber",
	RoleUV:           "UV / Purple",
	RoleLime:         "Lime",
	RoleCyan:         "Cyan",
	RoleMagenta:      "Magenta",
	RoleYellow:       "Yellow",
	RoleColorWheel:   "Colour wheel",
	RoleStrobe:       "Strobe / shutter",
	RolePan:          "Pan",
	RolePanFine:      "Pan (fine)",
	RoleTilt:         "Tilt",
	RoleTiltFine:     "Tilt (fine)",
	RoleZoom:         "Zoom",
	RoleFocus:        "Focus",
	RoleIris:         "Iris",
	RoleGobo:         "Gobo",
	RoleGoboRotation: "Gobo rotation",
	RolePrism:        "Prism",
	RoleSpeed:        "Movement speed",
	RoleProgram:      "Built-in programmes",
	RoleControl:      "Control / reset",
}

// Label names r for a picker, falling back to the bare role.
func (r Role) Label() string {
	if l, ok := RoleLabels[r]; ok {
		return l
	}
	return string(r)
}

// A RoleGroup is one heading of the role picker, so related functions
// sit together.
type RoleGroup struct {
	Label string
	Roles []Role
}

// RoleGroups arrange the roles for the picker.
var RoleGroups = []RoleGroup{
	{"Intensity", []Role{RoleDimmer, RoleStrobe}},
	{"Colour", []Role{RoleRed, RoleGreen, RoleBlue, RoleWhite, RoleAmber, RoleUV,
		RoleLime, RoleCyan, RoleMagenta, RoleYellow, RoleColorWheel}},
	{"Movement", []Role{RolePan, RoleTilt, RolePanFine, RoleTiltFine, RoleSpeed}},
	{"Beam", []Role{RoleZoom, RoleFocus, RoleIris, RoleGobo, RoleGoboRotation, RolePrism}},
	{"Other", []Role{RoleProgram, RoleControl}},
}

// RoleOrder is every role in picker order, for sorting the roles a
// group offers.
var RoleOrder = func() []Role {
	var out []Role
	for _, g := range RoleGroups {
		out = append(out, g.Roles...)
	}
	return out
}()

// A Channel is one slot of a fixture's chart.
type Channel struct {
	// Offset is the position within the fixture, counted from one as
	// the manual prints it.
	Offset int
	// Name is the full name from the manual, such as "Intensity Purple
	// / UV".
	Name string
	Kind Kind
	// Role is the function this channel performs across products, where
	// it has one. Set it wherever a group might reasonably target the
	// channel, and leave it off for channels that only make sense on
	// this one product.
	Role Role
	// Home is the value the channel should sit at when the fixture is
	// idle.
	Home int
	// Ranges are the documented value bands, shown as help when
	// targeting the channel.
	Ranges []Range
	// Note is anything worth warning about, such as a band that seizes
	// control from the desk.
	Note string
}

// A Mode is one personality of a fixture. Many products switch between
// channel counts, and each of those is a mode with its own chart.
type Mode struct {
	ID string
	// Name shows in the mode picker, such as "9-channel".
	Name     string
	Channels []Channel
}

// A Definition is one product of the built-in library.
type Definition struct {
	ID           string
	Manufacturer string
	Model        string
	Modes        []Mode
	// Source says where the chart was transcribed from, so it can be
	// checked again.
	Source string
	Notes  string
}

// Label names a definition, such as "Fun Generation LED Diamond Dome
// MK II".
func (d Definition) Label() string {
	if d.Manufacturer == "" {
		return d.Model
	}
	return d.Manufacturer + " " + d.Model
}

// A Patch is one fixture the user has patched into the rig. It is saved
// with the configuration.
type Patch struct {
	ID string
	// Name is the user's name for this unit, such as "Dome 1".
	Name         string
	DefinitionID string
	ModeID       string
	// StartAddress is the DMX start address, 1 to 512.
	StartAddress int
	// Gamma is the dimmer curve exponent for this unit's intensity and
	// colour channels. Zero and one both leave values untouched.
	//
	// An LED driven by pulse width puts out light roughly in proportion
	// to its DMX value, but the eye reads brightness as something close
	// to a cube root of that, so a straight fade looks like it leaps to
	// bright and then crawls. Raising values to a power around 2.2
	// undoes it, and a fader at half then looks half. Lighting desks
	// call this the dimmer curve.
	Gamma float64
	// MaxLevel is the ceiling for this unit's intensity and colour
	// channels, 0 to 255, for trimming one fixture that out-shouts the
	// rest of the rig. Zero means 255.
	MaxLevel int
}

// A Group is a named set of fixtures a route drives as one, such as
// "Front", "Back" or "FX".
//
// Targeting a group by Role rather than a fixture by channel offset is
// what makes a show portable: at a new venue you re-point the group's
// members and every route follows, untouched.
//
// Member order means something — a spreading group route walks it in
// order, so a span of CC numbers chases along the bar the way the
// fixtures are hung.
type Group struct {
	ID string
	// Name is the user's name for the group, such as "Back wash".
	Name string
	// MemberIDs are Patch ids, in the order they should chase.
	MemberIDs []string
}
