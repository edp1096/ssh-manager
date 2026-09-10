package terminal

import (
	"fmt"
	"strconv"
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

func TestWindowsGridTargetsCreationIDs(t *testing.T) {
	for _, fill := range []string{"horizontal", "vertical", "vertical-left"} {
		for count := 3; count <= 32; count++ {
			for columns := 1; columns <= count; columns++ {
				args := make([]SshClientArgument, count)
				plan, err := PlanGrid(args, columns, fill)
				if err != nil {
					t.Fatal(err)
				}
				command, err := windowsBatchArguments("ssh-client.exe", plan)
				if err != nil {
					t.Fatal(err)
				}
				active, created := 0, 1
				for i := 0; i < len(command); i++ {
					switch command[i] {
					case "move-focus":
						t.Fatal("grid must not depend on tree traversal order")
					case "focus-pane":
						if command[i+1] != "--target" {
							t.Fatal("missing target")
						}
						active, err = strconv.Atoi(command[i+2])
						if err != nil || active < 0 || active >= created {
							t.Fatal("invalid pane ID")
						}
					case "split-pane":
						if active != plan[created].GridTarget-1 {
							t.Fatalf("%s %d hosts/%d columns: split %d targets ID %d, expected %d", fill, count, columns, created, active, plan[created].GridTarget-1)
						}
						want := "-V"
						if plan[created].SplitVertical {
							want = "-H"
						}
						if command[i+1] != want {
							t.Fatal("wrong split direction")
						}
						active = created
						created++
					}
				}
				if created != count {
					t.Fatal("missing panes")
				}
			}
		}
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
