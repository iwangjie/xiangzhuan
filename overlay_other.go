//go:build !darwin

package main

// configureOverlay is the macOS overlay tweak; see overlay_darwin.go.
// Elsewhere the rest window keeps the options it was created with (see
// main.go): Frameless and AlwaysOnTop over the whole display, and
// SkipTaskbar, which leaves it out of the Windows taskbar.
func configureOverlay(uintptr) {}
