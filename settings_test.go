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
	want := settings{45, 90, false}
	if err := saveSettings(p, want); err != nil {
		t.Fatal(err)
	}
	if got := loadSettings(p); got != want {
		t.Fatal(got)
	}
	for _, data := range []string{"{bad", `{"work_minutes":45,broken}`} {
		if err := os.WriteFile(p, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if got := loadSettings(p); got != defaultSettings() {
			t.Fatal(got)
		}
	}
	os.WriteFile(p, []byte(`{"work_minutes":999,"rest_seconds":1}`), 0600)
	if got := loadSettings(p); got != (settings{50, 60, true}) {
		t.Fatal(got)
	}
}
func TestSettingsDoNotInterruptCurrentRound(t *testing.T) {
	a := &app{settings: defaultSettings(), remaining: 12 * time.Minute, workDuration: defaultWork, restDuration: defaultRest, activeRestDuration: defaultRest}
	a.applySettings(settings{45, 90, false})
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
	a.applySettings(settings{40, 120, true})
	if a.activeRestDuration != 90*time.Second {
		t.Fatal("active rest changed")
	}
	a.skipRest()
	if a.phase != working || a.remaining != 40*time.Minute {
		t.Fatal("enabled skip failed")
	}
}
