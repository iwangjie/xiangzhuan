package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func newRestingApp() *app {
	start := time.Now()
	a := &app{settings: defaultSettings(), workDuration: defaultWork, restDuration: defaultRest, remaining: defaultWork, lastTick: start}
	a.beginRest()
	return a
}

func TestThirdPostponeForcesTheNextRest(t *testing.T) {
	previous := spawnRestChild
	spawnRestChild = func(*app) bool { return false }
	defer func() { spawnRestChild = previous }()

	a := newRestingApp()
	for i := 1; i <= maxPostpones; i++ {
		if a.forcedReason != "" {
			t.Fatalf("rest %d is forced too early", i)
		}
		a.postponeRest(time.Now())
		if a.phase != working || a.postponeCount != i {
			t.Fatalf("postpone %d: phase=%v count=%d", i, a.phase, a.postponeCount)
		}
		a.beginRest()
	}
	if a.forcedReason != forcedByPostpone {
		t.Fatalf("after %d postpones the rest must be forced, got %q", maxPostpones, a.forcedReason)
	}
	a.postponeRest(time.Now())
	a.skipRest()
	if a.phase != resting || a.postponeCount != maxPostpones || a.skipCount != 0 {
		t.Fatal("a forced rest must refuse both postponing and skipping")
	}
	a.userStartWork()
	if a.phase != resting {
		t.Fatal("start work must not end a forced rest")
	}
}

func TestSkipOnceForcesTheNextRest(t *testing.T) {
	previous := spawnRestChild
	spawnRestChild = func(*app) bool { return false }
	defer func() { spawnRestChild = previous }()

	a := newRestingApp()
	a.skipRest()
	if a.phase != working || a.skipCount != 1 {
		t.Fatalf("the first skip must work: phase=%v count=%d", a.phase, a.skipCount)
	}
	a.beginRest()
	if a.forcedReason != forcedBySkip {
		t.Fatalf("the rest after a skip must be forced, got %q", a.forcedReason)
	}
	a.skipRest()
	a.postponeRest(time.Now())
	if a.phase != resting {
		t.Fatal("a forced rest must have no way out")
	}
}

func TestSkipOutranksPostponeInTheMessage(t *testing.T) {
	a := &app{postponeCount: maxPostpones, skipCount: maxSkips}
	if got := a.forcedReasonLocked(); got != forcedBySkip {
		t.Fatalf("got %q", got)
	}
}

func TestAFullRestClearsTheLimits(t *testing.T) {
	previous := spawnRestChild
	spawnRestChild = func(*app) bool { return false }
	defer func() { spawnRestChild = previous }()

	a := newRestingApp()
	a.postponeRest(time.Now())
	a.beginRest()
	a.applyRestOutcome(outcomeDone)
	if a.postponeCount != 0 || a.skipCount != 0 || a.forcedReason != "" || a.phase != working {
		t.Fatalf("limits not cleared: %+v", a)
	}
	a.beginRest()
	if a.forcedReason != "" {
		t.Fatal("the rest after a full one is not forced")
	}
}

func TestAnUnexplainedChildExitKeepsTheLimits(t *testing.T) {
	previous := spawnRestChild
	spawnRestChild = func(*app) bool { return false }
	defer func() { spawnRestChild = previous }()

	a := newRestingApp()
	a.skipCount = 1
	a.applyRestOutcome("")
	if a.skipCount != 1 {
		t.Fatal("a child that left without an outcome must not clear the limits")
	}
}

func TestForcedRestScreenShowsTheNoticeOnly(t *testing.T) {
	start := time.Now()
	a := &app{settings: defaultSettings(), phase: resting, remaining: time.Minute, lastTick: start, restStart: start, activeRestDuration: time.Minute, forcedReason: forcedByPostpone}
	tst := ui.NewTester(a.restView, 800, 600)
	if !tst.HasText(forcedByPostpone) {
		t.Fatalf("notice missing: %v", tst.Texts())
	}
	if tst.HasText("延后 5 分钟") || tst.HasText("拂灰起行") || tst.HasText("Esc") {
		t.Fatalf("a forced rest must offer no way out: %v", tst.Texts())
	}
}

func TestRestChildCarriesTheCounters(t *testing.T) {
	flags, err := parseRestChildArgs([]string{"--postponed", "3", "--skipped", "1"})
	if err != nil {
		t.Fatal(err)
	}
	if flags.postponed != 3 || flags.skipped != 1 {
		t.Fatalf("flags = %+v", flags)
	}
	a := &app{postponeCount: flags.postponed, skipCount: flags.skipped}
	if a.forcedReasonLocked() != forcedBySkip {
		t.Fatal("the child must work out the same forced rest as the tray")
	}
	if _, err := parseRestChildArgs([]string{"--skipped", "-1"}); err == nil {
		t.Fatal("negative counters must be refused")
	}
}
