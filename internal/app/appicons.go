package app

import "github.com/egoist/mygo/ui"

// appIcon returns the cached application icon bitmap, loading it in the
// background the first time so the view never blocks on the filesystem.
func (a *App) appIcon(bundleID string) *ui.Bitmap {
	a.appIconMu.Lock()
	if b, ok := a.appIcons[bundleID]; ok {
		a.appIconMu.Unlock()
		return b
	}
	if a.appIconLoading[bundleID] {
		a.appIconMu.Unlock()
		return nil
	}
	a.appIconLoading[bundleID] = true
	a.appIconMu.Unlock()
	go func() {
		png := appIconPNG(bundleID)
		a.post(func() {
			a.appIconMu.Lock()
			delete(a.appIconLoading, bundleID)
			if png == nil {
				a.appIcons[bundleID] = nil
				a.appIconMu.Unlock()
				return
			}
			if bmp, err := ui.DecodeBitmap(png); err == nil {
				a.appIcons[bundleID] = bmp
			} else {
				a.appIcons[bundleID] = nil
			}
			a.appIconMu.Unlock()
			a.refresh()
		})
	}()
	return nil
}
