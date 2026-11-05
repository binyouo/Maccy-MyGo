//go:build !darwin

package clipboard

// recognizeImage is unavailable on this platform.
func recognizeImage(data []byte) string { return "" }
