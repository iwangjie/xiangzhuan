package main

import (
	"flag"
	"fmt"
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
	fs := flag.NewFlagSet("--rest", flag.ContinueOnError)
	seconds := fs.Int("seconds", int(defaultRest.Seconds()), "how long the rest lasts")
	postpone := fs.Int("postpone", defaultSettings().PostponeMinutes, "minutes the postpone button asks for")
	allowSkip := fs.Bool("skip", true, "whether Esc may skip the rest")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	a := &app{
		settings:     settings{AllowSkip: *allowSkip, PostponeMinutes: *postpone},
		workDuration: defaultWork,
		restDuration: time.Duration(*seconds) * time.Second,
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
	mygo.App.WhenReady(func() { a.beginRest() })
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
	// The screens belong to the tray: when it goes, so do they.
	go func() {
		parent := os.Getppid()
		for {
			time.Sleep(time.Second)
			if os.Getppid() != parent {
				mygo.App.Quit()
				return
			}
		}
	}()
	if err := mygo.App.Run(); err != nil {
		log.Printf("rest screens: %v", err)
		os.Exit(1)
	}
}
