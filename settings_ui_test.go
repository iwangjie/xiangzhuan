package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestWorkViewControlsSaveSettings drives the real settings view in the
// headless UI tester: changing a control must persist to settings.json at
// once, and a restart must use the stored value.
func TestWorkViewControlsSaveSettings(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	a := &app{settings: defaultSettings(), settingsPath: p, remaining: defaultWork, workDuration: defaultWork, restDuration: defaultRest}
	tst := ui.NewTester(a.workView, 420, 440)
	if !tst.HasText("允许跳过休息") || !tst.HasText("登录时自动启动") || !tst.HasText("工作时长") || !tst.HasText("休息时长") {
		t.Fatalf("settings controls missing: %v", tst.Texts())
	}
	// Uncheck 登录时自动启动: the file must show the opt-out.
	if err := tst.Click("登录时自动启动"); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(p); got.OpenAtLogin {
		t.Fatalf("unchecking login startup did not save: %+v", got)
	}
	if err := tst.Click("登录时自动启动"); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(p); !got.OpenAtLogin {
		t.Fatalf("checking login startup again did not save: %+v", got)
	}
	// Uncheck 允许跳过休息: the file must show skipping off.
	if err := tst.Click("允许跳过休息"); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(p); got != (settings{WorkMinutes: 40, RestSeconds: 60, AllowSkip: false, PostponeMinutes: 5, OpenAtLogin: true}) {
		t.Fatalf("unchecking did not save: %+v", got)
	}
	// Check it again: back to the defaults.
	if err := tst.Click("允许跳过休息"); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(p); got != defaultSettings() {
		t.Fatalf("checking again did not save: %+v", got)
	}
	// One step up on the work minutes (the first + is the work row).
	if err := tst.Click("+"); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(p); got != (settings{WorkMinutes: 41, RestSeconds: 60, AllowSkip: true, PostponeMinutes: 5, OpenAtLogin: true}) {
		t.Fatalf("stepping up did not save: %+v", got)
	}
	// 重新起香 starts a fresh round with the stored 41 minutes.
	if err := tst.Click("重新起香"); err != nil {
		t.Fatal(err)
	}
	if a.remaining != 41*time.Minute {
		t.Fatalf("restart used %v, want 41m", a.remaining)
	}
}
