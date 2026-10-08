package main

import "testing"

func TestUpdateProgress(t *testing.T) {
	for _, tc := range []struct {
		downloaded, total int64
		want              string
	}{
		{0, 0, "香篆 · 正在下载更新…"},
		{50, 100, "香篆 · 正在下载更新 50%"},
		{120, 100, "香篆 · 正在下载更新 100%"},
		{-1, 100, "香篆 · 正在下载更新 0%"},
	} {
		if got := updateProgress(tc.downloaded, tc.total); got != tc.want {
			t.Fatalf("updateProgress(%d, %d) = %q, want %q", tc.downloaded, tc.total, got, tc.want)
		}
	}
}

func TestUpdateCheckGuards(t *testing.T) {
	a := &app{}
	a.quitting.Store(true)
	a.checkUpdates(true)
	if a.updateBusy.Load() {
		t.Fatal("quitting must not start an update")
	}
	a.quitting.Store(false)
	a.updateBusy.Store(true)
	a.checkUpdates(true)
	if !a.updateBusy.Load() {
		t.Fatal("duplicate check must not release the active session")
	}
}
