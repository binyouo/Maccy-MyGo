//go:build darwin

package clipboard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// recognizeImage runs the bundled Vision OCR helper on PNG data.
func recognizeImage(data []byte) string {
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil || dir == "" {
		return ""
	}
	helper := filepath.Join(dir, "ocr")
	if _, err := os.Stat(helper); err != nil {
		return ""
	}
	f, err := os.CreateTemp("", "maccy-ocr-*.png")
	if err != nil {
		return ""
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return ""
	}
	f.Close()
	out, err := exec.Command(helper, name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
