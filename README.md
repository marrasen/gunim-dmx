# gunim-dmx

Drive stage lights over DMX from Go. Pure Go, no cgo, Windows and Linux.

DMX is the cable standard stage lights listen on. One cable carries a
*universe* of 512 channels, each a value from 0 to 255, and each light
listens to a run of them from its *start address*. This library keeps
universes, merges every source of light into them, and sends them to the
box that drives the lights.

It is the lighting engine of [Hej DAW!](https://github.com/marrasen/marrdaw)
and [crossroads](https://github.com/marrasen/crossroads). It knows nothing
about either: anything can use it.

## Packages

| Package | What it does |
|---|---|
| `dmx` | A universe, the `Device` interface, and the `Engine` that sends a universe to a device no faster than the cable takes it (40 frames a second), holds a blackout, and survives an unplug |
| `enttec` | ENTTEC DMX USB Pro (and boxes that copy its protocol: Eurolite USB-DMX512 PRO, DMXking ultraDMX), and ENTTEC Open DMX USB |
| `artnet` | Art-Net 4 over the network: send a universe to a node, and find the nodes |
| `sacn` | sACN (ANSI E1.31) over the network, multicast or to one receiver |
| `fixtures` | Light types: what each channel of a light does, its modes, where it rests; the built-in library; a rig's own types; address maths and overlap checks |
| `mix` | Merges layers of light into one universe: strongest wins (HTP) or latest wins (LTP) per channel, home values, grand master |

## Try a box

```sh
go run github.com/marrasen/gunim-dmx/cmd/dmxtest@latest -list
go run github.com/marrasen/gunim-dmx/cmd/dmxtest@latest -pro auto -from 1 -to 9
```

`dmxtest` fades a run of channels slowly up and down. It also takes
`-opendmx PORT`, `-artnet IP -universe N` and `-sacn -universe N`.

On Windows, an Open DMX USB needs its FTDI driver's virtual COM port
turned on: Device Manager, the device's properties, "Load VCP".

## Use it

```go
e := dmx.NewEngine()
defer e.Close()
if err := e.Connect(func() (dmx.Device, error) { return enttec.FindPro() }); err != nil {
	log.Fatal(err)
}

m := mix.NewMixer()
front := mix.NewLayer()
m.Add(front)
stop := mix.Drive(m, e, nil)
defer stop()

front.Set(1, 255) // channel 1 full
```

## Licence

Apache-2.0, as gunim. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
