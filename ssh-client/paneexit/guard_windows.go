package paneexit

import (
	"crypto/sha256"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

type Guard struct{ handle uintptr }

// New opens the batch mutex while the client is still running so its lifetime
// spans all clients in this window, including clients not yet ready to exit.
func New(group string) (*Guard, error) {
	if group == "" {
		return &Guard{}, nil
	}
	name, err := syscall.UTF16PtrFromString(fmt.Sprintf("Local\\ssh-manager-exit-%x", sha256.Sum256([]byte(group))))
	if err != nil {
		return nil, err
	}
	handle, _, err := kernel32.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return nil, err
	}
	return &Guard{handle: handle}, nil
}

// BeforeProcessExit must be the final operation before the process exits.
// Keep ownership on this OS thread until process termination. Windows then
// abandons the mutex, proving to the next client that this process has ended.
// Do not release it early: that would allow concurrent process termination.
func (g *Guard) BeforeProcessExit() error {
	if g == nil || g.handle == 0 {
		return nil
	}
	runtime.LockOSThread()
	result, _, err := kernel32.NewProc("WaitForSingleObject").Call(g.handle, 35000)
	switch result {
	case 0: // First process in the batch to exit.
		return nil
	case 0x80: // Previous owner exited; allow WT's pane animation to finish.
		time.Sleep(time.Second)
		return nil
	default:
		kernel32.NewProc("CloseHandle").Call(g.handle)
		g.handle = 0
		runtime.UnlockOSThread()
		return fmt.Errorf("pane exit wait failed (status %d): %v", result, err)
	}
}
