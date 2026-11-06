//go:build !darwin

package clipboard

// Other platforms retain the existing timer/fingerprint or sequence path.
func (w *Watcher) startNativePolling(_ <-chan struct{}) *nativePolling { return nil }
