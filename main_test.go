package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestAdvanceWorkToRest(t *testing.T) {
	a := &app{phase: working, remaining: time.Second, workDuration: time.Minute, restDuration: time.Minute, lastTick: time.Now().Add(-2 * time.Second)}
	changed, old := a.advance(time.Now())
	if !changed || old != working || a.phase != resting {
		t.Fatalf("changed=%v old=%v phase=%v", changed, old, a.phase)
	}
}

func TestAdvanceRestToWork(t *testing.T) {
	a := &app{phase: resting, remaining: time.Second, workDuration: time.Minute, restDuration: time.Minute, lastTick: time.Now().Add(-2 * time.Second)}
	changed, old := a.advance(time.Now())
	if !changed || old != resting || a.phase != working {
		t.Fatalf("changed=%v old=%v phase=%v", changed, old, a.phase)
	}
}

// TestAdvanceFirstTickStartsCounting pins the launch guard: the zero
// lastTick must start the clock instead of looking like an age of elapsed
// time, which used to jump straight into a rest.
func TestAdvanceFirstTickStartsCounting(t *testing.T) {
	a := &app{phase: working, remaining: time.Minute, workDuration: time.Minute, restDuration: time.Minute}
	if changed, ph := a.advance(time.Now()); changed || ph != working {
		t.Fatalf("the first tick jumped: changed=%v ph=%v", changed, ph)
	}
	if a.lastTick.IsZero() {
		t.Fatal("the first tick left lastTick unset")
	}
	if changed, _ := a.advance(a.lastTick.Add(2 * time.Second)); changed {
		t.Fatal("two seconds into a one-minute work phase jumped to rest")
	}
	if got := a.remaining; got != time.Minute-2*time.Second {
		t.Fatalf("remaining=%v after two seconds", got)
	}
}

// TestMenuLabelsAreChinese keeps every menu the user sees free of pinyin
// or English: the app menu bar and the menu bar icon's dropdown.
func TestMenuLabelsAreChinese(t *testing.T) {
	mygo.App.SetName("香篆")
	menus := []*mygo.Menu{appMenuBar(), trayMenu(&app{})}
	var walk func(items []*mygo.MenuItem)
	walk = func(items []*mygo.MenuItem) {
		for _, it := range items {
			if it.Type != mygo.MenuItemSeparator {
				for _, r := range it.Label {
					if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
						t.Errorf("menu label %q is not Chinese", it.Label)
						break
					}
				}
			}
			walk(it.Submenu)
		}
	}
	for _, m := range menus {
		walk(m.Items())
	}
}

// TestWorkWindowCloseHidesButNeverBlocksQuit pins the fix for a silent
// 退出香篆: a quit closes every window first, so the work window must not
// veto the close during a quit — but while the app runs, closing it must
// only hide it.
func TestWorkWindowCloseHidesButNeverBlocksQuit(t *testing.T) {
	a := &app{}
	var e mygo.CloseEvent
	a.windowClose(&e)
	if !e.DefaultPrevented() {
		t.Fatal("closing the work window while running must hide it, not close it")
	}
	a.quitting.Store(true)
	var q mygo.CloseEvent
	a.windowClose(&q)
	if q.DefaultPrevented() {
		t.Fatal("a close during a quit must go through, or the quit is canceled")
	}
}

// TestTrayMenuFollowsSettings pins the dynamic tray menu: the labels show
// the configured durations, and skipping is disabled while it is off.
func TestTrayMenuFollowsSettings(t *testing.T) {
	labels := func(a *app) map[string]*mygo.MenuItem {
		m := map[string]*mygo.MenuItem{}
		for _, it := range trayMenu(a).Items() {
			m[it.Label] = it
		}
		return m
	}
	m := labels(&app{settings: settings{45, 90, false}})
	if m["开始工作 45 分钟"] == nil || m["即刻休息 90 秒"] == nil {
		t.Errorf("labels do not follow the settings: %v", m)
	}
	if it := m["跳过休息"]; it == nil || !it.Disabled {
		t.Error("跳过休息 must be disabled while skipping is off")
	}
	m = labels(&app{settings: settings{40, 60, true}})
	if m["开始工作 40 分钟"] == nil || m["即刻休息 60 秒"] == nil {
		t.Errorf("default labels wrong: %v", m)
	}
	if it := m["跳过休息"]; it == nil || it.Disabled {
		t.Error("跳过休息 must be enabled while skipping is on")
	}
}

// TestTrayTitle pins the short titles the menu bar shows. The bar gives a
// status item only a few characters before macOS 27 folds it behind a
// chevron, so nothing here may grow past two characters.
func TestTrayTitle(t *testing.T) {
	cases := []struct {
		ph   phase
		left time.Duration
		want string
	}{
		{working, 40 * time.Minute, "40"},
		{working, 39 * time.Minute, "39"},
		{working, 90 * time.Second, "1"},
		{working, 59 * time.Second, "59"},
		{working, time.Second, "1"},
		{working, 0, "0"},
		{resting, 60 * time.Second, "休"},
		{resting, 3 * time.Second, "休"},
	}
	for _, c := range cases {
		if got := trayTitle(c.ph, c.left); got != c.want {
			t.Errorf("trayTitle(%v, %v) = %q, want %q", c.ph, c.left, got, c.want)
		}
		if n := len([]rune(trayTitle(c.ph, c.left))); n > 2 {
			t.Errorf("trayTitle(%v, %v) is %d runes wide, the tray wants at most 2", c.ph, c.left, n)
		}
	}
}

// TestTrayTooltip pins the countdown of the notification area's tooltip,
// which is where the remaining time goes on platforms whose tray icon shows
// no title beside it.
func TestTrayTooltip(t *testing.T) {
	cases := []struct {
		ph   phase
		left time.Duration
		want string
	}{
		{working, 40 * time.Minute, "香篆 · 工作中，还剩 40 分钟"},
		{working, 90 * time.Second, "香篆 · 工作中，还剩 1 分钟"},
		{working, 59 * time.Second, "香篆 · 工作中，还剩 59 秒"},
		{working, 0, "香篆 · 工作中，还剩 0 秒"},
		{working, -time.Minute, "香篆 · 工作中，还剩 0 秒"},
		{resting, 60 * time.Second, "香篆 · 休息中，还剩 60 秒"},
		{resting, 3 * time.Second, "香篆 · 休息中，还剩 3 秒"},
	}
	for _, c := range cases {
		if got := trayTooltip(c.ph, c.left); got != c.want {
			t.Errorf("trayTooltip(%v, %v) = %q, want %q", c.ph, c.left, got, c.want)
		}
	}
}
