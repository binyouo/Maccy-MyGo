//go:build darwin

package clipboard

import (
	"os/exec"
	"strings"
)

// FrontmostApp returns the bundle identifier and display name of the
// frontmost application.
func FrontmostApp() (string, string) {
	asn := runOut("lsappinfo", "front")
	if asn == "" {
		return "", ""
	}
	id := plistValue(runOut("lsappinfo", "info", "-only", "bundleid", asn))
	name := plistValue(runOut("lsappinfo", "info", "-only", "name", asn))
	return id, name
}

func runOut(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// plistValue extracts the value from a line like KEY="value".
func plistValue(line string) string {
	line = strings.TrimSpace(line)
	if i := strings.IndexByte(line, '='); i >= 0 {
		line = line[i+1:]
	}
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "\"")
	if line == "[ NULL ]" || strings.EqualFold(line, "null") {
		return ""
	}
	return line
}
