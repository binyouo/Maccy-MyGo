//go:build darwin

package clipboard

import (
	"os/exec"
)

// Paste best-effort types the platform's paste chord into the frontmost app.
// It needs Accessibility permission; errors are swallowed.
func Paste() error {
	return exec.Command("osascript", "-e", `tell application "System Events" to keystroke "v" using command down`).Run()
}
