//go:build !darwin

package app

// appIconPNG is unavailable on this platform.
func appIconPNG(bundleID string) []byte { return nil }
