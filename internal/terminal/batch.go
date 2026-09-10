package terminal

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

func batchID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return "ssh-manager-batch-" + hex.EncodeToString(id[:])
}

func launchBatch(args []SshClientArgument, launch func(SshClientArgument) (int, error)) (int, error) {
	if len(args) == 0 {
		return 0, fmt.Errorf("no hosts selected")
	}
	id := batchID()
	for i, arg := range args {
		arg.NewWindow = i == 0
		arg.BatchWindow = id
		if _, err := launch(arg); err != nil {
			return i, err
		}
	}
	return len(args), nil
}

// One WT invocation targets a fresh named window; it never uses the user's
// active window. Command separators are generated here, not from host names.
func windowsBatchArguments(client string, args []SshClientArgument) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no hosts selected")
	}
	result := []string{"-w", batchID()}
	for i, arg := range args {
		if strings.ContainsAny(client+arg.HostsFile, ";\r\n") {
			return nil, fmt.Errorf("Windows Terminal batch paths cannot contain semicolons or newlines")
		}
		if i == 0 {
			result = append(result, "new-tab")
		} else {
			if arg.GridColumns > 0 {
				if arg.GridTarget < 1 || arg.GridTarget > i {
					return nil, fmt.Errorf("invalid grid target")
				}
				// A new split already focuses its new pane. Avoid redundant
				// navigation when the next split targets that same pane.
				if arg.GridTarget != i {
					result = append(result, ";", "move-focus", "first")
					for n := 1; n < arg.GridTarget; n++ {
						result = append(result, ";", "move-focus", "nextInOrder")
					}
				}
			}
			orientation := "-V"
			if arg.SplitVertical {
				orientation = "-H"
			}
			result = append(result, ";", "split-pane", orientation)
			if arg.GridColumns > 0 {
				result = append(result, "--size", strconv.FormatFloat(arg.SplitSize, 'f', 8, 64))
			}
		}
		result = append(result, client, "-f", arg.HostsFile, "-k", base64.URLEncoding.EncodeToString(arg.HostFileKEY), "-ci", strconv.Itoa(arg.CategoryIndex), "-hi", strconv.Itoa(arg.HostIndex))
		result = append(result, relayArguments(arg)...)
	}
	// Conservative bound includes quoting/UTF-16 expansion below the Windows limit.
	if len(strings.Join(result, " ")) > 14000 {
		return nil, fmt.Errorf("too many hosts for one Windows Terminal command; select fewer hosts")
	}
	return result, nil
}
