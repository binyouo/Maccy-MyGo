package app

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestStatusItemOptionClicksControlCapture(t *testing.T) {
	a := popupTestApp(t)
	a.statusItemClick(ui.Alt)
	if cfg := a.cfg.Snapshot(); !cfg.IgnoreEvents || cfg.IgnoreOnlyNextEvent {
		t.Fatal("Option-click did not pause capture")
	}
	a.statusItemClick(ui.Alt)
	if a.cfg.Snapshot().IgnoreEvents {
		t.Fatal("second Option-click did not resume capture")
	}
	a.statusItemClick(ui.Alt | ui.Shift)
	if cfg := a.cfg.Snapshot(); !cfg.IgnoreEvents || !cfg.IgnoreOnlyNextEvent {
		t.Fatal("Option-Shift click did not ignore the next copy")
	}
	if !a.cfg.ConsumeIgnoreNext() || a.cfg.Snapshot().IgnoreEvents {
		t.Fatal("next copy did not consume the temporary pause")
	}
	a.statusItemClick(ui.Alt | ui.Shift)
	a.statusItemClick(ui.Alt | ui.Shift)
	if cfg := a.cfg.Snapshot(); cfg.IgnoreEvents || cfg.IgnoreOnlyNextEvent {
		t.Fatal("repeating Option-Shift click did not cancel the temporary pause")
	}
}

func TestOriginalMenuIconsAreDistinct(t *testing.T) {
	names := []string{"maccy", "clipboard", "scissors", "paperclip"}
	for i, name := range names {
		icon := trayIconFor(name)
		if len(icon) == 0 {
			t.Fatalf("missing %s icon", name)
		}
		for _, other := range names[:i] {
			if bytes.Equal(icon, trayIconFor(other)) {
				t.Fatalf("%s and %s use the same asset", name, other)
			}
		}
	}
}

func TestHistoryClickPassesExactModifiers(t *testing.T) {
	for _, mods := range []ui.Modifiers{0, ui.Alt, ui.Alt | ui.Shift, ui.Ctrl | ui.Alt, ui.Shift} {
		t.Run(fmt.Sprint(mods), func(t *testing.T) {
			a := popupTestApp(t)
			tt := ui.NewTester(a.popupViewPanel, 450, 300)
			want := a.vitems[0]
			if err := tt.ClickWith(mods, want.Title); err != nil {
				t.Fatal(err)
			}
			valid := mods == 0 || itemAction(a.cfg.Snapshot(), mods) != actUnknown
			if (a.lastCopiedID == want.ID) != valid {
				t.Fatalf("click with %v executed=%v want=%v", mods, a.lastCopiedID == want.ID, valid)
			}
		})
	}
}
