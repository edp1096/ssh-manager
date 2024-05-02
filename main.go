package main // import "my-ssh-manager"

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
)

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
)

//go:embed html/*
var embedFiles embed.FS

//go:embed tmux.conf
var tmuxConf embed.FS

func init() {
	if runtime.GOOS != "windows" {
		exportTmuxConf()
	}
}

func main() {
	var err error

	binaryPath, _, err = getBinaryPath()
	if err != nil {
		fmt.Printf("error get binary path: %s", err)
	}

	runServer()
}
