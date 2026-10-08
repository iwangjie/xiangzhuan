package main

import (
	_ "embed"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed resources/tray.png
var trayIconTemplate []byte

//go:embed resources/tray-windows.png
var trayIconColored []byte

type phase int

const (
	working phase = iota
	resting
)

type point struct{ x, y float32 }

var incensePath = []point{{.18, .18}, {.82, .18}, {.82, .82}, {.18, .82}, {.18, .28}, {.72, .28}, {.72, .72}, {.28, .72}, {.28, .38}, {.62, .38}, {.62, .62}, {.38, .62}, {.38, .47}, {.54, .47}, {.54, .54}, {.46, .54}}

const defaultWork = 40 * time.Minute
const defaultRest = 60 * time.Second

// trayTagline is the tray icon's tooltip on macOS, where the countdown sits
// next to the icon as its title instead.
const trayTagline = "香篆 · 一篆香消，万事且抛"

type app struct {
	mu                         sync.Mutex
	phase                      phase
	remaining                  time.Duration
	workDuration, restDuration time.Duration
	lastTick                   time.Time
	restStart                  time.Time
	activeRestDuration         time.Duration
	settings                   settings
	settingsPath               string
	// fadeSeq numbers the rest window's fades; a fade that no longer
	// matches the newest stops, so a skip then a new rest cannot fight
	// over the window.
	fadeSeq int
	// quitting turns true when a quit begins: the work window's close
	// handler vetoes closes to hide the window instead, and that veto
	// must not cancel the quit itself.
	quitting           atomic.Bool
	window, restWindow *mygo.Window
	tray               *mygo.Tray
}

func (a *app) startWork() {
	a.mu.Lock()
	a.phase = working
	a.remaining = a.workDuration
	a.lastTick = time.Now()
	a.mu.Unlock()
	a.hideRest()
	a.invalidate()
}
func (a *app) beginRest() {
	a.mu.Lock()
	a.phase = resting
	a.remaining = a.restDuration
	a.lastTick = time.Now()
	a.restStart = a.lastTick
	a.activeRestDuration = a.restDuration
	a.fadeSeq++
	seq := a.fadeSeq
	a.mu.Unlock()
	if a.restWindow != nil {
		// The overlay covers the whole screen of the display the rest is
		// on, then fades in: no full screen Space to swipe away, and no
		// window-shrinking system animation when it ends.
		if d := mygo.Screen.PrimaryDisplay(); d.Bounds.Width > 0 {
			a.restWindow.SetBounds(d.Bounds)
		}
		a.restWindow.SetOpacity(0)
		a.restWindow.Show()
		mygo.App.Focus()
		a.restWindow.Focus()
		go a.fadeRest(1, 10, seq)
	}
	a.invalidate()
}
func (a *app) skipRest() {
	if a.canSkip() {
		a.startWork()
	}
}

// windowClose hides the work window instead of closing it — except during a
// quit. A quit closes every window first, and a veto there would cancel the
// whole quit (this once made 退出香篆 do nothing).
func (a *app) windowClose(e *mygo.CloseEvent) {
	if a.quitting.Load() {
		return
	}
	e.PreventDefault()
	if a.window != nil {
		a.window.Hide()
	}
}
func (a *app) hideRest() {
	if a.restWindow == nil {
		return
	}
	a.mu.Lock()
	a.fadeSeq++
	seq := a.fadeSeq
	a.mu.Unlock()
	go func() {
		a.fadeRest(0, 15, seq)
		a.mu.Lock()
		stale := a.fadeSeq != seq
		a.mu.Unlock()
		if stale {
			return
		}
		a.restWindow.Hide()
		a.restWindow.SetOpacity(1)
	}()
}

// fadeRest eases the rest window to the given opacity in steps of 16ms,
// giving up once a newer fade has taken over.
func (a *app) fadeRest(to float64, steps, seq int) {
	from := a.restWindow.Opacity()
	for i := 1; i <= steps; i++ {
		a.mu.Lock()
		stale := a.fadeSeq != seq
		a.mu.Unlock()
		if stale {
			return
		}
		a.restWindow.SetOpacity(from + (to-from)*float64(i)/float64(steps))
		time.Sleep(16 * time.Millisecond)
	}
}
func (a *app) invalidate() {
	if a.window != nil {
		a.window.Invalidate()
	}
	if a.restWindow != nil {
		a.restWindow.Invalidate()
	}
}
func (a *app) advance(now time.Time) (bool, phase) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastTick.IsZero() {
		// The first tick after launch: start counting from now, so the
		// zero time.Time does not look like an elapsed age.
		a.lastTick = now
		return false, a.phase
	}
	a.remaining -= now.Sub(a.lastTick)
	a.lastTick = now
	if a.remaining > 0 {
		return false, a.phase
	}
	old := a.phase
	if a.phase == working {
		a.phase = resting
		a.remaining = a.restDuration
	} else {
		a.phase = working
		a.remaining = a.workDuration
	}
	a.lastTick = now
	return true, old
}
func (a *app) tick() {
	changed, old := a.advance(time.Now())
	if changed {
		if old == working {
			a.beginRest()
		} else {
			a.startWork()
		}
		return
	}
	a.mu.Lock()
	left, ph := a.remaining, a.phase
	a.mu.Unlock()
	if a.tray != nil {
		a.tray.SetTitle(trayTitle(ph, left))
		updateTrayTooltip(a.tray, ph, left)
	}
	a.invalidate()
}

// trayTitle is the text next to the menu bar icon. The menu bar folds wide
// status items away on a crowded bar (macOS 27 shows a chevron instead), so
// keep it short: the minutes left while working, the seconds in the last
// minute, and 休 while resting.
func trayTitle(ph phase, left time.Duration) string {
	if ph == resting {
		return "休"
	}
	secs := int(left / time.Second)
	if secs <= 0 {
		return "0"
	}
	if secs >= 60 {
		return fmt.Sprintf("%d", secs/60)
	}
	return fmt.Sprintf("%d", secs)
}

// trayTooltip is the countdown of the notification area's tooltip. An icon
// there carries no text beside it (Windows ignores the title the menu bar
// shows), so the remaining time lives in the tooltip.
func trayTooltip(ph phase, left time.Duration) string {
	secs := max(0, int(left/time.Second))
	if ph == resting {
		return fmt.Sprintf("香篆 · 休息中，还剩 %d 秒", secs)
	}
	if secs >= 60 {
		return fmt.Sprintf("香篆 · 工作中，还剩 %d 分钟", secs/60)
	}
	return fmt.Sprintf("香篆 · 工作中，还剩 %d 秒", secs)
}

func (a *app) workView(c *ui.Context) {
	a.mu.Lock()
	left := a.remaining
	s := a.settings
	a.mu.Unlock()
	workMin, restSec, allowSkip := float64(s.WorkMinutes), float64(s.RestSeconds), s.AllowSkip
	ui.Column(c).Fill().Center().Gap(16).Padding(24, 28).Children(func() {
		ui.Column(c).Center().Gap(4).Children(func() {
			ui.Text(c, "香篆").FontSize(24).Bold()
			ui.Text(c, "焚香理绪，神定案明。").FontSize(13).TextColor(c.Theme().TextMuted)
			ui.Textf(c, "%02d:%02d", int(left/time.Minute), int(left/time.Second)%60).FontSize(38).Bold()
		})
		ui.Column(c).Radius(8).Padding(12, 16).Gap(10).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, "工作时长").FontSize(13)
				ui.Spacer(c)
				ui.NumberInput(c, &workMin, 30, 50, 1)
				ui.Text(c, "分钟").FontSize(12).TextColor(c.Theme().TextMuted)
			})
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, "休息时长").FontSize(13)
				ui.Spacer(c)
				ui.NumberInput(c, &restSec, 60, 120, 5)
				ui.Text(c, "秒").FontSize(12).TextColor(c.Theme().TextMuted)
			})
			ui.Checkbox(c, &allowSkip, "允许跳过休息").FontSize(13)
		})
		updated := settings{int(workMin), int(restSec), allowSkip}
		if updated != s {
			a.applySettings(updated)
		}
		if ui.PrimaryButton(c, "重新起香").Width(110).Clicked() {
			a.startWork()
		}
	})
}
func pathFor(points []point, from, to float32, r ui.Rect) ui.Path {
	var p ui.Path
	if from >= to {
		return p
	}
	prefix := make([]float32, len(points))
	for i := 1; i < len(points); i++ {
		prefix[i] = prefix[i-1] + float32(math.Hypot(float64(points[i].x-points[i-1].x), float64(points[i].y-points[i-1].y)))
	}
	total := prefix[len(points)-1]
	start, end := from*total, to*total
	started := false
	for i := 0; i < len(points)-1; i++ {
		segStart, segEnd := prefix[i], prefix[i+1]
		if segEnd <= start || segStart >= end {
			continue
		}
		lo, hi := float32(0), float32(1)
		if start > segStart {
			lo = (start - segStart) / (segEnd - segStart)
		}
		if end < segEnd {
			hi = (end - segStart) / (segEnd - segStart)
		}
		x1, y1 := points[i].x+(points[i+1].x-points[i].x)*lo, points[i].y+(points[i+1].y-points[i].y)*lo
		x2, y2 := points[i].x+(points[i+1].x-points[i].x)*hi, points[i].y+(points[i+1].y-points[i].y)*hi
		if !started {
			p.MoveTo(r.X+x1*r.W, r.Y+y1*r.H)
			started = true
		}
		p.LineTo(r.X+x2*r.W, r.Y+y2*r.H)
	}
	return p
}

func pointAt(points []point, t float32) point {
	if t <= 0 {
		return points[0]
	}
	if t >= 1 {
		return points[len(points)-1]
	}
	prefix := make([]float32, len(points))
	for i := 1; i < len(points); i++ {
		prefix[i] = prefix[i-1] + float32(math.Hypot(float64(points[i].x-points[i-1].x), float64(points[i].y-points[i-1].y)))
	}
	d := t * prefix[len(points)-1]
	for i := 1; i < len(points); i++ {
		if d <= prefix[i] {
			q := (d - prefix[i-1]) / (prefix[i] - prefix[i-1])
			return point{points[i-1].x + (points[i].x-points[i-1].x)*q, points[i-1].y + (points[i].y-points[i-1].y)*q}
		}
	}
	return points[len(points)-1]
}
func (a *app) restView(c *ui.Context) {
	a.mu.Lock()
	total := a.activeRestDuration
	allowSkip := a.settings.AllowSkip
	start := a.restStart
	a.mu.Unlock()
	progress := float32(time.Since(start)) / float32(total)
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	c.AnimationFrame()
	ui.Column(c).Fill().Background(ui.RGB(17, 17, 19)).Padding(72, 0, 40, 0).Children(func() {
		ui.Spacer(c)
		ui.Column(c).Gap(12).Center().Children(func() {
			ui.Text(c, "一篆香消，万事且抛").FontSize(22).TextColor(ui.RGB(232, 230, 225))
			ui.Text(c, "闭目调息，神游案外").FontSize(14).TextColor(ui.RGB(136, 134, 128))
			ui.Box(c).Size(320, 320).Draw(func(p *ui.Painter, r ui.Rect) {
				base := pathFor(incensePath, 0, 1, r)
				p.StrokePath(&base, 5, ui.RGB(34, 34, 37))
				lit := pathFor(incensePath, progress, 1, r)
				p.StrokePath(&lit, 5, ui.RGB(90, 86, 79))
				if progress < 1 {
					q := pointAt(incensePath, progress)
					x := r.X + q.x*r.W
					y := r.Y + q.y*r.H
					// Faint ember tail just behind the moving firehead.
					tailFrom := progress - 0.035
					if tailFrom < 0 {
						tailFrom = 0
					}
					tail := pathFor(incensePath, tailFrom, progress, r)
					p.StrokePath(&tail, 5, ui.RGB(158, 56, 38).Alpha(0.45))
					pulse := float32(1 + 0.04*math.Sin(31*float64(p.Now().UnixNano())/1e9))
					var glow ui.Path
					glow.Circle(x, y, 3.5*pulse)
					p.FillPath(&glow, ui.RGB(230, 90, 66))
					var core ui.Path
					core.Circle(x, y, 1.5*pulse)
					p.FillPath(&core, ui.RGB(255, 194, 122))
				}
				p.After(16 * time.Millisecond)
			})
		})
		ui.Spacer(c)
		if allowSkip {
			ui.Row(c).Center().Children(func() {
				quit := ui.ButtonBase(c)
				quit.Padding(7, 20).Radius(10)
				fg := ui.RGB(136, 134, 128)
				border := ui.RGB(255, 255, 255).Alpha(0.14)
				if quit.Hovered() {
					border, fg = ui.RGB(255, 255, 255).Alpha(0.32), ui.RGB(232, 230, 225)
				}
				quit.Border(1, border)
				quit.Children(func() { ui.Text(c, "拂灰起行   Esc").TextColor(fg).FontSize(13) })
				if quit.Clicked() {
					a.skipRest()
				}
			})
			if c.Shortcut(0, ui.KeyEscape) {
				a.skipRest()
			}
		}
	})
}

// appMenuBar is the app's menu bar while it is active: the app menu, in
// Chinese like the tray menu.
func appMenuBar() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Label: "香篆", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout, Label: "关于香篆"},
			mygo.Separator(),
			{Role: mygo.RoleHide, Label: "隐藏香篆"},
			{Role: mygo.RoleHideOthers, Label: "隐藏其他"},
			{Role: mygo.RoleUnhide, Label: "全部显示"},
			mygo.Separator(),
			{Role: mygo.RoleQuit, Label: "退出香篆"},
		}},
	})
}

// trayMenu is the dropdown of the menu bar icon: start work, take a rest,
// skip the rest, quit.
func trayMenu(a *app) *mygo.Menu {
	a.mu.Lock()
	s := a.settings
	a.mu.Unlock()
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开香篆", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Focus(); a.window.Show() }},
		{Label: fmt.Sprintf("开始工作 %d 分钟", s.WorkMinutes), Click: func(*mygo.MenuItem, *mygo.Window) { a.startWork() }},
		{Label: fmt.Sprintf("即刻休息 %d 秒", s.RestSeconds), Click: func(*mygo.MenuItem, *mygo.Window) { a.beginRest() }},
		{Label: "跳过休息", Disabled: !s.AllowSkip, Click: func(*mygo.MenuItem, *mygo.Window) { a.skipRest() }},
		mygo.Separator(),
		{Role: mygo.RoleQuit, Label: "退出香篆"},
	})
}

func main() {
	// A menu bar app launched from Finder has no stderr: keep startup
	// diagnostics on disk.
	if cache, err := os.UserCacheDir(); err == nil {
		dir := filepath.Join(cache, "香篆")
		if err := os.MkdirAll(dir, 0o700); err == nil {
			if f, err := os.OpenFile(filepath.Join(dir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
				defer f.Close()
				log.SetOutput(f)
			}
		}
	}
	log.Print("香篆启动")
	a := &app{workDuration: defaultWork, restDuration: defaultRest, remaining: defaultWork}
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.WhenReady(func() {
		if dir, err := mygo.App.Path(mygo.PathUserData); err == nil {
			a.settingsPath = filepath.Join(dir, "settings.json")
		}
		a.settings = loadSettings(a.settingsPath)
		a.workDuration = time.Duration(a.settings.WorkMinutes) * time.Minute
		a.restDuration = time.Duration(a.settings.RestSeconds) * time.Second
		a.remaining = a.workDuration
		// macOS shows this menu bar beside the app's windows; Windows would
		// put it inside the settings window, where its macOS roles (关于香篆、
		// 隐藏其他、全部显示) do nothing. 退出香篆 is in the tray menu there.
		if menuBarApp {
			mygo.App.SetMenu(appMenuBar())
		}
		// A quit closes every window; this window vetoes closes to hide
		// instead, which would cancel the quit (退出香篆 did nothing).
		// OnBeforeQuit runs before the windows are closed: drop the veto.
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting.Store(true) })
		a.window = mygo.NewWindow(mygo.WindowOptions{Title: "香篆", Width: 420, Height: 340, MinWidth: 420, MinHeight: 340, Hidden: true, StateKey: "main.v2", Content: ui.View(a.workView)})
		a.window.OnClose(a.windowClose)
		if !menuBarApp && !settingsExist(a.settingsPath) {
			// Nothing shows that the app is running on Windows: the window
			// is hidden and the tray icon is small. Open the window on the
			// first launch, and record it in the settings file so that the
			// launches after it come up quiet.
			a.window.Show()
			if err := saveSettings(a.settingsPath, a.settings); err != nil {
				log.Printf("保存设置失败: %v", err)
			}
		}
		// The rest screen is an overlay above the other windows, not a
		// window of its own in the taskbar: SkipTaskbar leaves it out of
		// the Windows taskbar, and macOS ignores it (overlay_darwin.go).
		a.restWindow = mygo.NewWindow(mygo.WindowOptions{Title: "香篆 · 休息", Frameless: true, AlwaysOnTop: true, DisableShadow: true, SkipTaskbar: true, Hidden: true, Content: ui.View(a.restView)})
		configureOverlay(a.restWindow.NativeHandle())
		menu := trayMenu(a)
		var err error
		icon, template := trayIcon()
		a.tray, err = mygo.NewTray(mygo.TrayOptions{Icon: icon, IconIsTemplate: template, Title: trayTitle(working, a.workDuration), ToolTip: trayTagline, Menu: menu})
		if err != nil {
			log.Printf("tray creation failed: %v", err)
			log.Fatal(err)
		}
		log.Printf("tray created (%d icon bytes)", len(icon))
		updateTrayTooltip(a.tray, working, a.workDuration)
		go func() {
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for range t.C {
				a.tick()
			}
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
