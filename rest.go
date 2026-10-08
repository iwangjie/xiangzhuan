package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *app) overlayWindows() []*mygo.Window {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*mygo.Window, 0, len(a.restWindows))
	for _, w := range a.restWindows {
		out = append(out, w)
	}
	return out
}

// Reconcile only between rounds: retain connected displays' overlays and close
// detached ones. Mid-round display changes take effect at the next rest.
func (a *app) syncRestWindows(displays []mygo.Display) []*mygo.Window {
	return a.syncOverlays(displays, func() *mygo.Window {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "香篆 · 休息", Frameless: true, AlwaysOnTop: true, DisableShadow: true, SkipTaskbar: true, Hidden: true, Content: ui.View(a.restView)})
		configureOverlay(w.NativeHandle())
		return w
	}, func(w *mygo.Window, d mygo.Display) { w.SetBounds(d.Bounds) }, func(w *mygo.Window) { w.Hide(); w.Close() })
}

// Call native APIs outside mu: drawing on the UI thread also takes mu.
func (a *app) syncOverlays(displays []mygo.Display, create func() *mygo.Window, place func(*mygo.Window, mygo.Display), remove func(*mygo.Window)) []*mygo.Window {
	a.mu.Lock()
	old := a.restWindows
	a.mu.Unlock()
	next := make(map[int64]*mygo.Window, len(displays))
	windows := make([]*mygo.Window, 0, len(displays))
	for _, d := range displays {
		w := old[d.ID]
		if w == nil {
			w = create()
		}
		next[d.ID] = w
		place(w, d)
		windows = append(windows, w)
	}
	a.mu.Lock()
	a.restWindows = next
	a.mu.Unlock()
	for id, w := range old {
		if _, ok := next[id]; !ok {
			remove(w)
		}
	}
	return windows
}

func (a *app) restProgressLocked(now time.Time) float32 {
	if a.activeRestDuration <= 0 {
		return 1
	}
	elapsed := max(0, min(1, float64(now.Sub(a.restStart))/float64(a.activeRestDuration)))
	return a.restProgressBase + (1-a.restProgressBase)*float32(elapsed)
}

func (a *app) extendRest(now time.Time) {
	a.mu.Lock()
	if a.phase != resting {
		a.mu.Unlock()
		return
	}
	progress := a.restProgressLocked(now)
	left := a.remaining - max(time.Duration(0), now.Sub(a.lastTick))
	// An already-ended round cannot be resurrected by a late click.
	if left <= 0 {
		a.mu.Unlock()
		return
	}
	a.remaining = left + time.Duration(a.settings.ExtendMinutes)*time.Minute
	a.lastTick = now
	a.restStart = now
	a.restProgressBase = progress
	a.activeRestDuration = a.remaining
	a.mu.Unlock()
	a.invalidate()
}
