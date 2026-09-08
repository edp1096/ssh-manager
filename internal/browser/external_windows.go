package browser

import (
	"runtime"
	"syscall"

	"golang.org/x/sys/windows"
)

func openExternalURL(url string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Shell URL handlers may require an STA COM apartment. S_FALSE is also success.
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err != nil && err != syscall.Errno(1) {
		return err
	}
	defer windows.CoUninitialize()
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL)
}
