package main

import (
	"fmt"
	"os/exec"
)

type SshArgument struct {
	HostsFile     string `json:"hosts-file"`
	CategoryIndex int    `json:"category-index"`
	HostIndex     int    `json:"host-index"`
	NewWindow     bool
}

var CmdTerminal *exec.Cmd
var ShellRuntimePath string

func openSession(arg SshArgument) {
	_, err := openTerminal(arg)
	if err != nil {
		fmt.Println("Error open terminal:", err)
	}
}
