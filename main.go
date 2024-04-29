package main // import "my-ssh-manager"

import (
	_ "embed"
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
	Password       string `json:"password"`
	PrivateKeyFile string `json:"private-key-file"`
	PrivateKeyText string `json:"private-key-text"`
}

var shellRuntimePath = os.Getenv("LocalAppData") + "/Microsoft/WindowsApps/wt.exe"
var (
	cmd        *exec.Cmd
	server     *http.Server
	binaryPath string
)

//go:embed index.html
var html string

func main() {
	var err error

	binaryPath, _, err = getBinaryPath()
	if err != nil {
		fmt.Printf("error get binary path: %s", err)
	}

	runServer()
}
