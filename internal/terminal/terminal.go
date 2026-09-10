package terminal

import (
	"os/exec"
)

type SshClientArgument struct {
	HostsFile     string `json:"hosts-file"`
	CategoryIndex int    `json:"category-index"`
	HostIndex     int    `json:"host-index"`
	NewWindow     bool
	HostFileKEY   []byte
	SplitVertical bool
	RelayAddress  string `json:"-"`
	RelayToken    string `json:"-"`
}

func relayArguments(arg SshClientArgument) []string {
	if arg.RelayAddress == "" {
		return nil
	}
	return []string{"-relay-address", arg.RelayAddress, "-relay-token", arg.RelayToken}
}

var (
	WorkingDir       string
	ShellRuntimePath string
	CmdTerminal      *exec.Cmd
)

func OpenSession(arg SshClientArgument) error {
	_, err := openTerminal(arg)
	return err
}
