package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestPostponeRestReturnsToWork(t *testing.T) {
	start := time.Now()
	a := &app{phase: resting, settings: defaultSettings(), remaining: time.Minute, lastTick: start, restStart: start, activeRestDuration: time.Minute, restDuration: time.Minute}
	a.settings.AllowSkip = false
	now := start.Add(30 * time.Second)
	a.postponeRest(now)
	if a.phase != working || a.remaining != 5*time.Minute || a.lastTick != now {
		t.Fatal("postpone must start a temporary work countdown")
	}
	a.postponeRest(now.Add(time.Minute))
	if a.remaining != 5*time.Minute {
		t.Fatal("working clicks must be ignored")
	}
	changed, old := a.advance(now.Add(5 * time.Minute))
	if !changed || old != working || a.phase != resting || a.remaining != time.Minute {
		t.Fatal("postponed countdown must lead to a full rest")
	}
	a.beginRest()
	if a.restProgressLocked(a.restStart) != 0 || a.activeRestDuration != time.Minute {
		t.Fatal("next rest must restart from the beginning")
	}
	if a.restProgressLocked(a.restStart.Add(time.Minute)) != 1 {
		t.Fatal("rest must finish at 1")
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

func TestRestViewPostponeButtonAndSkipHint(t *testing.T) {
	start := time.Now()
	a := &app{settings: defaultSettings(), phase: resting, remaining: time.Minute, lastTick: start, restStart: start, activeRestDuration: time.Minute}
	tst := ui.NewTester(a.restView, 800, 600)
	if !tst.HasText("延后 5 分钟") || !tst.HasText("拂灰起行   Esc") {
		t.Fatalf("rest screen texts missing: %v", tst.Texts())
	}
	if err := tst.Click("延后 5 分钟"); err != nil {
		t.Fatal(err)
	}
	if a.phase != working || a.remaining != 5*time.Minute {
		t.Fatalf("clicking 延后 did not extend the rest: %v", a.remaining)
	}
	// Skipping turned off: the hint goes away with the Esc shortcut.
	noSkip := defaultSettings()
	noSkip.AllowSkip = false
	b := &app{settings: noSkip, phase: resting, remaining: time.Minute, lastTick: start, restStart: start, activeRestDuration: time.Minute}
	tst = ui.NewTester(b.restView, 800, 600)
	if tst.HasText("拂灰起行   Esc") {
		t.Fatal("the skip hint must hide while skipping is off")
	}
	if !tst.HasText("延后 5 分钟") {
		t.Fatal("延后 must stay available without skipping")
	}
}
