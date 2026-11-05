//go:build !darwin

package app

import (
	"errors"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

var errPopupKeyMonitorUnsupported = errors.New("local modifier events are unavailable")

func installPopupKeyMonitor(*mygo.Window, func(popupKeyEvent) bool) (func(), error) {
	return nil, errPopupKeyMonitorUnsupported
}

func currentKeyboardModifiers() ui.Modifiers { return 0 }
