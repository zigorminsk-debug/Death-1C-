//go:build !windows

package platform

import (
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
)

// OpenURI opens a URI using Fyne's OpenURL implementation on non-Windows platforms.
func OpenURI(rawURI string) error {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return fmt.Errorf("некорректная ссылка %s: %w", rawURI, err)
	}
	if app := fyne.CurrentApp(); app != nil {
		return app.OpenURL(parsed)
	}
	return fmt.Errorf("приложение Fyne не инициализировано")
}
