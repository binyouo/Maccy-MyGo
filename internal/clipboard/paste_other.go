//go:build !darwin && !linux && !windows

package clipboard

// Paste is unsupported on this platform.
func Paste() error { return nil }
