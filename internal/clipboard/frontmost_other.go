//go:build !darwin

package clipboard

// FrontmostApp is unavailable on this platform.
func FrontmostApp() (string, string) { return "", "" }
