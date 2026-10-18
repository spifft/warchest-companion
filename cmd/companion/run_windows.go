//go:build windows

package main

import (
	"github.com/spifft/warchest-companion/internal/app"
	"github.com/spifft/warchest-companion/internal/winui"
)

func run(e *app.Engine, deviceName string, background bool) {
	(&winui.Tray{Engine: e, Version: version, DeviceName: deviceName, Repo: repo, Background: background}).Run()
}
