package fixtures

import (
	"math"
	"slices"
	"strconv"

	"github.com/marrasen/gunim-dmx/dmx"
)

// PatchMode is the mode a patch uses. It reports false when the
// definition is unknown, and falls back to the first mode when only the
// mode id is.
func (cat Catalog) PatchMode(p Patch) (Mode, bool) {
	def, ok := cat.Find(p.DefinitionID)
	if !ok || len(def.Modes) == 0 {
		return Mode{}, false
	}
	if i := slices.IndexFunc(def.Modes, func(m Mode) bool { return m.ID == p.ModeID }); i >= 0 {
		return def.Modes[i], true
	}
	return def.Modes[0], true
}

// ChannelCount is how many slots the patch takes up, and zero when the
// definition is unknown.
func (cat Catalog) ChannelCount(p Patch) int {
	m, ok := cat.PatchMode(p)
	if !ok {
		return 0
	}
	return len(m.Channels)
}

// ChannelAt is the channel at a one-based offset within the patch.
func (cat Catalog) ChannelAt(p Patch, offset int) (Channel, bool) {
	m, ok := cat.PatchMode(p)
	if !ok {
		return Channel{}, false
	}
	i := slices.IndexFunc(m.Channels, func(c Channel) bool { return c.Offset == offset })
	if i < 0 {
		return Channel{}, false
	}
	return m.Channels[i], true
}

// ResolveChannel is the universe slot a one-based offset within the
// patch lands on. It reports false when the offset is outside the
// fixture, or would fall past the end of the universe.
func (cat Catalog) ResolveChannel(p Patch, offset int) (int, bool) {
	if offset < 1 || offset > cat.ChannelCount(p) {
		return 0, false
	}
	slot := p.StartAddress + offset - 1
	if slot < 1 || slot > dmx.Channels {
		return 0, false
	}
	return slot, true
}

// A Resolved is a fixture channel together with the universe slot it
// lands on now.
type Resolved struct {
	Offset int
	// Slot is the universe slot, and InUniverse is false when it falls
	// outside it.
	Slot       int
	InUniverse bool
	Channel    Channel
}

// ResolvedChannels is every channel of a patch, with its slot.
func (cat Catalog) ResolvedChannels(p Patch) []Resolved {
	m, ok := cat.PatchMode(p)
	if !ok {
		return nil
	}
	out := make([]Resolved, len(m.Channels))
	for i, c := range m.Channels {
		slot, in := cat.ResolveChannel(p, c.Offset)
		out[i] = Resolved{Offset: c.Offset, Slot: slot, InUniverse: in, Channel: c}
	}
	return out
}

// A Footprint is the span of universe slots a patch occupies.
type Footprint struct{ Start, End int }

// FootprintOf is the span a patch occupies, and false when it has no
// channels.
func (cat Catalog) FootprintOf(p Patch) (Footprint, bool) {
	n := cat.ChannelCount(p)
	if n == 0 {
		return Footprint{}, false
	}
	return Footprint{Start: p.StartAddress, End: p.StartAddress + n - 1}, true
}

// ExceedsUniverse reports whether a patch runs past the end of the
// 512-slot universe.
func (cat Catalog) ExceedsUniverse(p Patch) bool {
	f, ok := cat.FootprintOf(p)
	return ok && f.End > dmx.Channels
}

// FindPatch looks a patched fixture up by id.
func FindPatch(patches []Patch, id string) (Patch, bool) {
	i := slices.IndexFunc(patches, func(p Patch) bool { return p.ID == id })
	if i < 0 {
		return Patch{}, false
	}
	return patches[i], true
}

// Overlapping is the ids of patches whose address windows collide.
//
// Overlapping fixtures answer each other's channels, which is now and
// then deliberate — a matched pair mirroring each other — but usually a
// patching mistake, so the interface flags it.
func (cat Catalog) Overlapping(patches []Patch) map[string]bool {
	clashing := map[string]bool{}
	for i := range patches {
		a, ok := cat.FootprintOf(patches[i])
		if !ok {
			continue
		}
		for j := i + 1; j < len(patches); j++ {
			b, ok := cat.FootprintOf(patches[j])
			if !ok {
				continue
			}
			if a.Start <= b.End && b.Start <= a.End {
				clashing[patches[i].ID] = true
				clashing[patches[j].ID] = true
			}
		}
	}
	return clashing
}

// SuggestStartAddress is the next free start address that fits a
// fixture of size slots, and false when the universe is full.
func (cat Catalog) SuggestStartAddress(patches []Patch, size int) (int, bool) {
	if size <= 0 {
		return 0, false
	}
	var taken []Footprint
	for _, p := range patches {
		if f, ok := cat.FootprintOf(p); ok {
			taken = append(taken, f)
		}
	}
	slices.SortFunc(taken, func(a, b Footprint) int { return a.Start - b.Start })
	candidate := 1
	for _, span := range taken {
		if candidate+size-1 < span.Start {
			break
		}
		candidate = max(candidate, span.End+1)
	}
	if candidate+size-1 > dmx.Channels {
		return 0, false
	}
	return candidate, true
}

// ChannelLabel names a fixture channel, such as "Dome 1 · Intensity
// Green".
func (cat Catalog) ChannelLabel(p Patch, offset int) string {
	name := p.Name
	if name == "" {
		name = "Untitled fixture"
	}
	if c, ok := cat.ChannelAt(p, offset); ok {
		return name + " · " + c.Name
	}
	return name + " · ch" + strconv.Itoa(offset)
}

// BandFor is the documented band a value falls into, for explaining
// what a channel is doing.
func BandFor(c Channel, value int) (string, bool) {
	for _, r := range c.Ranges {
		if value >= r.From && value <= r.To {
			return r.Label, true
		}
	}
	return "", false
}

// dimmable are the channels a fixture's curve and level trim apply to.
//
// It is deliberately narrow: a motor, strobe or programme channel
// carries documented value bands where a number means a particular
// behaviour, so bending those would quietly change what the fixture
// does rather than how bright it looks.
var dimmable = map[Kind]bool{KindIntensity: true, KindColor: true}

// Dimmable reports whether the fixture's curve applies to a channel.
func Dimmable(c Channel) bool { return dimmable[c.Kind] }

// CurveValue applies a patch's dimmer curve and level ceiling to a
// value bound for one of its channels. A channel the curve does not
// apply to passes through untouched.
func CurveValue(p Patch, c Channel, value int) int {
	if !Dimmable(c) {
		return value
	}
	gamma := p.Gamma
	if gamma <= 0 {
		gamma = 1
	}
	maxLevel := p.MaxLevel
	if maxLevel <= 0 {
		maxLevel = 255
	}
	if gamma == 1 && maxLevel == 255 {
		return value
	}
	normalised := min(1, max(0, float64(value)/255))
	if gamma != 1 {
		normalised = math.Pow(normalised, gamma)
	}
	return int(dmx.Clamp8f(normalised * float64(maxLevel)))
}

// CurveValueAt curves a value for the channel at an offset, and leaves
// it alone when the offset names no channel.
func (cat Catalog) CurveValueAt(p Patch, offset, value int) int {
	if c, ok := cat.ChannelAt(p, offset); ok {
		return CurveValue(p, c, value)
	}
	return value
}

// ChannelForRole is the channel performing a role on this fixture.
//
// A fixture may in principle declare a role twice; the first wins,
// matching the order the manual prints.
func (cat Catalog) ChannelForRole(p Patch, role Role) (Resolved, bool) {
	for _, c := range cat.ResolvedChannels(p) {
		if c.Channel.Role == role {
			return c, true
		}
	}
	return Resolved{}, false
}

// RolesOf is every role this fixture offers, in picker order.
func (cat Catalog) RolesOf(p Patch) []Role {
	m, ok := cat.PatchMode(p)
	if !ok {
		return nil
	}
	present := map[Role]bool{}
	for _, c := range m.Channels {
		if c.Role != "" {
			present[c.Role] = true
		}
	}
	var out []Role
	for _, r := range RoleOrder {
		if present[r] {
			out = append(out, r)
		}
	}
	return out
}

// FindGroup looks a group up by id.
func FindGroup(groups []Group, id string) (Group, bool) {
	i := slices.IndexFunc(groups, func(g Group) bool { return g.ID == id })
	if i < 0 {
		return Group{}, false
	}
	return groups[i], true
}

// GroupMembers is a group's fixtures, in membership order.
//
// Members that are no longer patched are dropped, so a group outlives
// the deletion of one of its fixtures rather than breaking every route
// that targets it.
func GroupMembers(g Group, patches []Patch) []Patch {
	var out []Patch
	for _, id := range g.MemberIDs {
		if p, ok := FindPatch(patches, id); ok {
			out = append(out, p)
		}
	}
	return out
}

// RolesInGroup is every role offered by at least one member, in picker
// order.
//
// A group of mixed fixtures offers the union rather than the
// intersection: targeting amber on a group where only half the members
// have amber is a reasonable thing to want, and the rest are skipped.
func (cat Catalog) RolesInGroup(members []Patch) []Role {
	present := map[Role]bool{}
	for _, p := range members {
		for _, r := range cat.RolesOf(p) {
			present[r] = true
		}
	}
	var out []Role
	for _, r := range RoleOrder {
		if present[r] {
			out = append(out, r)
		}
	}
	return out
}

// MembersWithRole is the members of a group that have a role.
func (cat Catalog) MembersWithRole(members []Patch, role Role) []Patch {
	var out []Patch
	for _, p := range members {
		if _, ok := cat.ChannelForRole(p, role); ok {
			out = append(out, p)
		}
	}
	return out
}

// GroupRoleLabel names a group target, such as "Back wash · Red".
func GroupRoleLabel(g Group, role Role) string {
	name := g.Name
	if name == "" {
		name = "Untitled group"
	}
	return name + " · " + role.Label()
}

// GroupsContaining is the groups a fixture belongs to.
func GroupsContaining(groups []Group, fixtureID string) []Group {
	var out []Group
	for _, g := range groups {
		if slices.Contains(g.MemberIDs, fixtureID) {
			out = append(out, g)
		}
	}
	return out
}
