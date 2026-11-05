// Package app implements the Maccy clone: a menu bar clipboard manager.
package app

import (
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/clipboard"
	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
	"github.com/binyouo/Maccy-MyGo/internal/search"
)

// App is the running application state.
type App struct {
	cfg     *config.Config
	store   *model.Store
	watcher *clipboard.Watcher
	tray    *mygo.Tray

	panel      atomic.Pointer[mygo.Window]
	dispatcher *mygo.Window

	mu                  sync.Mutex
	items               []*model.Item
	clipboardSessionLog map[int64]*model.Item
	vitems              []*model.Item
	ranges              map[string][]search.Range
	shortcuts           map[string]string
	pinnedCount         int
	selected            int
	footerIndex         int
	footerClearAll      bool
	query               string
	list                ui.ListState
	devList             ui.ListState
	scrollToSelected    bool
	kbdNav              bool
	lastHovered         int

	images           map[string]*ui.Bitmap
	imageBytes       map[string]int64
	imageFailedAt    map[string]time.Time
	appIcons         map[string]*ui.Bitmap
	appIconLoading   map[string]bool
	imageLoading     map[string]bool
	imageMu          sync.Mutex
	appIconMu        sync.Mutex
	lastPopupW       int
	lastPopupH       int
	lastPopupMode    int
	popupResizeGen   uint64
	popupResizeTimer *time.Timer
	popupMotions     map[*mygo.Window]*popupWindowMotion
	popupGeometry    popupGeometry
	dividerDrag      popupDividerDrag
	dividerHandle    ui.Handle
	reduceMotion     bool
	focusSearch      bool
	searchHeight     int
	lastCopiedID     string
	trayIconName     string
	trayDisabled     bool
	currentShortcut  string

	// Main-thread only.
	settingsWin            *mygo.Window
	devWin                 *mygo.Window
	aboutWin               *mygo.Window
	aboutBitmap            *ui.Bitmap
	prefs                  prefsDraft
	prefsTab               int
	prefsScreens           []string
	prefsWindowAnchor      mygo.Point
	prefsWindowBounds      mygo.Rectangle
	prefsWindowAnchored    bool
	prefsMotion            *settingsWindowMotion
	prefsResizeTimer       *time.Timer
	prefsResizeGen         uint64
	prefsReduceMotion      bool
	prefsIgnoreTab         int
	prefsRecordingShortcut string
	prefsNewType           string
	prefsNewRegexp         string
	prefsRegexpError       string
	prefsPinDrafts         map[string]*pinDraft
	justOpened             time.Time
	suppressClose          bool
	previewOpen            bool
	previewClosing         bool
	previewQueued          bool
	previewAutoSuppressed  bool
	previewSelectionID     string
	previewAutoQueuedFor   string
	editing                *model.Item
	editTitle              string
	editContent            string
	editOrig               string
	editPin                string
	editPinChoices         []string
	editIsText             bool
	editTooLarge           bool
	lastX, lastY           int
	hasLast                bool
	previewGen             atomic.Int64
	previewTimer           *time.Timer
	autoUpdates            bool
	popupPinAccel          string
	popupHotkey            *popupHotkeySession
}

// New creates the app.
func New(cfg *config.Config, store *model.Store) *App {
	a := &App{cfg: cfg, store: store, images: map[string]*ui.Bitmap{}, appIcons: map[string]*ui.Bitmap{}, appIconLoading: map[string]bool{}, imageLoading: map[string]bool{}, footerIndex: -1, lastHovered: -1, kbdNav: true}
	a.items = store.Items
	a.reconcilePinsLocked(cfg)
	_ = cfg.Save()
	a.trimLocked()
	a.saveLocked()
	a.rebuildLocked()
	return a
}

// Start sets up the tray, menus, shortcut and clipboard watcher. It must run
// once the application is ready.
func (a *App) Start() {
	a.developmentScreen()
	if os.Getenv("MACCY_PREVIEW") != "" {
		a.previewOpen = true
	}
	if q := os.Getenv("MACCY_QUERY"); q != "" {
		a.query = q
		a.mu.Lock()
		a.rebuildLocked()
		a.mu.Unlock()
	}
	if v := os.Getenv("MACCY_HIGHLIGHT"); v != "" {
		_ = a.cfg.Update(func(c *config.Config) { c.HighlightMatch = config.HighlightMatch(v) })
	}
	if os.Getenv("MACCY_FOOTER") != "" {
		a.footerIndex = 0
		a.selected = -1
		a.mu.Lock()
		a.rebuildLocked()
		a.mu.Unlock()
	}
	if os.Getenv("MACCY_APPICONS") != "" {
		_ = a.cfg.Update(func(c *config.Config) { c.ShowApplicationIcons = true })
	}
	if os.Getenv("MACCY_EDIT") != "" {
		a.mu.Lock()
		var first *model.Item
		if len(a.vitems) > 0 {
			first = a.vitems[0]
		}
		a.mu.Unlock()
		if first != nil {
			a.beginEdit(first)
		}
	}
	a.setupMenu()
	if !a.hasDevelopmentScreen() && a.cfg.Snapshot().ShowInStatusBar {
		a.setupTray()
	}
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		a.stopPopupHotkeySession(false)
		a.flushPopupGeometry()
		a.unregisterPopupShortcuts()
		a.cancelAutoPreview()
		a.cancelPopupResize()
		a.cancelSettingsTransition()
		if a.watcher != nil {
			a.watcher.Stop()
		}
		if a.cfg.Snapshot().ClearOnQuit {
			a.clearHistory(false)
		}
		if a.tray != nil {
			a.tray.Destroy()
		}
	})
	a.applyShortcut(a.cfg.Snapshot().GlobalShortcut)

	a.watcher = clipboard.New(a.cfg, a.store)
	a.watcher.OnCaptureState = func() { a.post(a.refresh) }
	a.watcher.OnNew = func(it *model.Item) {
		a.post(func() {
			defer it.ReleaseBlobs()
			a.addItem(it)
			a.refresh()
		})
	}
	a.watcher.OnTitle = func(id, title string) {
		a.post(func() {
			a.setItemTitle(id, title)
			a.refresh()
		})
	}
	a.watcher.Start()

	a.autoUpdates = updater.AutomaticChecks()
	updater.OnChange(func() {
		a.autoUpdates = updater.AutomaticChecks()
		a.refresh()
	})

	// A hidden window gives us a handle to run work on the main thread.
	a.dispatcher = mygo.NewWindow(mygo.WindowOptions{
		Title:  "Maccy MyGo",
		Width:  1,
		Height: 1,
		Hidden: true,
		Content: ui.View(func(c *ui.Context) {
			c.Root().Background(ui.Transparent)
		}),
	})

	// Development aid: open the preferences window.
	if os.Getenv("MACCY_PREFS") != "" {
		a.OpenPreferences()
	}

	// Development aid: show the popup UI in an ordinary window.
	if os.Getenv("MACCY_WINDOW") != "" {
		devW, _ := a.popupSizeLocked()
		devH := a.cfg.Snapshot().WindowHeight
		if devH <= 0 {
			devH = 560
		}
		a.devWin = mygo.NewWindow(mygo.WindowOptions{
			Title:           "Maccy (dev)",
			Width:           devW,
			Height:          devH,
			AlwaysOnTop:     true,
			BackgroundColor: "light-dark(#f5f5f7, #1c1c1e)",
			Content:         ui.View(func(c *ui.Context) { a.popupView(c, &a.devList) }),
			Hidden:          a.hasDevelopmentScreen(),
		})
		a.showOnDevelopmentScreen(a.devWin)
		a.rememberPopupMotion(a.devWin)
		a.observePopupGeometry(a.devWin)
		a.lastPopupMode = a.popupMode()
	}

	// Development aid: open the popup shortly after launch for testing.
	if os.Getenv("MACCY_AUTOSHOW") != "" {
		time.AfterFunc(1500*time.Millisecond, func() {
			a.post(a.openPopup)
		})
	}
}

// post runs fn on the UI (main) thread.
func (a *App) post(fn func()) {
	if p := a.panel.Load(); p != nil {
		p.Update(fn)
		return
	}
	if a.dispatcher != nil {
		a.dispatcher.Update(fn)
		return
	}
	fn()
}

// refresh rebuilds and redraws the open popup.
func (a *App) refresh() {
	a.syncAutoPreviewSelection()
	a.updateTray()
	a.reflow()
	if p := a.panel.Load(); p != nil {
		p.Invalidate()
	}
	if a.devWin != nil {
		a.devWin.Invalidate()
	}
	if a.settingsWin != nil {
		a.settingsWin.Invalidate()
	}
}

// updateTray updates the tray title with the most recent copy.
func (a *App) updateTray() {
	if a.tray == nil {
		return
	}
	cfg := a.cfg.Snapshot()
	a.updateTrayCaptureState(cfg)
	if !cfg.ShowRecentCopyInMenuBar {
		a.tray.SetTitle("")
		return
	}
	title := a.recentTitle()
	a.tray.SetTitle(title)
}

func (a *App) recentTitle() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.cfg.Snapshot()
	for _, it := range orderItems(a.items, cfg) {
		if !it.IsPinned() {
			t := strings.TrimSpace(firstCharacters(a.itemText(it), 100))
			t = strings.Map(func(r rune) rune {
				switch r {
				case '\n', '\r', '\v', '\f', '\u0085', '\u2028', '\u2029':
					return -1
				}
				return r
			}, t)
			return firstCharacters(t, 20)
		}
	}
	return ""
}

func (a *App) currentPanel() *mygo.Window { return a.panel.Load() }

// TogglePopup shows or hides the popup.
func (a *App) TogglePopup() {
	if a.panel.Load() != nil {
		a.closePopup()
		return
	}
	a.openPopup()
}

func (a *App) openPopup() { a.openPopupAt(a.cfg.Snapshot().PopupPosition) }

func (a *App) openPopupAt(position config.PopupPosition) {
	a.previewAutoSuppressed = false
	a.previewQueued = false
	a.previewSelectionID = ""
	a.mu.Lock()
	a.footerIndex = -1
	a.rebuildLocked()
	w, h := a.popupSizeLocked()
	cfg := a.cfg.Snapshot()
	cfg.PopupPosition = position
	a.mu.Unlock()
	if !a.hasLast && cfg.HasPopupPosition {
		a.lastX, a.lastY, a.hasLast = cfg.PopupX, cfg.PopupY, true
	}

	p := mygo.NewWindow(mygo.WindowOptions{
		Title:             "Maccy MyGo",
		Width:             w,
		Height:            h,
		UseContentSize:    true,
		Frameless:         true,
		Transparent:       true,
		Vibrancy:          mygo.VibrancyPopover,
		CornerRadius:      12,
		Panel:             true,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		MinWidth:          popupMinimumPaneWidth,
		MinHeight:         120,
		DisableMove:       position == config.PopupStatusItem,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		Hidden:            true,
		Content:           ui.View(func(c *ui.Context) { a.popupViewPanel(c) }),
	})
	a.panel.Store(p)
	a.justOpened = time.Now()
	a.kbdNav = true
	a.lastHovered = -1
	p.OnClosed(func() {
		a.stopPopupHotkeySessionFor(p, true)
		a.flushPopupGeometry()
		a.dividerDrag = popupDividerDrag{}
		a.cancelPopupResize()
		a.cancelAutoPreview()
		a.previewSelectionID = ""
		a.previewClosing = false
		a.previewQueued = false
		a.searchHeight = 0
		a.unregisterPopupShortcuts()
		x, y := p.Position()
		if m := a.popupMotions[p]; m != nil {
			x, y = m.placement.anchor.X, m.placement.anchor.Y
		}
		delete(a.popupMotions, p)
		a.previewOpen, a.editing = false, nil
		a.rememberLastPopupPosition(x, y, a.popupConfig().WindowWidth)
		a.panel.CompareAndSwap(p, nil)
		a.lastPopupW, a.lastPopupH = 0, 0
		a.mu.Lock()
		a.query = ""
		a.selected = 0
		a.footerIndex = -1
		a.rebuildLocked()
		a.mu.Unlock()
	})
	p.OnBlur(func() {
		if os.Getenv("MACCY_KEEPOPEN") != "" {
			return
		}
		if a.suppressClose || a.editing != nil || time.Since(a.justOpened) < 200*time.Millisecond {
			return
		}
		a.closePopup()
	})
	x, y := a.popupOrigin(cfg, w, h)
	p.SetPosition(x, y)
	a.rememberPopupMotion(p)
	a.observePopupGeometry(p)
	a.lastPopupW, a.lastPopupH = w, h
	a.lastPopupMode = a.popupMode()
	p.Show()
	p.Focus()
	a.registerPopupShortcuts()
	a.scrollToSelected = true
	a.scheduleAutoPreview()
	if os.Getenv("MACCY_DEBUG") != "" {
		log.Printf("maccy: popup n=%d size=%dx%d bounds=%+v", len(a.vitems), w, h, p.Bounds())
	}
}

// popupSizeLocked computes the popup size from the visible rows.
func (a *App) popupSizeLocked() (int, int) {
	cfg := a.popupConfig()
	header := 28 // the 23pt field plus Maccy's 5pt top padding
	searchVisible := cfg.ShowSearch && (cfg.SearchVisibility != config.SearchDuringSearch || a.query != "")
	if !searchVisible {
		header = 2
	}
	content := 0
	for _, it := range a.vitems {
		rowH := popupRowHeight
		if clipboard.HasImage(it) {
			if mh := cfg.ImageMaxHeight + 10; mh > rowH {
				rowH = mh
			}
		}
		content += rowH
	}
	if minRows := 3 * popupRowHeight; content < minRows {
		content = minRows
	}
	// The divider between the pinned block and the rest.
	pinSep := 0
	pins := a.pinnedCount
	if cfg.PinTo == config.PinBottom {
		pins = 0
		for i := len(a.vitems) - 1; i >= 0 && a.vitems[i].IsPinned(); i-- {
			pins++
		}
	}
	if pins > 0 && pins < len(a.vitems) {
		pinSep = popupSepPad + 1
	}
	footer := 0
	if cfg.ShowFooter {
		footer = popupSepPad + 1 + len(a.footerActions())*popupRowHeight + popupPadX
	}
	h := header + pinSep + content + footer
	if h < 120 {
		h = 120
	}
	if a.previewOpen && len(a.vitems) > 0 && h < 150 {
		h = 150
	}
	if cfg.WindowHeight > 0 && h > cfg.WindowHeight {
		h = cfg.WindowHeight
	}
	// Filtering changes the list's contents, not the size under the
	// user's hands. Release the reserved height when the search clears.
	if a.query != "" && a.searchHeight > 0 {
		h = a.searchHeight
	}
	w := cfg.WindowWidth
	if w <= 0 {
		w = 450
	}
	if a.previewOpen {
		pw := cfg.PreviewWidth
		if pw <= 0 {
			pw = 400
		}
		w += pw + 1
	}
	if a.editing != nil {
		if w < 520 {
			w = 520
		}
		h = 380
	}
	return w, h
}

// reflow resizes the open popup to fit its rows and keeps it on its
// display: widening it (the preview, the editor) must not push it off a
// screen edge.
func (a *App) reflow() {
	a.mu.Lock()
	w, h := a.popupSizeLocked()
	a.mu.Unlock()
	if w == a.lastPopupW && h == a.lastPopupH {
		return
	}
	a.lastPopupW, a.lastPopupH = w, h
	mode := a.popupMode()
	animate := mode != a.lastPopupMode
	a.lastPopupMode = mode
	a.resizePopup(w, h, animate)
}

// popupOrigin returns where to place the popup for the configured position.
func (a *App) popupOrigin(cfg config.Config, w, h int) (int, int) {
	if wa, ok := a.developmentScreen(); ok {
		return developmentOrigin(wa, w, h)
	}
	switch cfg.PopupPosition {
	case config.PopupStatusItem:
		if a.tray != nil {
			r := a.tray.Bounds()
			if r.Width > 0 {
				return constrainToDisplay(r.X, r.Y+r.Height, w, h, mygo.Point{X: r.X, Y: r.Y})
			}
		}
	case config.PopupWindow:
		if fr, ok := frontmostWindowRect(); ok {
			return fr.X + (fr.Width-w)/2, fr.Y + (fr.Height-h)/2
		}
	case config.PopupCenter:
		wa := a.targetWorkArea(cfg)
		return wa.X + (wa.Width-w)/2, wa.Y + (wa.Height-h)/2
	case config.PopupLast:
		if cfg.HasRelativePopupPosition {
			return relativePopupOrigin(a.targetWorkArea(cfg), w, h, cfg.PopupAnchorX, cfg.PopupAnchorY)
		}
		if a.hasLast {
			wa := a.targetWorkArea(cfg)
			return constrainToRect(a.lastX, a.lastY, w, h, wa)
		}
		return relativePopupOrigin(a.targetWorkArea(cfg), w, h, 0.5, 0.2)
	}
	pt := mygo.Screen.CursorScreenPoint()
	return constrainToDisplay(pt.X, pt.Y, w, h, pt)
}

// targetWorkArea returns the work area of the configured screen; screen
// 0 is the active one, approximated by the display under the cursor as
// Maccy's NSScreen.main is the screen holding the key window.
func (a *App) targetWorkArea(cfg config.Config) mygo.Rectangle {
	ds := mygo.Screen.Displays()
	if cfg.PopupScreen > 0 && cfg.PopupScreen <= len(ds) {
		return ds[cfg.PopupScreen-1].WorkArea
	}
	if len(ds) > 0 {
		if id, ok := activePopupScreenID(); ok {
			for _, d := range ds {
				if d.ID == id {
					return d.WorkArea
				}
			}
		}
		return mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint()).WorkArea
	}
	return mygo.Screen.PrimaryDisplay().WorkArea
}

func constrainToRect(x, y, w, h int, wa mygo.Rectangle) (int, int) {
	return max(wa.X, min(x, wa.X+wa.Width-w)), max(wa.Y, min(y, wa.Y+wa.Height-h))
}

func constrainToDisplay(x, y, w, h int, near mygo.Point) (int, int) {
	d := mygo.Screen.DisplayNearestPoint(near)
	wa := d.WorkArea
	if x < wa.X {
		x = wa.X
	}
	if x+w > wa.X+wa.Width {
		x = wa.X + wa.Width - w
	}
	if y < wa.Y {
		y = wa.Y
	}
	if y+h > wa.Y+wa.Height {
		y = wa.Y + wa.Height - h
	}
	return x, y
}

// registerPopupShortcuts grabs the Pin shortcut while the popup is open,
// as Maccy's local key monitor does.
func (a *App) registerPopupShortcuts() {
	if !mygo.App.IsReady() {
		return
	}
	a.unregisterPopupShortcuts()
	acc := strings.TrimSpace(a.cfg.Snapshot().PinShortcut)
	if acc == "" {
		return
	}
	if err := mygo.GlobalShortcut.Register(acc, func() {
		a.post(func() {
			a.pinSelected()
			a.refresh()
		})
	}); err == nil {
		a.popupPinAccel = acc
	} else {
		log.Printf("maccy: pin shortcut %q: %v", acc, err)
	}
}

func (a *App) unregisterPopupShortcuts() {
	if a.popupPinAccel != "" {
		mygo.GlobalShortcut.Unregister(a.popupPinAccel)
		a.popupPinAccel = ""
	}
}

func (a *App) closePopup() {
	if p := a.panel.Load(); p != nil {
		p.Close()
	}
}

// applyShortcut registers the global toggle shortcut; an empty value
// disables it, as clearing Maccy's recorder does.
func (a *App) applyShortcut(acc string) {
	if !mygo.App.IsReady() {
		return
	}
	a.stopPopupHotkeySession(false)
	if a.currentShortcut != "" {
		mygo.GlobalShortcut.Unregister(a.currentShortcut)
		a.currentShortcut = ""
	}
	if acc == "" {
		return
	}
	if err := mygo.GlobalShortcut.Register(acc, a.handlePopupShortcut); err != nil {
		log.Printf("maccy: global shortcut %q: %v", acc, err)
		return
	}
	a.currentShortcut = acc
}

func (a *App) setupMenu() {
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Label: "关于 Maccy MyGo", Click: func(*mygo.MenuItem, *mygo.Window) { a.OpenAbout() }},
			updater.MenuItem(),
			{Label: "偏好设置…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { a.OpenPreferences() }},
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleWindowMenu},
	})
	mygo.App.SetMenu(menu)
}

func (a *App) setupTray() {
	icon := trayIconFor(a.cfg.Snapshot().MenuIcon)
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "maccy",
		Icon:           icon,
		IconIsTemplate: true,
		ToolTip:        "Maccy MyGo",
	})
	if err != nil {
		log.Printf("maccy: tray: %v", err)
		return
	}
	a.tray = tray
	a.trayIconName = ""
	if err := tray.SetIcon(icon, true); err != nil {
		log.Printf("maccy: tray icon: %v", err)
	}

	if runtime.GOOS == "linux" {
		// AppIndicator only shows menus and reports no clicks.
		tray.SetMenu(a.buildTrayMenu())
	} else {
		tray.OnClick(func() { a.statusItemClick(currentKeyboardModifiers()) })
		tray.OnRightClick(func() {
			a.closePopup()
			tray.SetMenu(a.buildTrayMenu())
			defer tray.SetMenu(nil)
			tray.PopUpMenu()
		})
	}
	a.updateTray()
}

// setStatusBarVisible shows or removes the menu bar icon.
func (a *App) setStatusBarVisible(visible bool) {
	if visible {
		if a.tray == nil {
			a.setupTray()
			a.updateTray()
		}
		return
	}
	if a.tray != nil {
		a.tray.Destroy()
		a.tray = nil
	}
}

func (a *App) buildTrayMenu() *mygo.Menu {
	ignoreLabel := "开启忽略"
	if a.cfg.Snapshot().IgnoreEvents {
		ignoreLabel = "关闭忽略"
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开 Maccy MyGo", Click: func(*mygo.MenuItem, *mygo.Window) { a.TogglePopup() }},
		{Label: "偏好设置…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) { a.OpenPreferences() }},
		updater.MenuItem(),
		{Label: "关于 Maccy MyGo", Click: func(*mygo.MenuItem, *mygo.Window) { a.OpenAbout() }},
		mygo.Separator(),
		{Label: ignoreLabel, Click: func(*mygo.MenuItem, *mygo.Window) {
			c := a.cfg.Snapshot()
			_ = a.cfg.SetIgnoreEvents(!c.IgnoreEvents)
			a.refresh()
		}},
		{Label: "忽略下一次复制", Click: func(*mygo.MenuItem, *mygo.Window) {
			_ = a.cfg.Update(func(c *config.Config) {
				c.IgnoreEvents = true
				c.IgnoreOnlyNextEvent = true
			})
			a.refresh()
		}},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
}

var _ = time.Second
