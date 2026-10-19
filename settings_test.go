package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSettingsPersistenceAndFallback(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	if got := loadSettings(p); got != defaultSettings() {
		t.Fatal(got)
	}
	if _, stored := readSettings(p); stored {
		t.Fatal("a missing file must not claim a stored login setting")
	}
	want := settings{WorkMinutes: 45, RestSeconds: 90, AllowSkip: false, PostponeMinutes: 5, OpenAtLogin: false}
	if err := saveSettings(p, want); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(p); got != want {
		t.Fatal(got)
	}
	if _, stored := readSettings(p); !stored {
		t.Fatal("saved settings must store the login setting")
	}
	for _, data := range []string{"{bad", `{"work_minutes":45,broken}`} {
		if err := os.WriteFile(p, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if got := loadSettings(p); got != defaultSettings() {
			t.Fatal(got)
		}
	}
	// A file from before the setting existed gets its default: on, and does
	// not look like a stored choice.
	os.WriteFile(p, []byte(`{"work_minutes":999,"rest_seconds":1}`), 0600)
	got, stored := readSettings(p)
	if got != (settings{WorkMinutes: 50, RestSeconds: 60, AllowSkip: true, PostponeMinutes: 5, OpenAtLogin: true}) {
		t.Fatal(got)
	}
	if stored {
		t.Fatal("a legacy file must not claim a stored login setting")
	}
	os.WriteFile(p, []byte(`{"open_at_login":false}`), 0600)
	got, stored = readSettings(p)
	if !stored || got.OpenAtLogin {
		t.Fatalf("stored opt-out ignored: %+v, stored=%v", got, stored)
	}
}
func TestSettingsDoNotInterruptCurrentRound(t *testing.T) {
	a := &app{settings: defaultSettings(), remaining: 12 * time.Minute, workDuration: defaultWork, restDuration: defaultRest, activeRestDuration: defaultRest}
	a.applySettings(settings{WorkMinutes: 45, RestSeconds: 90, AllowSkip: false, PostponeMinutes: 5, OpenAtLogin: true})
	if a.remaining != 12*time.Minute || a.activeRestDuration != defaultRest {
		t.Fatal("current round changed")
	}
	a.skipRest()
	if a.remaining != 12*time.Minute {
		t.Fatal("disabled skip was allowed")
	}
	a.startWork()
	if a.remaining != 45*time.Minute {
		t.Fatal("restart ignored settings")
	}
	a.beginRest()
	if a.activeRestDuration != 90*time.Second {
		t.Fatal("next rest ignored settings")
	}
	a.applySettings(settings{WorkMinutes: 40, RestSeconds: 120, AllowSkip: true, PostponeMinutes: 5, OpenAtLogin: true})
	if a.activeRestDuration != 90*time.Second {
		t.Fatal("active rest changed")
	}
	a.skipRest()
	if a.phase != working || a.remaining != 40*time.Minute {
		t.Fatal("enabled skip failed")
	}
}

// TestLoginItemFollowsTheSetting pins that the system is touched only when
// the option changes, and in the direction the option asks for.
func TestLoginItemFollowsTheSetting(t *testing.T) {
	var calls []bool
	a := &app{
		settings:     defaultSettings(),
		settingsPath: filepath.Join(t.TempDir(), "settings.json"),
		setLoginItem: func(open bool) error {
			calls = append(calls, open)
			return nil
		},
	}
	off := settings{WorkMinutes: 40, RestSeconds: 60, AllowSkip: true, PostponeMinutes: 5, OpenAtLogin: false}
	a.applySettings(off)
	if len(calls) != 1 || calls[0] {
		t.Fatalf("turning it off called %v", calls)
	}
	// Another change must not register or unregister the app again.
	a.applySettings(settings{WorkMinutes: 45, RestSeconds: 60, AllowSkip: true, PostponeMinutes: 5, OpenAtLogin: false})
	if len(calls) != 1 {
		t.Fatalf("an unrelated change touched the login item: %v", calls)
	}
	a.applySettings(settings{WorkMinutes: 45, RestSeconds: 60, AllowSkip: true, PostponeMinutes: 5, OpenAtLogin: true})
	if len(calls) != 2 || !calls[1] {
		t.Fatalf("turning it on called %v", calls)
	}
}

// TestStartupLoginItem pins the launch rules: a settings file from before
// the option exists registers the default, a stored opt-out removes an old
// entry, and a stored opt-in is left to the system's own login settings.
func TestStartupLoginItem(t *testing.T) {
	calls := 0
	a := &app{
		settings: defaultSettings(),
		setLoginItem: func(bool) error {
			calls++
			return nil
		},
	}
	a.startupOpenAtLogin(false)
	if calls != 1 {
		t.Fatalf("legacy settings registered the default %d times", calls)
	}
	a.startupOpenAtLogin(true)
	if calls != 1 {
		t.Fatalf("a stored opt-in was applied again: %d calls", calls)
	}
	a.settings.OpenAtLogin = false
	a.startupOpenAtLogin(true)
	if calls != 2 {
		t.Fatalf("a stored opt-out was not applied: %d calls", calls)
	}
}
