package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"death1c/assets"
	"death1c/internal/platform"
	"death1c/internal/ui"
	"death1c/internal/version"
)

func main() {
	platform.InitMesaIfRequired()

	fyneApp := app.NewWithID(version.AppID)
	fyneApp.SetIcon(assets.AppIcon())
	fyneApp.Settings().SetTheme(ui.NewAppTheme())

	win := fyneApp.NewWindow(version.WindowTitle())
	win.SetIcon(assets.AppIcon())
	win.Resize(fyne.NewSize(1280, 760))
	win.CenterOnScreen()

	_ = ui.NewController(fyneApp, win)

	win.ShowAndRun()
}
