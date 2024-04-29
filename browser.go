package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

func openBrowser(url string) bool {
	userAgent := "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36 Edg/124.0.0.0"
	dataPath := filepath.FromSlash(binaryPath + "/browser_data")

	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"open"}
	case "windows":
		args = []string{
			"C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe",
			"--user-data-dir=" + dataPath,
			"--app=" + url,
			"--window-size=640,720",
			"--user-agent=" + userAgent,
		}
	default:
		// args = []string{"xdg-open"}
		args = []string{
			"/usr/bin/chromium-browser",
			"--user-data-dir=" + dataPath,
			"--app=" + url,
			"--window-size=640,720",
			"--user-agent=" + userAgent,
		}
	}

	cmd := exec.Command(args[0], append(args[1:], url)...)
	return cmd.Start() == nil
}
