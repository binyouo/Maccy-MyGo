//go:build darwin

package app

import (
	"fmt"
	"github.com/egoist/mygo"
	"os"
	"path/filepath"
)

// applyLaunchAtLogin installs or removes a LaunchAgent for this executable.
func (a *App) applyLaunchAtLogin(enabled bool) {
	if !mygo.App.IsReady() {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, "Library", "LaunchAgents")
	plist := filepath.Join(dir, "io.github.binyouo.maccy-mygo.plist")
	if !enabled {
		_ = os.Remove(plist)
		return
	}
	_ = os.MkdirAll(dir, 0o755)
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>io.github.binyouo.maccy-mygo</string>
  <key>ProgramArguments</key><array><string>%s</string></array>
  <key>RunAtLoad</key><true/>
  <key>ProcessType</key><string>Interactive</string>
</dict>
</plist>
`, exe)
	_ = os.WriteFile(plist, []byte(content), 0o644)
}
