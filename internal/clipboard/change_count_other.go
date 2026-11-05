//go:build !darwin && !windows

package clipboard

// Other backends retain the structural fingerprint until they expose a
// portable clipboard ownership/change sequence.
func ChangeCount() (int64, bool) { return 0, false }
