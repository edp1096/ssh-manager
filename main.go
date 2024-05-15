package main // import "my-ssh-manager"

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
)

type HostList struct {
	Categories []HostCategory `json:"host-categories"`
}

type HostCategory struct {
	Name  string     `json:"name"`
	Hosts []HostInfo `json:"hosts"`
}

type HostInfo struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"-"`
	PrivateKeyText string `json:"private-key-text"`
}

type HostRequestInfo struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	PrivateKeyText string `json:"private-key-text"`
}

var shellRuntimePath = os.Getenv("LocalAppData") + "/Microsoft/WindowsApps/wt.exe"
var hostFileKEY []byte = []byte("0123456789!#$%^&*()abcdefghijklm")
var (
	cmdTerminal        *exec.Cmd
	cmdBrowser         *exec.Cmd
	browserWindowTitle string
	server             *http.Server
	binaryPath         string
	availablePort      int
)

//go:embed html/*
var embedFiles embed.FS

//go:embed embeds/browser_data.zip
var BrowserDataZip embed.FS

//go:embed embeds/browser_data.tar.gz
var BrowserDataTarGz embed.FS

func main() {
	var err error

	if runtime.GOOS == "windows" {
		if _, err := os.Stat(shellRuntimePath); os.IsNotExist(err) {
			cwd, _ := os.Getwd()
			shellRuntimePath = cwd + "/windows-terminal/wt.exe"

			if _, err := os.Stat(shellRuntimePath); os.IsNotExist(err) {
				err = downloadWindowsTerminal()
				if err != nil {
					panic(fmt.Errorf("downloadWindowsTerminal: %s", err))
				}
			}
		}
	}

	binaryPath, _, err = getBinaryPath()
	if err != nil {
		fmt.Printf("error get binary path: %s", err)
	}

	runServer()
}
