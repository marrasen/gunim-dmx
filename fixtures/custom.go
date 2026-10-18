package fixtures

import (
	"slices"
	"strconv"
)

// A Catalog is every light type a rig can use: its own, then the
// built-in library.
//
// A rig makes types of its own for a generic RGBW par, or a product
// nobody has transcribed. Their ids start with CustomPrefix, which keeps
// them clear of the library's. A Catalog is a value: a rig that changes
// its types makes a new one.
type Catalog struct {
	// Own are the rig's own types.
	Own []Definition
}

// CustomPrefix starts the id of every type a rig made.
const CustomPrefix = "custom-"

// All is every type a patch can use: the rig's own first, then the
// built-in library.
func (cat Catalog) All() []Definition {
	return append(slices.Clone(cat.Own), Library...)
}

// IsCustom reports whether a type id is one a rig made.
func IsCustom(id string) bool {
	return len(id) > len(CustomPrefix) && id[:len(CustomPrefix)] == CustomPrefix
}

// Find looks a type up among the rig's own and then the library.
func (cat Catalog) Find(id string) (Definition, bool) {
	defs := Library
	if IsCustom(id) {
		defs = cat.Own
	}
	i := slices.IndexFunc(defs, func(d Definition) bool { return d.ID == id })
	if i < 0 {
		return Definition{}, false
	}
	return defs[i], true
}

// A Function is what one channel of a fixture type does, as the type
// editor offers it: one of the roles, or a channel that does something
// unnamed, or one that does nothing.
type Function string

const (
	// FunctionUnknown is a channel that does something, but nothing a
	// role names. It can still be driven, by its number.
	FunctionUnknown Function = "unknown"
	// FunctionUnused is a channel the fixture has, so it takes an
	// address, but nothing needs to drive.
	FunctionUnused Function = "unused"
)

// unusedName is what an unused channel is called. A channel of no role,
// of kind other, under this name reads back as unused.
const unusedName = "Not used"

// KindOf is the kind a channel of a role has, the way the built-in
// library sorts them.
func KindOf(r Role) Kind {
	//exhaustive:ignore every other role moves the light, as a motor's.
	switch r {
	case RoleDimmer:
		return KindIntensity
	case RoleRed, RoleGreen, RoleBlue, RoleWhite, RoleAmber, RoleUV, RoleLime,
		RoleCyan, RoleMagenta, RoleYellow:
		return KindColor
	case RoleStrobe:
		return KindStrobe
	case RoleProgram, RoleControl:
		return KindProgram
	case "":
		return KindOther
	}
	return KindMotor
}

// ChannelFor is a channel of a type made in the editor: offset n, doing
// f, under name, resting at home. An empty name takes the function's.
func ChannelFor(n int, f Function, name string, home int) Channel {
	switch f {
	case FunctionUnused:
		return Channel{Offset: n, Name: unusedName, Kind: KindOther}
	case FunctionUnknown:
		if name == "" {
			name = "Channel " + strconv.Itoa(n)
		}
		return Channel{Offset: n, Name: name, Kind: KindOther, Home: home}
	}
	r := Role(f)
	if name == "" {
		name = r.Label()
	}
	return Channel{Offset: n, Name: name, Kind: KindOf(r), Role: r, Home: home}
}

// FunctionOf is what the editor shows a channel as doing.
func FunctionOf(c Channel) Function {
	switch {
	case c.Role != "":
		return Function(c.Role)
	case c.Kind == KindOther && c.Name == unusedName:
		return FunctionUnused
	}
	return FunctionUnknown
}

// A Preset is a fixture type to start the editor from, named the way
// fixture makers name their modes.
type Preset struct {
	Name      string
	Functions []Function
}

// Presets are the common generic types, offered as a starting point.
var Presets = func() []Preset {
	f := func(roles ...Role) []Function {
		out := make([]Function, len(roles))
		for i, r := range roles {
			out[i] = Function(r)
		}
		return out
	}
	return []Preset{
		{"RGB", f(RoleRed, RoleGreen, RoleBlue)},
		{"RGBW", f(RoleRed, RoleGreen, RoleBlue, RoleWhite)},
		{"RGBA", f(RoleRed, RoleGreen, RoleBlue, RoleAmber)},
		{"RGBWA+UV", f(RoleRed, RoleGreen, RoleBlue, RoleWhite, RoleAmber, RoleUV)},
		{"Dimmer + RGB", f(RoleDimmer, RoleRed, RoleGreen, RoleBlue)},
		{"Dimmer + RGBW + strobe", f(RoleDimmer, RoleRed, RoleGreen, RoleBlue, RoleWhite, RoleStrobe)},
		{"Dimmer + RGBWA+UV + strobe", f(RoleDimmer, RoleRed, RoleGreen, RoleBlue, RoleWhite,
			RoleAmber, RoleUV, RoleStrobe)},
		{"Dimmer", f(RoleDimmer)},
	}
}()
