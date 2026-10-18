//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spifft/warchest-companion/internal/app"
)

// console mode: prints the status, pairs when needed, relays until Ctrl+C
func run(e *app.Engine, deviceName string, _ bool) {
	e.OnStatus = func(s app.Status) {
		fmt.Printf("%s  status=%s code=%s char=%s room=%s sent=%d  %s\n", time.Now().Format("15:04:05"), s.State, s.Code, s.Char, s.Room, s.Sent, s.Message)
	}
	e.Notify = func(title, msg string) { fmt.Printf("NOTICE %s: %s\n", title, msg) }
	e.Open = func(url string) { fmt.Println("OPEN", url) }
	if os.Getenv("COMPANION_INTERVAL_MS") != "" {
		var ms int
		fmt.Sscanf(os.Getenv("COMPANION_INTERVAL_MS"), "%d", &ms)
		e.Interval = time.Duration(ms) * time.Millisecond
	}
	if e.Cfg.Token() == "" {
		if err := e.Pair(deviceName); err != nil {
			fmt.Println("ERROR", err)
			os.Exit(1)
		}
	}
	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() { <-sig; close(stop) }()
	e.Run(stop)
}
