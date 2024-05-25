package terminal

import (
	"fmt"
	"os/exec"
)

type SshClientArgument struct {
	HostsFile     string `json:"hosts-file"`
	CategoryIndex int    `json:"category-index"`
	HostIndex     int    `json:"host-index"`
	NewWindow     bool
	HostFileKEY   []byte
	SplitVertical bool
}

var (
	WorkingDir       string
	ShellRuntimePath string
	CmdTerminal      *exec.Cmd
)

func OpenSession(arg SshClientArgument) {
	_, err := openTerminal(arg)
	if err != nil {
		fmt.Println("Error open terminal:", err)
	}
}
