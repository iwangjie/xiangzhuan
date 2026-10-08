//go:build !darwin

package main

import (
	"time"

	"github.com/egoist/mygo"
)

// Windows has no menu bar or Dock of its own for the app: the tray icon in
// the notification area is its only face, an icon there carries no text,
// and the window opens on the first launch so that starting the app shows
// something. (Other platforms than macOS and Windows are not shipped; they
// follow the same rules as Windows.)
const menuBarApp = false

// trayIcon is the seal glyph in the ember color of the rest screen: Windows
// does not tint tray icons, so the black template image of the menu bar
// would disappear on a dark taskbar.
func trayIcon() ([]byte, bool) { return trayIconColored, false }

// updateTrayTooltip puts the countdown in the tooltip, since the title next
// to the icon is macOS only.
func updateTrayTooltip(t *mygo.Tray, ph phase, left time.Duration) {
	t.SetToolTip(trayTooltip(ph, left))
}
