//go:build !darwin

package app

func activePopupScreenID() (int64, bool) { return 0, false }
