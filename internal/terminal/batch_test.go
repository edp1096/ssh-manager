package terminal

import (
	"fmt"
	"strings"
	"testing"
)

func TestBatchOrderAndFailure(t *testing.T) {
	args := []SshClientArgument{{HostIndex: 1}, {HostIndex: 2}, {HostIndex: 3}}
	var seen []SshClientArgument
	n, err := launchBatch(args, func(arg SshClientArgument) (int, error) {
		seen = append(seen, arg)
		if arg.HostIndex == 2 {
			return 0, fmt.Errorf("split failed")
		}
		return 1, nil
	})
	if n != 1 || err == nil || len(seen) != 2 || !seen[0].NewWindow || seen[1].NewWindow || seen[0].BatchWindow == "" || seen[0].BatchWindow != seen[1].BatchWindow {
		t.Fatal(n, err, seen)
	}
	if args[0].BatchWindow != "" || args[0].NewWindow {
		t.Fatal("mutated caller arguments")
	}
}
func TestWindowsBatchHasDedicatedWindow(t *testing.T) {
	args := []SshClientArgument{{HostsFile: `C:\test data\hosts.dat`, HostIndex: 1, CategoryIndex: 1}, {HostsFile: `C:\test data\hosts.dat`, HostIndex: 2, CategoryIndex: 1, SplitVertical: true}}
	result, err := windowsBatchArguments(`C:\test app\ssh-client.exe`, args)
	if err != nil {
		t.Fatal(err)
	}
	if result[0] != "-w" || !strings.HasPrefix(result[1], "ssh-manager-batch-") || result[2] != "new-tab" {
		t.Fatal("wrong window selection")
	}
	if !strings.Contains(strings.Join(result, "|"), ";|split-pane|-H|") {
		t.Fatal("missing split")
	}
	if _, err = windowsBatchArguments(`C:\bad;path\ssh-client.exe`, args); err == nil {
		t.Fatal("accepted WT separator in path")
	}
}
