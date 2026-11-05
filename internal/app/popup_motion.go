package app

import (
	"math"
	"time"

	"github.com/egoist/mygo"
)

const (
	popupSpringFrequency = 28.0
	popupPositionEpsilon = 0.05 // DIPs: smaller than a physical pixel at Retina scale.
	popupVelocityEpsilon = 0.5  // DIPs per second.
)

// popupSpring keeps the actual position and velocity between targets.
// A repeated click changes only the destination, so reversing an in-flight
// resize does not restart an easing curve at an unrelated velocity.
type popupSpring struct {
	position, velocity, target float64
}

func (s *popupSpring) reset(target float64) {
	s.position, s.velocity, s.target = target, 0, target
}

func (s *popupSpring) retarget(target float64) { s.target = target }

func (s *popupSpring) settled() bool {
	return math.Abs(s.position-s.target) <= popupPositionEpsilon && math.Abs(s.velocity) <= popupVelocityEpsilon
}

// advance solves a critically damped spring exactly for elapsed time.
// It does not depend on a particular timer cadence or display refresh rate.
func (s *popupSpring) advance(dt time.Duration) {
	if dt <= 0 {
		return
	}
	t := dt.Seconds()
	x := s.position - s.target
	b := s.velocity + popupSpringFrequency*x
	decay := math.Exp(-popupSpringFrequency * t)
	s.position = s.target + (x+b*t)*decay
	s.velocity = (s.velocity - popupSpringFrequency*b*t) * decay
	if s.settled() {
		s.reset(s.target)
	}
}

// popupPlacement's anchor is the closed history column, not the last
// partially expanded window. Keep this placement for an opening/closing
// session so repeated reversals cannot gradually move the history column.
type popupPlacement struct {
	anchor, workArea mygo.Rectangle
	left             bool
}

func newPopupPlacement(anchor, workArea mygo.Rectangle, openWidth int) popupPlacement {
	return popupPlacement{
		anchor:   anchor,
		workArea: workArea,
		left:     workArea.Width > 0 && anchor.X+openWidth > workArea.X+workArea.Width,
	}
}

func (p popupPlacement) bounds(width, height float64) mygo.Rectangle {
	w, h := max(1, int(math.Round(width))), max(1, int(math.Round(height)))
	if p.workArea.Width > 0 {
		w = min(w, p.workArea.Width)
	}
	if p.workArea.Height > 0 {
		h = min(h, p.workArea.Height)
	}
	x, y := p.anchor.X, p.anchor.Y
	if p.left {
		x -= w - p.anchor.Width
	}
	if p.workArea.Width > 0 {
		x = max(p.workArea.X, min(x, p.workArea.X+p.workArea.Width-w))
	}
	if p.workArea.Height > 0 {
		y = max(p.workArea.Y, min(y, p.workArea.Y+p.workArea.Height-h))
	}
	return mygo.Rectangle{X: x, Y: y, Width: w, Height: h}
}
