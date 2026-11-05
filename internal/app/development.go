package app

import (
	"log"
	"os"
	"strconv"

	"github.com/egoist/mygo"
)

// MACCY_TEST_SCREEN is one-based, like the screen picker. Validate it
// before creating any visible windows; a missing display must never make
// a secondary-display test fall back to the primary display.
func (a *App) developmentScreen() (mygo.Rectangle, bool) {
	v := os.Getenv("MACCY_TEST_SCREEN")
	if v == "" {
		return mygo.Rectangle{}, false
	}
	i, err := strconv.Atoi(v)
	displays := mygo.Screen.Displays()
	if err != nil || i < 1 || i > len(displays) {
		log.Fatalf("maccy: invalid MACCY_TEST_SCREEN %q (%d displays)", v, len(displays))
	}
	if displays[i-1].Primary {
		log.Fatal("maccy: MACCY_TEST_SCREEN must select a secondary display")
	}
	return displays[i-1].WorkArea, true
}

func (a *App) hasDevelopmentScreen() bool { return os.Getenv("MACCY_TEST_SCREEN") != "" }

func (a *App) showOnDevelopmentScreen(w *mygo.Window) {
	if wa, ok := a.developmentScreen(); ok {
		width, height := w.Size()
		x, y := developmentOrigin(wa, width, height)
		w.SetPosition(x, y)
		w.Show()
		w.Focus()
	}
}

func developmentOrigin(wa mygo.Rectangle, width, height int) (int, int) {
	x := wa.X + (wa.Width-width)/2
	switch os.Getenv("MACCY_TEST_EDGE") {
	case "right":
		x = wa.X + wa.Width - width - 12
	case "left":
		x = wa.X + 12
	}
	return constrainToRect(x, wa.Y+(wa.Height-height)/2, width, height, wa)
}
