package fixtures

// Library is the built-in fixture database.
//
// Each entry mirrors the DMX chart printed in the manufacturer's
// manual, value bands included, so the route editor can say what a
// channel does rather than show a bare number. Definitions live in code
// rather than in user data: to add a fixture, transcribe its chart here
// and cite the manual in Source so the numbers can be checked later.
var Library = []Definition{
	{
		ID:           "fun-generation-led-diamond-dome-mk2",
		Manufacturer: "Fun Generation",
		Model:        "LED Diamond Dome MK II",
		Source:       "https://fast-images.static-thomann.de/pics/atg/atgdata/document/manual/542517_c_542517_v2_en_online.pdf",
		Notes: "Six 6-in-1 RGBWA+UV LEDs on a rotating mirror dome. There is no master " +
			"dimmer or shutter: raise a colour channel to get light out of it. Set " +
			"the fixture's menu to DMX (display shows A001 for address 1), otherwise " +
			"it runs its own programmes and ignores the console entirely.",
		Modes: []Mode{{
			ID:   "9ch",
			Name: "9-channel",
			Channels: []Channel{
				{Offset: 1, Name: "Intensity Red", Kind: KindColor, Role: RoleRed},
				{Offset: 2, Name: "Intensity Green", Kind: KindColor, Role: RoleGreen},
				{Offset: 3, Name: "Intensity Blue", Kind: KindColor, Role: RoleBlue},
				{Offset: 4, Name: "Intensity White", Kind: KindColor, Role: RoleWhite},
				{Offset: 5, Name: "Intensity Amber", Kind: KindColor, Role: RoleAmber},
				{Offset: 6, Name: "Intensity Purple / UV", Kind: KindColor, Role: RoleUV},
				{Offset: 7, Name: "Stroboscope", Kind: KindStrobe, Role: RoleStrobe, Ranges: []Range{
					{0, 9, "No function (steady light)"},
					{10, 255, "Strobe on, increasing speed"},
				}},
				{Offset: 8, Name: "Motor", Kind: KindMotor, Role: RoleSpeed,
					Note: "Below 6 the dome holds still. The middle band parks it at a " +
						"chosen angle; the top band rotates it continuously, faster as " +
						"the value rises.",
					Ranges: []Range{
						{0, 5, "Motor on, static position"},
						{6, 127, "Motor position (parks at an angle)"},
						{128, 255, "Motor speed, continuous rotation"},
					}},
				{Offset: 9, Name: "Built-in programmes", Kind: KindProgram, Role: RoleProgram,
					Note: "Keep this low for manual colour control. From 51 upward the " +
						"fixture runs its own show, and from 201 it listens to its " +
						"microphone instead, overriding the colour channels either way.",
					Ranges: []Range{
						{0, 50, "Random effects"},
						{51, 100, "Double colour change, increasing motor speed"},
						{101, 150, "Change of all colours, increasing motor speed"},
						{151, 200, "Transition of all colours, increasing motor speed"},
						{201, 250, "Sound mode 1 (microphone)"},
						{251, 255, "Sound mode 2 (microphone)"},
					}},
			},
		}},
	},
	{
		ID:           "generic-dimmer-1ch",
		Manufacturer: "Generic",
		Model:        "Dimmer",
		Notes:        "A single level, for a dimmer pack channel or any one-channel device.",
		Modes: []Mode{{
			ID:       "1ch",
			Name:     "1-channel",
			Channels: []Channel{{Offset: 1, Name: "Intensity", Kind: KindIntensity, Role: RoleDimmer}},
		}},
	},
	{
		ID:           "generic-rgb",
		Manufacturer: "Generic",
		Model:        "RGB LED",
		Notes:        "The common three-channel colour mixer, with no master dimmer.",
		Modes: []Mode{{
			ID:   "3ch",
			Name: "3-channel",
			Channels: []Channel{
				{Offset: 1, Name: "Intensity Red", Kind: KindColor, Role: RoleRed},
				{Offset: 2, Name: "Intensity Green", Kind: KindColor, Role: RoleGreen},
				{Offset: 3, Name: "Intensity Blue", Kind: KindColor, Role: RoleBlue},
			},
		}},
	},
	{
		ID:           "generic-rgbw-dimmer",
		Manufacturer: "Generic",
		Model:        "RGBW LED with dimmer",
		Notes: "Four colours behind a master dimmer. The dimmer has to be up before " +
			"any colour shows.",
		Modes: []Mode{{
			ID:   "5ch",
			Name: "5-channel",
			Channels: []Channel{
				{Offset: 1, Name: "Master dimmer", Kind: KindIntensity, Role: RoleDimmer},
				{Offset: 2, Name: "Intensity Red", Kind: KindColor, Role: RoleRed},
				{Offset: 3, Name: "Intensity Green", Kind: KindColor, Role: RoleGreen},
				{Offset: 4, Name: "Intensity Blue", Kind: KindColor, Role: RoleBlue},
				{Offset: 5, Name: "Intensity White", Kind: KindColor, Role: RoleWhite},
			},
		}},
	},
}
