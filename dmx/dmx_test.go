package dmx

import "testing"

func TestMidiToDmx(t *testing.T) {
	cases := []struct {
		in   int
		want byte
	}{{0, 0}, {127, 255}, {64, 129}, {200, 255}}
	for _, c := range cases {
		if got := MidiToDmx(c.in); got != c.want {
			t.Errorf("MidiToDmx(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestLevel(t *testing.T) {
	for _, c := range []struct {
		in   float64
		want byte
	}{{0, 0}, {1, 255}, {0.5, 128}, {2, 255}, {-1, 0}} {
		if got := Level(c.in); got != c.want {
			t.Errorf("Level(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}
