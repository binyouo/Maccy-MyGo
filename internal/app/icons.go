package app

import "embed"

// Original template assets from Maccy f0b571e, under the included MIT license.
// macOS tints their alpha masks for the current menu bar appearance.
//
//go:embed assets/maccy/*.png assets/maccy/LICENSE
var menuIcons embed.FS

func trayIconPNG() []byte { return trayIconFor("maccy") }

func trayIconFor(name string) []byte {
	switch name {
	case "clipboard", "scissors", "paperclip":
	default:
		name = "maccy"
	}
	data, _ := menuIcons.ReadFile("assets/maccy/" + name + ".png")
	return data
}
