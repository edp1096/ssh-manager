package browser

import (
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func focusBrowserWindow(pid int) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	enumWindows := user32.NewProc("EnumWindows")
	getPID := user32.NewProc("GetWindowThreadProcessId")
	isVisible := user32.NewProc("IsWindowVisible")
	setForeground := user32.NewProc("SetForegroundWindow")
	var window uintptr
	callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var windowPID uint32
		getPID.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
		if windowPID == uint32(pid) {
			visible, _, _ := isVisible.Call(hwnd)
			if visible != 0 {
				window = hwnd
				return 0
			}
		}
		return 1
	})
	// Wait only at startup and activate only a window owned by this browser process.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		enumWindows.Call(callback, 0)
		if window != 0 {
			setForeground.Call(window)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
