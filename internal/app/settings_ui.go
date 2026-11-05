package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/clipboard"
	"github.com/binyouo/Maccy-MyGo/internal/config"
	"github.com/binyouo/Maccy-MyGo/internal/model"
)

// prefsDraft is the form state of the preferences window.
type prefsDraft struct {
	LaunchAtLogin             bool
	PasteByDefault            bool
	RemoveFormattingByDefault bool
	ShowInStatusBar           bool
	ShowRecentCopyInMenuBar   bool
	ShowSearch                bool
	ShowTitle                 bool
	ShowFooter                bool
	ShowApplicationIcons      bool
	ShowHexColorSwatch        bool
	ShowSpecialSymbols        bool
	OpenPreviewAutomatically  bool
	SaveFiles                 bool
	SaveImages                bool
	SaveText                  bool
	ClearOnQuit               bool
	ClearSystemClipboard      bool
	ConfirmBeforeClear        bool
	IgnoreEvents              bool
	IgnoreAllAppsExceptListed bool
	ShowNotifications         bool

	SearchMode       int
	PinTo            int
	SortBy           int
	Highlight        int
	PopupPosition    int
	PopupScreen      int
	SearchVisibility int
	MenuIcon         int

	HistorySize  float64
	ImageHeight  float64
	PreviewDelay float64

	GlobalShortcut  string
	PinShortcut     string
	DeleteShortcut  string
	PreviewShortcut string

	IgnoredPasteboardTypes string
	IgnoreRegexp           string
	IgnoredApps            string
}

func (a *App) draftFromConfig() {
	c := a.cfg.Snapshot()
	a.prefs = prefsDraft{
		LaunchAtLogin:             c.LaunchAtLogin,
		PasteByDefault:            c.PasteByDefault,
		RemoveFormattingByDefault: c.RemoveFormattingByDefault,
		ShowInStatusBar:           c.ShowInStatusBar,
		ShowRecentCopyInMenuBar:   c.ShowRecentCopyInMenuBar,
		ShowSearch:                c.ShowSearch,
		ShowTitle:                 c.ShowTitle,
		ShowFooter:                c.ShowFooter,
		ShowApplicationIcons:      c.ShowApplicationIcons,
		ShowHexColorSwatch:        c.ShowHexColorSwatch,
		ShowSpecialSymbols:        c.ShowSpecialSymbols,
		OpenPreviewAutomatically:  c.OpenPreviewAutomatically,
		SaveFiles:                 c.SaveFiles,
		SaveImages:                c.SaveImages,
		SaveText:                  c.SaveText,
		ClearOnQuit:               c.ClearOnQuit,
		ClearSystemClipboard:      c.ClearSystemClipboard,
		ConfirmBeforeClear:        !c.SuppressClearAlert,
		IgnoreEvents:              c.IgnoreEvents,
		IgnoreAllAppsExceptListed: c.IgnoreAllAppsExceptListed,
		ShowNotifications:         c.ShowNotifications,
		SearchMode:                searchModeIndex(c.SearchMode),
		PinTo:                     pinToIndex(c.PinTo),
		SortBy:                    sortByIndex(c.SortBy),
		Highlight:                 highlightIndex(c.HighlightMatch),
		PopupPosition:             popupPositionIndex(c.PopupPosition),
		PopupScreen:               c.PopupScreen,
		SearchVisibility:          searchVisibilityIndex(c.SearchVisibility),
		MenuIcon:                  menuIconIndex(c.MenuIcon),
		HistorySize:               float64(c.HistorySize),
		ImageHeight:               float64(c.ImageMaxHeight),
		PreviewDelay:              float64(c.PreviewDelay),
		GlobalShortcut:            c.GlobalShortcut,
		PinShortcut:               c.PinShortcut,
		DeleteShortcut:            c.DeleteShortcut,
		PreviewShortcut:           c.PreviewShortcut,
		IgnoredPasteboardTypes:    strings.Join(c.IgnoredPasteboardTypes, "\n"),
		IgnoreRegexp:              strings.Join(c.IgnoreRegexp, "\n"),
		IgnoredApps:               strings.Join(c.IgnoredApps, "\n"),
	}
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// syncPrefs writes the draft back into the configuration.
func (a *App) syncPrefs() {
	p := a.prefs
	old := a.cfg.Snapshot()
	var newShortcut string
	_ = a.cfg.Update(func(c *config.Config) {
		c.LaunchAtLogin = p.LaunchAtLogin
		c.PasteByDefault = p.PasteByDefault
		c.RemoveFormattingByDefault = p.RemoveFormattingByDefault
		c.ShowInStatusBar = p.ShowInStatusBar
		c.ShowRecentCopyInMenuBar = p.ShowRecentCopyInMenuBar
		c.ShowSearch = p.ShowSearch
		c.ShowTitle = p.ShowTitle
		c.ShowFooter = p.ShowFooter
		c.ShowApplicationIcons = p.ShowApplicationIcons
		c.ShowHexColorSwatch = p.ShowHexColorSwatch
		c.ShowSpecialSymbols = p.ShowSpecialSymbols
		c.OpenPreviewAutomatically = p.OpenPreviewAutomatically
		c.SaveFiles = p.SaveFiles
		c.SaveImages = p.SaveImages
		c.SaveText = p.SaveText
		c.ClearOnQuit = p.ClearOnQuit
		c.ClearSystemClipboard = p.ClearSystemClipboard
		c.SuppressClearAlert = !p.ConfirmBeforeClear
		c.IgnoreEvents = p.IgnoreEvents
		c.IgnoreAllAppsExceptListed = p.IgnoreAllAppsExceptListed
		c.ShowNotifications = p.ShowNotifications
		c.SearchMode = searchModeAt(p.SearchMode)
		c.PinTo = pinToAt(p.PinTo)
		c.SortBy = sortByAt(p.SortBy)
		c.HighlightMatch = highlightAt(p.Highlight)
		c.PopupPosition = popupPositionAt(p.PopupPosition)
		c.PopupScreen = p.PopupScreen
		c.SearchVisibility = searchVisibilityAt(p.SearchVisibility)
		c.MenuIcon = menuIconAt(p.MenuIcon)
		if p.HistorySize >= 1 {
			c.HistorySize = int(p.HistorySize)
		}
		if p.ImageHeight >= 1 {
			c.ImageMaxHeight = int(p.ImageHeight)
		}
		if p.PreviewDelay >= 200 {
			c.PreviewDelay = int(p.PreviewDelay)
		}
		c.GlobalShortcut = strings.TrimSpace(p.GlobalShortcut)
		c.PinShortcut = strings.TrimSpace(p.PinShortcut)
		c.DeleteShortcut = strings.TrimSpace(p.DeleteShortcut)
		c.PreviewShortcut = strings.TrimSpace(p.PreviewShortcut)
		c.IgnoredPasteboardTypes = lines(p.IgnoredPasteboardTypes)
		c.IgnoreRegexp = lines(p.IgnoreRegexp)
		c.IgnoredApps = lines(p.IgnoredApps)
		newShortcut = c.GlobalShortcut
	})
	if newShortcut != a.currentShortcut {
		a.applyShortcut(newShortcut)
	}
	a.updateTray()
	if old.ShowInStatusBar != p.ShowInStatusBar {
		a.setStatusBarVisible(p.ShowInStatusBar)
	}
	if old.LaunchAtLogin != p.LaunchAtLogin {
		a.applyLaunchAtLogin(p.LaunchAtLogin)
	}
	if old.OpenPreviewAutomatically != p.OpenPreviewAutomatically || old.PreviewDelay != int(p.PreviewDelay) {
		a.scheduleAutoPreview()
	}
	a.mu.Lock()
	if int(p.HistorySize) < old.HistorySize {
		a.trimLocked()
		a.saveLocked()
	}
	a.rebuildLocked()
	a.mu.Unlock()
	a.refresh()
}

// OpenPreferences shows the preferences window.
func (a *App) OpenPreferences() {
	if a.settingsWin != nil && !a.settingsWin.IsDestroyed() {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		return
	}
	a.draftFromConfig()
	a.updatePrefsScreens()
	a.prefsRecordingShortcut = ""
	a.prefsNewType = ""
	a.prefsNewRegexp = ""
	a.prefsRegexpError = ""
	a.prefsPinDrafts = nil
	if v := os.Getenv("MACCY_PREFSTAB"); v != "" {
		fmt.Sscanf(v, "%d", &a.prefsTab)
	}
	if a.prefsTab < 0 || a.prefsTab >= len(settingsSections) {
		a.prefsTab = 0
	}
	width, height := settingsPaneSize(a.prefsTab)
	a.settingsWin = mygo.NewWindow(mygo.WindowOptions{
		Title:           settingsSections[a.prefsTab].title,
		Width:           width,
		Height:          height,
		UseContentSize:  true,
		MinWidth:        510,
		MinHeight:       300,
		AlwaysOnTop:     os.Getenv("MACCY_PREFS") != "",
		Content:         ui.View(a.settingsView),
		BackgroundColor: "light-dark(#f5f5f7, #1c1c1e)",
		Hidden:          a.hasDevelopmentScreen(),
	})
	a.resizePreferences()
	a.showOnDevelopmentScreen(a.settingsWin)
	offScreens := mygo.Screen.OnDisplaysChanged(func() {
		a.updatePrefsScreens()
		if a.settingsWin != nil {
			a.settingsWin.Invalidate()
		}
	})
	a.settingsWin.OnClosed(func() {
		offScreens()
		a.cancelSettingsTransition()
		a.prefsMotion = nil
		a.prefsWindowAnchored = false
		a.settingsWin = nil
	})
	mygo.App.Focus()
	a.settingsWin.Focus()
}

// Screen queries belong to the window lifecycle, not to every UI frame.
func (a *App) updatePrefsScreens() {
	displays := mygo.Screen.Displays()
	a.prefsScreens = []string{"活动屏幕"}
	for i := range displays {
		a.prefsScreens = append(a.prefsScreens, fmt.Sprintf("屏幕 %d", i+1))
	}
}

// SetAboutIcon provides the application icon for the about window; the
// embedded copy makes it appear in unbundled development builds too.
func (a *App) SetAboutIcon(data []byte) {
	a.aboutBitmap, _ = ui.DecodeBitmap(data)
}

// OpenAbout shows the about window.
func (a *App) OpenAbout() {
	if a.aboutWin != nil && !a.aboutWin.IsDestroyed() {
		a.aboutWin.Show()
		a.aboutWin.Focus()
		return
	}
	a.aboutWin = mygo.NewWindow(mygo.WindowOptions{
		Title:         "关于 Maccy MyGo",
		Width:         340,
		Height:        260,
		DisableResize: true,
		Content:       ui.View(a.aboutView),
		Hidden:        a.hasDevelopmentScreen(),
	})
	a.showOnDevelopmentScreen(a.aboutWin)
	a.aboutWin.OnClosed(func() { a.aboutWin = nil })
}

func (a *App) aboutView(c *ui.Context) {
	t := c.Theme()
	if a.aboutBitmap == nil {
		if dir, err := mygo.App.Path(mygo.PathResources); err == nil {
			if data, err := os.ReadFile(filepath.Join(dir, "icon.png")); err == nil {
				a.aboutBitmap, _ = ui.DecodeBitmap(data)
			}
		}
	}
	ui.Column(c).Fill().Center().Gap(8).Padding(20).Children(func() {
		if a.aboutBitmap != nil {
			ui.Image(c, a.aboutBitmap).Size(72, 72).Radius(16)
		}
		ui.Text(c, "Maccy MyGo").FontSize(28).Bold()
		ui.Text(c, "轻量的桌面剪贴板管理器。").FontSize(12).TextColor(t.TextMuted).Wrap()
		ui.Text(c, "版本 1.0.0").FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, "基于 Maccy 的非官方 Go / MyGo 重写。").FontSize(11).TextColor(t.TextMuted)
	})
}

// settingsView draws Maccy's settings window: a toolbar of icon+label
// panes on top and, below it, the pane's fields laid out as the Settings
// package does — right-aligned labels beside the controls, the groups
// split by hairlines, no cards.
func (a *App) settingsView(c *ui.Context) {
	t := settingsTheme(c)
	a.prefsReduceMotion = c.Preferences().ReduceMotion
	c.Root().Background(t.Background)
	ui.Column(c).Fill().Children(func() {
		tabs := ui.TabsBase(c, &a.prefsTab, len(settingsSections))
		toolbarColor := ui.Hex("#36393d")
		if !t.Dark {
			toolbarColor = ui.Hex("#e7e7e7")
		}
		width, _ := c.Size()
		tabs.List.Label("偏好设置分类").FillWidth().Height(58).Justify(ui.Center).Gap(1).Padding(0, settingsToolbarRightInset(width), 5, 8).Background(toolbarColor).Children(func() {
			for i := range settingsSections {
				a.settingsTab(c, t, tabs.Tab(i), i)
			}
		})
		tabs.List.OnChange(func() { a.prefsRecordingShortcut = ""; a.resizePreferences() })
		ui.Divider(c)
		// Keep the incoming form at its final layout width while the host
		// reveals it. Otherwise long labels rewrap on every resize frame.
		ui.Column(c).Key("settings-body").Grow(1).FillWidth().Clip().Children(func() {
			paneWidth, _ := settingsPaneSize(a.prefsTab)
			ui.Scroll(c).Key(settingsSections[a.prefsTab].id).Width(float32(paneWidth)).FillHeight().Opacity(a.settingsPaneOpacity()).Children(func() {
				ui.Column(c).FillWidth().PaddingX(30).PaddingY(16).Children(func() {
					switch a.prefsTab {
					case 0:
						a.generalPrefs(c)
					case 1:
						a.storagePrefs(c)
					case 2:
						a.appearancePrefs(c)
					case 3:
						a.pinsPrefs(c)
					case 4:
						a.ignorePrefs(c)
					case 5:
						a.advancedPrefs(c)
					}
				})
			})
		})
	})
}

// Pane dimensions follow the original content widths. Resize once when
// the pane changes, never while rebuilding controls or typing into them.
func (a *App) resizePreferences() {
	if a.settingsWin == nil || a.settingsWin.IsDestroyed() {
		return
	}
	width, height := settingsPaneSize(a.prefsTab)
	r := a.settingsWin.ContentBounds()
	initial := !a.prefsWindowAnchored
	if !a.prefsWindowAnchored || r != a.prefsWindowBounds {
		// A drag establishes a new center; our own clamping must not.
		a.prefsWindowAnchor = mygo.Point{X: r.X + r.Width/2, Y: r.Y}
		a.prefsWindowAnchored = true
		a.prefsMotion = nil
	}
	wa := mygo.Screen.DisplayNearestPoint(mygo.Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}).WorkArea
	if test, ok := a.developmentScreen(); ok {
		wa = test
	}
	a.animatePreferences(width, height, wa, initial)
	a.settingsWin.SetTitle(settingsSections[a.prefsTab].title)
}

func settingsBoundsAtAnchor(anchor mygo.Point, width, height int, workArea mygo.Rectangle) mygo.Rectangle {
	width, height = max(1, width), max(1, height)
	if workArea.Width > 0 {
		width = min(width, workArea.Width)
	}
	if workArea.Height > 0 {
		height = min(height, workArea.Height)
	}
	x, y := constrainToRect(anchor.X-width/2, anchor.Y, width, height, workArea)
	return mygo.Rectangle{X: x, Y: y, Width: width, Height: height}
}

func settingsPaneSize(index int) (int, int) {
	sizes := [][2]int{{510, 528}, {510, 300}, {710, 552}, {710, 430}, {560, 450}, {510, 420}}
	size := sizes[clampIdx(index, len(sizes))]
	return size[0], size[1]
}

// MyGo owns tab selection, keyboard navigation and accessibility. Only
// the icon, label and selected background are styled here.
func (a *App) settingsTab(c *ui.Context, t *ui.Theme, b ui.Element, i int) {
	sec := settingsSections[i]
	sel := a.prefsTab == i
	ic, tc := t.Text, t.TextMuted
	if sel {
		ic, tc = t.Accent, t.Accent
	}
	b.Column().AlignItems(ui.Center).Gap(2).Size(55, 45).Padding(3, 0).Radius(6).Cursor(ui.CursorPointer).
		Label(sec.title)
	b.OnClick(func() { b.Focus() })
	b.Transition(ui.ElementTransition{Duration: 120 * time.Millisecond, Colors: true})
	if sel {
		b.Background(t.TextMuted.Alpha(0.18))
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Icon(c, sec.icon).Size(23, 23).TextColor(ic).PassThrough()
		ui.Text(c, sec.title).FontSize(11).TextColor(tc).SingleLine().PassThrough()
	})
}

// A new pane gets its own input and scroll state. Fade only the incoming
// pane so an outgoing form cannot overlap it or keep handling input.
var paneTransition = ui.ElementTransition{
	Duration: 160 * time.Millisecond,
	Enter:    &ui.Motion{Y: 4},
}

var settingsSections = []struct {
	id, title string
	icon      *ui.SVG
}{
	{"general", "通用", iconSettingsGeneral},
	{"storage", "存储", iconSettingsStorage},
	{"appearance", "外观", iconSettingsAppearance},
	{"pins", "置顶项", iconSettingsPins},
	{"ignore", "忽略", iconSettingsIgnore},
	{"advanced", "高级", iconSettingsAdvanced},
}

// settingsInlineTip is a muted note under a field's controls.
func settingsInlineTip(c *ui.Context, text string) {
	t := c.Theme()
	ui.Text(c, text).FontSize(11).TextColor(t.TextMuted).Wrap()
}

// settingsForm lays out a pane's fields as Maccy's Settings.Section does:
// the widest label sets one column the controls share, the gap a little
// tighter than the default so a pane fits without scrolling far.
func settingsForm(c *ui.Context, fn func()) {
	ui.Form(c, fn).Gap(9)
}

// formDivider is the hairline between two sections of a pane.
func formDivider(c *ui.Context) {
	ui.Divider(c).Margin(6, 0, 6, 0)
}

// toggleRow is a Maccy-style toggle: a labeled checkbox, the label
// toggling it as well.
func (a *App) toggleRow(c *ui.Context, label string, v *bool) {
	ui.Checkbox(c, v, label).AlignSelf(ui.Start).OnChange(a.syncPrefs)
}

// toggleRowTip is a toggleRow carrying the control's help text, as
// Maccy's Toggle.help does.
func (a *App) toggleRowTip(c *ui.Context, label string, v *bool, tip string) {
	ui.Checkbox(c, v, label).AlignSelf(ui.Start).Tooltip(tip).OnChange(a.syncPrefs)
}

// toggleRowCtl is a toggleRow with more controls trailing the label, as
// Maccy's "ShowMenuIcon" toggle carries the icon picker beside it.
func (a *App) toggleRowCtl(c *ui.Context, label string, v *bool, fn func()) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		sw := ui.Checkbox(c, v, label)
		if fn != nil {
			fn()
		}
		sw.OnChange(a.syncPrefs)
	})
}

// shortcutDisplay renders a shortcut for a key cap, as ⌘⇧C.
func shortcutDisplay(s string) string {
	if strings.TrimSpace(s) == "" {
		return "未设置"
	}
	parts := strings.Split(s, "+")
	var b strings.Builder
	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "cmdorctrl", "cmd", "command", "super", "meta":
			b.WriteString("⌘")
		case "ctrl", "control":
			b.WriteString("⌃")
		case "option", "opt", "alt":
			b.WriteString("⌥")
		case "shift":
			b.WriteString("⇧")
		}
	}
	key := strings.TrimSpace(parts[len(parts)-1])
	switch strings.ToLower(key) {
	case "return", "enter":
		key = "↩"
	case "delete", "backspace":
		key = "⌫"
	case "forwarddelete":
		key = "⌦"
	case "escape", "esc":
		key = "⎋"
	case "tab":
		key = "⇥"
	case "space":
		key = "空格"
	case "up":
		key = "↑"
	case "down":
		key = "↓"
	case "left":
		key = "←"
	case "right":
		key = "→"
	}
	b.WriteString(key)
	return b.String()
}

// shortcutFromEvent names the key combination an input event pressed, or
// returns "" for a key the app cannot use as a shortcut.
func shortcutFromEvent(ev ui.InputEvent) string {
	key := keyName(ev.Key)
	if key == "" {
		key = keyName(ev.PhysicalKey)
	}
	if key == "" {
		return ""
	}
	var parts []string
	if ev.Mods&ui.Super != 0 {
		parts = append(parts, "Cmd")
	}
	if ev.Mods&ui.Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if ev.Mods&ui.Alt != 0 {
		parts = append(parts, "Option")
	}
	if ev.Mods&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, key), "+")
}

// shortcutRecorder is a button that records the next key combination, like
// Maccy's KeyboardShortcuts.Recorder.
func (a *App) shortcutRecorder(c *ui.Context, field string, value *string, def string) {
	t := c.Theme()
	row := ui.Row(c).Key(field).Gap(8).AlignItems(ui.Center)
	errorText := ui.Local(row, "validation", func() string { return "" })
	row.Children(func() {
		box := ui.ButtonBase(c).Height(20).Width(128).Gap(2).Padding(0, 4, 0, 7).Radius(4).
			Border(1, t.Border).Background(t.Surface).Cursor(ui.CursorPointer)
		recording := a.prefsRecordingShortcut == field
		if recording && !box.Focused() {
			a.prefsRecordingShortcut = ""
			recording = false
		}
		if !recording {
			*errorText = ""
		}
		text := shortcutDisplay(*value)
		if recording {
			text = "按下快捷键…"
			box.Border(1, t.Accent)
		}
		if *errorText != "" {
			box.Border(1, t.Danger).Tooltip(*errorText).Description(*errorText)
		}
		box.Label(text).OnClick(func() { a.prefsRecordingShortcut = field; *errorText = ""; box.Focus() })
		box.Children(func() {
			ui.Text(c, text).FontSize(12).SingleLine().Grow(1).PassThrough()
			if !recording && *value != "" {
				clear := ui.ButtonBase(c).Size(16, 18).Label("清除快捷键").Tooltip("清除快捷键")
				clear.Children(func() { ui.Icon(c, iconClear).Size(12, 12).TextColor(t.TextMuted).PassThrough() })
				clear.OnClick(func() { *value = ""; a.prefsRecordingShortcut = ""; a.syncPrefs() })
			}
		})
		box.ContextMenu(func(m *ui.Menu) {
			if m.Item("恢复默认").Disabled(*value == def).Chosen() {
				*value = def
				a.prefsRecordingShortcut = ""
				a.syncPrefs()
			}
		})
		box.HandleInput(func(ev ui.InputEvent) bool {
			if a.prefsRecordingShortcut != field {
				return false
			}
			if ev.Kind != ui.InputKeyDown {
				return ev.Kind == ui.InputText
			}
			if ev.Mods == 0 {
				switch ev.Key {
				case ui.KeyEscape:
					a.prefsRecordingShortcut = ""
					return true
				case ui.KeyTab:
					a.prefsRecordingShortcut = ""
					return false
				case ui.KeyBackspace, ui.KeyDelete:
					*value = ""
					a.prefsRecordingShortcut = ""
					a.syncPrefs()
					return true
				}
			}
			name := shortcutFromEvent(ev)
			if name == "" {
				return true
			}
			if reason := a.validateRecordedShortcut(field, name); reason != "" {
				*errorText = reason
				return true
			}
			previous := *value
			*value = name
			a.prefsRecordingShortcut = ""
			*errorText = ""
			a.syncPrefs()
			if field == "open" && mygo.App.IsReady() && a.currentShortcut != name {
				*value = previous
				a.syncPrefs()
				a.prefsRecordingShortcut = field
				*errorText = "此快捷键无法注册，可能已被其他应用占用。请重新录制。"
			}
			return true
		})
	})
}

// ignoreList shows a newline-separated preference as removable rows.
func (a *App) ignoreList(c *ui.Context, items []string, empty string, remove func(int), edit ...func(int, string) string) {
	t := c.Theme()
	height := float32(180)
	if len(items) < 5 {
		height = 44 + float32(len(items))*34
	}
	ui.Scroll(c).Height(height).Shrink(0).Border(1, t.Border).Radius(t.Radius).Background(t.Surface).Children(func() {
		ui.Column(c).Gap(0).Children(func() {
			if len(items) == 0 {
				ui.Text(c, empty).FontSize(12).TextColor(t.TextMuted).Padding(t.Space(3), t.Space(3))
				return
			}
			for i, item := range items {
				idx := i
				row := ui.Row(c).Key(item).FillWidth().AlignItems(ui.Center).Gap(8).Padding(t.Space(1.5), t.Space(2.5))
				if i%2 == 1 {
					row.Background(t.SurfaceHover.Alpha(0.35))
				}
				row.Children(func() {
					if len(edit) > 0 {
						draft := ui.Local(row, "text", func() string { return item })
						validation := ui.Local(row, "error", func() string { return "" })
						input := ui.TextInputBase(c, draft).Label(item).FontSize(12).Grow(1).Height(20).Padding(0, 3).Tooltip("编辑后按 Return 保存")
						if input.Focused() {
							input.Border(1, t.Accent).Radius(3)
						}
						if *validation != "" {
							input.Border(1, t.Danger).Tooltip(*validation).Description(*validation)
						}
						input.OnSubmit(func() { *validation = edit[0](idx, *draft) })
					} else {
						ui.Text(c, item).FontSize(12).Grow(1).SingleLine()
					}
					settingsButton(c, "移除").Padding(t.Space(0.5), t.Space(2)).OnClick(func() {
						remove(idx)
					})
				})
				if i < len(items)-1 {
					ui.Divider(c)
				}
			}
		})
	})
}

func (a *App) editIgnoredLine(value *string, index int, text string, isRegexp bool) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "内容不能为空；可使用移除按钮删除此项。"
	}
	if isRegexp {
		if _, err := regexp2.Compile(text, 0); err != nil {
			return "正则表达式无效：" + err.Error()
		}
	}
	items := lines(*value)
	if index < 0 || index >= len(items) {
		return "此条目已移除。"
	}
	for i, item := range items {
		if i != index && item == text {
			return "此项已存在。"
		}
	}
	items[index] = text
	*value = strings.Join(items, "\n")
	a.syncPrefs()
	return ""
}

// removeLine removes line i of a newline-separated preference.
func removeLine(s string, i int) string {
	list := lines(s)
	if i < 0 || i >= len(list) {
		return s
	}
	return strings.Join(append(list[:i], list[i+1:]...), "\n")
}

// addLine appends v to a newline-separated preference unless it is there.
func addLine(s, v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return s
	}
	for _, x := range lines(s) {
		if x == v {
			return s
		}
	}
	if strings.TrimSpace(s) == "" {
		return v
	}
	return strings.TrimRight(s, "\n") + "\n" + v
}

// resetPopupPosition forgets the remembered popup position.
func (a *App) resetPopupPosition() {
	a.hasLast, a.lastX, a.lastY = false, 0, 0
	_ = a.cfg.Update(func(c *config.Config) {
		c.HasPopupPosition, c.PopupX, c.PopupY = false, 0, 0
		c.HasRelativePopupPosition, c.PopupAnchorX, c.PopupAnchorY = false, 0, 0
	})
}

// generalPrefs mirrors Maccy's GeneralSettingsPane: an unlabeled first
// section, the shortcut recorders, the search mode, the selection
// behavior, and the link to the system's notification settings.
func (a *App) generalPrefs(c *ui.Context) {
	settingsForm(c, func() {
		ui.Field(c, "", func() {
			ui.Column(c).Gap(7).AlignItems(ui.Start).Children(func() {
				a.toggleRow(c, "登录时启动", &a.prefs.LaunchAtLogin)
				ui.Checkbox(c, &a.autoUpdates, "自动检查更新").AlignSelf(ui.Start).OnChange(func() { updater.SetAutomaticChecks(a.autoUpdates) })
				settingsButton(c, "检查更新").AlignSelf(ui.Start).OnClick(updater.CheckForUpdates)
			})
		})
		formDivider(c)
		for _, shortcut := range []struct {
			label, field string
			value        *string
			def, tip     string
		}{
			{"打开:", "open", &a.prefs.GlobalShortcut, "CmdOrCtrl+Shift+C", "用于打开应用的全局快捷键。默认：⇧⌘C。右键可恢复默认。"},
			{"置顶:", "pin", &a.prefs.PinShortcut, "Option+P", "置顶历史记录项。默认：⌥P。右键可恢复默认。"},
			{"删除:", "delete", &a.prefs.DeleteShortcut, "Option+Delete", "删除历史记录项。默认：⌥⌫。右键可恢复默认。"},
			{"预览:", "preview", &a.prefs.PreviewShortcut, "Ctrl+Space", "显示附加信息的预览。默认：⌃空格。右键可恢复默认。"},
		} {
			ui.Field(c, shortcut.label, func() { a.shortcutRecorder(c, shortcut.field, shortcut.value, shortcut.def) }).Tooltip(shortcut.tip)
		}
		formDivider(c)
		ui.Field(c, "搜索:", func() {
			settingsPicker(c, &a.prefs.SearchMode, "搜索", 180, "精确", "模糊", "正则表达式", "混合").Tooltip("决定输入的关键词如何匹配条目。").OnChange(a.syncPrefs)
		})
		formDivider(c)
		ui.Field(c, "行为:", func() {
			ui.Column(c).Gap(7).AlignItems(ui.Start).Children(func() {
				a.toggleRow(c, "自动粘贴", &a.prefs.PasteByDefault)
				a.toggleRow(c, "纯文本粘贴", &a.prefs.RemoveFormattingByDefault)
				settingsInlineTip(c, a.modifierLegend())
			})
		})
		formDivider(c)
		if runtime.GOOS == "darwin" {
			ui.Field(c, "", func() {
				ui.Link(c, "通知和声音 ↗", "x-apple.systempreferences:com.apple.preference.notifications?id=io.github.binyouo.maccy-mygo").FontSize(12)
			})
		}
	})
}

func (a *App) modifierLegend() string {
	c := a.cfg.Snapshot()
	combinations := []struct {
		mods  ui.Modifiers
		label string
	}{{ui.Super, "⌘"}, {ui.Alt, "⌥"}, {ui.Alt | ui.Shift, "⌥⇧"}, {ui.Super | ui.Shift, "⌘⇧"}}
	find := func(action itemActionKind) string {
		for _, combination := range combinations {
			if itemAction(c, combination.mods) == action {
				return combination.label
			}
		}
		return ""
	}
	return fmt.Sprintf("自定义选择项目时的行为：\n• 按住 %s 选择项目以复制。\n• 按住 %s 选择项目以复制并粘贴。\n• 按住 %s 选择项目以复制并匹配格式粘贴。", find(actCopy), find(actPaste), find(actPasteNoFormatting))
}

func (a *App) storagePrefs(c *ui.Context) {
	settingsForm(c, func() {
		ui.Field(c, "保存:", func() {
			ui.Column(c).Gap(7).AlignItems(ui.Start).Children(func() {
				a.toggleRow(c, "文件", &a.prefs.SaveFiles)
				a.toggleRow(c, "图像", &a.prefs.SaveImages)
				a.toggleRow(c, "文本", &a.prefs.SaveText)
				settingsInlineTip(c, "设置何种类型的内容会被保存。")
			})
		})
		formDivider(c)
		ui.Field(c, "数量:", func() {
			row := ui.Row(c).Gap(10).AlignItems(ui.Center)
			// As in the original pane's onAppear, scan the store once when
			// this pane opens, rather than walking every blob on every frame.
			size := ui.Local(row, "disk-size", a.storeSize)
			row.Children(func() {
				settingsNumber(c, &a.prefs.HistorySize, "数量", 1, 999, func() { a.syncPrefs(); *size = a.storeSize() }).Tooltip("保留的历史记录条数。默认：200。")
				ui.Text(c, *size).FontSize(11).TextColor(c.Theme().TextMuted).Tooltip("磁盘上的当前大小。")
			})
		})
		ui.Field(c, "排序方式:", func() {
			settingsPicker(c, &a.prefs.SortBy, "排序方式", 160, "上次复制时间", "首次复制时间", "复制次数").OnChange(a.syncPrefs)
		})
	})
}

// appearancePrefs mirrors Maccy's AppearanceSettingsPane: the popup
// fields first, the preview pair, the highlight picker, then the
// unlabeled display toggles.
func (a *App) appearancePrefs(c *ui.Context) {
	settingsForm(c, func() {
		ui.Field(c, "弹窗位置:", func() {
			ui.Row(c).Gap(8).Children(func() {
				settingsPicker(c, &a.prefs.PopupPosition, "弹窗位置", 141, "光标", "菜单栏图标", "窗口中心", "屏幕中央", "最后位置").Tooltip("更改弹窗显示的位置。默认：光标。").OnChange(a.syncPrefs)
				if a.prefs.PopupPosition == popupPositionIndex(config.PopupLast) {
					settingsButton(c, "重置位置").Disabled(!a.hasLast && !a.cfg.Snapshot().HasPopupPosition).Tooltip("忘记已保存的弹窗位置。").OnClick(a.resetPopupPosition)
				}
			})
		})
		if len(a.prefsScreens) > 2 && (a.prefs.PopupPosition == popupPositionIndex(config.PopupCenter) || a.prefs.PopupPosition == popupPositionIndex(config.PopupLast)) {
			ui.Field(c, "屏幕:", func() {
				settingsPicker(c, &a.prefs.PopupScreen, "屏幕", 180, a.prefsScreens...).OnChange(a.syncPrefs)
			})
		}
		ui.Field(c, "置顶到:", func() { settingsPicker(c, &a.prefs.PinTo, "置顶到", 141, "顶部", "底部").OnChange(a.syncPrefs) })
		ui.Field(c, "图片高度:", func() {
			settingsNumber(c, &a.prefs.ImageHeight, "图片高度", 1, 200, a.syncPrefs).Tooltip("图像预览最大高度。默认：40。")
		})
		ui.Field(c, "", func() { a.toggleRow(c, "自动打开预览", &a.prefs.OpenPreviewAutomatically) })
		ui.Field(c, "预览延迟:", func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Disabled(!a.prefs.OpenPreviewAutomatically).Children(func() {
				settingsNumber(c, &a.prefs.PreviewDelay, "预览延迟", 200, 100000, a.syncPrefs).Tooltip("显示预览前的延迟，以毫秒为单位。默认：1500。")
				ui.Text(c, "毫秒").FontSize(11).TextColor(c.Theme().TextMuted)
			})
		})
		ui.Field(c, "高亮匹配:", func() {
			settingsPicker(c, &a.prefs.Highlight, "高亮匹配", 141, "粗体", "斜体", "强调", "颜色").OnChange(a.syncPrefs)
		})
		formDivider(c)
		ui.Field(c, "", func() {
			ui.Column(c).Gap(7).AlignItems(ui.Start).Children(func() {
				a.toggleRowTip(c, "显示特殊符号", &a.prefs.ShowSpecialSymbols, "通过特殊符号显示换行符、制表符、前导和尾随空格。")
				a.toggleRowCtl(c, "显示菜单栏图标", &a.prefs.ShowInStatusBar, func() {
					settingsPicker(c, &a.prefs.MenuIcon, "菜单栏图标样式", 92, "默认", "剪贴板", "剪刀", "回形针").Disabled(!a.prefs.ShowInStatusBar).OnChange(a.syncPrefs)
				})
				a.toggleRow(c, "在菜单栏图标旁显示剪贴板内容", &a.prefs.ShowRecentCopyInMenuBar)
				a.toggleRowCtl(c, "显示搜索框", &a.prefs.ShowSearch, func() {
					settingsPicker(c, &a.prefs.SearchVisibility, "搜索框显示时机", 140, "始终", "在搜索过程中").Disabled(!a.prefs.ShowSearch).OnChange(a.syncPrefs)
				})
				a.toggleRow(c, "在搜索框前显示应用名", &a.prefs.ShowTitle)
				a.toggleRow(c, "显示应用程序图标", &a.prefs.ShowApplicationIcons)
				a.toggleRowTip(c, "显示十六进制颜色色块", &a.prefs.ShowHexColorSwatch, "在十六进制颜色代码旁边显示颜色色块。")
				a.toggleRow(c, "显示底部菜单", &a.prefs.ShowFooter)
				opacity := float32(1)
				if a.prefs.ShowFooter {
					opacity = 0
				}
				ui.Text(c, "隐藏底部菜单时仍可按 ⌘,（Command+逗号）打开偏好设置。").FontSize(11).TextColor(c.Theme().TextMuted).Wrap().MaxWidth(380).Opacity(opacity)
			})
		})
	})
}

// pinDraft is the in-place edit buffer of one pinned item's row.
type pinDraft struct {
	title    string
	content  string
	tooLarge bool
}

// pinnedItems returns the pinned items in their display order.
func (a *App) pinnedItems() []*model.Item {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg := a.cfg.Snapshot()
	pos := map[string]int{}
	for i, p := range cfg.PinOrder {
		pos[p] = i
	}
	var pinned []*model.Item
	for _, it := range a.items {
		if it.IsPinned() {
			pinned = append(pinned, it)
		}
	}
	sort.SliceStable(pinned, func(i, j int) bool {
		return pos[pinned[i].PinOrderKey()] < pos[pinned[j].PinOrderKey()]
	})
	return pinned
}

func (a *App) pinDraftFor(it *model.Item) *pinDraft {
	if a.prefsPinDrafts == nil {
		a.prefsPinDrafts = map[string]*pinDraft{}
	}
	d := a.prefsPinDrafts[it.ID]
	if d == nil {
		d = &pinDraft{title: it.Title, content: a.itemText(it)}
		d.content, d.tooLarge = sizedPreviewText(d.content)
		a.prefsPinDrafts[it.ID] = d
	}
	return d
}

// applyPinKey records a pin key change, keeping the pin's position in
// the pinned block.
func (a *App) applyPinKey(it *model.Item, old string) {
	a.mu.Lock()
	newPin := it.Pin
	it.Pin = old
	a.updatePinKeyLocked(it, newPin)
	a.rebuildLocked()
	a.saveLocked()
	a.mu.Unlock()
	a.refresh()
}

func (a *App) applyPinTitle(it *model.Item, d *pinDraft) {
	title := d.title
	if title == it.Title || clipboard.HasImage(it) {
		return
	}
	a.mu.Lock()
	it.Title = title
	a.rebuildLocked()
	a.saveLocked()
	a.mu.Unlock()
	a.refresh()
}

func (a *App) applyPinContent(it *model.Item, d *pinDraft) {
	if d.tooLarge || d.content == a.itemText(it) || !canEditItemContent(it) {
		return
	}
	a.mu.Lock()
	if !a.replaceItemTextLocked(it, d.content) {
		a.mu.Unlock()
		return
	}
	a.rebuildLocked()
	a.saveLocked()
	a.mu.Unlock()
	a.refresh()
}

// pinsPrefs mirrors Maccy's PinsSettingsPane: the pinned items as a
// table — key, alias and content — editable in place.
func (a *App) pinsPrefs(c *ui.Context) {
	t := c.Theme()
	pinned := a.pinnedItems()
	if len(pinned) == 0 {
		ui.Column(c).FillWidth().Gap(10).AlignItems(ui.Center).Padding(40, 0).Children(func() {
			ui.Text(c, "尚无置顶条目").FontSize(13).TextColor(t.TextMuted)
			ui.Text(c, "在弹出面板中选中条目后按置顶快捷键（默认 ⌥P）即可置顶。").FontSize(11).TextColor(t.TextMuted).Wrap()
		})
		return
	}
	ui.Row(c).Gap(10).Padding(0, 4, 0, 4).Children(func() {
		ui.Text(c, "键位").FontSize(11).TextColor(t.TextMuted).Width(76)
		ui.Text(c, "标题").FontSize(11).TextColor(t.TextMuted).Grow(2)
		ui.Text(c, "内容").FontSize(11).TextColor(t.TextMuted).Grow(3)
		ui.Box(c).Width(20)
	})
	for _, it := range pinned {
		a.pinPrefsRow(c, t, it)
	}
	ui.Box(c).Height(8)
	ui.Text(c, "您可以自定义任何置顶项目的热键、标题和内容。\n编辑标题或内容后按 Return 保存。").
		FontSize(11).TextColor(t.TextMuted).Wrap()
}

func (a *App) pinPrefsRow(c *ui.Context, t *ui.Theme, it *model.Item) {
	d := a.pinDraftFor(it)
	ui.Row(c).Key(it.ID).Gap(10).AlignItems(ui.Center).Padding(0, 0, 6, 0).Children(func() {
		// Key: pick any free pin key, as Maccy's PinPickerView does.
		a.mu.Lock()
		choices := a.availablePinsLocked()
		a.mu.Unlock()
		choices = append(append([]string(nil), choices...), it.Pin)
		sortPinChoices(choices)
		old := it.Pin
		ui.Select(c, &it.Pin, choices).Width(76).Height(20).Padding(0, 6).OnChange(func() { a.applyPinKey(it, old) })
		// Alias.
		title := ui.TextInput(c, &d.title).Grow(2).Label("标题").Height(20).Padding(0, 5).Disabled(clipboard.HasImage(it))
		title.OnSubmit(func() { a.applyPinTitle(it, d) })
		// Content: only plain-text items are editable, as in the editor.
		if canEditItemContent(it) && !d.tooLarge {
			content := ui.TextInput(c, &d.content).Grow(3).Label("内容").Height(20).Padding(0, 5)
			content.OnSubmit(func() { a.applyPinContent(it, d) })
		} else {
			ui.Text(c, "不可编辑的内容（图像或文件）").FontSize(12).Italic().TextColor(t.TextMuted).Grow(3)
		}
		ui.Icon(c, iconPinSlash).Size(14, 14).TextColor(t.TextMuted).Cursor(ui.CursorPointer).
			Label("取消置顶").Tooltip("取消置顶").OnClick(func() {
			a.togglePin(it)
			a.refresh()
		})
	})
}

// ignorePrefs mirrors Maccy's IgnoreSettingsPane: a tab for each ignore
// kind, the list inside it, and the controls to add entries.
func (a *App) ignorePrefs(c *ui.Context) {
	ui.Row(c).Children(func() {
		settingsIgnoreTabs(c, &a.prefsIgnoreTab)
	})
	ui.Box(c).Height(12)
	ui.Column(c).Key(a.prefsIgnoreTab).FillWidth().Transition(paneTransition).Children(func() {
		settingsForm(c, func() {
			switch a.prefsIgnoreTab {
			case 1:
				ui.Field(c, "忽略的类型", func() {
					a.ignoreList(c, lines(a.prefs.IgnoredPasteboardTypes), "尚未添加剪贴板类型", func(i int) {
						a.prefs.IgnoredPasteboardTypes = removeLine(a.prefs.IgnoredPasteboardTypes, i)
						a.syncPrefs()
					}, func(index int, text string) string {
						return a.editIgnoredLine(&a.prefs.IgnoredPasteboardTypes, index, text, false)
					})
				}).Description("忽略特定剪贴板内容类型。\n默认提供了一些已知的适用于特定应用的类型。您可以删除预置类型，或根据需要添加自定义类型。")
				ui.Field(c, "添加类型", func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						add := func() {
							if strings.TrimSpace(a.prefsNewType) == "" {
								return
							}
							a.prefs.IgnoredPasteboardTypes = addLine(a.prefs.IgnoredPasteboardTypes, a.prefsNewType)
							a.prefsNewType = ""
							a.syncPrefs()
						}
						in := ui.TextInput(c, &a.prefsNewType).Placeholder("com.example.type").Label("添加类型").Width(220).Height(20).Padding(0, 5)
						in.OnSubmit(add)
						settingsButton(c, "添加").Disabled(strings.TrimSpace(a.prefsNewType) == "").OnClick(add)
						settingsButton(c, "重置").OnClick(func() {
							a.prefs.IgnoredPasteboardTypes = strings.Join(config.Default().IgnoredPasteboardTypes, "\n")
							a.syncPrefs()
						})
					})
				})
			case 2:
				ui.Field(c, "忽略匹配", func() {
					a.ignoreList(c, lines(a.prefs.IgnoreRegexp), "尚未添加正则", func(i int) {
						a.prefs.IgnoreRegexp = removeLine(a.prefs.IgnoreRegexp, i)
						a.syncPrefs()
					}, func(index int, text string) string {
						return a.editIgnoredLine(&a.prefs.IgnoreRegexp, index, text, true)
					})
				}).Description("可以根据定义的正则表达式忽略某些副本。")
				ui.Field(c, "添加正则", func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						add := func() {
							if strings.TrimSpace(a.prefsNewRegexp) == "" {
								return
							}
							if _, err := regexp2.Compile(a.prefsNewRegexp, 0); err != nil {
								a.prefsRegexpError = err.Error()
								return
							}
							a.prefsRegexpError = ""
							a.prefs.IgnoreRegexp = addLine(a.prefs.IgnoreRegexp, a.prefsNewRegexp)
							a.prefsNewRegexp = ""
							a.syncPrefs()
						}
						in := ui.TextInput(c, &a.prefsNewRegexp).Placeholder("^[a-zA-Z0-9]{50}$").Label("添加正则").Width(220).Height(20).Padding(0, 5)
						in.OnSubmit(add)
						settingsButton(c, "添加").Disabled(strings.TrimSpace(a.prefsNewRegexp) == "").OnClick(add)
						settingsButton(c, "清空").OnClick(func() { a.prefs.IgnoreRegexp = ""; a.prefsRegexpError = ""; a.syncPrefs() })
					})
				}).Error(a.prefsRegexpError)
			default:
				ui.Field(c, "", func() {
					ui.Column(c).Gap(8).AlignItems(ui.Start).Children(func() {
						a.toggleRow(c, "忽略列表之外的所有应用程序", &a.prefs.IgnoreAllAppsExceptListed)
						settingsInlineTip(c, "忽略来自特定应用的内容。\n请注意此选项并非总是有效，最好使用忽略剪贴板类型设置。")
					})
				})
				ui.Field(c, "忽略的应用", func() {
					a.ignoreList(c, lines(a.prefs.IgnoredApps), "尚未添加应用", func(i int) {
						a.prefs.IgnoredApps = removeLine(a.prefs.IgnoredApps, i)
						a.syncPrefs()
					}, func(index int, text string) string {
						return a.editIgnoredLine(&a.prefs.IgnoredApps, index, text, false)
					})
				}).Description("每个应用占一行，使用 Bundle 标识符。")
				ui.Field(c, "", func() {
					settingsButton(c, "选择应用…").AlignSelf(ui.Start).OnClick(a.pickIgnoredApp)
				})
			}
		})
	})
}

// advancedPrefs mirrors Maccy's AdvancedSettingsPane: the turn-off
// toggle and the clear-history options, split by a hairline.
func (a *App) advancedPrefs(c *ui.Context) {
	settingsForm(c, func() {
		ui.Field(c, "", func() {
			ui.Column(c).Gap(8).AlignItems(ui.Start).Children(func() {
				a.toggleRow(c, "暂停", &a.prefs.IgnoreEvents)
				settingsInlineTip(c, "暂停记录新的剪贴板项目。\n如果您想在复制敏感数据时通过程序调用来禁用本程序。")
				settingsInlineTip(c, "也可以从菜单栏图标的菜单中切换；菜单中的“忽略下一次复制”只对下一次复制有效。")
			})
		})
		formDivider(c)
		ui.Field(c, "", func() {
			ui.Column(c).Gap(8).AlignItems(ui.Start).Children(func() {
				a.toggleRowTip(c, "退出时清除历史记录", &a.prefs.ClearOnQuit, "退出应用时仅保留已置顶的项目(其他历史记录将被抛弃)。")
				a.toggleRowTip(c, "同时清除系统剪贴板", &a.prefs.ClearSystemClipboard, "启用后，清除历史记录也会清除当前系统剪贴板。")
				a.toggleRow(c, "复制时显示通知", &a.prefs.ShowNotifications)
				a.toggleRow(c, "清空历史前确认", &a.prefs.ConfirmBeforeClear)
			})
		})
		formDivider(c)
		ui.Field(c, "", func() {
			ui.Row(c).Gap(8).Children(func() {
				settingsButton(c, "清空未置顶").OnClick(func() { a.confirmClear(false) })
				settingsButton(c, "清空全部").OnClick(func() { a.confirmClear(true) })
			})
		})
	})
}

func (a *App) prefsMenuIcon() string { return menuIconAt(a.prefs.MenuIcon) }

// pickIgnoredApp asks for an application and adds its bundle id.
func (a *App) pickIgnoredApp() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:      a.settingsWin,
			Title:       "选择应用",
			ButtonLabel: "添加",
			Message:     "选择要忽略其复制内容的应用",
			DefaultPath: "/Applications",
			Filters:     []mygo.FileFilter{{Name: "应用程序", Extensions: []string{"app"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		id := bundleIDForApp(paths[0])
		if id == "" {
			return
		}
		a.post(func() {
			for _, line := range strings.Split(a.prefs.IgnoredApps, "\n") {
				if strings.TrimSpace(line) == id {
					return
				}
			}
			if strings.TrimSpace(a.prefs.IgnoredApps) != "" {
				a.prefs.IgnoredApps += "\n"
			}
			a.prefs.IgnoredApps += id
			a.syncPrefs()
		})
	}()
}

// storeSize returns a human size of the store directory.
func (a *App) storeSize() string {
	var total int64
	_ = filepath.WalkDir(a.store.Dir(), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return fmt.Sprintf("%d 项 · 磁盘占用 %s", len(a.items), humanBytes(total))
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// Enum <-> index mappings.
func searchModeIndex(v config.SearchMode) int {
	switch v {
	case config.SearchFuzzy:
		return 1
	case config.SearchRegexp:
		return 2
	case config.SearchMixed:
		return 3
	}
	return 0
}
func searchModeAt(i int) config.SearchMode {
	return [...]config.SearchMode{config.SearchExact, config.SearchFuzzy, config.SearchRegexp, config.SearchMixed}[clampIdx(i, 4)]
}
func pinToIndex(v config.PinTo) int {
	if v == config.PinBottom {
		return 1
	}
	return 0
}
func pinToAt(i int) config.PinTo {
	if i == 1 {
		return config.PinBottom
	}
	return config.PinTop
}
func sortByIndex(v config.SortBy) int {
	switch v {
	case config.SortFirstCopied:
		return 1
	case config.SortCopies:
		return 2
	}
	return 0
}
func sortByAt(i int) config.SortBy {
	return [...]config.SortBy{config.SortLastCopied, config.SortFirstCopied, config.SortCopies}[clampIdx(i, 3)]
}
func highlightIndex(v config.HighlightMatch) int {
	switch v {
	case config.HighlightItalic:
		return 1
	case config.HighlightUnderline:
		return 2
	case config.HighlightColor:
		return 3
	}
	return 0
}
func highlightAt(i int) config.HighlightMatch {
	return [...]config.HighlightMatch{config.HighlightBold, config.HighlightItalic, config.HighlightUnderline, config.HighlightColor}[clampIdx(i, 4)]
}
func popupPositionIndex(v config.PopupPosition) int {
	switch v {
	case config.PopupStatusItem:
		return 1
	case config.PopupWindow:
		return 2
	case config.PopupCenter:
		return 3
	case config.PopupLast:
		return 4
	}
	return 0
}
func popupPositionAt(i int) config.PopupPosition {
	return [...]config.PopupPosition{config.PopupCursor, config.PopupStatusItem, config.PopupWindow, config.PopupCenter, config.PopupLast}[clampIdx(i, 5)]
}
func searchVisibilityIndex(v config.SearchVisibility) int {
	if v == config.SearchDuringSearch {
		return 1
	}
	return 0
}
func searchVisibilityAt(i int) config.SearchVisibility {
	if i == 1 {
		return config.SearchDuringSearch
	}
	return config.SearchAlways
}
func menuIconIndex(v string) int {
	switch v {
	case "clipboard":
		return 1
	case "scissors":
		return 2
	case "paperclip":
		return 3
	}
	return 0
}
func menuIconAt(i int) string {
	return [...]string{"maccy", "clipboard", "scissors", "paperclip"}[clampIdx(i, 4)]
}
func clampIdx(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}
