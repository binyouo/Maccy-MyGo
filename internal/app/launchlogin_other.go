//go:build !darwin

package app

// applyLaunchAtLogin is unsupported on this platform.
func (a *App) applyLaunchAtLogin(enabled bool) {}
