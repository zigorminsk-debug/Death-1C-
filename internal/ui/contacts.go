package ui

import (
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"death1c/assets"
	"death1c/internal/platform"
	"death1c/internal/version"
)

func newContactLink(label, rawURI, copyValue string, app fyne.App, win fyne.Window) *widget.Hyperlink {
	parsed, _ := url.Parse(rawURI)
	hl := widget.NewHyperlink(label, parsed)
	hl.OnTapped = func() {
		if err := platform.OpenURI(rawURI); err != nil {
			if app != nil && copyValue != "" {
				app.Clipboard().SetContent(copyValue)
			}
			dialog.ShowInformation(
				"Контакт скопирован",
				fmt.Sprintf("Контакт %s скопирован в буфер обмена.\n(%v)", copyValue, err),
				win,
			)
		}
	}
	return hl
}

// ShowDeveloperDialog displays detailed build metadata and clickable developer contacts.
func ShowDeveloperDialog(app fyne.App, win fyne.Window) {
	iconImg := canvas.NewImageFromResource(assets.AppIcon())
	iconImg.FillMode = canvas.ImageFillContain
	iconImg.SetMinSize(fyne.NewSize(64, 64))

	titleLbl := widget.NewLabelWithStyle(
		"Death-1C — Удаление старых сеансов 1С",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)
	buildInfo := version.DisplayBuild()
	if version.Commit != "" && version.Commit != "dev" {
		shortSHA := version.Commit
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		buildInfo += fmt.Sprintf("  •  commit %s", shortSHA)
	}
	if version.BuildDate != "" {
		buildInfo += fmt.Sprintf("  •  %s", version.BuildDate)
	}
	buildLbl := widget.NewLabel(buildInfo)

	header := container.NewHBox(
		iconImg,
		container.NewVBox(titleLbl, buildLbl),
	)

	devNameLink := newContactLink(
		version.DeveloperName,
		"mailto:"+version.DeveloperEmail+"?subject=Death-1C",
		version.DeveloperName+" <"+version.DeveloperEmail+">",
		app,
		win,
	)
	emailLink := newContactLink(
		version.DeveloperEmail,
		"mailto:"+version.DeveloperEmail,
		version.DeveloperEmail,
		app,
		win,
	)
	phoneLink := newContactLink(
		version.DeveloperPhone,
		"tel:"+version.DeveloperPhone,
		version.DeveloperPhone,
		app,
		win,
	)

	copyEmailBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		app.Clipboard().SetContent(version.DeveloperEmail)
	})
	copyEmailBtn.Importance = widget.LowImportance

	copyPhoneBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		app.Clipboard().SetContent(version.DeveloperPhone)
	})
	copyPhoneBtn.Importance = widget.LowImportance

	contactGrid := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Разработчик и контакты:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(widget.NewLabel("Разработчик:"), devNameLink),
		container.NewHBox(widget.NewLabel("E-mail:"), emailLink, copyEmailBtn),
		container.NewHBox(widget.NewLabel("Телефон:"), phoneLink, copyPhoneBtn),
	)

	body := container.NewPadded(container.NewVBox(header, contactGrid))
	dialog.ShowCustom("О программе", "Закрыть", body, win)
}

// BuildFooterBar creates the persistent footer bar showing the sequential build number
// and clickable developer contacts (Zakharevich Igor, ziv@csl.by, +375293371412).
func BuildFooterBar(app fyne.App, win fyne.Window) fyne.CanvasObject {
	buildBadge := widget.NewLabelWithStyle(
		version.DisplayBuild(),
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	devLink := widget.NewHyperlink(version.DeveloperName, nil)
	devLink.OnTapped = func() {
		ShowDeveloperDialog(app, win)
	}

	emailLink := newContactLink(
		version.DeveloperEmail,
		"mailto:"+version.DeveloperEmail,
		version.DeveloperEmail,
		app,
		win,
	)

	phoneLink := newContactLink(
		version.DeveloperPhone,
		"tel:"+version.DeveloperPhone,
		version.DeveloperPhone,
		app,
		win,
	)

	row := container.NewHBox(
		buildBadge,
		layout.NewSpacer(),
		widget.NewLabel("Разработчик:"),
		devLink,
		widget.NewLabel("•"),
		emailLink,
		widget.NewLabel("•"),
		phoneLink,
	)

	return container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(row),
	)
}
