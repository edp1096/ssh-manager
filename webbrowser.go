package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

func openBrowser(url string) bool {
	userAgent := "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36 Edg/124.0.0.0"
	dataPath := filepath.FromSlash(binaryPath + "/browser_data")

	args := []string{
		"browser_command_here",
		"--user-data-dir=" + dataPath,
		"--app=" + url,
		// "--auto-open-devtools-for-tabs ",
		// "--window-position=0,0",
		"--window-size=920,600",
		"--user-agent=" + userAgent,
		"--enable-local-file-accesses",
		"--no-initial-navigation",
		"--no-default-browser-check",
		"--allow-file-access-from-files",
		"--disable-background-mode",
		"--no-experiments",
		"--no-proxy-server",
		// "--ignore-autocomplete-off-autofill",
		"--disable-speech-api",
		"--disable-logging",
		"--disable-translate",
		"--disable-features=Translate",
	}

	switch runtime.GOOS {
	case "darwin":
		args = []string{"open"}
	case "windows":
		args[0] = "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe"
	default:
		// args = []string{"xdg-open"}
		args[0] = "/usr/bin/chromium-browser"
	}

	cmdBrowser = exec.Command(args[0], append(args[1:], url)...)
	return cmdBrowser.Start() == nil
}
