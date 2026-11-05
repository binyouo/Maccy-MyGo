package app

import "time"

func (a *App) selectedPreviewID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.footerIndex >= 0 || a.selected < 0 || a.selected >= len(a.vitems) {
		return ""
	}
	return a.vitems[a.selected].ID
}

// NavigationManager in Maccy resets suppression when the lead history
// item changes. Returning from the footer counts as a new selection too.
func (a *App) syncAutoPreviewSelection() {
	if a.selectedPreviewID() != a.previewSelectionID {
		a.scheduleAutoPreview()
	}
}

func (a *App) cancelAutoPreview() {
	if a.previewAutoQueuedFor != "" {
		a.previewQueued = false
		a.previewAutoQueuedFor = ""
	}
	if a.previewTimer != nil {
		a.previewGen.Add(1)
		a.previewTimer.Stop()
		a.previewTimer = nil
	}
}

// Cancel before checking eligibility, as SlideoutController.startAutoOpen
// does: losing a selection or disabling the preference cancels old work.
func (a *App) scheduleAutoPreview() {
	itemID := a.selectedPreviewID()
	if itemID != a.previewSelectionID {
		a.previewSelectionID = itemID
		if itemID != "" {
			a.previewAutoSuppressed = false
		}
	}
	a.cancelAutoPreview()
	cfg := a.cfg.Snapshot()
	if itemID == "" || !cfg.OpenPreviewAutomatically || a.previewOpen || a.previewAutoSuppressed || a.previewQueued || a.editing != nil {
		return
	}
	gen := a.previewGen.Add(1)
	a.previewTimer = time.AfterFunc(time.Duration(max(200, cfg.PreviewDelay))*time.Millisecond, func() {
		a.post(func() {
			// Only inspect UI state after dispatching back to its host.
			if (a.panel.Load() == nil && a.devWin == nil) || a.previewGen.Load() != gen {
				return
			}
			a.previewTimer = nil
			if !a.cfg.Snapshot().OpenPreviewAutomatically || a.selectedPreviewID() != itemID || a.editing != nil || a.previewOpen || a.previewAutoSuppressed || a.previewQueued {
				return
			}
			// Share the manual request queue so auto-open cannot reverse a
			// closing animation or open a pane while the host is resizing.
			a.previewQueued = true
			a.previewAutoQueuedFor = itemID
			a.applyQueuedPreview()
			a.refresh()
		})
	})
}
