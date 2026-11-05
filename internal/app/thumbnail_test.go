package app

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// thumbSize keeps the aspect inside Maccy's 340×maxH box without
// enlarging small pictures.
func TestThumbSize(t *testing.T) {
	for _, tc := range []struct {
		bw, bh, maxH int
		wantW, wantH float32
	}{
		{680, 760, 40, 35.79, 40},   // tall screenshot: capped by height
		{1920, 1080, 40, 71.11, 40}, // landscape: capped by height
		{8000, 100, 40, 340, 4.25},  // panorama: capped by width
		{10, 8, 40, 10, 8},          // tiny: never enlarged
		{0, 0, 40, 0, 0},            // degenerate
		{500, 2000, 40, 10, 40},     // very tall
	} {
		w, h := thumbSize(tc.bw, tc.bh, tc.maxH)
		if w < tc.wantW-0.1 || w > tc.wantW+0.1 || h < tc.wantH-0.1 || h > tc.wantH+0.1 {
			t.Errorf("thumbSize(%d,%d,%d) = %.2f×%.2f, want %.2f×%.2f",
				tc.bw, tc.bh, tc.maxH, w, h, tc.wantW, tc.wantH)
		}
	}
}

// A row's thumbnail must sit at its leading edge, not centered. The
// flex base of an image sized only by Height is the bitmap's pixel
// width, which the row shrinks to full width and Contain then centers;
// Size(w, h) avoids it.
func TestImageRowLeadingEdge(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1200, 600))
	for y := 0; y < 600; y++ {
		for x := 0; x < 1200; x++ {
			src.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}
	bmp := ui.NewBitmap(src)
	var ls ui.ListState
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			ui.List(c, &ls, 1, func(i int) {
				row := ui.ButtonBase(c).AlignItems(ui.Center).Radius(4).FillWidth()
				row.MinHeight(50)
				row.Children(func() {
					ui.Box(c).Width(10)
					w, h := thumbSize(1200, 600, 40)
					ui.Image(c, bmp).Size(w, h).Radius(4).Margin(0, 5, 0, 0).PassThrough()
					ui.Spacer(c)
				})
			}).Grow(1)
		})
	}, 450, 60)
	pix := tt.Image()
	minX, maxX := -1, -1
	for x := 0; x < 450; x++ {
		for y := 0; y < 60; y++ {
			r, g, b, _ := pix.At(x, y).RGBA()
			if r>>8 > 200 && g>>8 < 80 && b>>8 < 80 {
				if minX < 0 {
					minX = x
				}
				maxX = x
			}
		}
	}
	if minX < 0 {
		t.Fatal("thumbnail not drawn")
	}
	// 1200×600 fits 340×40 as 80×40; at the leading edge that lands
	// near x=10, far from the row's center.
	if minX > 40 || maxX > 120 {
		t.Fatalf("thumbnail spans x=%d..%d, expected a leading-edge ~80px span", minX, maxX)
	}
}
