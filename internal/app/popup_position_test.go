package app

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestRelativePopupAnchorFollowsDisplayAndWidth(t *testing.T) {
	for _, tc := range []struct {
		area  mygo.Rectangle
		width int
		wantX int
		wantY int
	}{
		{mygo.Rectangle{X: 0, Y: 25, Width: 1920, Height: 1000}, 450, 735, 225},
		{mygo.Rectangle{X: -1710, Y: 38, Width: 1710, Height: 1000}, 450, -1080, 238},
		{mygo.Rectangle{X: -1710, Y: 38, Width: 1710, Height: 1000}, 650, -1180, 238},
	} {
		x, y := relativePopupOrigin(tc.area, tc.width, 400, 0.5, 0.2)
		if x != tc.wantX || y != tc.wantY {
			t.Fatalf("area=%+v width=%d origin=%d,%d want=%d,%d", tc.area, tc.width, x, y, tc.wantX, tc.wantY)
		}
	}
}

func TestOversizedPopupStaysAtDisplayOrigin(t *testing.T) {
	area := mygo.Rectangle{X: -800, Y: 25, Width: 800, Height: 600}
	x, y := constrainToRect(-600, 100, 900, 700, area)
	if x != area.X || y != area.Y {
		t.Fatalf("oversized popup escaped its display: %d,%d", x, y)
	}
}
