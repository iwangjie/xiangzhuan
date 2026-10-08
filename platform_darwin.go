//go:build darwin

package main

import (
	"time"

	"github.com/egoist/mygo"
)

// The app is a menu bar app: macOS shows the tray icon with a title beside
// it, tints a template image to match the menu bar, and gives the app menu
// the whole window's width above every window.
const menuBarApp = true

// trayIcon is the seal glyph as a template image — black and transparent,
// which the menu bar tints for light and dark mode.
func trayIcon() ([]byte, bool) { return trayIconTemplate, true }

// updateTrayTooltip does nothing: the countdown is the title next to the
// icon, and the tooltip keeps the app's tagline.
func updateTrayTooltip(*mygo.Tray, phase, time.Duration) {}
