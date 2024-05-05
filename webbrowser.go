package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	_ "modernc.org/sqlite"
)

func editLogin(url string) {
	dbPath := "./browser_data/Default/Login Data"

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("UPDATE logins SET origin_url = ?, signon_realm = ? WHERE id = (SELECT id FROM logins LIMIT 1)", url, url, 3)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("UPDATE stats SET origin_domain = ? WHERE update_time = (SELECT update_time FROM stats LIMIT 1)", url)
	if err != nil {
		log.Fatal(err)
	}
}

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
		extractPath := "browser_data"

		// Use chrome if exist
		args[0] = "C:/Program Files/Google/Chrome/Application/chrome.exe"
		if _, err := os.Stat(args[0]); !os.IsNotExist(err) {
			// Use Chrome
			embedTgzFileName := "embeds/chrome_browser_data.tar.gz"
			embedTgzData, err := chromeBrowserData.ReadFile(embedTgzFileName)
			if err != nil {
				panic(fmt.Errorf("failed to extract embedded tar.gz file: %s", err))
			}

			if err := untar(embedTgzData, extractPath); err != nil {
				panic(fmt.Errorf("failed to extract embedded tar.gz file: %s", err))
			}

			editLogin(url + "/")
		} else {
			// Use Edge
			args[0] = "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe"

			embedZipFileName := "embeds/edge_browser_data.zip"
			embedZipData, err := edgeBrowserData.ReadFile(embedZipFileName)
			if err != nil {
				panic(fmt.Errorf("failed to read embedded zip file: %s", err))
			}

			if err := unzip(embedZipData, extractPath); err != nil {
				panic(fmt.Errorf("failed to unzip embedded zip file: %s", err))
			}

			editLogin(url + "/")
		}
	default:
		extractPath := "browser_data"

		// args = []string{"xdg-open"}
		args[0] = "/usr/bin/chromium-browser"

		embedTgzFileName := "embeds/chrome_browser_data.tar.gz"
		embedTgzData, err := chromeBrowserData.ReadFile(embedTgzFileName)
		if err != nil {
			panic(fmt.Errorf("failed to extract embedded tar.gz file: %s", err))
		}

		if err := untar(embedTgzData, extractPath); err != nil {
			panic(fmt.Errorf("failed to extract embedded tar.gz file: %s", err))
		}

		editLogin(url + "/")
	}

	cmdBrowser = exec.Command(args[0], append(args[1:], url)...)
	return cmdBrowser.Start() == nil
}
