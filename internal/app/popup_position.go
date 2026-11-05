package app

import (
	"math"

	"github.com/egoist/mygo"

	"github.com/binyouo/Maccy-MyGo/internal/config"
)

// Match Maccy's top-middle anchor while using MyGo's top-down screen axes.
func relativePopupOrigin(area mygo.Rectangle, w, h int, anchorX, anchorY float64) (int, int) {
	x := area.X + int(math.Round(float64(area.Width)*anchorX)) - w/2
	y := area.Y + int(math.Round(float64(area.Height)*anchorY))
	return constrainToRect(x, y, w, h, area)
}

func (a *App) rememberLastPopupPosition(x, y, width int) {
	a.lastX, a.lastY, a.hasLast = x, y, true
	area := mygo.Screen.DisplayNearestPoint(mygo.Point{X: x + width/2, Y: y}).WorkArea
	if testArea, ok := a.developmentScreen(); ok {
		area = testArea
	}
	_ = a.cfg.Update(func(c *config.Config) {
		c.HasPopupPosition, c.PopupX, c.PopupY = true, x, y
		if area.Width > 0 && area.Height > 0 {
			c.HasRelativePopupPosition = true
			c.PopupAnchorX = float64(x+width/2-area.X) / float64(area.Width)
			c.PopupAnchorY = float64(y-area.Y) / float64(area.Height)
		}
	})
}
