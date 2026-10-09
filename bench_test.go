package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func benchRestApp() *app {
	dur := 60 * time.Second
	start := time.Now().Add(-5 * time.Second)
	return &app{settings: defaultSettings(), phase: resting, remaining: time.Minute, lastTick: start, restStart: start, activeRestDuration: dur}
}

// BenchmarkRestFrame measures one frame of the rest screen while the ember
// moves: the view build, the layout and the paint, at 1512x982.
func BenchmarkRestFrame(b *testing.B) {
	a := benchRestApp()
	tst := ui.NewTester(a.restView, 1512, 982)
	b.ReportAllocs()
	for b.Loop() {
		tst.Frame()
	}
}

// BenchmarkTick measures the once-a-second tick that counts down and updates
// the menu bar title.
func BenchmarkTick(b *testing.B) {
	a := benchRestApp()
	a.phase = working
	a.remaining = 39 * time.Minute
	a.lastTick = time.Now()
	b.ReportAllocs()
	for b.Loop() {
		a.tick()
	}
}

// BenchmarkSealGeometry measures the seal paths one frame draws.
func BenchmarkSealGeometry(b *testing.B) {
	r := ui.Rect{W: 320, H: 320}
	b.ReportAllocs()
	for b.Loop() {
		base := pathFor(incensePath, 0, 1, r)
		lit := pathFor(incensePath, 0.5, 1, r)
		tail := pathFor(incensePath, 0.49, 0.5, r)
		_ = pointAt(incensePath, 0.5)
		_, _, _ = base, lit, tail
	}
}
