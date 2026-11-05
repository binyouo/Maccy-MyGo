package clipboard

import (
	"testing"

	"github.com/egoist/mygo/transfer"

	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

func TestBuildAndRestore(t *testing.T) {
	dir := t.TempDir()
	store, err := model.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	data := transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte("hello world")),
		transfer.Bytes(transfer.HTML, []byte("<b>hello world</b>")),
	))
	cfg := *config.Default()
	it, ok, err := Build(store, cfg, data)
	if err != nil || !ok {
		t.Fatalf("build failed: ok=%v err=%v", ok, err)
	}
	if it.Title != "hello world" {
		t.Fatalf("title = %q", it.Title)
	}
	if len(it.Contents) != 2 {
		t.Fatalf("want 2 contents, got %d", len(it.Contents))
	}
	restored, err := Data(store, it, false)
	if err != nil {
		t.Fatal(err)
	}
	text, err := restored.Read(transfer.Text)
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "hello world" {
		t.Fatalf("restored text = %q", string(text))
	}
}

func TestBuildIgnoresBlankText(t *testing.T) {
	dir := t.TempDir()
	store, _ := model.Open(dir)
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("   \n "))))
	cfg := *config.Default()
	if _, ok, _ := Build(store, cfg, data); ok {
		t.Fatal("blank text should be ignored")
	}
}

func TestBuildRespectsDisabledText(t *testing.T) {
	dir := t.TempDir()
	store, _ := model.Open(dir)
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("hi"))))
	cfg := *config.Default()
	cfg.SaveText = false
	if _, ok, _ := Build(store, cfg, data); ok {
		t.Fatal("text should be ignored when saving text is off")
	}
}

func TestRemoveFormatting(t *testing.T) {
	dir := t.TempDir()
	store, _ := model.Open(dir)
	data := transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte("bold text")),
		transfer.Bytes(transfer.HTML, []byte("<b>bold text</b>")),
	))
	cfg := *config.Default()
	it, ok, _ := Build(store, cfg, data)
	if !ok {
		t.Fatal("build failed")
	}
	plain, err := Data(store, it, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range plain.Formats() {
		if f == transfer.HTML {
			t.Fatal("HTML should be removed")
		}
	}
}
