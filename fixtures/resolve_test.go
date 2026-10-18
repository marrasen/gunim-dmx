package fixtures

import (
	"slices"
	"testing"
)

// lib is the built-in library alone.
var lib Catalog

const domeID = "fun-generation-led-diamond-dome-mk2"

func dome(over func(*Patch)) Patch {
	p := Patch{ID: "f1", Name: "Dome 1", DefinitionID: domeID, ModeID: "9ch", StartAddress: 1}
	if over != nil {
		over(&p)
	}
	return p
}

func at(start int) Patch { return dome(func(p *Patch) { p.StartAddress = start }) }

func TestLibraryHasTheDiamondDome(t *testing.T) {
	def, ok := lib.Find(domeID)
	if !ok {
		t.Fatal("the Diamond Dome is missing from the library")
	}
	if len(def.Modes[0].Channels) != 9 {
		t.Errorf("mode has %d channels, want 9", len(def.Modes[0].Channels))
	}
	if def.Source == "" {
		t.Error("the definition cites no manual")
	}
}

func TestLibraryNumbersChannelsFromOne(t *testing.T) {
	for _, def := range Library {
		for _, mode := range def.Modes {
			for i, c := range mode.Channels {
				if c.Offset != i+1 {
					t.Errorf("%s %s channel %d has offset %d", def.ID, mode.ID, i+1, c.Offset)
				}
			}
		}
	}
}

func TestResolveChannel(t *testing.T) {
	cases := []struct {
		name   string
		patch  Patch
		offset int
		want   int
		ok     bool
	}{
		{"offset one is the start address", at(1), 1, 1, true},
		{"and again higher up", at(10), 1, 10, true},
		{"the window slides with the address", at(10), 2, 11, true},
		{"right to the last channel", at(10), 9, 18, true},
		{"offset zero is outside the fixture", at(1), 0, 0, false},
		{"and so is one past the end", at(1), 10, 0, false},
		{"a window may end on the last slot", at(504), 9, 512, true},
		{"but not run past it", at(512), 2, 0, false},
		{"an unknown definition resolves nothing",
			dome(func(p *Patch) { p.DefinitionID = "nope" }), 1, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := lib.ResolveChannel(c.patch, c.offset)
			if ok != c.ok || got != c.want {
				t.Errorf("lib.ResolveChannel(offset %d) = %d, %v; want %d, %v", c.offset, got, ok, c.want, c.ok)
			}
		})
	}
	if n := lib.ChannelCount(dome(func(p *Patch) { p.DefinitionID = "nope" })); n != 0 {
		t.Errorf("an unknown definition has %d channels, want 0", n)
	}
}

func TestFootprint(t *testing.T) {
	for _, c := range []struct {
		start int
		want  Footprint
	}{{1, Footprint{1, 9}}, {100, Footprint{100, 108}}} {
		got, ok := lib.FootprintOf(at(c.start))
		if !ok || got != c.want {
			t.Errorf("lib.FootprintOf(start %d) = %+v, %v; want %+v", c.start, got, ok, c.want)
		}
	}
	if lib.ExceedsUniverse(at(504)) {
		t.Error("a fixture ending on slot 512 was called too big")
	}
	if !lib.ExceedsUniverse(at(505)) {
		t.Error("a fixture running past slot 512 was not flagged")
	}
}

func TestOverlapping(t *testing.T) {
	a, b := at(1), at(9)
	a.ID, b.ID = "a", "b"
	got := lib.Overlapping([]Patch{a, b})
	if !got["a"] || !got["b"] {
		t.Errorf("overlapping fixtures were not both flagged: %v", got)
	}
	b.StartAddress = 10 // packed back to back, not overlapping
	if got := lib.Overlapping([]Patch{a, b}); len(got) != 0 {
		t.Errorf("fixtures packed back to back were flagged: %v", got)
	}
}

func TestSuggestStartAddress(t *testing.T) {
	a, b := at(1), at(20)
	a.ID, b.ID = "a", "b"
	cases := []struct {
		name    string
		patches []Patch
		size    int
		want    int
		ok      bool
	}{
		{"nothing patched starts at one", nil, 9, 1, true},
		{"the next fixture packs in after the last", []Patch{at(1)}, 9, 10, true},
		{"a big enough gap is filled", []Patch{a, b}, 9, 10, true},
		{"the space below a fixture parked high up is used", []Patch{at(505)}, 9, 1, true},
		{"a fixture too big for the universe fits nowhere", nil, 513, 0, false},
		{"and neither does one of no size", nil, 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := lib.SuggestStartAddress(c.patches, c.size)
			if ok != c.ok || got != c.want {
				t.Errorf("SuggestStartAddress = %d, %v; want %d, %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestResolvedChannels(t *testing.T) {
	resolved := lib.ResolvedChannels(at(10))
	if len(resolved) != 9 {
		t.Fatalf("got %d channels, want 9", len(resolved))
	}
	if resolved[1].Channel.Name != "Intensity Green" || resolved[1].Slot != 11 {
		t.Errorf("second channel = %q at slot %d", resolved[1].Channel.Name, resolved[1].Slot)
	}
}

func TestChannelLabel(t *testing.T) {
	if got, want := lib.ChannelLabel(at(1), 6), "Dome 1 · Intensity Purple / UV"; got != want {
		t.Errorf("ChannelLabel = %q, want %q", got, want)
	}
}

func TestBandFor(t *testing.T) {
	motor, _ := lib.ChannelAt(at(1), 8)
	for _, c := range []struct {
		value int
		want  string
	}{
		{0, "Motor on, static position"},
		{64, "Motor position (parks at an angle)"},
		{200, "Motor speed, continuous rotation"},
	} {
		got, ok := BandFor(motor, c.value)
		if !ok || got != c.want {
			t.Errorf("BandFor(motor, %d) = %q, %v; want %q", c.value, got, ok, c.want)
		}
	}
	programmes, _ := lib.ChannelAt(at(1), 9)
	if got, _ := BandFor(programmes, 255); got != "Sound mode 2 (microphone)" {
		t.Errorf("BandFor(programmes, 255) = %q", got)
	}
	red, _ := lib.ChannelAt(at(1), 1)
	if _, ok := BandFor(red, 128); ok {
		t.Error("a plain colour channel reported a band")
	}
}

func TestChannelForRole(t *testing.T) {
	// Purple/UV is the Dome's sixth channel, so slot 20 + 6 - 1.
	got, ok := lib.ChannelForRole(at(20), RoleUV)
	if !ok || got.Offset != 6 || got.Slot != 25 {
		t.Errorf("lib.ChannelForRole(uv) = %+v, %v", got, ok)
	}
	// The Dome has no master dimmer: colour channels are its only levels.
	for _, role := range []Role{RoleDimmer, RolePan} {
		if _, ok := lib.ChannelForRole(at(1), role); ok {
			t.Errorf("the Dome claimed a %s channel", role)
		}
	}
}

func TestChannelForRoleAcrossFixtures(t *testing.T) {
	rgb := Patch{ID: "b", DefinitionID: "generic-rgb", ModeID: "3ch", StartAddress: 100}
	d, _ := lib.ChannelForRole(at(1), RoleRed)
	r, _ := lib.ChannelForRole(rgb, RoleRed)
	if d.Slot != 1 || r.Slot != 100 {
		t.Errorf("red resolved to %d and %d, want 1 and 100", d.Slot, r.Slot)
	}
}

func TestRolesOfIsInPickerOrder(t *testing.T) {
	// Intensity before colour before movement, whatever order the manual used.
	want := []Role{RoleStrobe, RoleRed, RoleGreen, RoleBlue, RoleWhite, RoleAmber,
		RoleUV, RoleSpeed, RoleProgram}
	if got := lib.RolesOf(at(1)); !slices.Equal(got, want) {
		t.Errorf("RolesOf = %v, want %v", got, want)
	}
}

func groupFixtures() (dome1, par1 Patch, back Group) {
	d := dome(func(p *Patch) { p.ID, p.Name, p.StartAddress = "a", "Dome 1", 1 })
	rgb := Patch{ID: "b", Name: "Par 1", DefinitionID: "generic-rgb", ModeID: "3ch", StartAddress: 100}
	return d, rgb, Group{ID: "g1", Name: "Back", MemberIDs: []string{"a", "b"}}
}

func TestGroupMembersFollowMembershipOrder(t *testing.T) {
	d, rgb, g := groupFixtures()
	g.MemberIDs = []string{"b", "a"}
	names := make([]string, 0, 2)
	for _, p := range GroupMembers(g, []Patch{d, rgb}) {
		names = append(names, p.Name)
	}
	if !slices.Equal(names, []string{"Par 1", "Dome 1"}) {
		t.Errorf("members = %v", names)
	}
}

func TestGroupMembersDropUnpatchedFixtures(t *testing.T) {
	// The group outlives the fixture rather than breaking every route on it.
	_, rgb, g := groupFixtures()
	members := GroupMembers(g, []Patch{rgb})
	if len(members) != 1 || members[0].ID != "b" {
		t.Errorf("members = %+v", members)
	}
}

func TestRolesInGroupIsTheUnion(t *testing.T) {
	d, rgb, _ := groupFixtures()
	roles := lib.RolesInGroup([]Patch{d, rgb})
	if !slices.Contains(roles, RoleRed) { // both have it
		t.Error("red is missing from the group's roles")
	}
	if !slices.Contains(roles, RoleUV) { // only the Dome has it
		t.Error("uv is missing from the group's roles")
	}
}

func TestMembersWithRole(t *testing.T) {
	d, rgb, _ := groupFixtures()
	members := []Patch{d, rgb}
	for _, c := range []struct {
		role Role
		want int
	}{{RoleRed, 2}, {RoleUV, 1}, {RolePan, 0}} {
		if got := len(lib.MembersWithRole(members, c.role)); got != c.want {
			t.Errorf("%d members have %s, want %d", got, c.role, c.want)
		}
	}
}

func TestGroupsContaining(t *testing.T) {
	_, _, g := groupFixtures()
	other := Group{ID: "g2", Name: "Front", MemberIDs: []string{"b"}}
	ids := func(gs []Group) []string {
		out := make([]string, 0, len(gs))
		for _, g := range gs {
			out = append(out, g.ID)
		}
		return out
	}
	if got := ids(GroupsContaining([]Group{g, other}, "a")); !slices.Equal(got, []string{"g1"}) {
		t.Errorf("groups holding a = %v", got)
	}
	if got := ids(GroupsContaining([]Group{g, other}, "b")); !slices.Equal(got, []string{"g1", "g2"}) {
		t.Errorf("groups holding b = %v", got)
	}
}

func TestCurveValue(t *testing.T) {
	red, _ := lib.ChannelAt(at(1), 1)   // Intensity Red, a colour channel
	motor, _ := lib.ChannelAt(at(1), 8) // Motor, a behaviour channel

	t.Run("off by default", func(t *testing.T) {
		p := at(1)
		if got := CurveValue(p, red, 128); got != 128 {
			t.Errorf("128 became %d", got)
		}
		if got := CurveValue(p, red, 255); got != 255 {
			t.Errorf("255 became %d", got)
		}
	})

	t.Run("pulls the middle down so half looks half", func(t *testing.T) {
		// The complaint that prompted this: at half the Dome already looked
		// nearly full. Gamma 2.2 puts mid-scale near a fifth of full output.
		p := dome(func(p *Patch) { p.Gamma = 2.2 })
		if got := CurveValue(p, red, 128); got != 56 {
			t.Errorf("128 became %d, want 56", got)
		}
	})

	t.Run("keeps the endpoints fixed", func(t *testing.T) {
		p := dome(func(p *Patch) { p.Gamma = 2.2 })
		if got := CurveValue(p, red, 0); got != 0 {
			t.Errorf("0 became %d", got)
		}
		if got := CurveValue(p, red, 255); got != 255 {
			t.Errorf("255 became %d", got)
		}
	})

	t.Run("never falls as the input rises", func(t *testing.T) {
		p := dome(func(p *Patch) { p.Gamma = 2.2 })
		previous := -1
		for v := range 256 {
			out := CurveValue(p, red, v)
			if out < previous {
				t.Fatalf("%d gave %d, below the %d before it", v, out, previous)
			}
			previous = out
		}
	})

	t.Run("leaves behaviour channels untouched", func(t *testing.T) {
		// 222 on the motor means one rotation speed; bending it would change
		// what the fixture does, not how bright it looks.
		p := dome(func(p *Patch) { p.Gamma, p.MaxLevel = 2.2, 128 })
		if got := CurveValue(p, motor, 222); got != 222 {
			t.Errorf("the motor's 222 became %d", got)
		}
		if Dimmable(motor) || !Dimmable(red) {
			t.Error("the wrong channels are dimmable")
		}
	})

	t.Run("caps at the maximum level", func(t *testing.T) {
		p := dome(func(p *Patch) { p.MaxLevel = 128 })
		if got := CurveValue(p, red, 255); got != 128 {
			t.Errorf("255 became %d, want 128", got)
		}
		if got := CurveValue(p, red, 0); got != 0 {
			t.Errorf("0 became %d", got)
		}
	})

	t.Run("combines the curve and the cap", func(t *testing.T) {
		// (128/255)^2 is 0.2516, times the 128 ceiling.
		p := dome(func(p *Patch) { p.Gamma, p.MaxLevel = 2, 128 })
		if got := CurveValue(p, red, 128); got != 32 {
			t.Errorf("128 became %d, want 32", got)
		}
	})
}

func TestCurveValueAt(t *testing.T) {
	p := dome(func(p *Patch) { p.Gamma = 2.2 })
	for _, c := range []struct{ offset, in, want int }{
		{1, 128, 56},   // a colour channel is curved
		{8, 222, 222},  // the motor is not
		{99, 128, 128}, // and neither is an offset that does not exist
	} {
		if got := lib.CurveValueAt(p, c.offset, c.in); got != c.want {
			t.Errorf("lib.CurveValueAt(offset %d, %d) = %d, want %d", c.offset, c.in, got, c.want)
		}
	}
}
