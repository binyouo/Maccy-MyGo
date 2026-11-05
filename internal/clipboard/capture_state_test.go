package clipboard

import (
	"testing"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
	"github.com/egoist/mygo/transfer"
)

func TestOneCopyPauseImmediatelyNotifiesRecordingResumed(t *testing.T) {
	dir := t.TempDir()
	cfg, _ := config.Load(dir + "/config.json")
	store, err := model.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = cfg.Update(func(c *config.Config) { c.IgnoreEvents, c.IgnoreOnlyNextEvent = true, true })
	w := New(cfg, store)
	resumed, copies := 0, 0
	w.OnCaptureState = func() { resumed++ }
	w.OnNew = func(it *model.Item) { copies++; it.ReleaseBlobs() }
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Format("text/plain"), []byte("fixture"))))
	w.capture(data, 1, true, "", "")
	if resumed != 1 || copies != 0 || cfg.Snapshot().IgnoreEvents {
		t.Fatalf("one-copy pause state: resumed=%d captured=%d ignored=%v", resumed, copies, cfg.Snapshot().IgnoreEvents)
	}
	w.capture(data, 2, true, "", "")
	if resumed != 1 || copies != 1 {
		t.Fatalf("recording did not resume exactly once: resumed=%d captured=%d", resumed, copies)
	}
}
