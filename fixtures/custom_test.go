package fixtures

import "testing"

// TestARigsOwnTypeIsFoundLikeABuiltInOne: once handed over, a rig's
// type resolves a patch's channels as a library one does.
func TestARigsOwnTypeIsFoundLikeABuiltInOne(t *testing.T) {
	def := Definition{ID: CustomPrefix + "par", Model: "Generic RGBW", Modes: []Mode{{ID: "default",
		Channels: []Channel{
			ChannelFor(1, Function(RoleRed), "", 0),
			ChannelFor(2, Function(RoleGreen), "", 0),
			ChannelFor(3, Function(RoleBlue), "", 0),
			ChannelFor(4, FunctionUnused, "", 0),
			ChannelFor(5, FunctionUnknown, "", 0),
		}}}}
	cat := Catalog{Own: []Definition{def}}

	p := Patch{ID: "p1", DefinitionID: def.ID, ModeID: "default", StartAddress: 10}
	if n := cat.ChannelCount(p); n != 5 {
		t.Fatalf("the patch has %d channels, want 5", n)
	}
	if c, ok := cat.ChannelForRole(p, RoleBlue); !ok || c.Slot != 12 {
		t.Errorf("blue resolves to %+v, %v, want slot 12", c, ok)
	}
	if def.Label() != "Generic RGBW" {
		t.Errorf("a type with no maker is called %q", def.Label())
	}
}

// TestAChannelReadsBackAsItsFunction: what the editor sets a channel to
// do is what it reads back as, the unknown and the unused included.
func TestAChannelReadsBackAsItsFunction(t *testing.T) {
	for _, f := range []Function{Function(RoleDimmer), Function(RoleUV), Function(RolePan),
		FunctionUnknown, FunctionUnused} {
		c := ChannelFor(3, f, "", 0)
		if got := FunctionOf(c); got != f {
			t.Errorf("a channel made as %q reads back as %q", f, got)
		}
	}
	if c := ChannelFor(7, FunctionUnknown, "", 0); c.Name != "Channel 7" || c.Kind != KindOther {
		t.Errorf("an unknown channel is %+v", c)
	}
	if c := ChannelFor(2, Function(RoleRed), "", 0); c.Kind != KindColor || !Dimmable(c) {
		t.Errorf("a red channel is %+v, and must take the dimmer curve", c)
	}
	if c := ChannelFor(2, Function(RoleRed), "Red LED", 0); c.Name != "Red LED" {
		t.Errorf("a name given is not kept: %q", c.Name)
	}
}
