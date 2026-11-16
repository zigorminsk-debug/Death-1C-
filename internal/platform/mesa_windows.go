//go:build windows

package platform

import (
	_ "embed"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

//go:embed opengl32.dll
var mesaDLL []byte

func needsMesa() bool {
	if os.Getenv("DEATH1C_MESA_LAUNCHED") == "1" {
		return false
	}

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

	currentExe, err := os.Executable()
	if err != nil {
		return
	}

	newExePath := filepath.Join(binDir, filepath.Base(currentExe))
	
	if strings.EqualFold(currentExe, newExePath) {
		os.Setenv("DEATH1C_MESA_LAUNCHED", "1")
		return
	}

	statSrc, errSrc := os.Stat(currentExe)
	statDst, errDst := os.Stat(newExePath)
	needsCopy := errSrc == nil && (errDst != nil || statSrc.Size() != statDst.Size() || statSrc.ModTime().After(statDst.ModTime()))

	if needsCopy {
		os.Remove(newExePath) // Try to remove old one if it exists
		err = copyFile(currentExe, newExePath)
		if err != nil {
			// Fallback: copy to a temporary name
			newExePath = filepath.Join(binDir, "Death-1C-tmp.exe")
			os.Remove(newExePath)
			if copyFile(currentExe, newExePath) != nil {
				return // Failed to copy, fallback to normal launch (which might crash)
			}
		}
	}

	// Relaunch from the new directory so the OS Loader prefers the local opengl32.dll
	cmd := exec.Command(newExePath, os.Args[1:]...)
	cmd.Env = append(os.Environ(), "DEATH1C_MESA_LAUNCHED=1")
	cmd.Dir, _ = os.Getwd()

	if err := cmd.Start(); err == nil {
		os.Exit(0)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
