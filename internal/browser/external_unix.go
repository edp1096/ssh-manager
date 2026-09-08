//go:build linux || freebsd

package browser

import (
	"fmt"
	"os/exec"
)

func openExternalURL(url string) error {
	var cmd *exec.Cmd
	if opener, err := exec.LookPath("xdg-open"); err == nil {
		cmd = exec.Command(opener, url)
	} else if opener, err := exec.LookPath("gio"); err == nil {
		cmd = exec.Command(opener, "open", url)
	} else {
		return fmt.Errorf("xdg-open or gio is required to open the default browser")
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			fmt.Println("Default browser launch failed:", err)
		}
	}()
	return nil
}
