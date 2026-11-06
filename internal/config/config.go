// Package config holds Maccy's preferences, persisted as JSON in the
// application's user data directory.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// SortBy is how the history is ordered.
type SortBy string

const (
	SortLastCopied  SortBy = "lastCopiedAt"
	SortFirstCopied SortBy = "firstCopiedAt"
	SortCopies      SortBy = "numberOfCopies"
)

// PinTo is where pinned items sit.
type PinTo string

const (
	PinTop    PinTo = "top"
	PinBottom PinTo = "bottom"
)

// SearchMode is how the query matches item titles.
type SearchMode string

const (
	SearchExact  SearchMode = "exact"
	SearchFuzzy  SearchMode = "fuzzy"
	SearchRegexp SearchMode = "regexp"
	SearchMixed  SearchMode = "mixed"
)

// HighlightMatch styles the matched part of a title.
type HighlightMatch string

const (
	HighlightBold      HighlightMatch = "bold"
	HighlightItalic    HighlightMatch = "italic"
	HighlightUnderline HighlightMatch = "underline"
	HighlightColor     HighlightMatch = "color"
)

// PopupPosition is where the popup opens.
type PopupPosition string

const (
	PopupCursor     PopupPosition = "cursor"
	PopupStatusItem PopupPosition = "statusItem"
	PopupWindow     PopupPosition = "window"
	PopupCenter     PopupPosition = "center"
	PopupLast       PopupPosition = "lastPosition"
)

// SearchVisibility controls when the search field shows.
type SearchVisibility string

const (
	SearchAlways       SearchVisibility = "always"
	SearchDuringSearch SearchVisibility = "duringSearch"
)

// Config is the full preference set. Defaults mirror Maccy.
type Config struct {
	LaunchAtLogin             bool             `json:"launchAtLogin"`
	ClearOnQuit               bool             `json:"clearOnQuit"`
	ClearSystemClipboard      bool             `json:"clearSystemClipboard"`
	ClipboardCheckInterval    float64          `json:"clipboardCheckInterval"`
	SaveFiles                 bool             `json:"saveFiles"`
	SaveImages                bool             `json:"saveImages"`
	SaveText                  bool             `json:"saveText"`
	HighlightMatch            HighlightMatch   `json:"highlightMatch"`
	IgnoreAllAppsExceptListed bool             `json:"ignoreAllAppsExceptListed"`
	IgnoreEvents              bool             `json:"ignoreEvents"`
	IgnoreOnlyNextEvent       bool             `json:"ignoreOnlyNextEvent"`
	IgnoreRegexp              []string         `json:"ignoreRegexp"`
	IgnoredApps               []string         `json:"ignoredApps"`
	IgnoredPasteboardTypes    []string         `json:"ignoredPasteboardTypes"`
	ImageMaxHeight            int              `json:"imageMaxHeight"`
	MenuIcon                  string           `json:"menuIcon"`
	PasteByDefault            bool             `json:"pasteByDefault"`
	PinTo                     PinTo            `json:"pinTo"`
	PinOrder                  []string         `json:"pinOrder"`
	PopupPosition             PopupPosition    `json:"popupPosition"`
	PopupScreen               int              `json:"popupScreen"`
	HasPopupPosition          bool             `json:"hasPopupPosition"`
	PopupX                    int              `json:"popupX"`
	PopupY                    int              `json:"popupY"`
	HasRelativePopupPosition  bool             `json:"hasRelativePopupPosition"`
	PopupAnchorX              float64          `json:"popupAnchorX"`
	PopupAnchorY              float64          `json:"popupAnchorY"`
	OpenPreviewAutomatically  bool             `json:"openPreviewAutomatically"`
	PreviewDelay              int              `json:"previewDelay"`
	RemoveFormattingByDefault bool             `json:"removeFormattingByDefault"`
	SearchMode                SearchMode       `json:"searchMode"`
	ShowFooter                bool             `json:"showFooter"`
	ShowInStatusBar           bool             `json:"showInStatusBar"`
	ShowRecentCopyInMenuBar   bool             `json:"showRecentCopyInMenuBar"`
	ShowSearch                bool             `json:"showSearch"`
	SearchVisibility          SearchVisibility `json:"searchVisibility"`
	ShowSpecialSymbols        bool             `json:"showSpecialSymbols"`
	ShowTitle                 bool             `json:"showTitle"`
	HistorySize               int              `json:"historySize"`
	SortBy                    SortBy           `json:"sortBy"`
	SuppressClearAlert        bool             `json:"suppressClearAlert"`
	ShowNotifications         bool             `json:"showNotifications"`
	WindowWidth               int              `json:"windowWidth"`
	WindowHeight              int              `json:"windowHeight"`
	ShowApplicationIcons      bool             `json:"showApplicationIcons"`
	ShowHexColorSwatch        bool             `json:"showHexColorSwatch"`
	PreviewWidth              int              `json:"previewWidth"`
	GlobalShortcut            string           `json:"globalShortcut"`
	PinShortcut               string           `json:"pinShortcut"`
	DeleteShortcut            string           `json:"deleteShortcut"`
	PreviewShortcut           string           `json:"previewShortcut"`

	path                string
	intervalSubscribers map[chan struct{}]struct{}
}

var mu sync.Mutex

// Default returns a configuration with Maccy's defaults.
func Default() *Config {
	return &Config{
		ClipboardCheckInterval: 0.5,
		SaveFiles:              true,
		SaveImages:             true,
		SaveText:               true,
		HighlightMatch:         HighlightBold,
		IgnoredPasteboardTypes: []string{
			"Pasteboard generator type",
			"com.agilebits.onepassword",
			"com.typeit4me.clipping",
			"de.petermaurer.TransientPasteboardType",
			"net.antelle.keeweb",
		},
		ImageMaxHeight:           40,
		MenuIcon:                 "maccy",
		PinTo:                    PinTop,
		PopupPosition:            PopupCursor,
		OpenPreviewAutomatically: true,
		PreviewDelay:             1500,
		SearchMode:               SearchExact,
		ShowFooter:               true,
		ShowInStatusBar:          true,
		ShowSearch:               true,
		SearchVisibility:         SearchAlways,
		ShowSpecialSymbols:       true,
		ShowTitle:                true,
		HistorySize:              200,
		SortBy:                   SortLastCopied,
		WindowWidth:              450,
		WindowHeight:             800,
		ShowHexColorSwatch:       true,
		ShowNotifications:        true,
		PreviewWidth:             400,
		GlobalShortcut:           "CmdOrCtrl+Shift+C",
		PinShortcut:              "Option+P",
		DeleteShortcut:           "Option+Delete",
		PreviewShortcut:          "Ctrl+Space",
	}
}

// Load reads the configuration at path, keeping defaults for missing fields.
func Load(path string) (*Config, error) {
	c := Default()
	c.path = path
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return c, nil
	}
	return c, nil
}

// Save writes the configuration atomically.
func (c *Config) Save() error {
	mu.Lock()
	defer mu.Unlock()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}

// Snapshot returns a copy safe to read without locking.
func (c *Config) Snapshot() Config {
	mu.Lock()
	defer mu.Unlock()
	cp := *c
	cp.intervalSubscribers = nil
	cp.IgnoreRegexp = append([]string(nil), c.IgnoreRegexp...)
	cp.IgnoredApps = append([]string(nil), c.IgnoredApps...)
	cp.IgnoredPasteboardTypes = append([]string(nil), c.IgnoredPasteboardTypes...)
	cp.PinOrder = append([]string(nil), c.PinOrder...)
	return cp
}

// Update applies fn to a copy, then stores it.
func (c *Config) Update(fn func(*Config)) error {
	mu.Lock()
	oldInterval := c.ClipboardCheckInterval
	cp := *c
	cp.IgnoreRegexp = append([]string(nil), c.IgnoreRegexp...)
	cp.IgnoredApps = append([]string(nil), c.IgnoredApps...)
	cp.IgnoredPasteboardTypes = append([]string(nil), c.IgnoredPasteboardTypes...)
	cp.PinOrder = append([]string(nil), c.PinOrder...)
	path := c.path
	subscribers := c.intervalSubscribers
	fn(&cp)
	cp.path = path
	cp.intervalSubscribers = subscribers
	*c = cp
	if oldInterval != c.ClipboardCheckInterval {
		for ch := range c.intervalSubscribers {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
	mu.Unlock()
	return c.Save()
}

// SubscribeIntervalChanges lets a native timer follow configuration without
// a second polling timer. Events coalesce; readers use Interval for the latest
// value. Cancel removes only this subscription and does not close the channel.
func (c *Config) SubscribeIntervalChanges() (<-chan struct{}, func()) {
	mu.Lock()
	if c.intervalSubscribers == nil {
		c.intervalSubscribers = make(map[chan struct{}]struct{})
	}
	ch := make(chan struct{}, 1)
	c.intervalSubscribers[ch] = struct{}{}
	mu.Unlock()
	return ch, func() {
		mu.Lock()
		delete(c.intervalSubscribers, ch)
		mu.Unlock()
	}
}

// ReplacePinOrder stores a new pin order and saves.
func (c *Config) ReplacePinOrder(order []string) error {
	return c.Update(func(cp *Config) { cp.PinOrder = append([]string(nil), order...) })
}

// SetIgnoreEvents stores the ignore-events flag.
func (c *Config) SetIgnoreEvents(v bool) error {
	return c.Update(func(cp *Config) { cp.IgnoreEvents = v })
}

// SetIgnoreOnlyNext stores the ignore-only-next flag.
func (c *Config) SetIgnoreOnlyNext(v bool) error {
	return c.Update(func(cp *Config) { cp.IgnoreOnlyNextEvent = v })
}

// Interval returns the clipboard polling interval.
func (c *Config) Interval() float64 {
	mu.Lock()
	defer mu.Unlock()
	if c.ClipboardCheckInterval <= 0 {
		return 0.5
	}
	return c.ClipboardCheckInterval
}

// ConsumeIgnoreNext clears a pending "ignore only the next copy" and
// reports whether it did. It leaves a plain ignore-events setting alone.
func (c *Config) ConsumeIgnoreNext() bool {
	mu.Lock()
	changed := c.IgnoreEvents && c.IgnoreOnlyNextEvent
	if changed {
		c.IgnoreEvents = false
		c.IgnoreOnlyNextEvent = false
	}
	mu.Unlock()
	if changed {
		_ = c.Save()
	}
	return changed
}
