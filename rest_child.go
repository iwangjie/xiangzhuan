package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/egoist/mygo"
)

// What a rest child reports on its stdout: how the rest ended.
const (
	outcomeDone     = "done"
	outcomeSkip     = "skip"
	outcomePostpone = "postpone"
)

// runRestChild draws the rest screens for the tray and prints how the rest
// ended. It is this same binary run again: the tray must not draw, because a
// window costs the engine a surface and wakes the app for it, and the
// framework pages that drawing one touches stay with the process for good.
//
// The child owns the screens, not the countdown: it reports and exits, and
// the tray moves its own state.
func runRestChild(args []string) {
	flags, err := parseRestChildArgs(args)
	if err != nil {
		os.Exit(2)
	}

	a := &app{
		settings:     settings{AllowSkip: flags.allowSkip, PostponeMinutes: flags.postpone},
		workDuration: defaultWork,
		restDuration: time.Duration(flags.seconds) * time.Second,
		phase:        resting,
		showsWindows: true,
	}
	outcome := make(chan string, 1)
	a.reportOutcome = func(o string) {
		select {
		case outcome <- o:
		default:
		}
	}

	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	var release func()
	mygo.App.WhenReady(func() {
		a.beginRest()
		// A rest is watched by someone who is not touching anything, and
		// macOS naps a process that looks idle: without an activity the
		// repaint timer slows to a crawl and the ember stops moving.
		release = mygo.Power.KeepAwake("rest screens", true)
	})
	// The screens ask for a frame themselves instead of relying on the
	// painter's own chain: an input event (a mouse moving over the screens)
	// rebuilds the host's frame and drops that chain, which froze the ember
	// until the next event. A frame every 33ms cannot be dropped.
	go func() {
		for range time.Tick(33 * time.Millisecond) {
			for _, w := range a.overlayWindows() {
				w.Invalidate()
			}
		}
	}()
	go func() {
		o := outcomeDone
		select {
		case o = <-outcome:
		case <-time.After(a.restDuration):
		}
		// The screens come down before the tray hears about it, so the rest
		// is never half-applied.
		a.hideRest()
		time.Sleep(300 * time.Millisecond)
		fmt.Println(o)
		os.Stdout.Sync()
		mygo.App.Quit()
	}()
	// The screens belong to the tray: when it goes, so do they. The tray
	// holds the other end of our stdin, so the pipe closing is the signal,
	// which works the same on every platform (a parent's pid does not).
	if os.Getenv(restChildEnv) != "" {
		go watchTheTray(os.Stdin, mygo.App.Quit)
	}
	if err := mygo.App.Run(); err != nil {
		log.Printf("rest screens: %v", err)
		os.Exit(1)
	}
	if release != nil {
		release()
	}
}

// restChildFlags are what the tray tells the process that draws the rest
// screens.
type restChildFlags struct {
	seconds   int
	postpone  int
	allowSkip bool
}

// parseRestChildArgs reads the flags of a rest child.
func parseRestChildArgs(args []string) (restChildFlags, error) {
	fs := flag.NewFlagSet("--rest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	seconds := fs.Int("seconds", int(defaultRest.Seconds()), "how long the rest lasts")
	postpone := fs.Int("postpone", defaultSettings().PostponeMinutes, "minutes the postpone button asks for")
	allowSkip := fs.Bool("skip", true, "whether Esc may skip the rest")
	if err := fs.Parse(args); err != nil {
		return restChildFlags{}, err
	}
	if *seconds <= 0 || *postpone <= 0 {
		return restChildFlags{}, fmt.Errorf("rest child: seconds and postpone must be positive")
	}
	return restChildFlags{seconds: *seconds, postpone: *postpone, allowSkip: *allowSkip}, nil
}

// watchTheTray calls leave when the tray closes its end of the pipe. It is
// how a rest child dies with the process that put it on screen.
func watchTheTray(tray io.Reader, leave func()) {
	_, _ = io.Copy(io.Discard, tray)
	leave()
}
