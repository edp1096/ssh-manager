package terminal

import (
	"os/exec"
	"sync"
)

type SshClientArgument struct {
	HostsFile     string `json:"hosts-file"`
	CategoryIndex int    `json:"category-index"`
	HostIndex     int    `json:"host-index"`
	NewWindow     bool
	HostFileKEY   []byte
	SplitVertical bool
	RelayAddress  string  `json:"-"`
	RelayToken    string  `json:"-"`
	BatchWindow   string  `json:"-"`
	GridColumns   int     `json:"-"`
	GridVertical  bool    `json:"-"`
	GridTarget    int     `json:"-"` // One-based launch index of the pane to split.
	SplitSize     float64 `json:"-"`
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

var launchMu sync.Mutex

func OpenSession(arg SshClientArgument) error {
	launchMu.Lock()
	defer launchMu.Unlock()
	_, err := openTerminal(arg)
	return err
}

func OpenBatch(args []SshClientArgument) (int, error) {
	launchMu.Lock()
	defer launchMu.Unlock()
	return openBatch(args)
}
