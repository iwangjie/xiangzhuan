//go:build darwin

package main

import "github.com/ebitengine/purego/objc"

// The rest screen is a borderless overlay, not a full screen Space: macOS
// lets a Space be swiped away, and leaving full screen plays a slow,
// shrinking animation. An overlay simply covers everything and can fade.
//
// NSWindow levels: the Dock sits at 20, the menu bar at 24 and status items
// at 25, so 26 covers them all. NSWindowCollectionBehavior canJoinAllSpaces
// keeps the overlay on every Space (swiping cannot reach the desktop), and
// fullScreenAuxiliary shows it over another app's full screen window.
const (
	overlayLevel       = 26
	collectionBehavior = (1 << 0) | (1 << 8) // canJoinAllSpaces | fullScreenAuxiliary
)

var (
	selSetStyleMask      = objc.RegisterName("setStyleMask:")
	selSetLevel          = objc.RegisterName("setLevel:")
	selSetCollectionBeha = objc.RegisterName("setCollectionBehavior:")
)

// configureOverlay makes the window of handle a borderless, menu-bar-high
// overlay. Call it on the main thread once the window exists.
func configureOverlay(handle uintptr) {
	if handle == 0 {
		return
	}
	w := objc.ID(handle)
	// Borderless: a titled window is held below the menu bar by macOS.
	w.Send(selSetStyleMask, 0)
	w.Send(selSetLevel, overlayLevel)
	w.Send(selSetCollectionBeha, collectionBehavior)
}
