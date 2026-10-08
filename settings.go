package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

type settings struct {
	WorkMinutes   int  `json:"work_minutes"`
	RestSeconds   int  `json:"rest_seconds"`
	AllowSkip     bool `json:"allow_skip"`
	ExtendMinutes int  `json:"extend_minutes"`
}

func defaultSettings() settings { return settings{40, 60, true, 5} }
func (s settings) normalized() settings {
	s.WorkMinutes = max(30, min(50, s.WorkMinutes))
	s.RestSeconds = max(60, min(120, s.RestSeconds))
	s.ExtendMinutes = max(1, min(30, s.ExtendMinutes))
	return s
}
func loadSettings(path string) settings {
	s := defaultSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	// Decode into defaults so omitted fields retain their defaults. Bad JSON
	// must not leave partially decoded values behind.
	candidate := s
	if json.Unmarshal(data, &candidate) != nil {
		return s
	}
	return candidate.normalized()
}

// settingsExist reports whether the app has run before: the first launch on
// a platform without a menu bar writes the settings file (see main.go).
func settingsExist(path string) bool {
	if path == "" {
		// Nowhere to record it: treat every launch as a later one, so the
		// window does not open every time.
		return true
	}
	_, err := os.Stat(path)
	return err == nil
}

func saveSettings(path string, s settings) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (a *app) applySettings(s settings) {
	s = s.normalized()
	a.mu.Lock()
	a.settings = s
	a.workDuration = time.Duration(s.WorkMinutes) * time.Minute
	a.restDuration = time.Duration(s.RestSeconds) * time.Second
	path := a.settingsPath
	a.mu.Unlock()
	if err := saveSettings(path, s); err != nil {
		log.Printf("保存设置失败: %v", err)
	}
	if a.tray != nil {
		a.tray.SetMenu(trayMenu(a))
	}
	a.invalidate()
}
func (a *app) canSkip() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.AllowSkip
}
