//go:build linux

package clipboard

import "os/exec"

// Paste best-effort types the paste chord with xdotool, when installed.
func Paste() error {
	if _, err := exec.LookPath("xdotool"); err != nil {
		return err
	}
	return exec.Command("xdotool", "key", "ctrl+v").Run()
}
