//go:build windows

package platform

import (
	"fmt"
	"net/url"
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
)

var (
	shell32       = syscall.NewLazyDLL("shell32.dll")
	procShellExec = shell32.NewProc("ShellExecuteW")
)

const swShowNormal = 1

// OpenURI opens any URI scheme (mailto:, tel:, https:, etc.) using the native
// Windows ShellExecuteW API, falling back to Fyne's OpenURL if needed.
func OpenURI(rawURI string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := syscall.UTF16PtrFromString(rawURI)
	if err != nil {
		return err
	}

	r1, _, callErr := procShellExec.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		0,
		0,
		swShowNormal,
	)
	if r1 > 32 {
		return nil
	}

	if app := fyne.CurrentApp(); app != nil {
		if parsed, parseErr := url.Parse(rawURI); parseErr == nil {
			if openErr := app.OpenURL(parsed); openErr == nil {
				return nil
			}
		}
	}

	if callErr != nil && callErr != syscall.Errno(0) {
		return fmt.Errorf("не удалось открыть ссылку %s (код %d): %w", rawURI, r1, callErr)
	}
	return fmt.Errorf("в системе не настроено приложение по умолчанию для открытия %s (код %d)", rawURI, r1)
}
