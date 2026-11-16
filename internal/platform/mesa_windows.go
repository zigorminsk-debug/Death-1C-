//go:build windows

package platform

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

//go:embed opengl32.dll
var mesaDLL []byte

func needsMesa() bool {
	// 1. Check if forced via env
	if os.Getenv("DEATH1C_MESA") == "1" {
		return true
	}

	// 2. Check if running on Windows Server
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err == nil {
		val, _, err := k.GetStringValue("InstallationType")
		k.Close()
		if err == nil && strings.Contains(strings.ToLower(val), "server") {
			return true
		}
	}

	// 3. Check if running inside an RDP session
	user32 := syscall.NewLazyDLL("user32.dll")
	getSystemMetrics := user32.NewProc("GetSystemMetrics")
	// SM_REMOTESESSION = 0x1000
	ret, _, _ := getSystemMetrics.Call(0x1000)
	if ret != 0 {
		return true
	}

	return false
}

// InitMesaIfRequired detects if the app is running on Windows Server or RDP,
// and extracts the bundled Mesa3D software renderer (opengl32.dll) to ensure Fyne launches.
func InitMesaIfRequired() {
	if len(mesaDLL) == 0 {
		return // Not bundled (e.g. local dev build)
	}
	if !needsMesa() {
		return
	}

	appData, err := os.UserConfigDir()
	if err != nil {
		appData = os.TempDir()
	}
	binDir := filepath.Join(appData, "Death1C", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return
	}

	dllPath := filepath.Join(binDir, "opengl32.dll")
	// Overwrite to ensure it's up to date
	err = os.WriteFile(dllPath, mesaDLL, 0644)
	if err != nil {
		return
	}

	// SetDllDirectoryW will inject binDir to the front of the DLL search path
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setDllDir := kernel32.NewProc("SetDllDirectoryW")
	p, _ := syscall.UTF16PtrFromString(binDir)
	setDllDir.Call(uintptr(unsafe.Pointer(p)))
}
