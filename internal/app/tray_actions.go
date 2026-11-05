package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/egoist/mygo/ui"

	"github.com/binyouo/Maccy-MyGo/internal/config"
)

func captureDisabled(cfg config.Config) bool {
	return cfg.IgnoreEvents || (!cfg.SaveText && !cfg.SaveFiles && !cfg.SaveImages)
}

func trayIconWithState(name string, disabled bool) []byte {
	data := trayIconFor(name)
	if !disabled {
		return data
	}
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	dimmed := image.NewNRGBA(source.Bounds())
	for y := source.Bounds().Min.Y; y < source.Bounds().Max.Y; y++ {
		for x := source.Bounds().Min.X; x < source.Bounds().Max.X; x++ {
			pixel := color.NRGBAModel.Convert(source.At(x, y)).(color.NRGBA)
			pixel.A = uint8(float64(pixel.A) * 0.45)
			dimmed.SetNRGBA(x, y, pixel)
		}
	}
	var out bytes.Buffer
	_ = png.Encode(&out, dimmed)
	return out.Bytes()
}

func (a *App) updateTrayCaptureState(cfg config.Config) {
	disabled := captureDisabled(cfg)
	if a.trayIconName == cfg.MenuIcon && a.trayDisabled == disabled {
		return
	}
	_ = a.tray.SetIcon(trayIconWithState(cfg.MenuIcon, disabled), true)
	tip := "Maccy MyGo"
	if disabled {
		tip += "（已暂停记录）"
	}
	a.tray.SetToolTip(tip)
	a.trayIconName, a.trayDisabled = cfg.MenuIcon, disabled
}

// AppDelegate.performStatusItemClick uses Option to pause capture, with
// Shift limiting the pause to the next copy. These clicks do not open a panel.
func (a *App) statusItemClick(mods ui.Modifiers) {
	if mods&ui.Alt != 0 {
		_ = a.cfg.Update(func(c *config.Config) {
			c.IgnoreEvents = !c.IgnoreEvents
			if mods&ui.Shift != 0 {
				c.IgnoreOnlyNextEvent = c.IgnoreEvents
			}
		})
		a.refresh()
		return
	}
	if a.panel.Load() != nil {
		a.closePopup()
		return
	}
	a.openPopupAt(config.PopupStatusItem)
}
