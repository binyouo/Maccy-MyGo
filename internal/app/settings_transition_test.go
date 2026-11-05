package app

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestSettingsTransitionHasIntermediateBoundsAndPaneOpacity(t *testing.T) {
	start := time.Unix(100, 0)
	m := newSettingsWindowMotion(mygo.Rectangle{X: -1110, Y: 100, Width: 510, Height: 528}, start)
	m.retarget(710, 552, start, false)
	area := mygo.Rectangle{X: -1710, Y: 0, Width: 1710, Height: 1080}
	anchor := mygo.Point{X: -855, Y: 100}
	previous := m.bounds(anchor, area)
	intermediate := 0
	for frame := 1; frame <= 50; frame++ {
		m.advance(start.Add(time.Duration(frame) * time.Second / 120))
		bounds := m.bounds(anchor, area)
		if bounds.X+bounds.Width/2 != anchor.X || bounds.Y != anchor.Y {
			t.Fatalf("animation moved its canonical anchor: %+v", bounds)
		}
		if bounds.Width < previous.Width || bounds.Height < previous.Height {
			t.Fatalf("opening reversed during frame %d: %+v -> %+v", frame, previous, bounds)
		}
		if bounds.Width > 510 && bounds.Width < 710 && m.opacity.position > 0 && m.opacity.position < 1 {
			intermediate++
		}
		previous = bounds
	}
	if intermediate < 5 {
		t.Fatalf("only %d intermediate layout/opacity frames", intermediate)
	}
	m.advance(start.Add(time.Second))
	if got := m.bounds(anchor, area); got.Width != 710 || got.Height != 552 || m.moving() {
		t.Fatalf("animation did not finish exactly: %+v opacity=%f", got, m.opacity.position)
	}
}

func TestSettingsTransitionRetargetKeepsPositionAndVelocity(t *testing.T) {
	start := time.Unix(100, 0)
	m := newSettingsWindowMotion(mygo.Rectangle{Width: 510, Height: 528}, start)
	m.retarget(710, 552, start, false)
	at := start.Add(65 * time.Millisecond)
	m.advance(at)
	position, velocity := m.width.position, m.width.velocity
	m.retarget(510, 300, at, false)
	if m.width.position != position || m.width.velocity != velocity {
		t.Fatal("retarget restarted the in-flight resize")
	}
	m.advance(at.Add(time.Millisecond))
	if math.Abs(m.width.position-position) > math.Abs(velocity)*0.002+1 {
		t.Fatal("reversal teleported the window")
	}
	m.advance(at.Add(time.Second))
	if !m.width.settled() || !m.height.settled() || m.width.position != 510 || m.height.position != 300 {
		t.Fatal("reversed resize did not reach its final category")
	}
}

func TestSettingsTransitionReduceMotionIsImmediate(t *testing.T) {
	start := time.Unix(100, 0)
	m := newSettingsWindowMotion(mygo.Rectangle{Width: 510, Height: 528}, start)
	m.retarget(710, 552, start, true)
	if m.moving() || m.width.position != 710 || m.height.position != 552 || m.opacity.position != 1 {
		t.Fatal("Reduce Motion retained an animation")
	}
}

func TestSettingsTransitionClipsPaneWithoutReflowingControls(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 2
	start := time.Unix(100, 0)
	a.prefsMotion = newSettingsWindowMotion(mygo.Rectangle{Width: 710, Height: 552}, start)
	tt := ui.NewTester(a.settingsView, 710, 552)
	before, ok := tt.Find("显示菜单栏图标")
	if !ok {
		t.Fatal("missing appearance control")
	}
	a.prefsMotion.retarget(710, 552, start, false)
	for i, width := range []int{510, 540, 600, 650, 710} {
		a.prefsMotion.advance(start.Add(time.Duration(i+1) * 40 * time.Millisecond))
		tt.SetSize(width, 552)
		tt.Frame()
		after, ok := tt.Find("显示菜单栏图标")
		if !ok || after.X != before.X || after.Y != before.Y || after.H != before.H {
			t.Fatalf("host resize reflowed a fixed-width pane at %d: before=%v after=%v", width, before, after)
		}
		if opacity := a.settingsPaneOpacity(); opacity <= 0 || opacity >= 1 {
			t.Fatalf("frame %d did not show intermediate pane opacity: %f", i, opacity)
		}
	}
}

func TestSettingsSwitchRemovesOutgoingPane(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 1
	tt := ui.NewTester(a.settingsView, 510, 300)
	if err := tt.Click("外观"); err != nil {
		t.Fatal(err)
	}
	// The host and incoming pane already share a resize/opacity clock.
	// Compare their pixels to a fresh pane to catch inert exit ghosts,
	// which deliberately do not appear in the accessibility tree.
	tt.Advance(40 * time.Millisecond)
	fresh := ui.NewTester(a.settingsView, 510, 300)
	got, want := tt.Image(), fresh.Image()
	for y := 60; y < 290; y++ {
		for x := 0; x < 500; x++ {
			if got.RGBAAt(x, y) != want.RGBAAt(x, y) {
				t.Fatalf("outgoing form still paints over incoming controls at (%d,%d)", x, y)
			}
		}
	}
}

func TestSettingsToolbarKeepsScreenPositionAcrossOddWidths(t *testing.T) {
	a := newTestApp(t)
	a.draftFromConfig()
	a.prefsTab = 3
	tt := ui.NewTester(a.settingsView, 510, 430)
	area := mygo.Rectangle{Width: 1920, Height: 1080}
	anchor := mygo.Point{X: 960, Y: 100}
	base, ok := tt.Find("置顶项")
	if !ok {
		t.Fatal("missing pins tab")
	}
	initial := settingsBoundsAtAnchor(anchor, 510, 430, area)
	wantX := float32(initial.X) + base.X
	for _, width := range []int{511, 512, 513, 560, 601, 650, 709, 710, 709, 651, 511, 510} {
		tt.SetSize(width, 430)
		r, ok := tt.Find("置顶项")
		bounds := settingsBoundsAtAnchor(anchor, width, 430, area)
		if !ok || float32(bounds.X)+r.X != wantX {
			t.Fatalf("pins tab shifted during resize at width %d: screen x=%v, want %v", width, float32(bounds.X)+r.X, wantX)
		}
	}
}

func TestPinsTabTextRasterStaysFixedDuringWindowResize(t *testing.T) {
	for _, scale := range []int{1, 2} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("scale-%d/dark-%v", scale, dark), func(t *testing.T) {
				a := newTestApp(t)
				a.draftFromConfig()
				a.prefsTab = 3
				tt := ui.NewTester(a.settingsView, 510, 430)
				tt.SetScale(float32(scale))
				tt.SetDark(dark)
				tt.Advance(time.Second)
				tab, ok := tt.Find("置顶项")
				if !ok {
					t.Fatal("missing pins tab")
				}
				area := mygo.Rectangle{Width: 1920, Height: 1080}
				anchor := mygo.Point{X: 960, Y: 100}
				initial := settingsBoundsAtAnchor(anchor, 510, 430, area)
				// Sample the label in screen coordinates, accounting for the
				// native host moving left while the centered toolbar stays put.
				screenX := int(math.Floor(float64(float32(initial.X)+tab.X) * float64(scale)))
				top := int(math.Floor(float64(tab.Y+28) * float64(scale)))
				bottom := int(math.Ceil(float64(tab.Y+44) * float64(scale)))
				width := int(math.Ceil(float64(tab.W) * float64(scale)))
				pixels := func(originX int) []byte {
					img := tt.Image()
					x := screenX - originX*scale
					var out []byte
					for y := top; y < bottom; y++ {
						start := y*img.Stride + x*4
						out = append(out, img.Pix[start:start+width*4]...)
					}
					return out
				}
				want := pixels(initial.X)
				for _, w := range []int{511, 512, 513, 560, 601, 709, 710, 709, 651, 511, 510} {
					tt.SetSize(w, 430)
					origin := settingsBoundsAtAnchor(anchor, w, 430, area)
					if !bytes.Equal(pixels(origin.X), want) {
						t.Fatalf("pins label pixels changed during resize at width %d", w)
					}
				}
			})
		}
	}
}
