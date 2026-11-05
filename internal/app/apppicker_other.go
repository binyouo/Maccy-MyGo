//go:build !darwin

package app

func bundleIDForApp(path string) string { return "" }
