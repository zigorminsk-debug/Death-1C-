package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procEnumDisplayDevices = user32.NewProc("EnumDisplayDevicesW")
)

type DISPLAY_DEVICE struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

func main() {
	var dd DISPLAY_DEVICE
	dd.Cb = uint32(unsafe.Sizeof(dd))
	ret, _, _ := procEnumDisplayDevices.Call(0, 0, uintptr(unsafe.Pointer(&dd)), 0)
	if ret != 0 {
		fmt.Println(syscall.UTF16ToString(dd.DeviceString[:]))
	}
}
