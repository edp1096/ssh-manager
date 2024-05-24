package main // import "ssh-manager"

import (
	"embed"

	"ssh-manager/internal/server"
	"ssh-manager/internal/terminal"
)

var HostFileKEY []byte = []byte("0123456789!#$%^&*()abcdefghijklm")

//go:embed html/*
var EmbedFiles embed.FS

//go:embed embeds/browser_data.zip embeds/browser_data.tar.gz
var BrowserData embed.FS

var VERSION string

func main() {
	data := server.InitData{
		HostFileKEY: HostFileKEY,
		EmbedFiles:  EmbedFiles,
		BrowserData: BrowserData,
		Version:     VERSION,
	}

	terminal.CheckTerminalExist()
	server.RunServer(data)
}
