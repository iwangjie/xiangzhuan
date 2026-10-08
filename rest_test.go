package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestExtendRestKeepsProgress(t *testing.T) {
	start := time.Now()
	a := &app{phase: resting, settings: defaultSettings(), remaining: time.Minute, lastTick: start, restStart: start, activeRestDuration: time.Minute}
	now := start.Add(30 * time.Second)
	before := a.restProgressLocked(now)
	a.extendRest(now)
	if a.remaining != 330*time.Second || a.restProgressLocked(now) != before {
		t.Fatalf("remaining=%v progress=%v, before=%v", a.remaining, a.restProgressLocked(now), before)
	}
	later := now.Add(time.Minute)
	if a.restProgressLocked(later) < before {
		t.Fatal("progress went backwards")
	}
	p := a.restProgressLocked(later)
	a.extendRest(later)
	if a.remaining != 570*time.Second || a.restProgressLocked(later) != p {
		t.Fatal("stacked extension changed progress or remaining")
	}
	if a.restProgressLocked(later.Add(a.remaining)) != 1 {
		t.Fatal("progress did not finish at 1")
	}
}

func TestOverlayCountMatchesDisplays(t *testing.T) {
	a := &app{}
	created, removed := 0, 0
	create := func() *mygo.Window { created++; return new(mygo.Window) }
	place := func(*mygo.Window, mygo.Display) {}
	remove := func(*mygo.Window) { removed++ }
	displays := []mygo.Display{{ID: 1}, {ID: 2}}
	windows := a.syncOverlays(displays, create, place, remove)
	if len(windows) != 2 || len(a.overlayWindows()) != 2 || created != 2 {
		t.Fatal("expected one overlay per display")
	}
	first := a.restWindows[1]
	a.syncOverlays([]mygo.Display{{ID: 1}, {ID: 3}}, create, place, remove)
	if a.restWindows[1] != first || created != 3 || removed != 1 || len(a.overlayWindows()) != 2 {
		t.Fatal("display reconciliation did not reuse and remove overlays")
	}
}

func TestRestViewExtendButtonAndSkipHint(t *testing.T) {
	start := time.Now()
	a := &app{settings: defaultSettings(), phase: resting, remaining: time.Minute, lastTick: start, restStart: start, activeRestDuration: time.Minute}
	tst := ui.NewTester(a.restView, 800, 600)
	if !tst.HasText("续香 5 分钟") || !tst.HasText("拂灰起行   Esc") {
		t.Fatalf("rest screen texts missing: %v", tst.Texts())
	}
	if err := tst.Click("续香 5 分钟"); err != nil {
		t.Fatal(err)
	}
	if a.remaining < 5*time.Minute {
		t.Fatalf("clicking 续香 did not extend the rest: %v", a.remaining)
	}
	// Skipping turned off: the hint goes away with the Esc shortcut.
	noSkip := defaultSettings()
	noSkip.AllowSkip = false
	b := &app{settings: noSkip, phase: resting, remaining: time.Minute, lastTick: start, restStart: start, activeRestDuration: time.Minute}
	tst = ui.NewTester(b.restView, 800, 600)
	if tst.HasText("拂灰起行   Esc") {
		t.Fatal("the skip hint must hide while skipping is off")
	}
	if !tst.HasText("续香 5 分钟") {
		t.Fatal("续香 must stay available without skipping")
	}
}
