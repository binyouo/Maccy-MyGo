//go:build darwin

package app

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// bundleIDForApp reads an app bundle's CFBundleIdentifier.
func bundleIDForApp(path string) string {
	out, err := exec.Command("plutil", "-extract", "CFBundleIdentifier", "raw", "-o", "-", filepath.Join(path, "Contents", "Info.plist")).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
