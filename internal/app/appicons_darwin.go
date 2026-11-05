//go:build darwin

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	appIconMu    sync.Mutex
	appIconCache = map[string][]byte{}
)

// appIconPNG returns a 32x32 PNG of an app's icon, or nil.
func appIconPNG(bundleID string) []byte {
	if bundleID == "" {
		return nil
	}
	appIconMu.Lock()
	if b, ok := appIconCache[bundleID]; ok {
		appIconMu.Unlock()
		return b
	}
	appIconMu.Unlock()
	b := loadAppIcon(bundleID)
	appIconMu.Lock()
	appIconCache[bundleID] = b
	appIconMu.Unlock()
	return b
}

func loadAppIcon(id string) []byte {
	out, err := exec.Command("mdfind", "kMDItemCFBundleIdentifier == '"+id+"'").Output()
	if err != nil {
		return nil
	}
	appPath := ""
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			appPath = line
			break
		}
	}
	if appPath == "" {
		return nil
	}
	name := ""
	if b, err := exec.Command("plutil", "-extract", "CFBundleIconFile", "raw", "-o", "-", filepath.Join(appPath, "Contents", "Info.plist")).Output(); err == nil {
		name = strings.TrimSpace(string(b))
	}
	if name == "" {
		name = "AppIcon"
	}
	res := filepath.Join(appPath, "Contents", "Resources")
	for _, ext := range []string{"", ".icns", ".png"} {
		cand := filepath.Join(res, name+ext)
		if _, err := os.Stat(cand); err != nil {
			continue
		}
		if strings.HasSuffix(strings.ToLower(cand), ".png") {
			b, err := os.ReadFile(cand)
			if err == nil {
				return b
			}
			continue
		}
		tmp := filepath.Join(os.TempDir(), "maccy-icon-"+safeName(id)+".png")
		cmd := exec.Command("sips", "-s", "format", "png", cand, "--out", tmp, "--resampleHeightWidth", "32", "32")
		if err := cmd.Run(); err == nil {
			if b, err := os.ReadFile(tmp); err == nil {
				return b
			}
		}
	}
	return nil
}

func safeName(s string) string {
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(s)
}
