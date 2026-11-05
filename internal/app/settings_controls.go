package app

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"math"
	"strconv"
	"strings"
)

// Keep this palette local to preferences. The popup keeps the desktop's
// translucent material; the settings form uses macOS's window colors.
func settingsTheme(c *ui.Context) *ui.Theme {
	t := *c.Theme()
	t.FontSize, t.Spacing, t.Radius = 13, 3.5, 4
	t.Accent, t.AccentText = ui.Hex("#0a84ff"), ui.Hex("#ffffff")
	if t.Dark {
		t.Background, t.Surface = ui.Hex("#272a2e"), ui.Hex("#383b3f")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#46494d"), ui.Hex("#515459")
		t.Border, t.Text, t.TextMuted = ui.Hex("#4b4e52"), ui.Hex("#e5e5e7"), ui.Hex("#96999f")
	} else {
		t.Background, t.Surface = ui.Hex("#f2f2f2"), ui.Hex("#ffffff")
		t.SurfaceHover, t.SurfacePressed = ui.Hex("#e6e6e6"), ui.Hex("#d8d8d8")
		t.Border, t.Text, t.TextMuted = ui.Hex("#c4c4c4"), ui.Hex("#242424"), ui.Hex("#777777")
	}
	c.SetTheme(&t)
	return c.Theme()
}

func settingsButton(c *ui.Context, title string) ui.Element {
	return ui.Button(c, title).Height(20).Padding(0, 8).Radius(4).FontSize(12)
}

func settingsIgnoreTabs(c *ui.Context, selected *int) ui.Element {
	t := c.Theme()
	parts := ui.SegmentedBase(c, selected, 3)
	parts.Track.Height(22).Padding(1).Gap(1).Radius(4).Background(t.Surface).Border(1, t.Border).AlignSelf(ui.Center).Children(func() {
		for i, title := range []string{"忽略应用", "忽略剪贴板类型", "正则表达式"} {
			segment := parts.Segment(i).Padding(0, 12).Radius(3)
			if i == *selected {
				segment.Background(t.SurfacePressed)
			}
			segment.Children(func() { ui.Text(c, title).FontSize(12).SingleLine().PassThrough() })
		}
	})
	return parts.Track
}

// SelectBase owns popup dismissal, arrow keys, focus and accessibility;
// bind the actual index so no temporary string can lose a selection.
func settingsPicker(c *ui.Context, value *int, label string, width float32, choices ...string) ui.Element {
	t := c.Theme()
	sel := ui.SelectBase(c, value)
	b := sel.Trigger.Label(label).Value(choices[clampIdx(*value, len(choices))]).Width(width).Height(20).Padding(0, 0, 0, 6).
		AlignSelf(ui.Start).Justify(ui.SpaceBetween).Gap(4).Radius(4).Border(1, t.Border).Background(t.Surface)
	// Accessibility presses activate the trigger without the pointer's
	// usual focus change. Keep subsequent arrow and Enter keys here too.
	b.OnClick(func() { b.Focus() })
	b.Children(func() {
		ui.Text(c, choices[clampIdx(*value, len(choices))]).SingleLine().Grow(1).PassThrough()
		ui.Box(c).Size(16, 18).Radius(3).Background(t.Accent).PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
			var up, down ui.Path
			up.MoveTo(r.X+4, r.Y+7).LineTo(r.X+8, r.Y+3).LineTo(r.X+12, r.Y+7)
			down.MoveTo(r.X+4, r.Y+11).LineTo(r.X+8, r.Y+15).LineTo(r.X+12, r.Y+11)
			p.StrokePath(&up, 1.2, t.AccentText)
			p.StrokePath(&down, 1.2, t.AccentText)
		})
	})
	sel.Popup(func(panel ui.Element) {
		panel.Padding(3).Radius(5).Background(t.Surface).Border(1, t.Border).Shadow(0, 3, 10, 0, ui.RGBA(0, 0, 0, 0.2))
		for i, title := range choices {
			item := sel.Item(i).Label(title).Padding(3, 7).Radius(3)
			if item.Highlighted() {
				item.Background(t.Accent).TextColor(t.AccentText)
			}
			item.Children(func() { ui.Text(c, title).SingleLine().PassThrough() })
		}
	})
	return b
}

// A text field and a vertical stepper, as in Maccy's number preferences.
// The draft accepts incomplete input; only an in-range number is saved.
func settingsNumber(c *ui.Context, value *float64, label string, lo, hi float64, commit func()) ui.Element {
	t := c.Theme()
	row := ui.Row(c).Key(label).Gap(4).Height(20).AlignItems(ui.Center)
	text := ui.Local(row, "number", func() string { return fmt.Sprintf("%.0f", *value) })
	set := func(v float64) {
		v = math.Max(lo, math.Min(hi, math.Round(v)))
		if v != *value {
			*value = v
			commit()
		}
		*text = fmt.Sprintf("%.0f", *value)
	}
	row.Children(func() {
		in := ui.TextInputBase(c, text).Label(label).Width(80).Height(20).Padding(0, 5).
			Radius(4).Border(1, t.Border).Background(t.Surface)
		if in.Focused() {
			in.Border(1, t.Accent)
		} else {
			*text = fmt.Sprintf("%.0f", *value)
		}
		in.OnChange(func() {
			if v, err := strconv.ParseFloat(strings.TrimSpace(*text), 64); err == nil && v >= lo && v <= hi {
				if v != *value {
					*value = math.Round(v)
					commit()
				}
			}
		})
		in.OnTextKey(func(k ui.TextKey) bool {
			if k.Mods != 0 {
				return false
			}
			switch k.Key {
			case ui.KeyUp:
				set(*value + 1)
				return true
			case ui.KeyDown:
				set(*value - 1)
				return true
			case ui.KeyEnter:
				set(*value)
				return true
			}
			return false
		})
		ui.Column(c).Size(14, 20).Radius(3).Background(t.Surface).Border(1, t.Border).Children(func() {
			for _, direction := range []int{1, -1} {
				delta := direction
				name := "增加" + label
				if delta < 0 {
					name = "减少" + label
				}
				button := ui.ButtonBase(c).Size(14, 10).Label(name).Disabled(delta > 0 && *value >= hi || delta < 0 && *value <= lo)
				if button.Pressed() {
					button.Background(t.SurfacePressed)
				}
				button.Draw(func(p *ui.Painter, r ui.Rect) {
					mid, top := r.X+r.W/2, r.Y+3
					var arrow ui.Path
					if delta > 0 {
						arrow.MoveTo(mid-3, top+3).LineTo(mid, top).LineTo(mid+3, top+3)
					} else {
						arrow.MoveTo(mid-3, top).LineTo(mid, top+3).LineTo(mid+3, top)
					}
					p.StrokePath(&arrow, 1, t.Text)
				})
				button.OnClick(func() { set(*value + float64(delta)) })
			}
		})
	})
	return row
}
