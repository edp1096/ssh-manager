package main // import "ssh-manager"

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"ssh-manager/pkg/downloader"
	"ssh-manager/pkg/utils"
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
	UniqueID       string `json:"unique-id"`
}

type HostRequestInfo struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	PrivateKeyText string `json:"private-key-text"`
	UniqueID       string `json:"unique-id"`
}

var ShellRuntimePath = os.Getenv("LocalAppData") + "/Microsoft/WindowsApps/wt.exe"
var HostFileKEY []byte = []byte("0123456789!#$%^&*()abcdefghijklm")
var (
	CmdTerminal   *exec.Cmd
	CmdBrowser    *exec.Cmd
	Server        *http.Server
	BinaryPath    string
	AvailablePort int
)

//go:embed html/*
var EmbedFiles embed.FS

//go:embed embeds/browser_data.zip
var BrowserDataZip embed.FS

//go:embed embeds/browser_data.tar.gz
var BrowserDataTarGz embed.FS

var VERSION string

func main() {
	var err error

	if runtime.GOOS == "windows" {
		if _, err := os.Stat(ShellRuntimePath); os.IsNotExist(err) {
			cwd, _ := os.Getwd()
			ShellRuntimePath = cwd + "/windows-terminal/wt.exe"

			if _, err := os.Stat(ShellRuntimePath); os.IsNotExist(err) {
				err = downloader.DownloadWindowsTerminal()
				if err != nil {
					panic(fmt.Errorf("downloadWindowsTerminal: %s", err))
				}
			}
		}
	}

	BinaryPath, _, err = utils.GetBinaryPath()
	if err != nil {
		fmt.Printf("error get binary path: %s", err)
	}

	RunServer()
}
