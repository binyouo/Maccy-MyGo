//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestNativePopupGeometry(t *testing.T) {
	if os.Getenv("MACCY_NATIVE_GEOMETRY_TEST") != "1" {
		t.Skip("set MACCY_NATIVE_GEOMETRY_TEST=1 for the isolated native resize probe")
	}
	command := exec.Command(os.Args[0], "-test.run=^$")
	command.Env = append(os.Environ(), "MACCY_NATIVE_GEOMETRY_PROBE=1")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native geometry: %v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}

// Use real NSWindow style and resize callbacks in a hidden, isolated process.
// No global input, user settings or clipboard contents are accessed.
func probeNativePopupGeometry() error {
	dir, err := os.MkdirTemp("", "maccy-native-geometry-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	cfg, _ := config.Load(dir + "/config.json")
	_ = cfg.Update(func(c *config.Config) {
		c.WindowHeight, c.WindowWidth = 450, 450
		c.OpenPreviewAutomatically, c.ShowNotifications = false, false
	})
	store, err := model.Open(dir)
	if err != nil {
		return err
	}
	a := New(cfg, store)
	for i := 0; i < 20; i++ {
		text := fmt.Sprintf("Native resize fixture %02d", i)
		hash, size, _ := store.PutBlob([]byte(text))
		a.addItem(&model.Item{Title: text, Contents: []model.Content{{Type: "text/plain", Hash: hash, Size: size}}})
	}
	var result error
	completed := false
	mygo.App.SetName("MaccyGeometryProbe")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(func() {
		p := mygo.NewWindow(mygo.WindowOptions{Title: "hidden geometry probe", Hidden: true, Panel: true, Frameless: true,
			Width: 450, Height: 450, UseContentSize: true, Content: ui.View(a.popupViewPanel)})
		a.panel.Store(p)
		a.rememberPopupMotion(p)
		a.observePopupGeometry(p)
		a.reduceMotion = true
		fail := func(format string, args ...any) {
			result = fmt.Errorf(format, args...)
			mygo.App.Quit()
		}
		whenBounds := func(width, height int, next func()) {
			attempts := 0
			var check func()
			check = func() {
				b := p.ContentBounds()
				if b.Width == width && b.Height == height {
					next()
					return
				}
				attempts++
				if attempts > 200 {
					fail("native bounds failed to reach %dx%d: %+v", width, height, b)
					return
				}
				time.AfterFunc(5*time.Millisecond, func() { p.Update(check) })
			}
			p.Update(check)
		}
		// An external native resize enters through the same delegate callback
		// as live resizing, including the geometry persistence path.
		b := p.ContentBounds()
		b.Width, b.Height = 510, 420
		p.SetContentBounds(b)
		a.flushPopupGeometry()
		if cfg.WindowWidth != 510 || cfg.WindowHeight != 420 {
			fail("native resize was not captured: %d x %d", cfg.WindowWidth, cfg.WindowHeight)
			return
		}
		a.beginEdit(a.vitems[0])
		whenBounds(520, 380, func() {
			if cfg.WindowHeight != 420 || p.IsResizable() {
				fail("editor native style polluted preferences or stayed resizable: height=%d", cfg.WindowHeight)
				return
			}
			a.cancelEdit()
			whenBounds(510, 420, func() {
				b := p.ContentBounds()
				if cfg.WindowHeight != 420 || cfg.WindowWidth != 510 || b.Width != 510 || b.Height != 420 || !p.IsResizable() {
					fail("editor return changed saved/native dimensions: cfg=%d x %d native=%+v resizable=%v", cfg.WindowWidth, cfg.WindowHeight, b, p.IsResizable())
					return
				}
				fmt.Println("native resize persisted; editor style and return preserved geometry; hidden window only")
				completed = true
				mygo.App.Quit()
			})
		})
	})
	deadline := time.AfterFunc(10*time.Second, func() { mygo.App.Quit() })
	defer deadline.Stop()
	if err := mygo.App.Run(); err != nil {
		return err
	}
	if !completed && result == nil {
		return fmt.Errorf("native geometry probe did not complete before its deadline")
	}
	return result
}
