package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

// emberTail is how far the ember's glow trails the firehead: 1% of the 1760pt
// path, about 18pt, roughly two fireheads of glow.
const emberTail = 0.01

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
	updateBusy        atomic.Bool
	updateDownloading atomic.Bool
	quitting          atomic.Bool
	// restCmd is the child that draws the rest screens, when one is up, and
	// trayPipe is the end of its stdin that keeps it alive: the child leaves
	// when this end closes, which happens when this process goes.
	restCmd  *exec.Cmd
	trayPipe *os.File
	// reportOutcome is set by the rest child: reporting how the rest ended
	// replaces moving the countdown, which only the tray owns.
	reportOutcome func(outcome string)
	// showsWindows says this process puts the rest screens on a display.
	// run() sets it: the tests build an app that never shows anything, and
	// a rest must not depend on the settings window, which is only built
	// when it is opened.
	showsWindows  bool
	window        *mygo.Window
	restWindows   map[int64]*mygo.Window
	tray          *mygo.Tray
	lastTrayTitle string
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
	// The tray runs the process that draws the screens; that process draws
	// them itself. (A rest child must never run another: it would fork
	// forever.)
	if a.showsWindows && a.reportOutcome == nil && spawnRestChild(a) {
		return
	}
	var windows []*mygo.Window
	if a.showsWindows {
		windows = restScreens(a)
	}
	for _, w := range windows {
		w.SetOpacity(0)
		w.Show()
	}
	if len(windows) > 0 {
		mygo.App.Focus()
		windows[0].Focus()
		go a.fadeRest(windows, 1, 10, seq)
	}
	a.invalidate()
}
func (a *app) skipRest() {
	if !a.canSkip() {
		return
	}
	if a.reportOutcome != nil {
		a.reportOutcome(outcomeSkip)
		return
	}
	a.startWork()
}

// windowClose hides the work window instead of closing it — except during a
// quit. A quit closes every window first, and a veto there would cancel the
// whole quit (this once made 退出香篆 do nothing).
// windowClose drops the settings window as it closes: it is built again the
// next time it is opened, where mygo remembers it was, and 香篆 keeps running
// in the tray (see OnWindowAllClosed).
func (a *app) windowClose(*mygo.CloseEvent) {
	a.mu.Lock()
	a.window = nil
	a.mu.Unlock()
}

// restScreens builds the rest screens. It is a variable so a test can watch
// that a rest asks for them: the settings window is built only when it is
// opened, and a rest that waited for it did nothing at all.
var restScreens = func(a *app) []*mygo.Window {
	return a.syncRestWindows(mygo.Screen.Displays())
}

// restChildEnv marks the process the tray runs to draw the rest screens.
const restChildEnv = "XIANGZHUAN_REST_CHILD"

// spawnRestChild starts the process that draws the rest screens, and reports
// false when it could not be started: the app then draws them itself, because
// a rest that shows nothing is worse than one that costs memory. It is a
// variable so a test never starts a process of its own.
var spawnRestChild = func(a *app) bool { return a.startRestChild() }

func (a *app) startRestChild() bool {
	// Belt and braces against a child that runs children.
	if os.Getenv(restChildEnv) != "" {
		log.Print("rest screens: refusing to run another child")
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		log.Printf("rest screens: %v", err)
		return false
	}
	a.mu.Lock()
	settings := a.settings
	seconds := int(a.restDuration.Seconds())
	a.mu.Unlock()
	cmd := exec.Command(exe, "--rest", "--seconds", strconv.Itoa(seconds),
		"--postpone", strconv.Itoa(settings.PostponeMinutes),
		"--skip", strconv.FormatBool(settings.AllowSkip))
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), restChildEnv+"=1")
	cmd.SysProcAttr = childSysProcAttr()
	out, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("rest screens: %v", err)
		return false
	}
	// The child watches this pipe: closing it (which happens when this
	// process goes) takes the screens down with us.
	tray, keepTray, err := os.Pipe()
	if err != nil {
		log.Printf("rest screens: %v", err)
		return false
	}
	cmd.Stdin = tray
	if err := cmd.Start(); err != nil {
		log.Printf("rest screens: %v", err)
		tray.Close()
		keepTray.Close()
		return false
	}
	tray.Close()
	a.mu.Lock()
	a.trayPipe = keepTray
	a.mu.Unlock()
	a.mu.Lock()
	a.restCmd = cmd
	a.mu.Unlock()
	go func() {
		outcome := ""
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			if line := strings.TrimSpace(sc.Text()); line != "" {
				outcome = line
			}
		}
		_ = cmd.Wait()
		a.mu.Lock()
		a.restCmd = nil
		pipe := a.trayPipe
		a.trayPipe = nil
		a.mu.Unlock()
		if pipe != nil {
			_ = pipe.Close()
		}
		a.applyRestOutcome(outcome)
	}()
	return true
}

// stopRestChild takes the rest screens down with the process drawing them.
func (a *app) stopRestChild() {
	a.mu.Lock()
	cmd := a.restCmd
	a.restCmd = nil
	pipe := a.trayPipe
	a.trayPipe = nil
	a.mu.Unlock()
	if pipe != nil {
		// The child watches this end: closing it is what tells a child that
		// outlived its welcome to leave.
		_ = pipe.Close()
	}
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := cmd.Process.Kill(); err != nil {
		log.Printf("rest screens: %v", err)
	}
}

// applyRestOutcome moves the countdown for a rest that ended in the child:
// the child draws the screens, the tray owns the state.
func (a *app) applyRestOutcome(outcome string) {
	a.mu.Lock()
	resting := a.phase == resting
	a.mu.Unlock()
	if !resting {
		return
	}
	switch outcome {
	case outcomeSkip:
		a.skipRest()
	case outcomePostpone:
		a.postponeRest(time.Now())
	default:
		a.startWork()
	}
}

// settingsWindow returns the settings window, building it the first time it
// is asked for. A window the user has not opened is not worth its surface and
// the timer the engine keeps for it, which wake a resident app several times
// a second while it only counts down in the tray.
func (a *app) settingsWindow() *mygo.Window {
	a.mu.Lock()
	w := a.window
	a.mu.Unlock()
	if w != nil {
		return w
	}
	w = mygo.NewWindow(mygo.WindowOptions{Title: "香篆", Width: 420, Height: 410, MinWidth: 420, MinHeight: 410, Hidden: true, StateKey: "main.v4", Content: ui.View(a.workView)})
	w.OnClose(a.windowClose)
	a.mu.Lock()
	a.window = w
	a.mu.Unlock()
	return w
}
func (a *app) hideRest() {
	a.stopRestChild()
	windows := a.overlayWindows()
	if len(windows) == 0 {
		return
	}
	a.mu.Lock()
	a.fadeSeq++
	seq := a.fadeSeq
	a.mu.Unlock()
	go func() {
		a.fadeRest(windows, 0, 15, seq)
		a.mu.Lock()
		stale := a.fadeSeq != seq
		a.mu.Unlock()
		if stale {
			return
		}
		for _, w := range windows {
			w.Hide()
			w.SetOpacity(1)
		}
		a.releaseRestWindows(seq)
	}()
}

// fadeRest eases the rest window to the given opacity in steps of 16ms,
// giving up once a newer fade has taken over.
func (a *app) fadeRest(windows []*mygo.Window, to float64, steps, seq int) {
	from := make([]float64, len(windows))
	for i, w := range windows {
		from[i] = w.Opacity()
	}
	for i := 1; i <= steps; i++ {
		a.mu.Lock()
		stale := a.fadeSeq != seq
		a.mu.Unlock()
		if stale {
			return
		}
		for j, w := range windows {
			w.SetOpacity(from[j] + (to-from[j])*float64(i)/float64(steps))
		}
		time.Sleep(16 * time.Millisecond)
	}
}

// releaseRestWindows closes the overlays once they are hidden. Each holds a
// display-sized surface (a display's worth of pixels), and a resident app has
// no reason to keep that while it works; the next rest creates them again,
// still at opacity 0, so the fade hides the creation.
func (a *app) releaseRestWindows(seq int) {
	a.mu.Lock()
	if a.fadeSeq != seq {
		a.mu.Unlock()
		return
	}
	windows := make([]*mygo.Window, 0, len(a.restWindows))
	for _, w := range a.restWindows {
		windows = append(windows, w)
	}
	a.restWindows = nil
	a.mu.Unlock()
	for _, w := range windows {
		w.Close()
	}
}

func (a *app) invalidate() {
	// A hidden window has nothing to redraw, and asking one for a frame wakes
	// the app and its GPU for nothing: while the app works in the background
	// the countdown it would redraw is not on screen.
	if a.window != nil && a.window.IsVisible() {
		a.window.Invalidate()
	}
	for _, w := range a.overlayWindows() {
		if w.IsVisible() {
			w.Invalidate()
		}
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
		// setTitle: relayouts the status item and repaints the menu bar, so
		// only ask when the text changes: once a minute while working.
		if title := trayTitle(ph, left); title != a.lastTrayTitle {
			a.lastTrayTitle = title
			a.tray.SetTitle(title)
		}
		if !a.updateDownloading.Load() {
			updateTrayTooltip(a.tray, ph, left)
		}
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
	workMin, restSec, postponeMin, allowSkip := float64(s.WorkMinutes), float64(s.RestSeconds), float64(s.PostponeMinutes), s.AllowSkip
	ui.Column(c).Fill().Center().Gap(16).Padding(24, 28).Children(func() {
		ui.Column(c).Center().Gap(4).Children(func() {
			ui.Text(c, "香篆").FontSize(24).Bold()
			ui.Text(c, "焚香理绪，神定案明。").FontSize(13).TextColor(c.Theme().TextMuted)
			ui.Textf(c, "%02d:%02d", int(left/time.Minute), int(left/time.Second)%60).FontSize(38).Bold()
		})
		ui.Column(c).Width(280).Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(8).Padding(14, 16).Gap(12).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, "工作时长").FontSize(13)
				ui.Spacer(c)
				ui.NumberInput(c, &workMin, 30, 50, 1)
				ui.Text(c, "分钟").FontSize(12).TextColor(c.Theme().TextMuted).Width(28)
			})
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, "休息时长").FontSize(13)
				ui.Spacer(c)
				ui.NumberInput(c, &restSec, 60, 120, 5)
				ui.Text(c, "秒").FontSize(12).TextColor(c.Theme().TextMuted).Width(28)
			})
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, "延后时长").FontSize(13)
				ui.Spacer(c)
				ui.NumberInput(c, &postponeMin, 1, 30, 1)
				ui.Text(c, "分钟").FontSize(12).TextColor(c.Theme().TextMuted).Width(28)
			})
			ui.Checkbox(c, &allowSkip, "允许跳过休息").FontSize(13)
		})
		updated := settings{int(workMin), int(restSec), allowSkip, int(postponeMin)}
		if updated != s {
			a.applySettings(updated)
		}
		if ui.PrimaryButton(c, "重新起香").Width(130).Clicked() {
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
	allowSkip := a.settings.AllowSkip
	postponeMin := a.settings.PostponeMinutes
	a.mu.Unlock()
	ui.Column(c).Fill().Background(ui.RGB(17, 17, 19)).Padding(72, 0, 40, 0).Children(func() {
		ui.Spacer(c)
		ui.Column(c).Gap(12).Center().Children(func() {
			ui.Text(c, "一篆香消，万事且抛").FontSize(22).TextColor(ui.RGB(232, 230, 225))
			ui.Text(c, "闭目调息，神游案外").FontSize(14).TextColor(ui.RGB(136, 134, 128))
			ui.Box(c).Size(320, 320).Draw(func(p *ui.Painter, r ui.Rect) {
				a.mu.Lock()
				progress := a.restProgressLocked(p.Now())
				a.mu.Unlock()
				base := pathFor(incensePath, 0, 1, r)
				p.StrokePath(&base, 5, ui.RGB(34, 34, 37))
				lit := pathFor(incensePath, progress, 1, r)
				p.StrokePath(&lit, 5, ui.RGB(90, 86, 79))
				if progress < 1 {
					q := pointAt(incensePath, progress)
					x := r.X + q.x*r.W
					y := r.Y + q.y*r.H
					// Faint ember tail just behind the moving firehead.
					tailFrom := progress - emberTail
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
				// 30fps: the ember moves about a pixel a frame at 2x, and
				// each repaint presents the whole screen.
				p.After(33 * time.Millisecond)
			})
		})
		ui.Spacer(c)
		ui.Row(c).Center().Children(func() {
			if postponeButton(c, fmt.Sprintf("延后 %d 分钟", postponeMin)) {
				a.postponeRest(time.Now())
			}
		})
		if allowSkip {
			// A hint, not a button: 延后 owns the button spot now, and
			// skipping is Esc or the tray menu.
			ui.Row(c).Center().Gap(8).Margin(14, 0, 0, 0).Children(func() {
				ui.Text(c, "拂灰起行").FontSize(12).TextColor(ui.RGB(110, 108, 104))
				keyCap(c, "Esc")
			})
			if c.Shortcut(0, ui.KeyEscape) {
				a.skipRest()
			}
		}
	})
}

// postponeButton draws the rest screen's action and reports a click. The
// theme's accent blue fights the seal's warm dark palette, so this is a
// heated bronze plate instead: an ember panel, a copper edge and warm ink,
// which brightens under the pointer and dims when pressed.
func postponeButton(c *ui.Context, label string) bool {
	b := ui.ButtonBase(c)
	b.Padding(11, 28).Radius(9)
	top, bottom, edge, ink := ui.RGB(40, 26, 22), ui.RGB(26, 17, 15), ui.RGB(86, 48, 36), ui.RGB(240, 198, 164)
	switch {
	case b.Pressed():
		top, bottom, ink = ui.RGB(22, 15, 13), ui.RGB(20, 14, 12), ui.RGB(226, 184, 152)
	case b.Hovered():
		top, bottom, edge, ink = ui.RGB(58, 37, 28), ui.RGB(38, 24, 20), ui.RGB(124, 66, 46), ui.RGB(246, 210, 178)
	}
	b.LinearGradient(ui.LinearGradient{From: top, To: bottom, Angle: 90}).Border(1, edge)
	b.Children(func() {
		ui.Text(c, label).FontSize(14).TextColor(ink).SingleLine()
	})
	return b.Clicked()
}

// keyCap draws a shortcut the way a keyboard draws it, so the hint under the
// button reads as a key to press rather than a link to click.
func keyCap(c *ui.Context, key string) {
	cap := ui.Row(c).Center().Padding(3, 7).Radius(5)
	cap.Background(ui.RGB(28, 28, 31)).Border(1, ui.RGB(51, 49, 45))
	cap.Children(func() {
		ui.Text(c, key).FontSize(11).TextColor(ui.RGB(157, 154, 147)).SingleLine()
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
		{Label: "打开香篆", Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Focus(); a.settingsWindow().Show() }},
		{Label: fmt.Sprintf("开始工作 %d 分钟", s.WorkMinutes), Click: func(*mygo.MenuItem, *mygo.Window) { a.startWork() }},
		{Label: fmt.Sprintf("即刻休息 %d 秒", s.RestSeconds), Click: func(*mygo.MenuItem, *mygo.Window) { a.beginRest() }},
		{Label: "跳过休息", Disabled: !s.AllowSkip, Click: func(*mygo.MenuItem, *mygo.Window) { a.skipRest() }},
		{Label: "检查更新…", Click: func(*mygo.MenuItem, *mygo.Window) { go a.checkUpdates(true) }},
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
	if len(os.Args) > 1 && os.Args[1] == "--rest" {
		runRestChild(os.Args[2:])
		return
	}
	log.Print("香篆启动")
	a := &app{workDuration: defaultWork, restDuration: defaultRest, remaining: defaultWork}
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	a.showsWindows = true
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
		// Closing the settings window is not quitting the app: 香篆 lives in
		// the tray, so it stays up with no windows at all.
		mygo.App.OnWindowAllClosed(func() {})
		// A quit closes every window, and the countdown must not be counted
		// as a user close while that runs.
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
			a.quitting.Store(true)
			a.stopRestChild()
		})
		if !menuBarApp && !settingsExist(a.settingsPath) {
			// Nothing shows that the app is running on Windows: the tray
			// icon is small. Open the window on the first launch, and record
			// it in the settings file so that the launches after it come up
			// quiet.
			a.settingsWindow().Show()
			if err := saveSettings(a.settingsPath, a.settings); err != nil {
				log.Printf("保存设置失败: %v", err)
			}
		}
		// The rest screen is an overlay above the other windows, not a
		// window of its own in the taskbar: SkipTaskbar leaves it out of
		// the Windows taskbar, and macOS ignores it (overlay_darwin.go).

		menu := trayMenu(a)
		var err error
		icon, template := trayIcon()
		a.tray, err = mygo.NewTray(mygo.TrayOptions{Icon: icon, IconIsTemplate: template, Title: trayTitle(working, a.workDuration), ToolTip: trayTagline, Menu: menu})
		if err != nil {
			log.Printf("tray creation failed: %v", err)
			log.Fatal(err)
		}
		log.Printf("tray created (%d icon bytes)", len(icon))
		go func() {
			time.Sleep(8 * time.Second)
			a.checkUpdates(false)
		}()
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
