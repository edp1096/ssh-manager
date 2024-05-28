package browser

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"ssh-manager/pkg/archiver"
	"ssh-manager/pkg/utils"
)

type WebBrowserInfo struct {
	Name string
	Path string
}

func OpenBrowser(url string, BrowserData embed.FS) (*exec.Cmd, bool) {
	var browsers []WebBrowserInfo

	workingDir, _, _ := utils.GetCWD()

	userAgent := "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36 Edg/124.0.0.0"
	dataPath := filepath.FromSlash(workingDir + "/browser_data")

	// url = url + "?system-os=" +

	args := []string{
		"browser_command_here",
		"--user-data-dir=" + dataPath,
		"--app=" + url,
		// "--auto-open-devtools-for-tabs ",
		// "--window-position=0,0",
		"--window-size=720,520",
		"--user-agent=" + userAgent,
		"--password-store=basic",
		"--no-initial-navigation",
		"--no-default-browser-check",
		"--allow-file-access-from-files",
		"--enable-local-file-accesses",

		// "--remote-debugging-port=9222",
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
	case "windows":
		browsers = []WebBrowserInfo{
			{"chromium", os.Getenv("LocalAppData") + "/Chromium/Application/chrome.exe"},
			{"chromium", os.Getenv("ProgramFiles") + "/Chromium/Application/chrome.exe"},
			{"chromium", os.Getenv("ProgramFiles(x86)") + "/Chromium/Application/chrome.exe"},
			{"chrome", os.Getenv("LocalAppData") + "/Google/Chrome/Application/chrome.exe"},
			{"chrome", os.Getenv("ProgramFiles") + "/Google/Chrome/Application/chrome.exe"},
			{"chrome", os.Getenv("ProgramFiles(x86)") + "/Google/Chrome/Application/chrome.exe"},
			{"msedge", os.Getenv("ProgramFiles") + "/Microsoft/Edge/Application/msedge.exe"},
			{"msedge", os.Getenv("ProgramFiles(x86)") + "/Microsoft/Edge/Application/msedge.exe"},
		}
	case "freebsd", "linux":
		browsers = []WebBrowserInfo{
			{"chromium", "/usr/bin/chromium"},
			{"chromium", "/usr/bin/chromium-browser"},
			{"chromium", "/usr/local/share/chromium/chrome"},
			{"chromium", "/snap/bin/chromium"},
			{"chrome", "/usr/bin/google-chrome"},
			{"chrome", "/usr/bin/google-chrome-stable"},
			{"msedge", "/usr/bin/msedge"},
		}
	default:
		panic(fmt.Errorf("os not support"))
	}

	extractPath := "browser_data"

	// Find web browser binary
	foundWebBrowser := ""
	for _, b := range browsers {
		if _, err := os.Stat(b.Path); !os.IsNotExist(err) {
			args[0] = b.Path
			foundWebBrowser = b.Name
			break
		}
	}

	if foundWebBrowser == "" {
		panic(fmt.Errorf("chrome or chromium or msedge not found"))
	}

	var archiveData archiver.Archiver
	switch foundWebBrowser {
	case "msedge":
		archiveData = archiver.Zip{
			FileName:   "embeds/browser_data.zip",
			TargetPath: extractPath,
			FSdata:     BrowserData,
		}
	case "chrome", "chromium":
		archiveData = archiver.Tgz{
			FileName:   "embeds/browser_data.tar.gz",
			TargetPath: extractPath,
			FSdata:     BrowserData,
		}
	default:
		panic("no available browser") // User should not meet this
	}

	archiveData.UnArchive()
	EditBrowserDataLogins(url + "/")

	cmdBrowser := exec.Command(args[0], append(args[1:], url)...)
	return cmdBrowser, cmdBrowser.Start() == nil
}
