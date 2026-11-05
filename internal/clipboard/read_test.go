package clipboard

import (
	"strings"
	"testing"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/egoist/mygo/transfer"
	"github.com/rivo/uniseg"
)

func TestCaptureNegotiationDoesNotReadConcealedPayload(t *testing.T) {
	for _, ignored := range []string{"org.nspasteboard.ConcealedType", "org.example.Private"} {
		cfg := *config.Default()
		cfg.IgnoredPasteboardTypes = []string{"org.example.Private"}
		called := false
		_, accepted, err := readForCapture(cfg, []transfer.Format{transfer.Text, transfer.Format(ignored)}, func(...transfer.Format) (transfer.Data, error) {
			called = true
			return transfer.TextData("private data"), nil
		})
		if err != nil || accepted || called {
			t.Fatalf("ignored copy requested native data: accepted=%v read=%v err=%v", accepted, called, err)
		}
	}
}

func TestCaptureNegotiationSkipsDisabledAndUnpersistableProviders(t *testing.T) {
	cfg := *config.Default()
	cfg.SaveImages, cfg.SaveFiles = false, false
	read := map[transfer.Format]bool{}
	var reps []transfer.Representation
	for _, format := range []transfer.Format{transfer.Text, transfer.PNG, transfer.FileList, "com.apple.webarchive", "dyn.example", "com.microsoft.ole.source.large", "org.example.Metadata"} {
		reps = append(reps, transfer.Lazy(format, func() ([]byte, error) { read[format] = true; return []byte("copy"), nil }))
	}
	data := transfer.New(transfer.NewItem(reps...))
	got, accepted, err := readForCapture(cfg, data.Formats(), func(formats ...transfer.Format) (transfer.Data, error) { return data.Materialize(formats) })
	if err != nil || !accepted {
		t.Fatalf("read: %v %v", accepted, err)
	}
	it, ok, err := Build(clipboardStore(t), cfg, got)
	if err != nil || !ok || it.Title != "copy" {
		t.Fatalf("accepted payload lost: %#v %v %v", it, ok, err)
	}
	defer it.ReleaseBlobs()
	if len(read) != 2 || !read[transfer.Text] || !read["org.example.Metadata"] {
		t.Fatalf("disabled/unpersistable native providers were requested: %v", read)
	}
}

func TestCaptureNegotiationKeepsBlankStringFilterBeforeDisabledText(t *testing.T) {
	cfg := *config.Default()
	cfg.SaveText = false
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte(" \n")), transfer.Bytes(transfer.PNG, []byte("image"))))
	got, accepted, err := readForCapture(cfg, data.Formats(), func(formats ...transfer.Format) (transfer.Data, error) { return data.Materialize(formats) })
	if err != nil || !accepted {
		t.Fatal("enabled image copy was not negotiated")
	}
	if _, ok, _ := Build(clipboardStore(t), cfg, got); ok {
		t.Fatal("selective read bypassed Swift's blank-string item filter")
	}
}

func TestTitleLimitCountsCompleteSwiftCharacters(t *testing.T) {
	store := clipboardStore(t)
	body := strings.Repeat("👩‍💻", 999) + "e\u0301tail"
	it := buildCopy(t, store, *config.Default(), transfer.NewItem(transfer.Bytes(transfer.Text, []byte(body))))
	defer it.ReleaseBlobs()
	want := strings.Repeat("👩‍💻", 999) + "e\u0301"
	if it.Title != want || uniseg.GraphemeClusterCount(it.Title) != 1000 || Text(store, it) != body {
		t.Fatal("title truncation split a joined emoji/combining character or truncated original content")
	}
}
