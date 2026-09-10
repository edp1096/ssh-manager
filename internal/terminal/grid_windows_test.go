package terminal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestLiveWindowsGrid(t *testing.T) {
	wt := os.Getenv("SSH_MANAGER_WT_LIVE")
	if wt == "" {
		t.Skip("set SSH_MANAGER_WT_LIVE to wt.exe for live desktop tests")
	}
	tmp := t.TempDir()
	helper := filepath.Join(tmp, "grid probe.exe")
	if output, err := exec.Command("go", "build", "-o", helper, "./testdata/wtprobe").CombinedOutput(); err != nil {
		t.Fatalf("probe build: %s: %v", output, err)
	}
	for _, tc := range []struct {
		count, columns int
		fill           string
	}{{3, 2, "horizontal"}, {3, 2, "vertical"}, {3, 2, "vertical-left"}, {6, 3, "horizontal"}, {6, 3, "vertical"}} {
		t.Run(fmt.Sprintf("%d-%s", tc.count, tc.fill), func(t *testing.T) {
			dir := t.TempDir()
			var window uintptr
			t.Cleanup(func() {
				// Default to staggered exits. Other orders exercise WT's
				// animation-related orphaned-window regression explicitly.
				for i := tc.count; i >= 1; i-- {
					id := i
					if os.Getenv("SSH_MANAGER_WT_CLOSE_ORDER") == "forward" {
						id = tc.count - i + 1
					}
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("close-%d", id)), nil, 0600); err != nil {
						t.Error(err)
					}
					if os.Getenv("SSH_MANAGER_WT_CLOSE_ORDER") != "simultaneous" {
						time.Sleep(time.Second)
					}
				}
				if window != 0 {
					isWindow := syscall.NewLazyDLL("user32.dll").NewProc("IsWindow")
					deadline := time.Now().Add(time.Duration(tc.count+5) * time.Second)
					for time.Now().Before(deadline) {
						if exists, _, _ := isWindow.Call(window); exists == 0 {
							return
						}
						time.Sleep(50 * time.Millisecond)
					}
					t.Errorf("test window HWND=%d remains after all probes exited (close order %q)", window, os.Getenv("SSH_MANAGER_WT_CLOSE_ORDER"))
				}
			})
			args := make([]SshClientArgument, tc.count)
			for i := range args {
				args[i] = SshClientArgument{HostsFile: dir, HostIndex: i + 1, CategoryIndex: 1, HostFileKEY: make([]byte, 32)}
			}
			plan, err := PlanGrid(args, tc.columns, tc.fill)
			if err != nil {
				t.Fatal(err)
			}
			opened, err := launchWindowsBatch(wt, helper, plan)
			if err != nil || opened != len(plan) {
				t.Fatalf("opened %d: %v", opened, err)
			}
			window = findProbeWindow(helper)
			if window == 0 {
				t.Fatal("cannot identify the live probe window")
			}
			if err := os.WriteFile(filepath.Join(dir, "sample"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			time.Sleep(3 * time.Second)
			dimensions := make([]struct{ Width, Height int }, tc.count)
			for i := range dimensions {
				data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%d.json", i+1)))
				if err != nil {
					t.Fatalf("pane %d did not start: %v", i+1, err)
				}
				if err = json.Unmarshal(data, &dimensions[i]); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("host dimensions: %+v", dimensions)
			if tc.count == 6 {
				for _, d := range dimensions {
					if abs(d.Width-dimensions[0].Width) > 2 || abs(d.Height-dimensions[0].Height) > 2 {
						t.Fatalf("unequal grid: %+v", dimensions)
					}
				}
			} else if tc.fill == "vertical-left" {
				if dimensions[0].Height < dimensions[1].Height*3/2 || dimensions[1].Width != dimensions[2].Width {
					t.Fatalf("wrong left-expanded layout: %+v", dimensions)
				}
			} else if tc.fill == "vertical" {
				if dimensions[1].Height < dimensions[0].Height*3/2 || dimensions[0].Width != dimensions[2].Width {
					t.Fatalf("wrong vertical layout: %+v", dimensions)
				}
			} else if dimensions[2].Width < dimensions[0].Width*3/2 || dimensions[0].Height != dimensions[1].Height {
				t.Fatalf("wrong horizontal layout: %+v", dimensions)
			}
		})
	}
}

func findProbeWindow(helper string) uintptr {
	user32 := syscall.NewLazyDLL("user32.dll")
	var found uintptr
	callback := syscall.NewCallback(func(window, _ uintptr) uintptr {
		var title [1024]uint16
		user32.NewProc("GetWindowTextW").Call(window, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		if strings.Contains(syscall.UTF16ToString(title[:]), helper) {
			found = window
			return 0
		}
		return 1
	})
	user32.NewProc("EnumWindows").Call(callback, 0)
	return found
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
