package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
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
