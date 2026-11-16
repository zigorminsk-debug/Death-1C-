package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

// AppIcon returns the embedded application icon resource for Fyne windows and dialogs.
func AppIcon() fyne.Resource {
	return fyne.NewStaticResource("icon.png", iconPNG)
}
