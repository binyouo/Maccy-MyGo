package app

import (
	"bytes"
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
	"github.com/binyouo/Maccy-MyGo/internal/search"
)

func TestHistoryMatchStylesActuallyPaintText(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprint("dark=", dark), func(t *testing.T) {
			a := newTestApp(t)
			item := &model.Item{ID: "match", Title: "Search match"}
			theme := ui.LightTheme()
			if dark {
				theme = ui.DarkTheme()
			}
			render := func(style config.HighlightMatch, matched bool) *image.RGBA {
				s := popupSnapshot{cfg: *config.Default(), items: []*model.Item{item}, selected: -1}
				s.cfg.HighlightMatch = style
				if matched {
					s.ranges = map[string][]search.Range{item.ID: {{Start: 0, End: len([]rune(item.Title))}}}
				}
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme)
					ui.Column(c).Fill().Background(theme.Background).Padding(8).Children(func() {
						var chosen func()
						hovered := -1
						a.row(c, c.Theme(), s, 0, &chosen, &hovered)
					})
				}, 240, 60)
				return tt.Image()
			}
			plain := render(config.HighlightBold, false)
			for _, style := range []config.HighlightMatch{config.HighlightBold, config.HighlightItalic, config.HighlightUnderline, config.HighlightColor} {
				painted := render(style, true)
				if bytes.Equal(painted.Pix, plain.Pix) {
					t.Fatalf("%s highlight did not change the row's rendered text", style)
				}
				if style == config.HighlightColor {
					var yellow, black int
					for y := painted.Rect.Min.Y; y < painted.Rect.Max.Y; y++ {
						for x := painted.Rect.Min.X; x < painted.Rect.Max.X; x++ {
							p := painted.RGBAAt(x, y)
							if p.R > 240 && p.G > 240 && p.B < 20 {
								yellow++
							}
							if p.R < 10 && p.G < 10 && p.B < 10 && p.A > 240 {
								black++
							}
						}
					}
					if yellow < 100 || black < 10 {
						t.Fatalf("color style did not paint a yellow background and black text: yellow=%d black=%d", yellow, black)
					}
				}
			}
		})
	}
}

func TestToolbarHelpTracksConfiguredShortcuts(t *testing.T) {
	for _, tc := range []struct {
		label, want string
		set         func(*config.Config)
	}{
		{"置顶/取消置顶", "置顶（⌃⇧B）", func(c *config.Config) { c.PinShortcut = "Ctrl+Shift+B" }},
		{"删除条目", "删除（⌘⌦）", func(c *config.Config) { c.DeleteShortcut = "Cmd+ForwardDelete" }},
		{"切换预览", "切换预览（⌥V）", func(c *config.Config) { c.PreviewShortcut = "Option+V" }},
		{"置顶/取消置顶", "置顶", func(c *config.Config) { c.PinShortcut = "" }},
		{"删除条目", "删除", func(c *config.Config) { c.DeleteShortcut = "" }},
		{"切换预览", "切换预览", func(c *config.Config) { c.PreviewShortcut = "" }},
	} {
		t.Run(tc.want, func(t *testing.T) {
			a := popupTestApp(t)
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Padding(20).Children(func() {
					a.header(c, c.Theme(), a.snapshotPopup())
					a.previewToolbar(c, c.Theme(), a.vitems[0])
				})
			}, 480, 180)
			// Change the setting after the view exists: help must follow
			// the current binding rather than its initial/default value.
			if err := a.cfg.Update(tc.set); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			r, ok := tt.Find(tc.label)
			if !ok {
				t.Fatalf("button %q not found", tc.label)
			}
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Advance(700 * time.Millisecond)
			if !tt.HasText(tc.want) {
				t.Fatalf("wanted help %q, got %q", tc.want, tt.Texts())
			}
		})
	}
}
