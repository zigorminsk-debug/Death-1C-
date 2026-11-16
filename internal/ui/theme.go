package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"death1c/internal/onec"
)

type appTheme struct {
	base fyne.Theme
}

// NewAppTheme returns a clean desktop theme tuned for readability in 1C administration workflows.
func NewAppTheme() fyne.Theme {
	return &appTheme{base: theme.DefaultTheme()}
}

func (t *appTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 245, G: 184, B: 20, A: 255}
		}
		return color.NRGBA{R: 185, G: 28, B: 28, A: 255}
	case theme.ColorNameHyperlink:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 96, G: 175, B: 255, A: 255}
		}
		return color.NRGBA{R: 15, G: 98, B: 210, A: 255}
	}
	return t.base.Color(name, variant)
}

func (t *appTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *appTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *appTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 13.5
	}
	return t.base.Size(name)
}

func statusBadgeColors(st onec.SessionStatus, variant fyne.ThemeVariant) (bg color.Color, fg color.Color) {
	dark := variant == theme.VariantDark
	switch st {
	case onec.StatusDuplicateOld:
		if dark {
			return color.NRGBA{R: 145, G: 28, B: 28, A: 220}, color.NRGBA{R: 255, G: 235, B: 235, A: 255}
		}
		return color.NRGBA{R: 254, G: 226, B: 226, A: 255}, color.NRGBA{R: 153, G: 27, B: 27, A: 255}
	case onec.StatusProtected:
		if dark {
			return color.NRGBA{R: 30, G: 64, B: 135, A: 210}, color.NRGBA{R: 219, G: 234, B: 254, A: 255}
		}
		return color.NRGBA{R: 219, G: 234, B: 254, A: 255}, color.NRGBA{R: 30, G: 64, B: 175, A: 255}
	case onec.StatusSystem:
		if dark {
			return color.NRGBA{R: 64, G: 68, B: 76, A: 200}, color.NRGBA{R: 205, G: 210, B: 218, A: 255}
		}
		return color.NRGBA{R: 229, G: 231, B: 235, A: 255}, color.NRGBA{R: 75, G: 85, B: 99, A: 255}
	default: // StatusCurrent
		if dark {
			return color.NRGBA{R: 20, G: 105, B: 58, A: 210}, color.NRGBA{R: 220, G: 252, B: 231, A: 255}
		}
		return color.NRGBA{R: 220, G: 252, B: 231, A: 255}, color.NRGBA{R: 22, G: 101, B: 52, A: 255}
	}
}
