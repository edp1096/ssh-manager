package main // import "ssh-manager"

import (
	"embed"
	"fmt"
	"os/exec"

	"ssh-manager/pkg/utils"
)

var HostFileKEY []byte = []byte("0123456789!#$%^&*()abcdefghijklm")
var (
	CmdBrowser *exec.Cmd
	WorkingDir string
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

	WorkingDir, _, err = utils.GetCWD()
	if err != nil {
		fmt.Printf("error get binary path: %s", err)
	}
	if VERSION == "dev" {
		fmt.Println("WorkingDir:", WorkingDir)
	}

	CheckTerminalExist()
	RunServer()
}
