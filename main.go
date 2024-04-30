package main // import "my-ssh-manager"

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
)

type HostInfo struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string
	PrivateKeyFile string `json:"private-key-file"`
	PrivateKeyText string `json:"private-key-text"`
}

type HostRequestInfo struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Address        string `json:"address"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	PrivateKeyFile string `json:"private-key-file"`
	PrivateKeyText string `json:"private-key-text"`
}

var shellRuntimePath = os.Getenv("LocalAppData") + "/Microsoft/WindowsApps/wt.exe"
var (
	cmdTerminal *exec.Cmd
	cmdBrowser  *exec.Cmd
	server      *http.Server
	binaryPath  string
)

//go:embed html/*
var embedFiles embed.FS

func main() {
	var err error

	binaryPath, _, err = getBinaryPath()
	if err != nil {
		fmt.Printf("error get binary path: %s", err)
	}

	runServer()
}
