package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

type settings struct {
	WorkMinutes     int  `json:"work_minutes"`
	RestSeconds     int  `json:"rest_seconds"`
	AllowSkip       bool `json:"allow_skip"`
	PostponeMinutes int  `json:"postpone_minutes"`
	OpenAtLogin     bool `json:"open_at_login"`
}

func defaultSettings() settings {
	return settings{
		WorkMinutes:     40,
		RestSeconds:     60,
		AllowSkip:       true,
		PostponeMinutes: 5,
		OpenAtLogin:     true,
	}
}
func (s settings) normalized() settings {
	s.WorkMinutes = max(30, min(50, s.WorkMinutes))
	s.RestSeconds = max(60, min(120, s.RestSeconds))
	s.PostponeMinutes = max(1, min(30, s.PostponeMinutes))
	return s
}

// readSettings reads settings.json and reports whether the file names the
// login-at-startup option. A file from before the option existed says no, so
// startup registers the default instead of mistaking the decoded default for
// an earlier choice.
func readSettings(path string) (settings, bool) {
	s := defaultSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		return s, false
	}
	// Decode into defaults so omitted fields retain their defaults. Bad JSON
	// must not leave partially decoded values behind.
	candidate := s
	if json.Unmarshal(data, &candidate) != nil {
		return s, false
	}
	var fields map[string]json.RawMessage
	stored := false
	if json.Unmarshal(data, &fields) == nil {
		_, stored = fields["open_at_login"]
	}
	return candidate.normalized(), stored
}

func loadSettings(path string) settings {
	s, _ := readSettings(path)
	return s
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
	loginChanged := a.settings.OpenAtLogin != s.OpenAtLogin
	a.settings = s
	a.workDuration = time.Duration(s.WorkMinutes) * time.Minute
	a.restDuration = time.Duration(s.RestSeconds) * time.Second
	path := a.settingsPath
	a.mu.Unlock()
	if err := saveSettings(path, s); err != nil {
		log.Printf("保存设置失败: %v", err)
	}
	if loginChanged {
		a.applyOpenAtLogin(s.OpenAtLogin)
	}
	if a.tray != nil {
		a.tray.SetMenu(trayMenu(a))
	}
	a.invalidate()
}

// startupOpenAtLogin applies the stored setting. A file from before the
// option existed registers the default; a stored opt-out removes an entry
// left behind. A stored opt-in is not applied again, so turning the item off
// in the system's own login settings stays off.
func (a *app) startupOpenAtLogin(stored bool) {
	if !stored || !a.settings.OpenAtLogin {
		a.applyOpenAtLogin(a.settings.OpenAtLogin)
	}
}

// applyOpenAtLogin makes the system start the app when the user logs in, or
// stop doing so. A nil setLoginItem (tests, the rest child) leaves the
// machine alone.
func (a *app) applyOpenAtLogin(open bool) {
	if a.setLoginItem == nil {
		return
	}
	if err := a.setLoginItem(open); err != nil {
		log.Printf("设置登录自启动失败: %v", err)
	}
}
func (a *app) canSkip() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.AllowSkip
}
