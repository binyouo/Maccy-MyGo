package app

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestPopupSpringRetargetKeepsPositionAndVelocity(t *testing.T) {
	var s popupSpring
	s.reset(450)
	s.retarget(851)
	s.advance(50 * time.Millisecond)
	position, velocity := s.position, s.velocity
	if velocity <= 0 || position <= 450 || position >= 851 {
		t.Fatalf("spring did not begin opening: %+v", s)
	}
	s.retarget(450)
	if s.position != position || s.velocity != velocity {
		t.Fatalf("retarget jumped position or velocity: before=(%v, %v), after=%+v", position, velocity, s)
	}
	// A reversal first decelerates the ongoing motion; it cannot instantly
	// start moving the other way at an arbitrary new easing velocity.
	s.advance(time.Microsecond)
	if s.position < position || s.velocity <= 0 || math.Abs(s.velocity-velocity) > 1 {
		t.Fatalf("reversal changed velocity discontinuously: before=%v after=%+v", velocity, s)
	}
}

func TestPopupSpringReversalRestoresExactEndpoint(t *testing.T) {
	for _, when := range []time.Duration{time.Millisecond, 16 * time.Millisecond, 50 * time.Millisecond, 120 * time.Millisecond, 250 * time.Millisecond} {
		for _, initial := range []float64{450, 851} {
			var s popupSpring
			s.reset(initial)
			s.retarget(1301 - initial)
			s.advance(when)
			s.retarget(initial)
			for i := 0; i < 150; i++ {
				s.advance(time.Second / 120)
				if s.position < 450-1e-9 || s.position > 851+1e-9 {
					t.Fatalf("reversal at %v overshot its window interval: %+v", when, s)
				}
			}
			if !s.settled() || s.position != initial || s.velocity != 0 {
				t.Fatalf("reversal at %v did not restore %v exactly: %+v", when, initial, s)
			}
		}
	}
}

func TestPopupSpringRepeatedClicksStayWithinEndpoints(t *testing.T) {
	var s popupSpring
	s.reset(450)
	for click := 0; click < 20; click++ {
		target := float64(450)
		if click%2 == 0 {
			target = 851
		}
		s.retarget(target)
		for frame := 0; frame < 7; frame++ {
			s.advance(5 * time.Millisecond)
			if s.position < 450-1e-9 || s.position > 851+1e-9 {
				t.Fatalf("click %d overshot: %+v", click, s)
			}
		}
	}
	s.advance(time.Second)
	if s.position != 450 || s.velocity != 0 || !s.settled() {
		t.Fatalf("repeated clicks left a different endpoint: %+v", s)
	}
}

func TestPopupSpringDoesNotDependOnFrameCadence(t *testing.T) {
	var single, many popupSpring
	single.reset(450)
	single.retarget(851)
	many = single
	single.advance(200 * time.Millisecond)
	for i := 0; i < 20; i++ {
		many.advance(10 * time.Millisecond)
	}
	if math.Abs(single.position-many.position) > 1e-9 || math.Abs(single.velocity-many.velocity) > 1e-9 {
		t.Fatalf("frame cadence changed trajectory: single=%+v, many=%+v", single, many)
	}
	before := many
	many.advance(0)
	many.advance(-time.Second)
	if many != before {
		t.Fatalf("a nonpositive elapsed time changed motion: before=%+v after=%+v", before, many)
	}
}

func TestPopupSpringSnapsOnlyWithinSubpixelRest(t *testing.T) {
	var s popupSpring
	s.reset(450)
	s.retarget(851)
	for i := 0; i < 120; i++ {
		previous := s
		s.advance(time.Second / 240)
		if s.settled() {
			if s.position != 851 || s.velocity != 0 {
				t.Fatalf("settled spring retained error: %+v", s)
			}
			if math.Abs(previous.position-851) > 0.1 {
				t.Fatalf("spring snapped before reaching subpixel distance: %+v", previous)
			}
			return
		}
	}
	t.Fatal("spring did not settle")
}

func TestPopupPlacementKeepsHistoryAnchorAtDisplayEdges(t *testing.T) {
	work := mygo.Rectangle{X: -1710, Y: 25, Width: 1710, Height: 1055}
	for _, anchorX := range []int{-1710, -1400, -450} {
		anchor := mygo.Rectangle{X: anchorX, Y: 100, Width: 450, Height: 380}
		p := newPopupPlacement(anchor, work, 851)
		for step := 0; step <= 401; step++ {
			width := float64(450) + float64(step)
			r := p.bounds(width, 380)
			historyX := r.X
			if p.left {
				historyX += r.Width - anchor.Width
			}
			if historyX != anchor.X {
				t.Fatalf("history moved at anchor %d, width %v: bounds=%+v left=%v historyX=%d", anchor.X, width, r, p.left, historyX)
			}
			if r.X < work.X || r.X+r.Width > work.X+work.Width {
				t.Fatalf("popup crossed displays: %+v", r)
			}
		}
		if got := p.bounds(450, 380); got != anchor {
			t.Fatalf("closed endpoint changed: got=%+v want=%+v", got, anchor)
		}
	}
}

func TestPopupPlacementReversalCannotDriftAnchor(t *testing.T) {
	work := mygo.Rectangle{Width: 1920, Height: 1080}
	anchor := mygo.Rectangle{X: 1470, Y: 100, Width: 450, Height: 380}
	p := newPopupPlacement(anchor, work, 851)
	var s popupSpring
	s.reset(450)
	for click := 0; click < 12; click++ {
		if click%2 == 0 {
			s.retarget(851)
		} else {
			s.retarget(450)
		}
		s.advance(time.Duration(17+click*3) * time.Millisecond)
		r := p.bounds(s.position, 380)
		if historyX := r.X + r.Width - anchor.Width; historyX != anchor.X {
			t.Fatalf("click %d moved history to %d: %+v", click, historyX, r)
		}
	}
	s.advance(time.Second)
	if got := p.bounds(s.position, 380); got != anchor {
		t.Fatalf("reversing repeatedly drifted closed origin: got=%+v want=%+v", got, anchor)
	}
}

func TestPopupPlacementRoundsAtOutputAndFitsWorkArea(t *testing.T) {
	work := mygo.Rectangle{X: -1710, Y: 25, Width: 1710, Height: 1055}
	p := newPopupPlacement(mygo.Rectangle{X: -450, Y: 900, Width: 450, Height: 180}, work, 851)
	r := p.bounds(650.6, 380.6)
	if r != (mygo.Rectangle{X: -651, Y: 699, Width: 651, Height: 381}) {
		t.Fatalf("unexpected rounded/clamped bounds: %+v", r)
	}
	r = p.bounds(2000, 2000)
	if r != work {
		t.Fatalf("oversized popup escaped work area: %+v", r)
	}
}
