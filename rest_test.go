package main

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// TestRestsAskForScreensWithoutTheSettingsWindow pins the coupling that once
// made a rest do nothing: the settings window is only built when it is
// opened, so a rest must not wait for it.
// TestARestChildNeverRunsAnother pins the guard against the fork bomb the
// first cut of this had: a rest child ran another, which ran another, until
// the chain was killed.
func TestARestChildNeverRunsAnother(t *testing.T) {
	t.Setenv(restChildEnv, "1")
	if spawnRestChild(&app{}) {
		t.Fatal("a rest child must not run another child")
	}
}

// TestTheDrawerDrawsInsteadOfRunningAChild pins the other half of that
// guard: the process that has a restOutcome is the one drawing the screens.
func TestTheDrawerDrawsInsteadOfRunningAChild(t *testing.T) {
	previousSpawn, previousScreens := spawnRestChild, restScreens
	defer func() { spawnRestChild, restScreens = previousSpawn, previousScreens }()

	spawned, drew := false, false
	spawnRestChild = func(*app) bool {
		spawned = true
		return true
	}
	restScreens = func(*app) []*mygo.Window {
		drew = true
		return nil
	}

	a := &app{settings: defaultSettings(), showsWindows: true, reportOutcome: func(string) {}}
	a.beginRest()
	if spawned {
		t.Fatal("the drawer must not run a child of its own")
	}
	if !drew {
		t.Fatal("the drawer must draw the screens itself")
	}
}

func TestParseRestChildArgs(t *testing.T) {
	flags, err := parseRestChildArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if flags.seconds != int(defaultRest.Seconds()) || flags.postpone != defaultSettings().PostponeMinutes || !flags.allowSkip {
		t.Fatalf("defaults = %+v", flags)
	}
	flags, err = parseRestChildArgs([]string{"--seconds", "90", "--postpone", "3", "--skip=false"})
	if err != nil {
		t.Fatal(err)
	}
	if flags.seconds != 90 || flags.postpone != 3 || flags.allowSkip {
		t.Fatalf("flags = %+v", flags)
	}
	if _, err := parseRestChildArgs([]string{"--seconds", "0"}); err == nil {
		t.Fatal("a rest of no seconds must be refused")
	}
	if _, err := parseRestChildArgs([]string{"--nonsense"}); err == nil {
		t.Fatal("an unknown flag must be refused")
	}
}

// TestARestChildLeavesWithTheTray pins how a child that is showing rest
// screens dies with the process that ran it: the tray holds the other end of
// the pipe, so the close is the signal. A pid check would not work on
// Windows, where a dead parent's pid stays readable.
func TestARestChildLeavesWithTheTray(t *testing.T) {
	tray, child := pipeForTest(t)
	left := make(chan struct{})
	go watchTheTray(child, func() { close(left) })
	select {
	case <-left:
		t.Fatal("the child must stay while the tray holds the pipe")
	case <-time.After(100 * time.Millisecond):
	}
	tray.Close()
	select {
	case <-left:
	case <-time.After(2 * time.Second):
		t.Fatal("the child must leave when the tray closes its end")
	}
}

func TestRestsAskForScreensWithoutTheSettingsWindow(t *testing.T) {
	asked := 0
	previous, previousSpawn := restScreens, spawnRestChild
	restScreens = func(*app) []*mygo.Window {
		asked++
		return nil
	}
	spawnRestChild = func(*app) bool { return false }
	defer func() { restScreens, spawnRestChild = previous, previousSpawn }()

	a := &app{settings: defaultSettings(), showsWindows: true}
	if a.window != nil {
		t.Fatal("this test is about a rest with no settings window built")
	}
	a.beginRest()
	if asked != 1 {
		t.Fatalf("a rest asked for its screens %d times, want 1", asked)
	}
}

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
	if !tst.HasText("延后 5 分钟") || !tst.HasText("拂灰起行") || !tst.HasText("Esc") {
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
	if tst.HasText("拂灰起行") || tst.HasText("Esc") {
		t.Fatal("the skip hint must hide while skipping is off")
	}
	if !tst.HasText("延后 5 分钟") {
		t.Fatal("延后 must stay available without skipping")
	}
}

// pipeForTest is a pipe pair: the tray keeps the first end, the child reads
// the second.
func pipeForTest(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	child, tray, err := os.Pipe() // the read end goes to the child, this keeps the write end
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Close() })
	return tray, child
}
